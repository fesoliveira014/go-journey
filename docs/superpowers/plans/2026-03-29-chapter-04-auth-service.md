# Chapter 4: Auth Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an authentication service with email/password registration, JWT tokens, OAuth2 with Google, and a shared gRPC auth interceptor that protects endpoints across services.

**Architecture:** Layered handler → service → repository (same as catalog). Shared `pkg/auth/` module provides JWT utilities and a gRPC unary interceptor. Auth service gets its own PostgreSQL database (`postgres-auth`). The interceptor is added to both the auth service (protecting `GetUser`) and the catalog service (protecting write operations as admin-only).

**Tech Stack:** Go, gRPC, GORM, PostgreSQL, golang-jwt/jwt/v5, golang.org/x/crypto (bcrypt), golang.org/x/oauth2, golang-migrate, protobuf

**Spec:** `docs/superpowers/specs/2026-03-29-chapter-04-auth-service-design.md`

---

## File Structure

```
proto/auth/v1/
└── auth.proto                     # Auth service protobuf definition

gen/auth/v1/
├── auth.pb.go                     # Generated protobuf types
└── auth_grpc.pb.go                # Generated gRPC client/server

pkg/auth/
├── go.mod                         # Separate workspace module
├── jwt.go                         # JWT generation and validation
├── jwt_test.go                    # JWT unit tests
├── interceptor.go                 # gRPC unary auth interceptor
├── interceptor_test.go            # Interceptor unit tests
├── context.go                     # Context helpers (UserIDFromContext, etc.)
└── context_test.go                # Context helper tests

services/auth/
├── go.mod                         # Module with replace directives for gen/ and pkg/auth/
├── cmd/
│   └── main.go                    # DI wiring, migrations, gRPC server startup
├── internal/
│   ├── model/
│   │   ├── user.go                # Domain User struct with GORM tags
│   │   └── errors.go              # Domain error types
│   ├── repository/
│   │   ├── user.go                # GORM UserRepository implementation
│   │   └── user_test.go           # Integration tests (real PostgreSQL)
│   ├── service/
│   │   ├── auth.go                # Business logic, UserRepository interface, OAuth2
│   │   └── auth_test.go           # Service tests with mock repository
│   └── handler/
│       ├── auth.go                # gRPC handler, proto <-> domain conversion
│       └── auth_test.go           # Handler tests with in-memory repo
├── migrations/
│   ├── embed.go                   # //go:embed *.sql
│   ├── 000001_create_users.up.sql
│   └── 000001_create_users.down.sql
├── Dockerfile                     # Multi-stage production build
├── Dockerfile.dev                 # Air hot-reload dev build
└── .air.toml                      # Air configuration

deploy/
├── docker-compose.yml             # MODIFY: add postgres-auth + auth + auth-data volume
├── docker-compose.dev.yml         # MODIFY: add auth dev override
└── .env                           # MODIFY: add auth env vars

services/catalog/cmd/main.go       # MODIFY: add auth interceptor to gRPC server

go.work                            # MODIFY: add ./services/auth and ./pkg/auth

docs/src/
├── SUMMARY.md                     # MODIFY: add Chapter 4 entries
└── ch04/
    ├── index.md                   # Chapter overview
    ├── auth-fundamentals.md       # 4.1: bcrypt, JWT theory
    ├── auth-service.md            # 4.2: building the auth service
    ├── oauth2.md                  # 4.3: OAuth2 with Google
    └── interceptors.md            # 4.4: gRPC interceptors, protecting catalog
```

---

### Task 1: Proto Definition and Code Generation

**Context:** Define the auth service protobuf API and generate Go code. This is the foundation — all other tasks depend on the generated types and interfaces.

**Files:**
- Create: `proto/auth/v1/auth.proto`
- Generate: `gen/auth/v1/auth.pb.go`, `gen/auth/v1/auth_grpc.pb.go`

- [ ] **Step 1: Create the auth proto file**

Create `proto/auth/v1/auth.proto`:

```protobuf
syntax = "proto3";
package auth.v1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/fesoliveira014/library-system/gen/auth/v1;authv1";

service AuthService {
  rpc Register(RegisterRequest) returns (AuthResponse);
  rpc Login(LoginRequest) returns (AuthResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
  rpc GetUser(GetUserRequest) returns (User);
  rpc InitOAuth2(InitOAuth2Request) returns (InitOAuth2Response);
  rpc CompleteOAuth2(CompleteOAuth2Request) returns (AuthResponse);
}

message User {
  string id = 1;
  string email = 2;
  string name = 3;
  string role = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

message RegisterRequest {
  string email = 1;
  string password = 2;
  string name = 3;
}

message LoginRequest {
  string email = 1;
  string password = 2;
}

message AuthResponse {
  string token = 1;
  User user = 2;
}

message ValidateTokenRequest {
  string token = 1;
}

message ValidateTokenResponse {
  string user_id = 1;
  string role = 2;
}

message GetUserRequest {
  string id = 1;
}

message InitOAuth2Request {}

message InitOAuth2Response {
  string redirect_url = 1;
}

message CompleteOAuth2Request {
  string code = 1;
  string state = 2;
}
```

