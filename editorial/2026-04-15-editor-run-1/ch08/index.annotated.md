# Chapter 8: Full-Text Search — Meilisearch & Event-Driven Indexing

<!-- [STRUCTURAL] Opening delivers: one-paragraph architecture summary followed by learning objectives is a solid template-style landing page. Consider whether a brief "Prerequisites" note (Kafka covered in prior chapter? catalog service exists?) would help readers who skipped ahead. -->
<!-- [LINE EDIT] "In this chapter we add" → "This chapter adds" — slightly more direct for a landing page, though either is acceptable for a tutor voice. Keeping author voice. -->
In this chapter we add full-text search to the library system. The Catalog service publishes `catalog.books.changed` events to Kafka whenever books are created, updated, or deleted. A new Search service consumes those events, maintains a Meilisearch index, and exposes Search and Suggest gRPC RPCs. The Gateway gets a search page with HTMX-powered autocomplete.
<!-- [COPY EDIT] "Search and Suggest gRPC RPCs" — "RPC" already means "remote procedure call"; "gRPC RPC" is redundant but conventional in the gRPC community. Acceptable. -->
<!-- [COPY EDIT] "Catalog service" vs "Catalog" vs "Gateway" — verify capitalization is consistent across chapter. Here "Catalog service" (lowercase service) and "Gateway" (capitalized) appear; recommend lowercase "service" throughout when used as a common noun: "the Gateway service gets..." or lowercase both. -->

## What You'll Learn

<!-- [COPY EDIT] "What You'll Learn" uses title case; other ch08 section headings use title case too — consistent. Good. -->
- Publishing domain events from an existing service (Catalog → Kafka)
- Meilisearch fundamentals: indexes, searchable/filterable attributes, faceted search
- The meilisearch-go client library
<!-- [COPY EDIT] "meilisearch-go" — product/library name; keep lowercase per repo convention (github.com/meilisearch/meilisearch-go). Consistent with later usage. -->
- Bootstrap pattern: syncing state on startup when events are unavailable
- Building autocomplete with HTMX and server-rendered partials
- Adding a new service end-to-end (proto → index → service → handler → gateway)

## Architecture Overview

<!-- [STRUCTURAL] ASCII diagram is minimal but serves. Consider labeling arrows (produce / consume / index / query) to make data direction explicit for a reader skimming. Optional. -->
```
Catalog Service → Kafka "catalog.books.changed" → Search Consumer → Meilisearch

Gateway → Search Service (gRPC) → Meilisearch
```

## Sections

<!-- [LINE EDIT] "Adding a Kafka producer" / "Building the Search service" — mixed gerund style is fine and parallel. Keep. -->
- [8.1 Catalog Event Publishing](./catalog-events.md) — Adding a Kafka producer to the Catalog service
- [8.2 Search Service](./search-service.md) — Building the Search service: index layer, service, handler
- [8.3 Meilisearch Integration](./meilisearch.md) — Meilisearch concepts, configuration, and the Go client
- [8.4 Search UI](./search-ui.md) — Gateway search page, autocomplete, Docker setup
<!-- [FINAL] Cross-ref check: 8.4 description mentions "Docker setup" but search-ui.md focuses on HTMX, templates, and data flow — there is no Docker section. Either remove "Docker setup" from this line or add a Docker subsection to 8.4. Likely a stale description. -->
