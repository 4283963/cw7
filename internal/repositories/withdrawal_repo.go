package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cw7/config"
	"cw7/internal/models"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	withdrawRedisPrefix  = "withdraw:today:"
	driverDailyCntPrefix = "withdraw:daily_cnt:"
)

type WithdrawalRepository interface {
	Create(ctx context.Context, w *models.Withdrawal) error
	GetByWithdrawNo(ctx context.Context, no string) (*models.Withdrawal, error)
	GetByDriverAndDate(ctx context.Context, driverID int64, date string) ([]models.Withdrawal, error)
	ListPendingByDate(ctx context.Context, date string) ([]models.Withdrawal, error)
	UpdateStatus(ctx context.Context, id int64, status int, reason string) error
	MarkReconciled(ctx context.Context, ids []int64) error
	CacheTodayWithdrawal(ctx context.Context, w *models.Withdrawal) error
	GetCachedWithdrawals(ctx context.Context, date string) ([]models.Withdrawal, error)
	IncrDriverDailyCount(ctx context.Context, driverID int64, date string) (int64, error)
	GetDriverDailyCount(ctx context.Context, driverID int64, date string) (int64, error)
}

type withdrawalRepo struct {
	db  *gorm.DB
	rdb *redis.Client
	ttl time.Duration
}

func NewWithdrawalRepository(db *gorm.DB, rdb *redis.Client) WithdrawalRepository {
	return &withdrawalRepo{
		db:  db,
		rdb: rdb,
		ttl: config.AppConfig.GetRedisTTL(),
	}
}

func (r *withdrawalRepo) Create(ctx context.Context, w *models.Withdrawal) error {
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("create withdrawal error: %w", err)
	}
	return nil
}

func (r *withdrawalRepo) GetByWithdrawNo(ctx context.Context, no string) (*models.Withdrawal, error) {
	var w models.Withdrawal
	err := r.db.WithContext(ctx).Where("withdraw_no = ?", no).First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get withdrawal by no error: %w", err)
	}
	return &w, nil
}

func (r *withdrawalRepo) GetByDriverAndDate(ctx context.Context, driverID int64, date string) ([]models.Withdrawal, error) {
	var list []models.Withdrawal
	err := r.db.WithContext(ctx).
		Where("driver_id = ? AND apply_date = ?", driverID, date).
		Order("apply_time DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("get driver withdrawals error: %w", err)
	}
	return list, nil
}

func (r *withdrawalRepo) ListPendingByDate(ctx context.Context, date string) ([]models.Withdrawal, error) {
	var list []models.Withdrawal
	err := r.db.WithContext(ctx).
		Where("apply_date = ? AND status IN ?", date, []int{
			models.WithdrawStatusPending,
			models.WithdrawStatusSuccess,
		}).
		Order("apply_time ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list pending withdrawals error: %w", err)
	}
	return list, nil
}

func (r *withdrawalRepo) UpdateStatus(ctx context.Context, id int64, status int, reason string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if reason != "" {
		updates["fail_reason"] = reason
	}
	if status == models.WithdrawStatusSuccess || status == models.WithdrawStatusFailed {
		now := time.Now()
		updates["finish_time"] = &now
	}
	res := r.db.WithContext(ctx).Model(&models.Withdrawal{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update withdrawal status error: %w", res.Error)
	}
	return nil
}

func (r *withdrawalRepo) MarkReconciled(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&models.Withdrawal{}).
		Where("id IN ?", ids).
		Update("status", models.WithdrawStatusReconciled)
	if res.Error != nil {
		return fmt.Errorf("mark reconciled error: %w", res.Error)
	}
	return nil
}

func (r *withdrawalRepo) CacheTodayWithdrawal(ctx context.Context, w *models.Withdrawal) error {
	key := withdrawRedisPrefix + w.ApplyDate
	data, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("marshal withdrawal error: %w", err)
	}
	pipe := r.rdb.TxPipeline()
	pipe.LPush(ctx, key, data)
	pipe.Expire(ctx, key, r.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache withdrawal redis error: %w", err)
	}
	return nil
}

func (r *withdrawalRepo) GetCachedWithdrawals(ctx context.Context, date string) ([]models.Withdrawal, error) {
	key := withdrawRedisPrefix + date
	rawList, err := r.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("get cached withdrawals error: %w", err)
	}
	result := make([]models.Withdrawal, 0, len(rawList))
	for _, raw := range rawList {
		var w models.Withdrawal
		if err := json.Unmarshal([]byte(raw), &w); err != nil {
			continue
		}
		result = append(result, w)
	}
	return result, nil
}

func (r *withdrawalRepo) IncrDriverDailyCount(ctx context.Context, driverID int64, date string) (int64, error) {
	key := fmt.Sprintf("%s%d:%s", driverDailyCntPrefix, driverID, date)
	pipe := r.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, r.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("incr daily count error: %w", err)
	}
	return incr.Val(), nil
}

func (r *withdrawalRepo) GetDriverDailyCount(ctx context.Context, driverID int64, date string) (int64, error) {
	key := fmt.Sprintf("%s%d:%s", driverDailyCntPrefix, driverID, date)
	cnt, err := r.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("get daily count error: %w", err)
	}
	return cnt, nil
}
