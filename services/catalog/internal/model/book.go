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
