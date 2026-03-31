package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceLogHandler_WithActiveSpan(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTraceLogHandler(&buf)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	logger := slog.New(handler)
	logger.InfoContext(ctx, "hello with trace")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}

	sc := span.SpanContext()
	if got := record["trace_id"]; got != sc.TraceID().String() {
		t.Errorf("trace_id = %v, want %v", got, sc.TraceID().String())
	}
	if got := record["span_id"]; got != sc.SpanID().String() {
		t.Errorf("span_id = %v, want %v", got, sc.SpanID().String())
	}
}

func TestTraceLogHandler_WithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTraceLogHandler(&buf)

	logger := slog.New(handler)
	logger.Info("hello no trace")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}

	if _, ok := record["trace_id"]; ok {
		t.Error("trace_id should not be present without active span")
	}
	if _, ok := record["span_id"]; ok {
		t.Error("span_id should not be present without active span")
	}
}

func TestTraceLogHandler_InvalidSpanContext(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTraceLogHandler(&buf)

	ctx := trace.ContextWithSpanContext(context.Background(), trace.SpanContext{})

	logger := slog.New(handler)
	logger.InfoContext(ctx, "hello invalid span")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}

	if _, ok := record["trace_id"]; ok {
		t.Error("trace_id should not be present with invalid span context")
	}
}
