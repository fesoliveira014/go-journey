# 9.3 Structured Logging with slog

<!-- [STRUCTURAL] Strong framing sentence: "Traces and metrics tell you what happened and how fast. Logs tell you why." That's the whole value proposition of the section. Keep. -->
<!-- [LINE EDIT] "But only if you can find the right log lines, and only if they contain enough context to be useful." — good. Might tighten the double "only if" to one. -->
<!-- [COPY EDIT] "`log/slog`" — code font correct; lowercase per project style. OK. -->
Traces and metrics tell you what happened and how fast. Logs tell you why. But only if you can find the right log lines, and only if they contain enough context to be useful. This section covers Go's `log/slog` package, our custom `TraceLogHandler`, and the migration from `log.Printf` to structured, context-aware logging.

---

## Why Structured Logging Matters

<!-- [STRUCTURAL] Good side-by-side comparison — shows the reader the "before and after" in concrete form rather than telling them structured is better. -->
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

<!-- [LINE EDIT] "The unstructured line is readable by a human scanning a terminal. The structured line is queryable by a machine." — classic contrast; keep. -->
The unstructured line is readable by a human scanning a terminal. The structured line is queryable by a machine. With structured logs flowing into Loki, you can write queries like:

<!-- [COPY EDIT] Code-fenced single-line queries use LogQL syntax. OK. -->
<!-- [COPY EDIT] "`{container_name=~".+"} | json | trace_id="4bf92f35..."`" — the regex `.+` will match every container, which is the intent. OK. -->
- `{container_name="catalog"} | json | level="ERROR"` -- all errors from the catalog
- `{container_name=~".+"} | json | trace_id="4bf92f35..."` -- every log line from every service involved in a single request
- `{container_name="catalog"} | json | book_id="abc123"` -- everything that happened to a specific book

<!-- [STRUCTURAL] The "click a slow trace → see every log line" workflow is the key payoff. Good, and it sets up the TraceLogHandler discussion below. -->
<!-- [LINE EDIT] 37-word sentence. Split: "The `trace_id` field is the bridge between logging and tracing. When you see a slow trace in Tempo, you click it, and Grafana queries Loki for all log lines with that `trace_id`. This is trace-to-log correlation, and it only works if your logs are structured and carry the trace ID." — already three sentences; OK. -->
The `trace_id` field is the bridge between logging and tracing. When you see a slow trace in Tempo, you click it, and Grafana queries Loki for all log lines with that `trace_id`. This is trace-to-log correlation, and it only works if your logs are structured and carry the trace ID.

<!-- [LINE EDIT] "In the Java world, SLF4J with Logback does this via MDC (Mapped Diagnostic Context)." — good; MDC expansion on this occurrence helps readers. -->
<!-- [COPY EDIT] "%X{traceId}" — Logback pattern syntax correct. -->
<!-- [COPY EDIT] "SLF4J with Logback" — product names correct. -->
In the Java world, SLF4J with Logback does this via MDC (Mapped Diagnostic Context). Spring Cloud Sleuth automatically puts `traceId` and `spanId` into the MDC, and your `logback.xml` pattern includes `%X{traceId}`. The Go equivalent is passing `context.Context` through the call chain and extracting span information from it at log time.

---

## Go's `log/slog` Package

<!-- [COPY EDIT] Please verify: "Go 1.21 introduced `log/slog`" — confirmed; slog landed in Go 1.21 (August 2023). OK. -->
Go 1.21 introduced `log/slog` in the standard library[^1]. Before this, Go's `log` package was deliberately minimal -- `log.Printf("message: %v", err)` with no levels, no structured fields, no handlers. The community used third-party libraries (zerolog, zap, logrus) to fill the gap. `slog` brings structured logging into the stdlib with a design that learned from all of them.

### Core Concepts

<!-- [STRUCTURAL] Bulletted definitions are clean. Each concept gets one or two sentences. Good tutorial rhythm. -->
<!-- [COPY EDIT] "`slog.Info()`, `slog.Error()`, `slog.InfoContext()`" — serial comma, code font correct. -->
**Logger** -- the type you call methods on: `slog.Info()`, `slog.Error()`, `slog.InfoContext()`. There is a global default logger accessed via package-level functions.

**Handler** -- the interface that decides how to format and write log records. Two built-in handlers exist:
- `slog.TextHandler` -- human-readable key=value format
- `slog.JSONHandler` -- machine-readable JSON (what we use in production)

**Record** -- a single log entry containing the time, level, message, and attributes.

**Attributes** -- key-value pairs attached to a record. Added via variadic arguments:

```go
slog.Info("book created", "book_id", "abc123", "title", "The Go Programming Language")
```

