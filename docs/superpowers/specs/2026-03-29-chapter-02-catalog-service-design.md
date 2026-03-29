# Chapter 2: Catalog Service — Design Spec

## Overview

Build the first standalone microservice: the Catalog service. It manages the book registry with CRUD operations, exposes a gRPC API, stores data in PostgreSQL via GORM, and uses versioned SQL migrations. No Kafka in this chapter — event publishing is added in Chapter 6.

## Goals

- Teach protobuf and gRPC fundamentals (service definitions, code generation with buf)
- Teach PostgreSQL interaction via GORM with the repository pattern
- Teach database migrations with golang-migrate
- Produce a working gRPC service that can be tested with grpcurl
- Continue building the tutorial content (Markdown + mdBook)

## Key Implementation Decisions

- **ORM:** GORM (most popular Go ORM, practical value in knowing it)
- **Migrations:** golang-migrate (versioned SQL files, not GORM AutoMigrate)
- **Protobuf toolchain:** buf (linting, breaking change detection, simpler than raw protoc)
- **Architecture:** Layered — handler → service → repository, with interfaces between layers
- **Kafka:** Not included in Chapter 2. Added retroactively in Chapter 6.

## Service Architecture

### Layers

**Handler layer** (`internal/handler/`):
- Implements the generated gRPC server interface (`catalogv1.CatalogServiceServer`)
- Validates incoming requests (required fields, formats)
- Converts protobuf types ↔ domain model types
- Translates service errors → gRPC status codes (`codes.NotFound`, `codes.InvalidArgument`, etc.)

**Service layer** (`internal/service/`):
- Contains business logic and validation rules
- Defines the `BookRepository` interface (dependency inversion)
- Enforces domain invariants (e.g., `available_copies <= total_copies`)
- Returns domain-specific errors

**Repository layer** (`internal/repository/`):
- Implements `BookRepository` using GORM
- Translates GORM errors → domain errors
- Handles pagination and filtering queries

**Model** (`internal/model/`):
- Domain `Book` struct with GORM tags
- Domain error types (`ErrBookNotFound`, `ErrDuplicateISBN`, etc.)

### Key Interfaces

```go
// Defined in internal/service/catalog.go
type BookRepository interface {
    Create(ctx context.Context, book *model.Book) (*model.Book, error)
    GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
    Update(ctx context.Context, book *model.Book) (*model.Book, error)
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, filter BookFilter, page Pagination) ([]*model.Book, int64, error)
    UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}
```

The service layer defines this interface. The repository layer implements it. The handler calls the service, which calls the repository. Dependencies flow inward — business logic doesn't know about GORM.

## Protobuf & gRPC API

### Proto file: `proto/catalog/v1/catalog.proto`

```protobuf
syntax = "proto3";
package catalog.v1;

import "google/protobuf/timestamp.proto";

service CatalogService {
  rpc CreateBook(CreateBookRequest) returns (Book);
  rpc GetBook(GetBookRequest) returns (Book);
  rpc UpdateBook(UpdateBookRequest) returns (Book);
  rpc DeleteBook(DeleteBookRequest) returns (DeleteBookResponse);
  rpc ListBooks(ListBooksRequest) returns (ListBooksResponse);
  rpc UpdateAvailability(UpdateAvailabilityRequest) returns (UpdateAvailabilityResponse);
}

message Book {
  string id = 1;
  string title = 2;
  string author = 3;
  string isbn = 4;
  string genre = 5;
  string description = 6;
  int32 published_year = 7;
  int32 total_copies = 8;
  int32 available_copies = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

message CreateBookRequest {
  string title = 1;
  string author = 2;
  string isbn = 3;
  string genre = 4;
  string description = 5;
  int32 published_year = 6;
  int32 total_copies = 7;
}

message GetBookRequest {
  string id = 1;
}

message UpdateBookRequest {
  string id = 1;
  string title = 2;
  string author = 3;
  string isbn = 4;
  string genre = 5;
  string description = 6;
  int32 published_year = 7;
  int32 total_copies = 8;
}

message DeleteBookRequest {
  string id = 1;
}

message DeleteBookResponse {}

message ListBooksRequest {
  string genre = 1;
  string author = 2;
  bool available_only = 3;
  int32 page = 4;
  int32 page_size = 5;
}

message ListBooksResponse {
  repeated Book books = 1;
  int32 total_count = 2;
}

message UpdateAvailabilityRequest {
  string id = 1;
  int32 delta = 2;
}

message UpdateAvailabilityResponse {
  int32 available_copies = 1;
}
```

### Buf configuration

**`proto/buf.yaml`** — module definition with standard linting rules.

**`proto/buf.gen.yaml`** — generates Go code using `protoc-gen-go` and `protoc-gen-go-grpc`, output to `gen/catalog/v1/`.

**`gen/`** — a separate Go module (`github.com/fesoliveira014/library-system/gen`) so services can import the generated types. Added to `go.work`.

## Database

### Migration: `000001_create_books.up.sql`

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE books (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title            VARCHAR(500) NOT NULL,
    author           VARCHAR(500) NOT NULL,
    isbn             VARCHAR(13) UNIQUE,
    genre            VARCHAR(100),
    description      TEXT,
    published_year   INTEGER,
    total_copies     INTEGER NOT NULL DEFAULT 1,
    available_copies INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT available_lte_total CHECK (available_copies <= total_copies),
    CONSTRAINT copies_non_negative CHECK (available_copies >= 0 AND total_copies >= 0)
);

