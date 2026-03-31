# Chapter 7: Search Service & Meilisearch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full-text search to the library system via Catalog event publishing, a new Search service backed by Meilisearch, and gateway UI with autocomplete.

**Architecture:** Catalog publishes `catalog.books.changed` events after book CRUD and availability changes. A new Search service consumes those events, maintains a Meilisearch index, and exposes Search/Suggest gRPC RPCs. The Gateway gets a search page with HTMX autocomplete.

**Tech Stack:** Go, gRPC, Apache Kafka (sarama), Meilisearch (meilisearch-go), HTMX, Docker Compose

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `proto/search/v1/search.proto` | Search/Suggest RPC definitions |
| `gen/search/v1/search.pb.go` | Generated protobuf code |
| `gen/search/v1/search_grpc.pb.go` | Generated gRPC code |
| `services/catalog/internal/kafka/publisher.go` | Sarama SyncProducer for catalog events |
| `services/search/go.mod` | Search service module |
| `services/search/cmd/main.go` | Service wiring and startup |
| `services/search/internal/model/model.go` | BookDocument struct |
| `services/search/internal/index/index.go` | IndexRepository interface + Meilisearch impl |
| `services/search/internal/service/service.go` | SearchService business logic |
| `services/search/internal/service/service_test.go` | Service layer tests |
| `services/search/internal/handler/handler.go` | gRPC handler |
| `services/search/internal/handler/handler_test.go` | Handler tests |
| `services/search/internal/consumer/consumer.go` | Kafka consumer for catalog events |
| `services/search/internal/consumer/consumer_test.go` | Consumer tests |
| `services/search/internal/bootstrap/bootstrap.go` | Full sync from Catalog on startup |
| `services/search/internal/bootstrap/bootstrap_test.go` | Bootstrap tests |
| `services/search/Dockerfile` | Production multi-stage build |
| `services/search/Dockerfile.dev` | Dev image with hot reload |
| `services/search/.air.toml` | Air config |
| `services/search/Earthfile` | Build targets |
| `services/gateway/internal/handler/search.go` | Search page + suggest handlers |
| `services/gateway/internal/handler/search_test.go` | Gateway search tests |
| `services/gateway/templates/search.html` | Search results page |
| `services/gateway/templates/partials/suggestions.html` | Autocomplete dropdown partial |
| `docs/src/ch07/index.md` | Chapter 7 overview |
| `docs/src/ch07/catalog-events.md` | Section stub |
| `docs/src/ch07/search-service.md` | Section stub |
| `docs/src/ch07/meilisearch.md` | Section stub |
| `docs/src/ch07/search-ui.md` | Section stub |

### Modified Files

| File | Change |
|------|--------|
| `services/catalog/internal/service/catalog.go` | Add EventPublisher, publish events, refactor UpdateAvailability |
| `services/catalog/internal/service/catalog_test.go` | Add mock publisher, update NewCatalogService calls, add event tests |
| `services/catalog/internal/handler/catalog.go` | Update UpdateAvailability to use returned book |
| `services/catalog/internal/handler/catalog_test.go` | Update NewCatalogService calls to pass nil publisher |
| `services/catalog/internal/consumer/consumer.go` | Update AvailabilityUpdater interface signature |
| `services/catalog/internal/consumer/consumer_test.go` | Update mock to match new signature |
| `services/catalog/cmd/main.go` | Wire Kafka producer, pass to NewCatalogService |
| `services/gateway/internal/handler/server.go` | Add search client field, update New() |
| `services/gateway/cmd/main.go` | Wire search gRPC connection + routes |
| `services/gateway/templates/partials/nav.html` | Add search input with HTMX |
| `services/gateway/internal/handler/auth_test.go` | Update 8 handler.New() calls (add nil search) |
| `services/gateway/internal/handler/catalog_test.go` | Update 11 handler.New() calls |
| `services/gateway/internal/handler/reservation_test.go` | Update 6 handler.New() calls |
| `services/gateway/internal/handler/render_test.go` | Update 2 handler.New() calls |
| `services/gateway/internal/handler/health_test.go` | Update 1 handler.New() call |
| `go.work` | Add `./services/search` |
| `deploy/docker-compose.yml` | Add meilisearch + search services, update gateway |
| `deploy/docker-compose.dev.yml` | Add search dev override |
| `deploy/.env` | Add Meilisearch + search env vars |
| `Earthfile` | Add search to ci/lint/test |
| `docs/src/SUMMARY.md` | Append Chapter 7 |

---

### Task 1: Search Proto Definition & Code Generation

**Files:**
- Create: `proto/search/v1/search.proto`
- Create: `gen/search/v1/search.pb.go` (generated)
- Create: `gen/search/v1/search_grpc.pb.go` (generated)

- [ ] **Step 1: Create search proto**

Create `proto/search/v1/search.proto`:

```protobuf
syntax = "proto3";
package search.v1;
option go_package = "github.com/fesoliveira014/library-system/gen/search/v1;searchv1";

service SearchService {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc Suggest(SuggestRequest) returns (SuggestResponse);
}

message SearchRequest {
  string query = 1;
  string genre = 2;
  string author = 3;
  bool available_only = 4;
  int32 page = 5;
  int32 page_size = 6;
}

message SearchResponse {
  repeated BookResult books = 1;
  int64 total_hits = 2;
  int64 query_time_ms = 3;
}

message BookResult {
  string id = 1;
  string title = 2;
  string author = 3;
  string isbn = 4;
  string genre = 5;
  string description = 6;
  int32 published_year = 7;
  int32 total_copies = 8;
  int32 available_copies = 9;
}

message SuggestRequest {
  string prefix = 1;
  int32 limit = 2;
}

message SuggestResponse {
  repeated Suggestion suggestions = 1;
}

message Suggestion {
  string book_id = 1;
  string title = 2;
  string author = 3;
}
```

- [ ] **Step 2: Generate Go code**

Run:
```bash
protoc --go_out=gen --go_opt=paths=source_relative \
       --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
       proto/search/v1/search.proto
```

Expected: Files `gen/search/v1/search.pb.go` and `gen/search/v1/search_grpc.pb.go` created.

- [ ] **Step 3: Verify gen compiles**

Run: `go build ./gen/search/...`
Expected: PASS (no errors)

- [ ] **Step 4: Commit**

```bash
git add proto/search/v1/search.proto gen/search/v1/
git commit -m "feat: add search service proto definition and generated code"
```

---

### Task 2: Catalog Event Publishing — Service Layer Changes

**Files:**
- Modify: `services/catalog/internal/service/catalog.go`
- Modify: `services/catalog/internal/service/catalog_test.go`

**Context:** The existing `CatalogService` has methods: `CreateBook`, `GetBook`, `UpdateBook`, `DeleteBook`, `ListBooks`, `UpdateAvailability`. We're adding an `EventPublisher` dependency and publishing events after successful writes. The `UpdateAvailability` method signature changes from `(ctx, id, delta) error` to `(ctx, id, delta) (*model.Book, error)`.

- [ ] **Step 1: Update service tests with mock publisher and new signature**

Replace the entire `services/catalog/internal/service/catalog_test.go` with:

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

// mockBookRepository is an in-memory mock for testing the service layer.
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

func (m *mockBookRepository) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
	if _, ok := m.books[book.ID]; !ok {
		return nil, model.ErrBookNotFound
	}
	m.books[book.ID] = book
	return book, nil
}

func (m *mockBookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.books[id]; !ok {
		return model.ErrBookNotFound
	}
	delete(m.books, id)
	return nil
}

func (m *mockBookRepository) List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	var result []*model.Book
	for _, b := range m.books {
		result = append(result, b)
	}
	return result, int64(len(result)), nil
}

func (m *mockBookRepository) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	b, ok := m.books[id]
	if !ok {
		return model.ErrBookNotFound
	}
	b.AvailableCopies += delta
	return nil
}

// mockPublisher records published events for assertions.
type mockPublisher struct {
	events []service.BookEvent
}

func (m *mockPublisher) Publish(_ context.Context, event service.BookEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestCatalogService_CreateBook_Success(t *testing.T) {
	pub := &mockPublisher{}
	svc := service.NewCatalogService(newMockRepo(), pub)
	ctx := context.Background()

	book := &model.Book{
		Title:       "Go in Action",
		Author:      "William Kennedy",
		TotalCopies: 3,
	}
	created, err := svc.CreateBook(ctx, book)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.AvailableCopies != 3 {
		t.Errorf("expected available_copies to equal total_copies (3), got %d", created.AvailableCopies)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].EventType != "book.created" {
		t.Errorf("expected event type book.created, got %s", pub.events[0].EventType)
	}
	if pub.events[0].Title != "Go in Action" {
		t.Errorf("expected event title 'Go in Action', got %s", pub.events[0].Title)
	}
}

func TestCatalogService_CreateBook_MissingTitle(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo(), &mockPublisher{})
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

func TestCatalogService_CreateBook_MissingAuthor(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo(), &mockPublisher{})
	ctx := context.Background()

	book := &model.Book{Title: "Some Title", TotalCopies: 1}
	_, err := svc.CreateBook(ctx, book)
	if err == nil {
		t.Fatal("expected error for missing author")
	}
	if !errors.Is(err, model.ErrInvalidBook) {
		t.Errorf("expected ErrInvalidBook, got %v", err)
	}
}

