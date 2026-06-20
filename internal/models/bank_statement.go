package models

import "time"

const (
	BankFlowTypeCredit = 1
	BankFlowTypeDebit  = 2
)

type BankStatement struct {
	ID          int64     `json:"id" gorm:"column:id;primaryKey"`
	BatchNo     string    `json:"batch_no" gorm:"column:batch_no;type:varchar(32);index;not null"`
	BankRefNo   string    `json:"bank_ref_no" gorm:"column:bank_ref_no;type:varchar(64);uniqueIndex;not null"`
	TrxDate     string    `json:"trx_date" gorm:"column:trx_date;type:varchar(10);index;not null"`
	TrxTime     time.Time `json:"trx_time" gorm:"column:trx_time;not null"`
	FlowType    int       `json:"flow_type" gorm:"column:flow_type;not null"`
	Amount      float64   `json:"amount" gorm:"column:amount;type:decimal(18,2);not null"`
	Balance     float64   `json:"balance" gorm:"column:balance;type:decimal(18,2);not null;default:0"`
	OppBankCard string    `json:"opp_bank_card" gorm:"column:opp_bank_card;type:varchar(32);index;not null"`
	OppName     string    `json:"opp_name" gorm:"column:opp_name;type:varchar(64);not null"`
	Summary     string    `json:"summary" gorm:"column:summary;type:varchar(255)"`
	Matched     bool      `json:"matched" gorm:"column:matched;not null;default:false"`
	MatchedID   *int64    `json:"matched_id,omitempty" gorm:"column:matched_id;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (BankStatement) TableName() string {
	return "bank_statements"
}
