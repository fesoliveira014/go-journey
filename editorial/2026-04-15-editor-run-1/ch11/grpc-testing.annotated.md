# 11.3 gRPC Testing with bufconn

<!-- [STRUCTURAL] Section is well-scoped: motivation → mechanism (bufconn) → setup helper → interceptor tests → combining with Testcontainers → auth-service parallel → summary. Good analogy to 11.2. One gap: no discussion of streaming RPCs, which is fair for this project scope but consider a brief "out of scope" mention. -->

## The gap in current unit tests

<!-- [LINE EDIT] "The unit tests written so far call handler methods directly:" — fine. -->
The unit tests written so far call handler methods directly:

```go
h := handler.NewCatalogHandler(svc)
resp, err := h.CreateBook(ctx, req)
```

<!-- [LINE EDIT] "This is fast and useful for testing business logic in isolation, but it bypasses a significant part of the system." — fine. -->
<!-- [LINE EDIT] "A real gRPC request travels through several layers before it reaches a handler method:" — fine. -->
This is fast and useful for testing business logic in isolation, but it bypasses a significant part of the system. A real gRPC request travels through several layers before it reaches a handler method:

<!-- [COPY EDIT] Numbered list mixes "The client serialises..." (UK) with later US spelling. Normalize. -->
<!-- [COPY EDIT] "serialises" → "serializes" (US). -->
<!-- [COPY EDIT] "protobuf wire format" — lowercase; Google style actually uses "protobuf" lowercase as a shortened adjective form, but Protocol Buffers as the full name. Acceptable as is, but the lowercase is slightly inconsistent with "Protobuf" in index.md. Align throughout chapter. -->
<!-- [COPY EDIT] Item 4: "Registered server interceptors run — in the library system, that means `UnaryAuthInterceptor`, which reads the `authorization` metadata header, validates the JWT, and either passes the request down the chain or returns `codes.Unauthenticated`." — 39 words; acceptable. -->
1. The client serialises the request struct into protobuf wire format.
2. The bytes cross a transport (TCP, Unix socket, etc.).
3. The server deserialises the bytes back into a Go struct.
4. Registered server interceptors run — in the library system, that means `UnaryAuthInterceptor`, which reads the `authorization` metadata header, validates the JWT, and either passes the request down the chain or returns `codes.Unauthenticated`.
5. gRPC metadata (request headers) is propagated from the incoming context.
6. The handler runs and returns a response or a gRPC `status.Status` error.
7. The response is serialised and sent back to the client.
<!-- [COPY EDIT] Item 7 "serialised" → "serialized" (US). -->
<!-- [COPY EDIT] Item 2: "etc." — ensure comma after per CMOS 6.20 in a sentence continuation; here in parentheses, OK. -->

<!-- [LINE EDIT] "When you call `h.CreateBook(ctx, req)` directly, none of steps 1–4 execute. The interceptor is never invoked, so a test that sends no token will still succeed. The gRPC status codes your handler produces are not tested as they appear over the wire. Metadata is whatever you manually put into the context, not what the gRPC runtime would propagate." — fine. -->
<!-- [COPY EDIT] "over the wire" — idiom, retain. -->
When you call `h.CreateBook(ctx, req)` directly, none of steps 1–4 execute. The interceptor is never invoked, so a test that sends no token will still succeed. The gRPC status codes your handler produces are not tested as they appear over the wire. Metadata is whatever you manually put into the context, not what the gRPC runtime would propagate.

<!-- [LINE EDIT] "For a system where authentication, authorisation, and correct error codes are security properties, that gap matters. The `bufconn` package closes it without requiring any external process or open network port." — fine. -->
<!-- [COPY EDIT] "authorisation" → "authorization" (US spelling). -->
For a system where authentication, authorisation, and correct error codes are security properties, that gap matters. The `bufconn` package closes it without requiring any external process or open network port.

---

## How bufconn works

<!-- [LINE EDIT] "`google.golang.org/grpc/test/bufconn` provides a single type: `Listener`. Under the hood it is a pair of in-memory byte buffers that implement `net.Listener` and `net.Conn`. When a goroutine writes to one end, another goroutine reads from the other — exactly like a network socket, but with no kernel involvement and no port allocation." 56 words across three sentences; acceptable. -->
`google.golang.org/grpc/test/bufconn` provides a single type: `Listener`. Under the hood it is a pair of in-memory byte buffers that implement `net.Listener` and `net.Conn`. When a goroutine writes to one end, another goroutine reads from the other — exactly like a network socket, but with no kernel involvement and no port allocation.

