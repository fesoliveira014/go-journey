# Chapter 2: Catalog Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Catalog microservice with a gRPC API, PostgreSQL database via GORM, versioned migrations, and produce the Chapter 2 tutorial content.

**Architecture:** Layered service (handler → service → repository) with interfaces between layers. Proto definitions compiled with buf. Migrations managed by golang-migrate. No Kafka — that comes in Chapter 6.

**Tech Stack:** Go 1.26+, gRPC, Protocol Buffers (buf), GORM, PostgreSQL, golang-migrate, grpcurl

**Spec reference:** `docs/superpowers/specs/2026-03-29-chapter-02-catalog-service-design.md`

**Scope note:** This plan covers Chapter 2 only. A local PostgreSQL instance (Docker) is required for integration tests but the full Docker Compose setup is Chapter 3.

---

## File Structure

```
proto/
├── buf.yaml                              # buf module config
├── buf.gen.yaml                          # code generation config
└── catalog/
    └── v1/
        └── catalog.proto                 # gRPC service definition

gen/
├── go.mod                                # separate module for generated code
└── catalog/
    └── v1/
        ├── catalog.pb.go                 # generated protobuf types
        └── catalog_grpc.pb.go            # generated gRPC server/client

services/catalog/
├── go.mod
├── Earthfile
├── cmd/
│   └── main.go                           # gRPC server startup, DI wiring
├── internal/
│   ├── model/
│   │   ├── book.go                       # domain Book struct (GORM model)
│   │   └── errors.go                     # domain error types
│   ├── repository/
│   │   ├── book.go                       # GORM BookRepository implementation
│   │   └── book_test.go                  # integration tests (real PostgreSQL)
│   ├── service/
│   │   ├── catalog.go                    # business logic + BookRepository interface
│   │   └── catalog_test.go              # unit tests (mock repository)
│   └── handler/
│       ├── catalog.go                    # gRPC handler implementation
│       └── catalog_test.go              # unit tests (mock service)
├── migrations/
│   ├── embed.go                          # Go embed directive for migration files
│   ├── 000001_create_books.up.sql
│   └── 000001_create_books.down.sql
└── Dockerfile

docs/src/
├── SUMMARY.md                            # update: add Chapter 2 entries
└── ch02/
    ├── index.md                          # Chapter 2 overview
    ├── protobuf-grpc.md                  # 2.1
    ├── postgresql-migrations.md          # 2.2
    ├── repository-pattern.md             # 2.3
    ├── service-layer.md                  # 2.4
    └── wiring.md                         # 2.5

go.work                                   # update: add ./services/catalog and ./gen
```

---

## Task 1: Protobuf Definition and buf Setup

**Files:**
- Create: `proto/buf.yaml`
- Create: `proto/buf.gen.yaml`
- Create: `proto/catalog/v1/catalog.proto`
- Create: `gen/go.mod`
- Modify: `go.work`

- [ ] **Step 1: Install buf** (if not already installed)

```bash
# Linux
curl -sSL https://github.com/bufbuild/buf/releases/download/v1.30.0/buf-Linux-x86_64 -o /usr/local/bin/buf
chmod +x /usr/local/bin/buf
```

Verify: `buf --version`

- [ ] **Step 2: Create proto directory structure**

```bash
mkdir -p proto/catalog/v1
```

- [ ] **Step 3: Create buf module config**

Create `proto/buf.yaml`:

```yaml
version: v2
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

- [ ] **Step 4: Create buf code generation config**

Create `proto/buf.gen.yaml`:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: ../gen
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: ../gen
    opt: paths=source_relative
```

- [ ] **Step 5: Write the catalog proto file**

Create `proto/catalog/v1/catalog.proto`:

```protobuf
syntax = "proto3";

package catalog.v1;

option go_package = "github.com/fesoliveira014/library-system/gen/catalog/v1;catalogv1";

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

- [ ] **Step 6: Lint the proto file**

```bash
cd proto && buf lint
```

Expected: no errors.

- [ ] **Step 7: Generate Go code**

```bash
cd proto && buf generate
```

Expected: creates `gen/catalog/v1/catalog.pb.go` and `gen/catalog/v1/catalog_grpc.pb.go`.

- [ ] **Step 8: Initialize the gen Go module**

```bash
cd gen
go mod init github.com/fesoliveira014/library-system/gen
```

Then add the required dependencies:

```bash
go mod tidy
```

- [ ] **Step 9: Update go.work to include gen**

Add `./gen` to the `use` block in `go.work`:

```go
go 1.26.1

use (
    ./services/gateway
    ./services/catalog
    ./gen
)
```

Note: `./services/catalog` is also added here (will be created in Task 2).

- [ ] **Step 10: Commit**

```bash
git add proto/ gen/ go.work
git commit -m "feat: add protobuf definitions and buf setup for catalog service"
```

---

## Task 2: Catalog Service Scaffold and Domain Model

**Files:**
- Create: `services/catalog/go.mod`
- Create: `services/catalog/internal/model/book.go`
- Create: `services/catalog/internal/model/errors.go`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p services/catalog/cmd
mkdir -p services/catalog/internal/{model,repository,service,handler}
mkdir -p services/catalog/migrations
```

- [ ] **Step 2: Initialize the catalog Go module**

```bash
cd services/catalog
go mod init github.com/fesoliveira014/library-system/services/catalog
```

