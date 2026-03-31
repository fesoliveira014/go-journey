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

type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

func TestCatalogHandler_CreateBook_Success(t *testing.T) {
	svc := service.NewCatalogService(newInMemoryRepo(), &noopPublisher{})
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
	svc := service.NewCatalogService(newInMemoryRepo(), &noopPublisher{})
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
	svc := service.NewCatalogService(newInMemoryRepo(), &noopPublisher{})
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
	svc := service.NewCatalogService(newInMemoryRepo(), &noopPublisher{})
	h := handler.NewCatalogHandler(svc)

	_, err := h.GetBook(context.Background(), &catalogv1.GetBookRequest{
		Id: "not-a-uuid",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for bad UUID, got %v", st.Code())
	}
}
