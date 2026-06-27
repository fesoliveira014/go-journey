# Chapter 9: Observability with OpenTelemetry

> **Chapter checkpoint**
> Start from: `git checkout chapter-09-start`
> End state: `git checkout chapter-09-end`
>
> Chapter snippets are point-in-time snapshots. Later chapters intentionally change the same files.

In this chapter, we add end-to-end observability to the library system with OpenTelemetry and the Grafana stack.

## What You'll Learn

- The three pillars of observability: tracing, metrics, and logging
- Wiring OpenTelemetry into Go microservices
- Instrumenting HTTP, gRPC, Kafka, and PostgreSQL
- Migrating to structured logging with `slog`
- Deploying Grafana, Tempo, Prometheus, and Loki
- Correlating traces with logs to debug production incidents
