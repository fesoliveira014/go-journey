package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// AvailabilityUpdater is the subset of the catalog service the consumer needs.
type AvailabilityUpdater interface {
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) (*model.Book, error)
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
			slog.Error("consumer error", "error", err)
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
			slog.ErrorContext(ctx, "failed to handle event", "error", err)
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
		slog.WarnContext(ctx, "unknown event type", "event_type", event.EventType)
		return nil
	}

	_, err = svc.UpdateAvailability(ctx, bookID, delta)
	return err
}
