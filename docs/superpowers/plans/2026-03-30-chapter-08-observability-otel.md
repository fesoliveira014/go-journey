# Chapter 8: Observability with OpenTelemetry — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add end-to-end observability (tracing, metrics, structured logging) to the library system using OpenTelemetry and the Grafana stack, instrumenting gateway, catalog, and reservation services.

**Architecture:** Services export traces and metrics via OTLP/gRPC to a shared OTel Collector, which fans out to Tempo (traces) and exposes a Prometheus scrape endpoint (metrics). Promtail collects JSON-structured logs from Docker containers and ships to Loki. A shared `pkg/otel` package provides `Init()` for TracerProvider/MeterProvider setup and a custom slog handler for trace-log correlation. All `log.Printf` calls in the three instrumented services are migrated to `slog`.

**Tech Stack:** Go, OpenTelemetry SDK (`go.opentelemetry.io/otel`), OTLP gRPC exporters, `otelhttp`, `otelgrpc`, `otelgorm` (GORM plugin), `log/slog`, OTel Collector Contrib, Grafana, Tempo, Prometheus, Loki, Promtail, Docker Compose

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `pkg/otel/go.mod` | Module declaration for shared OTel package |
| `pkg/otel/otel.go` | `Init()` function: creates TracerProvider, MeterProvider, configures slog |
| `pkg/otel/otel_test.go` | Tests for Init and shutdown |
| `pkg/otel/loghandler.go` | Custom `slog.Handler` wrapping `slog.JSONHandler` — injects `trace_id`/`span_id` |
| `pkg/otel/loghandler_test.go` | Tests for trace-log correlation handler |
| `deploy/otel-collector-config.yaml` | Collector pipeline: OTLP receiver → batch processor → Tempo (traces) + Prometheus (metrics) |
| `deploy/tempo-config.yaml` | Tempo storage config (local filesystem for dev) |
| `deploy/prometheus.yaml` | Prometheus scrape config targeting OTel Collector on port 8889 |
| `deploy/promtail-config.yaml` | Docker log scraping, JSON parsing, label extraction → Loki |
| `deploy/grafana/provisioning/datasources/datasources.yaml` | Auto-register Tempo, Prometheus, Loki datasources |
| `deploy/grafana/provisioning/dashboards/dashboards.yaml` | Dashboard provisioning config |
| `deploy/grafana/dashboards/library-system.json` | Pre-built "Library System Overview" dashboard |
| `docs/src/ch08/index.md` | Chapter 8 overview stub |
| `docs/src/ch08/otel-fundamentals.md` | OTel concepts stub |
| `docs/src/ch08/instrumentation.md` | Instrumentation guide stub |
| `docs/src/ch08/structured-logging.md` | slog migration guide stub |
| `docs/src/ch08/grafana-stack.md` | Backend stack setup stub |
| `docs/src/ch08/sidecar-pattern.md` | Sidecar collector pattern docs stub |

### Modified Files

| File | Change |
|------|--------|
| `go.work` | Add `./pkg/otel` to workspace |
| `services/gateway/go.mod` | Add `pkg/otel`, `otelhttp`, `otelgrpc` dependencies |
| `services/gateway/cmd/main.go` | Call `otel.Init`, wrap mux with `otelhttp`, add `otelgrpc` dial options |
| `services/gateway/internal/middleware/logging.go` | Migrate `log.Printf` → `slog.InfoContext` |
| `services/gateway/internal/handler/render.go` | Migrate 6 `log.Printf` calls → `slog` |
| `services/catalog/go.mod` | Add `pkg/otel`, `otelgrpc`, `otelgorm` dependencies |
| `services/catalog/cmd/main.go` | Call `otel.Init`, add gRPC server handler, add GORM plugin |
| `services/catalog/internal/kafka/publisher.go` | Accept `context.Context`, inject trace context into Kafka headers |
| `services/catalog/internal/consumer/consumer.go` | Extract trace context from Kafka headers, migrate `log.Printf` → `slog` |
| `services/catalog/internal/service/catalog.go` | Migrate `log.Printf` → `slog`, add `catalog.books.total` custom metric |
| `services/reservation/go.mod` | Add `pkg/otel`, `otelgrpc`, `otelgorm` dependencies |
| `services/reservation/cmd/main.go` | Call `otel.Init`, add gRPC server handler, add GORM plugin, add `otelgrpc` client dial option |
| `services/reservation/internal/kafka/publisher.go` | Accept `context.Context`, inject trace context into Kafka headers |
| `services/reservation/internal/service/service.go` | Migrate `log.Printf` → `slog` |
| `deploy/docker-compose.yml` | Add 6 observability containers, add `OTEL_COLLECTOR_ENDPOINT` to gateway/catalog/reservation |
| `deploy/.env` | Add `OTEL_COLLECTOR_ENDPOINT` |
| `deploy/docker-compose.dev.yml` | Add `pkg/otel` volume mounts to catalog, gateway, reservation |
| `docs/src/SUMMARY.md` | Add Chapter 8 entries |

---

### Task 1: Shared OTel Package — Custom Slog Handler

**Files:**
- Create: `pkg/otel/go.mod`
- Create: `pkg/otel/loghandler.go`
- Create: `pkg/otel/loghandler_test.go`

This task creates the custom slog handler that injects `trace_id` and `span_id` from the context into JSON log records. We build and test this first since `Init()` depends on it.

- [ ] **Step 1: Create `pkg/otel/go.mod`**

```
module github.com/fesoliveira014/library-system/pkg/otel

go 1.26.1
```

