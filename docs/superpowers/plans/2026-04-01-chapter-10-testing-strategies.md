# Chapter 10: Testing Strategies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write Chapter 10 documentation (6 sections) and implement integration tests with testcontainers, bufconn gRPC tests, and Kafka integration tests across auth, catalog, reservation, and search services.

**Architecture:** Bottom-up complexity ladder: documentation sections written first (10.1-10.6), then test code implemented per service. Documentation tasks are independent and can be parallelized. Test code tasks depend on the testcontainers dependency being added first.

**Tech Stack:** testcontainers-go (Postgres + Kafka modules), google.golang.org/grpc/test/bufconn, IBM/sarama, golang-migrate, GORM

---

## File Structure

### Documentation (new)
- `docs/src/ch10/index.md` — Chapter introduction and testing pyramid
- `docs/src/ch10/unit-testing-patterns.md` — Table-driven tests, subtests, helpers
- `docs/src/ch10/integration-testing-postgres.md` — Testcontainers + Postgres
- `docs/src/ch10/grpc-testing.md` — bufconn gRPC testing
- `docs/src/ch10/kafka-testing.md` — Kafka producer/consumer testing
- `docs/src/ch10/e2e-testing.md` — Service-level end-to-end tests
- `docs/src/SUMMARY.md` — Add chapter 10 entries

### Test Code (new)
- `services/catalog/internal/repository/integration_test.go` — Testcontainers Postgres tests
- `services/auth/internal/repository/integration_test.go` — Testcontainers Postgres tests
- `services/reservation/internal/repository/integration_test.go` — Testcontainers Postgres tests
- `services/catalog/internal/handler/grpc_integration_test.go` — bufconn + testcontainers tests
- `services/auth/internal/handler/grpc_integration_test.go` — bufconn + testcontainers tests
- `services/reservation/internal/kafka/publisher_integration_test.go` — Kafka producer tests
- `services/catalog/internal/consumer/integration_test.go` — Kafka consumer + Postgres tests
- `services/search/internal/consumer/integration_test.go` — Kafka consumer tests
- `services/catalog/internal/e2e/catalog_e2e_test.go` — Full service e2e
- `services/reservation/internal/e2e/reservation_e2e_test.go` — Full service e2e
- `services/auth/internal/e2e/auth_e2e_test.go` — Full service e2e

### Build (modified)
- `Earthfile` — Add `integration-test` orchestration target
- `services/*/Earthfile` — Add `integration-test` target with `WITH DOCKER`

---

### Task 1: Write Section 10.1 — Introduction & Testing Pyramid

**Files:**
- Create: `docs/src/ch10/index.md`
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Create the chapter index file**

Write `docs/src/ch10/index.md` covering:

1. Opening: chapters 1-9 built unit tests with hand-written mocks. These catch logic bugs but are blind to infrastructure failures (wrong SQL, serialization mismatches, interceptor miswiring).
2. The testing pyramid for microservices:
   - **Unit tests** (fast, many): test business logic in isolation with mocks. Already established.
   - **Integration tests** (slower, fewer): test against real Postgres, real Kafka. New in this chapter.
   - **End-to-end tests** (slowest, fewest): test full service flows through the real API with real infrastructure. New in this chapter.
3. Each layer tests what the layer below cannot. Give concrete examples:
   - Unit test with mock repo passes, but real SQL has wrong column name → integration test catches it.
   - Handler test calls Go method directly, missing that auth interceptor isn't wired → bufconn test catches it.
   - Kafka consumer test with mocked broker misses serialization format mismatch → testcontainers Kafka test catches it.
4. Cost model: unit tests run in milliseconds, integration tests in seconds (container startup), e2e in tens of seconds. The chapter teaches when each is worth the cost.
5. Tools introduced: testcontainers-go (ephemeral Docker containers for Postgres and Kafka), bufconn (in-memory gRPC connections), `//go:build integration` tag for gating slow tests.
6. Brief chapter roadmap: sections 10.2-10.6.

Target length: ~100-150 lines. No code in this section.

Include references:
- [^1]: The Test Pyramid — Martin Fowler: https://martinfowler.com/bliki/TestPyramid.html
- [^2]: Testcontainers for Go: https://golang.testcontainers.org/
- [^3]: gRPC bufconn package: https://pkg.go.dev/google.golang.org/grpc/test/bufconn

- [ ] **Step 2: Update SUMMARY.md**

Add chapter 10 entries to `docs/src/SUMMARY.md` after the Chapter 9 section:

```markdown
- [Chapter 10: Testing Strategies](./ch10/index.md)
  - [10.1 Unit Testing Patterns](./ch10/unit-testing-patterns.md)
  - [10.2 Integration Testing with Testcontainers](./ch10/integration-testing-postgres.md)
  - [10.3 gRPC Testing with bufconn](./ch10/grpc-testing.md)
  - [10.4 Kafka Testing](./ch10/kafka-testing.md)
  - [10.5 Service-Level End-to-End Tests](./ch10/e2e-testing.md)
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch10/index.md docs/src/SUMMARY.md
git commit -m "docs: add Chapter 10 index and testing pyramid introduction"
```

---

### Task 2: Write Section 10.2 — Unit Testing Patterns

