## 1.2 Go Language Essentials

This section moves fast. You already know what a struct is, what an interface is, and why error handling matters. The goal here is to show you how Go's versions of these things differ from what you already know — and where those differences bite you if you come in with C++/Java assumptions.

---

### Types and Zero Values

Go's primitive types will feel familiar: `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `float32`, `float64`, `bool`, `string`, `byte` (alias for `uint8`), `rune` (alias for `int32`).

Two things to internalize immediately:

**No implicit conversions.** This compiles in C, not in Go:

```go
var x int32 = 10
var y int64 = x // compile error: cannot use x (type int32) as type int64
```

You must be explicit:

```go
var y int64 = int64(x)
```

This is intentional. Go treats implicit numeric promotion as a source of bugs and refuses to do it.

**Every type has a zero value.** Variables declared but not initialized are not garbage — they have deterministic defaults:

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

This eliminates an entire class of uninitialized-variable bugs. You will miss it when you go back to C.

**No pointer arithmetic.** `p++` on a pointer is a compile error. Go is not C — the runtime manages memory, and the GC needs pointer provenance intact.

---

### Structs

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

**Embedding instead of inheritance.** Go has no `extends`. Go achieves composition by embedding one struct inside another:

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

This is not inheritance. The embedded type has no knowledge of the outer type. There is no `super`. If `BookRecord` needs different behavior, it defines its own method with the same name — the outer type's method takes precedence, and the inner one is still accessible via `record.Book.String()`.

---

### Interfaces

In Java/Kotlin, you write `implements Serializable`. In C++ you inherit from a pure-virtual base class. In Go, interface satisfaction is **implicit**: if your type has the methods, it satisfies the interface — no declaration required.

```go
// Standard library interface — defined in package fmt
type Stringer interface {
    String() string
}
```

`Book` above already satisfies `fmt.Stringer` because it has a `String() string` method. There is no annotation, no `implements`, nothing to register. The compiler verifies satisfaction where the interface is used.

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

Any type with those three methods satisfies `Repository`. The concrete type — a PostgreSQL implementation, a Redis cache, an in-memory fake — never needs to reference this interface.

**Why this matters for testing.** Because interfaces are satisfied implicitly, you can define a minimal interface in your test package and point it at a fake implementation without touching production code at all. You will see this pattern constantly in this project.

**The empty interface.** `any` (alias for `interface{}`) accepts any value:

```go
var v any = 42
v = "now a string"
v = Book{...}
```

Use it sparingly. It bypasses the type system. You will see it in generic utility functions and JSON unmarshaling. When you need to get the concrete type back, use a type assertion or type switch:

```go
if book, ok := v.(Book); ok {
    fmt.Println(book.Title)
}
```

---

### Slices and Maps

**Slices** are Go's equivalent of `std::vector` or `ArrayList` — a dynamically sized view over an underlying array. Unlike C arrays, slices carry their length and capacity.

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

A nil slice and an empty slice behave the same for `len`, `append`, and `range` — but they are not equal under `reflect.DeepEqual`. The convention is to return a nil slice for "no results" rather than an empty one, and to let callers use `len(result) == 0` to check emptiness.

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

Unlike C++, range gives you a **copy** of each element. Mutating `book` inside the loop does not affect the slice. Use the index if you need to modify in place: `books[i].Year = 2024`.

**Slicing a slice** produces a new header over the same backing array — not a copy:

```go
first3 := books[0:3] // shares memory with books
```

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

Map iteration order is deliberately randomized on each run — do not rely on it.

---

### Pointers

You know pointers from C. Go pointers are simpler — same concept, no arithmetic, and the runtime enforces nil safety automatically.

```go
b := Book{Title: "TGPL"}
p := &b           // *Book — pointer to b
p.Title = "The Go Programming Language" // implicit dereference, same as (*p).Title
```

`new(T)` allocates a zero-value T and returns a `*T`. Most Go code uses `&T{...}` instead.

**When to use a pointer receiver vs. a value receiver:**

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

Pick one consistently per type. Mixing pointer and value receivers on the same type is allowed but confusing — you will lose track of which interfaces the type satisfies.

---

### Error Handling

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

Contrast with Java: No try/catch, no checked exceptions, no `throws` declarations. The caller always knows a function can fail because the signature says so. The downside is that error handling is verbose — you will write `if err != nil` hundreds of times. The upside is that the control flow is always explicit and local. There are no surprise exceptions propagating through six stack frames.

Contrast with C: Returning an `int` error code loses the message and the chain. Go's `error` interface carries both the message and the wrapped cause, so you get readable stack-like context without stack-unwinding overhead.

One firm convention: **never ignore errors**. The blank identifier `_` can discard a return value, but silently discarding errors is considered a serious bug in Go code review.

```go
_ = repo.Save(ctx, book) // do not do this
```

---

### Exercise

Implement the following in a file `cmd/catalog/main.go` (or a standalone `_test.go` file if you prefer):

1. Define a `Book` struct with fields: `ID string`, `Title string`, `Author string`, `Genre string`, `Year int`.

2. Implement `String() string` on `Book` so it satisfies `fmt.Stringer`. Format: `"Title by Author (Year) [Genre]"`.

3. Create a slice of at least four books spanning at least two genres.

4. Write a function `FilterByGenre(books []Book, genre string) ([]Book, error)` that:
   - Returns all books matching the given genre (case-insensitive).
   - Returns `(nil, error)` when no books match — with an error message that includes the genre name.

5. In `main` (or a test), call `FilterByGenre` for both a genre that exists and one that does not. Print the results using `fmt.Println` (which calls `String()` automatically on `Stringer` values). Handle the error from the second call by printing it with `fmt.Fprintf(os.Stderr, ...)`.

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

Section 1.3 wires these types into an HTTP server. You will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses.

---

### References

[^1]: The Go Authors. *A Tour of Go*. <https://go.dev/tour/>

[^2]: The Go Authors. *Effective Go*. <https://go.dev/doc/effective_go>

[^3]: Damien Neil and Jonathan Amsterdam. *Working with Errors in Go 1.13*. Go Blog, 2019. <https://go.dev/blog/go1.13-errors>
