# 8.1 Catalog Event Publishing

<!-- [STRUCTURAL] Opening paragraph motivates the pattern well — concrete ("search index needs to stay in sync") before abstract ("event-driven architecture"). The three-paragraph lede (what/why/Spring analogy) is balanced; no gaps. -->
<!-- [LINE EDIT] "other parts of the system need to know about it" → "other parts of the system need to know" (trim trailing pronoun). Small cut. -->
When the catalog service creates, updates, or deletes a book, other parts of the system need to know about it. The search index needs to stay in sync. Future services -- recommendations, analytics, audit logs -- will want the same information. Rather than coupling every downstream consumer to the catalog's database or API, we publish **domain events** to Kafka and let consumers process them independently.
<!-- [COPY EDIT] "catalog service" lowercase — matches index.md's looser usage but conflicts with "Catalog Service" in architecture diagrams elsewhere. Pick one and enforce across chapter. Recommend lowercase "catalog service" in running prose. -->
<!-- [COPY EDIT] En dashes vs em dashes: the source uses `--` (two hyphens) consistently throughout. CMOS 6.85 calls for em dashes (—) in prose without spaces. If the publishing toolchain converts `--` to em dashes, fine; if not, consider replacing `--` with `—`. Consistency within the chapter is already good — flagging for awareness, not changes. -->

<!-- [LINE EDIT] "This is the core idea behind event-driven architecture" is fine. Sentence two is clean. -->
This is the core idea behind event-driven architecture: the producer does not know or care who is listening. It publishes a fact ("book X was created") and moves on. Consumers subscribe to the topic and react on their own schedule. This decoupling is what makes it possible to add the search service without modifying any existing consumer code.

<!-- [COPY EDIT] "Spring's `ApplicationEventPublisher` or Kotlin's `Flow`-based event buses" — product/library names correctly capitalized. Good. -->
If you have used Spring's `ApplicationEventPublisher` or Kotlin's `Flow`-based event buses, the concept is identical. The difference is that Kafka events cross process boundaries and survive restarts -- they are durable, ordered, and replayable.

---

## The BookEvent Struct

<!-- [STRUCTURAL] Good flow: introduce the struct, then enumerated design-decision commentary. Tutor voice. -->
Every event needs a shape. Ours carries the full book data plus metadata about what happened:

```go
// services/catalog/internal/service/catalog.go

type BookEvent struct {
    EventType       string    `json:"event_type"`
    BookID          string    `json:"book_id"`
    Title           string    `json:"title,omitempty"`
    Author          string    `json:"author,omitempty"`
    ISBN            string    `json:"isbn,omitempty"`
    Genre           string    `json:"genre,omitempty"`
    Description     string    `json:"description,omitempty"`
    PublishedYear   int       `json:"published_year,omitempty"`
    TotalCopies     int       `json:"total_copies,omitempty"`
    AvailableCopies int       `json:"available_copies,omitempty"`
    Timestamp       time.Time `json:"timestamp"`
}
```

A few design decisions to note:

<!-- [LINE EDIT] "You could use `iota` constants, but JSON serialization makes string values more practical" — tight. Keep. -->
<!-- [COPY EDIT] Item 1 — "book.created", "book.updated", "book.deleted" — double-check that these string values appear identically in the consumer (meilisearch.md line ~432 uses the same). Confirmed earlier read. Consistent. -->
1. **`EventType` is a string, not an enum.** Go does not have sum types. You could use `iota` constants, but JSON serialization makes string values more practical -- consumers can switch on the string without importing a shared constants package. The values we use are `"book.created"`, `"book.updated"`, and `"book.deleted"`.

<!-- [LINE EDIT] "Including the full book data would be wasteful and potentially misleading" — consider "wasteful and misleading" (cut "potentially"). The rhetorical-question aside softens claim enough. -->
2. **`omitempty` on data fields.** A `book.deleted` event only needs the `BookID` and `EventType`. Including the full book data would be wasteful and potentially misleading (what does "title" mean for a deleted book?). The `omitempty` tag ensures those fields are absent from the JSON payload when they carry zero values.