**Files:**
- Create: `docs/src/ch10/unit-testing-patterns.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch10/unit-testing-patterns.md` covering:

1. **Table-driven tests** — the canonical Go pattern. Show a worked example refactoring the catalog service's `CreateBook` validation into table-driven form:

```go
func TestCreateBook_Validation(t *testing.T) {
    repo := newMockRepo()
    pub := &noopPublisher{}
    svc := service.NewCatalogService(repo, pub)

    tests := []struct {
        name    string
        book    *model.Book
        wantErr bool
    }{
        {
            name:    "missing title",
            book:    &model.Book{Author: "A", ISBN: "978-0000000001", TotalCopies: 1},
            wantErr: true,
        },
        {
            name:    "missing author",
            book:    &model.Book{Title: "T", ISBN: "978-0000000001", TotalCopies: 1},
            wantErr: true,
        },
        {
            name:    "negative copies",
            book:    &model.Book{Title: "T", Author: "A", ISBN: "978-0000000001", TotalCopies: -1},
            wantErr: true,
        },
        {
            name:    "valid book",
            book:    &model.Book{Title: "T", Author: "A", ISBN: "978-0000000001", TotalCopies: 3},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := svc.CreateBook(context.Background(), tt.book)
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateBook() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

Explain the pattern: declare a slice of anonymous structs, iterate with `t.Run`. Benefits: easy to add cases, clear failure output showing which case failed, each case runs as a named subtest.

2. **Subtests with `t.Run`** — already used above but formalize:
   - Naming: `TestFunctionName/descriptive_case_name`. Go converts spaces to underscores.
   - Selective execution: `go test -run "TestCreateBook/missing_title"`.
   - Parallel subtests with `t.Parallel()` — when safe (no shared mutable state). Note: the current mock patterns use shared state, so parallelism requires per-subtest mock instances.

3. **Test helpers and `t.Helper()`** — the project already uses this in repository tests (e.g., `setupTestDB` calls `t.Helper()`). Explain why: when a helper calls `t.Fatalf`, Go reports the failure at the helper's line. `t.Helper()` tells Go to report at the caller's line instead. Show the pattern:

```go
func mustCreateBook(t *testing.T, svc *service.CatalogService, title string) *model.Book {
    t.Helper()
    book, err := svc.CreateBook(context.Background(), &model.Book{
        Title: title, Author: "Test Author",
        ISBN: fmt.Sprintf("978-%010d", rand.Intn(1e10)), TotalCopies: 5,
    })
    if err != nil {
        t.Fatalf("setup: create book %q: %v", title, err)
    }
    return book
}
```

4. **Test fixtures with `testdata/`** — mention Go's convention: files in `testdata/` directories are ignored by `go build` but accessible in tests via relative paths. Useful for large JSON payloads or golden files. For this project's test code, inline data is preferred since payloads are small.

Target length: ~150-200 lines. Code-heavy section.

References:
- [^1]: Go blog — Using Subtests and Sub-benchmarks: https://go.dev/blog/subtests
- [^2]: Go testing package documentation: https://pkg.go.dev/testing

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch10/unit-testing-patterns.md
git commit -m "docs: write Section 10.1 unit testing patterns"
```

---

### Task 3: Write Section 10.3 — Integration Testing with Testcontainers

**Files:**
- Create: `docs/src/ch10/integration-testing-postgres.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch10/integration-testing-postgres.md` covering:

1. **The problem with `t.Skip`** — show the current pattern from `services/catalog/internal/repository/book_test.go`:

```go
func testDB(t *testing.T) *gorm.DB {
    t.Helper()
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        dsn = "host=localhost port=5432 ..."
    }
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        t.Skipf("skipping integration test: cannot connect to PostgreSQL: %v", err)
    }
    // ...
}
```

This test silently skips when there's no database. In CI, it never runs. Bugs in SQL, constraints, and migrations go undetected until deployment.

2. **What testcontainers does** — spins up a real Docker container running Postgres, waits for it to be ready, gives you a connection string, and tears it down when the test finishes. The test is fully self-contained.

3. **Adding the dependency** — `go get github.com/testcontainers/testcontainers-go` and `go get github.com/testcontainers/testcontainers-go/modules/postgres`. Show the import paths.

4. **The test helper pattern** — show the complete helper function:

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
    "gorm.io/driver/postgres"
    "gorm.io/gorm"

    // Import the service's embedded migrations
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

    db, err := gorm.Open(pg.Open(connStr), &gorm.Config{})
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

Explain each part: the `//go:build integration` tag, container configuration, wait strategy, `t.Cleanup` for teardown, running real migrations.

5. **Catalog integration test example** — show a test that uses `setupPostgres`:

```go
func TestBookRepository_Integration_CreateAndGet(t *testing.T) {
    db := setupPostgres(t)
    repo := repository.NewBookRepository(db)

    book := &model.Book{
        Title: "Integration Test Book", Author: "Test Author",
        ISBN: "978-1234567890", TotalCopies: 5,
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

6. **What integration tests catch that mocks miss** — the duplicate ISBN constraint test is the canonical example. The mock implementation in `catalog_test.go` checks `b.ISBN == book.ISBN` in a loop — a simplified approximation. The real Postgres constraint is a unique index. Integration tests verify the real behavior: error type, error message, constraint name.

7. **Auth and reservation** — briefly note these follow the same pattern. Auth tests: create user, duplicate email constraint, lookup by email. Reservation tests: create, count active, list by user. The reservation service's existing test uses `db.AutoMigrate()` — the integration test should use the embedded `migrations.FS` instead for consistency.

8. **Running in Earthly** — show the `WITH DOCKER` target:

```earthfile
integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END
```

Explain `WITH DOCKER` starts a Docker daemon inside the Earthly container. GitHub Actions needs `--allow-privileged`. The existing `test` target (which scopes to specific packages) is unchanged.

Target length: ~300-400 lines.

References:
- [^1]: Testcontainers for Go — Postgres module: https://golang.testcontainers.org/modules/postgres/
- [^2]: golang-migrate — Usage with Go: https://github.com/golang-migrate/migrate
- [^3]: Earthly — WITH DOCKER: https://docs.earthly.dev/docs/earthfile#with-docker

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch10/integration-testing-postgres.md
git commit -m "docs: write Section 10.2 integration testing with testcontainers"
```

---

### Task 4: Write Section 10.4 — gRPC Testing with bufconn

**Files:**
- Create: `docs/src/ch10/grpc-testing.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch10/grpc-testing.md` covering:

1. **The gap in current tests** — show the current pattern from `services/catalog/internal/handler/catalog_test.go`:

```go
h := handler.NewCatalogHandler(svc)
resp, err := h.CreateBook(ctx, req)
```

This calls the Go method directly. It works, but bypasses: protobuf serialization/deserialization, the auth interceptor (`UnaryAuthInterceptor`), gRPC metadata propagation, and gRPC status code mapping. A test might pass but a real client gets `Unauthenticated` because the interceptor isn't wired.

2. **How bufconn works** — `google.golang.org/grpc/test/bufconn` creates an in-memory `net.Listener`. The server listens on it, the client dials it with `grpc.WithContextDialer`. Same process, no network, no port allocation. The full gRPC stack runs: protobuf marshaling, interceptor chain, metadata, status codes.

3. **Setting up a bufconn server** — show the complete pattern:

```go
//go:build integration

package handler_test

import (
    "context"
    "net"
    "testing"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"

    catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
    pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
    "github.com/fesoliveira014/library-system/services/catalog/internal/handler"
    "github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

const bufSize = 1024 * 1024

func startCatalogServer(t *testing.T, svc *service.CatalogService, jwtSecret string) catalogv1.CatalogServiceClient {
    t.Helper()
    lis := bufconn.Listen(bufSize)

    srv := grpc.NewServer(
        grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, nil)),
    )
    catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))

    go func() { srv.Serve(lis) }()
    t.Cleanup(func() { srv.GracefulStop() })

    conn, err := grpc.NewClient("passthrough:///bufconn",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("dial bufconn: %v", err)
    }
    t.Cleanup(func() { conn.Close() })

    return catalogv1.NewCatalogServiceClient(conn)
}
```

Explain: `bufconn.Listen` creates the in-memory listener, the server is registered with the real auth interceptor, `grpc.WithContextDialer` makes the client dial the bufconn listener instead of a real address.

4. **Testing with the interceptor** — show tests that verify interceptor behavior:

```go
func TestCreateBook_Unauthenticated(t *testing.T) {
    // ... setup with mock repo and bufconn server ...
    client := startCatalogServer(t, svc, "test-secret")

    // Call without auth metadata
    _, err := client.CreateBook(context.Background(), &catalogv1.CreateBookRequest{
        Title: "Test", Author: "Author", Isbn: "978-0000000001", TotalCopies: 1,
    })
    if status.Code(err) != codes.Unauthenticated {
        t.Errorf("expected Unauthenticated, got %v", err)
    }
}

func TestCreateBook_WithAuth(t *testing.T) {
    // ... setup ...
    client := startCatalogServer(t, svc, "test-secret")

    // Generate a valid admin JWT
    token, _ := pkgauth.GenerateToken(uuid.New(), "admin", "test-secret", time.Hour)
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    resp, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
        Title: "Test Book", Author: "Author", Isbn: "978-0000000001", TotalCopies: 3,
    })
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }
    if resp.GetTitle() != "Test Book" {
        t.Errorf("expected title %q, got %q", "Test Book", resp.GetTitle())
    }
}
```

5. **Combining bufconn with testcontainers** — the most powerful pattern. The bufconn server uses a real repository backed by testcontainers Postgres:

```go
func TestCreateBook_Integration(t *testing.T) {
    db := setupPostgres(t) // from section 10.2's helper
    repo := repository.NewBookRepository(db)
    svc := service.NewCatalogService(repo, &noopPublisher{})
    client := startCatalogServer(t, svc, "test-secret")
    // ... full-stack test ...
}
```

This exercises: gRPC client → protobuf → interceptors → handler → service → repository → real Postgres. All in-process, no network.

6. **Auth service bufconn tests** — same pattern but for auth. Key difference: auth has public methods (Register, Login) that skip the interceptor, so the `skipMethods` parameter matters. Show how to test the register → login → validate token flow.

