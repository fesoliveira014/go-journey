# Chapter 8: Observability with OpenTelemetry — Design Spec

## Goal

Add end-to-end observability (tracing, metrics, structured logging) to the library system using OpenTelemetry and the Grafana stack. Instrument three services (gateway, catalog, reservation), deploy a shared OTel Collector with Tempo, Prometheus, Loki, and Grafana, and migrate from `log.Printf` to `slog` with trace-log correlation. Auth and search instrumentation are left as exercises.

## Architecture Overview

```
┌──────────┐  ┌──────────┐  ┌──────────────┐
│ Gateway  │  │ Catalog  │  │ Reservation  │
│ (HTTP)   │  │ (gRPC)   │  │ (gRPC)       │
└────┬─────┘  └────┬─────┘  └──────┬───────┘
     │  OTLP/gRPC  │               │
     └──────┬──────┘───────────────┘
            ▼
   ┌────────────────┐
   │ OTel Collector │
   │ (shared)       │
   └──┬──────┬──────┘
      │      │      │
      ▼      ▼      ▼
   Tempo  Prometheus  (Loki via Promtail)
      │      │            │
      └──────┴────────────┘
             ▼
          Grafana
```

Services export traces and metrics via OTLP/gRPC to a single shared OTel Collector. The collector fans out to Tempo (traces) and exposes a Prometheus scrape endpoint (metrics). Promtail collects JSON-structured logs from Docker containers and ships to Loki. Grafana provides a unified UI with trace-log correlation.

## Tech Stack

