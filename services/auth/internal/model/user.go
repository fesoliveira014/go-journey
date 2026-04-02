package model

import (
	"time"

	"github.com/google/uuid"
)

// User is the domain model for authentication.
type User struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Email         string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash  *string   `gorm:"type:varchar(255)"`
	Name          string    `gorm:"type:varchar(255);not null"`
	Role          string    `gorm:"type:varchar(20);not null;default:'user'"`
	OAuthProvider *string   `gorm:"column:oauth_provider;type:varchar(50)"`
	OAuthID       *string   `gorm:"column:oauth_id;type:varchar(255)"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
