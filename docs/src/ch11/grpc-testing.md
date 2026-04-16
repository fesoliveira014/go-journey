# 11.3 gRPC Testing with bufconn

## The gap in current unit tests

The unit tests written so far call handler methods directly:

```go
h := handler.NewCatalogHandler(svc)
resp, err := h.CreateBook(ctx, req)
```

This is fast and useful for testing business logic in isolation, but it bypasses a significant part of the system. A real gRPC request travels through several layers before it reaches a handler method:

1. The client serializes the request struct into protobuf wire format.
2. The bytes cross a transport (TCP, Unix socket, etc.).
3. The server deserializes the bytes back into a Go struct.
4. Registered server interceptors run—in the library system, that means `UnaryAuthInterceptor`, which reads the `authorization` metadata header, validates the JWT, and either passes the request down the chain or returns `codes.Unauthenticated`.
5. gRPC metadata (request headers) is propagated from the incoming context.
6. The handler runs and returns a response or a gRPC `status.Status` error.
7. The response is serialized and sent back to the client.

When you call `h.CreateBook(ctx, req)` directly, none of steps 1–4 execute. The interceptor is never invoked, so a test that sends no token will still succeed. The gRPC status codes your handler produces are not tested as they appear over the wire. Metadata is whatever you manually put into the context, not what the gRPC runtime would propagate.

For a system where authentication, authorization, and correct error codes are security properties, this gap matters. The `bufconn` package closes it without requiring any external process or open network port.

---

## How bufconn works

`google.golang.org/grpc/test/bufconn` provides a single type: `Listener`. Under the hood it is a pair of in-memory byte buffers that implement `net.Listener` and `net.Conn`. When a goroutine writes to one end, another goroutine reads from the other—exactly like a network socket, but with no kernel involvement and no port allocation.

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

Because the bytes still flow through the full gRPC stack on both sides—encoding, decoding, header frames, trailers—the behavior is identical to what you would observe over a real TCP connection. The only thing removed is the OS network stack.

To connect a gRPC client to a `bufconn.Listener` you supply a custom dialer via `grpc.WithContextDialer`. Instead of resolving a hostname and opening a TCP socket, the dialer calls `lis.DialContext`, which returns an in-memory connection to the server.

---

## Setting up a bufconn server

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

Walk through each part:

**`bufconn.Listen(bufSize)`**—creates the in-memory listener. The buffer size (1 MiB here) is the maximum amount of data that can be in flight between client and server at any one time. For tests this value is rarely a bottleneck; 1 MiB is a safe default.

**`grpc.NewServer(grpc.UnaryInterceptor(...))`**—creates a real gRPC server with the same interceptor chain that runs in production. Passing `pkgauth.UnaryAuthInterceptor(jwtSecret, nil)` means every unary RPC goes through JWT validation before reaching the handler. This is the critical difference from calling the handler directly.

**`catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))`**—registers the handler with the server, exactly as `main.go` does.

**`go func() { srv.Serve(lis) }()`**—starts the server in a goroutine. `Serve` blocks until the server stops, so it must not run on the test goroutine.

**`t.Cleanup(func() { srv.GracefulStop() })`**—registers a cleanup function that runs when the test (or subtest) completes. `GracefulStop` finishes in-flight RPCs before shutting down. Using `t.Cleanup` rather than `defer` is idiomatic for test helpers because `defer` in a helper runs when the helper returns, not when the test finishes.

**`grpc.NewClient("passthrough:///bufconn", ...)`**—creates the client connection. The scheme `passthrough:///` tells gRPC's name resolver to use the address string as-is and not attempt DNS lookup. The actual address (`"bufconn"`) is irrelevant because the custom dialer ignores it.

**`grpc.WithContextDialer(...)`**—supplies the dialer that routes connections through the in-memory listener instead of the OS network stack.

**`grpc.WithTransportCredentials(insecure.NewCredentials())`**—disables TLS. The bufconn pipe is already within the same process; TLS adds no security benefit in tests and would require certificate setup.

The second `t.Cleanup` closes the client connection after the test completes, releasing the underlying resources.

The `//go:build integration` constraint keeps these tests out of the normal `go test ./...` run. Run them explicitly:

```sh
go test -tags integration ./services/catalog/internal/handler/...
```

---

## Testing interceptor behavior

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

`metadata.Pairs` builds gRPC metadata—the equivalent of HTTP headers. `metadata.NewOutgoingContext` attaches the metadata to the context so the client stub includes it in the request. The interceptor reads the `authorization` header, validates the JWT signed with `"test-secret"`, extracts the claims, and allows the request to proceed.

Note that `pkgauth.GenerateToken` uses the same `"test-secret"` that was passed to `startCatalogServer`. If the secrets differ, the interceptor will return `codes.Unauthenticated` even with a valid-looking token—worth testing as a third case.

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

## Combining bufconn with Testcontainers

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

The `setupPostgres` helper from Section 11.2 starts the container, runs migrations, and registers `t.Cleanup` to terminate the container. Because `startCatalogServer` also registers its cleanup via `t.Cleanup`, the shutdown order is correct: the gRPC server stops first, then the database container terminates. Go runs `t.Cleanup` functions in LIFO order—last registered, first called.

This test validates things that no unit test can:

- The SQL schema accepts and stores the data correctly.
- The repository translates between the domain model and the database row without loss.
- Protobuf field mapping survives a full encode/decode round-trip.
- The auth interceptor correctly passes claims through the context so the handler can read the caller's identity.

The trade-off is speed. Starting a container takes a few seconds. Keep these tests behind the `integration` build tag and run them in CI, not on every file save.

---

## Auth service bufconn tests

The pattern is identical for the Auth Service, but the interceptor configuration differs. The Auth Service has endpoints that must be reachable without a valid JWT—you cannot authenticate if you need a token to call `Register` or `Login`. The `skipMethods` parameter of `UnaryAuthInterceptor` lists the fully-qualified method names that bypass the check:

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

With the helper in place, you can write a test that exercises the full register → login → validate-token flow in a single test function:

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

Three assertions in one test are acceptable here because the steps are sequential and dependent—you cannot log in without registering, and you cannot validate a token without logging in. Splitting them into separate tests would require repeated setup. If any step fails, `t.Fatalf` halts the test immediately, so a failure message pinpoints exactly which step broke.

The `ValidateToken` call also verifies that the token the Auth Service issues is accepted by the same interceptor it uses for protected endpoints—a circular check that confirms the signing key is consistent throughout the service.

---

## Summary

| Approach | Interceptors run | Real protobuf | Real database | Speed |
|---|---|---|---|---|
| `h.CreateBook(ctx, req)` | No | No | Optional | Fast |
| bufconn + mock repo | Yes | Yes | No | Fast |
| bufconn + Testcontainers | Yes | Yes | Yes | Slow |

Use direct handler calls for logic-heavy unit tests where you want fast feedback and fine-grained control over inputs. Use bufconn with a mock repository to test authentication, error-code mapping, and metadata propagation. Use bufconn with Testcontainers for integration tests that verify the full stack before merging.

---

[^1]: bufconn package reference—https://pkg.go.dev/google.golang.org/grpc/test/bufconn
[^2]: gRPC Go testing guide—https://grpc.io/docs/languages/go/testing/
