package otel

import (
	"context"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_SetsGlobalProviders(t *testing.T) {
	shutdown, err := Init(context.Background(), "test-service", "localhost:4317")
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
	shutdown, err := Init(context.Background(), "test-service", "localhost:4317")
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}
