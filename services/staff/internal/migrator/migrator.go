package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RomanKovalev007/barber_crm/services/staff/migrations"
)

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations
(
    version    BIGINT      PRIMARY KEY,
    name       TEXT        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

type migration struct {
	version uint64
	name    string
	sql     string
}

type Migrator struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func New(db *pgxpool.Pool, log *slog.Logger) *Migrator {
	return &Migrator{db: db, log: log}
}

func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}

	pending, err := m.loadMigrations("up", applied)
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		m.log.Info("no pending migrations")
		return nil
	}

	for _, mig := range pending {
		m.log.Info("applying migration", slog.Uint64("version", mig.version), slog.String("name", mig.name))
		if err := m.applyInTx(ctx, mig); err != nil {
			return err
		}
		m.log.Info("applied", slog.Uint64("version", mig.version), slog.String("name", mig.name))
	}

	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		m.log.Info("nothing to roll back")
		return nil
	}

	last := applied[len(applied)-1]
	filename := fmt.Sprintf("%03d_%s.down.sql", last.version, last.name)

	content, err := migrations.FS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	m.log.Info("rolling back", slog.Uint64("version", last.version), slog.String("name", last.name))

	if err := m.rollbackInTx(ctx, last.version, strings.TrimSpace(string(content))); err != nil {
		return err
	}

	m.log.Info("rolled back", slog.Uint64("version", last.version), slog.String("name", last.name))
	return nil
}

// internal

func (m *Migrator) ensureTable(ctx context.Context) error {
	if _, err := m.db.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	return nil
}

func (m *Migrator) appliedVersions(ctx context.Context) ([]migration, error) {
	rows, err := m.db.Query(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	var result []migration
	for rows.Next() {
		var mig migration
		if err := rows.Scan(&mig.version, &mig.name); err != nil {
			return nil, err
		}
		result = append(result, mig)
	}
	return result, rows.Err()
}

func (m *Migrator) loadMigrations(direction string, applied []migration) ([]migration, error) {
	appliedSet := make(map[uint64]struct{}, len(applied))
	for _, a := range applied {
		appliedSet[a.version] = struct{}{}
	}

	suffix := "." + direction + ".sql"
	var result []migration

	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, suffix) {
			return err
		}

		version, name, err := parseFilename(path, direction)
		if err != nil {
			return err
		}

		if _, ok := appliedSet[version]; ok {
			return nil
		}

		content, err := migrations.FS.ReadFile(path)
		if err != nil {
			return err
		}

		result = append(result, migration{
			version: version,
			name:    name,
			sql:     strings.TrimSpace(string(content)),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].version < result[j].version
	})
	return result, nil
}

// applyInTx выполняет миграцию и запись в schema_migrations в одной транзакции.
func (m *Migrator) applyInTx(ctx context.Context, mig migration) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, mig.sql); err != nil {
		return fmt.Errorf("apply %03d_%s: %w", mig.version, mig.name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		mig.version, mig.name,
	); err != nil {
		return fmt.Errorf("record migration %d: %w", mig.version, err)
	}

	return tx.Commit(ctx)
}

// rollbackInTx выполняет rollback-SQL и удаляет запись из schema_migrations в одной транзакции.
func (m *Migrator) rollbackInTx(ctx context.Context, version uint64, sql string) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("rollback sql: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM schema_migrations WHERE version = $1`, version,
	); err != nil {
		return fmt.Errorf("delete migration record %d: %w", version, err)
	}

	return tx.Commit(ctx)
}

// parseFilename разбирает "001_create_barbers.up.sql" → (1, "create_barbers").
func parseFilename(path, direction string) (uint64, string, error) {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	base = strings.TrimSuffix(base, "."+direction+".sql")

	versionStr, name, ok := strings.Cut(base, "_")
	if !ok {
		return 0, "", fmt.Errorf("invalid migration filename: %s", path)
	}

	version, err := strconv.ParseUint(versionStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid version in %s: %w", path, err)
	}

	return version, name, nil
}