<!-- [COPY EDIT] "Go's UUID package" — Go does not ship a UUID package; the common community choice is `github.com/google/uuid`. Recommend "the `github.com/google/uuid` package" or "a UUID package" for precision. CMOS 7.52 / factual accuracy. -->
3. **`BookID` is a string, not a `uuid.UUID`.** Events cross service boundaries. The consumer should not need to import Go's UUID package to deserialize a message -- a plain string is universally portable.

<!-- [LINE EDIT] "useful for debugging, ordering heuristics, and consumer-side deduplication" — serial comma present. Good (CMOS 6.19). -->
4. **`Timestamp` records when the event was produced.** This is useful for debugging, ordering heuristics, and consumer-side deduplication. It is the wall-clock time on the producer, not a Kafka-assigned timestamp.

---

## The EventPublisher Interface

<!-- [STRUCTURAL] Clean introduction of dependency-inversion via interface. Spring analogy lands. -->
The service layer does not depend on Kafka directly. It depends on an interface:

```go
// services/catalog/internal/service/catalog.go

type EventPublisher interface {
    Publish(ctx context.Context, event BookEvent) error
}
```

<!-- [LINE EDIT] "This is the same dependency-inversion pattern we use everywhere" — "everywhere" is informal but acceptable in tutor voice. Keep. -->
This is the same dependency-inversion pattern we use everywhere: the service defines what it needs (an `EventPublisher`), and `main.go` wires in the concrete implementation (Kafka). In tests, you can substitute a mock publisher that records events in a slice.

<!-- [COPY EDIT] "In Spring, you would achieve this with `@Autowired` on an interface type and a `@Component` on the implementation." — accurate. Good. -->
In Spring, you would achieve this with `@Autowired` on an interface type and a `@Component` on the implementation. In Go, there is no container -- you pass the implementation explicitly through the constructor:

```go
// services/catalog/internal/service/catalog.go

type CatalogService struct {
    repo      BookRepository
    publisher EventPublisher
}

func NewCatalogService(repo BookRepository, publisher EventPublisher) *CatalogService {
    return &CatalogService{repo: repo, publisher: publisher}
}
```

---

## Publishing Events from CRUD Operations

<!-- [STRUCTURAL] The "write first, publish second, log on failure" pattern is clearly motivated. The tradeoff paragraph (availability vs. strict consistency) is one of the chapter's strongest passages — keep. -->
Each mutation method follows the same pattern: perform the database operation first, then publish the event. If the publish fails, **we log the error but do not fail the request**.

Here is `CreateBook`:

```go
// services/catalog/internal/service/catalog.go

func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
    if err := validateBook(book); err != nil {
        return nil, err
    }
    book.AvailableCopies = book.TotalCopies
    created, err := s.repo.Create(ctx, book)
    if err != nil {
        return nil, err
    }

    bookCounter.Add(ctx, 1)

    if err := s.publisher.Publish(ctx, bookToEvent("book.created", created)); err != nil {
        slog.ErrorContext(ctx, "failed to publish event",
            "event", "book.created", "book_id", created.ID, "error", err)
    }

    return created, nil
}
```

<!-- [LINE EDIT] "The critical decision here is error handling." → acceptable opener; alternative "Error handling is the critical decision here." inverts to subject-first. Kept. -->
<!-- [LINE EDIT] "The database write succeeded -- the book exists." — tight, good. -->
The critical decision here is error handling. The database write succeeded -- the book exists. If the Kafka publish fails (broker is down, network partition), we do not roll back the database. Instead, we log the error and return success to the caller. The search index will be temporarily inconsistent, but the bootstrap mechanism (covered in section 8.3) will catch it up on restart.
<!-- [COPY EDIT] "section 8.3" — Bootstrap is indeed in 8.3 (meilisearch.md, "Bootstrap: Initial Index Population"). Cross-ref verified. -->

