# 10.4 Linting & Code Quality

<!-- [STRUCTURAL] Section structure: go-vet limits → meta-linter intro → config walkthrough (linter-by-linter) → issues section → local runs → violation patterns → JVM comparison → exercises. Strong progression. -->
<!-- [LINE EDIT] "Every language ecosystem has a point at which the built-in compiler checks stop being enough." — great opening line. -->
Every language ecosystem has a point at which the built-in compiler checks stop being enough. Go's compiler is strict about syntax and types, but it says nothing about unchecked errors, dead code, or whether your `fmt.Sprintf` format string matches its arguments. `go vet` fills some of that gap. `golangci-lint` fills the rest.

<!-- [COPY EDIT] "`fmt.Sprintf`" — backticks consistent. Good. -->

---

## Why `go vet` Is Not Enough

<!-- [LINE EDIT] "`go vet` is a correctness checker. It ships with the Go toolchain and catches a specific, curated set of bugs:" — clean. -->
`go vet` is a correctness checker. It ships with the Go toolchain and catches a specific, curated set of bugs:

<!-- [COPY EDIT] "-- " used as bullet separator; inconsistent with typography elsewhere. Convert bullet-internal `--` to em dash or replace with a colon. Example: "**Printf format string mismatches** — `fmt.Sprintf(...)` ..." -->
- **Printf format string mismatches** -- `fmt.Sprintf("%d", "not an int")` compiles fine but produces wrong output at runtime. `go vet` catches this statically.
- **Struct tag validity** -- `json:"name,omitempty"` has correct syntax; `json:"name omitempty"` does not. `go vet` validates struct tags.
<!-- [LINE EDIT] "Unreachable code -- code after a `return` statement in a branch." — minor redundancy ("code ... code"). Suggest: "**Unreachable code** — statements after an unconditional `return`." -->
- **Unreachable code** -- code after a `return` statement in a branch.
- **Misuse of `sync/atomic`** -- passing a non-pointer to `atomic.AddInt64` or similar.
<!-- [COPY EDIT] "calling `t.Fatal` from a goroutine, which panics instead of failing the test" — `t.Fatal` from a goroutine will actually just exit that goroutine (runtime.Goexit), not the test or main; `go vet` flags this because the test continues running. Please verify the exact wording: the failure mode is "does not fail the test as expected" rather than "panics". -->
- **Incorrect use of `testing.T`** -- calling `t.Fatal` from a goroutine, which panics instead of failing the test.

<!-- [LINE EDIT] "These are real bugs. Running `go vet ./...` as part of your CI pipeline is non-negotiable." — strong. Keep. -->
These are real bugs. Running `go vet ./...` as part of your CI pipeline is non-negotiable. But `go vet` deliberately has a narrow scope. It does not tell you:

- Which function calls return an error that you silently discarded
- Which exported functions have been dead code for months
<!-- [COPY EDIT] "Whether `if x == true` could be simplified to `if x`" — CMOS 10.3: code inline. Good. -->
- Whether `if x == true` could be simplified to `if x`
- Which variable is assigned a value that is immediately overwritten and never read

<!-- [LINE EDIT] "For that, you need a broader set of static analysis tools. Running them separately is slow, because each one must parse and type-check the entire codebase independently. `golangci-lint` solves this." — good. -->
For that, you need a broader set of static analysis tools. Running them separately is slow, because each one must parse and type-check the entire codebase independently. `golangci-lint` solves this.

---

## What golangci-lint Is

<!-- [STRUCTURAL] The "meta-linter" framing is the key insight. Good placement. -->
<!-- [LINE EDIT] "`golangci-lint` is a meta-linter[^1]. It wraps dozens of individual Go linters and runs them in parallel, sharing a single AST parse and type-check pass across all of them." — good. -->
`golangci-lint` is a meta-linter[^1]. It wraps dozens of individual Go linters and runs them in parallel, sharing a single AST parse and type-check pass across all of them. This makes it significantly faster than running each linter in sequence -- a codebase that would take 30 seconds to analyze five times takes roughly 5 seconds to analyze once.

<!-- [COPY EDIT] "30 seconds to analyze five times takes roughly 5 seconds" — CMOS 9.7 prefers spelled-out numbers <100 in prose; "30 seconds" and "5 seconds" acceptable for measurements (CMOS 9.13), but "five times" correctly spelled out. Consistency is what matters. Acceptable as written. -->
It is the de facto standard Go linting tool. Most open-source Go projects, cloud-native projects, and enterprise Go codebases use it. It produces consistent, structured output and integrates with editors, GitHub Actions, and pre-commit hooks.

