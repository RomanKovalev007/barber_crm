package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/migrator"
	"github.com/RomanKovalev007/barber_crm/services/client/migrations"
)

func main() {
	log := logger.New("client-migrate")
	cfg, err := config.ParseClientConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := postgres.NewPostgres(ctx, cfg.DbCfg)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	m := migrator.New(pool, migrations.FS)

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	switch direction {
	case "up":
		if err := m.Up(ctx); err != nil {
			log.Error("migration up failed", "error", err)
			os.Exit(1)
		}
		log.Info("migrations applied successfully")
	case "down":
		if err := m.Down(ctx); err != nil {
			log.Error("migration down failed", "error", err)
			os.Exit(1)
		}
		log.Info("migrations rolled back successfully")
	default:
		log.Error("unknown direction", slog.String("direction", direction))
		os.Exit(1)
	}
}
