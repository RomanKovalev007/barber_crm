package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

const TopicBookingEvents = "bookings.events"

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
			Balancer:               &kafka.LeastBytes{},
			AllowAutoTopicCreation: true,
		},
	}
}

func (p *Producer) Publish(ctx context.Context, key string, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal proto: %w", err)
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: TopicBookingEvents,
		Key:   []byte(key),
		Value: data,
	})
}

func (p *Producer) Ping(ctx context.Context) error {
	addr := p.writer.Addr.String()
	conn, err := kafka.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("kafka ping failed: %w", err)
	}
	conn.Close()
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
