package otel

import (
	"context"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_SetsGlobalProviders(t *testing.T) {
	t.Parallel()
	shutdown, err := Init(context.Background(), "test-service", "0.0.1", "localhost:4317")
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	defer shutdown(context.Background())

	tp := otelapi.GetTracerProvider()
	if _, ok := tp.(*trace.TracerProvider); !ok {
		t.Errorf("expected *trace.TracerProvider, got %T", tp)
	}
}

func TestInit_ShutdownIsIdempotent(t *testing.T) {
	t.Parallel()
	shutdown, err := Init(context.Background(), "test-service", "0.0.1", "localhost:4317")
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// First call may return an error (no collector running); that's fine.
	// The point is that calling shutdown twice does not panic.
	err1 := shutdown(context.Background())
	err2 := shutdown(context.Background())

	// Second call should return the same error (sync.Once caches the result).
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("idempotency broken: first=%v, second=%v", err1, err2)
	}
}