CREATE INDEX idx_books_genre ON books(genre);
CREATE INDEX idx_books_author ON books(author);
```

### Migration: `000001_create_books.down.sql`

```sql
DROP TABLE IF EXISTS books;
```

### Key database decisions

- **UUID primary keys** generated by PostgreSQL (`uuid_generate_v4()`), not the application
- **CHECK constraints** enforce invariants at the DB level — defense in depth alongside service-layer validation
- **Indexes** on `genre` and `author` for filtered list queries
- **TIMESTAMPTZ** for timezone-aware timestamps
- **Migrations run on startup** via golang-migrate, embedded using Go's `embed` package
- **Schema managed by migrations, not GORM AutoMigrate** — GORM is used only for queries

### GORM model

```go
type Book struct {
    ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
    Title          string    `gorm:"type:varchar(500);not null"`
    Author         string    `gorm:"type:varchar(500);not null"`
    ISBN           string    `gorm:"type:varchar(13);uniqueIndex"`
    Genre          string    `gorm:"type:varchar(100)"`
    Description    string    `gorm:"type:text"`
    PublishedYear  int       `gorm:"type:integer"`
    TotalCopies    int       `gorm:"type:integer;not null;default:1"`
    AvailableCopies int      `gorm:"type:integer;not null;default:1"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

## File Structure

```
services/catalog/
├── cmd/
│   └── main.go                    # gRPC server startup, DI wiring
├── internal/
│   ├── handler/
│   │   ├── catalog.go             # gRPC handler implementation
│   │   └── catalog_test.go        # handler tests (mock service)
│   ├── service/
│   │   ├── catalog.go             # business logic + BookRepository interface
│   │   └── catalog_test.go        # service tests (mock repository)
│   ├── repository/
│   │   ├── book.go                # GORM repository implementation
│   │   └── book_test.go           # integration tests (real DB)
│   └── model/
│       ├── book.go                # domain Book struct
│       └── errors.go              # domain error types
├── migrations/
│   ├── 000001_create_books.up.sql
│   └── 000001_create_books.down.sql
├── Dockerfile
├── Earthfile
└── go.mod

proto/
├── buf.yaml
├── buf.gen.yaml
└── catalog/
    └── v1/
        └── catalog.proto

gen/
├── go.mod
└── catalog/
    └── v1/
        ├── catalog.pb.go          # generated
        └── catalog_grpc.pb.go     # generated
```

## Error Handling

### Domain errors (`internal/model/errors.go`)

```go
var (
    ErrBookNotFound  = errors.New("book not found")
    ErrDuplicateISBN = errors.New("duplicate ISBN")
    ErrInvalidBook   = errors.New("invalid book data")
)
```

### Error translation (handler layer)

| Domain error | gRPC status code |
|---|---|
| `ErrBookNotFound` | `codes.NotFound` |
| `ErrDuplicateISBN` | `codes.AlreadyExists` |
| `ErrInvalidBook` | `codes.InvalidArgument` |
| Unexpected error | `codes.Internal` |

## Testing Strategy

- **Handler tests:** Mock the service interface. Test protobuf conversion and error mapping.
- **Service tests:** Mock the repository interface. Test business logic and validation.
- **Repository tests:** Integration tests against a real PostgreSQL (Docker container via testcontainers-go or a test Docker Compose). Test GORM queries, pagination, filtering.

## Tutorial Chapter Outline

1. **2.1 Protocol Buffers & gRPC** — what protobuf is, why gRPC for internal services, writing `.proto` files, buf toolchain setup, code generation walkthrough
2. **2.2 PostgreSQL & Migrations** — local PostgreSQL via Docker, golang-migrate, writing up/down migrations, embedding in Go, why not AutoMigrate
3. **2.3 The Repository Pattern with GORM** — GORM basics (model tags, CRUD operations), implementing `BookRepository`, error translation, pagination and filtering
4. **2.4 Service Layer & Business Logic** — defining interfaces, business validation, the service as orchestrator, domain error types
5. **2.5 Wiring It All Together** — gRPC server setup in `main.go`, constructor-based dependency injection (no framework), handler implementation, testing with `grpcurl`

## Dependencies (Go modules)

- `google.golang.org/grpc` — gRPC framework
- `google.golang.org/protobuf` — protobuf runtime
- `gorm.io/gorm` — ORM
- `gorm.io/driver/postgres` — GORM PostgreSQL driver
- `github.com/golang-migrate/migrate/v4` — database migrations
- `github.com/google/uuid` — UUID type
- `github.com/grpc-ecosystem/go-grpc-middleware/v2` — gRPC interceptors (logging, recovery)

## Implementation Notes

- **Partial updates:** Proto3 scalar fields default to zero values, making it impossible to distinguish "clear this field" from "field not sent." The tutorial should acknowledge this limitation and explain that `google.protobuf.FieldMask` is the production solution, but we skip it for simplicity.
- **UpdateAvailability RPC:** This endpoint exists in anticipation of Chapter 6 (Kafka integration), where the Reservation service will trigger availability changes. The tutorial should note this forward reference so readers understand why it exists before Kafka is introduced.
- **go.work entries:** The root `go.work` must be updated to include both `./services/catalog` and `./gen` so local cross-module imports resolve correctly.

## What This Chapter Does NOT Include

- Kafka integration (Chapter 6)
- Docker/Docker Compose setup (Chapter 3)
- Authentication/authorization (Chapter 4)
- Observability/tracing (Chapter 8)
- Comprehensive testing patterns (Chapter 9 — this chapter includes basic tests only)
