package config

import (
	"errors"
	"fmt"
	"time"

	"cw7/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitMySQL(cfg *MySQLConfig) error {
	if cfg == nil || cfg.DSN == "" {
		return errors.New("mysql config is empty")
	}
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("open mysql error: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db error: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping mysql error: %w", err)
	}
	DB = db
	return nil
}

func AutoMigrate() error {
	if DB == nil {
		return errors.New("mysql is not initialized")
	}
	return DB.AutoMigrate(
		&models.Driver{},
		&models.Withdrawal{},
		&models.BankStatement{},
		&models.ReconcileLog{},
		&models.ReconcileDiff{},
	)
}
