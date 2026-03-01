package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type BookingConfig struct {
	GRPCPort    string `env:"GRPC_PORT" env-default:":50051"`
	JWTSecret   string `env:"JWT_SECRET" env-default:"jwt-secret"`
	DbCfg PostgresConfig
	RedisCfg RedisConfig
}

func ParseBookingConfig() (*BookingConfig, error) {
	var cfg BookingConfig

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config from env: %w", err)
	}

	cfg.DbCfg.DSN = cfg.DbCfg.FormatConnectionString()
	
	return &cfg, nil
}