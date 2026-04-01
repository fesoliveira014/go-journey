# 4.2 The Auth Service

With the fundamentals in place, we can now build the Auth service end to end. This section walks through each layer -- proto definition, database migration, repository, service, handler, and wiring -- following the same architecture patterns established in Chapter 2 for the Catalog service. If you followed that chapter, the structure will be familiar. The interesting differences are in the details: nullable fields for OAuth users, password hashing in the service layer, and a richer error-to-gRPC-code mapping in the handler.

---

## Proto Definition

The Auth service defines six RPCs in `proto/auth/v1/auth.proto`:

```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (AuthResponse);
  rpc Login(LoginRequest) returns (AuthResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
  rpc GetUser(GetUserRequest) returns (User);
  rpc InitOAuth2(InitOAuth2Request) returns (InitOAuth2Response);
  rpc CompleteOAuth2(CompleteOAuth2Request) returns (AuthResponse);
}
```

The first three are the core authentication RPCs. `Register` and `Login` return an `AuthResponse` containing a JWT token and the user object. `ValidateToken` is used by other services (or a gateway) to verify a token and extract the user ID and role without needing the JWT secret themselves -- though in our architecture, services share the secret and validate locally via the interceptor.

`GetUser` is a simple lookup by ID. `InitOAuth2` and `CompleteOAuth2` implement the OAuth2 authorization code flow, which we cover in detail in section 4.3.

The `AuthResponse` message pairs a token with a user:

```protobuf
message AuthResponse {
  string token = 1;
  User user = 2;
}
```

This pattern -- returning both the token and the user data in one response -- avoids a separate round-trip after login. The client gets everything it needs to display a profile and make authenticated requests.

---

## Database Migration

The `users` table lives in `services/auth/migrations/000001_create_users.up.sql`:

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

Several design decisions deserve explanation:

**`password_hash` is nullable.** OAuth users authenticate through Google -- they never set a password. Making this column `NOT NULL` would force us to store a dummy value, which is both inelegant and a potential security risk (what if someone tries to log in with the dummy password?). A `NULL` password hash explicitly means "this user cannot authenticate with a password." The `Login` handler checks for this:

```go
if user.PasswordHash == nil {
    return "", nil, model.ErrInvalidCredentials
}
```

**The `valid_role` CHECK constraint** restricts the `role` column to `'user'` or `'admin'`. This is database-level enforcement -- even if a bug in the application tries to set `role = 'superadmin'`, PostgreSQL rejects it. Defense in depth.

**The `oauth_unique` composite constraint** ensures that each OAuth provider + ID combination is unique. A Google user with ID `12345` can only have one row. But the same email address could exist as both a password user and a Google user -- this is a deliberate choice. In a production system, you might want to merge these accounts. For our learning project, we keep it simple.

**`uuid-ossp` extension** provides the `uuid_generate_v4()` function for generating UUIDs at the database level. Same pattern as the Catalog service's `books` table.

---

## Repository Layer

The repository in `services/auth/internal/repository/user.go` follows the same GORM pattern as the Catalog service, with methods tailored to authentication:

```go
type UserRepository struct {
    db *gorm.DB
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
```

`GetByEmail` is used during login. `GetByOAuthID` is used during the OAuth2 flow to check whether a user already exists for a given Google account.

The `isDuplicateKeyError` helper detects PostgreSQL unique constraint violations:

```go
func isDuplicateKeyError(err error) bool {
    if err == nil {
        return false
    }
    msg := err.Error()
    return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
```

This is the same string-matching pattern from Chapter 2. GORM does not expose structured database errors, so we check the error message for PostgreSQL's unique violation code (23505). In the `Create` method, this maps to `model.ErrDuplicateEmail`, which the handler translates to gRPC `AlreadyExists`.

---

## Service Layer

The service layer in `services/auth/internal/service/auth.go` contains the business logic. It depends on a `UserRepository` interface -- not the concrete GORM implementation:

```go
type UserRepository interface {
    Create(ctx context.Context, user *model.User) (*model.User, error)
    GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
    GetByEmail(ctx context.Context, email string) (*model.User, error)
    GetByOAuthID(ctx context.Context, provider, oauthID string) (*model.User, error)
    Update(ctx context.Context, user *model.User) (*model.User, error)
}
```

If you come from Spring, this is the same dependency inversion pattern -- a Spring `@Service` depends on a `@Repository` interface, not the JPA implementation. In Go, we achieve this without annotations or DI frameworks: the interface is defined in the `service` package (the consumer), and the `repository` package provides a concrete struct that satisfies it. No `@Autowired` magic -- the wiring happens explicitly in `main.go`.

### Registration Flow

```go
func (s *AuthService) Register(ctx context.Context, email, password, name string) (string, *model.User, error) {
    // 1. Validate inputs
    if email == "" { return "", nil, fmt.Errorf("email is required") }
    if password == "" { return "", nil, fmt.Errorf("password is required") }
    if name == "" { return "", nil, fmt.Errorf("name is required") }

    // 2. Hash password with bcrypt
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil { return "", nil, fmt.Errorf("failed to hash password: %w", err) }
    hashStr := string(hash)

    // 3. Create user in database
    user := &model.User{
        Email: email, PasswordHash: &hashStr, Name: name, Role: "user",
    }
    created, err := s.repo.Create(ctx, user)
    if err != nil { return "", nil, err }

    // 4. Issue JWT
    token, err := pkgauth.GenerateToken(created.ID, created.Role, s.jwtSecret, s.jwtExpiry)
    if err != nil { return "", nil, fmt.Errorf("failed to generate token: %w", err) }
    return token, created, nil
}
```

