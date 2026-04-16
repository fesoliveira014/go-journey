# 4.2 The Auth Service

<!-- [STRUCTURAL] Good bridging opener — connects forward to the layered architecture pattern established in Ch.2 and names the *interesting differences* (nullable fields, error mapping). Reader knows what to pay attention to. -->
<!-- [LINE EDIT] "This section walks through each layer -- proto definition, database migration, repository, service, handler, and wiring -- following the same architecture patterns established in Chapter 2 for the Catalog service." → 28 words, fine. Keep. -->
<!-- [LINE EDIT] "The interesting differences are in the details: nullable fields for OAuth users, password hashing in the service layer, and a richer error-to-gRPC-code mapping in the handler." → Good — specific preview. Keep. -->
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

<!-- [LINE EDIT] "The first three are the core authentication RPCs." → Good topical sentence. -->
<!-- [LINE EDIT] "`ValidateToken` is used by other services (or a gateway) to verify a token and extract the user ID and role without needing the JWT secret themselves -- though in our architecture, services share the secret and validate locally via the interceptor." → 41 words and clause-heavy. Split: "Other services (or a gateway) call `ValidateToken` to verify a token and extract the user ID and role without holding the JWT secret themselves. In our architecture, however, services share the secret and validate locally via the interceptor." -->
The first three are the core authentication RPCs. `Register` and `Login` return an `AuthResponse` containing a JWT token and the user object. `ValidateToken` is used by other services (or a gateway) to verify a token and extract the user ID and role without needing the JWT secret themselves -- though in our architecture, services share the secret and validate locally via the interceptor.

<!-- [STRUCTURAL] Consider a brief note on why `ValidateToken` exists at all given the interceptor path. The current sentence hints ("though in our architecture...") but the reader may wonder whether the RPC is dead code. A sentence like "We keep it for clients that do not run the shared interceptor — for instance, a future HTTP gateway or CLI tool" would justify its continued presence. -->
`GetUser` is a simple lookup by ID. `InitOAuth2` and `CompleteOAuth2` implement the OAuth2 authorization code flow, which we cover in detail in section 4.3.

The `AuthResponse` message pairs a token with a user:

```protobuf
message AuthResponse {
  string token = 1;
  User user = 2;
}
```

<!-- [LINE EDIT] "This pattern -- returning both the token and the user data in one response -- avoids a separate round-trip after login." → Good. Keep. -->
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

<!-- [STRUCTURAL] Good structure — three design decisions called out as bold-headed paragraphs is readable. Keep. -->
Several design decisions deserve explanation:

<!-- [LINE EDIT] "(what if someone tries to log in with the dummy password?)" → The rhetorical question is effective. Keep. -->
<!-- [COPY EDIT] "A `NULL` password hash explicitly means \"this user cannot authenticate with a password.\"" — period inside closing quote, correct per CMOS 6.9. -->
**`password_hash` is nullable.** OAuth users authenticate through Google -- they never set a password. Making this column `NOT NULL` would force us to store a dummy value, which is both inelegant and a potential security risk (what if someone tries to log in with the dummy password?). A `NULL` password hash explicitly means "this user cannot authenticate with a password." The `Login` handler checks for this:

```go
if user.PasswordHash == nil {
    return "", nil, model.ErrInvalidCredentials
}
```

<!-- [LINE EDIT] "This is database-level enforcement -- even if a bug in the application tries to set `role = 'superadmin'`, PostgreSQL rejects it." → Good. Keep. -->
<!-- [COPY EDIT] "Defense in depth." — acceptable sentence fragment as a rhetorical stinger. -->
**The `valid_role` CHECK constraint** restricts the `role` column to `'user'` or `'admin'`. This is database-level enforcement -- even if a bug in the application tries to set `role = 'superadmin'`, PostgreSQL rejects it. Defense in depth.

