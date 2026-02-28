package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/model"
)


type Repository struct {
	db driver.Conn
}

func New(db driver.Conn) *Repository {
	return &Repository{db: db}
}

// ─── Write ───────────────────────────────────────────────────────────────────

func (r *Repository) InsertBooking(ctx context.Context, b *model.Booking) error {
	return r.db.Exec(ctx,
		`INSERT INTO bookings
			(booking_id, barber_id, client_phone, client_name, service_id, service_name,
			 price, start_time, end_time, status, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.BookingID, b.BarberID, b.ClientPhone, b.ClientName, b.ServiceID, b.ServiceName,
		b.Price, b.StartTime, b.EndTime, b.Status, b.OccurredAt,
	)
}

func (r *Repository) InsertSchedule(ctx context.Context, s *model.Schedule) error {
	date, err := time.Parse("2006-01-02", s.Date)
	if err != nil {
		return fmt.Errorf("parse date %q: %w", s.Date, err)
	}
	isDeleted := uint8(0)
	if s.IsDeleted {
		isDeleted = 1
	}
	return r.db.Exec(ctx,
		`INSERT INTO schedule_hours
			(schedule_id, barber_id, date, start_time, end_time, is_deleted, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ScheduleID, s.BarberID, date, s.StartTime, s.EndTime, isDeleted, s.OccurredAt,
	)
}

// ─── Read ────────────────────────────────────────────────────────────────────

// GetBookingStats возвращает агрегированные метрики по записям барбера за период.
func (r *Repository) GetBookingStats(ctx context.Context, barberID, from, to string) (*model.BookingStats, error) {
	dateClause, dateArgs := dateFilter("toDate(start_time)", from, to)
	args := append([]any{barberID}, dateArgs...)

	query := `
		SELECT
			countIf(status = 'completed')                                              AS clients_served,
			sumIf(price, status = 'completed')                                         AS total_revenue,
			count()                                                                    AS bookings_total,
			countIf(status = 'completed')                                              AS bookings_completed,
			countIf(status = 'cancelled')                                              AS bookings_cancelled,
			countIf(status = 'no_show')                                                AS bookings_no_show,
			sumIf(dateDiff('minute', start_time, end_time), status = 'completed')      AS booked_minutes
		FROM bookings FINAL
		WHERE barber_id = ?` + dateClause

	var s model.BookingStats
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&s.ClientsServed,
		&s.TotalRevenue,
		&s.BookingsTotal,
		&s.BookingsCompleted,
		&s.BookingsCancelled,
		&s.BookingsNoShow,
		&s.BookedMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("get booking stats: %w", err)
	}
	return &s, nil
}

// GetScheduleMinutes возвращает суммарное рабочее время барбера в минутах за период.
func (r *Repository) GetScheduleMinutes(ctx context.Context, barberID, from, to string) (float64, error) {
	dateClause, dateArgs := dateFilter("date", from, to)
	args := append([]any{barberID}, dateArgs...)

	query := `
		SELECT coalesce(sum(
			(toUInt32(substring(end_time, 1, 2)) * 60 + toUInt32(substring(end_time, 4, 2))) -
			(toUInt32(substring(start_time, 1, 2)) * 60 + toUInt32(substring(start_time, 4, 2)))
		), 0) AS total_minutes
		FROM schedule_hours FINAL
		WHERE barber_id = ? AND is_deleted = 0` + dateClause

	var totalMinutes float64
	if err := r.db.QueryRow(ctx, query, args...).Scan(&totalMinutes); err != nil {
		return 0, fmt.Errorf("get schedule minutes: %w", err)
	}
	return totalMinutes, nil
}

