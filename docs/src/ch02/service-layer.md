# 2.4 Service Layer & Business Logic

The repository layer you built in the previous section knows exactly one thing: how to talk to a database. That's intentional — it has no opinion about whether a book title is required, whether you can delete a book that's currently checked out, or how to coordinate multiple repository calls into a single operation. That's the job of the **service layer**.

The service layer is where business logic lives. It sits between the transport layer (gRPC handlers) and the persistence layer (repositories), and it depends on neither. This section covers how Go interfaces enable that clean separation, how to express domain errors in a way callers can inspect, and how to test business logic in isolation using hand-written mocks.

---

## Defining Interfaces in Go

In Java or Kotlin, you define an interface and then explicitly declare that a class implements it:

```kotlin
// Kotlin
interface BookRepository {
    fun findById(id: UUID): Book?
}

class PostgresBookRepository : BookRepository {  // explicit declaration
    override fun findById(id: UUID): Book? = TODO()
}
```

Go works differently. Interface satisfaction is **implicit** — a type satisfies an interface simply by having the right method signatures. There is no `implements` keyword and no declaration of intent:

```go
// The interface lives in the service package — the consumer owns it.
type BookRepository interface {
    Create(ctx context.Context, book *model.Book) (*model.Book, error)
    GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
    Update(ctx context.Context, book *model.Book) (*model.Book, error)
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error)
    UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}
```

The GORM-backed `BookRepository` struct in `services/catalog/internal/repository/` has all these methods with matching signatures. That's enough — it satisfies the interface automatically. The repository package doesn't import the service package, doesn't know the interface exists, and doesn't care.

This is a deliberate inversion of the Java pattern. In Java, the interface usually lives near the implementation and the consumer depends on it from there. In Go, the convention is for the **consumer to define the interface it needs** — the service package declares exactly the repository surface it uses, and any type that matches can be plugged in. This makes interfaces smaller, more focused, and trivial to mock.

The practical consequence: you can introduce a new `BookRepository` implementation (an in-memory cache, a read-replica adapter) without touching anything in the service package. The dependency arrow points inward.

---

## The Service as Orchestrator

The `CatalogService` struct holds a `BookRepository` field and nothing else:

```go
type CatalogService struct {
    repo BookRepository
}

func NewCatalogService(repo BookRepository) *CatalogService {
    return &CatalogService{repo: repo}
}
```

This is manual dependency injection — no framework, just a constructor that takes what it needs. The service has no idea whether `repo` is backed by PostgreSQL, SQLite, or an in-memory map. That's the point.

Most service methods are thin orchestrators. `GetBook` simply delegates:

```go
func (s *CatalogService) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
    return s.repo.GetByID(ctx, id)
}
```

There's no business logic here worth adding a layer for — but the layer still matters because it's the right place for logic *when it does exist*, and it decouples the gRPC handler from the repository interface. If you later need to add an audit log entry every time a book is fetched, this is where it goes, without touching the handler or the repository.

`CreateBook` is where the service earns its keep:

```go
func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
    if err := validateBook(book); err != nil {
        return nil, err
    }
    book.AvailableCopies = book.TotalCopies
    return s.repo.Create(ctx, book)
}
```

Two things happen here that have nothing to do with SQL: the input is validated, and an invariant is enforced (`available_copies` must equal `total_copies` for a new book). The repository doesn't know about either rule — it would happily insert a book with a blank title and negative availability if asked. The service layer is the last line of defence before data reaches the database.

---

## Business Validation and Domain Errors

`validateBook` encodes the non-negotiable rules for a valid book:

```go
func validateBook(book *model.Book) error {
    if book.Title == "" {
        return fmt.Errorf("%w: title is required", model.ErrInvalidBook)
    }
    if book.Author == "" {
        return fmt.Errorf("%w: author is required", model.ErrInvalidBook)
    }
    if book.TotalCopies < 0 {
        return fmt.Errorf("%w: total copies must be non-negative", model.ErrInvalidBook)
    }
    return nil
}
```

The sentinel errors live in `services/catalog/internal/model/errors.go`:

```go
var (
    ErrBookNotFound  = errors.New("book not found")
    ErrDuplicateISBN = errors.New("duplicate ISBN")
    ErrInvalidBook   = errors.New("invalid book data")
)
```

The `%w` verb in `fmt.Errorf` **wraps** the sentinel error. The resulting error carries a human-readable message ("title is required") but also embeds `ErrInvalidBook` as a cause. Callers can inspect which type of error they're dealing with using `errors.Is`:

```go
_, err := svc.CreateBook(ctx, &model.Book{Author: "Kennedy"})
if errors.Is(err, model.ErrInvalidBook) {
    // respond with HTTP 400 / gRPC InvalidArgument
}
```

`errors.Is` traverses the error chain — it unwraps wrapped errors recursively until it finds a match. This means the gRPC handler doesn't need to parse the error string to decide how to respond. It just checks `errors.Is(err, model.ErrBookNotFound)` and maps to `codes.NotFound`, or checks `model.ErrInvalidBook` and maps to `codes.InvalidArgument`.

Compare this to Java's exception hierarchy. The Go pattern achieves the same classification goal without inheritance or `instanceof` — just wrap a sentinel, unwrap it with `errors.Is`. [^2]