Run: `cd /home/fesol/docs/go-journey/pkg/otel && go mod tidy`

This will pull the OTel SDK dependencies we need. Don't add explicit requires — let the code and `go mod tidy` resolve them.

- [ ] **Step 2: Write failing tests for the log handler**

Create `pkg/otel/loghandler_test.go`:

```go
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

	// Use a context with an invalid (non-recording) span
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
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/fesol/docs/go-journey/pkg/otel && go test ./... -v -count=1`
Expected: Compilation failure — `NewTraceLogHandler` not defined.

- [ ] **Step 4: Implement the log handler**

Create `pkg/otel/loghandler.go`:

```go
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
```

- [ ] **Step 5: Run `go mod tidy` and then tests**

Run: `cd /home/fesol/docs/go-journey/pkg/otel && go mod tidy && go test ./... -v -count=1`
Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/otel/go.mod pkg/otel/go.sum pkg/otel/loghandler.go pkg/otel/loghandler_test.go
git commit -m "feat: add pkg/otel with custom slog handler for trace-log correlation"
```

---

### Task 2: Shared OTel Package — Init Function

**Files:**
- Create: `pkg/otel/otel.go`
- Create: `pkg/otel/otel_test.go`
- Modify: `go.work` (add `./pkg/otel`)

This task creates the `Init()` function that sets up TracerProvider, MeterProvider, and the default slog logger. It also adds `pkg/otel` to the Go workspace.

- [ ] **Step 1: Add `./pkg/otel` to `go.work`**

Current `go.work`:
```
go 1.26.1

use (
	./gen
	./pkg/auth
	./services/auth
	./services/catalog
	./services/gateway
	./services/reservation
	./services/search
)
```

Add `./pkg/otel` after `./pkg/auth`:
```
go 1.26.1

use (
	./gen
	./pkg/auth
	./pkg/otel
	./services/auth
	./services/catalog
	./services/gateway
	./services/reservation
	./services/search
)
```

- [ ] **Step 2: Write failing tests for Init**

Create `pkg/otel/otel_test.go`:

```go
package otel

import (
	"context"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_SetsGlobalProviders(t *testing.T) {
	// Use empty endpoint — Init should still set global providers even if
	// the exporter connection fails later (exporters connect lazily).
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

	// Calling shutdown multiple times should not panic.
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/fesol/docs/go-journey/pkg/otel && go test ./... -v -count=1`
Expected: Compilation failure — `Init` not defined.

- [ ] **Step 4: Implement Init**

Create `pkg/otel/otel.go`:

```go
package otel

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"log/slog"
	"time"
)

// Init sets up OpenTelemetry tracing and metrics with OTLP/gRPC exporters.
// It configures the global TracerProvider, MeterProvider, text map propagator,
// and sets the default slog logger to use the TraceLogHandler.
// Returns a shutdown function that flushes and closes both providers.
func Init(ctx context.Context, serviceName, collectorEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collectorEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Metric exporter
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
	otel.SetMeterProvider(mp)

	// Structured logging with trace correlation
	slog.SetDefault(slog.New(NewTraceLogHandler(os.Stdout)))

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}

	return shutdown, nil
}
```

- [ ] **Step 5: Run `go mod tidy` and then tests**

Run: `cd /home/fesol/docs/go-journey/pkg/otel && go mod tidy && go test ./... -v -count=1`
Expected: All 5 tests PASS (3 from loghandler + 2 from otel).

- [ ] **Step 6: Commit**

```bash
git add go.work pkg/otel/otel.go pkg/otel/otel_test.go pkg/otel/go.mod pkg/otel/go.sum
git commit -m "feat: add otel.Init for TracerProvider, MeterProvider, and slog setup"
```

---

### Task 3: Observability Backend Configuration Files

**Files:**
- Create: `deploy/otel-collector-config.yaml`
- Create: `deploy/tempo-config.yaml`
- Create: `deploy/prometheus.yaml`
- Create: `deploy/promtail-config.yaml`
- Create: `deploy/grafana/provisioning/datasources/datasources.yaml`
- Create: `deploy/grafana/provisioning/dashboards/dashboards.yaml`
- Create: `deploy/grafana/dashboards/library-system.json`

This task creates all observability infrastructure configuration files. No Go code — just YAML and JSON configs.

- [ ] **Step 1: Create OTel Collector config**

Create `deploy/otel-collector-config.yaml`:

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

- [ ] **Step 2: Create Tempo config**

Create `deploy/tempo-config.yaml`:

```yaml
server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317

storage:
  trace:
    backend: local
    local:
      path: /var/tempo/traces
    wal:
      path: /var/tempo/wal
```

- [ ] **Step 3: Create Prometheus config**

Create `deploy/prometheus.yaml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "otel-collector"
    static_configs:
      - targets: ["otel-collector:8889"]
```

- [ ] **Step 4: Create Promtail config**

Create `deploy/promtail-config.yaml`:

```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
    relabel_configs:
      - source_labels: ["__meta_docker_container_name"]
        target_label: "container_name"
      - source_labels: ["__meta_docker_container_name"]
        regex: ".*(?:gateway|catalog|reservation).*"
        action: keep
    pipeline_stages:
      - json:
          expressions:
            level: level
            msg: msg
            trace_id: trace_id
            span_id: span_id
      - labels:
          level:
```

- [ ] **Step 5: Create Grafana datasource provisioning**

Create `deploy/grafana/provisioning/datasources/datasources.yaml`:

```yaml
apiVersion: 1

