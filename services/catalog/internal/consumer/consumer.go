package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

// AvailabilityUpdater is the subset of the catalog service the consumer needs.
type AvailabilityUpdater interface {
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}

type reservationEvent struct {
	EventType string `json:"event_type"`
	BookID    string `json:"book_id"`
}

// Run starts a Kafka consumer group that processes reservation events.
// It blocks until ctx is cancelled.
func Run(ctx context.Context, brokers []string, topic string, svc AvailabilityUpdater) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, "catalog-availability-updater", config)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer group.Close()

	handler := &consumerHandler{svc: svc}

	for {
		if err := group.Consume(ctx, []string{topic}, handler); err != nil {
			log.Printf("consumer error: %v", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

type consumerHandler struct {
	svc AvailabilityUpdater
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	for msg := range claim.Messages() {
		if err := handleEvent(ctx, h.svc, msg.Value); err != nil {
			log.Printf("failed to handle event: %v", err)
			continue
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

// handleEvent processes a single reservation event message.
func handleEvent(ctx context.Context, svc AvailabilityUpdater, data []byte) error {
	var event reservationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	bookID, err := uuid.Parse(event.BookID)
	if err != nil {
		return fmt.Errorf("parse book ID: %w", err)
	}

	var delta int
	switch event.EventType {
	case "reservation.created":
		delta = -1
	case "reservation.returned", "reservation.expired":
		delta = 1
	default:
		log.Printf("unknown event type: %s", event.EventType)
		return nil
	}

	return svc.UpdateAvailability(ctx, bookID, delta)
}
