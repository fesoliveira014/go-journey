# Chapter 7: Search Service & Meilisearch — Design Spec

## Goal

Add full-text search to the library system by (1) making the Catalog service publish `catalog.books.changed` events to Kafka, and (2) building a new Search service that consumes those events, indexes books in Meilisearch, and exposes Search/Suggest gRPC RPCs. The Gateway gets a search page with autocomplete.

## Architecture Overview

```
Admin creates/updates/deletes book
  → Catalog Service writes to PostgreSQL
  → Catalog Service publishes catalog.books.changed event to Kafka

Search Service (Kafka consumer)
  → consumes catalog.books.changed events
  → upserts/deletes documents in Meilisearch

Gateway → Search Service (gRPC: Search, Suggest)
  → renders search results page
  → renders autocomplete suggestions partial
```

**Reads are synchronous** — Gateway calls Search via gRPC for queries and suggestions.

**Writes are asynchronous** — Catalog publishes events to Kafka; Search consumes them and updates the Meilisearch index.

**Bootstrap** — On startup, if the Meilisearch index is empty, the Search service calls Catalog's `ListBooks` RPC in a paginated loop to build the initial index before accepting requests.

---

## Part 1: Catalog Event Publishing

### Changes to CatalogService

The existing `CatalogService` (`services/catalog/internal/service/catalog.go`) gains an `EventPublisher` dependency — the same pattern used in the Reservation service.

```go
type EventPublisher interface {
    Publish(ctx context.Context, event BookEvent) error
}

type BookEvent struct {
    EventType      string    `json:"event_type"`
    BookID         string    `json:"book_id"`
    Title          string    `json:"title,omitempty"`
    Author         string    `json:"author,omitempty"`
    ISBN           string    `json:"isbn,omitempty"`
    Genre          string    `json:"genre,omitempty"`
    Description    string    `json:"description,omitempty"`
    PublishedYear  int       `json:"published_year,omitempty"`
    TotalCopies    int       `json:"total_copies,omitempty"`
    AvailableCopies int     `json:"available_copies,omitempty"`
    Timestamp      time.Time `json:"timestamp"`
}
```

**Event types:**
- `book.created` — after successful `Create`. Carries full book state.
- `book.updated` — after successful `Update` or `UpdateAvailability`. Carries full book state.
- `book.deleted` — after successful `Delete`. Carries only `book_id`, `event_type`, `timestamp`.

**Kafka topic:** `catalog-books-changed`

**Message key:** `book_id` (ensures per-book ordering within a partition).

**Publish is fire-and-forget with logging** — same as Reservation. The write to PostgreSQL is the source of truth; a failed publish means the index will be temporarily stale but will catch up on subsequent events or a restart (bootstrap).

### New files

- `services/catalog/internal/kafka/publisher.go` — Sarama SyncProducer wrapping `EventPublisher`, keyed by `book_id`, `WaitForAll` acks.

### Modified files

- `services/catalog/internal/service/catalog.go` — `NewCatalogService` takes `EventPublisher` parameter. `CreateBook`, `UpdateBook`, `DeleteBook`, `UpdateAvailability` publish events after successful repo calls.
- `services/catalog/internal/service/catalog_test.go` — Add mock publisher, verify events are published with correct type and fields.
- `services/catalog/cmd/main.go` — Wire Kafka producer, pass to `NewCatalogService`.

### UpdateAvailability event detail

When the Catalog consumer processes a reservation event and calls `UpdateAvailability`, the service needs the full book state for the event payload. After the availability delta is applied, the service calls `repo.GetByID` to fetch the updated book, then publishes `book.updated`. This means each reservation event that changes availability triggers a `catalog.books.changed` event — this is how Search learns about availability changes.

---

## Part 2: Search Service

### Proto Definition

File: `proto/search/v1/search.proto`

