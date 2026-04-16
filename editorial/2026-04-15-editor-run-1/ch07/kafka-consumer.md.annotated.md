# 7.3 Kafka Consumer

<!-- [STRUCTURAL] Good flow: ConsumerGroupHandler interface → consume loop → message processing → error strategy → routing → availability update → trace context → run loop → Spring comparison. Each subsection has one clear teaching goal. Keep. -->

Section 7.1 covered the theory of event-driven architecture. This section gets into the mechanical details: how the catalog service actually consumes reservation events from Kafka using the Sarama library's consumer group API.

<!-- [LINE EDIT] "how the catalog service actually consumes" → "how the catalog service consumes" — "actually" is filler. -->
<!-- [COPY EDIT] "Sarama" — proper noun, capitalized. Good. -->

If you have used Spring Kafka, consumer setup there is a matter of annotating a method with `@KafkaListener` and letting the framework handle group management, deserialization, and offset commits. In Go with Sarama, you implement an interface and manage the consume loop explicitly. More code, but nothing is hidden.

<!-- [LINE EDIT] "consumer setup there is a matter of annotating a method" → "consumer setup is a matter of annotating a method" — "there" is redundant with "Spring Kafka" in the conditional clause. -->

---

## The ConsumerGroupHandler Interface

Sarama's consumer group API is built around a single interface:

```go
type ConsumerGroupHandler interface {
    Setup(ConsumerGroupSession) error
    Cleanup(ConsumerGroupSession) error
    ConsumeClaim(ConsumerGroupSession, ConsumerGroupClaim) error
}
```

The lifecycle works like this:

1. **Setup** is called at the start of a new consumer group session -- after a rebalance assigns partitions to this consumer. Use it to initialize resources if needed.
2. **ConsumeClaim** is called once per assigned partition. It runs in its own goroutine. You read messages from the claim's channel and process them.
3. **Cleanup** is called when the session ends (before the next rebalance). Use it to flush buffers or release resources.

<!-- [COPY EDIT] Dash style: ` -- ` spaced. Normalize chapter-wide. (CMOS 6.85) -->

This three-phase lifecycle maps roughly to the `ConsumerRebalanceListener` in the Java Kafka client, where `onPartitionsAssigned` and `onPartitionsRevoked` serve the same purpose as Setup and Cleanup.

<!-- [STRUCTURAL] Good cross-language mapping for the target reader. Keep. -->

Our implementation keeps Setup and Cleanup empty -- we have no session-scoped resources to manage:

```go
// services/catalog/internal/consumer/consumer.go

type consumerHandler struct {
    svc AvailabilityUpdater
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
```

The `AvailabilityUpdater` interface is the consumer's only dependency:

```go
type AvailabilityUpdater interface {
    UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) (*model.Book, error)
}
```

This is a **role interface** -- it describes the one capability the consumer needs from the catalog service. The full `CatalogService` has many methods (Create, Update, Delete, List), but the consumer only calls `UpdateAvailability`. Defining a narrow interface means the consumer is decoupled from the rest of the catalog service. In tests, you mock one method, not twenty.

<!-- [STRUCTURAL] Solid introduction of the role-interface pattern. Good. -->
<!-- [COPY EDIT] "Create, Update, Delete, List" — capitalized as method names. Correct in context. -->
<!-- [COPY EDIT] "twenty" — spelled out, consistent with CMOS 9.2 (spell out zero through ninety-nine in prose). Good. -->

---

## The Consume Loop

The `Run` function sets up the consumer group and enters an infinite consume loop:

```go
func Run(ctx context.Context, brokers []string, topic string, svc AvailabilityUpdater) error {
    config := sarama.NewConfig()
    config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
        sarama.NewBalanceStrategyRoundRobin(),
    }
    config.Consumer.Offsets.Initial = sarama.OffsetOldest

    group, err := sarama.NewConsumerGroup(brokers, "catalog-availability-updater", config)
    if err != nil {
        return fmt.Errorf("create consumer group: %w", err)
    }
    defer group.Close()

    handler := &consumerHandler{svc: svc}

    for {
        if err := group.Consume(ctx, []string{topic}, handler); err != nil {
            slog.Error("consumer error", "error", err)
        }
        if ctx.Err() != nil {
            return nil
        }
    }
}
```

