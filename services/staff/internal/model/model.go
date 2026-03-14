package model

import "time"

type Barber struct {
	ID           string
	Name         string
	Login        string
	PasswordHash string
	IsActive     bool
	Services     []Service
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Service struct {
	ID              string
	BarberID        string
	Name            string
	Price           int
	DurationMinutes int
	IsActive        bool
}

type PartOfDay string

const (
	PartOfDayAM PartOfDay = "am"
	PartOfDayPM PartOfDay = "pm"
)

type ScheduleDay struct {
	ID        string
	BarberID  string
	Date      string
	StartTime string
	EndTime   string
	PartOfDay PartOfDay
}
