# 11.4 Kafka Testing

<!-- [STRUCTURAL] Section is well-structured: four motivating gaps → setup → producer test → consumer test → capturing indexer pattern → gotchas → summary. The four-gap opening is effective and parallels the three-failure opening from the chapter index. -->
<!-- [LINE EDIT] "The unit tests you wrote for the catalog consumer (`consumer_test.go`) are correct and fast. They call `handleEvent` directly with hand-crafted byte slices, check that the right `UpdateAvailability` delta is applied, and verify that unknown event types are silently ignored. They run in under a millisecond each." 49 words across three sentences; fine. -->
The unit tests you wrote for the catalog consumer (`consumer_test.go`) are correct and fast. They call `handleEvent` directly with hand-crafted byte slices, check that the right `UpdateAvailability` delta is applied, and verify that unknown event types are silently ignored. They run in under a millisecond each.

<!-- [LINE EDIT] "But they leave four things completely untested:" — fine. -->
But they leave four things completely untested:

<!-- [LINE EDIT] Bullet 1: "Sarama's consumer group implementation negotiates with a Kafka coordinator before any message is delivered. If the broker address is wrong, the Kafka version is mismatched, or the group ID clashes with a running instance, the consumer never starts. Unit tests cannot expose this." — fine. -->
<!-- [COPY EDIT] "Sarama" — proper noun; correctly capitalized. -->
1. **Consumer group joining.** Sarama's consumer group implementation negotiates with a Kafka coordinator before any message is delivered. If the broker address is wrong, the Kafka version is mismatched, or the group ID clashes with a running instance, the consumer never starts. Unit tests cannot expose this.

<!-- [LINE EDIT] Bullet 2: "`session.MarkMessage` in `ConsumeClaim` advances the consumer group's committed offset. If you forget to call it — or call it before handling is complete — messages are reprocessed on restart. Unit tests cannot verify commit timing." — fine. -->
2. **Offset management.** `session.MarkMessage` in `ConsumeClaim` advances the consumer group's committed offset. If you forget to call it — or call it before handling is complete — messages are reprocessed on restart. Unit tests cannot verify commit timing.

<!-- [LINE EDIT] Bullet 3 is 51 words and has two dependent clauses about sarama header key serialization. Consider tightening: "**Header propagation.** The reservation publisher injects OpenTelemetry trace context into Kafka message headers via `headerCarrier`; the catalog consumer extracts it in `ConsumeClaim`. If sarama ever serializes header keys differently than the consumer expects, traces break and monitoring silently degrades. Only a test through the real serialization path catches this." -->
<!-- [COPY EDIT] "OpenTelemetry" — capitalize product name; correct. -->
3. **Header propagation.** The reservation publisher injects OpenTelemetry trace context into Kafka message headers via `headerCarrier`. The catalog consumer extracts that context in `ConsumeClaim` before starting a new span. If the header key encoding changes — for example, if sarama serializes header keys differently than expected — the trace is broken and monitoring silently degrades. Only a test that goes through the real serialization path can catch this.

<!-- [LINE EDIT] Bullet 4: "The serialization contract between producer and consumer is implicit: JSON with specific field names, a specific key encoding, a specific topic name. Both sides can change independently. A test that publishes through the actual `kafka.Publisher` and consumes through the actual `consumer.Run` detects any divergence immediately." — fine. -->
4. **Real Kafka interaction.** The serialization contract between producer and consumer is implicit: JSON with specific field names, a specific key encoding, a specific topic name. Both sides can change independently. A test that publishes through the actual `kafka.Publisher` and consumes through the actual `consumer.Run` detects any divergence immediately.

<!-- [LINE EDIT] "If you are coming from Spring, this is analogous to the difference between testing a `@KafkaListener` method in isolation versus using Spring Kafka's `EmbeddedKafka` to run the full producer-to-consumer path." — fine. -->
<!-- [LINE EDIT] "The unit test covers business logic; the integration test covers the wiring." — tight. -->
If you are coming from Spring, this is analogous to the difference between testing a `@KafkaListener` method in isolation versus using Spring Kafka's `EmbeddedKafka` to run the full producer-to-consumer path. The unit test covers business logic; the integration test covers the wiring.

---

## Setting up a Kafka testcontainer

<!-- [COPY EDIT] Heading: "testcontainer" → "Testcontainer" is debatable; here "a Kafka testcontainer" reads as a common noun (an instance of a testcontainer), so lowercase is arguably acceptable. But chapter uses "Testcontainers" (capitalized) for the brand. For consistency consider "Setting up a Kafka Testcontainer" or rewrite as "Setting up a Kafka container with Testcontainers". -->
<!-- [LINE EDIT] "The `testcontainers-go` Kafka module[^1] starts a Confluent local Kafka image in Docker. Because Kafka requires a running ZooKeeper (or KRaft controller), starting the container takes roughly 10–15 seconds — noticeably slower than a PostgreSQL container. The standard mitigation is to start Kafka once in `TestMain` and share it across all tests in a suite." 55 words across three sentences; fine. -->
<!-- [COPY EDIT] "Confluent local Kafka image" — the image name is `confluentinc/confluent-local`; product is "Confluent Local". Capitalize. -->
<!-- [COPY EDIT] "ZooKeeper" — capitalized correctly. "KRaft" — capitalized correctly. -->
The `testcontainers-go` Kafka module[^1] starts a Confluent local Kafka image in Docker. Because Kafka requires a running ZooKeeper (or KRaft controller), starting the container takes roughly 10–15 seconds — noticeably slower than a PostgreSQL container. The standard mitigation is to start Kafka once in `TestMain` and share it across all tests in a suite.