datasources:
  - name: Tempo
    type: tempo
    access: proxy
    url: http://tempo:3200
    isDefault: false
    jsonData:
      tracesToLogsV2:
        datasourceUid: loki
        filterByTraceID: true
        filterBySpanID: false
        customQuery: false

  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true

  - name: Loki
    uid: loki
    type: loki
    access: proxy
    url: http://loki:3100
```

- [ ] **Step 6: Create Grafana dashboard provisioning**

Create `deploy/grafana/provisioning/dashboards/dashboards.yaml`:

```yaml
apiVersion: 1

providers:
  - name: "Library System"
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: false
```

- [ ] **Step 7: Create pre-built Grafana dashboard**

Create `deploy/grafana/dashboards/library-system.json`:

```json
{
  "id": null,
  "uid": "library-system-overview",
  "title": "Library System Overview",
  "tags": ["library-system"],
  "timezone": "browser",
  "schemaVersion": 39,
  "version": 1,
  "refresh": "10s",
  "panels": [
    {
      "title": "Request Rate by Service",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 0 },
      "datasource": { "type": "prometheus", "uid": "" },
      "targets": [
        {
          "expr": "rate(http_server_request_duration_seconds_count[5m])",
          "legendFormat": "{{http_route}} {{http_method}}"
        }
      ]
    },
    {
      "title": "Request Latency p95",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 0 },
      "datasource": { "type": "prometheus", "uid": "" },
      "targets": [
        {
          "expr": "histogram_quantile(0.95, rate(http_server_request_duration_seconds_bucket[5m]))",
          "legendFormat": "p95"
        }
      ]
    },
    {
      "title": "gRPC Server Latency by Method",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 8 },
      "datasource": { "type": "prometheus", "uid": "" },
      "targets": [
        {
          "expr": "rate(rpc_server_duration_seconds_sum[5m]) / rate(rpc_server_duration_seconds_count[5m])",
          "legendFormat": "{{rpc_method}}"
        }
      ]
    },
    {
      "title": "Recent Logs",
      "type": "logs",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 8 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "{container_name=~\".*catalog.*|.*gateway.*|.*reservation.*\"}"
        }
      ]
    }
  ]
}
```

- [ ] **Step 8: Commit**

```bash
git add deploy/otel-collector-config.yaml deploy/tempo-config.yaml deploy/prometheus.yaml deploy/promtail-config.yaml deploy/grafana/
git commit -m "feat: add observability backend configs (collector, tempo, prometheus, promtail, grafana)"
```

---

### Task 4: Docker Compose — Observability Services

**Files:**
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/.env`

Add the 6 observability containers to Docker Compose and the `OTEL_COLLECTOR_ENDPOINT` env var.

- [ ] **Step 1: Add `OTEL_COLLECTOR_ENDPOINT` to `deploy/.env`**

Append to the end of `deploy/.env`:

```
OTEL_COLLECTOR_ENDPOINT=otel-collector:4317
```

- [ ] **Step 2: Add observability services to `deploy/docker-compose.yml`**

Add these services before the `volumes:` section:

```yaml
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.115.0
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml
    ports:
      - "4317:4317"
      - "8889:8889"
    networks:
      - library-net

  tempo:
    image: grafana/tempo:2.7.0
    volumes:
      - ./tempo-config.yaml:/etc/tempo/config.yaml
      - tempo-data:/var/tempo
    command: ["-config.file=/etc/tempo/config.yaml"]
    ports:
      - "3200:3200"
    networks:
      - library-net

  prometheus:
    image: prom/prometheus:v3.1.0
    volumes:
      - ./prometheus.yaml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
    networks:
      - library-net

  loki:
    image: grafana/loki:3.3.0
    ports:
      - "3100:3100"
    networks:
      - library-net

  promtail:
    image: grafana/promtail:3.3.0
    volumes:
      - ./promtail-config.yaml:/etc/promtail/config.yml
      - /var/run/docker.sock:/var/run/docker.sock:ro
    command: ["-config.file=/etc/promtail/config.yml"]
    depends_on:
      - loki
    networks:
      - library-net

  grafana:
    image: grafana/grafana:11.5.0
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Viewer
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./grafana/dashboards:/var/lib/grafana/dashboards
    ports:
      - "3000:3000"
    depends_on:
      - tempo
      - prometheus
      - loki
    networks:
      - library-net
```

Add `tempo-data:` to the `volumes:` section (alongside the existing `catalog-data:`, etc.).

- [ ] **Step 3: Add `OTEL_COLLECTOR_ENDPOINT` to gateway, catalog, and reservation services**

In the `gateway:` service's `environment:` block, add:

```yaml
      OTEL_COLLECTOR_ENDPOINT: ${OTEL_COLLECTOR_ENDPOINT:-otel-collector:4317}
```

In the `catalog:` service's `environment:` block, add:

```yaml
      OTEL_COLLECTOR_ENDPOINT: ${OTEL_COLLECTOR_ENDPOINT:-otel-collector:4317}
```

In the `reservation:` service's `environment:` block, add:

```yaml
      OTEL_COLLECTOR_ENDPOINT: ${OTEL_COLLECTOR_ENDPOINT:-otel-collector:4317}
```

- [ ] **Step 4: Update `deploy/docker-compose.dev.yml` with `pkg/otel` volume mounts**

Add `../pkg/otel:/app/pkg/otel` volume mount to the `catalog:`, `gateway:`, and `reservation:` services (after the existing `../pkg/auth:/app/pkg/auth` line in each).

