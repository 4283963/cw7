package repositories

import (
	"context"
	"errors"
	"fmt"

	"cw7/internal/models"

	"gorm.io/gorm"
)

type BankStatementRepository interface {
	BatchInsert(ctx context.Context, list []models.BankStatement) error
	GetByBankRefNo(ctx context.Context, refNo string) (*models.BankStatement, error)
	ListByBatchNo(ctx context.Context, batchNo string) ([]models.BankStatement, error)
	ListByDate(ctx context.Context, date string) ([]models.BankStatement, error)
	MarkMatched(ctx context.Context, stmtID int64, withdrawID int64) error
	UpdateMatchedBatch(ctx context.Context, matchedMap map[int64]int64) error
}

type bankStmtRepo struct {
	db *gorm.DB
}

func NewBankStatementRepository(db *gorm.DB) BankStatementRepository {
	return &bankStmtRepo{db: db}
}

func (r *bankStmtRepo) BatchInsert(ctx context.Context, list []models.BankStatement) error {
	if len(list) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&list).Error; err != nil {
		return fmt.Errorf("batch insert bank statements error: %w", err)
	}
	return nil
}

func (r *bankStmtRepo) GetByBankRefNo(ctx context.Context, refNo string) (*models.BankStatement, error) {
	var s models.BankStatement
	err := r.db.WithContext(ctx).Where("bank_ref_no = ?", refNo).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get bank statement by ref error: %w", err)
	}
	return &s, nil
}

func (r *bankStmtRepo) ListByBatchNo(ctx context.Context, batchNo string) ([]models.BankStatement, error) {
	var list []models.BankStatement
	err := r.db.WithContext(ctx).
		Where("batch_no = ?", batchNo).
		Order("trx_time ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list bank statements by batch error: %w", err)
	}
	return list, nil
}

func (r *bankStmtRepo) ListByDate(ctx context.Context, date string) ([]models.BankStatement, error) {
	var list []models.BankStatement
	err := r.db.WithContext(ctx).
		Where("trx_date = ?", date).
		Order("trx_time ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list bank statements by date error: %w", err)
	}
	return list, nil
}

func (r *bankStmtRepo) MarkMatched(ctx context.Context, stmtID int64, withdrawID int64) error {
	res := r.db.WithContext(ctx).Model(&models.BankStatement{}).
		Where("id = ?", stmtID).
		Updates(map[string]interface{}{
			"matched":    true,
			"matched_id": withdrawID,
		})
	if res.Error != nil {
		return fmt.Errorf("mark matched error: %w", res.Error)
	}
	return nil
}

func (r *bankStmtRepo) UpdateMatchedBatch(ctx context.Context, matchedMap map[int64]int64) error {
	if len(matchedMap) == 0 {
		return nil
	}
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin tx error: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	for stmtID, withdrawID := range matchedMap {
		res := tx.Model(&models.BankStatement{}).
			Where("id = ?", stmtID).
			Updates(map[string]interface{}{
				"matched":    true,
				"matched_id": withdrawID,
			})
		if res.Error != nil {
			tx.Rollback()
			return fmt.Errorf("update matched stmt %d error: %w", stmtID, res.Error)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit tx error: %w", err)
	}
	return nil
}
