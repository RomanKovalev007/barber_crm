package model

import "time"

type Booking struct {
	BookingID   string
	BarberID    string
	ClientPhone string
	ClientName  string
	ServiceID   string
	ServiceName string
	Price       int32
	StartTime   time.Time
	EndTime     time.Time
	Status      string // pending | completed | cancelled | no_show
	OccurredAt  time.Time
}

type Schedule struct {
	ScheduleID string
	BarberID   string
	Date       string // YYYY-MM-DD
	StartTime  string // HH:MM
	EndTime    string // HH:MM
	IsDeleted  bool
	OccurredAt time.Time
}

// BookingStats — агрегированные числовые метрики из таблицы bookings.
type BookingStats struct {
	ClientsServed     int64
	TotalRevenue      int64
	BookingsTotal     int64
	BookingsCompleted int64
	BookingsCancelled int64
	BookingsNoShow    int64
	BookedMinutes     float64 // сумма длительностей завершённых записей
}

type TopService struct {
	ServiceID   string
	ServiceName string
	Count       int64
	Revenue     int64
}

type DayStat struct {
	Date        string // YYYY-MM-DD
	Clients     int64
	Revenue     int64
	HoursWorked float64
}
