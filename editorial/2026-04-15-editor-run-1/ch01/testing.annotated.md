<!-- [STRUCTURAL] Heading level continues the inconsistency from 1.2 and 1.3 (H2). Same chapter-wide decision applies. -->
## 1.4 Testing

<!-- [STRUCTURAL] Opening paragraph has strong tutor voice. However, it commits to "you will not use testify" ("the standard library alone is sufficient and is what you will use here") and is immediately contradicted by implicit later chapters using testify. Verify with the author whether the constraint holds across the whole book. If the intent is "in Chapter 1 we use only stdlib, testify comes later," phrase it that way. -->
<!-- [LINE EDIT] "Go's testing story is opinionated and minimal." — crisp. Keep. -->
<!-- [LINE EDIT] "There is no JUnit, no TestNG, no pytest — just the standard library's `testing` package and a handful of conventions baked into the toolchain." — 28 words; good. -->
<!-- [LINE EDIT] "If you are used to writing `@Test` annotations or `assert.assertEquals`, the first thing you will notice is the absence of assertions." — tighten: "If you're used to writing `@Test` annotations or `assert.assertEquals`, the first thing you'll notice is the absence of assertions." (contractions set a conversational tone consistent with the rest of the book). -->
<!-- [LINE EDIT] "Go's philosophy is that your test functions are ordinary code, and `if` is perfectly capable of expressing a failure condition." — good. -->
Go's testing story is opinionated and minimal. There is no JUnit, no TestNG, no pytest — just the standard library's `testing` package and a handful of conventions baked into the toolchain. If you are used to writing `@Test` annotations or `assert.assertEquals`, the first thing you will notice is the absence of assertions. Go's philosophy is that your test functions are ordinary code, and `if` is perfectly capable of expressing a failure condition.

---

### The `_test.go` Convention

<!-- [LINE EDIT] "The Go toolchain identifies test files by their name: any file ending in `_test.go` is excluded from the production build and only compiled when running tests." — good. -->
<!-- [LINE EDIT] "There is no annotation, no test runner configuration — the filename is the contract." — good line. Keep. -->
The Go toolchain identifies test files by their name: any file ending in `_test.go` is excluded from the production build and only compiled when running tests. There is no annotation, no test runner configuration — the filename is the contract.

Inside a test file, test functions follow a strict signature:

```go
func TestXxx(t *testing.T) { ... }
```

<!-- [LINE EDIT] "The function must start with `Test`, the next character must be uppercase (or end of name)" — "(or end of name)" means `Test` alone is also valid? Actually `func Test(t *testing.T)` IS a valid test name per the Go testing package (though rarely used). The parenthetical is technically correct but confusing. Consider: "...the next character must be uppercase" and drop the parenthetical; a reader who writes `TestFoo` will be fine. -->
The function must start with `Test`, the next character must be uppercase (or end of name), and the parameter must be `*testing.T`. The `T` type provides methods for logging, marking failures, and stopping execution:

<!-- [COPY EDIT] Table header "Behaviour" — British spelling. CMOS prefers American ("Behavior"). Rest of chapter uses American spelling ("initialize", "serialize", "synchronised" appears in "synchronised" later — mixed). Standardize to American across book. -->
<!-- [COPY EDIT] "`System.out.println`" in the Java analogue column is correct Java. Fine. -->
| Method | Behaviour | Java/Kotlin analogue |
|---|---|---|
| `t.Errorf(...)` | Marks the test as failed, continues running | `fail(message)` — soft |
| `t.Fatalf(...)` | Marks the test as failed, stops immediately | `fail(message)` — hard |
| `t.Logf(...)` | Logs a message, only visible with `-v` | `System.out.println` |

<!-- [LINE EDIT] "This is the only assertion mechanism you get by default." — "This" has a slightly ambiguous antecedent (the table as a whole, or `t.Errorf` specifically). Consider: "These three methods are the only assertion mechanism you get by default." -->
<!-- [LINE EDIT] "Many Go projects add `github.com/stretchr/testify/assert` for convenience, but the standard library alone is sufficient and is what you will use here.[^1]" — as noted in pass 1, confirm the "here" scope (this chapter only, or entire book). -->
This is the only assertion mechanism you get by default. It forces you to write precise, readable failure messages instead of relying on a framework to format them for you. Many Go projects add `github.com/stretchr/testify/assert` for convenience, but the standard library alone is sufficient and is what you will use here.[^1]