<!-- [COPY EDIT] ASCII diagram uses hyphen + pipe art; consistent style with index.md pyramid. Fine. -->
```
+------------------------------------------------------------------+
|                         test process                             |
|                                                                  |
|  +------------------+  in-memory pipe  +---------------------+  |
|  |   gRPC client    | <--------------> |   gRPC server       |  |
|  |  (test code)     |  bufconn.Listen  |  + interceptors     |  |
|  +------------------+                  |  + handlers         |  |
|                                        +---------------------+  |
+------------------------------------------------------------------+
```

<!-- [LINE EDIT] "Because the bytes still flow through the full gRPC stack on both sides — encoding, decoding, header frames, trailers — the behaviour is identical to what you would observe over a real TCP connection. The only thing removed is the OS network stack." — fine. -->
<!-- [COPY EDIT] "behaviour" → "behavior" (US). -->
Because the bytes still flow through the full gRPC stack on both sides — encoding, decoding, header frames, trailers — the behaviour is identical to what you would observe over a real TCP connection. The only thing removed is the OS network stack.

<!-- [LINE EDIT] "To connect a gRPC client to a `bufconn.Listener` you supply a custom dialer via `grpc.WithContextDialer`. Instead of resolving a host name and opening a TCP socket, the dialer calls `lis.DialContext`, which returns an in-memory connection to the server." — fine. -->
<!-- [COPY EDIT] "host name" — CMOS 7.85: "hostname" (closed compound) is current standard. -->
To connect a gRPC client to a `bufconn.Listener` you supply a custom dialer via `grpc.WithContextDialer`. Instead of resolving a host name and opening a TCP socket, the dialer calls `lis.DialContext`, which returns an in-memory connection to the server.

---

## Setting up a bufconn server

<!-- [LINE EDIT] "The following helper encapsulates everything needed to start a gRPC server over bufconn and return a connected client. Place it in a `_test.go` file that belongs to the `handler_test` package." — fine. -->
The following helper encapsulates everything needed to start a gRPC server over bufconn and return a connected client. Place it in a `_test.go` file that belongs to the `handler_test` package.

```go
//go:build integration

package handler_test

import (
    "context"
    "net"
    "testing"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"

    catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
    pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
    "github.com/fesoliveira014/library-system/services/catalog/internal/handler"
    "github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

const bufSize = 1024 * 1024

func startCatalogServer(t *testing.T, svc *service.CatalogService, jwtSecret string) catalogv1.CatalogServiceClient {
    t.Helper()
    lis := bufconn.Listen(bufSize)

    srv := grpc.NewServer(
        grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, nil)),
    )
    catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))

    go func() { srv.Serve(lis) }()
    t.Cleanup(func() { srv.GracefulStop() })

    conn, err := grpc.NewClient("passthrough:///bufconn",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("dial bufconn: %v", err)
    }
    t.Cleanup(func() { conn.Close() })

    return catalogv1.NewCatalogServiceClient(conn)
}
```

<!-- [STRUCTURAL] 11.5 uses `grpc.DialContext` with the "bufnet" address; this section uses `grpc.NewClient` with "passthrough:///bufconn". Two different patterns in the same book. Align: `grpc.NewClient` is the current recommended API (NewClient was introduced in grpc-go 1.64 as the replacement for Dial/DialContext). Use `grpc.NewClient` throughout. -->
<!-- [COPY EDIT] Please verify: the "passthrough:///bufconn" target string is correct for `grpc.NewClient`. Current API expects `target` like `"passthrough:bufnet"` or `"passthrough:///bufnet"`. Double-check against grpc-go current docs. -->

Walk through each part:

<!-- [LINE EDIT] "**`bufconn.Listen(bufSize)`** — creates the in-memory listener. The buffer size (1 MiB here) is the maximum amount of data that can be in flight between client and server at one time. For tests this value is rarely a bottleneck; 1 MiB is a safe default." — fine. -->
<!-- [COPY EDIT] "1 MiB" — correct binary unit symbol (IEC). -->
**`bufconn.Listen(bufSize)`** — creates the in-memory listener. The buffer size (1 MiB here) is the maximum amount of data that can be in flight between client and server at one time. For tests this value is rarely a bottleneck; 1 MiB is a safe default.

