package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go"
)

const (
	TopicScheduleAdded  = "staff.schedule.added"
	TopicServiceCreated = "staff.service.created"
	TopicServiceUpdated = "staff.service.updated"
	TopicServiceDeleted = "staff.service.deleted"
)

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

func (p *Producer) Publish(ctx context.Context, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
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
