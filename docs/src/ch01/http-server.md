## 1.3 Building an HTTP Server

Go ships with a production-capable HTTP server in its standard library.[^1] No framework required, no servlet container to configure, no application server to deploy to—`net/http` handles all of it. This section covers the core abstractions, builds the gateway's health and books endpoints line by line, and shows you how the server is wired together in `main`.

---

### The `http.Handler` Interface

Everything in Go's HTTP stack revolves around one interface:

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

If a type implements `ServeHTTP`, it can handle HTTP requests. This is the Go equivalent of implementing `javax.servlet.Servlet`—but with far less ceremony.

`http.ResponseWriter` is an interface for writing the response: status code, headers, and body. `*http.Request` is a struct carrying everything about the incoming request: method, URL, headers, body, and more.

#### HandlerFunc: Functions as Handlers

Most of the time you do not want to define a whole type just to handle one route. Go provides `http.HandlerFunc`, a function type that implements `Handler`:

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

This means any function with the signature `func(http.ResponseWriter, *http.Request)` can be used as a handler directly. If you have used Kotlin's SAM interfaces or Java's `@FunctionalInterface`, you have seen this pattern. The type adapter lets the compiler treat a plain function as an object satisfying an interface.

#### ServeMux: The Router

`http.ServeMux` is Go's built-in request multiplexer (router). It matches incoming request paths to registered handlers:

```go
mux := http.NewServeMux()
mux.HandleFunc("/healthz", myHandlerFunc)
mux.HandleFunc("/books", anotherHandlerFunc)
```

Prior to Go 1.22, `ServeMux` only matched on path prefix or exact path. **Go 1.22 added method-aware routing and path parameters**, making patterns like `GET /books/{id}` valid without a third-party router.[^2]

---

### Writing Handlers

A handler reads from `*http.Request` and writes to `http.ResponseWriter`. Here is the sequence for almost every handler you will write:

1. Validate the request (method, path parameters, body).
2. Set response headers (`w.Header().Set(...)`).
3. Write the status code (`w.WriteHeader(statusCode)`).
4. Write the body (`w.Write(...)` or via an encoder).

One important rule: **`WriteHeader` must be called before any call to `Write`**. Once you call `Write`, Go automatically sends `200 OK` if `WriteHeader` has not already been called; calling it afterward is a no-op that logs a warning.

Reading from the request:

```go
r.Method          // "GET", "POST", etc.
r.URL.Path        // "/books"
r.Header.Get("Authorization")
r.PathValue("id") // Go 1.22+: value of {id} in the route pattern
```

---

### JSON Responses

The `encoding/json` package handles serialization.[^3] The idiomatic way to write a JSON response uses `json.NewEncoder`:

```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(value)
```

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

Struct tags live inside Go's raw-string literal (backticks), which is why backslashes in them are literal rather than escaped. The `json:"id"` tag tells the encoder to use `"id"` as the JSON key instead of `"ID"`. Without the tag, Go uses the exact field name—so `ID` would serialize as `"ID"`, and `Title` as `"Title"`. Tags are the Go equivalent of Jackson's `@JsonProperty` annotation in Java, but they live inline on the struct field rather than as a separate annotation.

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

**Line by line:**

- `func Health(w http.ResponseWriter, r *http.Request)`—matches the `HandlerFunc` signature, so this plain function can be passed directly to `mux.HandleFunc`.
- `if r.Method != http.MethodGet`—method guard. `http.MethodGet` is the constant `"GET"`. Returning `405 Method Not Allowed` for unsupported methods is correct HTTP semantics. Notice the early return; there is no `else` branch.
- `w.WriteHeader(http.StatusMethodNotAllowed)`—sends the 405 status. `http.StatusMethodNotAllowed` is the constant `405`.
- `w.Header().Set("Content-Type", "application/json")`—sets the `Content-Type` header. This must be called before `WriteHeader` or `Write`—once the header is flushed, you cannot change it.
- `map[string]string{"status": "ok"}`—a map literal. Go infers the type from the declaration; you do not need `new` or a constructor.
- `json.NewEncoder(w).Encode(response)`—serializes the map to `{"status":"ok"}\n` and writes it directly into the response body.

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

`sampleBooks` is a package-level variable—a slice of `Book` structs initialized with composite literals. The `var` keyword at package scope is the Go equivalent of a static field. This is in-memory stub data; later chapters replace it with real database queries.

`json.NewEncoder(w).Encode(sampleBooks)` serializes the entire slice as a JSON array. The encoder handles slices, structs, maps, and primitives out of the box.

---

### Wiring It Together: `main.go`

The `main.go` entry point from Section 1.1 wires the handlers to the router. The key additions are the route registrations:

```go
mux.HandleFunc("/healthz", handler.Health)
mux.HandleFunc("/books", handler.Books)
```

See Section 1.1 for the full `main.go` listing, including environment configuration and server startup.

#### Environment Configuration

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

`os.Getenv` returns an empty string when the variable is not set—never an error, never a panic. The explicit fallback to `"8080"` is a common Go idiom for optional environment configuration. You will see this pattern throughout the project for every configurable value (database URL, Kafka brokers, gRPC addresses). It makes the service runnable locally without any environment setup while remaining configurable in containers.

#### Starting the Server

```go
if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatalf("server failed: %v", err)
}
```

`http.ListenAndServe` starts a TCP listener and begins accepting connections. It only returns if an error occurs (e.g., the port is already in use). `log.Fatalf` logs the message and calls `os.Exit(1)`. The `:=` in the `if` initializer is idiomatic Go—it declares `err` scoped to the `if` block and checks it in one line. (Java programmers will recognize the scoping idea from try-with-resources, though the purpose is different.)

You can run the server directly:

```bash
go run ./services/gateway/cmd/main.go
# In another terminal:
curl http://localhost:8080/healthz
curl http://localhost:8080/books
```

---

### Exercise

Add a `GET /books/{id}` endpoint that returns a single book by ID from `sampleBooks`, or `404 Not Found` if no book with that ID exists.

**Requirements:**
- Register the route using Go 1.22's method-aware pattern: `mux.HandleFunc("GET /books/{id}", handler.BookByID)`
- In the handler, use `r.PathValue("id")` to extract the ID from the URL.
- Loop through `sampleBooks` to find a match. If found, encode and return it. If not found, return `http.StatusNotFound`.
- Return `Content-Type: application/json` for both the found and not-found cases. For 404, return a JSON body like `{"error": "not found"}`.

**Hint:** Since the route pattern `GET /books/{id}` already constrains the method, you do not need a manual method guard in this handler.

Test it:

```bash
curl http://localhost:8080/books/1    # should return the first book
curl http://localhost:8080/books/99   # should return 404
```

---

[^1]: [net/http package documentation](https://pkg.go.dev/net/http)
[^2]: [Go 1.22: enhanced routing patterns](https://go.dev/blog/routing-enhancements)
[^3]: [encoding/json package documentation](https://pkg.go.dev/encoding/json)
