# 8.2 Instrumenting Go Services

With the theory from section 8.1 in hand, we now instrument three services: the gateway (HTTP), the catalog (gRPC + Kafka + PostgreSQL), and the reservation service (gRPC + Kafka + PostgreSQL). The pattern is the same each time: initialize OTel once in `main()`, attach auto-instrumentation to transports, and add manual instrumentation where auto-instrumentation does not reach.

---

## The Shared `pkg/otel` Package

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

Let us walk through each piece.

### Resource

A `resource.Resource` describes the entity producing telemetry. At minimum, it needs `service.name` so your traces and metrics are labeled with the originating service. The `semconv` package provides standardized attribute keys from the OpenTelemetry Semantic Conventions[^1] -- this is the OTel equivalent of well-known metric tags in Micrometer.

### TracerProvider

The `TracerProvider` is the factory for creating tracers. It is configured with:

- A **BatchSpanProcessor** that buffers completed spans and exports them in batches. This is critical for performance -- you do not want to make a network call for every span. The batch processor has a bounded queue; if the queue fills (e.g., the Collector is down), new spans are dropped silently. Your application keeps running.
- The **resource** we just created.

After creation, we register it globally with `otel.SetTracerProvider(tp)`. This means any code that calls `otel.Tracer("name")` gets a tracer backed by this provider. Libraries that depend on the OTel API will automatically use it.

### MeterProvider

Same pattern for metrics. The `MeterProvider` uses a `PeriodicReader` that flushes metric data every 30 seconds via the OTLP/gRPC exporter. This is the equivalent of Micrometer's `MeterRegistry` with a `StepMeterRegistry` that pushes at fixed intervals.

### TextMapPropagator

`propagation.TraceContext{}` implements the W3C Trace Context standard. Registering it globally means every OTel contrib library (otelhttp, otelgrpc) will use it to inject/extract `traceparent` headers. You register it once and forget about it.

### The Package Naming Collision

Notice the import alias at the top of `otel.go`:

```go
otelgo "go.opentelemetry.io/otel"
```

Our package is also named `otel` (it lives at `pkg/otel`). Without the alias, calling `otel.SetTracerProvider()` would refer to our own package, not the upstream OTel library. The alias `otelgo` resolves this. You will see this pattern in the service layer too -- the catalog service uses the same alias for the same reason.

In Java, this would not happen because packages and imports use fully qualified names. In Go, import paths and package names are separate concepts, and collisions require aliases.

### The `sync.Once` Shutdown Pattern

The shutdown function uses `sync.Once` to ensure the TracerProvider and MeterProvider are shut down exactly once, even if `shutdown()` is called multiple times. This is a defensive pattern -- `defer shutdown(ctx)` in `main()` might execute alongside signal handlers or other cleanup code.

`errors.Join` (added in Go 1.20) combines multiple errors into one. If both shutdowns fail, the caller gets both error messages.

---

## Auto-Instrumentation

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

`otelhttp.NewHandler` is the outermost wrapper. For every incoming HTTP request, it:

1. Extracts trace context from the `traceparent` header (if present)
2. Creates a server span named after the HTTP route
3. Attaches standard attributes: `http.method`, `http.route`, `http.status_code`, `http.request_content_length`
4. Records the `http_server_request_duration_seconds` histogram metric

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

Every outgoing gRPC call from the gateway creates a client span and propagates the trace context via gRPC metadata. The span name follows the gRPC convention: `catalog.v1.CatalogService/GetBook`.

**Server side** (catalog and reservation receiving calls):

```go
// services/catalog/cmd/main.go

grpcServer := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
    grpc.UnaryInterceptor(interceptor),
)
```

The server handler extracts the trace context from incoming metadata and creates a server span. The result: when the gateway calls `GetBook`, you see a connected trace with a client span in the gateway and a server span in the catalog.

Note that `otelgrpc` uses `StatsHandler`, not interceptors. Earlier versions of `otelgrpc` used `UnaryInterceptor`/`StreamInterceptor`, but `StatsHandler` is the preferred approach since it works at a lower level and does not conflict with your own interceptors (like the auth interceptor).

### PostgreSQL: GORM Plugin

GORM provides an official OTel plugin that creates a span for every database query:

```go
// services/catalog/cmd/main.go

if err := db.Use(tracing.NewPlugin()); err != nil {
    slog.Error("failed to add otel gorm plugin", "error", err)
}
```

After this, every `db.Find()`, `db.Create()`, `db.Delete()` call produces a span with the SQL query as an attribute. In your trace waterfall, you will see spans like `gorm.query` nested under the gRPC handler span, with the actual SQL visible in the span attributes.

This is the equivalent of the Hibernate or JDBC auto-instrumentation you get with the Java OTel agent. The difference is that Go does not have a universal instrumentation agent -- you attach plugins explicitly per library.