The flow is straightforward: validate, hash, persist, issue token. New users always get the `"user"` role. Promoting to admin is a deliberate manual operation (direct SQL update) -- there is no "register as admin" RPC.

### Login Flow

```go
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
    user, err := s.repo.GetByEmail(ctx, email)
    if err != nil {
        if errors.Is(err, model.ErrUserNotFound) {
            return "", nil, model.ErrInvalidCredentials // Don't leak whether email exists
        }
        return "", nil, err // Propagate unexpected errors (e.g. database failures)
    }

    if user.PasswordHash == nil {
        return "", nil, model.ErrInvalidCredentials // OAuth-only user
    }

    if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
        return "", nil, model.ErrInvalidCredentials
    }

    token, err := pkgauth.GenerateToken(user.ID, user.Role, s.jwtSecret, s.jwtExpiry)
    if err != nil { return "", nil, fmt.Errorf("failed to generate token: %w", err) }
    return token, user, nil
}
```

Notice the security detail: whether the email does not exist, the user is OAuth-only, or the password is wrong, the same error is returned -- `ErrInvalidCredentials`. This prevents user enumeration attacks where an attacker probes different emails to discover which ones are registered.

---

## Handler Layer

The handler in `services/auth/internal/handler/auth.go` translates between gRPC (protobuf messages, gRPC status codes) and the domain (Go structs, sentinel errors).

### Input Validation

Each RPC validates required fields and returns `codes.InvalidArgument`:

```go
func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
    if req.GetEmail() == "" {
        return nil, status.Error(codes.InvalidArgument, "email is required")
    }
    // ... more validation, then delegate to service
}
```

### Error Mapping

The `toGRPCError` function maps six domain errors to appropriate gRPC status codes:

```go
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

This is analogous to a Spring `@ControllerAdvice` exception handler that maps domain exceptions to HTTP status codes. The default case returns `Internal` with a generic message -- never leaking internal details to the client.

---

## DI Wiring in main.go

The `main.go` wires everything together using constructor functions -- no framework, no reflection:

```go
// Configuration from environment
jwtSecret := os.Getenv("JWT_SECRET")
jwtExpiry := os.Getenv("JWT_EXPIRY")
googleClientID := os.Getenv("GOOGLE_CLIENT_ID")

// Database
db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})

// Migrations
runMigrations(db)

// Dependency chain: repo → service → handler → gRPC server
userRepo := repository.NewUserRepository(db)
authSvc := service.NewAuthService(userRepo, jwtSecret, jwtExpiry)
authHandler := handler.NewAuthHandlerWithOAuth(authSvc, googleClientID, googleClientSecret, googleRedirectURL)

// gRPC server with auth interceptor
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
authv1.RegisterAuthServiceServer(grpcServer, authHandler)
```

Each layer only knows about the layer directly below it. The handler does not know about GORM. The service does not know about gRPC. This is the same layered architecture as the Catalog service, and it makes testing straightforward -- you can mock any interface boundary.

---

## Testing with grpcurl

With the stack running (`docker compose up`), test the service:

```bash
# Register a new user
grpcurl -plaintext -d '{
  "email": "alice@example.com",
  "password": "secret123",
  "name": "Alice"
}' localhost:50051 auth.v1.AuthService/Register

# Login
grpcurl -plaintext -d '{
  "email": "alice@example.com",
  "password": "secret123"
}' localhost:50051 auth.v1.AuthService/Login

# Validate the token (paste the token from the login response)
grpcurl -plaintext -d '{
  "token": "<token-from-login>"
}' localhost:50051 auth.v1.AuthService/ValidateToken
```

The Register and Login responses include both a `token` string and a `user` object with the ID, email, name, role, and timestamps.

---

## Summary

- The Auth service follows the same four-layer architecture as Catalog: proto, repository, service, handler
- Nullable `password_hash` supports both email/password and OAuth-only users
- The service layer handles all business logic (hashing, validation) while the handler handles gRPC translation
- Domain errors map to specific gRPC codes via `toGRPCError`
- DI wiring is explicit in `main.go` -- no framework magic

---

## References

[^1]: [gRPC status codes](https://grpc.io/docs/guides/status-codes/) -- official documentation on gRPC canonical status codes and when to use each.
[^2]: [GORM documentation](https://gorm.io/docs/) -- the Go ORM used for database access in both the Auth and Catalog services.
[^3]: [golang-migrate](https://github.com/golang-migrate/migrate) -- the migration tool used for database schema management.
[^4]: [Dependency Inversion Principle](https://en.wikipedia.org/wiki/Dependency_inversion_principle) -- the SOLID principle behind interface-based DI in Go.
[^5]: [User enumeration prevention](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html#authentication-responses) -- OWASP guidance on preventing user enumeration through consistent error messages.
