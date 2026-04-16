# 11.1 Unit Testing Patterns

<!-- [STRUCTURAL] Section delivers on its title: four patterns clearly named and demonstrated. Order (table-driven → subtests → helpers → testdata) is logical: each pattern reuses the previous one. Good scaffolding. One gap: no treatment of mocking libraries (gomock, testify/mock) even though the index promises it. Either add a brief pointer here or trim the index's promise. -->
<!-- [LINE EDIT] "Go's standard library ships with a testing package that is deliberately minimal." — good opener. -->
<!-- [COPY EDIT] "strong conventions" — acceptable. "expressive, maintainable test suites" — serial-comma-free pair; fine per CMOS 6.19 (serial comma required only for three+ items). -->
Go's standard library ships with a testing package that is deliberately minimal. There is no built-in assertion library, no test runner framework, and no magic annotations. What Go does provide — subtests, helper functions, and a set of strong conventions — turns out to be enough for writing expressive, maintainable test suites. This section walks through the patterns you will reach for most often.

---

## Table-Driven Tests

<!-- [COPY EDIT] Heading: title case — "Table-Driven Tests" is correct (CMOS 8.159). Note that a subhead one level down uses sentence case ("Why a slice of anonymous structs?") — this mixed style should be unified. Recommend title case throughout this section for H2/H3 consistency. -->
<!-- [STRUCTURAL] The subhead hierarchy in this section is inconsistent: H2 "Table-Driven Tests" → H3 "Why a slice of anonymous structs?" (sentence case) → H3 "Worked example: `CreateBook` validation" (sentence case). Other H2 headings later use title case. Pick one convention. -->
The single most important pattern in Go testing is the table-driven test. Rather than writing a separate function for each input combination, you declare a slice of anonymous structs — one per scenario — and loop over them. The Go standard library itself uses this pattern extensively.

### Why a slice of anonymous structs?

<!-- [LINE EDIT] "Each element of the slice is a small, self-contained record describing one test case: its human-readable name, its inputs, and what the expected outcome looks like." — 27 words; reads fine. -->
<!-- [LINE EDIT] "The anonymous struct type is defined inline, so there is no ceremony of naming a type you will never use elsewhere." → "The anonymous struct type is defined inline, so you avoid naming a type you will never use elsewhere." Removes the noun "ceremony" which is stylistic but imprecise. -->
Each element of the slice is a small, self-contained record describing one test case: its human-readable name, its inputs, and what the expected outcome looks like. The anonymous struct type is defined inline, so there is no ceremony of naming a type you will never use elsewhere. Adding a new case is a one-liner that does not require touching any control flow.

<!-- [LINE EDIT] "Compare this to the C++ or Java approach of parameterized tests (Google Test's `INSTANTIATE_TEST_SUITE_P`, JUnit's `@ParameterizedTest`): the Go version has no framework machinery — it is just a slice and a loop." 38 words; fine as is. -->
<!-- [COPY EDIT] "Google Test's `INSTANTIATE_TEST_SUITE_P`" — product uses "Google Test" or "GoogleTest"; current canonical form is "GoogleTest" (per the repo). Please verify. -->
Compare this to the C++ or Java approach of parameterized tests (Google Test's `INSTANTIATE_TEST_SUITE_P`, JUnit's `@ParameterizedTest`): the Go version has no framework machinery — it is just a slice and a loop.

### Worked example: `CreateBook` validation

<!-- [LINE EDIT] "The catalog service validates incoming books before persisting them. Let's test several invalid inputs and one valid input in a single function:" — 23 words; fine. -->
<!-- [COPY EDIT] "Let's" — contraction in tutor-register prose is fine; ensure consistency across chapter. -->
The catalog service validates incoming books before persisting them. Let's test several invalid inputs and one valid input in a single function:

