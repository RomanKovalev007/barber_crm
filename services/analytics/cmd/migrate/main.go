package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/migrator"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up | down")
	flag.Parse()

	log := logger.New("migrate")

	cfg, err := config.ParseAnalyticsConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.CHCfg.Host + ":" + cfg.CHCfg.Port},
		Auth: clickhouse.Auth{
			Database: cfg.CHCfg.Database,
			Username: cfg.CHCfg.Username,
			Password: cfg.CHCfg.Password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Error("failed to open clickhouse", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.Ping(ctx); err != nil {
		log.Error("failed to ping clickhouse", "error", err)
		os.Exit(1)
	}

	m := migrator.New(conn, log)

	switch *direction {
	case "up":
		err = m.Up(ctx)
	case "down":
		err = m.Down(ctx)
	default:
		log.Error("unknown direction, use: up | down", slog.String("got", *direction))
		os.Exit(1)
	}

	if err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}

	log.Info("done", slog.String("direction", *direction))
}