#### Package Naming: `_test` Suffix

<!-- [LINE EDIT] "Notice the test files in this project use `package handler_test` rather than `package handler`. This is a Go idiom for **black-box testing**: the test package sits outside the package under test and can only access exported identifiers." — good explanation. -->
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

<!-- [COPY EDIT] "synchronised" — British spelling; rest of chapter uses American. Change to "synchronized". -->
<!-- [COPY EDIT] "(~2–20x slowdown)" — en dash (–) used for numeric range per CMOS 6.78; correct. "20x" should be "20×" (multiplication sign) per CMOS; or at least "20x" consistently in lowercase. -->
<!-- [LINE EDIT] "It has a runtime cost (~2–20x slowdown) but catches real bugs. You should run it in CI even if not locally every time." — "catches real bugs" is imprecise. Consider: "It has a runtime cost (roughly 2–20× slowdown) but catches genuine concurrency bugs that are otherwise nearly impossible to reproduce. Run it in CI, even if you skip it locally." -->
The `-race` flag instruments the binary to detect concurrent memory accesses that are not properly synchronised. It has a runtime cost (~2–20x slowdown) but catches real bugs. You should run it in CI even if not locally every time.

<!-- [LINE EDIT] "`./...` is a Go path wildcard meaning "this module and all packages recursively below it". Think of it as the Go equivalent of Maven's `mvn test` applied to all submodules at once." — clear. -->
`./...` is a Go path wildcard meaning "this module and all packages recursively below it". Think of it as the Go equivalent of Maven's `mvn test` applied to all submodules at once.

<!-- [COPY EDIT] CMOS 6.9: periods/commas go inside closing quotation marks. "all packages recursively below it". — period should be inside the closing quote: ...below it." — assuming American style (the project otherwise uses AmE). -->

---

### The `httptest` Package

<!-- [LINE EDIT] "Testing an HTTP handler normally means spinning up a real server, making a real network call, and tearing down afterwards. Go eliminates this with `net/http/httptest`.[^3]" — fine. -->
<!-- [COPY EDIT] "tearing down afterwards" — "afterwards" is BrE; "afterward" is AmE. Align with chapter/book spelling convention. -->
Testing an HTTP handler normally means spinning up a real server, making a real network call, and tearing down afterwards. Go eliminates this with `net/http/httptest`.[^3]

- `httptest.NewRequest(method, target, body)` — creates an `*http.Request` suitable for passing directly to a handler, without a real TCP connection.
- `httptest.NewRecorder()` — creates a `*httptest.ResponseRecorder` that implements `http.ResponseWriter` and captures the response status, headers, and body in memory.

This means you can test a handler as a plain function call:

```go
req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
rec := httptest.NewRecorder()
handler.Health(rec, req)
// rec.Code, rec.Body, rec.Header() are all available now
```

<!-- [LINE EDIT] "No ports, no goroutines, no teardown. This is the standard Go pattern — if you have ever used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical, but the implementation is lighter because the handler is already just a function." — 44 words. Split: "No ports, no goroutines, no teardown. This is the standard Go pattern. If you've used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical — but the implementation is lighter because the handler is already just a function." -->
No ports, no goroutines, no teardown. This is the standard Go pattern — if you have ever used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical, but the implementation is lighter because the handler is already just a function.

---

### Table-Driven Tests

<!-- [LINE EDIT] "The most idiomatic way to write parameterized tests in Go is the **table-driven** pattern.[^2]" — good framing. -->
<!-- [COPY EDIT] "parameterized" — AmE spelling; consistent with chapter. Keep. -->
<!-- [COPY EDIT] "@ParameterizedTest" (JUnit 5), "@pytest.mark.parametrize" — confirm exact capitalization of pytest decorator (lowercase is correct). -->
The most idiomatic way to write parameterized tests in Go is the **table-driven** pattern.[^2] Rather than writing one test function per case, you define a slice of test cases and range over them. This is Go's answer to JUnit's `@ParameterizedTest` or pytest's `@pytest.mark.parametrize`.