- [ ] **Step 2: Generate Go code from the proto**

Run from project root:

```bash
protoc --go_out=gen --go_opt=paths=source_relative \
       --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
       proto/auth/v1/auth.proto
```

Expected: Creates `gen/auth/v1/auth.pb.go` and `gen/auth/v1/auth_grpc.pb.go`.

- [ ] **Step 3: Verify generation**

```bash
ls gen/auth/v1/
```

Expected: `auth.pb.go  auth_grpc.pb.go`

- [ ] **Step 4: Run `go mod tidy` on gen module**

```bash
cd gen && go mod tidy
```

Expected: `gen/go.sum` updated with any new protobuf dependencies (likely no changes since catalog already uses the same protobuf/grpc versions).

- [ ] **Step 5: Commit**

```bash
git add proto/auth/v1/auth.proto gen/auth/v1/ gen/go.mod gen/go.sum
git commit -m "feat(auth): add auth service proto definition and generated code"
```

---

### Task 2: Shared Auth Library (`pkg/auth/`)

**Context:** The `pkg/auth/` module provides JWT utilities (generation, validation, claims parsing) and a gRPC unary interceptor. Any service can import it. This module is self-contained — it depends only on `golang-jwt/jwt/v5`, `google/uuid`, and `google.golang.org/grpc`. It does NOT import from `gen/`.

**Files:**
- Create: `pkg/auth/go.mod`
- Create: `pkg/auth/jwt.go`
- Create: `pkg/auth/jwt_test.go`
- Create: `pkg/auth/context.go`
- Create: `pkg/auth/context_test.go`
- Create: `pkg/auth/interceptor.go`
- Create: `pkg/auth/interceptor_test.go`
- Modify: `go.work` — add `./pkg/auth`

- [ ] **Step 1: Create `pkg/auth/go.mod`**

```
module github.com/fesoliveira014/library-system/pkg/auth

go 1.26.1

require (
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	google.golang.org/grpc v1.79.3
)
```

Then run:
```bash
cd pkg/auth && go mod tidy
```

- [ ] **Step 2: Add `./pkg/auth` to `go.work`**

Update `go.work` to include the new module:

```
go 1.26.1

use (
	./gen
	./pkg/auth
	./services/catalog
	./services/gateway
)
```

- [ ] **Step 3: Write JWT tests — `pkg/auth/jwt_test.go`**

```go
package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key"
	userID := uuid.New()
	role := "user"

	token, err := auth.GenerateToken(userID, role, secret, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := auth.ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, claims.UserID)
	}
	if claims.Role != role {
		t.Errorf("expected role %q, got %q", role, claims.Role)
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret-1", time.Hour)

	_, err := auth.ValidateToken(token, "secret-2")
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret", -time.Hour)

	_, err := auth.ValidateToken(token, "secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	_, err := auth.ValidateToken("not.a.jwt", "secret")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd pkg/auth && go test ./...
```

Expected: Compilation error — `auth.GenerateToken` and `auth.ValidateToken` not defined.

- [ ] **Step 5: Implement JWT — `pkg/auth/jwt.go`**

```go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims holds the JWT payload extracted after validation.
type Claims struct {
	UserID uuid.UUID
	Role   string
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT with the given user ID, role, secret, and expiry duration.
func GenerateToken(userID uuid.UUID, role, secret string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT string, returning the claims if valid.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd pkg/auth && go test ./... -v
```

Expected: All 4 tests pass.

- [ ] **Step 7: Write context helper tests — `pkg/auth/context_test.go`**

```go
package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

func TestUserIDFromContext(t *testing.T) {
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
	_, err := auth.UserIDFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error for missing user ID")
	}
}

func TestRoleFromContext(t *testing.T) {
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
	ctx := auth.ContextWithUser(context.Background(), uuid.New(), "admin")
	if err := auth.RequireRole(ctx, "admin"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequireRole_Unauthorized(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), uuid.New(), "user")
	err := auth.RequireRole(ctx, "admin")
	if err == nil {
		t.Fatal("expected error for unauthorized role")
	}
}
```

- [ ] **Step 8: Run tests to verify they fail**

```bash
cd pkg/auth && go test ./... -v
```

Expected: Compilation error — context helpers not defined.

- [ ] **Step 9: Implement context helpers — `pkg/auth/context.go`**

```go
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
```

- [ ] **Step 10: Run tests to verify they pass**

```bash
cd pkg/auth && go test ./... -v
```

Expected: All 9 tests pass (4 JWT + 5 context).

- [ ] **Step 11: Write interceptor tests — `pkg/auth/interceptor_test.go`**

```go
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
```

- [ ] **Step 12: Run tests to verify they fail**

```bash
cd pkg/auth && go test ./... -v
```

Expected: Compilation error — `auth.UnaryAuthInterceptor` not defined.

- [ ] **Step 13: Implement interceptor — `pkg/auth/interceptor.go`**

```go
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
```

- [ ] **Step 14: Run tests to verify they pass**

```bash
cd pkg/auth && go test ./... -v
```

Expected: All 13 tests pass (4 JWT + 5 context + 4 interceptor).

- [ ] **Step 15: Commit**

