<!-- [STRUCTURAL] Same heading-level issue as go-basics.md — opens at H2. Make chapter-wide decision on file-level heading hierarchy. -->
## 1.3 Building an HTTP Server

<!-- [STRUCTURAL] Strong opening. The three negatives ("No framework required, no servlet container to configure, no application server to deploy into") are a good frame for the JVM reader. -->
<!-- [LINE EDIT] "Go ships with a production-capable HTTP server in its standard library." — "production-capable" (CMOS 7.81) correctly hyphenated before a noun. But here it functions predicatively after "ships with a" — the compound is still pre-nominal to "HTTP server", so the hyphen is correct. Keep. -->
<!-- [LINE EDIT] "No framework required, no servlet container to configure, no application server to deploy into — `net/http` handles all of it." — slightly clunky "to deploy into"; consider "no application server to deploy to". Minor. -->
Go ships with a production-capable HTTP server in its standard library. No framework required, no servlet container to configure, no application server to deploy into — `net/http` handles all of it. This section covers the core abstractions, builds the gateway's health and books endpoints line by line, and shows you how the server is wired together in `main`.

---

### The `http.Handler` Interface

Everything in Go's HTTP stack revolves around one interface:

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

<!-- [LINE EDIT] "If a type implements `ServeHTTP`, it can handle HTTP requests." — direct. -->
<!-- [COPY EDIT] "`javax.servlet.Servlet`" — Jakarta EE renamed the package to `jakarta.servlet.Servlet` in 2019. For currency, consider "`javax.servlet.Servlet` (now `jakarta.servlet.Servlet`)". Minor pedantry. -->
If a type implements `ServeHTTP`, it can handle HTTP requests. This is the Go equivalent of implementing `javax.servlet.Servlet` — but with far less ceremony.

<!-- [LINE EDIT] "`http.ResponseWriter` is an interface for writing the response: status code, headers, and body. `*http.Request` is a struct carrying everything about the incoming request: method, URL, headers, body, and more." — parallel structure is effective. Keep. -->
`http.ResponseWriter` is an interface for writing the response: status code, headers, and body. `*http.Request` is a struct carrying everything about the incoming request: method, URL, headers, body, and more.

#### HandlerFunc: Functions as Handlers

<!-- [LINE EDIT] "Most of the time you do not want to define a whole type just to handle one route." — could be: "Defining a whole type for a single route is overkill." More active. -->
Most of the time you do not want to define a whole type just to handle one route. Go provides `http.HandlerFunc`, a function type that implements `Handler`:

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

<!-- [LINE EDIT] "This means any function with the signature `func(http.ResponseWriter, *http.Request)` can be used as a handler directly." — "can be used as" → "can serve as". -->
<!-- [LINE EDIT] "You have seen this pattern before if you have used Kotlin's SAM interfaces or Java's `@FunctionalInterface` — it is the same idea." — the 38-word composite is fine. -->
This means any function with the signature `func(http.ResponseWriter, *http.Request)` can be used as a handler directly. You have seen this pattern before if you have used Kotlin's SAM interfaces or Java's `@FunctionalInterface` — it is the same idea. The type adapter lets the compiler treat a plain function as an object satisfying an interface.

#### ServeMux: The Router

`http.ServeMux` is Go's built-in request multiplexer (router). It matches incoming request paths to registered handlers:

```go
mux := http.NewServeMux()
mux.HandleFunc("/healthz", myHandlerFunc)
mux.HandleFunc("/books", anotherHandlerFunc)
```

<!-- [COPY EDIT] "Prior to Go 1.22, `ServeMux` only matched on path prefix or exact path." — factually correct. -->
<!-- [COPY EDIT] "**Go 1.22 added method-aware routing and path parameters**, making patterns like `GET /books/{id}` valid without a third-party router.[^2]" — CMOS: emphasis bolding is fine here. Confirm footnote target resolves (Go 1.22 routing enhancements blog). -->
Prior to Go 1.22, `ServeMux` only matched on path prefix or exact path. **Go 1.22 added method-aware routing and path parameters**, making patterns like `GET /books/{id}` valid without a third-party router.[^2]

---

### Writing Handlers

<!-- [STRUCTURAL] Good teaching move — the 4-step handler pattern prepares the reader for every code block that follows. -->
A handler reads from `*http.Request` and writes to `http.ResponseWriter`. Here is the sequence for almost every handler you will write:

1. Validate the request (method, path parameters, body).
2. Set response headers (`w.Header().Set(...)`).
3. Write the status code (`w.WriteHeader(statusCode)`).
4. Write the body (`w.Write(...)` or via an encoder).

