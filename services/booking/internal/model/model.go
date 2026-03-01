package model

import "time"

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusNoShow    = "no_show"
)

type Booking struct {
	ID         string
	ClientName string
	BarberID   string
	ServID     string
	Date       time.Time
	TimeStart  time.Time
	TimeEnd    time.Time
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Slot struct {
	Status    SlotStatus
	TimeStart time.Time
	TimeEnd   time.Time
}

type SlotStatus int32

const (
	SlotUnknown SlotStatus = 0
	SlotFree    SlotStatus = 1
	SlotBooked  SlotStatus = 2
	SlotBlocked SlotStatus = 3
)

type SlotsResult struct {
	BarberID string
	Date     time.Time
	Slots    []Slot
	Priority int32
}

type DeleteResult struct {
	BookingID  string
	ClientName string
}
