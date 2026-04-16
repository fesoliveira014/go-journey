# 9.2 Instrumenting Go Services

<!-- [STRUCTURAL] Good opener — explicit cross-ref back to 9.1 ("With the theory from section 9.1 in hand") orients the reader. The three-service preview matches the architecture diagram from 9.1. Keep. -->
<!-- [LINE EDIT] 43-word opening sentence. Split at "The pattern is the same each time": "With the theory from section 9.1 in hand, we now instrument three services: the gateway (HTTP), the catalog (gRPC + Kafka + PostgreSQL), and the reservation service (gRPC + Kafka + PostgreSQL). The pattern is the same each time. Initialize OTel once in `main()`, attach auto-instrumentation to transports, and add manual instrumentation where auto-instrumentation does not reach." -->
<!-- [COPY EDIT] Serial comma on "(HTTP), the catalog (gRPC + Kafka + PostgreSQL), and the reservation service" — correct (CMOS 6.19). -->
With the theory from section 9.1 in hand, we now instrument three services: the gateway (HTTP), the catalog (gRPC + Kafka + PostgreSQL), and the reservation service (gRPC + Kafka + PostgreSQL). The pattern is the same each time: initialize OTel once in `main()`, attach auto-instrumentation to transports, and add manual instrumentation where auto-instrumentation does not reach.

---

## The Shared `pkg/otel` Package

<!-- [STRUCTURAL] Setting up a shared package is the right sequence — teaches the "DRY it once, reuse everywhere" pattern before showing call sites. -->
<!-- [LINE EDIT] "a shared package in the monorepo, just like `pkg/auth` from earlier chapters" — good cross-ref. -->
All three services share a single `Init` function that configures the OTel SDK. This lives in `pkg/otel/otel.go` -- a shared package in the monorepo, just like `pkg/auth` from earlier chapters.

```go
// pkg/otel/otel.go

func Init(ctx context.Context, serviceName, serviceVersion, collectorEndpoint string) (func(context.Context) error, error) {
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(serviceVersion),
        ),
    )
    if err != nil {
        return nil, err
    }

    traceExporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(collectorEndpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(res),
    )
    otelgo.SetTracerProvider(tp)
    otelgo.SetTextMapPropagator(propagation.TraceContext{})

    metricExporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(collectorEndpoint),
        otlpmetricgrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    mp := metric.NewMeterProvider(
        metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(30*time.Second))),
        metric.WithResource(res),
    )
    otelgo.SetMeterProvider(mp)

    slog.SetDefault(slog.New(NewTraceLogHandler(os.Stdout)))

    var once sync.Once
    var shutdownErr error
    shutdown := func(ctx context.Context) error {
        once.Do(func() {
            shutdownErr = errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
            if shutdownErr != nil {
                slog.Default().Error("otel shutdown error", "err", shutdownErr)
            }
        })
        return shutdownErr
    }

    return shutdown, nil
}
```

<!-- [STRUCTURAL] The code dump is 60 lines. The walkthrough that follows breaks it into labeled pieces, which helps. Consider foreshadowing the walkthrough order above the code block so the reader knows what to look for. -->
Let us walk through each piece.

### Resource

<!-- [LINE EDIT] "semconv package provides standardized attribute keys" — good. -->
<!-- [COPY EDIT] "OpenTelemetry Semantic Conventions[^1]" — footnote placement after the term rather than the sentence; inconsistent with other chapters. Normalize. -->
<!-- [COPY EDIT] Please verify: current `semconv` API uses `semconv.ServiceName(serviceName)` helper in newer versions (v1.26+). The code uses `semconv.ServiceNameKey.String(serviceName)`, which still works but may be considered the older form. Confirm whichever is in the actual codebase. -->
A `resource.Resource` describes the entity producing telemetry. At minimum, it needs `service.name` so your traces and metrics are labeled with the originating service. The `semconv` package provides standardized attribute keys from the OpenTelemetry Semantic Conventions[^1] -- this is the OTel equivalent of well-known metric tags in Micrometer.

### TracerProvider

The `TracerProvider` is the factory for creating tracers. It is configured with:

<!-- [LINE EDIT] "dropped silently. Your application keeps running." — consider "dropped silently, but your application keeps running." — cleaner cause/effect. -->
<!-- [COPY EDIT] "BatchSpanProcessor" — one word, correct Go OTel type name. -->
- A **BatchSpanProcessor** that buffers completed spans and exports them in batches. This is critical for performance -- you do not want to make a network call for every span. The batch processor has a bounded queue; if the queue fills (e.g., the Collector is down), new spans are dropped silently. Your application keeps running.
- The **resource** we just created.

After creation, we register it globally with `otel.SetTracerProvider(tp)`. This means any code that calls `otel.Tracer("name")` gets a tracer backed by this provider. Libraries that depend on the OTel API will automatically use it.

### MeterProvider

<!-- [LINE EDIT] "flushes metric data every 30 seconds" — "flushes" is informal but OK. More precise: "periodically exports (every 30 seconds)". -->
<!-- [COPY EDIT] "30 seconds" — numeral + spelled-out unit is fine for prose per CMOS 10.49. OK. -->
<!-- [COPY EDIT] "StepMeterRegistry" — confirm Micrometer class name; current Micrometer uses `StepMeterRegistry` (base) and concrete `StepRegistryConfig`. OK as written. -->
Same pattern for metrics. The `MeterProvider` uses a `PeriodicReader` that flushes metric data every 30 seconds via the OTLP/gRPC exporter. This is the equivalent of Micrometer's `MeterRegistry` with a `StepMeterRegistry` that pushes at fixed intervals.

### TextMapPropagator

<!-- [LINE EDIT] "inject/extract" — slash acceptable as informal alternative listing (CMOS 6.106). Keep. -->
`propagation.TraceContext{}` implements the W3C Trace Context standard. Registering it globally means every OTel contrib library (otelhttp, otelgrpc) will use it to inject/extract `traceparent` headers. You register it once and forget about it.

### The Package Naming Collision

Notice the import alias at the top of `otel.go`:

```go
otelgo "go.opentelemetry.io/otel"
```

<!-- [STRUCTURAL] This subsection earns its place — it's a Go-specific pitfall that a Java reader would not anticipate. Good tutor move. -->
<!-- [LINE EDIT] "Without the alias, calling `otel.SetTracerProvider()` would refer to our own package, not the upstream OTel library." — clear, keep. -->
Our package is also named `otel` (it lives at `pkg/otel`). Without the alias, calling `otel.SetTracerProvider()` would refer to our own package, not the upstream OTel library. The alias `otelgo` resolves this. You will see this pattern in the service layer too -- the catalog service uses the same alias for the same reason.

<!-- [COPY EDIT] "fully qualified names" — hyphenate as compound adjective: "fully qualified names" is fine because "fully" is an -ly adverb (CMOS 7.82 exception). OK. -->
In Java, this would not happen because packages and imports use fully qualified names. In Go, import paths and package names are separate concepts, and collisions require aliases.

### The `sync.Once` Shutdown Pattern

<!-- [LINE EDIT] "The shutdown function uses `sync.Once` to ensure the TracerProvider and MeterProvider are shut down exactly once, even if `shutdown()` is called multiple times." — 27 words, clear. Keep. -->
<!-- [COPY EDIT] "`defer shutdown(ctx)` in `main()`" — code font correct. -->
The shutdown function uses `sync.Once` to ensure the TracerProvider and MeterProvider are shut down exactly once, even if `shutdown()` is called multiple times. This is a defensive pattern -- `defer shutdown(ctx)` in `main()` might execute alongside signal handlers or other cleanup code.

<!-- [COPY EDIT] "Go 1.20" — version-specific claim; verify that `errors.Join` was indeed introduced in Go 1.20 (it was). OK. -->
`errors.Join` (added in Go 1.20) combines multiple errors into one. If both shutdowns fail, the caller gets both error messages.

---

## Auto-Instrumentation

<!-- [STRUCTURAL] Pivot from shared-init code to auto-instrumentation is clean. -->
<!-- [LINE EDIT] "without modifying business logic" → "without touching business logic" — more colloquial. Either is fine. -->
Auto-instrumentation means attaching OTel to existing transports (HTTP, gRPC, SQL) without modifying business logic. You wrap your server or client with an OTel-aware middleware, and spans are created automatically.

### HTTP: `otelhttp`

