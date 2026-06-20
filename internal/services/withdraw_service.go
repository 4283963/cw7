package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"cw7/config"
	"cw7/internal/models"
	"cw7/internal/repositories"
	"cw7/pkg/snowflake"
)

type ApplyRequest struct {
	DriverID int64   `json:"driver_id" binding:"required,min=1"`
	Amount   float64 `json:"amount" binding:"required"`
}

type ApplyResult struct {
	WithdrawNo   string  `json:"withdraw_no"`
	Amount       float64 `json:"amount"`
	Fee          float64 `json:"fee"`
	ActualAmount float64 `json:"actual_amount"`
	Status       int     `json:"status"`
	ApplyTime    string  `json:"apply_time"`
}

var ErrCircuitOpen = errors.New("withdraw circuit breaker is open")

type WithdrawService interface {
	Apply(ctx context.Context, req *ApplyRequest) (*ApplyResult, error)
	GetByNo(ctx context.Context, no string) (*models.Withdrawal, error)
	ListByDriver(ctx context.Context, driverID int64, date string) ([]models.Withdrawal, error)
	ListCachedToday(ctx context.Context, date string) ([]models.Withdrawal, error)
}

type withdrawService struct {
	driverRepo     repositories.DriverRepository
	withdrawalRepo repositories.WithdrawalRepository
	circuitSvc     CircuitBreakerService
	idGen          *snowflake.Snowflake
	cfg            *config.WithdrawConfig
}

func NewWithdrawService(
	driverRepo repositories.DriverRepository,
	withdrawalRepo repositories.WithdrawalRepository,
	circuitSvc CircuitBreakerService,
	idGen *snowflake.Snowflake,
	cfg *config.WithdrawConfig,
) WithdrawService {
	return &withdrawService{
		driverRepo:     driverRepo,
		withdrawalRepo: withdrawalRepo,
		circuitSvc:     circuitSvc,
		idGen:          idGen,
		cfg:            cfg,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func calcFee(amount float64) float64 {
	return round2(amount * 0.001)
}

func todayStr() string {
	return time.Now().Format("2006-01-02")
}

func (s *withdrawService) Apply(ctx context.Context, req *ApplyRequest) (*ApplyResult, error) {
	if s.circuitSvc != nil {
		open, msg, err := s.circuitSvc.IsOpen(ctx)
		if err != nil {
			fmt.Printf("warn: check circuit breaker error: %v\n", err)
		}
		if open {
			return nil, fmt.Errorf("%s: %w", msg, ErrCircuitOpen)
		}
	}
	if req == nil {
		return nil, errors.New("empty request")
	}
	amount := round2(req.Amount)
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive: %w", errors.New("invalid amount"))
	}
	if amount < s.cfg.MinAmount {
		return nil, fmt.Errorf("amount %.2f less than min %.2f", amount, s.cfg.MinAmount)
	}
	if amount > s.cfg.MaxAmount {
		return nil, fmt.Errorf("amount %.2f exceed max %.2f", amount, s.cfg.MaxAmount)
	}

	date := todayStr()
	cnt, err := s.withdrawalRepo.GetDriverDailyCount(ctx, req.DriverID, date)
	if err != nil {
		return nil, fmt.Errorf("check daily count error: %w", err)
	}
	if cnt >= int64(s.cfg.DailyLimit) {
		return nil, fmt.Errorf("daily withdraw limit %d reached", s.cfg.DailyLimit)
	}

	driver, err := s.driverRepo.GetByID(ctx, req.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, fmt.Errorf("driver %d not found", req.DriverID)
	}
	if driver.BankCard == "" {
		return nil, errors.New("driver bank card not bind")
	}

	updated, err := s.driverRepo.UpdateBalanceWithLock(ctx, req.DriverID, amount)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("driver not found during lock")
	}

	fee := calcFee(amount)
	actual := round2(amount - fee)
	now := time.Now()
	withdrawNo := fmt.Sprintf("WD%s%014d", now.Format("20060102150405"), s.idGen.NextID()%100000000000000)

	w := &models.Withdrawal{
		ID:           s.idGen.NextID(),
		WithdrawNo:   withdrawNo,
		DriverID:     req.DriverID,
		Amount:       amount,
		Fee:          fee,
		ActualAmount: actual,
		Status:       models.WithdrawStatusPending,
		BankCard:     driver.BankCard,
		BankName:     driver.BankName,
		ApplyDate:    date,
		ApplyTime:    now,
	}

	if err := s.withdrawalRepo.Create(ctx, w); err != nil {
		_ = s.driverRepo.RestoreBalance(ctx, req.DriverID, amount)
		return nil, fmt.Errorf("create withdrawal record error: %w", err)
	}

	if _, err := s.withdrawalRepo.IncrDriverDailyCount(ctx, req.DriverID, date); err != nil {
		// 非致命错误，仅记录
		fmt.Printf("warn: incr daily count error: %v\n", err)
	}

	if err := s.withdrawalRepo.CacheTodayWithdrawal(ctx, w); err != nil {
		fmt.Printf("warn: cache today withdrawal error: %v\n", err)
	}

	return &ApplyResult{
		WithdrawNo:   withdrawNo,
		Amount:       amount,
		Fee:          fee,
		ActualAmount: actual,
		Status:       models.WithdrawStatusPending,
		ApplyTime:    now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *withdrawService) GetByNo(ctx context.Context, no string) (*models.Withdrawal, error) {
	return s.withdrawalRepo.GetByWithdrawNo(ctx, no)
}

func (s *withdrawService) ListByDriver(ctx context.Context, driverID int64, date string) ([]models.Withdrawal, error) {
	if date == "" {
		date = todayStr()
	}
	return s.withdrawalRepo.GetByDriverAndDate(ctx, driverID, date)
}

func (s *withdrawService) ListCachedToday(ctx context.Context, date string) ([]models.Withdrawal, error) {
	if date == "" {
		date = todayStr()
	}
	return s.withdrawalRepo.GetCachedWithdrawals(ctx, date)
}