<!-- [LINE EDIT] "**`grpc.NewServer(grpc.UnaryInterceptor(...))`** — creates a real gRPC server with the same interceptor chain that runs in production. Passing `pkgauth.UnaryAuthInterceptor(jwtSecret, nil)` means every unary RPC goes through JWT validation before reaching the handler. This is the critical difference from calling the handler directly." — fine. -->
**`grpc.NewServer(grpc.UnaryInterceptor(...))`** — creates a real gRPC server with the same interceptor chain that runs in production. Passing `pkgauth.UnaryAuthInterceptor(jwtSecret, nil)` means every unary RPC goes through JWT validation before reaching the handler. This is the critical difference from calling the handler directly.

<!-- [LINE EDIT] "**`catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))`** — registers the handler with the server, exactly as `main.go` does." — fine. -->
**`catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))`** — registers the handler with the server, exactly as `main.go` does.

<!-- [LINE EDIT] "**`go func() { srv.Serve(lis) }()`** — starts the server in a goroutine. `Serve` blocks until the server stops, so it must not run on the test goroutine." — fine. -->
<!-- [COPY EDIT] "test goroutine" — fine. -->
**`go func() { srv.Serve(lis) }()`** — starts the server in a goroutine. `Serve` blocks until the server stops, so it must not run on the test goroutine.

<!-- [LINE EDIT] "**`t.Cleanup(func() { srv.GracefulStop() })`** — registers a cleanup function that runs when the test (or subtest) completes. `GracefulStop` finishes in-flight RPCs before shutting down. Using `t.Cleanup` instead of `defer` is idiomatic for test helpers because `defer` in a helper function runs when the helper returns, not when the test finishes." 47 words; acceptable. -->
**`t.Cleanup(func() { srv.GracefulStop() })`** — registers a cleanup function that runs when the test (or subtest) completes. `GracefulStop` finishes in-flight RPCs before shutting down. Using `t.Cleanup` instead of `defer` is idiomatic for test helpers because `defer` in a helper function runs when the helper returns, not when the test finishes.

<!-- [LINE EDIT] "**`grpc.NewClient("passthrough:///bufconn", ...)`** — creates the client connection. The scheme `passthrough:///` tells gRPC's name resolver to use the address string as-is and not attempt DNS lookup. The actual address (`"bufconn"`) is irrelevant because the custom dialer ignores it." — fine. -->
**`grpc.NewClient("passthrough:///bufconn", ...)`** — creates the client connection. The scheme `passthrough:///` tells gRPC's name resolver to use the address string as-is and not attempt DNS lookup. The actual address (`"bufconn"`) is irrelevant because the custom dialer ignores it.

<!-- [LINE EDIT] "**`grpc.WithContextDialer(...)`** — supplies the dialer that routes connections through the in-memory listener instead of the OS network stack." — fine. -->
**`grpc.WithContextDialer(...)`** — supplies the dialer that routes connections through the in-memory listener instead of the OS network stack.

<!-- [LINE EDIT] "**`grpc.WithTransportCredentials(insecure.NewCredentials())`** — disables TLS. The bufconn pipe is already within the same process; TLS adds no security benefit in tests and would require certificate setup." — fine. -->
**`grpc.WithTransportCredentials(insecure.NewCredentials())`** — disables TLS. The bufconn pipe is already within the same process; TLS adds no security benefit in tests and would require certificate setup.

<!-- [LINE EDIT] "The second `t.Cleanup` closes the client connection after the test completes, releasing the underlying resources." — fine. -->
The second `t.Cleanup` closes the client connection after the test completes, releasing the underlying resources.

<!-- [LINE EDIT] "The `//go:build integration` constraint keeps these tests out of the normal `go test ./...` run. Run them explicitly:" — fine. -->
The `//go:build integration` constraint keeps these tests out of the normal `go test ./...` run. Run them explicitly:

```sh
go test -tags integration ./services/catalog/internal/handler/...
```

---

## Testing interceptor behaviour

<!-- [COPY EDIT] Heading: "behaviour" → "behavior" (US). -->
<!-- [LINE EDIT] "With the helper above, testing the authentication interceptor becomes straightforward. Two cases cover the most important paths." — fine. -->
With the helper above, testing the authentication interceptor becomes straightforward. Two cases cover the most important paths.

### Unauthenticated request

