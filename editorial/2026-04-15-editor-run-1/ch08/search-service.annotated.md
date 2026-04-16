# 8.2 Search Service

<!-- [STRUCTURAL] Strong opening: mission statement (single responsibility), transport (gRPC), data-source boundary (no ownership, projection kept in sync). Spring analogy is apt. -->
The search service is a standalone microservice with a single responsibility: full-text search over the book catalog. It exposes two gRPC RPCs -- `Search` and `Suggest` -- and delegates all indexing work to an abstracted search engine interface. The service does not own any book data; it maintains a read-optimized projection of the catalog, kept in sync through Kafka events (section 8.1) and a bootstrap mechanism (section 8.3).
<!-- [LINE EDIT] "The service does not own any book data" — good. -->
<!-- [COPY EDIT] "gRPC RPCs" conventional; see index.md note. -->
<!-- [COPY EDIT] "read-optimized" — compound adjective before noun correctly hyphenated (CMOS 7.81). Good. -->

<!-- [LINE EDIT] "handler (transport) -> service (business logic) -> repository (data access)" uses ASCII arrows. If the toolchain renders Markdown → to an actual arrow elsewhere in the chapter (e.g. "proto → index → service → handler → gateway" in index.md), prefer consistency: either ASCII `->` everywhere or Unicode `→` everywhere. -->
If you are coming from Spring, think of this service as a `@Service` + `@RestController` pair, except the transport layer is gRPC instead of HTTP, and the data store is a search engine instead of a relational database. The layering is the same: handler (transport) -> service (business logic) -> repository (data access).

---

## Architecture

<!-- [STRUCTURAL] Diagram is clean; arrow direction and layer labels both helpful. -->
```
gRPC Client (Gateway)
       |
  SearchHandler       (transport: proto <-> domain)
       |
  SearchService       (business logic: pagination, limits)
       |
  IndexRepository     (interface: search engine abstraction)
       |
  MeilisearchIndex    (concrete: meilisearch-go client)
```

<!-- [LINE EDIT] "Each layer depends only on the one below it, and only through an interface." — tight. -->
Each layer depends only on the one below it, and only through an interface. The handler does not know about Meilisearch. The service does not know about gRPC. This makes every layer independently testable.

---

## The BookDocument Model

<!-- [STRUCTURAL] Good rationale for separate model. Concise. -->
The search service defines its own document model, separate from the catalog's `model.Book`. This is intentional -- the search index stores a denormalized, search-optimized view of the data:

```go
// services/search/internal/model/model.go

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

<!-- [LINE EDIT] "The JSON tags matter: they match the Meilisearch document field names exactly." — strong. -->
The JSON tags matter: they match the Meilisearch document field names exactly. Meilisearch uses `id` as the default primary key, so the `ID` field maps to `"id"` in JSON.

There is also a lightweight `Suggestion` type for autocomplete responses:

```go
// services/search/internal/model/model.go

type Suggestion struct {
    BookID string
    Title  string
    Author string
}
```

`Suggestion` has no JSON tags because it never gets serialized to JSON directly -- it is used only as an internal transfer type between the service and handler layers.

---

## The IndexRepository Interface

<!-- [STRUCTURAL] Explicit boundary claim is valuable. Four-value tuple return is justified. -->
The service layer talks to the search engine through an interface defined in the `index` package:

```go
// services/search/internal/index/index.go

