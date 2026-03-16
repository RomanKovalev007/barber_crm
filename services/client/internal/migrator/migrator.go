package migrator

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Migrator struct {
	pool *pgxpool.Pool
	fs   embed.FS
}

func New(pool *pgxpool.Pool, fs embed.FS) *Migrator {
	return &Migrator{pool: pool, fs: fs}
}

func (m *Migrator) Up(ctx context.Context) error {
	entries, err := m.fs.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if _, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, f := range files {
		version := strings.TrimSuffix(f, ".up.sql")

		var exists bool
		err := m.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		sql, err := m.fs.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		if _, err := m.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}

		if _, err := m.pool.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1)`, version,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
	}

	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	entries, err := m.fs.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".down.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	for _, f := range files {
		sql, err := m.fs.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := m.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply down migration %s: %w", f, err)
		}
		version := strings.TrimSuffix(f, ".down.sql")
		if _, err := m.pool.Exec(ctx,
			`DELETE FROM schema_migrations WHERE version = $1`, version,
		); err != nil {
			return fmt.Errorf("remove migration record %s: %w", version, err)
		}
	}

	return nil
}
