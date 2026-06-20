package config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis(cfg *RedisConfig) error {
	if cfg == nil || cfg.Addr == "" {
		return errors.New("redis config is empty")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("ping redis error: %w", err)
	}
	if pong != "PONG" {
		return fmt.Errorf("unexpected redis pong: %s", pong)
	}
	RDB = rdb
	return nil
}
