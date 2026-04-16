# 4.4 Protecting Services with Interceptors

<!-- [STRUCTURAL] Strong opener: names the gap from 4.2 (issuing vs requiring), states the artifact built in this section, and previews the end state ("only admins can..."). Ideal section intro. -->
<!-- [LINE EDIT] "Now we need to make other services *require* them." → Good italic emphasis on "require". Keep. -->
<!-- [LINE EDIT] "By the end, unauthenticated users can browse the catalog but cannot modify it, and only admins can create, update, or delete books." → Serial comma present. Good. -->
We have an Auth service that issues JWTs. Now we need to make other services *require* them. In this section, we build the shared `pkg/auth` library and wire its interceptor into both the Auth and Catalog services. By the end, unauthenticated users can browse the catalog but cannot modify it, and only admins can create, update, or delete books.

---

## gRPC Interceptors Explained

<!-- [STRUCTURAL] Three-analogy framing (servlet filters, HandlerInterceptor, ContainerRequestFilter) is great for the target reader — covers Spring, Jakarta EE, and JAX-RS. Keep. -->
<!-- [LINE EDIT] "They sit in front of your handlers and can inspect, modify, or reject requests before they reach your business logic." → Serial comma present. Good. -->
If you come from the Java world, gRPC interceptors are the equivalent of **servlet filters**, Spring's `HandlerInterceptor`, or JAX-RS `ContainerRequestFilter`. They sit in front of your handlers and can inspect, modify, or reject requests before they reach your business logic.

<!-- [COPY EDIT] "Unary interceptors -- for request/response RPCs (the kind we use)" — parenthetical is helpful context. -->
gRPC defines two types:
- **Unary interceptors** -- for request/response RPCs (the kind we use)
- **Stream interceptors** -- for streaming RPCs

The signature of a unary server interceptor is:

```go
type UnaryServerInterceptor func(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error)
```

<!-- [STRUCTURAL] Excellent enumeration of what an interceptor can do (4 bullets). Keep. -->
<!-- [LINE EDIT] "This is the **chain of responsibility** pattern." → Good name-drop, reader can look up GoF. Keep. -->
<!-- [COPY EDIT] "info.FullMethod" — inline code font. Good. -->
This is the **chain of responsibility** pattern. The interceptor receives the request context, the request itself, metadata about the RPC being called (`info.FullMethod`), and a `handler` function that calls the next interceptor or the actual RPC implementation. The interceptor can:
- Call `handler(ctx, req)` to proceed normally
- Return an error to reject the request
- Modify the context (add values) before calling the handler
- Wrap the handler to measure timing or log results

<!-- [LINE EDIT] "Our auth interceptor does all three: it rejects requests without valid tokens, extracts user info from the token, injects it into the context, and then calls the handler." → Reader counts four actions, not three. Fix: "Our auth interceptor does three of these: it rejects requests without valid tokens, injects user info into the context, and then calls the handler." OR: "Our auth interceptor combines all three: it rejects, modifies, and proceeds." -->
Our auth interceptor does all three: it rejects requests without valid tokens, extracts user info from the token, injects it into the context, and then calls the handler.

---

## The `pkg/auth/` Shared Library

<!-- [STRUCTURAL] Good — table gives a file map, then prose explains *why* this lives in pkg/ rather than in the Auth service. Addresses the most likely reader question. -->
<!-- [LINE EDIT] "`pkg/auth` directory is a **separate Go module** (with its own `go.mod`) that services import via `replace` directives in the workspace." → Slightly dense. Split? "The `pkg/auth` directory is a **separate Go module** with its own `go.mod`. Services import it via `replace` directives in the workspace." -->
<!-- [COPY EDIT] "pkg/auth" — the trailing slash in the heading "`pkg/auth/`" but no trailing slash in the prose references. Minor inconsistency. Decide on one and apply globally. -->
The `pkg/auth` directory is a **separate Go module** (with its own `go.mod`) that services import via `replace` directives in the workspace. Both the Auth and Catalog services depend on it. It contains three files:

| File | Purpose |
|---|---|
| `jwt.go` | `GenerateToken`, `ValidateToken`, `Claims` struct |
| `context.go` | `ContextWithUser`, `UserIDFromContext`, `RoleFromContext`, `RequireRole` |
| `interceptor.go` | `UnaryAuthInterceptor` |