```protobuf
syntax = "proto3";
package search.v1;
option go_package = "github.com/fesoliveira014/library-system/gen/search/v1;searchv1";

service SearchService {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc Suggest(SuggestRequest) returns (SuggestResponse);
}

message SearchRequest {
  string query = 1;
  string genre = 2;          // optional filter
  string author = 3;         // optional filter
  bool available_only = 4;
  int32 page = 5;
  int32 page_size = 6;
}

message SearchResponse {
  repeated BookResult books = 1;
  int64 total_hits = 2;
  int64 query_time_ms = 3;
}

message BookResult {
  string id = 1;
  string title = 2;
  string author = 3;
  string isbn = 4;
  string genre = 5;
  string description = 6;
  int32 published_year = 7;
  int32 total_copies = 8;
  int32 available_copies = 9;
}

message SuggestRequest {
  string prefix = 1;
  int32 limit = 2;
}

message SuggestResponse {
  repeated Suggestion suggestions = 1;
}

message Suggestion {
  string book_id = 1;
  string title = 2;
  string author = 3;
}
```

### Service Structure

```
services/search/
├── cmd/main.go
├── go.mod
├── internal/
│   ├── model/model.go             # BookDocument struct
│   ├── index/index.go             # IndexRepository interface + Meilisearch impl
│   ├── service/service.go         # SearchService
│   ├── service/service_test.go
│   ├── handler/handler.go         # gRPC handler
│   ├── handler/handler_test.go
│   ├── consumer/consumer.go       # Kafka consumer
│   ├── consumer/consumer_test.go
│   └── bootstrap/bootstrap.go     # Full sync from Catalog
├── Dockerfile
├── Dockerfile.dev
├── .air.toml
└── Earthfile
```

### Model

```go
// BookDocument is the shape stored in and retrieved from Meilisearch.
type BookDocument struct {
    ID              string `json:"id"`
    Title           string `json:"title"`
    Author          string `json:"author"`
    ISBN            string `json:"isbn"`
    Genre           string `json:"genre"`
    Description     string `json:"description"`
    PublishedYear   int    `json:"published_year"`
    TotalCopies     int    `json:"total_copies"`
    AvailableCopies int    `json:"available_copies"`
}
```

### Index Layer

The `index` package replaces the `repository` package — same concept (data access abstraction), different backing store.

```go
type IndexRepository interface {
    Upsert(ctx context.Context, doc model.BookDocument) error
    Delete(ctx context.Context, id string) error
    Search(ctx context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
    Suggest(ctx context.Context, prefix string, limit int) ([]model.BookDocument, error)
    Count(ctx context.Context) (int64, error)
    EnsureIndex(ctx context.Context) error
}

type SearchFilters struct {
    Genre         string
    Author        string
    AvailableOnly bool
}
```

**Meilisearch implementation details:**
- Index name: `books`
- Primary key: `id`
- Searchable attributes: `title`, `author`, `isbn`, `description`, `genre`
- Filterable attributes: `genre`, `author`, `available_copies`
- Sortable attributes: `title`, `published_year`
- `EnsureIndex` creates the index and configures attributes if it doesn't exist
- `Search` builds a Meilisearch filter string from `SearchFilters` (e.g., `genre = "fiction" AND available_copies > 0`)
- `Suggest` uses Meilisearch search with a low limit and returns only id/title/author
- `Count` returns the number of documents in the index (used by bootstrap to check if empty)

### Service Layer

```go
type SearchService struct {
    index IndexRepository
}

func (s *SearchService) Search(ctx, query, filters, page, pageSize) → ([]BookDocument, totalHits, queryTimeMs, error)
func (s *SearchService) Suggest(ctx, prefix, limit) → ([]Suggestion, error)
func (s *SearchService) Upsert(ctx, BookDocument) → error
func (s *SearchService) Delete(ctx, id) → error
```

The service layer is thin — mostly delegates to the index. Its value is as the dependency boundary: the handler depends on a `Service` interface, the consumer depends on an `Indexer` interface (subset: `Upsert` + `Delete`), and the bootstrap depends on the full service.

### gRPC Handler

Maps proto requests to service calls. Applies default pagination (page 1, size 20, max 100). Returns `codes.InvalidArgument` for empty queries on Search.

