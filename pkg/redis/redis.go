package redis

import (
	"context"
	"fmt"

	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/redis/go-redis/v9"
)

func NewRedis(ctx context.Context, cfg config.RedisConfig) (*redis.Client, int, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost + ":" + cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, 0, fmt.Errorf("ping redis: %w", err)
	}

	ttl := cfg.RedisTtlMinute

	return client, ttl, nil
}