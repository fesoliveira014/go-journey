# 8.3 Meilisearch Integration

<!-- [STRUCTURAL] Strong opener. Meilisearch positioning (vs. Elasticsearch), Spring Data mapping table, and then the implementation dive — a tutor-friendly sequence. -->
<!-- [COPY EDIT] "Meilisearch" capitalized correctly throughout. Good. -->
Meilisearch is a lightweight, open-source search engine written in Rust. It is designed for end-user-facing search: fast typo-tolerant queries, faceted filtering, and instant suggestions out of the box. It ships as a single binary, requires no JVM, and runs comfortably in a Docker container with 256MB of RAM.
<!-- [COPY EDIT] "open-source" correctly hyphenated as compound adjective before noun (CMOS 7.81). Good. -->
<!-- [COPY EDIT] "end-user-facing" — double-compound modifier. Correct per CMOS 7.89 to hyphenate both joints. Good. -->
<!-- [COPY EDIT] "typo-tolerant" correctly hyphenated (CMOS 7.81). Good. -->
<!-- [COPY EDIT] "256MB" — per CMOS 9.16/10.49 technical measurements as numerals + unit abbreviation; recommend a non-breaking space ("256 MB") or at minimum consistency with usage elsewhere in the book. Flagging for house style. -->

<!-- [LINE EDIT] "**Why not Elasticsearch?** Elasticsearch is the industry standard, but it is also a complex distributed system." — works. -->
**Why not Elasticsearch?** Elasticsearch is the industry standard, but it is also a complex distributed system. It needs a JVM, consumes significant resources at rest, and requires careful tuning for production use. For a learning project with a modest dataset, Meilisearch gives us the same search concepts (indexes, documents, filterable attributes, relevance ranking) with a fraction of the operational overhead. The Go client library (`meilisearch-go`) is well-maintained and straightforward.
<!-- [COPY EDIT] "well-maintained" correctly hyphenated (CMOS 7.81). -->

If you have used Elasticsearch with Spring Data, the conceptual mapping is:

<!-- [STRUCTURAL] Mapping table is a nice quick reference. Good. -->
| Elasticsearch | Meilisearch | Our code |
|--------------|-------------|----------|
| Index | Index | `"books"` |
| Document | Document | `BookDocument` |
| Mapping (field types) | Searchable/filterable attributes | `EnsureIndex()` |
| `@Document` annotation | JSON tags on struct | `json:"title"` |
| `ElasticsearchRepository` | `IndexRepository` interface | `index.go` |

---

## The IndexRepository Implementation

The `MeilisearchIndex` struct wraps the `meilisearch-go` client and satisfies the `IndexRepository` interface from section 8.2:

```go
// services/search/internal/index/index.go

const indexName = "books"

type MeilisearchIndex struct {
    client meilisearch.ServiceManager
}

func NewMeilisearchIndex(url, apiKey string) *MeilisearchIndex {
    client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
    return &MeilisearchIndex{client: client}
}
```

<!-- [COPY EDIT] "`MEILI_MASTER_KEY`" — all caps env var correct. Good. -->
<!-- [LINE EDIT] "In development, you can run Meilisearch without a key ..." — good. -->
The constructor creates a Meilisearch client with the server URL and an optional API key. In development, you can run Meilisearch without a key (the `MEILI_MASTER_KEY` env var defaults to empty). In production, you would always set a key.

### Index Configuration: EnsureIndex

<!-- [COPY EDIT] Heading "Index Configuration: EnsureIndex" — after colon the second part is not a complete sentence (just a function name). CMOS 8.159 says capitalize the word after a colon only if what follows is a complete sentence OR a proper name. `EnsureIndex` is a proper-noun-like identifier; capitalization fine. Keep. -->
Before we can search, the index must exist and its attributes must be configured. The `EnsureIndex` method handles this idempotently:

```go
// services/search/internal/index/index.go

func (m *MeilisearchIndex) EnsureIndex(_ context.Context) error {
    _, err := m.client.CreateIndex(&meilisearch.IndexConfig{
        Uid:        indexName,
        PrimaryKey: "id",
    })
    if err != nil {
        if meiliErr, ok := err.(*meilisearch.Error); ok {
            if meiliErr.MeilisearchApiError.Code == "index_already_exists" {
                // Index exists, continue to configure attributes
            } else {
                return fmt.Errorf("create index: %w", err)
            }
        } else {
            return fmt.Errorf("create index: %w", err)
        }
    }

    idx := m.client.Index(indexName)

    if _, err := idx.UpdateSearchableAttributes(&[]string{
        "title", "author", "isbn", "description", "genre",
    }); err != nil {
        return fmt.Errorf("update searchable attributes: %w", err)
    }
    if _, err := idx.UpdateFilterableAttributes(&[]interface{}{
        "genre", "author", "available_copies",
    }); err != nil {
        return fmt.Errorf("update filterable attributes: %w", err)
    }
    if _, err := idx.UpdateSortableAttributes(&[]string{
        "title", "published_year",
    }); err != nil {
        return fmt.Errorf("update sortable attributes: %w", err)
    }

    return nil
}
```

<!-- [COPY EDIT] Please verify: `meiliErr.MeilisearchApiError.Code == "index_already_exists"` — check current meilisearch-go release (v0.28+) API shape. In older versions the error embedded field was `MeilisearchApiError`; newer versions may use `ErrCode` or a typed constant. Recommend consulting the library docs for the current attribute name. -->
<!-- [COPY EDIT] "MeilisearchApiError" Go naming: the `Api` acronym should be all-caps per Go naming conventions (golint/revive: `ApiError` → `APIError`). However, if the library exports `MeilisearchApiError`, you cannot change it — this is a library-side nit. No action for the book. -->
<!-- [COPY EDIT] Please verify: the error discriminator string `"index_already_exists"`. Meilisearch API documentation lists this code under "Errors" — verify current spelling (snake_case, lowercase). -->
<!-- [STRUCTURAL] "continue to configure attributes" comment in the else branch is the best way to show the idempotent-except-for-attributes logic. Good. -->

Three categories of attributes are configured:

<!-- [LINE EDIT] "Order matters: Meilisearch ranks matches in earlier fields higher." — strong and accurate per Meilisearch docs (attribute ranking rule). Keep. -->
- **Searchable attributes** (`title`, `author`, `isbn`, `description`, `genre`) -- These are the fields Meilisearch will scan when a user types a query. Order matters: Meilisearch ranks matches in earlier fields higher. A match in `title` scores higher than a match in `description`.

<!-- [LINE EDIT] "Without declaring a field as filterable, you cannot use it in filter expressions." — clear. -->
- **Filterable attributes** (`genre`, `author`, `available_copies`) -- These enable exact-match filtering. Without declaring a field as filterable, you cannot use it in filter expressions. The `available_copies` field is filterable so we can implement "available only" searches with `available_copies > 0`.
<!-- [COPY EDIT] "exact-match" correctly hyphenated as compound modifier (CMOS 7.81). -->

- **Sortable attributes** (`title`, `published_year`) -- These enable explicit sort ordering in queries.

<!-- [LINE EDIT] "The `index_already_exists` error is handled with a type assertion on the Meilisearch error type." — good. -->
<!-- [COPY EDIT] "This is Go's pattern for typed errors -- there is no exception hierarchy to catch, so you assert the error to a concrete type and inspect its fields." — prefer `errors.As` idiom for typed-error extraction (Go 1.13+). Flagging, because the code shown uses the older `.(*T)` type assertion. Consider a footnote noting `errors.As` is now idiomatic. -->
The `index_already_exists` error is handled with a type assertion on the Meilisearch error type. This is Go's pattern for typed errors -- there is no exception hierarchy to catch, so you assert the error to a concrete type and inspect its fields. The attribute updates are always re-applied even if the index exists, making the method idempotent.

---

## Search and Suggest

The `Search` method constructs a Meilisearch query with optional filters:

```go
// services/search/internal/index/index.go

func (m *MeilisearchIndex) Search(_ context.Context, query string, filters SearchFilters,
    page, pageSize int) ([]model.BookDocument, int64, int64, error) {
    idx := m.client.Index(indexName)

    filterParts := buildFilterString(filters)

    offset := int64(0)
    if page > 1 {
        offset = int64((page - 1) * pageSize)
    }

    req := &meilisearch.SearchRequest{
        Limit:  int64(pageSize),
        Offset: offset,
    }
    if len(filterParts) > 0 {
        req.Filter = strings.Join(filterParts, " AND ")
    }

    resp, err := idx.Search(query, req)
    if err != nil {
        return nil, 0, 0, fmt.Errorf("search: %w", err)
    }

    docs := make([]model.BookDocument, 0, len(resp.Hits))
    for _, hit := range resp.Hits {
        doc, err := hitToDocument(hit)
        if err != nil {
            continue
        }
        docs = append(docs, doc)
    }

    return docs, resp.EstimatedTotalHits, int64(resp.ProcessingTimeMs), nil
}
```

<!-- [LINE EDIT] "Meilisearch uses offset/limit pagination, not cursor-based." — crisp. -->
<!-- [COPY EDIT] "offset/limit" — slash used in a technical compound; standard. Good. -->
Meilisearch uses offset/limit pagination, not cursor-based. We convert the page number to an offset: page 1 starts at offset 0, page 2 at offset `pageSize`, and so on.

The filter string is built by combining individual filter conditions with `AND`:

```go
// services/search/internal/index/index.go

func buildFilterString(filters SearchFilters) []string {
    var parts []string
    if filters.Genre != "" {
        parts = append(parts, fmt.Sprintf("genre = %q", filters.Genre))
    }
    if filters.Author != "" {
        parts = append(parts, fmt.Sprintf("author = %q", filters.Author))
    }
    if filters.AvailableOnly {
        parts = append(parts, "available_copies > 0")
    }
    return parts
}
```

<!-- [LINE EDIT] "The `%q` format verb produces a quoted string, which is what Meilisearch's filter syntax requires for string comparisons." — clear. -->
<!-- [COPY EDIT] Please verify: Meilisearch filter syntax accepts Go's `%q`-produced quoted strings. Meilisearch uses either double quotes or single quotes around string values; Go's `%q` always emits double quotes with Go-style escaping (e.g. embedded `"` is escaped as `\"`). Verify that Meilisearch's parser handles Go's escape sequences — otherwise string filters containing quotes or backslashes could fail. Query, not a change. -->
The `%q` format verb produces a quoted string, which is what Meilisearch's filter syntax requires for string comparisons. The `available_copies > 0` filter uses numeric comparison -- this only works because `available_copies` was declared as a filterable attribute in `EnsureIndex`.

### Suggest (Autocomplete)

The `Suggest` method is a lightweight search that returns only the fields needed for autocomplete:

```go
// services/search/internal/index/index.go

func (m *MeilisearchIndex) Suggest(_ context.Context, prefix string, limit int) ([]model.BookDocument, error) {
    idx := m.client.Index(indexName)

    resp, err := idx.Search(prefix, &meilisearch.SearchRequest{
        Limit:                int64(limit),
        AttributesToRetrieve: []string{"id", "title", "author"},
    })
    if err != nil {
        return nil, fmt.Errorf("suggest: %w", err)
    }

    docs := make([]model.BookDocument, 0, len(resp.Hits))
    for _, hit := range resp.Hits {
        doc, err := hitToDocument(hit)
        if err != nil {
            continue
        }
        docs = append(docs, doc)
    }

    return docs, nil
}
```

<!-- [LINE EDIT] "The key optimization is `AttributesToRetrieve`." — crisp. -->
The key optimization is `AttributesToRetrieve`. By requesting only `id`, `title`, and `author`, Meilisearch sends less data over the wire -- important for autocomplete where you are making requests on every keystroke.

### Hit Parsing

<!-- [STRUCTURAL] The "Go's less elegant areas" paragraph is honest and instructive. Keep. -->
Meilisearch returns hits as `interface{}` (really `map[string]interface{}` under the hood), which requires type assertions to extract values:

