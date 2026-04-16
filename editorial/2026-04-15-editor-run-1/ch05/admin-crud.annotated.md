# 5.4 Admin CRUD

<!-- [STRUCTURAL] Opener lists the four topics this section covers, mirrored by the section headings below. Good roadmap. -->
<!-- [LINE EDIT] "The library system needs an admin interface for managing books -- creating, editing, and deleting entries in the catalog." — tight. -->
<!-- [COPY EDIT] CRUD is a common acronym; no need to expand in a tech book for a 7+ yr engineer, but CMOS 10.2 strict style would spell out at first use ("create, read, update, delete (CRUD)"). Optional. -->
The library system needs an admin interface for managing books -- creating, editing, and deleting entries in the catalog. This section covers the patterns used in the admin handlers: role-based access control, form handling, gRPC error mapping, and the Docker build.

---

## Role-Based Access: The `requireAdmin` Helper

<!-- [STRUCTURAL] Motivation-first: "every admin handler needs two checks" → extract helper. Good pedagogy. -->
<!-- [LINE EDIT] "Every admin handler needs to verify two things: the user is authenticated, and the user has the `\"admin\"` role." — "to verify two things" is slightly wordy. Consider: "Every admin handler verifies two things:". -->
Every admin handler needs to verify two things: the user is authenticated, and the user has the `"admin"` role. Rather than repeating this check in every handler, we extract it into a helper method:

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

<!-- [LINE EDIT] "The pattern is: call `requireAdmin` at the top of the handler, and return immediately if it returns `false`." — clear usage contract. -->
<!-- [LINE EDIT] "The helper has already written the response" — good side-effect disclosure. -->
The pattern is: call `requireAdmin` at the top of the handler, and return immediately if it returns `false`. The helper has already written the response (either a redirect to login or a 403 error page).

```go
func (s *Server) AdminBookNew(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) {
        return
    }
    s.render(w, r, "admin_book_new.html", map[string]any{})
}
```

<!-- [LINE EDIT] "This is a simpler approach than Spring Security's `@PreAuthorize(\"hasRole('ADMIN')\")` annotation" — accurate Java-bridge. -->
<!-- [LINE EDIT] "The tradeoff is that you must remember to call `requireAdmin` in every admin handler -- there is no framework enforcing it at the routing level." — honest. Keep. -->
<!-- [COPY EDIT] "tradeoff" — see session-management.md note: lock "trade-off" or "tradeoff" chapter-wide (CMOS prefers "trade-off"). -->
<!-- [LINE EDIT] "For a small codebase, this explicitness is a feature. For a large one, you might extract it into middleware that applies to an entire route prefix." — strong, scale-aware. -->
This is a simpler approach than Spring Security's `@PreAuthorize("hasRole('ADMIN')")` annotation, but it achieves the same result. The tradeoff is that you must remember to call `requireAdmin` in every admin handler -- there is no framework enforcing it at the routing level. For a small codebase, this explicitness is a feature. For a large one, you might extract it into middleware that applies to an entire route prefix.

---

## Form Handling in Go

<!-- [STRUCTURAL] Two-function overview → concrete handler. Order is right. -->
HTML forms submit data as URL-encoded key-value pairs. Go's `net/http` package provides two ways to access form data:

<!-- [LINE EDIT] "Returns a single value. Calls `r.ParseForm()` implicitly on the first call." — accurate. -->
<!-- [COPY EDIT] "`r.Form` (a `map[string][]string`)" — parenthetical type is correctly inline-coded. -->
- **`r.FormValue("key")`** -- Returns a single value. Calls `r.ParseForm()` implicitly on the first call.
- **`r.ParseForm()`** -- Parses the request body (for POST) and URL query parameters. After calling this, you can access `r.Form` (a `map[string][]string`) directly.

<!-- [LINE EDIT] "For the admin book creation handler, we need to convert string form values to integers using `strconv.Atoi`:" — tight. -->
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

<!-- [STRUCTURAL] Code is long but necessary — readers need to see the full validation/conversion sequence. Consider trimming the repeated re-render blocks to show only the first, with "// ... similar handling for totalCopies ..." — saves ~8 lines without losing pedagogy. Optional. -->
<!-- [COPY EDIT] "setFlash(w, ...)" — this is called as a package-level function here, but §5.3 shows `s.setFlash(w, ...)` as a method on `*Server`. Please verify this is not a bug or stale snippet. If `setFlash` is the package-level exported variant, state that explicitly; otherwise change to `s.setFlash(w, "Book created")`. -->
<!-- [LINE EDIT] "In Spring, `@ModelAttribute` or `@RequestBody` would handle this binding and conversion automatically." — accurate. -->
<!-- [LINE EDIT] "This is more verbose but leaves no ambiguity about what happens when a field is missing or cannot be parsed." — honest trade-off framing. Keep. -->
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

