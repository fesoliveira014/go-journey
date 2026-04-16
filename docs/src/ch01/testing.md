## 1.4 Testing

Go's testing story is opinionated and minimal. There is no JUnit, no TestNG, no pytest—just the standard library's `testing` package and a handful of conventions baked into the toolchain. If you are used to writing `@Test` annotations or `assert.assertEquals`, you will immediately notice the absence of assertions. Go's philosophy is that your test functions are ordinary code, and `if` is perfectly capable of expressing a failure condition.

---

### The `_test.go` Convention

The Go toolchain identifies test files by their name: Any file ending in `_test.go` is excluded from the production build and only compiled when running tests. There is no annotation, no test runner configuration—the filename is the contract.

Inside a test file, test functions follow a strict signature:

```go
func TestXxx(t *testing.T) { ... }
```

The function must start with `Test`, the next character must be uppercase, and the parameter must be `*testing.T`. The `T` type provides methods for logging, marking failures, and stopping execution:

| Method | Behavior | Java/Kotlin analogue |
|---|---|---|
| `t.Errorf(...)` | Marks the test as failed, continues running | `fail(message)`—soft |
| `t.Fatalf(...)` | Marks the test as failed, stops immediately | `fail(message)`—hard |
| `t.Logf(...)` | Logs a message, only visible with `-v` | `System.out.println` |

These three methods are the only assertion mechanism you get by default. This constraint forces you to write precise, readable failure messages instead of relying on a framework to format them for you. Many Go projects add `github.com/stretchr/testify/assert` for convenience, but the standard library suffices and is what this project uses.[^1]

#### Package Naming: `_test` Suffix

Notice the test files in this project use `package handler_test` rather than `package handler`. This is a Go idiom for **black-box testing**: the test package sits outside the package under test and can only access exported identifiers. It catches the common mistake of testing internal state instead of the public contract.

If you need to access unexported identifiers in a test, use `package handler` (no suffix). Both styles are valid and can coexist in the same directory.

---

### Running Tests

```bash
go test ./...                   # run all tests in all packages
go test -v ./...                # verbose: print each test name and result
go test -race ./...             # enable the data race detector
go test -cover ./...            # print a coverage summary per package
```

The `-race` flag instruments the binary to detect concurrent memory accesses that are not properly synchronized. It has a runtime cost (roughly 2–20× slowdown) but catches genuine concurrency bugs that are otherwise nearly impossible to reproduce. Run it in CI, even if you skip it locally.

`./...` is a Go path wildcard meaning "this module and all packages recursively below it." Think of it as the Go equivalent of Maven's `mvn test` applied to all submodules at once.

---

### The `httptest` Package

Testing an HTTP handler normally means spinning up a real server, making a real network call, and tearing down afterward. Go eliminates this with `net/http/httptest`.[^3]

- `httptest.NewRequest(method, target, body)`—creates an `*http.Request` suitable for passing directly to a handler, without a real TCP connection.
- `httptest.NewRecorder()`—creates a `*httptest.ResponseRecorder` that implements `http.ResponseWriter` and captures the response status, headers, and body in memory.

This means you can test a handler as a plain function call:

```go
req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
rec := httptest.NewRecorder()
handler.Health(rec, req)
// rec.Code, rec.Body, rec.Header() are all available now
```

No ports, no goroutines, no teardown. This is the standard Go pattern. If you've used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical—but the implementation is lighter because the handler is already just a function.

---

### Table-Driven Tests

The most idiomatic way to write parameterized tests in Go is the **table-driven** pattern.[^2] Rather than writing one test function per case, you define a slice of test cases and range over them. This is Go's answer to JUnit's `@ParameterizedTest` or pytest's `@pytest.mark.parametrize`.

Here is the health handler rewritten as a table-driven test:

```go
package handler_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func TestHealthHandler(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        wantStatus     int
        wantBody       string
    }{
        {
            name:       "GET returns 200 with ok body",
            method:     http.MethodGet,
            wantStatus: http.StatusOK,
            wantBody:   "{\"status\":\"ok\"}\n",
        },
        {
            name:       "POST returns 405",
            method:     http.MethodPost,
            wantStatus: http.StatusMethodNotAllowed,
            wantBody:   "",
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            req := httptest.NewRequest(tc.method, "/healthz", nil)
            rec := httptest.NewRecorder()
            handler.Health(rec, req)

            if rec.Code != tc.wantStatus {
                t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
            }
            if tc.wantBody != "" {
                if body := rec.Body.String(); body != tc.wantBody {
                    t.Errorf("body: got %q, want %q", body, tc.wantBody)
                }
            }
        })
    }
}
```

Key points:

- The anonymous struct slice is the table. Each field corresponds to something that varies between test cases.
- `t.Run(tc.name, ...)` creates a **subtest**. Subtests are individually addressable—you can run a single one with `go test -run TestHealthHandler/GET_returns_200`.
- Failure messages are scoped to the subtest, so a multi-case failure clearly identifies which case broke.

The books handler follows the same structure:

```go
func TestBooksHandler(t *testing.T) {
    tests := []struct {
        name       string
        method     string
        wantStatus int
        wantJSON   bool
    }{
        {
            name:       "GET returns 200 with JSON list",
            method:     http.MethodGet,
            wantStatus: http.StatusOK,
            wantJSON:   true,
        },
        {
            name:       "POST returns 405",
            method:     http.MethodPost,
            wantStatus: http.StatusMethodNotAllowed,
            wantJSON:   false,
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            req := httptest.NewRequest(tc.method, "/books", nil)
            rec := httptest.NewRecorder()
            handler.Books(rec, req)

            if rec.Code != tc.wantStatus {
                t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
            }
            if tc.wantJSON {
                if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
                    t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
                }
                var books []handler.Book
                if err := json.NewDecoder(rec.Body).Decode(&books); err != nil {
                    t.Fatalf("decode failed: %v", err)
                }
                if len(books) == 0 {
                    t.Error("expected at least one book")
                }
            }
        })
    }
}
```

Adding a new test case now means adding one entry to the slice, not writing a new function. This also makes code review easier: diffs show a row addition, not a new block of setup/teardown.

---

### Test Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

The first command runs tests and writes a coverage profile to `coverage.out`. The second opens it in your browser, color-coding each line: green for covered, red for not. This is the built-in Go workflow—no plugin or external tool required.

For a quick summary without the browser:

```bash
go test -cover ./...
# ok   github.com/fesoliveira014/library-system/services/gateway/internal/handler   coverage: 87.5% of statements
```

Coverage is a proxy metric, not a goal. 100% coverage does not mean the code is correct; it means every line was executed at least once. What matters is that your cases cover the meaningful behavioral boundaries—which is exactly what table-driven tests are good at making explicit.

---

### Testing with Earthly

The gateway's `Earthfile` includes a `+test` target that runs the full suite inside a reproducible container. Chapter 10 covers Earthly and CI/CD in depth; for now, here is the target for reference:

```earthly
test:
    FROM +src
    RUN go test -v -race -cover ./...
```

Running `earthly +test` executes the tests in the same `golang:1.22-alpine` image used for CI, eliminating "works on my machine" failures.[^4]

---

### Exercise

In section 1.3 you were asked to implement a `GET /books/{id}` endpoint. Write table-driven tests for it covering three cases:

1. A valid numeric ID (e.g., `1`) returns HTTP 200 and a JSON body containing the correct book.
2. An ID that does not exist (e.g., `9999`) returns HTTP 404.
3. A non-GET method (e.g., `DELETE`) on a valid path returns HTTP 405.

Your test function should have the signature `func TestBooksIDHandler(t *testing.T)` and live in `services/gateway/internal/handler/books_test.go`. Each case should be a subtest named clearly enough that a failure message alone tells you what broke.

---

[^1]: [Go testing package—pkg.go.dev](https://pkg.go.dev/testing)
[^2]: [Go Wiki: Table-driven tests—go.dev](https://go.dev/wiki/TableDrivenTests)
[^3]: [httptest package—pkg.go.dev](https://pkg.go.dev/net/http/httptest)
[^4]: [Earthly documentation—docs.earthly.dev](https://docs.earthly.dev/)
