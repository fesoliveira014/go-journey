# 7.1 Event-Driven Architecture

<!-- [STRUCTURAL] Section opens with a well-framed motivation (synchronous call fails → temporal coupling). Strong lead. Progression through the section — motivation → sync vs async → commands vs events → Kafka fundamentals → Sarama → event flow → error/idempotency → tracing — is logical and reader-friendly. No structural rearrangement required. -->

So far, our services communicate synchronously: the gateway calls the catalog service over gRPC, waits for a response, and renders the result. This works well for queries -- the user asks for a book list and expects an immediate answer. But what happens when a user reserves a book? The reservation service needs to decrement the catalog's `available_copies` count. Should it call the catalog service directly and wait?

<!-- [COPY EDIT] Dash style: this file uses ` -- ` (spaced double hyphens, commonly rendered as spaced en dashes). CMOS 6.85 prefers unspaced em dashes. Either accept the spaced en dash house style chapter-wide (some technical publishers do) or normalize to em dashes. Pick one and apply consistently across ch07. -->

It could. A synchronous gRPC call from the reservation service to the catalog service would work. But it introduces **temporal coupling**: the reservation cannot succeed unless the catalog service is up and responding at that exact moment. If the catalog service is restarting, deploying, or experiencing a brief network hiccup, the reservation fails -- even though the reservation itself was valid.

<!-- [LINE EDIT] "It could." — good punchy opener. Keep. -->

This is where event-driven architecture earns its keep.

---

## Sync vs. Async: Choosing the Right Tool

<!-- [COPY EDIT] Heading: "Sync vs. Async" — abbreviations fine in a subheading (CMOS 10.47). Subtitle after colon is a fragment, so the first word is capitalized correctly (CMOS 6.63). OK. -->

Synchronous communication (gRPC, REST) is the right choice when:

