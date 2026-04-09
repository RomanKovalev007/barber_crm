package model

import "time"

type Client struct {
	ID          string
	BarberID    string
	Phone       string
	Name        string
	Notes       string
	VisitsCount int32
	LastVisit   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