```go
// services/search/internal/index/index.go

func hitToDocument(hit interface{}) (model.BookDocument, error) {
    m, ok := hit.(map[string]interface{})
    if !ok {
        return model.BookDocument{}, fmt.Errorf("unexpected hit type")
    }

    doc := model.BookDocument{}
    if v, ok := m["id"].(string); ok {
        doc.ID = v
    }
    if v, ok := m["title"].(string); ok {
        doc.Title = v
    }
    // ... remaining fields
    if v, ok := m["published_year"].(float64); ok {
        doc.PublishedYear = int(v)
    }
    // ...
    return doc, nil
}
```

<!-- [COPY EDIT] "Go's less elegant areas" — "less elegant" slightly weak; alternative "rougher edges". Author voice kept. -->
<!-- [LINE EDIT] "In Go, you can use `json.Unmarshal` into a typed struct to avoid this, but the Meilisearch client returns raw `interface{}` hits, so manual extraction is necessary." — 34 words; clear. Keep. -->
<!-- [COPY EDIT] Please verify: some versions of `meilisearch-go` expose a `Decode`/`UnmarshalHits` helper or allow generic `SearchRaw` for typed decoding. If such a helper exists in the version used, a footnote recommending it would strengthen the section. -->
This is one of Go's less elegant areas. JSON unmarshaling into `interface{}` produces `float64` for all numbers (because JSON does not distinguish integers from floats). The `published_year.(float64)` assertion followed by `int()` conversion is the standard workaround. In Java, Jackson would handle this mapping automatically via annotations. In Go, you can use `json.Unmarshal` into a typed struct to avoid this, but the Meilisearch client returns raw `interface{}` hits, so manual extraction is necessary.

---

## Bootstrap: Initial Index Population

<!-- [STRUCTURAL] Bootstrap section is well-motivated. Good flow: why bootstrap → interface → Run function → explanation → tests. -->
When the search service starts for the first time, the Meilisearch index is empty. There are no Kafka events to replay (Kafka has a retention window, and events from before the search service existed are gone). We need to seed the index from the catalog.
<!-- [LINE EDIT] "Kafka has a retention window, and events from before the search service existed are gone" — factually correct (default 7-day retention). Tight. -->

The bootstrap package handles this:

```go
// services/search/internal/bootstrap/bootstrap.go

type IndexBootstrapper interface {
    EnsureIndex(ctx context.Context) error
    Count(ctx context.Context) (int64, error)
    Upsert(ctx context.Context, doc model.BookDocument) error
}

func Run(ctx context.Context, catalog catalogv1.CatalogServiceClient, svc IndexBootstrapper) error {
    if err := svc.EnsureIndex(ctx); err != nil {
        return err
    }

    count, err := svc.Count(ctx)
    if err != nil {
        return err
    }
    if count > 0 {
        log.Printf("search index already has %d documents, skipping bootstrap", count)
        return nil
    }

    log.Println("search index is empty, bootstrapping from catalog...")

    var page int32 = 1
    var total int64

    for {
        resp, err := catalog.ListBooks(ctx, &catalogv1.ListBooksRequest{
            Page:     page,
            PageSize: 100,
        })
        if err != nil {
            return err
        }

        for _, b := range resp.Books {
            doc := model.BookDocument{
                ID:    b.Id,
                Title: b.Title,
                // ... remaining fields
            }
            if err := svc.Upsert(ctx, doc); err != nil {
                log.Printf("failed to index book %s: %v", b.Id, err)
            }
            total++
        }

        if len(resp.Books) < 100 {
            break
        }
        page++
    }

    log.Printf("bootstrap complete: %d books indexed", total)
    return nil
}
```

The logic is straightforward:

1. **Ensure the index exists** with the correct attribute configuration.
2. **Check if the index already has documents.** If so, skip -- the index was already populated, either by a previous bootstrap or by Kafka events.
3. **Page through the catalog via gRPC**, upserting each book into Meilisearch.
<!-- [COPY EDIT] Numbered list style — bold lead-in phrases consistent. Good. -->