```bash
git add pkg/auth/ go.work
git commit -m "feat(auth): add shared pkg/auth library with JWT, interceptor, and context helpers"
```

---

### Task 3: Auth Service Foundation — Model, Migrations, Repository

**Context:** Build the persistence layer for the auth service. This follows the exact same pattern as the catalog service: domain model, embedded SQL migrations, GORM repository. The auth service gets its own `go.mod` with `replace` directives for both `gen/` and `pkg/auth/`.

**Files:**
- Create: `services/auth/go.mod`
- Create: `services/auth/internal/model/user.go`
- Create: `services/auth/internal/model/errors.go`
- Create: `services/auth/migrations/embed.go`
- Create: `services/auth/migrations/000001_create_users.up.sql`
- Create: `services/auth/migrations/000001_create_users.down.sql`
- Create: `services/auth/internal/repository/user.go`
- Create: `services/auth/internal/repository/user_test.go`
- Modify: `go.work` — add `./services/auth`

- [ ] **Step 1: Create `services/auth/go.mod`**

```
module github.com/fesoliveira014/library-system/services/auth

go 1.26.1

require (
	github.com/fesoliveira014/library-system/gen v0.0.0
	github.com/fesoliveira014/library-system/pkg/auth v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.39.0
	golang.org/x/oauth2 v0.30.0
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
)

replace (
	github.com/fesoliveira014/library-system/gen => ../../gen
	github.com/fesoliveira014/library-system/pkg/auth => ../../pkg/auth
)
```

Then run:
```bash
cd services/auth && go mod tidy
```

- [ ] **Step 2: Add `./services/auth` to `go.work`**

```
go 1.26.1

use (
	./gen
	./pkg/auth
	./services/auth
	./services/catalog
	./services/gateway
)
```

- [ ] **Step 3: Create domain model — `services/auth/internal/model/user.go`**

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

// User is the domain model for authentication.
type User struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Email         string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash  *string   `gorm:"type:varchar(255)"`
	Name          string    `gorm:"type:varchar(255);not null"`
	Role          string    `gorm:"type:varchar(20);not null;default:'user'"`
	OAuthProvider *string   `gorm:"type:varchar(50)"`
	OAuthID       *string   `gorm:"type:varchar(255)"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
```

- [ ] **Step 4: Create domain errors — `services/auth/internal/model/errors.go`**

```go
package model

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrDuplicateEmail     = errors.New("duplicate email")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrOAuthFailed        = errors.New("oauth2 authentication failed")
)
```

- [ ] **Step 5: Create migration embed — `services/auth/migrations/embed.go`**

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 6: Create migration up — `services/auth/migrations/000001_create_users.up.sql`**

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255),
    name            VARCHAR(255) NOT NULL,
    role            VARCHAR(20) NOT NULL DEFAULT 'user',
    oauth_provider  VARCHAR(50),
    oauth_id        VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_role CHECK (role IN ('user', 'admin')),
    CONSTRAINT oauth_unique UNIQUE (oauth_provider, oauth_id)
);

CREATE INDEX idx_users_email ON users(email);
```

- [ ] **Step 7: Create migration down — `services/auth/migrations/000001_create_users.down.sql`**

```sql
DROP TABLE IF EXISTS users;
```

- [ ] **Step 8: Write repository tests — `services/auth/internal/repository/user_test.go`**

```go
package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
	"github.com/fesoliveira014/library-system/services/auth/migrations"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5434 user=postgres password=postgres dbname=auth_test sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to PostgreSQL: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("failed to create migration driver: %v", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("failed to create migration source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}

	db.Exec("TRUNCATE TABLE users CASCADE")
	return db
}

func strPtr(s string) *string { return &s }

func TestUserRepository_Create(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: strPtr("$2a$10$hashedpassword"),
		Name:         "Test User",
		Role:         "user",
	}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected UUID to be set")
	}
	if created.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", created.Email)
	}
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user1 := &model.User{Email: "dup@example.com", PasswordHash: strPtr("hash"), Name: "User 1", Role: "user"}
	if _, err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	user2 := &model.User{Email: "dup@example.com", PasswordHash: strPtr("hash"), Name: "User 2", Role: "user"}
	_, err := repo.Create(ctx, user2)
	if err != model.ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "get@example.com", PasswordHash: strPtr("hash"), Name: "Get User", Role: "user"}
	created, _ := repo.Create(ctx, user)

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "get@example.com" {
		t.Errorf("expected email %q, got %q", "get@example.com", found.Email)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "email@example.com", PasswordHash: strPtr("hash"), Name: "Email User", Role: "user"}
	repo.Create(ctx, user)

	found, err := repo.GetByEmail(ctx, "email@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Name != "Email User" {
		t.Errorf("expected name %q, got %q", "Email User", found.Name)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByOAuthID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	provider := "google"
	oauthID := "google-123"
	user := &model.User{
		Email:         "oauth@example.com",
		Name:          "OAuth User",
		Role:          "user",
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
	}
	repo.Create(ctx, user)

	found, err := repo.GetByOAuthID(ctx, "google", "google-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "oauth@example.com" {
		t.Errorf("expected email %q, got %q", "oauth@example.com", found.Email)
	}
}

