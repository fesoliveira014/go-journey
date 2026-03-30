# Chapter 6: Reservation Service & Kafka — Design Spec

## Overview

Chapter 6 introduces event-driven architecture by building a Reservation service and integrating Apache Kafka for asynchronous inter-service communication. Users can reserve and return books through the gateway. The reservation service validates availability via synchronous gRPC calls to the catalog, then publishes state-change events to Kafka. The catalog service runs a co-located consumer goroutine that listens for these events and updates book availability.

This chapter teaches: Kafka fundamentals (topics, partitions, consumer groups), the sarama Go client, event-driven vs. synchronous communication tradeoffs, eventual consistency, and building a complete microservice end-to-end.

## Architecture

### Synchronous Flow (gRPC)

```
Browser → Gateway (HTTP) → Reservation Service (gRPC)
                                    ↓
                           Catalog Service (gRPC) — availability check only
```

The reservation service reads from the catalog synchronously to verify a book has available copies before creating a reservation.

### Asynchronous Flow (Kafka)

```
Reservation Service → publishes event → "reservations" topic → Catalog consumer → updates available copies
```

The reservation service does NOT call `UpdateAvailability` directly. Instead, it publishes an event and trusts the event pipeline. The catalog service runs a consumer goroutine (in `internal/consumer/`) that processes events and updates copies internally via the service layer.

### Key Design Choice

Reads are synchronous (gRPC). Writes are asynchronous (Kafka). This creates a brief window of eventual consistency between reservation creation and availability update. In practice the delay is milliseconds, but the tutorial documents this explicitly.

## Reservation Service

### Proto Definition

File: `proto/reservation/v1/reservation.proto`

```protobuf
syntax = "proto3";
package reservation.v1;

option go_package = "github.com/fesoliveira014/library-system/gen/reservation/v1;reservationv1";

import "google/protobuf/timestamp.proto";

service ReservationService {
  rpc CreateReservation(CreateReservationRequest) returns (CreateReservationResponse);
  rpc ReturnBook(ReturnBookRequest) returns (ReturnBookResponse);
  rpc GetReservation(GetReservationRequest) returns (Reservation);
  rpc ListUserReservations(ListUserReservationsRequest) returns (ListUserReservationsResponse);
}

message CreateReservationRequest {
  string book_id = 1;
}

message CreateReservationResponse {
  Reservation reservation = 1;
}

message ReturnBookRequest {
  string reservation_id = 1;
}

message ReturnBookResponse {
  Reservation reservation = 1;
}

message GetReservationRequest {
  string reservation_id = 1;
}

message ListUserReservationsRequest {}

message ListUserReservationsResponse {
  repeated Reservation reservations = 1;
}

message Reservation {
  string id = 1;
  string user_id = 2;
  string book_id = 3;
  string status = 4;
  google.protobuf.Timestamp reserved_at = 5;
  google.protobuf.Timestamp due_at = 6;
  google.protobuf.Timestamp returned_at = 7;
}
```

Notes:
- `user_id` is NOT in `CreateReservationRequest` — it comes from the JWT context (users can only create their own reservations).
- `ListUserReservationsRequest` has no fields — the user ID comes from context.
- All four RPCs are protected by the `pkg/auth` interceptor (no anonymous access).

### Data Model

Table: `reservations`

| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK, generated |
| user_id | UUID | From JWT context |
| book_id | UUID | References catalog book (no cross-DB FK) |
| status | VARCHAR(20) | `active`, `returned`, `expired` |
| reserved_at | TIMESTAMPTZ | Set on creation |
| due_at | TIMESTAMPTZ | reserved_at + 14 days |
| returned_at | TIMESTAMPTZ | Nullable, set on return |

Indexes:
- `idx_reservations_user_status` on `(user_id, status)` — for counting active reservations and listing by user.
- `idx_reservations_book_status` on `(book_id, status)` — for future queries by book.

The model is designed with future extensibility in mind (e.g., adding a waitlist/notification feature later). No columns need to be added for that — a separate `waitlist_entries` table would reference `book_id`.

