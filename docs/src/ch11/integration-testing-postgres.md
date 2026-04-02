# 11.2 Integration Testing with Testcontainers

Unit tests with mocks are fast, isolated, and deterministic. But they can only verify logic that your mock correctly models. As soon as the real behavior of a dependency — a database constraint, a transaction rollback, an index scan — differs from your mock's assumptions, the discrepancy is invisible. This section closes that gap by showing how to run repository tests against a real PostgreSQL instance that is spun up on demand, requires no external setup, and is torn down automatically after the test run.

---

## The Problem with `t.Skip`

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

The `t.Skipf` call on a connection failure is pragmatic for local development but dangerous in CI. When `go test` skips a test, the output line reads `--- SKIP` and the overall run still exits with code zero. Your CI pipeline sees green. Nobody notices that the repository tests never ran.

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

It skips on `testing.Short()` and again if `TEST_DATABASE_URL` is absent. Both skips are silent. In a fresh CI environment with no pre-configured Postgres, all repository tests are skipped and the suite still passes.

There is also a subtler issue in the reservation test: it calls `db.AutoMigrate(&model.Reservation{})` rather than running the embedded migration files. `AutoMigrate` only adds columns — it never creates `CHECK` constraints or the `UNIQUE` indexes that the real SQL migrations define. A test relying on `AutoMigrate` can miss an entire class of database-enforced invariants.

The fix for both problems is the same: use Testcontainers to spin up a real Postgres instance that the test controls, so there is never an external dependency to skip over.

---

## What Testcontainers Does

Testcontainers is a library (available for Go, Java, Python, .NET, and others) that starts Docker containers from within test code. The Go module wraps the Docker daemon's API; when your test calls `postgres.Run(...)`, Testcontainers:

1. Pulls the requested image if it is not already cached locally.
2. Starts a container with the given configuration.
3. Polls the container's logs (or a TCP port) until a configurable readiness signal appears.
4. Returns a handle from which you retrieve the container's mapped host port and connection string.
5. Registers a cleanup hook that terminates and removes the container when the test finishes.

The container runs on the same Docker daemon you use for development. No separate service, no CI environment variable, no `docker-compose up` step. The only prerequisite is that the Docker daemon is reachable when the test runs.

If you have used Spring Boot's `@Testcontainers` + `@Container` annotations, the Go approach is equivalent but explicit: there is no annotation magic. You call functions, receive values, and register cleanup with `t.Cleanup`. This is a good fit for Go's philosophy of making control flow visible.

---

## Adding the Dependency

From inside the catalog service directory:

```
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

The `testcontainers-go` module provides the core container API and wait strategies. The `modules/postgres` sub-module wraps it with Postgres-specific convenience functions: it knows the right wait log pattern, sets `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` environment variables, and exposes `ConnectionString`.

These are test-only dependencies, but Go does not have a separate test-scope in `go.mod` (unlike Gradle's `testImplementation`). The packages will be imported only in files with `_test.go` suffixes or behind build tags, so they will not be included in your production binary.

---

## The Test Helper Pattern

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

### Annotated walkthrough

**`//go:build integration`**

This is a build constraint. Go's toolchain evaluates it before compiling. Without `-tags integration`, the file is invisible to `go test`, `go build`, and `go vet`. This keeps the fast unit-test loop untouched: `go test ./...` runs in seconds because it never pulls a Docker image.

The line immediately below — `package repository_test` — is the package declaration. The build constraint must be on the very first line, before any blank lines or comments. If you place it after the package declaration or after an import, it is treated as a regular comment and has no effect.

**`postgres.Run`**

The `modules/postgres` package provides a typed `Run` function that accepts functional options. Compare this to the Testcontainers Java API where you construct a `PostgreSQLContainer` object and call methods on it — the Go API uses the same options pattern you have seen throughout this project.

The image tag `postgres:16-alpine` is pinned to a major version. Using `postgres:latest` in tests is fragile: a major Postgres version bump could change behavior or fail silently if the local image cache is stale.

**`testcontainers.WithWaitStrategy`**

The wait strategy solves a common race condition. Docker reports a container as "started" once its process has been launched, but "started" does not mean "ready to accept connections". Without a wait strategy, your first `gorm.Open` call might fail because Postgres is still initialising the data directory. The log-based wait strategy polls the container's stdout until the phrase `"database system is ready to accept connections"` appears twice. The phrase appears once when Postgres completes initialisation and once more after the server enters normal operation — the `.WithOccurrence(2)` requirement ensures both messages have been emitted. `WithStartupTimeout(30*time.Second)` causes the test to fail loudly if Postgres has not started within 30 seconds rather than hanging indefinitely.