<!-- [LINE EDIT] "The `IndexBootstrapper` interface is another example of interface segregation. The bootstrap code needs `EnsureIndex`, `Count`, and `Upsert` -- it does not need `Search` or `Suggest`." — good callback to 8.2. -->
The `IndexBootstrapper` interface is another example of interface segregation. The bootstrap code needs `EnsureIndex`, `Count`, and `Upsert` -- it does not need `Search` or `Suggest`. Defining a narrow interface makes the mock trivial and the dependency explicit.

<!-- [LINE EDIT] "Notice that bootstrap errors on individual books are logged but do not stop the process." — good. -->
Notice that bootstrap errors on individual books are logged but do not stop the process. If one book fails to index (maybe it has unusual characters that Meilisearch rejects), we continue with the rest. The missing book will appear in the index the next time it is updated in the catalog.

### Testing Bootstrap

<!-- [STRUCTURAL] Two representative tests: empty-index path and populated-index path. Sufficient. -->
The bootstrap tests use mocks for both the search service and the catalog gRPC client:

```go
// services/search/internal/bootstrap/bootstrap_test.go

func TestBootstrap_SkipsWhenIndexHasDocuments(t *testing.T) {
    svc := &mockSearchService{count: 5}
    catalog := &mockCatalogClient{}

    err := bootstrap.Run(context.Background(), catalog, svc)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !svc.ensured {
        t.Error("expected EnsureIndex to be called")
    }
    if len(svc.upserted) != 0 {
        t.Errorf("expected no upserts when index is populated, got %d", len(svc.upserted))
    }
}

func TestBootstrap_IndexesAllBooksWhenEmpty(t *testing.T) {
    svc := &mockSearchService{count: 0}
    catalog := &mockCatalogClient{
        books: []*catalogv1.Book{
            {Id: "1", Title: "Go Book", Author: "Author1"},
            {Id: "2", Title: "Rust Book", Author: "Author2"},
        },
    }

    err := bootstrap.Run(context.Background(), catalog, svc)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(svc.upserted) != 2 {
        t.Fatalf("expected 2 upserts, got %d", len(svc.upserted))
    }
}
```

<!-- [LINE EDIT] "embed the interface to satisfy the compiler (all unimplemented methods will panic if called), then override the methods your test actually exercises" — good pattern description. -->
<!-- [COPY EDIT] Please verify: "The `mockCatalogClient` embeds the generated `CatalogServiceClient` interface and overrides only `ListBooks`." — the claim that embedded-interface unimplemented methods "panic" is partly tooling-dependent. In Go, embedding an interface in a struct creates a nil-pointer interface field; calling an unimplemented method will cause a nil pointer dereference panic, which is colloquially "panic if called". Accurate enough for the tutor context. -->
The `mockCatalogClient` embeds the generated `CatalogServiceClient` interface and overrides only `ListBooks`. This is a Go testing pattern worth knowing: embed the interface to satisfy the compiler (all unimplemented methods will panic if called), then override the methods your test actually exercises. In Java/Kotlin, Mockito handles this automatically with `when(...).thenReturn(...)`.

---

## Kafka Consumer: Real-Time Index Updates

<!-- [STRUCTURAL] Natural transition: bootstrap gets us to parity, consumer keeps us live. Good. -->
<!-- [COPY EDIT] "Real-Time" — heading case; compound modifier hyphenated before noun per CMOS 7.81. Good. -->
After bootstrap, the index stays current through a Kafka consumer that processes `catalog.books.changed` events:

```go
// services/search/internal/consumer/consumer.go

func Run(ctx context.Context, brokers []string, topic string, idx Indexer) error {
    config := sarama.NewConfig()
    config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
        sarama.NewBalanceStrategyRoundRobin(),
    }
    config.Consumer.Offsets.Initial = sarama.OffsetOldest

    group, err := sarama.NewConsumerGroup(brokers, "search-indexer", config)
    if err != nil {
        return fmt.Errorf("create consumer group: %w", err)
    }
    defer group.Close()

    handler := &consumerHandler{idx: idx}

    for {
        if err := group.Consume(ctx, []string{topic}, handler); err != nil {
            log.Printf("consumer error: %v", err)
        }
        if ctx.Err() != nil {
            return nil
        }
    }
}
```