```go
func TestCreateBook_Unauthenticated(t *testing.T) {
    repo := repository.NewInMemoryBookRepository()
    svc := service.NewCatalogService(repo, &noopPublisher{})
    client := startCatalogServer(t, svc, "test-secret")

    _, err := client.CreateBook(context.Background(), &catalogv1.CreateBookRequest{
        Title:       "Test",
        Author:      "Author",
        Isbn:        "978-0000000001",
        TotalCopies: 1,
    })

    if status.Code(err) != codes.Unauthenticated {
        t.Errorf("expected Unauthenticated, got %v", err)
    }
}
```

<!-- [LINE EDIT] "The request carries no `authorization` metadata header. `UnaryAuthInterceptor` finds no token, returns `status.Error(codes.Unauthenticated, ...)`, and the handler never runs. `status.Code(err)` extracts the gRPC status code from the error returned by the client stub. Had you called `h.CreateBook` directly, `err` would be `nil` and this test would fail to catch the missing auth check." — fine. -->
The request carries no `authorization` metadata header. `UnaryAuthInterceptor` finds no token, returns `status.Error(codes.Unauthenticated, ...)`, and the handler never runs. `status.Code(err)` extracts the gRPC status code from the error returned by the client stub. Had you called `h.CreateBook` directly, `err` would be `nil` and this test would fail to catch the missing auth check.

### Authenticated request

```go
func TestCreateBook_WithAuth(t *testing.T) {
    repo := repository.NewInMemoryBookRepository()
    svc := service.NewCatalogService(repo, &noopPublisher{})
    client := startCatalogServer(t, svc, "test-secret")

    token, _ := pkgauth.GenerateToken(uuid.New(), "admin", "test-secret", time.Hour)
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    resp, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
        Title:       "Test Book",
        Author:      "Author",
        Isbn:        "978-0000000001",
        TotalCopies: 3,
    })
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }
    if resp.GetTitle() != "Test Book" {
        t.Errorf("expected title %q, got %q", "Test Book", resp.GetTitle())
    }
}
```

<!-- [FINAL] `token, _ := pkgauth.GenerateToken(...)` discards an error on a security-sensitive path. Consider replacing with explicit `if err != nil { t.Fatalf(...) }` for pedagogical clarity. -->

<!-- [LINE EDIT] "`metadata.Pairs` builds gRPC metadata — the equivalent of HTTP headers. `metadata.NewOutgoingContext` attaches the metadata to the context so the client stub includes it in the request. The interceptor reads the `authorization` header, validates the JWT signed with `"test-secret"`, extracts the claims, and allows the request to proceed." — fine. -->
`metadata.Pairs` builds gRPC metadata — the equivalent of HTTP headers. `metadata.NewOutgoingContext` attaches the metadata to the context so the client stub includes it in the request. The interceptor reads the `authorization` header, validates the JWT signed with `"test-secret"`, extracts the claims, and allows the request to proceed.

<!-- [LINE EDIT] "Note that `pkgauth.GenerateToken` uses the same `"test-secret"` that was passed to `startCatalogServer`. If the secrets differ, the interceptor will return `codes.Unauthenticated` even with a valid-looking token — worth testing as a third case." — fine. -->
<!-- [COPY EDIT] "valid-looking token" — compound adjective before noun; correct. -->
Note that `pkgauth.GenerateToken` uses the same `"test-secret"` that was passed to `startCatalogServer`. If the secrets differ, the interceptor will return `codes.Unauthenticated` even with a valid-looking token — worth testing as a third case.

The full import list for these tests:

```go
import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"

    catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
    pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
    "github.com/fesoliveira014/library-system/services/catalog/internal/repository"
    "github.com/fesoliveira014/library-system/services/catalog/internal/service"
)
```

---

## Combining bufconn with testcontainers

<!-- [COPY EDIT] Heading: "testcontainers" → "Testcontainers" (product brand in prose). -->
<!-- [LINE EDIT] "The mock repository used above keeps data in memory. For a full integration test you want a real PostgreSQL instance. Testcontainers (covered in Section 11.2) provides one. Combining the two gives you an end-to-end path through every layer:" — fine. -->
<!-- [COPY EDIT] "Section 11.2" — capital S on cross-reference; CMOS 8.180 allows; consistent with later cross-refs. -->
The mock repository used above keeps data in memory. For a full integration test you want a real PostgreSQL instance. Testcontainers (covered in Section 11.2) provides one. Combining the two gives you an end-to-end path through every layer:

```
gRPC client -> protobuf encode -> bufconn -> interceptor -> handler -> service -> repository -> PostgreSQL
```

