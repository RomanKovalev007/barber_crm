package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/staffclient"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const slotDuration = 60 * time.Minute

var ErrActiveBookingExists = errors.New("client has an active booking")
var ErrInvalidStatusTransition = errors.New("invalid status transition")

type BookingIntr interface {
	CreateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error)
	GetBooking(ctx context.Context, id string) (*model.Booking, error)
	UpdateBookingDetails(ctx context.Context, bookingID, barberID, serviceID string, timeStart time.Time) (*model.Booking, error)
	UpdateBookingStatus(ctx context.Context, bookingID, barberID, newStatus string) (*model.Booking, error)
	DeleteBooking(ctx context.Context, id string) error
	GetSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error)
	GetFreeSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error)
}

type bookingService struct {
	log         *slog.Logger
	repo        *repo.BookingRepo
	redis       *redis.Client
	staffClient *staffclient.Client
}

func New(r *repo.BookingRepo, rc *redis.Client, ttl int, jwt string, log *slog.Logger, staffClient *staffclient.Client) BookingIntr {
	return &bookingService{
		log:         log,
		repo:        r,
		redis:       rc,
		staffClient: staffClient,
	}
}

func (s *bookingService) CreateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error) {
	if _, err := s.staffClient.GetBarber(ctx, b.BarberID); err != nil {
		s.log.Warn("create booking: barber not found", "barber_id", b.BarberID, "error", err)
		return nil, fmt.Errorf("barber not found: %w", err)
	}

	svcResp, err := s.staffClient.ListServices(ctx, b.BarberID, false)
	if err != nil {
		s.log.Error("create booking: failed to get services", "barber_id", b.BarberID, "error", err)
		return nil, fmt.Errorf("get services: %w", err)
	}
	serviceName := ""
	for _, svc := range svcResp.Services {
		if svc.ServiceId == b.ServiceID {
			serviceName = svc.Name
			break
		}
	}

	hasActive, err := s.repo.HasActiveBooking(ctx, b.ClientPhone)
	if err != nil {
		s.log.Error("create booking: failed to check active booking", "client_phone", b.ClientPhone, "error", err)
		return nil, fmt.Errorf("check active booking: %w", err)
	}
	if hasActive {
		s.log.Warn("create booking: client already has active booking", "client_phone", b.ClientPhone)
		return nil, ErrActiveBookingExists
	}

	b.TimeEnd = b.TimeStart.Add(slotDuration)
	b.Date = b.TimeStart.UTC().Truncate(24 * time.Hour)

	existing, err := s.repo.GetBookingsByBarberAndDate(ctx, b.BarberID, b.Date)
	if err != nil {
		s.log.Error("create booking: failed to get existing bookings", "barber_id", b.BarberID, "date", b.Date, "error", err)
		return nil, fmt.Errorf("get existing bookings: %w", err)
	}
	for _, e := range existing {
		if b.TimeStart.Before(e.TimeEnd) && b.TimeEnd.After(e.TimeStart) {
			s.log.Warn("create booking: slot conflict", "barber_id", b.BarberID, "time_start", b.TimeStart)
			return nil, fmt.Errorf("time slot already booked")
		}
	}

	b.ID = uuid.New().String()
	b.Status = model.StatusPending
	b.ServiceName = serviceName

	if err := s.repo.CreateBooking(ctx, b); err != nil {
		s.log.Error("create booking: failed to save", "barber_id", b.BarberID, "error", err)
		return nil, fmt.Errorf("create booking: %w", err)
	}

	s.log.Info("booking created", "booking_id", b.ID, "barber_id", b.BarberID, "client_phone", b.ClientPhone)
	return b, nil
}

func (s *bookingService) GetBooking(ctx context.Context, id string) (*model.Booking, error) {
	b, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			s.log.Error("get booking: failed", "booking_id", id, "error", err)
		}
		return nil, err
	}
	return b, nil
}

func (s *bookingService) UpdateBookingDetails(ctx context.Context, bookingID, barberID, serviceID string, timeStart time.Time) (*model.Booking, error) {
	existing, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			s.log.Error("update booking: failed to get booking", "booking_id", bookingID, "error", err)
		}
		return nil, err
	}
	if existing.BarberID != barberID {
		s.log.Warn("update booking: ownership mismatch", "booking_id", bookingID, "barber_id", barberID)
		return nil, repo.ErrNotFound
	}

	svcResp, err := s.staffClient.ListServices(ctx, barberID, false)
	if err != nil {
		s.log.Error("update booking: failed to get services", "barber_id", barberID, "error", err)
		return nil, fmt.Errorf("get services: %w", err)
	}
	serviceName := existing.ServiceName
	for _, svc := range svcResp.Services {
		if svc.ServiceId == serviceID {
			serviceName = svc.Name
			break
		}
	}

	timeEnd := timeStart.Add(slotDuration)

	bookings, err := s.repo.GetBookingsByBarberAndDate(ctx, barberID, timeStart.UTC().Truncate(24*time.Hour))
	if err != nil {
		s.log.Error("update booking: failed to get bookings for conflict check", "barber_id", barberID, "error", err)
		return nil, fmt.Errorf("get bookings: %w", err)
	}
	for _, b := range bookings {
		if b.ID == bookingID {
			continue
		}
		if timeStart.Before(b.TimeEnd) && timeEnd.After(b.TimeStart) {
			s.log.Warn("update booking: slot conflict", "booking_id", bookingID, "time_start", timeStart)
			return nil, fmt.Errorf("time slot already booked")
		}
	}

	if err := s.repo.UpdateBookingDetails(ctx, bookingID, serviceID, serviceName, timeStart, timeEnd); err != nil {
		s.log.Error("update booking: failed to save", "booking_id", bookingID, "error", err)
		return nil, err
	}

	s.log.Info("booking updated", "booking_id", bookingID, "barber_id", barberID, "service_id", serviceID)
	return s.repo.GetBooking(ctx, bookingID)
}