<!-- [LINE EDIT] "One important rule: **`WriteHeader` must be called before any call to `Write`**." — fine. -->
<!-- [LINE EDIT] "Once you call `Write`, Go automatically sends a `200 OK` if you have not called `WriteHeader` yet. Calling `WriteHeader` after `Write` has no effect and produces a warning in the logs." — the two sentences can be joined for rhythm: "Once you call `Write`, Go automatically sends `200 OK` if `WriteHeader` has not already been called; calling it afterwards is a no-op that logs a warning." -->
One important rule: **`WriteHeader` must be called before any call to `Write`**. Once you call `Write`, Go automatically sends a `200 OK` if you have not called `WriteHeader` yet. Calling `WriteHeader` after `Write` has no effect and produces a warning in the logs.

Reading from the request:

```go
r.Method          // "GET", "POST", etc.
r.URL.Path        // "/books"
r.Header.Get("Authorization")
r.PathValue("id") // Go 1.22+: value of {id} in the route pattern
```

---

### JSON Responses

<!-- [STRUCTURAL] Solid explanation. The "avoids allocating an intermediate byte slice" point is a nice systems-level detail for the target reader. -->
<!-- [LINE EDIT] "The `encoding/json` package handles serialization.[^3] The idiomatic way to write a JSON response uses `json.NewEncoder`:" — separate sentences. Fine. -->
The `encoding/json` package handles serialization.[^3] The idiomatic way to write a JSON response uses `json.NewEncoder`:

```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(value)
```

<!-- [COPY EDIT] The code snippet drops the return value of `Encode`. In production code this is a lint finding (errcheck). The text below discusses the idiom but doesn't address error handling. Consider adding one sentence: "We're ignoring the error here for brevity — production handlers should check it, which we'll revisit once structured logging lands in Chapter 4." -->
<!-- [LINE EDIT] "Calling `.Encode(value)` serializes `value` and writes it, appending a newline." — keep. -->
<!-- [LINE EDIT] "This is more efficient than `json.Marshal` + `w.Write` because it avoids allocating an intermediate byte slice." — concise; keep. -->
`json.NewEncoder(w)` creates an encoder that writes directly into the `ResponseWriter` (which implements `io.Writer`). Calling `.Encode(value)` serializes `value` and writes it, appending a newline. This is more efficient than `json.Marshal` + `w.Write` because it avoids allocating an intermediate byte slice.

#### Struct Tags

Go controls JSON field names through struct tags:

```go
type Book struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Author string `json:"author"`
}
```

<!-- [LINE EDIT] "The backtick syntax is Go's raw string literal for struct tags." — minor: backticks are the raw string literal, not specifically "for struct tags" — they're a general string literal whose syntax happens to suit struct tags. Consider: "Struct tags live inside Go's raw-string literal (backticks), which is why backslashes in them are literal rather than escaped." -->
<!-- [COPY EDIT] Confirm term: "backtick" is a common informal name for U+0060 (grave accent). CMOS accepts either; keep. -->
The backtick syntax is Go's raw string literal for struct tags. The `json:"id"` tag tells the encoder to use `"id"` as the JSON key instead of `"ID"`. Without the tag, Go uses the exact field name — so `ID` would serialize as `"ID"`, and `Title` as `"Title"`. Tags are the Go equivalent of Jackson's `@JsonProperty` annotation in Java, but they live inline on the struct field rather than as a separate annotation.

<!-- [LINE EDIT] "Additional options: `json:"name,omitempty"` skips the field when it holds its zero value. `json:"-"` excludes the field from serialization entirely." — keep; the parallel syntax is effective. -->
Additional options: `json:"name,omitempty"` skips the field when it holds its zero value. `json:"-"` excludes the field from serialization entirely.

---

### Walkthrough: Health Handler

File: `services/gateway/internal/handler/health.go`

```go
package handler

import (
    "encoding/json"
    "net/http"
)

func Health(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    response := map[string]string{"status": "ok"}
    json.NewEncoder(w).Encode(response)
}
```

<!-- [STRUCTURAL] Line-by-line annotations are a strong tutorial device and reinforce the 4-step handler pattern. Keep. -->
<!-- [LINE EDIT] "matches the `HandlerFunc` signature, so this plain function can be passed directly to `mux.HandleFunc`" — good. -->
**Line by line:**

