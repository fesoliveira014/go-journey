# 11.1 Unit Testing Patterns

Go's standard library ships with a testing package that is deliberately minimal. There is no built-in assertion library, no test runner framework, and no magic annotations. What Go does provide—subtests, helper functions, and a set of strong conventions—turns out to be enough for writing expressive, maintainable test suites. This section walks through the patterns you will reach for most often.

---

## Table-Driven Tests

The single most important pattern in Go testing is the table-driven test. Rather than writing a separate function for each input combination, you declare a slice of anonymous structs—one per scenario—and loop over them. The Go standard library itself uses this pattern extensively.

### Why a Slice of Anonymous Structs?

Each element of the slice is a small, self-contained record describing one test case: its human-readable name, its inputs, and what the expected outcome looks like. The anonymous struct type is defined inline, so you avoid naming a type you will never use elsewhere. Adding a new case is a single line that does not require touching any control flow.

Compare this to the C++ or Java approach of parameterized tests (GoogleTest's `INSTANTIATE_TEST_SUITE_P`, JUnit's `@ParameterizedTest`): the Go version has no framework machinery—it is just a slice and a loop.

### Worked Example: `CreateBook` Validation

The Catalog Service validates incoming books before persisting them. Let's test several invalid inputs and one valid input in a single function:

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

Walk through the mechanics:

- `tests` is a slice of an anonymous struct literal. The struct has three fields: the display name, the input, and the expected outcome expressed as a boolean rather than a concrete error value. Checking `wantErr` rather than a specific error message keeps the test resilient to phrasing changes in error strings.
- The `for _, tt := range tests` loop gives you each case. The variable is conventionally named `tt` (short for "table test").
- `t.Run(tt.name, func(t *testing.T) { ... })` registers each case as a named subtest. This is covered in detail in the next section.
- Inside the subtest, the test calls `t.Errorf` (not `t.Fatalf`) so that all cases run even when one fails. If you used `t.Fatalf`, the first failure would abort the entire loop.

When a case fails, the output looks like:

```
--- FAIL: TestCreateBook_Validation/missing_title (0.00s)
    catalog_test.go:42: CreateBook() error = <nil>, wantErr true
```

The path `TestCreateBook_Validation/missing_title` tells you exactly which row failed without any extra tooling.

---

## Subtests with `t.Run`

`t.Run` registers a subtest under the parent test. Each call creates a new `*testing.T` bound to the provided name. If any subtest fails, the parent test is also marked as failed, but the remaining subtests continue to run.

### Naming conventions

Go subtests use forward-slash-separated hierarchical names. The full name of a subtest is `TestFunctionName/case_name`. Spaces in the case name are replaced with underscores in the output and when used as `-run` filters. Use names that are specific enough to be self-documenting:

```
TestCreateBook_Validation/missing_title      -- good
TestCreateBook_Validation/case1              -- bad: tells you nothing
TestCreateBook_Validation/valid              -- acceptable but imprecise
TestCreateBook_Validation/valid_book         -- better
```

The test function name itself follows the convention `TestFunctionName_Context`, where `Context` describes what aspect is being tested: `TestCreateBook_Validation`, `TestCreateBook_DuplicateISBN`, `TestListBooks_Pagination`.

### Selective execution

Because each case is a named subtest, you can target a single case from the command line without changing code:

```bash
# Run only the "missing title" case
go test ./catalog/internal/service/... -run "TestCreateBook_Validation/missing_title"

# Run all validation subtests
go test ./catalog/internal/service/... -run "TestCreateBook_Validation"

# Run all tests whose name contains "Validation"
go test ./catalog/internal/service/... -run "Validation"
```

The `-run` flag accepts a regular expression matched against the full subtest path. This is particularly useful during development when you want to iterate on a single failing case without waiting for the entire suite.

### Parallel subtests

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

When `t.Parallel()` is called, the subtest pauses until all non-parallel siblings have finished, then all parallel subtests run concurrently. This can significantly reduce wall-clock time for I/O-bound tests.

**Important caveat for this project.** The mock objects used in these tests—`newMockRepo()` and `noopPublisher`—maintain shared state (an in-memory slice of books, event counters). If multiple parallel subtests share a single mock instance, they will race. There are two safe approaches:

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

For the validation table above, the cases are independent, so option 1 is appropriate. For tests that verify state built up across calls (e.g., "create a book, then retrieve it"), sequential subtests sharing a mock make the test easier to read.

### Top-level `t.Parallel()`

`t.Parallel()` also works at the top of a test function—not just inside subtests. When a top-level test calls `t.Parallel()`, it runs concurrently with every other top-level parallel test in the same package. Non-parallel tests still run first; then all parallel tests run together.

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

For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed. A package with thirty independent tests—each doing a bit of setup and one assertion—finishes in the time of the slowest single test rather than the sum of all of them. On a CI runner with many cores, the wall-clock difference can be significant.

Two conditions must hold for a test to safely opt into `t.Parallel()`:

