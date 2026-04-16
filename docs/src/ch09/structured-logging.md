# 9.3 Structured Logging with slog

Traces and metrics tell you what happened and how fast. Logs tell you why. But only if you can find the right log lines, and only if they contain enough context to be useful. This section covers Go's `log/slog` package, our custom `TraceLogHandler`, and the migration from `log.Printf` to structured, context-aware logging.

---

## Why Structured Logging Matters

Consider two log lines from a book creation failure:

**Unstructured:**
```
2026-03-31 14:22:03 ERROR: failed to publish event for book abc123: connection refused
```

**Structured (JSON):**
```json
{
  "time": "2026-03-31T14:22:03.456Z",
  "level": "ERROR",
  "msg": "failed to publish event",
  "event": "book.created",
  "book_id": "abc123",
  "error": "connection refused",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

The unstructured line is readable by a human scanning a terminal, but it is not queryable. The structured line is queryable by a machine. With structured logs flowing into Loki, you can write queries like:

- `{container_name="catalog"} | json | level="ERROR"`—all errors from the catalog
- `{container_name=~".+"} | json | trace_id="4bf92f35..."`—every log line from every service involved in a single request
- `{container_name="catalog"} | json | book_id="abc123"`—everything that happened to a specific book

The `trace_id` field is the bridge between logging and tracing. When you see a slow trace in Tempo, you click it, and Grafana queries Loki for all log lines with that `trace_id`. This is trace-to-log correlation, and it only works if your logs are structured and carry the trace ID.

In Java, SLF4J with Logback does this via MDC (Mapped Diagnostic Context). Spring Cloud Sleuth automatically puts `traceId` and `spanId` into the MDC, and your `logback.xml` pattern includes `%X{traceId}`. The Go equivalent is passing `context.Context` through the call chain and extracting span information from it at log time.

---

## Go's `log/slog` Package

Go 1.21 introduced `log/slog` in the standard library[^1]. Before this, Go's `log` package was deliberately minimal—`log.Printf("message: %v", err)` with no levels, no structured fields, no handlers. The community used third-party libraries (zerolog, zap, logrus) to fill the gap. `slog` brings structured logging into the stdlib with a design that learned from all of them.

### Core Concepts

**Logger**—the type you call methods on: `slog.Info()`, `slog.Error()`, `slog.InfoContext()`. There is a global default logger accessed via package-level functions.

**Handler**—the interface that decides how to format and write log records. Two built-in handlers exist:
- `slog.TextHandler`—human-readable key=value format
- `slog.JSONHandler`—machine-readable JSON (what we use in production)

**Record**—a single log entry containing the time, level, message, and attributes.

**Attributes**—key-value pairs attached to a record. Added via variadic arguments:

```go
slog.Info("book created", "book_id", "abc123", "title", "The Go Programming Language")
```

This produces:
```json
{"time":"...","level":"INFO","msg":"book created","book_id":"abc123","title":"The Go Programming Language"}
```

**Levels**—`slog.LevelDebug`, `slog.LevelInfo`, `slog.LevelWarn`, `slog.LevelError`. These map directly to SLF4J's `DEBUG`, `INFO`, `WARN`, `ERROR`.

### The `Handler` Interface

The power of `slog` is in the `Handler` interface:

```go
type Handler interface {
    Enabled(context.Context, Level) bool
    Handle(context.Context, Record) error
    WithAttrs(attrs []Attr) Handler
    WithGroup(name string) Handler
}
```

- `Enabled`—called before constructing the record. Return `false` to skip the log entirely (like SLF4J's `isDebugEnabled()`).
- `Handle`—called to process the record. This is where formatting and writing happen.
- `WithAttrs`—returns a new handler with pre-set attributes (like MDC values that persist across log calls).
- `WithGroup`—returns a new handler that nests attributes under a named group.

You can wrap one handler with another to add behavior. This is exactly what our `TraceLogHandler` does.

---

## The TraceLogHandler

Our custom handler wraps `slog.JSONHandler` and injects `trace_id` and `span_id` from the request context:

```go
// pkg/otel/loghandler.go

