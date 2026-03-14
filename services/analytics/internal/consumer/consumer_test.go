package consumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	bookingpb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	staffpb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ─── bookingStatusToString ───────────────────────────────────────────────────

func TestBookingStatusToString(t *testing.T) {
	cases := []struct {
		in  bookingpb.BookingStatus
		out string
	}{
		{bookingpb.BookingStatus_BOOKING_STATUS_PENDING, "pending"},
		{bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED, "completed"},
		{bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED, "cancelled"},
		{bookingpb.BookingStatus_BOOKING_STATUS_NO_SHOW, "no_show"},
		{bookingpb.BookingStatus_BOOKING_STATUS_UNSPECIFIED, "pending"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.out, bookingStatusToString(tc.in), "status=%v", tc.in)
	}
}

// ─── bookingEventToModel ─────────────────────────────────────────────────────

func TestBookingEventToModel(t *testing.T) {
	ts := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	te := time.Date(2026, 3, 16, 11, 0, 0, 0, time.UTC)
	oc := time.Date(2026, 3, 16, 9, 55, 0, 0, time.UTC)

	event := &bookingpb.BookingEvent{
		BookingId:   "bk-1",
		BarberId:    "b-1",
		ClientPhone: "+79001234567",
		ClientName:  "Ivan",
		ServiceId:   "svc-1",
		ServiceName: "Haircut",
		Price:       500,
		TimeStart:   timestamppb.New(ts),
		TimeEnd:     timestamppb.New(te),
		Status:      bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED,
		OccurredAt:  timestamppb.New(oc),
	}

	b := bookingEventToModel(event)

	assert.Equal(t, "bk-1", b.BookingID)
	assert.Equal(t, "b-1", b.BarberID)
	assert.Equal(t, "+79001234567", b.ClientPhone)
	assert.Equal(t, "Ivan", b.ClientName)
	assert.Equal(t, "svc-1", b.ServiceID)
	assert.Equal(t, "Haircut", b.ServiceName)
	assert.Equal(t, int32(500), b.Price)
	assert.Equal(t, ts, b.StartTime)
	assert.Equal(t, te, b.EndTime)
	assert.Equal(t, "completed", b.Status)
	assert.Equal(t, oc, b.OccurredAt)
}

// ─── scheduleEventToModel ────────────────────────────────────────────────────

func TestScheduleEventToModel_Added(t *testing.T) {
	oc := time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)

	event := &staffpb.ScheduleEvent{
		ScheduleId: "sched-1",
		BarberId:   "b-1",
		Date:       "2026-03-16",
		StartTime:  "09:00",
		EndTime:    "18:00",
		EventType:  staffpb.ScheduleEventType_SCHEDULE_EVENT_ADDED,
		OccurredAt: timestamppb.New(oc),
	}

	s := scheduleEventToModel(event)

	assert.Equal(t, "sched-1", s.ScheduleID)
	assert.Equal(t, "b-1", s.BarberID)
	assert.Equal(t, "2026-03-16", s.Date)
	assert.Equal(t, "09:00", s.StartTime)
	assert.Equal(t, "18:00", s.EndTime)
	assert.False(t, s.IsDeleted)
	assert.Equal(t, oc, s.OccurredAt)
}

func TestScheduleEventToModel_Deleted(t *testing.T) {
	event := &staffpb.ScheduleEvent{
		ScheduleId: "sched-2",
		BarberId:   "b-1",
		Date:       "2026-03-17",
		EventType:  staffpb.ScheduleEventType_SCHEDULE_EVENT_DELETED,
		OccurredAt: timestamppb.New(time.Now()),
	}

	s := scheduleEventToModel(event)

	assert.True(t, s.IsDeleted)
	assert.Equal(t, "sched-2", s.ScheduleID)
}

// ─── retryInsert ─────────────────────────────────────────────────────────────

func newTestConsumer() *Consumer {
	return &Consumer{log: discardLogger()}
}

func TestRetryInsert_SuccessOnFirstAttempt(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		return nil
	}, "test", "id-1")

	assert.True(t, ok)
	assert.Equal(t, 1, calls)
}

func TestRetryInsert_SuccessOnSecondAttempt(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		if calls < 2 {
			return errors.New("transient error")
		}
		return nil
	}, "test", "id-1")

	assert.True(t, ok)
	assert.Equal(t, 2, calls)
}

func TestRetryInsert_ExhaustsAllRetries(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		return errors.New("permanent error")
	}, "test", "id-1")

	assert.False(t, ok)
	assert.Equal(t, maxRetries, calls)
}

func TestRetryInsert_CancelledContext(t *testing.T) {
	c := newTestConsumer()
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	ok := c.retryInsert(ctx, func() error {
		calls++
		cancel() // отменяем контекст после первой попытки
		return errors.New("error")
	}, "test", "id-1")

	assert.False(t, ok)
	assert.Equal(t, 1, calls)
}

func TestRetryInsert_ZeroWaitOnSuccess(t *testing.T) {
	c := newTestConsumer()
	start := time.Now()

	c.retryInsert(context.Background(), func() error { return nil }, "test", "id-1")

	assert.Less(t, time.Since(start), 100*time.Millisecond)
}