---

## Manual Instrumentation: Kafka

HTTP and gRPC have mature contrib libraries that handle context propagation automatically. Kafka does not. The `otelsarama` contrib package exists but may not be compatible with every version of the `sarama` library. Our approach is manual: we implement the `propagation.TextMapCarrier` interface ourselves.

### The TextMapCarrier Adapter

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

`otelgo.GetTextMapPropagator().Inject(ctx, carrier)` reads the span context from `ctx` and writes the `traceparent` header into the Kafka message headers. The `ctx` already has an active span from the gRPC server handler (set up by `otelgrpc`), so the trace ID is automatically carried forward.

We then create a `catalog.publish` span to measure the Kafka send itself. This span becomes a child of the gRPC handler span.

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

`Extract` reads the `traceparent` header from the Kafka message and returns a new context with the trace information. When we call `Start` with that context, the new `catalog.consume.reservation` span becomes a child of the producer's span. The result: a single trace connects the gRPC request that triggered the publish, the Kafka message, and the consumer processing.

This is the asynchronous equivalent of what `otelhttp` and `otelgrpc` do for synchronous calls. The difference is that HTTP/gRPC propagation is handled by contrib libraries, while Kafka propagation requires the manual adapter pattern.

---

## Custom Metrics: The Book Counter

Beyond auto-instrumented metrics (HTTP latency, gRPC duration), we add one custom metric: `catalog.books.total`, an `Int64UpDownCounter` that tracks how many books exist in the catalog.

```go
// services/catalog/internal/service/catalog.go

var bookCounter, _ = otelgo.Meter("catalog").Int64UpDownCounter("catalog.books.total",
    metric.WithDescription("Total number of books in the catalog"),
)
```

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

An `UpDownCounter` (as opposed to a regular `Counter`) can decrease. Regular counters are monotonically increasing -- they only go up or reset. An UpDownCounter models values that naturally go up and down, like queue depth or, in our case, the number of books.

In Micrometer, this would be a `Gauge` backed by an `AtomicLong`. The semantics are similar, but OTel distinguishes between "gauge" (a value you *observe*, like CPU temperature) and "UpDownCounter" (a value you *add to*). The distinction matters for backends that handle push vs. pull metrics differently.

---

## How Existing Tests Remain Unaffected

A natural concern: does adding OTel break existing unit tests? No. The key design decision is that `Init()` is only called in `main()`. Unit tests that create service objects directly (e.g., `service.NewCatalogService(mockRepo, mockPublisher)`) never call `Init()`. The global TracerProvider and MeterProvider remain no-ops, so:

- `otelgo.Tracer("catalog").Start(ctx, "name")` returns a no-op span
- `bookCounter.Add(ctx, 1)` does nothing
- `slog` uses its default text handler (no trace fields injected)

No test changes needed. This is the benefit of the API/SDK split: the API works everywhere (as a no-op if no SDK is present), and the SDK is only configured in the production entry point.

---

## Exercises

1. **Instrument the auth service.** Apply the same patterns to `services/auth/cmd/main.go`: call `pkgotel.Init()`, add `otelgrpc.NewServerHandler()` to the gRPC server, add the GORM tracing plugin. Verify that login traces now show the auth service as a span.

2. **Instrument the search service.** The search service uses Meilisearch, which has no OTel plugin. Create manual spans around Meilisearch client calls using `otel.Tracer("search").Start(ctx, "meilisearch.search")`. How would you record the query string as a span attribute?

3. **Add a custom metric.** Add a `reservation.active.total` UpDownCounter to the reservation service that increments on `Reserve()` and decrements on `Return()`. Verify it appears in Prometheus after running the stack.

4. **Trace a full request.** With the stack running (`docker compose up`), create a book via the gateway, then reserve it. Open Tempo in Grafana and find the trace. You should see spans from gateway, catalog (gRPC + DB + Kafka publish), and the linked consumer trace. Count the total number of spans.

5. **Simulate Collector failure.** Stop the OTel Collector container (`docker compose stop otel-collector`). Make several requests through the gateway. What happens? Do requests still succeed? What happens to traces? Restart the Collector and check if any buffered traces appear.

---

## References

[^1]: [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) -- Standardized attribute names for spans and metrics across all languages.
[^2]: [otelhttp package documentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) -- Auto-instrumentation for `net/http` servers and clients.
[^3]: [otelgrpc package documentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc) -- Auto-instrumentation for gRPC servers and clients.
[^4]: [GORM OpenTelemetry Plugin](https://github.com/go-gorm/opentelemetry) -- Automatic span creation for GORM database operations.
[^5]: [W3C Trace Context in Kafka](https://opentelemetry.io/docs/specs/otel/context/api-propagators/) -- OTel specification for context propagation across messaging systems.