```go
func TestCreateBook_Integration(t *testing.T) {
    // Start a real PostgreSQL container (see Section 11.2 for setupPostgres).
    db := setupPostgres(t)

    repo := repository.NewBookRepository(db)
    svc := service.NewCatalogService(repo, &noopPublisher{})
    client := startCatalogServer(t, svc, "test-secret")

    token, _ := pkgauth.GenerateToken(uuid.New(), "admin", "test-secret", time.Hour)
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    resp, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
        Title:       "The Go Programming Language",
        Author:      "Donovan & Kernighan",
        Isbn:        "978-0134190440",
        TotalCopies: 5,
    })
    if err != nil {
        t.Fatalf("CreateBook: %v", err)
    }
    if resp.GetIsbn() != "978-0134190440" {
        t.Errorf("unexpected ISBN: %s", resp.GetIsbn())
    }

    // Verify persistence: fetch the book back through the same client.
    getResp, err := client.GetBook(ctx, &catalogv1.GetBookRequest{Id: resp.GetId()})
    if err != nil {
        t.Fatalf("GetBook: %v", err)
    }
    if getResp.GetTitle() != "The Go Programming Language" {
        t.Errorf("unexpected title after round-trip: %s", getResp.GetTitle())
    }
}
```

<!-- [LINE EDIT] "The `setupPostgres` helper from Section 11.2 starts the container, runs migrations, and registers `t.Cleanup` to terminate the container. Because `startCatalogServer` also registers its cleanup via `t.Cleanup`, the shutdown order is correct: the gRPC server stops first, then the database container terminates. Go runs `t.Cleanup` functions in LIFO order — last registered, first called." 53 words across three sentences; acceptable. -->
<!-- [COPY EDIT] "LIFO order — last registered, first called" — explaining acronym inline is helpful; keep. -->
The `setupPostgres` helper from Section 11.2 starts the container, runs migrations, and registers `t.Cleanup` to terminate the container. Because `startCatalogServer` also registers its cleanup via `t.Cleanup`, the shutdown order is correct: the gRPC server stops first, then the database container terminates. Go runs `t.Cleanup` functions in LIFO order — last registered, first called.

<!-- [LINE EDIT] "This test validates things that no unit test can:" — fine. -->
This test validates things that no unit test can:

- The SQL schema accepts and stores the data correctly.
- The repository translates between the domain model and the database row without loss.
- Protobuf field mapping survives a full encode/decode round-trip.
- The auth interceptor correctly passes claims through the context so the handler can read the caller's identity.

<!-- [LINE EDIT] "The trade-off is speed. Starting a container takes a few seconds. Keep these tests behind the `integration` build tag and run them in CI, not on every file save." — fine. -->
The trade-off is speed. Starting a container takes a few seconds. Keep these tests behind the `integration` build tag and run them in CI, not on every file save.

---

## Auth service bufconn tests

<!-- [LINE EDIT] "The pattern is identical for the auth service, but the interceptor configuration differs. The auth service has endpoints that must be reachable without a valid JWT — you cannot authenticate if you need a token to call `Register` or `Login`. The `skipMethods` parameter of `UnaryAuthInterceptor` lists the fully-qualified method names that bypass the check:" 51 words across three sentences; fine. -->
<!-- [COPY EDIT] "fully-qualified" — compound adjective before noun; correct. "fully qualified" (two words) when predicative. -->
The pattern is identical for the auth service, but the interceptor configuration differs. The auth service has endpoints that must be reachable without a valid JWT — you cannot authenticate if you need a token to call `Register` or `Login`. The `skipMethods` parameter of `UnaryAuthInterceptor` lists the fully-qualified method names that bypass the check:

```go
func startAuthServer(t *testing.T, svc *authservice.AuthService, jwtSecret string) authv1.AuthServiceClient {
    t.Helper()
    lis := bufconn.Listen(bufSize)

    skipMethods := []string{
        "/auth.v1.AuthService/Register",
        "/auth.v1.AuthService/Login",
    }

    srv := grpc.NewServer(
        grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)),
    )
    authv1.RegisterAuthServiceServer(srv, handler.NewAuthHandler(svc))

    go func() { srv.Serve(lis) }()
    t.Cleanup(func() { srv.GracefulStop() })

    conn, err := grpc.NewClient("passthrough:///bufconn",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("dial bufconn: %v", err)
    }
    t.Cleanup(func() { conn.Close() })

    return authv1.NewAuthServiceClient(conn)
}
```

