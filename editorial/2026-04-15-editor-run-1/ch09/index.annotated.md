<!-- [STRUCTURAL] Index page is very short — fine for a landing card, but consider a one-sentence bridge explaining *why* observability now (i.e., how it ties to chapters 7–8 and to the operational readiness story in ch10+). A learner-facing chapter opener usually sets stakes: "without this, you cannot debug the system you just built." -->
# Chapter 9: Observability with OpenTelemetry

<!-- [STRUCTURAL] The opening line tells the reader WHAT but not WHY. One extra sentence on "why" would lift motivation: e.g., "Instrumenting a system you can debug in production is the difference between a demo and a service." -->
<!-- [LINE EDIT] "add end-to-end observability to the library system using OpenTelemetry and the Grafana stack" → "add end-to-end observability to the library system with OpenTelemetry and the Grafana stack" — prefer "with" when listing tools. -->
<!-- [COPY EDIT] "end-to-end" correctly hyphenated as compound adjective before noun (CMOS 7.81). OK. -->
In this chapter, we add end-to-end observability to the library system using OpenTelemetry and the Grafana stack.

<!-- [COPY EDIT] "What You'll Learn" — headline-style title case per CMOS 8.159; OK as-is. Confirm consistency with other chapter indexes. -->
## What You'll Learn

<!-- [STRUCTURAL] Five bullets cover the scope, but a sixth bullet on the sidecar/Collector topology would mirror the actual table of contents (9.5). -->
<!-- [COPY EDIT] Serial comma consistency (CMOS 6.19): all list items use Oxford comma; OK. -->
- The three pillars of observability: tracing, metrics, and logging
<!-- [LINE EDIT] "Setting up OpenTelemetry in Go microservices" — "Setting up" is vague; consider "Wiring OpenTelemetry into Go microservices" for active, specific verb. -->
- Setting up OpenTelemetry in Go microservices
<!-- [COPY EDIT] "HTTP, gRPC, Kafka, and PostgreSQL" — serial comma present; product names correctly capitalized. OK. -->
- Instrumenting HTTP, gRPC, Kafka, and PostgreSQL
<!-- [COPY EDIT] "structured logging with `slog`" — `slog` in code font is correct per the style rule. OK. -->
- Migrating to structured logging with `slog`
<!-- [COPY EDIT] "Grafana, Tempo, Prometheus, and Loki" — serial comma present, product capitalization correct. OK. -->
- Deploying Grafana, Tempo, Prometheus, and Loki
<!-- [LINE EDIT] "Correlating traces with logs for debugging" → "Correlating traces with logs to debug production incidents" — adds concrete payoff. -->
- Correlating traces with logs for debugging

<!-- [FINAL] File ends without a trailing newline-terminated reference or "Next:" pointer. Most index pages in a multi-section chapter include a "Sections" list linking to 9.1–9.5. Consider adding. -->