---

## Testing with Hand-Written Mocks

Go's standard library includes `testing` but no mocking framework. The ecosystem has tools like `gomock` and `testify/mock`, but for learning, hand-written mocks are better — they show you exactly what's happening rather than hiding it behind code generation.

A mock just needs to implement the same interface. Here's the one from `catalog_test.go`:

```go
type mockBookRepository struct {
    books map[uuid.UUID]*model.Book
}

func newMockRepo() *mockBookRepository {
    return &mockBookRepository{books: make(map[uuid.UUID]*model.Book)}
}

func (m *mockBookRepository) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
    book.ID = uuid.New()
    for _, b := range m.books {
        if b.ISBN == book.ISBN && book.ISBN != "" {
            return nil, model.ErrDuplicateISBN
        }
    }
    m.books[book.ID] = book
    return book, nil
}

func (m *mockBookRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
    b, ok := m.books[id]
    if !ok {
        return nil, model.ErrBookNotFound
    }
    return b, nil
}

// ... Delete, Update, List, UpdateAvailability implemented similarly
```

This is not a fake of the database — it's a fake of the repository interface. The service tests run entirely in memory, with no network, no PostgreSQL container, no Docker. They are fast and deterministic.

The tests themselves follow a consistent pattern:

```go
func TestCatalogService_CreateBook_MissingTitle(t *testing.T) {
    svc := service.NewCatalogService(newMockRepo())
    ctx := context.Background()

    book := &model.Book{Author: "Some Author", TotalCopies: 1}
    _, err := svc.CreateBook(ctx, book)
    if err == nil {
        t.Fatal("expected error for missing title")
    }
    if !errors.Is(err, model.ErrInvalidBook) {
        t.Errorf("expected ErrInvalidBook, got %v", err)
    }
}
```

Notice what's being tested: not "does GORM insert correctly" (the repository tests cover that), but "does the service reject invalid input". Each test exercises one business rule. When a test fails, you know exactly which rule broke.

Why hand-written over `mockgen`? Small, focused interfaces (often 1–3 methods) are Go's norm [^1] — they're trivial to implement by hand. The mock above also stores actual state in a map, which makes tests more realistic than `gomock`'s call-expectation style. And there's no generated code to decode when something fails.

---

## Exercise: Enforce a Deletion Invariant

The current `DeleteBook` implementation delegates directly to the repository:

```go
func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
    return s.repo.Delete(ctx, id)
}
```

This has a bug: a book can be deleted even if copies are currently checked out (i.e., `available_copies < total_copies`). That means outstanding reservations would reference a book that no longer exists.

**Your task:** Add a new sentinel error and enforce the invariant in the service layer. Write the test first, then fix the implementation.

**Step 1 — Add a sentinel error** to `model/errors.go`:

```go
ErrBookHasActiveReservations = errors.New("book has active reservations")
```

**Step 2 — Write a failing test** in `catalog_test.go`:

```go
func TestCatalogService_DeleteBook_WithActiveReservations(t *testing.T) {
    repo := newMockRepo()
    svc := service.NewCatalogService(repo)
    ctx := context.Background()

    // Create a book with 3 copies
    book, _ := svc.CreateBook(ctx, &model.Book{
        Title:       "Clean Code",
        Author:      "Martin",
        TotalCopies: 3,
    })

    // Simulate one copy being checked out
    _ = svc.UpdateAvailability(ctx, book.ID, -1)

    // Attempt to delete should fail
    err := svc.DeleteBook(ctx, book.ID)
    if !errors.Is(err, model.ErrBookHasActiveReservations) {
        t.Errorf("expected ErrBookHasActiveReservations, got %v", err)
    }
}
```

Run `go test ./...` from `services/catalog/` — the test should fail because `DeleteBook` doesn't check anything yet.

**Step 3 — Implement the rule.** Think about what the service needs to do before reading the solution.

<details>
<summary>Solution</summary>

The service must fetch the book first, check the invariant, then delete:

```go
func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
    book, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return err
    }
    if book.AvailableCopies < book.TotalCopies {
        return fmt.Errorf("%w: %d of %d copies are checked out",
            model.ErrBookHasActiveReservations,
            book.TotalCopies-book.AvailableCopies,
            book.TotalCopies,
        )
    }
    return s.repo.Delete(ctx, id)
}
```

A few things worth noting:

- The service makes **two repository calls** — `GetByID` then `Delete`. This is the service as orchestrator: neither the repository nor the gRPC handler has any reason to know this check needs to happen.
- The error message embeds the counts for observability — a caller logging the error gets context without having to re-fetch the book.
- The mock's `UpdateAvailability` modifies `AvailableCopies` in the in-memory map, so the test correctly reflects the post-checkout state. No database required.

Run `go test ./...` again — all tests should pass.

</details>

---

## What Comes Next

The service layer is complete. It validates input, enforces invariants, wraps domain errors, and coordinates repository calls — all without coupling itself to GORM or gRPC. The next section connects everything: the gRPC handler calls the service, which calls the repository, and `main.go` wires all three together with dependency injection.

---

[^1]: [Effective Go — Interfaces](https://go.dev/doc/effective_go#interfaces)
[^2]: [Go Blog: Error handling and Go](https://go.dev/blog/error-handling-and-go)