func TestCatalogService_CreateBook_NegativeCopies(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo(), &mockPublisher{})
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: -1}
	_, err := svc.CreateBook(ctx, book)
	if !errors.Is(err, model.ErrInvalidBook) {
		t.Errorf("expected ErrInvalidBook for negative copies, got %v", err)
	}
}

func TestCatalogService_GetBook_NotFound(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo(), &mockPublisher{})
	ctx := context.Background()

	_, err := svc.GetBook(ctx, uuid.New())
	if !errors.Is(err, model.ErrBookNotFound) {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestCatalogService_UpdateBook_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := service.NewCatalogService(repo, pub)
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: 3}
	created, _ := svc.CreateBook(ctx, book)
	pub.events = nil // reset after create

	created.Title = "Updated Title"
	_, err := svc.UpdateBook(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].EventType != "book.updated" {
		t.Errorf("expected event type book.updated, got %s", pub.events[0].EventType)
	}
}

func TestCatalogService_DeleteBook_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := service.NewCatalogService(repo, pub)
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: 1}
	created, _ := svc.CreateBook(ctx, book)
	pub.events = nil

	err := svc.DeleteBook(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].EventType != "book.deleted" {
		t.Errorf("expected event type book.deleted, got %s", pub.events[0].EventType)
	}
	if pub.events[0].Title != "" {
		t.Errorf("expected empty title for delete event, got %s", pub.events[0].Title)
	}
}

func TestCatalogService_UpdateAvailability_ReturnsBook(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := service.NewCatalogService(repo, pub)
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: 5, AvailableCopies: 5}
	created, _ := svc.CreateBook(ctx, book)
	pub.events = nil

	updated, err := svc.UpdateAvailability(ctx, created.ID, -1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.AvailableCopies != 4 {
		t.Errorf("expected 4 available copies, got %d", updated.AvailableCopies)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].EventType != "book.updated" {
		t.Errorf("expected event type book.updated, got %s", pub.events[0].EventType)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./services/catalog/internal/service/... -v -count=1`
Expected: FAIL — `NewCatalogService` doesn't accept publisher yet, `UpdateAvailability` returns only error

- [ ] **Step 3: Update CatalogService implementation**

Replace `services/catalog/internal/service/catalog.go` with:

```go
package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// BookRepository defines the interface for book persistence.
type BookRepository interface {
	Create(ctx context.Context, book *model.Book) (*model.Book, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
	Update(ctx context.Context, book *model.Book) (*model.Book, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error)
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}

// BookEvent represents a catalog book change event published to Kafka.
type BookEvent struct {
	EventType       string    `json:"event_type"`
	BookID          string    `json:"book_id"`
	Title           string    `json:"title,omitempty"`
	Author          string    `json:"author,omitempty"`
	ISBN            string    `json:"isbn,omitempty"`
	Genre           string    `json:"genre,omitempty"`
	Description     string    `json:"description,omitempty"`
	PublishedYear   int       `json:"published_year,omitempty"`
	TotalCopies     int       `json:"total_copies,omitempty"`
	AvailableCopies int       `json:"available_copies,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

// EventPublisher publishes book change events.
type EventPublisher interface {
	Publish(ctx context.Context, event BookEvent) error
}

// CatalogService contains business logic for managing the book catalog.
type CatalogService struct {
	repo      BookRepository
	publisher EventPublisher
}

// NewCatalogService creates a new catalog service with the given repository and event publisher.
func NewCatalogService(repo BookRepository, publisher EventPublisher) *CatalogService {
	return &CatalogService{repo: repo, publisher: publisher}
}

func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if err := validateBook(book); err != nil {
		return nil, err
	}
	book.AvailableCopies = book.TotalCopies
	created, err := s.repo.Create(ctx, book)
	if err != nil {
		return nil, err
	}

	if err := s.publisher.Publish(ctx, bookToEvent("book.created", created)); err != nil {
		log.Printf("failed to publish book.created event for book %s: %v", created.ID, err)
	}

	return created, nil
}

func (s *CatalogService) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CatalogService) UpdateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if book.TotalCopies < 0 {
		return nil, fmt.Errorf("%w: total copies must be non-negative", model.ErrInvalidBook)
	}
	updated, err := s.repo.Update(ctx, book)
	if err != nil {
		return nil, err
	}

	if err := s.publisher.Publish(ctx, bookToEvent("book.updated", updated)); err != nil {
		log.Printf("failed to publish book.updated event for book %s: %v", updated.ID, err)
	}

	return updated, nil
}

func (s *CatalogService) DeleteBook(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.publisher.Publish(ctx, BookEvent{
		EventType: "book.deleted",
		BookID:    id.String(),
		Timestamp: time.Now(),
	}); err != nil {
		log.Printf("failed to publish book.deleted event for book %s: %v", id, err)
	}

	return nil
}

func (s *CatalogService) ListBooks(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	return s.repo.List(ctx, filter, page)
}

func (s *CatalogService) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) (*model.Book, error) {
	if err := s.repo.UpdateAvailability(ctx, id, delta); err != nil {
		return nil, err
	}

	book, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch book after availability update: %w", err)
	}

	if err := s.publisher.Publish(ctx, bookToEvent("book.updated", book)); err != nil {
		log.Printf("failed to publish book.updated event for book %s: %v", id, err)
	}

	return book, nil
}

func bookToEvent(eventType string, book *model.Book) BookEvent {
	return BookEvent{
		EventType:       eventType,
		BookID:          book.ID.String(),
		Title:           book.Title,
		Author:          book.Author,
		ISBN:            book.ISBN,
		Genre:           book.Genre,
		Description:     book.Description,
		PublishedYear:   book.PublishedYear,
		TotalCopies:     book.TotalCopies,
		AvailableCopies: book.AvailableCopies,
		Timestamp:       time.Now(),
	}
}

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

- [ ] **Step 4: Run service tests — expect pass**

Run: `go test ./services/catalog/internal/service/... -v -count=1`
Expected: PASS — all 8 tests pass

- [ ] **Step 5: Update catalog handler for new UpdateAvailability signature**

In `services/catalog/internal/handler/catalog.go`, replace the `UpdateAvailability` method (lines 138-156) with:

```go
func (h *CatalogHandler) UpdateAvailability(ctx context.Context, req *catalogv1.UpdateAvailabilityRequest) (*catalogv1.UpdateAvailabilityResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book, err := h.svc.UpdateAvailability(ctx, id, int(req.GetDelta()))
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &catalogv1.UpdateAvailabilityResponse{
		AvailableCopies: int32(book.AvailableCopies),
	}, nil
}
```

- [ ] **Step 6: Update catalog handler tests for new NewCatalogService signature**

In `services/catalog/internal/handler/catalog_test.go`, replace all occurrences of `service.NewCatalogService(newInMemoryRepo())` with `service.NewCatalogService(newInMemoryRepo(), &noopPublisher{})`.

Add the noopPublisher at the top of the file (after the `inMemoryRepo` definition):

```go
type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }
```

There are 6 occurrences of `service.NewCatalogService(newInMemoryRepo())` at lines 76, 96, 115, 134, 146 (TestCatalogService_UpdateAvailability calls `svc` at line 145 — but this test is in `catalog_test.go` in handler package, with the same pattern). Each needs updating.

- [ ] **Step 7: Update consumer for new UpdateAvailability signature**

In `services/catalog/internal/consumer/consumer.go`, update the `AvailabilityUpdater` interface:

```go
type AvailabilityUpdater interface {
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) (*model.Book, error)
}
```

Add the import for model:
```go
"github.com/fesoliveira014/library-system/services/catalog/internal/model"
```

In `handleEvent`, update the call to discard the book return value:
```go
_, err = svc.UpdateAvailability(ctx, bookID, delta)
return err
```

- [ ] **Step 8: Update consumer tests for new interface**

In `services/catalog/internal/consumer/consumer_test.go`, update `mockCatalogService`:

```go
type mockCatalogService struct {
	calls []struct {
		ID    uuid.UUID
		Delta int
	}
}

func (m *mockCatalogService) UpdateAvailability(_ context.Context, id uuid.UUID, delta int) (*model.Book, error) {
	m.calls = append(m.calls, struct {
		ID    uuid.UUID
		Delta int
	}{id, delta})
	return &model.Book{}, nil
}
```

Add the model import to the test file.

- [ ] **Step 9: Run all catalog tests**

Run: `go test ./services/catalog/... -v -count=1 -short`
Expected: PASS — all service, handler, and consumer tests pass

- [ ] **Step 10: Commit**

```bash
git add services/catalog/internal/
git commit -m "feat: add event publishing to catalog service, refactor UpdateAvailability"
```

---

### Task 3: Catalog Kafka Publisher & Main Wiring

**Files:**
- Create: `services/catalog/internal/kafka/publisher.go`
- Modify: `services/catalog/cmd/main.go`

- [ ] **Step 1: Create catalog Kafka publisher**

Create `services/catalog/internal/kafka/publisher.go`:

```go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"

	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

// Publisher wraps a sarama SyncProducer and implements service.EventPublisher.
type Publisher struct {
	producer sarama.SyncProducer
	topic    string
}

// NewPublisher creates a Kafka publisher for the given topic.
func NewPublisher(brokers []string, topic string) (*Publisher, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &Publisher{producer: producer, topic: topic}, nil
}

// Publish sends a book event to Kafka, keyed by book_id.
func (p *Publisher) Publish(_ context.Context, event service.BookEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.BookID),
		Value: sarama.ByteEncoder(value),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	return nil
}

// Close shuts down the producer.
func (p *Publisher) Close() error {
	return p.producer.Close()
}
```

- [ ] **Step 2: Update catalog main.go to wire publisher**

Replace `services/catalog/cmd/main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/catalog/internal/consumer"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	catalogkafka "github.com/fesoliveira014/library-system/services/catalog/internal/kafka"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// noopPublisher is a no-op EventPublisher used when Kafka is not configured.
type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

func main() {
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "host=localhost port=5432 user=postgres password=postgres dbname=catalog sslmode=disable"
	}
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50052"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")

	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to PostgreSQL")

	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	bookRepo := repository.NewBookRepository(db)

	var publisher service.EventPublisher = &noopPublisher{}
	var brokers []string
	if kafkaBrokers != "" {
		brokers = strings.Split(kafkaBrokers, ",")
		pub, err := catalogkafka.NewPublisher(brokers, "catalog.books.changed")
		if err != nil {
			log.Fatalf("failed to create kafka publisher: %v", err)
		}
		defer pub.Close()
		publisher = pub
		log.Println("kafka publisher initialized for catalog.books.changed topic")
	}

	catalogSvc := service.NewCatalogService(bookRepo, publisher)
	catalogHandler := handler.NewCatalogHandler(catalogSvc)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if len(brokers) > 0 {
		go func() {
			log.Println("starting kafka consumer for reservations topic")
			if err := consumer.Run(ctx, brokers, "reservations", catalogSvc); err != nil {
				log.Printf("kafka consumer error: %v", err)
			}
		}()
	}

	skipMethods := []string{
		"/catalog.v1.CatalogService/GetBook",
		"/catalog.v1.CatalogService/ListBooks",
		"/catalog.v1.CatalogService/UpdateAvailability",
	}
	interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	catalogv1.RegisterCatalogServiceServer(grpcServer, catalogHandler)
	reflection.Register(grpcServer)

	log.Printf("catalog service listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
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
```

- [ ] **Step 3: Verify build**

Run: `go build -o /tmp/catalog-svc ./services/catalog/cmd/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add services/catalog/internal/kafka/ services/catalog/cmd/main.go
git commit -m "feat: add Kafka publisher to catalog service for book change events"
```

---

### Task 4: Search Service Scaffold — Module, Model, Index Layer

**Files:**
- Create: `services/search/go.mod`
- Create: `services/search/internal/model/model.go`
- Create: `services/search/internal/index/index.go`
- Modify: `go.work`

- [ ] **Step 1: Create search service module**

Create a minimal `services/search/go.mod` with just the module declaration and replace directives — `go mod tidy` will resolve all dependencies:

```
module github.com/fesoliveira014/library-system/services/search

go 1.26.1

replace github.com/fesoliveira014/library-system/gen => ../../gen

replace github.com/fesoliveira014/library-system/pkg/auth => ../../pkg/auth
```

Then run `go mod tidy` from the search directory to resolve all dependencies.

- [ ] **Step 2: Update go.work**

Add `./services/search` to `go.work`:

```
go 1.26.1

use (
	./gen
	./pkg/auth
	./services/auth
	./services/catalog
	./services/gateway
	./services/reservation
	./services/search
)
```

- [ ] **Step 3: Create model**

Create `services/search/internal/model/model.go`:

```go
package model

// BookDocument is the shape stored in and retrieved from Meilisearch.
type BookDocument struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	ISBN            string `json:"isbn"`
	Genre           string `json:"genre"`
	Description     string `json:"description"`
	PublishedYear   int    `json:"published_year"`
	TotalCopies     int    `json:"total_copies"`
	AvailableCopies int    `json:"available_copies"`
}

