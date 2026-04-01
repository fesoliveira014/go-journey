//go:build integration

package consumer_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"

	"github.com/fesoliveira014/library-system/services/search/internal/consumer"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// setupKafka starts a Kafka testcontainer and returns the broker addresses.
func setupKafka(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()

	kafkaContainer, err := kafkatc.Run(ctx,
		"confluentinc/cp-kafka:7.6.1",
	)
	if err != nil {
		t.Fatalf("failed to start kafka container: %v", err)
	}
	t.Cleanup(func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate kafka container: %v", err)
		}
	})

	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		t.Fatalf("failed to get kafka brokers: %v", err)
	}
	return brokers
}

type capturingIndexer struct {
	mu       sync.Mutex
	upserted []model.BookDocument
	deleted  []string
}

func (c *capturingIndexer) Upsert(_ context.Context, doc model.BookDocument) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.upserted = append(c.upserted, doc)
	return nil
}

func (c *capturingIndexer) Delete(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleted = append(c.deleted, id)
	return nil
}

func TestConsumer_Integration_BookCreated(t *testing.T) {
	brokers := setupKafka(t)
	topic := "books"
	bookID := uuid.New().String()

	event := map[string]interface{}{
		"event_type":       "book.created",
		"book_id":          bookID,
		"title":            "The Go Programming Language",
		"author":           "Alan Donovan",
		"isbn":             "978-0134190440",
		"genre":            "programming",
		"description":      "A comprehensive guide to Go.",
		"published_year":   2015,
		"total_copies":     5,
		"available_copies": 5,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	defer producer.Close()

	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	indexer := &capturingIndexer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Run(ctx, brokers, topic, indexer); err != nil {
			t.Logf("consumer.Run returned: %v", err)
		}
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			indexer.mu.Lock()
			n := len(indexer.upserted)
			indexer.mu.Unlock()
			if n >= 1 {
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for book.created event to be indexed")
		}
	}

done:
	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	doc := indexer.upserted[0]
	if doc.ID != bookID {
		t.Errorf("expected ID %q, got %q", bookID, doc.ID)
	}
	if doc.Title != "The Go Programming Language" {
		t.Errorf("expected title %q, got %q", "The Go Programming Language", doc.Title)
	}
	if doc.Author != "Alan Donovan" {
		t.Errorf("expected author %q, got %q", "Alan Donovan", doc.Author)
	}
	if doc.ISBN != "978-0134190440" {
		t.Errorf("expected ISBN %q, got %q", "978-0134190440", doc.ISBN)
	}
	if doc.Genre != "programming" {
		t.Errorf("expected genre %q, got %q", "programming", doc.Genre)
	}
	if doc.PublishedYear != 2015 {
		t.Errorf("expected published_year 2015, got %d", doc.PublishedYear)
	}
	if doc.TotalCopies != 5 {
		t.Errorf("expected total_copies 5, got %d", doc.TotalCopies)
	}
	if doc.AvailableCopies != 5 {
		t.Errorf("expected available_copies 5, got %d", doc.AvailableCopies)
	}
}

func TestConsumer_Integration_BookDeleted(t *testing.T) {
	brokers := setupKafka(t)
	topic := "books"
	bookID := uuid.New().String()

	event := map[string]interface{}{
		"event_type": "book.deleted",
		"book_id":    bookID,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	defer producer.Close()

	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	indexer := &capturingIndexer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Run(ctx, brokers, topic, indexer); err != nil {
			t.Logf("consumer.Run returned: %v", err)
		}
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			indexer.mu.Lock()
			n := len(indexer.deleted)
			indexer.mu.Unlock()
			if n >= 1 {
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for book.deleted event to be indexed")
		}
	}

done:
	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if indexer.deleted[0] != bookID {
		t.Errorf("expected deleted ID %q, got %q", bookID, indexer.deleted[0])
	}
}
