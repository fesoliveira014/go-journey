<!-- [STRUCTURAL] Heading level inconsistency across chapter: index.md uses `# Chapter 1`, project-setup.md uses `# 1.1 — Project Setup` (H1), but this file opens with `## 1.2 Go Language Essentials` (H2). Pick one — if the chapter is rendered by stitching these files, H1 per file risks duplicating the chapter title; if each file is rendered standalone, H2 means the document has no H1. Recommend H1 per file with an explicit chapter anchor (consistent with project-setup). Same issue in http-server.md and testing.md. -->
## 1.2 Go Language Essentials

<!-- [LINE EDIT] "This section moves fast." — strong opener, keep. -->
<!-- [LINE EDIT] "You already know what a struct is, what an interface is, and why error handling matters." — consider tightening: "You already know what structs and interfaces are, and why error handling matters." The anaphora emphasises Go's parallelism but padding is noticeable. Judgment call. -->
This section moves fast. You already know what a struct is, what an interface is, and why error handling matters. The goal here is to show you how Go's versions of these things differ from what you already know — and where those differences bite you if you come in with C++/Java assumptions.

---

### Types and Zero Values

<!-- [STRUCTURAL] The list of primitive types is dense but serves as reference. Consider wrapping it in a `<details>`/collapsed block so it doesn't dominate the reader's first impression of the section — they can expand when they need the reference. Judgment call. -->
<!-- [COPY EDIT] "`byte` (alias for `uint8`), `rune` (alias for `int32`)" — CMOS 6.19: serial comma not needed here (series ends with parenthetical). OK. -->
Go's primitive types will feel familiar: `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `float32`, `float64`, `bool`, `string`, `byte` (alias for `uint8`), `rune` (alias for `int32`).

<!-- [COPY EDIT] "`uint`" appears but the other unsigned sizes (`uint8`, `uint16`, `uint32`, `uint64`) are omitted. Either drop `uint` or include the full family for completeness. Also `uintptr` missing; acceptable to skip with a note. -->
Two things to internalize immediately:

**No implicit conversions.** This compiles in C, not in Go:

```go
var x int32 = 10
var y int64 = x // compile error: cannot use x (type int32) as type int64
```

<!-- [COPY EDIT] Please verify: the compiler error message text. Go 1.22+ typically phrases it as `cannot use x (variable of type int32) as int64 value in variable declaration`. Worth matching actual toolchain output, or loosening the comment to "compile error — type mismatch". -->
You must be explicit:

```go
var y int64 = int64(x)
```

This is intentional. Go treats implicit numeric promotion as a source of bugs and refuses to do it.

<!-- [LINE EDIT] "**Every type has a zero value.** Variables declared but not initialized are not garbage — they have deterministic defaults:" — solid. -->
**Every type has a zero value.** Variables declared but not initialized are not garbage — they have deterministic defaults:

<!-- [COPY EDIT] Table cell "`""` (empty string, not `null`)" — `null` is a JavaScript/Java term; Go has `nil`. The comparison to `null` is a bridge for non-Go readers and is fine, but consider "(empty string, not nil)" to stay in the Go idiom. -->
| Type | Zero value |
|------|-----------|
| `int`, `float64` | `0` |
| `bool` | `false` |
| `string` | `""` (empty string, not `null`) |
| pointer, slice, map, channel, func | `nil` |
| struct | all fields set to their zero values |

```go
var count int    // 0
var name string  // ""
var active bool  // false
var p *int       // nil
```

<!-- [LINE EDIT] "This eliminates an entire class of uninitialized-variable bugs. You will miss it when you go back to C." — nice closing line. -->
This eliminates an entire class of uninitialized-variable bugs. You will miss it when you go back to C.

