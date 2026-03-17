package consumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	bookingpb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
)

const (
	maxRetries     = 5
	retryBaseWait  = 500 * time.Millisecond
	kafkaRetryWait = 5 * time.Second
)

var msgsProcessed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "client_kafka_messages_processed_total",
		Help: "Total Kafka booking events processed by client service",
	},
	[]string{"status"},
)

func init() {
	prometheus.MustRegister(msgsProcessed)
}

type clientRepo interface {
	UpsertByBooking(ctx context.Context, barberID, phone, name, bookingID string, lastVisit time.Time) error
}

type Consumer struct {
	reader *kafka.Reader
	repo   clientRepo
	log    *slog.Logger
	wg     sync.WaitGroup
}

func New(brokers []string, repo clientRepo, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   "bookings.events",
			GroupID: "client-service",
		}),
		repo: repo,
		log:  log,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.consume(ctx)
	<-ctx.Done()
}

func (c *Consumer) Close() {
	c.wg.Wait()
	c.reader.Close()
}

func (c *Consumer) consume(ctx context.Context) {
	defer c.wg.Done()
	for {
		msg, err := c.reader.FetchMessage(ctx)
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
			c.commitOrLog(ctx, msg)
			continue
		}

		// Обрабатываем только завершённые визиты
		if event.Status != bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED {
			msgsProcessed.WithLabelValues("skipped").Inc()
			c.commitOrLog(ctx, msg)
			continue
		}

		ok := c.retryInsert(ctx, func() error {
			return c.repo.UpsertByBooking(ctx,
				event.BarberId,
				event.ClientPhone,
				event.ClientName,
				event.BookingId,
				event.OccurredAt.AsTime(),
			)
		}, event.BookingId)
		if !ok {
			msgsProcessed.WithLabelValues("error").Inc()
		} else {
			msgsProcessed.WithLabelValues("ok").Inc()
		}
		c.commitOrLog(ctx, msg)
	}
}

func (c *Consumer) retryInsert(ctx context.Context, fn func() error, id string) bool {
	wait := retryBaseWait
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := fn(); err != nil {
			c.log.Error("insert failed, retrying",
				"id", id, "attempt", attempt, "max", maxRetries, "error", err,
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
	c.log.Error("poison message: exceeded max retries, skipping", "id", id)
	return false
}

func (c *Consumer) commitOrLog(ctx context.Context, msg kafka.Message) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		c.log.Error("commit message", "topic", "bookings.events", "error", err)
	}
}
