# 11.2 Integration Testing with Testcontainers

<!-- [STRUCTURAL] Section scope is clear and progression is sensible: motivate (t.Skip problem) → tool introduction (Testcontainers) → dependency setup → helper pattern → tests → extensions (other services) → Earthfile integration → summary. Good bridge from 11.1. -->
<!-- [LINE EDIT] Opening paragraph is 44 words and has two "but" clauses. Split: "Unit tests with mocks are fast, isolated, and deterministic. But they can only verify logic your mock correctly models. When the real behavior of a dependency diverges from the mock's assumptions, the discrepancy is invisible. This section closes that gap: we run repository tests against a real PostgreSQL instance that is spun up on demand, requires no external setup, and is torn down automatically after the run." -->
<!-- [COPY EDIT] "on demand" noun form; "on-demand" when adjectival (7.81). Here it functions adverbially, so "on demand" is correct. -->
Unit tests with mocks are fast, isolated, and deterministic. But they can only verify logic that your mock correctly models. As soon as the real behavior of a dependency — a database constraint, a transaction rollback, an index scan — differs from your mock's assumptions, the discrepancy is invisible. This section closes that gap by showing how to run repository tests against a real PostgreSQL instance that is spun up on demand, requires no external setup, and is torn down automatically after the test run.

---

## The Problem with `t.Skip`

<!-- [STRUCTURAL] Motivational section works. Shows two concrete code examples (catalog and reservation) that illustrate the same problem. -->
<!-- [LINE EDIT] "Open the catalog service's existing repository test helper:" — fine. -->
Open the catalog service's existing repository test helper:

```go
// services/catalog/internal/repository/book_test.go

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
    // ...
}
```

<!-- [LINE EDIT] "The `t.Skipf` call on a connection failure is pragmatic for local development but dangerous in CI." — good. -->
<!-- [LINE EDIT] "When `go test` skips a test, the output line reads `--- SKIP` and the overall run still exits with code zero. Your CI pipeline sees green. Nobody notices that the repository tests never ran." — fine three-sentence staccato. -->
<!-- [COPY EDIT] "exits with code zero" — CMOS 9.2: spell out zero–ninety-nine in prose; correct. Consider "exits with exit code 0" for technical precision, but spelled-out form is fine. -->
The `t.Skipf` call on a connection failure is pragmatic for local development but dangerous in CI. When `go test` skips a test, the output line reads `--- SKIP` and the overall run still exits with code zero. Your CI pipeline sees green. Nobody notices that the repository tests never ran.

<!-- [LINE EDIT] "The reservation service test helper is more explicit about its intent but has the same flaw:" — fine. -->
The reservation service test helper is more explicit about its intent but has the same flaw:

```go
// services/reservation/internal/repository/repository_test.go

func setupTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }
    // ...
}
```

<!-- [LINE EDIT] "It skips on `testing.Short()` and again if `TEST_DATABASE_URL` is absent. Both skips are silent. In a fresh CI environment with no pre-configured Postgres, all repository tests are skipped and the suite still passes." — good. -->
<!-- [COPY EDIT] "pre-configured" — hyphenated compound before noun, correct per CMOS 7.81. -->
It skips on `testing.Short()` and again if `TEST_DATABASE_URL` is absent. Both skips are silent. In a fresh CI environment with no pre-configured Postgres, all repository tests are skipped and the suite still passes.

<!-- [LINE EDIT] "There is also a subtler issue in the reservation test: it calls `db.AutoMigrate(&model.Reservation{})` rather than running the embedded migration files." — fine. -->
<!-- [LINE EDIT] "`AutoMigrate` only adds columns — it never creates `CHECK` constraints or the `UNIQUE` indexes that the real SQL migrations define." — fine. -->
<!-- [COPY EDIT] "database-enforced invariants" — compound adjective; correct. -->
There is also a subtler issue in the reservation test: it calls `db.AutoMigrate(&model.Reservation{})` rather than running the embedded migration files. `AutoMigrate` only adds columns — it never creates `CHECK` constraints or the `UNIQUE` indexes that the real SQL migrations define. A test relying on `AutoMigrate` can miss an entire class of database-enforced invariants.

<!-- [LINE EDIT] "The fix for both problems is the same: use Testcontainers to spin up a real Postgres instance that the test controls, so there is never an external dependency to skip over." — fine. -->
<!-- [COPY EDIT] "Postgres" vs "PostgreSQL": chapter mixes both. CMOS permits conventional nicknames; pick one convention — recommend "PostgreSQL" in first introduction per paragraph, "Postgres" acceptable thereafter. Normalize. -->
The fix for both problems is the same: use Testcontainers to spin up a real Postgres instance that the test controls, so there is never an external dependency to skip over.

