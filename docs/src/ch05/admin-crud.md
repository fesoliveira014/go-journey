# 5.4 Admin CRUD

The library system needs an admin interface for managing books—creating, editing, and deleting entries in the catalog. This section covers the patterns used in the admin handlers: role-based access control, form handling, gRPC error mapping, and the Docker build.

---

## Role-Based Access: The `requireAdmin` Helper

Every admin handler verifies two things: the user is authenticated, and the user has the `"admin"` role. Rather than repeating this check in every handler, we extract it into a helper method:

```go
// services/gateway/internal/handler/catalog.go

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
    u := userFromContext(r.Context())
    if u == nil {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return false
    }
    if u.Role != "admin" {
        s.renderError(w, r, http.StatusForbidden, "Access denied")
        return false
    }
    return true
}
```

The pattern is: call `requireAdmin` at the top of the handler, and return immediately if it returns `false`. The helper has already written the response (either a redirect to login or a 403 error page).

```go
func (s *Server) AdminBookNew(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) {
        return
    }
    s.render(w, r, "admin_book_new.html", map[string]any{})
}
```

This is a simpler approach than Spring Security's `@PreAuthorize("hasRole('ADMIN')")` annotation, but it achieves the same result. The trade-off is that you must remember to call `requireAdmin` in every admin handler—there is no framework enforcing it at the routing level. For a small codebase, this explicitness is a feature. For a large one, you might extract it into middleware that applies to an entire route prefix.

---

## Form Handling in Go

HTML forms submit data as URL-encoded key-value pairs. Go's `net/http` package provides two ways to access form data:

- **`r.FormValue("key")`**—Returns a single value. Calls `r.ParseForm()` implicitly on the first call.
- **`r.ParseForm()`**—Parses the request body (for POST) and URL query parameters. After calling this, you can access `r.Form` (a `map[string][]string`) directly.

For the admin book creation handler, we need to convert string form values to integers using `strconv.Atoi`:

```go
// services/gateway/internal/handler/catalog.go

func (s *Server) AdminBookCreate(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) {
        return
    }

    if err := r.ParseForm(); err != nil {
        s.renderError(w, r, http.StatusBadRequest, "Invalid form data")
        return
    }

    title := r.FormValue("title")
    author := r.FormValue("author")
    isbn := r.FormValue("isbn")
    genre := r.FormValue("genre")
    description := r.FormValue("description")
    publishedYearStr := r.FormValue("published_year")
    totalCopiesStr := r.FormValue("total_copies")

    if title == "" || author == "" || isbn == "" || genre == "" ||
        publishedYearStr == "" || totalCopiesStr == "" {
        s.render(w, r, "admin_book_new.html", map[string]any{
            "Error": "Title, author, ISBN, genre, published year, and total copies are required",
        })
        return
    }

    publishedYear, err := strconv.Atoi(publishedYearStr)
    if err != nil {
        s.render(w, r, "admin_book_new.html", map[string]any{
            "Error": "Published year must be a number",
        })
        return
    }

    totalCopies, err := strconv.Atoi(totalCopiesStr)
    if err != nil {
        s.render(w, r, "admin_book_new.html", map[string]any{
            "Error": "Total copies must be a number",
        })
        return
    }

    _, err = s.catalog.CreateBook(r.Context(), &catalogv1.CreateBookRequest{
        Title:         title,
        Author:        author,
        Isbn:          isbn,
        Genre:         genre,
        Description:   description,
        PublishedYear: int32(publishedYear),
        TotalCopies:   int32(totalCopies),
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to create book")
        return
    }

    setFlash(w, "Book created")
    http.Redirect(w, r, "/books", http.StatusSeeOther)
}
```

In Spring, `@ModelAttribute` or `@RequestBody` would handle this binding and conversion automatically. In Go, the mapping from form values to typed fields is manual. This is more verbose but leaves no ambiguity about what happens when a field is missing or cannot be parsed.

The update handler follows the same pattern but includes the book ID from the path:

```go
id := r.PathValue("id")
// ... parse form values ...
_, err = s.catalog.UpdateBook(r.Context(), &catalogv1.UpdateBookRequest{
    Id:            id,
    Title:         title,
    // ... remaining fields ...
})
```

After a successful create or update, the handler sets a flash message and redirects—the PRG pattern from section 5.3.

---

## gRPC Error Mapping

When a gRPC call fails, the error contains a status code and message. The `handleGRPCError` function translates gRPC status codes into appropriate HTTP responses:

