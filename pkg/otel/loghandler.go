package otel

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceLogHandler wraps slog.JSONHandler, injecting trace_id and span_id
// from the context when an active span is present.
type TraceLogHandler struct {
	inner slog.Handler
}

// NewTraceLogHandler creates a TraceLogHandler that writes JSON to w.
func NewTraceLogHandler(w io.Writer) *TraceLogHandler {
	return &TraceLogHandler{
		inner: slog.NewJSONHandler(w, nil),
	}
}

func (h *TraceLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *TraceLogHandler) Handle(ctx context.Context, record slog.Record) error {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, record)
}

func (h *TraceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceLogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *TraceLogHandler) WithGroup(name string) slog.Handler {
	return &TraceLogHandler{inner: h.inner.WithGroup(name)}
}