- The caller **needs** the result to continue (e.g., "does this book exist? how many copies are available?")
- The operation must be **consistent immediately** (e.g., checking the user's password during login)
- The interaction is a **query** rather than a notification

<!-- [COPY EDIT] "e.g.," usage — correct per CMOS 6.43 (followed by a comma). Consistent across the section. Good. -->

Asynchronous communication (message queues, event streams) is the right choice when:

- The caller does not need to wait for the side effect to complete
- The side effect can tolerate **eventual consistency** (a short delay is acceptable)
- You want to **decouple** the producer from the consumer -- they do not need to know about each other
- Multiple consumers might react to the same event independently

<!-- [LINE EDIT] "side effect" vs "side-effect" — used both as a noun ("the side effect") and as a hyphenated compound modifier later ("async event flow handles the write side-effect afterward" on line ~28). Keep noun form "side effect" (open) and "side-effect" hyphenated only when used attributively before another noun. Current usage in line 28 ("side-effect afterward") is noun form, not attributive — should be "side effect" (two words). CMOS 7.81, 7.89. -->

In our system, when a user reserves a book, the reservation service records the reservation in its own database and returns success immediately. It then publishes a `reservation.created` event. The catalog service consumes that event and decrements `available_copies`. If the catalog service is temporarily down, the event sits in Kafka until it comes back. No data is lost, no reservation fails.

<!-- [STRUCTURAL] Note: this paragraph describes the write-first-then-publish flow, but section 7.2 establishes that the reservation service actually calls `catalog.UpdateAvailability` *before* creating the reservation row (the "TOCTOU trap" pattern). That is a deliberate deviation from the "pure" event-driven write. Consider adding a one-sentence forward reference: "We will refine this flow in section 7.2 — the first step in CreateReservation is actually a synchronous decrement, for concurrency-safety reasons." Otherwise, attentive readers will spot the inconsistency between 7.1 and 7.2 and wonder which is the real design. -->

The reservation service does call the catalog service synchronously for one thing: checking availability *before* creating the reservation. This is a deliberate read-before-write pattern -- we need current data to make the decision. The async event flow handles the write side-effect afterward.

<!-- [STRUCTURAL] This sentence describes the old (check-then-write) pattern that 7.2 explicitly calls out as *wrong* under concurrency ("TOCTOU trap"). The actual code does a guarded `UpdateAvailability(-1)` instead of a `GetBook` availability check. This section should either (a) stay abstract and not describe the check, or (b) describe what the code actually does. As written, it contradicts the TOCTOU discussion in 7.2. Recommend: "The reservation service does call the catalog service synchronously for one thing: reserving a copy before creating the reservation row — a guarded decrement, not a read-then-write check. We unpack why in section 7.2." -->
<!-- [COPY EDIT] "side-effect" here should be "side effect" (noun, two words). CMOS 7.89. -->

If you have used Spring's `@TransactionalEventListener` or `ApplicationEventPublisher`, the concept is the same: decouple the "something happened" notification from the "react to it" logic. The difference is that Spring events are in-process by default (same JVM), while Kafka events cross process and machine boundaries.

<!-- [STRUCTURAL] Good analogy — this is exactly the kind of bridging the Java/Kotlin-background reader will appreciate. Keep. -->

---

## Commands vs. Events

Two terms get used loosely in messaging systems, and it is worth distinguishing them:

<!-- [LINE EDIT] "Two terms get used loosely in messaging systems, and it is worth distinguishing them" → "Two terms get used loosely in messaging systems; the distinction matters." — Tighter and more direct. -->

**Commands** tell a service to do something: "create this reservation," "update this book." They are directed at a specific recipient. They can fail. They usually expect exactly one handler. In our system, gRPC calls are commands.

<!-- [COPY EDIT] "create this reservation," "update this book." — commas and period inside quotation marks per CMOS 6.9. Correct. -->

**Events** announce that something happened: "a reservation was created," "a book was returned." They are broadcast to anyone who cares. The publisher does not know (or care) who consumes them. They cannot "fail" in the same way -- the fact already happened.

This distinction maps to CQRS (Command Query Responsibility Segregation), a pattern where the write side (commands) and read side (queries) are modeled separately. Our system uses a lightweight version of this: the reservation service owns the write model (reservation records), and the catalog service maintains its own read-optimized data (available copy counts) by consuming events. Neither service directly modifies the other's database.

<!-- [STRUCTURAL] Slight inaccuracy: the reservation service does mutate the catalog's `available_copies` indirectly via a synchronous gRPC call (`UpdateAvailability`), not only via events. The event-driven path handles `returned` and `expired` increments, but the *decrement* on create is a direct gRPC write. Consider rephrasing "Neither service directly modifies the other's database" → "Neither service reaches into the other's database — writes happen through APIs or events the owning service controls." More accurate, same teaching point. -->
<!-- [COPY EDIT] CQRS expansion: "Command Query Responsibility Segregation" — capitalized as a proper pattern name, per its standard styling (Fowler, Young). OK. -->

The `ReservationEvent` struct in our codebase is a true event -- it describes a fact in the past tense:

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

The `Type` field uses past-tense naming: `reservation.created`, `reservation.returned`, `reservation.expired`. This is a convention worth following -- it makes the event's nature unambiguous. If you see `reservation.create` (imperative), it looks like a command. If you see `reservation.created` (past tense), it is clearly a notification of something that already happened.

<!-- [LINE EDIT] "something that already happened" → "something that happened" — "already" is redundant with the past tense. -->

---

## Kafka Fundamentals

Apache Kafka is a distributed event streaming platform. Unlike traditional message queues (RabbitMQ, ActiveMQ) where messages are consumed and deleted, Kafka is a **commit log**: messages are appended to an ordered, immutable log and retained for a configurable period (or indefinitely). Consumers read from the log at their own pace.

<!-- [COPY EDIT] "RabbitMQ, ActiveMQ" — proper product names, capitalized correctly. OK. -->
<!-- [COPY EDIT] Comma after "deleted" would improve readability: "Unlike traditional message queues (RabbitMQ, ActiveMQ), where messages are consumed and deleted, Kafka is..." — the subordinate clause is non-restrictive. CMOS 6.27. -->

### Topics and Partitions

A **topic** is a named stream of events. Our system uses a `reservations` topic for reservation lifecycle events. Topics are divided into **partitions** -- ordered, append-only logs. Each message within a partition has a sequential **offset** (0, 1, 2, ...).

<!-- [COPY EDIT] "message within a partition has a sequential offset" — terminology: Kafka docs use "record" as the canonical term for what is stored in a partition, though "message" is widely understood. The book earlier contrasts "event" vs "message"; consider "Each record (or event, once deserialized) within a partition has a sequential offset". Optional. -->

When a producer sends a message, it includes a **key**. Kafka hashes the key to determine which partition receives the message. Our publisher uses the book ID as the key:

```go
msg := &sarama.ProducerMessage{
    Topic: p.topic,
    Key:   sarama.StringEncoder(event.BookID),
    Value: sarama.ByteEncoder(value),
}
```

<!-- [COPY EDIT] "When a producer sends a message, it includes a key" — keys are optional in Kafka. Suggest: "A producer can attach a key to each message; Kafka hashes the key to choose the partition." This is more precise and avoids implying keys are required. -->

This guarantees that all events for the same book land in the same partition and are processed in order. If book `abc-123` has a `reservation.created` followed by a `reservation.returned`, the consumer sees them in that order. Without key-based partitioning, the events could arrive out of order across different partitions, leading to incorrect availability counts.

<!-- [STRUCTURAL] Good concrete example (same-book ordering) to motivate partitioning-by-key. Keep. -->
<!-- [COPY EDIT] "events could arrive out of order across different partitions" → "events could be spread across different partitions and processed out of order" — minor clarity. The original is OK. -->

### Consumer Groups

A **consumer group** is a set of consumers that cooperate to consume a topic. Kafka assigns each partition to exactly one consumer in the group. If you have 3 partitions and 2 consumers in the group, one consumer gets 2 partitions and the other gets 1. If a consumer dies, Kafka **rebalances** -- reassigning its partitions to the surviving consumers.

<!-- [COPY EDIT] "3 partitions and 2 consumers" / "2 partitions and the other gets 1" — CMOS 9.2 says spell out zero–nine in general prose, use numerals for 10 and above; but for technical measurements and comparative ratios, numerals are acceptable. Since the counts are technical and the example reads better with numerals, keep. (Alternative: "three partitions and two consumers" — more formal.) -->

Our catalog consumer uses the group ID `catalog-availability-updater`:

```go
group, err := sarama.NewConsumerGroup(brokers, "catalog-availability-updater", config)
```

If we later need a second service to react to reservation events (say, a notification service that emails users), it would use a *different* group ID. Each group gets its own independent read position on the topic, so both services see every event.

<!-- [COPY EDIT] "read position" → canonical Kafka term is "committed offset" or "consumer offset"; "read position" is informal but clear. Consider: "Each group has its own independent committed offset on the topic, so both services see every event." -->

### At-Least-Once Delivery

<!-- [COPY EDIT] Heading: "At-Least-Once Delivery" — hyphenated compound modifier before noun, each element capitalized. CMOS 8.161. Correct. -->

Kafka provides **at-least-once** delivery by default. After a consumer processes a message, it commits the offset back to Kafka. If the consumer crashes *before* committing, Kafka re-delivers the message on the next rebalance. This means your consumer might see the same message twice.

<!-- [COPY EDIT] "re-delivers" — CMOS 7.89 prefers the closed form "redelivers" for prefixes with "re-" unless needed for clarity. Prefer "redelivers". -->

Our consumer commits offsets explicitly by calling `session.MarkMessage`:

```go
session.MarkMessage(msg, "")
```

This marks the message as processed. Sarama periodically commits marked offsets in the background. If the consumer crashes between processing a message and the next background commit, that message will be redelivered.

<!-- [COPY EDIT] Note: "commits offsets explicitly by calling session.MarkMessage" is slightly misleading — MarkMessage does not commit; it *marks for commit*. Sarama's background goroutine performs the actual commit (per `Consumer.Offsets.AutoCommit.Interval`). The next sentence softens this ("Sarama periodically commits marked offsets in the background"), but the first sentence oversells it. Suggest: "Our consumer marks offsets explicitly by calling session.MarkMessage — Sarama batches and commits the marks in the background." This also avoids the "commits/commit" repetition. -->

This has implications for idempotency, which we will return to shortly.

### KRaft Mode

Historically, Kafka required Apache ZooKeeper for cluster metadata management. Since Kafka 3.3, **KRaft mode** (Kafka Raft) replaces ZooKeeper with a built-in consensus protocol. ZooKeeper support was removed entirely in Kafka 4.0. Our Docker Compose setup uses KRaft -- one fewer service to manage.

<!-- [COPY EDIT] Please verify: "Since Kafka 3.3" — KRaft went GA for new clusters in Kafka 3.3 (Oct 2022). Correct. "ZooKeeper support was removed entirely in Kafka 4.0" — confirm against Kafka release notes for 4.0. As of the book's target date (2026), Kafka 4.0 has shipped and has removed ZK support. Verify the exact version. -->
<!-- [COPY EDIT] "KRaft (Kafka Raft)" — acronym expansion on first use, good. CMOS 10.3. -->

---

## The Sarama Client Library

Go has several Kafka client libraries. We use **Sarama** (`github.com/IBM/sarama`), the oldest and best-known pure-Go implementation. It supports both producing and consuming, consumer groups, and all the Kafka protocol features we need.

<!-- [COPY EDIT] "oldest and best-known" — hyphenated "best-known" as a compound adjective before the implied noun. CMOS 7.81. Correct. -->

The alternatives are:

- **confluent-kafka-go** -- a CGo wrapper around librdkafka. Better performance, but requires a C toolchain for building.
- **franz-go** (`github.com/twmb/franz-go`) -- a newer pure-Go client with a more modern API, first-class support for transactions, and generally cleaner ergonomics. See its [comparison page][franz-comparison] for specifics.
- **segmentio/kafka-go** -- another pure-Go option, simpler API but fewer features.

<!-- [COPY EDIT] "a CGo wrapper around librdkafka" — CGo is the accepted capitalization (rendered as "cgo" in the Go toolchain but "Cgo"/"CGo" in prose). librdkafka is lowercase, correct. OK. -->
<!-- [COPY EDIT] "better performance, but requires a C toolchain for building" → "better performance, but it requires a C toolchain to build." Smoother. -->

> **A note on picking a client in 2026.** Sarama is in maintenance mode. IBM still takes security patches and critical fixes, but active Go Kafka development has largely moved to franz-go — it is what most new Go-on-Kafka projects use today and is the client you will see in recent Kafka-related OSS code. We use Sarama in this book because (a) its API is closer to the raw Kafka protocol concepts most readers already know from other languages, so the code stays didactic, and (b) every Sarama idiom you learn here translates directly to "how would I do this in franz-go?" — the [migration notes][franz-comparison] are short. If you are starting a greenfield Go service against Kafka today, evaluate franz-go first and only fall back to Sarama if you hit a specific gap.
>
> Everything below is correct for Sarama. The patterns (consumer groups, offset commits, backpressure) are library-independent.

<!-- [COPY EDIT] Please verify: "Sarama is in maintenance mode. IBM still takes security patches and critical fixes". The claim is supported by commit activity and IBM's stewardship statements, but confirm a citable source (e.g., the IBM/sarama README status block or a release note). -->
<!-- [STRUCTURAL] This sidebar is well-placed and gives the reader an honest recommendation. Keep as-is. -->
<!-- [COPY EDIT] "Go-on-Kafka" as a coinage is readable but unusual. "Go Kafka projects" is more conventional. Optional. -->
<!-- [COPY EDIT] "OSS code" — spell out on first use: "open-source (OSS) code" or just "open-source projects". CMOS 10.3. -->

[franz-comparison]: https://github.com/twmb/franz-go#comparisons

<!-- [COPY EDIT] Please verify: URL and fragment https://github.com/twmb/franz-go#comparisons — confirm the anchor `#comparisons` still exists in the current README (headings can be renamed upstream). -->

Sarama's API is lower-level than Spring Kafka's `@KafkaListener` annotation. In Spring, you annotate a method and the framework handles consumer group setup, deserialization, and offset management. In Sarama, you implement an interface and manage the consume loop yourself. This is more code, but the control flow is explicit and there is no annotation magic to debug.

### Producer Setup

The publisher creates a `SyncProducer` -- it blocks until Kafka acknowledges the message:

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

`RequiredAcks = WaitForAll` means the producer waits until all in-sync replicas have written the message. This is the safest setting -- it ensures no data loss if a broker crashes. The tradeoff is higher latency. For our use case (a handful of events per reservation), this latency is negligible.

<!-- [COPY EDIT] "all in-sync replicas have written the message" — technically "all in-sync replicas have acknowledged the write" (or "have replicated the message"). "Written" implies disk sync which is not quite what ISR acknowledgment guarantees by default. Minor precision point. -->
<!-- [COPY EDIT] "in-sync replicas" — hyphenated compound modifier before noun. CMOS 7.81. Correct. -->

`Return.Successes = true` is required for `SyncProducer` -- without it, you cannot detect when a send completes.

An `AsyncProducer` is also available for high-throughput scenarios where you send messages on one goroutine and read acknowledgments on another. We do not need that complexity here.

<!-- [COPY EDIT] "high-throughput" — hyphenated compound modifier before "scenarios". CMOS 7.81. Correct. -->

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

In production systems with high throughput or strict schema evolution requirements, you would use **Avro** or **Protobuf** with a Schema Registry. The Schema Registry enforces backward/forward compatibility rules, preventing a producer from publishing events that consumers cannot deserialize. For a learning project, JSON is fine -- just know that it offers no schema enforcement and no built-in evolution guarantees.

<!-- [COPY EDIT] "Avro" — proper noun, capitalized. "Protobuf" — accepted short form; the formal product name is "Protocol Buffers". Both are fine. -->
<!-- [COPY EDIT] "Schema Registry" — capitalized consistently as a product name (Confluent Schema Registry); fine in this context as a generic-ish pattern term, but consider lowercase "schema registry" for the generic concept and title case only when naming the Confluent product. Current usage is OK. -->
<!-- [COPY EDIT] "backward/forward compatibility" — slash usage per CMOS 6.106 is acceptable in technical prose; "backward- and forward-compatibility" with hyphens and coordinated prefixes is more formal. Keep current for readability. -->

---

## Our Event Flow

Here is the complete flow when a user reserves a book:

1. **Gateway** receives `POST /books/{id}/reserve` and calls the reservation service via gRPC.
2. **Reservation service** checks availability by calling the catalog service via gRPC (synchronous read).
3. **Reservation service** creates the reservation record in its database.
4. **Reservation service** publishes a `reservation.created` event to the `reservations` Kafka topic.
5. **Catalog service** consumer picks up the event and decrements `available_copies` in the catalog database.

<!-- [STRUCTURAL] Critical inconsistency with 7.2: this numbered list says "checks availability… (synchronous read)" in step 2, implying a GetBook/read pattern. Section 7.2 ("The TOCTOU trap") explicitly describes the implementation as a *guarded decrement via UpdateAvailability*, not a read, and calls the read-then-write version the wrong pattern. Either the step list here must change or the TOCTOU explanation in 7.2 must be reconciled. Suggested rewrite of step 2: "**Reservation service** reserves a copy by calling `catalog.UpdateAvailability(-1)` — a guarded decrement that also serves as the availability check." Then step 3 stays the same. -->

Steps 1-4 are synchronous from the user's perspective -- they get a response after step 3. Step 5 happens asynchronously. There is a brief window where the reservation exists but the catalog still shows the old availability count. This is **eventual consistency** in action.

<!-- [STRUCTURAL] The "catalog still shows the old availability count" claim is true *only if* step 2 is a read, not a decrement. If 7.2 is correct (decrement happens synchronously in step 2), the catalog availability is already decremented by the time the user gets a response — there is no eventual-consistency window on decrement. The eventual-consistency window only matters for returns/expirations, not for creates. This needs reconciling with 7.2. -->
<!-- [COPY EDIT] "Steps 1-4" — en dash, not hyphen, for number ranges (CMOS 6.78). Use `1–4`. Same applies to "1-5", "0, 1, 2, ..." list throughout. -->

The same pattern applies in reverse for returns: the reservation service publishes `reservation.returned`, and the catalog consumer increments `available_copies`.

---

## Error Handling and Idempotency

### What happens when publishing fails?

<!-- [COPY EDIT] Heading: "What happens when publishing fails?" — sentence case with question mark. Heading hierarchy looks fine, but note that most other H3s in this file use title case ("Topics and Partitions", "Consumer Groups"). This H3 breaks the pattern. Either convert all H3s to sentence case or keep title case here: "Publishing Failures". -->

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

The error is logged but not returned. The reservation was already created in the database -- we do not roll it back. This is a pragmatic choice: the reservation is the source of truth. The availability count being slightly off is less harmful than losing the reservation. A reconciliation process (or manual intervention) can fix stale counts.

<!-- [LINE EDIT] "The availability count being slightly off is less harmful than losing the reservation." → "A slightly stale availability count is less harmful than a lost reservation." — Parallels the two nouns; cuts the gerund ("being") and the "-ing" clause. -->

The alternative -- wrapping the database write and the Kafka publish in a single transaction -- requires the **Outbox pattern** or **two-phase commit**. Both add significant complexity. The Outbox pattern writes the event to a database table in the same transaction as the business data, then a separate process tails the outbox table and publishes to Kafka. This guarantees at-least-once publishing. For our learning project, the fire-and-log approach is sufficient.

<!-- [COPY EDIT] "two-phase commit" — lowercase, hyphenated. CMOS 7.89. Correct. -->
<!-- [COPY EDIT] "Outbox pattern" — proper pattern name, capitalized on first reference. Later references can be lowercase "outbox". Consistent so far. Good. -->
<!-- [COPY EDIT] "fire-and-log" — hyphenated compound adjective before "approach". CMOS 7.81. Correct. -->

### Idempotency on the Consumer Side

Since Kafka provides at-least-once delivery, a `reservation.created` event might be delivered twice. If the consumer blindly decrements `available_copies` each time, the count drifts.

<!-- [COPY EDIT] "at-least-once" — hyphenated as a noun/adjective phrase (the delivery semantic). CMOS 7.89. Consistent with earlier usage. Good. -->

Our catalog repository uses a SQL guard to prevent negative counts:

```go
// services/catalog/internal/repository/book.go

result := r.db.WithContext(ctx).
    Model(&model.Book{}).
    Where("id = ? AND available_copies + ? >= 0", id, delta).
    Update("available_copies", gorm.Expr("available_copies + ?", delta))
```

The `WHERE available_copies + ? >= 0` clause prevents the count from going below zero. This is a safety net, not true idempotency. True idempotency would require tracking which events have already been processed (e.g., storing the event ID or Kafka offset alongside the update). For our system, the guard is good enough -- a duplicate decrement is caught by the non-negative constraint, and a duplicate increment is harmless (the count might be off by one until the next event corrects it).

<!-- [STRUCTURAL] The claim "a duplicate increment is harmless" is not quite right. A duplicate `reservation.returned` increments `available_copies` twice, driving it above `total_copies`. Nothing in the code self-corrects this — future events correct the delta, not the absolute value. Section 7.3 kafka-consumer.md makes the same observation more accurately: "a duplicate reservation.returned could increment the count beyond total_copies". Reconcile: either remove the "harmless" sentence or qualify it ("a duplicate increment is visible only as an inflated count until the next decrement cancels it"). -->

In a production system, you would likely store a deduplication key (the reservation ID + event type) and check it before applying the update.

<!-- [COPY EDIT] "reservation ID + event type" — the plus here is shorthand for composition, fine in technical prose. -->

---

## Trace Propagation

Both the publisher and consumer propagate OpenTelemetry trace context through Kafka message headers. This allows you to see a single trace that spans the HTTP request, the gRPC call, the Kafka publish, and the consumer processing -- across three different services.

<!-- [COPY EDIT] "three different services" — count the services: gateway + reservation + catalog = 3. Correct. -->

The publisher injects the trace context:

```go
otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})
```

The consumer extracts it:

```go
msgCtx := otelgo.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
```

<!-- [COPY EDIT] The package alias `otelgo` is used here but `otelgrpc` is the convention in 7.2 and 7.4. Please verify the actual import alias used in the repo (is it `otel` from `go.opentelemetry.io/otel`?). If the repo uses `otel.GetTextMapPropagator()`, update the snippets accordingly. -->

The `headerCarrier` and `consumerHeaderCarrier` types adapt Kafka headers to the `propagation.TextMapCarrier` interface that OpenTelemetry expects. This is a thin adapter -- Kafka headers are key-value byte pairs, and OTel propagation expects string key-value pairs. The adapter converts between the two.

<!-- [COPY EDIT] "OTel" — accepted abbreviation for OpenTelemetry in the community. Consistent with common usage. Good. -->

We will cover observability in detail in a later chapter. For now, note that this plumbing exists and enables end-to-end tracing through the async boundary.

<!-- [COPY EDIT] "end-to-end" — hyphenated compound adjective before "tracing". CMOS 7.81. Correct. -->

---

## Exercises

1. **Trace the event flow.** Starting from `ReservationService.CreateReservation`, follow the code path through the publisher, Kafka, and into the catalog consumer's `handleEvent`. Write down each function call in order, noting which service owns each step.

2. **Design a new event.** Suppose we add a feature where admins can add more copies of a book. What event would the catalog service publish? What would the event type be named? Which services might consume it?

3. **Outbox pattern sketch.** Write pseudocode for the Outbox pattern: instead of calling `publisher.Publish()` directly, the service writes an outbox row in the same database transaction. A background goroutine reads unpublished outbox rows and sends them to Kafka. What are the tradeoffs compared to our current approach?

4. **Idempotency key.** Modify the `reservationEvent` struct to include a unique event ID. Sketch how the consumer would use this ID to avoid processing the same event twice. What storage would you need?

<!-- [COPY EDIT] "reservationEvent" — this is the consumer's private struct name (lowercase `r`), per section 7.3. The producer struct is `ReservationEvent` (uppercase `R`). Correct as written if the exercise is specifically about the consumer-side struct; otherwise clarify. -->

5. **Async producer.** Sarama offers `AsyncProducer` in addition to `SyncProducer`. Read the Sarama documentation and describe how `AsyncProducer` differs. When would you prefer it over `SyncProducer`?

<!-- [STRUCTURAL] Exercises progress from concrete code-tracing (1) to design (2) to pattern sketching (3, 4) to library-specific research (5). Good gradient. Consider adding one more exercise on the `ReservationEvent` schema-evolution question — e.g., "Add a `reservation.extended` event type. What changes in the producer, consumer, and topic? What would break if the consumer shipped before the producer, or vice versa?" This reinforces the schema-registry discussion and the tolerant-reader pattern introduced in 7.3. -->

---

## References

[^1]: [Apache Kafka Documentation](https://kafka.apache.org/documentation/) -- Official Kafka documentation covering topics, partitions, consumer groups, and delivery semantics.
[^2]: [IBM/sarama GitHub repository](https://github.com/IBM/sarama) -- The Sarama Go client library for Apache Kafka.
[^3]: [Martin Kleppmann -- Designing Data-Intensive Applications, Chapter 12](https://dataintensive.net/) -- Excellent coverage of stream processing, event sourcing, and exactly-once semantics.
[^4]: [Microservices.io -- Event-Driven Architecture pattern](https://microservices.io/patterns/data/event-driven-architecture.html) -- Pattern catalog entry with tradeoff analysis.
[^5]: [Chris Richardson -- Transactional Outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html) -- The Outbox pattern for reliable event publishing.
[^6]: [KRaft: Apache Kafka Without ZooKeeper](https://developer.confluent.io/learn/kraft/) -- Overview of Kafka's built-in metadata management replacing ZooKeeper.

<!-- [COPY EDIT] Footnote-marker hyphens: " -- " between author and title. For reference-list entries, a period or an em dash is cleaner than spaced double-hyphen. Consider standardizing to em dash or colon across all chapters. -->
<!-- [COPY EDIT] Please verify: URL https://dataintensive.net/ (Kleppmann's site) — still live. -->
<!-- [COPY EDIT] Please verify: URL https://developer.confluent.io/learn/kraft/ — the Confluent learning paths are regularly restructured. -->
