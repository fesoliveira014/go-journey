package consumer

import (
	"context"
	"testing"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

type mockIndexer struct {
	upserted []model.BookDocument
	deleted  []string
}

func (m *mockIndexer) Upsert(_ context.Context, doc model.BookDocument) error {
	m.upserted = append(m.upserted, doc)
	return nil
}

func (m *mockIndexer) Delete(_ context.Context, id string) error {
	m.deleted = append(m.deleted, id)
	return nil
}

func TestHandleEvent_BookCreated(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.created",
		"book_id": "abc-123",
		"title": "Go Book",
		"author": "Author",
		"isbn": "1234567890",
		"genre": "programming",
		"total_copies": 5,
		"available_copies": 5
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(idx.upserted))
	}
	if idx.upserted[0].Title != "Go Book" {
		t.Errorf("expected title 'Go Book', got %s", idx.upserted[0].Title)
	}
	if idx.upserted[0].ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %s", idx.upserted[0].ID)
	}
}

func TestHandleEvent_BookUpdated(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.updated",
		"book_id": "abc-123",
		"title": "Updated",
		"author": "Author",
		"available_copies": 3
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(idx.upserted))
	}
	if idx.upserted[0].AvailableCopies != 3 {
		t.Errorf("expected 3 available copies, got %d", idx.upserted[0].AvailableCopies)
	}
}

func TestHandleEvent_BookDeleted(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.deleted",
		"book_id": "abc-123"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.deleted) != 1 {
		t.Fatalf("expected 1 delete, got %d", len(idx.deleted))
	}
	if idx.deleted[0] != "abc-123" {
		t.Errorf("expected deleted ID 'abc-123', got %s", idx.deleted[0])
	}
}

func TestHandleEvent_UnknownType(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.unknown",
		"book_id": "abc-123"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 0 || len(idx.deleted) != 0 {
		t.Error("expected no operations for unknown event type")
	}
}