type IndexRepository interface {
    Upsert(ctx context.Context, doc model.BookDocument) error
    Delete(ctx context.Context, id string) error
    Search(ctx context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
    Suggest(ctx context.Context, prefix string, limit int) ([]model.BookDocument, error)
    Count(ctx context.Context) (int64, error)
    EnsureIndex(ctx context.Context) error
}
```

<!-- [LINE EDIT] "This interface is the boundary between 'search service logic' and 'search engine specifics.'" — Good. Note single-quote vs double-quote: chapter uses double quotes in prose elsewhere; here single quotes used for inline phrases. Recommend consistent curly double-quotes (CMOS 6.9/11.8). -->
<!-- [COPY EDIT] Single quotes 'search service logic' / 'search engine specifics' → double quotes "search service logic" / "search engine specifics". CMOS 11.8 — single quotes reserved for quotes within quotes. -->
This interface is the boundary between "search service logic" and "search engine specifics." If you wanted to swap Meilisearch for Elasticsearch, Typesense, or even an in-memory implementation for tests, you would only need to write a new struct that satisfies `IndexRepository`.
<!-- [COPY EDIT] Product names: "Meilisearch", "Elasticsearch", "Typesense" all correctly capitalized. -->

<!-- [LINE EDIT] "Returning the query time as a separate value (rather than burying it in a response struct) keeps the interface simple and lets the handler pass it directly to the gRPC response." — clear. -->
The `Search` method returns four values: `(docs, totalHits, queryTimeMs, error)`. Returning the query time as a separate value (rather than burying it in a response struct) keeps the interface simple and lets the handler pass it directly to the gRPC response.

The `SearchFilters` struct captures optional filter parameters:

```go
// services/search/internal/index/index.go

type SearchFilters struct {
    Genre         string
    Author        string
    AvailableOnly bool
}
```

<!-- [LINE EDIT] "Using a struct means the compiler catches typos in filter names and makes the available filters self-documenting." — good. -->
This is a plain struct, not a map. Using a struct means the compiler catches typos in filter names and makes the available filters self-documenting.

---

## The Service Layer

<!-- [STRUCTURAL] Good: constants, struct, constructor, then method examples. Expected Go layout. -->
The `SearchService` is thin by design. It enforces pagination defaults and limits, then delegates to the index:

```go
// services/search/internal/service/service.go

const (
    defaultPageSize = 20
    maxPageSize     = 100
)

type SearchService struct {
    index index.IndexRepository
}

func NewSearchService(idx index.IndexRepository) *SearchService {
    return &SearchService{index: idx}
}
```

The `Search` method normalizes pagination parameters:

```go
// services/search/internal/service/service.go

func (s *SearchService) Search(ctx context.Context, query string, filters index.SearchFilters,
    page, pageSize int) ([]model.BookDocument, int64, int64, error) {
    if page <= 0 {
        page = 1
    }
    if pageSize <= 0 {
        pageSize = defaultPageSize
    }
    if pageSize > maxPageSize {
        pageSize = maxPageSize
    }
    return s.index.Search(ctx, query, filters, page, pageSize)
}
```

<!-- [LINE EDIT] "This is defensive programming at the service boundary." — punchy. Keep. -->
<!-- [COPY EDIT] "10,000" — numeric with thousands separator, correct per CMOS 9.56 for four-digit figures in technical prose. -->
This is defensive programming at the service boundary. The caller (the gRPC handler) passes whatever the client sent. The service ensures the values are sane before they reach the search engine. A `pageSize` of 0 becomes 20; a `pageSize` of 10,000 becomes 100.

The `Suggest` method follows the same pattern, capping the limit and converting `BookDocument` results into `Suggestion` values:

```go
// services/search/internal/service/service.go

func (s *SearchService) Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error) {
    if limit <= 0 {
        limit = 5
    }
    if limit > 10 {
        limit = 10
    }

    docs, err := s.index.Suggest(ctx, prefix, limit)
    if err != nil {
        return nil, err
    }

    suggestions := make([]model.Suggestion, len(docs))
    for i, d := range docs {
        suggestions[i] = model.Suggestion{
            BookID: d.ID,
            Title:  d.Title,
            Author: d.Author,
        }
    }
    return suggestions, nil
}
```

<!-- [STRUCTURAL] Explanation of pass-through methods is clear and defends the "single entry point" design decision. Good. -->
<!-- [LINE EDIT] "These do not add business logic; they exist so that the consumer and bootstrap code depend on the service rather than reaching directly into the index layer." — 32 words, OK. -->
The service also exposes `Upsert`, `Delete`, `EnsureIndex`, and `Count` -- pass-through methods used by the Kafka consumer and bootstrap logic. These do not add business logic; they exist so that the consumer and bootstrap code depend on the service rather than reaching directly into the index layer. This keeps the dependency graph clean: everything flows through `SearchService`.

---

## The gRPC Handler

<!-- [STRUCTURAL] Excellent section. The interface-segregation callout is a highlight of the chapter. -->
The handler translates between gRPC (protobuf types) and the service layer (Go domain types). It defines its own `Service` interface -- a subset of what `SearchService` provides:

```go
// services/search/internal/handler/handler.go

type Service interface {
    Search(ctx context.Context, query string, filters index.SearchFilters,
        page, pageSize int) ([]model.BookDocument, int64, int64, error)
    Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error)
}

type SearchHandler struct {
    searchv1.UnimplementedSearchServiceServer
    svc Service
}