<!-- [LINE EDIT] "Create a helper file in your integration test package:" — fine. -->
Create a helper file in your integration test package:

```go
//go:build integration

package kafka_test

import (
    "context"
    "testing"

    kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"
)

func setupKafka(t *testing.T) []string {
    t.Helper()
    ctx := context.Background()

    container, err := kafkatc.Run(ctx, "confluentinc/confluent-local:7.6.0")
    if err != nil {
        t.Fatalf("start kafka container: %v", err)
    }
    t.Cleanup(func() { container.Terminate(context.Background()) })

    brokers, err := container.Brokers(ctx)
    if err != nil {
        t.Fatalf("get kafka brokers: %v", err)
    }
    return brokers
}
```

<!-- [COPY EDIT] Please verify: `confluentinc/confluent-local:7.6.0` image tag. Confirm current supported tag for 2026-04. A newer tag may exist; if so, consider noting "pin to a recent stable tag". -->

<!-- [LINE EDIT] "`kafkatc.Run` returns a running container with the broker's listener exposed on a dynamic port. `container.Brokers` resolves to something like `["localhost:32801"]` — the actual port varies per run. Because ports are ephemeral, tests never conflict with each other or with production services." — fine. -->
`kafkatc.Run` returns a running container with the broker's listener exposed on a dynamic port. `container.Brokers` resolves to something like `["localhost:32801"]` — the actual port varies per run. Because ports are ephemeral, tests never conflict with each other or with production services.

<!-- [LINE EDIT] "The `//go:build integration` tag at the top of the file ensures this code is only compiled when you explicitly run:" — fine. -->
The `//go:build integration` tag at the top of the file ensures this code is only compiled when you explicitly run:

```bash
go test -tags integration ./...
```

<!-- [LINE EDIT] "Without the tag, the testcontainers import is never compiled, so your standard `go test ./...` invocation remains fast and free of Docker dependencies." — fine. -->
Without the tag, the testcontainers import is never compiled, so your standard `go test ./...` invocation remains fast and free of Docker dependencies.

### Sharing a container across tests with TestMain

<!-- [LINE EDIT] "Starting a new container for every test function would make your suite unbearably slow. The idiomatic Go approach is to use `TestMain` to start infrastructure once:" — fine. -->
Starting a new container for every test function would make your suite unbearably slow. The idiomatic Go approach is to use `TestMain` to start infrastructure once:

```go
//go:build integration

package kafka_test

import (
    "os"
    "testing"
)

var sharedBrokers []string

func TestMain(m *testing.M) {
    // Use a dummy *testing.T just for setup — TestMain has no *testing.T.
    setup := &testing.T{}
    sharedBrokers = setupKafka(setup)
    os.Exit(m.Run())
}
```

<!-- [FINAL] Constructing a bare `&testing.T{}` works but is fragile. `t.Helper()`, `t.Cleanup`, and `t.Fatalf` rely on unexported state. In particular, `t.Fatalf` on a bare `testing.T` calls `runtime.Goexit` on the wrong goroutine, and `t.Cleanup` registers on a T that never gets "finished", so nothing runs. The code is a footgun. Either: (a) use `log.Fatalf` in TestMain paths and clean up manually with `container.Terminate`, or (b) factor `setupKafka` into a helper that returns `(brokers, cleanupFn, error)` and the T-based version wraps it. Strongly recommend revising. -->
<!-- [COPY EDIT] Please verify: `&testing.T{}` usage — the testing package does not document this as supported; in practice `t.Fatalf` on a zero T panics or misbehaves. Pattern appears in some blog posts but is not idiomatic. -->

<!-- [LINE EDIT] "`TestMain` is called before any test function. `m.Run()` executes all tests in the package and returns an exit code. By assigning the broker list to a package-level variable, every test function in the suite can use it without paying the container startup cost more than once." — fine. -->
`TestMain` is called before any test function. `m.Run()` executes all tests in the package and returns an exit code. By assigning the broker list to a package-level variable, every test function in the suite can use it without paying the container startup cost more than once.

<!-- [LINE EDIT] "One limitation: `t.Cleanup` registered in `setupKafka` is attached to the dummy `*testing.T`, not the real test harness. If you need guaranteed cleanup, call `container.Terminate` explicitly at the end of `TestMain` instead." — fine; but see FINAL note above — the limitation is actually a broader footgun. Strengthen the warning. -->
One limitation: `t.Cleanup` registered in `setupKafka` is attached to the dummy `*testing.T`, not the real test harness. If you need guaranteed cleanup, call `container.Terminate` explicitly at the end of `TestMain` instead.

---

## Testing the reservation service producer

<!-- [LINE EDIT] "The reservation service's `kafka.Publisher` wraps a `sarama.SyncProducer`. Its `Publish` method serializes a `ReservationEvent` to JSON, sets the message key to `event.BookID`, and injects the OTel trace context into message headers via `headerCarrier`. That is three things to verify." — fine. -->
<!-- [COPY EDIT] "OTel" — informal short form; acceptable if introduced. The full name "OpenTelemetry" appeared above; consider introducing "OTel" parenthetically on first use: "OpenTelemetry (OTel)". -->
The reservation service's `kafka.Publisher` wraps a `sarama.SyncProducer`. Its `Publish` method serializes a `ReservationEvent` to JSON, sets the message key to `event.BookID`, and injects the OTel trace context into message headers via `headerCarrier`. That is three things to verify.