<!-- [LINE EDIT] "After a successful create or update, the handler sets a flash message and redirects -- the PRG pattern from section 5.3." — good back-reference. -->
After a successful create or update, the handler sets a flash message and redirects -- the PRG pattern from section 5.3.

---

## gRPC Error Mapping

<!-- [STRUCTURAL] Function shown, then table. Table is the authoritative lookup; function is the concrete mechanism. Good. -->
<!-- [LINE EDIT] "When a gRPC call fails, the error contains a status code and message." — accurate. -->
<!-- [LINE EDIT] "The `handleGRPCError` function translates gRPC status codes into appropriate HTTP responses:" — good framing. -->
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

<!-- [LINE EDIT] "The mapping follows standard conventions:" — "standard" is a small claim; the Google API Design Guide reference [^4] backs it up. OK. -->
The mapping follows standard conventions:

<!-- [COPY EDIT] Table "Response" column mixes sentence case and fragments. Keep. -->
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

<!-- [LINE EDIT] "Notice that `InvalidArgument`, `AlreadyExists`, and `FailedPrecondition` pass through the gRPC message (`st.Message()`) because it contains useful context (e.g., \"ISBN already exists\")." — good concrete example. -->
<!-- [COPY EDIT] "e.g.," — comma correct (CMOS 6.43). -->
<!-- [LINE EDIT] "For `NotFound` and `PermissionDenied`, we use generic messages to avoid leaking internal details." — important security point, tucked in cleanly. -->
<!-- [LINE EDIT] "`ResourceExhausted` uses a hardcoded user-friendly message for the reservation limit." — "hardcoded user-friendly" reads awkwardly. Consider: "`ResourceExhausted` uses a fixed, user-friendly message for the reservation limit." -->
<!-- [COPY EDIT] "user-friendly" — hyphenated compound adjective before "message" (CMOS 7.81). Correct. -->
<!-- [COPY EDIT] "hardcoded" — one word per recent style guides. Correct. -->
Notice that `InvalidArgument`, `AlreadyExists`, and `FailedPrecondition` pass through the gRPC message (`st.Message()`) because it contains useful context (e.g., "ISBN already exists"). For `NotFound` and `PermissionDenied`, we use generic messages to avoid leaking internal details. `ResourceExhausted` uses a hardcoded user-friendly message for the reservation limit.

---

## Docker Build

<!-- [STRUCTURAL] Good: short intro, full Dockerfile, then post-hoc commentary on the key delta. -->
<!-- [LINE EDIT] "with one addition: it copies the `templates/` and `static/` directories into the runtime image" — tight. -->
The gateway Dockerfile follows the same multi-stage pattern from Chapter 3, with one addition: it copies the `templates/` and `static/` directories into the runtime image.

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

<!-- [COPY EDIT] Please verify: `golang:1.26-alpine` is a valid Docker tag. As of April 2026, Go 1.26 would be the current stable (1.22 Feb 2024, ~6mo cadence: 1.23 Aug 2024, 1.24 Feb 2025, 1.25 Aug 2025, 1.26 Feb 2026). Likely accurate but confirm. -->
<!-- [COPY EDIT] Please verify: `alpine:3.19` is an actively maintained Alpine tag. As of April 2026, 3.19 (released Dec 2023) is near or past EOL; Alpine 3.22 or 3.23 would be current. Recommend bumping to a current Alpine tag for consistency with chapter's "current best practice" tone. -->
<!-- [LINE EDIT] "The key difference from the Auth and Catalog Dockerfiles: the runtime image includes `templates/` and `static/` because the gateway reads these at startup." — good delta statement. -->
<!-- [LINE EDIT] "The Go binary handles compiled code, but templates and CSS are loaded from the filesystem." — accurate. -->
<!-- [LINE EDIT] "The `WORKDIR /app` ensures the binary can find `templates/` and `static/` at the relative paths used in `main.go`." — good explanation of the WORKDIR's functional purpose. Consider forward-noting that in production an `embed.FS` approach would be preferable — a common cloud-native refinement. Optional. -->
The key difference from the Auth and Catalog Dockerfiles: the runtime image includes `templates/` and `static/` because the gateway reads these at startup. The Go binary handles compiled code, but templates and CSS are loaded from the filesystem. The `WORKDIR /app` ensures the binary can find `templates/` and `static/` at the relative paths used in `main.go`.

---

## Testing Strategy