<!-- [LINE EDIT] "It would not be acceptable for the ..." — slightly long; fine as is. -->
This is a deliberate tradeoff: **availability over strict consistency**. In a library system, it is acceptable for a newly created book to not appear in search results for a few seconds (or even minutes). It would not be acceptable for the "create book" API to fail because the search infrastructure is having problems.
<!-- [COPY EDIT] "tradeoff" vs "trade-off" — CMOS/Merriam-Webster accepts both; "tradeoff" is common in tech writing. Check consistency: both forms appear only as "tradeoff" in this chapter (search-service.md line ~385 uses "tradeoffs"). Consistent. -->

<!-- [LINE EDIT] Sentence is 48 words. Consider splitting: "The alternative is a transactional outbox pattern: the event is written to the same database as the book and published later by a separate process. This provides stronger guarantees but adds significant complexity." -->
The alternative -- using a transactional outbox pattern where the event is written to the same database as the book and published later by a separate process -- provides stronger guarantees but adds significant complexity. We keep things simple here.

`UpdateBook` and `DeleteBook` follow the same structure:

```go
// services/catalog/internal/service/catalog.go

func (s *CatalogService) UpdateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
    if book.TotalCopies < 0 {
        return nil, fmt.Errorf("%w: total copies must be non-negative", model.ErrInvalidBook)
    }
    updated, err := s.repo.Update(ctx, book)
    if err != nil {
        return nil, err
    }

    if err := s.publisher.Publish(ctx, bookToEvent("book.updated", updated)); err != nil {
        slog.ErrorContext(ctx, "failed to publish event",
            "event", "book.updated", "book_id", updated.ID, "error", err)
    }

    return updated, nil
}

func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
    if err := s.repo.Delete(ctx, id); err != nil {
        return err
    }

    bookCounter.Add(ctx, -1)

    if err := s.publisher.Publish(ctx, BookEvent{
        EventType: "book.deleted",
        BookID:    id.String(),
        Timestamp: time.Now(),
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish event",
            "event", "book.deleted", "book_id", id, "error", err)
    }

    return nil
}
```

<!-- [LINE EDIT] "Notice that `DeleteBook` constructs ..." — good. -->
Notice that `DeleteBook` constructs the `BookEvent` inline rather than using `bookToEvent`. The book has already been deleted from the database -- there is no `*model.Book` to convert. The event only needs the ID and the event type.

The `bookToEvent` helper maps the internal model to the event struct:

```go
// services/catalog/internal/service/catalog.go

func bookToEvent(eventType string, book *model.Book) BookEvent {
    return BookEvent{
        EventType:       eventType,
        BookID:          book.ID.String(),
        Title:           book.Title,
        Author:          book.Author,
        ISBN:            book.ISBN,
        Genre:           book.Genre,
        Description:     book.Description,
        PublishedYear:   book.PublishedYear,
        TotalCopies:     book.TotalCopies,
        AvailableCopies: book.AvailableCopies,
        Timestamp:       time.Now(),
    }
}
```

<!-- [STRUCTURAL] Paragraph below introduces UpdateAvailability but no code sample or deeper discussion. Consider either (a) showing the method for completeness or (b) cutting — the mention is load-bearing enough to keep, since it explains why search stays fresh after reservations. Keep as a short bridge, but mark that this behavior was introduced in an earlier chapter fix (per repo commit log). -->
There is also `UpdateAvailability`, which publishes a `book.updated` event after changing the copy count. This keeps the search index in sync when books are reserved or returned.

---

## The Kafka Publisher