```go
package service_test

import (
    "context"
    "testing"

    "github.com/your-org/library/catalog/internal/model"
    "github.com/your-org/library/catalog/internal/service"
)

func TestCreateBook_Validation(t *testing.T) {
    repo := newMockRepo()
    pub := &noopPublisher{}
    svc := service.NewCatalogService(repo, pub)

    tests := []struct {
        name    string
        book    *model.Book
        wantErr bool
    }{
        {
            name:    "missing title",
            book:    &model.Book{Author: "A", ISBN: "978-0000000001", TotalCopies: 1},
            wantErr: true,
        },
        {
            name:    "missing author",
            book:    &model.Book{Title: "T", ISBN: "978-0000000001", TotalCopies: 1},
            wantErr: true,
        },
        {
            name:    "negative copies",
            book:    &model.Book{Title: "T", Author: "A", ISBN: "978-0000000001", TotalCopies: -1},
            wantErr: true,
        },
        {
            name:    "valid book",
            book:    &model.Book{Title: "T", Author: "A", ISBN: "978-0000000001", TotalCopies: 3},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := svc.CreateBook(context.Background(), tt.book)
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateBook() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

<!-- [LINE EDIT] "Walk through the mechanics:" — imperative; good. -->
Walk through the mechanics:

<!-- [COPY EDIT] Bullet style: each bullet starts with a capital; consistent. Verify none end with a stray period while others don't. Currently first three end with ".", fourth bullet also ends with "." — OK. -->
<!-- [LINE EDIT] Bullet 1: "The struct has three fields: the display name, the input, and the expected outcome expressed as a boolean rather than a concrete error value." 28 words; reads fine. -->
- `tests` is a slice of an anonymous struct literal. The struct has three fields: the display name, the input, and the expected outcome expressed as a boolean rather than a concrete error value. Checking `wantErr` rather than a specific error message keeps the test resilient to phrasing changes in error strings.
- The `for _, tt := range tests` loop gives you each case. The variable is conventionally named `tt` (short for "table test").
- `t.Run(tt.name, func(t *testing.T) { ... })` registers each case as a named subtest. This is covered in detail in the next section.
- Inside the subtest, `t.Errorf` (not `t.Fatalf`) is used so that all cases run even when one fails. If you used `t.Fatalf`, the first failure would abort the entire loop.

<!-- [LINE EDIT] "When a case fails, the output looks like:" — fine. -->
When a case fails, the output looks like:

```
--- FAIL: TestCreateBook_Validation/missing_title (0.00s)
    catalog_test.go:42: CreateBook() error = <nil>, wantErr true
```

<!-- [LINE EDIT] "The path `TestCreateBook_Validation/missing_title` tells you exactly which row failed without any extra tooling." — fine. -->
The path `TestCreateBook_Validation/missing_title` tells you exactly which row failed without any extra tooling.

---

## Subtests with `t.Run`

<!-- [STRUCTURAL] Subtests section grows very long — covers naming, filtering, parallel subtests, top-level parallel, safety conditions, -race/-count tooling, and a pointer to a talk. Consider splitting: "Subtests with t.Run" (first 3 subsections) and a new H2 "Parallelism with t.Parallel" for the rest. Currently the parallel material dominates subtests. -->
`t.Run` registers a subtest under the parent test. Each call creates a new `*testing.T` bound to the provided name. If any subtest fails, the parent test is also marked as failed, but the remaining subtests continue to run.

### Naming conventions

<!-- [LINE EDIT] "Go subtests use forward-slash-separated hierarchical names." — fine. "The full name of a subtest is `TestFunctionName/case_name`." — fine. -->
<!-- [COPY EDIT] "forward-slash-separated" — hyphenation acceptable; CMOS permits stacked compound modifiers. -->
Go subtests use forward-slash-separated hierarchical names. The full name of a subtest is `TestFunctionName/case_name`. Spaces in the case name are replaced with underscores in the output and when used as `-run` filters. Use names that are specific enough to be self-documenting:

```
TestCreateBook_Validation/missing_title      -- good
TestCreateBook_Validation/case1              -- bad: tells you nothing
TestCreateBook_Validation/valid              -- acceptable but imprecise
TestCreateBook_Validation/valid_book         -- better
```

<!-- [LINE EDIT] "The test function name itself follows the convention `TestFunctionName_Context`, where `Context` describes what aspect is being tested:" — fine. -->
The test function name itself follows the convention `TestFunctionName_Context`, where `Context` describes what aspect is being tested: `TestCreateBook_Validation`, `TestCreateBook_DuplicateISBN`, `TestListBooks_Pagination`.

### Selective execution

<!-- [LINE EDIT] "Because each case is a named subtest, you can target a single case from the command line without changing code:" — fine. -->
Because each case is a named subtest, you can target a single case from the command line without changing code:

```bash
# Run only the "missing title" case
go test ./catalog/internal/service/... -run "TestCreateBook_Validation/missing_title"

