package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
)

// BookRepository defines the interface for book persistence.
// The repository layer implements this; the service depends on the abstraction.
type BookRepository interface {
	Create(ctx context.Context, book *model.Book) (*model.Book, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error)
	Update(ctx context.Context, book *model.Book) (*model.Book, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error)
	UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error
}

// CatalogService contains business logic for managing the book catalog.
type CatalogService struct {
	repo BookRepository
}

// NewCatalogService creates a new catalog service with the given repository.
func NewCatalogService(repo BookRepository) *CatalogService {
	return &CatalogService{repo: repo}
}

func (s *CatalogService) CreateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if err := validateBook(book); err != nil {
		return nil, err
	}
	// Set available copies to total copies for new books
	book.AvailableCopies = book.TotalCopies
	return s.repo.Create(ctx, book)
}

func (s *CatalogService) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CatalogService) UpdateBook(ctx context.Context, book *model.Book) (*model.Book, error) {
	if book.Title != "" || book.Author != "" {
		if err := validateBook(book); err != nil {
			return nil, err
		}
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

// validateBook checks required fields and business rules.
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
