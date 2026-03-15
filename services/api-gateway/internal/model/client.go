package model

type Client struct {
	ID          string `json:"client_id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Notes       string `json:"notes"`
	VisitsCount int32  `json:"visits_count"`
	LastVisit   string `json:"last_visit,omitempty"` // YYYY-MM-DD
}
