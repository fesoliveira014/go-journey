# 7.1 Event-Driven Architecture

So far, our services communicate synchronously: the gateway calls the catalog service over gRPC, waits for a response, and renders the result. This works well for queries—the user asks for a book list and expects an immediate answer. But what happens when a user reserves a book? The reservation service needs to decrement the catalog's `available_copies` count. Should it call the catalog service directly and wait?

It could. A synchronous gRPC call from the reservation service to the catalog service would work. But it introduces **temporal coupling**: the reservation cannot succeed unless the catalog service is up and responding at that exact moment. If the catalog service is restarting, deploying, or experiencing a brief network hiccup, the reservation fails—even though the reservation itself was valid.

This is where event-driven architecture earns its keep.

---

## Sync vs. Async: Choosing the Right Tool

Synchronous communication (gRPC, REST) is the right choice when:

- The caller **needs** the result to continue (e.g., "does this book exist? how many copies are available?")
- The operation must be **consistent immediately** (e.g., checking the user's password during login)
- The interaction is a **query** rather than a notification

Asynchronous communication (message queues, event streams) is the right choice when:

- The caller does not need to wait for the side effect to complete
- The side effect can tolerate **eventual consistency** (a short delay is acceptable)
- You want to **decouple** the producer from the consumer—they do not need to know about each other
- Multiple consumers might react to the same event independently

In our system, when a user reserves a book, the reservation service records the reservation in its own database and returns success immediately. It then publishes a `reservation.created` event. The catalog service consumes that event and decrements `available_copies`. If the catalog service is temporarily down, the event sits in Kafka until it comes back. No data is lost, no reservation fails.

The reservation service does call the catalog service synchronously for one thing: checking availability *before* creating the reservation. This is a deliberate read-before-write pattern—we need current data to make the decision. The async event flow handles the write side effect afterward.

If you have used Spring's `@TransactionalEventListener` or `ApplicationEventPublisher`, the concept is the same: decouple the "something happened" notification from the "react to it" logic. The difference is that Spring events are in-process by default (same JVM), while Kafka events cross process and machine boundaries.

---

## Commands vs. Events

Two terms get used loosely in messaging systems; the distinction matters.

**Commands** tell a service to do something: "create this reservation," "update this book." They are directed at a specific recipient. They can fail. They usually expect exactly one handler. In our system, gRPC calls are commands.

**Events** announce that something happened: "a reservation was created," "a book was returned." They are broadcast to anyone who cares. The publisher does not know (or care) who consumes them. They cannot "fail" in the same way—the fact already happened.

This distinction maps to CQRS (Command Query Responsibility Segregation), a pattern where the write side (commands) and read side (queries) are modeled separately. Our system uses a lightweight version of this: the reservation service owns the write model (reservation records), and the catalog service maintains its own read-optimized data (available copy counts) by consuming events. Neither service directly modifies the other's database.

The `ReservationEvent` struct in our codebase is a true event—it describes a fact in the past tense:

```go
// services/reservation/internal/service/service.go

type ReservationEvent struct {
    Type          string    `json:"event_type"`
    ReservationID string    `json:"reservation_id"`
    UserID        string    `json:"user_id"`
    BookID        string    `json:"book_id"`
    Timestamp     time.Time `json:"timestamp"`
}
```

The `Type` field uses past-tense naming: `reservation.created`, `reservation.returned`, `reservation.expired`. This is a convention worth following—it makes the event's nature unambiguous. If you see `reservation.create` (imperative), it looks like a command. If you see `reservation.created` (past tense), it is clearly a notification of something that happened.

---

## Kafka Fundamentals

Apache Kafka is a distributed event streaming platform. Unlike traditional message queues (RabbitMQ, ActiveMQ), where messages are consumed and deleted, Kafka is a **commit log**: messages are appended to an ordered, immutable log and retained for a configurable period (or indefinitely). Consumers read from the log at their own pace.

### Topics and Partitions

A **topic** is a named stream of events. Our system uses a `reservations` topic for reservation lifecycle events. Topics are divided into **partitions**—ordered, append-only logs. Each message within a partition has a sequential **offset** (0, 1, 2, ...).

When a producer sends a message, it includes a **key**. Kafka hashes the key to determine which partition receives the message. Our publisher uses the book ID as the key:

```go
msg := &sarama.ProducerMessage{
    Topic: p.topic,
    Key:   sarama.StringEncoder(event.BookID),
    Value: sarama.ByteEncoder(value),
}
```

This guarantees that all events for the same book land in the same partition and are processed in order. If book `abc-123` has a `reservation.created` followed by a `reservation.returned`, the consumer sees them in that order. Without key-based partitioning, the events could arrive out of order across different partitions, leading to incorrect availability counts.

### Consumer Groups

A **consumer group** is a set of consumers that cooperate to consume a topic. Kafka assigns each partition to exactly one consumer in the group. If you have three partitions and two consumers in the group, one consumer gets two partitions and the other gets one. If a consumer dies, Kafka **rebalances**—reassigning its partitions to the surviving consumers.

Our catalog consumer uses the group ID `catalog-availability-updater`:

```go
group, err := sarama.NewConsumerGroup(brokers, "catalog-availability-updater", config)
```

If we later need a second service to react to reservation events (say, a notification service that emails users), it would use a *different* group ID. Each group gets its own independent read position on the topic, so both services see every event.

### At-Least-Once Delivery

Kafka provides **at-least-once** delivery by default. After a consumer processes a message, it commits the offset back to Kafka. If the consumer crashes *before* committing, Kafka redelivers the message on the next rebalance. This means your consumer might see the same message twice.

Our consumer marks offsets explicitly by calling `session.MarkMessage`; Sarama batches and commits the marks in the background:

```go
session.MarkMessage(msg, "")
```

This marks the message as processed. Sarama periodically commits marked offsets in the background. If the consumer crashes between processing a message and the next background commit, that message will be redelivered.

This has implications for idempotency, discussed below.

### KRaft Mode

Historically, Kafka required Apache ZooKeeper for cluster metadata management. Since Kafka 3.3, **KRaft mode** (Kafka Raft) replaces ZooKeeper with a built-in consensus protocol. ZooKeeper support was removed entirely in Kafka 4.0. Our Docker Compose setup uses KRaft—one fewer service to manage.

---

## The Sarama Client Library

Go has several Kafka client libraries. We use **Sarama** (`github.com/IBM/sarama`), the oldest and best-known pure-Go implementation. It supports both producing and consuming, consumer groups, and all the Kafka protocol features we need.

The alternatives are:

- **confluent-kafka-go**—a CGo wrapper around librdkafka. Better performance, but it requires a C toolchain to build.
- **franz-go** (`github.com/twmb/franz-go`)—a newer pure-Go client with a more modern API, first-class support for transactions, and generally cleaner ergonomics. See its [comparison page][franz-comparison] for specifics.
- **segmentio/kafka-go**—another pure-Go option, simpler API but fewer features.

> **A note on picking a client in 2026.** Sarama is in maintenance mode. IBM still takes security patches and critical fixes, but active Go Kafka development has largely moved to franz-go — it is what most new Go-on-Kafka projects use today and is the client you will see in recent Kafka-related OSS code. We use Sarama in this book because (a) its API is closer to the raw Kafka protocol concepts most readers already know from other languages, so the code stays didactic, and (b) every Sarama idiom you learn here translates directly to "how would I do this in franz-go?" — the [migration notes][franz-comparison] are short. If you are starting a greenfield Go service against Kafka today, evaluate franz-go first and only fall back to Sarama if you hit a specific gap.
>
> Everything below is correct for Sarama. The patterns (consumer groups, offset commits, backpressure) are library-independent.

[franz-comparison]: https://github.com/twmb/franz-go#comparisons

Sarama's API is lower-level than Spring Kafka's `@KafkaListener` annotation. In Spring, you annotate a method and the framework handles consumer group setup, deserialization, and offset management. In Sarama, you implement an interface and manage the consume loop yourself. This is more code, but the control flow is explicit and there is no annotation magic to debug.

### Producer Setup

The publisher creates a `SyncProducer`—it blocks until Kafka acknowledges the message:

```go
// services/reservation/internal/kafka/publisher.go

func NewPublisher(brokers []string, topic string) (*Publisher, error) {
    config := sarama.NewConfig()
    config.Producer.Return.Successes = true
    config.Producer.RequiredAcks = sarama.WaitForAll

    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        return nil, fmt.Errorf("create kafka producer: %w", err)
    }
    return &Publisher{producer: producer, topic: topic}, nil
}
```

`RequiredAcks = WaitForAll` means the producer waits until all in-sync replicas have acknowledged the write. This is the safest setting—it ensures no data loss if a broker crashes. The trade-off is higher latency. For our use case (a handful of events per reservation), this latency is negligible.

`Return.Successes = true` is required for `SyncProducer`—without it, you cannot detect when a send completes.

An `AsyncProducer` is also available for high-throughput scenarios where you send messages on one goroutine and read acknowledgments on another. We do not need that complexity here.

### Message Serialization

We use JSON for event payloads. This is the simplest choice and works well for low-throughput systems:

```go
func (p *Publisher) Publish(ctx context.Context, event service.ReservationEvent) error {
    value, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }

    msg := &sarama.ProducerMessage{
        Topic: p.topic,
        Key:   sarama.StringEncoder(event.BookID),
        Value: sarama.ByteEncoder(value),
    }
    // ...
}
```

In production systems with high throughput or strict schema evolution requirements, you would use **Avro** or **Protobuf** with a Schema Registry. The Schema Registry enforces backward/forward compatibility rules, preventing a producer from publishing events that consumers cannot deserialize. For a learning project, JSON is fine—just know that it offers no schema enforcement and no built-in evolution guarantees.

---

## Our Event Flow

Here is the complete flow when a user reserves a book:

1. **Gateway** receives `POST /books/{id}/reserve` and calls the reservation service via gRPC.
2. **Reservation service** checks availability by calling the catalog service via gRPC (synchronous read).
3. **Reservation service** creates the reservation record in its database.
4. **Reservation service** publishes a `reservation.created` event to the `reservations` Kafka topic.
5. **Catalog service** consumer picks up the event and decrements `available_copies` in the catalog database.

Steps 1–4 are synchronous from the user's perspective—they get a response after step 3. Step 5 happens asynchronously. There is a brief window where the reservation exists but the catalog still shows the old availability count. This is **eventual consistency** in action.

The same pattern applies in reverse for returns: the reservation service publishes `reservation.returned`, and the catalog consumer increments `available_copies`.

---

## Error Handling and Idempotency

### What happens when publishing fails?

Look at how `CreateReservation` handles a publish failure:

```go
if err := s.publisher.Publish(ctx, ReservationEvent{
    Type:          "reservation.created",
    ReservationID: created.ID.String(),
    UserID:        userID.String(),
    BookID:        bookID.String(),
    Timestamp:     now,
}); err != nil {
    slog.ErrorContext(ctx, "failed to publish event", ...)
}
```

The error is logged but not returned. The reservation was already created in the database—we do not roll it back. This is a pragmatic choice: the reservation is the source of truth. A slightly stale availability count is less harmful than a lost reservation. A reconciliation process (or manual intervention) can fix stale counts.

The alternative—wrapping the database write and the Kafka publish in a single transaction—requires the **Outbox pattern** or **two-phase commit**. Both add significant complexity. The Outbox pattern writes the event to a database table in the same transaction as the business data, then a separate process tails the outbox table and publishes to Kafka. This guarantees at-least-once publishing. For our learning project, the fire-and-log approach is sufficient.

### Idempotency on the Consumer Side

Since Kafka provides at-least-once delivery, a `reservation.created` event might be delivered twice. If the consumer blindly decrements `available_copies` each time, the count drifts.

Our catalog repository uses a SQL guard to prevent negative counts:

```go
// services/catalog/internal/repository/book.go

result := r.db.WithContext(ctx).
    Model(&model.Book{}).
    Where("id = ? AND available_copies + ? >= 0", id, delta).
    Update("available_copies", gorm.Expr("available_copies + ?", delta))
```

The `WHERE available_copies + ? >= 0` clause prevents the count from going below zero. This is a safety net, not true idempotency. True idempotency would require tracking which events have already been processed (e.g., storing the event ID or Kafka offset alongside the update). For our system, the guard is good enough—a duplicate decrement is caught by the non-negative constraint, and a duplicate increment is harmless (the count might be off by one until the next event corrects it).

In a production system, you would likely store a deduplication key (the reservation ID + event type) and check it before applying the update.

---

## Trace Propagation

Both the publisher and consumer propagate OpenTelemetry trace context through Kafka message headers. This allows you to see a single trace that spans the HTTP request, the gRPC call, the Kafka publish, and the consumer processing—across three different services.

The publisher injects the trace context:

```go
otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})
```

The consumer extracts it:

```go
msgCtx := otelgo.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
```

The `headerCarrier` and `consumerHeaderCarrier` types adapt Kafka headers to the `propagation.TextMapCarrier` interface that OpenTelemetry expects. This is a thin adapter—Kafka headers are key-value byte pairs, and OTel propagation expects string key-value pairs. The adapter converts between the two.

We will cover observability in detail in a later chapter. For now, note that this plumbing exists and enables end-to-end tracing through the async boundary.

---

## Exercises

1. **Trace the event flow.** Starting from `ReservationService.CreateReservation`, follow the code path through the publisher, Kafka, and into the catalog consumer's `handleEvent`. Write down each function call in order, noting which service owns each step.

2. **Design a new event.** Suppose we add a feature where admins can add more copies of a book. What event would the catalog service publish? What would the event type be named? Which services might consume it?

3. **Outbox pattern sketch.** Write pseudocode for the Outbox pattern: instead of calling `publisher.Publish()` directly, the service writes an outbox row in the same database transaction. A background goroutine reads unpublished outbox rows and sends them to Kafka. What are the trade-offs compared to our current approach?

4. **Idempotency key.** Modify the `reservationEvent` struct to include a unique event ID. Sketch how the consumer would use this ID to avoid processing the same event twice. What storage would you need?

5. **Async producer.** Sarama offers `AsyncProducer` in addition to `SyncProducer`. Read the Sarama documentation and describe how `AsyncProducer` differs. When would you prefer it over `SyncProducer`?

---

## References

[^1]: [Apache Kafka Documentation](https://kafka.apache.org/documentation/)—Official Kafka documentation covering topics, partitions, consumer groups, and delivery semantics.
[^2]: [IBM/sarama GitHub repository](https://github.com/IBM/sarama)—The Sarama Go client library for Apache Kafka.
[^3]: [Martin Kleppmann—Designing Data-Intensive Applications, Chapter 12](https://dataintensive.net/)—Excellent coverage of stream processing, event sourcing, and exactly-once semantics.
[^4]: [Microservices.io—Event-Driven Architecture pattern](https://microservices.io/patterns/data/event-driven-architecture.html)—Pattern catalog entry with trade-off analysis.
[^5]: [Chris Richardson—Transactional Outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html)—The Outbox pattern for reliable event publishing.
[^6]: [KRaft: Apache Kafka Without ZooKeeper](https://developer.confluent.io/learn/kraft/)—Overview of Kafka's built-in metadata management replacing ZooKeeper.
