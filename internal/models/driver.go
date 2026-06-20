package models

import "time"

type Driver struct {
	ID        int64     `json:"id" gorm:"column:id;primaryKey"`
	Name      string    `json:"name" gorm:"column:name;type:varchar(64);not null"`
	Phone     string    `json:"phone" gorm:"column:phone;type:varchar(20);uniqueIndex;not null"`
	Balance   float64   `json:"balance" gorm:"column:balance;type:decimal(18,2);not null;default:0"`
	Frozen    float64   `json:"frozen" gorm:"column:frozen;type:decimal(18,2);not null;default:0"`
	BankCard  string    `json:"bank_card" gorm:"column:bank_card;type:varchar(32);not null"`
	BankName  string    `json:"bank_name" gorm:"column:bank_name;type:varchar(64);not null"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (Driver) TableName() string {
	return "drivers"
}