Let us unpack the configuration:

<!-- [LINE EDIT] "Let us unpack the configuration:" → "The configuration, piece by piece:" — Drops the tutorial preamble. Optional. -->

**`BalanceStrategyRoundRobin`** controls how partitions are distributed among consumers in the group during a rebalance. Round-robin assigns them evenly. Other strategies exist (`Range`, `Sticky`), but round-robin is the simplest and works well for most cases.

<!-- [COPY EDIT] Please verify: Sarama's API surface. The current Sarama (>= v1.34) uses `sarama.NewBalanceStrategyRoundRobin()` as shown; earlier versions exposed `BalanceStrategyRoundRobin` directly as a struct. Confirm the factory-function form is accurate for the repo's pinned Sarama version. -->

**`OffsetOldest`** means that when the consumer group has no previously committed offset (first startup, or after offset expiry), it starts reading from the oldest available message. The alternative is `OffsetNewest`, which skips all existing messages and only reads new ones. We use `OffsetOldest` so that if the catalog service was down while reservations were being made, it catches up on all missed events when it restarts.

<!-- [LINE EDIT] "all existing messages and only reads new ones" → "existing messages and reads only new ones" — Places "only" next to "new" (the thing it modifies). Minor clarity. -->
<!-- [STRUCTURAL] Good practical motivation for the choice. Keep. -->

**The `for` loop.** `group.Consume` blocks until the session ends (due to a rebalance or context cancellation). When it returns, we check if the context is done. If not, we loop back and rejoin -- this handles rebalances gracefully. If the context is cancelled (application shutdown), we return.

<!-- [COPY EDIT] "cancelled" — UK spelling. If US house style, "canceled". (CMOS 7.4) -->

This pattern -- `for { Consume(); if ctx.Err() != nil { return } }` -- is idiomatic Sarama. The `Consume` call manages the entire lifecycle: joining the group, receiving partition assignments, calling Setup/ConsumeClaim/Cleanup, and then returning when the session ends.

---

## Processing Messages

The core logic is in `ConsumeClaim`:

```go
func (h *consumerHandler) ConsumeClaim(
    session sarama.ConsumerGroupSession,
    claim sarama.ConsumerGroupClaim,
) error {
    ctx := session.Context()
    for msg := range claim.Messages() {
        msgCtx := otelgo.GetTextMapPropagator().Extract(ctx, consumerHeaderCarrier(msg.Headers))
        msgCtx, span := otelgo.Tracer("catalog").Start(msgCtx, "catalog.consume.reservation")

        if err := handleEvent(msgCtx, h.svc, msg.Value); err != nil {
            slog.ErrorContext(msgCtx, "failed to handle event", "error", err)
            span.End()
            continue
        }

        span.End()
        session.MarkMessage(msg, "")
    }
    return nil
}
```

<!-- [COPY EDIT] Please verify: OpenTelemetry Go package alias — `otelgo` here and in 7.1, but the canonical alias in most Go repos is `otel` (`go.opentelemetry.io/otel`). Confirm against the repo and use whichever alias the source actually uses. If the code uses `otel`, update the snippets across 7.1, 7.2, and 7.3 together. -->

Step by step:

1. **Get the session context.** `session.Context()` returns a context that is cancelled when the session ends (rebalance or shutdown). This is your cancellation signal.

<!-- [COPY EDIT] "cancelled" — spelling consistency (see above). -->

2. **Range over messages.** `claim.Messages()` is a Go channel. The `for range` loop reads messages until the channel closes (session end). This is a clean, idiomatic pattern -- no polling, no sleep loops.

3. **Extract trace context.** The producer injected OpenTelemetry headers into the Kafka message. We extract them here so the consumer's span is linked to the producer's trace. This gives you a single distributed trace from the HTTP request through the reservation service, through Kafka, into the catalog consumer.