1. **The test must not depend on global mutable state.** If the test sets a package-level variable, mutates a shared map, or re-registers a singleton (OpenTelemetry global tracer, `log.SetOutput`, `os.Setenv`), another parallel test could observe the change mid-run. Either isolate the state into test-local instances or keep the test sequential.
2. **The test must not depend on shared external state.** Anything that hits a real database, a real Kafka broker, a real filesystem path shared with other tests, or reserves a well-known TCP port cannot run in parallel with siblings that touch the same resource. In this project, files named `integration_test.go`, `e2e_test.go`, and the repository tests that reach a real PostgreSQL instance are deliberately left sequential.

The project's unit tests build a fresh mock repository, fake publisher, and `httptest.Server` per test invocation, so they satisfy both conditions and have `t.Parallel()` at the top of every `Test*` function. Running the suite under `go test -race ./...` confirms the promise—the race detector fails the build if any test accidentally shares state across goroutines.

A good sanity check when adding new tests is to run `go test -race -count=3 ./...`. The `-count=3` flag re-runs each test three times, which tends to surface flakes that only show up under specific scheduler interleavings. If a test passes with `-count=1` but fails with `-count=3`, there is almost certainly shared state you have not noticed yet.

Mitchell Hashimoto's "Advanced Testing with Go" talk[^3] is a good deeper dive into this and several adjacent patterns (dependency injection, integration boundaries, golden files). The `t.Parallel()` discussion there is short but memorable: parallel tests force you to write tests that do not leak state, which is a property worth having even before the wall-clock win.

---

## Test Helpers and `t.Helper()`

As tests grow, you will find yourself repeating setup code: create a book, confirm it succeeded, return the result. Extracting this into a helper function is natural, but there is one detail that makes it work well in Go: `t.Helper()`.

### The problem without `t.Helper()`

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

Without `t.Helper()`, a failure inside `mustCreateBook` is reported as occurring on the `t.Fatalf` line inside the helper. When you look at the failure output, you see a line number inside `mustCreateBook`—not the line in the test where you called `mustCreateBook`. You have to trace back from the helper to the caller to understand what was being set up.

### The fix: `t.Helper()`

```go
func mustCreateBook(t *testing.T, svc *service.CatalogService, title string) *model.Book {
    t.Helper()
    book, err := svc.CreateBook(context.Background(), &model.Book{
        Title:       title,
        Author:      "Test Author",
        ISBN:        fmt.Sprintf("978-%010d", rand.Intn(1_000_000_000)),
        TotalCopies: 5,
    })
    if err != nil {
        t.Fatalf("setup: create book %q: %v", title, err)
    }
    return book
}
```

Calling `t.Helper()` at the top of the function tells the testing framework to skip this frame when reporting failure locations. The error is now attributed to the line in the test that called `mustCreateBook`, which is exactly where a human reader would look first.

This is equivalent to what the `__FILE__` and `__LINE__` macros provide in C/C++ assertion wrappers, except that Go handles it automatically through the call stack.

### Convention for helper naming

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

The helper keeps the test body focused on the behavior under test rather than boilerplate.

---

## Test Fixtures with `testdata/`

Go has a built-in convention for test fixture files: any directory named `testdata` inside a package is ignored by `go build` and `go vet`, but it is accessible from tests using relative paths.

### How it works

When a test binary runs, the working directory is set to the package directory. That means a test in `catalog/internal/service/` can open files like:

```go
data, err := os.ReadFile("testdata/import_books.json")
```

and Go guarantees the working directory is correct at test time, even when tests are invoked from the repository root with `go test ./...`.

### When to use `testdata/`

External fixture files are most useful when:

- The payload is large and would clutter the test source (several hundred lines of JSON or XML).
- The fixture is shared with other tools (e.g., a JSON schema also used in documentation or integration tests).
- The data needs to be independently editable without touching Go source files (e.g., a product manager updates expected catalog records).

For this project, book payloads are small (four to six fields), so inline struct literals are more readable than external files. If the Catalog Service later gains a bulk import feature that accepts multi-book JSON files, those test inputs belong in `testdata/`.

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

One subtlety: the `testdata/` directory is not special to the Go module system—it is just a naming convention that the toolchain respects. Do not place Go source files in `testdata/`; they are not compiled, which can cause confusion.

---

## Summary

| Pattern | When to use |
|---|---|
| Table-driven test | Any function with multiple input/output combinations |
| `t.Run` subtests | Always; pairs with table-driven to give named, filterable cases |
| `t.Helper()` | Every test helper that calls `t.Fatal` or `t.Error` |
| `testdata/` | Large payloads, shared fixtures, or independently editable data |
| `t.Parallel()` | When subtests have no shared mutable state and wall-clock time matters |

These five patterns cover the vast majority of unit test needs in a Go service. The next section shifts from mocks to real infrastructure, using Testcontainers to run repository tests against a real PostgreSQL instance.

---

[^1]: Go blog—Using Subtests and Sub-benchmarks: https://go.dev/blog/subtests
[^2]: Go testing package documentation: https://pkg.go.dev/testing
[^3]: Mitchell Hashimoto—Advanced Testing with Go (GopherCon 2017): https://www.youtube.com/watch?v=8hQG7QlcLBk
