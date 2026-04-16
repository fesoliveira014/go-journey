# 2.3 Repository Pattern with GORM

<!-- [STRUCTURAL] Opening two paragraphs motivate the ORM+repository pairing well, with explicit scope signalling ("every method", "integration tests", "real PostgreSQL — not mocks"). Strong lede. -->
Raw SQL migrations give you precise control over the database schema. But for day-to-day CRUD operations — inserting a record, fetching one by ID, running paginated queries — writing SQL strings by hand gets tedious fast. This is where an ORM earns its keep.

<!-- [LINE EDIT] "You'll also see how to write integration tests against a real PostgreSQL database — not mocks, not in-memory fakes, the real thing." — the "real thing" at the end is casual-punchy; retains voice. Leave. -->
This section introduces GORM (Go's dominant ORM), explains the repository pattern as a clean boundary around data access, and walks through every method in the Catalog service's `BookRepository`. You'll also see how to write integration tests against a real PostgreSQL database — not mocks, not in-memory fakes, the real thing.

---

## GORM Basics

<!-- [LINE EDIT] "GORM's mental model will feel familiar if you've used JPA/Hibernate." — good bridge. "You annotate your structs with tags, call `gorm.Open()` to get a `*gorm.DB` handle, and then call chainable methods to build and execute queries." — fine. -->
GORM's mental model will feel familiar if you've used JPA/Hibernate. You annotate your structs with tags, call `gorm.Open()` to get a `*gorm.DB` handle, and then call chainable methods to build and execute queries. The concepts map cleanly:

| JPA/Hibernate | GORM |
|---|---|
| `@Entity` | struct with `gorm:` tags |
| `@Id`, `@GeneratedValue` | `gorm:"primaryKey;default:..."` |
| `@Column(nullable=false)` | `gorm:"not null"` |
| `@UniqueConstraint` | `gorm:"uniqueIndex"` |
| `EntityManager.persist()` | `db.Create(&record)` |
| `EntityManager.find()` | `db.First(&record, ...)` |
| `TypedQuery` / JPQL | `db.Where(...).Find(...)` |
| `EntityManager.merge()` | `db.Updates(...)` |
| `EntityManager.remove()` | `db.Delete(...)` |

<!-- [LINE EDIT] "The surface-level difference is Go's lack of generics at GORM's level of maturity — you pass `&record` pointers instead of type parameters, and the ORM uses reflection to figure out which table to target. This feels awkward at first but becomes second nature quickly." — "at GORM's level of maturity" is awkward (Go has had generics since 1.18; the phrase reads as dated). Suggest: "The surface-level difference is that GORM predates Go generics — you pass `&record` pointers and the ORM uses reflection to locate the target table. This feels awkward at first but becomes second nature." -->
The surface-level difference is Go's lack of generics at GORM's level of maturity — you pass `&record` pointers instead of type parameters, and the ORM uses reflection to figure out which table to target. This feels awkward at first but becomes second nature quickly.
<!-- [COPY EDIT] "Please verify: is GORM v2 still not using generics in its public API as of 2026? gorm.io/gen exists but the main Find/First/Where chain remains reflection-based. Confirm current state. -->

### The Book Model

Here's the `Book` struct from `services/catalog/internal/model/book.go`:

```go
type Book struct {
    ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
    Title           string    `gorm:"type:varchar(500);not null"`
    Author          string    `gorm:"type:varchar(500);not null"`
    ISBN            string    `gorm:"type:varchar(13);uniqueIndex"`
    Genre           string    `gorm:"type:varchar(100)"`
    Description     string    `gorm:"type:text"`
    PublishedYear   int       `gorm:"type:integer"`
    TotalCopies     int       `gorm:"type:integer;not null;default:1"`
    AvailableCopies int       `gorm:"type:integer;not null;default:1"`
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

A few things worth calling out:

<!-- [LINE EDIT] "tells GORM the column is a UUID primary key, and that the database should generate a new value if one isn't provided" — fine. -->
- **`gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`** — tells GORM the column is a UUID primary key, and that the database should generate a new value if one isn't provided. The `uuid_generate_v4()` function comes from PostgreSQL's `uuid-ossp` extension, which is enabled in the migration.
<!-- [LINE EDIT] "GORM won't create the index (migrations handle that), but the tag is documentation: it tells readers why a unique violation error might appear on this field." — good; explicit. -->
- **`uniqueIndex`** — GORM knows this field maps to a unique index. GORM won't create the index (migrations handle that), but the tag is documentation: it tells readers why a unique violation error might appear on this field.
- **`CreatedAt` / `UpdatedAt`** — GORM's convention. If a struct has these fields, GORM automatically populates `CreatedAt` on insert and updates `UpdatedAt` on every update. No annotation required — the field names are enough. This is Go's convention-over-configuration at work.
<!-- [COPY EDIT] "convention-over-configuration" — hyphenated compound noun/adjective (CMOS 7.81). Correct. -->

<!-- [LINE EDIT] "Notice there's no `tableName` annotation." → "Note that there's no `tableName` annotation." or "There's no `tableName` annotation." — "Notice" is a mild mannerism used several times in the file; see broader note in copy-edit pass. -->
Notice there's no `tableName` annotation. By convention, GORM pluralizes the struct name: `Book` → `books`. You can override this with a `TableName() string` method on the struct if needed.

### Opening a Connection

```go
import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

dsn := "host=localhost port=5432 user=postgres password=postgres dbname=catalog sslmode=disable"
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
```

<!-- [LINE EDIT] "`gorm.Open` returns a `*gorm.DB`. This isn't a single connection — it wraps Go's `database/sql` connection pool under the hood. The pool is safe for concurrent use across goroutines." — clean. Leave. -->
`gorm.Open` returns a `*gorm.DB`. This isn't a single connection — it wraps Go's `database/sql` connection pool under the hood. The pool is safe for concurrent use across goroutines. In the Catalog service's `main.go`, this handle is created once at startup and passed into the repository constructor.

### Configuring the Connection Pool

<!-- [STRUCTURAL] The pool-configuration sidebar is practical content that would otherwise bite the reader later. Well placed here at the point of `gorm.Open`. -->
The default `database/sql` pool has no upper bound on open connections. In a long-running service with multiple Kubernetes replicas, that will eventually collide with PostgreSQL's `max_connections` (default 100) — once it's exhausted, new connection attempts fail and healthy traffic degrades. Always set limits explicitly:

```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
if err != nil {
    return nil, err
}
sqlDB, err := db.DB()
if err != nil {
    return nil, err
}
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(5)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

<!-- [COPY EDIT] "replicas × MaxOpenConns < postgres.max_connections − headroom" — uses proper Unicode minus sign and multiplication sign — good typography. Verify rendering in target output formats (HTML vs Markdown). -->
- `SetMaxOpenConns` caps in-flight queries. Pick a number such that `replicas × MaxOpenConns < postgres.max_connections − headroom`.
- `SetMaxIdleConns` keeps a small pool of warm connections for burst traffic. Setting it above `SetMaxOpenConns` is pointless.
<!-- [COPY EDIT] "~30 minutes" — tilde as "approximately" is informal but acceptable in technical prose; consistent across chapter. Acceptable. -->
- `SetConnMaxLifetime` forces the pool to recycle connections. Managed PostgreSQL services (AWS RDS, Cloud SQL) silently close idle connections after ~30 minutes; a bounded lifetime avoids handing stale connections to the app.

<!-- [LINE EDIT] "In this project the three services share a small `pkg/db.Open` helper that applies these defaults. Any of them can be overridden via `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, and `DB_CONN_MAX_LIFETIME` environment variables." — serial comma correct; "three services" references a project-wide fact that the reader may not yet have context for. Consider "the services in this project share" or forward-reference the number. -->
In this project the three services share a small `pkg/db.Open` helper that applies these defaults. Any of them can be overridden via `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, and `DB_CONN_MAX_LIFETIME` environment variables. The [GORM documentation on connection pools](https://gorm.io/docs/connecting_to_the_database.html#Connection-Pool) explains the tuning knobs in more detail.

---

## The Repository Pattern

<!-- [LINE EDIT] "The repository pattern puts a named interface between your business logic and your data access code.[^3]" — "data access code" → "data-access code" (compound modifier before noun — CMOS 7.81). -->
The repository pattern puts a named interface between your business logic and your data access code.[^3] Instead of calling `db.Where(...).Find(...)` directly inside a handler or service method, you define a contract:

```go
// In services/catalog/internal/service/catalog.go
type BookRepository interface {
    Create(ctx context.Context, book *model.Book) (*model.Book, error)
    GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
    Update(ctx context.Context, book *model.Book) (*model.Book, error)
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error)
    UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}
