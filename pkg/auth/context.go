package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	userIDKey contextKey = "auth_user_id"
	roleKey   contextKey = "auth_role"
)

// ContextWithUser returns a new context with user ID and role embedded.
func ContextWithUser(ctx context.Context, userID uuid.UUID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, roleKey, role)
	return ctx
}

// UserIDFromContext extracts the user ID from the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	v, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}
	return v, nil
}

// RoleFromContext extracts the role from the context.
func RoleFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(roleKey).(string)
	if !ok {
		return "", fmt.Errorf("role not found in context")
	}
	return v, nil
}

// RequireRole checks that the context user has the required role.
// Returns a gRPC PermissionDenied error if not.
func RequireRole(ctx context.Context, required string) error {
	role, err := RoleFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, "no role in context")
	}
	if role != required {
		return status.Errorf(codes.PermissionDenied, "requires %s role", required)
	}
	return nil
}