func NewSearchHandler(svc Service) *SearchHandler {
    return &SearchHandler{svc: svc}
}
```

<!-- [LINE EDIT] "Defining a narrow `Service` interface in the handler package rather than depending on `*service.SearchService` directly is a Go idiom worth highlighting." — clear, 23 words. Keep. -->
Defining a narrow `Service` interface in the handler package rather than depending on `*service.SearchService` directly is a Go idiom worth highlighting. The handler only needs `Search` and `Suggest` -- it does not need `Upsert`, `Delete`, or `EnsureIndex`. By declaring exactly what it needs, the handler:

<!-- [COPY EDIT] "Documents its dependencies precisely (you can read the interface to know what the handler uses)." — good. -->
1. Documents its dependencies precisely (you can read the interface to know what the handler uses).
<!-- [COPY EDIT] "Makes testing trivial (the mock only needs two methods, not seven)." — spells out "two" and "seven" correctly per CMOS 9.2 for numbers under 100 in prose. Good. -->
2. Makes testing trivial (the mock only needs two methods, not seven).
<!-- [COPY EDIT] "Interface Segregation Principle" — bolded; capitalization as a named principle is acceptable (CMOS 8.79). Good. -->
3. Follows the **Interface Segregation Principle** -- depend on the smallest interface that satisfies your needs.

<!-- [LINE EDIT] "In Java, this pattern exists too (you can `@Autowired` an interface instead of a concrete class), but it is less common because Java developers tend to create one interface per implementation class." — 39 words. OK. -->
<!-- [COPY EDIT] "In Go, interfaces are defined by the consumer, not the provider -- a cultural difference that leads to smaller, more focused interfaces." — strong closer. -->
In Java, this pattern exists too (you can `@Autowired` an interface instead of a concrete class), but it is less common because Java developers tend to create one interface per implementation class. In Go, interfaces are defined by the consumer, not the provider -- a cultural difference that leads to smaller, more focused interfaces.

### The Search RPC

```go
// services/search/internal/handler/handler.go

func (h *SearchHandler) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
    if req.GetQuery() == "" {
        return nil, status.Error(codes.InvalidArgument, "query is required")
    }

    filters := index.SearchFilters{
        Genre:         req.GetGenre(),
        Author:        req.GetAuthor(),
        AvailableOnly: req.GetAvailableOnly(),
    }

    docs, totalHits, queryTimeMs, err := h.svc.Search(
        ctx, req.GetQuery(), filters, int(req.GetPage()), int(req.GetPageSize()))
    if err != nil {
        return nil, status.Error(codes.Internal, "search failed")
    }

    books := make([]*searchv1.BookResult, len(docs))
    for i, d := range docs {
        books[i] = &searchv1.BookResult{
            Id:              d.ID,
            Title:           d.Title,
            Author:          d.Author,
            Isbn:            d.ISBN,
            Genre:           d.Genre,
            Description:     d.Description,
            PublishedYear:   int32(d.PublishedYear),
            TotalCopies:     int32(d.TotalCopies),
            AvailableCopies: int32(d.AvailableCopies),
        }
    }

    return &searchv1.SearchResponse{
        Books:       books,
        TotalHits:   totalHits,
        QueryTimeMs: queryTimeMs,
    }, nil
}
```

<!-- [LINE EDIT] "The handler's job is mechanical: validate input, translate protobuf -> domain types, call the service, translate domain types -> protobuf." — arrows to → for consistency if toolchain supports Unicode arrows. Tone good. -->
The handler's job is mechanical: validate input, translate protobuf -> domain types, call the service, translate domain types -> protobuf. There is no business logic here. The `int32` casts on `PublishedYear`, `TotalCopies`, and `AvailableCopies` are necessary because protobuf uses fixed-width integer types while Go's `int` is platform-dependent.
<!-- [COPY EDIT] "fixed-width" correctly hyphenated as compound modifier before noun (CMOS 7.81). -->
<!-- [COPY EDIT] "platform-dependent" correctly hyphenated (CMOS 7.81). -->

<!-- [LINE EDIT] "Error handling follows gRPC conventions: return `status.Error` with an appropriate `codes.*` constant." — clear. -->
<!-- [COPY EDIT] "It does **not** leak implementation details" — bold on "not" effective for tutor emphasis. Keep. -->
Error handling follows gRPC conventions: return `status.Error` with an appropriate `codes.*` constant. The handler maps empty queries to `InvalidArgument` and internal failures to `Internal`. It does **not** leak implementation details (the error message says "search failed", not "meilisearch connection refused").
<!-- [COPY EDIT] "meilisearch connection refused" — inside quotes as an example error message; since it's an example server-emitted string, lowercase is fine. -->

### The Suggest RPC

```go
// services/search/internal/handler/handler.go

