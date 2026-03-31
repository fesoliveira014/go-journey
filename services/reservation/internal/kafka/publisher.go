package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	otelgo "go.opentelemetry.io/otel"

	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

// Publisher wraps a sarama SyncProducer and implements service.EventPublisher.
type Publisher struct {
	producer sarama.SyncProducer
	topic    string
}

// NewPublisher creates a Kafka publisher for the given topic.
func NewPublisher(brokers []string, topic string) (*Publisher, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &Publisher{producer: producer, topic: topic}, nil
}

type headerCarrier struct {
	msg *sarama.ProducerMessage
}

func (c *headerCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *headerCarrier) Set(key, value string) {
	c.msg.Headers = append(c.msg.Headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c *headerCarrier) Keys() []string {
	keys := make([]string, len(c.msg.Headers))
	for i, h := range c.msg.Headers {
		keys[i] = string(h.Key)
	}
	return keys
}

// Publish sends a reservation event to Kafka, keyed by book_id.
func (p *Publisher) Publish(ctx context.Context, event service.ReservationEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.BookID),
		Value: sarama.ByteEncoder(value),
	}

	otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	return nil
}

// Close shuts down the producer.
func (p *Publisher) Close() error {
	return p.producer.Close()
}
