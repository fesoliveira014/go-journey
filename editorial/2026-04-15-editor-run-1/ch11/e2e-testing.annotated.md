# 11.5 Service-Level End-to-End Tests

<!-- [STRUCTURAL] Section scope is precisely defined upfront (good!) and the definition is load-bearing for the rest of the chapter. Flow: motivation → scoped definition → infra setup → catalog test → reservation test → auth test → organization → Earthfile → "what this doesn't cover". Good wrap-up with cross-layer summary table. -->
<!-- [STRUCTURAL] Note: the chapter index (index.md) promises that 11.5 "drives the scenario through the gateway's HTTP API" — this is inconsistent with 11.5 as written, which explicitly excludes multi-service flows. Roadmap needs update (flagged in index.md annotated). -->
<!-- [LINE EDIT] "Sections 11.1 through 11.4 each tested one layer of the stack in isolation. Unit tests verified service logic with mocked dependencies. Testcontainers-backed integration tests verified that the SQL was correct. bufconn tests verified that the gRPC wiring — interceptors, metadata, codec — was correct. Kafka tests verified that the serialization round-trip between producer and consumer was correct." 63 words; breaks well by sentence. -->
<!-- [COPY EDIT] "11.1 through 11.4" — correct en dash-free form for section-number range per CMOS 9.63 (use "through" in prose). -->
Sections 11.1 through 11.4 each tested one layer of the stack in isolation. Unit tests verified service logic with mocked dependencies. Testcontainers-backed integration tests verified that the SQL was correct. bufconn tests verified that the gRPC wiring — interceptors, metadata, codec — was correct. Kafka tests verified that the serialization round-trip between producer and consumer was correct.

<!-- [LINE EDIT] "Each of those tests answers a narrow question. None of them answers the question a system operator actually cares about: "if a client sends a valid gRPC request, does the right thing happen all the way down to the database and the message broker?"" 45 words; fine. -->
Each of those tests answers a narrow question. None of them answers the question a system operator actually cares about: "if a client sends a valid gRPC request, does the right thing happen all the way down to the database and the message broker?"

That is the question this section answers.

---

## What "service-level end-to-end" means

<!-- [STRUCTURAL] Excellent structural choice: define the ambiguous term, narrow it, give a diagram, contrast with the bufconn test. This addresses a real confusion reader will have. -->
<!-- [LINE EDIT] "The phrase "end-to-end test" is overloaded. In most contexts it implies a full system test: all five services deployed, a gateway accepting HTTP traffic, a real user logging in and performing an action. That style of test is valuable and is discussed briefly at the end of this section. It is not what we are building here." — fine. -->
<!-- [COPY EDIT] "end-to-end test" — on first use consider the chapter-standard "end-to-end (E2E) test". Align with 11.1's introduction (if adopted). -->
The phrase "end-to-end test" is overloaded. In most contexts it implies a full system test: all five services deployed, a gateway accepting HTTP traffic, a real user logging in and performing an action. That style of test is valuable and is discussed briefly at the end of this section. It is not what we are building here.

<!-- [LINE EDIT] "What we are building is narrower in scope but higher in fidelity than anything in 10.2 through 10.4." — SECTION NUMBER ERROR: "10.2 through 10.4" should be "11.2 through 11.4". -->
<!-- [FINAL] Section-number typo: "10.2 through 10.4" → "11.2 through 11.4". -->
What we are building is narrower in scope but higher in fidelity than anything in 10.2 through 10.4. The term used in this project is **service-level end-to-end test**, and it has a precise meaning:

<!-- [LINE EDIT] Blockquote definition is 44 words. Long but it is a definition; acceptable. -->
<!-- [COPY EDIT] "business-logic handler" — compound adjective; correct. -->
> A test that exercises one service — in isolation from other services — from its public API boundary through every real dependency: gRPC transport, interceptor chain, business-logic handler, service layer, repository, and PostgreSQL. Kafka side effects are verified using a real broker. No mocks replace infrastructure components.

<!-- [LINE EDIT] "The diagram below shows what "end-to-end" means at the service level for the catalog service:" — fine. -->
The diagram below shows what "end-to-end" means at the service level for the catalog service:

```
Test client (bufconn)
     |
     | gRPC request (with JWT in metadata)
     v
 [Auth Interceptor]       <- runs for real, not bypassed
     |
 [CatalogServer.CreateBook]
     |
 [CatalogService]
     |
 [BookRepository]         <- real GORM, real SQL
     |
 [PostgreSQL container]   <- real database, real schema

     ... also:

 [KafkaPublisher]         <- real sarama producer
     |
 [Kafka container]        <- real broker, real topic
```

<!-- [LINE EDIT] "Compare this to the bufconn test from section 11.3. That test also ran through the interceptor and gRPC server. The difference is in the repository layer. Section 11.3's bufconn test could use either a real or a mocked repository — the test goal was to verify the gRPC wiring, so a mock was sufficient. Here the repository is always real. The test goal is to verify that the full vertical slice works end-to-end within a single service." — six sentences; good cadence. -->
<!-- [COPY EDIT] "section 11.3" then "Section 11.3" on next line — inconsistent capitalization. Normalize. -->
Compare this to the bufconn test from section 11.3. That test also ran through the interceptor and gRPC server. The difference is in the repository layer. Section 11.3's bufconn test could use either a real or a mocked repository — the test goal was to verify the gRPC wiring, so a mock was sufficient. Here the repository is always real. The test goal is to verify that the full vertical slice works end-to-end within a single service.

<!-- [LINE EDIT] "This is the Go equivalent of a Spring Boot `@SpringBootTest` with `DEFINED_PORT` and a Testcontainers datasource — the full application context, real database, real request/response cycle." — fine. -->
This is the Go equivalent of a Spring Boot `@SpringBootTest` with `DEFINED_PORT` and a Testcontainers datasource — the full application context, real database, real request/response cycle.

<!-- [LINE EDIT] "Critically, this is **not** a multi-service test. The reservation service does not know about the catalog service's database, and the catalog service does not call the reservation service. Each service is tested in its own test binary, in its own directory, with its own containers. Cross-service flows — a reservation triggering a catalog stock update — are discussed at the end of this section under "what we are not testing."" 62 words across three sentences; third sentence reads fine. -->
<!-- [COPY EDIT] Terminal quote mark: `testing."` — US convention puts period inside quotes (CMOS 6.9). Correct here. -->
Critically, this is **not** a multi-service test. The reservation service does not know about the catalog service's database, and the catalog service does not call the reservation service. Each service is tested in its own test binary, in its own directory, with its own containers. Cross-service flows — a reservation triggering a catalog stock update — are discussed at the end of this section under "what we are not testing."

---

## Test infrastructure setup

<!-- [LINE EDIT] "All service-level e2e tests live under `internal/e2e/` within each service directory. The build tag is `integration`, consistent with everything from section 11.2 onward:" — fine. -->
All service-level e2e tests live under `internal/e2e/` within each service directory. The build tag is `integration`, consistent with everything from section 11.2 onward:

```
services/
  catalog/
    internal/
      e2e/
        catalog_e2e_test.go      // //go:build integration
        helpers_test.go          // shared setup helpers
  reservation/
    internal/
      e2e/
        reservation_e2e_test.go
        helpers_test.go
  auth/
    internal/
      e2e/
        auth_e2e_test.go
        helpers_test.go
```