Here is the health handler rewritten as a table-driven test:

```go
package handler_test

import (
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

<!-- [STRUCTURAL] The test code uses escaped quotes `"{\"status\":\"ok\"}\n"` — technically correct but harder to read than a backtick raw string `` `{"status":"ok"}` + "\n" ``. Consider switching to raw-string for readability. Optional. -->
<!-- [STRUCTURAL] The loop body does not call `t.Parallel()`. For a first-introduction of table-driven tests, that's fine — but ch01 recent commits include "test: add t.Parallel to mock-based unit tests" suggesting the author values parallelism. A brief forward-reference ("we'll add `t.Parallel()` when tests become slower in later chapters") would tie the teaching arc together. -->
<!-- [COPY EDIT] Go 1.22+ fixed the loop-variable capture issue, so the `tc := tc` shadow line is no longer needed. The code correctly omits it — flag for reader awareness in prose. -->
Key points:

- The anonymous struct slice is the table. Each field corresponds to something that varies between test cases.
- `t.Run(tc.name, ...)` creates a **subtest**. Subtests are individually addressable — you can run a single one with `go test -run TestHealthHandler/GET_returns_200`.
- Failure messages are scoped to the subtest, so a multi-case failure clearly identifies which case broke.

<!-- [COPY EDIT] "`go test -run TestHealthHandler/GET_returns_200`" — the subtest name in the code is "GET returns 200 with ok body" (with spaces). `go test -run` converts spaces to underscores, so this should match "GET_returns_200_with_ok_body". The shortened form in the prose is confusing — either complete it or explain the name-mangling rule. -->
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

<!-- [COPY EDIT] The snippet references `json.NewDecoder` but the shown import list in the previous snippet (TestHealthHandler) does not include `encoding/json`. The reader running this verbatim will hit an undeclared-identifier error. Add "encoding/json" to the import block or make the import list explicit in this snippet. -->

<!-- [LINE EDIT] "Adding a new test case now means adding one entry to the slice, not writing a new function. This also makes code review easier: diffs show a row addition, not a new block of setup/teardown." — good closing. Keep. -->
Adding a new test case now means adding one entry to the slice, not writing a new function. This also makes code review easier: diffs show a row addition, not a new block of setup/teardown.

---

### Test Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

<!-- [LINE EDIT] "The first command runs tests and writes a coverage profile to `coverage.out`. The second opens it in your browser, colour-coding each line: green for covered, red for not. This is the standard Go workflow — there is no plugin or external tool required." — "standard Go workflow" — again, soften to "the built-in Go workflow" if other tools (gocov, gcov2lcov) are acknowledged. -->
<!-- [COPY EDIT] "colour-coding" — BrE spelling; align to "color-coding" for AmE consistency. -->
The first command runs tests and writes a coverage profile to `coverage.out`. The second opens it in your browser, colour-coding each line: green for covered, red for not. This is the standard Go workflow — there is no plugin or external tool required.

For a quick summary without the browser:

```bash
go test -cover ./...
# ok   github.com/fesoliveira014/library-system/services/gateway/internal/handler   coverage: 87.5% of statements
```

<!-- [LINE EDIT] "Coverage is a proxy metric, not a goal. 100% coverage does not mean the code is correct; it means every line was executed at least once. What matters is that your cases cover the meaningful behavioural boundaries — which is exactly what table-driven tests are good at making explicit." — strong closing paragraph. Keep. -->
<!-- [COPY EDIT] "100%" — CMOS 9.18: percentage uses figure + "percent" or "%"; "100%" is fine in technical prose. -->
<!-- [COPY EDIT] "behavioural" — BrE; align with AmE "behavioral". -->
Coverage is a proxy metric, not a goal. 100% coverage does not mean the code is correct; it means every line was executed at least once. What matters is that your cases cover the meaningful behavioural boundaries — which is exactly what table-driven tests are good at making explicit.

---

### Testing with Earthly

<!-- [STRUCTURAL] This section jumps from stdlib testing into containerised testing without a strong bridge. Consider one sentence: "Running tests locally is fine during development, but CI needs reproducibility. That's what Earthly provides." -->
The gateway's `Earthfile` defines a `+test` target:

<!-- [COPY EDIT] Code fence language "earthly" — not all renderers highlight Earthfile syntax; common alternatives are "dockerfile" or no language. Renderer-dependent; low priority. -->
```earthly
test:
    FROM +src
    RUN go test -v -race -cover ./...