```

<!-- [STRUCTURAL] Declaring the interface here, before the struct, is pedagogically correct — interface-first framing reinforces the consumer-owns-interface idea that 2.4 will make explicit. Leave. -->
The service layer only knows about this interface. It doesn't know about GORM, PostgreSQL, or SQL at all. The `BookRepository` struct in the `repository` package implements this interface using GORM.

**Why bother?**

<!-- [COPY EDIT] Numbered list: periods ending each item — fine. Parallel structure is preserved (all three items are noun phrases headed by single word). -->
1. **Testability.** The service layer can be tested with a mock or stub `BookRepository` — no database needed. You test business logic in isolation, at unit-test speed.
2. **Swappability.** If you ever needed to swap PostgreSQL for a different store (unlikely but not impossible), you'd write a new implementation of the interface. The service layer wouldn't change.
3. **Readability.** `repo.GetByID(ctx, id)` tells you what's happening. `db.WithContext(ctx).First(&book, "id = ?", id)` also tells you, but you have to decode GORM's API first. The abstraction raises the vocabulary of the call site.

**Compare to using GORM directly in handlers:**

<!-- [LINE EDIT] "or a mock of `*gorm.DB` (which is painful — GORM's interface is large)" — "is painful" is fine; consider "is impractical — GORM's API is large." for register. -->
If a gRPC handler calls `db.Where(...).Find(...)` directly, testing that handler requires either a real database or a mock of `*gorm.DB` (which is painful — GORM's interface is large). With the repository pattern, the handler calls `service.ListBooks(...)`, the service calls `repo.List(...)`, and each layer can be tested independently.

---

## Implementing BookRepository

The full implementation lives in `services/catalog/internal/repository/book.go`. Let's walk through each method.

### Construction

```go
type BookRepository struct {
    db *gorm.DB
}

