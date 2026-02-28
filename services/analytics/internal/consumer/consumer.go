package consumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	bookingpb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	staffpb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/model"
)

const (
	maxRetries      = 5
	retryBaseWait   = 500 * time.Millisecond
	kafkaRetryWait  = 5 * time.Second
)

type analyticsRepo interface {
	InsertBooking(ctx context.Context, b *model.Booking) error
	InsertSchedule(ctx context.Context, s *model.Schedule) error
}

type Consumer struct {
	bookingReader  *kafka.Reader
	scheduleReader *kafka.Reader
	repo           analyticsRepo
	log            *slog.Logger
	wg             sync.WaitGroup
}

func New(brokers []string, repo analyticsRepo, log *slog.Logger) *Consumer {
	return &Consumer{
		bookingReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   "bookings.events",
			GroupID: "analytics",
		}),
		scheduleReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   "schedule.events",
			GroupID: "analytics",
		}),
		repo: repo,
		log:  log,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	c.wg.Add(2)
	go c.consumeBookings(ctx)
	go c.consumeSchedule(ctx)
	<-ctx.Done()
}

func (c *Consumer) Close() {
	c.wg.Wait()
	c.bookingReader.Close()
	c.scheduleReader.Close()
}

func (c *Consumer) consumeBookings(ctx context.Context) {
	defer c.wg.Done()
	for {
		msg, err := c.bookingReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("read booking event", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(kafkaRetryWait):
			}
			continue
		}

		var event bookingpb.BookingEvent
		if err := proto.Unmarshal(msg.Value, &event); err != nil {
			c.log.Error("unmarshal booking event", "error", err)
			c.commitOrLog(ctx, c.bookingReader, msg, "booking")
			continue
		}

		b := &model.Booking{
			BookingID:   event.BookingId,
			BarberID:    event.BarberId,
			ClientPhone: event.ClientPhone,
			ClientName:  event.ClientName,
			ServiceID:   event.ServiceId,
			ServiceName: event.ServiceName,
			Price:       event.Price,
			StartTime:   event.StartTime.AsTime(),
			EndTime:     event.EndTime.AsTime(),
			Status:      bookingStatusToString(event.Status),
			OccurredAt:  event.OccurredAt.AsTime(),
		}

		if !c.retryInsert(ctx, func() error { return c.repo.InsertBooking(ctx, b) }, "booking", b.BookingID) {
			continue
		}

		c.commitOrLog(ctx, c.bookingReader, msg, "booking")
	}
}

func (c *Consumer) consumeSchedule(ctx context.Context) {
	defer c.wg.Done()
	for {
		msg, err := c.scheduleReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("read schedule event", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(kafkaRetryWait):
			}
			continue
		}

		var event staffpb.ScheduleEvent
		if err := proto.Unmarshal(msg.Value, &event); err != nil {
			c.log.Error("unmarshal schedule event", "error", err)
			c.commitOrLog(ctx, c.scheduleReader, msg, "schedule")
			continue
		}

		s := &model.Schedule{
			ScheduleID: event.ScheduleId,
			BarberID:   event.BarberId,
			Date:       event.Date,
			StartTime:  event.StartTime,
			EndTime:    event.EndTime,
			IsDeleted:  event.EventType == staffpb.ScheduleEventType_SCHEDULE_EVENT_DELETED,
			OccurredAt: event.OccurredAt.AsTime(),
		}

		if !c.retryInsert(ctx, func() error { return c.repo.InsertSchedule(ctx, s) }, "schedule", s.ScheduleID) {
			continue
		}

		c.commitOrLog(ctx, c.scheduleReader, msg, "schedule")
	}
}

// retryInsert выполняет fn до maxRetries раз с экспоненциальным backoff.
// Возвращает true если успешно, false если исчерпаны попытки (poison message — коммитим и пропускаем).
func (c *Consumer) retryInsert(ctx context.Context, fn func() error, topic, id string) bool {
	wait := retryBaseWait
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := fn(); err != nil {
			c.log.Error("insert failed, retrying",
				"topic", topic, "id", id,
				"attempt", attempt, "max", maxRetries, "error", err,
			)
			select {
			case <-ctx.Done():
				return false
			case <-time.After(wait):
			}
			wait *= 2
			continue
		}
		return true
	}
	c.log.Error("poison message: exceeded max retries, skipping", "topic", topic, "id", id)
	return false
}

func (c *Consumer) commitOrLog(ctx context.Context, r *kafka.Reader, msg kafka.Message, topic string) {
	if err := r.CommitMessages(ctx, msg); err != nil {
		c.log.Error("commit message", "topic", topic, "error", err)
	}
}

func bookingStatusToString(s bookingpb.BookingStatus) string {
	switch s {
	case bookingpb.BookingStatus_BOOKING_STATUS_PENDING:
		return "pending"
	case bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED:
		return "completed"
	case bookingpb.BookingStatus_BOOKING_STATUS_CANCELLED:
		return "cancelled"
	case bookingpb.BookingStatus_BOOKING_STATUS_NO_SHOW:
		return "no_show"
	default:
		return "pending"
	}
}
