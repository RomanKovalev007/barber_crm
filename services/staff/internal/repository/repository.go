package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetBarberByLogin(ctx context.Context, login string) (*model.Barber, error) {
	var b model.Barber
	err := r.db.QueryRow(ctx,
		`SELECT id, name, login, password_hash, is_active
		 FROM barbers WHERE login = $1 AND is_active = true`, login).
		Scan(&b.ID, &b.Name, &b.Login, &b.PasswordHash, &b.IsActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("barber not found")
		}
		return nil, err
	}
	return &b, nil
}

func (r *Repository) GetBarber(ctx context.Context, id string) (*model.Barber, error) {
	var b model.Barber
	err := r.db.QueryRow(ctx,
		`SELECT id, name, login, is_active
		 FROM barbers WHERE id = $1`, id).
		Scan(&b.ID, &b.Name, &b.Login, &b.IsActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("barber not found")
		}
		return nil, err
	}
	services, err := r.ListServices(ctx, id, false)
	if err != nil {
		return nil, err
	}
	b.Services = services
	return &b, nil
}

func (r *Repository) ListBarbers(ctx context.Context) ([]model.Barber, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name
		 FROM barbers WHERE is_active = true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var barbers []model.Barber
	for rows.Next() {
		var b model.Barber
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		services, err := r.ListServices(ctx, b.ID, false)
		if err != nil {
			return nil, err
		}
		b.Services = services
		barbers = append(barbers, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return barbers, nil
}

func (r *Repository) ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error) {
	query := `SELECT id, barber_id, name, price, is_active
	          FROM services WHERE barber_id = $1`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY name`

	rows, err := r.db.Query(ctx, query, barberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.Service
	for rows.Next() {
		var s model.Service
		if err := rows.Scan(&s.ID, &s.BarberID, &s.Name, &s.Price, &s.IsActive); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return services, nil
}

func (r *Repository) CreateService(ctx context.Context, s *model.Service) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO services (barber_id, name, price)
		 VALUES ($1, $2, $3) RETURNING id`,
		s.BarberID, s.Name, s.Price).Scan(&s.ID)
}

func (r *Repository) UpdateService(ctx context.Context, s *model.Service) error {
	_, err := r.db.Exec(ctx,
		`UPDATE services SET name=$1, price=$2, is_active=$3
		 WHERE id=$4 AND barber_id=$5`,
		s.Name, s.Price, s.IsActive, s.ID, s.BarberID)
	return err
}

func (r *Repository) DeleteService(ctx context.Context, id, barberID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE services SET is_active = false WHERE id = $1 AND barber_id = $2`, id, barberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("service not found")
	}
	return nil
}

func (r *Repository) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, barber_id, date::text, COALESCE(start_time::text,''), COALESCE(end_time::text,'')
		 FROM schedule
		 WHERE barber_id = $1 AND date >= date_trunc('week', to_date($2 || '-1', 'IYYY-"W"IW-D'))
		   AND date < date_trunc('week', to_date($2 || '-1', 'IYYY-"W"IW-D')) + interval '7 days'
		 ORDER BY date`, barberID, week)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []model.ScheduleDay
	for rows.Next() {
		var d model.ScheduleDay
		if err := rows.Scan(&d.ID, &d.BarberID, &d.Date, &d.StartTime, &d.EndTime); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return days, nil
}

func (r *Repository) AddSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO schedule (barber_id, date, start_time, end_time)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (barber_id, date) DO UPDATE SET
			start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time
			RETURNING id`,
		barberID, day.Date, day.StartTime, day.EndTime).Scan(&day.ID)
	if err != nil {
		return nil, err
	}

	day.BarberID = barberID

	return day, nil
}
