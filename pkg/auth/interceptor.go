package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryForwardAuthInterceptor returns a gRPC unary client interceptor that
// forwards the JWT token from the context (set by the HTTP auth middleware)
// as a "Bearer" authorization header in outgoing gRPC metadata.
func UnaryForwardAuthInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if token, ok := TokenFromContext(ctx); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryAuthInterceptor returns a gRPC unary server interceptor that validates
// JWT tokens from the "authorization" metadata header.
//
// skipMethods is a list of full gRPC method names (e.g., "/auth.v1.AuthService/Register")
// that bypass authentication.
func UnaryAuthInterceptor(jwtSecret string, skipMethods []string) grpc.UnaryServerInterceptor {
	skip := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = true
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip authentication for public methods
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader[0], " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		// Validate JWT
		claims, err := ValidateToken(parts[1], jwtSecret)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Parse user ID from claims
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user ID in token")
		}

		// Inject user info into context
		ctx = ContextWithUser(ctx, userID, claims.Role)
		return handler(ctx, req)
	}
}