### State Machine

```
ACTIVE → RETURNED  (user returns book)
       → EXPIRED   (checked on read, past due_at)
```

Two terminal states. No `CANCELLED` state — once a reservation is active, the book is in the user's hands.

### Service Layer Logic

**CreateReservation:**
1. Count active reservations for user. If >= `MAX_ACTIVE_RESERVATIONS` (env var, default 5), return `codes.ResourceExhausted`.
2. Call catalog `GetBook` via gRPC to check `available_copies > 0`. If not, return `codes.FailedPrecondition` with message "no copies available".
3. Insert reservation row with status `active`, `due_at = now + 14 days`.
4. Publish `reservation.created` event to Kafka.
5. Return the reservation.

**ReturnBook:**
1. Find reservation by ID. If not found, return `codes.NotFound`.
2. Verify reservation belongs to the requesting user (from JWT context). If not, return `codes.PermissionDenied`.
3. Verify status is `active`. If not, return `codes.FailedPrecondition`.
4. Set status `returned`, set `returned_at = now`.
5. Publish `reservation.returned` event.
6. Return the updated reservation.

**GetReservation / ListUserReservations:**
1. Fetch reservation(s) for the requesting user.
2. For each `active` reservation where `due_at < now`: transition to `expired`, publish `reservation.expired` event.
3. Return the (possibly updated) reservation(s).

This "expiration on read" pattern avoids introducing a background worker or cron job. The tradeoff is that expiration only happens when someone queries — if no one reads, expiration events are delayed. This is acceptable for a learning project and the tutorial should note it as a simplification.

### Architecture

Same layered pattern as auth and catalog:

```
internal/
  handler/    — gRPC handler, converts proto ↔ domain
  service/    — business logic, orchestrates repo + events
  repository/ — GORM persistence
  model/      — domain types
```

The event publisher is injected as an interface:

```go
type EventPublisher interface {
    Publish(ctx context.Context, event ReservationEvent) error
}
```

This allows tests to use a mock publisher without touching Kafka.

## Kafka Integration

### Infrastructure

Apache Kafka 3.9 in KRaft mode (no Zookeeper). The official `apache/kafka` image handles KRaft setup automatically.

Single broker, sufficient for local development.

### Topic

One topic: `reservations`, 3 partitions. Messages are keyed by `book_id` so all events for the same book land on the same partition, guaranteeing per-book ordering.

Auto-created on first publish (`KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`).

### Event Schema

JSON-encoded messages:

```json
{
  "event_type": "reservation.created",
  "reservation_id": "uuid",
  "user_id": "uuid",
  "book_id": "uuid",
  "timestamp": "2026-03-30T12:00:00Z"
}
```

Three event types: `reservation.created`, `reservation.returned`, `reservation.expired`.

The consumer only needs `event_type` and `book_id`:
- `reservation.created` → decrement available copies
- `reservation.returned` → increment available copies
- `reservation.expired` → increment available copies

### Producer

The reservation service uses `github.com/IBM/sarama` to create a `SyncProducer`. Synchronous publishing ensures the event is durably written before the RPC returns. The producer is created in `main.go` and injected into the service layer.

The producer wraps sarama behind the `EventPublisher` interface so the service layer is not coupled to the Kafka library.

### Consumer (in Catalog Service)

A new package: `services/catalog/internal/consumer/`

```go
func Run(ctx context.Context, brokers []string, topic string, svc *service.CatalogService) error
```

- Creates a `sarama.ConsumerGroup` with group ID `catalog-availability-updater`.
- Listens on the `reservations` topic.
- On `reservation.created`: calls `svc.DecrementAvailability(ctx, bookID)`.
- On `reservation.returned` or `reservation.expired`: calls `svc.IncrementAvailability(ctx, bookID)`.

The consumer runs as a goroutine started from the catalog service's `main.go`. A `context.Context` with cancel controls graceful shutdown.

### New Catalog Service Layer Methods

Two new methods on the catalog service:

```go
func (s *CatalogService) IncrementAvailability(ctx context.Context, bookID string) error
func (s *CatalogService) DecrementAvailability(ctx context.Context, bookID string) error
```

These perform atomic `UPDATE books SET available_copies = available_copies + 1 WHERE id = ?` (or `- 1`) operations. The decrement includes a check `available_copies > 0` to prevent negative values.

These are internal-only methods used by the co-located consumer — not exposed via gRPC. When the consumer is eventually extracted to its own service, it would call the existing `UpdateAvailability` gRPC endpoint instead.

### Error Handling

If the consumer fails to process a message (e.g., catalog DB is down), it does not commit the offset. Kafka redelivers on the next poll. The increment/decrement operations are idempotent in the sense that reprocessing a create+return pair nets to zero. However, double-processing a single event would over-count. For this chapter, at-least-once delivery with the ordering guarantee (same partition per book) is sufficient. The tutorial should note that production systems would add deduplication (e.g., tracking processed event IDs).

## Gateway Changes

### New Routes

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| POST | /books/{id}/reserve | ReserveBook | Required |
| POST | /reservations/{id}/return | ReturnBook | Required |
| GET | /reservations | MyReservations | Required |

### New Helper

```go
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool
```

Like `requireAdmin` but only checks that a user is logged in (any role). Redirects to `/login` if anonymous.

### UI Changes

- **Book detail page** (`book.html`): Add a "Reserve" button, visible only when user is logged in and `available_copies > 0`.
- **New page** (`reservations.html`): Lists the user's reservations. Active reservations show a "Return" button and the due date. Returned/expired reservations show the status.
- **Nav partial** (`partials/nav.html`): Add "My Reservations" link, visible when logged in.

### Server Struct

The `Server` struct gains a fourth client field:

```go
type Server struct {
    auth        authv1.AuthServiceClient
    catalog     catalogv1.CatalogServiceClient
    reservation reservationv1.ReservationServiceClient
    tmpl        map[string]*template.Template
    baseTmpl    *template.Template
}
```

The `New()` constructor adds the reservation client parameter.

### gRPC Error Mapping

`handleGRPCError` already handles the relevant codes. Two additions:
- `codes.ResourceExhausted` → 429 with message "You have reached the maximum number of active reservations"
- `codes.FailedPrecondition` → 409 with message from gRPC status (e.g., "no copies available")

## Docker & Infrastructure

### New Containers

| Service | Image | Ports | Networks |
|---------|-------|-------|----------|
| kafka | `apache/kafka:3.9` | 9092:9092 | library-net |
| postgres-reservation | `postgres:16-alpine` | 5435:5432 | library-net |
| reservation | `services/reservation/Dockerfile` | 50053:50053 | library-net |

### Kafka Environment

```yaml
kafka:
  image: apache/kafka:3.9
  environment:
    KAFKA_NODE_ID: "1"
    KAFKA_PROCESS_ROLES: "broker,controller"
    KAFKA_LISTENERS: "PLAINTEXT://:9092,CONTROLLER://:9093"
    KAFKA_ADVERTISED_LISTENERS: "PLAINTEXT://kafka:9092"
    KAFKA_CONTROLLER_QUORUM_VOTERS: "1@kafka:9093"
    KAFKA_CONTROLLER_LISTENER_NAMES: "CONTROLLER"
    KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
    KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
  ports:
    - "9092:9092"
  networks:
    - library-net
```

### Updated Dependencies

- **reservation** depends on: `postgres-reservation` (healthy), `kafka`
- **catalog** gains dependency on: `kafka`
- **gateway** gains dependency on: `reservation`

### New Environment Variables

In `deploy/.env`:
```
KAFKA_BROKERS=kafka:9092
RESERVATION_GRPC_ADDR=reservation:50053
RESERVATION_GRPC_PORT=50053
MAX_ACTIVE_RESERVATIONS=5
POSTGRES_RESERVATION_PORT=5435
POSTGRES_RESERVATION_USER=postgres
POSTGRES_RESERVATION_PASSWORD=postgres
POSTGRES_RESERVATION_DB=reservation
```