For example, in `catalog:`:
```yaml
  catalog:
    build:
      context: ..
      dockerfile: services/catalog/Dockerfile.dev
    volumes:
      - ../services/catalog:/app/services/catalog
      - ../gen:/app/gen
      - ../pkg/auth:/app/pkg/auth
      - ../pkg/otel:/app/pkg/otel
```

Apply the same change to `gateway:` and `reservation:`.

- [ ] **Step 5: Commit**

```bash
git add deploy/docker-compose.yml deploy/docker-compose.dev.yml deploy/.env
git commit -m "feat: add observability services to docker-compose (collector, tempo, prometheus, loki, promtail, grafana)"
```

---

### Task 5: Gateway Instrumentation

**Files:**
- Modify: `services/gateway/cmd/main.go`
- Modify: `services/gateway/go.mod` (via `go mod tidy`)

This task instruments the gateway with OTel: HTTP tracing, gRPC client tracing, and slog setup.

- [ ] **Step 1: Add OTel imports and Init call to gateway main.go**

Add these imports to `services/gateway/cmd/main.go`:

```go
"context"
"log/slog"

pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
```

At the start of `main()`, before any env var reads, add:

```go
ctx := context.Background()
shutdown, err := pkgotel.Init(ctx, "gateway", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
if err != nil {
    slog.Error("failed to init otel", "error", err)
} else {
    defer shutdown(ctx)
}
```

Note: OTel init failure is not fatal — the service can run without observability (noop providers).

- [ ] **Step 2: Add `otelgrpc` dial option to all gRPC connections**

Change each `grpc.NewClient` call to include the stats handler. For example, for auth:

Before:
```go
authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
```

After:
```go
authConn, err := grpc.NewClient(authAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

Apply the same change to all 4 connections: `authConn`, `catalogConn`, `reservationConn`, `searchConn`.

- [ ] **Step 3: Wrap the mux with `otelhttp`**

Change the handler wrapping near the end of `main()`:

Before:
```go
var h http.Handler = mux
h = middleware.Auth(h, jwtSecret)
h = middleware.Logging(h)
```

After:
```go
var h http.Handler = mux
h = middleware.Auth(h, jwtSecret)
h = middleware.Logging(h)
h = otelhttp.NewHandler(h, "gateway")
```

- [ ] **Step 4: Migrate `log.Fatal`/`log.Fatalf`/`log.Printf` calls**

Replace the `"log"` import with `"log/slog"` and `"os"` (if not already imported).

Migrate calls according to these patterns:

| Old | New |
|-----|-----|
| `log.Fatal("JWT_SECRET is required")` | `slog.Error("JWT_SECRET is required"); os.Exit(1)` |
| `log.Fatalf("connect to auth service: %v", err)` | `slog.Error("connect to auth service", "error", err); os.Exit(1)` |
| `log.Fatalf("parse templates: %v", err)` | `slog.Error("parse templates", "error", err); os.Exit(1)` |
| `log.Printf("gateway listening on %s", addr)` | `slog.Info("gateway listening", "addr", addr)` |
| `log.Fatalf("server failed: %v", err)` | `slog.Error("server failed", "error", err); os.Exit(1)` |

Apply to all `log.*` calls in `main()`. Remove the `"log"` import.

- [ ] **Step 5: Run `go mod tidy` and verify compilation**

Run: `cd /home/fesol/docs/go-journey/services/gateway && go mod tidy && go build ./cmd/`
Expected: Compiles successfully.

- [ ] **Step 6: Run existing gateway tests**

Run: `cd /home/fesol/docs/go-journey/services/gateway && go test ./internal/... -v -count=1`
Expected: All existing tests PASS (they don't depend on OTel initialization).

- [ ] **Step 7: Commit**

```bash
git add services/gateway/cmd/main.go services/gateway/go.mod services/gateway/go.sum
git commit -m "feat: instrument gateway with otelhttp, otelgrpc, and slog"
```

---

### Task 6: Gateway Slog Migration (Handler & Middleware)

**Files:**
- Modify: `services/gateway/internal/middleware/logging.go`
- Modify: `services/gateway/internal/handler/render.go`

Migrate all `log.Printf` calls in the gateway's handler and middleware files to `slog`.

- [ ] **Step 1: Migrate `services/gateway/internal/middleware/logging.go`**

Current:
```go
import (
	"log"
	"net/http"
	"time"
)
```

Change to:
```go
import (
	"log/slog"
	"net/http"
	"time"
)
```

Change the log call:

Before:
```go
log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
```

After:
```go
slog.InfoContext(r.Context(), "http request",
    "method", r.Method,
    "path", r.URL.Path,
    "status", sw.status,
    "duration", time.Since(start),
)
```

Note: Using `r.Context()` here enables trace-log correlation — the trace_id/span_id will be injected by our custom handler.

- [ ] **Step 2: Migrate `services/gateway/internal/handler/render.go`**

Change import from `"log"` to `"log/slog"`.

Migrate calls. These render functions don't have access to `context.Context` directly, but `render` has `r *http.Request` and `renderError` has `r *http.Request`. `renderPartial` does not have a request — use `slog.Error` without context.

| Old | New |
|-----|-----|
| `log.Printf("template not found: %q", name)` | `slog.ErrorContext(r.Context(), "template not found", "name", name)` |
| `log.Printf("template error: %v", err)` (in `render`) | `slog.ErrorContext(r.Context(), "template error", "error", err)` |
| `log.Printf("no templates loaded; cannot render partial %q", name)` | `slog.Error("no templates loaded", "partial", name)` |
| `log.Printf("template error: %v", err)` (in `renderPartial`) | `slog.Error("template error", "error", err)` |
| `log.Printf("error template not found")` | `slog.ErrorContext(r.Context(), "error template not found")` |
| `log.Printf("template error: %v", err)` (in `renderError`) | `slog.ErrorContext(r.Context(), "template error", "error", err)` |

- [ ] **Step 3: Run existing tests**

Run: `cd /home/fesol/docs/go-journey/services/gateway && go test ./internal/... -v -count=1`
Expected: All existing tests PASS.

- [ ] **Step 4: Commit**

```bash
git add services/gateway/internal/middleware/logging.go services/gateway/internal/handler/render.go
git commit -m "feat: migrate gateway middleware and handler logging from log to slog"
```

---

### Task 7: Catalog Service Instrumentation

**Files:**
- Modify: `services/catalog/cmd/main.go`
- Modify: `services/catalog/go.mod` (via `go mod tidy`)

Instrument catalog with OTel: gRPC server tracing, GORM DB tracing, and slog setup.

- [ ] **Step 1: Add OTel imports and Init call to catalog main.go**

Add these imports:

```go
"log/slog"

pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
```

At the start of `main()`, before env var reads, add:

```go
ctx := context.Background()
shutdown, err := pkgotel.Init(ctx, "catalog", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
if err != nil {
    slog.Error("failed to init otel", "error", err)
} else {
    defer shutdown(ctx)
}
```

- [ ] **Step 2: Add GORM OTel plugin**

After `gorm.Open(...)`, add the GORM tracing plugin. First, check if the plugin is available and determine the correct import. The plan uses the GORM OpenTelemetry plugin.

Add the import:
```go
"gorm.io/plugin/opentelemetry/tracing"
```

After `gorm.Open`:
```go
db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
if err != nil {
    slog.Error("failed to connect to database", "error", err)
    os.Exit(1)
}
if err := db.Use(tracing.NewPlugin()); err != nil {
    slog.Error("failed to add otel gorm plugin", "error", err)
}
```

Note: If `gorm.io/plugin/opentelemetry` is not available or has a different import path, fall back to `github.com/go-gorm/opentelemetry`. Run `go mod tidy` to verify.

- [ ] **Step 3: Add `otelgrpc` server handler**

Change the gRPC server creation:

Before:
```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

After:
```go
grpcServer := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
    grpc.UnaryInterceptor(interceptor),
)
```

- [ ] **Step 4: Migrate `log.*` calls to `slog`**

Replace `"log"` import with `"log/slog"` and `"os"`.

| Old | New |
|-----|-----|
| `log.Fatalf("failed to connect to database: %v", err)` | `slog.Error("failed to connect to database", "error", err); os.Exit(1)` |
| `log.Println("connected to PostgreSQL")` | `slog.Info("connected to PostgreSQL")` |
| `log.Fatalf("failed to run migrations: %v", err)` | `slog.Error("failed to run migrations", "error", err); os.Exit(1)` |
| `log.Println("migrations completed")` | `slog.Info("migrations completed")` |
| `log.Fatalf("failed to create kafka publisher: %v", err)` | `slog.Error("failed to create kafka publisher", "error", err); os.Exit(1)` |
| `log.Println("kafka publisher initialized for catalog.books.changed topic")` | `slog.Info("kafka publisher initialized", "topic", "catalog.books.changed")` |
| `log.Println("starting kafka consumer for reservations topic")` | `slog.Info("starting kafka consumer", "topic", "reservations")` |
| `log.Printf("kafka consumer error: %v", err)` | `slog.Error("kafka consumer error", "error", err)` |
| `log.Fatalf("failed to listen: %v", err)` | `slog.Error("failed to listen", "error", err); os.Exit(1)` |
| `log.Printf("catalog service listening on :%s", grpcPort)` | `slog.Info("catalog service listening", "port", grpcPort)` |
| `log.Fatalf("failed to serve: %v", err)` | `slog.Error("failed to serve", "error", err); os.Exit(1)` |

- [ ] **Step 5: Run `go mod tidy` and verify compilation**

Run: `cd /home/fesol/docs/go-journey/services/catalog && go mod tidy && go build ./cmd/`
Expected: Compiles successfully.

- [ ] **Step 6: Run existing catalog tests**

Run: `cd /home/fesol/docs/go-journey/services/catalog && go test ./internal/service/... ./internal/handler/... -v -count=1`
Expected: Existing tests PASS (note: pre-existing CreateBook admin context failures are expected).

- [ ] **Step 7: Commit**

```bash
git add services/catalog/cmd/main.go services/catalog/go.mod services/catalog/go.sum
git commit -m "feat: instrument catalog with otelgrpc server handler, GORM plugin, and slog"
```

---

### Task 8: Catalog Slog Migration & Custom Metric

**Files:**
- Modify: `services/catalog/internal/service/catalog.go`
- Modify: `services/catalog/internal/consumer/consumer.go`

Migrate `log.Printf` to `slog` in the catalog service layer and consumer, and add the `catalog.books.total` custom metric.

- [ ] **Step 1: Migrate catalog service logging**

In `services/catalog/internal/service/catalog.go`:

Change import from `"log"` to `"log/slog"`.

Migrate the 4 fire-and-forget publisher log calls:

| Old | New |
|-----|-----|
| `log.Printf("failed to publish book.created event for book %s: %v", created.ID, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "book.created", "book_id", created.ID, "error", err)` |
| `log.Printf("failed to publish book.updated event for book %s: %v", updated.ID, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "book.updated", "book_id", updated.ID, "error", err)` |
| `log.Printf("failed to publish book.deleted event for book %s: %v", id, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "book.deleted", "book_id", id, "error", err)` |
| `log.Printf("failed to publish book.updated event for book %s: %v", id, err)` (in UpdateAvailability) | `slog.ErrorContext(ctx, "failed to publish event", "event", "book.updated", "book_id", id, "error", err)` |

- [ ] **Step 2: Add `catalog.books.total` custom metric**

Add these imports to `services/catalog/internal/service/catalog.go`:

```go
"go.opentelemetry.io/otel"
)
```

Add a package-level meter and counter:

```go
var bookCounter, _ = otel.Meter("catalog").Int64UpDownCounter("catalog.books.total",
	metric.WithDescription("Total number of books in the catalog"),
)
```

Also add the metric import:
```go
"go.opentelemetry.io/otel/metric"
```

In `CreateBook`, after successful creation (before the publish call):
```go
bookCounter.Add(ctx, 1)
```

In `DeleteBook`, after successful deletion (before the publish call):
```go
bookCounter.Add(ctx, -1)
```

- [ ] **Step 3: Migrate catalog consumer logging**

In `services/catalog/internal/consumer/consumer.go`:

Change import from `"log"` to `"log/slog"`.

| Old | New |
|-----|-----|
| `log.Printf("consumer error: %v", err)` | `slog.Error("consumer error", "error", err)` |
| `log.Printf("failed to handle event: %v", err)` | `slog.ErrorContext(ctx, "failed to handle event", "error", err)` |
| `log.Printf("unknown event type: %s", event.EventType)` | `slog.WarnContext(ctx, "unknown event type", "event_type", event.EventType)` |

Note: The `handleEvent` function already receives `ctx`. The `ConsumeClaim` method has `ctx := session.Context()`. The `Run` function's `for` loop error uses `slog.Error` without context (no active span in the consumer loop itself).

- [ ] **Step 4: Run `go mod tidy` and existing tests**

Run: `cd /home/fesol/docs/go-journey/services/catalog && go mod tidy && go test ./internal/service/... -v -count=1`
Expected: Existing service tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/catalog/internal/service/catalog.go services/catalog/internal/consumer/consumer.go services/catalog/go.mod services/catalog/go.sum
git commit -m "feat: migrate catalog service/consumer to slog, add catalog.books.total metric"
```

---

### Task 9: Catalog Kafka Trace Context Propagation

**Files:**
- Modify: `services/catalog/internal/kafka/publisher.go`
- Modify: `services/catalog/internal/consumer/consumer.go`

Add trace context injection to the Kafka publisher and extraction in the consumer.

- [ ] **Step 1: Update catalog publisher to inject trace context**

In `services/catalog/internal/kafka/publisher.go`:

The `Publish` method currently ignores context (`_ context.Context`). Change it to use context and inject trace context into Kafka message headers.

Add imports:

```go
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/propagation"
```

Add a `TextMapCarrier` adapter for sarama message headers. Since `otelsarama` may have version compatibility issues with our sarama version, we implement a simple carrier directly:

```go
// headerCarrier adapts sarama message headers to propagation.TextMapCarrier.
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

Update the `Publish` method:

Before:
```go
func (p *Publisher) Publish(_ context.Context, event service.BookEvent) error {
```

After:
```go
func (p *Publisher) Publish(ctx context.Context, event service.BookEvent) error {
```

After creating the `msg`, before `SendMessage`, add:

```go
otel.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})
```

- [ ] **Step 2: Update catalog consumer to extract trace context**

In `services/catalog/internal/consumer/consumer.go`:

Add imports:

```go
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/propagation"
```

Add a consumer message carrier:

```go
// consumerHeaderCarrier adapts sarama consumer message headers to propagation.TextMapCarrier.
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

In `ConsumeClaim`, after getting the message from the channel, extract trace context before handling:

Before:
```go
for msg := range claim.Messages() {
    if err := handleEvent(ctx, h.svc, msg.Value); err != nil {
```

After:
```go
for msg := range claim.Messages() {
    msgCtx := otel.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
    if err := handleEvent(msgCtx, h.svc, msg.Value); err != nil {
```

And update `MarkMessage` error logging to use `msgCtx`:
```go
    slog.ErrorContext(msgCtx, "failed to handle event", "error", err)
```

- [ ] **Step 3: Run `go mod tidy` and verify compilation**

Run: `cd /home/fesol/docs/go-journey/services/catalog && go mod tidy && go build ./cmd/`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add services/catalog/internal/kafka/publisher.go services/catalog/internal/consumer/consumer.go services/catalog/go.mod services/catalog/go.sum
git commit -m "feat: add trace context propagation across Kafka in catalog service"
```

---

### Task 10: Reservation Service Instrumentation

**Files:**
- Modify: `services/reservation/cmd/main.go`
- Modify: `services/reservation/internal/kafka/publisher.go`
- Modify: `services/reservation/internal/service/service.go`
- Modify: `services/reservation/go.mod` (via `go mod tidy`)

This is the same pattern as catalog: gRPC server + client tracing, GORM plugin, Kafka trace propagation, and slog migration.

- [ ] **Step 1: Add OTel imports and Init call to reservation main.go**

Add these imports:

```go
"log/slog"

pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
```

At the start of `main()`:

```go
ctx := context.Background()
shutdown, err := pkgotel.Init(ctx, "reservation", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
if err != nil {
    slog.Error("failed to init otel", "error", err)
} else {
    defer shutdown(ctx)
}
```

- [ ] **Step 2: Add GORM plugin after `gorm.Open`**

Add import:
```go
"gorm.io/plugin/opentelemetry/tracing"
```

After `gorm.Open`:
```go
if err := db.Use(tracing.NewPlugin()); err != nil {
    slog.Error("failed to add otel gorm plugin", "error", err)
}
```

- [ ] **Step 3: Add `otelgrpc` to catalog gRPC client connection**

The reservation service connects to catalog as a gRPC client. Add the stats handler:

Before:
```go
catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
```

After:
```go
catalogConn, err := grpc.NewClient(catalogAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

- [ ] **Step 4: Add `otelgrpc` server handler to gRPC server**

Before:
```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

After:
```go
grpcServer := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
    grpc.UnaryInterceptor(interceptor),
)
```

- [ ] **Step 5: Migrate `log.*` calls to `slog` in main.go**

Replace `"log"` with `"log/slog"` and `"os"`.

| Old | New |
|-----|-----|
| `log.Fatalf("failed to connect to database: %v", err)` | `slog.Error("failed to connect to database", "error", err); os.Exit(1)` |
| `log.Println("connected to PostgreSQL")` | `slog.Info("connected to PostgreSQL")` |
| `log.Fatalf("failed to run migrations: %v", err)` | `slog.Error("failed to run migrations", "error", err); os.Exit(1)` |
| `log.Println("migrations completed")` | `slog.Info("migrations completed")` |
| `log.Fatalf("connect to catalog service: %v", err)` | `slog.Error("connect to catalog service", "error", err); os.Exit(1)` |
| `log.Fatalf("create kafka publisher: %v", err)` | `slog.Error("create kafka publisher", "error", err); os.Exit(1)` |
| `log.Fatalf("failed to listen: %v", err)` | `slog.Error("failed to listen", "error", err); os.Exit(1)` |
| `log.Printf("reservation service listening on :%s", grpcPort)` | `slog.Info("reservation service listening", "port", grpcPort)` |
| `log.Fatalf("failed to serve: %v", err)` | `slog.Error("failed to serve", "error", err); os.Exit(1)` |

- [ ] **Step 6: Update reservation Kafka publisher with trace context propagation**

In `services/reservation/internal/kafka/publisher.go`:

This is the same pattern as the catalog publisher. Add the `headerCarrier` type and inject trace context. The current `Publish` ignores context (`_ context.Context`) — change to use `ctx`:

Add imports:
```go
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/propagation"
```

Add the same `headerCarrier` type as in catalog publisher (or if the code is identical enough, consider this a minor duplication — it's 3 types across 2 publishers, acceptable for this scope).

Update `Publish` signature:
```go
func (p *Publisher) Publish(ctx context.Context, event service.ReservationEvent) error {
```

After creating `msg`, before `SendMessage`:
```go
otel.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})
```

- [ ] **Step 7: Migrate reservation service logging**

In `services/reservation/internal/service/service.go`:

Change import from `"log"` to `"log/slog"`.

| Old | New |
|-----|-----|
| `log.Printf("failed to publish created event for reservation %s: %v", created.ID, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.created", "reservation_id", created.ID, "error", err)` |
| `log.Printf("failed to publish returned event for reservation %s: %v", updated.ID, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.returned", "reservation_id", updated.ID, "error", err)` |
| `log.Printf("failed to expire reservation %s: %v", r.ID, err)` | `slog.ErrorContext(ctx, "failed to expire reservation", "reservation_id", r.ID, "error", err)` |
| `log.Printf("failed to publish expired event for reservation %s: %v", r.ID, err)` | `slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.expired", "reservation_id", r.ID, "error", err)` |

- [ ] **Step 8: Run `go mod tidy` and verify compilation**

Run: `cd /home/fesol/docs/go-journey/services/reservation && go mod tidy && go build ./cmd/`
Expected: Compiles successfully.

- [ ] **Step 9: Run existing reservation tests**

Run: `cd /home/fesol/docs/go-journey/services/reservation && go test ./internal/service/... ./internal/handler/... -v -count=1`
Expected: Existing tests PASS.

- [ ] **Step 10: Commit**

```bash
git add services/reservation/cmd/main.go services/reservation/internal/kafka/publisher.go services/reservation/internal/service/service.go services/reservation/go.mod services/reservation/go.sum
git commit -m "feat: instrument reservation with otelgrpc, GORM plugin, Kafka trace propagation, and slog"
```

---

### Task 11: Documentation Stubs & SUMMARY.md

**Files:**
- Create: `docs/src/ch08/index.md`
- Create: `docs/src/ch08/otel-fundamentals.md`
- Create: `docs/src/ch08/instrumentation.md`
- Create: `docs/src/ch08/structured-logging.md`
- Create: `docs/src/ch08/grafana-stack.md`
- Create: `docs/src/ch08/sidecar-pattern.md`
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Create documentation stubs**

Create `docs/src/ch08/index.md`:
```markdown
# Chapter 8: Observability with OpenTelemetry

In this chapter, we add end-to-end observability to the library system using OpenTelemetry and the Grafana stack.

## What You'll Learn

- The three pillars of observability: tracing, metrics, and logging
- Setting up OpenTelemetry in Go microservices
- Instrumenting HTTP, gRPC, Kafka, and PostgreSQL
- Migrating to structured logging with `slog`
- Deploying Grafana, Tempo, Prometheus, and Loki
- Correlating traces with logs for debugging
```

Create `docs/src/ch08/otel-fundamentals.md`:
```markdown
# 8.1 OpenTelemetry Fundamentals

TODO: Cover traces, spans, metrics, context propagation, and collector architecture.
```

Create `docs/src/ch08/instrumentation.md`:
```markdown
# 8.2 Instrumenting Go Services

TODO: Cover SDK setup, HTTP/gRPC/Kafka/DB auto-instrumentation, custom metrics.
```

Create `docs/src/ch08/structured-logging.md`:
```markdown
# 8.3 Structured Logging with slog

TODO: Cover slog migration, custom handler, trace-log correlation.
```

Create `docs/src/ch08/grafana-stack.md`:
```markdown
# 8.4 The Grafana Stack

TODO: Cover backend stack setup, Grafana dashboards, trace-to-log correlation, manual verification walkthrough.
```

Create `docs/src/ch08/sidecar-pattern.md`:
```markdown
# 8.5 Sidecar Collector Pattern

TODO: Cover shared vs. sidecar vs. DaemonSet collector patterns, trade-offs, when to switch.
```

- [ ] **Step 2: Update `docs/src/SUMMARY.md`**

Append after the Chapter 7 entries (after line 39):

```markdown
- [Chapter 8: Observability with OpenTelemetry](./ch08/index.md)
  - [8.1 OpenTelemetry Fundamentals](./ch08/otel-fundamentals.md)
  - [8.2 Instrumenting Go Services](./ch08/instrumentation.md)
  - [8.3 Structured Logging with slog](./ch08/structured-logging.md)
  - [8.4 The Grafana Stack](./ch08/grafana-stack.md)
  - [8.5 Sidecar Collector Pattern](./ch08/sidecar-pattern.md)
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch08/ docs/src/SUMMARY.md
git commit -m "docs: add Chapter 8 documentation structure and stubs"
```

---

### Task 12: Dockerfile, Dockerfile.dev, and Earthfile Updates

**Files:**
- Modify: `services/gateway/Dockerfile` (add `pkg/otel` COPY lines)
- Modify: `services/gateway/Dockerfile.dev` (add `pkg/otel` COPY lines)
- Modify: `services/catalog/Dockerfile` (add `pkg/otel` COPY lines)
- Modify: `services/catalog/Dockerfile.dev` (add `pkg/otel` COPY lines)
- Modify: `services/reservation/Dockerfile` (add `pkg/otel` COPY lines)
- Modify: `services/reservation/Dockerfile.dev` (add `pkg/otel` COPY lines)
- Modify: `services/gateway/Earthfile` (add `pkg/otel` to deps)
- Modify: `services/catalog/Earthfile` (add `pkg/otel` to deps)
- Modify: `services/reservation/Earthfile` (add `pkg/otel` to deps)

Each service's build files need to copy `pkg/otel` as a local module dependency, mirroring the existing `pkg/auth` pattern. Without this, `go mod download` fails during Docker builds because `pkg/otel` is not in the build context.

- [ ] **Step 1: Check existing patterns**

Read `services/catalog/Dockerfile` to see the existing `pkg/auth` pattern:

```dockerfile
# In the deps section (go.mod caching):
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/

# In the source section (full copy):
COPY pkg/auth/ ./pkg/auth/
```

`Dockerfile.dev` has a single full copy:
```dockerfile
COPY pkg/auth/ ./pkg/auth/
```

Earthfile pattern:
```
deps:
    COPY ../../pkg/auth/go.mod ../../pkg/auth/go.sum* ../pkg/auth/
src:
    COPY ../../pkg/auth/ ../pkg/auth/
```

- [ ] **Step 2: Update production Dockerfiles**

For each of `services/gateway/Dockerfile`, `services/catalog/Dockerfile`, `services/reservation/Dockerfile`:

Add after the existing `COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/` line:
```dockerfile
COPY pkg/otel/go.mod pkg/otel/go.sum* ./pkg/otel/
```

Add after the existing `COPY pkg/auth/ ./pkg/auth/` line:
```dockerfile
COPY pkg/otel/ ./pkg/otel/
```

- [ ] **Step 3: Update dev Dockerfiles**

For each of `services/gateway/Dockerfile.dev`, `services/catalog/Dockerfile.dev`, `services/reservation/Dockerfile.dev`:

Add after the existing `COPY pkg/auth/ ./pkg/auth/` line:
```dockerfile
COPY pkg/otel/ ./pkg/otel/
```

- [ ] **Step 4: Update Earthfiles**

For each of `services/gateway/Earthfile`, `services/catalog/Earthfile`, `services/reservation/Earthfile`:

In the `deps:` target, add after the `pkg/auth` line:
```
COPY ../../pkg/otel/go.mod ../../pkg/otel/go.sum* ../pkg/otel/
```

In the `src:` target, add after the `pkg/auth` line:
```
COPY ../../pkg/otel/ ../pkg/otel/
```

- [ ] **Step 5: Commit**

```bash
git add services/gateway/Dockerfile services/gateway/Dockerfile.dev services/gateway/Earthfile \
      services/catalog/Dockerfile services/catalog/Dockerfile.dev services/catalog/Earthfile \
      services/reservation/Dockerfile services/reservation/Dockerfile.dev services/reservation/Earthfile
git commit -m "build: add pkg/otel to Dockerfiles, Dockerfile.devs, and Earthfiles"
```