func TestUserRepository_GetByOAuthID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByOAuthID(ctx, "google", "nonexistent")
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "update@example.com", PasswordHash: strPtr("hash"), Name: "Original", Role: "user"}
	created, _ := repo.Create(ctx, user)

	created.Name = "Updated"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name %q, got %q", "Updated", updated.Name)
	}
}
```

- [ ] **Step 9: Run tests to verify they fail**

```bash
cd services/auth && go test ./internal/repository/... -v
```

Expected: Compilation error — `repository.NewUserRepository` not defined.

- [ ] **Step 10: Implement repository — `services/auth/internal/repository/user.go`**

```go
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

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
```

- [ ] **Step 11: Run tests**

```bash
cd services/auth && go test ./internal/repository/... -v
```

Expected: Tests pass if PostgreSQL is available on port 5434, or skip if not. Unit tests in model compile cleanly.

- [ ] **Step 12: Verify compilation of all modules**

```bash
cd /home/fesol/docs/go-journey && go build ./...
```

Expected: All workspace modules compile.

- [ ] **Step 13: Commit**

```bash
git add services/auth/go.mod services/auth/go.sum services/auth/internal/model/ services/auth/migrations/ services/auth/internal/repository/ go.work
git commit -m "feat(auth): add user model, migrations, and GORM repository"
```

---

### Task 4: Auth Service Logic — Service Layer

**Context:** The service layer contains all business logic: password hashing with bcrypt, JWT generation via `pkg/auth`, OAuth2 token exchange with Google, and the find-or-create user pattern. The `UserRepository` interface is defined here (dependency inversion). Tests use a mock repository.

**Files:**
- Create: `services/auth/internal/service/auth.go`
- Create: `services/auth/internal/service/auth_test.go`

- [ ] **Step 1: Write service tests — `services/auth/internal/service/auth_test.go`**

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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

	svc.Register(context.Background(), "dup@example.com", "pass1", "User 1")
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
	svc.Register(context.Background(), "login@example.com", "mypassword", "Login User")

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

	svc.Register(context.Background(), "wrong@example.com", "correct", "User")
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/auth && go test ./internal/service/... -v
```

Expected: Compilation error — `service.NewAuthService` not defined.

- [ ] **Step 3: Implement service — `services/auth/internal/service/auth.go`**

```go
package service

import (
	"context"
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
		// Don't leak whether the email exists
		return "", nil, model.ErrInvalidCredentials
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd services/auth && go test ./internal/service/... -v
```

Expected: All 10 tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/auth/internal/service/
git commit -m "feat(auth): add auth service layer with registration, login, and JWT validation"
```

---

### Task 5: Auth Service — gRPC Handler and Server Wiring

**Context:** The handler layer implements `authv1.AuthServiceServer`, translates proto types to domain types, and maps domain errors to gRPC status codes. The `cmd/main.go` wires everything together: DB connection, migrations, dependency injection, gRPC server with the auth interceptor (protecting `GetUser`). OAuth2 RPCs (`InitOAuth2`, `CompleteOAuth2`) are also wired here — they use `golang.org/x/oauth2` to exchange codes with Google.

**Files:**
- Create: `services/auth/internal/handler/auth.go`
- Create: `services/auth/internal/handler/auth_test.go`
- Create: `services/auth/cmd/main.go`

- [ ] **Step 1: Write handler tests — `services/auth/internal/handler/auth_test.go`**

```go
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
	h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "login@example.com", Password: "pass123", Name: "User",
	})

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

	h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "wrong@example.com", Password: "correct", Name: "User",
	})

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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/auth && go test ./internal/handler/... -v
```

Expected: Compilation error — `handler.NewAuthHandler` not defined.

- [ ] **Step 3: Implement handler — `services/auth/internal/handler/auth.go`**

```go
package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
)

// AuthHandler implements the generated authv1.AuthServiceServer interface.
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	svc         *service.AuthService
	oauthConfig *oauth2.Config
	states      map[string]time.Time
	mu          sync.Mutex
}

// NewAuthHandler creates a new gRPC handler backed by the given service.
func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{
		svc:    svc,
		states: make(map[string]time.Time),
	}
}

// NewAuthHandlerWithOAuth creates a handler with OAuth2 configuration.
func NewAuthHandlerWithOAuth(svc *service.AuthService, clientID, clientSecret, redirectURL string) *AuthHandler {
	h := NewAuthHandler(svc)
	if clientID != "" {
		h.oauthConfig = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}
	return h
}

func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	token, user, err := h.svc.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetName())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.AuthResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	token, user, err := h.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	userID, role, err := h.svc.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.ValidateTokenResponse{
		UserId: userID.String(),
		Role:   role,
	}, nil
}

func (h *AuthHandler) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.User, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := h.svc.GetUser(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return userToProto(user), nil
}

