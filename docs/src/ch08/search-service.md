# 8.2 Search Service

The Search Service is a standalone microservice with a single responsibility: full-text search over the book catalog. It exposes two gRPC RPCs—`Search` and `Suggest`—and delegates all indexing work to an abstracted search engine interface. The service does not own any book data; it maintains a read-optimized projection of the catalog, kept in sync through Kafka events (section 8.1) and a bootstrap mechanism (section 8.3).

If you are coming from Spring, think of this service as a `@Service` + `@RestController` pair, except the transport layer is gRPC instead of HTTP, and the data store is a search engine instead of a relational database. The layering is the same: handler (transport) -> service (business logic) -> repository (data access).

---

## Architecture

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

Each layer depends only on the one below it, and only through an interface. The handler does not know about Meilisearch. The service does not know about gRPC. This makes every layer independently testable.

---

## The BookDocument Model

The Search Service defines its own document model, separate from the catalog's `model.Book`. This is intentional—the search index stores a denormalized, search-optimized view of the data:

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

`Suggestion` has no JSON tags because it never gets serialized to JSON directly—it is used only as an internal transfer type between the service and handler layers.

---

## The IndexRepository Interface

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

This interface is the boundary between "search service logic" and "search engine specifics." If you wanted to swap Meilisearch for Elasticsearch, Typesense, or even an in-memory implementation for tests, you would only need to write a new struct that satisfies `IndexRepository`.

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

This is a plain struct, not a map. Using a struct means the compiler catches typos in filter names and makes the available filters self-documenting.

---

## The Service Layer

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

The service also exposes `Upsert`, `Delete`, `EnsureIndex`, and `Count`—pass-through methods used by the Kafka consumer and bootstrap logic. These do not add business logic; they exist so that the consumer and bootstrap code depend on the service rather than reaching directly into the index layer. This keeps the dependency graph clean: everything flows through `SearchService`.

---

## The gRPC Handler

The handler translates between gRPC (protobuf types) and the service layer (Go domain types). It defines its own `Service` interface—a subset of what `SearchService` provides:

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

Defining a narrow `Service` interface in the handler package rather than depending on `*service.SearchService` directly is a Go idiom worth highlighting. The handler only needs `Search` and `Suggest`—it does not need `Upsert`, `Delete`, or `EnsureIndex`. By declaring exactly what it needs, the handler:

1. Documents its dependencies precisely (you can read the interface to know what the handler uses).
2. Makes testing trivial (the mock only needs two methods, not seven).
3. Follows the **Interface Segregation Principle**—depend on the smallest interface that satisfies your needs.

In Java, this pattern exists too (you can `@Autowired` an interface instead of a concrete class), but it is less common because Java developers tend to create one interface per implementation class. In Go, interfaces are defined by the consumer, not the provider—a cultural difference that leads to smaller, more focused interfaces.

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

The handler's job is mechanical: validate input, translate protobuf -> domain types, call the service, translate domain types -> protobuf. There is no business logic here. The `int32` casts on `PublishedYear`, `TotalCopies`, and `AvailableCopies` are necessary because protobuf uses fixed-width integer types while Go's `int` is platform-dependent.

Error handling follows gRPC conventions: return `status.Error` with an appropriate `codes.*` constant. The handler maps empty queries to `InvalidArgument` and internal failures to `Internal`. It does **not** leak implementation details (the error message says "search failed", not "meilisearch connection refused").

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

Because the `Service` interface only has two methods, the mock is trivial. Compare this to mocking a Spring `@Service` class with Mockito—conceptually similar, but here there is no framework, no annotation, and no proxy generation. The mock is a plain struct that you write by hand.

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

This test calls the handler method directly—no gRPC server, no network, no dialing. The handler is just a Go struct with methods. You can test it like any other function. This is one of the advantages of the layered architecture: the handler does not depend on the gRPC server infrastructure, only on the generated protobuf types.

---

## Wiring It Together in main.go

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

Notice the flow: `index.NewMeilisearchIndex` -> `service.NewSearchService` -> `handler.NewSearchHandler`. Each constructor takes exactly the dependencies it needs. The Kafka consumer and bootstrap both depend on `searchSvc` (not on the index directly), maintaining the single entry point for all write operations.

The Kafka consumer runs in a goroutine—it blocks indefinitely, processing messages until the context is cancelled. The gRPC server runs on the main goroutine. If the consumer crashes, the service continues to serve search requests from whatever is already in the index.

---

## Exercises

1. **Add a `GetDocument` method to `SearchService`.** Implement a method that retrieves a single book document by ID from the index. Define the method on the `IndexRepository` interface, implement it in `MeilisearchIndex`, and expose it through a new `GetBook` gRPC RPC.

2. **Write a test for the `Suggest` handler.** The existing tests cover the happy path and empty prefix. Write a test that verifies the handler correctly maps multiple suggestions from the service to protobuf `Suggestion` messages.

3. **Examine the `UnimplementedSearchServiceServer` embedding.** What happens if you add a new RPC to the proto file, regenerate the code, but forget to implement it in the handler? Try it: add a `GetStats` RPC to the proto, regenerate, and call it with `grpcurl`. What error code do you get?

4. **Compare the interface segregation approach.** The handler defines its own `Service` interface with 2 methods, while `SearchService` has 7. In Java/Kotlin, would you typically create a separate interface for the controller's use? What are the trade-offs?

---

## References

[^1]: [gRPC Go—Basics tutorial](https://grpc.io/docs/languages/go/basics/)—Official guide for implementing gRPC services in Go, including server setup and handler registration.
[^2]: [Go Wiki—Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)—The Go team's recommendation on where to define interfaces: at the call site, not the implementation site.
[^3]: [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html)—Reference for all gRPC status codes and when to use each one.
[^4]: Robert C. Martin, *Agile Software Development: Principles, Patterns, and Practices* (Prentice Hall, 2003)—Chapter 12 covers the Interface Segregation Principle as part of the SOLID principles.
