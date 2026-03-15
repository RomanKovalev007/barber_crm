package model

type TopService struct {
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	Count       int64  `json:"count"`
	Revenue     int64  `json:"revenue"`
}

type DayStat struct {
	Date        string  `json:"date"`
	Clients     int64   `json:"clients"`
	Revenue     int64   `json:"revenue"`
	HoursWorked float64 `json:"hours_worked"`
}

type BarberStats struct {
	BarberID          string       `json:"barber_id"`
	DateFrom          string       `json:"date_from"`
	DateTo            string       `json:"date_to"`
	ClientsServed     int64        `json:"clients_served"`
	TotalRevenue      int64        `json:"total_revenue"`
	HoursWorked       float64      `json:"hours_worked"`
	AverageCheck      float64      `json:"average_check"`
	BookingsTotal     int64        `json:"bookings_total"`
	BookingsCompleted int64        `json:"bookings_completed"`
	BookingsCancelled int64        `json:"bookings_cancelled"`
	BookingsNoShow    int64        `json:"bookings_no_show"`
	OccupancyRate     float64      `json:"occupancy_rate"`
	TopServices       []TopService `json:"top_services"`
	DailyBreakdown    []DayStat    `json:"daily_breakdown"`
}
