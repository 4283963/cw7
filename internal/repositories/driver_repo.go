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
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("begin tx error: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var d models.Driver
	err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", id).First(&d).Error
	if err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock driver error: %w", err)
	}
	if d.Balance < amount {
		tx.Rollback()
		return nil, errors.New("insufficient balance")
	}
	newBalance := d.Balance - amount
	newFrozen := d.Frozen + amount
	res := tx.Model(&d).Where("id = ?", id).Updates(map[string]interface{}{
		"balance": newBalance,
		"frozen":  newFrozen,
	})
	if res.Error != nil {
		tx.Rollback()
		return nil, fmt.Errorf("update driver balance error: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return nil, errors.New("update driver balance affected 0 rows")
	}
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("commit tx error: %w", err)
	}
	d.Balance = newBalance
	d.Frozen = newFrozen
	return &d, nil
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
