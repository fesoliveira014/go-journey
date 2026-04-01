# Chapter 10: Testing Strategies for Microservices — Design Spec

## Goal

Take the reader from the unit test patterns established in chapters 1-9 to integration and end-to-end testing with real infrastructure, using testcontainers and bufconn. By the end the reader can write tests that exercise the full path from gRPC request through service logic to a real database and real Kafka broker — all running ephemerally in Docker, requiring no manual infrastructure setup.

## Context

### What Exists

- **Chapter 1.4** covers Go testing basics: `testing.T`, `-race`, `-cover`, `httptest`, black-box testing convention.
- **All 5 services** have unit tests with hand-written mocks (28 test files total). Patterns: black-box `_test` packages, function-based mocks, `t.Helper()`.
- **Repository tests** in catalog and reservation have `t.Skip("TEST_DATABASE_URL not set")` guards — they require manual Postgres setup and rarely run.
- **No external test libraries** — no testify, no gomock. All mocking is interface-based stubs.
- **Kafka producers/consumers** exist (reservation publishes, catalog and search consume) but have no integration tests against a real broker.
- **gRPC handler tests** call handler methods directly as Go functions, bypassing protobuf serialization, interceptors, and status code mapping.

### What's Missing

1. Integration tests against real Postgres (currently mocked or skipped).
2. Integration tests against real Kafka (producer and consumer).
3. gRPC tests that exercise the full stack (serialization, interceptors, status codes).
4. End-to-end tests combining all of the above within a single service boundary.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Test libraries | No new assertion/mock libraries | Keep consistency with ch1-9. Testify/gomock deferred to future revision. |
| Testcontainers scope | All 3 DB services (auth, catalog, reservation) | Complete coverage, no "exercise for the reader" gaps. |
| gRPC testing | bufconn (in-process) | Idiomatic Go, fast, no port conflicts. |
| Kafka testing | Both producer and consumer | Both sides have integration nuances worth teaching. |
| E2E scope | Service-level (single service + real deps) | Sweet spot for learning. Multi-service e2e mentioned but not implemented. |
| Chapter structure | Bottom-up complexity ladder | Each section builds on the previous. |
| Integration test gating | `//go:build integration` tag | Prevents slow tests from running on `go test ./...`. |

## Chapter Structure

### 10.1 — Introduction & Testing Pyramid

Conceptual framing. Chapters 1-9 built unit tests with mocks — fast, focused, but blind to schema mismatches, SQL bugs, serialization errors, and interceptor ordering. This chapter fills the gap.

**Content:**
- The testing pyramid applied to microservices: unit (fast, many) → integration (slower, fewer) → e2e (slowest, fewest).
- Each layer tests what the layer below cannot.
- The cost model: unit tests catch logic bugs cheaply; integration tests catch infrastructure bugs; e2e tests verify flows.
- When mocks lie: examples of bugs that unit tests with mocks cannot catch (wrong SQL column name, Kafka serialization mismatch, gRPC interceptor not wired).

**Length:** ~100-150 lines. No code.

### 10.2 — Unit Testing Patterns

Builds on chapter 1's foundation with patterns the reader has seen but not formalized.

**Content:**
- **Table-driven tests** — the canonical Go pattern. Refactor an existing test (e.g., catalog service `CreateBook` validation) into table-driven form. Show the `[]struct{ name string; ... }` pattern with `t.Run`.
- **Subtests with `t.Run`** — naming conventions, selective execution with `-run "TestCreate/duplicate_isbn"`, parallel subtests with `t.Parallel()`.
- **Test helpers and `t.Helper()`** — formalize the pattern already used in repository tests. Show how `t.Helper()` improves error reporting by attributing failures to the caller.
- **Test fixtures** — when to use `testdata/` directories (e.g., JSON payloads for Kafka events) vs inline data. Mention `embed` for loading fixtures.

**New files:** None. Refactors existing tests.
**New dependencies:** None.
**Length:** ~150-200 lines.

### 10.3 — Integration Testing with Testcontainers (PostgreSQL)

The meatiest section. Replaces `t.Skip("TEST_DATABASE_URL not set")` with ephemeral containers.

**Content:**
- **Why testcontainers** — the problem with manual infrastructure. Tests that skip don't run in CI and rot. Testcontainers spins up real Postgres in Docker, runs tests, tears down. Zero external setup.
- **Adding the dependency** — `testcontainers-go` and the Postgres module.
- **Shared test helper** — a per-service `testutil` (or inline helper function) that:
  1. Starts a Postgres container with the service's database name.
  2. Runs the service's real migrations (using the embedded `migrations.FS`).
  3. Returns a `*gorm.DB` (or `*sql.DB` for auth).
  4. Registers `t.Cleanup()` to terminate the container.
  5. Container reuse across tests in the same package for speed (start once, truncate between tests).