<!-- [STRUCTURAL] The "No pointer arithmetic" note floats orphaned after the zero-values discussion. It belongs either in the Pointers section (lines 299–328) or with a short heading here. As-is, it feels wedged in. -->
<!-- [LINE EDIT] "No pointer arithmetic. `p++` on a pointer is a compile error." — fragment without bold lead-in, inconsistent with "**No implicit conversions.**" above. Apply same structure: "**No pointer arithmetic.** `p++` on a pointer is a compile error." -->
No pointer arithmetic. `p++` on a pointer is a compile error. Go is not C — the runtime manages memory, and the GC needs pointer provenance intact.

---

### Structs

<!-- [LINE EDIT] "Go has no classes. Structs are the unit of data grouping, and methods are attached to types — not enclosed within them." — strong. Keep. -->
Go has no classes. Structs are the unit of data grouping, and methods are attached to types — not enclosed within them.

```go
type Book struct {
    ID     string
    Title  string
    Author string
    Genre  string
    Year   int
}
```

Methods are defined outside the struct body, with an explicit receiver:

```go
func (b Book) String() string {
    return fmt.Sprintf("[%s] %s by %s (%d)", b.ID, b.Title, b.Author, b.Year)
}
```

**Initialization:** Go has two common forms.

```go
// Named fields — preferred, order-independent, future-proof
b := Book{
    ID:     "978-0-13-468599-1",
    Title:  "The Go Programming Language",
    Author: "Donovan & Kernighan",
    Genre:  "Technology",
    Year:   2015,
}

// Positional — fragile, avoid for structs with more than one or two fields
b2 := Book{"978-0-13-468599-1", "The Go Programming Language", "Donovan & Kernighan", "Technology", 2015}
```

<!-- [COPY EDIT] "Donovan & Kernighan" — ampersand in author list is informal; CMOS prefers "and" in running prose. In a code literal this is author style; no change. -->
**Embedding instead of inheritance.** Go has no `extends`. Composition is achieved by embedding one struct inside another:

```go
type Timestamps struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

type BookRecord struct {
    Book                   // embedded — fields and methods promoted
    Timestamps             // embedded
    AvailableCopies int
}
```

Fields and methods of the embedded types are promoted to the outer struct:

```go
record := BookRecord{
    Book:            b,
    Timestamps:      Timestamps{CreatedAt: time.Now(), UpdatedAt: time.Now()},
    AvailableCopies: 3,
}

fmt.Println(record.Title)      // promoted from Book
fmt.Println(record.CreatedAt)  // promoted from Timestamps
fmt.Println(record.String())   // promoted method from Book
```

<!-- [LINE EDIT] "This is not inheritance. The embedded type has no knowledge of the outer type. There is no `super`." — three short sentences in sequence are effective here. Keep. -->
<!-- [LINE EDIT] "If `BookRecord` needs different behavior, it defines its own method with the same name — the outer type's method takes precedence, and the inner one is still accessible via `record.Book.String()`." — 37 words; fine, and the structure mirrors the mechanism. -->
This is not inheritance. The embedded type has no knowledge of the outer type. There is no `super`. If `BookRecord` needs different behavior, it defines its own method with the same name — the outer type's method takes precedence, and the inner one is still accessible via `record.Book.String()`.

---

### Interfaces

<!-- [LINE EDIT] "In Java/Kotlin you write `implements Serializable`. In C++ you inherit from a pure-virtual base class. In Go, interface satisfaction is **implicit**: if your type has the methods, it satisfies the interface — no declaration required." — excellent tri-part contrast. Keep verbatim. -->
<!-- [COPY EDIT] "`implements Serializable`" — Serializable is a Java marker interface; fine. Kotlin also uses `:` with the interface name, not `implements`. Consider: "In Java you write `implements Serializable`; in Kotlin, you write `: Serializable`." Pedantic; author's call. -->
In Java/Kotlin you write `implements Serializable`. In C++ you inherit from a pure-virtual base class. In Go, interface satisfaction is **implicit**: if your type has the methods, it satisfies the interface — no declaration required.

```go
// Standard library interface — defined in package fmt
type Stringer interface {
    String() string
}
```