func (h *AuthHandler) InitOAuth2(ctx context.Context, req *authv1.InitOAuth2Request) (*authv1.InitOAuth2Response, error) {
	if h.oauthConfig == nil {
		return nil, status.Error(codes.Unavailable, "OAuth2 not configured")
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, status.Error(codes.Internal, "failed to generate state")
	}
	state := hex.EncodeToString(stateBytes)

	h.mu.Lock()
	h.states[state] = time.Now().Add(5 * time.Minute)
	now := time.Now()
	for k, v := range h.states {
		if now.After(v) {
			delete(h.states, k)
		}
	}
	h.mu.Unlock()

	url := h.oauthConfig.AuthCodeURL(state)
	return &authv1.InitOAuth2Response{RedirectUrl: url}, nil
}

func (h *AuthHandler) CompleteOAuth2(ctx context.Context, req *authv1.CompleteOAuth2Request) (*authv1.AuthResponse, error) {
	if h.oauthConfig == nil {
		return nil, status.Error(codes.Unavailable, "OAuth2 not configured")
	}
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}

	h.mu.Lock()
	expiry, ok := h.states[req.GetState()]
	if ok {
		delete(h.states, req.GetState())
	}
	h.mu.Unlock()

	if !ok || time.Now().After(expiry) {
		return nil, status.Error(codes.InvalidArgument, "invalid or expired state")
	}

	oauthToken, err := h.oauthConfig.Exchange(ctx, req.GetCode())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to exchange code: %v", err))
	}

	client := h.oauthConfig.Client(ctx, oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to fetch user info")
	}
	defer resp.Body.Close()

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, status.Error(codes.Internal, "failed to parse user info")
	}

	token, user, err := h.svc.FindOrCreateOAuthUser(ctx, "google", googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authv1.AuthResponse{
		Token: token,
		User:  userToProto(user),
	}, nil
}

func userToProto(u *model.User) *authv1.User {
	return &authv1.User{
		Id:        u.ID.String(),
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrDuplicateEmail):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, model.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, model.ErrOAuthFailed):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
```

- [ ] **Step 4: Run handler tests**

```bash
cd services/auth && go test ./internal/handler/... -v
```

Expected: All 6 handler tests pass.

- [ ] **Step 5: Create `services/auth/cmd/main.go`**

```go
package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/auth/internal/handler"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
	"github.com/fesoliveira014/library-system/services/auth/migrations"
)