# Run all validation subtests
go test ./catalog/internal/service/... -run "TestCreateBook_Validation"

# Run all tests whose name contains "Validation"
go test ./catalog/internal/service/... -run "Validation"
```

<!-- [LINE EDIT] "The `-run` flag accepts a regular expression matched against the full subtest path." — fine. -->
<!-- [COPY EDIT] Please verify: `-run` flag regex semantics — CLI contract is a slash-separated regex per level, not a single regex against the full path. Readers may be confused. -->
The `-run` flag accepts a regular expression matched against the full subtest path. This is particularly useful during development when you want to iterate on a single failing case without waiting for the entire suite.

### Parallel subtests

<!-- [LINE EDIT] "Subtests can be parallelized with `t.Parallel()`:" — fine. -->
Subtests can be parallelized with `t.Parallel()`:

```go
for _, tt := range tests {
    tt := tt // capture loop variable — required before Go 1.22
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // ... test body
    })
}
```

<!-- [LINE EDIT] "When `t.Parallel()` is called, the subtest pauses until all non-parallel siblings have finished, then all parallel subtests run concurrently." — fine. -->
<!-- [COPY EDIT] "wall-clock time" — fine (not hyphenated when functioning as noun). "I/O-bound" — acronym with hyphen; correct. -->
When `t.Parallel()` is called, the subtest pauses until all non-parallel siblings have finished, then all parallel subtests run concurrently. This can significantly reduce wall-clock time for I/O-bound tests.

<!-- [LINE EDIT] "**Important caveat for this project.**" bold + period as a "mini-heading" pattern is stylistic. Some books prefer the formal H4 or a colon. Keep if consistent across chapter; if not, normalize. -->
**Important caveat for this project.** The mock objects used in these tests — `newMockRepo()` and `noopPublisher` — maintain shared state (an in-memory slice of books, event counters). If multiple parallel subtests share a single mock instance, they will race. There are two safe approaches:

<!-- [LINE EDIT] "1. Construct a fresh set of mocks inside each subtest body (before calling `t.Parallel()`):" — fine. -->
1. Construct a fresh set of mocks inside each subtest body (before calling `t.Parallel()`):

```go
for _, tt := range tests {
    tt := tt
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        repo := newMockRepo()   // fresh instance per subtest
        pub := &noopPublisher{}
        svc := service.NewCatalogService(repo, pub)

        _, err := svc.CreateBook(context.Background(), tt.book)
        if (err != nil) != tt.wantErr {
            t.Errorf("CreateBook() error = %v, wantErr %v", err, tt.wantErr)
        }
    })
}
```

2. Keep subtests sequential (no `t.Parallel()`) when the test is fast and the shared setup is intentional.

<!-- [LINE EDIT] "For the validation table above, the cases are independent, so option 1 is appropriate. For tests that verify state built up across calls (e.g., "create a book, then retrieve it"), sequential subtests sharing a mock make the test easier to read." — fine. -->
<!-- [COPY EDIT] "e.g.," takes comma after per CMOS 6.43; correct here. -->
For the validation table above, the cases are independent, so option 1 is appropriate. For tests that verify state built up across calls (e.g., "create a book, then retrieve it"), sequential subtests sharing a mock make the test easier to read.

### Top-level `t.Parallel()`

<!-- [LINE EDIT] "`t.Parallel()` also works at the top of a test function — not just inside subtests." — good. -->
<!-- [LINE EDIT] "When a top-level test calls `t.Parallel()`, it runs concurrently with every other top-level parallel test in the same package." — fine. -->
`t.Parallel()` also works at the top of a test function — not just inside subtests. When a top-level test calls `t.Parallel()`, it runs concurrently with every other top-level parallel test in the same package. Non-parallel tests still run first; then all parallel tests run together.

```go
func TestCreateBook_Validation(t *testing.T) {
    t.Parallel()
    // ... test body using fresh mocks per invocation
}