<!-- [LINE EDIT] "The helpers file in each service is responsible for the three setup functions that every e2e test will call: `setupPostgres`, `setupKafka`, and `startCatalogServer` (or the service-appropriate variant). These are not test cases — they are test infrastructure, so they live in a separate file but in the same `e2e_test` package." — fine. -->
The helpers file in each service is responsible for the three setup functions that every e2e test will call: `setupPostgres`, `setupKafka`, and `startCatalogServer` (or the service-appropriate variant). These are not test cases — they are test infrastructure, so they live in a separate file but in the same `e2e_test` package.

### setupPostgres

```go
//go:build integration

package e2e_test

import (
    "context"
    "testing"
    "time"

    "github.com/golang-migrate/migrate/v4"
    pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/source/iofs"
    "github.com/testcontainers/testcontainers-go"
    kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"
    tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
    gormpostgres "gorm.io/driver/postgres"
    "gorm.io/gorm"

    "github.com/yourorg/library/services/catalog/internal/repository"
    "github.com/yourorg/library/services/catalog/migrations"
)

func setupPostgres(t *testing.T) *gorm.DB {
    t.Helper()
    ctx := context.Background()

    pgContainer, err := tcpostgres.Run(ctx,
        "postgres:16-alpine",
        tcpostgres.WithDatabase("catalog_test"),
        tcpostgres.WithUsername("test"),
        tcpostgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    if err != nil {
        t.Fatalf("failed to start postgres container: %v", err)
    }
    t.Cleanup(func() {
        if err := pgContainer.Terminate(ctx); err != nil {
            t.Logf("failed to terminate postgres container: %v", err)
        }
    })

    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("failed to get connection string: %v", err)
    }

    db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to connect to postgres: %v", err)
    }

    // Run migrations using golang-migrate with embedded SQL files.
    sqlDB, err := db.DB()
    if err != nil {
        t.Fatalf("failed to get sql.DB: %v", err)
    }
    driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
    if err != nil {
        t.Fatalf("failed to create migration driver: %v", err)
    }
    source, err := iofs.New(migrations.FS, ".")
    if err != nil {
        t.Fatalf("failed to create migration source: %v", err)
    }
    m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
    if err != nil {
        t.Fatalf("failed to create migrator: %v", err)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("failed to run migrations: %v", err)
    }

    return db
}
```

<!-- [COPY EDIT] `repository` import is declared but the `setupPostgres` function as shown does not use it. Either remove the import or note that it is for other helpers in the same file. -->
<!-- [COPY EDIT] `kafkatc` import is declared in `setupPostgres`'s import block but only used by `setupKafka` below. Acceptable if helpers share the file; verify code is meant to be in a single helpers_test.go. -->
<!-- [STRUCTURAL] The helper here uses `t.Fatalf` on each migration step (strict) while 11.2's version discards errors with `_` (lax). Pick one convention and align. Strict is safer. -->

<!-- [LINE EDIT] "Two things to note here. First, `setupPostgres` uses the Testcontainers Postgres module (`testcontainers-go/modules/postgres`) rather than the lower-level `GenericContainer`. The module provides typed helpers like `WithDatabase` and `ConnectionString` that eliminate manual host/port assembly. Second, migrations run through `golang-migrate` using the same embedded SQL files that production uses (`migrations.FS`). This guarantees the test schema matches production exactly — if you drift from that, you risk passing e2e tests against a schema that does not match what runs in deployment." 84 words across four sentences; fourth sentence is 26 words. Fine. -->
<!-- [COPY EDIT] "host/port assembly" — slash acceptable; fine. -->
Two things to note here. First, `setupPostgres` uses the Testcontainers Postgres module (`testcontainers-go/modules/postgres`) rather than the lower-level `GenericContainer`. The module provides typed helpers like `WithDatabase` and `ConnectionString` that eliminate manual host/port assembly. Second, migrations run through `golang-migrate` using the same embedded SQL files that production uses (`migrations.FS`). This guarantees the test schema matches production exactly — if you drift from that, you risk passing e2e tests against a schema that does not match what runs in deployment.

### setupKafka

```go
func setupKafka(t *testing.T) []string {
    t.Helper()
    ctx := context.Background()

    kafkaContainer, err := kafkatc.Run(ctx,
        "confluentinc/confluent-local:7.6.0",
    )
    if err != nil {
        t.Fatalf("failed to start kafka container: %v", err)
    }
    t.Cleanup(func() {
        if err := kafkaContainer.Terminate(ctx); err != nil {
            t.Logf("failed to terminate kafka container: %v", err)
        }
    })

    brokers, err := kafkaContainer.Brokers(ctx)
    if err != nil {
        t.Fatalf("failed to get kafka brokers: %v", err)
    }
    return brokers
}
```

<!-- [LINE EDIT] "Unlike PostgreSQL, where we used `GenericContainer`, Kafka uses the Testcontainers Kafka module (`testcontainers-go/modules/kafka`)." — WAIT: the Postgres helper above uses `tcpostgres.Run` from the Postgres MODULE, not `GenericContainer`. This sentence contradicts the preceding code. Correction needed. -->
<!-- [FINAL] Factual inconsistency: the sentence says "Unlike PostgreSQL, where we used `GenericContainer`" but `setupPostgres` above uses `tcpostgres.Run` from the Postgres module. Rewrite to: "Both helpers use Testcontainers modules (`modules/postgres` and `modules/kafka`) rather than the lower-level `GenericContainer`. The Kafka module handles all the KRaft-mode configuration internally — ..." -->
Unlike PostgreSQL, where we used `GenericContainer`, Kafka uses the Testcontainers Kafka module (`testcontainers-go/modules/kafka`). The module handles all the KRaft-mode configuration internally — node IDs, controller quorum voters, listener protocols — so you don't have to set any Kafka environment variables yourself. The `confluent-local` image is purpose-built for single-node testing: it starts in KRaft mode (no ZooKeeper) and auto-creates topics by default.

<!-- [COPY EDIT] "ZooKeeper" — correct brand capitalization. "single-node" — compound adjective before noun; correct. -->
<!-- [COPY EDIT] "don't" — contraction in tutor prose; acceptable. Keep consistent across chapter. -->

<!-- [LINE EDIT] "The return value is a `[]string` of broker addresses — the same type that `sarama.NewSyncProducer` and `sarama.NewConsumerGroup` both accept. Keeping the signature consistent with what your application packages expect means you can pass the slice directly without any adaptation." — fine. -->
The return value is a `[]string` of broker addresses — the same type that `sarama.NewSyncProducer` and `sarama.NewConsumerGroup` both accept. Keeping the signature consistent with what your application packages expect means you can pass the slice directly without any adaptation.

### startCatalogServer

<!-- [LINE EDIT] "The server setup function wires the real dependency graph and starts a bufconn gRPC server, identical to the approach in section 11.3 except it uses a real repository and a real publisher rather than mocks." — fine. -->
The server setup function wires the real dependency graph and starts a bufconn gRPC server, identical to the approach in section 11.3 except it uses a real repository and a real publisher rather than mocks.

