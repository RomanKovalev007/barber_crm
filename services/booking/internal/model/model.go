package model

import "time"

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusNoShow    = "no_show"
)

// FinalStatuses — статусы, из которых нельзя перейти в другой.
var FinalStatuses = map[string]bool{
	StatusCompleted: true,
	StatusCancelled: true,
	StatusNoShow:    true,
}

type Booking struct {
	ID          string
	ClientName  string
	ClientPhone string
	BarberID    string
	ServiceID   string
	ServiceName string
	Price       int32     // цена услуги на момент записи (руб.)
	Date        time.Time // хранится в БД для индексации; производное от TimeStart
	TimeStart   time.Time
	TimeEnd     time.Time
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SlotStatus int32

const (
	SlotUnknown SlotStatus = 0
	SlotFree    SlotStatus = 1
	SlotBooked  SlotStatus = 2
	SlotBlocked SlotStatus = 3
)

type SlotBooking struct {
	BookingID   string
	ClientName  string
	ClientPhone string
	ServiceName string
}

type Slot struct {
	Status    SlotStatus
	TimeStart time.Time
	TimeEnd   time.Time
	Booking   *SlotBooking // non-nil только если Status == SlotBooked
}

type SlotsResult struct {
	BarberID string
	Date     string // YYYY-MM-DD
	Slots    []Slot
}

type BarberSettings struct {
	BarberID            string
	CompactSlotsEnabled bool
}