func NewBookRepository(db *gorm.DB) *BookRepository {
    return &BookRepository{db: db}
}
```

<!-- [LINE EDIT] "The repository holds one field: the `*gorm.DB` handle. This is dependency injection Go-style — no framework, just a constructor that accepts the dependency." — banned word "just". Rewrite: "This is dependency injection Go-style — no framework, a constructor that accepts the dependency and nothing more." -->
The repository holds one field: the `*gorm.DB` handle. This is dependency injection Go-style — no framework, just a constructor that accepts the dependency. The `*gorm.DB` is shared across all repository method calls; GORM manages connection pooling internally.

### Create

```go
func (r *BookRepository) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
    if err := r.db.WithContext(ctx).Create(book).Error; err != nil {
        if isDuplicateKeyError(err) {
            return nil, model.ErrDuplicateISBN
        }
        return nil, err
    }
    return book, nil
}
```

<!-- [LINE EDIT] "`db.WithContext(ctx)` returns a new `*gorm.DB` scoped to the provided context. This is important for cancellation and deadline propagation — if the caller's context is cancelled (e.g., the gRPC request times out), the database query is cancelled too. Always use `WithContext` in production code." — good. Leave. -->
`db.WithContext(ctx)` returns a new `*gorm.DB` scoped to the provided context. This is important for cancellation and deadline propagation — if the caller's context is cancelled (e.g., the gRPC request times out), the database query is cancelled too. Always use `WithContext` in production code.

<!-- [COPY EDIT] "`Create(book)` issues `INSERT INTO books (...) VALUES (...)`." — backticked SQL treatment is consistent throughout; good. -->
`Create(book)` issues `INSERT INTO books (...) VALUES (...)`. GORM populates `book.ID`, `book.CreatedAt`, and `book.UpdatedAt` in-place — the same pointer you passed in comes back enriched.

<!-- [LINE EDIT] "The error handling translates a raw PostgreSQL error into a domain error. `isDuplicateKeyError` checks the SQLSTATE on the typed `*pgconn.PgError` that the `pgx` driver returns. `23505` is the standard SQL state for a unique violation:" — compound sentence chain; acceptable. Leave. -->
The error handling translates a raw PostgreSQL error into a domain error. `isDuplicateKeyError` checks the SQLSTATE on the typed `*pgconn.PgError` that the `pgx` driver returns. `23505` is the standard SQL state for a unique violation:

```go
import "github.com/jackc/pgx/v5/pgconn"