**`t.Cleanup`**

`t.Cleanup` registers a function that runs when the test (or subtest) that called it finishes, regardless of whether it passed or failed. It is the testing package's equivalent of `defer` but scoped to the test's lifetime rather than the function call stack. Using `t.Cleanup` here means you never need to remember to call `container.Terminate` at the end of each test — it happens automatically, even if the test panics or calls `t.Fatal`.

Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional: `ctx` was created at the start of `setupPostgres` and is no longer in scope when the cleanup runs. Using a fresh context ensures that a cancelled parent context does not prevent the container from being cleaned up.

**`gormpostgres.Open`**

The import alias `gormpostgres` is used because both the `modules/postgres` import and the GORM Postgres driver would otherwise both be referred to as `postgres` in their package identifiers. GORM's driver is imported from `gorm.io/driver/postgres` — not `gorm.io/driver/pg`, which does not exist. The alias avoids a name collision with the `postgres` identifier already in scope from the `modules/postgres` import.

**Running real migrations**

The helper calls `m.Up()` using the same embedded `migrations.FS` that the production binary uses. This means the test database has exactly the schema that production has: `UNIQUE` indexes, `CHECK` constraints, foreign key relationships. This is the critical difference from `db.AutoMigrate`. GORM's `AutoMigrate` looks at your model structs and creates or alters tables to match — but it has no knowledge of raw SQL constraints that live only in migration files.

The idiomatic production-grade approach is: embed migration SQL files with `//go:embed`, run them in tests. Never derive schema from struct tags in an integration test context.

---

## Writing the Integration Tests

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

Each test calls `setupPostgres` directly. Each test gets its own container. This is slightly slower than sharing a container across tests but eliminates inter-test contamination entirely — no truncation step required, no risk of test ordering affecting results.

If startup time becomes a problem (containers taking three or more seconds each), use `TestMain` to start one container for the entire package and share the `*gorm.DB` handle. The trade-off is that you must truncate tables between tests yourself. For most projects, per-test containers are the right default until profiling shows otherwise.

---

## What Integration Tests Catch That Mocks Cannot

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

A mock repository can simulate this by checking an in-memory map of ISBNs and returning `model.ErrDuplicateISBN`. But that simulation encodes an assumption: that the production `Create` method actually checks for duplicates and translates the database error correctly. If the error translation code in the repository has a bug — for example, if it only catches `pq.Error` but the driver returns a different type — the mock test passes while the real system silently inserts a duplicate.

The integration test uses the actual `UNIQUE` index on the `isbn` column defined in `000001_create_books.up.sql`. The database enforces the constraint unconditionally. If `repository.Create` does not correctly translate the Postgres `23505` unique violation error code into `model.ErrDuplicateISBN`, the integration test fails. The mock test would have passed regardless.

Other behaviors that only appear with a real database:

- **`CHECK` constraint violations**: if a migration adds `CHECK (total_copies >= 0)`, only an integration test exercises the rejection path.
- **Transaction isolation**: two concurrent goroutines operating on the same row have defined behaviour in Postgres (row-level locking, serialisation failures) that an in-memory mock cannot reproduce.
- **Pagination correctness**: `LIMIT` and `OFFSET` interact with `ORDER BY` in ways that depend on how Postgres plans the query. Sorting without `ORDER BY` produces non-deterministic results that only manifest at scale or with specific data layouts.
- **Index-dependent query plans**: a query that performs adequately on ten rows in a mock may be slow or incorrect on large data because the mock does not exercise the query planner.

---

## Applying the Pattern to Auth and Reservation