func (s *bookingService) UpdateBookingStatus(ctx context.Context, bookingID, barberID, newStatus string) (*model.Booking, error) {
	existing, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			s.log.Error("update booking status: failed to get booking", "booking_id", bookingID, "error", err)
		}
		return nil, err
	}
	if existing.BarberID != barberID {
		s.log.Warn("update booking status: ownership mismatch", "booking_id", bookingID, "barber_id", barberID)
		return nil, repo.ErrNotFound
	}
	if model.FinalStatuses[existing.Status] {
		s.log.Warn("update booking status: invalid transition", "booking_id", bookingID, "current_status", existing.Status, "new_status", newStatus)
		return nil, ErrInvalidStatusTransition
	}

	if err := s.repo.UpdateBookingStatus(ctx, bookingID, newStatus); err != nil {
		s.log.Error("update booking status: failed to save", "booking_id", bookingID, "error", err)
		return nil, err
	}

	s.log.Info("booking status updated", "booking_id", bookingID, "barber_id", barberID, "status", newStatus)
	return s.repo.GetBooking(ctx, bookingID)
}

func (s *bookingService) DeleteBooking(ctx context.Context, id string) error {
	if err := s.repo.DeleteBooking(ctx, id); err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			s.log.Error("delete booking: failed", "booking_id", id, "error", err)
		}
		return err
	}
	s.log.Info("booking deleted", "booking_id", id)
	return nil
}

func (s *bookingService) GetSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	return s.buildSlots(ctx, barberID, date, false)
}

func (s *bookingService) GetFreeSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	return s.buildSlots(ctx, barberID, date, true)
}

func (s *bookingService) buildSlots(ctx context.Context, barberID string, date time.Time, onlyFree bool) (*model.SlotsResult, error) {
	year, week := date.ISOWeek()
	isoWeek := fmt.Sprintf("%d-W%02d", year, week)

	schedResp, err := s.staffClient.GetSchedule(ctx, barberID, isoWeek)
	if err != nil {
		s.log.Error("build slots: failed to get schedule", "barber_id", barberID, "week", isoWeek, "error", err)
		return nil, fmt.Errorf("get schedule: %w", err)
	}

	dateStr := date.Format("2006-01-02")
	var workStart, workEnd time.Time
	hasWork := false
	for _, day := range schedResp.Days {
		if day.Date == dateStr {
			workStart, err = parseTimeOnDate(date, day.StartTime)
			if err != nil {
				return nil, fmt.Errorf("parse start_time: %w", err)
			}
			workEnd, err = parseTimeOnDate(date, day.EndTime)
			if err != nil {
				return nil, fmt.Errorf("parse end_time: %w", err)
			}
			hasWork = true
			break
		}
	}

	if !hasWork {
		s.log.Info("build slots: no working day found", "barber_id", barberID, "date", dateStr)
		return &model.SlotsResult{BarberID: barberID, Date: dateStr}, nil
	}

	bookings, err := s.repo.GetBookingsByBarberAndDate(ctx, barberID, date.UTC().Truncate(24*time.Hour))
	if err != nil {
		s.log.Error("build slots: failed to get bookings", "barber_id", barberID, "date", dateStr, "error", err)
		return nil, fmt.Errorf("get bookings: %w", err)
	}

	var slots []model.Slot
	t := workStart
	for t.Before(workEnd) {
		slotEnd := t.Add(slotDuration)

		status := model.SlotFree
		var slotBooking *model.SlotBooking
		for i := range bookings {
			b := &bookings[i]
			if t.Before(b.TimeEnd) && slotEnd.After(b.TimeStart) {
				status = model.SlotBooked
				slotBooking = &model.SlotBooking{
					BookingID:   b.ID,
					ClientName:  b.ClientName,
					ClientPhone: b.ClientPhone,
					ServiceName: b.ServiceName,
				}
				break
			}
		}

		if !onlyFree || status == model.SlotFree {
			slots = append(slots, model.Slot{
				Status:    status,
				TimeStart: t,
				TimeEnd:   slotEnd,
				Booking:   slotBooking,
			})
		}
		t = slotEnd
	}

	return &model.SlotsResult{
		BarberID: barberID,
		Date:     dateStr,
		Slots:    slots,
	}, nil
}

func parseTimeOnDate(date time.Time, timeStr string) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return time.Time{}, err
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return time.Time{}, err
	}
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, date.Location()), nil
}