// Suggestion is a lightweight result for autocomplete.
type Suggestion struct {
	BookID string
	Title  string
	Author string
}
```

- [ ] **Step 4: Create index layer**

Create `services/search/internal/index/index.go`:

```go
package index

import (
	"context"
	"fmt"
	"strings"

	"github.com/meilisearch/meilisearch-go"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

const indexName = "books"

// SearchFilters holds optional filter parameters for search queries.
type SearchFilters struct {
	Genre         string
	Author        string
	AvailableOnly bool
}

// IndexRepository defines the interface for the search index data store.
type IndexRepository interface {
	Upsert(ctx context.Context, doc model.BookDocument) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
	Suggest(ctx context.Context, prefix string, limit int) ([]model.BookDocument, error)
	Count(ctx context.Context) (int64, error)
	EnsureIndex(ctx context.Context) error
}

// MeilisearchIndex implements IndexRepository backed by Meilisearch.
type MeilisearchIndex struct {
	client meilisearch.ServiceManager
}

// NewMeilisearchIndex creates a new Meilisearch-backed index.
func NewMeilisearchIndex(url, apiKey string) *MeilisearchIndex {
	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	return &MeilisearchIndex{client: client}
}

func (m *MeilisearchIndex) EnsureIndex(_ context.Context) error {
	_, err := m.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexName,
		PrimaryKey: "id",
	})
	// Ignore "index_already_exists" error
	if err != nil {
		if meiliErr, ok := err.(*meilisearch.Error); ok {
			if meiliErr.MeilisearchApiError.Code == "index_already_exists" {
				// Index exists, continue to configure attributes
			} else {
				return fmt.Errorf("create index: %w", err)
			}
		} else {
			return fmt.Errorf("create index: %w", err)
		}
	}

	idx := m.client.Index(indexName)

	if _, err := idx.UpdateSearchableAttributes(&[]string{
		"title", "author", "isbn", "description", "genre",
	}); err != nil {
		return fmt.Errorf("update searchable attributes: %w", err)
	}
	if _, err := idx.UpdateFilterableAttributes(&[]string{
		"genre", "author", "available_copies",
	}); err != nil {
		return fmt.Errorf("update filterable attributes: %w", err)
	}
	if _, err := idx.UpdateSortableAttributes(&[]string{
		"title", "published_year",
	}); err != nil {
		return fmt.Errorf("update sortable attributes: %w", err)
	}

	return nil
}

func (m *MeilisearchIndex) Upsert(_ context.Context, doc model.BookDocument) error {
	idx := m.client.Index(indexName)
	_, err := idx.AddDocuments([]model.BookDocument{doc}, "id")
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}
	return nil
}

