package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type ClientConfig struct {
	GRPCPort string `env:"GRPC_PORT" env-default:":50053"`
	DbCfg    PostgresConfig
	KafkaCfg KafkaConfig
}

func ParseClientConfig() (*ClientConfig, error) {
	var cfg ClientConfig
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config from env: %w", err)
	}
	cfg.DbCfg.DSN = cfg.DbCfg.FormatConnectionString()
	return &cfg, nil
}