```

<!-- [COPY EDIT] Please verify: "`golang:1.22-alpine` image" — prerequisites claim Go 1.26+, and project-setup.md pins `go 1.26.1`. If the Earthfile actually uses `golang:1.22-alpine`, this is a real inconsistency that will break the build on code using Go 1.22+ features (generics range-over-func in Go 1.23, etc.). Author must confirm whether the Earthfile tracks the prerequisite. -->
<!-- [LINE EDIT] "Running `earthly +test` from the gateway directory executes the full test suite inside a reproducible container — the same `golang:1.22-alpine` image used for CI." — fine once version resolved. -->
Running `earthly +test` from the gateway directory executes the full test suite inside a reproducible container — the same `golang:1.22-alpine` image used for CI. This eliminates "works on my machine" failures caused by different Go versions, OS-level race detector support, or missing environment variables.

<!-- [COPY EDIT] "OS-level" — compound modifier before noun, correctly hyphenated per CMOS 7.81. -->

The root `Earthfile` composes this target into a `ci` pipeline alongside linting:

```earthly
ci:
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
```

<!-- [STRUCTURAL] `+lint` is referenced here but never introduced — the reader has not seen a lint target defined. Either add the lint target to this page or forward-reference ("We'll define `+lint` in Chapter 2 when we add golangci-lint"). -->
<!-- [LINE EDIT] "This is why reproducible environments matter: the same `earthly +ci` invocation runs identically on a developer laptop, in a GitHub Actions runner, or inside any other container runtime." — 32 words; fine. Reads well. -->
<!-- [LINE EDIT] "No JVM toolchain to configure, no Gradle wrapper, no Maven settings XML — just a container and the `earthly` binary.[^4]" — strong. Keep. -->
This is why reproducible environments matter: the same `earthly +ci` invocation runs identically on a developer laptop, in a GitHub Actions runner, or inside any other container runtime. No JVM toolchain to configure, no Gradle wrapper, no Maven settings XML — just a container and the `earthly` binary.[^4]

---

### Exercise

<!-- [STRUCTURAL] Exercise depends on §1.3's `BookByID` exercise — good continuity, but state the dependency explicitly at the top so readers jumping in know what they need. -->
In section 1.3 you were asked to implement a `GET /books/{id}` endpoint. Write table-driven tests for it covering three cases:

<!-- [COPY EDIT] "e.g." in items 1 and 3 — CMOS 6.43: "e.g.," with comma after. "e.g. `1`" should be "e.g., `1`". Two occurrences in this list. -->
1. A valid numeric ID (e.g. `1`) returns HTTP 200 and a JSON body containing the correct book.
2. An ID that does not exist (e.g. `9999`) returns HTTP 404.
3. A non-GET method (e.g. `DELETE`) on a valid path returns HTTP 405.

<!-- [LINE EDIT] "Your test function should have the signature `func TestBooksIDHandler(t *testing.T)` and live in `services/gateway/internal/handler/books_test.go`. Each case should be a subtest named clearly enough that a failure message alone tells you what broke." — good guidance. -->
Your test function should have the signature `func TestBooksIDHandler(t *testing.T)` and live in `services/gateway/internal/handler/books_test.go`. Each case should be a subtest named clearly enough that a failure message alone tells you what broke.

---

<!-- [COPY EDIT] Footnote style here mixes format with other files: uses "title — source" pattern. Align across chapter. -->
[^1]: [Go testing package — pkg.go.dev](https://pkg.go.dev/testing)
[^2]: [Go Wiki: Table-driven tests — go.dev](https://go.dev/wiki/TableDrivenTests)
[^3]: [httptest package — pkg.go.dev](https://pkg.go.dev/net/http/httptest)
[^4]: [Earthly documentation — docs.earthly.dev](https://docs.earthly.dev/)
