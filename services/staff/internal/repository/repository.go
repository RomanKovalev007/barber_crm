package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
)

var ErrNotFound = errors.New("not found")

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
	services, _, err := r.ListServices(ctx, id, false, 0, 0)
	if err != nil {
		return nil, err
	}
	b.Services = services
	return &b, nil
}

func (r *Repository) ListBarbers(ctx context.Context, limit, offset int) ([]model.Barber, int, error) {
	var total int

	if limit > 0 {
		// Get total count of active barbers
		if err := r.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM barbers WHERE is_active = true`).Scan(&total); err != nil {
			return nil, 0, err
		}

		// Paginated query using a subquery for barbers
		rows, err := r.db.Query(ctx,
			`SELECT b.id, b.name,
			        s.id, s.barber_id, s.name, s.price, s.duration_minutes, s.is_active
			 FROM (SELECT id, name FROM barbers WHERE is_active = true ORDER BY name LIMIT $1 OFFSET $2) b
			 LEFT JOIN services s ON s.barber_id = b.id AND s.is_active = true
			 ORDER BY b.name, s.name`,
			limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		var barbers []model.Barber
		index := map[string]int{}
		for rows.Next() {
			var bID, bName string
			var sID, sBarberID, sName *string
			var sPrice, sDuration *int
			var sIsActive *bool
			if err := rows.Scan(&bID, &bName, &sID, &sBarberID, &sName, &sPrice, &sDuration, &sIsActive); err != nil {
				return nil, 0, err
			}
			i, seen := index[bID]
			if !seen {
				barbers = append(barbers, model.Barber{ID: bID, Name: bName})
				i = len(barbers) - 1
				index[bID] = i
			}
			if sID != nil {
				barbers[i].Services = append(barbers[i].Services, model.Service{
					ID:              *sID,
					BarberID:        *sBarberID,
					Name:            *sName,
					Price:           *sPrice,
					DurationMinutes: *sDuration,
					IsActive:        *sIsActive,
				})
			}
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		return barbers, total, nil
	}

	// No pagination — return all
	rows, err := r.db.Query(ctx,
		`SELECT b.id, b.name,
		        s.id, s.barber_id, s.name, s.price, s.duration_minutes, s.is_active
		 FROM barbers b
		 LEFT JOIN services s ON s.barber_id = b.id AND s.is_active = true
		 WHERE b.is_active = true
		 ORDER BY b.name, s.name`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var barbers []model.Barber
	index := map[string]int{}
	for rows.Next() {
		var bID, bName string
		var sID, sBarberID, sName *string
		var sPrice, sDuration *int
		var sIsActive *bool
		if err := rows.Scan(&bID, &bName, &sID, &sBarberID, &sName, &sPrice, &sDuration, &sIsActive); err != nil {
			return nil, 0, err
		}
		i, seen := index[bID]
		if !seen {
			barbers = append(barbers, model.Barber{ID: bID, Name: bName})
			i = len(barbers) - 1
			index[bID] = i
		}
		if sID != nil {
			barbers[i].Services = append(barbers[i].Services, model.Service{
				ID:              *sID,
				BarberID:        *sBarberID,
				Name:            *sName,
				Price:           *sPrice,
				DurationMinutes: *sDuration,
				IsActive:        *sIsActive,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return barbers, len(barbers), nil
}

func (r *Repository) ListServices(ctx context.Context, barberID string, includeInactive bool, limit, offset int) ([]model.Service, int, error) {
	if limit > 0 {
		query := `SELECT id, barber_id, name, price, duration_minutes, is_active, COUNT(*) OVER() as total
		          FROM services WHERE barber_id = $1`
		if !includeInactive {
			query += ` AND is_active = true`
		}
		query += ` ORDER BY name LIMIT $2 OFFSET $3`

		rows, err := r.db.Query(ctx, query, barberID, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		var services []model.Service
		var total int
		for rows.Next() {
			var s model.Service
			if err := rows.Scan(&s.ID, &s.BarberID, &s.Name, &s.Price, &s.DurationMinutes, &s.IsActive, &total); err != nil {
				return nil, 0, err
			}
			services = append(services, s)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		return services, total, nil
	}

	// No pagination — return all
	query := `SELECT id, barber_id, name, price, duration_minutes, is_active
	          FROM services WHERE barber_id = $1`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY name`

	rows, err := r.db.Query(ctx, query, barberID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var services []model.Service
	for rows.Next() {
		var s model.Service
		if err := rows.Scan(&s.ID, &s.BarberID, &s.Name, &s.Price, &s.DurationMinutes, &s.IsActive); err != nil {
			return nil, 0, err
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return services, len(services), nil
}

func (r *Repository) GetService(ctx context.Context, id, barberID string) (*model.Service, error) {
	var s model.Service
	err := r.db.QueryRow(ctx,
		`SELECT id, barber_id, name, price, duration_minutes, is_active
		 FROM services WHERE id = $1 AND barber_id = $2`,
		id, barberID).Scan(&s.ID, &s.BarberID, &s.Name, &s.Price, &s.DurationMinutes, &s.IsActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) CreateService(ctx context.Context, s *model.Service) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO services (barber_id, name, price, duration_minutes)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		s.BarberID, s.Name, s.Price, s.DurationMinutes).Scan(&s.ID)
}

func (r *Repository) UpdateService(ctx context.Context, s *model.Service) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE services SET name=$1, price=$2, duration_minutes=$3, is_active=$4
		 WHERE id=$5 AND barber_id=$6`,
		s.Name, s.Price, s.DurationMinutes, s.IsActive, s.ID, s.BarberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteService(ctx context.Context, id, barberID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE services SET is_active = false WHERE id = $1 AND barber_id = $2`, id, barberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, barber_id, date::text, COALESCE(start_time::text,''), COALESCE(end_time::text,''), part_of_day
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
		if err := rows.Scan(&d.ID, &d.BarberID, &d.Date, &d.StartTime, &d.EndTime, &d.PartOfDay); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return days, nil
}

func (r *Repository) UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO schedule (barber_id, date, start_time, end_time, part_of_day)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (barber_id, date) DO UPDATE SET
			start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time, part_of_day = EXCLUDED.part_of_day
			RETURNING id`,
		barberID, day.Date, day.StartTime, day.EndTime, day.PartOfDay).Scan(&day.ID)
	if err != nil {
		return nil, err
	}
	day.BarberID = barberID
	return day, nil
}

func (r *Repository) UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error) {
	// Строим один INSERT ... VALUES ($1,$2,$3,$4,$5), ($6,...) ON CONFLICT DO UPDATE
	args := make([]any, 0, len(days)*5)
	valueClauses := make([]string, 0, len(days))
	for i, d := range days {
		base := i * 5
		valueClauses = append(valueClauses, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5))
		args = append(args, barberID, d.Date, d.StartTime, d.EndTime, d.PartOfDay)
	}
	query := `INSERT INTO schedule (barber_id, date, start_time, end_time, part_of_day)
		VALUES ` + strings.Join(valueClauses, ",") + `
		ON CONFLICT (barber_id, date) DO UPDATE SET
		  start_time = EXCLUDED.start_time,
		  end_time   = EXCLUDED.end_time,
		  part_of_day = EXCLUDED.part_of_day
		RETURNING id, barber_id, date::text, start_time::text, end_time::text, part_of_day
		ORDER BY date`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.ScheduleDay
	for rows.Next() {
		var d model.ScheduleDay
		if err := rows.Scan(&d.ID, &d.BarberID, &d.Date, &d.StartTime, &d.EndTime, &d.PartOfDay); err != nil {
			return nil, err
		}
		result = append(result, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) DeleteSchedule(ctx context.Context, barberID, date string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`DELETE FROM schedule WHERE barber_id = $1 AND date = $2 RETURNING id`,
		barberID, date).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}
	return id, nil
}
