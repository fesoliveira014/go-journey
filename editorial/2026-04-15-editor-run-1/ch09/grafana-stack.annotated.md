# 9.4 The Grafana Observability Stack

<!-- [STRUCTURAL] Good recap opener — "You have instrumented your services ... Now you need somewhere to send them." This closes the instrumentation-to-backend loop. Keep. -->
<!-- [LINE EDIT] 48-word opening sentence. Already two sentences; second is a comma-list of components. Consider: "You have instrumented your services with traces, metrics, and structured logs. Now you need somewhere to send them, store them, and visualize them. This section covers the backend stack: the OTel Collector as the central pipeline, Tempo for traces, Prometheus for metrics, Loki for logs, and Grafana as the unified dashboard." — already balanced; OK. -->
<!-- [COPY EDIT] Serial comma throughout ("traces, metrics, and structured logs"; "Tempo for traces, Prometheus for metrics, Loki for logs, and Grafana ..."). Correct (CMOS 6.19). -->
You have instrumented your services with traces, metrics, and structured logs. Now you need somewhere to send them, store them, and visualize them. This section covers the backend stack: the OTel Collector as the central pipeline, Tempo for traces, Prometheus for metrics, Loki for logs, and Grafana as the unified dashboard.

---

## Architecture Recap

<!-- [STRUCTURAL] Revisiting the 9.1 architecture diagram in compressed form is the right move. Keep. -->
```
Services (OTLP/gRPC)  ──►  OTel Collector  ──►  Tempo (traces)
                                            ──►  Prometheus (metrics, via scrape)

Docker stdout (JSON)   ──►  Promtail  ──►  Loki (logs)

                            Grafana  ◄──  Tempo + Prometheus + Loki
```

<!-- [LINE EDIT] "There are two data paths." — good concise opener. -->
There are two data paths. Traces and metrics flow from the services through the OTel Collector to their respective backends. Logs take a separate path: services write structured JSON to stdout, Docker captures it, and Promtail scrapes the Docker socket to ship logs to Loki.

<!-- [STRUCTURAL] The "why two paths" paragraph is honest pedagogy — tells the reader why the architecture is not uniform. -->
<!-- [LINE EDIT] "The Promtail approach is simpler, well-tested, and decouples the log pipeline from the OTel SDK." — good. -->
<!-- [COPY EDIT] "OTel's log support in Go is maturing but not yet the standard path." — date-sensitive claim; may need refresh for 2026 publication. See Pass 3. -->
<!-- [COPY EDIT] Please verify: status of OTel logs in Go at publication date (2026). The `go.opentelemetry.io/otel/log` API status may have evolved; confirm whether "maturing but not yet the standard path" is still accurate. -->
This separation is intentional. OTel's log support in Go is maturing but not yet the standard path. The Promtail approach is simpler, well-tested, and decouples the log pipeline from the OTel SDK. In production, you might consolidate everything through the Collector using the `filelog` receiver, but for a development stack this works well.

---

## The OTel Collector

<!-- [LINE EDIT] "Think of it as a reverse proxy for observability data." — good analogy. Keep. -->
The Collector is a standalone process that receives, processes, and exports telemetry. Think of it as a reverse proxy for observability data. Our configuration is minimal:

```yaml
# deploy/otel-collector-config.yaml

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

### Receivers

<!-- [COPY EDIT] "port 4317" — numerals for technical port references, correct (CMOS 9.2). -->
<!-- [COPY EDIT] "`OTEL_COLLECTOR_ENDPOINT`" — env var in code font, correct. -->
The `otlp` receiver listens on port 4317 for gRPC connections. This is the standard OTLP port. All three services (gateway, catalog, reservation) export to this endpoint via the `OTEL_COLLECTOR_ENDPOINT` environment variable.

### Processors

<!-- [LINE EDIT] "`timeout: 5s` means a batch is sent at least every 5 seconds, even if it has not reached `send_batch_size: 1024` entries." — good concrete explanation. -->
<!-- [COPY EDIT] "5 seconds" — numeral for time with unit, OK in this context (CMOS 9.14 allows numerals for measurements). -->
The `batch` processor buffers incoming telemetry and sends it downstream in batches. `timeout: 5s` means a batch is sent at least every 5 seconds, even if it has not reached `send_batch_size: 1024` entries. This reduces the number of outbound connections and improves throughput.

<!-- [STRUCTURAL] The "in production you would add more processors" paragraph is an effective signpost. Keep. -->
<!-- [LINE EDIT] 41-word sentence. Split: "In production, you would add more processors here. `memory_limiter` prevents the Collector from using too much RAM. `probabilistic_sampler` drops a percentage of traces. `attributes` adds or removes span attributes. `tail_sampling` makes sampling decisions based on trace duration or error status." — gives each processor its own line for scannability. -->
<!-- [COPY EDIT] "`tail_sampling`" — OK. -->
In production, you would add more processors here: `memory_limiter` to prevent the Collector from using too much RAM, `probabilistic_sampler` to drop a percentage of traces, `attributes` to add or remove span attributes, or `tail_sampling` for intelligent sampling decisions based on trace duration or error status.

### Exporters

Two exporters, one per signal type:

<!-- [LINE EDIT] "`/tempo` suffix is just a label" — clear explanation of the otlp/name convention. Good. -->
<!-- [COPY EDIT] "`tls.insecure: true`" — YAML-style attribute in code font; OK. -->
- `otlp/tempo` forwards traces to Tempo via OTLP/gRPC. The `/tempo` suffix is just a label -- it lets you define multiple OTLP exporters with different configurations. `tls.insecure: true` disables TLS for the inter-container connection (acceptable in Docker Compose, not in production).

<!-- [LINE EDIT] 49-word sentence. "Rather than pushing metrics to Prometheus, the Collector converts OTLP metrics to Prometheus format and serves them on an HTTP endpoint. Prometheus then pulls from this endpoint on its regular scrape interval. This push-to-pull conversion is one of the Collector's most useful features." — already 3 sentences; OK. -->
<!-- [COPY EDIT] "push-to-pull" — hyphenated compound; correct as noun phrase. -->
- `prometheus` exposes a Prometheus scrape endpoint on port 8889. Rather than pushing metrics to Prometheus, the Collector converts OTLP metrics to Prometheus format and serves them on an HTTP endpoint. Prometheus then pulls from this endpoint on its regular scrape interval. This push-to-pull conversion is one of the Collector's most useful features.

### Pipelines

<!-- [LINE EDIT] "Each pipeline is independent -- a failure in the trace exporter does not affect metrics." — important failure-mode statement. Keep. -->
The `service.pipelines` block wires everything together. The traces pipeline flows from `otlp` receiver through `batch` processor to `otlp/tempo` exporter. The metrics pipeline flows from `otlp` receiver through `batch` processor to `prometheus` exporter. Each pipeline is independent -- a failure in the trace exporter does not affect metrics.

<!-- [STRUCTURAL] The Spring Cloud Data Flow / Kafka Connect analogy is useful for Java readers. Keep. -->
<!-- [COPY EDIT] "sources, processors, and sinks" — serial comma correct. -->
If you have used Spring Cloud Data Flow or Kafka Connect, the pipeline concept is similar: sources, processors, and sinks composed declaratively. The Collector's pipeline model is simpler (no dynamic routing), but the mental model is the same.

---

## Tempo: Trace Storage

<!-- [LINE EDIT] "Tempo is Grafana's distributed tracing backend." — concise intro. Keep. -->
<!-- [COPY EDIT] "trace ID, service name, duration, and other attributes" — serial comma correct. -->
Tempo is Grafana's distributed tracing backend. It stores traces and makes them searchable by trace ID, service name, duration, and other attributes.

```yaml
# deploy/tempo-config.yaml

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

