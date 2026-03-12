package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("booking not found")

type BookingRepo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *BookingRepo {
	return &BookingRepo{pool: pool}
}

func (r *BookingRepo) CreateBooking(ctx context.Context, b *model.Booking) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bookings (id, client_name, client_phone, barber_id, service_id, service_name, date, time_start, time_end, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		b.ID, b.ClientName, b.ClientPhone, b.BarberID, b.ServiceID, b.ServiceName,
		b.Date, b.TimeStart, b.TimeEnd, b.Status,
	)
	return err
}

func (r *BookingRepo) GetBooking(ctx context.Context, id string) (*model.Booking, error) {
	var b model.Booking
	err := r.pool.QueryRow(ctx, `
		SELECT id, client_name, client_phone, barber_id, service_id, service_name,
		       date, time_start, time_end, status, created_at, updated_at
		FROM bookings WHERE id = $1`, id,
	).Scan(
		&b.ID, &b.ClientName, &b.ClientPhone, &b.BarberID, &b.ServiceID, &b.ServiceName,
		&b.Date, &b.TimeStart, &b.TimeEnd, &b.Status, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *BookingRepo) UpdateBookingDetails(ctx context.Context, id, serviceID, serviceName string, timeStart, timeEnd time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE bookings
		SET service_id=$2, service_name=$3, time_start=$4, time_end=$5,
		    date=$6, updated_at=NOW()
		WHERE id=$1`,
		id, serviceID, serviceName, timeStart, timeEnd, timeStart.UTC().Truncate(24*time.Hour),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookingRepo) UpdateBookingStatus(ctx context.Context, id, status string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE bookings SET status=$2, updated_at=NOW() WHERE id=$1`,
		id, status,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookingRepo) DeleteBooking(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM bookings WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookingRepo) GetBookingsByBarberAndDate(ctx context.Context, barberID string, date time.Time) ([]model.Booking, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, client_name, client_phone, barber_id, service_id, service_name,
		       date, time_start, time_end, status, created_at, updated_at
		FROM bookings
		WHERE barber_id=$1 AND date=$2 AND status NOT IN ($3,$4)
		ORDER BY time_start`,
		barberID, date, model.StatusCancelled, model.StatusNoShow,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(
			&b.ID, &b.ClientName, &b.ClientPhone, &b.BarberID, &b.ServiceID, &b.ServiceName,
			&b.Date, &b.TimeStart, &b.TimeEnd, &b.Status, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func (r *BookingRepo) HasActiveBooking(ctx context.Context, clientPhone string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE client_phone=$1 AND status=$2
		)`, clientPhone, model.StatusPending,
	).Scan(&exists)
	return exists, err
}
