package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/migrator"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up | down")
	flag.Parse()

	log := logger.New("migrate")

	cfg, err := config.ParseStaffConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPostgres(ctx, cfg.DbCfg)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	m := migrator.New(pool, log)

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