4. **Handle the event.** `handleEvent` deserializes and processes the message. If it fails, we log and continue -- we do not retry, and we do not mark the message.

<!-- [COPY EDIT] "deserializes" — US spelling. Good. -->

5. **Mark the message.** `session.MarkMessage(msg, "")` tells Sarama this message has been processed. Sarama periodically commits marked offsets to Kafka in the background (controlled by `Consumer.Offsets.AutoCommit.Interval`, default 1 second).

<!-- [COPY EDIT] Please verify: Sarama default `Consumer.Offsets.AutoCommit.Interval` — default is 1 second. Confirm in the current Sarama config reference. Accurate historically; recheck against the pinned version. -->

### The "Log and Continue" Error Strategy

When `handleEvent` fails, the consumer logs the error and moves on to the next message. The failed message is *not* marked, so it will not be committed -- but it will not be retried either (not until the next rebalance or restart). This is a pragmatic choice:

<!-- [STRUCTURAL] Technically important nuance: "not marked, so it will not be committed" — but if a *later* message in the same partition is marked, Sarama's auto-commit commits offsets up to (and including) the latest marked offset, which *skips over* the failed-and-unmarked message. Effectively the failed message is silently skipped, not retried on the next restart. The prose says "will not be retried either (not until the next rebalance or restart)" — on restart, if a later message was committed, the failed message is never seen again. This is worth clarifying. Suggest: "not marked — but marking a later message in the same partition will move the committed offset past the failed one, so on restart the failed message will not be re-read. Effectively, log-and-continue silently skips the bad message." -->

- **Retrying immediately** could cause an infinite loop if the error is permanent (bad data, schema mismatch).
- **Dead-letter queues** (DLQs) are the production answer: send failed messages to a separate topic for manual inspection. We skip this to keep the code simple.
- **Blocking** on the failed message would halt processing of all subsequent messages in that partition, which is usually worse than skipping one.

<!-- [COPY EDIT] "Dead-letter queues" — hyphenated open compound modifier; noun is "dead-letter queue" (DLQ). Consistent with industry usage. Good. -->

In a production system, you would add retry logic (with backoff) and a dead-letter topic. For learning purposes, log-and-continue is sufficient.

---

## Event Routing

The `handleEvent` function deserializes the message and routes by event type:

```go
func handleEvent(ctx context.Context, svc AvailabilityUpdater, data []byte) error {
    var event reservationEvent
    if err := json.Unmarshal(data, &event); err != nil {
        return fmt.Errorf("unmarshal event: %w", err)
    }

    bookID, err := uuid.Parse(event.BookID)
    if err != nil {
        return fmt.Errorf("parse book ID: %w", err)
    }

    var delta int
    switch event.EventType {
    case "reservation.created":
        delta = -1
    case "reservation.returned", "reservation.expired":
        delta = 1
    default:
        slog.WarnContext(ctx, "unknown event type", "event_type", event.EventType)
        return nil
    }

    _, err = svc.UpdateAvailability(ctx, bookID, delta)
    return err
}
```

The `reservationEvent` struct mirrors the producer's `ReservationEvent` -- but it only includes the fields the consumer needs:

<!-- [LINE EDIT] "but it only includes the fields the consumer needs" → "but it includes only the fields the consumer needs" — Places "only" next to "the fields" (what it modifies). CMOS 5.184 on limiting modifiers. -->

```go
type reservationEvent struct {
    EventType string `json:"event_type"`
    BookID    string `json:"book_id"`
}
```

This is intentional. The consumer does not need the reservation ID, user ID, or timestamp to update availability. By defining a minimal struct, the consumer is resilient to the producer adding new fields -- `json.Unmarshal` ignores unknown fields by default.

The routing logic is a simple switch:

<!-- [LINE EDIT] "is a simple switch" → "is a switch" — Drop "simple"; the code shows it is straightforward. -->