### Dockerfiles

`services/reservation/Dockerfile` and `Dockerfile.dev` follow the same pattern as auth and catalog:
- Multi-stage build
- `GOWORK=off`
- Copy `gen/`, `pkg/auth/`
- Dependency caching via `go.mod` first

### Earthfile

Add reservation service to `+lint` and `+test` targets.

## Testing Strategy

### Reservation Service

- **Repository tests:** GORM operations against test database — create, find, count active, update status. Skipped with `testing.Short()`.
- **Service layer tests:** Mock repository, mock catalog gRPC client, mock `EventPublisher`. Test: max active limit (6th reservation fails), availability check (0 copies fails), expiration on read (past-due transitions), event publishing (mock captures events).
- **Handler tests:** Mock service layer, verify gRPC status codes for error cases.

### Catalog Consumer

- Mock the catalog service layer. Feed test JSON messages to the consumer handler. Verify `IncrementAvailability` / `DecrementAvailability` called with correct book IDs and correct direction per event type.

### Gateway

- Same `httptest` + mock gRPC client pattern. Mock reservation client. Test: reserve redirects correctly, return redirects correctly, `requireAuth` blocks anonymous users, error cases (resource exhausted, failed precondition) render appropriate messages.

### Not Tested in This Chapter

- End-to-end Kafka integration (real broker). Deferred to a future integration testing chapter.
- Consumer reconnection/retry behavior.

## Documentation

### Chapter Structure

| File | Section | Words |
|------|---------|-------|
| `index.md` | Chapter overview + architecture diagram | ~400 |
| `event-driven-architecture.md` | 6.1 — Kafka fundamentals, events vs commands, sarama | ~1500 |
| `reservation-service.md` | 6.2 — Building the service, state machine, expiration on read | ~1500 |
| `kafka-consumer.md` | 6.3 — Consumer goroutine, consumer groups, idempotency | ~1200 |
| `reservation-ui.md` | 6.4 — Gateway changes, eventual consistency in UI, Docker | ~1000 |

Each section includes code from the actual implementation, comparisons to Java/Spring equivalents (JMS, `@KafkaListener`, `@TransactionalEventListener`), and exercises.

## File Structure

### New Files

```
proto/reservation/v1/reservation.proto
gen/reservation/v1/                          (generated)

services/reservation/
  cmd/main.go
  go.mod
  go.sum
  .air.toml
  Dockerfile
  Dockerfile.dev
  internal/
    handler/handler.go
    handler/handler_test.go
    service/service.go
    service/service_test.go
    repository/repository.go
    repository/repository_test.go
    model/model.go
  migrations/
    000001_create_reservations.up.sql
    000001_create_reservations.down.sql

services/catalog/internal/consumer/
  consumer.go
  consumer_test.go

services/gateway/internal/handler/reservation.go
services/gateway/internal/handler/reservation_test.go
services/gateway/templates/reservations.html

docs/src/ch06/
  index.md
  event-driven-architecture.md
  reservation-service.md
  kafka-consumer.md
  reservation-ui.md
```

### Modified Files

```
go.work                                      (add services/reservation)
services/catalog/cmd/main.go                 (start consumer goroutine)
services/catalog/internal/service/service.go (add Increment/DecrementAvailability)
services/catalog/internal/service/service_test.go
services/gateway/internal/handler/server.go  (add reservation client)
services/gateway/internal/handler/render.go  (add ResourceExhausted/FailedPrecondition mappings)
services/gateway/templates/book.html         (add Reserve button)
services/gateway/templates/partials/nav.html (add My Reservations link)
services/gateway/cmd/main.go                 (add reservation gRPC client + routes)
deploy/docker-compose.yml                    (add kafka, postgres-reservation, reservation)
deploy/docker-compose.dev.yml                (add reservation dev override)
deploy/.env                                  (add new variables)
Earthfile                                    (add reservation to lint/test)
docs/src/SUMMARY.md                          (add Chapter 6 entries)
```