---

## What Testcontainers Does

<!-- [STRUCTURAL] Strong, well-scoped explainer. Numbered list is effective. -->
<!-- [LINE EDIT] "Testcontainers is a library (available for Go, Java, Python, .NET, and others) that starts Docker containers from within test code." — fine. -->
<!-- [COPY EDIT] "Java, Python, .NET" — serial comma after "Python" present; good (CMOS 6.19). -->
Testcontainers is a library (available for Go, Java, Python, .NET, and others) that starts Docker containers from within test code. The Go module wraps the Docker daemon's API; when your test calls `postgres.Run(...)`, Testcontainers:

<!-- [LINE EDIT] Bullets are crisp. -->
1. Pulls the requested image if it is not already cached locally.
2. Starts a container with the given configuration.
3. Polls the container's logs (or a TCP port) until a configurable readiness signal appears.
4. Returns a handle from which you retrieve the container's mapped host port and connection string.
5. Registers a cleanup hook that terminates and removes the container when the test finishes.

<!-- [LINE EDIT] "The container runs on the same Docker daemon you use for development. No separate service, no CI environment variable, no `docker-compose up` step. The only prerequisite is that the Docker daemon is reachable when the test runs." — fine. -->
<!-- [COPY EDIT] "docker-compose up" → "`docker compose up`" — Compose v2 is the current Docker standard (Docker Compose v1 "docker-compose" was deprecated; v2 uses space). Please verify/modernize. -->
The container runs on the same Docker daemon you use for development. No separate service, no CI environment variable, no `docker-compose up` step. The only prerequisite is that the Docker daemon is reachable when the test runs.

<!-- [LINE EDIT] "If you have used Spring Boot's `@Testcontainers` + `@Container` annotations, the Go approach is equivalent but explicit: there is no annotation magic. You call functions, receive values, and register cleanup with `t.Cleanup`. This is a good fit for Go's philosophy of making control flow visible." 48 words; acceptable, split at the period. -->
If you have used Spring Boot's `@Testcontainers` + `@Container` annotations, the Go approach is equivalent but explicit: there is no annotation magic. You call functions, receive values, and register cleanup with `t.Cleanup`. This is a good fit for Go's philosophy of making control flow visible.

---

## Adding the Dependency

<!-- [LINE EDIT] "From inside the catalog service directory:" — fine. -->
From inside the catalog service directory:

```
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

<!-- [LINE EDIT] "The `testcontainers-go` module provides the core container API and wait strategies." — fine. -->
<!-- [LINE EDIT] "The `modules/postgres` sub-module wraps it with Postgres-specific convenience functions: it knows the right wait log pattern, sets `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` environment variables, and exposes `ConnectionString`." 30 words; fine. -->
<!-- [COPY EDIT] "sub-module" — CMOS 7.85 recommends "submodule" as closed compound. Normalize. -->
The `testcontainers-go` module provides the core container API and wait strategies. The `modules/postgres` sub-module wraps it with Postgres-specific convenience functions: it knows the right wait log pattern, sets `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` environment variables, and exposes `ConnectionString`.

<!-- [LINE EDIT] "These are test-only dependencies, but Go does not have a separate test-scope in `go.mod` (unlike Gradle's `testImplementation`). The packages will be imported only in files with `_test.go` suffixes or behind build tags, so they will not be included in your production binary." 44 words; acceptable. -->
<!-- [COPY EDIT] "test-scope" → "test scope" — no hyphen needed as noun phrase. -->
These are test-only dependencies, but Go does not have a separate test-scope in `go.mod` (unlike Gradle's `testImplementation`). The packages will be imported only in files with `_test.go` suffixes or behind build tags, so they will not be included in your production binary.

---

## The Test Helper Pattern

<!-- [LINE EDIT] "Create a new file alongside the existing repository tests. The `//go:build integration` tag at the top of the file tells the Go toolchain to compile this file only when the `integration` build tag is passed explicitly. `go test ./...` (with no tags) will not touch it. `go test -tags integration ./...` will include it." 54 words in three sentences; fine. -->
Create a new file alongside the existing repository tests. The `//go:build integration` tag at the top of the file tells the Go toolchain to compile this file only when the `integration` build tag is passed explicitly. `go test ./...` (with no tags) will not touch it. `go test -tags integration ./...` will include it.