Several configuration choices are significant:

<!-- [LINE EDIT] "Kafka tracks which messages each group has consumed." — good. -->
- **Consumer group: `"search-indexer"`** -- This is the Kafka consumer group ID. Kafka tracks which messages each group has consumed. If the search service restarts, it picks up where it left off. If you add a second consumer with a different group ID, both receive all messages independently.

<!-- [LINE EDIT] "belt-and-suspenders" — idiom, fine. -->
<!-- [COPY EDIT] "belt-and-suspenders" correctly hyphenated as a unit modifier (CMOS 7.81). Good. -->
- **`OffsetOldest`** -- When the consumer group starts for the first time (no committed offsets), begin from the oldest available message. Combined with bootstrap, this is belt-and-suspenders: bootstrap loads the current state, and `OffsetOldest` catches any events published between bootstrap completion and consumer start.

<!-- [LINE EDIT] "If you scale the search service to multiple instances, this strategy distributes Kafka partitions evenly across them." — good. -->
- **`NewBalanceStrategyRoundRobin()`** -- If you scale the search service to multiple instances, this strategy distributes Kafka partitions evenly across them.

<!-- [LINE EDIT] "The `Consume` call blocks until the context is cancelled or an error occurs." — "cancelled" UK spelling (see earlier house-style note). -->
The `Consume` call blocks until the context is cancelled or an error occurs. The outer `for` loop retries after transient errors (broker rebalancing, temporary network issues). When the context is cancelled (shutdown signal), the function returns cleanly.

### Event Processing

The `consumerHandler` implements Sarama's `ConsumerGroupHandler` interface. The interesting method is `ConsumeClaim`:

```go
// services/search/internal/consumer/consumer.go

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession,
    claim sarama.ConsumerGroupClaim) error {
    ctx := session.Context()
    for msg := range claim.Messages() {
        if err := handleEvent(ctx, h.idx, msg.Value); err != nil {
            log.Printf("failed to handle event: %v", err)
            continue
        }
        session.MarkMessage(msg, "")
    }
    return nil
}
```

<!-- [STRUCTURAL] The "skip failed messages" discussion is honest about the tradeoff; the exercise below asks students to implement a dead-letter strategy, which is the right pedagogy. -->
<!-- [LINE EDIT] "On failure, we log the error and **move on**. The message offset is not committed, so it would be redelivered on the next rebalance." — there's a subtle inconsistency. If the offset is not committed, the message will be redelivered — not "move on" semantically. Recommend tightening: "On failure, we log the error and continue without committing the offset. The message will be redelivered on the next rebalance (when a broker may reassign this partition to another consumer)." Flag for author review. -->
<!-- [COPY EDIT] "**move on**" in bold contradicts the next sentence's description of redelivery on rebalance. Clarify whether this is best-effort fire-and-forget (commit on success, skip on failure entirely, losing messages until restart) or at-least-once-ish (offsets advance only when MarkMessage is called, so failures retry on rebalance). The code shown does the latter; the prose suggests the former. Resolve. -->
For each message, we attempt to process it. On success, `MarkMessage` commits the offset -- Kafka will not deliver this message again. On failure, we log the error and **move on**. The message offset is not committed, so it would be redelivered on the next rebalance. In practice, for a search index, skipping a failed message is acceptable -- the next update to the same book will overwrite the stale data.

The `handleEvent` function deserializes the JSON and dispatches by event type:

```go
// services/search/internal/consumer/consumer.go

func handleEvent(ctx context.Context, idx Indexer, data []byte) error {
    var event bookEvent
    if err := json.Unmarshal(data, &event); err != nil {
        return fmt.Errorf("unmarshal event: %w", err)
    }

    switch event.EventType {
    case "book.created", "book.updated":
        doc := model.BookDocument{
            ID:              event.BookID,
            Title:           event.Title,
            Author:          event.Author,
            ISBN:            event.ISBN,
            Genre:           event.Genre,
            Description:     event.Description,
            PublishedYear:   event.PublishedYear,
            TotalCopies:     event.TotalCopies,
            AvailableCopies: event.AvailableCopies,
        }
        return idx.Upsert(ctx, doc)
    case "book.deleted":
        return idx.Delete(ctx, event.BookID)
    default:
        log.Printf("unknown event type: %s", event.EventType)
        return nil
    }
}
```