<!-- [COPY EDIT] "de facto" — italicized per CMOS 7.55 when used as foreign phrase. Some style guides treat it as naturalized English; consistent with Merriam-Webster roman. Either is acceptable; match book-wide convention. -->
<!-- [LINE EDIT] "The configuration lives in a single `.golangci.yml` file at the repo root (or service root). You opt into linters explicitly -- the list is long, and turning everything on produces too much noise for a new codebase." — good. -->
The configuration lives in a single `.golangci.yml` file at the repo root (or service root). You opt into linters explicitly -- the list is long, and turning everything on produces too much noise for a new codebase.

---

## Walking Through `.golangci.yml`

Here is the configuration used across all five services in this project:

<!-- [COPY EDIT] Please verify: `.golangci.yml` schema — `linters.enable` is correct field name; `issues.exclude-use-default` still valid in golangci-lint v1.64.x. Recent golangci-lint v2 introduced schema changes (renamed fields, new config format). Confirm v1.x schema vs v2.x. If the Earthfile installs `v1.64.8`, the v1 schema applies. -->
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

<!-- [STRUCTURAL] Good "conservative starting set" framing — manages reader expectations. -->
This is a conservative starting set. It catches the bugs that matter most in Go without overwhelming you with stylistic opinions on a codebase you are still actively developing.

### `govet`

Runs the same checks as `go vet` but through golangci-lint's reporting pipeline. This means you get a consistent output format alongside your other linters, rather than switching tools. Printf mismatches, struct tag errors, atomic misuse -- all covered here.

### `errcheck`

<!-- [STRUCTURAL] Flagging errcheck as "the most important linter for Go newcomers" is exactly right. Strong tutor voice. -->
<!-- [LINE EDIT] "In Go, errors are return values, not exceptions. A function that can fail returns `(result, error)`. The language does not force you to handle the error -- you can discard it with `_` or just not capture the second return value at all." — good. -->
This is the most important linter for Go newcomers. In Go, errors are return values, not exceptions. A function that can fail returns `(result, error)`. The language does not force you to handle the error -- you can discard it with `_` or just not capture the second return value at all.

`errcheck` finds every place where you called a function that returns an error and did not check it. This is extremely common when calling I/O operations:

```go
// errcheck will flag this:
file.Close()

// Fix:
if err := file.Close(); err != nil {
    return fmt.Errorf("close file: %w", err)
}
```

<!-- [LINE EDIT] "The unflagged version will silently swallow any error that `Close` returns -- a partial write, a network flush failure, a permission error on a temp file." — good. -->
The unflagged version will silently swallow any error that `Close` returns -- a partial write, a network flush failure, a permission error on a temp file. In production this surfaces as data corruption or silent data loss. `errcheck` makes these invisible discards visible.

### `staticcheck`

<!-- [COPY EDIT] "Dominik Honnef" — verify spelling. Good as written. -->
`staticcheck` is a broad static analysis suite maintained by Dominik Honnef[^2]. It is not a single check -- it is a collection of hundreds of checks organized by category:

<!-- [COPY EDIT] Bullet list uses "--" as dash; convert to em dash. -->
<!-- [COPY EDIT] "(static analysis)", "(simple)", "(style)", "(quickfix)" — parenthetical glosses. OK. -->
- `SA` (static analysis) -- bugs and correctness issues. `SA1006` catches `Printf` calls with no formatting verbs; `SA4003` catches comparing unsigned integers to negative numbers.
- `S` (simple) -- code that can be rewritten more idiomatically. `S1000` flags `select` with a single case (just use the channel operation directly).
<!-- [COPY EDIT] "should be MixedCaps, not snake_case" — Go convention name. MixedCaps is correct (CamelCase with leading cap is reserved for exported). Good. -->
- `ST` (style) -- conventions. `ST1003` checks naming conventions (exported names should be MixedCaps, not snake_case).
- `QF` (quickfix) -- suggestions with known automated fixes.

