package handler

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var weekRegex = regexp.MustCompile(`^\d{4}-W(0[1-9]|[1-4]\d|5[0-3])$`)
var phoneRegex = regexp.MustCompile(`^\+7\d{10}$`)

func validateBarberID(barberID string) error {
	if barberID == "" {
		return errors.New("invalid barber_id")
	}
	if _, err := uuid.Parse(barberID); err != nil {
		return errors.New("invalid barber_id")
	}
	return nil
}

func validateServiceID(serviceID string) error {
	if serviceID == "" {
		return errors.New("invalid service_id")
	}
	if _, err := uuid.Parse(serviceID); err != nil {
		return errors.New("invalid service_id")
	}
	return nil
}

func validateBookingID(bookingID string) error {
	if bookingID == "" {
		return errors.New("invalid booking_id")
	}
	if _, err := uuid.Parse(bookingID); err != nil {
		return errors.New("invalid booking_id")
	}
	return nil
}

func validateClientID(clientID string) error {
	if clientID == "" {
		return errors.New("invalid client_id")
	}
	if _, err := uuid.Parse(clientID); err != nil {
		return errors.New("invalid client_id")
	}
	return nil
}

func validateClientName(clientName string) error {
	if clientName == "" {
		return errors.New("invalid client_name")
	}
	return nil
}

func validateClientPhone(clientPhone string) error {
	if clientPhone == "" {
		return errors.New("invalid client_phone")
	}
	if !phoneRegex.MatchString(clientPhone) {
		return errors.New("invalid client_phone")
	}
	return nil
}

func validateTime(t time.Time) error {
	if t.IsZero() {
		return errors.New("invalid time")
	}
	return nil
}

func validateDate(date string) error {
	if date == "" {
		return errors.New("invalid date")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("invalid date")
	}
	return nil
}

func isValidWeek(week string) bool {
	return weekRegex.MatchString(week)
}