```go
//go:build integration

package repository_test

import (
    "context"
    "testing"
    "time"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
    gormpostgres "gorm.io/driver/postgres"
    "gorm.io/gorm"

    "github.com/fesoliveira014/library-system/services/catalog/migrations"
    "github.com/golang-migrate/migrate/v4"
    pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

func setupPostgres(t *testing.T) *gorm.DB {
    t.Helper()
    ctx := context.Background()

    container, err := postgres.Run(ctx, "postgres:16-alpine",
        postgres.WithDatabase("catalog_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    if err != nil {
        t.Fatalf("start postgres container: %v", err)
    }
    t.Cleanup(func() { container.Terminate(context.Background()) })

    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("get connection string: %v", err)
    }

    db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{})
    if err != nil {
        t.Fatalf("connect to postgres: %v", err)
    }

    // Run real migrations
    sqlDB, _ := db.DB()
    driver, _ := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
    source, _ := iofs.New(migrations.FS, ".")
    m, _ := migrate.NewWithInstance("iofs", source, "postgres", driver)
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("run migrations: %v", err)
    }

    return db
}
```

<!-- [COPY EDIT] Code block: the migration setup silently discards errors via `_`. The 11.5 version surfaces them via `t.Fatalf`. Inconsistency between sections; if this is deliberate for brevity, consider a comment. Otherwise align. Please verify author intent. -->
<!-- [FINAL] Errors from `db.DB()`, `pgmigrate.WithInstance`, `iofs.New`, `migrate.NewWithInstance` are all swallowed with `_`. On failure the next step will panic or the migration will silently skip. Align with 11.5's `t.Fatalf` pattern. -->

### Annotated walkthrough

**`//go:build integration`**

<!-- [LINE EDIT] "This is a build constraint. Go's toolchain evaluates it before compiling. Without `-tags integration`, the file is invisible to `go test`, `go build`, and `go vet`. This keeps the fast unit-test loop untouched: `go test ./...` runs in seconds because it never pulls a Docker image." 47 words; four sentences; fine. -->
This is a build constraint. Go's toolchain evaluates it before compiling. Without `-tags integration`, the file is invisible to `go test`, `go build`, and `go vet`. This keeps the fast unit-test loop untouched: `go test ./...` runs in seconds because it never pulls a Docker image.

<!-- [LINE EDIT] "The line immediately below — `package repository_test` — is the package declaration. The build constraint must be on the very first line, before any blank lines or comments. If you place it after the package declaration or after an import, it is treated as a regular comment and has no effect." 50 words; three sentences; fine. -->
<!-- [COPY EDIT] "very first line" — "first line" suffices. Delete "very". -->
The line immediately below — `package repository_test` — is the package declaration. The build constraint must be on the very first line, before any blank lines or comments. If you place it after the package declaration or after an import, it is treated as a regular comment and has no effect.

<!-- [COPY EDIT] Please verify: build constraint placement rule. Go 1.17+ accepts `//go:build` anywhere above the `package` clause, with blank line required between build constraint and package declaration. Current text says "before any blank lines or comments" which is imprecise — the blank line between constraint and package is actually required. -->

**`postgres.Run`**

<!-- [LINE EDIT] "The `modules/postgres` package provides a typed `Run` function that accepts functional options." — fine. -->
<!-- [LINE EDIT] "Compare this to the Testcontainers Java API where you construct a `PostgreSQLContainer` object and call methods on it — the Go API uses the same options pattern you have seen throughout this project." 35 words; fine. -->
<!-- [COPY EDIT] "Testcontainers Java API" — as product: "Testcontainers for Java". Flag for style consistency. -->
The `modules/postgres` package provides a typed `Run` function that accepts functional options. Compare this to the Testcontainers Java API where you construct a `PostgreSQLContainer` object and call methods on it — the Go API uses the same options pattern you have seen throughout this project.

<!-- [LINE EDIT] "The image tag `postgres:16-alpine` is pinned to a major version. Using `postgres:latest` in tests is fragile: a major Postgres version bump could change behavior or fail silently if the local image cache is stale." — fine. -->
The image tag `postgres:16-alpine` is pinned to a major version. Using `postgres:latest` in tests is fragile: a major Postgres version bump could change behavior or fail silently if the local image cache is stale.

**`testcontainers.WithWaitStrategy`**