- [ ] **Step 3: Add dependencies**

```bash
cd services/catalog
go get gorm.io/gorm
go get gorm.io/driver/postgres
go get github.com/google/uuid
go get github.com/golang-migrate/migrate/v4
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/fesoliveira014/library-system/gen
```

Run `go mod tidy` afterward.

- [ ] **Step 4: Create the domain model**

Create `services/catalog/internal/model/book.go`:

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

// Book is the domain model for a book in the catalog.
// GORM uses these struct tags to map to the PostgreSQL table.
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

// BookFilter holds optional filter parameters for listing books.
type BookFilter struct {
	Genre         string
	Author        string
	AvailableOnly bool
}

// Pagination holds pagination parameters.
type Pagination struct {
	Page     int
	PageSize int
}

// DefaultPageSize is used when no page size is specified.
const DefaultPageSize = 20

// MaxPageSize prevents excessively large queries.
const MaxPageSize = 100
```

- [ ] **Step 5: Create domain error types**

Create `services/catalog/internal/model/errors.go`:

```go
package model

import "errors"

var (
	ErrBookNotFound  = errors.New("book not found")
	ErrDuplicateISBN = errors.New("duplicate ISBN")
	ErrInvalidBook   = errors.New("invalid book data")
)
```

- [ ] **Step 6: Verify module compiles**

```bash
cd services/catalog && go build ./...
```

Expected: no errors.

- [ ] **Step 7: Run go work sync from project root**

```bash
go work sync
```

- [ ] **Step 8: Commit**

```bash
git add services/catalog/go.mod services/catalog/go.sum services/catalog/internal/model/ go.work
git commit -m "feat(catalog): add domain model and error types"
```

---

## Task 3: Database Migrations

**Files:**
- Create: `services/catalog/migrations/000001_create_books.up.sql`
- Create: `services/catalog/migrations/000001_create_books.down.sql`
- Create: `services/catalog/migrations/embed.go`

- [ ] **Step 1: Write the up migration**

Create `services/catalog/migrations/000001_create_books.up.sql`:

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

- [ ] **Step 2: Write the down migration**

Create `services/catalog/migrations/000001_create_books.down.sql`:

```sql
DROP TABLE IF EXISTS books;
```

- [ ] **Step 3: Create the embed file**

Create `services/catalog/migrations/embed.go`:

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 4: Verify compilation**

```bash
cd services/catalog && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add services/catalog/migrations/
git commit -m "feat(catalog): add database migration for books table"
```

---

## Task 4: Repository Layer (GORM)

**Files:**
- Create: `services/catalog/internal/repository/book.go`
- Create: `services/catalog/internal/repository/book_test.go`

- [ ] **Step 1: Write the GORM repository implementation**

Create `services/catalog/internal/repository/book.go`:

```go
package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// BookRepository implements the service.BookRepository interface using GORM.
type BookRepository struct {
	db *gorm.DB
}

// NewBookRepository creates a new GORM-backed book repository.
func NewBookRepository(db *gorm.DB) *BookRepository {
	return &BookRepository{db: db}
}

func (r *BookRepository) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
	if err := r.db.WithContext(ctx).Create(book).Error; err != nil {
		if isDuplicateKeyError(err) {
			return nil, model.ErrDuplicateISBN
		}
		return nil, err
	}
	return book, nil
}

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

func (r *BookRepository) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
	result := r.db.WithContext(ctx).Model(book).Updates(map[string]interface{}{
		"title":          book.Title,
		"author":         book.Author,
		"isbn":           book.ISBN,
		"genre":          book.Genre,
		"description":    book.Description,
		"published_year": book.PublishedYear,
		"total_copies":   book.TotalCopies,
	})
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return nil, model.ErrDuplicateISBN
		}
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, model.ErrBookNotFound
	}
	// Reload to get updated_at
	return r.GetByID(ctx, book.ID)
}

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

	pageSize := page.PageSize
	if pageSize <= 0 {
		pageSize = model.DefaultPageSize
	}
	if pageSize > model.MaxPageSize {
		pageSize = model.MaxPageSize
	}
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

func (r *BookRepository) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	result := r.db.WithContext(ctx).
		Model(&model.Book{}).
		Where("id = ?", id).
		Update("available_copies", gorm.Expr("available_copies + ?", delta))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.ErrBookNotFound
	}
	return nil
}

// isDuplicateKeyError checks if a PostgreSQL error is a unique constraint violation.
// The GORM postgres driver wraps pgx errors — check the error message for the
// PostgreSQL unique_violation SQLSTATE (23505) or the "duplicate key" text.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
```

- [ ] **Step 2: Write repository integration tests**

Create `services/catalog/internal/repository/book_test.go`:

```go
package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// testDB returns a GORM connection to a test PostgreSQL database.
// Set TEST_DATABASE_URL env var or it defaults to localhost.
// These are integration tests — they require a running PostgreSQL.
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

	// Run the real migrations (same as production) so CHECK constraints and
	// indexes exist in the test database.
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

	// Clean table before each test
	db.Exec("TRUNCATE TABLE books CASCADE")

	return db
}

func newTestBook(suffix string) *model.Book {
	return &model.Book{
		Title:           fmt.Sprintf("Test Book %s", suffix),
		Author:          fmt.Sprintf("Author %s", suffix),
		ISBN:            fmt.Sprintf("978000000%s", suffix[:4]),
		Genre:           "Testing",
		Description:     "A test book",
		PublishedYear:   2024,
		TotalCopies:     3,
		AvailableCopies: 3,
	}
}

