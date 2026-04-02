package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
)

// UserRepository implements the service.UserRepository interface using GORM.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new GORM-backed user repository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		if isDuplicateKeyError(err) {
			return nil, model.ErrDuplicateEmail
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "oauth_provider = ? AND oauth_id = ?", provider, oauthID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		if isDuplicateKeyError(err) {
			return nil, model.ErrDuplicateEmail
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	var users []*model.User
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