<!-- [STRUCTURAL] Good. The introduction, config choices, and subsections (Topic Naming, Message Keying, Trace Propagation) form a clean narrative. -->
<!-- [COPY EDIT] Please verify: "Sarama, the most widely used Go Kafka client library". As of 2024–2025, Sarama is the oldest and most-cited but `franz-go` (twmb/franz-go) has become widely adopted for new projects. Recent commit in this repo (37c217a) already notes "Sarama maintenance status and recommend evaluating franz-go" — consider softening from "the most widely used" to "the long-standing and widely used Go Kafka client". Factual nuance. -->
The concrete `EventPublisher` implementation uses **Sarama**, the most widely used Go Kafka client library[^1]. It wraps a `SyncProducer` -- a producer that blocks until the broker acknowledges the message.

```go
// services/catalog/internal/kafka/publisher.go

type Publisher struct {
    producer sarama.SyncProducer
    topic    string
}

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

Two configuration choices matter here:

<!-- [LINE EDIT] "Required for `SyncProducer`." — tight. Keep. -->
- **`Return.Successes = true`** -- Required for `SyncProducer`. The producer waits for the broker to confirm receipt before returning. Without this, you would need an `AsyncProducer` and a goroutine reading from the `Successes()` channel.

<!-- [LINE EDIT] "The broker does not acknowledge the message until all in-sync replicas have written it." — good. -->
<!-- [COPY EDIT] "in-sync replicas" — standard Kafka terminology (ISR). Correctly hyphenated as a compound adjective before a noun (CMOS 7.81). -->
- **`RequiredAcks = WaitForAll`** -- The broker does not acknowledge the message until all in-sync replicas have written it. This is the strongest durability guarantee Kafka offers. For a search index that can be rebuilt from scratch, `WaitForLocal` (leader-only ack) would also be fine, but `WaitForAll` is a sensible default.

### Topic Naming

Our topic is `catalog.books.changed`. The convention is `<service>.<entity>.<action>`:

| Segment | Value | Purpose |
|---------|-------|---------|
| `catalog` | The producing service | Prevents name collisions across services |
| `books` | The entity type | Groups related events |
| `changed` | A generic action | Covers create, update, and delete in one topic |

<!-- [LINE EDIT] "An alternative design would use separate topics" — clear comparison, but 57 words. Consider a split at "simpler:": "The single-topic approach is simpler. One consumer-group subscription covers all book mutations, and message ordering within a partition is preserved across event types." -->
An alternative design would use separate topics per event type (`catalog.books.created`, `catalog.books.deleted`). The single-topic approach is simpler: one consumer group subscription covers all book mutations, and message ordering within a partition is preserved across event types. The `event_type` field inside the JSON payload distinguishes the operations.

### Message Keying

```go
msg := &sarama.ProducerMessage{
    Topic: p.topic,
    Key:   sarama.StringEncoder(event.BookID),
    Value: sarama.ByteEncoder(value),
}
```

<!-- [LINE EDIT] Good short paragraph. No change. -->
The message key is set to the `BookID`. Kafka guarantees ordering within a partition, and messages with the same key always go to the same partition. This means all events for a given book are ordered: you will never process a `book.deleted` before a `book.created` for the same book ID.

### Trace Propagation

The publisher injects OpenTelemetry trace context into the Kafka message headers:

```go
// services/catalog/internal/kafka/publisher.go

otelgo.GetTextMapPropagator().Inject(ctx, &headerCarrier{msg: msg})

ctx, span := otelgo.Tracer("catalog").Start(ctx, "catalog.publish")
defer span.End()
```

<!-- [STRUCTURAL] The two lines above are shown in order (Inject, then Start span) — but conceptually the span should start first so that it wraps the injection. Please verify against the actual source; this may be a snippet-extraction issue. If the code really runs Inject before Start, the trace context being injected is the parent context, not the new publish span's context — which would be an observability bug. Flag for fact-check. -->
<!-- [COPY EDIT] Please verify: the order of `Inject` vs `Start` in the real `publisher.go`. If `Start` comes first, this snippet should be reordered. -->

The `headerCarrier` type adapts Sarama's `RecordHeader` slice to the `propagation.TextMapCarrier` interface that OpenTelemetry expects. This is boilerplate, but it is important: it allows a trace that starts with an HTTP request to the gateway to flow through the catalog service, into Kafka, and out to the search consumer -- giving you end-to-end visibility across asynchronous boundaries.
<!-- [COPY EDIT] "end-to-end" hyphenated as compound modifier before noun — CMOS 7.81. Good. -->

```go
// services/catalog/internal/kafka/publisher.go

