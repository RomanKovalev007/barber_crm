package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	bookingpb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/staffclient"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bookingRepo interface {
	CreateBookingTx(ctx context.Context, b *model.Booking) error
	GetBooking(ctx context.Context, id string) (*model.Booking, error)
	UpdateBookingDetails(ctx context.Context, id, serviceID, serviceName string, price int32, timeStart, timeEnd time.Time) error
	UpdateBookingStatus(ctx context.Context, id, status string) error
	DeleteBooking(ctx context.Context, id string) error
	GetBookingsByBarberAndDate(ctx context.Context, barberID string, date time.Time) ([]model.Booking, error)
}

type staffClientIntr interface {
	GetBarber(ctx context.Context, barberID string) (*staffv1.BarberResponse, error)
	ListServices(ctx context.Context, barberID string, includeInactive bool) (*staffv1.ListServicesResponse, error)
	GetSchedule(ctx context.Context, barberID, week string) (*staffv1.GetScheduleResponse, error)
}

type eventProducer interface {
	Publish(ctx context.Context, key string, msg proto.Message) error
}

const slotDuration = 60 * time.Minute

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
	repo        bookingRepo
	redis       *redis.Client
	staffClient staffClientIntr
	producer    eventProducer
}

func New(r *repo.BookingRepo, rc *redis.Client, ttl int, jwt string, log *slog.Logger, staffClient *staffclient.Client, producer eventProducer) BookingIntr {
	return &bookingService{
		log:         log,
		repo:        r,
		redis:       rc,
		staffClient: staffClient,
		producer:    producer,
	}
}

func (s *bookingService) CreateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error) {
	if _, err := s.staffClient.GetBarber(ctx, b.BarberID); err != nil {
		s.log.Warn("create booking: barber not found", "barber_id", b.BarberID, "error", err)
		return nil, apperr.NotFound("barber not found")
	}

	svcResp, err := s.staffClient.ListServices(ctx, b.BarberID, false)
	if err != nil {
		s.log.Error("create booking: failed to get services", "barber_id", b.BarberID, "error", err)
		return nil, apperr.Internal("failed to get services")
	}
	for _, svc := range svcResp.Services {
		if svc.ServiceId == b.ServiceID {
			b.ServiceName = svc.Name
			b.Price = svc.Price
			break
		}
	}

	b.ID = uuid.New().String()
	b.Status = model.StatusPending
	b.TimeEnd = b.TimeStart.Add(slotDuration)
	b.Date = b.TimeStart.UTC().Truncate(24 * time.Hour)

	if err := s.repo.CreateBookingTx(ctx, b); err != nil {
		switch {
		case errors.Is(err, repo.ErrActiveBookingExists):
			s.log.Warn("create booking: client already has active booking", "client_phone", b.ClientPhone)
			return nil, apperr.AlreadyExists("client already has an active booking")
		case errors.Is(err, repo.ErrSlotConflict):
			s.log.Warn("create booking: slot conflict", "barber_id", b.BarberID, "time_start", b.TimeStart)
			return nil, apperr.AlreadyExists("time slot already booked")
		default:
			s.log.Error("create booking: failed to save", "barber_id", b.BarberID, "error", err)
			return nil, apperr.Internal("failed to create booking")
		}
	}

	s.log.Info("booking created", "booking_id", b.ID, "barber_id", b.BarberID, "client_phone", b.ClientPhone)
	s.publishEvent(ctx, b, bookingpb.BookingStatus_BOOKING_STATUS_PENDING)
	return b, nil
}

func (s *bookingService) GetBooking(ctx context.Context, id string) (*model.Booking, error) {
	b, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, apperr.NotFound("booking not found")
		}
		s.log.Error("get booking: failed", "booking_id", id, "error", err)
		return nil, apperr.Internal("failed to get booking")
	}
	return b, nil
}