<!-- [LINE EDIT] "The wait strategy solves a common race condition. Docker reports a container as "started" once its process has been launched, but "started" does not mean "ready to accept connections"." — fine. -->
<!-- [LINE EDIT] "Without a wait strategy, your first `gorm.Open` call might fail because Postgres is still initialising the data directory." — fine. -->
<!-- [COPY EDIT] "initialising" — UK spelling. Normalize to US "initializing" for consistency. Also "initialisation" later in same paragraph. -->
<!-- [LINE EDIT] The following sentence is 50+ words: "The log-based wait strategy polls the container's stdout until the phrase `"database system is ready to accept connections"` appears twice. The phrase appears once when Postgres completes initialisation and once more after the server enters normal operation — the `.WithOccurrence(2)` requirement ensures both messages have been emitted." 51 words across two sentences; acceptable. -->
<!-- [LINE EDIT] "`WithStartupTimeout(30*time.Second)` causes the test to fail loudly if Postgres has not started within 30 seconds rather than hanging indefinitely." — fine. -->
The wait strategy solves a common race condition. Docker reports a container as "started" once its process has been launched, but "started" does not mean "ready to accept connections". Without a wait strategy, your first `gorm.Open` call might fail because Postgres is still initialising the data directory. The log-based wait strategy polls the container's stdout until the phrase `"database system is ready to accept connections"` appears twice. The phrase appears once when Postgres completes initialisation and once more after the server enters normal operation — the `.WithOccurrence(2)` requirement ensures both messages have been emitted. `WithStartupTimeout(30*time.Second)` causes the test to fail loudly if Postgres has not started within 30 seconds rather than hanging indefinitely.

**`t.Cleanup`**

<!-- [LINE EDIT] "`t.Cleanup` registers a function that runs when the test (or subtest) that called it finishes, regardless of whether it passed or failed." — fine. -->
<!-- [LINE EDIT] "It is the testing package's equivalent of `defer` but scoped to the test's lifetime rather than the function call stack." — fine. -->
<!-- [LINE EDIT] "Using `t.Cleanup` here means you never need to remember to call `container.Terminate` at the end of each test — it happens automatically, even if the test panics or calls `t.Fatal`." — fine. -->
`t.Cleanup` registers a function that runs when the test (or subtest) that called it finishes, regardless of whether it passed or failed. It is the testing package's equivalent of `defer` but scoped to the test's lifetime rather than the function call stack. Using `t.Cleanup` here means you never need to remember to call `container.Terminate` at the end of each test — it happens automatically, even if the test panics or calls `t.Fatal`.

<!-- [LINE EDIT] "Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional: `ctx` was created at the start of `setupPostgres` and is no longer in scope when the cleanup runs. Using a fresh context ensures that a cancelled parent context does not prevent the container from being cleaned up." 51 words; split: "Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional. `ctx` was created at the start of `setupPostgres` and is no longer in scope when the cleanup runs; a fresh context ensures a cancelled parent context does not prevent cleanup." -->
<!-- [COPY EDIT] "cancelled" — UK double-l. US form is "canceled". Normalize to US. -->
Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional: `ctx` was created at the start of `setupPostgres` and is no longer in scope when the cleanup runs. Using a fresh context ensures that a cancelled parent context does not prevent the container from being cleaned up.

<!-- [COPY EDIT] Minor technical nit: the `ctx` is captured by closure scope in the test function if the cleanup were defined there; the reasoning is more that by the time cleanup runs, the test may have timed out and the original context may be cancelled. The current prose says `ctx` is "no longer in scope" which is arguably technically off — it *is* in scope via closure, but using it would risk cancellation. Consider tightening. -->

**`gormpostgres.Open`**

<!-- [LINE EDIT] "The import alias `gormpostgres` is used because both the `modules/postgres` import and the GORM Postgres driver would otherwise both be referred to as `postgres` in their package identifiers." 30 words; fine. -->
<!-- [LINE EDIT] "GORM's driver is imported from `gorm.io/driver/postgres` — not `gorm.io/driver/pg`, which does not exist." — defensive phrasing; reader would have to be unusually confused to benefit. Consider trimming: "GORM's driver is imported from `gorm.io/driver/postgres`." -->
<!-- [LINE EDIT] "The alias avoids a name collision with the `postgres` identifier already in scope from the `modules/postgres` import." — fine. -->
<!-- [COPY EDIT] "both ... both" doubled — delete one. -->
The import alias `gormpostgres` is used because both the `modules/postgres` import and the GORM Postgres driver would otherwise both be referred to as `postgres` in their package identifiers. GORM's driver is imported from `gorm.io/driver/postgres` — not `gorm.io/driver/pg`, which does not exist. The alias avoids a name collision with the `postgres` identifier already in scope from the `modules/postgres` import.

**Running real migrations**

<!-- [LINE EDIT] "The helper calls `m.Up()` using the same embedded `migrations.FS` that the production binary uses. This means the test database has exactly the schema that production has: `UNIQUE` indexes, `CHECK` constraints, foreign key relationships. This is the critical difference from `db.AutoMigrate`." — fine. -->
The helper calls `m.Up()` using the same embedded `migrations.FS` that the production binary uses. This means the test database has exactly the schema that production has: `UNIQUE` indexes, `CHECK` constraints, foreign key relationships. This is the critical difference from `db.AutoMigrate`. GORM's `AutoMigrate` looks at your model structs and creates or alters tables to match — but it has no knowledge of raw SQL constraints that live only in migration files.