<!-- [LINE EDIT] "The compiler checks at the point of use." — the prepositional phrase order is slightly awkward; "The compiler checks the fit at the point of use." Or "The compiler verifies satisfaction where the interface is used." -->
`Book` above already satisfies `fmt.Stringer` because it has a `String() string` method. There is no annotation, no `implements`, nothing to register. The compiler checks at the point of use.

```go
func printItem(s fmt.Stringer) {
    fmt.Println(s.String())
}

printItem(b) // works — Book satisfies fmt.Stringer
```

**Defining your own interface:**

```go
type Repository interface {
    FindByID(ctx context.Context, id string) (Book, error)
    FindByGenre(ctx context.Context, genre string) ([]Book, error)
    Save(ctx context.Context, book Book) error
}
```

Any type with those three methods satisfies `Repository`. The concrete type — a Postgres implementation, a Redis cache, an in-memory fake — never needs to reference this interface.

<!-- [COPY EDIT] "Postgres" — CMOS product capitalization: the official product name is "PostgreSQL"; "Postgres" is acceptable as an accepted nickname. CLAUDE.md tech stack explicitly says PostgreSQL. For consistency across the book, prefer "PostgreSQL" in prose, "postgres" only when referring to the CLI/Docker image. Apply consistently. -->
**Why this matters for testing.** Because interfaces are satisfied implicitly, you can define a minimal interface in your test package and point it at a fake implementation without touching production code at all. This pattern will come up constantly in this project.

<!-- [LINE EDIT] "This pattern will come up constantly in this project." → "You will see this pattern constantly in this project." (active voice, author-reader relationship). -->

**The empty interface.** `any` (alias for `interface{}`) accepts any value:

<!-- [COPY EDIT] Please verify: `any` was added as a predeclared alias in Go 1.18; phrasing "alias for `interface{}`" is technically correct. No change needed. -->
```go
var v any = 42
v = "now a string"
v = Book{...}
```

<!-- [LINE EDIT] "Use it sparingly. It bypasses the type system. You will see it in generic utility functions and JSON unmarshalling." — rhythm is strong. -->
<!-- [COPY EDIT] "JSON unmarshalling" — CMOS/AmE prefers "unmarshaling" (single l) since "unmarshal" is a suffix with -ing. Go's own docs use "unmarshaling". Prefer "unmarshaling" for consistency with Go ecosystem. (Also flag: any other instances across the chapter.) -->
Use it sparingly. It bypasses the type system. You will see it in generic utility functions and JSON unmarshalling. When you need to get the concrete type back, use a type assertion or type switch:

```go
if book, ok := v.(Book); ok {
    fmt.Println(book.Title)
}
```

---

### Slices and Maps

<!-- [STRUCTURAL] This section combines slices and maps under one heading but gives most of the attention to slices; consider splitting into `### Slices` and `### Maps` for easier scanning. Minor. -->
<!-- [LINE EDIT] "**Slices** are Go's equivalent of `std::vector` or `ArrayList` — a dynamically-sized view over an underlying array." — "dynamically-sized" is a compound adj; correctly hyphenated. -->
<!-- [COPY EDIT] CMOS 7.81: "dynamically-sized" before noun — correct. But note Go convention often spells "dynamically sized" without hyphen (adverb-adjective compound where adverb ends in -ly is not hyphenated per CMOS 7.82). Should be "dynamically sized view". -->
**Slices** are Go's equivalent of `std::vector` or `ArrayList` — a dynamically-sized view over an underlying array. Unlike C arrays, slices carry their length and capacity.

```go
// nil slice — zero value, no allocation
var books []Book

// empty slice — allocated, length 0
books = make([]Book, 0)
books = []Book{}

// with initial elements
books = []Book{b1, b2, b3}

// append — may reallocate, always reassign
books = append(books, Book{Title: "Clean Code"})
```