<!-- [LINE EDIT] "Both `book.created` and `book.updated` map to `Upsert` -- Meilisearch's `AddDocuments` method is an upsert (insert or replace) based on the primary key." — clear. -->
<!-- [COPY EDIT] "insert or replace" — accurate for Meilisearch `AddDocuments`. Good. -->
Both `book.created` and `book.updated` map to `Upsert` -- Meilisearch's `AddDocuments` method is an upsert (insert or replace) based on the primary key. There is no need to distinguish between creating a new document and updating an existing one.

<!-- [LINE EDIT] "Unknown event types are logged and ignored. This is forward-compatible: if a future version of the catalog service publishes a new event type (say, `book.archived`), the current consumer will not crash." — good. -->
<!-- [COPY EDIT] "forward-compatible" correctly hyphenated (CMOS 7.81). -->
Unknown event types are logged and ignored. This is forward-compatible: if a future version of the catalog service publishes a new event type (say, `book.archived`), the current consumer will not crash.

### Testing the Consumer

The consumer tests exercise `handleEvent` directly, bypassing Kafka entirely:

```go
// services/search/internal/consumer/consumer_test.go

func TestHandleEvent_BookCreated(t *testing.T) {
    idx := &mockIndexer{}
    err := handleEvent(context.Background(), idx, []byte(`{
        "event_type": "book.created",
        "book_id": "abc-123",
        "title": "Go Book",
        "author": "Author",
        "isbn": "1234567890",
        "genre": "programming",
        "total_copies": 5,
        "available_copies": 5
    }`))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(idx.upserted) != 1 {
        t.Fatalf("expected 1 upsert, got %d", len(idx.upserted))
    }
    if idx.upserted[0].Title != "Go Book" {
        t.Errorf("expected title 'Go Book', got %s", idx.upserted[0].Title)
    }
}

func TestHandleEvent_BookDeleted(t *testing.T) {
    idx := &mockIndexer{}
    err := handleEvent(context.Background(), idx, []byte(`{
        "event_type": "book.deleted",
        "book_id": "abc-123"
    }`))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(idx.deleted) != 1 {
        t.Fatalf("expected 1 delete, got %d", len(idx.deleted))
    }
    if idx.deleted[0] != "abc-123" {
        t.Errorf("expected deleted ID 'abc-123', got %s", idx.deleted[0])
    }
}
```

<!-- [LINE EDIT] "This is the advantage of extracting `handleEvent` as a standalone function: you can test the JSON -> index operation mapping without any Kafka infrastructure." — good. -->
<!-- [COPY EDIT] "JSON -> index" — ASCII arrow; consider Unicode → for consistency. -->
This is the advantage of extracting `handleEvent` as a standalone function: you can test the JSON -> index operation mapping without any Kafka infrastructure. The `mockIndexer` records what was called, and the tests assert the expected behavior.

---

## Async Indexing and Meilisearch Task IDs

<!-- [STRUCTURAL] Good advanced-topic sidebar. Right length. -->
<!-- [LINE EDIT] "One subtlety worth understanding: Meilisearch operations are **asynchronous**." — effective. -->
One subtlety worth understanding: Meilisearch operations are **asynchronous**. When you call `AddDocuments` or `DeleteDocument`, Meilisearch enqueues the operation and returns a task ID immediately. The document is not searchable until the task completes (typically under 100ms for single-document operations, longer for bulk imports).

<!-- [LINE EDIT] "Our code ignores the task ID -- we call `AddDocuments` and move on without waiting for completion." — clear. -->
<!-- [COPY EDIT] "100ms" — technical unit, numeral + unit without space is common in tech writing; house-style-dependent. Flag. -->
Our code ignores the task ID -- we call `AddDocuments` and move on without waiting for completion. This is acceptable for a real-time index fed by Kafka events: the latency between a catalog change and it appearing in search results is already measured in seconds (Kafka delivery + consumer processing). Adding a few more milliseconds of Meilisearch task processing does not meaningfully change the user experience.
<!-- [COPY EDIT] "real-time" correctly hyphenated as compound modifier (CMOS 7.81). -->

