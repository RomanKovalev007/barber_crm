package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgres(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	db, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	db.MaxConns = cfg.PGMaxConns
	db.MinConns = cfg.PGMinConns
	db.MaxConnLifetime = time.Duration(cfg.PGMaxConnLifetime) * time.Minute
	db.MaxConnIdleTime = time.Duration(cfg.PGMaxConnIdleTime) * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