- `go.opentelemetry.io/otel` — OTel SDK for Go
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` — OTLP trace exporter
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` — OTLP metric exporter
- `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` — HTTP middleware
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` — gRPC interceptors
- `log/slog` — structured logging (stdlib, Go 1.21+)
- `gorm.io/plugin/opentelemetry/tracing` — GORM auto-instrumentation
- OTel Collector Contrib (`otel/opentelemetry-collector-contrib`)
- Grafana, Tempo, Prometheus, Loki, Promtail

---

## Part 1: Shared OTel Package (`pkg/otel`)

### File Structure

| File | Responsibility |
|------|---------------|
| `pkg/otel/otel.go` | `Init()` function: creates TracerProvider, MeterProvider, configures slog |
| `pkg/otel/loghandler.go` | Custom `slog.Handler` that injects `trace_id`/`span_id` from context |
| `pkg/otel/otel_test.go` | Tests for Init and shutdown |
| `pkg/otel/loghandler_test.go` | Tests for trace-log correlation handler |

### `Init` Function

```go
func Init(ctx context.Context, serviceName, collectorEndpoint string) (shutdown func(context.Context) error, err error)
```

Responsibilities:
1. Create an OTLP/gRPC trace exporter pointed at `collectorEndpoint`
2. Build a `TracerProvider` with a `BatchSpanProcessor`, resource attributes (`service.name`, `service.version`)
3. Register the `TracerProvider` globally via `otel.SetTracerProvider()`
4. Set up `propagation.TraceContext{}` as the global text map propagator
5. Create an OTLP/gRPC metric exporter
6. Build a `MeterProvider` with a `PeriodicReader` (30s interval)
7. Register the `MeterProvider` globally via `otel.SetMeterProvider()`
8. Create the custom slog handler and set it as the default logger via `slog.SetDefault()`
9. Return a composite shutdown function that flushes and shuts down both providers

### Custom Slog Handler

Wraps `slog.JSONHandler`. On each `Handle(ctx, record)` call:
- Extracts `trace.SpanFromContext(ctx)`
- If the span context is valid, adds `trace_id` and `span_id` string attributes to the record
- Delegates to the inner `JSONHandler`
- Logs without active spans emit normally (no trace fields)

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `OTEL_COLLECTOR_ENDPOINT` | `otel-collector:4317` | Collector gRPC endpoint |
| `OTEL_SERVICE_NAME` | (per service) | Service name in traces/metrics |

---

## Part 2: Service Instrumentation

### Gateway (HTTP)

**`main.go` changes:**
1. Call `otel.Init(ctx, "gateway", collectorEndpoint)` early in main, defer `shutdown(ctx)`
2. Wrap the `http.ServeMux` with `otelhttp.NewHandler(mux, "gateway")` — auto-creates spans for every HTTP request
3. Add `otelgrpc.NewClientHandler()` as a `grpc.DialOption` on all outgoing gRPC connections (auth, catalog, reservation, search)

**Spans produced:**
- `HTTP GET /search` → server span with `http.method`, `http.route`, `http.status_code`
- `catalog.v1.CatalogService/ListBooks` → client span propagated to catalog

### Catalog (gRPC Server + Kafka Producer + PostgreSQL)

**`main.go` changes:**
1. Call `otel.Init(ctx, "catalog", collectorEndpoint)`, defer shutdown
2. Add `otelgrpc.NewServerHandler()` as a `grpc.ServerOption` — auto-creates spans for incoming RPCs
3. Add GORM OpenTelemetry plugin: `db.Use(tracing.NewPlugin())` — auto-creates spans for every DB query
4. Kafka publisher: inject trace context into message headers before sending

**`internal/kafka/publisher.go` changes:**
- Before `SendMessage`, call `otel.GetTextMapPropagator().Inject(ctx, otelsarama.NewProducerMessageCarrier(msg))` to propagate trace context via Kafka headers
- Create a span `catalog.publish` wrapping the publish call

**`internal/consumer/consumer.go` changes:**
- On message receipt, call `otel.GetTextMapPropagator().Extract(ctx, otelsarama.NewConsumerMessageCarrier(msg))` to continue the trace
- Create a span `catalog.consume.reservation` wrapping event handling

**Custom metric:**
- `catalog.books.total` — Int64 gauge updated in `CreateBook` (+1) and `DeleteBook` (-1)
- Registered via `otel.Meter("catalog").Int64UpDownCounter("catalog.books.total")`

### Reservation (gRPC Server + Kafka Producer + PostgreSQL)

**`main.go` changes:**
- Same pattern as catalog: `otel.Init`, gRPC server handler, GORM plugin

**`internal/kafka/publisher.go` changes:**
- Same trace context injection pattern as catalog publisher

**`internal/service/service.go` changes:**
- The `Reserve` and `Return` methods already call catalog via gRPC — trace context propagates automatically via the client interceptor set up in `main.go`

### Kafka Trace Propagation Detail

The trace context propagation across Kafka works as follows:

1. **Producer side** (catalog or reservation):
   - Active span exists from the gRPC handler
   - `Inject()` writes `traceparent` header into Kafka message headers
   - Message is published with trace context embedded

2. **Consumer side** (catalog consumer for reservation events, search consumer for catalog events):
   - `Extract()` reads `traceparent` from Kafka message headers
   - Creates a new span linked to the producer's trace
   - The consumer span becomes a child of the producer span, creating a connected trace across the async boundary

If the `otelsarama` contrib package is not compatible with the sarama version in use, fall back to manual propagation using `propagation.TraceContext{}` with a custom `TextMapCarrier` adapter over sarama message headers.

### slog Migration

Replace all `log.Printf`/`log.Println`/`log.Fatalf` calls in the three instrumented services:

| Old Pattern | New Pattern |
|-------------|-------------|
| `log.Printf("message: %v", err)` | `slog.ErrorContext(ctx, "message", "error", err)` |
| `log.Printf("info %s", val)` | `slog.InfoContext(ctx, "info", "key", val)` |
| `log.Println("starting...")` | `slog.Info("starting...")` |
| `log.Fatalf("fatal: %v", err)` | `slog.Error("fatal", "error", err); os.Exit(1)` |

Where `ctx` is not available (startup code before server starts), use `slog.Info()` / `slog.Error()` without context — no trace fields will be injected, which is correct.

---

## Part 3: Observability Backend Stack

### New Docker Compose Services

| Service | Image | Ports | Purpose |
|---------|-------|-------|---------|
| `otel-collector` | `otel/opentelemetry-collector-contrib:0.115.0` | 4317 (OTLP gRPC), 8889 (Prometheus scrape) | Central telemetry pipeline |
| `tempo` | `grafana/tempo:2.7.0` | 4317 (OTLP gRPC), 3200 (API) | Distributed trace storage |
| `prometheus` | `prom/prometheus:v3.1.0` | 9090 | Metrics storage, scrapes OTel Collector |
| `loki` | `grafana/loki:3.3.0` | 3100 | Log aggregation |
| `promtail` | `grafana/promtail:3.3.0` | — | Collects Docker container logs → Loki |
| `grafana` | `grafana/grafana:11.5.0` | 3000 | Unified visualization |

### Configuration Files

| File | Purpose |
|------|---------|
| `deploy/otel-collector-config.yaml` | Collector pipeline: receivers, processors, exporters |
| `deploy/tempo-config.yaml` | Tempo storage config (local filesystem for dev) |
| `deploy/prometheus.yaml` | Prometheus scrape config targeting OTel Collector |
| `deploy/promtail-config.yaml` | Docker log scraping, JSON parsing, label extraction |
| `deploy/grafana/provisioning/datasources/datasources.yaml` | Auto-register Tempo, Prometheus, Loki datasources |
| `deploy/grafana/provisioning/dashboards/dashboards.yaml` | Dashboard provisioning config |
| `deploy/grafana/dashboards/library-system.json` | Pre-built "Library System Overview" dashboard |

### OTel Collector Config

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024

exporters:
  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true
  prometheus:
    endpoint: 0.0.0.0:8889

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/tempo]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

### Promtail Config

- Scrapes Docker container logs via `/var/run/docker.sock`
- Pipeline stages: JSON parse → extract `level`, `msg`, `trace_id`, `span_id` → label `level` and `container_name`
- Ships to `http://loki:3100/loki/api/v1/push`