<!-- [LINE EDIT] "`staticcheck` overlaps with `govet` in some areas but goes considerably further. In the JVM world, it is closest to the combination of SpotBugs and FindBugs -- broad pattern-based analysis that finds real bugs without requiring annotations or custom rules." — good. -->
<!-- [COPY EDIT] SpotBugs and FindBugs: FindBugs is effectively deprecated; SpotBugs is its successor. Listing "SpotBugs and FindBugs" is redundant/misleading. Recommend: "closest to SpotBugs (the successor to FindBugs) — broad pattern-based analysis..." -->
`staticcheck` overlaps with `govet` in some areas but goes considerably further. In the JVM world, it is closest to the combination of SpotBugs and FindBugs -- broad pattern-based analysis that finds real bugs without requiring annotations or custom rules.

### `unused`

Dead code detection. `unused` finds unexported identifiers -- functions, types, variables, constants -- that are defined but never referenced. This matters in a microservices project where services share packages: it is easy to add a helper function during development and then refactor away all its callers without noticing.

<!-- [STRUCTURAL] Good clarification about exported-identifier caveat. Essential for the JVM reader who might expect global dead-code detection. -->
Note that `unused` only checks unexported identifiers. Exported identifiers are assumed to be part of a public API and could be used by external code that the linter cannot see.

### `gosimple`

<!-- [LINE EDIT] "Flags code that is correct but unnecessarily verbose. Some examples:" — fine. -->
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

