package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"

	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
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

// Publish sends a book event to Kafka, keyed by book_id.
func (p *Publisher) Publish(_ context.Context, event service.BookEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.BookID),
		Value: sarama.ByteEncoder(value),
	}

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