<!-- [LINE EDIT] "The idiomatic production-grade approach is: embed migration SQL files with `//go:embed`, run them in tests. Never derive schema from struct tags in an integration test context." — fine. -->
<!-- [COPY EDIT] "production-grade" — compound adjective; correct hyphenation. "`//go:embed`" — fine in monospace. -->
The idiomatic production-grade approach is: embed migration SQL files with `//go:embed`, run them in tests. Never derive schema from struct tags in an integration test context.

---

## Writing the Integration Tests

<!-- [LINE EDIT] "With `setupPostgres` in place, tests look nearly identical to the unit tests, except they use a real database handle and will catch real constraint violations:" — fine. -->
With `setupPostgres` in place, tests look nearly identical to the unit tests, except they use a real database handle and will catch real constraint violations:

```go
//go:build integration

package repository_test

import (
    "context"
    "testing"

    "github.com/fesoliveira014/library-system/services/catalog/internal/model"
    "github.com/fesoliveira014/library-system/services/catalog/internal/repository"
)

func TestBookRepository_Integration_CreateAndGet(t *testing.T) {
    db := setupPostgres(t)
    repo := repository.NewBookRepository(db)

    book := &model.Book{
        Title:       "Integration Test Book",
        Author:      "Test Author",
        ISBN:        "978-1234567890",
        TotalCopies: 5,
    }
    created, err := repo.Create(context.Background(), book)
    if err != nil {
        t.Fatalf("Create: %v", err)
    }

    found, err := repo.GetByID(context.Background(), created.ID)
    if err != nil {
        t.Fatalf("GetByID: %v", err)
    }
    if found.Title != "Integration Test Book" {
        t.Errorf("expected title %q, got %q", "Integration Test Book", found.Title)
    }
}
```

<!-- [LINE EDIT] "Each test calls `setupPostgres` directly. Each test gets its own container. This is slightly slower than sharing a container across tests but eliminates inter-test contamination entirely — no truncation step required, no risk of test ordering affecting results." 40 words; acceptable. -->
Each test calls `setupPostgres` directly. Each test gets its own container. This is slightly slower than sharing a container across tests but eliminates inter-test contamination entirely — no truncation step required, no risk of test ordering affecting results.

<!-- [LINE EDIT] "If startup time becomes a problem (containers taking three or more seconds each), use `TestMain` to start one container for the entire package and share the `*gorm.DB` handle. The trade-off is that you must truncate tables between tests yourself. For most projects, per-test containers are the right default until profiling shows otherwise." — fine. -->
<!-- [COPY EDIT] "trade-off" — CMOS-standard hyphenation; correct. -->
If startup time becomes a problem (containers taking three or more seconds each), use `TestMain` to start one container for the entire package and share the `*gorm.DB` handle. The trade-off is that you must truncate tables between tests yourself. For most projects, per-test containers are the right default until profiling shows otherwise.

<!-- [STRUCTURAL] Minor inconsistency with the earlier chapter claim that starting a Postgres container takes "5–8 seconds" (index.md) but here "three or more seconds". Reconcile: use a consistent figure or explain the range. -->

---

## What Integration Tests Catch That Mocks Cannot

<!-- [STRUCTURAL] Strong section — uses a concrete test as anchor for a general point about error-translation bugs. -->
Consider the duplicate ISBN test that already exists in `book_test.go`:

```go
func TestBookRepository_Create_DuplicateISBN(t *testing.T) {
    db := testDB(t)
    repo := repository.NewBookRepository(db)
    ctx := context.Background()

    book1 := newTestBook("0002")
    if _, err := repo.Create(ctx, book1); err != nil {
        t.Fatalf("first create failed: %v", err)
    }

    book2 := newTestBook("0003")
    book2.ISBN = book1.ISBN // same ISBN
    _, err := repo.Create(ctx, book2)
    if err != model.ErrDuplicateISBN {
        t.Errorf("expected ErrDuplicateISBN, got %v", err)
    }
}
```

