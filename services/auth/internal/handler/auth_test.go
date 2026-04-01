package handler_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	"github.com/fesoliveira014/library-system/services/auth/internal/handler"
	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"

	"github.com/google/uuid"
)

// inMemoryRepo implements service.UserRepository for handler tests.
type inMemoryRepo struct {
	users map[uuid.UUID]*model.User
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{users: make(map[uuid.UUID]*model.User)}
}

func (r *inMemoryRepo) Create(ctx context.Context, user *model.User) (*model.User, error) {
	for _, u := range r.users {
		if u.Email == user.Email {
			return nil, model.ErrDuplicateEmail
		}
	}
	user.ID = uuid.New()
	r.users[user.ID] = user
	return user, nil
}

func (r *inMemoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return u, nil
}

func (r *inMemoryRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, model.ErrUserNotFound
}

func (r *inMemoryRepo) GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error) {
	for _, u := range r.users {
		if u.OAuthProvider != nil && *u.OAuthProvider == provider && u.OAuthID != nil && *u.OAuthID == oauthID {
			return u, nil
		}
	}
	return nil, model.ErrUserNotFound
}

func (r *inMemoryRepo) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if _, ok := r.users[user.ID]; !ok {
		return nil, model.ErrUserNotFound
	}
	r.users[user.ID] = user
	return user, nil
}

func TestAuthHandler_Register_Success(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	resp, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.GetToken() == "" {
		t.Error("expected non-empty token")
	}
	if resp.GetUser().GetEmail() != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", resp.GetUser().GetEmail())
	}
}

func TestAuthHandler_Register_MissingEmail(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	_, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Password: "password",
		Name:     "User",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestAuthHandler_Register_MissingPassword(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	_, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "test@example.com",
		Name:  "User",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	// Register
	if _, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "login@example.com", Password: "pass123", Name: "User",
	}); err != nil {
		t.Fatalf("setup: failed to register user: %v", err)
	}

	// Login
	resp, err := h.Login(context.Background(), &authv1.LoginRequest{
		Email: "login@example.com", Password: "pass123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.GetToken() == "" {
		t.Error("expected non-empty token")
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	if _, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "wrong@example.com", Password: "correct", Name: "User",
	}); err != nil {
		t.Fatalf("setup: failed to register user: %v", err)
	}

	_, err := h.Login(context.Background(), &authv1.LoginRequest{
		Email: "wrong@example.com", Password: "incorrect",
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestAuthHandler_GetUser_InvalidID(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	_, err := h.GetUser(context.Background(), &authv1.GetUserRequest{Id: "not-a-uuid"})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}