### Grafana Provisioning

**Datasources (auto-configured on startup):**
- Tempo at `http://tempo:3200` — with "Trace to logs" configured to correlate with Loki using `trace_id` field
- Prometheus at `http://prometheus:9090`
- Loki at `http://loki:3100`

**Pre-built dashboard panels:**
- Request rate by service (Prometheus: `http_server_request_duration_seconds_count`)
- Error rate by service (Prometheus: `http_server_request_duration_seconds_count` filtered by `http.status_code >= 400`)
- Latency p50/p95/p99 (Prometheus histogram)
- gRPC method latency (Prometheus: `rpc_server_duration_seconds`)
- Trace search (Tempo: service name filter, min duration)
- Recent logs (Loki: `{container_name=~".*catalog.*|.*gateway.*|.*reservation.*"}`)

### Service Environment Updates

All three instrumented services get:
```
OTEL_COLLECTOR_ENDPOINT=otel-collector:4317
```

Added to `deploy/.env` and referenced in `docker-compose.yml` environment blocks.

---

## Part 4: Documentation

### Chapter Structure

| File | Content |
|------|---------|
| `docs/src/ch08/index.md` | Overview: three pillars, why observability matters, chapter learning objectives |
| `docs/src/ch08/otel-fundamentals.md` | OTel concepts: traces, spans, metrics, context propagation, collector architecture |
| `docs/src/ch08/instrumentation.md` | Instrumenting Go services: SDK setup, HTTP/gRPC/Kafka/DB auto-instrumentation, custom metrics |
| `docs/src/ch08/structured-logging.md` | slog migration, custom handler, trace-log correlation, comparison to Java SLF4J/Logback |
| `docs/src/ch08/grafana-stack.md` | Backend stack setup, Grafana dashboards, trace-to-log correlation, manual verification walkthrough |
| `docs/src/ch08/sidecar-pattern.md` | Sidecar collector pattern explanation (theory only, no implementation) |

### Sidecar Pattern Subsection Content

Covers:
- **What changes vs. shared collector:** Each service exports to its own local collector instance instead of a shared one. In Docker Compose, this means N additional containers. In Kubernetes, a DaemonSet runs one collector per node.
- **Docker Compose example (not implemented):** Shows the YAML for `otel-collector-catalog`, `otel-collector-reservation` etc., each with service-specific config, and how each service's `OTEL_COLLECTOR_ENDPOINT` points to `localhost:4317` within a shared network namespace.
- **Kubernetes DaemonSet manifest snippet:** Shows a `DaemonSet` spec for the OTel Collector with `hostPort: 4317`, and how services use the downward API (`status.hostIP`) to find their node-local collector.
- **Trade-offs table:** Shared vs. sidecar vs. DaemonSet — operational complexity, failure isolation, resource overhead, configuration flexibility.
- **When to switch:** Team size, number of services, different sampling/export requirements per service.