The gateway wraps its entire handler chain with `otelhttp.NewHandler`:

```go
// services/gateway/cmd/main.go

var h http.Handler = mux
h = middleware.Auth(h, jwtSecret)
h = middleware.Logging(h)
h = otelhttp.NewHandler(h, "gateway")
```

<!-- [COPY EDIT] Numbered list uses periods at end of item 4 only? Check: list items end without periods consistently. Currently consistent. OK. -->
<!-- [COPY EDIT] "`http.method`, `http.route`, `http.status_code`, `http.request_content_length`" — current OTel Semantic Conventions use the `http.*` namespace; verify against HTTP semantic conventions v1.x. The exact attribute names changed in the transition to stable HTTP semconv (e.g., `http.request.method`). Please verify. -->
`otelhttp.NewHandler` is the outermost wrapper. For every incoming HTTP request, it:

1. Extracts trace context from the `traceparent` header (if present)
2. Creates a server span named after the HTTP route
3. Attaches standard attributes: `http.method`, `http.route`, `http.status_code`, `http.request_content_length`
4. Records the `http_server_request_duration_seconds` histogram metric

<!-- [COPY EDIT] "`WebMvcMetricsFilter`" and "`ServerHttpObservationFilter`" — product class names; verify casing. Spring's actual class is `ServerHttpObservationFilter`. OK. -->
This is the equivalent of Spring's `WebMvcMetricsFilter` combined with Micrometer Tracing's `ServerHttpObservationFilter`. One line of code gives you both tracing and HTTP metrics.

### gRPC: `otelgrpc`

For gRPC, instrumentation is attached via the `StatsHandler` option.

**Client side** (gateway calling backend services):

```go
// services/gateway/cmd/main.go

catalogConn, err := grpc.NewClient(catalogAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

<!-- [LINE EDIT] "follows the gRPC convention" — good. -->
Every outgoing gRPC call from the gateway creates a client span and propagates the trace context via gRPC metadata. The span name follows the gRPC convention: `catalog.v1.CatalogService/GetBook`.

**Server side** (catalog and reservation receiving calls):

```go
// services/catalog/cmd/main.go

grpcServer := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
    grpc.UnaryInterceptor(interceptor),
)
```

<!-- [LINE EDIT] "The result: when the gateway calls `GetBook`, you see a connected trace with a client span in the gateway and a server span in the catalog." — 30 words, clear. Keep. -->
The server handler extracts the trace context from incoming metadata and creates a server span. The result: when the gateway calls `GetBook`, you see a connected trace with a client span in the gateway and a server span in the catalog.

<!-- [STRUCTURAL] The "why StatsHandler, not interceptors" paragraph is an important historical footnote for Go OTel users. Keep — this is exactly the kind of "I was confused and then found out" tutoring the brief asks for. -->
<!-- [LINE EDIT] "Earlier versions of `otelgrpc` used `UnaryInterceptor`/`StreamInterceptor`" — OK; "used" is past tense, appropriate. -->
<!-- [COPY EDIT] "`StatsHandler`, not interceptors" — appositive construction with comma, correct (CMOS 6.28). -->
Note that `otelgrpc` uses `StatsHandler`, not interceptors. Earlier versions of `otelgrpc` used `UnaryInterceptor`/`StreamInterceptor`, but `StatsHandler` is the preferred approach since it works at a lower level and does not conflict with your own interceptors (like the auth interceptor).

### PostgreSQL: GORM Plugin

<!-- [STRUCTURAL] NOTE: earlier chapters of this book should confirm GORM is actually the ORM in use. Given the user has mentioned pgx in memory (ch04 isDuplicateKeyError typed pgx error), this chapter claims GORM, which may be inconsistent with the codebase. Please verify the chosen ORM matches what was introduced in chapter 4 or wherever persistence was covered. If the codebase uses pgx, this section needs to be rewritten for `otelpgx`. This is the single most impactful factual check in the chapter. -->
<!-- [COPY EDIT] Please verify: the project's database layer. Recent commits reference pgx (ch04 snippet updated to typed pgx error); if services use pgx rather than GORM, replace this entire subsection with `otelpgx` instrumentation. This is a critical consistency check. -->
GORM provides an official OTel plugin that creates a span for every database query:

```go
// services/catalog/cmd/main.go

