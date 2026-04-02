# 9.1 OpenTelemetry Fundamentals

Before we instrument a single line of Go code, we need to understand what we are building toward. Observability is not about installing a library -- it is about designing your system so you can ask arbitrary questions about its behavior in production, without deploying new code. This section covers the concepts and architecture that make that possible.

---

## The Three Pillars of Observability

Observability rests on three signal types, each answering a different class of question:

**Traces** answer "what happened during this request?" A trace follows a single operation across service boundaries -- from the HTTP request entering the gateway, through a gRPC call to the catalog service, into a PostgreSQL query, out through a Kafka publish, and into a consumer on the other side. Each step in the trace is a **span**. Spans form a tree: the root span is the entry point, and child spans represent sub-operations.

**Metrics** answer "how is the system performing right now?" Metrics are aggregated numerical measurements: request count, error rate, latency percentiles, queue depth, memory usage. Unlike traces (which capture individual requests), metrics summarize behavior over time windows. They are cheap to collect, cheap to store, and the foundation of alerting.

**Logs** answer "what did the code actually do?" Logs are timestamped text records emitted by your application. When structured (JSON with consistent fields), they become queryable. When correlated with traces (via a shared `trace_id`), they become powerful: you can click a slow trace in your dashboard and immediately see every log line from every service involved in that request.

Each pillar is useful alone. Together, they create a feedback loop: metrics tell you *something* is wrong (latency spike), traces tell you *where* it is wrong (the catalog-to-PostgreSQL span), and logs tell you *why* (a missing index causing a full table scan).

If you have used Spring Boot Actuator with Micrometer, you have worked with metrics. If you have used Sleuth or the newer Micrometer Tracing, you have worked with traces. If you have used SLF4J with Logback and MDC, you have worked with structured, context-aware logs. OpenTelemetry unifies all three under one standard.

---

## What OpenTelemetry Is

OpenTelemetry (OTel) is a CNCF project that provides a vendor-neutral standard for collecting telemetry data. It is the merger of two earlier projects: OpenTracing (tracing API) and OpenCensus (tracing + metrics). The goal is simple: instrument your code once, export to any backend.

The Java ecosystem went through a similar consolidation. Micrometer provided a vendor-neutral metrics facade (like SLF4J for metrics). OpenTracing provided a tracing API. Spring Cloud Sleuth wired them together. OTel replaces all of these with a single, unified project that covers traces, metrics, and (as of recently) logs.

OTel consists of several components:

| Component | Role |
|-----------|------|
| **API** | Stable interfaces for creating spans, recording metrics, emitting logs. Your application code depends on this. |
| **SDK** | The implementation of the API: span processors, metric readers, exporters. Configured once at startup. |
| **Exporters** | Plugins that send telemetry to backends: OTLP (the standard protocol), Prometheus, Jaeger, Zipkin, etc. |
| **Collector** | A standalone binary that receives, processes, and forwards telemetry. Acts as a proxy between your services and your backends. |
| **Contrib packages** | Auto-instrumentation libraries for popular frameworks: `net/http`, gRPC, database drivers, etc. |

The split between API and SDK is deliberate. Library authors depend on the API (lightweight, stable). Application authors depend on the SDK (heavier, configurable). If no SDK is configured, the API defaults to no-ops -- your library still compiles and runs, it just does not emit telemetry. This is the same pattern as SLF4J (API) + Logback (implementation) in Java.

---

## Key Concepts

### Spans and Traces

A **span** represents a single unit of work: an HTTP request, a gRPC call, a database query, a Kafka publish. Each span has:

- A **name** (e.g., `GET /books`, `catalog.v1.CatalogService/ListBooks`)
- A **trace ID** -- a 128-bit identifier shared by all spans in the same trace
- A **span ID** -- a 64-bit identifier unique to this span
- A **parent span ID** -- linking this span to its parent (absent for root spans)
- A **start time** and **end time**
- **Attributes** -- key-value pairs (e.g., `http.method=GET`, `http.status_code=200`)
- **Events** -- timestamped annotations within the span (e.g., an exception)
- A **status** -- OK, Error, or Unset

A **trace** is the collection of all spans sharing the same trace ID. Visually, a trace looks like a waterfall chart:

```
[gateway] HTTP GET /books/123          ─────────────────────────────── 45ms
  [gateway] gRPC catalog/GetBook       ──────────────────────── 38ms
    [catalog] gRPC catalog/GetBook     ─────────────────── 35ms
      [catalog] db SELECT books        ──────────── 12ms
```

Each indentation level is a child span. The total request took 45ms, and you can see that 12ms of that was the database query. Without tracing, you would know the request took 45ms (from your HTTP access log), but you would not know whether the time was spent in your code, in the database, or in network latency between services.

Spans also carry a **status**. If the catalog service returns a gRPC error, the span's status is set to `Error` with a description. Visualization tools (Grafana Tempo, Jaeger) highlight error spans in red, making it easy to spot failures in a trace waterfall.

### Context Propagation

For traces to work across service boundaries, the trace context (trace ID + parent span ID) must travel with the request. This is called **propagation**.

In HTTP, the trace context is carried in the `traceparent` header, defined by the W3C Trace Context specification[^1]:

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
              ^^                                  ^^                ^^
              version  trace-id (32 hex)          parent-id (16 hex) flags
```

In gRPC, the same header is propagated via metadata. In Kafka, we inject it into message headers. The mechanism changes, but the concept is the same: every hop in the request chain reads the incoming context and creates a child span linked to it.

OTel calls the component that does this a **TextMapPropagator**. Our system uses `propagation.TraceContext{}`, which implements the W3C standard. You register it once at startup, and every instrumented transport (HTTP, gRPC, Kafka) uses it automatically.

In the Java world, Spring Cloud Sleuth (now Micrometer Tracing) handled this transparently via servlet filters and RestTemplate interceptors. OTel in Go works the same way -- the contrib packages (`otelhttp`, `otelgrpc`) inject and extract context without manual intervention. Kafka is the exception: we handle it manually, which we will see in section 9.2.

### Metric Types

OTel defines three fundamental metric instruments:

| Type | Description | Example |
|------|-------------|---------|
| **Counter** | Monotonically increasing value. Only goes up (or resets to zero). | Total HTTP requests served |
| **Gauge** (UpDownCounter) | Value that can increase or decrease. | Current number of books in catalog |
| **Histogram** | Distribution of values. Records individual measurements and buckets them. | Request latency in milliseconds |

Counters and histograms are the most common. The `otelhttp` middleware automatically records `http_server_request_duration_seconds` as a histogram, giving you latency percentiles for free. We add a custom `Int64UpDownCounter` for tracking the total book count in the catalog.

If you have used Micrometer in Spring, these map directly: `Counter` is `Counter`, `Gauge` is `Gauge`, and `DistributionSummary` / `Timer` is roughly `Histogram`. The naming conventions differ (OTel favors `snake_case` with units as suffixes, Micrometer uses `dot.separated`), but the concepts are identical.

### The OTel API/SDK Split

A design decision worth understanding: OTel separates the **API** (interfaces and no-op implementations) from the **SDK** (the real implementation). This is the same split as SLF4J (API) vs. Logback (SDK) in Java.

When you write library code, you depend only on the API package (`go.opentelemetry.io/otel`). You call `otel.Tracer("mylib").Start(ctx, "span")` and it works -- if no SDK is configured, the call returns a no-op span that does nothing. Your library compiles, runs, and imposes zero overhead.

When you write application code (your `main.go`), you also depend on the SDK packages (`go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/exporters/...`). You configure a `TracerProvider` with real exporters and register it globally. From that point on, all API calls (including those from libraries) produce real spans.

This split has a practical consequence for testing. Your unit tests never call `Init()`, so the global providers remain no-ops. OTel-instrumented code runs without side effects -- no spans exported, no network calls, no test dependencies on a running Collector.

### Baggage and Context

Beyond trace context, OTel supports **baggage** -- arbitrary key-value pairs that propagate across service boundaries alongside the trace context. For example, you could set `baggage.Set("user.tier", "premium")` in the gateway, and every downstream service could read it.

We do not use baggage in our system, but it is worth knowing about. It is the OTel equivalent of passing metadata through the entire call chain without adding parameters to every function signature. The W3C Baggage specification[^5] defines the header format (`baggage: user.tier=premium`), and OTel propagates it alongside `traceparent`.

---

## OTel Architecture in Our System

Our system uses the following architecture:

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
   └──┬──────┬──────┘
      │      │
      ▼      ▼
   Tempo  Prometheus    Loki (via Promtail)
      │      │            │
      └──────┴────────────┘
             ▼
          Grafana
```