The same `setupPostgres` helper (adapted to the relevant service's `migrations.FS` and model) applies to the auth and reservation services.

**Auth service** — the primary constraint to test is duplicate email. The auth migration defines a `UNIQUE` constraint on the `email` column of the `users` table. An integration test that creates a user and then creates a second user with the same email will verify that the repository correctly translates the Postgres `23505` error into the appropriate domain error. The existing `testDB` helper in `services/auth/internal/repository/user_test.go` already runs migrations via `iofs` — the migration setup is correct. The only thing to change is the skip-on-failure behavior: replace `t.Skipf` with a Testcontainers startup so the test always runs in CI.

**Reservation service** — the existing helper in `services/reservation/internal/repository/repository_test.go` has two issues to fix:

1. It calls `db.AutoMigrate(&model.Reservation{})` rather than running embedded SQL migrations. Replace this with the `iofs` migration approach shown in `setupPostgres`.
2. It skips when `TEST_DATABASE_URL` is not set. Replace both `t.Skip` calls with a Testcontainers startup.

Useful integration scenarios for the reservation repository: create a reservation and verify the returned `ID` is non-nil; create two reservations for the same user and confirm `CountActive` returns two; verify that `List` with a user ID filter returns only that user's reservations.

---

## Running Integration Tests in Earthly

The existing `test` target in each service's `Earthfile` runs only unit tests:

```earthfile
test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1
```

Note that the repository tests at `./internal/repository/...` are deliberately excluded here — they require a database. Add a separate `integration-test` target that uses Earthly's `WITH DOCKER` block:

```earthfile
integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END
```

### How `WITH DOCKER` works

Earthly runs build targets inside containers on a Docker daemon. Normally, those containers cannot themselves start additional containers — they have no access to a Docker socket. `WITH DOCKER` is Earthly's escape hatch: it starts a Docker-in-Docker (DinD) daemon inside the build container, then executes the commands inside `WITH DOCKER ... END` with that daemon available. When Testcontainers calls `docker run postgres:16-alpine`, it reaches the DinD daemon, not the outer host daemon.

From your perspective as a developer, `WITH DOCKER` looks like a scoped block. Any `RUN` commands inside it can start containers. Once the `END` is reached, Earthly tears down the DinD daemon. Images pulled during the run are discarded, so subsequent runs pull them again unless you configure a registry mirror. For CI pipelines where build times matter, this is a worthwhile area to revisit, but it does not affect correctness.

### Running on GitHub Actions

Earthly's `WITH DOCKER` requires the Docker daemon to run with elevated privileges. On GitHub Actions, add `--allow-privileged` to the Earthly invocation:

```yaml
- name: Run integration tests
  run: earthly --allow-privileged +integration-test
```

The `--allow-privileged` flag permits the DinD daemon to use Linux capabilities (specifically `CAP_SYS_ADMIN`) that are needed for container-in-container operation. GitHub-hosted runners allow this by default. If you are using a self-hosted runner or a restricted environment, verify that the runner's security policy permits privileged containers.

### Keeping targets separate

Do not modify the existing `test` target. The separation of `test` (fast, no Docker required) and `integration-test` (slower, requires Docker) maps cleanly to the two feedback loops in a CI pipeline:

- **On every commit**: run `+lint` and `+test`. Fast. Catches logic errors and style issues within seconds.
- **Before merge**: run `+integration-test`. Slower but thorough. Catches database-level issues.

The root `Earthfile` can be extended with an `integration-test` aggregate target following the same pattern as the existing `test` aggregate:

```earthfile
integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/gateway+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test
```

Earthly will run all five in parallel where dependencies allow, so the total wall time is roughly the slowest single service rather than the sum of all services.

---

## Summary

| Approach | Real DB | CI-safe | Real constraints | Setup required |
|---|---|---|---|---|
| `t.Skip` on connection failure | Optional | No | Yes (if DB exists) | External Postgres |
| `db.AutoMigrate` + skip | Optional | No | Partial | External Postgres |
| Testcontainers | Always | Yes | Yes | Docker daemon |

Testcontainers shifts the prerequisite from "a Postgres instance somewhere on the network" to "Docker is running". In a local dev environment and on every major CI provider, Docker is already available. You gain full confidence in your database layer without managing test databases or writing fragile skip conditions.

The trade-off is startup time. A Postgres container takes two to four seconds to become ready. For a test suite with ten repository tests, that cost is paid once per package (if you use `TestMain` for shared setup) or once per test (if each test starts its own container). Budget accordingly and reach for per-test containers first — they are simpler and the isolation benefit is real.

---

## References

[^1]: Testcontainers for Go — Postgres module: <https://golang.testcontainers.org/modules/postgres/>
[^2]: golang-migrate — Usage with Go: <https://github.com/golang-migrate/migrate>
[^3]: Earthly — WITH DOCKER: <https://docs.earthly.dev/docs/earthfile#with-docker>