// GetTopServices возвращает топ-10 услуг по числу завершённых записей.
func (r *Repository) GetTopServices(ctx context.Context, barberID, from, to string) ([]model.TopService, error) {
	dateClause, dateArgs := dateFilter("toDate(start_time)", from, to)
	args := append([]any{barberID}, dateArgs...)

	query := `
		SELECT
			service_id,
			any(service_name)                  AS service_name,
			countIf(status = 'completed')      AS count,
			sumIf(price, status = 'completed') AS revenue
		FROM bookings FINAL
		WHERE barber_id = ?` + dateClause + `
		GROUP BY service_id
		ORDER BY count DESC
		LIMIT 10`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get top services: %w", err)
	}
	defer rows.Close()

	var result []model.TopService
	for rows.Next() {
		var s model.TopService
		if err := rows.Scan(&s.ServiceID, &s.ServiceName, &s.Count, &s.Revenue); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetDailyBreakdown возвращает разбивку по дням: клиенты, выручка, часы работы.
func (r *Repository) GetDailyBreakdown(ctx context.Context, barberID, from, to string) ([]model.DayStat, error) {
	dateClause, dateArgs := dateFilter("toDate(start_time)", from, to)
	bookingArgs := append([]any{barberID}, dateArgs...)

	bookingQuery := `
		SELECT
			toDate(start_time)                     AS date,
			countIf(status = 'completed')          AS clients,
			sumIf(price, status = 'completed')     AS revenue
		FROM bookings FINAL
		WHERE barber_id = ?` + dateClause + `
		GROUP BY date
		ORDER BY date`

	scheduleDateClause, scheduleDateArgs := dateFilter("date", from, to)
	scheduleArgs := append([]any{barberID}, scheduleDateArgs...)

	scheduleQuery := `
		SELECT
			date,
			(toUInt32(substring(end_time, 1, 2)) * 60 + toUInt32(substring(end_time, 4, 2))) -
			(toUInt32(substring(start_time, 1, 2)) * 60 + toUInt32(substring(start_time, 4, 2))) AS minutes
		FROM schedule_hours FINAL
		WHERE barber_id = ? AND is_deleted = 0` + scheduleDateClause + `
		ORDER BY date`

	dayMap := make(map[string]*model.DayStat)

	// Записи
	bookingRows, err := r.db.Query(ctx, bookingQuery, bookingArgs...)
	if err != nil {
		return nil, fmt.Errorf("daily bookings: %w", err)
	}
	defer bookingRows.Close()

	for bookingRows.Next() {
		var date time.Time
		var stat model.DayStat
		if err := bookingRows.Scan(&date, &stat.Clients, &stat.Revenue); err != nil {
			return nil, err
		}
		stat.Date = date.Format("2006-01-02")
		dayMap[stat.Date] = &stat
	}
	if err := bookingRows.Err(); err != nil {
		return nil, err
	}

	// Рабочие часы
	scheduleRows, err := r.db.Query(ctx, scheduleQuery, scheduleArgs...)
	if err != nil {
		return nil, fmt.Errorf("daily schedule: %w", err)
	}
	defer scheduleRows.Close()

	for scheduleRows.Next() {
		var date time.Time
		var minutes float64
		if err := scheduleRows.Scan(&date, &minutes); err != nil {
			return nil, err
		}
		dateStr := date.Format("2006-01-02")
		if _, ok := dayMap[dateStr]; !ok {
			dayMap[dateStr] = &model.DayStat{Date: dateStr}
		}
		dayMap[dateStr].HoursWorked = minutes / 60.0
	}
	if err := scheduleRows.Err(); err != nil {
		return nil, err
	}

	result := make([]model.DayStat, 0, len(dayMap))
	for _, s := range dayMap {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})
	return result, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// dateFilter возвращает SQL-фрагмент " AND <col> BETWEEN ? AND ?" и аргументы к нему.
// Если from пустой — период не ограничен (PERIOD_ALL), фрагмент пустой.
func dateFilter(col, from, to string) (string, []any) {
	if from == "" {
		return "", nil
	}
	return fmt.Sprintf(" AND %s BETWEEN ? AND ?", col), []any{from, to}
}
