# 9.1 OpenTelemetry Fundamentals

<!-- [STRUCTURAL] Strong opener: states the "why" before any code. The framing ("ask arbitrary questions about behavior in production without deploying new code") is Charity Majors's canonical definition and well chosen. Consider attributing (footnote) to honeycomb.io / Majors for honesty. -->
<!-- [LINE EDIT] "is not about installing a library -- it is about designing your system" → "is not about installing a library; it is about designing your system" — em dash (6.85) or semicolon both work, but the source uses two hyphens throughout. See Copy Edit pass for normalization. -->
<!-- [COPY EDIT] All `--` double-hyphen sequences in this file should be em dashes with no spaces (CMOS 6.85): "—". Flagging globally: this is a recurring, file-wide pattern. -->
Before we instrument a single line of Go code, we need to understand what we are building toward. Observability is not about installing a library -- it is about designing your system so you can ask arbitrary questions about its behavior in production, without deploying new code. This section covers the concepts and architecture that make that possible.

---

## The Three Pillars of Observability

<!-- [STRUCTURAL] Excellent framing — the "each answers a different question" cue works well. The three-pillars framing itself is occasionally controversial (Honeycomb argues for events-first), but for a learner this is the right scaffold. -->
Observability rests on three signal types, each answering a different class of question:

<!-- [LINE EDIT] 56-word sentence. Consider splitting at "Each step in the trace is a span.": break into two sentences. -->
<!-- [COPY EDIT] Bold "Traces" as lead-in — consistent with sibling paragraphs. OK. -->
<!-- [COPY EDIT] "sub-operations" → "suboperations" per CMOS 7.89 (the "sub" prefix is usually closed). Verify against project's usage elsewhere; consistency matters more than the rule. -->
**Traces** answer "what happened during this request?" A trace follows a single operation across service boundaries -- from the HTTP request entering the gateway, through a gRPC call to the catalog service, into a PostgreSQL query, out through a Kafka publish, and into a consumer on the other side. Each step in the trace is a **span**. Spans form a tree: the root span is the entry point, and child spans represent sub-operations.

<!-- [LINE EDIT] "Metrics are aggregated numerical measurements: request count, error rate, latency percentiles, queue depth, memory usage" — fine, but "numerical" is redundant with "measurements". Consider "aggregated measurements". -->
<!-- [COPY EDIT] Parenthetical "(which capture individual requests)" — comma style is fine; parentheses appropriate since it is a side gloss (CMOS 6.97). OK. -->
**Metrics** answer "how is the system performing right now?" Metrics are aggregated numerical measurements: request count, error rate, latency percentiles, queue depth, memory usage. Unlike traces (which capture individual requests), metrics summarize behavior over time windows. They are cheap to collect, cheap to store, and the foundation of alerting.

<!-- [LINE EDIT] "timestamped text records emitted by your application" → "timestamped records emitted by your application" — modern structured logs may not be "text" in any meaningful sense. Loosening the noun aligns better with the next sentence. -->
<!-- [COPY EDIT] "JSON with consistent fields" — OK as appositive; no comma needed before the parenthetical. -->
**Logs** answer "what did the code actually do?" Logs are timestamped text records emitted by your application. When structured (JSON with consistent fields), they become queryable. When correlated with traces (via a shared `trace_id`), they become powerful: you can click a slow trace in your dashboard and immediately see every log line from every service involved in that request.

<!-- [STRUCTURAL] Nice synthesis paragraph — "metrics → traces → logs" as a feedback loop. This should arguably live at the END of the three-pillars section rather than mid-flow. It currently works but could be promoted as the takeaway. -->
<!-- [LINE EDIT] "(latency spike)", "(the catalog-to-PostgreSQL span)", "(a missing index causing a full table scan)" — three parentheticals in one sentence; italics on the key words do the work. Consider: "metrics tell you something is wrong — a latency spike — traces tell you where — the catalog-to-PostgreSQL span — and logs tell you why: a missing index causing a full table scan." but that's subjective. -->
<!-- [COPY EDIT] "catalog-to-PostgreSQL" — hyphenated compound adjective before noun "span" (CMOS 7.81). OK. -->
Each pillar is useful alone. Together, they create a feedback loop: metrics tell you *something* is wrong (latency spike), traces tell you *where* it is wrong (the catalog-to-PostgreSQL span), and logs tell you *why* (a missing index causing a full table scan).

