# 10.4 Linting & Code Quality

Every language ecosystem has a point at which the built-in compiler checks stop being enough. Go's compiler is strict about syntax and types, but it says nothing about unchecked errors, dead code, or whether your `fmt.Sprintf` format string matches its arguments. `go vet` fills some of that gap. `golangci-lint` fills the rest.

---

## Why `go vet` Is Not Enough

`go vet` is a correctness checker. It ships with the Go toolchain and catches a specific, curated set of bugs:

- **Printf format string mismatches** — `fmt.Sprintf("%d", "not an int")` compiles fine but produces wrong output at runtime. `go vet` catches this statically.
- **Struct tag validity** — `json:"name,omitempty"` has correct syntax; `json:"name omitempty"` does not. `go vet` validates struct tags.
- **Unreachable code** — statements after an unconditional `return`.
- **Misuse of `sync/atomic`** — passing a non-pointer to `atomic.AddInt64` or similar.
- **Incorrect use of `testing.T`** — calling `t.Fatal` from a goroutine, which panics instead of failing the test.

These are real bugs. Running `go vet ./...` as part of your CI pipeline is non-negotiable. But `go vet` deliberately has a narrow scope. It does not tell you:

- Which function calls return an error that you silently discarded
- Which exported functions have been dead code for months
- Whether `if x == true` could be simplified to `if x`
- Which variable is assigned a value that is immediately overwritten and never read

For that, you need a broader set of static analysis tools. Running them separately is slow, because each one must parse and type-check the entire codebase independently. `golangci-lint` solves this.

---

## What golangci-lint Is

`golangci-lint` is a meta-linter[^1]. It wraps dozens of individual Go linters and runs them in parallel, sharing a single AST parse and type-check pass across all of them. This makes it significantly faster than running each linter in sequence — a codebase that would take 30 seconds to analyze five times takes roughly 5 seconds to analyze once.

It is the de facto standard Go linting tool. Most open-source Go projects, cloud-native projects, and enterprise Go codebases use it. It produces consistent, structured output and integrates with editors, GitHub Actions, and pre-commit hooks.

The configuration lives in a single `.golangci.yml` file at the repo root (or service root). You opt into linters explicitly — the list is long, and turning everything on produces too much noise for a new codebase.

---

## Walking Through `.golangci.yml`

Here is the configuration used across all five services in this project:

```yaml
run:
  timeout: 5m

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

issues:
  exclude-use-default: true
```

This is a conservative starting set. It catches the bugs that matter most in Go without overwhelming you with stylistic opinions on a codebase you are still actively developing.

### `govet`

Runs the same checks as `go vet` but through golangci-lint's reporting pipeline. This means you get a consistent output format alongside your other linters, rather than switching tools. Printf mismatches, struct tag errors, atomic misuse — all covered here.

### `errcheck`

This is the most important linter for Go newcomers. In Go, errors are return values, not exceptions. A function that can fail returns `(result, error)`. The language does not force you to handle the error — you can discard it with `_` or just not capture the second return value at all.

`errcheck` finds every place where you called a function that returns an error and did not check it. This is extremely common when calling I/O operations:

```go
// errcheck will flag this:
file.Close()

// Fix:
if err := file.Close(); err != nil {
    return fmt.Errorf("close file: %w", err)
}
```

The unflagged version will silently swallow any error that `Close` returns — a partial write, a network flush failure, a permission error on a temp file. In production this surfaces as data corruption or silent data loss. `errcheck` makes these invisible discards visible.

### `staticcheck`

`staticcheck` is a broad static analysis suite maintained by Dominik Honnef[^2]. It is not a single check — it is a collection of hundreds of checks organized by category:

- `SA` (static analysis) — bugs and correctness issues. `SA1006` catches `Printf` calls with no formatting verbs; `SA4003` catches comparing unsigned integers to negative numbers.
- `S` (simple) — code that can be rewritten more idiomatically. `S1000` flags `select` with a single case (just use the channel operation directly).
- `ST` (style) — conventions. `ST1003` checks naming conventions (exported names should be MixedCaps, not snake_case).
- `QF` (quickfix) — suggestions with known automated fixes.

`staticcheck` overlaps with `govet` in some areas but goes considerably further. In the JVM world, it is closest to SpotBugs (the successor to FindBugs) — broad pattern-based analysis that finds real bugs without requiring annotations or custom rules.

### `unused`

Dead code detection. `unused` finds unexported identifiers — functions, types, variables, constants — that are defined but never referenced. This matters in a microservices project where services share packages: it is easy to add a helper function during development and then refactor away all its callers without noticing.

Note that `unused` only checks unexported identifiers. Exported identifiers are assumed to be part of a public API and could be used by external code that the linter cannot see.

### `gosimple`

Flags code that is correct but unnecessarily verbose. Some examples:

```go
// gosimple flags this:
if x == true {
    doSomething()
}

// Simplification:
if x {
    doSomething()
}

// gosimple flags this:
var result []string
for _, s := range input {
    result = append(result, s)
}

// Simplification (when no transformation occurs):
result := input
```

`gosimple` is not about style preferences — it identifies patterns that are objectively longer than their equivalent. It is part of the `staticcheck` suite.

### `ineffassign`

Catches assignments to variables that are never read afterward. The most common case is reusing an `err` variable:

```go
func process() error {
    err := doFirst()
    err = doSecond()  // ineffassign: previous value of err is never used
    if err != nil {
        return err
    }
    return nil
}
```

In the example above, if `doFirst()` returns an error, it is silently overwritten by `doSecond()`. The fix is to check the error from `doFirst` before proceeding:

```go
func process() error {
    if err := doFirst(); err != nil {
        return fmt.Errorf("first step: %w", err)
    }
    if err := doSecond(); err != nil {
        return fmt.Errorf("second step: %w", err)
    }
    return nil
}
```

### `typecheck`

Runs a full type-check pass before other linters execute. If the code does not compile, other linters will produce misleading output (or crash). `typecheck` acts as a fast pre-check that short-circuits the run with clear compilation errors. You almost certainly want this enabled.

---

## The `issues` Section

```yaml
issues:
  exclude-use-default: true
```

`golangci-lint` ships with a list of default exclusions — patterns that are known to be noisy false positives. Examples: `errcheck` ignoring `fmt.Fprintf` errors to standard output (writing to a terminal rarely fails in a meaningful way), or `staticcheck` suppressing certain warnings in test files.

Setting `exclude-use-default: true` opts into this curated list. Without it, you get the full output including these patterns, which produces noise and trains developers to ignore linter output. Start with the defaults applied; only remove specific exclusions if you are confident you want to address them.

---

## Running Locally

You have two options.

**Option 1: golangci-lint installed locally**

```sh
# from a service directory
golangci-lint run ./...
```