### Auth & Search Exercise

The instrumentation chapter includes a callout box:

> **Exercise:** Apply the same patterns to the auth and search services. You'll need to:
> 1. Call `otel.Init()` in each service's `main.go`
> 2. Add gRPC server interceptors
> 3. Add GORM plugin (auth) or note that Meilisearch doesn't have an OTel plugin (search — you'd create manual spans around Meilisearch calls)
> 4. Migrate `log.Printf` calls to `slog`

---

## Part 5: Testing Strategy

### Unit Tests

| Test | What it verifies |
|------|-----------------|
| `pkg/otel/otel_test.go` | `Init()` returns shutdown function; global providers are set; shutdown is idempotent |
| `pkg/otel/loghandler_test.go` | Trace fields present in JSON when span context is active; absent when no span; log levels map correctly |
| Custom metric test in catalog service tests | `catalog.books.total` counter increments on create, decrements on delete (using in-memory metric reader) |

### Existing Tests Unaffected

OTel is initialized only in `main.go`. All existing unit tests create services directly without calling `otel.Init()`, so they get noop providers by default. No test changes needed for existing test files.

### Manual Verification (Documented)

Step-by-step guide in the chapter:
1. `docker compose up` the full stack
2. Open Grafana at `http://localhost:3000`
3. Create a book via gateway → find trace in Tempo showing `gateway → catalog (gRPC) → PostgreSQL`
4. Reserve a book → find trace showing `gateway → reservation (gRPC) → Kafka publish` and linked consumer trace `Kafka consume → catalog UpdateAvailability → PostgreSQL`
5. Search for a book → verify HTTP span in gateway
6. Check Loki logs filtered by `trace_id` from step 3 → see correlated logs from gateway and catalog
7. Check Prometheus metrics in the dashboard panels

---

## File Summary

### New Files

| File | Responsibility |
|------|---------------|
| `pkg/otel/otel.go` | Shared OTel initialization |
| `pkg/otel/loghandler.go` | slog handler with trace correlation |
| `pkg/otel/otel_test.go` | Init tests |
| `pkg/otel/loghandler_test.go` | Log handler tests |
| `deploy/otel-collector-config.yaml` | Collector pipeline config |
| `deploy/tempo-config.yaml` | Tempo storage config |
| `deploy/prometheus.yaml` | Prometheus scrape config |
| `deploy/promtail-config.yaml` | Log collection config |
| `deploy/grafana/provisioning/datasources/datasources.yaml` | Grafana datasource auto-config |
| `deploy/grafana/provisioning/dashboards/dashboards.yaml` | Dashboard provisioning |
| `deploy/grafana/dashboards/library-system.json` | Pre-built dashboard |
| `docs/src/ch08/*.md` | 6 documentation files |

### Modified Files

| File | Change |
|------|--------|
| `services/gateway/cmd/main.go` | Add `otel.Init`, `otelhttp` wrapper, `otelgrpc` client options |
| `services/gateway/internal/handler/*.go` | slog migration (all handler files) |
| `services/gateway/internal/middleware/*.go` | slog migration |
| `services/catalog/cmd/main.go` | Add `otel.Init`, gRPC server handler, GORM plugin |
| `services/catalog/internal/kafka/publisher.go` | Trace context injection |
| `services/catalog/internal/consumer/consumer.go` | Trace context extraction |
| `services/catalog/internal/service/catalog.go` | slog migration, custom metric |
| `services/catalog/internal/handler/catalog.go` | slog migration |
| `services/reservation/cmd/main.go` | Add `otel.Init`, gRPC server handler, GORM plugin |
| `services/reservation/internal/kafka/publisher.go` | Trace context injection |
| `services/reservation/internal/service/service.go` | slog migration |
| `services/reservation/internal/handler/handler.go` | slog migration |
| `deploy/docker-compose.yml` | Add 6 observability containers, update service env vars |
| `deploy/docker-compose.dev.yml` | Volume mounts for config files |
| `deploy/.env` | Add `OTEL_COLLECTOR_ENDPOINT` |
| `go.work` | No change (pkg/otel already in workspace via pkg/auth pattern) |
| `docs/src/SUMMARY.md` | Add Chapter 8 entries |