<!-- [STRUCTURAL] Section title says "Strategy" and delivers one — interface substitution. It's short (~6 lines). Given the Java reader's expectation of "testing chapter", more depth on `httptest.NewRecorder` or a table-driven example would be welcome. Flagging as a gap but not fatal since Chapter 1 is cited. -->
<!-- [LINE EDIT] "The gateway's handlers are straightforward to test because their dependencies are interfaces." — good. -->
<!-- [COPY EDIT] "generated by protoc" — "protoc" as lowercase tool name is conventional. Correct. -->
The gateway's handlers are straightforward to test because their dependencies are interfaces. The gRPC client types (`authv1.AuthServiceClient`, `catalogv1.CatalogServiceClient`) are interfaces generated by protoc -- you can substitute mock implementations in tests without any mocking framework.

The standard approach:

1. Create a mock that implements the gRPC client interface, returning canned responses.
2. Construct a `Server` with the mock clients and pre-parsed templates.
3. Use `httptest.NewRecorder()` to capture the HTTP response.
4. Assert on status codes, response bodies, and redirect locations.

<!-- [LINE EDIT] "We covered `httptest` in Chapter 1, so we will not repeat the details here." — appropriate cross-reference. -->
<!-- [LINE EDIT] "The key insight is that Go's interface-based design makes this kind of testing natural -- you do not need a DI framework or bytecode manipulation to substitute dependencies." — good closing observation; "bytecode manipulation" is a nice jab at the Java/JVM tooling world the reader comes from. Keep. -->
<!-- [COPY EDIT] "interface-based" — hyphenated compound adjective (CMOS 7.81). Correct. -->
We covered `httptest` in Chapter 1, so we will not repeat the details here. The key insight is that Go's interface-based design makes this kind of testing natural -- you do not need a DI framework or bytecode manipulation to substitute dependencies.

---

## Exercises

<!-- [STRUCTURAL] Three exercises, increasing in ambition. Scoped well — all are achievable within the current codebase with the patterns just taught. -->
<!-- [LINE EDIT] "Currently, the delete button immediately submits a POST." — accurate. -->
<!-- [LINE EDIT] "Add a JavaScript `confirm()` dialog or an HTMX-powered confirmation modal before deletion." — clear alternatives. -->
1. **Add a delete confirmation.** Currently, the delete button immediately submits a POST. Add a JavaScript `confirm()` dialog or an HTMX-powered confirmation modal before deletion.

<!-- [LINE EDIT] "Modify it to re-render the edit form with the submitted values pre-filled (like the login form does with the email field)." — good concrete reference. -->
2. **Pre-fill the edit form on validation error.** The current `AdminBookUpdate` handler renders a generic error page when validation fails. Modify it to re-render the edit form with the submitted values pre-filled (like the login form does with the email field).

<!-- [LINE EDIT] "Extend the `ListBooks` gRPC call with `page` and `page_size` parameters." — correct snake_case per protobuf convention. -->
<!-- [COPY EDIT] "`page` and `page_size`" — snake_case in proto field names is correct per protobuf style. -->
<!-- [LINE EDIT] "Update the HTMX filter to include pagination controls that swap results without full page reloads." — achievable. Good. -->
3. **Add pagination to the catalog.** Extend the `ListBooks` gRPC call with `page` and `page_size` parameters. Update the HTMX filter to include pagination controls that swap results without full page reloads.

---

<!-- [LINE EDIT] "The admin routes are ready, but we don't yet have an admin account or sample books." — "we don't" contraction; chapter is largely uncontracted. Lock one register. "we do not yet have" is the consistent form. -->
<!-- [LINE EDIT] "In the next chapter, we'll build CLI tools to create admin accounts and seed the catalog with sample data." — "we'll" same contraction issue. -->
<!-- [STRUCTURAL] Good chapter-to-chapter handoff. Teases the next concrete deliverable (CLI tools). -->
The admin routes are ready, but we don't yet have an admin account or sample books. In the next chapter, we'll build CLI tools to create admin accounts and seed the catalog with sample data.

---

## References

<!-- [COPY EDIT] Please verify all four URLs resolve (pkg.go.dev/net/http#Request.FormValue; grpc.github.io statuscodes; pkg.go.dev/net/http/httptest; cloud.google.com/apis/design/errors). -->
[^1]: [Go net/http package -- FormValue](https://pkg.go.dev/net/http#Request.FormValue) -- Documentation for HTTP form value parsing.
[^2]: [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html) -- Reference for gRPC status codes and their intended use.
[^3]: [httptest package](https://pkg.go.dev/net/http/httptest) -- Go standard library for testing HTTP handlers.
[^4]: [Google API Design Guide -- Errors](https://cloud.google.com/apis/design/errors) -- Google's guide to mapping gRPC error codes to HTTP status codes.