<!-- [LINE EDIT] "A nil slice and an empty slice behave the same for `len`, `append`, and `range` — but they are not equal under `reflect.DeepEqual`." — 22 words; fine. -->
<!-- [COPY EDIT] Please verify: `reflect.DeepEqual(nil []int, []int{})` — actually, `reflect.DeepEqual([]int(nil), []int{})` returns `false`. Claim is correct. Leave. -->
A nil slice and an empty slice behave the same for `len`, `append`, and `range` — but they are not equal under `reflect.DeepEqual`. The convention is to return a nil slice for "no results" rather than an empty one, and let callers use `len(result) == 0` to check emptiness.

**Iteration** always uses `range`:

```go
for i, book := range books {
    fmt.Printf("%d: %s\n", i, book.Title)
}

// discard index
for _, book := range books {
    fmt.Println(book.String())
}
```

<!-- [LINE EDIT] "Unlike C++, range gives you a **copy** of each element." — fine. -->
<!-- [COPY EDIT] Please verify: "Unlike C++, range gives you a **copy** of each element" — C++ range-for also gives a copy by default unless you bind to a reference (`for (auto& x : v)`). The contrast is accurate but could mislead. Consider: "Unlike C++'s range-for (where `auto&` is idiomatic for mutation), Go's range always gives you a copy — there is no reference binding." -->
Unlike C++, range gives you a **copy** of each element. Mutating `book` inside the loop does not affect the slice. Use the index if you need to modify in place: `books[i].Year = 2024`.

**Slicing a slice** produces a new header over the same backing array — not a copy:

```go
first3 := books[0:3] // shares memory with books
```

<!-- [LINE EDIT] "This is a common gotcha." — brief; keep. -->
This is a common gotcha. If you need an independent copy: `copy(dst, src)`.

**Maps** are Go's equivalent of `std::unordered_map` or `HashMap`:

```go
// nil map — reads return zero value, writes panic
var index map[string]Book

// always initialize before writing
index = make(map[string]Book)

index["978-0-13-468599-1"] = b

// two-value lookup — safe, no exception on missing key
book, ok := index["missing-key"]
if !ok {
    // key not present
}

// delete
delete(index, "978-0-13-468599-1")
```

<!-- [LINE EDIT] "Map iteration order is deliberately randomized on each run — do not rely on it." — strong, keep. -->
Map iteration order is deliberately randomized on each run — do not rely on it.

---

### Error Handling

<!-- [STRUCTURAL] The error-handling section is the conceptual heart of the chapter for a Java/C++ convert. It's well-motivated, but consider adding a brief note on `errors.New` vs `fmt.Errorf` (sentinel errors) before the wrapping discussion — readers often confuse the two when they first see `%w`. -->
Go has no exceptions. Errors are values — functions return them as an additional return value, and callers check them explicitly.

```go
func (r *InMemoryRepo) FindByGenre(genre string) ([]Book, error) {
    var result []Book
    for _, b := range r.books {
        if b.Genre == genre {
            result = append(result, b)
        }
    }
    if len(result) == 0 {
        return nil, fmt.Errorf("no books found for genre %q", genre)
    }
    return result, nil
}
```

<!-- [COPY EDIT] The snippet uses `fmt.Errorf("no books found for genre %q", genre)` here but the exercise solution later uses `fmt.Errorf("%w for genre %q", ErrNotFound, genre)` with a sentinel. The running example could introduce the sentinel earlier so the exercise doesn't seem to introduce a new pattern. -->
<!-- [LINE EDIT] "The `error` type is a built-in interface with one method: `Error() string`. Any type that implements it is an error." — clean. -->
The `error` type is a built-in interface with one method: `Error() string`. Any type that implements it is an error.

**Wrapping errors** with `%w` lets you add context while preserving the original error for programmatic inspection:

```go
books, err := repo.FindByGenre(genre)
if err != nil {
    return fmt.Errorf("catalog service: %w", err)
}
```

**Unwrapping** errors uses `errors.Is` (identity check) and `errors.As` (type check):

```go
var notFound *NotFoundError
if errors.As(err, &notFound) {
    // handle specifically
}

if errors.Is(err, ErrNotFound) {
    // sentinel value check
}
```

