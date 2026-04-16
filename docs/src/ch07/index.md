# Chapter 7: Event-Driven Architecture — Reservation Service & Kafka

In this chapter, we build the Reservation service and introduce Apache Kafka for asynchronous inter-service communication. Users can reserve and return books through the gateway, and the catalog's available copies update automatically via event-driven messaging.

> **Tip:** If you haven't already, create an admin account and seed the catalog using the CLI tools from Chapter 6. Sample books in the catalog make it easier to watch events flow through the system.

## What You'll Learn

- Kafka fundamentals: topics, partitions, consumer groups, KRaft mode
- The sarama Go client library for producing and consuming messages
- Event-driven vs. synchronous communication trade-offs
- Eventual consistency and its implications for UI design
- Building a complete microservice end-to-end (proto → DB → service → handler → gateway)

## Architecture Overview

```
Browser → Gateway (HTTP) → Reservation Service (gRPC)
                                    ↓ (sync read)
                           Catalog Service (gRPC)

Reservation Service → Kafka "reservations" topic → Catalog Consumer → updates available_copies
```

**Reads are synchronous** — the Reservation Service calls the Catalog via gRPC to check availability before creating a reservation.

**Writes are asynchronous** — state changes (created, returned, expired) are published to Kafka as events. The Catalog Service runs a consumer goroutine that processes these events and updates book availability.

## Sections

- [7.1 Event-Driven Architecture](./event-driven-architecture.md) — Kafka fundamentals and the sarama Go client
- [7.2 Reservation Service](./reservation-service.md) — Building the service: state machine, expiration on read, TDD
- [7.3 Kafka Consumer](./kafka-consumer.md) — Consumer groups, the co-located consumer pattern, idempotency
- [7.4 Reservation UI](./reservation-ui.md) — Gateway changes, eventual consistency in the UI, Docker setup
