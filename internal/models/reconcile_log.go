package models

import "time"

const (
	DiffTypeSystemOnly     = "system_only"
	DiffTypeBankOnly       = "bank_only"
	DiffTypeAmountMismatch = "amount_mismatch"
	DiffTypeCardMismatch   = "card_mismatch"
)

type ReconcileLog struct {
	ID              int64      `json:"id" gorm:"column:id;primaryKey"`
	BatchNo         string     `json:"batch_no" gorm:"column:batch_no;type:varchar(32);uniqueIndex;not null"`
	ReconcileDate   string     `json:"reconcile_date" gorm:"column:reconcile_date;type:varchar(10);index;not null"`
	TotalSystem     int        `json:"total_system" gorm:"column:total_system;not null"`
	TotalBank       int        `json:"total_bank" gorm:"column:total_bank;not null"`
	MatchedCount    int        `json:"matched_count" gorm:"column:matched_count;not null"`
	SystemTotalAmt  float64    `json:"system_total_amt" gorm:"column:system_total_amt;type:decimal(18,2);not null"`
	BankTotalAmt    float64    `json:"bank_total_amt" gorm:"column:bank_total_amt;type:decimal(18,2);not null"`
	MatchedTotalAmt float64    `json:"matched_total_amt" gorm:"column:matched_total_amt;type:decimal(18,2);not null"`
	DiffCount       int        `json:"diff_count" gorm:"column:diff_count;not null"`
	Status          int        `json:"status" gorm:"column:status;not null;default:1"`
	CreatedAt       time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	FinishedAt      *time.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
}

func (ReconcileLog) TableName() string {
	return "reconcile_logs"
}

type ReconcileDiff struct {
	ID           int64    `json:"id" gorm:"column:id;primaryKey"`
	BatchNo      string   `json:"batch_no" gorm:"column:batch_no;type:varchar(32);index;not null"`
	DiffType     string   `json:"diff_type" gorm:"column:diff_type;type:varchar(32);index;not null"`
	WithdrawID   *int64   `json:"withdraw_id,omitempty" gorm:"column:withdraw_id;index"`
	WithdrawNo   *string  `json:"withdraw_no,omitempty" gorm:"column:withdraw_no;type:varchar(32)"`
	BankStmtID   *int64   `json:"bank_stmt_id,omitempty" gorm:"column:bank_stmt_id;index"`
	BankRefNo    *string  `json:"bank_ref_no,omitempty" gorm:"column:bank_ref_no;type:varchar(64)"`
	SystemAmount *float64 `json:"system_amount,omitempty" gorm:"column:system_amount;type:decimal(18,2)"`
	BankAmount   *float64 `json:"bank_amount,omitempty" gorm:"column:bank_amount;type:decimal(18,2)"`
	DiffAmount   *float64 `json:"diff_amount,omitempty" gorm:"column:diff_amount;type:decimal(18,2)"`
	SystemCard   *string  `json:"system_card,omitempty" gorm:"column:system_card;type:varchar(32)"`
	BankCard     *string  `json:"bank_card,omitempty" gorm:"column:bank_card;type:varchar(32)"`
	Remark       string   `json:"remark" gorm:"column:remark;type:varchar(500)"`
}

func (ReconcileDiff) TableName() string {
	return "reconcile_diffs"
}