- `reservation.created` -> decrement availability (delta = -1)
- `reservation.returned` or `reservation.expired` -> increment availability (delta = +1)
- Unknown event types -> log a warning and return nil (no error)

<!-- [COPY EDIT] ASCII arrow `->` in prose again — Unicode `→` recommended for consistency with diagrams. -->

Returning `nil` for unknown events is important. If the reservation service starts publishing a new event type (say, `reservation.extended`), the catalog consumer should not crash -- it should ignore events it does not understand. This is the **tolerant reader** pattern: be liberal in what you accept.

<!-- [STRUCTURAL] Explicit naming of the tolerant-reader pattern (with Fowler link in references) — good pedagogy. Keep. -->

---

## Updating Availability

The consumer calls `svc.UpdateAvailability(ctx, bookID, delta)`, which runs this SQL through GORM:

```go
// services/catalog/internal/repository/book.go

func (r *BookRepository) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
    result := r.db.WithContext(ctx).
        Model(&model.Book{}).
        Where("id = ? AND available_copies + ? >= 0", id, delta).
        Update("available_copies", gorm.Expr("available_copies + ?", delta))
    if result.Error != nil {
        return result.Error
    }
    if delta >= 0 && result.RowsAffected == 0 {
        return model.ErrBookNotFound
    }
    return nil
}
```

The `WHERE available_copies + ? >= 0` clause is a database-level guard against negative counts. This is important for two reasons:

1. **Concurrent decrements.** If two `reservation.created` events for the same book are processed simultaneously (from different partitions, or after a rebalance), the SQL guard ensures one of them is a no-op rather than creating a negative count.

<!-- [STRUCTURAL] Minor factual nuance: the producer uses `event.BookID` as the partition key (section 7.1), so two events for the *same book* will land in the *same partition* and be processed *sequentially* by one consumer. Concurrent decrements for the same book via the Kafka path are therefore prevented by partition-key design, not only by the SQL guard. The guard still matters for (a) the race with 7.2's synchronous `UpdateAvailability` decrement happening in parallel with a consumer run, and (b) re-delivery on rebalance. Consider adding one sentence: "Note: key-based partitioning already serializes events for the same book across the Kafka path, so the guard is mostly a safety net for races between the sync decrement (7.2) and the consumer, plus redelivery on rebalance." -->

2. **Duplicate events.** Since Kafka provides at-least-once delivery, the same event might be processed twice. For decrements, the guard prevents double-counting below zero. For increments, there is no guard -- a duplicate `reservation.returned` could increment the count beyond `total_copies`. In a production system, you would track processed event IDs to prevent this.

<!-- [COPY EDIT] "at-least-once" — hyphenated compound. Consistent with 7.1. Good. -->
<!-- [STRUCTURAL] This paragraph is more accurate than the corresponding passage in 7.1 ("a duplicate increment is harmless"). Reconcile 7.1 with this wording, not the other way around. -->

The `delta >= 0 && result.RowsAffected == 0` check returns `ErrBookNotFound` only for positive deltas (returns/expirations). For negative deltas (reservations), zero affected rows could mean either "book not found" or "guard prevented negative count" -- the code treats both the same way (silently does nothing). This is a simplification noted in the code comments.

<!-- [STRUCTURAL] Good callout about the ambiguity. Keep. -->
<!-- [LINE EDIT] "treats both the same way (silently does nothing)" → "treats both the same way: silently does nothing." — Colon is cleaner than parentheses. -->

---

## Trace Context Propagation

The consumer extracts trace context from Kafka message headers using a `consumerHeaderCarrier` adapter:

```go
type consumerHeaderCarrier []*sarama.RecordHeader

func (c consumerHeaderCarrier) Get(key string) string {
    for _, h := range c {
        if string(h.Key) == key {
            return string(h.Value)
        }
    }
    return ""
}

func (c consumerHeaderCarrier) Set(key, value string) {
    // Consumer carrier is read-only; Set is a no-op.
}

func (c consumerHeaderCarrier) Keys() []string {
    keys := make([]string, len(c))
    for i, h := range c {
        keys[i] = string(h.Key)
    }
    return keys
}
```

