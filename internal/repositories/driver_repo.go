package repositories

import (
	"context"
	"errors"
	"fmt"

	"cw7/internal/models"

	"gorm.io/gorm"
)

type DriverRepository interface {
	GetByID(ctx context.Context, id int64) (*models.Driver, error)
	UpdateBalanceWithLock(ctx context.Context, id int64, amount float64) (*models.Driver, error)
	RestoreBalance(ctx context.Context, id int64, amount float64) error
}

type driverRepo struct {
	db *gorm.DB
}

func NewDriverRepository(db *gorm.DB) DriverRepository {
	return &driverRepo{db: db}
}

func (r *driverRepo) GetByID(ctx context.Context, id int64) (*models.Driver, error) {
	var d models.Driver
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get driver by id error: %w", err)
	}
	return &d, nil
}

func (r *driverRepo) UpdateBalanceWithLock(ctx context.Context, id int64, amount float64) (*models.Driver, error) {
	res := r.db.WithContext(ctx).Model(&models.Driver{}).
		Where("id = ? AND balance >= ?", id, amount).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance - ?", amount),
			"frozen":  gorm.Expr("frozen + ?", amount),
		})
	if res.Error != nil {
		return nil, fmt.Errorf("deduct driver balance error: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		d, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if d == nil {
			return nil, fmt.Errorf("driver %d not found", id)
		}
		return nil, fmt.Errorf("insufficient balance: have %.2f, need %.2f", d.Balance, amount)
	}
	return r.GetByID(ctx, id)
}

func (r *driverRepo) RestoreBalance(ctx context.Context, id int64, amount float64) error {
	res := r.db.WithContext(ctx).Model(&models.Driver{}).
		Where("id = ? AND frozen >= ?", id, amount).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance + ?", amount),
			"frozen":  gorm.Expr("frozen - ?", amount),
		})
	if res.Error != nil {
		return fmt.Errorf("restore balance error: %w", res.Error)
	}
	return nil
}