type TraceLogHandler struct {
    inner slog.Handler
}

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
```

The implementation is a decorator pattern. `Handle` is the interesting method:

1. Extract the span from the context via `trace.SpanFromContext(ctx)`. This is an OTel API call—it returns the current span stored in `ctx`, or a no-op span if none exists.
2. Check `sc.IsValid()`—this prevents injecting zero-valued trace/span IDs when there is no active trace (e.g., during startup logging before any request has arrived).
3. If valid, add `trace_id` and `span_id` as string attributes to the record.
4. Delegate to the inner `JSONHandler` for the actual JSON formatting and writing.

`WithAttrs` and `WithGroup` must return a new `TraceLogHandler` wrapping the inner handler's `WithAttrs`/`WithGroup` result. If you returned the inner handler directly, you would lose the trace injection behavior on derived loggers.

### Why This Works

The handler is registered as the default logger in `Init()`:

```go
slog.SetDefault(slog.New(NewTraceLogHandler(os.Stdout)))
```

From this point on, every `slog.InfoContext(ctx, ...)` call goes through our handler. The `ctx` parameter carries the OTel span (set by `otelhttp` or `otelgrpc` when they created the request span), so the trace fields appear automatically in every log line emitted during request processing.

### Comparison to Java SLF4J/MDC

In Spring with Sleuth/Micrometer Tracing, the pattern is:

1. A servlet filter puts `traceId` and `spanId` into the SLF4J MDC (a thread-local map)
2. The Logback pattern includes `%X{traceId} %X{spanId}`
3. Every log call on that thread automatically includes the fields

Go does not have thread-locals---goroutines are not threads, and context is explicit. Instead:

1. The `otelhttp`/`otelgrpc` middleware stores the span in `context.Context`
2. The context is passed explicitly through the call chain: `handler(ctx) → service(ctx) → repo(ctx)`
3. The `TraceLogHandler` extracts the span from the context at log time

The Go approach is more explicit but has the same outcome. The key discipline is: always pass `ctx` and always use `slog.InfoContext(ctx, ...)` instead of `slog.Info(...)`. If you forget the context, the log line is still emitted—it just lacks trace fields.

---

## Testing the Handler

The test file verifies all three cases: active span, no span, and invalid span context.

```go
// pkg/otel/loghandler_test.go

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
```

The test creates a real `TracerProvider` with an in-memory exporter (no network calls). It starts a span, logs through the handler, and checks that the JSON output contains the correct `trace_id` and `span_id`. The in-memory exporter (`tracetest.NewInMemoryExporter`) is part of the OTel SDK's test utilities—use it whenever you need to verify tracing behavior in unit tests without a live Collector.

The second test (`TestTraceLogHandler_WithoutSpan`) verifies that logging without a span does not inject empty trace fields. The third (`TestTraceLogHandler_InvalidSpanContext`) verifies that an invalid (zero-valued) span context is treated the same as no span.

---

## The slog Migration

Migrating from `log.Printf` to `slog` follows a consistent pattern:

| Old Pattern | New Pattern |
|-------------|-------------|
| `log.Printf("message: %v", err)` | `slog.ErrorContext(ctx, "message", "error", err)` |
| `log.Printf("info %s", val)` | `slog.InfoContext(ctx, "info", "key", val)` |
| `log.Println("starting...")` | `slog.Info("starting...")` |
| `log.Fatalf("fatal: %v", err)` | `slog.Error("fatal", "error", err); os.Exit(1)` |

The most important change is using `Context` variants (`InfoContext`, `ErrorContext`) whenever `ctx` is available. This is what triggers the trace injection.

### Logging Middleware

The gateway's logging middleware shows the pattern in action:

```go
// services/gateway/internal/middleware/logging.go

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(sw, r)
        slog.InfoContext(r.Context(), "http request",
            "method", r.Method,
            "path", r.URL.Path,
            "status", sw.status,
            "duration", time.Since(start),
        )
    })
}
```

Compare this to the pre-slog version from Chapter 5:

```go
log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
```

The old version produces a flat string that is hard to parse. The new version produces structured JSON with individual fields for method, path, status, and duration. And because it uses `r.Context()`, the `trace_id` and `span_id` appear automatically.

### Handler Layer

The render helpers in the gateway also use context-aware logging:

```go
// services/gateway/internal/handler/render.go

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
    // ...
    tmpl, ok := s.tmpl[name]
    if !ok {
        slog.ErrorContext(r.Context(), "template not found", "name", name)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }
    // ...
    if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
        slog.ErrorContext(r.Context(), "template error", "error", err)
    }
}
```

Every error log carries the request context, so if a template fails to render, you can trace the log back to the exact HTTP request that triggered it.

### When Context Is Not Available

During startup (before any request arrives), there is no active span. Use the non-context versions:

```go
slog.Info("catalog service listening", "port", grpcPort)
slog.Error("failed to connect to database", "error", err)
```

These log lines will not have `trace_id` or `span_id` fields, which is correct. Startup logs are not part of any request trace.

---

## Context Flow Through the Call Chain

For trace correlation to work, `ctx` must flow from the transport layer (where the span is created) through to the business logic and infrastructure (where logs are emitted). In our system, the flow is:

```
otelhttp middleware  →  creates span, stores in ctx
  → Auth middleware  →  passes ctx through
    → Logging middleware  →  logs with ctx (trace fields appear)
      → Handler method  →  receives ctx via r.Context()
        → gRPC client call  →  propagates ctx (and trace) to backend