<!-- [STRUCTURAL] The Java-analogy paragraph lands well given the tutor-to-experienced-engineer brief. Keep. -->
<!-- [COPY EDIT] "SLF4J with Logback and MDC" — product names correct; MDC (Mapped Diagnostic Context) might deserve a parenthetical gloss on first use for readers who used Logback but not MDC by name. -->
<!-- [COPY EDIT] "Micrometer Tracing" — OK. "Sleuth" — project name; capitalized correctly. -->
If you have used Spring Boot Actuator with Micrometer, you have worked with metrics. If you have used Sleuth or the newer Micrometer Tracing, you have worked with traces. If you have used SLF4J with Logback and MDC, you have worked with structured, context-aware logs. OpenTelemetry unifies all three under one standard.

---

## What OpenTelemetry Is

<!-- [STRUCTURAL] Good heading but "What X Is" is a bit juvenile-sounding. Consider "OpenTelemetry in One Paragraph" or keep. Minor. -->
<!-- [COPY EDIT] "CNCF project" — acronym fine on first use in chapter; arguably expand to "Cloud Native Computing Foundation (CNCF)" on first mention. Check ch08 for prior expansion. -->
<!-- [COPY EDIT] Please verify: "OpenTracing (tracing API) and OpenCensus (tracing + metrics)" — the OpenCensus project covered both tracing and metrics; OpenTracing was tracing-only. The gloss is correct. -->
<!-- [LINE EDIT] "instrument your code once, export to any backend" — this is the classic OTel tagline. Good. Consider italicizing or setting as a pull-quote for visual punch. -->
OpenTelemetry (OTel) is a CNCF project that provides a vendor-neutral standard for collecting telemetry data. It is the merger of two earlier projects: OpenTracing (tracing API) and OpenCensus (tracing + metrics). The goal is simple: instrument your code once, export to any backend.

<!-- [LINE EDIT] "(as of recently) logs" — "recently" is imprecise and will age badly. Recommend: "and, as of the 1.x SDK series, logs". Or footnote with a date/version anchor. -->
<!-- [COPY EDIT] Please verify: OTel logs signal — Go SDK log API became stable in late 2024 (`go.opentelemetry.io/otel/log` v0.x → stable). Confirm current status as of publication. -->
The Java ecosystem went through a similar consolidation. Micrometer provided a vendor-neutral metrics facade (like SLF4J for metrics). OpenTracing provided a tracing API. Spring Cloud Sleuth wired them together. OTel replaces all of these with a single, unified project that covers traces, metrics, and (as of recently) logs.

OTel consists of several components:

<!-- [COPY EDIT] Table column header "Role" — short and fine. Consistency with later tables in this file ("Type/Description/Example") is acceptable since each table answers a different question. -->
<!-- [COPY EDIT] "span processors, metric readers, exporters" — serial comma present (CMOS 6.19). OK. -->
<!-- [COPY EDIT] "OTLP (the standard protocol), Prometheus, Jaeger, Zipkin, etc." — "etc." and "e.g." should be followed by a comma only mid-sentence; at list end it is fine (CMOS 6.43). OK. -->
| Component | Role |
|-----------|------|
| **API** | Stable interfaces for creating spans, recording metrics, emitting logs. Your application code depends on this. |
| **SDK** | The implementation of the API: span processors, metric readers, exporters. Configured once at startup. |
| **Exporters** | Plugins that send telemetry to backends: OTLP (the standard protocol), Prometheus, Jaeger, Zipkin, etc. |
| **Collector** | A standalone binary that receives, processes, and forwards telemetry. Acts as a proxy between your services and your backends. |
| **Contrib packages** | Auto-instrumentation libraries for popular frameworks: `net/http`, gRPC, database drivers, etc. |

<!-- [STRUCTURAL] Strong closing analogy. The SLF4J/Logback mapping is the most useful possible mental model for a Java engineer. -->
<!-- [LINE EDIT] "(lightweight, stable)" and "(heavier, configurable)" — minor punctuation asymmetry (both are descriptive lists); acceptable. -->
The split between API and SDK is deliberate. Library authors depend on the API (lightweight, stable). Application authors depend on the SDK (heavier, configurable). If no SDK is configured, the API defaults to no-ops -- your library still compiles and runs, it just does not emit telemetry. This is the same pattern as SLF4J (API) + Logback (implementation) in Java.

---

## Key Concepts

### Spans and Traces

A **span** represents a single unit of work: an HTTP request, a gRPC call, a database query, a Kafka publish. Each span has:

<!-- [COPY EDIT] "e.g., `GET /books`" — "e.g." followed by comma (CMOS 6.43). OK. -->
<!-- [COPY EDIT] "128-bit identifier", "64-bit identifier" — hyphenation correct for compound adjective before noun (CMOS 7.81). OK. -->
<!-- [COPY EDIT] List item terminal punctuation: bullet items end without periods; consistent throughout the list. OK. -->
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

<!-- [LINE EDIT] "The total request took 45ms, and you can see that 12ms of that was the database query." → "The request took 45 ms total — 12 ms of which was the database query." — tighter; also applies the "numeral + space + unit" convention (CMOS 10.49; SI). -->
<!-- [COPY EDIT] "45ms", "12ms", "38ms", "35ms" — SI guidance recommends a space between numeral and unit (9.17). Book code output aside, the prose should use "45 ms" etc. Recommend global change. -->
Each indentation level is a child span. The total request took 45ms, and you can see that 12ms of that was the database query. Without tracing, you would know the request took 45ms (from your HTTP access log), but you would not know whether the time was spent in your code, in the database, or in network latency between services.

<!-- [LINE EDIT] Minor repetition: "Spans also carry a status." repeats what the bullet list already stated. Consider re-framing as "Span status matters at visualization time: ..." -->
<!-- [COPY EDIT] "(Grafana Tempo, Jaeger)" — parenthetical list of examples; fine. -->
Spans also carry a **status**. If the catalog service returns a gRPC error, the span's status is set to `Error` with a description. Visualization tools (Grafana Tempo, Jaeger) highlight error spans in red, making it easy to spot failures in a trace waterfall.

### Context Propagation

<!-- [STRUCTURAL] Good bridge. Context propagation is the single most important concept for a learner and deserves this standalone subsection. -->
For traces to work across service boundaries, the trace context (trace ID + parent span ID) must travel with the request. This is called **propagation**.

<!-- [COPY EDIT] "W3C Trace Context specification[^1]" — footnote reference inline; OK. -->
In HTTP, the trace context is carried in the `traceparent` header, defined by the W3C Trace Context specification[^1]:

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
              ^^                                  ^^                ^^
              version  trace-id (32 hex)          parent-id (16 hex) flags
