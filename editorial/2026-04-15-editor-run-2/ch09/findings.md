# Findings: Chapter 9

**Global issue for this chapter:** All files use ` -- ` for em dashes. Batch-replace with `—` (no spaces). ~20+ instances across the chapter.

---

## index.md

### Summary
Clean file. No issues.

---

## otel-fundamentals.md

### Summary
Reviewed ~180 lines. 1 structural, 2 line edits, 1 copy edit. 1 factual query.

### Structural
- **L104–111:** "OTel API/SDK Split" subsection repeats content from L38–39 (same SLF4J analogy). Condense one or cross-reference rather than restating.

### Line Edits
- **L3–4:** Remove comma before "without" in "ask arbitrary questions about its behavior in production, without deploying new code" — no pause needed before an adverbial phrase completing the verb.
- **L117:** "it is worth knowing about" → cut or rephrase — flab ("worth noting" variant).

### Copy Edit & Polish
- **L99:** "Gauge (UpDownCounter)" — clarify that OTel uses `UpDownCounter`, not `Gauge`, for push-based metrics. The parenthetical could cause confusion.

### Factual Queries
- **L157:** "Graduated" status — clarify that this refers to CNCF project maturity level (March 2024), not individual signal maturity. Tracing and metrics APIs/SDKs are stable; the signals have their own maturity levels.

---

## instrumentation.md

### Summary
Reviewed ~270 lines. 0 structural, 1 line edit, 0 copy edits. 1 factual query.

### Line Edits
- **L187:** "this is the equivalent of" → "this is equivalent to" — drop "the."

### Factual Queries
- **L99:** `http_server_request_duration_seconds` — metric was renamed in OTel Semantic Conventions v1.21+. Verify the exact name produced by the `otelhttp` version used in the project.

---

## structured-logging.md

### Summary
Reviewed ~320 lines. Clean file. No issues beyond the global em dash fix.

---

## grafana-stack.md

### Summary
Reviewed ~400 lines. 0 structural, 0 line edits, 1 copy edit. 5 factual queries.

### Copy Edit & Polish
- **L297:** "SLI (Service Level Indicator)" — while the parenthetical expansion is present, consider a brief gloss for readers unfamiliar with SRE terminology.

### Factual Queries
- **L337:** `otel/opentelemetry-collector-contrib:0.149.0` — verify this version exists at publication time. Plausible for April 2026 given monthly releases.
- **L345:** `grafana/tempo:2.7.0` — verify at publication.
- **L349:** `prom/prometheus:v3.1.0` — Prometheus v3.x was in development as of 2025. Verify v3.1.0 exists.
- **L354:** `grafana/loki:3.3.0` — plausible. Verify at publication.
- **L367:** `grafana/grafana:11.5.0` — plausible. Verify at publication.

---

## sidecar-pattern.md

### Summary
Reviewed ~215 lines. 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L128:** "Kubernetes DaemonSet runs one Collector per node" → "A Kubernetes DaemonSet runs one Collector per node" — missing article.
- **L179:** "The DaemonSet model dominates production Kubernetes deployments" — strong claim. Consider softening: "is the most common choice in production Kubernetes deployments."