### Kafka Consumer

Same pattern as Catalog's consumer from Chapter 6:
- Consumer group: `search-indexer`
- Topic: `catalog-books-changed`
- Deserializes JSON payload into `BookEvent`
- `book.created` / `book.updated` → `Upsert` with `BookDocument` built from event fields
- `book.deleted` → `Delete` with `book_id`
- Uses `session.Context()` for cancellation propagation
- Narrow `Indexer` interface (not full `SearchService`)

### Bootstrap

```go
func Bootstrap(ctx context.Context, catalog catalogv1.CatalogServiceClient, svc *service.SearchService, index index.IndexRepository) error
```

1. Call `index.EnsureIndex(ctx)` to create index + configure attributes
2. Call `index.Count(ctx)` — if > 0, skip (index already populated)
3. Paginate through `catalog.ListBooks(ctx, page)` until all books fetched
4. For each book, call `svc.Upsert(ctx, bookDocument)`
5. Log progress (every 100 books)
6. Return nil on success, error on failure

Called from `main.go` before starting the Kafka consumer and gRPC server. If Catalog is unreachable, log the error and start anyway with an empty index — events will populate it as books are created/updated.

---

## Part 3: Gateway UI

### New Routes

- `GET /search` — renders search page. Query params: `q`, `genre`, `author`, `available`, `page`.
- `GET /search/suggest` — returns HTML partial. Query param: `prefix`. Minimum 2 characters.

### Search Page Template (`search.html`)

- Text input for query (pre-filled from `q` param)
- Genre dropdown filter
- Author text filter
- "Available only" checkbox
- Results table: title (links to `/books/{id}`), author, genre, availability
- Pagination controls
- Query time display (e.g., "42 results in 3ms")
- Empty state when no results

### Autocomplete (`partials/suggestions.html`)

- Rendered as a dropdown list below the nav search input
- Each item: book title + author, links to `/books/{id}`
- Rendered via `renderPartial` (no base layout)

### Nav Bar Changes (`partials/nav.html`)

- Add search input with HTMX attributes:
  - `hx-get="/search/suggest"` with `hx-trigger="keyup changed delay:300ms"` and `hx-target="#suggestions"`
  - `hx-params` sends `prefix` param
  - Minimum length enforced client-side via `hx-trigger` with `[this.value.length >= 2]` filter
  - `name="prefix"` on the input
- Dropdown container `<div id="suggestions">` positioned below the input
- Pressing Enter submits to `/search?q=...` (standard form submission, not HTMX)

### Server Changes

- `Server` struct gains `search searchv1.SearchServiceClient` field (5th field)
- `New()` takes 5 arguments: auth, catalog, reservation, search, tmpl
- All existing test files update `handler.New()` calls to pass `nil` for search client

### New Files

- `services/gateway/internal/handler/search.go`
- `services/gateway/internal/handler/search_test.go`
- `services/gateway/templates/search.html`
- `services/gateway/templates/partials/suggestions.html`

### Modified Files

- `services/gateway/internal/handler/server.go` — 5-field Server struct
- `services/gateway/templates/partials/nav.html` — search input + HTMX
- `services/gateway/cmd/main.go` — wire search gRPC connection + routes
- `services/gateway/internal/handler/auth_test.go` — update New() calls
- `services/gateway/internal/handler/catalog_test.go` — update New() calls
- `services/gateway/internal/handler/reservation_test.go` — update New() calls
- `services/gateway/internal/handler/render_test.go` — update New() calls
- `services/gateway/internal/handler/health_test.go` — update New() calls

---

## Part 4: Infrastructure

### Docker Compose additions

**meilisearch service:**
```yaml
meilisearch:
  image: getmeili/meilisearch:v1.12
  environment:
    MEILI_ENV: development
    MEILI_NO_ANALYTICS: "true"
    MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-dev-master-key-change-in-production}
  ports:
    - "${MEILI_PORT:-7700}:7700"
  volumes:
    - meilisearch-data:/meili_data
  healthcheck:
    test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:7700/health"]
    interval: 5s
    timeout: 5s
    retries: 5
  networks:
    - library-net
```