<!-- [LINE EDIT] "If this code lived inside `services/auth/`, the Catalog service would need to import from the Auth service's internal packages -- which violates Go's `internal` package convention and creates a tight coupling between services." → 38 words, fine. Keep. -->
<!-- [COPY EDIT] "Go's `internal` package convention" — accurately describes the language rule (Go refuses cross-module imports from directories named `internal` above a common ancestor). Good. -->
Why put this in `pkg/` rather than inside the Auth service? Because multiple services need it. The Catalog service needs to validate tokens and check roles. If this code lived inside `services/auth/`, the Catalog service would need to import from the Auth service's internal packages -- which violates Go's `internal` package convention and creates a tight coupling between services.

<!-- [STRUCTURAL] The "pkg/ is a community convention, not a language rule" clarification is valuable and prevents confusion. Keep. -->
<!-- [COPY EDIT] "is a Go community convention (not a language requirement)" — could be rephrased to avoid parenthetical. Consider: "is a community convention rather than a language requirement". But the parenthetical is clearer for emphasis. Leave. -->
The `pkg/` directory is a Go community convention (not a language requirement) for packages intended to be imported by other packages within the same module. In a monorepo like ours, it is the natural home for shared libraries.

---

## JWT Validation Interceptor

Here is the complete interceptor from `pkg/auth/interceptor.go`:

```go
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

        // Parse user ID and inject into context
        userID, err := uuid.Parse(claims.Subject)
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid user ID in token")
        }

        ctx = ContextWithUser(ctx, userID, claims.Role)
        return handler(ctx, req)
    }
}
```

<!-- [LINE EDIT] "`UnaryAuthInterceptor` is called once at server startup with the JWT secret and skip list, and returns the actual interceptor function." → Good. Keep. -->
<!-- [LINE EDIT] "The skip map is built once and reused for every request." → Good. Keep. -->
<!-- [COPY EDIT] "closure factory" — informal term, acceptable in teaching context; some readers may know it as "higher-order function" or "factory function". -->
The function is a **closure factory**: `UnaryAuthInterceptor` is called once at server startup with the JWT secret and skip list, and returns the actual interceptor function. The skip map is built once and reused for every request.

<!-- [LINE EDIT] "The **skip-methods pattern** is essential. Not every RPC requires authentication -- `Register` and `Login` obviously cannot require a token (the user does not have one yet)." → Drop "obviously" — it's subtly condescending and the sentence works without it: "Not every RPC requires authentication — `Register` and `Login` cannot require a token because the user does not have one yet." -->
<!-- [COPY EDIT] "O(1) lookup" — big-O notation correctly capitalized. Good. -->
The **skip-methods pattern** is essential. Not every RPC requires authentication -- `Register` and `Login` obviously cannot require a token (the user does not have one yet). The skip list is a slice of full gRPC method names like `"/auth.v1.AuthService/Register"`. At startup, these are converted to a map for O(1) lookup.