if err := db.Use(tracing.NewPlugin()); err != nil {
    slog.Error("failed to add otel gorm plugin", "error", err)
}
```

<!-- [LINE EDIT] "After this, every `db.Find()`, `db.Create()`, `db.Delete()` call produces a span" — serial comma in code-method list; OK. -->
<!-- [COPY EDIT] "gorm.query" — verify plugin actually produces span names like this. Some versions use more specific names like `gorm.Find`. Please verify against the actual plugin version. -->
After this, every `db.Find()`, `db.Create()`, `db.Delete()` call produces a span with the SQL query as an attribute. In your trace waterfall, you will see spans like `gorm.query` nested under the gRPC handler span, with the actual SQL visible in the span attributes.

<!-- [LINE EDIT] "Go does not have a universal instrumentation agent -- you attach plugins explicitly per library" — good, honest statement of Go's instrumentation posture. Keep. -->
This is the equivalent of the Hibernate or JDBC auto-instrumentation you get with the Java OTel agent. The difference is that Go does not have a universal instrumentation agent -- you attach plugins explicitly per library.

---

## Manual Instrumentation: Kafka

<!-- [STRUCTURAL] Strong transition — establishes "here is where auto-instrumentation ends". -->
<!-- [COPY EDIT] "The `otelsarama` contrib package exists but may not be compatible with every version of the `sarama` library." — Please verify: `otelsarama` does exist at `go.opentelemetry.io/contrib/instrumentation/github.com/IBM/sarama/otelsarama`; recent commit history notes Sarama's maintenance status. Recommend citing. -->
<!-- [LINE EDIT] "we implement the `propagation.TextMapCarrier` interface ourselves" — clear. -->
HTTP and gRPC have mature contrib libraries that handle context propagation automatically. Kafka does not. The `otelsarama` contrib package exists but may not be compatible with every version of the `sarama` library. Our approach is manual: we implement the `propagation.TextMapCarrier` interface ourselves.

### The TextMapCarrier Adapter

<!-- [LINE EDIT] "with `Get(key)`, `Set(key, value)`, and `Keys()` methods" — serial comma, code font, correct. -->
<!-- [COPY EDIT] "`[]RecordHeader` (byte slices)" — parenthetical gloss fine. -->
The OTel propagator needs something that implements `TextMapCarrier` -- an interface with `Get(key)`, `Set(key, value)`, and `Keys()` methods. Sarama messages have headers, but they use `[]RecordHeader` (byte slices), not a string map. We bridge the gap with a small adapter:

```go
// services/catalog/internal/kafka/publisher.go

type headerCarrier struct {
    msg *sarama.ProducerMessage
}

func (c *headerCarrier) Get(key string) string {
    for _, h := range c.msg.Headers {
        if string(h.Key) == key {
            return string(h.Value)
        }
    }
    return ""
}

func (c *headerCarrier) Set(key, value string) {
    c.msg.Headers = append(c.msg.Headers, sarama.RecordHeader{
        Key:   []byte(key),
        Value: []byte(value),
    })
}

func (c *headerCarrier) Keys() []string {
    keys := make([]string, len(c.msg.Headers))
    for i, h := range c.msg.Headers {
        keys[i] = string(h.Key)
    }
    return keys
}
```

<!-- [STRUCTURAL] "critical glue" framing is good; the reader now understands why this tiny type matters. Keep. -->
<!-- [LINE EDIT] "Without it, your trace would end at the Kafka publish and a new, disconnected trace would start at the consumer." — great concrete consequence. Keep. -->
This is a small type, but it is the critical glue that makes cross-service tracing work across an asynchronous boundary. Without it, your trace would end at the Kafka publish and a new, disconnected trace would start at the consumer.

### Producer Side: Injecting Context

When publishing a message, we inject the current trace context into the Kafka headers:

```go
// services/catalog/internal/kafka/publisher.go

