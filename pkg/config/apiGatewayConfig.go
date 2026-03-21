package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type ApiGatewayConfig struct {
	HTTPPort      string `env:"HTTP_PORT"       env-default:":8080"`
	MetricsPort   string `env:"METRICS_PORT"    env-default:":9095"`
	JWTSecret     string `env:"JWT_SECRET"      env-default:"jwt-secret"`
	StaffAddr     string `env:"STAFF_ADDR"      env-default:"localhost:50051"`
	BookingAddr   string `env:"BOOKING_ADDR"    env-default:"localhost:50052"`
	AnalyticsAddr string `env:"ANALYTICS_ADDR"  env-default:"localhost:50053"`
	ClientAddr    string `env:"CLIENT_ADDR"     env-default:"localhost:50054"`
}

func ParseApiGatewayConfig() (*ApiGatewayConfig, error) {
	var cfg ApiGatewayConfig
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse api-gateway config: %w", err)
	}
	return &cfg, nil
}
