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
	tokenKey  contextKey = "auth_token"
)

// ContextWithUser returns a new context with user ID and role embedded.
func ContextWithUser(ctx context.Context, userID uuid.UUID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, roleKey, role)
	return ctx
}

// ContextWithToken returns a new context with the raw JWT token embedded.
func ContextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFromContext extracts the raw JWT token from the context.
func TokenFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tokenKey).(string)
	return v, ok
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
