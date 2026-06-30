# Chapter 7: Event-Driven Architecture—Reservation Service & Kafka

> **Chapter checkpoint**
> Start from: `git checkout chapter-07-start`
> End state: `git checkout chapter-07-end`
>
> Chapter snippets are point-in-time snapshots. Later chapters intentionally change the same files.

In this chapter, we build the Reservation service and introduce Apache Kafka for asynchronous inter-service communication. Users can reserve and return books through the gateway. Catalog still owns the book inventory: Reservation sends synchronous `UpdateAvailability` commands for availability changes, and Kafka carries reservation facts for audit, notification, or future projections.

> **Tip:** If you haven't already, create an admin account and seed the catalog using the CLI tools from Chapter 6. Sample books in the catalog make it easier to watch events flow through the system.

## What You'll Learn

- Kafka fundamentals: topics, partitions, consumer groups, KRaft mode
- The sarama Go client library for producing and consuming messages
- Event-driven vs. synchronous communication trade-offs
- Eventual consistency and its implications for UI design
- Building a complete microservice end-to-end (proto → DB → service → handler → gateway)
- Extending the admin dashboard with a reservation-owned read view

## Architecture Overview

```
Browser → Gateway (HTTP) → Reservation Service (gRPC)
                                    ↓ (sync read)
                           Catalog Service (gRPC)

Reservation Service → Kafka "reservations" topic → audit/notification consumers
```

**Inventory writes are synchronous**—the Reservation Service calls the Catalog via gRPC to atomically decrement or increment availability. Catalog's guarded database update is the source of truth for `available_copies`.

**Lifecycle events are asynchronous**—state changes (`reservation.created`, `reservation.returned`, `reservation.expired`) are published to Kafka after the Reservation Service records them. These are facts for downstream consumers, not commands back into Catalog.

## Sections

- [7.1 Event-Driven Architecture](./event-driven-architecture.md)—Kafka fundamentals and the sarama Go client
- [7.2 Reservation Service](./reservation-service.md)—Building the service: state machine, expiration on read, TDD
- [7.3 Kafka Consumer](./kafka-consumer.md)—Consumer groups, offset handling, and non-mutating reservation event observation
- [7.4 Reservation UI](./reservation-ui.md)—Gateway changes, eventual consistency in the UI, Docker setup
- [7.5 Reservation Admin Dashboard](./admin-dashboard.md)—Admin-only reservation visibility across Auth, Catalog, and Reservation