func main() {
	// Configuration from environment
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "host=localhost port=5434 user=postgres password=postgres dbname=auth sslmode=disable"
	}
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	jwtExpiry := os.Getenv("JWT_EXPIRY")
	if jwtExpiry == "" {
		jwtExpiry = "24h"
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	// Connect to PostgreSQL via GORM
	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to PostgreSQL")

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	// Wire dependencies
	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, jwtSecret, jwtExpiry)
	authHandler := handler.NewAuthHandlerWithOAuth(authSvc, googleClientID, googleClientSecret, googleRedirectURL)

	// Start gRPC server with auth interceptor
	skipMethods := []string{
		"/auth.v1.AuthService/Register",
		"/auth.v1.AuthService/Login",
		"/auth.v1.AuthService/ValidateToken",
		"/auth.v1.AuthService/InitOAuth2",
		"/auth.v1.AuthService/CompleteOAuth2",
	}
	interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	authv1.RegisterAuthServiceServer(grpcServer, authHandler)
	reflection.Register(grpcServer)

	log.Printf("auth service listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
```

- [ ] **Step 6: Verify compilation**

```bash
cd services/auth && go build ./cmd/
```

Expected: Builds successfully.

- [ ] **Step 7: Run all auth service tests**

```bash
cd services/auth && go test ./... -v
```

Expected: Handler and service tests pass. Repository tests skip if no PostgreSQL.

- [ ] **Step 8: Commit**

```bash
git add services/auth/internal/handler/ services/auth/cmd/
git commit -m "feat(auth): add gRPC handler with OAuth2 and server wiring"
```

---

### Task 6: Docker Compose and Dockerfiles

**Context:** Add Docker support for the auth service, following the exact pattern from Chapter 3. Production Dockerfile uses multi-stage build with `GOWORK=off` (auth depends on both `gen/` and `pkg/auth/`). Dev Dockerfile uses air for hot-reload. Docker Compose gets `postgres-auth` and `auth` containers added.

**Files:**
- Create: `services/auth/Dockerfile`
- Create: `services/auth/Dockerfile.dev`
- Create: `services/auth/.air.toml`
- Modify: `deploy/docker-compose.yml` — add postgres-auth, auth, auth-data volume
- Modify: `deploy/docker-compose.dev.yml` — add auth dev override
- Modify: `deploy/.env` — add auth env vars

- [ ] **Step 1: Create `services/auth/Dockerfile`**

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service, gen/, and pkg/auth/
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/

# 2. Download dependencies (cached unless go.mod changes)
WORKDIR /app/services/auth
RUN go mod download

# 3. Copy source code (invalidates cache only when source changes)
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY services/auth/ ./services/auth/

# 4. Build static binary
WORKDIR /app/services/auth
RUN CGO_ENABLED=0 go build -o /bin/auth ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/auth /usr/local/bin/auth
EXPOSE 50051
ENTRYPOINT ["/usr/local/bin/auth"]
```

- [ ] **Step 2: Create `services/auth/Dockerfile.dev`**

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Disable workspace mode
ENV GOWORK=off

# Copy shared modules and service source
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY services/auth/ ./services/auth/

WORKDIR /app/services/auth
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

- [ ] **Step 3: Create `services/auth/.air.toml`**

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor"]
  include_ext = ["go"]
  kill_delay = "0s"

[log]
  time = false

[misc]
  clean_on_exit = true
```

- [ ] **Step 4: Add auth containers to `deploy/docker-compose.yml`**

Add `postgres-auth` and `auth` services. Add `auth-data` to the top-level `volumes:` block. The complete updated file should contain (in addition to existing services):

```yaml
  postgres-auth:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_AUTH_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_AUTH_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_AUTH_DB:-auth}
    ports:
      - "${POSTGRES_AUTH_PORT:-5434}:5432"
    volumes:
      - auth-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - library-net

  auth:
    build:
      context: ..
      dockerfile: services/auth/Dockerfile
    environment:
      DATABASE_URL: "host=postgres-auth port=5432 user=${POSTGRES_AUTH_USER:-postgres} password=${POSTGRES_AUTH_PASSWORD:-postgres} dbname=${POSTGRES_AUTH_DB:-auth} sslmode=disable"
      GRPC_PORT: "50051"
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}
      JWT_EXPIRY: ${JWT_EXPIRY:-24h}
      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID:-}
      GOOGLE_CLIENT_SECRET: ${GOOGLE_CLIENT_SECRET:-}
      GOOGLE_REDIRECT_URL: ${GOOGLE_REDIRECT_URL:-http://localhost:8080/oauth2/callback}
    ports:
      - "${AUTH_GRPC_PORT:-50051}:50051"
    depends_on:
      postgres-auth:
        condition: service_healthy
    networks:
      - library-net
```

Update volumes block:
```yaml
volumes:
  catalog-data:
  auth-data:
```

- [ ] **Step 5: Add auth dev override to `deploy/docker-compose.dev.yml`**

Add:
```yaml
  auth:
    build:
      context: ..
      dockerfile: services/auth/Dockerfile.dev
    volumes:
      - ../services/auth:/app/services/auth
      - ../gen:/app/gen
      - ../pkg/auth:/app/pkg/auth
```

- [ ] **Step 6: Update `deploy/.env`**

Append:
```env
POSTGRES_AUTH_PORT=5434
POSTGRES_AUTH_USER=postgres
POSTGRES_AUTH_PASSWORD=postgres
POSTGRES_AUTH_DB=auth

AUTH_GRPC_PORT=50051
JWT_SECRET=dev-secret-change-in-production
JWT_EXPIRY=24h
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:8080/oauth2/callback
```

- [ ] **Step 7: Verify Docker build**

```bash
cd /home/fesol/docs/go-journey && docker compose -f deploy/docker-compose.yml build auth
```

Expected: Builds successfully.

- [ ] **Step 8: Smoke test full stack**

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

Wait for all services to be healthy, then:
```bash
docker compose -f deploy/docker-compose.yml ps
```

Expected: All 5 containers running (postgres-catalog, catalog, gateway, postgres-auth, auth).

- [ ] **Step 9: Test auth service with grpcurl**

```bash
# Register a user
grpcurl -plaintext -d '{"email":"admin@library.com","password":"admin123","name":"Admin User"}' localhost:50051 auth.v1.AuthService/Register

# Login
grpcurl -plaintext -d '{"email":"admin@library.com","password":"admin123"}' localhost:50051 auth.v1.AuthService/Login

# Validate the token (use the token from login response)
grpcurl -plaintext -d '{"token":"<TOKEN_FROM_LOGIN>"}' localhost:50051 auth.v1.AuthService/ValidateToken
```

Expected: Register returns token + user. Login returns token + user. ValidateToken returns user_id + role.

- [ ] **Step 10: Stop containers**

```bash
docker compose -f deploy/docker-compose.yml down
```

- [ ] **Step 11: Commit**

```bash
git add services/auth/Dockerfile services/auth/Dockerfile.dev services/auth/.air.toml deploy/
git commit -m "feat(auth): add Docker support and Docker Compose integration"
```

---

### Task 7: Add Auth Interceptor to Catalog Service

**Context:** The catalog service needs the auth interceptor to protect write operations (CreateBook, UpdateBook, DeleteBook) as admin-only. Read operations (GetBook, ListBooks) and UpdateAvailability remain public. This task modifies the catalog's `cmd/main.go` to wire the interceptor and updates `services/catalog/go.mod` to depend on `pkg/auth`.

**Files:**
- Modify: `services/catalog/go.mod` — add `pkg/auth` dependency and `replace` directive
- Modify: `services/catalog/cmd/main.go` — add auth interceptor to gRPC server
- Modify: `services/catalog/internal/handler/catalog.go` — add `RequireRole` checks to write methods

- [ ] **Step 1: Add `pkg/auth` dependency to catalog's `go.mod`**

Add to `services/catalog/go.mod`:
- Require: `github.com/fesoliveira014/library-system/pkg/auth v0.0.0`
- Replace: `github.com/fesoliveira014/library-system/pkg/auth => ../../pkg/auth`

Then:
```bash
cd services/catalog && go mod tidy
```

- [ ] **Step 2: Update catalog's `cmd/main.go` to add interceptor**

Add to imports:
```go
pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
```

Add environment variable for JWT secret:
```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    jwtSecret = "dev-secret-change-in-production"
}
```

Add interceptor configuration before creating the gRPC server:
```go
// Methods that don't require authentication
skipMethods := []string{
    "/catalog.v1.CatalogService/GetBook",
    "/catalog.v1.CatalogService/ListBooks",
    "/catalog.v1.CatalogService/UpdateAvailability",
}
interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)
```

Change gRPC server creation from:
```go
grpcServer := grpc.NewServer()
```
to:
```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

