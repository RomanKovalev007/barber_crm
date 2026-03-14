package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/RomanKovalev007/barber_crm/services/analytics/migrations"
)

// schema_migrations — append-only таблица.
// Rollback не удаляет строку, а добавляет direction='down'.
// Текущее состояние версии = argMax(direction, applied_at).
const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations
(
    version    UInt64,
    name       String,
    direction  String,
    applied_at DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree()
PARTITION BY tuple()
ORDER BY (applied_at, version)`

type migration struct {
	version uint64
	name    string
	sql     string
}

type appliedVersion struct {
	version uint64
	name    string
}

type Migrator struct {
	db  driver.Conn
	log *slog.Logger
}

func New(db driver.Conn, log *slog.Logger) *Migrator {
	return &Migrator{db: db, log: log}
}

// Up применяет все незатронутые миграции.
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
		if err := m.db.Exec(ctx, mig.sql); err != nil {
			return fmt.Errorf("apply %03d_%s: %w", mig.version, mig.name, err)
		}
		if err := m.record(ctx, mig.version, mig.name, "up"); err != nil {
			return err
		}
		m.log.Info("applied", slog.Uint64("version", mig.version), slog.String("name", mig.name))
	}

	return nil
}

// Down откатывает последнюю применённую миграцию.
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
	if err := m.db.Exec(ctx, strings.TrimSpace(string(content))); err != nil {
		return fmt.Errorf("rollback %03d_%s: %w", last.version, last.name, err)
	}
	if err := m.record(ctx, last.version, last.name, "down"); err != nil {
		return err
	}
	m.log.Info("rolled back", slog.Uint64("version", last.version), slog.String("name", last.name))

	return nil
}

// ─── internal ────────────────────────────────────────────────────────────────

func (m *Migrator) ensureTable(ctx context.Context) error {
	if err := m.db.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	return nil
}

// appliedVersions возвращает версии, у которых последняя запись direction='up'.
func (m *Migrator) appliedVersions(ctx context.Context) ([]appliedVersion, error) {
	rows, err := m.db.Query(ctx, `
		SELECT version, name
		FROM (
			SELECT version, name, argMax(direction, applied_at) AS last_direction
			FROM schema_migrations
			GROUP BY version, name
		)
		WHERE last_direction = 'up'
		ORDER BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	var result []appliedVersion
	for rows.Next() {
		var a appliedVersion
		if err := rows.Scan(&a.version, &a.name); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// loadMigrations читает embedded SQL-файлы и возвращает те, что ещё не применены.
func (m *Migrator) loadMigrations(direction string, applied []appliedVersion) ([]migration, error) {
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
			return nil // уже применена
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

func (m *Migrator) record(ctx context.Context, version uint64, name, direction string) error {
	return m.db.Exec(ctx,
		`INSERT INTO schema_migrations (version, name, direction) VALUES (?, ?, ?)`,
		version, name, direction,
	)
}

// parseFilename разбирает "001_create_bookings.up.sql" → (1, "create_bookings").
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
