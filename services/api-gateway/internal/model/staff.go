package model

type Barber struct {
	ID           string `json:"barber_id"`
	Name         string `json:"name"`
	Services     []Service `json:"services"`
}

type Service struct {
	ID              string `json:"service_id"`
	Name            string `json:"name"`
	Price           int32 `json:"price"`
	DurationMinutes int32 `json:"duration_minutes"`
	IsActive        bool `json:"is_active"`
}

type PartOfDay string

const (
	PartOfDayAM PartOfDay = "am"
	PartOfDayPM PartOfDay = "pm"
)

type ScheduleDay struct {
	ID        string `json:"schedule_day_id"`
	BarberID  string `json:"barber_id"`
	Date      string `json:"date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	PartOfDay PartOfDay `json:"part_of_day"`
}