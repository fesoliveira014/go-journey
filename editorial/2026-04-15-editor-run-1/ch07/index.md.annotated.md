# Chapter 7: Event-Driven Architecture — Reservation Service & Kafka

<!-- [STRUCTURAL] Chapter landing page. Concise and well-scoped. The sections list delivers a clear progression: theory → service → consumer → UI. One consideration: the title uses "&" which is fine for a TOC entry but is inconsistent with prose style elsewhere; acceptable for a chapter title. -->

<!-- [STRUCTURAL] Consider whether readers arriving here straight from Chapter 6 benefit from a one-sentence recap of what the catalog service already does, so the "we'll add eventing" framing lands. Not required — the chapter body reintroduces context. -->

In this chapter we build the Reservation service and introduce Apache Kafka for asynchronous inter-service communication. Users can reserve and return books through the gateway, and the catalog's available copies update automatically via event-driven messaging.

<!-- [COPY EDIT] "Reservation service" is used lowercase here but "Reservation Service" appears capitalized in the H1 title and in sibling section headings. Pick one and apply consistently. Recommendation: lowercase in running prose ("reservation service"), title case in headings. CMOS 8.1. -->

> **Tip:** If you haven't already, create an admin account and seed the catalog using the CLI tools from Chapter 6. Having sample books in the catalog will make it easier to see events flowing through the system.

<!-- [LINE EDIT] "Having sample books in the catalog will make it easier to see events flowing through the system." → "Sample books in the catalog make it easier to watch events flow through the system." — Cuts "will make" and removes the gerund opener. -->

## What You'll Learn

<!-- [COPY EDIT] Heading case: "What You'll Learn" uses headline caps (CMOS 8.159). Good. Ensure the same style applies to H2s elsewhere in the chapter. -->

- Kafka fundamentals: topics, partitions, consumer groups, KRaft mode
- The sarama Go client library for producing and consuming messages
- Event-driven vs. synchronous communication tradeoffs
- Eventual consistency and its implications for UI design
- Building a complete microservice end-to-end (proto → DB → service → handler → gateway)

<!-- [COPY EDIT] "sarama" — proper product name is "Sarama" (capitalized) in prose. Use lowercase `sarama` only when referring to the Go import path / package identifier. CMOS 8.153. -->
<!-- [COPY EDIT] "Event-driven vs. synchronous communication tradeoffs" — "vs." is acceptable per CMOS 10.47 in informal/technical prose; consistent with later use. OK. -->
<!-- [COPY EDIT] "tradeoffs" is used here and throughout. CMOS and Merriam-Webster prefer "trade-offs" but "tradeoffs" is widely accepted in technical writing. Be consistent across the chapter — either all "tradeoffs" or all "trade-offs". -->

## Architecture Overview

```
Browser → Gateway (HTTP) → Reservation Service (gRPC)
                                    ↓ (sync read)
                           Catalog Service (gRPC)

Reservation Service → Kafka "reservations" topic → Catalog Consumer → updates available_copies
```

<!-- [STRUCTURAL] Good visual early in the chapter. Consider replacing with a proper diagram (Mermaid / figure) in the HTML build, and labelling the two flows ("synchronous path", "asynchronous path") directly on the arrows. -->

**Reads are synchronous** — the reservation service calls the catalog via gRPC to check availability before creating a reservation.

<!-- [COPY EDIT] Em dash usage correct (CMOS 6.85, no spaces). Note: earlier in the repo convention (e.g., 7.1 body) uses " -- " double-hyphens which render as en dashes in many MD processors. Decide on em dash vs. spaced en dash chapter-wide and normalize. -->

**Writes are asynchronous** — state changes (created, returned, expired) are published as events to Kafka. The catalog service runs a consumer goroutine that processes these events and updates book availability.

<!-- [LINE EDIT] "state changes (created, returned, expired) are published as events to Kafka" → "state changes (created, returned, expired) are published to Kafka as events" — slightly more natural ordering. Optional. -->

## Sections

- [7.1 Event-Driven Architecture](./event-driven-architecture.md) — Kafka fundamentals and the sarama Go client
- [7.2 Reservation Service](./reservation-service.md) — Building the service: state machine, expiration on read, TDD
- [7.3 Kafka Consumer](./kafka-consumer.md) — Consumer groups, the co-located consumer pattern, idempotency
- [7.4 Reservation UI](./reservation-ui.md) — Gateway changes, eventual consistency in the UI, Docker setup

<!-- [STRUCTURAL] 7.2 sub-bullet mentions "TDD" but the reservation-service.md file does not actually cover TDD as a topic — it covers a state machine and expiration strategy. Either remove "TDD" from this description or add a TDD section to 7.2. Current wording is misleading. -->
<!-- [STRUCTURAL] 7.4 sub-bullet mentions "Docker setup" but the reservation-ui.md file has only a brief "Testing the Full Flow" paragraph that mentions Docker Compose — there is no Docker setup content. Consider "manual end-to-end testing" instead. -->
<!-- [COPY EDIT] Heading case inconsistency check: "Event-Driven Architecture" capitalizes both parts of the hyphenated compound (correct per CMOS 8.161). "Kafka Consumer" — good. "Reservation UI" — "UI" is an acronym, correct. -->
<!-- [FINAL] Cross-ref to "Chapter 6" — verify Chapter 6 actually introduces the CLI seed tools referenced in the Tip. -->