<!-- [LINE EDIT] "With the helper in place, you can write a test that exercises the full register -> login -> validate-token flow in a single test function:" — fine. -->
<!-- [COPY EDIT] "register -> login -> validate-token" — use en dash or arrow Unicode? Text arrows are fine; be consistent across chapter. Recommend "register → login → validate-token" using Unicode arrow OR keep ASCII consistently. -->
With the helper in place, you can write a test that exercises the full register -> login -> validate-token flow in a single test function:

```go
func TestAuthFlow(t *testing.T) {
    db := setupPostgres(t)
    repo := authrepository.NewUserRepository(db)
    svc := authservice.NewAuthService(repo, "test-secret")
    client := startAuthServer(t, svc, "test-secret")

    // Register — no token required.
    _, err := client.Register(context.Background(), &authv1.RegisterRequest{
        Email:    "alice@example.com",
        Password: "hunter2",
    })
    if err != nil {
        t.Fatalf("Register: %v", err)
    }

    // Login — no token required; returns a signed JWT.
    loginResp, err := client.Login(context.Background(), &authv1.LoginRequest{
        Email:    "alice@example.com",
        Password: "hunter2",
    })
    if err != nil {
        t.Fatalf("Login: %v", err)
    }
    if loginResp.GetToken() == "" {
        t.Fatal("expected non-empty token")
    }

    // ValidateToken — token required; use the one we just received.
    md := metadata.Pairs("authorization", "Bearer "+loginResp.GetToken())
    ctx := metadata.NewOutgoingContext(context.Background(), md)

    validateResp, err := client.ValidateToken(ctx, &authv1.ValidateTokenRequest{
        Token: loginResp.GetToken(),
    })
    if err != nil {
        t.Fatalf("ValidateToken: %v", err)
    }
    if validateResp.GetEmail() != "alice@example.com" {
        t.Errorf("unexpected email in claims: %s", validateResp.GetEmail())
    }
}
```

<!-- [LINE EDIT] "Three assertions in one test are acceptable here because the steps are sequential and dependent: you cannot log in without registering, and you cannot validate a token without logging in. Splitting them into separate tests would require repeated setup. If any step fails, `t.Fatalf` halts the test immediately, so a failure message pinpoints exactly which step broke." 57 words across three sentences; acceptable. -->
Three assertions in one test are acceptable here because the steps are sequential and dependent: you cannot log in without registering, and you cannot validate a token without logging in. Splitting them into separate tests would require repeated setup. If any step fails, `t.Fatalf` halts the test immediately, so a failure message pinpoints exactly which step broke.

<!-- [LINE EDIT] "The `ValidateToken` call also verifies that the token the auth service issues is accepted by the same interceptor it uses for protected endpoints — a circular check that confirms the signing key is consistent throughout the service." 37 words; fine. -->
<!-- [COPY EDIT] "circular check" — idiomatic; fine. -->
The `ValidateToken` call also verifies that the token the auth service issues is accepted by the same interceptor it uses for protected endpoints — a circular check that confirms the signing key is consistent throughout the service.

---

## Summary

| Approach | Interceptors run | Real protobuf | Real database | Speed |
|---|---|---|---|---|
| `h.CreateBook(ctx, req)` | No | No | Optional | Fast |
| bufconn + mock repo | Yes | Yes | No | Fast |
| bufconn + testcontainers | Yes | Yes | Yes | Slow |

<!-- [COPY EDIT] Table row "bufconn + testcontainers" — capitalize "Testcontainers" (product). -->

<!-- [LINE EDIT] "Use direct handler calls for logic-heavy unit tests where you want fast feedback and fine-grained control over inputs. Use bufconn with a mock repository to test authentication, error code mapping, and metadata propagation. Use bufconn with testcontainers for integration tests that verify the full stack before merging." 47 words across three sentences. Rhythm is effective (three parallel "Use"). Keep. -->
<!-- [COPY EDIT] "error code mapping" → "error-code mapping" (compound adjective, CMOS 7.81). -->
Use direct handler calls for logic-heavy unit tests where you want fast feedback and fine-grained control over inputs. Use bufconn with a mock repository to test authentication, error code mapping, and metadata propagation. Use bufconn with testcontainers for integration tests that verify the full stack before merging.

---

[^1]: bufconn package reference — https://pkg.go.dev/google.golang.org/grpc/test/bufconn
[^2]: gRPC Go testing guide — https://grpc.io/docs/languages/go/testing/
<!-- [COPY EDIT] Please verify: both footnote URLs still resolve. -->