func TestBookRepository_Create(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected UUID to be set")
	}
	if created.Title != book.Title {
		t.Errorf("expected title %q, got %q", book.Title, created.Title)
	}
}

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

func TestBookRepository_GetByID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0004")
	created, _ := repo.Create(ctx, book)

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Title != created.Title {
		t.Errorf("expected title %q, got %q", created.Title, found.Title)
	}
}

func TestBookRepository_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestBookRepository_Update(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0005")
	created, _ := repo.Create(ctx, book)

	created.Title = "Updated Title"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("expected title %q, got %q", "Updated Title", updated.Title)
	}
}

func TestBookRepository_Delete(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0006")
	created, _ := repo.Create(ctx, book)

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := repo.GetByID(ctx, created.ID)
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound after delete, got %v", err)
	}
}

func TestBookRepository_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestBookRepository_List(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	// Create books with different genres
	for i := 0; i < 3; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 100+i))
		b.Genre = "Fiction"
		repo.Create(ctx, b)
	}
	for i := 0; i < 2; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 200+i))
		b.Genre = "Science"
		repo.Create(ctx, b)
	}

	// List all
	books, total, err := repo.List(ctx, model.BookFilter{}, model.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(books) != 5 {
		t.Errorf("expected 5 books, got %d", len(books))
	}

	// Filter by genre
	books, total, err = repo.List(ctx, model.BookFilter{Genre: "Fiction"}, model.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3 for Fiction, got %d", total)
	}
}

func TestBookRepository_UpdateAvailability(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0007")
	book.TotalCopies = 5
	book.AvailableCopies = 5
	created, _ := repo.Create(ctx, book)

	// Decrement
	if err := repo.UpdateAvailability(ctx, created.ID, -1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	found, _ := repo.GetByID(ctx, created.ID)
	if found.AvailableCopies != 4 {
		t.Errorf("expected 4 available copies, got %d", found.AvailableCopies)
	}
}
```

- [ ] **Step 3: Run tests** (requires PostgreSQL)

```bash
# Start a test PostgreSQL if not running:
docker run -d --name catalog-test-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=catalog_test -p 5432:5432 postgres:16-alpine

# Wait for it to be ready:
sleep 3

# Run tests:
cd services/catalog && go test -v ./internal/repository/... -count=1
```

Expected: all tests pass. If PostgreSQL is not available, tests are skipped.

- [ ] **Step 4: Commit**

```bash
git add services/catalog/internal/repository/
git commit -m "feat(catalog): add GORM book repository with integration tests"
```

---

## Task 5: Service Layer

**Files:**
- Create: `services/catalog/internal/service/catalog.go`
- Create: `services/catalog/internal/service/catalog_test.go`

- [ ] **Step 1: Write the service layer with interface definition**

Create `services/catalog/internal/service/catalog.go`:

```go
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// BookRepository defines the interface for book persistence.
// Implemented by repository.BookRepository (GORM).
type BookRepository interface {
	Create(ctx context.Context, book *model.Book) (*model.Book, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
	Update(ctx context.Context, book *model.Book) (*model.Book, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error)
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}

// CatalogService implements catalog business logic.
type CatalogService struct {
	repo BookRepository
}

// NewCatalogService creates a new CatalogService.
func NewCatalogService(repo BookRepository) *CatalogService {
	return &CatalogService{repo: repo}
}

func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if err := validateBook(book); err != nil {
		return nil, err
	}
	if book.AvailableCopies == 0 {
		book.AvailableCopies = book.TotalCopies
	}
	return s.repo.Create(ctx, book)
}

func (s *CatalogService) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CatalogService) UpdateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if book.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing book ID", model.ErrInvalidBook)
	}
	// Fetch existing to check invariants
	existing, err := s.repo.GetByID(ctx, book.ID)
	if err != nil {
		return nil, err
	}
	// If total_copies changed, adjust available_copies proportionally
	if book.TotalCopies > 0 && book.TotalCopies != existing.TotalCopies {
		diff := book.TotalCopies - existing.TotalCopies
		book.AvailableCopies = existing.AvailableCopies + diff
		if book.AvailableCopies < 0 {
			book.AvailableCopies = 0
		}
	} else {
		book.AvailableCopies = existing.AvailableCopies
	}
	return s.repo.Update(ctx, book)
}

func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *CatalogService) ListBooks(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	return s.repo.List(ctx, filter, page)
}

func (s *CatalogService) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	return s.repo.UpdateAvailability(ctx, id, delta)
}

func validateBook(book *model.Book) error {
	if book.Title == "" {
		return fmt.Errorf("%w: title is required", model.ErrInvalidBook)
	}
	if book.Author == "" {
		return fmt.Errorf("%w: author is required", model.ErrInvalidBook)
	}
	if book.TotalCopies < 0 {
		return fmt.Errorf("%w: total copies cannot be negative", model.ErrInvalidBook)
	}
	return nil
}
```

- [ ] **Step 2: Write service unit tests with mock repository**

Create `services/catalog/internal/service/catalog_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

// mockRepo implements service.BookRepository for testing.
type mockRepo struct {
	books map[uuid.UUID]*model.Book
}

func newMockRepo() *mockRepo {
	return &mockRepo{books: make(map[uuid.UUID]*model.Book)}
}

