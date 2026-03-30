package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Reservation struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null"`
	BookID     uuid.UUID  `gorm:"type:uuid;not null"`
	Status     string     `gorm:"type:varchar(20);not null;default:'active'"`
	ReservedAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	DueAt      time.Time  `gorm:"type:timestamptz;not null"`
	ReturnedAt *time.Time `gorm:"type:timestamptz"`
}

const (
	StatusActive   = "active"
	StatusReturned = "returned"
	StatusExpired  = "expired"
)

var (
	ErrReservationNotFound = errors.New("reservation not found")
	ErrAlreadyReturned     = errors.New("reservation already returned or expired")
	ErrMaxReservations     = errors.New("maximum active reservations reached")
	ErrNoAvailableCopies   = errors.New("no available copies")
)