<!-- [LINE EDIT] "The verification approach is asymmetric: produce through the real publisher, then read back using sarama's lower-level partition consumer API. A partition consumer is simpler than a consumer group for this purpose — it does not join a group, does not negotiate offsets with a coordinator, and does not require group rebalance. It just reads raw messages from a topic partition. This is the Kafka equivalent of reading a single message off a queue for assertion purposes." — four sentences; read well. -->
<!-- [COPY EDIT] "sarama's" — lower-case "sarama" when referring to the package name; acceptable inline. -->
The verification approach is asymmetric: produce through the real publisher, then read back using sarama's lower-level partition consumer API. A partition consumer is simpler than a consumer group for this purpose — it does not join a group, does not negotiate offsets with a coordinator, and does not require group rebalance. It just reads raw messages from a topic partition. This is the Kafka equivalent of reading a single message off a queue for assertion purposes.

```go
//go:build integration

package kafka_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/IBM/sarama"

    kafkapub "github.com/fesoliveira014/library-system/services/reservation/internal/kafka"
    "github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

func TestPublisher_RoundTrip(t *testing.T) {
    brokers := sharedBrokers
    topic := "reservations-publisher-test"

    pub, err := kafkapub.NewPublisher(brokers, topic)
    if err != nil {
        t.Fatalf("create publisher: %v", err)
    }
    defer pub.Close()

    event := service.ReservationEvent{
        Type:          "reservation.created",
        ReservationID: "res-001",
        UserID:        "user-001",
        BookID:        "book-abc",
        Timestamp:     time.Now().UTC().Truncate(time.Second),
    }

    if err := pub.Publish(context.Background(), event); err != nil {
        t.Fatalf("publish event: %v", err)
    }

    // Read back using a simple partition consumer.
    config := sarama.NewConfig()
    config.Consumer.Offsets.Initial = sarama.OffsetOldest

    consumer, err := sarama.NewConsumer(brokers, config)
    if err != nil {
        t.Fatalf("create sarama consumer: %v", err)
    }
    defer consumer.Close()

    pc, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
    if err != nil {
        t.Fatalf("consume partition: %v", err)
    }
    defer pc.Close()

    select {
    case msg := <-pc.Messages():
        // Verify the message key is the book ID.
        if string(msg.Key) != event.BookID {
            t.Errorf("key: want %q, got %q", event.BookID, string(msg.Key))
        }

        // Verify the JSON payload round-trips correctly.
        var decoded service.ReservationEvent
        if err := json.Unmarshal(msg.Value, &decoded); err != nil {
            t.Fatalf("unmarshal message: %v", err)
        }
        if decoded.Type != event.Type {
            t.Errorf("event type: want %q, got %q", event.Type, decoded.Type)
        }
        if decoded.BookID != event.BookID {
            t.Errorf("book ID: want %q, got %q", event.BookID, decoded.BookID)
        }

        // Verify OTel trace headers are present.
        // The publisher calls otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg})
        // which writes headers like "traceparent" and "tracestate".
        headerKeys := make(map[string]bool)
        for _, h := range msg.Headers {
            headerKeys[string(h.Key)] = true
        }
        if !headerKeys["traceparent"] {
            t.Error("expected traceparent OTel header to be present")
        }

    case <-time.After(10 * time.Second):
        t.Fatal("timed out waiting for published message")
    }
}
```

<!-- [COPY EDIT] Please verify: Sarama (IBM/sarama) is currently in maintenance mode per project README (late 2024); ch10 already notes this per the recent commit ("note Sarama maintenance status and recommend evaluating franz-go"). This section does not mention it. Add a parallel note. -->

<!-- [LINE EDIT] "A few points worth noting." — sentence fragment as signpost; acceptable. -->
A few points worth noting.

<!-- [LINE EDIT] "`sarama.OffsetOldest` on the partition consumer means "start from offset 0". Without this, sarama defaults to `OffsetNewest` and the consumer would only see messages published after it connects — which, given our produce-then-consume ordering, would mean it sees nothing." 41 words; acceptable. -->
`sarama.OffsetOldest` on the partition consumer means "start from offset 0". Without this, sarama defaults to `OffsetNewest` and the consumer would only see messages published after it connects — which, given our produce-then-consume ordering, would mean it sees nothing.

<!-- [LINE EDIT] "The topic is not explicitly created. When the publisher sends the first message, sarama's producer creates the topic with default settings (one partition, replication factor 1). This is fine for tests. In production you would pre-create topics with explicit partition counts and retention policies." — fine. -->
<!-- [COPY EDIT] "pre-create" — CMOS 7.85: "pre-" typically closed ("precreate") unless ambiguous. "Pre-create" is acceptable given the "c" clash; keep. -->
The topic is not explicitly created. When the publisher sends the first message, sarama's producer creates the topic with default settings (one partition, replication factor 1). This is fine for tests. In production you would pre-create topics with explicit partition counts and retention policies.