- **Catalog repository integration tests** — rewrite `book_test.go` to use testcontainers. Tests: CRUD, duplicate ISBN constraint (real unique index), pagination with `LIMIT`/`OFFSET`, `UpdateAvailability` with concurrent updates.
- **Auth repository integration tests** — same pattern for user repository. Tests: create user, duplicate email (real unique constraint), lookup by email, OAuth user creation with provider fields.
- **Reservation repository integration tests** — same pattern. Tests: create reservation, count active (real `WHERE status = 'active'`), list by user, status transitions.
- **Migration verification** — integration tests implicitly verify migrations work. This catches migration bugs that unit tests never see (missing columns, wrong constraints, bad default values).
- **Running in CI** — testcontainers needs Docker. Brief note on Earthly compatibility (Earthly runs inside Docker, testcontainers can use the host Docker socket). Earthfile `+test` targets may need `--allow-privileged` or a separate integration test target.

**New files per service:**
- `internal/repository/testutil_test.go` (or helper in existing test file) — testcontainers setup.
- Modifications to existing `*_test.go` to replace manual DB setup.

**New dependencies:**
- `github.com/testcontainers/testcontainers-go`
- `github.com/testcontainers/testcontainers-go/modules/postgres`

**Length:** ~300-400 lines.

### 10.4 — gRPC Testing with bufconn

Closes the gap between "call handler method as Go function" and "call through real gRPC stack."

**Content:**
- **The gap** — current handler tests call `h.CreateBook(ctx, req)` directly. This bypasses protobuf marshaling, interceptors (auth, OTel), metadata, and gRPC status code mapping. A test might pass but the real gRPC call could fail due to a missing interceptor or wrong status code.
- **How bufconn works** — `grpc/test/bufconn` creates an in-memory `net.Listener`. Server listens on it, client dials it. Same process, no network, no ports. Full gRPC stack runs.
- **Catalog service as worked example** — register real `CatalogServiceServer` on a bufconn server with auth interceptor. Connect a real `CatalogServiceClient`. Tests:
  - Create a book with valid admin JWT → success.
  - Create without auth → `codes.Unauthenticated`.
  - Get nonexistent book → `codes.NotFound`.
  - Verify interceptor populates context (user ID, role available in handler).
- **Combining bufconn with testcontainers** — the bufconn server uses a real repository backed by testcontainers Postgres. Full path: gRPC client → protobuf → interceptors → handler → service → repository → real Postgres.
- **Auth service bufconn tests** — register → login → validate token through real gRPC. Verify JWT lifecycle end-to-end.
- **Testing interceptors** — verify that `pkg/auth` interceptor extracts JWT claims and populates context correctly. Something direct-call tests cannot verify.

**New files per service:**
- `internal/handler/grpc_test.go` (or `bufconn_test.go`) — bufconn server setup + integration tests.

**New dependencies:**
- `google.golang.org/grpc/test/bufconn` (already transitively available via grpc dependency).

**Length:** ~250-300 lines.

### 10.5 — Kafka Testing with Testcontainers

Tests both producer and consumer sides against a real broker.

**Content:**
- **Producer testing (reservation service)** — spin up Kafka via testcontainers Kafka module. Create the real `kafka.Publisher`. Publish a reservation event. Read it back with a test consumer (`sarama.NewConsumer`). Verify:
  - Message on correct topic ("reservations").
  - Key is `book_id` (partition ordering).
  - JSON payload deserializes to correct `ReservationEvent` struct.
  - OTel trace headers propagated in Kafka message headers.
- **Consumer testing (catalog service)** — spin up Kafka + Postgres via testcontainers. Write a test message to "reservations" topic. Start the real consumer. Verify:
  - Consumer picks up the message.
  - `UpdateAvailability` is called on the repository.
  - Database state reflects the availability change (verified via real Postgres query).