func TestCreateBook_DuplicateISBN(t *testing.T) {
    t.Parallel()
    // ... independent test
}
```

<!-- [LINE EDIT] "For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed: a package with thirty independent tests, each doing a bit of setup and one assertion, finishes in the time of the slowest single test rather than the sum of all of them." 51 words; consider splitting. Suggest: "For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed. A package with thirty independent tests — each doing a bit of setup and one assertion — finishes in the time of the slowest single test rather than the sum of all of them." -->
<!-- [COPY EDIT] "thirty" — CMOS 9.2: spell out zero through one hundred in prose. Correct here. -->
For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed: a package with thirty independent tests, each doing a bit of setup and one assertion, finishes in the time of the slowest single test rather than the sum of all of them. On a CI runner with many cores, the wall-clock difference can be significant.

<!-- [LINE EDIT] "Two conditions must hold for a test to safely opt into `t.Parallel()`:" — fine. -->
Two conditions must hold for a test to safely opt into `t.Parallel()`:

<!-- [LINE EDIT] Bullet 1: "If the test sets a package-level variable, mutates a shared map, or re-registers a singleton (OpenTelemetry global tracer, `log.SetOutput`, `os.Setenv`), another parallel test could observe the change mid-run." 32 words; fine. -->
<!-- [COPY EDIT] "OpenTelemetry global tracer" — capitalize OpenTelemetry (product name). Correct here. -->
1. **The test must not depend on global mutable state.** If the test sets a package-level variable, mutates a shared map, or re-registers a singleton (OpenTelemetry global tracer, `log.SetOutput`, `os.Setenv`), another parallel test could observe the change mid-run. Either isolate the state into test-local instances or keep the test sequential.
<!-- [LINE EDIT] "Anything that hits a real database, a real Kafka broker, a real filesystem path shared with other tests, or reserves a well-known TCP port cannot run in parallel with siblings that touch the same resource." 36 words; acceptable. -->
<!-- [COPY EDIT] "filesystem" — one word per CMOS 7.85 (commonly accepted technical compound). "TCP port" — uppercase TCP. Correct. -->
2. **The test must not depend on shared external state.** Anything that hits a real database, a real Kafka broker, a real filesystem path shared with other tests, or reserves a well-known TCP port cannot run in parallel with siblings that touch the same resource. In this project, files named `integration_test.go`, `e2e_test.go`, and the repository tests that reach a real PostgreSQL instance are deliberately left sequential.

<!-- [LINE EDIT] "The project's unit tests build a fresh mock repository, fake publisher, and `httptest.Server` per test invocation, so they satisfy both conditions and have `t.Parallel()` at the top of every `Test*` function." 34 words; acceptable. -->
<!-- [LINE EDIT] "Running the suite under `go test -race ./...` confirms the promise — the race detector will fail the build if any test does accidentally share state across goroutines." → "Running the suite under `go test -race ./...` confirms the promise: the race detector fails the build if any test accidentally shares state across goroutines." Active verb + cleaner connective. -->
The project's unit tests build a fresh mock repository, fake publisher, and `httptest.Server` per test invocation, so they satisfy both conditions and have `t.Parallel()` at the top of every `Test*` function. Running the suite under `go test -race ./...` confirms the promise — the race detector will fail the build if any test does accidentally share state across goroutines.

<!-- [LINE EDIT] "A good sanity check when adding new tests is to run `go test -race -count=3 ./...`. The `-count=3` flag re-runs each test three times, which tends to surface flakes that only show up under specific scheduler interleavings." — fine. -->
<!-- [COPY EDIT] "scheduler interleavings" — correct plural. -->
A good sanity check when adding new tests is to run `go test -race -count=3 ./...`. The `-count=3` flag re-runs each test three times, which tends to surface flakes that only show up under specific scheduler interleavings. If a test passes with `-count=1` but fails with `-count=3`, there is almost certainly shared state you have not noticed yet.

<!-- [LINE EDIT] "Mitchell Hashimoto's 'Advanced Testing with Go' talk[^3] is a good deeper dive into this and several adjacent patterns (dependency injection, integration boundaries, golden files)." — fine. -->
<!-- [COPY EDIT] "deeper dive" — idiomatic; fine. "golden files" — industry term; fine. -->
Mitchell Hashimoto's "Advanced Testing with Go" talk[^3] is a good deeper dive into this and several adjacent patterns (dependency injection, integration boundaries, golden files). The `t.Parallel()` discussion there is short but memorable: parallel tests force you to write tests that do not leak state, which is a property worth having even before the wall-clock win.

---

## Test Helpers and `t.Helper()`

<!-- [LINE EDIT] "As tests grow, you will find yourself repeating setup code: create a book, confirm it succeeded, return the result. Extracting this into a helper function is natural, but there is one detail that makes it work well in Go: `t.Helper()`." 40 words. Acceptable. -->
As tests grow, you will find yourself repeating setup code: create a book, confirm it succeeded, return the result. Extracting this into a helper function is natural, but there is one detail that makes it work well in Go: `t.Helper()`.

### The problem without `t.Helper()`

<!-- [LINE EDIT] "Suppose you write a helper that calls `t.Fatalf` when setup fails:" — fine. -->
Suppose you write a helper that calls `t.Fatalf` when setup fails:

```go
func mustCreateBook(t *testing.T, svc *service.CatalogService, title string) *model.Book {
    book, err := svc.CreateBook(context.Background(), &model.Book{
        Title:       title,
        Author:      "Test Author",
        ISBN:        "978-0000000001",
        TotalCopies: 5,
    })
    if err != nil {
        t.Fatalf("setup: create book %q: %v", title, err)
    }
    return book
}
```

<!-- [LINE EDIT] "Without `t.Helper()`, a failure inside `mustCreateBook` is reported as occurring on the `t.Fatalf` line inside the helper." — fine. -->
<!-- [LINE EDIT] "When you look at the failure output, you see a line number inside `mustCreateBook` — not the line in the test where you called `mustCreateBook`." — fine. -->
<!-- [LINE EDIT] "You have to mentally navigate from the helper back to the caller to understand what was being set up." → "You have to trace back from the helper to the caller to understand what was being set up." Tighter. -->
Without `t.Helper()`, a failure inside `mustCreateBook` is reported as occurring on the `t.Fatalf` line inside the helper. When you look at the failure output, you see a line number inside `mustCreateBook` — not the line in the test where you called `mustCreateBook`. You have to mentally navigate from the helper back to the caller to understand what was being set up.

### The fix: `t.Helper()`

```go
func mustCreateBook(t *testing.T, svc *service.CatalogService, title string) *model.Book {
    t.Helper()
    book, err := svc.CreateBook(context.Background(), &model.Book{
        Title:       title,
        Author:      "Test Author",
        ISBN:        fmt.Sprintf("978-%010d", rand.Intn(1e10)),
        TotalCopies: 5,
    })
    if err != nil {
        t.Fatalf("setup: create book %q: %v", title, err)
    }
    return book
}
```

<!-- [FINAL] `rand.Intn(1e10)` — `rand.Intn` signature is `Intn(n int) int`; `1e10` is a float64 literal. This will not compile. Use `rand.Int63n(1_000_000_000_0)` or a string-based generator. Flag as potential bug. -->
<!-- [COPY EDIT] Please verify: whether `rand.Intn(1e10)` compiles. Go requires an `int` argument; `1e10` is a float constant and on 32-bit platforms exceeds max int. Either write `rand.Int63n(1e10)` or cast. -->
<!-- [LINE EDIT] "Calling `t.Helper()` at the top of the function tells the testing framework to skip this frame when reporting failure locations." — fine. -->
Calling `t.Helper()` at the top of the function tells the testing framework to skip this frame when reporting failure locations. The error is now attributed to the line in the test that called `mustCreateBook`, which is exactly where a human reader would look first.

<!-- [LINE EDIT] "This is equivalent to what `__FILE__` and `__LINE__` macros give you in C/C++ when passing them through to assertion macros, except Go handles it automatically through the call stack." 32 words; fine. -->
<!-- [COPY EDIT] "C/C++" — slash shorthand acceptable. -->
This is equivalent to what `__FILE__` and `__LINE__` macros give you in C/C++ when passing them through to assertion macros, except Go handles it automatically through the call stack.

### Convention for helper naming

<!-- [LINE EDIT] "Helpers that are permitted to fail a test use `must` as a prefix by convention: `mustCreateBook`, `mustGetBook`, `mustInsertFixture`." — fine. -->
<!-- [LINE EDIT] "This signals to the reader that the function will call `t.Fatal` (aborting the test) rather than `t.Error` (marking a failure and continuing)." 22 words; fine. -->
<!-- [LINE EDIT] "The distinction matters: use `must`-style helpers for setup operations where it makes no sense to continue if they fail, and plain helpers for assertions where you want to collect all failures." 31 words; fine. -->
Helpers that are permitted to fail a test use `must` as a prefix by convention: `mustCreateBook`, `mustGetBook`, `mustInsertFixture`. This signals to the reader that the function will call `t.Fatal` (aborting the test) rather than `t.Error` (marking a failure and continuing). The distinction matters: use `must`-style helpers for setup operations where it makes no sense to continue if they fail, and plain helpers for assertions where you want to collect all failures.

### Using the helper in a test

```go
func TestListBooks_ReturnsPaginatedResults(t *testing.T) {
    repo := newMockRepo()
    pub := &noopPublisher{}
    svc := service.NewCatalogService(repo, pub)

    // Setup: create three books. If any fail, the test stops here with a
    // clear message pointing at these lines, not inside mustCreateBook.
    mustCreateBook(t, svc, "The Go Programming Language")
    mustCreateBook(t, svc, "Concurrency in Go")
    mustCreateBook(t, svc, "Cloud Native Go")

    books, err := svc.ListBooks(context.Background(), service.ListOptions{Limit: 2, Offset: 0})
    if err != nil {
        t.Fatalf("ListBooks: %v", err)
    }
    if len(books) != 2 {
        t.Errorf("got %d books, want 2", len(books))
    }
}
```

<!-- [LINE EDIT] "The helper keeps the test body focused on the behaviour under test rather than boilerplate." — fine. -->
<!-- [COPY EDIT] "behaviour" — British spelling; check overall spelling convention. Most Go/US tech writing uses "behavior". Normalize across chapter. Earlier sections (index.md) use "behavior" — standardize to US spelling. -->
The helper keeps the test body focused on the behaviour under test rather than boilerplate.

---

## Test Fixtures with `testdata/`

<!-- [LINE EDIT] "Go has a built-in convention for test fixture files: any directory named `testdata` inside a package is ignored by `go build` and `go vet`, but it is accessible from tests using relative paths." — fine. -->
Go has a built-in convention for test fixture files: any directory named `testdata` inside a package is ignored by `go build` and `go vet`, but it is accessible from tests using relative paths.

### How it works

<!-- [LINE EDIT] "When a test binary runs, the working directory is set to the package directory." — fine. -->
When a test binary runs, the working directory is set to the package directory. That means a test in `catalog/internal/service/` can open files like:

```go
data, err := os.ReadFile("testdata/import_books.json")
```

<!-- [LINE EDIT] "and Go guarantees the working directory is correct at test time, even when tests are invoked from the repository root with `go test ./...`." — fine. Minor: lowercase-"and" sentence start is informal but acceptable as a continuation of the preceding colon-split. -->
and Go guarantees the working directory is correct at test time, even when tests are invoked from the repository root with `go test ./...`.

### When to use `testdata/`

<!-- [LINE EDIT] "External fixture files are most useful when:" — fine. -->
External fixture files are most useful when:

<!-- [COPY EDIT] Bullet list terminal punctuation: first bullet ends with "." — check consistency with others (yes, all end with "."). OK. -->
- The payload is large and would clutter the test source (several hundred lines of JSON or XML).
- The fixture is shared with other tools (e.g., a JSON schema also used in documentation or integration tests).
- The data needs to be independently editable without touching Go source files (e.g., a product manager updates expected catalog records).

<!-- [LINE EDIT] "For this project, book payloads are small (four to six fields), so inline struct literals are more readable than external files." — fine. -->
<!-- [COPY EDIT] "four to six" — spelled out per CMOS 9.2; correct. -->
For this project, book payloads are small (four to six fields), so inline struct literals are more readable than external files. If the catalog service later gains a bulk import feature that accepts multi-book JSON files, those test inputs belong in `testdata/`.

### Structure example

```
catalog/
  internal/
    service/
      catalog_service.go
      catalog_service_test.go
      testdata/
        bulk_import_valid.json
        bulk_import_duplicate_isbn.json
        bulk_import_missing_fields.json