<!-- [LINE EDIT] "The OTel header check is deliberately minimal: it only verifies that `traceparent` is present, not that it contains a valid trace ID. A full trace ID assertion would require starting a real OTel SDK and a trace exporter in the test, which is overkill here. The presence of the header confirms that the propagation path — `Inject` in the publisher, `Extract` in the consumer — is not broken." 61 words across three sentences; acceptable. -->
<!-- [COPY EDIT] "trace ID" — two words; keep consistent throughout. -->
The OTel header check is deliberately minimal: it only verifies that `traceparent` is present, not that it contains a valid trace ID. A full trace ID assertion would require starting a real OTel SDK and a trace exporter in the test, which is overkill here. The presence of the header confirms that the propagation path — `Inject` in the publisher, `Extract` in the consumer — is not broken.

---

## Testing the catalog consumer end-to-end

<!-- [LINE EDIT] "The catalog consumer test is more ambitious. It needs to verify the full chain: a `reservation.created` event on Kafka causes `available_copies` to decrease by one in PostgreSQL. That requires both a Kafka container and a PostgreSQL container." — fine. -->
The catalog consumer test is more ambitious. It needs to verify the full chain: a `reservation.created` event on Kafka causes `available_copies` to decrease by one in PostgreSQL. That requires both a Kafka container and a PostgreSQL container.

<!-- [LINE EDIT] "You already have a PostgreSQL setup from section 11.2. Reuse the same `setupPostgres` helper. The test then needs to:" — fine. -->
<!-- [COPY EDIT] "section 11.2" — inconsistent capitalization: 11.3 uses "Section 11.2" (capital S). Pick one. CMOS 8.180 accepts both; project consistency matters. -->
You already have a PostgreSQL setup from section 11.2. Reuse the same `setupPostgres` helper. The test then needs to:

1. Insert a book with 5 available copies.
2. Produce a `reservation.created` event for that book's ID.
3. Start `consumer.Run` in a goroutine.
4. Poll the database until `available_copies` equals 4 or until a timeout.

<!-- [COPY EDIT] Numbered-list: "5 available copies" and "equals 4" — CMOS 9.2 "spell out zero through one hundred in prose". However, in a numbered step list describing data values these are technical/quantitative and numerals are acceptable (CMOS 9.4). Keep. -->

<!-- [LINE EDIT] "Step 4 is the tricky part. `consumer.Run` is asynchronous — it runs in a separate goroutine and delivers results via side effects on the database. You cannot use a channel here because the consumer does not signal when it has processed a message. The standard approach is a polling loop with a timeout." — fine. -->
Step 4 is the tricky part. `consumer.Run` is asynchronous — it runs in a separate goroutine and delivers results via side effects on the database. You cannot use a channel here because the consumer does not signal when it has processed a message. The standard approach is a polling loop with a timeout.

```go
//go:build integration

package consumer_test

import (
    "context"
    "encoding/json"
    "fmt"
    "testing"
    "time"

    "github.com/IBM/sarama"
    "github.com/google/uuid"

    catalogconsumer "github.com/fesoliveira014/library-system/services/catalog/internal/consumer"
    "github.com/fesoliveira014/library-system/services/catalog/internal/model"
    "github.com/fesoliveira014/library-system/services/catalog/internal/repository"
    "github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

func TestConsumer_ReservationCreated_DecreasesAvailability(t *testing.T) {
    brokers := sharedBrokers
    topic := fmt.Sprintf("reservations-consumer-test-%s", t.Name())

    // Produce the event before starting the consumer so the offset is waiting.
    bookID := uuid.New()
    event := map[string]string{
        "event_type": "reservation.created",
        "book_id":    bookID.String(),
    }
    payload, _ := json.Marshal(event)

    config := sarama.NewConfig()
    config.Producer.Return.Successes = true

    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        t.Fatalf("create producer: %v", err)
    }
    defer producer.Close()

    _, _, err = producer.SendMessage(&sarama.ProducerMessage{
        Topic: topic,
        Value: sarama.ByteEncoder(payload),
    })
    if err != nil {
        t.Fatalf("send message: %v", err)
    }

    // Insert a book with 5 available copies into the test database.
    repo := repository.NewBookRepository(sharedDB)
    svc := service.NewCatalogService(repo)

    book, err := repo.Create(context.Background(), &model.Book{
        ID:              bookID,
        Title:           "Integration Test Book",
        Author:          "Test Author",
        ISBN:            fmt.Sprintf("TEST-%s", bookID.String()[:8]),
        TotalCopies:     5,
        AvailableCopies: 5,
    })
    if err != nil {
        t.Fatalf("create book: %v", err)
    }

    // Start the consumer with a unique group ID to avoid offset collisions
    // with other test runs or parallel tests.
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    groupID := fmt.Sprintf("test-%s", t.Name())
    go func() {
        if err := catalogconsumer.Run(ctx, brokers, topic, svc, groupID); err != nil {
            t.Logf("consumer exited: %v", err)
        }
    }()

    // Poll the database until available_copies decreases to 4.
    deadline := time.After(10 * time.Second)
    for {
        select {
        case <-deadline:
            t.Fatal("timed out waiting for availability update")
        case <-time.After(200 * time.Millisecond):
            updated, err := repo.GetByID(context.Background(), book.ID)
            if err != nil {
                t.Fatalf("get book: %v", err)
            }
            if updated.AvailableCopies == 4 {
                cancel()
                return
            }
        }
    }
}
```

<!-- [FINAL] `sharedDB` is referenced but not declared in the code shown. Introduce it in the `TestMain` example alongside `sharedBrokers`, or note in prose: "(`sharedDB` is set up in the same TestMain that initializes `sharedBrokers`)". -->
<!-- [FINAL] The test calls `service.NewCatalogService(repo)` — one argument — but earlier code (11.1, 11.3) uses `service.NewCatalogService(repo, pub)` — two arguments. Is the signature variadic, or is this a divergence? Please verify. -->
<!-- [FINAL] `payload, _ := json.Marshal(event)` silently discards the error; a fixed map literal is unlikely to fail, but the project's other examples check the error. Align. -->