- **Consumer testing (search service)** — spin up Kafka via testcontainers. Write `book.created`, `book.updated`, `book.deleted` events to "books" topic. Verify the consumer calls the indexer interface correctly. Meilisearch dependency stays mocked (indexer interface) — the interesting part is Kafka consumption, not the Meilisearch API.
- **Shared Kafka test helper** — starts Kafka container, creates topics, returns broker address. Reused across producer and consumer tests.
- **Gotchas section:**
  - Consumer group rebalancing delays: use unique group IDs per test.
  - Topic auto-creation vs explicit creation.
  - Container startup time: Kafka is slower than Postgres (~10-15s). Consider container reuse.
  - Sarama consumer group lifecycle management in tests.

**New files:**
- `services/reservation/internal/kafka/publisher_test.go` — producer integration test.
- `services/catalog/internal/consumer/integration_test.go` — consumer integration test.
- `services/search/internal/consumer/integration_test.go` — consumer integration test.

**New dependencies:**
- `github.com/testcontainers/testcontainers-go/modules/kafka`

**Length:** ~300-350 lines.

### 10.6 — Service-Level End-to-End Tests

Capstone section. Combines testcontainers + bufconn to test full service flows.

**Content:**
- **What "service-level e2e" means** — one service, all real dependencies, real API. Tests verify the full path including side effects (database state, Kafka messages). Not multi-service — each service tested in isolation.
- **Catalog e2e test** — testcontainers (Postgres + Kafka) + bufconn. Flow:
  1. Create a book via gRPC → verify in Postgres.
  2. List books → confirm pagination.
  3. Update the book → verify changes persisted.
  4. Delete → verify `NotFound` on subsequent get.
  5. Verify book events published to Kafka.
- **Reservation e2e test** — testcontainers (Postgres + Kafka) + bufconn. Flow:
  1. Create reservation via gRPC → verify in Postgres.
  2. Verify Kafka event published with correct key/payload.
  3. Return the book → verify status changed.
  4. Verify second Kafka event.
  5. Test max-reservations rule: create 3, attempt 4th, expect `FailedPrecondition`.
- **Auth e2e test** — testcontainers (Postgres) + bufconn. Flow:
  1. Register user → login → validate token.
  2. Register duplicate email → expect `AlreadyExists`.
  3. Login with wrong password → expect `Unauthenticated`.
- **Test organization:**
  - E2e tests in `internal/e2e/` package or `e2e_test.go` with `//go:build integration` tag.
  - `go test ./...` skips them. Earthly `+integration-test` target runs them.
  - Earthfile additions: new `integration-test` target per service with `--allow-privileged` for testcontainers.
- **What we're NOT testing** — multi-service flows (reservation → Kafka → catalog availability update), gateway HTTP e2e, UI testing. Brief pointers to contract testing (Pact), gateway e2e with `httptest`, and frontend testing (Playwright) as "beyond this chapter."

**New files:**
- `services/catalog/internal/e2e/catalog_e2e_test.go`
- `services/reservation/internal/e2e/reservation_e2e_test.go`
- `services/auth/internal/e2e/auth_e2e_test.go`

**Length:** ~250-300 lines.

## New Dependencies Summary

| Dependency | Used In | Purpose |
|------------|---------|---------|
| `testcontainers-go` | auth, catalog, reservation, search | Container lifecycle management |
| `testcontainers-go/modules/postgres` | auth, catalog, reservation | Ephemeral Postgres instances |
| `testcontainers-go/modules/kafka` | catalog, reservation, search | Ephemeral Kafka broker |
| `grpc/test/bufconn` | auth, catalog, reservation | In-memory gRPC connections (already available transitively) |

## Files Created/Modified Summary

### Documentation (new)
- `docs/src/ch10/index.md`
- `docs/src/ch10/unit-testing-patterns.md`
- `docs/src/ch10/integration-testing-postgres.md`
- `docs/src/ch10/grpc-testing.md`
- `docs/src/ch10/kafka-testing.md`
- `docs/src/ch10/e2e-testing.md`
- `docs/src/SUMMARY.md` (modified — add chapter 10 entries)

### Test Code (new/modified)
- Per service: testcontainers helpers, bufconn test setups, integration tests, e2e tests.
- Existing unit tests refactored to table-driven where appropriate (section 10.2).

### Build (modified)
- Service Earthfiles: add `integration-test` target with `--allow-privileged`.
- Root Earthfile: add `integration-test` orchestration target.

## Out of Scope

- Testify, gomock, or any assertion/mocking library (deferred to future revision per roadmap-notes.md).
- Multi-service e2e tests.
- Gateway HTTP e2e tests.
- Frontend/browser testing.
- Contract testing (Pact).
- Benchmarking (`testing.B`).
- Fuzz testing (`testing.F`).
- Meilisearch testcontainers (no official module; search indexer stays mocked).