func (p *Publisher) Publish(ctx context.Context, event service.BookEvent) error {
    value, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }

    msg := &sarama.ProducerMessage{
        Topic: p.topic,
        Key:   sarama.StringEncoder(event.BookID),
        Value: sarama.ByteEncoder(value),
    }

    ctx, span := otelgo.Tracer("catalog").Start(ctx, "catalog.publish")
    defer span.End()

    otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})

    _, _, err = p.producer.SendMessage(msg)
    if err != nil {
        return fmt.Errorf("send kafka message: %w", err)
    }
    return nil
}
```

<!-- [LINE EDIT] 40-word sentence. Split: "`otelgo.GetTextMapPropagator().Inject(ctx, carrier)` reads the span context from `ctx` and writes the `traceparent` header into the Kafka message headers. The `ctx` already has an active span from the gRPC server handler (set up by `otelgrpc`), so the trace ID is automatically carried forward." — already two sentences; OK. -->
<!-- [COPY EDIT] "`traceparent`" — code font, correct. -->
`otelgo.GetTextMapPropagator().Inject(ctx, carrier)` reads the span context from `ctx` and writes the `traceparent` header into the Kafka message headers. The `ctx` already has an active span from the gRPC server handler (set up by `otelgrpc`), so the trace ID is automatically carried forward.

<!-- [LINE EDIT] "This span becomes a child of the gRPC handler span." — concise. Keep. -->
We then create a `catalog.publish` span to measure the Kafka send itself. This span becomes a child of the gRPC handler span.

<!-- [STRUCTURAL] Noting that the reservation service mirrors the catalog pattern saves a whole duplicated code block. Good tutor economy. -->
The reservation service has an identical pattern in `services/reservation/internal/kafka/publisher.go` -- same `headerCarrier` adapter, same `Inject` + `Start` sequence.

### Consumer Side: Extracting Context

On the receiving end, the catalog's consumer extracts the trace context from incoming Kafka messages:

```go
// services/catalog/internal/consumer/consumer.go

type consumerHeaderCarrier []*sarama.RecordHeader

func (c consumerHeaderCarrier) Get(key string) string {
    for _, h := range c {
        if string(h.Key) == key {
            return string(h.Value)
        }
    }
    return ""
}

func (c consumerHeaderCarrier) Set(key, value string) {
    // Consumer carrier is read-only; Set is a no-op.
}

func (c consumerHeaderCarrier) Keys() []string {
    keys := make([]string, len(c))
    for i, h := range c {
        keys[i] = string(h.Key)
    }
    return keys
}
```

<!-- [LINE EDIT] "Note the `Set` method is a no-op." — consider "The `Set` method is a no-op." — drop "Note". -->
<!-- [COPY EDIT] "no-op" — hyphenated compound; correct. -->
Note the `Set` method is a no-op. The consumer only reads headers; it never writes them back. This is a valid implementation -- the `TextMapCarrier` interface requires the method to exist, but the propagator only calls `Get` during extraction.

The consumer uses `Extract` to rebuild the context, then starts a new span linked to the producer's trace:

```go
// services/catalog/internal/consumer/consumer.go

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    ctx := session.Context()
    for msg := range claim.Messages() {
        msgCtx := otelgo.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
        msgCtx, span := otelgo.Tracer("catalog").Start(msgCtx, "catalog.consume.reservation")
        if err := handleEvent(msgCtx, h.svc, msg.Value); err != nil {
            slog.ErrorContext(msgCtx, "failed to handle event", "error", err)
            span.End()
            continue
        }
        span.End()
        session.MarkMessage(msg, "")
    }
    return nil
}
```

<!-- [STRUCTURAL] "a single trace connects the gRPC request that triggered the publish, the Kafka message, and the consumer processing" — the payoff line. Excellent. Keep. -->
<!-- [LINE EDIT] 40-word sentence. Split possibilities exist but sequence of clauses reads smoothly. OK. -->
<!-- [COPY EDIT] Serial comma in "the gRPC request that triggered the publish, the Kafka message, and the consumer processing" — correct (CMOS 6.19). -->
`Extract` reads the `traceparent` header from the Kafka message and returns a new context with the trace information. When we call `Start` with that context, the new `catalog.consume.reservation` span becomes a child of the producer's span. The result: a single trace connects the gRPC request that triggered the publish, the Kafka message, and the consumer processing.

<!-- [LINE EDIT] "This is the asynchronous equivalent of what `otelhttp` and `otelgrpc` do for synchronous calls. The difference is that HTTP/gRPC propagation is handled by contrib libraries, while Kafka propagation requires the manual adapter pattern." — clean summary. Keep. -->
This is the asynchronous equivalent of what `otelhttp` and `otelgrpc` do for synchronous calls. The difference is that HTTP/gRPC propagation is handled by contrib libraries, while Kafka propagation requires the manual adapter pattern.

---

## Custom Metrics: The Book Counter

<!-- [STRUCTURAL] Single custom metric example is the right pedagogical scope. One is enough to teach the pattern without overwhelming. -->
<!-- [LINE EDIT] "Beyond auto-instrumented metrics (HTTP latency, gRPC duration)" — good. -->
Beyond auto-instrumented metrics (HTTP latency, gRPC duration), we add one custom metric: `catalog.books.total`, an `Int64UpDownCounter` that tracks how many books exist in the catalog.

```go
// services/catalog/internal/service/catalog.go

