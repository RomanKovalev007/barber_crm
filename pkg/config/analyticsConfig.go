package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)


type AnalyticsConfig struct {
	GRPCPort string `env:"GRPC_PORT" env-default:":50052"`
	CHCfg    ClickHouseConfig
	KafkaCfg KafkaConfig
}

func ParseAnalyticsConfig() (*AnalyticsConfig, error) {
	var cfg AnalyticsConfig
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config from env: %w", err)
	}
	return &cfg, nil
}
