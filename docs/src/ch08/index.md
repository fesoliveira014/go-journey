# Chapter 8: Full-Text Search — Meilisearch & Event-Driven Indexing

In this chapter we add full-text search to the library system. The Catalog service publishes `catalog.books.changed` events to Kafka whenever books are created, updated, or deleted. A new Search service consumes those events, maintains a Meilisearch index, and exposes Search and Suggest gRPC RPCs. The Gateway gets a search page with HTMX-powered autocomplete.

## What You'll Learn

- Publishing domain events from an existing service (Catalog → Kafka)
- Meilisearch fundamentals: indexes, searchable/filterable attributes, faceted search
- The meilisearch-go client library
- Bootstrap pattern: syncing state on startup when events are unavailable
- Building autocomplete with HTMX and server-rendered partials
- Adding a new service end-to-end (proto → index → service → handler → gateway)

## Architecture Overview

```
Catalog Service → Kafka "catalog.books.changed" → Search Consumer → Meilisearch

Gateway → Search Service (gRPC) → Meilisearch
```

## Sections

- [8.1 Catalog Event Publishing](./catalog-events.md) — Adding a Kafka producer to the Catalog service
- [8.2 Search Service](./search-service.md) — Building the Search service: index layer, service, handler
- [8.3 Meilisearch Integration](./meilisearch.md) — Meilisearch concepts, configuration, and the Go client
- [8.4 Search UI](./search-ui.md) — Gateway search page, autocomplete, Docker setup
