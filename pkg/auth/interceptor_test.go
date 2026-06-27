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
	t.Parallel()
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
	t.Parallel()
	interceptor := auth.UnaryAuthInterceptor("secret", nil)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, fakeHandler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestInterceptor_InvalidToken(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestUnaryInternalTokenInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := auth.UnaryInternalTokenInterceptor("service-secret")

	err := interceptor(
		context.Background(),
		"/test.Service/Method",
		nil,
		nil,
		nil,
		func(ctx context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			got := md.Get("x-internal-service-token")
			if len(got) != 1 || got[0] != "service-secret" {
				t.Fatalf("expected internal service token, got %v", got)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireInternalToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{name: "valid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-service-token", "expected")), want: codes.OK},
		{name: "missing", ctx: context.Background(), want: codes.Unauthenticated},
		{name: "wrong", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-service-token", "wrong")), want: codes.PermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := auth.RequireInternalToken(tt.ctx, "expected")
			if tt.want == codes.OK {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("expected gRPC status error, got %v", err)
			}
			if st.Code() != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, st.Code())
			}
		})
	}
}