This type satisfies the `propagation.TextMapCarrier` interface, which requires `Get`, `Set`, and `Keys` methods. The `Set` method is a no-op because the consumer only reads headers, never writes them.

The type definition `consumerHeaderCarrier []*sarama.RecordHeader` is a **named type over a slice**. This is a Go pattern for adding methods to existing types without creating a wrapper struct. You cannot add methods to `[]*sarama.RecordHeader` directly (you can only add methods to types defined in the same package), but you can define a new type based on it.

<!-- [STRUCTURAL] Great teaching moment for the target reader (coming from Java/Kotlin where extension methods work differently). Keep. -->

On the producer side, the equivalent adapter wraps a `*sarama.ProducerMessage` (a struct, not a slice) because the producer needs to append headers to the message.

---

## Running the Consumer

In the catalog service's `main.go`, the consumer runs as a background goroutine alongside the gRPC server. The typical pattern is:

```go
go func() {
    if err := consumer.Run(ctx, brokers, "reservations", catalogSvc); err != nil {
        slog.Error("consumer exited", "error", err)
    }
}()
```

The `Run` function blocks until `ctx` is cancelled (application shutdown). The gRPC server blocks the main goroutine. When the process receives a shutdown signal, the context is cancelled, which causes both the consumer and the gRPC server to stop.

<!-- [COPY EDIT] "cancelled" — spelling consistency. -->

This is the **co-located consumer** pattern: the consumer runs inside the same process as the service it updates. The alternative is a standalone consumer process. Co-location is simpler (one deployment, shared database connection) but means the consumer's load competes with the gRPC server's load. For our traffic levels, this is not a concern.

<!-- [STRUCTURAL] Good pattern-naming. Consider one more sentence noting the operational implication: "If you later scale the catalog horizontally, every replica becomes a group member — Kafka's rebalancer will distribute partitions automatically." That anticipates a common follow-up question. -->
<!-- [COPY EDIT] "co-located" — hyphenated compound modifier; consistent with CMOS 7.89. Good. -->

---

## Comparison with Spring Kafka

In Spring Kafka, the equivalent consumer would look something like this:

```java
@Component
public class ReservationEventConsumer {

    private final CatalogService catalogService;

    @KafkaListener(topics = "reservations", groupId = "catalog-availability-updater")
    public void handle(ConsumerRecord<String, String> record) {
        ReservationEvent event = objectMapper.readValue(record.value(), ReservationEvent.class);
        int delta = switch (event.type()) {
            case "reservation.created" -> -1;
            case "reservation.returned", "reservation.expired" -> 1;
            default -> 0;
        };
        if (delta != 0) {
            catalogService.updateAvailability(UUID.fromString(event.bookId()), delta);
        }
    }
}
```

<!-- [STRUCTURAL] The Java snippet uses Java 17+ switch expressions, matching what a reader with a modern Spring Boot background would expect. Good. -->
<!-- [COPY EDIT] Java code: `private final CatalogService catalogService;` with no constructor shown — conventional Lombok/Spring readers will fill in a `@RequiredArgsConstructor` or similar, but for a technical book snippet, consider adding a one-line comment or the constructor so the code compiles as-shown. Minor. -->

Spring handles consumer group creation, the consume loop, offset commits, deserialization, and error handling behind the annotation. The Go version requires you to write all of that. The upside is that every aspect of the behavior is visible in your code -- there are no `@EnableKafka` configuration classes, no `ConsumerFactory` beans, no `ConcurrentKafkaListenerContainerFactory` to customize. If something goes wrong, you debug your code, not the framework.

<!-- [LINE EDIT] "every aspect of the behavior is visible in your code" → "every aspect of the behavior lives in your code" — Slightly stronger; "visible" suggests inspection, "lives" suggests ownership. Optional. -->

---

## Exercises

