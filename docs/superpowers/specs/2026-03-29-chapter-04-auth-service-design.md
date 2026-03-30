# Chapter 4: Auth Service — Design Spec

## Overview

Build the authentication service: user registration/login with bcrypt password hashing, JWT token issuance and validation, OAuth2 with Gmail, and a shared gRPC interceptor for protecting endpoints across services. This chapter teaches auth fundamentals through the library system, building on the patterns established in Chapters 1–3.

## Goals

- Teach password hashing with bcrypt (why not plain hashing, salts, cost factors)
- Teach JWT mechanics (stateless auth, claims, signing, expiry)
- Teach OAuth2 authorization code flow with Google
- Teach gRPC interceptors as middleware for auth
- Build a shared `pkg/auth/` library for JWT validation across services
- Produce a working auth service testable with grpcurl
- Add auth containers to Docker Compose

## Scope

**In scope:** Auth service (email/password + OAuth2), JWT issuance/validation, shared interceptor, Docker Compose additions (postgres-auth + auth service).

**Out of scope:** Kafka event publishing (Chapter 6 adds `auth.users.created` retroactively). Gateway integration (Chapter 5 — gateway calls auth for token validation and serves login/register pages). Refresh tokens (kept simple — single JWT with 24h expiry).

## Key Design Decisions

- **Layered architecture:** Same handler → service → repository pattern as the catalog service.
- **JWT validation: local + remote:** The gRPC interceptor validates JWT signatures locally using a shared secret (fast path). Services call `GetUser` RPC only when full user profile is needed.
- **Shared library in `pkg/auth/`:** JWT utilities and gRPC interceptor live here. Any service can import it.
- **Separate PostgreSQL container:** `postgres-auth` on external port 5432. True microservice data isolation.
- **OAuth2 in this chapter:** The full auth story in one chapter. OAuth2 RPCs exist on the auth service even though the gateway doesn't call them until Chapter 5.
- **No Kafka:** Event publishing deferred to Chapter 6 (same pattern as catalog).

## Service Architecture

### Layers

**Handler layer** (`internal/handler/`):
- Implements `authv1.AuthServiceServer`
- Validates incoming requests (required fields, email format)
- Converts protobuf types <-> domain model types
- Translates service errors -> gRPC status codes

**Service layer** (`internal/service/`):
- Contains business logic: password hashing, JWT generation, OAuth2 token exchange
- Defines the `UserRepository` interface (dependency inversion)
- Enforces invariants (e.g., email/password registration requires non-empty password)
- Returns domain-specific errors

**Repository layer** (`internal/repository/`):
- Implements `UserRepository` using GORM
- Translates GORM errors -> domain errors
- Handles user lookup by email, by ID, by OAuth provider+ID

**Model** (`internal/model/`):
- Domain `User` struct with GORM tags
- Domain error types (`ErrUserNotFound`, `ErrDuplicateEmail`, `ErrInvalidCredentials`)

### Key Interfaces

```go
// Defined in internal/service/auth.go
type UserRepository interface {
    Create(ctx context.Context, user *model.User) (*model.User, error)
    GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
    GetByEmail(ctx context.Context, email string) (*model.User, error)
    GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error)
    Update(ctx context.Context, user *model.User) (*model.User, error)
}
```

### Dependency Injection (cmd/main.go)

Same pattern as catalog — environment config, GORM connection, migrations, bottom-up wiring:

```
env vars → db connect → run migrations → repo → service → handler → gRPC server
```

Additional config for auth: `JWT_SECRET`, `JWT_EXPIRY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`.

## Protobuf & gRPC API

### Proto file: `proto/auth/v1/auth.proto`

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

Key points:
- **`User` message omits sensitive fields** — no password_hash, oauth_provider, oauth_id over the wire.
- **`AuthResponse` shared** by Register, Login, CompleteOAuth2 — all return a JWT + user profile.
- **`ValidateTokenResponse` is lightweight** — just user_id and role, for the interceptor fast path.
- **`role` is a string** not a proto enum — avoids proto enum evolution issues, auth service is the sole source of truth.

