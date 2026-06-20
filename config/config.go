package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type MySQLConfig struct {
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

type WithdrawConfig struct {
	MinAmount       float64 `yaml:"min_amount"`
	MaxAmount       float64 `yaml:"max_amount"`
	DailyLimit      int     `yaml:"daily_limit"`
	RedisTTLSeconds int     `yaml:"redis_ttl_seconds"`
}

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	MySQL    MySQLConfig    `yaml:"mysql"`
	Redis    RedisConfig    `yaml:"redis"`
	Withdraw WithdrawConfig `yaml:"withdraw"`
}

var AppConfig *Config

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file error: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file error: %w", err)
	}
	AppConfig = &cfg
	return &cfg, nil
}

func (c *Config) GetConnMaxLifetime() time.Duration {
	return time.Duration(c.MySQL.ConnMaxLifetime) * time.Second
}

func (c *Config) GetRedisTTL() time.Duration {
	return time.Duration(c.Withdraw.RedisTTLSeconds) * time.Second
}