<!-- [COPY EDIT] The snippet references `*NotFoundError` and `ErrNotFound` without prior definition. A one-liner above the block — "assume `ErrNotFound` is a package-level sentinel and `*NotFoundError` is a typed error" — would help the reader. -->

<!-- [LINE EDIT] "Contrast with Java: no try/catch, no checked exceptions, no `throws` declarations." — fine. -->
<!-- [COPY EDIT] CMOS 6.63 — capitalization after colon: "no try/catch..." is a list of noun phrases, not a full sentence, so lowercase is correct. Keep. -->
<!-- [LINE EDIT] "There are no surprise exceptions propagating through six stack frames." — "six" is arbitrary; could drop the specificity or rephrase to "no exceptions silently unwinding through the call stack". Judgment call. -->
Contrast with Java: no try/catch, no checked exceptions, no `throws` declarations. The caller always knows a function can fail because the signature says so. The downside is that error handling is verbose — you will write `if err != nil` hundreds of times. The upside is that the control flow is always explicit and local. There are no surprise exceptions propagating through six stack frames.

<!-- [LINE EDIT] "Contrast with C: returning an `int` error code loses the message and the chain." — good. -->
Contrast with C: returning an `int` error code loses the message and the chain. Go's `error` interface carries both the message and the wrapped cause, so you get readable stack-like context without stack-unwinding overhead.

<!-- [LINE EDIT] "One firm convention: **never ignore errors**." → "One firm convention applies: **never ignore errors**." Avoids the bare-noun-phrase-as-sentence; or keep the current for emphasis. Judgment call. -->
One firm convention: **never ignore errors**. The blank identifier `_` can discard a return value, but silently discarding errors is considered a serious bug in Go code review.

```go
_ = repo.Save(ctx, book) // do not do this
```

---

### Pointers

<!-- [STRUCTURAL] As noted earlier, the "no pointer arithmetic" floats at the top of the document. Move the relevant line down into this section. Also: this section is short relative to structs/interfaces/errors; fine, but consider whether escape-analysis implications (heap vs. stack) merit a sidebar here for experienced systems readers. -->
<!-- [LINE EDIT] "You know pointers from C. Go's are simpler: same concept, no arithmetic, automatic nil-safety enforcement via the runtime." — "automatic nil-safety enforcement" is a wordy noun pile. Consider: "You know pointers from C. Go's are simpler — same concept, no arithmetic, and the runtime enforces nil safety automatically." -->
You know pointers from C. Go's are simpler: same concept, no arithmetic, automatic nil-safety enforcement via the runtime.

```go
b := Book{Title: "TGPL"}
p := &b           // *Book — pointer to b
p.Title = "The Go Programming Language" // implicit dereference, same as (*p).Title
```

<!-- [COPY EDIT] "`new(T)` allocates a zeroed T and returns a `*T`." — "zeroed T" is fine. Consider "a zero-value T" for explicitness aligning with the earlier zero-values discussion. -->
`new(T)` allocates a zeroed T and returns a `*T`. Most Go code uses `&T{...}` instead.

**When to use a pointer receiver vs a value receiver:**

<!-- [COPY EDIT] "vs" — CMOS prefers "versus" or "vs." (with period) in formal prose. For technical headings, "vs" is common and acceptable. Apply consistently (also appears in 1.4). -->
- Use a **pointer receiver** when the method mutates the struct, or when the struct is large enough that copying is expensive.
- Use a **value receiver** when the method only reads, the struct is small, and value semantics make intent clear.

```go
// pointer receiver — mutates state
func (b *Book) SetYear(y int) {
    b.Year = y
}

// value receiver — read-only, small struct
func (b Book) String() string {
    return b.Title
}
```

<!-- [LINE EDIT] "Pick one consistently per type. Mixing pointer and value receivers on the same type is allowed but confusing — you will lose track of which interface a given receiver set satisfies." — 33 words, crisp. Keep. -->
Pick one consistently per type. Mixing pointer and value receivers on the same type is allowed but confusing — you will lose track of which interface a given receiver set satisfies.

