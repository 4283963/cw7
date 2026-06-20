package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cw7/internal/models"
	"cw7/internal/repositories"
	"cw7/pkg/snowflake"

	"github.com/redis/go-redis/v9"
)

const (
	circuitBreakerKey    = "withdraw:circuit_breaker"
	circuitStatusOpen    = "1"
	circuitStatusClosed  = "0"
	circuitBatchPrefix   = "OPS"
	circuitClosedMessage = "银行系统维护中，提现通道临时关闭，请稍后再试"
)

type CircuitToggleRequest struct {
	Operator string `json:"operator" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
	Open     bool   `json:"open"`
}

type CircuitStatus struct {
	Open        bool   `json:"open"`
	Message     string `json:"message"`
	Operator    string `json:"operator,omitempty"`
	Reason      string `json:"reason,omitempty"`
	OperateTime string `json:"operate_time,omitempty"`
}

type CircuitBreakerService interface {
	IsOpen(ctx context.Context) (bool, string, error)
	GetStatus(ctx context.Context) (*CircuitStatus, error)
	Toggle(ctx context.Context, req *CircuitToggleRequest) (*CircuitStatus, error)
}

type circuitBreakerService struct {
	rdb           *redis.Client
	reconcileRepo repositories.ReconcileRepository
	idGen         *snowflake.Snowflake
}

func NewCircuitBreakerService(
	rdb *redis.Client,
	reconcileRepo repositories.ReconcileRepository,
	idGen *snowflake.Snowflake,
) CircuitBreakerService {
	return &circuitBreakerService{
		rdb:           rdb,
		reconcileRepo: reconcileRepo,
		idGen:         idGen,
	}
}

func (s *circuitBreakerService) IsOpen(ctx context.Context) (bool, string, error) {
	if s.rdb == nil {
		return false, "", nil
	}
	val, err := s.rdb.Get(ctx, circuitBreakerKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("check circuit breaker error: %w", err)
	}
	if val == circuitStatusOpen {
		return true, circuitClosedMessage, nil
	}
	return false, "", nil
}

func (s *circuitBreakerService) GetStatus(ctx context.Context) (*CircuitStatus, error) {
	open, _, err := s.IsOpen(ctx)
	if err != nil {
		return nil, err
	}
	status := &CircuitStatus{
		Open:    open,
		Message: circuitClosedMessage,
	}
	if !open {
		status.Message = "提现通道正常运行"
	}
	return status, nil
}

func (s *circuitBreakerService) Toggle(ctx context.Context, req *CircuitToggleRequest) (*CircuitStatus, error) {
	if req == nil {
		return nil, errors.New("empty request")
	}
	if s.rdb == nil {
		return nil, errors.New("redis is not available, cannot toggle circuit breaker")
	}
	targetVal := circuitStatusClosed
	if req.Open {
		targetVal = circuitStatusOpen
	}
	if err := s.rdb.Set(ctx, circuitBreakerKey, targetVal, 0).Err(); err != nil {
		return nil, fmt.Errorf("set circuit breaker error: %w", err)
	}
	if err := s.logOpsAction(ctx, req); err != nil {
		fmt.Printf("warn: log ops circuit breaker action error: %v\n", err)
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	msg := circuitClosedMessage
	if !req.Open {
		msg = "提现通道已恢复正常"
	}
	return &CircuitStatus{
		Open:        req.Open,
		Message:     msg,
		Operator:    req.Operator,
		Reason:      req.Reason,
		OperateTime: now,
	}, nil
}

func (s *circuitBreakerService) logOpsAction(ctx context.Context, req *CircuitToggleRequest) error {
	if s.reconcileRepo == nil {
		return errors.New("reconcile repo is nil")
	}
	now := time.Now()
	date := now.Format("2006-01-02")
	batchNo := fmt.Sprintf("%s%s%014d", circuitBatchPrefix, now.Format("20060102150405"), s.idGen.NextID()%100000000000000)
	action := "关闭"
	if !req.Open {
		action = "恢复"
	}
	remark := fmt.Sprintf("[最高级别运维操作] %s于%s %s提现通道，原因：%s",
		req.Operator, now.Format("2006-01-02 15:04:05"), action, req.Reason)

	log := &models.ReconcileLog{
		ID:              s.idGen.NextID(),
		BatchNo:         batchNo,
		ReconcileDate:   date,
		TotalSystem:     0,
		TotalBank:       0,
		MatchedCount:    0,
		SystemTotalAmt:  0,
		BankTotalAmt:    0,
		MatchedTotalAmt: 0,
		DiffCount:       1,
		Status:          models.ReconcileStatusOpsAction,
		CreatedAt:       now,
		FinishedAt:      &now,
	}
	if err := s.reconcileRepo.CreateLog(ctx, log); err != nil {
		return fmt.Errorf("create ops log error: %w", err)
	}
	diff := models.ReconcileDiff{
		ID:       s.idGen.NextID(),
		BatchNo:  batchNo,
		DiffType: models.DiffTypeOpsAction,
		Remark:   remark,
	}
	if err := s.reconcileRepo.BatchInsertDiffs(ctx, []models.ReconcileDiff{diff}); err != nil {
		return fmt.Errorf("insert ops diff error: %w", err)
	}
	return nil
}