<!-- [LINE EDIT] "The polling loop deserves explanation. `time.After(10 * time.Second)` returns a channel that fires once after 10 seconds. `time.After(200 * time.Millisecond)` returns a new channel each iteration that fires after 200ms. The `select` blocks until one of them is ready. If 10 seconds pass without the condition being met, the test fails. If the condition is met in any 200ms window, the test cancels the consumer and returns. This is the standard Go pattern for asserting eventual consistency without busy-waiting." — six sentences; good cadence. -->
<!-- [COPY EDIT] "200ms" vs "200 Millisecond" — stay in one unit. "200ms" is a compact form; fine in this context. -->
<!-- [COPY EDIT] "busy-waiting" — hyphenated compound (CMOS 7.85/7.81). Correct. -->
The polling loop deserves explanation. `time.After(10 * time.Second)` returns a channel that fires once after 10 seconds. `time.After(200 * time.Millisecond)` returns a new channel each iteration that fires after 200ms. The `select` blocks until one of them is ready. If 10 seconds pass without the condition being met, the test fails. If the condition is met in any 200ms window, the test cancels the consumer and returns. This is the standard Go pattern for asserting eventual consistency without busy-waiting.

<!-- [LINE EDIT] "Do not use `time.Sleep` here. A fixed sleep that is long enough to be reliable is long enough to make your CI noticeably slower. The polling loop with a reasonable timeout gives the same guarantee with far lower average latency — in practice the consumer processes the message in under a second, and the loop will exit in 1–2 iterations." 58 words across three sentences; fine. -->
<!-- [COPY EDIT] "1–2 iterations" — en dash for range; correct (CMOS 6.78). -->
Do not use `time.Sleep` here. A fixed sleep that is long enough to be reliable is long enough to make your CI noticeably slower. The polling loop with a reasonable timeout gives the same guarantee with far lower average latency — in practice the consumer processes the message in under a second, and the loop will exit in 1–2 iterations.

<!-- [LINE EDIT] "Note that `consumer.Run` in the real code has a hardcoded group ID (`"catalog-availability-updater"`). For the consumer to be testable with per-test group IDs, you would need to refactor `Run` to accept the group ID as a parameter. The signature above reflects that refactored version:" — fine. -->
Note that `consumer.Run` in the real code has a hardcoded group ID (`"catalog-availability-updater"`). For the consumer to be testable with per-test group IDs, you would need to refactor `Run` to accept the group ID as a parameter. The signature above reflects that refactored version:

```go
func Run(ctx context.Context, brokers []string, topic string, svc AvailabilityUpdater, groupID string) error
```

<!-- [LINE EDIT] "This is a concrete example of how writing integration tests shapes your API design. The hardcoded group ID was an implementation convenience that becomes an obstacle the moment you want to run two tests simultaneously. Accepting the group ID as a parameter is the right production API anyway — it allows operators to tune the consumer group name without recompiling." — fine. -->
This is a concrete example of how writing integration tests shapes your API design. The hardcoded group ID was an implementation convenience that becomes an obstacle the moment you want to run two tests simultaneously. Accepting the group ID as a parameter is the right production API anyway — it allows operators to tune the consumer group name without recompiling.

---

## Testing the search consumer with a capturing indexer

