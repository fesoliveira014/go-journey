# 7.3 Kafka Consumer

Section 7.1 covered the theory of event-driven architecture. This section gets into the mechanical details: how the Catalog Service consumes reservation events from Kafka using the Sarama library's consumer group API.

If you have used Spring Kafka, consumer setup is a matter of annotating a method with `@KafkaListener` and letting the framework handle group management, deserialization, and offset commits. In Go with Sarama, you implement an interface and manage the consume loop explicitly. More code, but nothing is hidden.

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

1. **Setup** is called at the start of a new consumer group session—after a rebalance assigns partitions to this consumer. Use it to initialize resources if needed.
2. **ConsumeClaim** is called once per assigned partition. It runs in its own goroutine. You read messages from the claim's channel and process them.
3. **Cleanup** is called when the session ends (before the next rebalance). Use it to flush buffers or release resources.

This three-phase lifecycle maps roughly to the `ConsumerRebalanceListener` in the Java Kafka client, where `onPartitionsAssigned` and `onPartitionsRevoked` serve the same purpose as Setup and Cleanup.

Our implementation keeps Setup and Cleanup empty—we have no session-scoped resources to manage:

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

This is a **role interface**—it describes the one capability the consumer needs from the Catalog Service. The full `CatalogService` has many methods (Create, Update, Delete, List), but the consumer only calls `UpdateAvailability`. Defining a narrow interface means the consumer is decoupled from the rest of the Catalog Service. In tests, you mock one method, not twenty.

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

The configuration, piece by piece:

**`BalanceStrategyRoundRobin`** controls how partitions are distributed among consumers in the group during a rebalance. Round-robin assigns them evenly. Other strategies exist (`Range`, `Sticky`), but round-robin is the simplest and works well for most cases.

**`OffsetOldest`** means that when the consumer group has no previously committed offset (first startup, or after offset expiry), it starts reading from the oldest available message. The alternative is `OffsetNewest`, which skips existing messages and reads only new ones. We use `OffsetOldest` so that if the Catalog Service was down while reservations were being made, it catches up on all missed events when it restarts.

**The `for` loop.** `group.Consume` blocks until the session ends (due to a rebalance or context cancellation). When it returns, we check if the context is done. If not, we loop back and rejoin—this handles rebalances gracefully. If the context is cancelled (application shutdown), we return.

This pattern—`for { Consume(); if ctx.Err() != nil { return } }`—is idiomatic Sarama. The `Consume` call manages the entire lifecycle: joining the group, receiving partition assignments, calling Setup/ConsumeClaim/Cleanup, and then returning when the session ends.

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

Step by step:

1. **Get the session context.** `session.Context()` returns a context that is cancelled when the session ends (rebalance or shutdown). This is your cancellation signal.

2. **Range over messages.** `claim.Messages()` is a Go channel. The `for range` loop reads messages until the channel closes (session end). This is a clean, idiomatic pattern—no polling, no sleep loops.

3. **Extract trace context.** The producer injected OpenTelemetry headers into the Kafka message. We extract them here so the consumer's span is linked to the producer's trace. This gives you a single distributed trace from the HTTP request through the Reservation Service, through Kafka, into the catalog consumer.

4. **Handle the event.** `handleEvent` deserializes and processes the message. If it fails, we log and continue—we do not retry, and we do not mark the message.

5. **Mark the message.** `session.MarkMessage(msg, "")` tells Sarama this message has been processed. Sarama periodically commits marked offsets to Kafka in the background (controlled by `Consumer.Offsets.AutoCommit.Interval`, default 1 second).

### The "Log and Continue" Error Strategy

When `handleEvent` fails, the consumer logs the error and moves on to the next message. The failed message is *not* marked, so it will not be committed—but it will not be retried either (not until the next rebalance or restart). This is a pragmatic choice:

- **Retrying immediately** could cause an infinite loop if the error is permanent (bad data, schema mismatch).
- **Dead-letter queues** (DLQs) are the production answer: send failed messages to a separate topic for manual inspection. We skip this to keep the code simple.
- **Blocking** on the failed message would halt processing of all subsequent messages in that partition, which is usually worse than skipping one.

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

The `reservationEvent` struct mirrors the producer's `ReservationEvent`—but it includes only the fields the consumer needs:

```go
type reservationEvent struct {
    EventType string `json:"event_type"`
    BookID    string `json:"book_id"`
}
```

This is intentional. The consumer does not need the reservation ID, user ID, or timestamp to update availability. By defining a minimal struct, the consumer is resilient to the producer adding new fields—`json.Unmarshal` ignores unknown fields by default.

The routing logic is a switch:

- `reservation.created` -> decrement availability (delta = -1)
- `reservation.returned` or `reservation.expired` -> increment availability (delta = +1)
- Unknown event types -> log a warning and return nil (no error)

Returning `nil` for unknown events is important. If the Reservation Service starts publishing a new event type (say, `reservation.extended`), the catalog consumer should not crash—it should ignore events it does not understand. This is the **tolerant reader** pattern: be liberal in what you accept.

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

2. **Duplicate events.** Since Kafka provides at-least-once delivery, the same event might be processed twice. For decrements, the guard prevents double-counting below zero. For increments, there is no guard—a duplicate `reservation.returned` could increment the count beyond `total_copies`. In a production system, you would track processed event IDs to prevent this.

The `delta >= 0 && result.RowsAffected == 0` check returns `ErrBookNotFound` only for positive deltas (returns/expirations). For negative deltas (reservations), zero affected rows could mean either "book not found" or "guard prevented negative count"—the code treats both the same way: silently does nothing. This is a simplification noted in the code comments.

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

On the producer side, the equivalent adapter wraps a `*sarama.ProducerMessage` (a struct, not a slice) because the producer needs to append headers to the message.

---

## Running the Consumer

In the Catalog Service's `main.go`, the consumer runs as a background goroutine alongside the gRPC server. The typical pattern is:

```go
go func() {
    if err := consumer.Run(ctx, brokers, "reservations", catalogSvc); err != nil {
        slog.Error("consumer exited", "error", err)
    }
}()
```

The `Run` function blocks until `ctx` is cancelled (application shutdown). The gRPC server blocks the main goroutine. When the process receives a shutdown signal, the context is cancelled, which causes both the consumer and the gRPC server to stop.

This is the **co-located consumer** pattern: the consumer runs inside the same process as the service it updates. The alternative is a standalone consumer process. Co-location is simpler (one deployment, shared database connection) but means the consumer's load competes with the gRPC server's load. For our traffic levels, this is not a concern.

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

Spring handles consumer group creation, the consume loop, offset commits, deserialization, and error handling behind the annotation. The Go version requires you to write all of that. The upside is that every aspect of the behavior lives in your code—there are no `@EnableKafka` configuration classes, no `ConsumerFactory` beans, no `ConcurrentKafkaListenerContainerFactory` to customize. If something goes wrong, you debug your code, not the framework.

---

## Exercises

1. **Add retry logic.** Modify `ConsumeClaim` to retry failed messages up to 3 times with exponential backoff (1s, 2s, 4s). After 3 failures, log the message payload and move on. Think about what happens if the backoff blocks the goroutine—does it affect other messages in the partition?

2. **Dead-letter topic.** Extend the consumer to publish failed messages to a `reservations.dlq` topic after exhausting retries. You will need a Sarama `SyncProducer` alongside the consumer.

3. **Test handleEvent.** Write a unit test for the `handleEvent` function. Create a mock `AvailabilityUpdater` and verify that: (a) `reservation.created` calls `UpdateAvailability` with delta -1, (b) `reservation.returned` calls with delta +1, (c) an unknown event type returns nil without calling `UpdateAvailability`, (d) malformed JSON returns an error.

4. **Consumer lag monitoring.** Kafka consumer lag is the difference between the latest offset in a partition and the consumer's committed offset. High lag means the consumer is falling behind. Describe how you would expose this metric. (Hint: Sarama's `ConsumerGroupClaim` exposes `HighWaterMarkOffset()` per partition.)

5. **Multiple event types on one topic.** Our `reservations` topic carries three event types. An alternative is one topic per event type (`reservation-created`, `reservation-returned`, `reservation-expired`). What are the trade-offs? Consider ordering guarantees, consumer simplicity, and topic proliferation.

---

## References

[^1]: [IBM/sarama ConsumerGroup example](https://github.com/IBM/sarama/blob/main/examples/consumergroup/main.go)—Official Sarama example for consumer groups.
[^2]: [Kafka Consumer Group Protocol](https://kafka.apache.org/documentation/#consumerconfigs)—Kafka documentation on consumer group configuration and rebalancing.
[^3]: [Martin Fowler—Tolerant Reader](https://martinfowler.com/bliki/TolerantReader.html)—The pattern of ignoring unknown fields and event types.
[^4]: [OpenTelemetry Go—Propagation](https://opentelemetry.io/docs/languages/go/propagation/)—How to propagate trace context across process boundaries.
[^5]: [Confluent—Dead Letter Queue](https://www.confluent.io/blog/kafka-connect-deep-dive-error-handling-dead-letter-queues/)—Pattern for handling unprocessable messages.