var bookCounter, _ = otelgo.Meter("catalog").Int64UpDownCounter("catalog.books.total",
    metric.WithDescription("Total number of books in the catalog"),
)
```

<!-- [COPY EDIT] "The `otelgo.Meter("catalog")` call returns a meter from the global MeterProvider." — clear. -->
<!-- [LINE EDIT] "Before `Init()` runs, this returns a no-op meter -- the counter exists but does nothing." — good. -->
<!-- [COPY EDIT] Please verify: "Before `Init()` runs, this returns a no-op meter — the counter exists but does nothing." — semantically the instrument is bound to a meter provider at creation time. If the global provider is swapped AFTER this package init, subsequent calls still go through the originally-bound meter. In practice this works because the global provider's meter re-resolves via a proxy. Worth a brief note, or trust that the current code does rebinding correctly. Confirm against Go SDK behavior. -->
This is a package-level variable, initialized when the package loads. The `otelgo.Meter("catalog")` call returns a meter from the global MeterProvider. Before `Init()` runs, this returns a no-op meter -- the counter exists but does nothing. After `Init()` registers a real MeterProvider, the counter starts recording.

The counter is used in two places:

```go
func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
    // ... create book ...
    bookCounter.Add(ctx, 1)
    // ...
}

func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
    // ... delete book ...
    bookCounter.Add(ctx, -1)
    // ...
}
```

<!-- [STRUCTURAL] This section makes the Counter/UpDownCounter distinction crisper than the 9.1 table did (which conflated Gauge with UpDownCounter). Good. Perhaps back-reference from here to fix the 9.1 confusion: "(see table in 9.1; the instrument we're using is specifically an UpDownCounter, not a Gauge.)" -->
<!-- [LINE EDIT] "Regular counters are monotonically increasing -- they only go up or reset." — OK. -->
An `UpDownCounter` (as opposed to a regular `Counter`) can decrease. Regular counters are monotonically increasing -- they only go up or reset. An UpDownCounter models values that naturally go up and down, like queue depth or, in our case, the number of books.

<!-- [COPY EDIT] "push vs. pull" — "vs." acceptable. -->
<!-- [COPY EDIT] "(a value you *observe*, like CPU temperature)" — italic emphasis is CMOS-acceptable (7.51); OK. -->
In Micrometer, this would be a `Gauge` backed by an `AtomicLong`. The semantics are similar, but OTel distinguishes between "gauge" (a value you *observe*, like CPU temperature) and "UpDownCounter" (a value you *add to*). The distinction matters for backends that handle push vs. pull metrics differently.

---

## How Existing Tests Remain Unaffected

<!-- [STRUCTURAL] Important reassurance for a learner. Well-placed near the end. Keep. -->
<!-- [LINE EDIT] "A natural concern: does adding OTel break existing unit tests? No." — good dialogic style. Keep. -->
<!-- [LINE EDIT] 40-word sentence: "The key design decision is that `Init()` is only called in `main()`. Unit tests that create service objects directly (e.g., `service.NewCatalogService(mockRepo, mockPublisher)`) never call `Init()`. The global TracerProvider and MeterProvider remain no-ops, so:" — already broken up. Good. -->
A natural concern: does adding OTel break existing unit tests? No. The key design decision is that `Init()` is only called in `main()`. Unit tests that create service objects directly (e.g., `service.NewCatalogService(mockRepo, mockPublisher)`) never call `Init()`. The global TracerProvider and MeterProvider remain no-ops, so:

- `otelgo.Tracer("catalog").Start(ctx, "name")` returns a no-op span
- `bookCounter.Add(ctx, 1)` does nothing
- `slog` uses its default text handler (no trace fields injected)

<!-- [LINE EDIT] "No test changes needed." — nice tight summary. Keep. -->
<!-- [COPY EDIT] "production entry point" — no hyphen needed; "production entry point" reads cleanly as noun phrase. OK. -->
No test changes needed. This is the benefit of the API/SDK split: the API works everywhere (as a no-op if no SDK is present), and the SDK is only configured in the production entry point.

---

## Exercises

<!-- [STRUCTURAL] Five exercises, ranging from cloning the pattern (exercise 1) to actual failure injection (exercise 5). Good progression. -->
<!-- [COPY EDIT] "(services/auth/cmd/main.go)" — code font missing for file path. Other exercises use backticks around paths. Normalize: "`services/auth/cmd/main.go`". -->
1. **Instrument the auth service.** Apply the same patterns to `services/auth/cmd/main.go`: call `pkgotel.Init()`, add `otelgrpc.NewServerHandler()` to the gRPC server, add the GORM tracing plugin. Verify that login traces now show the auth service as a span.

<!-- [COPY EDIT] "Meilisearch" — product capitalization correct (one word, capital M). Verify on their brand page. -->
<!-- [LINE EDIT] Exercise 2: good prompt that requires thinking about span attributes. Keep. -->
2. **Instrument the search service.** The search service uses Meilisearch, which has no OTel plugin. Create manual spans around Meilisearch client calls using `otel.Tracer("search").Start(ctx, "meilisearch.search")`. How would you record the query string as a span attribute?

<!-- [LINE EDIT] Exercise 3: concrete and verifiable. Good. -->
3. **Add a custom metric.** Add a `reservation.active.total` UpDownCounter to the reservation service that increments on `Reserve()` and decrements on `Return()`. Verify it appears in Prometheus after running the stack.

<!-- [LINE EDIT] "`docker compose up`" — space variant (new CLI) vs. `docker-compose up` (legacy). Modern form is correct. Verify consistency with other chapters. -->
4. **Trace a full request.** With the stack running (`docker compose up`), create a book via the gateway, then reserve it. Open Tempo in Grafana and find the trace. You should see spans from gateway, catalog (gRPC + DB + Kafka publish), and the linked consumer trace. Count the total number of spans.

<!-- [LINE EDIT] Exercise 5: "Simulate Collector failure" — practical and pedagogically valuable. Keep. -->
<!-- [COPY EDIT] "`docker compose stop otel-collector`" — modern form, correct. -->
5. **Simulate Collector failure.** Stop the OTel Collector container (`docker compose stop otel-collector`). Make several requests through the gateway. What happens? Do requests still succeed? What happens to traces? Restart the Collector and check if any buffered traces appear.

---

## References

<!-- [COPY EDIT] Please verify URLs: all five are current. -->
<!-- [COPY EDIT] "[^1]: [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)" — verify link; confirmed stable URL. OK. -->
<!-- [COPY EDIT] "[^4]: [GORM OpenTelemetry Plugin](https://github.com/go-gorm/opentelemetry)" — verify link resolves; if project uses pgx not GORM this reference needs replacing with `otelpgx`. -->
<!-- [COPY EDIT] "[^5]: [W3C Trace Context in Kafka]" — the link text is misleading. The linked page is OTel's generic propagator specification, not Kafka-specific. Consider retitling: "OTel Context Propagation Specification" or linking a Kafka-specific doc. -->
[^1]: [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) -- Standardized attribute names for spans and metrics across all languages.
[^2]: [otelhttp package documentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) -- Auto-instrumentation for `net/http` servers and clients.
[^3]: [otelgrpc package documentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc) -- Auto-instrumentation for gRPC servers and clients.
[^4]: [GORM OpenTelemetry Plugin](https://github.com/go-gorm/opentelemetry) -- Automatic span creation for GORM database operations.
[^5]: [W3C Trace Context in Kafka](https://opentelemetry.io/docs/specs/otel/context/api-propagators/) -- OTel specification for context propagation across messaging systems.