func (m *MeilisearchIndex) Delete(_ context.Context, id string) error {
	idx := m.client.Index(indexName)
	_, err := idx.DeleteDocument(id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (m *MeilisearchIndex) Search(_ context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	idx := m.client.Index(indexName)

	filterParts := buildFilterString(filters)

	offset := int64(0)
	if page > 1 {
		offset = int64((page - 1) * pageSize)
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(pageSize),
		Offset: offset,
	}
	if len(filterParts) > 0 {
		req.Filter = strings.Join(filterParts, " AND ")
	}

	resp, err := idx.Search(query, req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("search: %w", err)
	}

	docs := make([]model.BookDocument, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		doc, err := hitToDocument(hit)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, resp.EstimatedTotalHits, int64(resp.ProcessingTimeMs), nil
}

func (m *MeilisearchIndex) Suggest(_ context.Context, prefix string, limit int) ([]model.BookDocument, error) {
	idx := m.client.Index(indexName)

	resp, err := idx.Search(prefix, &meilisearch.SearchRequest{
		Limit:                int64(limit),
		AttributesToRetrieve: []string{"id", "title", "author"},
	})
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}

	docs := make([]model.BookDocument, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		doc, err := hitToDocument(hit)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (m *MeilisearchIndex) Count(_ context.Context) (int64, error) {
	stats, err := m.client.Index(indexName).GetStats()
	if err != nil {
		return 0, fmt.Errorf("get index stats: %w", err)
	}
	return stats.NumberOfDocuments, nil
}

func buildFilterString(filters SearchFilters) []string {
	var parts []string
	if filters.Genre != "" {
		parts = append(parts, fmt.Sprintf("genre = %q", filters.Genre))
	}
	if filters.Author != "" {
		parts = append(parts, fmt.Sprintf("author = %q", filters.Author))
	}
	if filters.AvailableOnly {
		parts = append(parts, "available_copies > 0")
	}
	return parts
}

func hitToDocument(hit interface{}) (model.BookDocument, error) {
	m, ok := hit.(map[string]interface{})
	if !ok {
		return model.BookDocument{}, fmt.Errorf("unexpected hit type")
	}

	doc := model.BookDocument{}
	if v, ok := m["id"].(string); ok {
		doc.ID = v
	}
	if v, ok := m["title"].(string); ok {
		doc.Title = v
	}
	if v, ok := m["author"].(string); ok {
		doc.Author = v
	}
	if v, ok := m["isbn"].(string); ok {
		doc.ISBN = v
	}
	if v, ok := m["genre"].(string); ok {
		doc.Genre = v
	}
	if v, ok := m["description"].(string); ok {
		doc.Description = v
	}
	if v, ok := m["published_year"].(float64); ok {
		doc.PublishedYear = int(v)
	}
	if v, ok := m["total_copies"].(float64); ok {
		doc.TotalCopies = int(v)
	}
	if v, ok := m["available_copies"].(float64); ok {
		doc.AvailableCopies = int(v)
	}
	return doc, nil
}
```

- [ ] **Step 5: Run go mod tidy and verify build**

Run:
```bash
cd services/search && go mod tidy && cd ../..
go build ./services/search/internal/...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add services/search/go.mod services/search/go.sum services/search/internal/model/ services/search/internal/index/ go.work
git commit -m "feat: add search service scaffold — module, model, index layer"
```

---

### Task 5: Search Service Layer

**Files:**
- Create: `services/search/internal/service/service.go`
- Create: `services/search/internal/service/service_test.go`

- [ ] **Step 1: Write service tests**

Create `services/search/internal/service/service_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
	"github.com/fesoliveira014/library-system/services/search/internal/service"
)

type mockIndex struct {
	docs      map[string]model.BookDocument
	ensured   bool
	searchRes []model.BookDocument
}

func newMockIndex() *mockIndex {
	return &mockIndex{docs: make(map[string]model.BookDocument)}
}

func (m *mockIndex) Upsert(_ context.Context, doc model.BookDocument) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockIndex) Delete(_ context.Context, id string) error {
	delete(m.docs, id)
	return nil
}

func (m *mockIndex) Search(_ context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	return m.searchRes, int64(len(m.searchRes)), 1, nil
}

func (m *mockIndex) Suggest(_ context.Context, prefix string, limit int) ([]model.BookDocument, error) {
	return m.searchRes, nil
}

func (m *mockIndex) Count(_ context.Context) (int64, error) {
	return int64(len(m.docs)), nil
}

func (m *mockIndex) EnsureIndex(_ context.Context) error {
	m.ensured = true
	return nil
}

func TestSearchService_Upsert(t *testing.T) {
	idx := newMockIndex()
	svc := service.NewSearchService(idx)

	err := svc.Upsert(context.Background(), model.BookDocument{ID: "1", Title: "Go Book"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(idx.docs))
	}
}

func TestSearchService_Delete(t *testing.T) {
	idx := newMockIndex()
	idx.docs["1"] = model.BookDocument{ID: "1"}
	svc := service.NewSearchService(idx)

	err := svc.Delete(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(idx.docs))
	}
}

func TestSearchService_Search_DefaultPagination(t *testing.T) {
	idx := newMockIndex()
	idx.searchRes = []model.BookDocument{{ID: "1", Title: "Go Book"}}
	svc := service.NewSearchService(idx)

	docs, total, _, err := svc.Search(context.Background(), "go", index.SearchFilters{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 result, got %d", len(docs))
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestSearchService_Suggest(t *testing.T) {
	idx := newMockIndex()
	idx.searchRes = []model.BookDocument{{ID: "1", Title: "Go in Action", Author: "Kennedy"}}
	svc := service.NewSearchService(idx)

	suggestions, err := svc.Suggest(context.Background(), "go", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Title != "Go in Action" {
		t.Errorf("expected title 'Go in Action', got %s", suggestions[0].Title)
	}
}

func TestSearchService_EnsureIndex(t *testing.T) {
	idx := newMockIndex()
	svc := service.NewSearchService(idx)

	err := svc.EnsureIndex(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !idx.ensured {
		t.Error("expected EnsureIndex to be called")
	}
}

func TestSearchService_Count(t *testing.T) {
	idx := newMockIndex()
	idx.docs["1"] = model.BookDocument{ID: "1"}
	svc := service.NewSearchService(idx)

	count, err := svc.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./services/search/internal/service/... -v -count=1`
Expected: FAIL — service package doesn't exist

- [ ] **Step 3: Create service implementation**

Create `services/search/internal/service/service.go`:

```go
package service

import (
	"context"

	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// SearchService provides search and indexing operations.
type SearchService struct {
	index index.IndexRepository
}

// NewSearchService creates a new search service.
func NewSearchService(idx index.IndexRepository) *SearchService {
	return &SearchService{index: idx}
}

func (s *SearchService) Search(ctx context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return s.index.Search(ctx, query, filters, page, pageSize)
}

func (s *SearchService) Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	docs, err := s.index.Suggest(ctx, prefix, limit)
	if err != nil {
		return nil, err
	}

	suggestions := make([]model.Suggestion, len(docs))
	for i, d := range docs {
		suggestions[i] = model.Suggestion{
			BookID: d.ID,
			Title:  d.Title,
			Author: d.Author,
		}
	}
	return suggestions, nil
}

func (s *SearchService) Upsert(ctx context.Context, doc model.BookDocument) error {
	return s.index.Upsert(ctx, doc)
}

func (s *SearchService) Delete(ctx context.Context, id string) error {
	return s.index.Delete(ctx, id)
}

func (s *SearchService) EnsureIndex(ctx context.Context) error {
	return s.index.EnsureIndex(ctx)
}

func (s *SearchService) Count(ctx context.Context) (int64, error) {
	return s.index.Count(ctx)
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./services/search/internal/service/... -v -count=1`
Expected: PASS — all 6 tests pass

- [ ] **Step 5: Commit**

```bash
git add services/search/internal/service/
git commit -m "feat: add search service layer with tests"
```

---

### Task 6: Search gRPC Handler

**Files:**
- Create: `services/search/internal/handler/handler.go`
- Create: `services/search/internal/handler/handler_test.go`

- [ ] **Step 1: Write handler tests**

Create `services/search/internal/handler/handler_test.go`:

```go
package handler_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/handler"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

type mockService struct {
	searchDocs  []model.BookDocument
	suggestions []model.Suggestion
	totalHits   int64
	queryTimeMs int64
}

func (m *mockService) Search(_ context.Context, _ string, _ index.SearchFilters, _, _ int) ([]model.BookDocument, int64, int64, error) {
	return m.searchDocs, m.totalHits, m.queryTimeMs, nil
}

func (m *mockService) Suggest(_ context.Context, _ string, _ int) ([]model.Suggestion, error) {
	return m.suggestions, nil
}

func TestSearchHandler_Search_Success(t *testing.T) {
	svc := &mockService{
		searchDocs:  []model.BookDocument{{ID: "1", Title: "Go Book", Author: "Author"}},
		totalHits:   1,
		queryTimeMs: 2,
	}
	h := handler.NewSearchHandler(svc)

	resp, err := h.Search(context.Background(), &searchv1.SearchRequest{
		Query:    "go",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Books) != 1 {
		t.Errorf("expected 1 book, got %d", len(resp.Books))
	}
	if resp.TotalHits != 1 {
		t.Errorf("expected total_hits 1, got %d", resp.TotalHits)
	}
	if resp.Books[0].Title != "Go Book" {
		t.Errorf("expected title 'Go Book', got %s", resp.Books[0].Title)
	}
}

func TestSearchHandler_Search_EmptyQuery(t *testing.T) {
	h := handler.NewSearchHandler(&mockService{})

	_, err := h.Search(context.Background(), &searchv1.SearchRequest{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestSearchHandler_Suggest_Success(t *testing.T) {
	svc := &mockService{
		suggestions: []model.Suggestion{{BookID: "1", Title: "Go in Action", Author: "Kennedy"}},
	}
	h := handler.NewSearchHandler(svc)

	resp, err := h.Suggest(context.Background(), &searchv1.SuggestRequest{
		Prefix: "go",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(resp.Suggestions))
	}
	if resp.Suggestions[0].Title != "Go in Action" {
		t.Errorf("expected 'Go in Action', got %s", resp.Suggestions[0].Title)
	}
}

func TestSearchHandler_Suggest_EmptyPrefix(t *testing.T) {
	h := handler.NewSearchHandler(&mockService{})

	_, err := h.Suggest(context.Background(), &searchv1.SuggestRequest{Prefix: ""})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty prefix, got %v", st.Code())
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./services/search/internal/handler/... -v -count=1`
Expected: FAIL

- [ ] **Step 3: Create handler implementation**

Create `services/search/internal/handler/handler.go`:

```go
package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// Service defines the interface the handler depends on.
type Service interface {
	Search(ctx context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
	Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error)
}

// SearchHandler implements the generated searchv1.SearchServiceServer.
type SearchHandler struct {
	searchv1.UnimplementedSearchServiceServer
	svc Service
}

// NewSearchHandler creates a new gRPC handler backed by the given service.
func NewSearchHandler(svc Service) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	filters := index.SearchFilters{
		Genre:         req.GetGenre(),
		Author:        req.GetAuthor(),
		AvailableOnly: req.GetAvailableOnly(),
	}

	docs, totalHits, queryTimeMs, err := h.svc.Search(ctx, req.GetQuery(), filters, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	books := make([]*searchv1.BookResult, len(docs))
	for i, d := range docs {
		books[i] = &searchv1.BookResult{
			Id:              d.ID,
			Title:           d.Title,
			Author:          d.Author,
			Isbn:            d.ISBN,
			Genre:           d.Genre,
			Description:     d.Description,
			PublishedYear:   int32(d.PublishedYear),
			TotalCopies:     int32(d.TotalCopies),
			AvailableCopies: int32(d.AvailableCopies),
		}
	}

	return &searchv1.SearchResponse{
		Books:       books,
		TotalHits:   totalHits,
		QueryTimeMs: queryTimeMs,
	}, nil
}

func (h *SearchHandler) Suggest(ctx context.Context, req *searchv1.SuggestRequest) (*searchv1.SuggestResponse, error) {
	if req.GetPrefix() == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	suggestions, err := h.svc.Suggest(ctx, req.GetPrefix(), int(req.GetLimit()))
	if err != nil {
		return nil, status.Error(codes.Internal, "suggest failed")
	}

	pbSuggestions := make([]*searchv1.Suggestion, len(suggestions))
	for i, s := range suggestions {
		pbSuggestions[i] = &searchv1.Suggestion{
			BookId: s.BookID,
			Title:  s.Title,
			Author: s.Author,
		}
	}

	return &searchv1.SuggestResponse{Suggestions: pbSuggestions}, nil
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./services/search/internal/handler/... -v -count=1`
Expected: PASS — all 4 tests pass

- [ ] **Step 5: Commit**

```bash
git add services/search/internal/handler/
git commit -m "feat: add search gRPC handler with tests"
```

---

### Task 7: Search Kafka Consumer

**Files:**
- Create: `services/search/internal/consumer/consumer.go`
- Create: `services/search/internal/consumer/consumer_test.go`

- [ ] **Step 1: Write consumer tests**

Create `services/search/internal/consumer/consumer_test.go`:

```go
package consumer

import (
	"context"
	"testing"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

type mockIndexer struct {
	upserted []model.BookDocument
	deleted  []string
}

func (m *mockIndexer) Upsert(_ context.Context, doc model.BookDocument) error {
	m.upserted = append(m.upserted, doc)
	return nil
}

func (m *mockIndexer) Delete(_ context.Context, id string) error {
	m.deleted = append(m.deleted, id)
	return nil
}

func TestHandleEvent_BookCreated(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.created",
		"book_id": "abc-123",
		"title": "Go Book",
		"author": "Author",
		"isbn": "1234567890",
		"genre": "programming",
		"total_copies": 5,
		"available_copies": 5
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(idx.upserted))
	}
	if idx.upserted[0].Title != "Go Book" {
		t.Errorf("expected title 'Go Book', got %s", idx.upserted[0].Title)
	}
	if idx.upserted[0].ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %s", idx.upserted[0].ID)
	}
}

func TestHandleEvent_BookUpdated(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.updated",
		"book_id": "abc-123",
		"title": "Updated",
		"author": "Author",
		"available_copies": 3
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(idx.upserted))
	}
	if idx.upserted[0].AvailableCopies != 3 {
		t.Errorf("expected 3 available copies, got %d", idx.upserted[0].AvailableCopies)
	}
}

func TestHandleEvent_BookDeleted(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.deleted",
		"book_id": "abc-123"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.deleted) != 1 {
		t.Fatalf("expected 1 delete, got %d", len(idx.deleted))
	}
	if idx.deleted[0] != "abc-123" {
		t.Errorf("expected deleted ID 'abc-123', got %s", idx.deleted[0])
	}
}

func TestHandleEvent_UnknownType(t *testing.T) {
	idx := &mockIndexer{}
	err := handleEvent(context.Background(), idx, []byte(`{
		"event_type": "book.unknown",
		"book_id": "abc-123"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.upserted) != 0 || len(idx.deleted) != 0 {
		t.Error("expected no operations for unknown event type")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./services/search/internal/consumer/... -v -count=1`
Expected: FAIL

- [ ] **Step 3: Create consumer implementation**

Create `services/search/internal/consumer/consumer.go`:

```go
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// Indexer is the subset of the search service the consumer needs.
type Indexer interface {
	Upsert(ctx context.Context, doc model.BookDocument) error
	Delete(ctx context.Context, id string) error
}

type bookEvent struct {
	EventType       string `json:"event_type"`
	BookID          string `json:"book_id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	ISBN            string `json:"isbn"`
	Genre           string `json:"genre"`
	Description     string `json:"description"`
	PublishedYear   int    `json:"published_year"`
	TotalCopies     int    `json:"total_copies"`
	AvailableCopies int    `json:"available_copies"`
}

// Run starts a Kafka consumer group that processes catalog book change events.
// It blocks until ctx is cancelled.
func Run(ctx context.Context, brokers []string, topic string, idx Indexer) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, "search-indexer", config)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer group.Close()

	handler := &consumerHandler{idx: idx}

	for {
		if err := group.Consume(ctx, []string{topic}, handler); err != nil {
			log.Printf("consumer error: %v", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

type consumerHandler struct {
	idx Indexer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	for msg := range claim.Messages() {
		if err := handleEvent(ctx, h.idx, msg.Value); err != nil {
			log.Printf("failed to handle event: %v", err)
			continue
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

func handleEvent(ctx context.Context, idx Indexer, data []byte) error {
	var event bookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	switch event.EventType {
	case "book.created", "book.updated":
		doc := model.BookDocument{
			ID:              event.BookID,
			Title:           event.Title,
			Author:          event.Author,
			ISBN:            event.ISBN,
			Genre:           event.Genre,
			Description:     event.Description,
			PublishedYear:   event.PublishedYear,
			TotalCopies:     event.TotalCopies,
			AvailableCopies: event.AvailableCopies,
		}
		return idx.Upsert(ctx, doc)
	case "book.deleted":
		return idx.Delete(ctx, event.BookID)
	default:
		log.Printf("unknown event type: %s", event.EventType)
		return nil
	}
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./services/search/internal/consumer/... -v -count=1`
Expected: PASS — all 4 tests pass

- [ ] **Step 5: Commit**

```bash
git add services/search/internal/consumer/
git commit -m "feat: add search Kafka consumer for catalog book events"
```

---

### Task 8: Search Bootstrap & Main

**Files:**
- Create: `services/search/internal/bootstrap/bootstrap.go`
- Create: `services/search/internal/bootstrap/bootstrap_test.go`
- Create: `services/search/cmd/main.go`

- [ ] **Step 1: Write bootstrap tests**

Create `services/search/internal/bootstrap/bootstrap_test.go`:

```go
package bootstrap_test

import (
	"context"
	"testing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/bootstrap"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
	"google.golang.org/grpc"
)

type mockSearchService struct {
	ensured  bool
	count    int64
	upserted []model.BookDocument
}

func (m *mockSearchService) EnsureIndex(_ context.Context) error {
	m.ensured = true
	return nil
}

func (m *mockSearchService) Count(_ context.Context) (int64, error) {
	return m.count, nil
}

func (m *mockSearchService) Upsert(_ context.Context, doc model.BookDocument) error {
	m.upserted = append(m.upserted, doc)
	return nil
}

type mockCatalogClient struct {
	catalogv1.CatalogServiceClient
	books []*catalogv1.Book
}

func (m *mockCatalogClient) ListBooks(_ context.Context, req *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
	return &catalogv1.ListBooksResponse{Books: m.books, TotalCount: int64(len(m.books))}, nil
}

func TestBootstrap_SkipsWhenIndexHasDocuments(t *testing.T) {
	svc := &mockSearchService{count: 5}
	catalog := &mockCatalogClient{}

	err := bootstrap.Run(context.Background(), catalog, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.ensured {
		t.Error("expected EnsureIndex to be called")
	}
	if len(svc.upserted) != 0 {
		t.Errorf("expected no upserts when index is populated, got %d", len(svc.upserted))
	}
}

func TestBootstrap_IndexesAllBooksWhenEmpty(t *testing.T) {
	svc := &mockSearchService{count: 0}
	catalog := &mockCatalogClient{
		books: []*catalogv1.Book{
			{Id: "1", Title: "Go Book", Author: "Author1"},
			{Id: "2", Title: "Rust Book", Author: "Author2"},
		},
	}

	err := bootstrap.Run(context.Background(), catalog, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.upserted) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(svc.upserted))
	}
	if svc.upserted[0].Title != "Go Book" {
		t.Errorf("expected first book 'Go Book', got %s", svc.upserted[0].Title)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

Run: `go test ./services/search/internal/bootstrap/... -v -count=1`
Expected: FAIL — bootstrap package doesn't exist

- [ ] **Step 3: Create bootstrap**

The bootstrap function depends on an `IndexBootstrapper` interface (subset of SearchService) rather than the concrete type, enabling testability.

Create `services/search/internal/bootstrap/bootstrap.go`:

```go
package bootstrap

import (
	"context"
	"log"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// IndexBootstrapper is the subset of SearchService that bootstrap needs.
type IndexBootstrapper interface {
	EnsureIndex(ctx context.Context) error
	Count(ctx context.Context) (int64, error)
	Upsert(ctx context.Context, doc model.BookDocument) error
}

// Run syncs the Meilisearch index from the Catalog service if the index is empty.
func Run(ctx context.Context, catalog catalogv1.CatalogServiceClient, svc IndexBootstrapper) error {
	if err := svc.EnsureIndex(ctx); err != nil {
		return err
	}

	count, err := svc.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("search index already has %d documents, skipping bootstrap", count)
		return nil
	}

	log.Println("search index is empty, bootstrapping from catalog...")

	var page int32 = 1
	var total int64

	for {
		resp, err := catalog.ListBooks(ctx, &catalogv1.ListBooksRequest{
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return err
		}

		for _, b := range resp.Books {
			doc := model.BookDocument{
				ID:              b.Id,
				Title:           b.Title,
				Author:          b.Author,
				ISBN:            b.Isbn,
				Genre:           b.Genre,
				Description:     b.Description,
				PublishedYear:   int(b.PublishedYear),
				TotalCopies:     int(b.TotalCopies),
				AvailableCopies: int(b.AvailableCopies),
			}
			if err := svc.Upsert(ctx, doc); err != nil {
				log.Printf("failed to index book %s: %v", b.Id, err)
			}
			total++
		}

		if total%100 == 0 && total > 0 {
			log.Printf("bootstrap progress: %d books indexed", total)
		}

		if len(resp.Books) < 100 {
			break
		}
		page++
	}

	log.Printf("bootstrap complete: %d books indexed", total)
	return nil
}
```

- [ ] **Step 4: Run bootstrap tests — expect pass**

Run: `go test ./services/search/internal/bootstrap/... -v -count=1`
Expected: PASS — all 2 tests pass

- [ ] **Step 5: Create search service main.go**

Create `services/search/cmd/main.go`:

```go
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/bootstrap"
	"github.com/fesoliveira014/library-system/services/search/internal/consumer"
	"github.com/fesoliveira014/library-system/services/search/internal/handler"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/service"
)

func main() {
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50054"
	}
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = "http://localhost:7700"
	}
	meiliKey := os.Getenv("MEILI_MASTER_KEY")
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}

	idx := index.NewMeilisearchIndex(meiliURL, meiliKey)
	searchSvc := service.NewSearchService(idx)

	// Bootstrap: connect to catalog and sync if index is empty
	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := bootstrap.Run(ctx, catalogClient, searchSvc); err != nil {
		log.Printf("bootstrap failed (starting with empty index): %v", err)
	}

	// Start Kafka consumer
	if kafkaBrokers != "" {
		brokers := strings.Split(kafkaBrokers, ",")
		go func() {
			log.Println("starting kafka consumer for catalog.books.changed topic")
			if err := consumer.Run(ctx, brokers, "catalog.books.changed", searchSvc); err != nil {
				log.Printf("kafka consumer error: %v", err)
			}
		}()
	}

	// Start gRPC server
	searchHandler := handler.NewSearchHandler(searchSvc)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	searchv1.RegisterSearchServiceServer(grpcServer, searchHandler)
	reflection.Register(grpcServer)

	log.Printf("search service listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```

- [ ] **Step 6: Run go mod tidy and verify build**

Run:
```bash
cd services/search && go mod tidy && cd ../..
go build -o /tmp/search-svc ./services/search/cmd/...
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add services/search/internal/bootstrap/ services/search/cmd/ services/search/go.mod services/search/go.sum
git commit -m "feat: add search service bootstrap and main.go"
```

---

### Task 9: Gateway Changes — Server, Search Handlers, Templates

**Files:**
- Modify: `services/gateway/internal/handler/server.go`
- Create: `services/gateway/internal/handler/search.go`
- Create: `services/gateway/internal/handler/search_test.go`
- Create: `services/gateway/templates/search.html`
- Create: `services/gateway/templates/partials/suggestions.html`
- Modify: `services/gateway/templates/partials/nav.html`
- Modify: `services/gateway/cmd/main.go`
- Modify: `services/gateway/internal/handler/auth_test.go` (8 occurrences)
- Modify: `services/gateway/internal/handler/catalog_test.go` (11 occurrences)
- Modify: `services/gateway/internal/handler/reservation_test.go` (6 occurrences)
- Modify: `services/gateway/internal/handler/render_test.go` (2 occurrences)
- Modify: `services/gateway/internal/handler/health_test.go` (1 occurrence)

- [ ] **Step 1: Update server.go**

In `services/gateway/internal/handler/server.go`, add the search import and field:

Add import:
```go
searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
```

Update the `Server` struct to add the `search` field:
```go
type Server struct {
	auth        authv1.AuthServiceClient
	catalog     catalogv1.CatalogServiceClient
	reservation reservationv1.ReservationServiceClient
	search      searchv1.SearchServiceClient
	tmpl        map[string]*template.Template
	baseTmpl    *template.Template
}
```

Update `New()` to accept the search client:
```go
func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, reservation reservationv1.ReservationServiceClient, search searchv1.SearchServiceClient, tmpl map[string]*template.Template) *Server {
	var base *template.Template
	for _, t := range tmpl {
		base = t
		break
	}
	return &Server{auth: auth, catalog: catalog, reservation: reservation, search: search, tmpl: tmpl, baseTmpl: base}
}
```

- [ ] **Step 2: Update all existing test files**

In `services/gateway/internal/handler/auth_test.go`, replace all 8 occurrences of `handler.New(` — each gets an additional `nil` parameter inserted before `tmpl`:
- `handler.New(nil, nil, nil, tmpl)` → `handler.New(nil, nil, nil, nil, tmpl)`
- `handler.New(mock, nil, nil, tmpl)` → `handler.New(mock, nil, nil, nil, tmpl)`
- `handler.New(nil, nil, nil, nil)` → `handler.New(nil, nil, nil, nil, nil)`

In `services/gateway/internal/handler/catalog_test.go`, replace all 11 occurrences:
- `handler.New(nil, mock, nil, tmpl)` → `handler.New(nil, mock, nil, nil, tmpl)`
- `handler.New(nil, &mockCatalogClient{}, nil, tmpl)` → `handler.New(nil, &mockCatalogClient{}, nil, nil, tmpl)`

In `services/gateway/internal/handler/reservation_test.go`, replace all 6 occurrences:
- `handler.New(nil, nil, nil, tmpl)` → `handler.New(nil, nil, nil, nil, tmpl)`
- `handler.New(nil, nil, mock, tmpl)` → `handler.New(nil, nil, mock, nil, tmpl)`

In `services/gateway/internal/handler/render_test.go`, replace all 2 occurrences:
- `handler.New(nil, nil, nil, tmpl)` → `handler.New(nil, nil, nil, nil, tmpl)`

In `services/gateway/internal/handler/health_test.go`, replace the 1 occurrence:
- `handler.New(nil, nil, nil, nil)` → `handler.New(nil, nil, nil, nil, nil)`

- [ ] **Step 3: Write search handler tests**

Create `services/gateway/internal/handler/search_test.go`:

```go
package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

type mockSearchClient struct {
	searchv1.SearchServiceClient
	searchResp  *searchv1.SearchResponse
	suggestResp *searchv1.SuggestResponse
	err         error
}

func (m *mockSearchClient) Search(_ context.Context, _ *searchv1.SearchRequest, _ ...grpc.CallOption) (*searchv1.SearchResponse, error) {
	return m.searchResp, m.err
}

func (m *mockSearchClient) Suggest(_ context.Context, _ *searchv1.SuggestRequest, _ ...grpc.CallOption) (*searchv1.SuggestResponse, error) {
	return m.suggestResp, m.err
}

func TestSearchPage_Success(t *testing.T) {
	mock := &mockSearchClient{
		searchResp: &searchv1.SearchResponse{
			Books: []*searchv1.BookResult{
				{Id: "1", Title: "Go Book", Author: "Author"},
			},
			TotalHits:   1,
			QueryTimeMs: 2,
		},
	}
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, mock, tmpl)

	req := httptest.NewRequest("GET", "/search?q=go", nil)
	w := httptest.NewRecorder()
	srv.SearchPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestSearchPage_EmptyQuery(t *testing.T) {
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()
	srv.SearchPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty query (shows empty form), got %d", w.Code)
	}
}

func TestSearchSuggest_Success(t *testing.T) {
	mock := &mockSearchClient{
		suggestResp: &searchv1.SuggestResponse{
			Suggestions: []*searchv1.Suggestion{
				{BookId: "1", Title: "Go in Action", Author: "Kennedy"},
			},
		},
	}
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, mock, tmpl)

	req := httptest.NewRequest("GET", "/search/suggest?prefix=go", nil)
	w := httptest.NewRecorder()
	srv.SearchSuggest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSearchSuggest_ShortPrefix(t *testing.T) {
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest("GET", "/search/suggest?prefix=g", nil)
	w := httptest.NewRecorder()
	srv.SearchSuggest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (empty partial), got %d", w.Code)
	}
}
```

- [ ] **Step 4: Create search handler**

Create `services/gateway/internal/handler/search.go`:

```go
package handler

import (
	"net/http"
	"strconv"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
)

func (s *Server) SearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	genre := r.URL.Query().Get("genre")
	author := r.URL.Query().Get("author")
	available := r.URL.Query().Get("available")
	pageStr := r.URL.Query().Get("page")

	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}

	data := map[string]any{
		"Query":   query,
		"Genre":   genre,
		"Author":  author,
		"Available": available == "on",
		"Page":    page,
	}

	if query == "" {
		s.render(w, r, "search.html", data)
		return
	}

	resp, err := s.search.Search(r.Context(), &searchv1.SearchRequest{
		Query:         query,
		Genre:         genre,
		Author:        author,
		AvailableOnly: available == "on",
		Page:          int32(page),
		PageSize:      20,
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Search failed")
		return
	}

	data["Books"] = resp.Books
	data["TotalHits"] = resp.TotalHits
	data["QueryTimeMs"] = resp.QueryTimeMs
	data["HasResults"] = len(resp.Books) > 0

	s.render(w, r, "search.html", data)
}

func (s *Server) SearchSuggest(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if len(prefix) < 2 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	resp, err := s.search.Suggest(r.Context(), &searchv1.SuggestRequest{
		Prefix: prefix,
		Limit:  5,
	})
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	s.renderPartial(w, "suggestions.html", resp.Suggestions)
}
```

- [ ] **Step 5: Create search template**

Create `services/gateway/templates/search.html`:

```html
{{define "title"}}Search{{end}}
{{define "content"}}
<h1>Search Books</h1>
<form method="GET" action="/search">
    <input type="text" name="q" value="{{.Data.Query}}" placeholder="Search books...">
    <input type="text" name="genre" value="{{.Data.Genre}}" placeholder="Genre">
    <input type="text" name="author" value="{{.Data.Author}}" placeholder="Author">
    <label><input type="checkbox" name="available" {{if .Data.Available}}checked{{end}}> Available only</label>
    <button type="submit">Search</button>
</form>

{{if .Data.HasResults}}
<p>{{.Data.TotalHits}} results in {{.Data.QueryTimeMs}}ms</p>
<table>
    <thead>
        <tr>
            <th>Title</th>
            <th>Author</th>
            <th>Genre</th>
            <th>Available</th>
        </tr>
    </thead>
    <tbody>
        {{range .Data.Books}}
        <tr>
            <td><a href="/books/{{.Id}}">{{.Title}}</a></td>
            <td>{{.Author}}</td>
            <td>{{.Genre}}</td>
            <td>{{.AvailableCopies}} / {{.TotalCopies}}</td>
        </tr>
        {{end}}
    </tbody>
</table>
{{else if .Data.Query}}
<p>No results found for "{{.Data.Query}}".</p>
{{end}}
{{end}}
```

- [ ] **Step 6: Create suggestions partial**

Create `services/gateway/templates/partials/suggestions.html`:

```html
{{define "suggestions.html"}}
{{range .}}
<a href="/books/{{.BookId}}" class="suggestion">
    <strong>{{.Title}}</strong> — {{.Author}}
</a>
{{end}}
{{end}}
```

- [ ] **Step 7: Update nav.html with search input**

The nav input uses `name="prefix"` for HTMX suggest calls. Form submission to `/search?q=...` uses a separate hidden input populated by JavaScript on submit, keeping both endpoints clean.

Replace `services/gateway/templates/partials/nav.html` with:

```html
{{define "nav"}}
<nav>
    <a href="/">Library System</a>
    <a href="/books">Catalog</a>
    <a href="/search">Search</a>
    <div style="display:inline;position:relative">
        <form method="GET" action="/search" style="display:inline">
            <input type="hidden" name="q" id="nav-search-q">
            <input type="text" name="prefix" placeholder="Search..."
                   hx-get="/search/suggest" hx-trigger="keyup changed delay:300ms[this.value.length >= 2]"
                   hx-target="#suggestions" hx-swap="innerHTML"
                   autocomplete="off"
                   onchange="document.getElementById('nav-search-q').value=this.value"
                   onkeydown="if(event.key==='Enter'){document.getElementById('nav-search-q').value=this.value}">
        </form>
        <div id="suggestions" style="position:absolute;background:white;z-index:10"></div>
    </div>
    {{if .User}}
        <a href="/reservations">My Reservations</a>
        {{if eq .User.Role "admin"}}
            <a href="/admin/books/new">Add Book</a>
        {{end}}
        <form method="POST" action="/logout" style="display:inline">
            <button type="submit">Logout</button>
        </form>
    {{else}}
        <a href="/login">Login</a>
        <a href="/register">Register</a>
    {{end}}
</nav>
{{end}}
```

- [ ] **Step 8: Update gateway main.go**

In `services/gateway/cmd/main.go`, add the search gRPC connection and routes:

Add import:
```go
searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
```

After the reservation connection block, add:
```go
	searchAddr := os.Getenv("SEARCH_GRPC_ADDR")
	if searchAddr == "" {
		searchAddr = "localhost:50054"
	}
	searchConn, err := grpc.NewClient(searchAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to search service: %v", err)
	}
	defer searchConn.Close()
```

Update the client construction:
```go
	searchClient := searchv1.NewSearchServiceClient(searchConn)
	srv := handler.New(authClient, catalogClient, reservationClient, searchClient, tmpl)
```

Add routes after the reservation routes:
```go
	mux.HandleFunc("GET /search", srv.SearchPage)
	mux.HandleFunc("GET /search/suggest", srv.SearchSuggest)
```

- [ ] **Step 9: Run all gateway tests**

Run: `go test ./services/gateway/... -v -count=1 -short`
Expected: PASS — all existing tests + new search tests pass

- [ ] **Step 10: Commit**

```bash
git add services/gateway/
git commit -m "feat: add search page and autocomplete to gateway"
```

---

### Task 10: Docker & Infrastructure

**Files:**
- Create: `services/search/Dockerfile`
- Create: `services/search/Dockerfile.dev`
- Create: `services/search/.air.toml`
- Modify: `deploy/.env`
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/docker-compose.dev.yml`

- [ ] **Step 1: Create search Dockerfile**

Create `services/search/Dockerfile`:

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service and gen/, not all
# workspace members. The replace directive in go.mod handles the gen/ import.
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY services/search/go.mod services/search/go.sum* ./services/search/

# 2. Download dependencies
WORKDIR /app/services/search
RUN go mod download

# 3. Copy source code
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY services/search/ ./services/search/

# 4. Build static binary
WORKDIR /app/services/search
RUN CGO_ENABLED=0 go build -o /bin/search ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/search /usr/local/bin/search
EXPOSE 50054
ENTRYPOINT ["/usr/local/bin/search"]
```

- [ ] **Step 2: Create search Dockerfile.dev**

Create `services/search/Dockerfile.dev`:

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

ENV GOWORK=off

COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY services/search/ ./services/search/

WORKDIR /app/services/search
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

- [ ] **Step 3: Create search .air.toml**

Create `services/search/.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor"]
  include_ext = ["go"]
  kill_delay = "0s"

[log]
  time = false

[misc]
  clean_on_exit = true
```

- [ ] **Step 4: Update deploy/.env**

Append to `deploy/.env`:

```
MEILI_URL=http://meilisearch:7700
MEILI_MASTER_KEY=dev-master-key-change-in-production
MEILI_PORT=7700
SEARCH_GRPC_ADDR=search:50054
SEARCH_GRPC_PORT=50054
```

- [ ] **Step 5: Update deploy/docker-compose.yml**

Add `meilisearch` and `search` services. Update `gateway` to include `SEARCH_GRPC_ADDR` and depend on `search`. Add `meilisearch-data` volume.

The full updated `deploy/docker-compose.yml` should contain all existing services plus:

```yaml
  meilisearch:
    image: getmeili/meilisearch:v1.12
    environment:
      MEILI_ENV: development
      MEILI_NO_ANALYTICS: "true"
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-dev-master-key-change-in-production}
    ports:
      - "${MEILI_PORT:-7700}:7700"
    volumes:
      - meilisearch-data:/meili_data
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:7700/health"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - library-net

  search:
    build:
      context: ..
      dockerfile: services/search/Dockerfile
    environment:
      GRPC_PORT: "50054"
      MEILI_URL: ${MEILI_URL:-http://meilisearch:7700}
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-dev-master-key-change-in-production}
      KAFKA_BROKERS: ${KAFKA_BROKERS:-kafka:9092}
      CATALOG_GRPC_ADDR: ${CATALOG_GRPC_ADDR:-catalog:50052}
    ports:
      - "${SEARCH_GRPC_PORT:-50054}:50054"
    depends_on:
      meilisearch:
        condition: service_healthy
      kafka:
        condition: service_healthy
    networks:
      - library-net
```

Update `gateway` to add:
- Environment: `SEARCH_GRPC_ADDR: ${SEARCH_GRPC_ADDR:-search:50054}`
- depends_on: add `- search`

Add `meilisearch-data:` to the `volumes:` section.

- [ ] **Step 6: Update deploy/docker-compose.dev.yml**

Add the search dev override:

```yaml
  search:
    build:
      context: ..
      dockerfile: services/search/Dockerfile.dev
    volumes:
      - ../services/search:/app/services/search
      - ../gen:/app/gen
      - ../pkg/auth:/app/pkg/auth
```

- [ ] **Step 7: Commit**

```bash
git add services/search/Dockerfile services/search/Dockerfile.dev services/search/.air.toml deploy/
git commit -m "feat: add Docker infrastructure — Meilisearch, search service"
```

---

### Task 11: Earthfile Updates

**Files:**
- Create: `services/search/Earthfile`
- Modify: `Earthfile`

- [ ] **Step 1: Create search Earthfile**

Create `services/search/Earthfile`:

```
VERSION 0.8

FROM golang:1.26-alpine

WORKDIR /app

deps:
    COPY go.mod go.sum* ./
    COPY ../../gen/go.mod ../../gen/go.sum* ../gen/
    COPY ../../pkg/auth/go.mod ../../pkg/auth/go.sum* ../pkg/auth/
    ENV GOWORK=off
    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

src:
    FROM +deps
    COPY --dir cmd internal ./
    COPY ../../gen/ ../gen/
    COPY ../../pkg/auth/ ../pkg/auth/

lint:
    FROM +src
    RUN go vet ./...

test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... ./internal/consumer/... ./internal/bootstrap/... -v -count=1

build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/search ./cmd/
    SAVE ARTIFACT /bin/search

docker:
    FROM alpine:3.19
    COPY +build/search /usr/local/bin/search
    EXPOSE 50054
    ENTRYPOINT ["/usr/local/bin/search"]
    SAVE IMAGE search:latest
```

- [ ] **Step 2: Update root Earthfile**

Replace the root `Earthfile` with:

```
VERSION 0.8

ci:
    BUILD ./services/auth+lint
    BUILD ./services/auth+test
    BUILD ./services/catalog+lint
    BUILD ./services/catalog+test
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
    BUILD ./services/reservation+lint
    BUILD ./services/reservation+test
    BUILD ./services/search+lint
    BUILD ./services/search+test

lint:
    BUILD ./services/auth+lint
    BUILD ./services/catalog+lint
    BUILD ./services/gateway+lint
    BUILD ./services/reservation+lint
    BUILD ./services/search+lint

test:
    BUILD ./services/auth+test
    BUILD ./services/catalog+test
    BUILD ./services/gateway+test
    BUILD ./services/reservation+test
    BUILD ./services/search+test
```

- [ ] **Step 3: Commit**

```bash
git add Earthfile services/search/Earthfile
git commit -m "feat: add search to Earthfile"
```

---

### Task 12: Chapter 7 Documentation

**Files:**
- Create: `docs/src/ch07/index.md`
- Create: `docs/src/ch07/catalog-events.md`
- Create: `docs/src/ch07/search-service.md`
- Create: `docs/src/ch07/meilisearch.md`
- Create: `docs/src/ch07/search-ui.md`
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Create chapter 7 index**

Create `docs/src/ch07/index.md`:

```markdown
# Chapter 7: Full-Text Search — Meilisearch & Event-Driven Indexing

In this chapter we add full-text search to the library system. The Catalog service publishes `catalog.books.changed` events to Kafka whenever books are created, updated, or deleted. A new Search service consumes those events, maintains a Meilisearch index, and exposes Search and Suggest gRPC RPCs. The Gateway gets a search page with HTMX-powered autocomplete.

## What You'll Learn

- Publishing domain events from an existing service (Catalog → Kafka)
- Meilisearch fundamentals: indexes, searchable/filterable attributes, faceted search
- The meilisearch-go client library
- Bootstrap pattern: syncing state on startup when events are unavailable
- Building autocomplete with HTMX and server-rendered partials
- Adding a new service end-to-end (proto → index → service → handler → gateway)

## Architecture Overview

```
Catalog Service → Kafka "catalog.books.changed" → Search Consumer → Meilisearch

Gateway → Search Service (gRPC) → Meilisearch
```

## Sections

- [7.1 Catalog Event Publishing](./catalog-events.md) — Adding a Kafka producer to the Catalog service
- [7.2 Search Service](./search-service.md) — Building the Search service: index layer, service, handler
- [7.3 Meilisearch Integration](./meilisearch.md) — Meilisearch concepts, configuration, and the Go client
- [7.4 Search UI](./search-ui.md) — Gateway search page, autocomplete, Docker setup
```

- [ ] **Step 2: Create section stubs**

Create `docs/src/ch07/catalog-events.md`:
```markdown
# 7.1 Catalog Event Publishing

<!-- TODO: Write full content covering:
- Adding EventPublisher to existing service pattern
- BookEvent struct and event types (created, updated, deleted)
- Fire-and-forget publishing with logging
- UpdateAvailability refactoring to return full book state
- Kafka publisher implementation with sarama
- Testing with mock publisher
-->
```

Create `docs/src/ch07/search-service.md`:
```markdown
# 7.2 Search Service

<!-- TODO: Write full content covering:
- Proto definition (Search, Suggest RPCs)
- BookDocument model (Meilisearch document shape)
- IndexRepository interface (data access abstraction for Meilisearch)
- Service layer (thin delegation + pagination defaults)
- gRPC handler and error mapping
- Kafka consumer for catalog events
- Bootstrap pattern for initial index population
- Comparison to Java/Spring: @KafkaListener, ElasticSearch client
-->
```

Create `docs/src/ch07/meilisearch.md`:
```markdown
# 7.3 Meilisearch Integration

<!-- TODO: Write full content covering:
- Meilisearch vs Elasticsearch vs Algolia
- Indexes, documents, primary keys
- Searchable, filterable, and sortable attributes
- The meilisearch-go client library
- Filter string syntax
- Index configuration and EnsureIndex pattern
- Docker setup and healthchecks
-->
```

Create `docs/src/ch07/search-ui.md`:
```markdown
# 7.4 Search UI

<!-- TODO: Write full content covering:
- Search page with filters and pagination
- HTMX autocomplete pattern (hx-get, hx-trigger, hx-target)
- Server-rendered partials for suggestions
- Eventual consistency in search results
- Docker Compose setup with Meilisearch
- Testing the full flow
-->
```

- [ ] **Step 3: Update SUMMARY.md**

Append to `docs/src/SUMMARY.md`:

```markdown
- [Chapter 7: Full-Text Search](./ch07/index.md)
  - [7.1 Catalog Event Publishing](./ch07/catalog-events.md)
  - [7.2 Search Service](./ch07/search-service.md)
  - [7.3 Meilisearch Integration](./ch07/meilisearch.md)
  - [7.4 Search UI](./ch07/search-ui.md)
```

- [ ] **Step 4: Commit**

```bash
git add docs/src/ch07/ docs/src/SUMMARY.md
git commit -m "docs: add Chapter 7 documentation structure and stubs"
```
