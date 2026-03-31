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
	// Note: Proto3 cannot distinguish "field not sent" from "field set to zero value."
	// We only validate fields that have non-zero values. This means you cannot clear
	// a field by sending an empty string — a known proto3 limitation. The production
	// solution is google.protobuf.FieldMask, which we skip for simplicity.
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