```

On the backend side:

```
otelgrpc server handler  →  creates server span, stores in ctx
  → gRPC handler  →  receives ctx
    → Service method  →  receives ctx
      → slog.ErrorContext(ctx, ...)  →  trace fields appear
      → Repository method  →  receives ctx
        → GORM query  →  creates DB span from ctx
```

Every layer passes `ctx`. This is Go's explicit context propagation model. It requires more function parameters than Java's thread-local approach, but it is unambiguous—you can trace exactly where the context comes from by reading the code.

---

## Exercises

1. **Find a log line without context.** Search the codebase for any remaining `log.Printf` or `log.Println` calls in the gateway, catalog, or reservation services. If you find any, migrate them to `slog`. Consider whether they need `Context` variants or not.

2. **Add a custom attribute.** Modify the `TraceLogHandler` to also inject `service.name` from the OTel resource. You will need to pass the service name into the handler constructor and add it as a fixed attribute via `WithAttrs`.

3. **Write a test for the logging middleware.** Create a test that sends an HTTP request through the `Logging` middleware and verifies the structured output includes `method`, `path`, `status`, and `duration` fields. Use `httptest.NewRecorder` and a `bytes.Buffer` to capture the log output.

4. **Compare log output.** Start the system with `docker compose up`, create a book, and examine the raw log output from the catalog container (`docker compose logs catalog`). Identify which log lines have `trace_id` fields and which do not. Explain why.

5. **Simulate the MDC pattern.** Create a `slog.Logger` with pre-set attributes using `logger.With("request_id", "abc123")`. Log a message and verify the attribute appears in the output. How does this compare to putting a value in the MDC in Java?

---

## References

[^1]: [log/slog package documentation](https://pkg.go.dev/log/slog)—Official Go standard library documentation for structured logging.
[^2]: [slog proposal and design document](https://go.googlesource.com/proposal/+/master/design/56345-structured-logging.md)—Jonathan Amsterdam's design document explaining the rationale behind slog's API.
[^3]: [SLF4J MDC documentation](https://www.slf4j.org/manual.html#mdc)—The Java equivalent of context-aware logging, using thread-local storage.
[^4]: [OpenTelemetry Trace Context in Go](https://pkg.go.dev/go.opentelemetry.io/otel/trace#SpanFromContext)—API for extracting the current span from a Go context.