This produces:
```json
{"time":"...","level":"INFO","msg":"book created","book_id":"abc123","title":"The Go Programming Language"}
```

<!-- [LINE EDIT] "These map directly to SLF4J's `DEBUG`, `INFO`, `WARN`, `ERROR`." — good cross-reference. -->
<!-- [COPY EDIT] Serial comma in four-level list — correct. -->
**Levels** -- `slog.LevelDebug`, `slog.LevelInfo`, `slog.LevelWarn`, `slog.LevelError`. These map directly to SLF4J's `DEBUG`, `INFO`, `WARN`, `ERROR`.

### The `Handler` Interface

<!-- [STRUCTURAL] Showing the full interface signature before explaining each method is exactly the right move — the reader can match explanations to lines. Keep. -->
The power of `slog` is in the `Handler` interface:

```go
type Handler interface {
    Enabled(context.Context, Level) bool
    Handle(context.Context, Record) error
    WithAttrs(attrs []Attr) Handler
    WithGroup(name string) Handler
}
```

<!-- [LINE EDIT] "(like SLF4J's `isDebugEnabled()`)" — good tight Java analogy. -->
<!-- [COPY EDIT] "pre-set attributes" — hyphenated compound adjective before noun; correct (CMOS 7.81). Could be "preset" as closed compound per Merriam-Webster. Minor. -->
- `Enabled` -- called before constructing the record. Return `false` to skip the log entirely (like SLF4J's `isDebugEnabled()`).
- `Handle` -- called to process the record. This is where formatting and writing happen.
- `WithAttrs` -- returns a new handler with pre-set attributes (like MDC values that persist across log calls).
- `WithGroup` -- returns a new handler that nests attributes under a named group.

<!-- [LINE EDIT] "You can wrap one handler with another to add behavior." — good bridge into TraceLogHandler. -->
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

<!-- [STRUCTURAL] The four-step walkthrough of Handle() is well-paced and pedagogically sound. Keep. -->
The implementation is a decorator pattern. `Handle` is the interesting method:

<!-- [COPY EDIT] "via `trace.SpanFromContext(ctx)`" — OK; `trace.SpanFromContext` is the correct Go OTel API. -->
1. Extract the span from the context via `trace.SpanFromContext(ctx)`. This is an OTel API call -- it returns the current span stored in `ctx`, or a no-op span if none exists.
<!-- [LINE EDIT] "zero-valued trace/span IDs" — OK; hyphen correct. -->
2. Check `sc.IsValid()` -- this prevents injecting zero-valued trace/span IDs when there is no active trace (e.g., during startup logging before any request has arrived).
3. If valid, add `trace_id` and `span_id` as string attributes to the record.
4. Delegate to the inner `JSONHandler` for the actual JSON formatting and writing.

<!-- [LINE EDIT] "If you returned the inner handler directly, you would lose the trace injection behavior on derived loggers." — good explanation of the subtle pitfall. Keep. -->
<!-- [COPY EDIT] "`WithAttrs`/`WithGroup`" — slash separator OK (CMOS 6.106). -->
`WithAttrs` and `WithGroup` must return a new `TraceLogHandler` wrapping the inner handler's `WithAttrs`/`WithGroup` result. If you returned the inner handler directly, you would lose the trace injection behavior on derived loggers.

### Why This Works

<!-- [LINE EDIT] "From this point on" — phrase fine; could be "From then on". Minor. -->
The handler is registered as the default logger in `Init()`:

```go
slog.SetDefault(slog.New(NewTraceLogHandler(os.Stdout)))
```

<!-- [LINE EDIT] 43-word sentence. Split: "From this point on, every `slog.InfoContext(ctx, ...)` call goes through our handler. The `ctx` parameter carries the OTel span (set by `otelhttp` or `otelgrpc` when they created the request span), so trace fields appear automatically in every log line emitted during request processing." — minor tightening possible. -->
From this point on, every `slog.InfoContext(ctx, ...)` call goes through our handler. The `ctx` parameter carries the OTel span (set by `otelhttp` or `otelgrpc` when they created the request span), so the trace fields appear automatically in every log line emitted during request processing.

### Comparison to Java SLF4J/MDC

In Spring with Sleuth/Micrometer Tracing, the pattern is:

<!-- [LINE EDIT] Numbered step "A servlet filter puts..." — fine. -->
<!-- [COPY EDIT] "thread-local map" — compound adjective before noun, hyphenated (CMOS 7.81). OK. -->
1. A servlet filter puts `traceId` and `spanId` into the SLF4J MDC (a thread-local map)
2. The Logback pattern includes `%X{traceId} %X{spanId}`
3. Every log call on that thread automatically includes the fields

<!-- [LINE EDIT] "Go does not have thread-locals" — accurate (goroutines have no thread-local storage by design). -->
Go does not have thread-locals (goroutines are not threads, and context is explicit). Instead:

<!-- [COPY EDIT] Arrow characters `→` used in the "handler(ctx) → service(ctx) → repo(ctx)" chain — Unicode arrows, not ASCII "->". Consistent throughout chapter. OK. -->
1. The `otelhttp`/`otelgrpc` middleware stores the span in `context.Context`
2. The context is passed explicitly through the call chain: `handler(ctx) → service(ctx) → repo(ctx)`
3. The `TraceLogHandler` extracts the span from the context at log time

<!-- [STRUCTURAL] "The Go approach is more explicit but has the same outcome." — great tutorial line that respects both paradigms. Keep. -->
<!-- [LINE EDIT] "If you forget the context, the log line is still emitted — it just lacks trace fields." — keep; honest failure mode. -->
The Go approach is more explicit but has the same outcome. The key discipline is: always pass `ctx` and always use `slog.InfoContext(ctx, ...)` instead of `slog.Info(...)`. If you forget the context, the log line is still emitted -- it just lacks trace fields.

---

## Testing the Handler

<!-- [STRUCTURAL] Including a test example reinforces the "instrumentation shouldn't break tests" theme from 9.2. Good continuity. -->
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

<!-- [LINE EDIT] 50-word sentence. Split at "The in-memory exporter (...) is part of the OTel SDK's test utilities": "The test creates a real `TracerProvider` with an in-memory exporter (no network calls). It starts a span, logs through the handler, and checks that the JSON output contains the correct `trace_id` and `span_id`. The in-memory exporter, `tracetest.NewInMemoryExporter`, is part of the OTel SDK's test utilities — use it whenever you need to verify tracing behavior in unit tests without a live Collector." -->
<!-- [COPY EDIT] "`tracetest.NewInMemoryExporter`" — verify current package path; confirmed at `go.opentelemetry.io/otel/sdk/trace/tracetest`. OK. -->
The test creates a real `TracerProvider` with an in-memory exporter (no network calls). It starts a span, logs through the handler, and checks that the JSON output contains the correct `trace_id` and `span_id`. The in-memory exporter (`tracetest.NewInMemoryExporter`) is part of the OTel SDK's test utilities -- use it whenever you need to verify tracing behavior in unit tests without a live Collector.

<!-- [LINE EDIT] "The third (`TestTraceLogHandler_InvalidSpanContext`) verifies that an invalid (zero-valued) span context is treated the same as no span." — good detail that demonstrates defensive testing. Keep. -->
The second test (`TestTraceLogHandler_WithoutSpan`) verifies that logging without a span does not inject empty trace fields. The third (`TestTraceLogHandler_InvalidSpanContext`) verifies that an invalid (zero-valued) span context is treated the same as no span.

---

## The slog Migration

<!-- [STRUCTURAL] Migration-pattern table is the clearest way to present the before/after. Good pedagogical choice. -->
Migrating from `log.Printf` to `slog` follows a consistent pattern:

<!-- [COPY EDIT] Table columns "Old Pattern" / "New Pattern" use title case; consistent with other chapter tables. OK. -->
<!-- [COPY EDIT] Fourth row: "`slog.Error("fatal", "error", err); os.Exit(1)`" — semicolon-joined statements in one cell. Acceptable for comparison purposes. -->
| Old Pattern | New Pattern |
|-------------|-------------|
| `log.Printf("message: %v", err)` | `slog.ErrorContext(ctx, "message", "error", err)` |
| `log.Printf("info %s", val)` | `slog.InfoContext(ctx, "info", "key", val)` |
| `log.Println("starting...")` | `slog.Info("starting...")` |
| `log.Fatalf("fatal: %v", err)` | `slog.Error("fatal", "error", err); os.Exit(1)` |

<!-- [LINE EDIT] "The most important change is using `Context` variants (`InfoContext`, `ErrorContext`) whenever `ctx` is available. This is what triggers the trace injection." — keep. -->
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

<!-- [LINE EDIT] "Compare this to the pre-slog version from Chapter 5:" — good cross-ref. Verify Chapter 5 actually contains this line. -->
<!-- [COPY EDIT] Please verify: Chapter 5 contains the `log.Printf("%s %s %d %s", ...)` version. If the earlier chapter used a different log form, the cross-ref is wrong. -->
Compare this to the pre-slog version from Chapter 5:

```go
log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
```

<!-- [LINE EDIT] "The old version produces a flat string that is hard to parse. The new version produces structured JSON with individual fields for method, path, status, and duration." — good contrast. Keep. -->
<!-- [COPY EDIT] Serial comma in "method, path, status, and duration" — correct. -->
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

<!-- [LINE EDIT] "Every error log carries the request context, so if a template fails to render, you can trace the log back to the exact HTTP request that triggered it." — good concrete outcome. Keep. -->
Every error log carries the request context, so if a template fails to render, you can trace the log back to the exact HTTP request that triggered it.

### When Context Is Not Available

<!-- [LINE EDIT] "Startup logs are not part of any request trace." — good explanation of WHY no context is correct here. Keep. -->
During startup (before any request arrives), there is no active span. Use the non-context versions:

```go
slog.Info("catalog service listening", "port", grpcPort)
slog.Error("failed to connect to database", "error", err)
```

These log lines will not have `trace_id` or `span_id` fields, which is correct. Startup logs are not part of any request trace.

---

## Context Flow Through the Call Chain

<!-- [STRUCTURAL] The two ASCII flow diagrams (request-side / backend-side) are effective. Might also mention that the logging middleware itself sits BEFORE the span is populated with additional request attributes, which matters for trace ordering. Minor. -->
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

<!-- [COPY EDIT] Please verify: "GORM query → creates DB span from ctx" — same GORM consistency concern as 9.2. If the project uses pgx not GORM, change to "pgx query → creates DB span via otelpgx". -->
<!-- [LINE EDIT] 37-word sentence. "Every layer passes `ctx`. This is Go's explicit context propagation model. It requires more function parameters than Java's thread-local approach, but it is unambiguous — you can trace exactly where the context comes from by reading the code." — already multi-sentence; OK. -->
Every layer passes `ctx`. This is Go's explicit context propagation model. It requires more function parameters than Java's thread-local approach, but it is unambiguous -- you can trace exactly where the context comes from by reading the code.

---

## Exercises

<!-- [STRUCTURAL] Five exercises; mix of code archaeology (ex 1), extension (ex 2), testing (ex 3), investigation (ex 4), and Java comparison (ex 5). Good balance. -->
<!-- [LINE EDIT] Exercise 1: "Find a log line without context." — clear task. Keep. -->
1. **Find a log line without context.** Search the codebase for any remaining `log.Printf` or `log.Println` calls in the gateway, catalog, or reservation services. If you find any, migrate them to `slog`. Consider whether they need `Context` variants or not.

<!-- [LINE EDIT] Exercise 2: good technical exercise; forces reader to engage with resource attributes. Keep. -->
2. **Add a custom attribute.** Modify the `TraceLogHandler` to also inject `service.name` from the OTel resource. You will need to pass the service name into the handler constructor and add it as a fixed attribute via `WithAttrs`.

<!-- [COPY EDIT] "`httptest.NewRecorder`" — correct Go stdlib type. OK. -->
3. **Write a test for the logging middleware.** Create a test that sends an HTTP request through the `Logging` middleware and verifies the structured output includes `method`, `path`, `status`, and `duration` fields. Use `httptest.NewRecorder` and a `bytes.Buffer` to capture the log output.

<!-- [LINE EDIT] "(`docker compose logs catalog`)" — modern form. OK. -->
4. **Compare log output.** Start the system with `docker compose up`, create a book, and examine the raw log output from the catalog container (`docker compose logs catalog`). Identify which log lines have `trace_id` fields and which do not. Explain why.

<!-- [LINE EDIT] "`logger.With("request_id", "abc123")`" — correct slog API. OK. -->
5. **Simulate the MDC pattern.** Create a `slog.Logger` with pre-set attributes using `logger.With("request_id", "abc123")`. Log a message and verify the attribute appears in the output. How does this compare to putting a value in the MDC in Java?

---

## References

<!-- [COPY EDIT] Please verify all URLs resolve. -->
<!-- [COPY EDIT] "[^2]: [slog proposal and design document]" — URL is go.googlesource.com/proposal path. Verify still valid; the proposal document is at proposal/design/56345-structured-logging.md. Valid at publication. -->
<!-- [COPY EDIT] "[^4]: [OpenTelemetry Trace Context in Go]" — link text is slightly misleading (it's the `SpanFromContext` function, not the whole trace context). Consider retitling to "SpanFromContext (go.opentelemetry.io/otel/trace)". -->
[^1]: [log/slog package documentation](https://pkg.go.dev/log/slog) -- Official Go standard library documentation for structured logging.
[^2]: [slog proposal and design document](https://go.googlesource.com/proposal/+/master/design/56345-structured-logging.md) -- Jonathan Amsterdam's design document explaining the rationale behind slog's API.
[^3]: [SLF4J MDC documentation](https://www.slf4j.org/manual.html#mdc) -- The Java equivalent of context-aware logging, using thread-local storage.
[^4]: [OpenTelemetry Trace Context in Go](https://pkg.go.dev/go.opentelemetry.io/otel/trace#SpanFromContext) -- API for extracting the current span from a Go context.