This requires `golangci-lint` to be installed on your machine (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8`). The advantage is speed — no container startup, and the binary caches results between runs.

**Option 2: Earthly (no local install required)**

```sh
# lint all services from the repo root
earthly +lint

# lint a single service
earthly ./services/catalog+lint
```

Earthly runs the linter inside a container. The Go toolchain and `golangci-lint` binary are baked into the image, so this works identically on every developer's machine and in CI. The trade-off is slightly longer startup time on the first run (image pull). Subsequent runs are faster due to layer caching.

In CI, the Earthly path is always used. This ensures the CI environment is byte-for-byte identical to what developers run locally.

---

## Common Violation Patterns and Fixes

### 1. Unchecked error return (errcheck)

```go
// Violation: rows.Close() returns an error that is discarded
rows, err := db.QueryContext(ctx, query)
if err != nil {
    return nil, err
}
defer rows.Close()
```

```go
// Fix: wrap Close in a named return or a deferred check
defer func() {
    if cerr := rows.Close(); cerr != nil && err == nil {
        err = fmt.Errorf("close rows: %w", cerr)
    }
}()
```

### 2. Ineffective assignment (ineffassign)

```go
// Violation: err from json.Marshal is overwritten before being checked
data, err := json.Marshal(payload)
data, err = base64.StdEncoding.DecodeString(string(data))
if err != nil {
    return err
}
```

```go
// Fix: check the first error before proceeding
data, err := json.Marshal(payload)
if err != nil {
    return fmt.Errorf("marshal payload: %w", err)
}
decoded, err := base64.StdEncoding.DecodeString(string(data))
if err != nil {
    return fmt.Errorf("decode base64: %w", err)
}
```

### 3. Unnecessary if/else around a boolean return (S1008)

```go
// Violation: comparing bool to literal
func isAvailable(status string) bool {
    if status == "available" {
        return true
    } else {
        return false
    }
}
```

```go
// Fix: return the expression directly
func isAvailable(status string) bool {
    return status == "available"
}
```

### 4. Unused unexported function (unused)

```go
// Violation: parseISBN is defined but never called after a refactor
func parseISBN(raw string) (string, error) {
    // ...
}
```

The fix is either to delete the function, or — if it is genuinely useful — add a call to it or export it if it belongs in the package's public API. The linter is telling you that the function is currently dead weight.

---

## JVM Comparison

If you are coming from a Java or Kotlin background, the mental model maps well:

| Go tool | JVM equivalent |
|---|---|
| `golangci-lint` | Checkstyle + SpotBugs + PMD + ErrorProne in one runner |
| `.golangci.yml` | `checkstyle.xml` or Detekt config (`detekt.yml`) |
| `errcheck` | ErrorProne's `@CheckReturnValue` + `@MustBeClosed` |
| `staticcheck` | SpotBugs (successor to FindBugs) |
| `gosimple` | IntelliJ's inspection warnings |
| `ineffassign` | PMD's `UnusedAssignment` rule |
| `unused` | IntelliJ's "unused declaration" inspection |

The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, Vim, VS Code), but it is designed to be run as part of the build pipeline — not just to produce squiggly lines in the editor. That makes it straightforward to block a CI pipeline on lint failures, as this project does.

Another difference: Go does not have annotation-based suppression (`@SuppressWarnings`). You can add `//nolint:errcheck` comments to specific lines, but this is a last resort, not a routine practice. If `golangci-lint` flags something, the right response is usually to fix it.

---

## Exercises

1. **Introduce a violation and catch it.** In any service, add a call to a function that returns an error but do not check the return value (e.g., `w.Write(data)` in an HTTP handler). Run `golangci-lint run ./...` from that service directory and observe the errcheck output. Fix it.

2. **Audit the existing codebase.** Run `earthly +lint` from the repo root. If any violations appear, investigate each one — is it a real bug, a false positive, or a code smell worth addressing? Make a note of the patterns you find.

3. **Add a new linter.** Add `gofmt` or `goimports` to the `linters.enable` list in `.golangci.yml`. Run the linter. How many files have formatting issues? What does it say about the project's formatter usage?

4. **Understand the default exclusions.** Remove `exclude-use-default: true` from `.golangci.yml` and re-run the linter. How many additional violations appear? Are they false positives or real issues? Restore the setting and document what you found.

---

## References

[^1]: [golangci-lint documentation](https://golangci-lint.run/) — Official documentation covering installation, configuration, and the full list of supported linters.
[^2]: [staticcheck documentation](https://staticcheck.dev/docs/) — Reference for all SA, S, ST, and QF checks, including examples of each violation and its fix.
[^3]: [errcheck GitHub repository](https://github.com/kisielk/errcheck) — The standalone errcheck tool, which golangci-lint wraps. The README has additional examples and configuration options.