Target length: ~250-300 lines.

References:
- [^1]: gRPC Go — bufconn package: https://pkg.go.dev/google.golang.org/grpc/test/bufconn
- [^2]: gRPC Go — Testing: https://grpc.io/docs/languages/go/testing/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch10/grpc-testing.md
git commit -m "docs: write Section 10.3 gRPC testing with bufconn"
```

---

### Task 5: Write Section 10.5 — Kafka Testing

**Files:**
- Create: `docs/src/ch10/kafka-testing.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch10/kafka-testing.md` covering:

1. **Why test Kafka integration** — the existing consumer tests in `services/catalog/internal/consumer/consumer_test.go` test the `handleEvent` function directly with byte slices. They verify JSON parsing and delta logic. But they don't test: consumer group joining, offset management, message header propagation, or the interaction between the consumer and real Kafka.

2. **Kafka testcontainers setup** — show the helper:

```go
//go:build integration

package kafka_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go/modules/kafka"
)

func setupKafka(t *testing.T) []string {
    t.Helper()
    ctx := context.Background()

    container, err := kafka.Run(ctx, "confluentinc/confluent-local:7.6.0")
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

Note: Kafka containers take ~10-15 seconds to start (longer than Postgres). Container reuse across tests in the same package is recommended.

3. **Producer testing (reservation service)** — show the test that publishes and reads back:

```go
func TestPublisher_Integration(t *testing.T) {
    brokers := setupKafka(t)
    topic := "test-reservations"

    pub, err := kafka.NewPublisher(brokers, topic)
    if err != nil {
        t.Fatalf("create publisher: %v", err)
    }
    defer pub.Close()

    event := service.ReservationEvent{
        Type:          "reservation.created",
        ReservationID: uuid.New().String(),
        UserID:        uuid.New().String(),
        BookID:        uuid.New().String(),
        Timestamp:     time.Now(),
    }
    if err := pub.Publish(context.Background(), event); err != nil {
        t.Fatalf("publish: %v", err)
    }

    // Read back with a test consumer
    config := sarama.NewConfig()
    config.Consumer.Offsets.Initial = sarama.OffsetOldest
    consumer, err := sarama.NewConsumer(brokers, config)
    // ... consume partition, verify message key, payload, headers ...
}
```

Explain: we use `sarama.NewConsumer` (partition consumer, not consumer group) for the verification side because it's simpler — we just need to read one message. The producer is the code under test.

Verify: topic name, message key equals `event.BookID`, JSON payload deserializes correctly, OTel trace headers present in message headers.

4. **Consumer testing (catalog service)** — show the pattern:

```go
func TestConsumer_Integration(t *testing.T) {
    brokers := setupKafka(t)
    db := setupPostgres(t)
    repo := repository.NewBookRepository(db)
    svc := service.NewCatalogService(repo, &noopPublisher{})

    // Pre-create a book with 5 available copies
    book, _ := svc.CreateBook(ctx, &model.Book{...})

    // Write a reservation.created event to Kafka
    producer, _ := sarama.NewSyncProducer(brokers, producerConfig())
    eventJSON, _ := json.Marshal(reservationEvent{
        EventType: "reservation.created",
        BookID:    book.ID.String(),
    })
    producer.SendMessage(&sarama.ProducerMessage{
        Topic: "reservations",
        Key:   sarama.StringEncoder(book.ID.String()),
        Value: sarama.ByteEncoder(eventJSON),
    })

    // Start consumer in background
    ctx, cancel := context.WithCancel(context.Background())
    go consumer.Run(ctx, brokers, "reservations", svc)

    // Poll database until availability decreases
    deadline := time.After(10 * time.Second)
    for {
        select {
        case <-deadline:
            t.Fatal("timed out waiting for availability update")
        case <-time.After(200 * time.Millisecond):
            updated, _ := repo.GetByID(context.Background(), book.ID)
            if updated.AvailableCopies == 4 {
                cancel()
                return // success
            }
        }
    }
}
```

Explain the synchronization pattern: the consumer runs asynchronously, so poll the database with a timeout. Use unique consumer group IDs per test to avoid rebalancing interference.

5. **Search consumer testing** — same Kafka setup, but the indexer stays mocked:

```go
func TestSearchConsumer_Integration(t *testing.T) {
    brokers := setupKafka(t)
    idx := &capturingIndexer{} // captures Upsert/Delete calls

    // Write a book.created event
    // Start consumer
    // Wait for idx.upserted to have one entry
}
```

The search consumer doesn't need testcontainers Postgres because it talks to Meilisearch (which stays mocked via the `Indexer` interface). The test verifies Kafka consumption and event routing.

6. **Gotchas** section:
   - Consumer group rebalancing: use unique group IDs per test (e.g., `fmt.Sprintf("test-%s", t.Name())`).
   - Topic auto-creation: sarama's producer creates topics automatically with default settings. For tests, this is fine.
   - Container startup: Kafka takes ~10-15s. Share the container across tests in the same package using `TestMain`.
   - Consumer lifecycle: the consumer's `Run` blocks until context cancellation. Always cancel after verification.

Target length: ~300-350 lines.

References:
- [^1]: Testcontainers for Go — Kafka module: https://golang.testcontainers.org/modules/kafka/
- [^2]: IBM/sarama — Go Kafka client: https://github.com/IBM/sarama

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch10/kafka-testing.md
git commit -m "docs: write Section 10.4 Kafka testing"
```

---

### Task 6: Write Section 10.6 — Service-Level End-to-End Tests

**Files:**
- Create: `docs/src/ch10/e2e-testing.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch10/e2e-testing.md` covering:

1. **What "service-level e2e" means** — combine testcontainers (Postgres, Kafka) with bufconn (gRPC) to test one service through its real API with all real dependencies. Not multi-service — each service is tested in isolation. The test proves the entire stack works: gRPC → interceptor → handler → service → repository → Postgres, plus Kafka side effects.

2. **Catalog e2e test** — show the full test setup and flow:

```go
//go:build integration

package e2e_test

func TestCatalog_E2E(t *testing.T) {
    // Setup: testcontainers Postgres + Kafka, real repo, real publisher, bufconn server
    db := setupPostgres(t)
    brokers := setupKafka(t)
    repo := repository.NewBookRepository(db)
    pub, _ := kafkapkg.NewPublisher(brokers, "books")
    svc := service.NewCatalogService(repo, pub)
    client := startCatalogServer(t, svc, "test-secret")

    // Admin JWT for authorized operations
    token, _ := pkgauth.GenerateToken(uuid.New(), "admin", "test-secret", time.Hour)
    ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

    // Flow: create → get → list → update → delete → verify NotFound
    // Also verify Kafka events are published
}
```

Walk through each step of the flow with assertions.

3. **Reservation e2e test** — similar setup but with mock catalog gRPC client (since we're testing reservation in isolation, the catalog service is mocked). Flow: create reservation → verify Postgres → verify Kafka event → return book → verify status → test max-reservations rule.

4. **Auth e2e test** — simplest: just Postgres + bufconn. Flow: register → login → validate token → duplicate registration → wrong password.

5. **Test organization** — e2e tests in `internal/e2e/` package with `//go:build integration`. They import test helpers from the repository and handler test packages (or duplicate them — small helpers are fine to repeat). The `internal/e2e/` package prevents these tests from being importable outside the service.

6. **What we're NOT testing** — multi-service flows (reservation → Kafka → catalog). This would require running multiple services, which crosses the service-level boundary. Briefly mention contract testing (Pact) and gateway HTTP e2e as future directions.

7. **Earthfile integration** — show the complete Earthfile additions for one service and the root orchestration target:

```earthfile
# In services/catalog/Earthfile
integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END

# In root Earthfile
integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test
```

Target length: ~250-300 lines.

References:
- [^1]: Testing microservices — Sam Newman: https://samnewman.io/patterns/testing/
- [^2]: Earthly WITH DOCKER: https://docs.earthly.dev/docs/earthfile#with-docker

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch10/e2e-testing.md
git commit -m "docs: write Section 10.5 service-level end-to-end tests"
```

---

### Task 7: Add testcontainers dependencies to all services

**Files:**
- Modify: `services/catalog/go.mod`
- Modify: `services/auth/go.mod`
- Modify: `services/reservation/go.mod`
- Modify: `services/search/go.mod`

- [ ] **Step 1: Add testcontainers-go to catalog**

```bash
cd services/catalog
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/testcontainers/testcontainers-go/modules/kafka
go mod tidy
```

- [ ] **Step 2: Add testcontainers-go to auth**

```bash
cd services/auth
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
go mod tidy
```

- [ ] **Step 3: Add testcontainers-go to reservation**

```bash
cd services/reservation
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/testcontainers/testcontainers-go/modules/kafka
go mod tidy
```

- [ ] **Step 4: Add testcontainers-go to search**

```bash
cd services/search
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/kafka
go mod tidy
```

- [ ] **Step 5: Commit**

```bash
git add services/*/go.mod services/*/go.sum
git commit -m "deps: add testcontainers-go to all services"
```

---

### Task 8: Catalog Postgres integration tests

**Files:**
- Create: `services/catalog/internal/repository/integration_test.go`

- [ ] **Step 1: Write the integration test file**

Create `services/catalog/internal/repository/integration_test.go` with:

1. `//go:build integration` tag at top
2. `package repository_test`
3. `setupPostgres(t *testing.T) *gorm.DB` helper — starts a testcontainers Postgres, runs migrations from `migrations.FS`, returns `*gorm.DB`, registers `t.Cleanup` for container termination.
4. Integration tests:
   - `TestIntegration_Create` — create a book, verify non-nil UUID.
   - `TestIntegration_DuplicateISBN` — create two books with same ISBN, verify `ErrDuplicateISBN`.
   - `TestIntegration_GetByID` — create then retrieve, verify fields match.
   - `TestIntegration_GetByID_NotFound` — get with random UUID, verify `ErrBookNotFound`.
   - `TestIntegration_Update` — create, modify title, update, verify persisted.
   - `TestIntegration_Delete` — create, delete, verify `ErrBookNotFound` on get.
   - `TestIntegration_List` — create 5 books (3 Fiction, 2 Science), list all (verify total=5), filter by genre (verify total=3).
   - `TestIntegration_UpdateAvailability` — create book with 5 copies, update availability by -1, verify 4 available.

Use the same `newTestBook` helper pattern from the existing `book_test.go`.

- [ ] **Step 2: Verify tests compile**

```bash
cd services/catalog
go test -tags integration -run "TestIntegration" -count=1 ./internal/repository/... 2>&1 | head -20
```

The tests should run and pass (assuming Docker is available). If Docker is not available, they should fail with a clear testcontainers error, not a skip.

- [ ] **Step 3: Commit**

```bash
git add services/catalog/internal/repository/integration_test.go
git commit -m "test: add catalog Postgres integration tests with testcontainers"
```

---

### Task 9: Auth Postgres integration tests

**Files:**
- Create: `services/auth/internal/repository/integration_test.go`

- [ ] **Step 1: Write the integration test file**

Create `services/auth/internal/repository/integration_test.go` with:

1. `//go:build integration` tag
2. `package repository_test`
3. `setupPostgres(t *testing.T) *gorm.DB` helper — same pattern as catalog but using auth's `migrations.FS`.
4. Integration tests:
   - `TestIntegration_CreateUser` — create a user, verify UUID assigned.
   - `TestIntegration_DuplicateEmail` — create two users with same email, verify `ErrDuplicateEmail`.
   - `TestIntegration_GetByEmail` — create user, look up by email, verify match.
   - `TestIntegration_GetByID` — create user, look up by UUID.
   - `TestIntegration_GetByID_NotFound` — random UUID, verify `ErrUserNotFound`.
   - `TestIntegration_GetByOAuthID` — create user with OAuth fields, look up by provider+oauthID.
   - `TestIntegration_Update` — create user, change name, update, verify persisted.

- [ ] **Step 2: Verify tests compile**

```bash
cd services/auth
go test -tags integration -run "TestIntegration" -count=1 ./internal/repository/... 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add services/auth/internal/repository/integration_test.go
git commit -m "test: add auth Postgres integration tests with testcontainers"
```

---

### Task 10: Reservation Postgres integration tests

**Files:**
- Create: `services/reservation/internal/repository/integration_test.go`

- [ ] **Step 1: Write the integration test file**

Create `services/reservation/internal/repository/integration_test.go` with:

1. `//go:build integration` tag
2. `package repository_test`
3. `setupPostgres(t *testing.T) *gorm.DB` helper — uses reservation's `migrations.FS` (NOT `db.AutoMigrate`).
4. Integration tests:
   - `TestIntegration_Create` — create reservation, verify UUID.
   - `TestIntegration_CountActive` — create 3 active + 1 returned, verify count=3.
   - `TestIntegration_GetByID` — create then retrieve.
   - `TestIntegration_GetByID_NotFound` — random UUID.
   - `TestIntegration_ListByUser` — create 2 for user A + 1 for user B, list user A, verify count=2.
   - `TestIntegration_Update` — create, change status to returned, verify persisted.

- [ ] **Step 2: Verify tests compile**

```bash
cd services/reservation
go test -tags integration -run "TestIntegration" -count=1 ./internal/repository/... 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add services/reservation/internal/repository/integration_test.go
git commit -m "test: add reservation Postgres integration tests with testcontainers"
```

---

### Task 11: Catalog bufconn gRPC integration tests

**Files:**
- Create: `services/catalog/internal/handler/grpc_integration_test.go`

- [ ] **Step 1: Write the test file**

Create `services/catalog/internal/handler/grpc_integration_test.go` with:

1. `//go:build integration` tag
2. `package handler_test`
3. `startCatalogServer(t, svc, jwtSecret) catalogv1.CatalogServiceClient` helper — creates bufconn listener, registers CatalogHandler with `pkgauth.UnaryAuthInterceptor`, returns client.
4. `adminCtx(t, jwtSecret)` helper — generates admin JWT, returns context with auth metadata.
5. Tests using mock repo (test interceptor behavior without Postgres):
   - `TestGRPC_CreateBook_Unauthenticated` — call without auth metadata, verify `codes.Unauthenticated`.
   - `TestGRPC_CreateBook_WithAuth` — call with valid admin JWT, verify success.
   - `TestGRPC_GetBook_NotFound` — call with valid auth, random ID, verify `codes.NotFound`.
6. Tests combining bufconn + testcontainers Postgres:
   - `TestGRPC_Integration_CreateAndGet` — create book via gRPC, get via gRPC, verify fields round-trip through protobuf correctly.
   - `TestGRPC_Integration_ListBooks` — create 3 books, list, verify count.

- [ ] **Step 2: Verify tests compile and run**

```bash
cd services/catalog
go test -tags integration -run "TestGRPC" -count=1 ./internal/handler/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/catalog/internal/handler/grpc_integration_test.go
git commit -m "test: add catalog gRPC integration tests with bufconn"
```

---

### Task 12: Auth bufconn gRPC integration tests

**Files:**
- Create: `services/auth/internal/handler/grpc_integration_test.go`

- [ ] **Step 1: Write the test file**

Create `services/auth/internal/handler/grpc_integration_test.go` with:

1. `//go:build integration` tag
2. `package handler_test`
3. `startAuthServer(t, svc) authv1.AuthServiceClient` helper — creates bufconn listener, registers AuthHandler. The auth service skips authentication on Register, Login, and other public methods — pass the correct `skipMethods` to the interceptor.
4. Tests combining bufconn + testcontainers Postgres:
   - `TestGRPC_Register` — register a user via gRPC, verify response has token and user.
   - `TestGRPC_Login` — register then login, verify token returned.
   - `TestGRPC_Register_DuplicateEmail` — register twice with same email, verify `codes.AlreadyExists`.
   - `TestGRPC_Login_WrongPassword` — register, login with wrong password, verify `codes.Unauthenticated`.
   - `TestGRPC_ValidateToken` — register (get token), validate the token via gRPC, verify user ID matches.

- [ ] **Step 2: Verify**

```bash
cd services/auth
go test -tags integration -run "TestGRPC" -count=1 ./internal/handler/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/auth/internal/handler/grpc_integration_test.go
git commit -m "test: add auth gRPC integration tests with bufconn"
```

---

### Task 13: Reservation Kafka producer integration tests

**Files:**
- Create: `services/reservation/internal/kafka/publisher_integration_test.go`

- [ ] **Step 1: Write the test file**

Create `services/reservation/internal/kafka/publisher_integration_test.go` with:

1. `//go:build integration` tag
2. `package kafka_test`
3. Note: the local `kafka` package and `testcontainers-go/modules/kafka` share the same name. Use an import alias: `import kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"`.
4. `setupKafka(t) []string` helper — starts Kafka testcontainer using `kafkatc.Run(...)`, returns broker addresses.
5. Tests:
   - `TestPublisher_Integration_SendMessage` — create publisher, publish a `ReservationEvent`, read back with `sarama.NewConsumer` (partition consumer for simplicity), verify:
     - Message is on the correct topic.
     - Key equals `event.BookID`.
     - Value deserializes to matching `ReservationEvent`.
   - `TestPublisher_Integration_OTelHeaders` — publish with a traced context, verify OTel propagation headers exist in message headers (at least `traceparent`).

- [ ] **Step 2: Verify**

```bash
cd services/reservation
go test -tags integration -run "TestPublisher_Integration" -count=1 ./internal/kafka/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/reservation/internal/kafka/publisher_integration_test.go
git commit -m "test: add reservation Kafka producer integration tests"
```

---

### Task 14: Catalog Kafka consumer integration tests

**Files:**
- Create: `services/catalog/internal/consumer/integration_test.go`

- [ ] **Step 1: Write the test file**

Create `services/catalog/internal/consumer/integration_test.go` with:

1. `//go:build integration` tag
2. `package consumer_test`
3. `setupKafka(t) []string` helper.
4. `setupPostgres(t) *gorm.DB` helper (same pattern as Task 8).
5. Tests:
   - `TestConsumer_Integration_ReservationCreated` — pre-create a book with 5 available copies in Postgres. Produce a `reservation.created` event to Kafka with the book's ID. Start consumer in a goroutine with cancellable context. Poll database with 200ms ticker and 10s timeout until `available_copies == 4`. Cancel consumer. Verify final state.
   - `TestConsumer_Integration_ReservationReturned` — same setup but with `reservation.returned` event, verify availability goes from 5 to 6 (or from 4 to 5 if we decremented first).

Use unique consumer group IDs per test: `fmt.Sprintf("test-%s", t.Name())`.

- [ ] **Step 2: Verify**

```bash
cd services/catalog
go test -tags integration -run "TestConsumer_Integration" -count=1 ./internal/consumer/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/catalog/internal/consumer/integration_test.go
git commit -m "test: add catalog Kafka consumer integration tests"
```

---

### Task 15: Search Kafka consumer integration tests

**Files:**
- Create: `services/search/internal/consumer/integration_test.go`

- [ ] **Step 1: Write the test file**

Create `services/search/internal/consumer/integration_test.go` with:

1. `//go:build integration` tag
2. `package consumer_test`
3. `setupKafka(t) []string` helper.
4. `capturingIndexer` mock — records `Upsert` and `Delete` calls for verification:

```go
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

5. Tests:
   - `TestConsumer_Integration_BookCreated` — produce `book.created` event, start consumer, poll `capturingIndexer.upserted` until length=1, verify document fields.
   - `TestConsumer_Integration_BookDeleted` — produce `book.deleted` event, verify `capturingIndexer.deleted` contains the book ID.

- [ ] **Step 2: Verify**

```bash
cd services/search
go test -tags integration -run "TestConsumer_Integration" -count=1 ./internal/consumer/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/search/internal/consumer/integration_test.go
git commit -m "test: add search Kafka consumer integration tests"
```

---

### Task 16: Catalog service-level e2e test

**Files:**
- Create: `services/catalog/internal/e2e/catalog_e2e_test.go`

- [ ] **Step 1: Write the e2e test**

Create `services/catalog/internal/e2e/catalog_e2e_test.go` with:

1. `//go:build integration` tag
2. `package e2e_test`
3. Setup helpers (duplicated from other test packages since this is a separate package):
   - `setupPostgres(t)` — testcontainers Postgres with catalog migrations.
   - `setupKafka(t)` — testcontainers Kafka.
   - `startCatalogServer(t, svc, jwtSecret)` — bufconn server.
   - `adminCtx(t, jwtSecret)` — auth context.
4. `TestCatalog_E2E` — single test function exercising the full flow:
   - Create a book via gRPC → verify response.
   - Get the book by ID → verify fields match.
   - List books → verify count=1.
   - Update the book title → verify response.
   - Delete the book → verify success.
   - Get the deleted book → verify `codes.NotFound`.
   - (Optionally) Verify a `book.created` event landed on Kafka by reading from the "books" topic.

- [ ] **Step 2: Verify**

```bash
cd services/catalog
go test -tags integration -run "TestCatalog_E2E" -count=1 ./internal/e2e/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/catalog/internal/e2e/catalog_e2e_test.go
git commit -m "test: add catalog service-level e2e test"
```

---

### Task 17: Auth service-level e2e test

**Files:**
- Create: `services/auth/internal/e2e/auth_e2e_test.go`

- [ ] **Step 1: Write the e2e test**

Create `services/auth/internal/e2e/auth_e2e_test.go` with:

1. `//go:build integration` tag
2. `package e2e_test`
3. Setup helpers: `setupPostgres(t)`, `startAuthServer(t, svc)`.
4. `TestAuth_E2E` — full flow:
   - Register user via gRPC → verify token and user returned.
   - Login with correct credentials → verify token.
   - ValidateToken with the login token → verify user ID matches.
   - Register same email again → verify `codes.AlreadyExists`.
   - Login with wrong password → verify `codes.Unauthenticated`.

- [ ] **Step 2: Verify**

```bash
cd services/auth
go test -tags integration -run "TestAuth_E2E" -count=1 ./internal/e2e/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/auth/internal/e2e/auth_e2e_test.go
git commit -m "test: add auth service-level e2e test"
```

---

### Task 18: Reservation service-level e2e test

**Files:**
- Create: `services/reservation/internal/e2e/reservation_e2e_test.go`

- [ ] **Step 1: Write the e2e test**

Create `services/reservation/internal/e2e/reservation_e2e_test.go` with:

1. `//go:build integration` tag
2. `package e2e_test`
3. Setup helpers: `setupPostgres(t)`, `setupKafka(t)`, `startReservationServer(t, svc)`.
4. Mock catalog gRPC client — returns a book with `AvailableCopies > 0` for reservation to succeed. This is needed because `ReservationService` depends on `catalogv1.CatalogServiceClient`.
5. `TestReservation_E2E` — full flow:
   - Create reservation via gRPC (with user context) → verify response.
   - Get reservation → verify fields match.
   - List user reservations → verify count=1.
   - Verify Kafka "reservation.created" event by reading from topic.
   - Return book via gRPC → verify status=returned.
   - Verify Kafka "reservation.returned" event.
   - Test max reservations: create `maxActive` reservations, attempt one more, verify `codes.FailedPrecondition`.

- [ ] **Step 2: Verify**

```bash
cd services/reservation
go test -tags integration -run "TestReservation_E2E" -count=1 ./internal/e2e/... 2>&1 | head -30
```

- [ ] **Step 3: Commit**

```bash
git add services/reservation/internal/e2e/reservation_e2e_test.go
git commit -m "test: add reservation service-level e2e test"
```

---

### Task 19: Add Earthfile integration-test targets

**Files:**
- Modify: `services/catalog/Earthfile`
- Modify: `services/auth/Earthfile`
- Modify: `services/reservation/Earthfile`
- Modify: `services/search/Earthfile`
- Modify: `Earthfile` (root)

- [ ] **Step 1: Add integration-test target to each service Earthfile**

Add the following target to each service Earthfile (after the `test` target):

```earthfile
integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END
```

- [ ] **Step 2: Add orchestration target to root Earthfile**

Add to the root `Earthfile` after the `docker` target:

```earthfile
integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test
```

- [ ] **Step 3: Commit**

```bash
git add Earthfile services/*/Earthfile
git commit -m "build: add integration-test Earthfile targets with WITH DOCKER"
```

---

### Task 20: Final verification and lint

- [ ] **Step 1: Run lint across all services**

```bash
earthly +lint
```

Verify all services pass. Fix any errcheck or other lint violations in the new test files.

- [ ] **Step 2: Run unit tests (existing)**

```bash
earthly +test
```

Verify existing unit tests still pass — the new integration tests should NOT run here (gated by `//go:build integration`).

- [ ] **Step 3: Run integration tests locally (if Docker available)**

```bash
cd services/catalog && go test -tags integration -count=1 -v ./internal/repository/... ./internal/handler/... ./internal/consumer/... ./internal/e2e/...
cd services/auth && go test -tags integration -count=1 -v ./internal/repository/... ./internal/handler/... ./internal/e2e/...
cd services/reservation && go test -tags integration -count=1 -v ./internal/repository/... ./internal/kafka/... ./internal/e2e/...
cd services/search && go test -tags integration -count=1 -v ./internal/consumer/...
```

- [ ] **Step 4: Fix any issues and commit**

```bash
git add -A
git commit -m "fix: address lint and test issues in integration tests"
```
