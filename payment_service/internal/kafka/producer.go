package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Send(ctx context.Context, topic string, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal payment reminder: %w", err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Value: data}); err != nil {
		return fmt.Errorf("write payment reminder: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
