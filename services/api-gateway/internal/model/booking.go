package model

import "time"

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusNoShow    = "no_show"
)

type Booking struct {
	ID          string    `json:"booking_id"`
	ClientName  string    `json:"client_name"`
	ClientPhone string    `json:"client_phone"`
	BarberID    string    `json:"barber_id"`
	ServiceID   string    `json:"service_id"`
	ServiceName string    `json:"service_name"`
	Price       int32     `json:"price"`
	TimeStart   time.Time `json:"time_start"`
	TimeEnd     time.Time `json:"time_end"`
	Status      string    `json:"status"`
}

type SlotStatus string

const (
	SlotFree    SlotStatus = "free"
	SlotBooked  SlotStatus = "booked"
	SlotBlocked SlotStatus = "blocked"
)

type SlotBooking struct {
	BookingID   string `json:"booking_id"`
	ClientName  string `json:"client_name"`
	ClientPhone string `json:"client_phone"`
	ServiceName string `json:"service_name"`
}

type Slot struct {
	Status    SlotStatus   `json:"status"`
	TimeStart time.Time    `json:"time_start"`
	TimeEnd   time.Time    `json:"time_end"`
	Booking   *SlotBooking `json:"booking,omitempty"`
}