func (h *SearchHandler) Suggest(ctx context.Context, req *searchv1.SuggestRequest) (*searchv1.SuggestResponse, error) {
    if req.GetPrefix() == "" {
        return nil, status.Error(codes.InvalidArgument, "prefix is required")
    }

    suggestions, err := h.svc.Suggest(ctx, req.GetPrefix(), int(req.GetLimit()))
    if err != nil {
        return nil, status.Error(codes.Internal, "suggest failed")
    }

    pbSuggestions := make([]*searchv1.Suggestion, len(suggestions))
    for i, s := range suggestions {
        pbSuggestions[i] = &searchv1.Suggestion{
            BookId: s.BookID,
            Title:  s.Title,
            Author: s.Author,
        }
    }

    return &searchv1.SuggestResponse{Suggestions: pbSuggestions}, nil
}
```

Same pattern: validate, delegate, translate.

---

## Testing the Handler

<!-- [STRUCTURAL] Tests section is well-placed: shows mock, shows a real assertion, contrasts with Mockito. -->
The handler tests use a mock that satisfies the `Service` interface:

```go
// services/search/internal/handler/handler_test.go

type mockService struct {
    searchDocs  []model.BookDocument
    suggestions []model.Suggestion
    totalHits   int64
    queryTimeMs int64
}

func (m *mockService) Search(_ context.Context, _ string, _ index.SearchFilters,
    _, _ int) ([]model.BookDocument, int64, int64, error) {
    return m.searchDocs, m.totalHits, m.queryTimeMs, nil
}

func (m *mockService) Suggest(_ context.Context, _ string, _ int) ([]model.Suggestion, error) {
    return m.suggestions, nil
}
```

<!-- [LINE EDIT] "Compare this to mocking a Spring `@Service` class with Mockito -- conceptually similar, but here there is no framework, no annotation, no proxy generation. The mock is a plain struct that you write by hand." — punchy, good. -->
Because the `Service` interface only has two methods, the mock is trivial. Compare this to mocking a Spring `@Service` class with Mockito -- conceptually similar, but here there is no framework, no annotation, no proxy generation. The mock is a plain struct that you write by hand.

The tests verify both the happy path and input validation:

```go
// services/search/internal/handler/handler_test.go

func TestSearchHandler_Search_EmptyQuery(t *testing.T) {
    h := handler.NewSearchHandler(&mockService{})

    _, err := h.Search(context.Background(), &searchv1.SearchRequest{Query: ""})
    if err == nil {
        t.Fatal("expected error for empty query")
    }
    st, _ := status.FromError(err)
    if st.Code() != codes.InvalidArgument {
        t.Errorf("expected InvalidArgument, got %v", st.Code())
    }
}
```

<!-- [LINE EDIT] "This test calls the handler method directly -- no gRPC server, no network, no dialing." — rhythm works. -->
This test calls the handler method directly -- no gRPC server, no network, no dialing. The handler is just a Go struct with methods. You can test it like any other function. This is one of the advantages of the layered architecture: the handler does not depend on the gRPC server infrastructure, only on the generated protobuf types.
<!-- [LINE EDIT] "just" is on the "cut list" but here "just a Go struct with methods" is idiomatic and rhetorical (a deflationary "just"). Keep in author voice. -->

---

## Wiring It Together in main.go

<!-- [STRUCTURAL] Good wrap-up: shows main.go composition and explicitly points out the goroutine / blocking model. -->
The `main.go` file constructs all dependencies and starts the server:

```go
// services/search/cmd/main.go

idx := index.NewMeilisearchIndex(meiliURL, meiliKey)
searchSvc := service.NewSearchService(idx)

// Bootstrap: sync from catalog if index is empty
if err := bootstrap.Run(ctx, catalogClient, searchSvc); err != nil {
    log.Printf("bootstrap failed (starting with empty index): %v", err)
}

// Start Kafka consumer in a goroutine
if kafkaBrokers != "" {
    brokers := strings.Split(kafkaBrokers, ",")
    go func() {
        if err := consumer.Run(ctx, brokers, "catalog.books.changed", searchSvc); err != nil {
            log.Printf("kafka consumer error: %v", err)
        }
    }()
}

