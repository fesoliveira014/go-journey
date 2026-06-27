package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	otelgo "go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"

	kafkautil "github.com/fesoliveira014/library-system/pkg/kafka"
)

type reservationEvent struct {
	EventType string `json:"event_type"`
	BookID    string `json:"book_id"`
}

// Run starts a Kafka consumer group that observes reservation events.
// It blocks until ctx is cancelled. groupID is optional; tests can pass a
// unique value to avoid offset collisions.
func Run(ctx context.Context, brokers []string, topic string, tlsEnabled bool, groupID ...string) error {
	config := kafkautil.NewSaramaConfig(tlsEnabled)
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	id := "catalog-reservation-audit"
	if len(groupID) > 0 && groupID[0] != "" {
		id = groupID[0]
	}
	group, err := sarama.NewConsumerGroup(brokers, id, config)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer group.Close()

	handler := &consumerHandler{}

	for {
		if err := group.Consume(ctx, []string{topic}, handler); err != nil {
			slog.Error("consumer error", "error", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

// consumerHeaderCarrier adapts sarama consumer message headers to propagation.TextMapCarrier.
type consumerHeaderCarrier []*sarama.RecordHeader

func (c consumerHeaderCarrier) Get(key string) string {
	for _, h := range c {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c consumerHeaderCarrier) Set(key, value string) {
	// Consumer carrier is read-only; Set is a no-op.
}

func (c consumerHeaderCarrier) Keys() []string {
	keys := make([]string, len(c))
	for i, h := range c {
		keys[i] = string(h.Key)
	}
	return keys
}

type consumerHandler struct {
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	for msg := range claim.Messages() {
		msgCtx := otelgo.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
		msgCtx, span := otelgo.Tracer("catalog").Start(msgCtx, "catalog.consume.reservation")
		if err := handleEvent(msgCtx, msg.Value); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			slog.ErrorContext(msgCtx, "failed to handle event", "error", err)
			span.End()
			continue
		}
		span.End()
		session.MarkMessage(msg, "")
	}
	return nil
}

// handleEvent validates a single reservation event message. Catalog owns
// book availability, so reservation events are not commands to mutate
// available_copies. Reservation calls UpdateAvailability synchronously;
// these events remain available for audit/notification consumers.
func handleEvent(ctx context.Context, data []byte) error {
	var event reservationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	bookID, err := uuid.Parse(event.BookID)
	if err != nil {
		return fmt.Errorf("parse book ID: %w", err)
	}

	switch event.EventType {
	case "reservation.returned", "reservation.expired":
		slog.InfoContext(ctx, "observed reservation event", "event_type", event.EventType, "book_id", bookID)
	case "reservation.created":
		slog.InfoContext(ctx, "observed reservation event", "event_type", event.EventType, "book_id", bookID)
	default:
		slog.WarnContext(ctx, "unknown event type", "event_type", event.EventType)
		return nil
	}
	return nil
}
