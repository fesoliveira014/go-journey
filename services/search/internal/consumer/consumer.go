package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// Indexer is the subset of the search service the consumer needs.
type Indexer interface {
	Upsert(ctx context.Context, doc model.BookDocument) error
	Delete(ctx context.Context, id string) error
}

type bookEvent struct {
	EventType       string `json:"event_type"`
	BookID          string `json:"book_id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	ISBN            string `json:"isbn"`
	Genre           string `json:"genre"`
	Description     string `json:"description"`
	PublishedYear   int    `json:"published_year"`
	TotalCopies     int    `json:"total_copies"`
	AvailableCopies int    `json:"available_copies"`
}

// Run starts a Kafka consumer group that processes catalog book change events.
// It blocks until ctx is cancelled.
func Run(ctx context.Context, brokers []string, topic string, idx Indexer) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, "search-indexer", config)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer group.Close()

	handler := &consumerHandler{idx: idx}

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
	idx Indexer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	for msg := range claim.Messages() {
		if err := handleEvent(ctx, h.idx, msg.Value); err != nil {
			log.Printf("failed to handle event: %v", err)
			continue
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

func handleEvent(ctx context.Context, idx Indexer, data []byte) error {
	var event bookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	switch event.EventType {
	case "book.created", "book.updated":
		doc := model.BookDocument{
			ID:              event.BookID,
			Title:           event.Title,
			Author:          event.Author,
			ISBN:            event.ISBN,
			Genre:           event.Genre,
			Description:     event.Description,
			PublishedYear:   event.PublishedYear,
			TotalCopies:     event.TotalCopies,
			AvailableCopies: event.AvailableCopies,
		}
		return idx.Upsert(ctx, doc)
	case "book.deleted":
		return idx.Delete(ctx, event.BookID)
	default:
		log.Printf("unknown event type: %s", event.EventType)
		return nil
	}
}