## Database

### Migration: `000001_create_users.up.sql`

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

### Migration: `000001_create_users.down.sql`

```sql
DROP TABLE IF EXISTS users;
```

### Key database decisions

- **UUID primary keys** generated by PostgreSQL, same as catalog
- **`password_hash` nullable** — OAuth-only users have no password. Service layer enforces that email/password registration requires a non-empty password.
- **`role` as VARCHAR with CHECK** instead of PostgreSQL ENUM — easier to add new roles without `ALTER TYPE` migration.
- **Composite unique `(oauth_provider, oauth_id)`** — prevents duplicate OAuth accounts.
- **Index on `email`** — login lookups by email are the hot path.
- **Migrations run on startup** via golang-migrate, embedded using Go's `embed` package.

## JWT Design

### Claims

```json
{
  "sub": "<user-uuid>",
  "role": "user|admin",
  "exp": 1711756800,
  "iat": 1711670400
}
```

- Signed with **HMAC-SHA256** (`HS256`) using `JWT_SECRET` environment variable
- Default **24-hour expiry** (configurable via `JWT_EXPIRY`, default `24h`)
- No refresh tokens — kept simple for the tutorial

### Shared Library: `pkg/auth/`

**`jwt.go`** — JWT generation and validation:
- `GenerateToken(userID uuid.UUID, role string, secret string, expiry time.Duration) (string, error)`
- `ValidateToken(tokenString string, secret string) (*Claims, error)`
- `Claims` struct: `UserID uuid.UUID`, `Role string`, embeds `jwt.RegisteredClaims`

**`interceptor.go`** — gRPC unary interceptor:
- Extracts `authorization` from gRPC metadata (`Bearer <token>`)
- Validates JWT signature locally using the shared secret
- Injects `user_id` and `role` into the gRPC context
- Returns `codes.Unauthenticated` if token is missing or invalid
- Accepts a list of full method names to skip (e.g., `/auth.v1.AuthService/Register`, `/auth.v1.AuthService/Login`)

**`context.go`** — Context helpers:
- `UserIDFromContext(ctx) (uuid.UUID, error)` — extracts user ID from context
- `RoleFromContext(ctx) (string, error)` — extracts role from context
- `RequireRole(ctx, role) error` — returns `codes.PermissionDenied` if the context user doesn't have the required role

## OAuth2 Design

### Flow

1. Gateway calls `InitOAuth2()` → Auth service returns Google consent URL
2. User clicks URL → Google redirects to gateway's `/oauth2/callback?code=xxx&state=yyy`
3. Gateway calls `CompleteOAuth2(code, state)` → Auth service:
   a. Validates `state` parameter (CSRF protection)
   b. Exchanges authorization code for Google access token
   c. Fetches user profile from Google's userinfo API (`https://www.googleapis.com/oauth2/v2/userinfo`)
   d. Find-or-create user: looks up by `(oauth_provider="google", oauth_id=google_user_id)`. If not found, creates a new user with email and name from Google profile.
   e. Issues JWT and returns `AuthResponse`

### Configuration

Environment variables:
- `GOOGLE_CLIENT_ID` — from Google Cloud Console
- `GOOGLE_CLIENT_SECRET` — from Google Cloud Console
- `GOOGLE_REDIRECT_URL` — e.g., `http://localhost:8080/oauth2/callback`

### State Parameter

The `state` parameter for CSRF protection is a random string generated by `InitOAuth2`, stored temporarily in an in-memory map with a TTL (5 minutes). `CompleteOAuth2` validates and removes it. This is simple but sufficient — a production system would use a signed token or session-backed state.

## Error Handling

### Domain errors (`internal/model/errors.go`)

```go
var (
    ErrUserNotFound       = errors.New("user not found")
    ErrDuplicateEmail     = errors.New("duplicate email")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrInvalidToken       = errors.New("invalid token")
    ErrTokenExpired       = errors.New("token expired")
    ErrOAuthFailed        = errors.New("oauth2 authentication failed")
)
```

