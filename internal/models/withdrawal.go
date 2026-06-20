package models

import "time"

const (
	WithdrawStatusPending    = 1
	WithdrawStatusSuccess    = 2
	WithdrawStatusFailed     = 3
	WithdrawStatusReconciled = 4
)

type Withdrawal struct {
	ID           int64      `json:"id" gorm:"column:id;primaryKey"`
	WithdrawNo   string     `json:"withdraw_no" gorm:"column:withdraw_no;type:varchar(32);uniqueIndex;not null"`
	DriverID     int64      `json:"driver_id" gorm:"column:driver_id;index:idx_driver_date;not null"`
	Amount       float64    `json:"amount" gorm:"column:amount;type:decimal(18,2);not null"`
	Fee          float64    `json:"fee" gorm:"column:fee;type:decimal(18,2);not null;default:0"`
	ActualAmount float64    `json:"actual_amount" gorm:"column:actual_amount;type:decimal(18,2);not null"`
	Status       int        `json:"status" gorm:"column:status;index;not null;default:1"`
	BankCard     string     `json:"bank_card" gorm:"column:bank_card;type:varchar(32);not null"`
	BankName     string     `json:"bank_name" gorm:"column:bank_name;type:varchar(64);not null"`
	FailReason   string     `json:"fail_reason,omitempty" gorm:"column:fail_reason;type:varchar(255)"`
	ApplyDate    string     `json:"apply_date" gorm:"column:apply_date;type:varchar(10);index:idx_driver_date;not null"`
	ApplyTime    time.Time  `json:"apply_time" gorm:"column:apply_time;not null"`
	FinishTime   *time.Time `json:"finish_time,omitempty" gorm:"column:finish_time"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (Withdrawal) TableName() string {
	return "withdrawals"
}