---

### Exercise

<!-- [STRUCTURAL] Exercise is well-scoped and reinforces every subsection of this page (struct, Stringer interface, slice, error return, fmt.Fprintf). Good. -->
Implement the following in a file `cmd/catalog/main.go` (or a standalone `_test.go` file if you prefer):

1. Define a `Book` struct with fields: `ID string`, `Title string`, `Author string`, `Genre string`, `Year int`.

2. Implement `String() string` on `Book` so it satisfies `fmt.Stringer`. Format: `"Title by Author (Year) [Genre]"`.

3. Create a slice of at least four books spanning at least two genres.

4. Write a function `FilterByGenre(books []Book, genre string) ([]Book, error)` that:
   - Returns all books matching the given genre (case-insensitive).
   - Returns `(nil, error)` when no books match — with an error message that includes the genre name.

<!-- [LINE EDIT] "In `main` (or a test), call `FilterByGenre` for a genre that exists and one that does not." — "for a genre that exists and one that does not" is slightly terse; clearer: "for both a genre that exists and one that does not." -->
5. In `main` (or a test), call `FilterByGenre` for a genre that exists and one that does not. Print the results using `fmt.Println` (which calls `String()` automatically on `Stringer` values). Handle the error from the second call by printing it with `fmt.Fprintf(os.Stderr, ...)`.

**Reference implementation** — expand only after attempting:

<details>
<summary>Show solution</summary>

```go
package main

import (
    "errors"
    "fmt"
    "os"
    "strings"
)

type Book struct {
    ID     string
    Title  string
    Author string
    Genre  string
    Year   int
}

func (b Book) String() string {
    return fmt.Sprintf("%s by %s (%d) [%s]", b.Title, b.Author, b.Year, b.Genre)
}

var ErrNotFound = errors.New("no books found")

func FilterByGenre(books []Book, genre string) ([]Book, error) {
    var result []Book
    for _, b := range books {
        if strings.EqualFold(b.Genre, genre) {
            result = append(result, b)
        }
    }
    if len(result) == 0 {
        return nil, fmt.Errorf("%w for genre %q", ErrNotFound, genre)
    }
    return result, nil
}

func main() {
    books := []Book{
        {ID: "1", Title: "The Go Programming Language", Author: "Donovan & Kernighan", Genre: "Technology", Year: 2015},
        {ID: "2", Title: "Clean Code", Author: "Robert C. Martin", Genre: "Technology", Year: 2008},
        {ID: "3", Title: "Dune", Author: "Frank Herbert", Genre: "Science Fiction", Year: 1965},
        {ID: "4", Title: "Foundation", Author: "Isaac Asimov", Genre: "Science Fiction", Year: 1951},
    }

    techBooks, err := FilterByGenre(books, "technology")
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
    } else {
        for _, b := range techBooks {
            fmt.Println(b)
        }
    }

    _, err = FilterByGenre(books, "fantasy")
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        if errors.Is(err, ErrNotFound) {
            fmt.Fprintln(os.Stderr, "(this is expected — no fantasy books in catalog)")
        }
    }
}
```

</details>

---

### What's Next

<!-- [LINE EDIT] "Section 1.3 builds on these types by wiring them into an HTTP server — you will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses." — 40 words, at the threshold. Fine, but could be split: "Section 1.3 wires these types into an HTTP server. You will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses." -->
Section 1.3 builds on these types by wiring them into an HTTP server — you will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses.

---

### References

<!-- [COPY EDIT] Footnote 3 title — "Working with Errors in Go 1.13" capitalized as a title; verify matches Go blog article title exactly. (It does.) -->
[^1]: The Go Authors. *A Tour of Go*. <https://go.dev/tour/>

[^2]: The Go Authors. *Effective Go*. <https://go.dev/doc/effective_go>

[^3]: Damien Neil and Jonathan Amsterdam. *Working with Errors in Go 1.13*. Go Blog, 2019. <https://go.dev/blog/go1.13-errors>
