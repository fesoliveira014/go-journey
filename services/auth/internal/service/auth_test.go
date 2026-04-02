package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
)

type mockUserRepo struct {
	users map[uuid.UUID]*model.User
}

func newMockRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[uuid.UUID]*model.User)}
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) (*model.User, error) {
	for _, u := range m.users {
		if u.Email == user.Email {
			return nil, model.ErrDuplicateEmail
		}
	}
	user.ID = uuid.New()
	m.users[user.ID] = user
	return user, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, model.ErrUserNotFound
}

func (m *mockUserRepo) GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error) {
	for _, u := range m.users {
		if u.OAuthProvider != nil && *u.OAuthProvider == provider && u.OAuthID != nil && *u.OAuthID == oauthID {
			return u, nil
		}
	}
	return nil, model.ErrUserNotFound
}

func (m *mockUserRepo) List(_ context.Context) ([]*model.User, error) {
	users := make([]*model.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if _, ok := m.users[user.ID]; !ok {
		return nil, model.ErrUserNotFound
	}
	m.users[user.ID] = user
	return user, nil
}

func TestAuthService_Register_Success(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	token, user, err := svc.Register(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", user.Email)
	}
	if user.Role != "user" {
		t.Errorf("expected role 'user', got %q", user.Role)
	}
	// Verify password was hashed
	if user.PasswordHash == nil {
		t.Fatal("expected password hash to be set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte("password123")); err != nil {
		t.Error("password hash doesn't match")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	if _, _, err := svc.Register(context.Background(), "dup@example.com", "pass1", "User 1"); err != nil {
		t.Fatalf("setup: failed to register first user: %v", err)
	}
	_, _, err := svc.Register(context.Background(), "dup@example.com", "pass2", "User 2")
	if !errors.Is(err, model.ErrDuplicateEmail) {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestAuthService_Register_EmptyPassword(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	_, _, err := svc.Register(context.Background(), "test@example.com", "", "Test")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestAuthService_Register_EmptyEmail(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	_, _, err := svc.Register(context.Background(), "", "password", "Test")
	if err == nil {
		t.Fatal("expected error for empty email")
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	// Register first
	if _, _, err := svc.Register(context.Background(), "login@example.com", "mypassword", "Login User"); err != nil {
		t.Fatalf("setup: failed to register user: %v", err)
	}

	// Login
	token, user, err := svc.Login(context.Background(), "login@example.com", "mypassword")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.Email != "login@example.com" {
		t.Errorf("expected email %q, got %q", "login@example.com", user.Email)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	if _, _, err := svc.Register(context.Background(), "wrong@example.com", "correct", "User"); err != nil {
		t.Fatalf("setup: failed to register user: %v", err)
	}
	_, _, err := svc.Login(context.Background(), "wrong@example.com", "incorrect")
	if !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_NonexistentUser(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	_, _, err := svc.Login(context.Background(), "nobody@example.com", "password")
	if !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateToken_Success(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	token, _, _ := svc.Register(context.Background(), "validate@example.com", "pass", "User")

	userID, role, err := svc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if userID == uuid.Nil {
		t.Error("expected non-nil user ID")
	}
	if role != "user" {
		t.Errorf("expected role 'user', got %q", role)
	}
}

func TestAuthService_ValidateToken_Invalid(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	_, _, err := svc.ValidateToken(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthService_GetUser_Success(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	_, user, _ := svc.Register(context.Background(), "getuser@example.com", "pass", "Get User")

	found, err := svc.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "getuser@example.com" {
		t.Errorf("expected email %q, got %q", "getuser@example.com", found.Email)
	}
}

func TestAuthService_ListUsers(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	// Register two users
	_, _, err := svc.Register(context.Background(), "list1@example.com", "pass1", "User One")
	require.NoError(t, err)
	_, _, err = svc.Register(context.Background(), "list2@example.com", "pass2", "User Two")
	require.NoError(t, err)

	users, err := svc.ListUsers(context.Background())
	require.NoError(t, err)
	assert.Len(t, users, 2)
}
