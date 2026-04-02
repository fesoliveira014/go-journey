package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/auth/internal/model"
)

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	List(ctx context.Context) ([]*model.User, error)
}

// AuthService contains business logic for authentication.
type AuthService struct {
	repo      UserRepository
	jwtSecret string
	jwtExpiry time.Duration
}

// NewAuthService creates a new auth service.
func NewAuthService(repo UserRepository, jwtSecret, jwtExpiryStr string) *AuthService {
	expiry, err := time.ParseDuration(jwtExpiryStr)
	if err != nil {
		expiry = 24 * time.Hour
	}
	return &AuthService{
		repo:      repo,
		jwtSecret: jwtSecret,
		jwtExpiry: expiry,
	}
}

// Register creates a new user with email/password, hashes the password, and returns a JWT.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (string, *model.User, error) {
	if email == "" {
		return "", nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return "", nil, fmt.Errorf("password is required")
	}
	if name == "" {
		return "", nil, fmt.Errorf("name is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, fmt.Errorf("failed to hash password: %w", err)
	}
	hashStr := string(hash)

	user := &model.User{
		Email:        email,
		PasswordHash: &hashStr,
		Name:         name,
		Role:         "user",
	}

	created, err := s.repo.Create(ctx, user)
	if err != nil {
		return "", nil, err
	}

	token, err := pkgauth.GenerateToken(created.ID, created.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, created, nil
}

// Login authenticates a user by email/password and returns a JWT.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			// Don't leak whether the email exists
			return "", nil, model.ErrInvalidCredentials
		}
		return "", nil, err
	}

	if user.PasswordHash == nil {
		// OAuth-only user trying to log in with password
		return "", nil, model.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return "", nil, model.ErrInvalidCredentials
	}

	token, err := pkgauth.GenerateToken(user.ID, user.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, user, nil
}

// ValidateToken validates a JWT and returns the user ID and role.
func (s *AuthService) ValidateToken(_ context.Context, tokenString string) (uuid.UUID, string, error) {
	claims, err := pkgauth.ValidateToken(tokenString, s.jwtSecret)
	if err != nil {
		return uuid.Nil, "", model.ErrInvalidToken
	}
	return claims.UserID, claims.Role, nil
}

// GetUser retrieves a user by ID.
func (s *AuthService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

// ListUsers returns all users.
func (s *AuthService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.repo.List(ctx)
}

// FindOrCreateOAuthUser looks up a user by OAuth provider+ID, creating one if not found.
func (s *AuthService) FindOrCreateOAuthUser(ctx context.Context, provider, oauthID, email, name string) (string, *model.User, error) {
	// Try to find existing OAuth user
	user, err := s.repo.GetByOAuthID(ctx, provider, oauthID)
	if err == nil {
		// Existing user — issue token
		token, err := pkgauth.GenerateToken(user.ID, user.Role, s.jwtSecret, s.jwtExpiry)
		if err != nil {
			return "", nil, fmt.Errorf("failed to generate token: %w", err)
		}
		return token, user, nil
	}
	if !errors.Is(err, model.ErrUserNotFound) {
		return "", nil, fmt.Errorf("lookup oauth user: %w", err)
	}

	// Create new OAuth user (no password)
	providerStr := provider
	oauthIDStr := oauthID
	user = &model.User{
		Email:         email,
		Name:          name,
		Role:          "user",
		OAuthProvider: &providerStr,
		OAuthID:       &oauthIDStr,
	}

	created, err := s.repo.Create(ctx, user)
	if err != nil {
		return "", nil, err
	}

	token, err := pkgauth.GenerateToken(created.ID, created.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, created, nil
}