type headerCarrier struct {
    msg *sarama.ProducerMessage
}

func (c *headerCarrier) Get(key string) string {
    for _, h := range c.msg.Headers {
        if string(h.Key) == key {
            return string(h.Value)
        }
    }
    return ""
}

func (c *headerCarrier) Set(key, value string) {
    c.msg.Headers = append(c.msg.Headers, sarama.RecordHeader{
        Key:   []byte(key),
        Value: []byte(value),
    })
}

func (c *headerCarrier) Keys() []string {
    keys := make([]string, len(c.msg.Headers))
    for i, h := range c.msg.Headers {
        keys[i] = string(h.Key)
    }
    return keys
}
```

<!-- [LINE EDIT] "In Java/Kotlin Kafka clients, trace propagation is typically handled by an interceptor or a library like `opentelemetry-java-instrumentation` that patches the client automatically." — good. -->
In Java/Kotlin Kafka clients, trace propagation is typically handled by an interceptor or a library like `opentelemetry-java-instrumentation` that patches the client automatically. In Go, you wire it manually -- more code, but nothing is hidden.

---

## Exercises

<!-- [STRUCTURAL] Four exercises, well-graduated from practical (add event type) to conceptual (failure modes). Strong. -->
1. **Add a `book.availability_changed` event type.** Right now, `UpdateAvailability` publishes a generic `book.updated` event. Create a new event type that includes only the `BookID`, `AvailableCopies`, and `TotalCopies` fields. Update the consumer (section 8.3) to handle it.

<!-- [LINE EDIT] "Create a mock `EventPublisher` that stores published events in a slice" — clean. -->
2. **Write a test for `CreateBook` event publishing.** Create a mock `EventPublisher` that stores published events in a slice. Call `CreateBook` and assert that exactly one `book.created` event was published with the correct book ID.

3. **Implement a no-op publisher.** Write a `NopPublisher` that satisfies the `EventPublisher` interface but discards all events. When would you use this? (Hint: think about the catalog service's unit tests and local development without Kafka.)

<!-- [LINE EDIT] "Is this the right behavior?" — rhetorical question flow is fine in an exercise prompt. Keep. -->
4. **Think about failure modes.** If the Kafka broker is completely unreachable, `NewPublisher` will fail at startup. Is this the right behavior? What if you wanted the catalog to start even without Kafka? How would you change the code?

---

## References

<!-- [COPY EDIT] "Sarama library, originally by Shopify, now maintained by IBM" — accurate as of repo handover. Per recent commit note, maintenance status warrants a caveat; consider aligning footnote with chapter body. -->
[^1]: [IBM/sarama -- Go client for Apache Kafka](https://github.com/IBM/sarama) -- The Sarama library, originally by Shopify, now maintained by IBM. It provides both sync and async producers, consumer groups, and admin operations.
[^2]: [OpenTelemetry Go -- Propagation](https://opentelemetry.io/docs/languages/go/propagation/) -- How to propagate trace context across process boundaries using text map carriers.
[^3]: [Kafka documentation -- Producer Configs](https://kafka.apache.org/documentation/#producerconfigs) -- Reference for `acks`, `retries`, and other producer configuration knobs.
<!-- [COPY EDIT] Please verify: URL for DDIA Chapter 11 on dataintensive.net — the book's companion site does not deep-link to chapters. Recommend linking to the root `https://dataintensive.net/` (already used) and noting "Chapter 11" in the text (already done). URL is fine. -->
[^4]: [Martin Kleppmann -- Designing Data-Intensive Applications, Ch. 11](https://dataintensive.net/) -- The canonical reference on event-driven architectures, log-based messaging, and the tradeoffs between different consistency models.