func isDuplicateKeyError(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

<!-- [LINE EDIT] "Using the typed error — not the error message — is the correct pattern." — good. -->
<!-- [LINE EDIT] "a driver upgrade, a locale change, or switching from `pgx` to `lib/pq` can silently break string matching" — tight and well-chosen examples; keep. -->
`errors.As` walks the wrapped error chain, so it keeps working even when GORM wraps the driver error. Using the typed error — not the error message — is the correct pattern. Error messages are not a stable API: a driver upgrade, a locale change, or switching from `pgx` to `lib/pq` can silently break string matching. The Go blog's [Error handling and Go](https://go.dev/blog/error-handling-and-go) and Dave Cheney's [Don't just check errors, handle them gracefully](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully) both call this out explicitly.
<!-- [COPY EDIT] Inline hyperlinks here break the footnote-only citation convention used elsewhere in the chapter (e.g., protobuf-grpc.md uses only [^n] footnotes). Pick one style and apply uniformly. Recommend converting these to footnotes for consistency. -->

This lets callers check `errors.Is(err, model.ErrDuplicateISBN)` rather than parsing PostgreSQL error codes themselves. Domain errors are part of the public API; PostgreSQL error codes are an implementation detail that shouldn't leak upward.

### GetByID

```go
func (r *BookRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
    var book model.Book
    if err := r.db.WithContext(ctx).First(&book, "id = ?", id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, model.ErrBookNotFound
        }
        return nil, err
    }
    return &book, nil
}
```

<!-- [LINE EDIT] "`First` issues `SELECT * FROM books WHERE id = ? LIMIT 1 ORDER BY id`." — the generated SQL order-of-clauses is wrong: LIMIT after ORDER BY in standard SQL. Please verify and fix.
[COPY EDIT] Please verify: generated SQL — is it "WHERE id = ? ORDER BY id LIMIT 1" (correct) or "WHERE id = ? LIMIT 1 ORDER BY id" as written? The text suggests the implicit ORDER BY; the order is reversed in the string. If this is literal GORM output, capture the actual string; if paraphrased, fix the clause order. -->
`First` issues `SELECT * FROM books WHERE id = ? LIMIT 1 ORDER BY id`. The `ORDER BY id` is implicit with `First` — GORM sorts by primary key when you use `First` rather than `Take`. If no row matches, GORM returns `gorm.ErrRecordNotFound`.

<!-- [LINE EDIT] "The pattern `errors.Is(err, gorm.ErrRecordNotFound)` is idiomatic Go error handling. `errors.Is` unwraps error chains, so it handles wrapped errors correctly. This is analogous to catching a `NoResultException` in JPA and rethrowing a domain-specific exception." — good bridge. Leave. -->
The pattern `errors.Is(err, gorm.ErrRecordNotFound)` is idiomatic Go error handling. `errors.Is` unwraps error chains, so it handles wrapped errors correctly. This is analogous to catching a `NoResultException` in JPA and rethrowing a domain-specific exception.

### Update

```go
func (r *BookRepository) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
    result := r.db.WithContext(ctx).Model(book).Updates(map[string]interface{}{
        "title":          book.Title,
        "author":         book.Author,
        // ... other fields
    })
    if result.Error != nil { /* ... */ }
    if result.RowsAffected == 0 {
        return nil, model.ErrBookNotFound
    }
    return r.GetByID(ctx, book.ID)
}
```

<!-- [STRUCTURAL] The choice of `Updates(map[string]interface{})` over `Save(book)` is an important GORM footgun. The current text explains the "why" concisely — exactly right for the audience. -->
<!-- [LINE EDIT] "GORM issues an upsert-style `UPDATE` that sets every field — including zero-valued ones." — "upsert-style" is misleading; Save does not do an upsert in standard usage, it does an UPDATE that writes all fields. Please verify wording: probably drop "upsert-style" → "an `UPDATE` that sets every field, including zero-valued ones." -->
`Updates` with a `map[string]interface{}` is deliberate. If you call `db.Save(book)` instead, GORM issues an upsert-style `UPDATE` that sets every field — including zero-valued ones. Using a map gives you explicit control over which columns are updated. This matters when you have fields like `AvailableCopies` that are managed by a separate method (`UpdateAvailability`) and should not be clobbered by a general update.
<!-- [COPY EDIT] "map[string]interface{}" — modern Go style prefers "map[string]any" (since Go 1.18 aliased `any = interface{}`). Consider suggesting `any` for a 2026 book, or at least noting the alias in a footnote. -->

<!-- [LINE EDIT] "`result.RowsAffected == 0` catches the case where the ID doesn't exist. The `UPDATE` succeeded (no SQL error) but matched zero rows — so the book isn't there." — good. Leave. -->
`result.RowsAffected == 0` catches the case where the ID doesn't exist. The `UPDATE` succeeded (no SQL error) but matched zero rows — so the book isn't there. This is returned as `ErrBookNotFound`.

After a successful update, the repository calls `GetByID` to reload the record. This ensures the returned struct reflects the database state, including the new `updated_at` timestamp that PostgreSQL set.

### Delete

```go
func (r *BookRepository) Delete(ctx context.Context, id uuid.UUID) error {
    result := r.db.WithContext(ctx).Delete(&model.Book{}, "id = ?", id)
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return model.ErrBookNotFound
    }
    return nil
}
```

<!-- [LINE EDIT] "Our `Book` struct doesn't have that field, so this is a hard delete." — fine. -->
`Delete` issues `DELETE FROM books WHERE id = ?`. The same `RowsAffected == 0` pattern detects a missing record. Note that GORM supports soft deletes — if the struct had a `DeletedAt gorm.DeletedAt` field, `Delete` would set that timestamp instead of removing the row. Our `Book` struct doesn't have that field, so this is a hard delete.

### UpdateAvailability

```go
func (r *BookRepository) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
    result := r.db.WithContext(ctx).
        Model(&model.Book{}).
        Where("id = ? AND available_copies + ? >= 0", id, delta).
        Update("available_copies", gorm.Expr("available_copies + ?", delta))
    // ...
}
```

<!-- [STRUCTURAL] Race-condition framing ("rather than reading the current value... and writing it back — which would introduce a race condition") is precisely the level of depth this audience needs. Well placed. -->
<!-- [LINE EDIT] "This method is special." — "special" is a vague filler word. Consider: "This method deserves closer attention." or "This method is the race-condition trap — done wrong, it corrupts availability counts under concurrency." -->
This method is special. Rather than reading the current value into Go, incrementing it, and writing it back — which would introduce a race condition — it uses a SQL expression: `UPDATE books SET available_copies = available_copies + ? WHERE id = ?`. The increment happens atomically in the database. `gorm.Expr(...)` injects a raw SQL fragment into the query.

<!-- [LINE EDIT] "The `WHERE` clause includes `available_copies + ? >= 0` as a guard to prevent negative availability — the database will reject an update that would underflow, and `RowsAffected == 0` signals the error to the caller." — "reject" is slightly imprecise — the WHERE clause filters the row out rather than raising an error. Consider: "...the WHERE clause filters out rows that would go negative — so an underflow simply matches zero rows, and `RowsAffected == 0` signals the error to the caller." -->
The `WHERE` clause includes `available_copies + ? >= 0` as a guard to prevent negative availability — the database will reject an update that would underflow, and `RowsAffected == 0` signals the error to the caller.

---

## Pagination and Filtering

<!-- [LINE EDIT] "The `List` method is where GORM's chainable API shines." — "shines" is editorial judgment talking; acceptable voice. Leave. -->
The `List` method is where GORM's chainable API shines. The query is built up conditionally based on the provided filter, then a `Count` is executed on the same query scope before pagination is applied:

```go
func (r *BookRepository) List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
    query := r.db.WithContext(ctx).Model(&model.Book{})

    if filter.Genre != "" {
        query = query.Where("genre = ?", filter.Genre)
    }
    if filter.Author != "" {
        query = query.Where("author ILIKE ?", "%"+filter.Author+"%")
    }
    if filter.AvailableOnly {
        query = query.Where("available_copies > 0")
    }

    var total int64
    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }

    // clamp page size
    pageSize := page.PageSize
    if pageSize <= 0 { pageSize = model.DefaultPageSize }
    if pageSize > model.MaxPageSize { pageSize = model.MaxPageSize }
    offset := 0
    if page.Page > 1 {
        offset = (page.Page - 1) * pageSize
    }

    var books []*model.Book
    if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&books).Error; err != nil {
        return nil, 0, err
    }

    return books, total, nil
}
```

**Key points:**

<!-- [LINE EDIT] "**`query` is immutable per call.**" — "per call" is ambiguous. Suggest: "**`query` is immutable.** Each chainable method..." -->
- **`query` is immutable per call.** Each chainable method (`Where`, `Order`, etc.) returns a new `*gorm.DB` with the added clause. Reassigning `query = query.Where(...)` is the standard idiom. There are no side-effects on the original handle.
<!-- [COPY EDIT] "side-effects" → "side effects" (CMOS and common usage prefer the open form for the noun; hyphenate only as attributive adjective). -->
- **`Count` before `Limit`.** Running `Count` on the filtered-but-not-yet-paginated query gives the total number of matching records — which the caller needs to compute total pages. `Limit` and `Offset` are added afterward for the actual row fetch.
<!-- [COPY EDIT] "filtered-but-not-yet-paginated" — four-element hyphenated compound; technically correct per CMOS 7.81 but visually heavy. Consider "filtered query (before pagination)". -->
- **`ILIKE` for case-insensitive search.** PostgreSQL's `ILIKE` is the case-insensitive version of `LIKE`. The `%` wildcards match any characters on either side of the author string. This is a simple substring search — good enough for a catalog UI, though a production system might use PostgreSQL full-text search for more sophisticated matching.
<!-- [STRUCTURAL] Consider a brief note here on ILIKE indexing: a plain B-tree index on `author` does not help ILIKE with leading wildcards. This is exactly the kind of footgun the audience will hit next. -->
- **`Find` vs `First`.** `Find` returns zero or more rows into a slice; it does not return `ErrRecordNotFound` when the slice is empty. `First` returns exactly one row and errors if there are none. Use `First` for single-record lookups, `Find` for collections.

<!-- [LINE EDIT] "The `MaxPageSize` cap (100) prevents a client from issuing `?page_size=100000` and pulling the entire table in one request." — good concrete example. -->
The `MaxPageSize` cap (100) prevents a client from issuing `?page_size=100000` and pulling the entire table in one request.

---

## Integration Testing

<!-- [STRUCTURAL] The integration-testing section is a highlight. Strong framing ("mocks test logic; integration tests verify real SQL runs") and explicit architectural reasoning. Keep. -->
Unit tests with mocked repositories test business logic. But the `repository` package itself contains database interaction — you want to test that the actual SQL queries work, that GORM generates the right `WHERE` clauses, and that constraint violations behave as expected. For that you need a real PostgreSQL database.

### The `testDB` Helper

```go
func testDB(t *testing.T) *gorm.DB {
    t.Helper()

    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        dsn = "host=localhost port=5432 user=postgres password=postgres dbname=catalog_test sslmode=disable"
    }

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        t.Skipf("skipping integration test: cannot connect to PostgreSQL: %v", err)
    }

    // Run real migrations
    sqlDB, _ := db.DB()
    driver, _ := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
    source, _ := iofs.New(migrations.FS, ".")
    m, _ := migrate.NewWithInstance("iofs", source, "postgres", driver)
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("failed to run migrations: %v", err)
    }

    db.Exec("TRUNCATE TABLE books CASCADE")
    return db
}
```

<!-- [COPY EDIT] "pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})" and ignored errors with _ — pedagogically the snippet ignores errors on purpose to reduce noise. A footnote or inline note acknowledging this would be worth adding; otherwise the reader may copy-paste into production code. -->
<!-- [COPY EDIT] "err != migrate.ErrNoChange" — same idiom query as in postgresql-migrations.md. Please verify whether errors.Is is preferred here given current golang-migrate wrapping behaviour. -->

Three design decisions here worth understanding:

<!-- [COPY EDIT] "behaviour" (British spelling) used here; American "behavior" used elsewhere in chapter (e.g. service-layer uses US spelling). Standardize — suggest "behavior" for consistency with US English baseline, unless house style is explicitly BrE. -->
1. **`t.Skipf` on connection failure, not `t.Fatalf`.** If the database isn't available (e.g., in a CI environment that didn't start PostgreSQL), the tests skip gracefully rather than failing. This is the right behaviour for optional infrastructure dependencies — the developer is informed, not blocked.

2. **Real migrations, not `AutoMigrate`.** GORM has an `AutoMigrate` function that creates tables from struct definitions. It's convenient but dangerous in production: it can't drop columns, it won't create indexes you didn't annotate, and it diverges from your real schema over time. This test uses `golang-migrate` with the same `migrations/` SQL files as production — the test database has the exact same schema, including CHECK constraints and unique indexes.
<!-- [LINE EDIT] Redundancy with §2.2 — the "AutoMigrate is dangerous" warning was covered in postgresql-migrations.md. Consider cross-referencing: "See §2.2 for the full rationale." rather than repeating the three reasons. Or at least shortening to one sentence. -->

3. **`TRUNCATE TABLE books CASCADE` before each test.** Each call to `testDB` wipes the table, so test functions start with a clean slate regardless of order. `CASCADE` handles foreign key dependencies. This is simpler and faster than rolling back transactions, and it avoids subtle ordering dependencies between tests.
<!-- [STRUCTURAL] "Simpler and faster than rolling back transactions" — this is a judgment call that is actually debated in the Go testing community. A transaction-rollback per test is a common alternative that provides isolation without TRUNCATE's schema-wide effects. Consider acknowledging the alternative briefly with a reference. -->

### What the Tests Cover

The test file exercises every public method and every notable error path:

- `TestBookRepository_Create` — happy path, UUID populated on insert
- `TestBookRepository_Create_DuplicateISBN` — verifies `ErrDuplicateISBN` is returned, not a raw PostgreSQL error
- `TestBookRepository_GetByID_NotFound` — verifies `ErrBookNotFound` translation
- `TestBookRepository_Update` — field update and `updated_at` refresh
- `TestBookRepository_Delete` and `TestBookRepository_Delete_NotFound` — both sides of the `RowsAffected` check
- `TestBookRepository_List` — pagination and genre filter
- `TestBookRepository_UpdateAvailability` — atomic decrement

<!-- [LINE EDIT] "Each test is independent: it calls `testDB(t)`, which runs migrations (idempotent after the first run) and truncates. No shared state across tests." — good. Leave. -->
Each test is independent: it calls `testDB(t)`, which runs migrations (idempotent after the first run) and truncates. No shared state across tests.

---

## Exercise

<!-- [STRUCTURAL] Exercise scope is tight: one method, interface update, one test with two assertions. Good. -->
Add a `GetByISBN(ctx context.Context, isbn string) (*model.Book, error)` method to `BookRepository`. The method should return `model.ErrBookNotFound` when no book with that ISBN exists.

Then write an integration test `TestBookRepository_GetByISBN` that:
1. Creates a book with a known ISBN.
2. Calls `GetByISBN` with that ISBN and asserts the correct book is returned.
3. Calls `GetByISBN` with a non-existent ISBN and asserts `ErrBookNotFound` is returned.

<details>
<summary>Solution</summary>

**Method implementation** — add to `services/catalog/internal/repository/book.go`:

```go
func (r *BookRepository) GetByISBN(ctx context.Context, isbn string) (*model.Book, error) {
    var book model.Book
    if err := r.db.WithContext(ctx).First(&book, "isbn = ?", isbn).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, model.ErrBookNotFound
        }
        return nil, err
    }
    return &book, nil
}
```

<!-- [COPY EDIT] "`SELECT * FROM books WHERE isbn = ? ORDER BY id LIMIT 1`" — here the SQL clause order is correct, unlike the GetByID prose. Use this as the canonical form. -->
The structure is identical to `GetByID` — only the column and parameter type change. `First` with a string condition generates `SELECT * FROM books WHERE isbn = ? ORDER BY id LIMIT 1`.

**Interface update** — add to the `BookRepository` interface in `services/catalog/internal/service/catalog.go`:

```go
GetByISBN(ctx context.Context, isbn string) (*model.Book, error)
```

**Integration test** — add to `services/catalog/internal/repository/book_test.go`:

```go
func TestBookRepository_GetByISBN(t *testing.T) {
    db := testDB(t)
    repo := repository.NewBookRepository(db)
    ctx := context.Background()

    // Create a book with a known ISBN
    book := newTestBook("0010")
    created, err := repo.Create(ctx, book)
    if err != nil {
        t.Fatalf("create failed: %v", err)
    }

    // Happy path: find by ISBN
    found, err := repo.GetByISBN(ctx, created.ISBN)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    if found.ID != created.ID {
        t.Errorf("expected ID %v, got %v", created.ID, found.ID)
    }

    // Not found: random ISBN
    _, err = repo.GetByISBN(ctx, "000-not-real")
    if err != model.ErrBookNotFound {
        t.Errorf("expected ErrBookNotFound, got %v", err)
    }
}
```
<!-- [COPY EDIT] "if err != model.ErrBookNotFound" — this should idiomatically be `if !errors.Is(err, model.ErrBookNotFound)` to handle wrapped errors. The body text in this chapter advocates `errors.Is` everywhere; the test code contradicts that guidance. Recommend aligning. -->

</details>

---

## Summary

- GORM struct tags declare column types, constraints, and indexes — they mirror JPA annotations but use Go's backtick tag syntax.
- `db.WithContext(ctx)` propagates cancellation and deadlines into every query. Always use it.
- The repository pattern wraps GORM behind a named interface, isolating data access from business logic and making each layer independently testable.
- `gorm.ErrRecordNotFound` and PostgreSQL error messages are translated into domain errors at the repository boundary — callers never handle GORM or SQL internals directly.
- Dynamic queries are built by chaining `Where` calls on a `*gorm.DB` value. `Count` is called before `Offset`/`Limit` to capture the total matching rows for pagination metadata.
- Integration tests use real migrations and real PostgreSQL, with `TRUNCATE` for isolation between tests. If the database isn't available, tests skip rather than fail.

---

[^1]: [GORM documentation](https://gorm.io/docs/)
[^2]: [GORM PostgreSQL driver](https://gorm.io/docs/connecting_to_the_database.html#PostgreSQL)
[^3]: [Repository pattern — Martin Fowler](https://martinfowler.com/eaaCatalog/repository.html)
<!-- [COPY EDIT] Footnotes [^1] and [^2] appear only here in the references block but do not seem to be cited in body text. Verify: either cite them inline or remove. [^3] is cited in §"The Repository Pattern". -->