```

<!-- [LINE EDIT] "Loading a fixture in a test:" — fine. -->
Loading a fixture in a test:

```go
func TestBulkImport_Valid(t *testing.T) {
    data, err := os.ReadFile("testdata/bulk_import_valid.json")
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }

    var books []*model.Book
    if err := json.Unmarshal(data, &books); err != nil {
        t.Fatalf("unmarshal fixture: %v", err)
    }

    // ... rest of test
}
```

<!-- [LINE EDIT] "One subtlety: the `testdata/` directory is not special to the Go module system — it is just a naming convention that the toolchain respects." 24 words; fine. -->
<!-- [LINE EDIT] "Do not put Go source files in `testdata/`; they will not be compiled, which can cause confusion." — fine. -->
One subtlety: the `testdata/` directory is not special to the Go module system — it is just a naming convention that the toolchain respects. Do not put Go source files in `testdata/`; they will not be compiled, which can cause confusion.

---

## Summary

<!-- [STRUCTURAL] Summary table helpfully recaps. Consider a closing sentence that sets up 11.2 — current close ("The next section introduces mock objects…") is good; keep. -->
| Pattern | When to use |
|---|---|
| Table-driven test | Any function with multiple input/output combinations |
| `t.Run` subtests | Always; pairs with table-driven to give named, filterable cases |
| `t.Helper()` | Every test helper that calls `t.Fatal` or `t.Error` |
| `testdata/` | Large payloads, shared fixtures, or independently editable data |
| `t.Parallel()` | When subtests have no shared mutable state and wall-clock time matters |

<!-- [STRUCTURAL] Closing line promises "mock objects" next. Check 11.2's actual content: it is about Testcontainers/PostgreSQL integration, not mock objects. Either revise this closing sentence to point to integration testing, or insert a mock-objects section before 11.2. -->
<!-- [FINAL] Cross-reference drift: "The next section introduces mock objects and how to keep them honest using interfaces." — 11.2 does NOT introduce mock objects. Rewrite to match actual next section content. -->
These four patterns cover the vast majority of unit test needs in a Go service. The next section introduces mock objects and how to keep them honest using interfaces.

---

[^1]: Go blog — Using Subtests and Sub-benchmarks: https://go.dev/blog/subtests
[^2]: Go testing package documentation: https://pkg.go.dev/testing
[^3]: Mitchell Hashimoto — Advanced Testing with Go (GopherCon 2017): https://www.youtube.com/watch?v=8hQG7QlcLBk
<!-- [COPY EDIT] Please verify: all three footnote URLs resolve. -->