<!-- [LINE EDIT] "The interceptor extracts the `authorization` metadata header (gRPC's equivalent of HTTP headers), parses the "Bearer \<token\>" format, validates the JWT, and injects the user ID and role into the context using `ContextWithUser`." → 40 words. Could split but reads OK. Keep. -->
<!-- [COPY EDIT] "Bearer \<token\>" — escaped angle brackets to avoid HTML interpretation; Markdown-safe. Good. -->
<!-- [COPY EDIT] Please verify: gRPC metadata keys are case-insensitive per the gRPC HTTP/2 spec; `md.Get("authorization")` returns values regardless of client capitalization ("Authorization" vs "authorization"). Confirmed — `metadata.MD.Get` normalizes keys to lowercase. -->
The interceptor extracts the `authorization` metadata header (gRPC's equivalent of HTTP headers), parses the "Bearer \<token\>" format, validates the JWT, and injects the user ID and role into the context using `ContextWithUser`.

---

## Context Helpers

The context helpers in `pkg/auth/context.go` provide type-safe access to user information stored in the request context:

```go
type contextKey string

const (
    userIDKey contextKey = "auth_user_id"
    roleKey   contextKey = "auth_role"
)

func ContextWithUser(ctx context.Context, userID uuid.UUID, role string) context.Context {
    ctx = context.WithValue(ctx, userIDKey, userID)
    ctx = context.WithValue(ctx, roleKey, role)
    return ctx
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
    v, ok := ctx.Value(userIDKey).(uuid.UUID)
    if !ok {
        return uuid.Nil, fmt.Errorf("user ID not found in context")
    }
    return v, nil
}
```

### Why Typed Context Keys?

<!-- [STRUCTURAL] Excellent sub-subsection — explains a non-obvious Go idiom with a concrete counter-example ("DON'T DO THIS") and names the language mechanism ("context.Value compares both the type and the value"). This is the pedagogical ideal. -->
<!-- [COPY EDIT] "unexported (lowercase)" — "unexported" is the correct Go term; "lowercase" is the mechanism. Good. -->
The `contextKey` type is an unexported (lowercase) string type. This is a critical Go pattern. If we used a plain `string` as the key:

```go
ctx = context.WithValue(ctx, "auth_user_id", userID) // DON'T DO THIS
```

<!-- [LINE EDIT] "Any package that uses the string `"auth_user_id"` as a context key would collide with ours." → Good. Keep. -->
<!-- [LINE EDIT] "Other packages literally cannot construct a key that would match, even if they use the same string content -- because `context.Value` compares both the type and the value." → "literally" is a filler intensifier per most style guides. Consider: "Other packages cannot construct a key that matches, even with the same string content — `context.Value` compares both the type and the value." -->
Any package that uses the string `"auth_user_id"` as a context key would collide with ours. By defining a custom type, only code in the `auth` package can create a value of type `contextKey`. Other packages literally cannot construct a key that would match, even if they use the same string content -- because `context.Value` compares both the type and the value.

<!-- [LINE EDIT] "This is analogous to using an enum or a typed constant as a map key in Java, rather than a raw string. The type system prevents accidental collisions." → Good analogy. Keep. -->
This is analogous to using an enum or a typed constant as a map key in Java, rather than a raw string. The type system prevents accidental collisions.

### RequireRole

`RequireRole` is a convenience function that checks the role in the context and returns a gRPC error if it doesn't match:

```go
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

<!-- [STRUCTURAL] The "This maps directly to HTTP 401 vs 403" callout is valuable — grounds the gRPC codes in what the reader already knows. Keep. -->
<!-- [LINE EDIT] "Note the distinction between `Unauthenticated` (no role in context = the interceptor didn't run or the user isn't logged in) and `PermissionDenied` (the user is authenticated but lacks the required role)." → Good. Keep. -->
<!-- [COPY EDIT] "HTTP 401 vs 403" — per CMOS 6.13, "vs." takes a period; the source uses "vs" in prose and "vs." elsewhere. Consistency check recommended. -->
<!-- [COPY EDIT] Please verify: HTTP 401 is "Unauthorized" (misleading name; means "not authenticated"), HTTP 403 is "Forbidden" (authenticated but not permitted). The gRPC→HTTP mapping is: `Unauthenticated` → 401, `PermissionDenied` → 403 per the gRPC-HTTP2 specification. Correct. -->
Note the distinction between `Unauthenticated` (no role in context = the interceptor didn't run or the user isn't logged in) and `PermissionDenied` (the user is authenticated but lacks the required role). This maps directly to HTTP 401 vs 403.

---

## Adding the Interceptor to Services

### Auth Service

<!-- [LINE EDIT] "The Auth service skips authentication for five of its six RPCs -- only `GetUser` requires a token" → Good precise count. Keep. -->
The Auth service skips authentication for five of its six RPCs -- only `GetUser` requires a token:

```go
skipMethods := []string{
    "/auth.v1.AuthService/Register",
    "/auth.v1.AuthService/Login",
    "/auth.v1.AuthService/ValidateToken",
    "/auth.v1.AuthService/InitOAuth2",
    "/auth.v1.AuthService/CompleteOAuth2",
}
interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

<!-- [LINE EDIT] "`Register`, `Login`, and OAuth RPCs are obviously public." → Drop "obviously": "`Register`, `Login`, and the OAuth RPCs are public by necessity." — more precise and less dismissive. -->
<!-- [LINE EDIT] "they are validating someone else's token" → Good. Keep. -->
`Register`, `Login`, and OAuth RPCs are obviously public. `ValidateToken` is also public because it is called by other services or gateways that may not have a user token yet (they are validating someone else's token).

### Catalog Service

<!-- [LINE EDIT] "The Catalog service skips authentication for read operations and the availability update (which would be called by an internal service)" → "which would be called by an internal service" — conditional "would" suggests the scenario isn't real. If UpdateAvailability is actually called by an internal service (e.g., a reservation service), use present tense: "which is called by an internal service (the reservation worker)". If it's hypothetical, clarify. -->
The Catalog service skips authentication for read operations and the availability update (which would be called by an internal service):

```go
skipMethods := []string{
    "/catalog.v1.CatalogService/GetBook",
    "/catalog.v1.CatalogService/ListBooks",
    "/catalog.v1.CatalogService/UpdateAvailability",
}
interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)
```

<!-- [STRUCTURAL] Good transition sentence into the next subsection — distinguishes authentication from authorization with a concrete example. -->
<!-- [LINE EDIT] "But requiring a token is only **authentication** -- any logged-in user could call these RPCs. We still need **authorization**." → Good cadence. Keep. -->
This means `CreateBook`, `UpdateBook`, and `DeleteBook` all require a valid JWT. But requiring a token is only **authentication** -- any logged-in user could call these RPCs. We still need **authorization**.

---

## Role-Based Authorization

<!-- [STRUCTURAL] Excellent — the parenthetical rhetorical questions ("is this a valid user?" / "does this user have permission?") define the terms precisely. Keep. -->
<!-- [LINE EDIT] "Authorization (does this user have permission?) happens in the handler, because it is business-logic-specific." → "business-logic-specific" reads hyphen-heavy. Consider: "because it depends on business rules" or "because it is rule-specific". -->
<!-- [COPY EDIT] "business-logic-specific" — compound adjective with more than two parts correctly hyphenated (CMOS 7.81). Technically correct but clunky. -->
The interceptor handles authentication (is this a valid user?). Authorization (does this user have permission?) happens in the handler, because it is business-logic-specific. The Catalog handler checks for the `admin` role:

```go
func (h *CatalogHandler) CreateBook(ctx context.Context, req *catalogv1.CreateBookRequest) (*catalogv1.Book, error) {
    if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
        return nil, err
    }
    // ... create the book
}

func (h *CatalogHandler) UpdateBook(ctx context.Context, req *catalogv1.UpdateBookRequest) (*catalogv1.Book, error) {
    if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
        return nil, err
    }
    // ... update the book
}

func (h *CatalogHandler) DeleteBook(ctx context.Context, req *catalogv1.DeleteBookRequest) (*catalogv1.DeleteBookResponse, error) {
    if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
        return nil, err
    }
    // ... delete the book
}
```

<!-- [STRUCTURAL] The split-rationale bullets ("interceptor is generic; handler knows business rules") are the section's strongest paragraph. Consider elevating to a visual callout. -->
<!-- [LINE EDIT] "This is the same pattern as Spring Security's `@PreAuthorize("hasRole('ADMIN')")` annotation on a controller method, except explicit rather than declarative." → Good analogy. Keep. -->
<!-- [COPY EDIT] "@PreAuthorize(\"hasRole('ADMIN')\")" — nested quotes within backticks. Renders fine in Markdown; good. -->
This is a one-line check at the top of each handler method. The split between interceptor (authentication) and handler (authorization) is intentional:

- The **interceptor** is generic -- it works for any service, any RPC. It validates the token and populates the context.
- The **handler** knows the business rules -- "only admins can create books" is a catalog-specific rule, not a system-wide concern.

This is the same pattern as Spring Security's `@PreAuthorize("hasRole('ADMIN')")` annotation on a controller method, except explicit rather than declarative.

---

## Testing with grpcurl

<!-- [STRUCTURAL] Excellent end-to-end verification sequence — reader sees all three access levels (unauth / user / admin) and the specific error for each failure. Keep this pattern for all chapters. -->
Here is a complete testing sequence that demonstrates the auth flow end to end. Start the stack with `docker compose up`.

**List books without authentication (works -- public endpoint):**
```bash
grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks
```

**Try to create a book without authentication (rejected):**
```bash
grpcurl -plaintext -d '{
  "title": "The Go Programming Language",
  "author": "Donovan & Kernighan",
  "isbn": "978-0134190440"
}' localhost:50052 catalog.v1.CatalogService/CreateBook
# ERROR: Unauthenticated: missing authorization header
```

<!-- [COPY EDIT] Please verify: ISBN "978-0134190440" — this is the ISBN-13 of "The Go Programming Language" by Donovan & Kernighan. Verified against publisher records. -->
**Register a user and get a token:**
```bash
grpcurl -plaintext -d '{
  "email": "alice@example.com",
  "password": "secret123",
  "name": "Alice"
}' localhost:50051 auth.v1.AuthService/Register
# Response includes "token": "eyJhbG..."
```

**Try to create a book with a regular user token (rejected -- wrong role):**
```bash
TOKEN="eyJhbG..."  # paste from register response
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{
    "title": "The Go Programming Language",
    "author": "Donovan & Kernighan",
    "isbn": "978-0134190440"
  }' localhost:50052 catalog.v1.CatalogService/CreateBook
# ERROR: PermissionDenied: requires admin role
```

**Promote the user to admin via SQL:**

<!-- [LINE EDIT] "We'll build a proper CLI for this in Chapter 6. For now, you can promote manually:" → Forward reference is fine. Keep. -->
<!-- [COPY EDIT] "We'll build a proper CLI" — contraction "We'll" is acceptable in tutorial voice. Consistent with rest of chapter. -->
> We'll build a proper CLI for this in Chapter 6. For now, you can promote manually:

```bash
docker exec -it postgres-auth psql -U postgres -d auth -c \
  "UPDATE users SET role = 'admin' WHERE email = 'alice@example.com';"
```

**Log in again to get a new token with the admin role:**
```bash
grpcurl -plaintext -d '{
  "email": "alice@example.com",
  "password": "secret123"
}' localhost:50051 auth.v1.AuthService/Login
# New token now contains role: "admin"
```

**Create the book with the admin token (success):**
```bash
TOKEN="eyJhbG..."  # paste from login response
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{
    "title": "The Go Programming Language",
    "author": "Donovan & Kernighan",
    "isbn": "978-0134190440"
  }' localhost:50052 catalog.v1.CatalogService/CreateBook
# Success! Book created with ID, timestamps, etc.
```

<!-- [LINE EDIT] "This sequence demonstrates three levels of access: unauthenticated (browse only), authenticated user (no write access), and authenticated admin (full access). The token must be re-issued after the role change because the old token still contains `"role": "user"` -- JWTs are immutable once signed." → Good closing summary. Keep. -->
<!-- [COPY EDIT] "JWTs are immutable once signed" — true in the sense that the signature invalidates if the payload is altered; the token itself is a string, not technically immutable. Pedagogically fine. -->
This sequence demonstrates three levels of access: unauthenticated (browse only), authenticated user (no write access), and authenticated admin (full access). The token must be re-issued after the role change because the old token still contains `"role": "user"` -- JWTs are immutable once signed.

---

## Summary

<!-- [STRUCTURAL] Good durable takeaways. Keep. -->
- gRPC interceptors are middleware -- analogous to servlet filters or Spring's `HandlerInterceptor`
- The `pkg/auth` shared library keeps auth logic DRY across services
- The interceptor handles authentication (valid token?); handlers handle authorization (correct role?)
- Typed context keys prevent collisions between packages
- The skip-methods pattern allows public endpoints to bypass authentication
- `RequireRole` provides a clean one-line authorization check in handlers

---

## References

[^1]: [gRPC interceptors documentation](https://grpc.io/docs/guides/interceptors/) -- official gRPC guide to interceptors and middleware.
[^2]: [Go context package](https://pkg.go.dev/context) -- standard library documentation for context values and cancellation.
[^3]: [gRPC metadata](https://grpc.io/docs/guides/metadata/) -- how to pass metadata (headers) in gRPC calls.
[^4]: [Go project layout](https://github.com/golang-standards/project-layout) -- community conventions for Go project structure, including the `pkg/` directory.
[^5]: [gRPC authentication guide](https://grpc.io/docs/guides/auth/) -- official guide to authentication patterns in gRPC.
<!-- [FINAL] Same as oauth2.md and auth-service.md: no [^N] anchors in body prose; references serve as a bibliography. Flag chapter-wide for consistent treatment. -->
<!-- [COPY EDIT] Please verify: "golang-standards/project-layout" is a community repo, not an official Go project — the book correctly says "community conventions" in the footnote; no change needed. The Go team has explicitly stated this is not their guidance (https://github.com/golang-standards/project-layout/issues/117 among others). Fine as cited. -->