- `func Health(w http.ResponseWriter, r *http.Request)` — matches the `HandlerFunc` signature, so this plain function can be passed directly to `mux.HandleFunc`.
<!-- [LINE EDIT] "Returning `405 Method Not Allowed` for unsupported methods is correct HTTP semantics." — correct; RFC 9110. -->
<!-- [COPY EDIT] "Notice the early return; there is no `else` branch." — semicolon connects two independent clauses; correct CMOS usage. -->
- `if r.Method != http.MethodGet` — method guard. `http.MethodGet` is the constant `"GET"`. Returning `405 Method Not Allowed` for unsupported methods is correct HTTP semantics. Notice the early return; there is no `else` branch.
<!-- [COPY EDIT] "the constant `405`" — Go's `http.StatusMethodNotAllowed` is an `int` constant with value 405; phrasing is acceptable. Consider "the integer constant 405" for precision. -->
- `w.WriteHeader(http.StatusMethodNotAllowed)` — sends the 405 status. `http.StatusMethodNotAllowed` is the constant `405`.
<!-- [LINE EDIT] "This must be called before `WriteHeader` or `Write` — once the header is flushed, you cannot change it." — clean and useful. -->
- `w.Header().Set("Content-Type", "application/json")` — sets the `Content-Type` header. This must be called before `WriteHeader` or `Write` — once the header is flushed, you cannot change it.
- `map[string]string{"status": "ok"}` — a map literal. Go infers the type from the declaration; you do not need `new` or a constructor.
<!-- [COPY EDIT] The annotation claims `Encode` writes `{"status":"ok"}\n`; verify that `json.NewEncoder`'s default output has no space after colon. (Correct by default — `Encode` produces compact output unless `SetIndent` is called.) -->
- `json.NewEncoder(w).Encode(response)` — serializes the map to `{"status":"ok"}\n` and writes it directly into the response body.

---

### Walkthrough: Books Handler

File: `services/gateway/internal/handler/books.go`

```go
package handler

import (
    "encoding/json"
    "net/http"
)

type Book struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Author string `json:"author"`
    Genre  string `json:"genre"`
    Year   int    `json:"year"`
}

var sampleBooks = []Book{
    {ID: "1", Title: "The Go Programming Language", Author: "Alan Donovan & Brian Kernighan", Genre: "Programming", Year: 2015},
    {ID: "2", Title: "Designing Data-Intensive Applications", Author: "Martin Kleppmann", Genre: "Distributed Systems", Year: 2017},
    {ID: "3", Title: "Building Microservices", Author: "Sam Newman", Genre: "Architecture", Year: 2021},
}

func Books(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(sampleBooks)
}
```

<!-- [STRUCTURAL] Continuity note: go-basics.md defines `Book` with an ID like "978-0-13-468599-1" (ISBN); here the IDs are just "1", "2", "3". Fine for a stub, but flag the inconsistency for anyone who compares chapters. -->
<!-- [COPY EDIT] Please verify: "Building Microservices" by Sam Newman — 2nd edition was published 2021; if the book referenced is the 2nd ed. the date is correct. First edition was 2015. Not a correctness issue, just flagging. -->
<!-- [LINE EDIT] "`sampleBooks` is a package-level variable — a slice of `Book` structs initialized with composite literals. The `var` keyword at package scope is the Go equivalent of a static field. This is in-memory stub data; later chapters replace it with real database queries." — the third sentence sets expectation well. Keep. -->
`sampleBooks` is a package-level variable — a slice of `Book` structs initialized with composite literals. The `var` keyword at package scope is the Go equivalent of a static field. This is in-memory stub data; later chapters replace it with real database queries.

<!-- [LINE EDIT] "The encoder handles slices, structs, maps, and primitives — no configuration needed." — "no configuration needed" → "with no configuration needed" or drop to "The encoder handles slices, structs, maps, and primitives out of the box." -->
`json.NewEncoder(w).Encode(sampleBooks)` serializes the entire slice as a JSON array. The encoder handles slices, structs, maps, and primitives — no configuration needed.

---

### Wiring It Together: `main.go`