func (m *mockRepo) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
	book.ID = uuid.New()
	// Check for duplicate ISBN
	for _, b := range m.books {
		if b.ISBN == book.ISBN && book.ISBN != "" {
			return nil, model.ErrDuplicateISBN
		}
	}
	m.books[book.ID] = book
	return book, nil
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	book, ok := m.books[id]
	if !ok {
		return nil, model.ErrBookNotFound
	}
	return book, nil
}

func (m *mockRepo) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
	if _, ok := m.books[book.ID]; !ok {
		return nil, model.ErrBookNotFound
	}
	m.books[book.ID] = book
	return book, nil
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.books[id]; !ok {
		return model.ErrBookNotFound
	}
	delete(m.books, id)
	return nil
}

func (m *mockRepo) List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	var result []*model.Book
	for _, b := range m.books {
		result = append(result, b)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepo) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	book, ok := m.books[id]
	if !ok {
		return model.ErrBookNotFound
	}
	book.AvailableCopies += delta
	return nil
}

func TestCatalogService_CreateBook(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	book := &model.Book{Title: "Test", Author: "Author", TotalCopies: 3}
	created, err := svc.CreateBook(ctx, book)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}
	if created.AvailableCopies != 3 {
		t.Errorf("expected available_copies = total_copies = 3, got %d", created.AvailableCopies)
	}
}

func TestCatalogService_CreateBook_ValidationError(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	// Missing title
	_, err := svc.CreateBook(ctx, &model.Book{Author: "Author"})
	if !errors.Is(err, model.ErrInvalidBook) {
		t.Errorf("expected ErrInvalidBook, got %v", err)
	}

	// Missing author
	_, err = svc.CreateBook(ctx, &model.Book{Title: "Title"})
	if !errors.Is(err, model.ErrInvalidBook) {
		t.Errorf("expected ErrInvalidBook, got %v", err)
	}
}

func TestCatalogService_UpdateBook_AdjustsAvailability(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	book := &model.Book{Title: "Test", Author: "Author", TotalCopies: 5}
	created, _ := svc.CreateBook(ctx, book)

	// Reduce total copies by 2 — available should also drop by 2
	update := &model.Book{
		ID:          created.ID,
		Title:       "Test",
		Author:      "Author",
		TotalCopies: 3,
	}
	updated, err := svc.UpdateBook(ctx, update)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.AvailableCopies != 3 {
		t.Errorf("expected available 3 (was 5, reduced by 2), got %d", updated.AvailableCopies)
	}
}

func TestCatalogService_GetBook_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	_, err := svc.GetBook(ctx, uuid.New())
	if !errors.Is(err, model.ErrBookNotFound) {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestCatalogService_DeleteBook(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	book := &model.Book{Title: "Test", Author: "Author", TotalCopies: 1}
	created, _ := svc.CreateBook(ctx, book)

	if err := svc.DeleteBook(ctx, created.ID); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := svc.GetBook(ctx, created.ID)
	if !errors.Is(err, model.ErrBookNotFound) {
		t.Errorf("expected ErrBookNotFound after delete, got %v", err)
	}
}
```

- [ ] **Step 3: Run service tests**

```bash
cd services/catalog && go test -v ./internal/service/... -count=1
```

Expected: all tests pass (no database needed — uses mock).

- [ ] **Step 4: Commit**

```bash
git add services/catalog/internal/service/
git commit -m "feat(catalog): add service layer with business logic and unit tests"
```

---

## Task 6: gRPC Handler Layer

**Files:**
- Create: `services/catalog/internal/handler/catalog.go`
- Create: `services/catalog/internal/handler/catalog_test.go`

- [ ] **Step 1: Write the gRPC handler**

Create `services/catalog/internal/handler/catalog.go`:

```go
package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

// CatalogHandler implements the catalogv1.CatalogServiceServer gRPC interface.
type CatalogHandler struct {
	catalogv1.UnimplementedCatalogServiceServer
	svc *service.CatalogService
}

// NewCatalogHandler creates a new gRPC handler.
func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{svc: svc}
}

func (h *CatalogHandler) CreateBook(ctx context.Context, req *catalogv1.CreateBookRequest) (*catalogv1.Book, error) {
	if req.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if req.GetAuthor() == "" {
		return nil, status.Error(codes.InvalidArgument, "author is required")
	}

	book := &model.Book{
		Title:         req.GetTitle(),
		Author:        req.GetAuthor(),
		ISBN:          req.GetIsbn(),
		Genre:         req.GetGenre(),
		Description:   req.GetDescription(),
		PublishedYear: int(req.GetPublishedYear()),
		TotalCopies:   int(req.GetTotalCopies()),
	}

	created, err := h.svc.CreateBook(ctx, book)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoBook(created), nil
}