### Error translation (handler layer)

| Domain error | gRPC status code |
|---|---|
| `ErrUserNotFound` | `codes.NotFound` |
| `ErrDuplicateEmail` | `codes.AlreadyExists` |
| `ErrInvalidCredentials` | `codes.Unauthenticated` |
| `ErrInvalidToken` | `codes.Unauthenticated` |
| `ErrTokenExpired` | `codes.Unauthenticated` |
| `ErrOAuthFailed` | `codes.Internal` |
| Validation errors | `codes.InvalidArgument` |
| Unexpected errors | `codes.Internal` |

## Docker Compose Additions

### New containers

| Container | Image | Ports | Purpose |
|---|---|---|---|
| `postgres-auth` | `postgres:16-alpine` | 5432:5432 | Auth service database |
| `auth` | built from `services/auth/Dockerfile` | 50051:50051 | gRPC Auth service |

### Environment additions to `deploy/.env`

```env
POSTGRES_AUTH_PORT=5432
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

### Compose service definitions

```yaml
postgres-auth:
  image: postgres:16-alpine
  environment:
    POSTGRES_USER: ${POSTGRES_AUTH_USER:-postgres}
    POSTGRES_PASSWORD: ${POSTGRES_AUTH_PASSWORD:-postgres}
    POSTGRES_DB: ${POSTGRES_AUTH_DB:-auth}
  ports:
    - "${POSTGRES_AUTH_PORT:-5432}:5432"
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

Dev override adds `Dockerfile.dev` build and volume mounts (same pattern as catalog).

## Testing Strategy

- **Handler tests:** Mock the service interface. Test proto conversion, input validation, error mapping.
- **Service tests:** Mock the repository. Test registration (password hashing), login (password verification), JWT generation/validation, OAuth2 find-or-create logic.
- **Repository tests:** Integration tests against real PostgreSQL. Test CRUD, duplicate email handling, OAuth lookup by provider+ID.
- **`pkg/auth/` tests:** Unit tests for JWT generation/validation (valid token, expired token, tampered token), interceptor behavior (valid/missing/invalid token, skipped methods), context helpers.

## Tutorial Chapter Outline

1. **4.1 Authentication Fundamentals** — Password hashing with bcrypt: why not SHA256, salts, cost factors, `bcrypt.GenerateFromPassword` and `bcrypt.CompareHashAndPassword`. JWTs explained: header, payload, signature, why stateless, claims, expiry. Compare to session-based auth for the Java dev. When to use each.

2. **4.2 The Auth Service** — Proto definition and code generation. Database migration. Repository, service, and handler layers (same pattern as catalog). Registration flow: validate input, hash password, create user, issue JWT. Login flow: find user by email, compare password hash, issue JWT. DI wiring in main.go. Testing with grpcurl.

3. **4.3 OAuth2 with Google** — OAuth2 authorization code flow explained (the 3-legged dance). Google Cloud Console setup: creating credentials, setting redirect URIs. The InitOAuth2 and CompleteOAuth2 RPCs. Google's userinfo API. Find-or-create user pattern. State parameter for CSRF protection. Environment-based configuration.

4. **4.4 Protecting Services with Interceptors** — gRPC interceptors explained (middleware analogy for the Java dev: like servlet filters or Spring interceptors). The `pkg/auth/` shared library. JWT validation interceptor: extracting from metadata, validating, injecting into context. Context helpers. Adding auth to the catalog service (protecting CreateBook, UpdateBook, DeleteBook as admin-only). Role-based access control with `RequireRole`.

## File Structure