<!-- [LINE EDIT] "A Google user with ID `12345` can only have one row." → Change "can only have" to "has only" for precision: "A Google user with ID `12345` has only one row." The "only" was modifying "one" in the original, and the simpler form is clearer. -->
<!-- [LINE EDIT] "For our learning project, we keep it simple." → Crisper: "For this learning project, we keep it simple." ("Our" drifts across the chapter; "this" is more direct.) -->
**The `oauth_unique` composite constraint** ensures that each OAuth provider + ID combination is unique. A Google user with ID `12345` can only have one row. But the same email address could exist as both a password user and a Google user -- this is a deliberate choice. In a production system, you might want to merge these accounts. For our learning project, we keep it simple.

<!-- [COPY EDIT] "uuid-ossp" — package name in code font; fine. The claim that it "provides the uuid_generate_v4() function" is accurate. -->
**`uuid-ossp` extension** provides the `uuid_generate_v4()` function for generating UUIDs at the database level. Same pattern as the Catalog service's `books` table.

---

## Repository Layer

<!-- [LINE EDIT] "follows the same GORM pattern as the Catalog service, with methods tailored to authentication" → Good, compact. Keep. -->
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
// isDuplicateKeyError reports whether err wraps a PostgreSQL unique-violation
// (SQLSTATE 23505). It checks the typed *pgconn.PgError code rather than
// matching the error message, which is not a stable API.
func isDuplicateKeyError(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

<!-- [STRUCTURAL] Good — prose below the snippet explicitly names the *lesson* (locale-dependent strings vs typed codes). That is the pedagogical payoff, and it is delivered. -->
<!-- [LINE EDIT] "That sidesteps the fragility of string-matching the message (the wording is locale-dependent and not part of any stable API)." → Good sentence. Keep. -->
<!-- [COPY EDIT] Please verify: PostgreSQL SQLSTATE "23505" is `unique_violation` per PostgreSQL docs (pgsql-docs appendix A). Confirmed. -->
<!-- [COPY EDIT] "GORM returns the raw `pgx` driver error" — lowercase "pgx" is the package name, correct. Good. -->
This is the same typed-error pattern we introduced in Chapter 2: GORM returns the raw `pgx` driver error, so we use `errors.As` to unwrap a `*pgconn.PgError` and check `Code == "23505"` directly. That sidesteps the fragility of string-matching the message (the wording is locale-dependent and not part of any stable API). In the `Create` method this maps to `model.ErrDuplicateEmail`, which the handler translates to gRPC `AlreadyExists`.

---

## Service Layer

<!-- [STRUCTURAL] The Spring analogy is apt and will land for the target reader. It correctly distinguishes `@Autowired` magic from explicit wiring. Keep. -->
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

<!-- [LINE EDIT] "No `@Autowired` magic -- the wiring happens explicitly in `main.go`." → Good, clean sentence. Keep. -->
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

<!-- [LINE EDIT] "The flow is straightforward: validate, hash, persist, issue token. New users always get the `"user"` role. Promoting to admin is a deliberate manual operation (direct SQL update) -- there is no "register as admin" RPC." → Good rhythm. Keep. -->
<!-- [COPY EDIT] "New users always get the `"user"` role." — the straight quotes inside the backtick-quoted string are fine in code contexts. Good. -->
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

<!-- [LINE EDIT] "Notice the security detail: whether the email does not exist, the user is OAuth-only, or the password is wrong, the same error is returned -- `ErrInvalidCredentials`." → Minor awkwardness with "whether" governing three parallel clauses. Consider: "Notice the security detail: all three failure paths — email not found, OAuth-only user, wrong password — return the same error, `ErrInvalidCredentials`." -->
<!-- [COPY EDIT] "e.g." per CMOS 6.43 takes a comma after: "e.g., database failures" — the source has "e.g. database failures" inside a code comment. Inside Go source comments, terseness wins over strict CMOS. Flagging but low priority. -->
Notice the security detail: whether the email does not exist, the user is OAuth-only, or the password is wrong, the same error is returned -- `ErrInvalidCredentials`. This prevents user enumeration attacks where an attacker probes different emails to discover which ones are registered.

---

## Handler Layer

<!-- [STRUCTURAL] Section flow is logical: role of handler → input validation snippet → error mapping → big-picture Spring analogy at the end. Keep. -->
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

<!-- [STRUCTURAL] Strong closing sentence. The "never leaking internal details" stinger is valuable — it names the *why* behind the default case. -->
<!-- [LINE EDIT] "This is analogous to a Spring `@ControllerAdvice` exception handler that maps domain exceptions to HTTP status codes." → Good. Keep. -->
<!-- [COPY EDIT] "ControllerAdvice" — Spring annotation, correctly capitalized as product/technology name (CMOS 8.154). -->
This is analogous to a Spring `@ControllerAdvice` exception handler that maps domain exceptions to HTTP status codes. The default case returns `Internal` with a generic message -- never leaking internal details to the client.

---

## DI Wiring in main.go

<!-- [COPY EDIT] Heading "DI Wiring in main.go" — "main.go" as a file name is correctly lowercased in code. Acceptable to mix case in headings. -->
<!-- [LINE EDIT] "no framework, no reflection" → good rhythm. Keep. -->
The `main.go` wires everything together using constructor functions -- no framework, no reflection:

```go
// Configuration from environment. JWT_SECRET is required — no default.
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable is required")
}
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

<!-- [STRUCTURAL] The snippet references `googleClientSecret`, `googleRedirectURL`, and `interceptor` without showing where they are assigned. A one-line comment in the snippet like "// (googleClientSecret, googleRedirectURL, interceptor omitted for brevity — full version in repo)" would prevent "is this code complete?" confusion. -->
<!-- [LINE EDIT] "Each layer only knows about the layer directly below it. The handler does not know about GORM. The service does not know about gRPC." → Good parallelism. Keep. -->
<!-- [LINE EDIT] "and it makes testing straightforward -- you can mock any interface boundary." → Good. Keep. -->
Each layer only knows about the layer directly below it. The handler does not know about GORM. The service does not know about gRPC. This is the same layered architecture as the Catalog service, and it makes testing straightforward -- you can mock any interface boundary.

<!-- [STRUCTURAL] This callout block is excellent — it tells a story (earlier draft had a fallback, we removed it, here's why) that turns a configuration detail into a durable lesson (12-Factor, fail-fast). Keep the voice. -->
<!-- [COPY EDIT] "The [12-Factor App's Config factor](https://12factor.net/config) is clear here" — possessive of "App's" is correct per CMOS 7.17 (named entity). -->
<!-- [COPY EDIT] "config belongs in the environment, and missing required config should fail fast" — parallel structure, well-balanced. -->
<!-- [LINE EDIT] "An earlier draft of this code fell back to `"dev-secret-change-in-production"` when the env var was unset." → The inner double-quoted string reads fine in Markdown but confirm SSG doesn't break on nested quotes. -->
> **Why `log.Fatal` on missing `JWT_SECRET`?** An earlier draft of this code fell back to `"dev-secret-change-in-production"` when the env var was unset. That pattern is dangerous: a misconfigured deployment (forgotten Kubernetes Secret, typo in a ConfigMap key) would silently start a production service that accepts tokens signed with a publicly known string. The [12-Factor App's Config factor](https://12factor.net/config) is clear here — config belongs in the environment, and missing required config should fail fast. Development defaults belong in a `.env` file or Compose env block, not in the binary itself.

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

<!-- [LINE EDIT] "The Register and Login responses include both a `token` string and a `user` object with the ID, email, name, role, and timestamps." → Serial comma present. Good. -->
The Register and Login responses include both a `token` string and a `user` object with the ID, email, name, role, and timestamps.

---

## Summary

<!-- [STRUCTURAL] Summary captures the durable takeaways, not a sentence-by-sentence recap. Good. -->
<!-- [COPY EDIT] "four-layer architecture" — compound adjective hyphenated before noun. Good (CMOS 7.81). -->
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
<!-- [FINAL] No footnotes ([^1]...[^5]) are actually referenced in the body text of this file — cross-check. The body refers to "section 4.3" and "Chapter 2" but does not use `[^N]` anchors. Either add anchors in relevant sentences (e.g., attach [^1] to the "gRPC `AlreadyExists`" mention) or accept that these references function as a bibliography rather than in-text citations. Flagging for author's attention. -->