func (h *CatalogHandler) GetBook(ctx context.Context, req *catalogv1.GetBookRequest) (*catalogv1.Book, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book, err := h.svc.GetBook(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoBook(book), nil
}

func (h *CatalogHandler) UpdateBook(ctx context.Context, req *catalogv1.UpdateBookRequest) (*catalogv1.Book, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book := &model.Book{
		ID:            id,
		Title:         req.GetTitle(),
		Author:        req.GetAuthor(),
		ISBN:          req.GetIsbn(),
		Genre:         req.GetGenre(),
		Description:   req.GetDescription(),
		PublishedYear: int(req.GetPublishedYear()),
		TotalCopies:   int(req.GetTotalCopies()),
	}

	updated, err := h.svc.UpdateBook(ctx, book)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoBook(updated), nil
}

func (h *CatalogHandler) DeleteBook(ctx context.Context, req *catalogv1.DeleteBookRequest) (*catalogv1.DeleteBookResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	if err := h.svc.DeleteBook(ctx, id); err != nil {
		return nil, toGRPCError(err)
	}
	return &catalogv1.DeleteBookResponse{}, nil
}

func (h *CatalogHandler) ListBooks(ctx context.Context, req *catalogv1.ListBooksRequest) (*catalogv1.ListBooksResponse, error) {
	filter := model.BookFilter{
		Genre:         req.GetGenre(),
		Author:        req.GetAuthor(),
		AvailableOnly: req.GetAvailableOnly(),
	}
	page := model.Pagination{
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	}

	books, total, err := h.svc.ListBooks(ctx, filter, page)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoBooks := make([]*catalogv1.Book, len(books))
	for i, b := range books {
		protoBooks[i] = toProtoBook(b)
	}

	return &catalogv1.ListBooksResponse{
		Books:      protoBooks,
		TotalCount: int32(total),
	}, nil
}

func (h *CatalogHandler) UpdateAvailability(ctx context.Context, req *catalogv1.UpdateAvailabilityRequest) (*catalogv1.UpdateAvailabilityResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	if err := h.svc.UpdateAvailability(ctx, id, int(req.GetDelta())); err != nil {
		return nil, toGRPCError(err)
	}

	// Fetch updated book to return current count
	book, err := h.svc.GetBook(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &catalogv1.UpdateAvailabilityResponse{
		AvailableCopies: int32(book.AvailableCopies),
	}, nil
}

// toProtoBook converts a domain Book to a protobuf Book message.
func toProtoBook(b *model.Book) *catalogv1.Book {
	return &catalogv1.Book{
		Id:              b.ID.String(),
		Title:           b.Title,
		Author:          b.Author,
		Isbn:            b.ISBN,
		Genre:           b.Genre,
		Description:     b.Description,
		PublishedYear:   int32(b.PublishedYear),
		TotalCopies:     int32(b.TotalCopies),
		AvailableCopies: int32(b.AvailableCopies),
		CreatedAt:       timestamppb.New(b.CreatedAt),
		UpdatedAt:       timestamppb.New(b.UpdatedAt),
	}
}

// toGRPCError translates domain errors to gRPC status errors.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrBookNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrDuplicateISBN):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, model.ErrInvalidBook):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
```

- [ ] **Step 2: Write handler unit tests**

Create `services/catalog/internal/handler/catalog_test.go`:

```go
package handler_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

// inMemoryRepo is a minimal in-memory implementation of service.BookRepository
// for handler tests. We test protobuf conversion and error mapping here —
// business logic is tested in service_test.go.
type inMemoryRepo struct {
	books map[uuid.UUID]*model.Book
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{books: make(map[uuid.UUID]*model.Book)}
}

func (r *inMemoryRepo) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
	book.ID = uuid.New()
	r.books[book.ID] = book
	return book, nil
}

func (r *inMemoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	b, ok := r.books[id]
	if !ok {
		return nil, model.ErrBookNotFound
	}
	return b, nil
}

func (r *inMemoryRepo) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
	if _, ok := r.books[book.ID]; !ok {
		return nil, model.ErrBookNotFound
	}
	r.books[book.ID] = book
	return book, nil
}

func (r *inMemoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.books[id]; !ok {
		return model.ErrBookNotFound
	}
	delete(r.books, id)
	return nil
}

func (r *inMemoryRepo) List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	var result []*model.Book
	for _, b := range r.books {
		result = append(result, b)
	}
	return result, int64(len(result)), nil
}

func (r *inMemoryRepo) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	b, ok := r.books[id]
	if !ok {
		return model.ErrBookNotFound
	}
	b.AvailableCopies += delta
	return nil
}