<!-- [LINE EDIT] "A mock repository can simulate this by checking an in-memory map of ISBNs and returning `model.ErrDuplicateISBN`. But that simulation encodes an assumption: that the production `Create` method actually checks for duplicates and translates the database error correctly. If the error translation code in the repository has a bug — for example, if it only catches `pq.Error` but the driver returns a different type — the mock test passes while the real system silently inserts a duplicate." 77 words across three sentences; OK but could be tightened. Suggest: "A mock repository can simulate this by checking an in-memory map of ISBNs and returning `model.ErrDuplicateISBN`. But that simulation encodes an assumption — that the production `Create` actually checks for duplicates and translates the database error correctly. If the translation code is buggy (for example, it catches `pq.Error` but the driver returns a different type), the mock passes while the real system silently inserts a duplicate." -->
<!-- [COPY EDIT] "error translation" → "error-translation" when adjectival ("error-translation code") per CMOS 7.81. -->
A mock repository can simulate this by checking an in-memory map of ISBNs and returning `model.ErrDuplicateISBN`. But that simulation encodes an assumption: that the production `Create` method actually checks for duplicates and translates the database error correctly. If the error translation code in the repository has a bug — for example, if it only catches `pq.Error` but the driver returns a different type — the mock test passes while the real system silently inserts a duplicate.

<!-- [COPY EDIT] Please verify: this chapter's other sections (ch04 already updated per recent commit) use pgx-typed error. The `pq.Error` example here is out of date with ch04. Consider aligning to pgx. -->

<!-- [LINE EDIT] "The integration test uses the actual `UNIQUE` index on the `isbn` column defined in `000001_create_books.up.sql`. The database enforces the constraint unconditionally. If `repository.Create` does not correctly translate the Postgres `23505` unique violation error code into `model.ErrDuplicateISBN`, the integration test fails. The mock test would have passed regardless." — fine. -->
<!-- [COPY EDIT] "unique violation error code" — parenthetical tech term; "unique-violation error code" would be compound-adjective correct. -->
The integration test uses the actual `UNIQUE` index on the `isbn` column defined in `000001_create_books.up.sql`. The database enforces the constraint unconditionally. If `repository.Create` does not correctly translate the Postgres `23505` unique violation error code into `model.ErrDuplicateISBN`, the integration test fails. The mock test would have passed regardless.

<!-- [LINE EDIT] "Other behaviors that only appear with a real database:" — fine. -->
Other behaviors that only appear with a real database:

<!-- [LINE EDIT] Bullet list mixes styles ("if a migration..." vs "two concurrent..."). Normalize to start each bullet with a noun or a conditional phrase; currently acceptable but slightly uneven. -->
<!-- [COPY EDIT] Bullet 2: "serialisation" → "serialization" (US spelling). -->
<!-- [COPY EDIT] "`CHECK (total_copies >= 0)`" — correct inline SQL. -->
<!-- [COPY EDIT] "Index-dependent query plans" — bullet title and sentence style consistent. -->
- **`CHECK` constraint violations**: if a migration adds `CHECK (total_copies >= 0)`, only an integration test exercises the rejection path.
- **Transaction isolation**: two concurrent goroutines operating on the same row have defined behaviour in Postgres (row-level locking, serialisation failures) that an in-memory mock cannot reproduce.
- **Pagination correctness**: `LIMIT` and `OFFSET` interact with `ORDER BY` in ways that depend on how Postgres plans the query. Sorting without `ORDER BY` produces non-deterministic results that only manifest at scale or with specific data layouts.
- **Index-dependent query plans**: a query that performs adequately on ten rows in a mock may be slow or incorrect on large data because the mock does not exercise the query planner.
<!-- [COPY EDIT] "behaviour" — UK. Normalize. -->
<!-- [COPY EDIT] "non-deterministic" — CMOS 7.85 prefers "nondeterministic" (closed). Flag for project convention. -->

---

## Applying the Pattern to Auth and Reservation