<!-- [STRUCTURAL] This duplicates the main.go snippet already shown in project-setup.md (§1.1). It is fair to re-show it here (this is the section that teaches the lines), but a short cross-reference sentence — "You saw this in §1.1 as layout; here we walk through what each line does." — would acknowledge the duplication gracefully. -->
File: `services/gateway/cmd/main.go`

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", handler.Health)
    mux.HandleFunc("/books", handler.Books)
    addr := fmt.Sprintf(":%s", port)
    log.Printf("gateway listening on %s", addr)
    if err := http.ListenAndServe(addr, mux); err != nil {
        log.Fatalf("server failed: %v", err)
    }
}
```

<!-- [COPY EDIT] Compared with the snippet in project-setup.md, the import block here is different: project-setup.md groups stdlib imports and puts the handler import after a blank line, per gofmt convention. Here the handler import is in the same group. Run `gofmt`/`goimports` and align. This is a subtle but real inconsistency. -->

#### Environment Configuration

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

<!-- [LINE EDIT] "`os.Getenv` returns an empty string when the variable is not set — never an error, never a panic." — good. -->
<!-- [LINE EDIT] "The explicit fallback to `"8080"` is the standard Go idiom for optional environment configuration." — "standard Go idiom" is one of a few common idioms; softening to "a common Go idiom" is more honest. -->
`os.Getenv` returns an empty string when the variable is not set — never an error, never a panic. The explicit fallback to `"8080"` is the standard Go idiom for optional environment configuration. You will see this pattern throughout the project for every configurable value (database URL, Kafka brokers, gRPC addresses). It makes the service runnable locally without any environment setup while remaining configurable in containers.

<!-- [STRUCTURAL] You introduce "gRPC addresses" casually here without having defined gRPC. Prior chapters don't exist before this, and introduction.md only mentions gRPC in passing. For a first mention, consider "gRPC service endpoints (covered in Chapter 3)" or similar. -->

#### Starting the Server

```go
if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatalf("server failed: %v", err)
}
```

<!-- [LINE EDIT] "`http.ListenAndServe` starts a TCP listener and begins accepting connections. It only returns if an error occurs (e.g., the port is already in use)." — good. -->
<!-- [COPY EDIT] "e.g., the port is already in use" — CMOS 6.43 "e.g.," with comma. Correct. -->
<!-- [LINE EDIT] "The `:=` in the `if` initializer is idiomatic Go — it declares `err` scoped to the `if` block and checks it in one line, equivalent to Java's try-with-resources pattern for the variable scoping benefit." — the Java analogy is a stretch; try-with-resources is about automatic resource closing, not scoped declaration. Consider: "The `:=` in the `if` initializer is idiomatic Go — it declares `err` scoped to the `if` block and checks it in one line. (Java programmers will recognise the scoping idea from try-with-resources, though the purpose is different.)" Or remove the Java analogy. -->
`http.ListenAndServe` starts a TCP listener and begins accepting connections. It only returns if an error occurs (e.g., the port is already in use). `log.Fatalf` logs the message and calls `os.Exit(1)`. The `:=` in the `if` initializer is idiomatic Go — it declares `err` scoped to the `if` block and checks it in one line, equivalent to Java's try-with-resources pattern for the variable scoping benefit.

You can run the server directly:

```bash
go run ./services/gateway/cmd/main.go
# In another terminal:
curl http://localhost:8080/healthz
curl http://localhost:8080/books
```

<!-- [COPY EDIT] "`go run ./services/gateway/cmd/main.go`" — passing a single `.go` file is fine; passing the package path (`./services/gateway/cmd`) is more canonical and handles multi-file `package main` cleanly. Consider `go run ./services/gateway/cmd`. -->

---

### Exercise

<!-- [STRUCTURAL] Exercise is well-scoped and tied to §1.4 (testing it). Good forward reference. -->
Add a `GET /books/{id}` endpoint that returns a single book by ID from `sampleBooks`, or `404 Not Found` if no book with that ID exists.

**Requirements:**
<!-- [LINE EDIT] "Register the route using Go 1.22's method-aware pattern" — good. -->
- Register the route using Go 1.22's method-aware pattern: `mux.HandleFunc("GET /books/{id}", handler.BookByID)`
- In the handler, use `r.PathValue("id")` to extract the ID from the URL.
- Loop through `sampleBooks` to find a match. If found, encode and return it. If not found, return `http.StatusNotFound`.
<!-- [LINE EDIT] "Return `Content-Type: application/json` for both the found and not-found cases." — "not-found" hyphenated as compound modifier, correct CMOS 7.81. -->
- Return `Content-Type: application/json` for both the found and not-found cases. For 404, return a JSON body like `{"error": "not found"}`.

<!-- [LINE EDIT] "Since the route pattern `GET /books/{id}` already constrains the method, you do not need a manual method guard in this handler." — good. -->
**Hint:** Since the route pattern `GET /books/{id}` already constrains the method, you do not need a manual method guard in this handler.

Test it:

```bash
curl http://localhost:8080/books/1    # should return the first book
curl http://localhost:8080/books/99   # should return 404
```

---

<!-- [COPY EDIT] Footnotes at the bottom differ from project-setup.md style (no descriptor text, just link). Align footnote style across the chapter. -->
[^1]: [net/http package documentation](https://pkg.go.dev/net/http)
[^2]: [Go 1.22: enhanced routing patterns](https://go.dev/blog/routing-enhancements)
[^3]: [encoding/json package documentation](https://pkg.go.dev/encoding/json)
