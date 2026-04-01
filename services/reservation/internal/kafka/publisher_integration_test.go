//go:build integration

package kafka_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"
	otelgo "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	reservationkafka "github.com/fesoliveira014/library-system/services/reservation/internal/kafka"
	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

// setupKafka starts a Kafka testcontainer and returns the broker addresses.
func setupKafka(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()

	container, err := kafkatc.Run(ctx, "confluentinc/confluent-local:7.6.0")
	if err != nil {
		t.Fatalf("start kafka container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate kafka container: %v", err)
		}
	})

	brokers, err := container.Brokers(ctx)
	if err != nil {
		t.Fatalf("get kafka brokers: %v", err)
	}
	return brokers
}

func TestPublisher_Integration_SendMessage(t *testing.T) {
	brokers := setupKafka(t)
	const topic = "reservations"

	pub, err := reservationkafka.NewPublisher(brokers, topic)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	event := service.ReservationEvent{
		Type:          "reservation.created",
		ReservationID: uuid.New().String(),
		UserID:        uuid.New().String(),
		BookID:        uuid.New().String(),
		Timestamp:     time.Now().UTC().Truncate(time.Millisecond),
	}

	if err := pub.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	// Consume the message back.
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Close() })

	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		t.Fatalf("consume partition: %v", err)
	}
	t.Cleanup(func() { _ = partitionConsumer.Close() })

	select {
	case msg := <-partitionConsumer.Messages():
		// Verify key equals BookID.
		if string(msg.Key) != event.BookID {
			t.Errorf("message key: got %q, want %q", string(msg.Key), event.BookID)
		}

		// Verify value deserializes to matching ReservationEvent.
		var got service.ReservationEvent
		if err := json.Unmarshal(msg.Value, &got); err != nil {
			t.Fatalf("unmarshal message value: %v", err)
		}
		if got.Type != event.Type {
			t.Errorf("event type: got %q, want %q", got.Type, event.Type)
		}
		if got.ReservationID != event.ReservationID {
			t.Errorf("reservation_id: got %q, want %q", got.ReservationID, event.ReservationID)
		}
		if got.UserID != event.UserID {
			t.Errorf("user_id: got %q, want %q", got.UserID, event.UserID)
		}
		if got.BookID != event.BookID {
			t.Errorf("book_id: got %q, want %q", got.BookID, event.BookID)
		}

	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestPublisher_Integration_OTelHeaders(t *testing.T) {
	brokers := setupKafka(t)
	const topic = "reservations"

	// Set up a real SDK tracer provider so OTel propagation actually injects headers.
	tp := sdktrace.NewTracerProvider()
	otelgo.SetTracerProvider(tp)
	otelgo.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		// Restore no-op defaults.
		otelgo.SetTracerProvider(otelgo.GetTracerProvider())
	})

	pub, err := reservationkafka.NewPublisher(brokers, topic)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	// Start a real span so the context carries a valid trace context.
	ctx, span := otelgo.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	event := service.ReservationEvent{
		Type:          "reservation.created",
		ReservationID: uuid.New().String(),
		UserID:        uuid.New().String(),
		BookID:        uuid.New().String(),
		Timestamp:     time.Now().UTC(),
	}

	if err := pub.Publish(ctx, event); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	// Consume and inspect headers.
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Close() })

	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		t.Fatalf("consume partition: %v", err)
	}
	t.Cleanup(func() { _ = partitionConsumer.Close() })

	select {
	case msg := <-partitionConsumer.Messages():
		var foundTraceparent bool
		for _, h := range msg.Headers {
			if string(h.Key) == "traceparent" {
				foundTraceparent = true
				break
			}
		}
		if !foundTraceparent {
			t.Error("expected traceparent header in message, but it was not found")
		}

	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}
