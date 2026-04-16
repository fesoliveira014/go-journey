# Changelog: grafana-stack.md

## Pass 1: Structural / Developmental
- 10 comments. Themes:
  - Two major factual/currency concerns: (1) Promtail is in maintenance mode, replaced by Grafana Alloy; for a 2026 book this needs at minimum a sidebar acknowledging Alloy. (2) OTel logs support status may need updating.
  - The `trace_id` as a Loki label is a high-cardinality anti-pattern in production; add a caveat.
  - The Trace-to-Log Correlation Walkthrough (9-step numbered list) is the chapter's conceptual climax; it's well-positioned.
  - Section "Manual Verification" could be retitled "Smoke Test" or "Verify the Stack" — "Manual Verification" sounds bureaucratic.
  - Gauge/UpDownCounter confusion from 9.1 surfaces again in the Prometheus metrics table; a one-line footnote would resolve.
  - Four-panel dashboard walkthrough is the right scope.
  - Architecture recap diagram effectively revisits 9.1's diagram in compressed form.

## Pass 2: Line Editing
- **Line ~68:** split 41-word processor sentence
  - Before: "In production, you would add more processors here: `memory_limiter` to prevent the Collector from using too much RAM, `probabilistic_sampler` to drop a percentage of traces, `attributes` to add or remove span attributes, or `tail_sampling` for intelligent sampling decisions based on trace duration or error status."
  - After: enumerate each processor on its own line or as a bulleted list for scannability.
  - Reason: single sentence with four commas is a wall of configuration.
- **Line ~114:** split 44-word Tempo object-storage sentence
  - Before: "In production, you would use object storage (S3, GCS, Azure Blob) as the backend instead of local disk. Tempo is designed for this -- it stores traces as compressed blocks in object storage, which is orders of magnitude cheaper than a traditional database. This is why Tempo can handle high trace volumes without breaking the budget."
  - After: already multi-sentence; minor tightening possible.
  - Reason: pacing.
- **Line ~162:** split 58-word Loki opener
  - Before: "The relationship is similar to Elasticsearch and Filebeat, but with a key difference: Loki does not index the full text of log lines. Instead, it indexes only labels (key-value pairs like `container_name=catalog`), and performs grep-like searches on the log content at query time. This makes it dramatically cheaper to run than Elasticsearch."
  - After: already broken; OK.
- **Line ~258:** split 44-word tracesToLogsV2 sentence
  - Before: "The critical piece is `tracesToLogsV2` on the Tempo datasource. This tells Grafana: 'when viewing a trace in Tempo, offer a link to Loki filtered by `trace_id`.' The `filterByTraceID: true` setting means clicking a trace will automatically query Loki for `{trace_id="<id>"}`, showing you every log line from every service that participated in that trace."
  - After: already three sentences; OK.
  - Reason: load-bearing paragraph, keep tight.

## Pass 3: Copy Editing
- **File-wide:** `--` double-hyphens should be em dashes without spaces (CMOS 6.85).
- **Line ~20:** Please verify — "OTel's log support in Go is maturing but not yet the standard path" for 2026 publication. The `go.opentelemetry.io/otel/log` API may have reached stable; update accordingly.
- **Line ~123:** Please verify — Tempo HTTP API endpoint `curl http://localhost:3200/api/traces/<id>`. Correct for Tempo 2.7 monolithic config on port 3200.
- **Line ~153:** Please verify — metric names on the Prometheus side. Current OTel HTTP semantic conventions use `http.server.request.duration`; Prometheus exporter emits underscore-form. Confirm exact Prometheus-side string.
- **Line ~163:** Please verify — Promtail status. As of 2024, Grafana labeled Promtail "maintenance mode" and recommends Grafana Alloy for new deployments. Major currency issue for 2026 publication. Options: (1) add sidebar noting Alloy as the modern path, (2) switch the whole section to Alloy, (3) keep Promtail with note. Author decision required.
- **Line ~196:** Please verify and add caveat — `trace_id` as a Loki label is high-cardinality (anti-pattern in production). Recommend sentence: "In production, you would keep `trace_id` as a structured field (not a label) to avoid index blowup."
- **Line ~207:** "Postgres" vs "PostgreSQL" — normalize. The style rule calls for "PostgreSQL" except when referring to the `postgres` CLI or container. Consistency fix.
- **Line ~222:** Footnote `[^6]` placement after "auto-provision" — move to end of sentence per chapter-wide footnote convention.
- **Line ~297:** "200ms" — should be "200 ms" per SI spacing (CMOS 9.17). Recurring SI spacing issue across the chapter.
- **Line ~325:** "steps 2-3" — hyphen should be en dash for numeric ranges (CMOS 6.78): "steps 2–3".
- **Line ~329:** "-- from metric alert, to trace, to logs --" — em dashes without spaces (CMOS 6.85).
- **Line ~338 (Docker Compose images):** Please verify all image tags at publication:
  - `otel/opentelemetry-collector-contrib:0.149.0`
  - `grafana/tempo:2.7.0`
  - `prom/prometheus:v3.1.0` (Prometheus 3.x exists; confirm 3.1.0)
  - `grafana/loki:3.3.0`
  - `grafana/promtail:3.3.0` (also flag deprecation)
  - `grafana/grafana:11.5.0`
- **Line ~405:** "Library System Overview" vs. provider name "Library System" in YAML — normalize dashboard title.
- **Line ~423:** "status codes >= 400" — consider Unicode ≥ for typographic polish; stylistic.
- **Line ~429:** Please verify — "`otel-collector:8888` (the Collector's self-telemetry port)". Collector's default self-telemetry is on 8888. Confirm for current Collector version.

## Pass 4: Final Polish
- **Line ~18:** "Services write structured JSON to stdout" — clear.
- **Line ~325 (walkthrough step 4):** "steps 2-3" — flagged above as hyphen vs. en dash.
- **Line ~394:** "Grafana handles reconnection internally" — clear practical note.
- **Line ~403:** "manual verification" heading — bureaucratic; suggest retitle.
- No typos, doubled words, or homophone errors detected.
- Cross-refs to `TraceLogHandler` (9.3), `otelhttp`/`otelgrpc` (9.2), and architecture diagram (9.1) verified.
