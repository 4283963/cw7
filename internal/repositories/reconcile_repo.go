package repositories

import (
	"context"
	"fmt"
	"time"

	"cw7/internal/models"

	"gorm.io/gorm"
)

type ReconcileRepository interface {
	CreateLog(ctx context.Context, log *models.ReconcileLog) error
	UpdateLogFinished(ctx context.Context, batchNo string, matchedCount int, diffCount int, matchedAmt float64) error
	BatchInsertDiffs(ctx context.Context, diffs []models.ReconcileDiff) error
	ListDiffsByBatchNo(ctx context.Context, batchNo string) ([]models.ReconcileDiff, error)
	GetLogByBatchNo(ctx context.Context, batchNo string) (*models.ReconcileLog, error)
}

type reconcileRepo struct {
	db *gorm.DB
}

func NewReconcileRepository(db *gorm.DB) ReconcileRepository {
	return &reconcileRepo{db: db}
}

func (r *reconcileRepo) CreateLog(ctx context.Context, log *models.ReconcileLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create reconcile log error: %w", err)
	}
	return nil
}

func (r *reconcileRepo) UpdateLogFinished(ctx context.Context, batchNo string, matchedCount int, diffCount int, matchedAmt float64) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&models.ReconcileLog{}).
		Where("batch_no = ?", batchNo).
		Updates(map[string]interface{}{
			"matched_count":     matchedCount,
			"diff_count":        diffCount,
			"matched_total_amt": matchedAmt,
			"status":            2,
			"finished_at":       &now,
		})
	if res.Error != nil {
		return fmt.Errorf("update reconcile log error: %w", res.Error)
	}
	return nil
}

func (r *reconcileRepo) BatchInsertDiffs(ctx context.Context, diffs []models.ReconcileDiff) error {
	if len(diffs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&diffs).Error; err != nil {
		return fmt.Errorf("batch insert diffs error: %w", err)
	}
	return nil
}

func (r *reconcileRepo) ListDiffsByBatchNo(ctx context.Context, batchNo string) ([]models.ReconcileDiff, error) {
	var list []models.ReconcileDiff
	err := r.db.WithContext(ctx).
		Where("batch_no = ?", batchNo).
		Order("diff_type ASC, id ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list diffs by batch error: %w", err)
	}
	return list, nil
}

func (r *reconcileRepo) GetLogByBatchNo(ctx context.Context, batchNo string) (*models.ReconcileLog, error) {
	var log models.ReconcileLog
	err := r.db.WithContext(ctx).Where("batch_no = ?", batchNo).First(&log).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get reconcile log error: %w", err)
	}
	return &log, nil
}