<!-- [COPY EDIT] "// Simplification (when no transformation occurs): result := input" — this is actually incorrect Go if `input` and `result` must remain independent slices. The "simplification" copies the slice header (aliasing the backing array). gosimple would NOT flag a plain identity loop like this (it doesn't know semantics). The textbook fix using `slices.Clone(input)` or `append([]string(nil), input...)` preserves isolation. Recommend replacing the second example with a canonical gosimple suggestion such as `S1039` (unnecessary `fmt.Sprintf` with only a single literal argument) or `S1011` (use `append(a, b...)` instead of a loop). -->
<!-- [STRUCTURAL] This example is technically risky. Consider replacing with a safer canonical gosimple case. -->
`gosimple` is not about style preferences -- it identifies patterns that are objectively longer than their equivalent. It is part of the `staticcheck` suite.

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

<!-- [LINE EDIT] "In the example above, if `doFirst()` returns an error, it is silently overwritten by `doSecond()`. The fix is to check the error from `doFirst` before proceeding:" — good. -->
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

<!-- [LINE EDIT] "Runs a full type-check pass before other linters execute. If the code does not compile, other linters will produce misleading output (or crash). `typecheck` acts as a fast pre-check that short-circuits the run with clear compilation errors. You almost certainly want this enabled." (45 words) — OK. -->
<!-- [COPY EDIT] Please verify: in golangci-lint, `typecheck` is not strictly a separate linter you enable — it's implicit. Enabling it in `linters.enable` is usually a no-op. Confirm the behavior in v1.64.x: does listing `typecheck` in `enable` do anything meaningful? -->
Runs a full type-check pass before other linters execute. If the code does not compile, other linters will produce misleading output (or crash). `typecheck` acts as a fast pre-check that short-circuits the run with clear compilation errors. You almost certainly want this enabled.

---

## The `issues` Section

```yaml
issues:
  exclude-use-default: true
```

<!-- [LINE EDIT] "`golangci-lint` ships with a list of default exclusions -- patterns that are known to be noisy false positives." — good. -->
<!-- [COPY EDIT] "false positives" — not hyphenated (noun phrase). Good. -->
`golangci-lint` ships with a list of default exclusions -- patterns that are known to be noisy false positives. Examples: `errcheck` ignoring `fmt.Fprintf` errors to standard output (writing to a terminal rarely fails in a meaningful way), or `staticcheck` suppressing certain warnings in test files.

<!-- [COPY EDIT] Please verify: as of golangci-lint v1.64.x, the correct key is `issues.exclude-use-default` (older docs) vs `issues.exclude-dirs-use-default` / `issues.exclude-use-default`. Verify field name. -->
Setting `exclude-use-default: true` opts into this curated list. Without it, you get the full output including these patterns, which produces noise and trains developers to ignore linter output. Start with the defaults applied; only remove specific exclusions if you are confident you want to address them.

<!-- [STRUCTURAL] Note: `exclude-use-default: true` is actually the default behavior in golangci-lint. Setting it explicitly is redundant but harmless. Consider noting that this line "makes the implicit explicit — the default is already true, but stating it documents the intent." -->

---

## Running Locally

<!-- [STRUCTURAL] Two options pattern is clear. Good balance. -->
You have two options.

**Option 1: golangci-lint installed locally**

```sh
# from a service directory
golangci-lint run ./...
```

<!-- [LINE EDIT] "This requires `golangci-lint` to be installed on your machine (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`). The advantage is speed -- no container startup, and the binary caches results between runs." — good. -->
<!-- [COPY EDIT] "`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`" — this installs latest, which contradicts the chapter's emphasis on pinning. Consider: "...`@v1.64.8`" (the version the Earthfile uses) to emphasize reproducibility. -->
This requires `golangci-lint` to be installed on your machine (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`). The advantage is speed -- no container startup, and the binary caches results between runs.

**Option 2: Earthly (no local install required)**

```sh
# lint all services from the repo root
earthly +lint

# lint a single service
earthly ./services/catalog+lint
```

<!-- [COPY EDIT] "tradeoff" vs "trade-off" — line 199. Earlier in chapter file uses "trade-off". Be consistent. Merriam-Webster accepts both; hyphenated form is more common in edited prose. -->
<!-- [LINE EDIT] "Earthly runs the linter inside a container. The Go toolchain and `golangci-lint` binary are baked into the image, so this works identically on every developer's machine and in CI. The tradeoff is slightly longer startup time on the first run (image pull). Subsequent runs are faster due to layer caching." (48 words) — acceptable. -->
Earthly runs the linter inside a container. The Go toolchain and `golangci-lint` binary are baked into the image, so this works identically on every developer's machine and in CI. The tradeoff is slightly longer startup time on the first run (image pull). Subsequent runs are faster due to layer caching.

<!-- [COPY EDIT] "byte-for-byte" — hyphenated compound adjective (CMOS 7.81). Good. -->
In CI, the Earthly path is always used. This ensures the CI environment is byte-for-byte identical to what developers run locally.

---

## Common Violation Patterns and Fixes

<!-- [STRUCTURAL] Four concrete violation-fix pairs. Strong pedagogy. Each pattern is one any experienced engineer has hit. -->

### 1. Unchecked error return (errcheck)

```go
// Violation: rows.Close() returns an error that is discarded
rows, err := db.QueryContext(ctx, query)
if err != nil {
    return nil, err
}
defer rows.Close()
```

<!-- [COPY EDIT] The "Fix" example below uses `err` as a named return, but the surrounding function signature is not shown. A reader might copy-paste and find `err` undefined. Consider: "```go\nfunc queryRows(ctx context.Context, db *sql.DB, query string) (_ *sql.Rows, err error) {\n    rows, err := db.QueryContext(ctx, query)\n    if err != nil {\n        return nil, err\n    }\n    defer func() {\n        if cerr := rows.Close(); cerr != nil && err == nil {\n            err = fmt.Errorf(\"close rows: %w\", cerr)\n        }\n    }()\n    ...\n}\n```" -->
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

<!-- [COPY EDIT] The fix uses `decoded` as the result variable, but the original code path uses `data`. Flow-break is deliberate (reader sees both the ineffassign fix and an idiomatic rename), but consider a comment: "// Fix: check the first error, and avoid shadowing `data`." -->

### 3. Simplifiable boolean expression (gosimple)

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

<!-- [COPY EDIT] This example is actually an `S1008` (comparing bool to literal) or similar — `gosimple`/`staticcheck` categories. The heading says "comparing bool to literal" but the code compares a string to a string literal. The violation rule flagged is `if … { return true } else { return false }` (equivalent to returning the condition). Consider renaming heading: "Unnecessary if/else around a returned expression (S1008)". -->

### 4. Unused unexported function (unused)

```go
// Violation: parseISBN is defined but never called after a refactor
func parseISBN(raw string) (string, error) {
    // ...
}
```

<!-- [LINE EDIT] "The fix is either to delete the function, or -- if it is genuinely useful -- add a call to it or export it if it belongs in the package's public API. The linter is telling you that the function is currently dead weight." — good. -->
The fix is either to delete the function, or -- if it is genuinely useful -- add a call to it or export it if it belongs in the package's public API. The linter is telling you that the function is currently dead weight.

---

## JVM Comparison

<!-- [STRUCTURAL] Analogies table: useful. -->
If you are coming from a Java or Kotlin background, the mental model maps well:

| Go tool | JVM equivalent |
|---|---|
| `golangci-lint` | Checkstyle + SpotBugs + PMD + ErrorProne in one runner |
| `.golangci.yml` | `checkstyle.xml` or Detekt config (`detekt.yml`) |
<!-- [COPY EDIT] "`@CheckReturnValue` + `MustBeClosed`" — ErrorProne annotation names. Verify: `@CheckReturnValue` is correct; `MustBeClosed` should be `@MustBeClosed`. -->
| `errcheck` | ErrorProne's `@CheckReturnValue` + `MustBeClosed` |
<!-- [COPY EDIT] "SpotBugs + FindBugs" — redundant (FindBugs is deprecated). Recommend "SpotBugs (successor to FindBugs)". -->
| `staticcheck` | SpotBugs + FindBugs |
| `gosimple` | IntelliJ's inspection warnings |
| `ineffassign` | PMD's `UnusedAssignment` rule |
| `unused` | IntelliJ's "unused declaration" inspection |

<!-- [LINE EDIT] "The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, vim, VS Code), but it is designed to be run as part of the build pipeline -- not just to produce squiggly lines in an editor. This makes it straightforward to block a CI pipeline on lint failures, which is how this project uses it." (65 words) — split: "The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, vim, VS Code), but it is designed to be run as part of the build pipeline — not just to produce squiggly lines in the editor. That makes it straightforward to block a CI pipeline on lint failures, as this project does." -->
The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, vim, VS Code), but it is designed to be run as part of the build pipeline -- not just to produce squiggly lines in an editor. This makes it straightforward to block a CI pipeline on lint failures, which is how this project uses it.

<!-- [COPY EDIT] "vim" — Vim is a proper noun; capitalize per vendor convention ("Vim"). "VS Code" capitalized correctly. -->
<!-- [LINE EDIT] "Another difference: Go does not have annotation-based suppression (`@SuppressWarnings`). You can add `//nolint:errcheck` comments to specific lines, but this is a last resort, not a routine practice." — good. -->
Another difference: Go does not have annotation-based suppression (`@SuppressWarnings`). You can add `//nolint:errcheck` comments to specific lines, but this is a last resort, not a routine practice. If `golangci-lint` flags something, the right response is usually to fix it.

---

## Exercises

<!-- [STRUCTURAL] Exercises: introduce violation, audit codebase, add formatter linter, investigate default exclusions. Good practical sequence. -->
1. **Introduce a violation and catch it.** In any service, add a call to a function that returns an error but do not check the return value (e.g., `w.Write(data)` in an HTTP handler). Run `golangci-lint run ./...` from that service directory and observe the errcheck output. Fix it.

2. **Audit the existing codebase.** Run `earthly +lint` from the repo root. If any violations appear, investigate each one -- is it a real bug, a false positive, or a code smell worth addressing? Make a note of the patterns you find.

<!-- [COPY EDIT] "Add `gofmt` or `goimports` to the `linters.enable` list" — `gofmt` and `goimports` are available as golangci-lint linters. Good. Consider noting that since golangci-lint v2, some linter names have changed; verify for v1.64.x. -->
3. **Add a new linter.** Add `gofmt` or `goimports` to the `linters.enable` list in `.golangci.yml`. Run the linter. How many files have formatting issues? What does it say about the project's formatter usage?

<!-- [LINE EDIT] "Remove `exclude-use-default: true` from `.golangci.yml` and re-run the linter. How many additional violations appear? Are they false positives or real issues? Restore the setting and document what you found." — good. -->
4. **Understand the default exclusions.** Remove `exclude-use-default: true` from `.golangci.yml` and re-run the linter. How many additional violations appear? Are they false positives or real issues? Restore the setting and document what you found.

---

## References

<!-- [COPY EDIT] Please verify URLs resolve: https://golangci-lint.run/ ; https://staticcheck.dev/docs/ ; https://github.com/kisielk/errcheck -->
[^1]: [golangci-lint documentation](https://golangci-lint.run/) -- Official documentation covering installation, configuration, and the full list of supported linters.
[^2]: [staticcheck documentation](https://staticcheck.dev/docs/) -- Reference for all SA, S, ST, and QF checks, including examples of each violation and its fix.
[^3]: [errcheck GitHub repository](https://github.com/kisielk/errcheck) -- The standalone errcheck tool, which golangci-lint wraps. The README has additional examples and configuration options.