1. **Add retry logic.** Modify `ConsumeClaim` to retry failed messages up to 3 times with exponential backoff (1s, 2s, 4s). After 3 failures, log the message payload and move on. Think about what happens if the backoff blocks the goroutine -- does it affect other messages in the partition?

<!-- [COPY EDIT] "3 times" / "3 failures" — technical numerals, OK. (CMOS 9.2 allows numerals for measurements and technical counts.) -->
<!-- [COPY EDIT] "1s, 2s, 4s" — numerals with units, no space. Common in tech prose. Could be written "1, 2, and 4 seconds" for formal prose. Keep as is. -->

2. **Dead-letter topic.** Extend the consumer to publish failed messages to a `reservations.dlq` topic after exhausting retries. You will need a Sarama `SyncProducer` alongside the consumer.

3. **Test handleEvent.** Write a unit test for the `handleEvent` function. Create a mock `AvailabilityUpdater` and verify that: (a) `reservation.created` calls `UpdateAvailability` with delta -1, (b) `reservation.returned` calls with delta +1, (c) an unknown event type returns nil without calling `UpdateAvailability`, (d) malformed JSON returns an error.

<!-- [COPY EDIT] "delta -1" / "delta +1" — consider formatting as "delta of -1", "delta of +1", or code-voicing the values. Minor. -->

4. **Consumer lag monitoring.** Kafka consumer lag is the difference between the latest offset in a partition and the consumer's committed offset. High lag means the consumer is falling behind. Describe how you would expose this metric. (Hint: Sarama's `ConsumerGroupSession` exposes `HighWaterMarkOffset()` per partition.)

<!-- [COPY EDIT] Please verify: `HighWaterMarkOffset()` is a method on `ConsumerGroupClaim`, not on `ConsumerGroupSession`, in Sarama. Confirm against the Sarama docs and correct the hint if needed. -->

5. **Multiple event types on one topic.** Our `reservations` topic carries three event types. An alternative is one topic per event type (`reservation-created`, `reservation-returned`, `reservation-expired`). What are the tradeoffs? Consider ordering guarantees, consumer simplicity, and topic proliferation.

<!-- [STRUCTURAL] Well-chosen exercise — forces the reader to reason about the key-partitioning guarantee, which the book has just taught. Keep. -->

---

## References

[^1]: [IBM/sarama ConsumerGroup example](https://github.com/IBM/sarama/blob/main/examples/consumergroup/main.go) -- Official Sarama example for consumer groups.
[^2]: [Kafka Consumer Group Protocol](https://kafka.apache.org/documentation/#consumerconfigs) -- Kafka documentation on consumer group configuration and rebalancing.
[^3]: [Martin Fowler -- Tolerant Reader](https://martinfowler.com/bliki/TolerantReader.html) -- The pattern of ignoring unknown fields and event types.
[^4]: [OpenTelemetry Go -- Propagation](https://opentelemetry.io/docs/languages/go/propagation/) -- How to propagate trace context across process boundaries.
[^5]: [Confluent -- Dead Letter Queue](https://www.confluent.io/blog/kafka-connect-deep-dive-error-handling-dead-letter-queues/) -- Pattern for handling unprocessable messages.

<!-- [COPY EDIT] Please verify: URLs
 - https://github.com/IBM/sarama/blob/main/examples/consumergroup/main.go — confirm file path still exists on main.
 - https://kafka.apache.org/documentation/#consumerconfigs — confirm anchor still valid.
 - https://opentelemetry.io/docs/languages/go/propagation/ — confirm URL path structure (OTel reorganized docs in 2024–25).
 - https://www.confluent.io/blog/kafka-connect-deep-dive-error-handling-dead-letter-queues/ — Confluent frequently re-slugs blog posts.
-->
<!-- [COPY EDIT] " -- " in reference entries — standardize to em dash or period. (CMOS 6.85) -->
<!-- [FINAL] Verify file-path comments in code blocks still match the current repo: `services/catalog/internal/consumer/consumer.go`, `services/catalog/internal/repository/book.go`. -->
