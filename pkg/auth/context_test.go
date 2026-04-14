package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

func TestUserIDFromContext(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	ctx := auth.ContextWithUser(context.Background(), id, "user")

	got, err := auth.UserIDFromContext(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != id {
		t.Errorf("expected %s, got %s", id, got)
	}
}

func TestUserIDFromContext_Missing(t *testing.T) {
	t.Parallel()
	_, err := auth.UserIDFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error for missing user ID")
	}
}

func TestRoleFromContext(t *testing.T) {
	t.Parallel()
	ctx := auth.ContextWithUser(context.Background(), uuid.New(), "admin")

	role, err := auth.RoleFromContext(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if role != "admin" {
		t.Errorf("expected %q, got %q", "admin", role)
	}
}

func TestRequireRole_Authorized(t *testing.T) {
	t.Parallel()
	ctx := auth.ContextWithUser(context.Background(), uuid.New(), "admin")
	if err := auth.RequireRole(ctx, "admin"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequireRole_Unauthorized(t *testing.T) {
	t.Parallel()
	ctx := auth.ContextWithUser(context.Background(), uuid.New(), "user")
	err := auth.RequireRole(ctx, "admin")
	if err == nil {
		t.Fatal("expected error for unauthorized role")
	}
}