<!-- [LINE EDIT] "The search consumer (`services/search/internal/consumer`) is structurally identical to the catalog consumer, but its downstream dependency is Meilisearch rather than PostgreSQL. You do not need a Meilisearch container to integration-test the Kafka consumption path. The indexer is an interface:" 43 words; acceptable. -->
<!-- [COPY EDIT] "Meilisearch" — product name with single capital (vendor's current canonical spelling). Correct. -->
<!-- [COPY EDIT] "integration-test" used as a verb — acceptable shorthand but some readers prefer "to test the Kafka consumption path as an integration test". Keep with awareness of audience. -->
The search consumer (`services/search/internal/consumer`) is structurally identical to the catalog consumer, but its downstream dependency is Meilisearch rather than PostgreSQL. You do not need a Meilisearch container to integration-test the Kafka consumption path. The indexer is an interface:

```go
type Indexer interface {
    Upsert(ctx context.Context, doc model.BookDocument) error
    Delete(ctx context.Context, id string) error
}
```

<!-- [LINE EDIT] "You can implement a thread-safe capturing indexer that records calls without touching any external system:" — fine. -->
<!-- [COPY EDIT] "thread-safe" — hyphenated compound; correct. -->
You can implement a thread-safe capturing indexer that records calls without touching any external system:

```go
//go:build integration

package consumer_test

import (
    "context"
    "sync"

    "github.com/fesoliveira014/library-system/services/search/internal/model"
)

type capturingIndexer struct {
    mu       sync.Mutex
    upserted []model.BookDocument
    deleted  []string
}

func (c *capturingIndexer) Upsert(_ context.Context, doc model.BookDocument) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.upserted = append(c.upserted, doc)
    return nil
}

func (c *capturingIndexer) Delete(_ context.Context, id string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.deleted = append(c.deleted, id)
    return nil
}
```

<!-- [LINE EDIT] "The `sync.Mutex` is not optional. `consumer.Run` delivers messages in the goroutine that calls `ConsumeClaim`, which is a different goroutine from your test's polling loop. Without the mutex, the slice appends in `Upsert` and `Delete` would race with the reads in your polling condition check. Go's race detector (`go test -race -tags integration`) will catch this immediately if you omit the lock." 64 words across four sentences; acceptable. -->
<!-- [COPY EDIT] "polling condition check" — noun stack; consider "polling-condition check" or "polling check" for clarity. -->
The `sync.Mutex` is not optional. `consumer.Run` delivers messages in the goroutine that calls `ConsumeClaim`, which is a different goroutine from your test's polling loop. Without the mutex, the slice appends in `Upsert` and `Delete` would race with the reads in your polling condition check. Go's race detector (`go test -race -tags integration`) will catch this immediately if you omit the lock.

<!-- [LINE EDIT] "With the capturing indexer in place, the test itself only needs Kafka — no Postgres, no Meilisearch:" — fine. -->
With the capturing indexer in place, the test itself only needs Kafka — no Postgres, no Meilisearch:

```go
//go:build integration

package consumer_test

import (
    "context"
    "encoding/json"
    "fmt"
    "testing"
    "time"

    "github.com/IBM/sarama"

    searchconsumer "github.com/fesoliveira014/library-system/services/search/internal/consumer"
)

func TestSearchConsumer_BookCreated_CallsUpsert(t *testing.T) {
    brokers := sharedBrokers
    topic := fmt.Sprintf("catalog-events-test-%s", t.Name())
    groupID := fmt.Sprintf("test-%s", t.Name())

    bookID := "book-search-001"
    event := map[string]interface{}{
        "event_type":       "book.created",
        "book_id":          bookID,
        "title":            "Clean Code",
        "author":           "Robert Martin",
        "isbn":             "9780132350884",
        "genre":            "programming",
        "total_copies":     3,
        "available_copies": 3,
    }
    payload, _ := json.Marshal(event)

    config := sarama.NewConfig()
    config.Producer.Return.Successes = true

    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        t.Fatalf("create producer: %v", err)
    }
    defer producer.Close()

    _, _, err = producer.SendMessage(&sarama.ProducerMessage{
        Topic: topic,
        Value: sarama.ByteEncoder(payload),
    })
    if err != nil {
        t.Fatalf("send message: %v", err)
    }

    idx := &capturingIndexer{}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        if err := searchconsumer.Run(ctx, brokers, topic, idx, groupID); err != nil {
            t.Logf("search consumer exited: %v", err)
        }
    }()

    // Poll until the indexer records the upsert.
    deadline := time.After(10 * time.Second)
    for {
        select {
        case <-deadline:
            t.Fatal("timed out waiting for indexer upsert")
        case <-time.After(200 * time.Millisecond):
            idx.mu.Lock()
            upserted := append([]any(nil), idx.upserted...) // snapshot
            idx.mu.Unlock()

            if len(upserted) == 0 {
                continue
            }
            doc := upserted[0].(model.BookDocument)
            if doc.ID != bookID {
                t.Errorf("upserted ID: want %q, got %q", bookID, doc.ID)
            }
            if doc.Title != "Clean Code" {
                t.Errorf("upserted title: want %q, got %q", "Clean Code", doc.Title)
            }
            cancel()
            return
        }
    }
}
```

<!-- [FINAL] In the snapshot line: `upserted := append([]any(nil), idx.upserted...)` — this creates a `[]any` slice but `idx.upserted` is `[]model.BookDocument`. The append would require each element to be assignable to `any`, which is fine at runtime (boxing), but the slice type mismatch triggers a compile error in typed append. Either use `append([]model.BookDocument(nil), idx.upserted...)` (type-matching) or `slices.Clone(idx.upserted)`. The later type-assert `upserted[0].(model.BookDocument)` would then be redundant / removed. Please verify. -->
<!-- [COPY EDIT] Please verify: the snapshot pattern compiles. Suggest rewriting with `slices.Clone` for clarity. -->

<!-- [LINE EDIT] "This test exercises the full Kafka path — group join, partition assignment, message deserialization, `ConsumeClaim` loop — without any dependency on Meilisearch. If you later add a `book.deleted` scenario, the same capturing indexer catches calls to `Delete`." — fine. -->
This test exercises the full Kafka path — group join, partition assignment, message deserialization, `ConsumeClaim` loop — without any dependency on Meilisearch. If you later add a `book.deleted` scenario, the same capturing indexer catches calls to `Delete`.

<!-- [LINE EDIT] "The snapshot pattern (`append([]any(nil), idx.upserted...)`) copies the slice while holding the lock, then releases the lock before doing assertions. This is important: holding a mutex while calling `t.Errorf` is not inherently unsafe in Go, but it is poor practice because `t.Errorf` may trigger housekeeping that takes non-trivial time under the lock." 52 words across two sentences; acceptable. -->
<!-- [COPY EDIT] "non-trivial" — CMOS 7.85: "nontrivial" closed. -->
The snapshot pattern (`append([]any(nil), idx.upserted...)`) copies the slice while holding the lock, then releases the lock before doing assertions. This is important: holding a mutex while calling `t.Errorf` is not inherently unsafe in Go, but it is poor practice because `t.Errorf` may trigger housekeeping that takes non-trivial time under the lock.

---

## Gotchas

### Consumer group rebalancing

<!-- [LINE EDIT] "When a new consumer group joins a Kafka cluster for the first time, the broker assigns partitions via a rebalance protocol. This takes time — typically 1–3 seconds. If you produce a message, then immediately start a consumer, the consumer may miss messages produced before the assignment completes if its initial offset policy is `OffsetNewest`." 52 words across three sentences; fine. -->
<!-- [COPY EDIT] "1–3 seconds" — en dash; correct. -->
When a new consumer group joins a Kafka cluster for the first time, the broker assigns partitions via a rebalance protocol. This takes time — typically 1–3 seconds. If you produce a message, then immediately start a consumer, the consumer may miss messages produced before the assignment completes if its initial offset policy is `OffsetNewest`.

<!-- [LINE EDIT] "The solution used throughout this section is to set `config.Consumer.Offsets.Initial = sarama.OffsetOldest`. This tells sarama that when there is no committed offset for the group (which is the case for a fresh group), start from the beginning of the topic. Since each test uses a unique group ID, there is never a committed offset, and `OffsetOldest` is always effective." — fine. -->
The solution used throughout this section is to set `config.Consumer.Offsets.Initial = sarama.OffsetOldest`. This tells sarama that when there is no committed offset for the group (which is the case for a fresh group), start from the beginning of the topic. Since each test uses a unique group ID, there is never a committed offset, and `OffsetOldest` is always effective.

<!-- [LINE EDIT] "This is also why tests produce the message *before* starting the consumer: the message sits in the topic waiting at offset 0, and the consumer reads it as soon as partition assignment completes. If you start the consumer first and produce later, you need `OffsetNewest` — but then the test is sensitive to timing between the goroutine that runs the consumer and the goroutine (your test function) that produces the message. The produce-first pattern is simpler." 78 words across three sentences; second sentence is 42 words. Consider splitting. -->
<!-- [COPY EDIT] "produce-first" — compound adjective; correct. -->
This is also why tests produce the message *before* starting the consumer: the message sits in the topic waiting at offset 0, and the consumer reads it as soon as partition assignment completes. If you start the consumer first and produce later, you need `OffsetNewest` — but then the test is sensitive to timing between the goroutine that runs the consumer and the goroutine (your test function) that produces the message. The produce-first pattern is simpler.

### Unique group IDs

<!-- [LINE EDIT] "Never use a hardcoded group ID in a test. If two test functions in the same suite use the same group ID, they share offset state. Whichever test runs first commits an offset; the second test starts from that committed offset and may find no messages. The failure is intermittent and order-dependent — the worst kind." — fine. -->
Never use a hardcoded group ID in a test. If two test functions in the same suite use the same group ID, they share offset state. Whichever test runs first commits an offset; the second test starts from that committed offset and may find no messages. The failure is intermittent and order-dependent — the worst kind.

The pattern used here is:

```go
groupID := fmt.Sprintf("test-%s", t.Name())
```

<!-- [LINE EDIT] "`t.Name()` returns a string like `TestConsumer_ReservationCreated_DecreasesAvailability`. It is unique within a test binary. For subtests, it includes the subtest name separated by a slash, e.g., `TestConsumer/reservation_created`. The resulting group ID is unique per test function and safe for parallel runs." — fine. -->
<!-- [COPY EDIT] "e.g.," — comma after per CMOS 6.43; correct. -->
`t.Name()` returns a string like `TestConsumer_ReservationCreated_DecreasesAvailability`. It is unique within a test binary. For subtests, it includes the subtest name separated by a slash, e.g., `TestConsumer/reservation_created`. The resulting group ID is unique per test function and safe for parallel runs.

### Topic auto-creation

<!-- [LINE EDIT] "Sarama's producer auto-creates topics with default broker settings (one partition, replication factor 1, default retention). This is acceptable in a test environment. If you need to test specific partition counts — for example, to verify that a round-robin consumer group distributes load correctly — you should create topics explicitly using sarama's `ClusterAdmin`:" 52 words across three sentences; fine. -->
<!-- [COPY EDIT] "round-robin" — hyphenated compound adjective; correct. -->
Sarama's producer auto-creates topics with default broker settings (one partition, replication factor 1, default retention). This is acceptable in a test environment. If you need to test specific partition counts — for example, to verify that a round-robin consumer group distributes load correctly — you should create topics explicitly using sarama's `ClusterAdmin`:

```go
admin, _ := sarama.NewClusterAdmin(brokers, sarama.NewConfig())
defer admin.Close()
admin.CreateTopic(topic, &sarama.TopicDetail{
    NumPartitions:     3,
    ReplicationFactor: 1,
}, false)
```

<!-- [LINE EDIT] "For the tests in this section, auto-creation is fine." — fine. -->
For the tests in this section, auto-creation is fine.

### Container startup time

<!-- [LINE EDIT] "Kafka takes 10–15 seconds to start. PostgreSQL takes 3–5 seconds. If you start both in `TestMain`, the total wait before the first test runs is around 15 seconds (they start concurrently). This is acceptable for a CI integration suite but frustrating during local development." — fine. -->
<!-- [COPY EDIT] "3–5 seconds" again differs from index.md's "5–8 seconds" and 11.2's "two to four seconds". Unify across chapter. -->
Kafka takes 10–15 seconds to start. PostgreSQL takes 3–5 seconds. If you start both in `TestMain`, the total wait before the first test runs is around 15 seconds (they start concurrently). This is acceptable for a CI integration suite but frustrating during local development.

Two strategies help:

<!-- [LINE EDIT] "**Testcontainers Reuse.** Testcontainers supports container reuse across runs via `testcontainers.WithReuseFlag()`. With reuse enabled, the first `go test` invocation starts the container; subsequent invocations on the same machine reattach to the already-running container. This reduces the wait on repeat runs from 15s to under 1s. Reuse requires the container to be stopped manually when no longer needed." 58 words across four sentences; fine. -->
<!-- [COPY EDIT] Please verify: `testcontainers.WithReuseFlag()` — the current API is `testcontainers.WithReuse()` (not `WithReuseFlag`). Confirm against testcontainers-go v0.32+. -->
<!-- [COPY EDIT] "15s" and "1s" — acceptable compact units in technical context. -->
**Testcontainers Reuse.** Testcontainers supports container reuse across runs via `testcontainers.WithReuseFlag()`. With reuse enabled, the first `go test` invocation starts the container; subsequent invocations on the same machine reattach to the already-running container. This reduces the wait on repeat runs from 15s to under 1s. Reuse requires the container to be stopped manually when no longer needed.

<!-- [LINE EDIT] "**Selective tagging.** Keep Kafka tests in a separate package (e.g., `services/catalog/internal/consumer/integration_test`) so you can run just the Kafka tests with `go test -tags integration ./services/catalog/internal/consumer/...` rather than the entire integration suite." — fine. -->
**Selective tagging.** Keep Kafka tests in a separate package (e.g., `services/catalog/internal/consumer/integration_test`) so you can run just the Kafka tests with `go test -tags integration ./services/catalog/internal/consumer/...` rather than the entire integration suite.

### Consumer goroutine lifecycle

<!-- [LINE EDIT] "`consumer.Run` blocks until the context is cancelled. If you start it in a goroutine and the test returns without cancelling the context, the goroutine leaks. This is not just a theoretical concern: leaked goroutines that hold sarama consumer group sessions can cause the group to remain "active" in Kafka, delaying rebalances in subsequent tests." 50 words across three sentences; fine. -->
<!-- [COPY EDIT] "cancelled" / "cancelling" — UK form. Normalize to US "canceled" / "canceling". -->
`consumer.Run` blocks until the context is cancelled. If you start it in a goroutine and the test returns without cancelling the context, the goroutine leaks. This is not just a theoretical concern: leaked goroutines that hold sarama consumer group sessions can cause the group to remain "active" in Kafka, delaying rebalances in subsequent tests.

<!-- [LINE EDIT] "The `defer cancel()` at the top of each test function is the defense. Even if the test panics or calls `t.Fatal`, deferred functions run, the context is cancelled, and `Run` returns within one consumer group heartbeat interval (default: 3 seconds in sarama). For faster teardown, sarama supports configuring `config.Consumer.Group.Session.Timeout`." 50 words across three sentences; fine. -->
<!-- [COPY EDIT] "consumer group heartbeat interval" — consider "consumer-group heartbeat interval" (compound adjective). -->
The `defer cancel()` at the top of each test function is the defense. Even if the test panics or calls `t.Fatal`, deferred functions run, the context is cancelled, and `Run` returns within one consumer group heartbeat interval (default: 3 seconds in sarama). For faster teardown, sarama supports configuring `config.Consumer.Group.Session.Timeout`.

---

## Summary

<!-- [LINE EDIT] "The tests in this section cover the two integration surfaces that unit tests cannot reach: the serialization path between the reservation publisher and the catalog consumer, and the Kafka-to-database path within the catalog service. The search consumer test demonstrates that you do not always need the full dependency chain — a capturing mock combined with a real Kafka round-trip is often sufficient to test message routing and deserialization." — 65 words across two sentences; second sentence is 30 words. Fine. -->
<!-- [COPY EDIT] "Kafka-to-database" — hyphenated compound; correct. -->
The tests in this section cover the two integration surfaces that unit tests cannot reach: the serialization path between the reservation publisher and the catalog consumer, and the Kafka-to-database path within the catalog service. The search consumer test demonstrates that you do not always need the full dependency chain — a capturing mock combined with a real Kafka round-trip is often sufficient to test message routing and deserialization.

<!-- [LINE EDIT] "The key patterns introduced here carry over to any Kafka integration test in Go:" — fine. -->
The key patterns introduced here carry over to any Kafka integration test in Go:

<!-- [COPY EDIT] Bullet list is parallel (imperative verbs). Good. -->
- Use testcontainers-go's Kafka module with `confluentinc/confluent-local:7.6.0`.
- Share a single container across tests with `TestMain`.
- Use unique group IDs per test to avoid offset state collisions.
- Use `sarama.OffsetOldest` for fresh groups; produce before starting the consumer.
- Poll with a ticker and deadline rather than sleeping.
- Always `defer cancel()` to ensure consumer goroutine cleanup.
- Use a mutex-protected capturing struct to verify async side effects without a real downstream service.
<!-- [COPY EDIT] "offset state collisions" → "offset-state collisions" (compound adjective) — minor. -->
<!-- [COPY EDIT] "mutex-protected capturing struct" — reads fine. -->

---

[^1]: Testcontainers for Go — Kafka module: https://golang.testcontainers.org/modules/kafka/
[^2]: IBM/sarama — Go Kafka client: https://github.com/IBM/sarama
<!-- [COPY EDIT] Please verify: both footnote URLs still resolve. Given Sarama's maintenance status, consider adding a footnote to franz-go as an alternative. -->