```

<!-- [LINE EDIT] "The mechanism changes, but the concept is the same: every hop in the request chain reads the incoming context and creates a child span linked to it." — good. Active voice throughout. -->
In gRPC, the same header is propagated via metadata. In Kafka, we inject it into message headers. The mechanism changes, but the concept is the same: every hop in the request chain reads the incoming context and creates a child span linked to it.

<!-- [COPY EDIT] "TextMapPropagator" — correct Go OTel API name; confirms against `go.opentelemetry.io/otel/propagation.TextMapPropagator`. -->
<!-- [COPY EDIT] Please verify: `propagation.TraceContext{}` — this is the correct Go type in `go.opentelemetry.io/otel/propagation`. -->
OTel calls the component that does this a **TextMapPropagator**. Our system uses `propagation.TraceContext{}`, which implements the W3C standard. You register it once at startup, and every instrumented transport (HTTP, gRPC, Kafka) uses it automatically.

<!-- [LINE EDIT] 42-word sentence. Consider breaking at "OTel in Go works the same way": "In the Java world, Spring Cloud Sleuth (now Micrometer Tracing) handled this transparently via servlet filters and RestTemplate interceptors. OTel in Go is similar: the contrib packages (otelhttp, otelgrpc) inject and extract context without manual intervention." -->
<!-- [COPY EDIT] "Sleuth (now Micrometer Tracing)" — good; parenthetical update helps Java readers. -->
In the Java world, Spring Cloud Sleuth (now Micrometer Tracing) handled this transparently via servlet filters and RestTemplate interceptors. OTel in Go works the same way -- the contrib packages (`otelhttp`, `otelgrpc`) inject and extract context without manual intervention. Kafka is the exception: we handle it manually, which we will see in section 9.2.

### Metric Types

<!-- [COPY EDIT] "UpDownCounter" — one word per OTel convention; correct. -->
<!-- [COPY EDIT] Table consistency: all types use title-case labels; OK. -->
OTel defines three fundamental metric instruments:

| Type | Description | Example |
|------|-------------|---------|
| **Counter** | Monotonically increasing value. Only goes up (or resets to zero). | Total HTTP requests served |
| **Gauge** (UpDownCounter) | Value that can increase or decrease. | Current number of books in catalog |
| **Histogram** | Distribution of values. Records individual measurements and buckets them. | Request latency in milliseconds |

<!-- [STRUCTURAL] Calling "Gauge (UpDownCounter)" in the table conflates two distinct OTel instruments. OTel has BOTH a Gauge (synchronous and asynchronous) and an UpDownCounter; they are not aliases. A Gauge is the value you observe; an UpDownCounter is the delta you add. This is a technical precision issue. Recommend splitting into two rows or using an asynchronous/synchronous gloss. See copy edit below. -->
<!-- [COPY EDIT] Technical precision: "Gauge" and "UpDownCounter" are separate OTel instruments (the current text even says so in the later paragraph). Please verify and correct the table row: consider replacing with "UpDownCounter — Value that can increase or decrease (synchronous)" and adding a separate row for Gauge as an observable instrument. -->
<!-- [COPY EDIT] Please verify: automatic histogram metric name. The otelhttp instrumentation emits `http.server.request.duration` in SI units (seconds) per current semantic conventions — not `http_server_request_duration_seconds`. Check against current `otelhttp` version; the underscore form is what Prometheus sees after translation. Clarify which layer we are naming. -->
Counters and histograms are the most common. The `otelhttp` middleware automatically records `http_server_request_duration_seconds` as a histogram, giving you latency percentiles for free. We add a custom `Int64UpDownCounter` for tracking the total book count in the catalog.

<!-- [COPY EDIT] "snake_case" and "dot.separated" — code font and lowercase, correctly used as terms of art. -->
<!-- [LINE EDIT] "the concepts are identical" → "the semantics are identical" — more precise. -->
If you have used Micrometer in Spring, these map directly: `Counter` is `Counter`, `Gauge` is `Gauge`, and `DistributionSummary` / `Timer` is roughly `Histogram`. The naming conventions differ (OTel favors `snake_case` with units as suffixes, Micrometer uses `dot.separated`), but the concepts are identical.

### The OTel API/SDK Split

<!-- [STRUCTURAL] This subsection partially duplicates the table + paragraph earlier ("What OpenTelemetry Is"). Consider consolidating: either reduce the table row on API/SDK to one line and expand here, OR shorten here to avoid repeating the SLF4J analogy. Currently the analogy is made twice. -->
<!-- [LINE EDIT] "A design decision worth understanding:" — phatic opener; cut or tighten to "The OTel API/SDK split is worth understanding." -->
A design decision worth understanding: OTel separates the **API** (interfaces and no-op implementations) from the **SDK** (the real implementation). This is the same split as SLF4J (API) vs. Logback (SDK) in Java.

<!-- [COPY EDIT] "vs." — acceptable in informal/technical prose; CMOS prefers "versus" in formal running text (5.250) but "vs." is common in tech writing. Keep for voice. -->
<!-- [COPY EDIT] "no-op" — hyphenated compound noun; correct. -->
When you write library code, you depend only on the API package (`go.opentelemetry.io/otel`). You call `otel.Tracer("mylib").Start(ctx, "span")` and it works -- if no SDK is configured, the call returns a no-op span that does nothing. Your library compiles, runs, and imposes zero overhead.

<!-- [COPY EDIT] Please verify: package paths. `go.opentelemetry.io/otel/sdk/trace` exists; `go.opentelemetry.io/otel/exporters/...` is a prefix for many sub-packages. The ellipsis is fine as shorthand. -->
When you write application code (your `main.go`), you also depend on the SDK packages (`go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/exporters/...`). You configure a `TracerProvider` with real exporters and register it globally. From that point on, all API calls (including those from libraries) produce real spans.

<!-- [LINE EDIT] "remain no-ops" — correct. Good detail about test isolation. -->
This split has a practical consequence for testing. Your unit tests never call `Init()`, so the global providers remain no-ops. OTel-instrumented code runs without side effects -- no spans exported, no network calls, no test dependencies on a running Collector.

### Baggage and Context

<!-- [STRUCTURAL] Well-placed short note. The "we don't use this, but know about it" framing is honest and tutor-appropriate. Keep. -->
<!-- [LINE EDIT] "arbitrary key-value pairs" — OK; "arbitrary" does work here but "application-defined" (as in the W3C footnote) is more precise. -->
Beyond trace context, OTel supports **baggage** -- arbitrary key-value pairs that propagate across service boundaries alongside the trace context. For example, you could set `baggage.Set("user.tier", "premium")` in the gateway, and every downstream service could read it.

<!-- [COPY EDIT] "The W3C Baggage specification[^5]" — footnote placement after the term, not after the sentence, is inconsistent with footnote [^1] style earlier. Normalize: all footnotes go at the end of the sentence they modify. -->
<!-- [COPY EDIT] "(`baggage: user.tier=premium`)" — code font correct. -->
We do not use baggage in our system, but it is worth knowing about. It is the OTel equivalent of passing metadata through the entire call chain without adding parameters to every function signature. The W3C Baggage specification[^5] defines the header format (`baggage: user.tier=premium`), and OTel propagates it alongside `traceparent`.

---

## OTel Architecture in Our System

<!-- [STRUCTURAL] Big diagram is the anchor for the chapter. Consider promoting this to section top OR linking back to it from every subsection that references "the Collector". -->
Our system uses the following architecture:

<!-- [COPY EDIT] ASCII diagram — box-drawing characters used correctly. Verify alignment in rendered Markdown; some Markdown renderers may mis-space due to Unicode width on the pipe characters. OK for plain text viewing. -->
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

<!-- [LINE EDIT] "The Collector is a standalone process that receives telemetry, batches it, and fans it out to backend-specific systems" — good active voice. Keep. -->
Each service uses the OTel SDK to create spans and record metrics. The SDK exports this data via OTLP/gRPC to a shared OTel Collector. The Collector is a standalone process that receives telemetry, batches it, and fans it out to backend-specific systems:

<!-- [COPY EDIT] "Grafana Labs trace store" — consider "Grafana Labs' open-source trace backend" for precision. -->
- **Traces** go to Tempo (a Grafana Labs trace store)
- **Metrics** are exposed on a Prometheus scrape endpoint, where Prometheus pulls them
<!-- [COPY EDIT] "Docker captures it, and Promtail ships it to Loki" — "ships" is informal but industry-standard. OK for voice. -->
- **Logs** take a different path: services write structured JSON to stdout, Docker captures it, and Promtail ships it to Loki

<!-- [LINE EDIT] "This decoupling is covered in depth in section 9.5." — good cross-ref. -->
The Collector is the key architectural piece. Services do not need to know which backend stores their traces or metrics. They speak OTLP to the Collector, and the Collector handles the rest. This decoupling is covered in depth in section 9.5.

---

## How OTel Fits in the CNCF Landscape

<!-- [STRUCTURAL] This section is context-setting and appropriate for a fundamentals chapter. But it currently comes AFTER the architecture diagram — a learner would benefit from ecosystem context BEFORE the specific architecture. Consider moving this section above "OTel Architecture in Our System". -->
<!-- [COPY EDIT] Please verify: "second most active CNCF project after Kubernetes[^2]" — this was true as of 2023–2024 CNCF velocity reports. Confirm current standing for 2026 publication; cite source directly in footnote. -->
<!-- [COPY EDIT] Please verify: "'Graduated' status for tracing and metrics, with logging in 'Stable' as of late 2024". The OTel logs spec reached stable in ~2024 but SDK maturity varies by language. For Go, check go.opentelemetry.io/otel/log status. -->
OpenTelemetry is the second most active CNCF project after Kubernetes[^2]. It has reached "Graduated" status for tracing and metrics, with logging in "Stable" as of late 2024. The ecosystem includes:

<!-- [COPY EDIT] "the de facto standard" — "de facto" is unitalicized per Merriam-Webster and CMOS 7.55. OK. -->
- **Jaeger** and **Zipkin** -- trace backends (OTel can export to both)
- **Prometheus** -- the de facto standard for metrics in Kubernetes
- **Grafana Tempo** -- a trace backend designed for high volume and low cost
- **Grafana Loki** -- a log aggregation system inspired by Prometheus's label model
- **Grafana** -- the visualization layer that ties them all together

<!-- [LINE EDIT] 55-word sentence. Split after "common open-source choice.": "Our stack uses the Grafana family (Tempo, Loki, Grafana) plus Prometheus — a common open-source choice. In production, you might use managed equivalents: AWS X-Ray for traces, CloudWatch for metrics, or a commercial platform like Datadog or Honeycomb." -->
<!-- [COPY EDIT] "open-source" — hyphenated as compound adjective (CMOS 7.81). OK. -->
Our stack uses the Grafana family (Tempo, Loki, Grafana) plus Prometheus. This is a common open-source choice. In production, you might use managed equivalents: AWS X-Ray for traces, CloudWatch for metrics, or a commercial platform like Datadog or Honeycomb. The point of OTel is that switching backends does not require changing your application code -- you change the Collector configuration.

<!-- [LINE EDIT] "The fact that OTel is vendor-neutral is its defining feature." → "OTel's defining feature is that it is vendor-neutral." — cuts four words, same meaning. -->
<!-- [COPY EDIT] "vendor-neutral" — compound adjective before noun, hyphenated (CMOS 7.81). OK. -->
The fact that OTel is vendor-neutral is its defining feature. Before OTel, you chose a vendor (Datadog, New Relic, Honeycomb) and instrumented with their proprietary SDK. Switching vendors meant re-instrumenting your entire codebase. With OTel, your instrumentation is permanent. The export destination is a configuration change.

<!-- [LINE EDIT] Strong closer; keep. "OTel does this for all three pillars of observability." — concise, ends with the chapter's core promise. -->
This is the same value proposition that SLF4J brought to Java logging: write `logger.info("message")` once, switch between Logback, Log4j2, or any other implementation by changing a dependency. OTel does this for all three pillars of observability.

---

## Exercises

<!-- [STRUCTURAL] Five exercises total; the set progresses from reading (W3C spec) to comparison (Micrometer) to exploration (go doc) to reasoning (failure modes). Good ladder. Consider adding a hands-on "sketch the trace for a reservation lease extension" exercise that bridges to section 9.2. -->
<!-- [COPY EDIT] "**Read the W3C Trace Context spec.**" — imperative + period; consistent with sibling items. OK. -->
<!-- [LINE EDIT] Exercise 1: "The `01` flags field" — "flags" here is singular; consider "the `01` flags byte" or "the flags field (`01`)". Minor. -->
1. **Read the W3C Trace Context spec.** Open [w3c.github.io/trace-context](https://www.w3.org/TR/trace-context/) and find the `traceparent` header format. Parse this example manually: `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`. What is the trace ID? The parent span ID? What does the `01` flags field mean?

<!-- [COPY EDIT] "`meterRegistry.counter("my.counter").increment()`" vs. "`meter.Int64Counter("my.counter")`" — accurate. -->
2. **Compare OTel to Micrometer.** If you have a Spring Boot project, compare how you register a custom counter in Micrometer (`meterRegistry.counter("my.counter").increment()`) versus OTel (`meter.Int64Counter("my.counter")`). What are the differences in lifecycle and registration?

<!-- [COPY EDIT] "Run `go doc go.opentelemetry.io/otel`" — command is accurate. -->
3. **Explore the OTel Go packages.** Run `go doc go.opentelemetry.io/otel` and `go doc go.opentelemetry.io/otel/sdk/trace` in a terminal. Identify the TracerProvider, SpanProcessor, and SpanExporter interfaces. How do they compose?

<!-- [FINAL] "Think about failure modes." — exercise 4 is the strongest prompt in the set. Keep as-is. -->
4. **Think about failure modes.** If the OTel Collector goes down, what happens to your traces and metrics? What happens to your application? (Hint: the SDK uses a BatchSpanProcessor with a bounded queue.)

---

## References

<!-- [COPY EDIT] All five footnotes include description text — good practice for a tutorial book. Check URL validity; verify https://www.w3.org/TR/trace-context/ and https://www.cncf.io/projects/opentelemetry/ still resolve to current canonical pages. -->
<!-- [COPY EDIT] Please verify: `https://opentelemetry.io/docs/languages/go/` — this is the current URL (formerly `/docs/instrumentation/go/`). Confirm still valid. -->
<!-- [COPY EDIT] "[^5]: W3C Baggage Specification" — OK. -->
[^1]: [W3C Trace Context Specification](https://www.w3.org/TR/trace-context/) -- The standard for propagating trace context across service boundaries via HTTP headers.
[^2]: [OpenTelemetry CNCF Project Page](https://www.cncf.io/projects/opentelemetry/) -- Project status, governance, and community links.
[^3]: [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/) -- Official getting-started guide and API reference for the Go SDK.
[^4]: [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/) -- The language-agnostic specification that all SDKs implement.
[^5]: [W3C Baggage Specification](https://www.w3.org/TR/baggage/) -- The standard for propagating application-defined key-value pairs alongside trace context.