<!-- [LINE EDIT] "If you needed stronger guarantees (for example, in a test that indexes a document and immediately searches for it), you would use `WaitForTask`:" — good. -->
If you needed stronger guarantees (for example, in a test that indexes a document and immediately searches for it), you would use `WaitForTask`:

```go
taskInfo, err := idx.AddDocuments(docs, &meilisearch.DocumentOptions{PrimaryKey: &pk})
// taskInfo.TaskUID contains the task ID
task, err := client.WaitForTask(taskInfo.TaskUID)
// task.Status == "succeeded" means the document is now searchable
```

<!-- [COPY EDIT] Please verify: `WaitForTask(taskInfo.TaskUID)` signature. In meilisearch-go v0.28+, the signature is `WaitForTask(taskUID int64, interval time.Duration) (*Task, error)` or similar; recent versions may take a context. Verify against the pinned library version. The code example uses a single argument — may be incomplete. -->
<!-- [COPY EDIT] Please verify: `meilisearch.DocumentOptions` struct exists and has `PrimaryKey *string` field. Field name and type per current library version. -->

---

## Exercises

<!-- [STRUCTURAL] Four well-scaled exercises; the dead-letter one directly addresses the inconsistency flagged earlier in the Event Processing section. Good. -->
1. **Add a year range filter.** Extend `SearchFilters` with `MinYear` and `MaxYear` fields. Update `buildFilterString` to generate Meilisearch filter expressions like `published_year >= 2020 AND published_year <= 2025`. Remember to add `published_year` to the filterable attributes in `EnsureIndex`.

2. **Write an integration test with Meilisearch.** Using Docker Compose or `testcontainers-go`, start a Meilisearch instance, call `EnsureIndex`, upsert a few documents, and verify that `Search` returns the expected results. This requires waiting for Meilisearch tasks to complete -- use `WaitForTask`.

<!-- [LINE EDIT] "after N failures" — "N" acceptable in tutor context as a math-style variable; optional italic for clarity. -->
3. **Handle malformed JSON in the consumer.** Currently, if the Kafka message contains invalid JSON, `handleEvent` returns an error and the message is not acknowledged. Over time, the consumer will be stuck retrying the same bad message. Implement a dead-letter strategy: after N failures, log the raw message and commit the offset.

<!-- [COPY EDIT] "title:asc" / "published_year:desc" — Meilisearch sort parameter syntax. Verify format (some versions take arrays like `["title:asc"]`). -->
4. **Implement sorted search.** Add a `SortBy` field to `SearchFilters` and pass it to Meilisearch as a `Sort` parameter. Test with `title:asc` and `published_year:desc`.

---

## References

<!-- [COPY EDIT] Please verify: Meilisearch docs URLs. All four point to `www.meilisearch.com/docs/...` which is the current doc domain; paths `/learn/filtering_and_sorting/filter_expression_reference` and `/learn/async/asynchronous_operations` may have moved in recent docs reorganizations. Spot-check. -->
[^1]: [Meilisearch documentation](https://www.meilisearch.com/docs) -- Official reference for indexes, search parameters, filtering, and the REST API.
[^2]: [meilisearch-go -- GitHub](https://github.com/meilisearch/meilisearch-go) -- The official Go client library for Meilisearch.
[^3]: [Meilisearch -- Filtering](https://www.meilisearch.com/docs/learn/filtering_and_sorting/filter_expression_reference) -- Filter expression syntax, including comparison operators and boolean combinators.
[^4]: [IBM/sarama -- ConsumerGroup](https://pkg.go.dev/github.com/IBM/sarama#ConsumerGroup) -- Sarama consumer group documentation, including rebalancing strategies and offset management.
[^5]: [Meilisearch -- Asynchronous operations](https://www.meilisearch.com/docs/learn/async/asynchronous_operations) -- How Meilisearch task queuing works and when to use `WaitForTask`.