<!-- [LINE EDIT] "The WAL (Write-Ahead Log) buffers incoming traces before they are flushed to the storage backend, providing durability across restarts." — "flushed" — passive; could be "before being written to the storage backend". Minor. -->
<!-- [COPY EDIT] "Write-Ahead Log" — hyphenated compound adjective serving as noun; CMOS would set as "write-ahead log" (lowercase in running prose unless proper noun). Expanded form is fine as capitalized glossing. OK. -->
The configuration is minimal for development. Tempo receives traces via OTLP/gRPC (from the Collector's `otlp/tempo` exporter) and stores them on local disk. The WAL (Write-Ahead Log) buffers incoming traces before they are flushed to the storage backend, providing durability across restarts.

<!-- [LINE EDIT] 44-word sentence. Split at "This is why Tempo can handle high trace volumes": "In production, you would use object storage (S3, GCS, Azure Blob) as the backend instead of local disk. Tempo is designed for this: it stores traces as compressed blocks in object storage, which is orders of magnitude cheaper than a traditional database. That's why Tempo can handle high trace volumes without breaking the budget." -->
<!-- [COPY EDIT] "orders of magnitude cheaper" — idiomatic; OK. -->
In production, you would use object storage (S3, GCS, Azure Blob) as the backend instead of local disk. Tempo is designed for this -- it stores traces as compressed blocks in object storage, which is orders of magnitude cheaper than a traditional database. This is why Tempo can handle high trace volumes without breaking the budget.

<!-- [COPY EDIT] "`curl http://localhost:3200/api/traces/4bf92f3577b34da6a3ce929d0e0e4736`" — verify endpoint path. Current Tempo API uses `/api/traces/{traceID}` on the query-frontend. Endpoint varies between the distributor (3200 HTTP) and query-frontend (3200 or separate). For the monolithic config shown, the endpoint is correct on port 3200. Please verify against Tempo 2.7.0. -->
Tempo exposes an HTTP API on port 3200 that Grafana uses to query traces. You can also query directly:

```bash
# Fetch a specific trace by ID
curl http://localhost:3200/api/traces/4bf92f3577b34da6a3ce929d0e0e4736
```

---

## Prometheus: Metrics Storage

<!-- [COPY EDIT] "scrapes the OTel Collector's metrics endpoint every 15 seconds" — measurement + unit numeral use, correct (CMOS 9.14). -->
Prometheus scrapes the OTel Collector's metrics endpoint every 15 seconds:

```yaml
# deploy/prometheus.yaml

global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "otel-collector"
    static_configs:
      - targets: ["otel-collector:8889"]
```

This is the simplest possible Prometheus configuration. It has one scrape target: the Collector's Prometheus exporter on port 8889. All metrics from all three services flow through this single endpoint.

The metrics you get automatically from `otelhttp` and `otelgrpc` include:

<!-- [COPY EDIT] Table header uses title case ("Metric", "Type", "Source") — OK. -->
<!-- [COPY EDIT] Please verify — four metric names in this table: `http_server_request_duration_seconds`, `rpc_server_duration_seconds`, `rpc_client_duration_seconds`, `catalog_books_total`. The HTTP metric in current OTel semantic conventions is `http.server.request.duration`; Prometheus translation drops dots to underscores and appends the unit. Confirm this is the Prometheus-side representation. Same check for `rpc_*`. -->
| Metric | Type | Source |
|--------|------|--------|
| `http_server_request_duration_seconds` | Histogram | otelhttp (gateway) |
| `rpc_server_duration_seconds` | Histogram | otelgrpc (catalog, reservation) |
| `rpc_client_duration_seconds` | Histogram | otelgrpc (gateway outbound) |
| `catalog_books_total` | Gauge | Custom UpDownCounter (catalog) |

<!-- [STRUCTURAL] "Gauge | Custom UpDownCounter" — again the Gauge/UpDownCounter conflation from 9.1. In Prometheus terms, an OTel UpDownCounter is indeed represented as a gauge because Prometheus has no "up-down counter" concept. This is technically correct at the Prometheus layer, but readers may be confused given the 9.1 table's ambiguity. Consider a brief footnote: "OTel UpDownCounters are serialized as Prometheus gauges since Prometheus lacks a direct equivalent." -->
<!-- [LINE EDIT] "These follow the OpenTelemetry Semantic Conventions naming, which differs slightly from Prometheus conventions." — good. -->
<!-- [COPY EDIT] "dots become underscores, units become suffixes" — concrete; OK. -->
These follow the OpenTelemetry Semantic Conventions naming, which differs slightly from Prometheus conventions. The Collector's Prometheus exporter handles the translation (e.g., dots become underscores, units become suffixes).

---

## Loki and Promtail: Log Aggregation

<!-- [STRUCTURAL] Strong opener: "Loki stores logs. Promtail collects them." — short, parallel, memorable. Keep. -->
<!-- [LINE EDIT] 58-word compound sentence. Split: "Loki stores logs. Promtail collects them. The relationship is similar to Elasticsearch and Filebeat, but with a key difference: Loki does not index the full text of log lines. Instead, it indexes only labels (key-value pairs like `container_name=catalog`), and performs grep-like searches on the log content at query time. That design makes it dramatically cheaper to run than Elasticsearch." -->
<!-- [COPY EDIT] "grep-like" — hyphenated compound adjective, correct (CMOS 7.81). -->
<!-- [COPY EDIT] "Elasticsearch" — product name capitalization correct (one word, capital E). -->
Loki stores logs. Promtail collects them. The relationship is similar to Elasticsearch and Filebeat, but with a key difference: Loki does not index the full text of log lines. Instead, it indexes only labels (key-value pairs like `container_name=catalog`), and performs grep-like searches on the log content at query time. This makes it dramatically cheaper to run than Elasticsearch.

### Promtail Configuration

<!-- [STRUCTURAL] Please verify: Promtail has been deprecated in favor of Grafana Alloy as of 2024. For a 2026-published book, this matters. The chapter should at minimum mention Alloy as the modern replacement. Consider updating the whole section to use Alloy, or add a sidebar note: "Promtail is transitioning to Grafana Alloy; the configuration concepts port directly." -->
<!-- [COPY EDIT] Please verify: Grafana announced Promtail is in maintenance mode and recommends migrating to Grafana Alloy for new deployments. For a 2026 book, consider using Alloy's Promtail-compatible mode or switching outright. Major editorial decision. -->
```yaml
# deploy/promtail-config.yaml

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
          trace_id:
```

Let us break this down:

<!-- [LINE EDIT] Paragraph structure uses bold lead-in + em dash — consistent. OK. -->
**`docker_sd_configs`** -- Promtail uses Docker service discovery via the Docker socket. It automatically finds all running containers and tails their stdout/stderr logs. No file paths to configure, no log rotation to manage.

**`relabel_configs`** -- Two rules:
<!-- [LINE EDIT] "Extract the container name from Docker metadata and set it as the `container_name` label" — clear. -->
1. Extract the container name from Docker metadata and set it as the `container_name` label
<!-- [LINE EDIT] "`action: keep` with the regex `.*(?:gateway|catalog|reservation).*` -- only collect logs from our application containers." — good concrete explanation. -->
2. `action: keep` with the regex `.*(?:gateway|catalog|reservation).*` -- only collect logs from our application containers. This filters out logs from Postgres, Kafka, and the observability stack itself.

<!-- [COPY EDIT] "Postgres" vs "PostgreSQL" — product name. CMOS/style rule says use the formal name "PostgreSQL" unless referring to the postgres CLI / container name. Here "Postgres" is acceptable as the informal short form in a casual list. Consider normalization across the chapter. -->
**`pipeline_stages`** -- This is where structured logging pays off:
<!-- [LINE EDIT] "The `json` stage parses each log line as JSON" — clear. -->
1. The `json` stage parses each log line as JSON and extracts `level`, `msg`, `trace_id`, and `span_id`
<!-- [LINE EDIT] "The `labels` stage promotes `level` and `trace_id` to Loki labels, making them indexed and fast to query" — good. Worth noting: high-cardinality labels like `trace_id` are controversial in Loki because they explode the index. See Pass 3. -->
<!-- [COPY EDIT] Please verify: Using `trace_id` as a Loki label is a performance anti-pattern in large deployments because trace IDs are high-cardinality (unique per trace). For a dev stack this is fine, but for production it is not recommended. Consider adding a one-sentence caveat: "Note: `trace_id` is high-cardinality; in production you would keep it as a structured field and filter at query time rather than as a label." -->
2. The `labels` stage promotes `level` and `trace_id` to Loki labels, making them indexed and fast to query

<!-- [LINE EDIT] "After this pipeline, you can query Loki with:" — good transition. -->
<!-- [COPY EDIT] "`{container_name="catalog", level="ERROR"}`" — LogQL syntax, OK. -->
After this pipeline, you can query Loki with:
```
{container_name="catalog", level="ERROR"}
```
And Loki will efficiently find all error logs from the catalog service without scanning every log line.

---

## Grafana: Unified Visualization

<!-- [LINE EDIT] "We auto-provision[^6] datasources and a dashboard so the stack is ready to use immediately after `docker compose up`." — good. -->
<!-- [COPY EDIT] "auto-provision" — hyphenated compound, correct. -->
<!-- [COPY EDIT] Footnote `[^6]` — placement on "auto-provision" verb rather than end of sentence. Inconsistent with other footnote placements in chapter. Normalize. -->
Grafana ties everything together. We auto-provision[^6] datasources and a dashboard so the stack is ready to use immediately after `docker compose up`.

### Datasource Provisioning

```yaml
# deploy/grafana/provisioning/datasources/datasources.yaml

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

<!-- [STRUCTURAL] The `tracesToLogsV2` paragraph is the conceptual climax of the chapter — the place where all three pillars finally connect. Treat it as a load-bearing paragraph; keep it tight. -->
<!-- [LINE EDIT] 44-word sentence. Split: "The critical piece is `tracesToLogsV2` on the Tempo datasource. This tells Grafana: 'when viewing a trace in Tempo, offer a link to Loki filtered by `trace_id`.' The `filterByTraceID: true` setting means clicking a trace will automatically query Loki for `{trace_id="<id>"}`, showing every log line from every service that participated in that trace." -->
<!-- [COPY EDIT] Single and double quotes inside the sentence: `"when viewing a trace in Tempo, offer a link to Loki filtered by `trace_id`."` — quoted hypothetical UI message; OK. -->
The critical piece is `tracesToLogsV2` on the Tempo datasource. This tells Grafana: "when viewing a trace in Tempo, offer a link to Loki filtered by `trace_id`." The `filterByTraceID: true` setting means clicking a trace will automatically query Loki for `{trace_id="<id>"}`, showing you every log line from every service that participated in that trace.

<!-- [LINE EDIT] "This is the trace-to-log correlation mentioned throughout this chapter." — good callback. Keep. -->
<!-- [COPY EDIT] "trace-to-log" — compound adjective before noun, correct (CMOS 7.81). -->
This is the trace-to-log correlation mentioned throughout this chapter. It is the reason we inject `trace_id` into log lines via the `TraceLogHandler`, and the reason Promtail promotes `trace_id` to a Loki label.

### Dashboard Provisioning

```yaml
# deploy/grafana/provisioning/dashboards/dashboards.yaml

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

<!-- [LINE EDIT] "This tells Grafana to load dashboard JSON files from `/var/lib/grafana/dashboards`, which is volume-mounted to `deploy/grafana/dashboards/` in the Docker Compose file." — clear. Keep. -->
This tells Grafana to load dashboard JSON files from `/var/lib/grafana/dashboards`, which is volume-mounted to `deploy/grafana/dashboards/` in the Docker Compose file.

### The Library System Dashboard

<!-- [STRUCTURAL] Four-panel dashboard with PromQL/LogQL examples is the right pedagogical scope. Keep. -->
The pre-built dashboard (`deploy/grafana/dashboards/library-system.json`) has four panels:

<!-- [COPY EDIT] "pre-built" — hyphenated compound, correct (CMOS 7.81). -->
<!-- [LINE EDIT] Strong subsection; each panel gets a short PromQL/LogQL example and one explanatory sentence. Economical. -->
**Request Rate by Service** -- A time series panel using:
```promql
rate(http_server_request_duration_seconds_count[5m])
```
<!-- [COPY EDIT] "`rate()` over the `_count` suffix of a histogram gives you the request rate." — technical precision OK. The `_count` suffix is from Prometheus conventions, not OTel directly. -->
This shows how many HTTP requests per second the gateway is handling, broken down by route and method. `rate()` over the `_count` suffix of a histogram gives you the request rate.

**Request Latency p95** -- A time series panel using:
```promql
histogram_quantile(0.95, rate(http_server_request_duration_seconds_bucket[5m]))
```
<!-- [LINE EDIT] "If p95 is 200ms, it means 95% of requests completed in under 200ms." — good concrete example. -->
<!-- [COPY EDIT] "200ms" — should be "200 ms" per SI (space between numeral and unit, CMOS 9.17). Global fix recommended. -->
<!-- [COPY EDIT] "SLI (Service Level Indicator)" — acronym gloss; OK. "SLO" would be the next logical concept; keep as-is. -->
This computes the 95th percentile latency from the histogram buckets. If p95 is 200ms, it means 95% of requests completed in under 200ms. This is the standard SLI (Service Level Indicator) for latency.

**gRPC Server Latency by Method** -- A time series panel using:
```promql
rate(rpc_server_duration_seconds_sum[5m]) / rate(rpc_server_duration_seconds_count[5m])
```
<!-- [LINE EDIT] "This is the average gRPC latency per method." — clear. -->
<!-- [COPY EDIT] "`rate(..._sum)/rate(..._count)`" is the classic "average of a histogram" pattern. Worth noting this is not a percentile — may confuse readers who saw the p95 example just above. -->
This is the average gRPC latency per method. You can see which RPC methods are slow at a glance.

**Recent Logs** -- A logs panel querying Loki:
```logql
{container_name=~".*catalog.*|.*gateway.*|.*reservation.*"}
```
<!-- [LINE EDIT] "live tail of all application logs" — good. Keep. -->
This shows a live tail of all application logs, with level coloring and JSON field extraction.

---

## Trace-to-Log Correlation Walkthrough

<!-- [STRUCTURAL] Numbered end-to-end walkthrough is a great summative section. This ties every concept in the chapter together. Keep. -->
Here is how the pieces connect end-to-end:

<!-- [COPY EDIT] "steps 2-3" — en dash for ranges (CMOS 6.78): "steps 2–3". -->
1. A user creates a book via the gateway. `otelhttp` creates a root span with trace ID `abc123...`.
2. The gateway calls `catalog.CreateBook` via gRPC. `otelgrpc` propagates the trace context.
3. The catalog service processes the request. GORM creates a DB span. The Kafka publisher injects the trace context into the message headers.
<!-- [COPY EDIT] "steps 2-3" uses hyphen; should be en dash "2–3" (CMOS 6.78). -->
4. Every `slog.ErrorContext(ctx, ...)` call in steps 2-3 includes `trace_id: abc123...` in the JSON output.
5. Promtail scrapes the container logs, parses the JSON, and sends them to Loki with `trace_id` as a label.
6. The OTel SDK sends spans to the Collector, which forwards them to Tempo.
7. In Grafana, you open the Tempo datasource, search for traces from the "catalog" service, and find the one for the CreateBook call.
8. Click the trace. Grafana shows the span waterfall: HTTP → gRPC → DB → Kafka publish.
9. Click "Logs for this span" (the link configured by `tracesToLogsV2`). Grafana queries Loki for `{trace_id="abc123..."}` and shows you every log line from the gateway and catalog services for this exact request.

<!-- [LINE EDIT] "This workflow -- from metric alert, to trace, to logs -- is the observability feedback loop." — strong summative statement. Keep. -->
<!-- [COPY EDIT] Em dashes around "-- from metric alert, to trace, to logs --" — should be em dashes without spaces (CMOS 6.85). Normalization needed file-wide. -->
This workflow -- from metric alert, to trace, to logs -- is the observability feedback loop. It is the reason all three pillars matter and the reason they must be correlated.

---

## Docker Compose: The Complete Stack

<!-- [STRUCTURAL] Whole-stack YAML is valuable as a single reference. Keep. Consider adding a one-line comment in the YAML explaining that this snippet elides the application services. -->
The observability services in `deploy/docker-compose.yml`:

```yaml
otel-collector:
  image: otel/opentelemetry-collector-contrib:0.149.0
  volumes:
    - ./otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml
  ports:
    - "4317:4317"
    - "8889:8889"

tempo:
  image: grafana/tempo:2.7.0
  volumes:
    - ./tempo-config.yaml:/etc/tempo/config.yaml
    - tempo-data:/var/tempo
  command: ["-config.file=/etc/tempo/config.yaml"]
  ports:
    - "3200:3200"

prometheus:
  image: prom/prometheus:v3.1.0
  volumes:
    - ./prometheus.yaml:/etc/prometheus/prometheus.yml
  ports:
    - "9090:9090"

loki:
  image: grafana/loki:3.3.0
  ports:
    - "3100:3100"

promtail:
  image: grafana/promtail:3.3.0
  volumes:
    - ./promtail-config.yaml:/etc/promtail/config.yml
    - /var/run/docker.sock:/var/run/docker.sock:ro
  command: ["-config.file=/etc/promtail/config.yml"]
  depends_on:
    - loki

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
```

<!-- [COPY EDIT] Please verify all image tags at publication date:
  - `otel/opentelemetry-collector-contrib:0.149.0` — verify tag exists.
  - `grafana/tempo:2.7.0` — verify.
  - `prom/prometheus:v3.1.0` — verify. (Prometheus 3.x is real as of late 2024.)
  - `grafana/loki:3.3.0` — verify.
  - `grafana/promtail:3.3.0` — verify; also flag Promtail deprecation.
  - `grafana/grafana:11.5.0` — verify.
-->
Notable details:

<!-- [LINE EDIT] Bulleted observations are a good wrap-up. Keep. -->
<!-- [COPY EDIT] "read-only" — hyphenated compound, correct (CMOS 7.81). -->
- Promtail mounts `/var/run/docker.sock:ro` (read-only) to discover containers via the Docker API. This is a common pattern for Docker log collection but requires the socket to be accessible.
<!-- [LINE EDIT] "Grafana enables anonymous access (`GF_AUTH_ANONYMOUS_ENABLED`) for development convenience." — good. -->
- Grafana enables anonymous access (`GF_AUTH_ANONYMOUS_ENABLED`) for development convenience. In production, you would disable this and use proper authentication.
<!-- [LINE EDIT] "`tempo-data` is a named volume that persists trace data across container restarts." — good. -->
- `tempo-data` is a named volume that persists trace data across container restarts.
<!-- [LINE EDIT] "Note that `depends_on` only waits for container start, not readiness -- Grafana handles reconnection internally." — important practical detail. Keep. -->
- The `depends_on` declarations ensure Grafana starts after its datasource backends. Note that `depends_on` only waits for container start, not readiness -- Grafana handles reconnection internally.

---

## Manual Verification

<!-- [STRUCTURAL] "Manual Verification" numbered list is effectively a checklist exercise. Consider retitling "Smoke Test" or "Verify the Stack" — "Manual Verification" sounds bureaucratic. -->
After `docker compose up`, verify each component:

<!-- [LINE EDIT] Step 1: clear. -->
1. **Grafana** -- Open `http://localhost:3000`. Log in as `admin/admin`. The "Library System Overview" dashboard should be available under Dashboards.

<!-- [COPY EDIT] Please verify: the dashboard is referred to as "Library System Overview" here but "Library System" as the provider name in the YAML. Consistency check — decide on one name. -->
2. **Create a book** -- Use the gateway UI or `curl`. This triggers HTTP spans (gateway), gRPC spans (catalog), DB spans (GORM), and a Kafka publish span.

<!-- [COPY EDIT] "Explore > Tempo" — Grafana uses "Explore" menu label with ">" as hierarchy indicator; consistent convention. OK. -->
3. **Find the trace** -- In Grafana, go to Explore > Tempo. Search by service name "catalog" or "gateway". Click a trace to see the span waterfall.

<!-- [LINE EDIT] "Query `{container_name=~".*catalog.*"}`." — good. -->
4. **Check logs** -- In Grafana, go to Explore > Loki. Query `{container_name=~".*catalog.*"}`. Verify that log lines include `trace_id` fields.

5. **Trace-to-log** -- Click a trace in Tempo. Click the "Logs" icon on a span. Verify that Loki shows log lines filtered by that trace ID.

6. **Check metrics** -- In Grafana, go to Explore > Prometheus. Query `http_server_request_duration_seconds_count`. Verify that data points are appearing.

7. **Check the dashboard** -- Open the "Library System Overview" dashboard. Make several requests and watch the request rate and latency panels update.

---

## Exercises

<!-- [STRUCTURAL] Five exercises progress nicely from observability hygiene (alerts, dashboards) to investigation (query by trace ID) to operational understanding (Collector metrics, overhead). Good. -->
<!-- [COPY EDIT] "status codes >= 400" — mathematical operator; keep inline. Consider "status codes ≥ 400" for polish (Unicode ≥). Minor. -->
1. **Add a PromQL alert rule.** Write a Prometheus alert rule that fires when the HTTP error rate (status codes >= 400) exceeds 5% of total requests over a 5-minute window. Add it to `deploy/prometheus.yaml` under `rule_files`.

<!-- [LINE EDIT] Exercise 2: "Hint: edit the JSON file or use Grafana's UI and export the updated JSON." — good tutor nudge. -->
2. **Create a custom dashboard panel.** Add a panel to the library-system dashboard that shows the `catalog_books_total` metric as a single stat (big number). Hint: edit the JSON file or use Grafana's UI and export the updated JSON.

<!-- [LINE EDIT] Exercise 3: directly exercises the trace-to-log workflow. Keep. -->
3. **Query Loki by trace ID.** Find a trace ID from Tempo, then manually query Loki with `{trace_id="<your-trace-id>"}`. Verify you see log lines from multiple services (gateway and catalog) for the same request.

<!-- [COPY EDIT] "port 8888 (the Collector's self-telemetry port)" — verify. Collector's internal Prometheus-format telemetry is indeed exposed on 8888 by default (different from the 8889 Prometheus exporter). -->
4. **Explore the Collector metrics.** The OTel Collector itself exposes internal metrics. Add a second Prometheus scrape target pointing at `otel-collector:8888` (the Collector's self-telemetry port). What metrics does the Collector report about its own pipeline health?

<!-- [LINE EDIT] Exercise 5: challenging but valuable. Keep. -->
5. **Stress test observability overhead.** Write a script that sends 100 rapid requests to the gateway. Compare the response latency with and without OTel enabled (stop the Collector and restart the gateway without `OTEL_COLLECTOR_ENDPOINT`). How much overhead does the instrumentation add?

---

## References

<!-- [COPY EDIT] Please verify all URLs resolve at publication. -->
<!-- [COPY EDIT] "[^5]: [Promtail Configuration]" — if Promtail is deprecated in favor of Alloy, either add a note or update the reference. -->
[^1]: [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/) -- Architecture, configuration, and deployment guide for the OTel Collector.
[^2]: [Grafana Tempo Documentation](https://grafana.com/docs/tempo/latest/) -- Tempo configuration, query API, and storage backends.
[^3]: [Prometheus Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) -- Scrape configuration, recording rules, and alerting rules.
[^4]: [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/) -- LogQL query language, label design, and deployment modes.
[^5]: [Promtail Configuration](https://grafana.com/docs/loki/latest/send-data/promtail/configuration/) -- Pipeline stages, Docker service discovery, and relabeling.
[^6]: [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/) -- Auto-configuring datasources and dashboards via YAML files.
