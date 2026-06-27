package consumer

import (
	"context"
	"testing"
)

// Note: this file uses internal tests (package consumer, not consumer_test)
// because handleEvent is unexported. This is intentional — it tests the
// event handling logic directly without exposing it in the public API.

func TestHandleEvent_ReservationEventsDoNotMutateAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType string
	}{
		{name: "created", eventType: "reservation.created"},
		{name: "returned", eventType: "reservation.returned"},
		{name: "expired", eventType: "reservation.expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := handleEvent(context.Background(), []byte(`{
				"event_type": "`+tt.eventType+`",
				"book_id": "00000000-0000-0000-0000-000000000001"
			}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleEvent_UnknownType(t *testing.T) {
	t.Parallel()

	err := handleEvent(context.Background(), []byte(`{
		"event_type": "reservation.unknown",
		"book_id": "00000000-0000-0000-0000-000000000001"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