// Start gRPC server
searchHandler := handler.NewSearchHandler(searchSvc)
grpcServer := grpc.NewServer()
searchv1.RegisterSearchServiceServer(grpcServer, searchHandler)
```

<!-- [LINE EDIT] "Notice the flow: `index.NewMeilisearchIndex` -> `service.NewSearchService` -> `handler.NewSearchHandler`." — Unicode → vs ASCII -> see earlier note. -->
Notice the flow: `index.NewMeilisearchIndex` -> `service.NewSearchService` -> `handler.NewSearchHandler`. Each constructor takes exactly the dependencies it needs. The Kafka consumer and bootstrap both depend on `searchSvc` (not on the index directly), maintaining the single entry point for all write operations.

<!-- [LINE EDIT] "it blocks indefinitely, processing messages until the context is cancelled" — "cancelled" uses UK spelling. The chapter uses "cancelled" in meilisearch.md too. Consistent internally. US-English "canceled" per Merriam-Webster; CMOS 7.60 is flexible. Flag for house-style decision. -->
<!-- [COPY EDIT] "cancelled" (UK) vs "canceled" (US). Chapter uses "cancelled" consistently; pick and document the house style. CMOS allows either but recommends consistency. -->
The Kafka consumer runs in a goroutine -- it blocks indefinitely, processing messages until the context is cancelled. The gRPC server runs on the main goroutine. If the consumer crashes, the service continues to serve search requests from whatever is already in the index.

---

## Exercises

<!-- [STRUCTURAL] Four exercises, nicely graduated. Good. -->
1. **Add a `GetDocument` method to `SearchService`.** Implement a method that retrieves a single book document by ID from the index. Define the method on the `IndexRepository` interface, implement it in `MeilisearchIndex`, and expose it through a new `GetBook` gRPC RPC.

2. **Write a test for the `Suggest` handler.** The existing tests cover the happy path and empty prefix. Write a test that verifies the handler correctly maps multiple suggestions from the service to protobuf `Suggestion` messages.

<!-- [LINE EDIT] "Examine the `UnimplementedSearchServiceServer` embedding." — good pedagogical exercise. Keep. -->
3. **Examine the `UnimplementedSearchServiceServer` embedding.** What happens if you add a new RPC to the proto file, regenerate the code, but forget to implement it in the handler? Try it: add a `GetStats` RPC to the proto, regenerate, and call it with `grpcurl`. What error code do you get?

<!-- [COPY EDIT] "7 methods" vs "seven methods" — here "7" is used in "The handler defines its own `Service` interface with 2 methods, while `SearchService` has 7." CMOS 9.2 prefers spelling out numbers under 100 in prose. Recommend "two methods, while `SearchService` has seven." -->
4. **Compare the interface segregation approach.** The handler defines its own `Service` interface with 2 methods, while `SearchService` has 7. In Java/Kotlin, would you typically create a separate interface for the controller's use? What are the tradeoffs?

---

## References

[^1]: [gRPC Go -- Basics tutorial](https://grpc.io/docs/languages/go/basics/) -- Official guide for implementing gRPC services in Go, including server setup and handler registration.
<!-- [COPY EDIT] Please verify: URL for Go Wiki interfaces section. "CodeReviewComments#interfaces" is the long-standing URL but some Go Wiki URLs moved after the go.dev migration. Verify the anchor `#interfaces` still resolves. -->
[^2]: [Go Wiki -- Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces) -- The Go team's recommendation on where to define interfaces: at the call site, not the implementation site.
<!-- [COPY EDIT] Please verify: URL `https://grpc.github.io/grpc/core/md_doc_statuscodes.html` — this is a C-core doc path. The canonical reference many Go teams use is `https://github.com/grpc/grpc/blob/master/doc/statuscodes.md` or `grpc.io/docs/guides/error/`. Verify the link still resolves. -->
[^3]: [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html) -- Reference for all gRPC status codes and when to use each one.
<!-- [COPY EDIT] Please verify: URL to "Robert C. Martin -- Interface Segregation Principle" uses web.archive.org with an ambiguous path (`web/2020*/...`). That is not a valid archive URL (the `*` should be a timestamp). Recommend replacing with a direct archive.org snapshot or a more stable reference (e.g., *Agile Software Development, Principles, Patterns, and Practices*, Martin 2002). -->
[^4]: [Robert C. Martin -- Interface Segregation Principle](https://web.archive.org/web/2020*/https://drive.google.com/file/d/0BwhCYaYDn8EgOTViYjJhYzMtMzYxMC00MzFjLWJjMzYtOGJiMDc5N2JkYmJi/view) -- The original ISP paper, part of the SOLID principles.