func TestCatalogHandler_CreateBook_Success(t *testing.T) {
	svc := service.NewCatalogService(newInMemoryRepo())
	h := handler.NewCatalogHandler(svc)

	resp, err := h.CreateBook(context.Background(), &catalogv1.CreateBookRequest{
		Title:       "Test Book",
		Author:      "Test Author",
		TotalCopies: 3,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.GetTitle() != "Test Book" {
		t.Errorf("expected title %q, got %q", "Test Book", resp.GetTitle())
	}
	if resp.GetAvailableCopies() != 3 {
		t.Errorf("expected 3 available copies, got %d", resp.GetAvailableCopies())
	}
}

func TestCatalogHandler_CreateBook_MissingTitle(t *testing.T) {
	svc := service.NewCatalogService(newInMemoryRepo())
	h := handler.NewCatalogHandler(svc)

	_, err := h.CreateBook(context.Background(), &catalogv1.CreateBookRequest{
		Author: "Author",
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestCatalogHandler_GetBook_NotFound(t *testing.T) {
	svc := service.NewCatalogService(newInMemoryRepo())
	h := handler.NewCatalogHandler(svc)

	_, err := h.GetBook(context.Background(), &catalogv1.GetBookRequest{
		Id: "00000000-0000-0000-0000-000000000001",
	})
	if err == nil {
		t.Fatal("expected error for non-existent book")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestCatalogHandler_GetBook_InvalidID(t *testing.T) {
	svc := service.NewCatalogService(newInMemoryRepo())
	h := handler.NewCatalogHandler(svc)

	_, err := h.GetBook(context.Background(), &catalogv1.GetBookRequest{
		Id: "not-a-uuid",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for bad UUID, got %v", st.Code())
	}
}
```

- [ ] **Step 3: Run handler tests**

```bash
cd services/catalog && go test -v ./internal/handler/... -count=1
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add services/catalog/internal/handler/
git commit -m "feat(catalog): add gRPC handler with protobuf conversion and error mapping"
```

---

## Task 7: Server Wiring (main.go)

**Files:**
- Create: `services/catalog/cmd/main.go`

- [ ] **Step 1: Write the main.go with dependency injection**

Create `services/catalog/cmd/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

func main() {
	// Configuration from environment
	port := envOrDefault("GRPC_PORT", "50052")
	dsn := envOrDefault("DATABASE_URL", "host=localhost port=5432 user=postgres password=postgres dbname=catalog sslmode=disable")

	// Database connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to PostgreSQL")

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations complete")

	// Dependency injection — constructor-based, no framework
	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo)
	hdlr := handler.NewCatalogHandler(svc)

	// gRPC server
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, hdlr)
	reflection.Register(grpcServer) // enables grpcurl discovery

	addr := fmt.Sprintf(":%s", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("catalog service listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd services/catalog && go build ./cmd/...
```

Expected: compiles with no errors. Run `go mod tidy` first if needed.

- [ ] **Step 3: Test manually** (requires PostgreSQL running)

```bash
# Ensure PostgreSQL is running (from Task 4)
cd services/catalog && go run ./cmd/

# In another terminal, test with grpcurl:
grpcurl -plaintext localhost:50052 list
# Expected: catalog.v1.CatalogService

grpcurl -plaintext -d '{"title":"The Go Programming Language","author":"Alan Donovan","isbn":"9780134190440","genre":"Programming","total_copies":5}' localhost:50052 catalog.v1.CatalogService/CreateBook

grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks
```

- [ ] **Step 4: Commit**

```bash
git add services/catalog/cmd/
git commit -m "feat(catalog): add gRPC server with migration runner and DI wiring"
```

---

## Task 8: Catalog Service Earthfile and Dockerfile

**Files:**
- Create: `services/catalog/Earthfile`
- Create: `services/catalog/Dockerfile`
- Modify: `Earthfile` (root)

- [ ] **Step 1: Create service Earthfile**

Create `services/catalog/Earthfile`:

```earthly
VERSION 0.8

FROM golang:1.26-alpine

WORKDIR /app

deps:
    COPY go.mod go.sum* ./
    RUN go mod download

src:
    FROM +deps
    COPY --dir cmd internal migrations ./

lint:
    FROM +src
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...

test:
    FROM +src
    RUN go test -v -race -cover ./internal/service/... ./internal/handler/...

build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/
    SAVE ARTIFACT /bin/catalog

docker:
    FROM alpine:3.19
    COPY +build/catalog /usr/local/bin/catalog
    EXPOSE 50052
    ENTRYPOINT ["/usr/local/bin/catalog"]
    SAVE IMAGE catalog:latest
```

Note: The `test` target only runs unit tests (service + handler). Integration tests (repository) require PostgreSQL and run separately.

- [ ] **Step 2: Create Dockerfile**

Create `services/catalog/Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/

# Runtime stage
FROM alpine:3.19
COPY --from=builder /bin/catalog /usr/local/bin/catalog
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/catalog"]
```

- [ ] **Step 3: Update root Earthfile**

Add catalog targets to the root `Earthfile`:

```earthly
VERSION 0.8

ci:
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
    BUILD ./services/catalog+lint
    BUILD ./services/catalog+test

lint:
    BUILD ./services/gateway+lint
    BUILD ./services/catalog+lint

test:
    BUILD ./services/gateway+test
    BUILD ./services/catalog+test
```

- [ ] **Step 4: Commit**

```bash
git add services/catalog/Earthfile services/catalog/Dockerfile Earthfile
git commit -m "feat(catalog): add Earthfile, Dockerfile, and update root CI targets"
```

---

## Task 9: Update mdBook Structure for Chapter 2

**Files:**
- Modify: `docs/src/SUMMARY.md`
- Create: `docs/src/ch02/index.md`

- [ ] **Step 1: Update SUMMARY.md**

Add Chapter 2 entries to `docs/src/SUMMARY.md`:

```markdown
# Summary

- [Introduction](./introduction.md)
- [Chapter 1: Go Foundations](./ch01/index.md)
    - [1.1 Project Setup](./ch01/project-setup.md)
    - [1.2 Go Language Essentials](./ch01/go-basics.md)
    - [1.3 Building an HTTP Server](./ch01/http-server.md)
    - [1.4 Testing in Go](./ch01/testing.md)
- [Chapter 2: First Microservice — Catalog](./ch02/index.md)
    - [2.1 Protocol Buffers & gRPC](./ch02/protobuf-grpc.md)
    - [2.2 PostgreSQL & Migrations](./ch02/postgresql-migrations.md)
    - [2.3 The Repository Pattern with GORM](./ch02/repository-pattern.md)
    - [2.4 Service Layer & Business Logic](./ch02/service-layer.md)
    - [2.5 Wiring It All Together](./ch02/wiring.md)
```

- [ ] **Step 2: Create Chapter 2 index page**

Create `docs/src/ch02/index.md`:

```markdown
# Chapter 2: First Microservice — Catalog

In this chapter, you will build the Catalog microservice from scratch. This is the first real microservice in our library system — it manages the book registry and exposes a gRPC API.

By the end of this chapter, you will have:

- A Protocol Buffers service definition compiled with buf
- A PostgreSQL database with versioned migrations
- A layered Go service (handler → service → repository)
- A running gRPC server you can test with grpcurl
- Unit tests for business logic and handler layers
- Integration tests for the database layer

## Prerequisites

Everything from Chapter 1, plus:

- **Docker** — for running PostgreSQL locally ([docs.docker.com](https://docs.docker.com/get-docker/))
- **buf** — Protocol Buffers toolchain ([buf.build](https://buf.build/docs/installation))
- **grpcurl** — command-line gRPC client ([github.com/fullstorydev/grpcurl](https://github.com/fullstorydev/grpcurl))

## Sections

1. [Protocol Buffers & gRPC](./protobuf-grpc.md) — service definitions, code generation, why gRPC
2. [PostgreSQL & Migrations](./postgresql-migrations.md) — Docker setup, golang-migrate, versioned schemas
3. [The Repository Pattern with GORM](./repository-pattern.md) — ORM basics, implementing the data layer
4. [Service Layer & Business Logic](./service-layer.md) — interfaces, validation, domain errors
5. [Wiring It All Together](./wiring.md) — gRPC server, dependency injection, testing with grpcurl
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/SUMMARY.md docs/src/ch02/
git commit -m "feat(docs): add Chapter 2 structure to mdBook"
```

---

## Task 10: Write Tutorial Section 2.1 — Protocol Buffers & gRPC

**Files:**
- Create: `docs/src/ch02/protobuf-grpc.md`

- [ ] **Step 1: Write section 2.1**

This section covers:

- **What is Protocol Buffers** — binary serialization format, `.proto` files as IDL, comparison to JSON (smaller, faster, typed, but not human-readable). Compare to Java's Serializable / Kotlin's kotlinx.serialization.
- **Why gRPC for internal services** — HTTP/2 multiplexing, bidirectional streaming, code generation for client/server, strong typing. REST is for external APIs, gRPC for service-to-service.
- **Writing a .proto file** — walk through the catalog.proto line by line: syntax, package, go_package option, service definition, message types, field numbers, repeated fields, imports.
- **The buf toolchain** — what problems buf solves vs raw protoc (dependency management, linting, breaking change detection). Walk through buf.yaml, buf.gen.yaml, running `buf lint` and `buf generate`.
- **Generated code walkthrough** — what `catalog.pb.go` and `catalog_grpc.pb.go` contain, the server interface you must implement, the client stub you'll use later.
- **Exercise:** Add a `SearchBooks` RPC to the proto file that takes a `query` string and returns a `ListBooksResponse`. Run `buf lint` to check it, then `buf generate`. (This is a preview of the Search service — don't implement it, just define the proto.)

Include footnoted references:
1. [Protocol Buffers Language Guide](https://protobuf.dev/programming-guides/proto3/)
2. [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
3. [buf documentation](https://buf.build/docs/)

Content length target: 1200-1600 words.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch02/protobuf-grpc.md
git commit -m "docs(ch02): write section 2.1 — Protocol Buffers and gRPC"
```

---

## Task 11: Write Tutorial Section 2.2 — PostgreSQL & Migrations

**Files:**
- Create: `docs/src/ch02/postgresql-migrations.md`

- [ ] **Step 1: Write section 2.2**

This section covers:

- **Running PostgreSQL locally** — `docker run` command for a development PostgreSQL instance, connecting with `psql`
- **Why versioned migrations** — the problem with manual schema changes, why GORM AutoMigrate is dangerous in production (no rollbacks, no version tracking, can silently drop columns), what golang-migrate gives you
- **Writing migrations** — walk through the up/down SQL files, UUID extension, CHECK constraints, indexes, TIMESTAMPTZ
- **Embedding migrations in Go** — `embed.FS`, why embed rather than external files (single binary deployment)
- **Running migrations programmatically** — the `runMigrations()` function from main.go, `migrate.ErrNoChange`
- **Exercise:** Write a second migration `000002_add_language_column.up.sql` that adds a `language VARCHAR(50) DEFAULT 'English'` column to the books table. Write the corresponding down migration. Run both up and down to verify.

Include footnoted references:
1. [golang-migrate documentation](https://github.com/golang-migrate/migrate)
2. [PostgreSQL CREATE TABLE](https://www.postgresql.org/docs/current/sql-createtable.html)
3. [Go embed package](https://pkg.go.dev/embed)

Content length target: 1000-1400 words.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch02/postgresql-migrations.md
git commit -m "docs(ch02): write section 2.2 — PostgreSQL and migrations"
```

---

## Task 12: Write Tutorial Section 2.3 — Repository Pattern with GORM

**Files:**
- Create: `docs/src/ch02/repository-pattern.md`

- [ ] **Step 1: Write section 2.3**

This section covers:

- **GORM basics** — model struct tags, `gorm.Open`, `Create`, `First`, `Find`, `Updates`, `Delete`. Compare to JPA/Hibernate (Java) — similar concept, different feel.
- **The repository pattern** — what it is (an abstraction over data access), why it matters (testability, swappability), how it differs from using GORM directly in handlers
- **Implementing BookRepository** — walk through each method, explain GORM queries, error handling (`gorm.ErrRecordNotFound`), the duplicate key detection pattern
- **Pagination and filtering** — building dynamic queries with GORM's chainable API, `Count` + `Offset` + `Limit`, `ILIKE` for case-insensitive search
- **Integration testing** — testing against a real PostgreSQL, `testDB()` helper function, AutoMigrate for test convenience vs golang-migrate for production, table truncation between tests
- **Exercise:** Add a method `GetByISBN(ctx, isbn) (*Book, error)` to the repository. Write an integration test for it.

Include footnoted references:
1. [GORM documentation](https://gorm.io/docs/)
2. [GORM PostgreSQL driver](https://gorm.io/docs/connecting_to_the_database.html#PostgreSQL)
3. [Repository pattern explained](https://martinfowler.com/eaaCatalog/repository.html)

Content length target: 1200-1600 words.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch02/repository-pattern.md
git commit -m "docs(ch02): write section 2.3 — repository pattern with GORM"
```

---

## Task 13: Write Tutorial Section 2.4 — Service Layer

**Files:**
- Create: `docs/src/ch02/service-layer.md`

- [ ] **Step 1: Write section 2.4**

This section covers:

- **Defining interfaces in Go** — the service layer defines what it needs (BookRepository interface), the repository implements it. Implicit satisfaction means no `implements` keyword. Compare to Java/Kotlin interface-based DI.
- **The service as orchestrator** — it coordinates repository calls and enforces business rules, but doesn't know about GORM or gRPC
- **Business validation** — `validateBook()`, availability invariants when updating total_copies, domain error types with `fmt.Errorf("%w", ...)` for wrapping
- **Testing with mocks** — hand-written mock implementing BookRepository, testing business logic without a database, why hand-written mocks are preferred in Go (vs mockgen/gomock for learning)
- **Exercise:** Add a business rule: a book cannot be deleted if it has active reservations (i.e., `available_copies < total_copies`). Write a test first, then implement.

Include footnoted references:
1. [Effective Go — Interfaces](https://go.dev/doc/effective_go#interfaces)
2. [Go Blog: Error handling and Go](https://go.dev/blog/error-handling-and-go)

Content length target: 1000-1400 words.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch02/service-layer.md
git commit -m "docs(ch02): write section 2.4 — service layer and business logic"
```

---

## Task 14: Write Tutorial Section 2.5 — Wiring It All Together

**Files:**
- Create: `docs/src/ch02/wiring.md`

- [ ] **Step 1: Write section 2.5**

This section covers:

- **Constructor-based dependency injection** — no framework needed, just `NewX(dep)` functions. Walk through main.go showing the wiring: `db → repo → svc → handler → grpcServer`. Compare to Spring's @Autowired — same concept, explicit instead of magic.
- **gRPC server setup** — `grpc.NewServer()`, registering the service, `reflection.Register` for grpcurl discovery, `net.Listen` and `Serve`
- **The handler layer** — proto ↔ domain conversion with `toProtoBook()`, error mapping with `toGRPCError()`, why proto types and domain types are separate
- **Testing with grpcurl** — install grpcurl, list services, create a book, list books with filters, update, delete. Full command examples.
- **Implementation notes** — proto3 partial update limitation (zero values vs missing fields), the UpdateAvailability RPC as a forward reference to Chapter 6
- **Exercise:** Start the service, use grpcurl to create 5 books across 2-3 genres. List all, filter by genre, filter by available_only. Update one book's total_copies, then check available_copies adjusted correctly. Delete a book and verify it's gone.

Include footnoted references:
1. [gRPC Go basics](https://grpc.io/docs/languages/go/basics/)
2. [grpcurl documentation](https://github.com/fullstorydev/grpcurl)
3. [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html)

Content length target: 1200-1600 words.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch02/wiring.md
git commit -m "docs(ch02): write section 2.5 — wiring and grpcurl testing"
```

---

## Task 15: Final Verification

**Files:**
- No new files — verification only

- [ ] **Step 1: Run unit tests (no DB required)**

```bash
cd services/catalog && go test -v -race ./internal/service/... ./internal/handler/...
```

Expected: all tests pass.

- [ ] **Step 2: Run integration tests (requires PostgreSQL)**

```bash
cd services/catalog && go test -v ./internal/repository/... -count=1
```

Expected: all tests pass (or skip if no DB).

- [ ] **Step 3: Verify gateway tests still pass**

```bash
cd services/gateway && go test -v ./...
```

Expected: 4 tests pass.

- [ ] **Step 4: Verify compilation**

```bash
go build ./services/catalog/... && go build ./services/gateway/...
```

- [ ] **Step 5: Start the service and test with grpcurl** (requires PostgreSQL)

```bash
cd services/catalog && go run ./cmd/ &
sleep 2
grpcurl -plaintext localhost:50052 list
grpcurl -plaintext -d '{"title":"Test","author":"Author","total_copies":3}' localhost:50052 catalog.v1.CatalogService/CreateBook
kill %1
```

- [ ] **Step 6: Commit any fixes**

```bash
git add -A
git commit -m "fix(ch02): address issues found during final verification"
```

Only if changes were made.
