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

// Update overwrites all mutable fields. Because proto3 cannot distinguish "field not
// sent" from "field set to zero value," this will set omitted fields to their zero
// values (empty string, 0). In production, use google.protobuf.FieldMask to send
// only the fields the client intended to change. We skip FieldMask for simplicity.
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
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