```go
func startCatalogServer(t *testing.T, svc catalogpb.CatalogServiceServer, jwtSecret string) catalogpb.CatalogServiceClient {
    t.Helper()

    lis := bufconn.Listen(1024 * 1024)
    t.Cleanup(func() { _ = lis.Close() })

    authInterceptor := interceptor.NewAuthInterceptor(jwtSecret)
    srv := grpc.NewServer(
        grpc.UnaryInterceptor(authInterceptor.Unary()),
    )
    catalogpb.RegisterCatalogServiceServer(srv, svc)

    go func() {
        if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
            t.Logf("server error: %v", err)
        }
    }()
    t.Cleanup(srv.GracefulStop)

    conn, err := grpc.DialContext(
        context.Background(),
        "bufnet",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("failed to dial bufconn: %v", err)
    }
    t.Cleanup(func() { _ = conn.Close() })

    return catalogpb.NewCatalogServiceClient(conn)
}
```

<!-- [FINAL] `grpc.DialContext` with raw `"bufnet"` address — deprecated in grpc-go 1.64+. Use `grpc.NewClient("passthrough:///bufnet", ...)` (as 11.3 does). Align with 11.3. -->
<!-- [COPY EDIT] Please verify: `interceptor.NewAuthInterceptor(jwtSecret)` API — 11.3 uses `pkgauth.UnaryAuthInterceptor(jwtSecret, nil)` returning a `grpc.UnaryServerInterceptor` directly. Two different shapes appear. Is this an intentional refactor across chapters, or a divergence? Unify. -->
<!-- [COPY EDIT] Please verify: `grpc.ErrServerStopped` — actual identifier is `grpc.ErrServerStopped` (available since grpc-go 1.x). Confirm. -->

---

## Catalog e2e test

<!-- [STRUCTURAL] Test walkthrough is long but each step is well-labeled with `--- Step N:` comments. Consider adding a brief intro listing the seven steps before the code block so the reader has a map. -->
With the helpers in place, the test itself reads as a straightforward scenario:

```go
//go:build integration

package e2e_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"

    pkgauth "github.com/yourorg/library/pkg/auth"
    kafkapkg "github.com/yourorg/library/pkg/kafka"
    catalogpb "github.com/yourorg/library/proto/gen/catalog/v1"
    "github.com/yourorg/library/services/catalog/internal/repository"
    "github.com/yourorg/library/services/catalog/internal/service"
)

func TestCatalog_E2E(t *testing.T) {
    db := setupPostgres(t)
    brokers := setupKafka(t)

    repo := repository.NewBookRepository(db)
    pub, err := kafkapkg.NewPublisher(brokers, "books")
    if err != nil {
        t.Fatalf("failed to create kafka publisher: %v", err)
    }
    t.Cleanup(func() { _ = pub.Close() })

    svc := service.NewCatalogService(repo, pub)
    client := startCatalogServer(t, svc, "test-secret")

    // Build an authenticated context using a token signed with the same
    // secret the server's interceptor will verify.
    token, err := pkgauth.GenerateToken(uuid.New(), "admin", "test-secret", time.Hour)
    if err != nil {
        t.Fatalf("failed to generate token: %v", err)
    }
    ctx := metadata.NewOutgoingContext(context.Background(),
        metadata.Pairs("authorization", "Bearer "+token))

    // --- Step 1: Create a book ---
    createResp, err := client.CreateBook(ctx, &catalogpb.CreateBookRequest{
        Title:  "The Go Programming Language",
        Author: "Donovan & Kernighan",
        Isbn:   "978-0134190440",
    })
    if err != nil {
        t.Fatalf("CreateBook failed: %v", err)
    }
    if createResp.Book.Id == "" {
        t.Fatal("CreateBook: expected non-empty book ID")
    }
    bookID := createResp.Book.Id

    // --- Step 2: Get the book back ---
    getResp, err := client.GetBook(ctx, &catalogpb.GetBookRequest{Id: bookID})
    if err != nil {
        t.Fatalf("GetBook failed: %v", err)
    }
    if getResp.Book.Title != "The Go Programming Language" {
        t.Errorf("GetBook: expected title %q, got %q", "The Go Programming Language", getResp.Book.Title)
    }
    if getResp.Book.Isbn != "978-0134190440" {
        t.Errorf("GetBook: expected ISBN %q, got %q", "978-0134190440", getResp.Book.Isbn)
    }

    // --- Step 3: List books — should contain our new entry ---
    listResp, err := client.ListBooks(ctx, &catalogpb.ListBooksRequest{PageSize: 10})
    if err != nil {
        t.Fatalf("ListBooks failed: %v", err)
    }
    if len(listResp.Books) < 1 {
        t.Fatalf("ListBooks: expected at least 1 book, got %d", len(listResp.Books))
    }
    found := false
    for _, b := range listResp.Books {
        if b.Id == bookID {
            found = true
            break
        }
    }
    if !found {
        t.Error("created book should appear in list response")
    }

    // --- Step 4: Update the book ---
    _, err = client.UpdateBook(ctx, &catalogpb.UpdateBookRequest{
        Id:     bookID,
        Title:  "The Go Programming Language (2nd Ed.)",
        Author: "Donovan & Kernighan",
        Isbn:   "978-0134190440",
    })
    if err != nil {
        t.Fatalf("UpdateBook failed: %v", err)
    }

    // Verify the update persisted.
    updatedResp, err := client.GetBook(ctx, &catalogpb.GetBookRequest{Id: bookID})
    if err != nil {
        t.Fatalf("GetBook after update failed: %v", err)
    }
    if updatedResp.Book.Title != "The Go Programming Language (2nd Ed.)" {
        t.Errorf("GetBook: expected updated title %q, got %q", "The Go Programming Language (2nd Ed.)", updatedResp.Book.Title)
    }

    // --- Step 5: Delete the book ---
    _, err = client.DeleteBook(ctx, &catalogpb.DeleteBookRequest{Id: bookID})
    if err != nil {
        t.Fatalf("DeleteBook failed: %v", err)
    }

    // --- Step 6: Get after delete should return NotFound ---
    _, err = client.GetBook(ctx, &catalogpb.GetBookRequest{Id: bookID})
    if err == nil {
        t.Fatal("GetBook after delete: expected error, got nil")
    }
    if status.Code(err) != codes.NotFound {
        t.Errorf("GetBook after delete: expected NotFound, got %v", status.Code(err))
    }

    // --- Step 7: Unauthenticated request should be rejected ---
    unauthCtx := context.Background() // no metadata
    _, err = client.ListBooks(unauthCtx, &catalogpb.ListBooksRequest{})
    if err == nil {
        t.Fatal("ListBooks unauthenticated: expected error, got nil")
    }
    if status.Code(err) != codes.Unauthenticated {
        t.Errorf("ListBooks unauthenticated: expected Unauthenticated, got %v", status.Code(err))
    }
}
```

<!-- [LINE EDIT] "Walk through what each step is actually testing:" — fine. -->
Walk through what each step is actually testing:

<!-- [LINE EDIT] "**Step 1 — Create:** The gRPC request traverses the auth interceptor (which validates the JWT and extracts the caller's identity), reaches the handler, is validated by the service layer, and is persisted by the GORM repository via a real `INSERT` statement. The Kafka publisher also fires a `BookCreated` event to the real broker. The returned `bookID` came from PostgreSQL's auto-generated UUID — if there were a schema mismatch in the primary-key column, this step fails." 77 words across three sentences; first sentence is 42 words. Consider splitting at "reaches the handler" — "The gRPC request traverses the auth interceptor, which validates the JWT and extracts the caller's identity. It then reaches the handler, is validated by the service layer, and is persisted by the GORM repository via a real `INSERT`. The Kafka publisher also fires a `BookCreated` event to the real broker." -->
<!-- [COPY EDIT] "auto-generated" — compound adjective; correct. "primary-key column" — correct. -->
**Step 1 — Create:** The gRPC request traverses the auth interceptor (which validates the JWT and extracts the caller's identity), reaches the handler, is validated by the service layer, and is persisted by the GORM repository via a real `INSERT` statement. The Kafka publisher also fires a `BookCreated` event to the real broker. The returned `bookID` came from PostgreSQL's auto-generated UUID — if there were a schema mismatch in the primary-key column, this step fails.

<!-- [COPY EDIT] "BookCreated" event name vs kafka-testing.md's "reservation.created" vs index.md's "BookReserved" — event naming appears inconsistent across chapter. Please verify canonical event schema. -->

<!-- [LINE EDIT] "**Steps 2 and 3 — Get and List:** These verify that the `SELECT` queries work correctly and that the schema matches what the struct tags declare. A column-name mismatch that the mock would never surface will cause step 2 to return an empty struct or an ORM error here." — fine. -->
**Steps 2 and 3 — Get and List:** These verify that the `SELECT` queries work correctly and that the schema matches what the struct tags declare. A column-name mismatch that the mock would never surface will cause step 2 to return an empty struct or an ORM error here.

<!-- [LINE EDIT] "**Step 4 — Update:** Exercises the `UPDATE` path and immediately re-reads to confirm the write was committed (not rolled back silently due to a missed transaction boundary)." — fine. -->
**Step 4 — Update:** Exercises the `UPDATE` path and immediately re-reads to confirm the write was committed (not rolled back silently due to a missed transaction boundary).

<!-- [LINE EDIT] "**Step 5 and 6 — Delete and NotFound:** Verifies that the soft-delete or hard-delete mechanism in the repository actually makes the row invisible to subsequent reads. Soft-delete bugs — where a `deleted_at` timestamp is set but the `FindByID` query does not filter on it — are caught here but invisible to unit tests." — fine. -->
<!-- [COPY EDIT] "**Step 5 and 6**" — should be "**Steps 5 and 6**" for parallelism with "**Steps 2 and 3**" above. -->
<!-- [FINAL] Heading/label inconsistency: "Step 5 and 6" should be "Steps 5 and 6" (parallel with "Steps 2 and 3"). -->
**Step 5 and 6 — Delete and NotFound:** Verifies that the soft-delete or hard-delete mechanism in the repository actually makes the row invisible to subsequent reads. Soft-delete bugs — where a `deleted_at` timestamp is set but the `FindByID` query does not filter on it — are caught here but invisible to unit tests.

<!-- [LINE EDIT] "**Step 7 — Unauthenticated rejection:** Verifies that the auth interceptor is actually wired into the server. A server started with `grpc.NewServer()` and no interceptors would pass steps 1 through 6 just as well. This step is the proof that the interceptor is present and active. It costs one extra test call and catches the most expensive category of wiring mistake." — four sentences; fine. -->
**Step 7 — Unauthenticated rejection:** Verifies that the auth interceptor is actually wired into the server. A server started with `grpc.NewServer()` and no interceptors would pass steps 1 through 6 just as well. This step is the proof that the interceptor is present and active. It costs one extra test call and catches the most expensive category of wiring mistake.

---

## Reservation e2e test

<!-- [LINE EDIT] "The reservation service is structurally similar to catalog, with two differences. First, it has a dependency on the catalog service's gRPC API — it needs to look up book details when creating a reservation. For a service-level test we mock that outbound gRPC client: we are not testing the catalog service here. Second, the business logic includes a max-active-reservations rule that should return `codes.ResourceExhausted`. That rule cannot be tested by a unit test in isolation — it queries the reservation count from the real database." 84 words across five sentences; fine. -->
<!-- [COPY EDIT] "max-active-reservations rule" — stacked compound; acceptable with hyphens. -->
The reservation service is structurally similar to catalog, with two differences. First, it has a dependency on the catalog service's gRPC API — it needs to look up book details when creating a reservation. For a service-level test we mock that outbound gRPC client: we are not testing the catalog service here. Second, the business logic includes a max-active-reservations rule that should return `codes.ResourceExhausted`. That rule cannot be tested by a unit test in isolation — it queries the reservation count from the real database.

```go
//go:build integration

package e2e_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"

    pkgauth "github.com/yourorg/library/pkg/auth"
    kafkapkg "github.com/yourorg/library/pkg/kafka"
    catalogpb "github.com/yourorg/library/proto/gen/catalog/v1"
    reservationpb "github.com/yourorg/library/proto/gen/reservation/v1"
    "github.com/yourorg/library/services/reservation/internal/repository"
    "github.com/yourorg/library/services/reservation/internal/service"
)

// mockCatalogClient satisfies the catalogpb.CatalogServiceClient interface
// but delegates only the methods the reservation service actually calls.
type mockCatalogClient struct {
    catalogpb.UnimplementedCatalogServiceServer
}

func (m *mockCatalogClient) GetBook(_ context.Context, req *catalogpb.GetBookRequest, _ ...grpc.CallOption) (*catalogpb.GetBookResponse, error) {
    return &catalogpb.GetBookResponse{
        Book: &catalogpb.Book{
            Id:    req.Id,
            Title: "Test Book",
        },
    }, nil
}

func TestReservation_E2E(t *testing.T) {
    db := setupPostgres(t)
    brokers := setupKafka(t)

    repo := repository.NewReservationRepository(db)
    pub, err := kafkapkg.NewPublisher(brokers, "reservations")
    if err != nil {
        t.Fatalf("failed to create kafka publisher: %v", err)
    }
    t.Cleanup(func() { _ = pub.Close() })

    catalogClient := &mockCatalogClient{}
    svc := service.NewReservationService(repo, pub, catalogClient)
    client := startReservationServer(t, svc, "test-secret")

    userID := uuid.New()
    token, err := pkgauth.GenerateToken(userID, "user", "test-secret", time.Hour)
    if err != nil {
        t.Fatalf("failed to generate token: %v", err)
    }
    ctx := metadata.NewOutgoingContext(context.Background(),
        metadata.Pairs("authorization", "Bearer "+token))

    bookID := uuid.New().String()

    // --- Step 1: Create a reservation ---
    createResp, err := client.CreateReservation(ctx, &reservationpb.CreateReservationRequest{
        BookId: bookID,
    })
    if err != nil {
        t.Fatalf("CreateReservation failed: %v", err)
    }
    if createResp.Reservation.Id == "" {
        t.Fatal("CreateReservation: expected non-empty reservation ID")
    }
    reservationID := createResp.Reservation.Id
    if createResp.Reservation.Status != reservationpb.ReservationStatus_ACTIVE {
        t.Errorf("CreateReservation: expected status ACTIVE, got %v", createResp.Reservation.Status)
    }

    // --- Step 2: Verify persistence in DB ---
    // Read directly from the database to confirm the row was written with
    // the correct status, user ID, and book ID — not just that the response
    // said so.
    var count int64
    db.Model(&repository.ReservationRow{}).
        Where("id = ? AND user_id = ? AND status = 'active'", reservationID, userID.String()).
        Count(&count)
    if count != 1 {
        t.Errorf("reservation should be persisted as active: expected count 1, got %d", count)
    }

    // --- Step 3: Verify Kafka event was published ---
    // Consume from the reservations topic and assert that the event matches
    // the reservation we just created.
    event := consumeOneEvent(t, brokers, "reservations")
    if event.Type != "ReservationCreated" {
        t.Errorf("expected event type %q, got %q", "ReservationCreated", event.Type)
    }
    if event.ReservationID != reservationID {
        t.Errorf("expected event reservation ID %q, got %q", reservationID, event.ReservationID)
    }

    // --- Step 4: Return the book ---
    _, err = client.ReturnReservation(ctx, &reservationpb.ReturnReservationRequest{
        ReservationId: reservationID,
    })
    if err != nil {
        t.Fatalf("ReturnReservation failed: %v", err)
    }

    // --- Step 5: Verify status changed to returned ---
    getResp, err := client.GetReservation(ctx, &reservationpb.GetReservationRequest{
        ReservationId: reservationID,
    })
    if err != nil {
        t.Fatalf("GetReservation failed: %v", err)
    }
    if getResp.Reservation.Status != reservationpb.ReservationStatus_RETURNED {
        t.Errorf("expected status RETURNED, got %v", getResp.Reservation.Status)
    }

    // --- Step 6: Max-reservations rule ---
    // The service enforces a maximum of `maxActive` concurrent reservations
    // per user. Create that many reservations for a fresh user, then attempt
    // one more and expect ResourceExhausted.
    const maxActive = 3
    limitedUserID := uuid.New()
    limitedToken, err := pkgauth.GenerateToken(limitedUserID, "user", "test-secret", time.Hour)
    if err != nil {
        t.Fatalf("failed to generate limited user token: %v", err)
    }
    limitedCtx := metadata.NewOutgoingContext(context.Background(),
        metadata.Pairs("authorization", "Bearer "+limitedToken))

    for i := 0; i < maxActive; i++ {
        _, err = client.CreateReservation(limitedCtx, &reservationpb.CreateReservationRequest{
            BookId: uuid.New().String(),
        })
        if err != nil {
            t.Fatalf("reservation %d of %d should succeed: %v", i+1, maxActive, err)
        }
    }

    // One more should be rejected.
    _, err = client.CreateReservation(limitedCtx, &reservationpb.CreateReservationRequest{
        BookId: uuid.New().String(),
    })
    if err == nil {
        t.Fatal("exceeding max active reservations: expected error, got nil")
    }
    if status.Code(err) != codes.ResourceExhausted {
        t.Errorf("exceeding max active reservations: expected ResourceExhausted, got %v", status.Code(err))
    }
}
```

<!-- [FINAL] `mockCatalogClient` struct embeds `catalogpb.UnimplementedCatalogServiceServer` (server-side), but the code then uses it as a `catalogpb.CatalogServiceClient`. The client interface and server interface are different types. Either embed the generated `UnimplementedCatalogServiceServer`-equivalent for the client (usually there is none) or implement each client method explicitly and do not embed. The current code will not compile if `NewReservationService` expects a `catalogpb.CatalogServiceClient`. Please verify. -->
<!-- [COPY EDIT] `grpc` import (for `grpc.CallOption`) is not declared in the import block. Add it. -->
<!-- [FINAL] `startReservationServer` is referenced but never defined in this chapter. Either add the helper (parallel to `startCatalogServer`) or note "(definition follows the same pattern as `startCatalogServer`)". -->
<!-- [FINAL] `consumeOneEvent(t, brokers, "reservations")` is referenced but never defined. Mentioned in prose below as "a helper", but the implementation is not shown. Either provide it inline or factor out to an appendix-style aside. -->

<!-- [LINE EDIT] "Step 3 uses a helper `consumeOneEvent` that creates a short-lived Sarama consumer, subscribes to the topic with a fresh consumer group ID, reads one message, and returns it deserialized. This is the same consumer-side code path that the reservation service uses internally — you are verifying not just that a message was sent, but that it can be received and deserialized by the exact code path a downstream consumer would use." 73 words across two sentences; second is 43 words. Split: "Step 3 uses a helper `consumeOneEvent` that creates a short-lived Sarama consumer, subscribes to the topic with a fresh group ID, reads one message, and returns it deserialized. This is the same consumer-side path the reservation service uses internally. You are verifying not just that a message was sent, but that it can be received and deserialized by the exact code path a downstream consumer would use." -->
Step 3 uses a helper `consumeOneEvent` that creates a short-lived Sarama consumer, subscribes to the topic with a fresh consumer group ID, reads one message, and returns it deserialized. This is the same consumer-side code path that the reservation service uses internally — you are verifying not just that a message was sent, but that it can be received and deserialized by the exact code path a downstream consumer would use.

<!-- [LINE EDIT] "Step 6 exercises the most important business rule in the reservation service, and it is a rule that requires the database to count active reservations. A unit test with a mocked repository can test this rule only by making the mock lie about the count. This e2e test counts real rows in a real table, so the rule is tested against the actual query." — fine. -->
Step 6 exercises the most important business rule in the reservation service, and it is a rule that requires the database to count active reservations. A unit test with a mocked repository can test this rule only by making the mock lie about the count. This e2e test counts real rows in a real table, so the rule is tested against the actual query.

---

## Auth e2e test

<!-- [LINE EDIT] "The auth service has no Kafka dependency. It receives registration and login requests, stores hashed credentials in PostgreSQL, and returns JWTs. The e2e test is the simplest of the three: Postgres plus bufconn." — fine. -->
The auth service has no Kafka dependency. It receives registration and login requests, stores hashed credentials in PostgreSQL, and returns JWTs. The e2e test is the simplest of the three: Postgres plus bufconn.

```go
//go:build integration

package e2e_test

import (
    "context"
    "testing"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    authpb "github.com/yourorg/library/proto/gen/auth/v1"
    "github.com/yourorg/library/services/auth/internal/repository"
    "github.com/yourorg/library/services/auth/internal/service"
)

func TestAuth_E2E(t *testing.T) {
    db := setupPostgres(t)

    repo := repository.NewUserRepository(db)
    svc := service.NewAuthService(repo, "test-jwt-secret")
    client := startAuthServer(t, svc)

    ctx := context.Background()

    // --- Step 1: Register a new user ---
    registerResp, err := client.Register(ctx, &authpb.RegisterRequest{
        Email:    "alice@example.com",
        Password: "correct-horse-battery-staple",
    })
    if err != nil {
        t.Fatalf("Register failed: %v", err)
    }
    if registerResp.UserId == "" {
        t.Fatal("Register: expected non-empty user ID")
    }

    // --- Step 2: Log in with correct credentials ---
    loginResp, err := client.Login(ctx, &authpb.LoginRequest{
        Email:    "alice@example.com",
        Password: "correct-horse-battery-staple",
    })
    if err != nil {
        t.Fatalf("Login failed: %v", err)
    }
    if loginResp.Token == "" {
        t.Fatal("Login: expected non-empty token")
    }

    // --- Step 3: Validate the token ---
    // The token returned by Login should be verifiable by the same service.
    validateResp, err := client.ValidateToken(ctx, &authpb.ValidateTokenRequest{
        Token: loginResp.Token,
    })
    if err != nil {
        t.Fatalf("ValidateToken failed: %v", err)
    }
    if validateResp.UserId != registerResp.UserId {
        t.Errorf("ValidateToken: expected user ID %q, got %q", registerResp.UserId, validateResp.UserId)
    }
    if validateResp.Email != "alice@example.com" {
        t.Errorf("ValidateToken: expected email %q, got %q", "alice@example.com", validateResp.Email)
    }
    if validateResp.Expired {
        t.Error("ValidateToken: expected Expired to be false")
    }

    // --- Step 4: Duplicate email should be rejected ---
    _, err = client.Register(ctx, &authpb.RegisterRequest{
        Email:    "alice@example.com",
        Password: "different-password",
    })
    if err == nil {
        t.Fatal("Register duplicate email: expected error, got nil")
    }
    if status.Code(err) != codes.AlreadyExists {
        t.Errorf("Register duplicate email: expected AlreadyExists, got %v", status.Code(err))
    }

    // --- Step 5: Wrong password should be rejected ---
    _, err = client.Login(ctx, &authpb.LoginRequest{
        Email:    "alice@example.com",
        Password: "wrong-password",
    })
    if err == nil {
        t.Fatal("Login wrong password: expected error, got nil")
    }
    if status.Code(err) != codes.Unauthenticated {
        t.Errorf("Login wrong password: expected Unauthenticated, got %v", status.Code(err))
    }

    // --- Step 6: Expired token should be rejected ---
    expiredToken, err := pkgauth.GenerateToken(uuid.New(), "user", "test-jwt-secret", -1*time.Second)
    if err != nil {
        t.Fatalf("failed to generate expired token: %v", err)
    }
    validateExpiredResp, err := client.ValidateToken(ctx, &authpb.ValidateTokenRequest{
        Token: expiredToken,
    })
    // Depending on your API design, ValidateToken may return a response with
    // Expired: true rather than an error. Assert the design you chose.
    if err != nil {
        if status.Code(err) != codes.Unauthenticated {
            t.Errorf("ValidateToken expired: expected Unauthenticated, got %v", status.Code(err))
        }
    } else {
        if !validateExpiredResp.Expired {
            t.Error("ValidateToken expired: expected Expired to be true")
        }
    }
}
```

<!-- [FINAL] `pkgauth` and `uuid` are used in Step 6 but are not in the import block for this test file. Add them (they exist in the reservation test's import list; just need to add them here too). -->
<!-- [FINAL] `startAuthServer` takes two arguments here (`t, svc`), while the parallel helper `startCatalogServer` takes three (`t, svc, jwtSecret`). Either show `startAuthServer`'s signature or note that it binds the JWT secret differently. The code is plausible but visibly underspecified. -->

<!-- [LINE EDIT] "The auth test deliberately avoids Kafka because the auth service does not publish events. Adding a Kafka container to this test would be dishonest — it would imply a dependency that does not exist and would slow down the suite for no benefit. Keep the infrastructure footprint of each e2e test matched to the actual dependencies of the service under test." 56 words; fine. -->
The auth test deliberately avoids Kafka because the auth service does not publish events. Adding a Kafka container to this test would be dishonest — it would imply a dependency that does not exist and would slow down the suite for no benefit. Keep the infrastructure footprint of each e2e test matched to the actual dependencies of the service under test.

<!-- [LINE EDIT] "Step 6 tests an edge case that only exists at the level of a full integration: the token is generated outside the auth service (simulating a token that was valid at login but has since expired), passed to `ValidateToken`, and rejected. The expiry check runs against the real system clock in the real JWT library — no mocked time, no stubbed clock interface." 64 words across two sentences; fine. -->
Step 6 tests an edge case that only exists at the level of a full integration: the token is generated outside the auth service (simulating a token that was valid at login but has since expired), passed to `ValidateToken`, and rejected. The expiry check runs against the real system clock in the real JWT library — no mocked time, no stubbed clock interface.

---

## Test organization

### Directory layout

<!-- [LINE EDIT] "The convention established in section 11.2 is to put integration tests in files named `*_integration_test.go`. Service-level e2e tests follow the same convention but live in a dedicated `internal/e2e/` package to keep them separate from the repository-level integration tests:" — fine. -->
<!-- [COPY EDIT] "section 11.2" — lowercase; consistent with rest of chapter after normalization. -->
The convention established in section 11.2 is to put integration tests in files named `*_integration_test.go`. Service-level e2e tests follow the same convention but live in a dedicated `internal/e2e/` package to keep them separate from the repository-level integration tests:

```
services/
  catalog/
    internal/
      repository/
        book_repository_test.go            // unit: uses t.Skip guard
        book_repository_integration_test.go // //go:build integration
      e2e/
        catalog_e2e_test.go                // //go:build integration
        helpers_test.go                    // //go:build integration
```

<!-- [LINE EDIT] "The existing repository tests that use `t.Skip` are left exactly as they are. They serve as documentation of the test intent and as a fallback for developers who do not have Docker available. The integration tests run in a separate build and complement rather than replace the existing tests." — fine. -->
<!-- [STRUCTURAL] This contradicts 11.2's recommendation to replace `t.Skip` with Testcontainers startup. 11.2: "replace `t.Skipf` with a Testcontainers startup so the test always runs in CI". 11.5 here: "The existing repository tests that use `t.Skip` are left exactly as they are." Resolve: either keep both (and explain as a deliberate "two-track" strategy) or align. -->
<!-- [FINAL] Direct contradiction with 11.2. 11.2 recommends replacing `t.Skip` with Testcontainers; 11.5 says leave it. Reconcile the advice. -->
The existing repository tests that use `t.Skip` are left exactly as they are. They serve as documentation of the test intent and as a fallback for developers who do not have Docker available. The integration tests run in a separate build and complement rather than replace the existing tests.

### Build tag discipline

<!-- [LINE EDIT] "Every file under `internal/e2e/` carries `//go:build integration` at the very top, before the `package` declaration. This is not optional — if any file in the package is missing the tag, `go test ./...` will try to compile the package and fail because `testcontainers-go` imports Docker client libraries that are heavy dependencies." 56 words across two sentences; fine. -->
<!-- [COPY EDIT] "very top" → "top" (cut filler). -->
Every file under `internal/e2e/` carries `//go:build integration` at the very top, before the `package` declaration. This is not optional — if any file in the package is missing the tag, `go test ./...` will try to compile the package and fail because `testcontainers-go` imports Docker client libraries that are heavy dependencies.

```go
//go:build integration

package e2e_test
```

<!-- [LINE EDIT] "The `_test` suffix on the package name is intentional. It places these tests in the external test package, which means they can only access exported symbols from the service. This enforces the same boundary that a real client of the service would face — if your service's public API is awkward to use from tests, it is awkward for real callers too." — fine. -->
<!-- [COPY EDIT] "external test package" — Go convention term. Correct. -->
The `_test` suffix on the package name is intentional. It places these tests in the external test package, which means they can only access exported symbols from the service. This enforces the same boundary that a real client of the service would face — if your service's public API is awkward to use from tests, it is awkward for real callers too.

### Running tests

```bash
# Run unit tests only — fast, no Docker required.
go test ./...

# Run everything including integration and e2e.
go test -tags integration ./...

# Run only the e2e package for one service.
go test -tags integration ./services/catalog/internal/e2e/...

# Verbose output with timing (useful during development).
go test -tags integration -v -count=1 ./services/catalog/internal/e2e/...
```

<!-- [LINE EDIT] "The `-count=1` flag disables Go's test result cache. Without it, Go will cache the result of a passing test and not re-run it. For e2e tests that depend on external state (containers that are freshly started each run), caching is almost always wrong." — fine. -->
<!-- [COPY EDIT] "test result cache" → "test-result cache" (compound adjective). -->
The `-count=1` flag disables Go's test result cache. Without it, Go will cache the result of a passing test and not re-run it. For e2e tests that depend on external state (containers that are freshly started each run), caching is almost always wrong.

---

## Earthfile integration

<!-- [STRUCTURAL] Mostly duplicative of 11.2's Earthly section. Consider trimming one of the two to avoid reader fatigue, or having one section refer back to the other for shared mechanics. -->
<!-- [LINE EDIT] "Earthly's `WITH DOCKER` block provides Docker-in-Docker capability that makes integration tests portable across CI environments. The pattern established in section 11.4 for Kafka tests applies directly to e2e tests — the test binary itself starts and stops containers via the Docker socket, so the only Earthly requirement is that Docker is available inside the build step." 55 words across two sentences; fine. -->
<!-- [COPY EDIT] "section 11.4" — lowercase after normalization. -->
Earthly's `WITH DOCKER` block provides Docker-in-Docker capability that makes integration tests portable across CI environments. The pattern established in section 11.4 for Kafka tests applies directly to e2e tests — the test binary itself starts and stops containers via the Docker socket, so the only Earthly requirement is that Docker is available inside the build step.

```earthfile
# In services/catalog/Earthfile

src:
    FROM golang:1.22-alpine
    WORKDIR /app
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .

test:
    FROM +src
    RUN go test ./...

integration-test:
    FROM +src
    WITH DOCKER
        RUN go test -tags integration ./... -v -count=1
    END
```

<!-- [COPY EDIT] Please verify: `golang:1.22-alpine` — with current Go being 1.24+ as of 2026, consider bumping or noting version-neutrality. -->

<!-- [LINE EDIT] "The `integration-test` target is separate from `test`. This means CI can run `+test` on every push (fast, no Docker required inside the build) and run `+integration-test` on pull requests or on a scheduled pipeline (slower, requires Docker)." — fine. -->
The `integration-test` target is separate from `test`. This means CI can run `+test` on every push (fast, no Docker required inside the build) and run `+integration-test` on pull requests or on a scheduled pipeline (slower, requires Docker).

The root Earthfile aggregates the per-service targets:

```earthfile
# In root Earthfile

test:
    BUILD ./services/auth+test
    BUILD ./services/catalog+test
    BUILD ./services/reservation+test
    BUILD ./services/search+test

integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test
```

<!-- [STRUCTURAL] 11.2's root Earthfile aggregate also includes the gateway service; this one omits it. Align: either both include gateway or both explain the omission. -->

<!-- [LINE EDIT] "Earthly executes independent `BUILD` targets in parallel by default. The four service integration tests will run concurrently, each with their own Docker daemon scope. There is no shared state between them — each service starts its own Postgres and Kafka containers, which means total wall-clock time for the full integration suite is bounded by the slowest single service rather than the sum of all four." 68 words; third sentence is 40 words. Fine. -->
<!-- [COPY EDIT] "each with their own Docker daemon scope" — "Docker daemon scope" is a noun chunk; consider "each with its own Docker-daemon scope" (singular possessive agreement + compound). -->
Earthly executes independent `BUILD` targets in parallel by default. The four service integration tests will run concurrently, each with their own Docker daemon scope. There is no shared state between them — each service starts its own Postgres and Kafka containers, which means total wall-clock time for the full integration suite is bounded by the slowest single service rather than the sum of all four.

<!-- [LINE EDIT] "Invoking the full suite from the project root:" — fine. -->
Invoking the full suite from the project root:

```bash
# Run all unit tests across all services.
earthly +test

# Run all integration and e2e tests across all services.
earthly +integration-test
```

<!-- [LINE EDIT] "In GitHub Actions, you add a separate job in the workflow:" — fine. -->
In GitHub Actions, you add a separate job in the workflow:

```yaml
integration-test:
  runs-on: ubuntu-latest
  needs: [test]           # Only run if unit tests pass.
  steps:
    - uses: actions/checkout@v4
    - uses: earthly/actions-setup@v1
      with:
        version: latest
    - run: earthly +integration-test
```

<!-- [COPY EDIT] "version: latest" — pinning to `latest` in CI is fragile; mention or use a pinned version like `v0.8`. -->
<!-- [COPY EDIT] Please verify: `earthly/actions-setup@v1` — confirm currency. -->
<!-- [COPY EDIT] Missing `--allow-privileged` flag that 11.2 notes is required. If 11.2 says `--allow-privileged` is needed on GitHub Actions and 11.5 omits it, reconcile. -->

<!-- [LINE EDIT] "The `needs: [test]` dependency means integration tests only run when the fast tests are green. This is the same principle as the test pyramid: do not spend 90 seconds starting containers if a 3-second unit test already failed." — fine. -->
<!-- [COPY EDIT] "90 seconds" / "3-second" — numerical style: "3-second unit test" is compound adjective, hyphenated correctly. "90 seconds" as noun, correct. Keep. -->
The `needs: [test]` dependency means integration tests only run when the fast tests are green. This is the same principle as the test pyramid: do not spend 90 seconds starting containers if a 3-second unit test already failed.

---

## What this section does not cover

<!-- [STRUCTURAL] Good "negative-space" section — explicitly lists what the suite does not test. This is strong editorial discipline for a test-strategy chapter. -->
<!-- [LINE EDIT] "These tests deliberately stop short of multi-service flows. The test suite as built tells you:" — fine. -->
These tests deliberately stop short of multi-service flows. The test suite as built tells you:

- The catalog service correctly persists a book, publishes an event, and serves the data back through its gRPC API.
- The reservation service correctly enforces business rules, persists reservations, and publishes events.
- The auth service correctly hashes passwords, issues JWTs, and rejects bad credentials.

What it does not tell you:

<!-- [LINE EDIT] "Whether the `BookCreated` event published by the catalog service can be correctly consumed by the reservation service. The event schema was verified in section 11.4, but a field rename in the proto definition would be caught by that Kafka round-trip test, not by this one." — fine. -->
<!-- [COPY EDIT] "BookCreated" event — again inconsistent with 11.4's "reservation.created". Also: 11.4's tests publish from reservation to be consumed by catalog (availability updates); catalog's `BookCreated` flowing to reservation is the reverse direction. Please verify the event flow model. -->
- Whether the `BookCreated` event published by the catalog service can be correctly consumed by the reservation service. The event schema was verified in section 11.4, but a field rename in the proto definition would be caught by that Kafka round-trip test, not by this one.
- Whether the gateway correctly routes HTTP requests to the right gRPC service and translates responses.
- Whether the OAuth2 login flow with Gmail works end-to-end, including the redirect and token exchange.

These gaps are addressable in several ways:

<!-- [LINE EDIT] "**Contract testing with Pact**[^1] defines a consumer-driven contract: the reservation service declares what it expects from the catalog service's Kafka events, and the catalog service verifies its output against those expectations. Neither service needs to run at the same time. This is the recommended approach for verifying the serialization contract between independently-deployed services." 53 words across three sentences; fine. -->
<!-- [COPY EDIT] "independently-deployed" — compound adjective; correct. "consumer-driven contract" — correct. -->
<!-- [COPY EDIT] Please verify: footnote [^1] URL for Pact/Sam Newman link — currently points to "testing microservices — Sam Newman". Pact has its own canonical documentation URL. Consider adding a dedicated Pact footnote. -->
**Contract testing with Pact**[^1] defines a consumer-driven contract: the reservation service declares what it expects from the catalog service's Kafka events, and the catalog service verifies its output against those expectations. Neither service needs to run at the same time. This is the recommended approach for verifying the serialization contract between independently-deployed services.

<!-- [LINE EDIT] "**Gateway-level HTTP e2e tests** start all services and the gateway in containers and exercise user-facing scenarios through HTTP. These tests are expensive — 60 to 120 seconds to start — and are appropriate for a small set of critical user paths: login, reserve a book, return a book." — fine. -->
<!-- [COPY EDIT] "60 to 120 seconds" — in the same chapter, index.md uses "30–120 s per scenario" in a table; here "60 to 120 seconds". Align. -->
**Gateway-level HTTP e2e tests** start all services and the gateway in containers and exercise user-facing scenarios through HTTP. These tests are expensive — 60 to 120 seconds to start — and are appropriate for a small set of critical user paths: login, reserve a book, return a book.

<!-- [LINE EDIT] "**Frontend and browser tests** using Playwright or Cypress are only relevant once the application has a browser-facing UI. They sit at the very top of the pyramid and should be reserved for the handful of user journeys that are truly business-critical." — fine. -->
<!-- [COPY EDIT] "browser-facing UI" — compound adjective; correct. "business-critical" — compound adjective; correct. -->
**Frontend and browser tests** using Playwright or Cypress are only relevant once the application has a browser-facing UI. They sit at the very top of the pyramid and should be reserved for the handful of user journeys that are truly business-critical.

<!-- [LINE EDIT] "The service-level e2e tests in this section occupy a useful middle ground. They are faster than full-system tests because they start one service's containers rather than all five. They are more realistic than integration tests because they test the full vertical slice through one service. For a team with limited CI budget, they are often the best return on investment: high confidence, moderate cost, zero cross-service coordination overhead." 65 words across four sentences; fine. -->
<!-- [COPY EDIT] "full-system tests" — compound adjective; correct. "cross-service coordination overhead" — noun stack; acceptable. "return on investment" — standard idiom, no hyphens. -->
The service-level e2e tests in this section occupy a useful middle ground. They are faster than full-system tests because they start one service's containers rather than all five. They are more realistic than integration tests because they test the full vertical slice through one service. For a team with limited CI budget, they are often the best return on investment: high confidence, moderate cost, zero cross-service coordination overhead.

---

## Putting it all together

<!-- [LINE EDIT] "Looking back at the full test strategy from this chapter:" — fine. -->
Looking back at the full test strategy from this chapter:

<!-- [COPY EDIT] Table: consistent casing in "Tool" column ("testing + testify/mock", "Testcontainers + GORM", "bufconn + interceptors", "Testcontainers + Sarama", "All of the above"). Minor: "testing + testify/mock" — the Go standard package is `testing`. Correct. "testify/mock" lowercase matches package path. -->
<!-- [COPY EDIT] Table row "Service e2e" — use "E2E" for consistency if "E2E" was adopted earlier. Or keep lowercase. Normalize across whole chapter. -->
| Layer | Tool | What it catches |
|---|---|---|
| Unit | `testing` + `testify/mock` | Logic bugs, edge cases in business rules |
| Integration | Testcontainers + GORM | SQL/schema mismatches, ORM configuration errors |
| gRPC | bufconn + interceptors | Missing interceptors, wrong service registration |
| Kafka | Testcontainers + Sarama | Serialization mismatches, consumer-group offsets |
| Service e2e | All of the above | Full vertical slice: request → DB → event |

<!-- [COPY EDIT] Unicode arrow → used in table cell "request → DB → event". Earlier sections use ASCII "->". Normalize one way across chapter. -->

<!-- [LINE EDIT] "Each layer catches a distinct category of bug. The service-level e2e tests catch the bugs that no individual layer would catch on its own: the subtle interactions between layers — an interceptor that passes but corrupts the context for the handler below it, a repository that writes the row but fails to commit it, a publisher that serializes the event but uses the wrong topic name." 67 words across two sentences; second is 49 words. Long but rhythmical (three parallel "a X that Y but Z" clauses). Keep. -->
Each layer catches a distinct category of bug. The service-level e2e tests catch the bugs that no individual layer would catch on its own: the subtle interactions between layers — an interceptor that passes but corrupts the context for the handler below it, a repository that writes the row but fails to commit it, a publisher that serializes the event but uses the wrong topic name.

<!-- [LINE EDIT] "These are the bugs that make it to production when teams trust only unit tests. They are also the bugs that are most expensive to diagnose in production, because they only manifest when the full stack is present." — fine. -->
These are the bugs that make it to production when teams trust only unit tests. They are also the bugs that are most expensive to diagnose in production, because they only manifest when the full stack is present.

<!-- [LINE EDIT] "The test suite you now have is not exhaustive. No test suite is. But it is stratified correctly: fast tests at the base for rapid feedback, expensive tests at the top for confidence, and a clear mapping from test to what it is designed to find." — fine, rhythmic ending. -->
The test suite you now have is not exhaustive. No test suite is. But it is stratified correctly: fast tests at the base for rapid feedback, expensive tests at the top for confidence, and a clear mapping from test to what it is designed to find.

---

<!-- [COPY EDIT] Footnote [^1]: "Testing microservices — Sam Newman" doesn't clearly correspond to "Contract testing with Pact" cited in prose. Consider citing Pact's docs directly; Sam Newman covers microservice testing broadly. Please verify the intended citation. -->
[^1]: Testing microservices — Sam Newman: https://samnewman.io/patterns/testing/
[^2]: Earthly WITH DOCKER: https://docs.earthly.dev/docs/earthfile#with-docker
<!-- [COPY EDIT] Please verify: both footnote URLs still resolve. -->