```
services/auth/
├── cmd/
│   └── main.go                    # gRPC server startup, DI wiring
├── internal/
│   ├── handler/
│   │   ├── auth.go                # gRPC handler implementation
│   │   └── auth_test.go           # handler tests (mock service)
│   ├── service/
│   │   ├── auth.go                # business logic + UserRepository interface
│   │   └── auth_test.go           # service tests (mock repository)
│   ├── repository/
│   │   ├── user.go                # GORM repository implementation
│   │   └── user_test.go           # integration tests (real DB)
│   └── model/
│       ├── user.go                # domain User struct
│       └── errors.go              # domain error types
├── migrations/
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   └── embed.go
├── Dockerfile
├── Dockerfile.dev
├── .air.toml
├── Earthfile
└── go.mod

pkg/auth/
├── jwt.go                         # JWT generation and validation
├── jwt_test.go                    # JWT unit tests
├── interceptor.go                 # gRPC auth interceptor
├── interceptor_test.go            # interceptor unit tests
├── context.go                     # context helpers (UserIDFromContext, etc.)
└── go.mod                         # separate module in workspace

proto/auth/v1/
└── auth.proto                     # Auth service proto definition

gen/auth/v1/
├── auth.pb.go                     # generated
└── auth_grpc.pb.go                # generated

deploy/
├── docker-compose.yml             # UPDATE: add postgres-auth + auth
├── docker-compose.dev.yml         # UPDATE: add auth dev override
└── .env                           # UPDATE: add auth env vars

docs/src/
├── SUMMARY.md                     # UPDATE: add Chapter 4 entries
└── ch04/
    ├── index.md
    ├── auth-fundamentals.md       # 4.1
    ├── auth-service.md            # 4.2
    ├── oauth2.md                  # 4.3
    └── interceptors.md            # 4.4
```

## Dependencies (Go modules)

### services/auth/go.mod
- `google.golang.org/grpc` — gRPC framework
- `google.golang.org/protobuf` — protobuf runtime
- `gorm.io/gorm` — ORM
- `gorm.io/driver/postgres` — GORM PostgreSQL driver
- `github.com/golang-migrate/migrate/v4` — database migrations
- `github.com/google/uuid` — UUID type
- `github.com/golang-jwt/jwt/v5` — JWT library
- `golang.org/x/crypto` — bcrypt
- `golang.org/x/oauth2` — OAuth2 client

### pkg/auth/go.mod
- `github.com/golang-jwt/jwt/v5` — JWT library
- `github.com/google/uuid` — UUID type
- `google.golang.org/grpc` — gRPC (for interceptor and metadata)

## Implementation Notes

- **`go.work` update:** Add `./services/auth` and `./pkg/auth` to the workspace.
- **`replace` directives:** `services/auth/go.mod` needs `replace` for both `gen` and `pkg/auth`. `pkg/auth/go.mod` is self-contained (no local dependencies).
- **bcrypt cost factor:** Use `bcrypt.DefaultCost` (10). The tutorial should explain what cost means and why 10 is reasonable.
- **OAuth2 state storage:** In-memory map with TTL. Simple but not production-grade (doesn't survive restarts, doesn't scale horizontally). The tutorial should acknowledge this and mention alternatives (signed state tokens, Redis).
- **Google OAuth2 is optional to run:** The service starts and works for email/password auth even without Google credentials configured. OAuth2 RPCs return an appropriate error if credentials are missing. This lets readers skip Google Cloud Console setup if they just want to learn the code.
- **Interceptor skip list:** Register, Login, InitOAuth2, and CompleteOAuth2 must be skipped (they're called before the user has a token). ValidateToken and GetUser require a valid token.
- **Catalog service update:** Section 4.4 adds the auth interceptor to the catalog's gRPC server. Admin-only RPCs: CreateBook, UpdateBook, DeleteBook. Read-only RPCs (GetBook, ListBooks) remain public. UpdateAvailability is also skipped (will be called by the reservation service internally in Chapter 7).

## What This Chapter Does NOT Include

- Kafka event publishing (Chapter 6 adds `auth.users.created`)
- Gateway integration (Chapter 5 — login pages, session cookies, token forwarding)
- Refresh tokens or token rotation
- Rate limiting on login attempts
- Email verification
- Password reset flow
- Multi-factor authentication
