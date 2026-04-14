package consumer

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// Note: this file uses internal tests (package consumer, not consumer_test)
// because handleEvent is unexported. This is intentional — it tests the
// event handling logic directly without exposing it in the public API.

type mockCatalogService struct {
	calls []struct {
		ID    uuid.UUID
		Delta int
	}
}

func (m *mockCatalogService) UpdateAvailability(_ context.Context, id uuid.UUID, delta int) (*model.Book, error) {
	m.calls = append(m.calls, struct {
		ID    uuid.UUID
		Delta int
	}{id, delta})
	return nil, nil
}

func TestHandleEvent_Created(t *testing.T) {
	t.Parallel()
	svc := &mockCatalogService{}
	bookID := uuid.New()

	err := handleEvent(context.Background(), svc, []byte(`{
		"event_type": "reservation.created",
		"book_id": "`+bookID.String()+`"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(svc.calls))
	}
	if svc.calls[0].ID != bookID {
		t.Errorf("expected book ID %s, got %s", bookID, svc.calls[0].ID)
	}
	if svc.calls[0].Delta != -1 {
		t.Errorf("expected delta -1, got %d", svc.calls[0].Delta)
	}
}

func TestHandleEvent_Returned(t *testing.T) {
	t.Parallel()
	svc := &mockCatalogService{}
	bookID := uuid.New()

	err := handleEvent(context.Background(), svc, []byte(`{
		"event_type": "reservation.returned",
		"book_id": "`+bookID.String()+`"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.calls[0].Delta != 1 {
		t.Errorf("expected delta +1, got %d", svc.calls[0].Delta)
	}
}

func TestHandleEvent_Expired(t *testing.T) {
	t.Parallel()
	svc := &mockCatalogService{}
	bookID := uuid.New()

	err := handleEvent(context.Background(), svc, []byte(`{
		"event_type": "reservation.expired",
		"book_id": "`+bookID.String()+`"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.calls[0].Delta != 1 {
		t.Errorf("expected delta +1, got %d", svc.calls[0].Delta)
	}
}

func TestHandleEvent_UnknownType(t *testing.T) {
	t.Parallel()
	svc := &mockCatalogService{}

	err := handleEvent(context.Background(), svc, []byte(`{
		"event_type": "reservation.unknown",
		"book_id": "`+uuid.New().String()+`"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.calls) != 0 {
		t.Errorf("expected no calls for unknown event type, got %d", len(svc.calls))
	}
}