- [ ] **Step 3: Add `RequireRole` checks to catalog handler write methods**

The interceptor handles authentication (valid token required) but not authorization (role checking). Add `RequireRole` calls to the catalog handler's write methods. In `services/catalog/internal/handler/catalog.go`:

Add import:
```go
pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
```

Add role check at the top of `CreateBook`, `UpdateBook`, and `DeleteBook`:
```go
func (h *CatalogHandler) CreateBook(ctx context.Context, req *catalogv1.CreateBookRequest) (*catalogv1.Book, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	// ... rest of existing code
```

```go
func (h *CatalogHandler) UpdateBook(ctx context.Context, req *catalogv1.UpdateBookRequest) (*catalogv1.Book, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	// ... rest of existing code
```

```go
func (h *CatalogHandler) DeleteBook(ctx context.Context, req *catalogv1.DeleteBookRequest) (*catalogv1.DeleteBookResponse, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	// ... rest of existing code
```

This ensures only admin users can create, update, or delete books.

- [ ] **Step 4: Add JWT_SECRET to catalog's Docker Compose environment**

In `deploy/docker-compose.yml`, add to the `catalog` service's environment:
```yaml
JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}
```

- [ ] **Step 5: Update catalog Dockerfile to copy `pkg/auth/`**

The catalog Dockerfile now depends on `pkg/auth/` in addition to `gen/`. Update `services/catalog/Dockerfile` to copy the pkg/auth module:

Add after the existing gen COPY lines:
```dockerfile
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
```

And in the source copy section:
```dockerfile
COPY pkg/auth/ ./pkg/auth/
```

- [ ] **Step 6: Update catalog dev Dockerfile**

Add `pkg/auth/` copy to `services/catalog/Dockerfile.dev`:
```dockerfile
COPY pkg/auth/ ./pkg/auth/
```

And add `pkg/auth` volume mount to `deploy/docker-compose.dev.yml` for catalog:
```yaml
  catalog:
    build:
      context: ..
      dockerfile: services/catalog/Dockerfile.dev
    volumes:
      - ../services/catalog:/app/services/catalog
      - ../gen:/app/gen
      - ../pkg/auth:/app/pkg/auth
```

- [ ] **Step 7: Verify compilation**

```bash
cd services/catalog && go build ./cmd/
```

Expected: Builds successfully.

- [ ] **Step 8: Verify all workspace modules compile**

```bash
cd /home/fesol/docs/go-journey && go build ./...
```

Expected: All modules compile.

- [ ] **Step 9: Test interceptor works end-to-end**

Start the stack:
```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

Test that public endpoints still work without auth:
```bash
grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks
```
Expected: Returns books (or empty list).

Test that protected endpoints require auth:
```bash
grpcurl -plaintext -d '{"title":"Test","author":"Author","total_copies":1}' localhost:50052 catalog.v1.CatalogService/CreateBook
```
Expected: Returns `Unauthenticated` error.

Test with a valid token:
```bash
# Get a token
TOKEN=$(grpcurl -plaintext -d '{"email":"admin@library.com","password":"admin123","name":"Admin"}' localhost:50051 auth.v1.AuthService/Register | grep -o '"token": "[^"]*"' | cut -d'"' -f4)

# Use it to create a book
grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"title":"Test Book","author":"Test Author","total_copies":3}' localhost:50052 catalog.v1.CatalogService/CreateBook
```
Expected: Returns PermissionDenied since new users have "user" role. To test admin operations, promote the user in the database, then **log in again** to get a new token with the admin role (the old token still has "user" baked into its JWT claims):

```bash
# Promote to admin
docker compose -f deploy/docker-compose.yml exec postgres-auth psql -U postgres -d auth -c "UPDATE users SET role = 'admin' WHERE email = 'admin@library.com';"

# Log in again to get a token with updated role
TOKEN=$(grpcurl -plaintext -d '{"email":"admin@library.com","password":"admin123"}' localhost:50051 auth.v1.AuthService/Login | grep -o '"token": "[^"]*"' | cut -d'"' -f4)