```go
// services/gateway/internal/handler/render.go

func (s *Server) handleGRPCError(w http.ResponseWriter, r *http.Request, err error, fallbackMsg string) {
    st, ok := status.FromError(err)
    if !ok {
        s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
        return
    }
    switch st.Code() {
    case codes.NotFound:
        s.renderError(w, r, http.StatusNotFound, "Not found")
    case codes.InvalidArgument:
        s.renderError(w, r, http.StatusBadRequest, st.Message())
    case codes.AlreadyExists:
        s.renderError(w, r, http.StatusConflict, st.Message())
    case codes.Unauthenticated:
        http.Redirect(w, r, "/login", http.StatusSeeOther)
    case codes.PermissionDenied:
        s.renderError(w, r, http.StatusForbidden, "Access denied")
    case codes.ResourceExhausted:
        s.renderError(w, r, http.StatusTooManyRequests, "You have reached the maximum number of active reservations")
    case codes.FailedPrecondition:
        s.renderError(w, r, http.StatusPreconditionFailed, st.Message())
    default:
        s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
    }
}
```

The mapping follows standard conventions:

| gRPC code | HTTP status | Response |
|---|---|---|
| `NotFound` | 404 | Error page |
| `InvalidArgument` | 400 | Error page with gRPC message |
| `AlreadyExists` | 409 | Error page with gRPC message |
| `Unauthenticated` | 303 redirect | Redirect to login |
| `PermissionDenied` | 403 | Error page |
| `ResourceExhausted` | 429 | Reservation limit message |
| `FailedPrecondition` | 412 | Error page with gRPC message |
| Everything else | 500 | Error page with fallback message |

Notice that `InvalidArgument`, `AlreadyExists`, and `FailedPrecondition` pass through the gRPC message (`st.Message()`) because it contains useful context (e.g., "ISBN already exists"). For `NotFound` and `PermissionDenied`, we use generic messages to avoid leaking internal details. `ResourceExhausted` uses a fixed, user-friendly message for the reservation limit.

---

## Docker Build

The gateway Dockerfile follows the same multi-stage pattern from Chapter 3, with one addition: It copies the `templates/` and `static/` directories into the runtime image.

```dockerfile
# services/gateway/Dockerfile

# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV GOWORK=off

COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY pkg/otel/go.mod pkg/otel/go.sum* ./pkg/otel/
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/
WORKDIR /app/services/gateway
RUN go mod download

WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY pkg/otel/ ./pkg/otel/
COPY services/gateway/ ./services/gateway/
WORKDIR /app/services/gateway
RUN CGO_ENABLED=0 go build -o /bin/gateway ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/gateway /usr/local/bin/gateway
COPY --from=builder /app/services/gateway/templates/ /app/templates/
COPY --from=builder /app/services/gateway/static/ /app/static/
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
```

The key difference from the Auth and Catalog Dockerfiles: the runtime image includes `templates/` and `static/` because the gateway reads these at startup. The Go binary handles compiled code, but templates and CSS are loaded from the filesystem. The `WORKDIR /app` ensures the binary can find `templates/` and `static/` at the relative paths used in `main.go`.

---

## Testing Strategy

The gateway's handlers are straightforward to test because their dependencies are interfaces. The gRPC client types (`authv1.AuthServiceClient`, `catalogv1.CatalogServiceClient`) are interfaces generated by protoc—you can substitute mock implementations in tests without any mocking framework.

The standard approach:

1. Create a mock that implements the gRPC client interface, returning canned responses.
2. Construct a `Server` with the mock clients and pre-parsed templates.
3. Use `httptest.NewRecorder()` to capture the HTTP response.
4. Assert on status codes, response bodies, and redirect locations.

We covered `httptest` in Chapter 1, so we will not repeat the details here. The key insight is that Go's interface-based design makes this kind of testing natural—you do not need a DI framework or bytecode manipulation to substitute dependencies.

---

## Exercises

1. **Add a delete confirmation.** Currently, the delete button immediately submits a POST. Add a JavaScript `confirm()` dialog or an HTMX-powered confirmation modal before deletion.

2. **Pre-fill the edit form on validation error.** The current `AdminBookUpdate` handler renders a generic error page when validation fails. Modify it to re-render the edit form with the submitted values pre-filled (like the login form does with the email field).

3. **Add pagination to the catalog.** Extend the `ListBooks` gRPC call with `page` and `page_size` parameters. Update the HTMX filter to include pagination controls that swap results without full page reloads.

---

The admin routes are ready, but we do not yet have an admin account or sample books. In the next chapter, we will build CLI tools to create admin accounts and seed the catalog with sample data.

---

## References

[^1]: [Go net/http package—FormValue](https://pkg.go.dev/net/http#Request.FormValue)—Documentation for HTTP form value parsing.
[^2]: [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html)—Reference for gRPC status codes and their intended use.
[^3]: [httptest package](https://pkg.go.dev/net/http/httptest)—Go standard library for testing HTTP handlers.
[^4]: [Google API Design Guide—Errors](https://cloud.google.com/apis/design/errors)—Google's guide to mapping gRPC error codes to HTTP status codes.