**search service:**
```yaml
search:
  build:
    context: ..
    dockerfile: services/search/Dockerfile
  environment:
    GRPC_PORT: "50054"
    MEILI_URL: ${MEILI_URL:-http://meilisearch:7700}
    MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-dev-master-key-change-in-production}
    KAFKA_BROKERS: ${KAFKA_BROKERS:-kafka:9092}
    CATALOG_GRPC_ADDR: ${CATALOG_GRPC_ADDR:-catalog:50052}
  ports:
    - "${SEARCH_GRPC_PORT:-50054}:50054"
  depends_on:
    meilisearch:
      condition: service_healthy
    kafka:
      condition: service_healthy
  networks:
    - library-net
```

**gateway update:** Add `SEARCH_GRPC_ADDR` env var, add `search` to `depends_on`.

**New volume:** `meilisearch-data`

### Environment variables (`deploy/.env`)

```
MEILI_URL=http://meilisearch:7700
MEILI_MASTER_KEY=dev-master-key-change-in-production
MEILI_PORT=7700
SEARCH_GRPC_ADDR=search:50054
SEARCH_GRPC_PORT=50054
```

### Docker files

- `services/search/Dockerfile` — multi-stage build, same pattern as reservation. Port 50054.
- `services/search/Dockerfile.dev` — air-based hot reload.
- `services/search/.air.toml` — standard air config.

### Earthfile

- `services/search/Earthfile` — deps (with gen + pkg/auth copies), src, lint, test, build, docker targets. Same pattern as reservation.
- Root `Earthfile` — add `search` to ci, lint, test targets.

### Documentation

- `docs/src/ch07/index.md` — chapter overview
- `docs/src/ch07/catalog-events.md` — stub for Catalog event publishing section
- `docs/src/ch07/search-service.md` — stub for Search service section
- `docs/src/ch07/meilisearch.md` — stub for Meilisearch integration section
- `docs/src/ch07/search-ui.md` — stub for Gateway search UI section
- `docs/src/SUMMARY.md` — append Chapter 7 entries

---

## Simplifications & Known Limitations

1. **No Meilisearch API key rotation.** The master key is set via env var. Production would use tenant tokens or API keys with restricted permissions.

2. **Bootstrap is best-effort.** If Catalog is down at startup, the Search service starts with an empty index. It catches up via Kafka events. A full re-index requires restarting the service with an empty Meilisearch volume.

3. **No search result highlighting.** Meilisearch supports returning highlighted matches. We skip this for simplicity — the tutorial can mention it as an exercise.

4. **Suggest uses search, not a dedicated autocomplete index.** Meilisearch's search is fast enough for autocomplete use. A production system might use a prefix trie or dedicated suggest index.

5. **Eventual consistency.** Search results may be slightly stale. The reservation flow validates availability via gRPC to Catalog before confirming, so correctness is maintained despite stale search results.

6. **UpdateAvailability double-event.** Each reservation triggers both a `reservations.*` event (consumed by Catalog) and a resulting `catalog.books.changed` event (consumed by Search). This is intentional — it decouples the event chains and means Search never needs to understand reservation semantics.

---

## Testing Strategy

- **Catalog service tests:** Mock `EventPublisher`, verify correct event type/fields published after each operation. Existing tests updated for new `NewCatalogService` signature.
- **Search index tests:** Mock `IndexRepository`, test service logic (pagination defaults, filter building).
- **Search handler tests:** Mock `Service` interface, test gRPC request/response mapping.
- **Search consumer tests:** Internal tests (same package) for `handleEvent`, testing upsert on created/updated and delete on deleted events. Same pattern as Catalog consumer.
- **Gateway search tests:** Mock `SearchServiceClient`, test search page rendering and suggest partial rendering.
- **No integration tests with live Meilisearch** — deferred to Chapter 9 (Testing Strategies).