# Now create a book with the admin token
grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"title":"Test Book","author":"Test Author","total_copies":3}' localhost:50052 catalog.v1.CatalogService/CreateBook
```

Expected: Returns the created book.

```bash
docker compose -f deploy/docker-compose.yml down
```

- [ ] **Step 10: Commit**

```bash
git add services/catalog/go.mod services/catalog/go.sum services/catalog/cmd/main.go services/catalog/internal/handler/catalog.go services/catalog/Dockerfile services/catalog/Dockerfile.dev deploy/
git commit -m "feat(auth): add auth interceptor to catalog service, protect write operations"
```

---

### Task 8: Tutorial Documentation

**Context:** Write the Chapter 4 tutorial content for mdBook. Four sections covering auth fundamentals, building the service, OAuth2, and interceptors. Follow the existing pattern from Chapters 1-3.

**Files:**
- Create: `docs/src/ch04/index.md`
- Create: `docs/src/ch04/auth-fundamentals.md`
- Create: `docs/src/ch04/auth-service.md`
- Create: `docs/src/ch04/oauth2.md`
- Create: `docs/src/ch04/interceptors.md`
- Modify: `docs/src/SUMMARY.md` — add Chapter 4 entries

**Reference:** Follow the style established in `docs/src/ch03/` — approximately 1400-1700 words per section, code examples with explanations, footnoted references.

- [ ] **Step 1: Update `docs/src/SUMMARY.md`**

Add Chapter 4 entries after Chapter 3:

```markdown
- Chapter 4: Authentication
    - 4.1 Authentication Fundamentals
    - 4.2 The Auth Service
    - 4.3 OAuth2 with Google
    - 4.4 Protecting Services with Interceptors
```

Use the same link format as existing entries (e.g., `[4.1 Authentication Fundamentals](ch04/auth-fundamentals.md)`).

- [ ] **Step 2: Create `docs/src/ch04/index.md`**

Chapter overview introducing what will be built: JWT auth, bcrypt password hashing, OAuth2 with Google, gRPC interceptors. Include a Mermaid architecture diagram showing the auth service, its database, and how the interceptor protects both auth and catalog services. ~300 words.

- [ ] **Step 3: Create `docs/src/ch04/auth-fundamentals.md` — Section 4.1**

Cover:
- Password hashing with bcrypt: why not SHA256, what salts are, cost factors, `bcrypt.GenerateFromPassword` and `bcrypt.CompareHashAndPassword`
- JWT explained: header, payload, signature structure, why stateless, claims (sub, role, exp, iat), expiry
- Compare JWT to session-based auth (for the Java dev — analogize to Spring Security sessions)
- When to use each approach
- ~1500 words with code examples

- [ ] **Step 4: Create `docs/src/ch04/auth-service.md` — Section 4.2**

Cover:
- Proto definition walkthrough (link to existing proto knowledge from Chapter 2)
- Database migration (users table, nullable password_hash, role CHECK constraint)
- Repository layer (GORM — same pattern as catalog, new methods: GetByEmail, GetByOAuthID)
- Service layer (registration flow: validate → hash → create → JWT, login flow: find → compare → JWT)
- Handler layer (proto conversion, error mapping)
- DI wiring in main.go
- Testing with grpcurl (register, login, validate)
- ~1600 words with full code walkthroughs

- [ ] **Step 5: Create `docs/src/ch04/oauth2.md` — Section 4.3**

Cover:
- OAuth2 authorization code flow explained (the three-legged dance — compare to SAML for the Java dev)
- Google Cloud Console setup: creating credentials, setting redirect URIs (mention this can be skipped — service works without Google creds)
- InitOAuth2 and CompleteOAuth2 RPCs
- State parameter for CSRF protection (in-memory map with sync.Mutex, TTL)
- Google's userinfo API
- Find-or-create user pattern
- Limitations of the in-memory state approach (mention alternatives: signed tokens, Redis)
- ~1500 words

- [ ] **Step 6: Create `docs/src/ch04/interceptors.md` — Section 4.4**

Cover:
- gRPC interceptors explained (middleware analogy — like servlet filters or Spring interceptors for the Java dev)
- Unary vs stream interceptors (we use unary)
- The `pkg/auth/` shared library: why a separate module, what it provides
- JWT validation interceptor: extracting bearer token from metadata, validating, injecting into context
- Context helpers: UserIDFromContext, RoleFromContext, RequireRole
- Adding the interceptor to both auth and catalog services
- Skip list design: which methods are public and why
- Adding role-based protection to catalog (CreateBook, UpdateBook, DeleteBook = admin-only)
- Testing protected endpoints with grpcurl (with and without tokens)
- ~1600 words

- [ ] **Step 7: Commit**

```bash
git add docs/src/SUMMARY.md docs/src/ch04/
git commit -m "docs(ch04): add Chapter 4 tutorial — authentication fundamentals, service, OAuth2, interceptors"
```

---

## Summary

| Task | Description | Dependencies |
|------|-------------|--------------|
| 1 | Proto definition and code generation | None |
| 2 | Shared auth library (`pkg/auth/`) | Task 1 (needs gen types for testing context, but pkg/auth doesn't import gen) |
| 3 | Model, migrations, repository | Task 1 |
| 4 | Service layer (business logic) | Task 2, Task 3 |
| 5 | gRPC handler and server wiring | Task 4 |
| 6 | Docker Compose and Dockerfiles | Task 5 |
| 7 | Add interceptor to catalog service | Task 2, Task 6 |
| 8 | Tutorial documentation | Task 7 |