Each service uses the OTel SDK to create spans and record metrics. The SDK exports this data via OTLP/gRPC to a shared OTel Collector. The Collector is a standalone process that receives telemetry, batches it, and fans it out to backend-specific systems:

- **Traces** go to Tempo (a Grafana Labs trace store)
- **Metrics** are exposed on a Prometheus scrape endpoint, where Prometheus pulls them
- **Logs** take a different path: services write structured JSON to stdout, Docker captures it, and Promtail ships it to Loki

The Collector is the key architectural piece. Services do not need to know which backend stores their traces or metrics. They speak OTLP to the Collector, and the Collector handles the rest. This decoupling is covered in depth in section 9.5.

---

## How OTel Fits in the CNCF Landscape

OpenTelemetry is the second most active CNCF project after Kubernetes[^2]. It has reached "Graduated" status for tracing and metrics, with logging in "Stable" as of late 2024. The ecosystem includes:

- **Jaeger** and **Zipkin** -- trace backends (OTel can export to both)
- **Prometheus** -- the de facto standard for metrics in Kubernetes
- **Grafana Tempo** -- a trace backend designed for high volume and low cost
- **Grafana Loki** -- a log aggregation system inspired by Prometheus's label model
- **Grafana** -- the visualization layer that ties them all together

Our stack uses the Grafana family (Tempo, Loki, Grafana) plus Prometheus. This is a common open-source choice. In production, you might use managed equivalents: AWS X-Ray for traces, CloudWatch for metrics, or a commercial platform like Datadog or Honeycomb. The point of OTel is that switching backends does not require changing your application code -- you change the Collector configuration.

The fact that OTel is vendor-neutral is its defining feature. Before OTel, you chose a vendor (Datadog, New Relic, Honeycomb) and instrumented with their proprietary SDK. Switching vendors meant re-instrumenting your entire codebase. With OTel, your instrumentation is permanent. The export destination is a configuration change.

This is the same value proposition that SLF4J brought to Java logging: write `logger.info("message")` once, switch between Logback, Log4j2, or any other implementation by changing a dependency. OTel does this for all three pillars of observability.

---

## Exercises

1. **Read the W3C Trace Context spec.** Open [w3c.github.io/trace-context](https://www.w3.org/TR/trace-context/) and find the `traceparent` header format. Parse this example manually: `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`. What is the trace ID? The parent span ID? What does the `01` flags field mean?

2. **Compare OTel to Micrometer.** If you have a Spring Boot project, compare how you register a custom counter in Micrometer (`meterRegistry.counter("my.counter").increment()`) versus OTel (`meter.Int64Counter("my.counter")`). What are the differences in lifecycle and registration?

3. **Explore the OTel Go packages.** Run `go doc go.opentelemetry.io/otel` and `go doc go.opentelemetry.io/otel/sdk/trace` in a terminal. Identify the TracerProvider, SpanProcessor, and SpanExporter interfaces. How do they compose?

4. **Think about failure modes.** If the OTel Collector goes down, what happens to your traces and metrics? What happens to your application? (Hint: the SDK uses a BatchSpanProcessor with a bounded queue.)

---

## References

[^1]: [W3C Trace Context Specification](https://www.w3.org/TR/trace-context/) -- The standard for propagating trace context across service boundaries via HTTP headers.
[^2]: [OpenTelemetry CNCF Project Page](https://www.cncf.io/projects/opentelemetry/) -- Project status, governance, and community links.
[^3]: [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/) -- Official getting-started guide and API reference for the Go SDK.
[^4]: [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/) -- The language-agnostic specification that all SDKs implement.
[^5]: [W3C Baggage Specification](https://www.w3.org/TR/baggage/) -- The standard for propagating application-defined key-value pairs alongside trace context.
