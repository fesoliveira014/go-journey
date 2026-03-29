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
	// Check for duplicate ISBN
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

func TestCatalogService_CreateBook_Success(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo())
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
}

func TestCatalogService_CreateBook_MissingTitle(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo())
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
	svc := service.NewCatalogService(newMockRepo())
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
	svc := service.NewCatalogService(newMockRepo())
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: -1}
	_, err := svc.CreateBook(ctx, book)
	if !errors.Is(err, model.ErrInvalidBook) {
		t.Errorf("expected ErrInvalidBook for negative copies, got %v", err)
	}
}

func TestCatalogService_GetBook_NotFound(t *testing.T) {
	svc := service.NewCatalogService(newMockRepo())
	ctx := context.Background()

	_, err := svc.GetBook(ctx, uuid.New())
	if !errors.Is(err, model.ErrBookNotFound) {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestCatalogService_UpdateAvailability(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewCatalogService(repo)
	ctx := context.Background()

	book := &model.Book{Title: "Title", Author: "Author", TotalCopies: 5, AvailableCopies: 5}
	created, _ := svc.CreateBook(ctx, book)

	if err := svc.UpdateAvailability(ctx, created.ID, -1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	found, _ := svc.GetBook(ctx, created.ID)
	if found.AvailableCopies != 4 {
		t.Errorf("expected 4 available copies, got %d", found.AvailableCopies)
	}
}