<!-- [LINE EDIT] "The same `setupPostgres` helper (adapted to the relevant service's `migrations.FS` and model) applies to the auth and reservation services." — fine. -->
The same `setupPostgres` helper (adapted to the relevant service's `migrations.FS` and model) applies to the auth and reservation services.

<!-- [LINE EDIT] "**Auth service** — the primary constraint to test is duplicate email. The auth migration defines a `UNIQUE` constraint on the `email` column of the `users` table. An integration test that creates a user and then creates a second user with the same email will verify that the repository correctly translates the Postgres `23505` error into the appropriate domain error. The existing `testDB` helper in `services/auth/internal/repository/user_test.go` already runs migrations via `iofs` — the migration setup is correct. The only thing to change is the skip-on-failure behavior: replace `t.Skipf` with a Testcontainers startup so the test always runs in CI." 95 words across five sentences; the fourth sentence (50+ words) is fine. Could split the first long sentence: "**Auth service** — the primary constraint to test is duplicate email. The migration defines a `UNIQUE` constraint on `users.email`. An integration test creates a user, then attempts to create another with the same email; the repository must translate Postgres's `23505` into the right domain error." -->
<!-- [COPY EDIT] "skip-on-failure" — compound adjective; correct. -->
**Auth service** — the primary constraint to test is duplicate email. The auth migration defines a `UNIQUE` constraint on the `email` column of the `users` table. An integration test that creates a user and then creates a second user with the same email will verify that the repository correctly translates the Postgres `23505` error into the appropriate domain error. The existing `testDB` helper in `services/auth/internal/repository/user_test.go` already runs migrations via `iofs` — the migration setup is correct. The only thing to change is the skip-on-failure behavior: replace `t.Skipf` with a Testcontainers startup so the test always runs in CI.

<!-- [LINE EDIT] "**Reservation service** — the existing helper in `services/reservation/internal/repository/repository_test.go` has two issues to fix:" — fine. -->
**Reservation service** — the existing helper in `services/reservation/internal/repository/repository_test.go` has two issues to fix:

1. It calls `db.AutoMigrate(&model.Reservation{})` rather than running embedded SQL migrations. Replace this with the `iofs` migration approach shown in `setupPostgres`.
2. It skips when `TEST_DATABASE_URL` is not set. Replace both `t.Skip` calls with a Testcontainers startup.

<!-- [LINE EDIT] "Useful integration scenarios for the reservation repository: create a reservation and verify the returned `ID` is non-nil; create two reservations for the same user and confirm `CountActive` returns two; verify that `List` with a user ID filter returns only that user's reservations." 42 words; fine. -->
Useful integration scenarios for the reservation repository: create a reservation and verify the returned `ID` is non-nil; create two reservations for the same user and confirm `CountActive` returns two; verify that `List` with a user ID filter returns only that user's reservations.

---

## Running Integration Tests in Earthly

<!-- [STRUCTURAL] Earthly section is the transitional glue between code and CI. Section 11.5 has a parallel Earthly discussion — audit to ensure minimal duplication. -->
The existing `test` target in each service's `Earthfile` runs only unit tests:

```earthfile
test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1
```

<!-- [LINE EDIT] "Note that the repository tests at `./internal/repository/...` are deliberately excluded here — they require a database. Add a separate `integration-test` target that uses Earthly's `WITH DOCKER` block:" — fine. -->
Note that the repository tests at `./internal/repository/...` are deliberately excluded here — they require a database. Add a separate `integration-test` target that uses Earthly's `WITH DOCKER` block:

```earthfile
integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END
```

### How `WITH DOCKER` works

<!-- [LINE EDIT] "Earthly runs build targets inside containers on a Docker daemon. Normally, those containers cannot themselves start additional containers — they have no access to a Docker socket. `WITH DOCKER` is Earthly's escape hatch: it starts a Docker-in-Docker (DinD) daemon inside the build container, then executes the commands inside `WITH DOCKER ... END` with that daemon available. When Testcontainers calls `docker run postgres:16-alpine`, it reaches the DinD daemon, not the outer host daemon." — fourth sentence reaches 40 words; fine. -->
<!-- [COPY EDIT] "escape hatch" — idiom, acceptable. -->
Earthly runs build targets inside containers on a Docker daemon. Normally, those containers cannot themselves start additional containers — they have no access to a Docker socket. `WITH DOCKER` is Earthly's escape hatch: it starts a Docker-in-Docker (DinD) daemon inside the build container, then executes the commands inside `WITH DOCKER ... END` with that daemon available. When Testcontainers calls `docker run postgres:16-alpine`, it reaches the DinD daemon, not the outer host daemon.

<!-- [LINE EDIT] "From your perspective as a developer, `WITH DOCKER` looks like a scoped block. Any `RUN` commands inside it can start containers. Once the `END` is reached, Earthly tears down the DinD daemon. Images pulled during the run are discarded, so subsequent runs pull them again unless you configure a registry mirror. For CI pipelines where build times matter, this is a worthwhile area to revisit, but it does not affect correctness." — four sentences, reads fine. -->
From your perspective as a developer, `WITH DOCKER` looks like a scoped block. Any `RUN` commands inside it can start containers. Once the `END` is reached, Earthly tears down the DinD daemon. Images pulled during the run are discarded, so subsequent runs pull them again unless you configure a registry mirror. For CI pipelines where build times matter, this is a worthwhile area to revisit, but it does not affect correctness.

### Running on GitHub Actions

<!-- [LINE EDIT] "Earthly's `WITH DOCKER` requires the Docker daemon to run with elevated privileges. On GitHub Actions, add `--allow-privileged` to the Earthly invocation:" — fine. -->
Earthly's `WITH DOCKER` requires the Docker daemon to run with elevated privileges. On GitHub Actions, add `--allow-privileged` to the Earthly invocation:

```yaml
- name: Run integration tests
  run: earthly --allow-privileged +integration-test
```

<!-- [COPY EDIT] Please verify: Earthly CLI flag `--allow-privileged`. Confirm current Earthly 0.8+ syntax. -->
<!-- [LINE EDIT] "The `--allow-privileged` flag permits the DinD daemon to use Linux capabilities (specifically `CAP_SYS_ADMIN`) that are needed for container-in-container operation. GitHub-hosted runners allow this by default. If you are using a self-hosted runner or a restricted environment, verify that the runner's security policy permits privileged containers." — fine. -->
<!-- [COPY EDIT] "GitHub-hosted runners" — proper compound modifier; GitHub is a proper noun; "GitHub-hosted" correct. -->
The `--allow-privileged` flag permits the DinD daemon to use Linux capabilities (specifically `CAP_SYS_ADMIN`) that are needed for container-in-container operation. GitHub-hosted runners allow this by default. If you are using a self-hosted runner or a restricted environment, verify that the runner's security policy permits privileged containers.

### Keeping targets separate

<!-- [LINE EDIT] "Do not modify the existing `test` target. The separation of `test` (fast, no Docker required) and `integration-test` (slower, requires Docker) maps cleanly to the two feedback loops in a CI pipeline:" — fine. -->
Do not modify the existing `test` target. The separation of `test` (fast, no Docker required) and `integration-test` (slower, requires Docker) maps cleanly to the two feedback loops in a CI pipeline:

- **On every commit**: run `+lint` and `+test`. Fast. Catches logic errors and style issues within seconds.
- **Before merge**: run `+integration-test`. Slower but thorough. Catches database-level issues.

<!-- [LINE EDIT] "The root `Earthfile` can be extended with an `integration-test` aggregate target following the same pattern as the existing `test` aggregate:" — fine. -->
The root `Earthfile` can be extended with an `integration-test` aggregate target following the same pattern as the existing `test` aggregate:

```earthfile
integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/gateway+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test
```

<!-- [LINE EDIT] "Earthly will run all five in parallel where dependencies allow, so the total wall time is roughly the slowest single service rather than the sum of all services." — fine. -->
Earthly will run all five in parallel where dependencies allow, so the total wall time is roughly the slowest single service rather than the sum of all services.

---

## Summary

<!-- [COPY EDIT] Table rows: "External Postgres" setup phrase vs "Docker daemon" setup phrase — fine. "Optional" used to mean "may or may not be present" which is slightly unclear vs "Optional/Required". Consider clearer labels: "Required/Optional/None". -->
| Approach | Real DB | CI-safe | Real constraints | Setup required |
|---|---|---|---|---|
| `t.Skip` on connection failure | Optional | No | Yes (if DB exists) | External Postgres |
| `db.AutoMigrate` + skip | Optional | No | Partial | External Postgres |
| Testcontainers | Always | Yes | Yes | Docker daemon |

<!-- [LINE EDIT] "Testcontainers shifts the prerequisite from "a Postgres instance somewhere on the network" to "Docker is running". In a local dev environment and on every major CI provider, Docker is already available. You gain full confidence in your database layer without managing test databases or writing fragile skip conditions." — fine. -->
Testcontainers shifts the prerequisite from "a Postgres instance somewhere on the network" to "Docker is running". In a local dev environment and on every major CI provider, Docker is already available. You gain full confidence in your database layer without managing test databases or writing fragile skip conditions.

<!-- [LINE EDIT] "The trade-off is startup time. A Postgres container takes two to four seconds to become ready. For a test suite with ten repository tests, that cost is paid once per package (if you use `TestMain` for shared setup) or once per test (if each test starts its own container). Budget accordingly and reach for per-test containers first — they are simpler and the isolation benefit is real." — fine. -->
<!-- [COPY EDIT] "two to four seconds" conflicts with earlier "three or more seconds" and index.md's "5–8 seconds". Pick one figure. -->
The trade-off is startup time. A Postgres container takes two to four seconds to become ready. For a test suite with ten repository tests, that cost is paid once per package (if you use `TestMain` for shared setup) or once per test (if each test starts its own container). Budget accordingly and reach for per-test containers first — they are simpler and the isolation benefit is real.

---

## References

[^1]: Testcontainers for Go — Postgres module: <https://golang.testcontainers.org/modules/postgres/>
[^2]: golang-migrate — Usage with Go: <https://github.com/golang-migrate/migrate>
[^3]: Earthly — WITH DOCKER: <https://docs.earthly.dev/docs/earthfile#with-docker>
<!-- [COPY EDIT] Please verify: all three footnote URLs resolve. -->
