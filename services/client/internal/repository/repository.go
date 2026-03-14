package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RomanKovalev007/barber_crm/services/client/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("client not found")

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// UpsertByBooking atomically deduplicates by booking_id and upserts the client record.
// If the booking_id was already processed, it is a no-op.
func (r *Repository) UpsertByBooking(ctx context.Context, barberID, phone, name, bookingID string, lastVisit time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Deduplication: mark event as processed. ON CONFLICT DO NOTHING means already seen.
	tag, err := tx.Exec(ctx,
		`INSERT INTO client_processed_events (booking_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		bookingID,
	)
	if err != nil {
		return fmt.Errorf("dedup insert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already processed — idempotent skip
		return tx.Commit(ctx)
	}

	// Upsert client: create or increment visit counter
	_, err = tx.Exec(ctx, `
		INSERT INTO clients (id, barber_id, phone, name, notes, visits_count, last_visit, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, '', 1, $4, NOW(), NOW())
		ON CONFLICT (barber_id, phone) DO UPDATE
		SET name         = EXCLUDED.name,
		    visits_count = clients.visits_count + 1,
		    last_visit   = GREATEST(clients.last_visit, EXCLUDED.last_visit),
		    updated_at   = NOW()`,
		barberID, phone, name, lastVisit,
	)
	if err != nil {
		return fmt.Errorf("upsert client: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.Client, error) {
	var c model.Client
	err := r.pool.QueryRow(ctx, `
		SELECT id, barber_id, phone, name, notes, visits_count, last_visit, created_at, updated_at
		FROM clients WHERE id = $1`, id,
	).Scan(&c.ID, &c.BarberID, &c.Phone, &c.Name, &c.Notes, &c.VisitsCount,
		&c.LastVisit, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) GetByPhone(ctx context.Context, barberID, phone string) (*model.Client, error) {
	var c model.Client
	err := r.pool.QueryRow(ctx, `
		SELECT id, barber_id, phone, name, notes, visits_count, last_visit, created_at, updated_at
		FROM clients WHERE barber_id = $1 AND phone = $2`, barberID, phone,
	).Scan(&c.ID, &c.BarberID, &c.Phone, &c.Name, &c.Notes, &c.VisitsCount,
		&c.LastVisit, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) List(ctx context.Context, barberID, search string) ([]model.Client, error) {
	var (
		query string
		args  []any
	)
	if search == "" {
		query = `
			SELECT id, barber_id, phone, name, notes, visits_count, last_visit, created_at, updated_at
			FROM clients WHERE barber_id = $1
			ORDER BY name`
		args = []any{barberID}
	} else {
		query = `
			SELECT id, barber_id, phone, name, notes, visits_count, last_visit, created_at, updated_at
			FROM clients WHERE barber_id = $1 AND (name ILIKE $2 OR phone ILIKE $2)
			ORDER BY name`
		args = []any{barberID, "%" + search + "%"}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []model.Client
	for rows.Next() {
		var c model.Client
		if err := rows.Scan(&c.ID, &c.BarberID, &c.Phone, &c.Name, &c.Notes, &c.VisitsCount,
			&c.LastVisit, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan client: %w", err)
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// Update overwrites name and notes. Returns the updated client.
func (r *Repository) Update(ctx context.Context, id, name, notes string) (*model.Client, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE clients SET name = $2, notes = $3, updated_at = NOW() WHERE id = $1`,
		id, name, notes,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id)
}
