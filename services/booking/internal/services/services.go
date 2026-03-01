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

const slotDuration = 15 * time.Minute

var ErrActiveBookingExists = errors.New("client has an active booking")

type BookingIntr interface {
	CreateBooking(ctx context.Context, req *model.Booking) (*model.Booking, error)
	GetBooking(ctx context.Context, id string) (*model.Booking, error)
	UpdateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error)
	DeleteBooking(ctx context.Context, id string) (*model.DeleteResult, error)
	GetWorkDay(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error)
	GetFree(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error)
}

type bookingService struct {
	log         *slog.Logger
	repo        *repo.BookingRepo
	redis       *redis.Client
	staffClient *staffclient.Client
}

func New(repo *repo.BookingRepo, rc *redis.Client, ttl int, jwt string, log *slog.Logger, staffClient *staffclient.Client) BookingIntr {
	return &bookingService{
		log:         log,
		repo:        repo,
		redis:       rc,
		staffClient: staffClient,
	}
}

func (s *bookingService) CreateBooking(ctx context.Context, req *model.Booking) (*model.Booking, error) {
	if _, err := s.staffClient.GetBarber(ctx, req.BarberID); err != nil {
		return nil, fmt.Errorf("barber not found: %w", err)
	}

	hasActive, err := s.repo.HasActiveBooking(ctx, req.ClientName)
	if err != nil {
		return nil, fmt.Errorf("check active booking: %w", err)
	}
	if hasActive {
		return nil, ErrActiveBookingExists
	}

	existing, err := s.repo.GetBookingsByBarberAndDate(ctx, req.BarberID, req.Date)
	if err != nil {
		return nil, fmt.Errorf("get existing bookings: %w", err)
	}
	for _, b := range existing {
		if req.TimeStart.Before(b.TimeEnd) && req.TimeEnd.After(b.TimeStart) {
			return nil, fmt.Errorf("time slot already booked")
		}
	}

	req.ID = uuid.New().String()
	req.Status = model.StatusPending
	if err := s.repo.CreateBooking(ctx, req); err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}
	return req, nil
}

func (s *bookingService) GetBooking(ctx context.Context, id string) (*model.Booking, error) {
	b, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *bookingService) UpdateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error) {
	if err := s.repo.UpdateBooking(ctx, b); err != nil {
		return nil, err
	}
	return s.repo.GetBooking(ctx, b.ID)
}

func (s *bookingService) DeleteBooking(ctx context.Context, id string) (*model.DeleteResult, error) {
	b, err := s.repo.DeleteBooking(ctx, id)
	if err != nil {
		return nil, err
	}
	return &model.DeleteResult{
		BookingID:  b.ID,
		ClientName: b.ClientName,
	}, nil
}

func (s *bookingService) GetWorkDay(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	slots, err := s.buildSlots(ctx, barberID, date)
	if err != nil {
		return nil, err
	}
	return slots, nil
}

func (s *bookingService) GetFree(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	result, err := s.buildSlots(ctx, barberID, date)
	if err != nil {
		return nil, err
	}
	var free []model.Slot
	for _, sl := range result.Slots {
		if sl.Status == model.SlotFree {
			free = append(free, sl)
		}
	}
	result.Slots = free
	return result, nil
}

func (s *bookingService) buildSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	year, week := date.ISOWeek()
	isoWeek := fmt.Sprintf("%d-W%02d", year, week)
	schedResp, err := s.staffClient.GetSchedule(ctx, barberID, isoWeek)
	if err != nil {
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

	bookings, err := s.repo.GetBookingsByBarberAndDate(ctx, barberID, date)
	if err != nil {
		return nil, fmt.Errorf("get bookings: %w", err)
	}

	totalMinutes := 0
	if hasWork {
		totalMinutes = int(workEnd.Sub(workStart).Minutes())
	}
	totalSlots := totalMinutes / int(slotDuration.Minutes())

	priority := int32(1)
	if totalSlots > 0 {
		halfPoint := workStart.Add(time.Duration(totalSlots/2) * slotDuration)
		freeInSecond := 0
		freeInFirst := 0
		t := workStart
		for i := 0; i < totalSlots; i++ {
			slotEnd := t.Add(slotDuration)
			booked := false
			for _, b := range bookings {
				if t.Before(b.TimeEnd) && slotEnd.After(b.TimeStart) {
					booked = true
					break
				}
			}
			if !booked {
				if t.Before(halfPoint) {
					freeInFirst++
				} else {
					freeInSecond++
				}
			}
			t = slotEnd
		}
		if freeInSecond > freeInFirst {
			priority = 2
		}
	}

	var slots []model.Slot
	if !hasWork {
		return &model.SlotsResult{
			BarberID: barberID,
			Date:     date,
			Slots:    slots,
			Priority: priority,
		}, nil
	}

	t := workStart
	for t.Before(workEnd) {
		slotEnd := t.Add(slotDuration)

		var status model.SlotStatus
		isBooked := false
		for _, b := range bookings {
			if t.Before(b.TimeEnd) && slotEnd.After(b.TimeStart) {
				isBooked = true
				break
			}
		}
		if isBooked {
			status = model.SlotBooked
		} else {
			status = model.SlotFree
		}

		slots = append(slots, model.Slot{
			Status:    status,
			TimeStart: t,
			TimeEnd:   slotEnd,
		})
		t = slotEnd
	}

	return &model.SlotsResult{
		BarberID: barberID,
		Date:     date,
		Slots:    slots,
		Priority: priority,
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