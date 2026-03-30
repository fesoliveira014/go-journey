package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

// fakeHandler is a gRPC handler that records whether it was called.
func fakeHandler(ctx context.Context, req interface{}) (interface{}, error) {
	// Verify user ID and role are in context
	if _, err := auth.UserIDFromContext(ctx); err != nil {
		return nil, err
	}
	return "ok", nil
}

func TestInterceptor_ValidToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", secret, time.Hour)

	interceptor := auth.UnaryAuthInterceptor(secret, nil)

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, fakeHandler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %v", resp)
	}
}

func TestInterceptor_MissingToken(t *testing.T) {
	interceptor := auth.UnaryAuthInterceptor("secret", nil)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, fakeHandler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestInterceptor_InvalidToken(t *testing.T) {
	interceptor := auth.UnaryAuthInterceptor("secret", nil)

	md := metadata.Pairs("authorization", "Bearer invalid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, fakeHandler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestInterceptor_SkippedMethod(t *testing.T) {
	skip := []string{"/test.Service/Public"}
	interceptor := auth.UnaryAuthInterceptor("secret", skip)

	// No token, but method is skipped — should pass through
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "public", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Public"}, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "public" {
		t.Errorf("expected 'public', got %v", resp)
	}
}