func (s *bookingService) UpdateBookingDetails(ctx context.Context, bookingID, barberID, serviceID string, timeStart time.Time) (*model.Booking, error) {
	existing, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, apperr.NotFound("booking not found")
		}
		s.log.Error("update booking: failed to get booking", "booking_id", bookingID, "error", err)
		return nil, apperr.Internal("failed to get booking")
	}
	if existing.BarberID != barberID {
		s.log.Warn("update booking: ownership mismatch", "booking_id", bookingID, "barber_id", barberID)
		return nil, apperr.NotFound("booking not found")
	}

	svcResp, err := s.staffClient.ListServices(ctx, barberID, false)
	if err != nil {
		s.log.Error("update booking: failed to get services", "barber_id", barberID, "error", err)
		return nil, apperr.Internal("failed to get services")
	}
	serviceName := existing.ServiceName
	price := existing.Price
	for _, svc := range svcResp.Services {
		if svc.ServiceId == serviceID {
			serviceName = svc.Name
			price = svc.Price
			break
		}
	}

	timeEnd := timeStart.Add(slotDuration)

	bookings, err := s.repo.GetBookingsByBarberAndDate(ctx, barberID, timeStart.UTC().Truncate(24*time.Hour))
	if err != nil {
		s.log.Error("update booking: failed to get bookings for conflict check", "barber_id", barberID, "error", err)
		return nil, apperr.Internal("failed to check slot availability")
	}
	for _, b := range bookings {
		if b.ID == bookingID {
			continue
		}
		if timeStart.Before(b.TimeEnd) && timeEnd.After(b.TimeStart) {
			s.log.Warn("update booking: slot conflict", "booking_id", bookingID, "time_start", timeStart)
			return nil, apperr.AlreadyExists("time slot already booked")
		}
	}

	if err := s.repo.UpdateBookingDetails(ctx, bookingID, serviceID, serviceName, price, timeStart, timeEnd); err != nil {
		s.log.Error("update booking: failed to save", "booking_id", bookingID, "error", err)
		return nil, apperr.Internal("failed to update booking")
	}

	s.log.Info("booking updated", "booking_id", bookingID, "barber_id", barberID, "service_id", serviceID)

	updated, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, apperr.Internal("failed to get updated booking")
	}
	s.publishEvent(ctx, updated, bookingStatusToProto(updated.Status))
	return updated, nil
}

func (s *bookingService) UpdateBookingStatus(ctx context.Context, bookingID, barberID, newStatus string) (*model.Booking, error) {
	existing, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, apperr.NotFound("booking not found")
		}
		s.log.Error("update booking status: failed to get booking", "booking_id", bookingID, "error", err)
		return nil, apperr.Internal("failed to get booking")
	}
	if existing.BarberID != barberID {
		s.log.Warn("update booking status: ownership mismatch", "booking_id", bookingID, "barber_id", barberID)
		return nil, apperr.NotFound("booking not found")
	}
	if model.FinalStatuses[existing.Status] {
		s.log.Warn("update booking status: invalid transition", "booking_id", bookingID, "current_status", existing.Status, "new_status", newStatus)
		return nil, apperr.FailedPrecondition("booking is already in a final status")
	}

	if err := s.repo.UpdateBookingStatus(ctx, bookingID, newStatus); err != nil {
		s.log.Error("update booking status: failed to save", "booking_id", bookingID, "error", err)
		return nil, apperr.Internal("failed to update booking status")
	}

	s.log.Info("booking status updated", "booking_id", bookingID, "barber_id", barberID, "status", newStatus)

	updated, err := s.repo.GetBooking(ctx, bookingID)
	if err != nil {
		return nil, apperr.Internal("failed to get updated booking")
	}
	s.publishEvent(ctx, updated, bookingStatusToProto(newStatus))
	return updated, nil
}

func (s *bookingService) DeleteBooking(ctx context.Context, id string) error {
	if err := s.repo.DeleteBooking(ctx, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return apperr.NotFound("booking not found")
		}
		s.log.Error("delete booking: failed", "booking_id", id, "error", err)
		return apperr.Internal("failed to delete booking")
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
		return nil, apperr.Internal("failed to get schedule")
	}

	dateStr := date.Format("2006-01-02")
	var workStart, workEnd time.Time
	hasWork := false
	for _, day := range schedResp.Days {
		if day.Date == dateStr {
			workStart, err = parseTimeOnDate(date, day.StartTime)
			if err != nil {
				return nil, apperr.Internal("failed to parse schedule start_time")
			}
			workEnd, err = parseTimeOnDate(date, day.EndTime)
			if err != nil {
				return nil, apperr.Internal("failed to parse schedule end_time")
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
		return nil, apperr.Internal("failed to get bookings")
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

// ─── helpers ─────────────────────────────────────────────────────────────────

func (s *bookingService) publishEvent(ctx context.Context, b *model.Booking, status bookingpb.BookingStatus) {
	event := &bookingpb.BookingEvent{
		BookingId:   b.ID,
		BarberId:    b.BarberID,
		ClientPhone: b.ClientPhone,
		ClientName:  b.ClientName,
		ServiceId:   b.ServiceID,
		ServiceName: b.ServiceName,
		Price:       b.Price,
		TimeStart:   timestamppb.New(b.TimeStart),
		TimeEnd:     timestamppb.New(b.TimeEnd),
		Status:      status,
		OccurredAt:  timestamppb.New(time.Now()),
	}
	if err := s.producer.Publish(ctx, b.ID, event); err != nil {
		s.log.Warn("publish booking event failed", "booking_id", b.ID, "status", status, "error", err)
	}
}

func bookingStatusToProto(s string) bookingpb.BookingStatus {
	switch s {
	case model.StatusPending:
		return bookingpb.BookingStatus_BOOKING_STATUS_PENDING
	case model.StatusCompleted:
		return bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED
	case model.StatusCancelled:
		return bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED
	case model.StatusNoShow:
		return bookingpb.BookingStatus_BOOKING_STATUS_NO_SHOW
	default:
		return bookingpb.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
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
