# Chapter 5: Gateway & Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform the stub gateway into a BFF (Backend for Frontend) that serves HTML pages via Go templates, manages JWT sessions in cookies, connects to Auth and Catalog over gRPC, and uses HTMX for catalog filtering.

**Architecture:** Flat handler package with `Server` struct holding gRPC clients + template cache + JWT secret. Middleware chain (logging → auth) wraps stdlib `ServeMux`. Templates use base layout + `{{block}}` partials. HTMX for catalog filter swaps only.

**Tech Stack:** Go 1.22+ stdlib `net/http`, `html/template`, `google.golang.org/grpc`, HTMX 2.0.4, `pkg/auth` (shared JWT library)

**Spec:** `docs/superpowers/specs/2026-03-30-chapter-05-gateway-frontend-design.md`

---

## File Structure

```
services/gateway/
├── cmd/main.go                      # MODIFY: DI wiring, gRPC clients, middleware, routes
├── go.mod                           # MODIFY: add grpc, gen, pkg/auth dependencies
├── internal/
│   ├── handler/
│   │   ├── server.go                # CREATE: Server struct, New() constructor
│   │   ├── render.go                # CREATE: PageData, render(), renderPartial(), renderError(), handleGRPCError()
│   │   ├── render_test.go           # CREATE: render and flash helper tests
│   │   ├── auth.go                  # CREATE: LoginPage, LoginSubmit, RegisterPage, RegisterSubmit, Logout, OAuth2Start, OAuth2Callback
│   │   ├── auth_test.go             # CREATE: auth handler tests with mock gRPC client
│   │   ├── catalog.go               # CREATE: Home, BookList, BookDetail, AdminBookNew, AdminBookCreate, AdminBookEdit, AdminBookUpdate, AdminBookDelete
│   │   ├── catalog_test.go          # CREATE: catalog handler tests with mock gRPC client
│   │   ├── health.go                # MODIFY: adapt to Server method
│   │   ├── books.go                 # DELETE: replaced by catalog.go
│   │   ├── books_test.go            # DELETE: replaced by catalog_test.go
│   │   └── health_test.go           # MODIFY: adapt to Server method tests
│   └── middleware/
│       ├── auth.go                  # CREATE: cookie → JWT validate → context injection
│       ├── auth_test.go             # CREATE: middleware unit tests
│       ├── logging.go               # CREATE: request logging with status capture
│       └── logging_test.go          # CREATE: logging middleware tests
├── templates/
│   ├── base.html                    # CREATE: HTML shell with blocks
│   ├── home.html                    # CREATE: landing page
│   ├── login.html                   # CREATE: login form
│   ├── register.html                # CREATE: registration form
│   ├── catalog.html                 # CREATE: book list with HTMX filter
│   ├── book.html                    # CREATE: book detail page
│   ├── error.html                   # CREATE: error page
│   ├── admin_book_new.html          # CREATE: create book form
│   ├── admin_book_edit.html         # CREATE: edit book form
│   └── partials/
│       ├── nav.html                 # CREATE: navigation bar
│       ├── book_card.html           # CREATE: single book card (HTMX target)
│       └── flash.html               # CREATE: flash message banner
├── static/
│   └── style.css                    # CREATE: minimal styles
├── Dockerfile                       # MODIFY: copy templates/ and static/, add gen + pkg/auth
├── Dockerfile.dev                   # MODIFY: add gen + pkg/auth, template watch
└── .air.toml                        # MODIFY: watch .html and .css files

deploy/
├── docker-compose.yml               # MODIFY: update gateway service config
├── docker-compose.dev.yml           # MODIFY: update gateway dev override
└── .env                             # MODIFY: add GATEWAY_PORT, AUTH_GRPC_ADDR, CATALOG_GRPC_ADDR

docs/src/
├── SUMMARY.md                       # MODIFY: add Chapter 5 entries
└── ch05/
    ├── index.md                     # CREATE: chapter overview
    ├── bff-pattern.md               # CREATE: 5.1 — BFF architecture, why a gateway
    ├── templates-htmx.md            # CREATE: 5.2 — Go templates, HTMX basics
    ├── session-management.md        # CREATE: 5.3 — JWT cookies, login/logout, OAuth2
    └── admin-crud.md               # CREATE: 5.4 — admin pages, error handling, Docker
```

---

### Task 1: Server Struct, Rendering, and Template Foundation

**Context:** Build the core infrastructure — `Server` struct with dependency injection, template parsing, rendering helpers, and the base layout with partials. This is the foundation everything else builds on.

**Files:**
- Create: `services/gateway/internal/handler/server.go`
- Create: `services/gateway/internal/handler/render.go`
- Create: `services/gateway/templates/base.html`
- Create: `services/gateway/templates/home.html`
- Create: `services/gateway/templates/error.html`
- Create: `services/gateway/templates/partials/nav.html`
- Create: `services/gateway/templates/partials/flash.html`
- Create: `services/gateway/static/style.css`
- Modify: `services/gateway/go.mod` — add `gen`, `pkg/auth`, `google.golang.org/grpc` dependencies with replace directives
- Modify: `services/gateway/internal/handler/health.go` — convert to `Server` method
- Delete: `services/gateway/internal/handler/books.go` (replaced in Task 4)
- Delete: `services/gateway/internal/handler/books_test.go` (replaced in Task 4)
- Modify: `services/gateway/internal/handler/health_test.go` — adapt to use `Server`

**Details:**

`server.go` — The `Server` struct:
```go
package handler

import (
    "html/template"

    authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
    catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

type Server struct {
    auth      authv1.AuthServiceClient
    catalog   catalogv1.CatalogServiceClient
    tmpl      *template.Template
}

func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, tmpl *template.Template) *Server {
    return &Server{auth: auth, catalog: catalog, tmpl: tmpl}
}
```

`render.go` — Shared rendering helpers:
```go
package handler

import (
    "log"
    "net/http"
)

type UserInfo struct {
    ID   string
    Role string
}

type PageData struct {
    User  *UserInfo
    Flash string
    Data  any
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
    pd := PageData{
        User:  userFromContext(r.Context()),
        Flash: consumeFlash(w, r),
        Data:  data,
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := s.tmpl.ExecuteTemplate(w, name, pd); err != nil {
        log.Printf("template error: %v", err)
    }
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
        log.Printf("template error: %v", err)
    }
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
    pd := PageData{
        User:  userFromContext(r.Context()),
        Flash: consumeFlash(w, r),
        Data: map[string]any{
            "Status":  code,
            "Message": message,
        },
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(code)
    if err := s.tmpl.ExecuteTemplate(w, "error.html", pd); err != nil {
        log.Printf("template error: %v", err)
    }
}

// userFromContext extracts UserInfo from the request context.
// Returns nil if no user is set (anonymous request).
func userFromContext(ctx context.Context) *UserInfo {
    uid, err := pkgauth.UserIDFromContext(ctx)
    if err != nil {
        return nil
    }
    role, _ := pkgauth.RoleFromContext(ctx)
    return &UserInfo{ID: uid.String(), Role: role}
}

func setFlash(w http.ResponseWriter, message string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "flash",
        Value:    message,
        Path:     "/",
        MaxAge:   10,
        HttpOnly: true,
    })
}

func consumeFlash(w http.ResponseWriter, r *http.Request) string {
    c, err := r.Cookie("flash")
    if err != nil {
        return ""
    }
    http.SetCookie(w, &http.Cookie{
        Name:   "flash",
        Path:   "/",
        MaxAge: -1,
    })
    return c.Value
}
```

`base.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Library System{{end}}</title>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    {{template "nav" .}}
    <main class="container">
        {{if .Flash}}{{template "flash" .}}{{end}}
        {{block "content" .}}{{end}}
    </main>
</body>
</html>
```

`partials/nav.html`:
```html
{{define "nav"}}
<nav>
    <a href="/">Library System</a>
    <a href="/books">Catalog</a>
    {{if .User}}
        {{if eq .User.Role "admin"}}
            <a href="/admin/books/new">Add Book</a>
        {{end}}
        <form method="POST" action="/logout" style="display:inline">
            <button type="submit">Logout</button>
        </form>
    {{else}}
        <a href="/login">Login</a>
        <a href="/register">Register</a>
    {{end}}
</nav>
{{end}}
```

`partials/flash.html`:
```html
{{define "flash"}}
<div class="flash">{{.Flash}}</div>
{{end}}
```

`home.html`:
```html
{{define "title"}}Library System{{end}}
{{define "content"}}
<h1>Welcome to the Library</h1>
<p>Browse our <a href="/books">catalog</a> or <a href="/login">log in</a> to manage your account.</p>
{{end}}
```

`error.html`:
```html
{{define "title"}}Error{{end}}
{{define "content"}}
<h1>{{.Data.Status}}</h1>
<p>{{.Data.Message}}</p>
<a href="/">Back to Home</a>
{{end}}
```

`health.go` — convert to method:
```go
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

`static/style.css` — minimal functional styles:
```css
/* Reset and base */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: system-ui, sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 960px; margin: 0 auto; padding: 1rem; }

/* Nav */
nav { background: #2c3e50; padding: 0.75rem 1rem; display: flex; gap: 1rem; align-items: center; }
nav a, nav button { color: #ecf0f1; text-decoration: none; background: none; border: none; cursor: pointer; font-size: 1rem; }
nav a:hover, nav button:hover { text-decoration: underline; }

/* Flash */
.flash { background: #d4edda; color: #155724; padding: 0.75rem; border-radius: 4px; margin-bottom: 1rem; }

/* Forms */
form.auth-form { max-width: 400px; margin: 2rem auto; }
form.auth-form label { display: block; margin-top: 1rem; font-weight: 600; }
form.auth-form input { width: 100%; padding: 0.5rem; margin-top: 0.25rem; border: 1px solid #ccc; border-radius: 4px; }
form.auth-form button { margin-top: 1rem; padding: 0.5rem 1.5rem; background: #2c3e50; color: white; border: none; border-radius: 4px; cursor: pointer; }
.form-error { color: #dc3545; margin-top: 0.25rem; }

/* Book cards */
.book-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; margin-top: 1rem; }
.book-card { border: 1px solid #ddd; border-radius: 4px; padding: 1rem; }
.book-card h3 { margin-bottom: 0.25rem; }
.book-card .meta { color: #666; font-size: 0.9rem; }

/* Filter bar */
.filter-bar { display: flex; gap: 1rem; align-items: center; margin-bottom: 1rem; }
.filter-bar select { padding: 0.4rem; border: 1px solid #ccc; border-radius: 4px; }

/* Admin table */
table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #ddd; }

/* Error page */
h1.error-code { font-size: 4rem; color: #dc3545; }
```

`go.mod` — add dependencies. The module needs `replace` directives for `../../gen` and `../../pkg/auth`, same pattern as the auth and catalog services. Add `google.golang.org/grpc` and `google.golang.org/protobuf` as requires.

- [ ] **Step 1:** Update `go.mod` with replace directives and new dependencies. Run `go mod tidy` from `services/gateway/`.

- [ ] **Step 2:** Delete `books.go` and `books_test.go` — they'll be replaced by `catalog.go` in Task 4.

- [ ] **Step 3:** Create `server.go` with the `Server` struct and `New()` constructor.

- [ ] **Step 4:** Create `render.go` with `PageData`, `UserInfo`, `render()`, `renderPartial()`, `renderError()`, `setFlash()`, `consumeFlash()`, and `userFromContext()`.

- [ ] **Step 5:** Create template files: `base.html`, `home.html`, `error.html`, `partials/nav.html`, `partials/flash.html`.

- [ ] **Step 6:** Create `static/style.css` with minimal styles.

- [ ] **Step 7:** Convert `health.go` to a `Server` method. Update `health_test.go` to construct a `Server` with `New(nil, nil, tmpl)` and parsed templates, then test `srv.Health()`.

- [ ] **Step 8:** Write test for `render()` — construct `Server` with `New(nil, nil, tmpl)` and parsed test templates, verify output contains expected HTML fragments. Write test for `consumeFlash()` — verify cookie is read and cleared (MaxAge: -1).

- [ ] **Step 9:** Run `go build ./...` and `go test ./...` from `services/gateway/`. Fix any issues.

- [ ] **Step 10:** Commit: `feat(gateway): add server struct, template rendering, and base layout`

---

### Task 2: Middleware (Auth + Logging)

**Context:** Create the middleware chain. Auth middleware reads the JWT from the `session` cookie and injects user info into request context. Logging middleware captures method, path, status, and duration.

**Files:**
- Create: `services/gateway/internal/middleware/auth.go`
- Create: `services/gateway/internal/middleware/auth_test.go`
- Create: `services/gateway/internal/middleware/logging.go`
- Create: `services/gateway/internal/middleware/logging_test.go`

**Details:**

`middleware/auth.go`:
```go
package middleware

import (
    "context"
    "net/http"

    pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
)

func Auth(next http.Handler, jwtSecret string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session")
        if err != nil || cookie.Value == "" {
            next.ServeHTTP(w, r)
            return
        }
        claims, err := pkgauth.ValidateToken(cookie.Value, jwtSecret)
        if err != nil {
            // Invalid/expired token — continue as anonymous
            next.ServeHTTP(w, r)
            return
        }
        ctx := pkgauth.ContextWithUser(r.Context(), claims.UserID, claims.Role)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

`middleware/logging.go`:
```go
package middleware

import (
    "log"
    "net/http"
    "time"
)

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(sw, r)
        log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
    })
}
```

Tests:

`middleware/auth_test.go`:
- `TestAuth_ValidCookie` — generate a real JWT with `pkgauth.GenerateToken()`, set as `session` cookie, verify downstream handler sees `UserIDFromContext` and `RoleFromContext`
- `TestAuth_NoCookie` — no cookie set, handler called, `UserIDFromContext` returns empty
- `TestAuth_InvalidToken` — cookie with garbage value, handler called as anonymous
- `TestAuth_ExpiredToken` — generate JWT with past expiry, handler called as anonymous

`middleware/logging_test.go`:
- `TestLogging_CapturesStatus` — verify `statusWriter` captures WriteHeader code
- `TestLogging_DefaultStatus200` — handler that writes body without WriteHeader, verify status is 200

- [ ] **Step 1:** Create `middleware/auth.go`.
- [ ] **Step 2:** Write `middleware/auth_test.go` with the 4 test cases. Run tests.
- [ ] **Step 3:** Create `middleware/logging.go`.
- [ ] **Step 4:** Write `middleware/logging_test.go`. Run tests.
- [ ] **Step 5:** Run `go test ./...` from `services/gateway/`.
- [ ] **Step 6:** Commit: `feat(gateway): add auth and logging middleware`

---

### Task 3: Auth Handlers (Login, Register, Logout, OAuth2)

**Context:** Implement all authentication-related handlers. These call the Auth gRPC service and manage session cookies. The gateway never handles passwords or tokens directly — it delegates to the auth service.

**Files:**
- Create: `services/gateway/internal/handler/auth.go`
- Create: `services/gateway/internal/handler/auth_test.go`
- Create: `services/gateway/templates/login.html`
- Create: `services/gateway/templates/register.html`

**Details:**

`auth.go` — handlers on `Server`:

```go
// LoginPage renders the login form
func (s *Server) LoginPage(w http.ResponseWriter, r *http.Request) {
    s.render(w, r, "login.html", nil)
}

// LoginSubmit handles POST /login
func (s *Server) LoginSubmit(w http.ResponseWriter, r *http.Request) {
    email := r.FormValue("email")
    password := r.FormValue("password")
    if email == "" || password == "" {
        s.render(w, r, "login.html", map[string]any{"Error": "Email and password are required"})
        return
    }
    resp, err := s.auth.Login(r.Context(), &authv1.LoginRequest{Email: email, Password: password})
    if err != nil {
        s.render(w, r, "login.html", map[string]any{"Error": "Invalid email or password", "Email": email})
        return
    }
    setSessionCookie(w, resp.Token)
    setFlash(w, "Welcome back!")
    http.Redirect(w, r, "/books", http.StatusSeeOther)
}

// RegisterPage renders the registration form
func (s *Server) RegisterPage(w http.ResponseWriter, r *http.Request)

// RegisterSubmit handles POST /register
func (s *Server) RegisterSubmit(w http.ResponseWriter, r *http.Request)
    // Calls s.auth.Register(), sets cookie, redirects

// Logout handles POST /logout
func (s *Server) Logout(w http.ResponseWriter, r *http.Request)
    // Clears session cookie (MaxAge: -1), redirects to /

// OAuth2Start handles GET /auth/oauth2/google
func (s *Server) OAuth2Start(w http.ResponseWriter, r *http.Request)
    // Calls s.auth.InitOAuth2(), redirects to Google

// OAuth2Callback handles GET /auth/oauth2/google/callback
func (s *Server) OAuth2Callback(w http.ResponseWriter, r *http.Request)
    // Extracts code+state from query, calls s.auth.CompleteOAuth2(), sets cookie, redirects
```

Helper:
```go
func setSessionCookie(w http.ResponseWriter, token string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   86400,
    })
}

func clearSessionCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:   "session",
        Path:   "/",
        MaxAge: -1,
    })
}
```

`login.html`:
```html
{{define "title"}}Login{{end}}
{{define "content"}}
<h1>Login</h1>
{{if .Data.Error}}<p class="form-error">{{.Data.Error}}</p>{{end}}
<form method="POST" action="/login" class="auth-form">
    <label for="email">Email</label>
    <input type="email" name="email" id="email" value="{{if .Data.Email}}{{.Data.Email}}{{end}}" required>
    <label for="password">Password</label>
    <input type="password" name="password" id="password" required>
    <button type="submit">Login</button>
</form>
<p>Don't have an account? <a href="/register">Register</a></p>
<p>Or <a href="/auth/oauth2/google">sign in with Google</a></p>
{{end}}
```

`register.html` — similar structure with email, password, name fields.

Tests (`auth_test.go`):
- Mock auth client: `mockAuthClient` implementing `AuthServiceClient`. Note: the mock must implement ALL 6 interface methods (Register, Login, ValidateToken, GetUser, InitOAuth2, CompleteOAuth2) even if only some are tested — unimplemented methods can return `status.Error(codes.Unimplemented, "not implemented")`. Same applies to `mockCatalogClient` (6 methods).
- `TestLoginPage_RendersForm` — GET, verify 200, body contains "Login"
- `TestLoginSubmit_Success` — mock returns token, verify cookie set, redirect 303
- `TestLoginSubmit_InvalidCredentials` — mock returns error, verify form re-rendered with error message
- `TestLoginSubmit_EmptyFields` — verify form re-rendered with validation error
- `TestRegisterSubmit_Success` — mock returns token, verify cookie + redirect
- `TestLogout_ClearsCookie` — verify session cookie MaxAge -1, redirect
- `TestOAuth2Start_RedirectsToGoogle` — mock InitOAuth2 returns URL, verify redirect
- `TestOAuth2Callback_Success` — mock CompleteOAuth2 returns token, verify cookie + redirect

- [ ] **Step 1:** Create `templates/login.html` and `templates/register.html`.
- [ ] **Step 2:** Create `auth.go` with `LoginPage`, `LoginSubmit`, `RegisterPage`, `RegisterSubmit`, `Logout`, `OAuth2Start`, `OAuth2Callback`, and the cookie helpers.
- [ ] **Step 3:** Write `auth_test.go` with mock auth client and all test cases.
- [ ] **Step 4:** Run `go test ./...` from `services/gateway/`.
- [ ] **Step 5:** Commit: `feat(gateway): add auth handlers with login, register, logout, OAuth2`

---

### Task 4: Catalog Handlers (Browse, Detail, HTMX Filter)

**Context:** Implement catalog browsing with HTMX-powered filtering. The book list supports genre filtering via dropdown that swaps the book card list without a full page reload. When the handler detects `HX-Request` header, it renders only the partial.

**Files:**
- Create: `services/gateway/internal/handler/catalog.go`
- Create: `services/gateway/internal/handler/catalog_test.go`
- Modify: `services/gateway/internal/handler/render.go` — add `handleGRPCError()` helper
- Create: `services/gateway/templates/catalog.html`
- Create: `services/gateway/templates/book.html`
- Create: `services/gateway/templates/partials/book_card.html`

**Details:**

`catalog.go`:
```go
func (s *Server) Home(w http.ResponseWriter, r *http.Request) {
    // Render home.html — already created in Task 1
    s.render(w, r, "home.html", nil)
}

func (s *Server) BookList(w http.ResponseWriter, r *http.Request) {
    genre := r.URL.Query().Get("genre")
    resp, err := s.catalog.ListBooks(r.Context(), &catalogv1.ListBooksRequest{
        Genre: genre,
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to load books")
        return
    }
    if r.Header.Get("HX-Request") != "" {
        // HTMX request — render only book cards
        s.renderPartial(w, "book_card", resp.Books)
        return
    }
    s.render(w, r, "catalog.html", map[string]any{
        "Books":        resp.Books,
        "SelectedGenre": genre,
    })
}

func (s *Server) BookDetail(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    book, err := s.catalog.GetBook(r.Context(), &catalogv1.GetBookRequest{Id: id})
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to load book")
        return
    }
    s.render(w, r, "book.html", book)
}
```

`handleGRPCError` helper (add to `render.go`):
```go
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
    default:
        s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
    }
}
```

`catalog.html`:
```html
{{define "title"}}Catalog{{end}}
{{define "content"}}
<h1>Book Catalog</h1>
<div class="filter-bar">
    <label for="genre">Filter by genre:</label>
    <select name="genre" id="genre"
            hx-get="/books"
            hx-target="#book-list"
            hx-swap="innerHTML">
        <option value="">All Genres</option>
        <option value="Programming" {{if eq .Data.SelectedGenre "Programming"}}selected{{end}}>Programming</option>
        <option value="Architecture" {{if eq .Data.SelectedGenre "Architecture"}}selected{{end}}>Architecture</option>
        <option value="Distributed Systems" {{if eq .Data.SelectedGenre "Distributed Systems"}}selected{{end}}>Distributed Systems</option>
    </select>
</div>
<div id="book-list" class="book-list">
    {{range .Data.Books}}
        {{template "book_card" .}}
    {{end}}
</div>
{{end}}
```

`partials/book_card.html`:
```html
{{define "book_card"}}
<div class="book-card">
    <h3><a href="/books/{{.Id}}">{{.Title}}</a></h3>
    <p class="meta">{{.Author}} · {{.PublishedYear}}</p>
    <p class="meta">{{.Genre}} · {{.AvailableCopies}}/{{.TotalCopies}} available</p>
</div>
{{end}}
```

`book.html`:
```html
{{define "title"}}{{.Data.Title}}{{end}}
{{define "content"}}
<h1>{{.Data.Title}}</h1>
<p><strong>Author:</strong> {{.Data.Author}}</p>
<p><strong>ISBN:</strong> {{.Data.Isbn}}</p>
<p><strong>Genre:</strong> {{.Data.Genre}}</p>
<p><strong>Published:</strong> {{.Data.PublishedYear}}</p>
<p><strong>Available:</strong> {{.Data.AvailableCopies}} / {{.Data.TotalCopies}}</p>
{{if .Data.Description}}<p>{{.Data.Description}}</p>{{end}}
<a href="/books">Back to catalog</a>
{{end}}
```

Tests (`catalog_test.go`):
- Mock catalog client: `mockCatalogClient`
- `TestBookList_RendersBooks` — mock returns books, verify HTML contains book titles
- `TestBookList_HTMXRequest` — set `HX-Request` header, verify partial response (no `<html>` tag)
- `TestBookList_GenreFilter` — verify genre parameter passed to mock
- `TestBookList_GRPCError` — mock returns NotFound, verify error page
- `TestBookDetail_Success` — mock GetBook returns book, verify detail page
- `TestBookDetail_NotFound` — mock returns NotFound, verify 404

- [ ] **Step 1:** Add `handleGRPCError` to `render.go` (requires `google.golang.org/grpc/status` and `codes` imports).
- [ ] **Step 2:** Create `templates/catalog.html`, `templates/book.html`, `templates/partials/book_card.html`.
- [ ] **Step 3:** Create `catalog.go` with `Home`, `BookList`, `BookDetail`.
- [ ] **Step 4:** Write `catalog_test.go` with mock catalog client and all test cases.
- [ ] **Step 5:** Run `go test ./...`.
- [ ] **Step 6:** Commit: `feat(gateway): add catalog handlers with HTMX filtering`

---

### Task 5: Admin Handlers (Create, Edit, Delete Books)

**Context:** Admin-only CRUD pages for managing the book catalog. These require the `admin` role — if the user isn't an admin, return 403. Standard POST-redirect-GET pattern for all mutations.

**Files:**
- Modify: `services/gateway/internal/handler/catalog.go` — add admin handlers
- Modify: `services/gateway/internal/handler/catalog_test.go` — add admin tests
- Create: `services/gateway/templates/admin_book_new.html`
- Create: `services/gateway/templates/admin_book_edit.html`

**Details:**

Admin handlers added to `catalog.go`:
```go
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

func (s *Server) AdminBookNew(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) { return }
    s.render(w, r, "admin_book_new.html", nil)
}

func (s *Server) AdminBookCreate(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) { return }
    // Parse form, call s.catalog.CreateBook(), setFlash, redirect to /books
}

func (s *Server) AdminBookEdit(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) { return }
    id := r.PathValue("id")
    book, err := s.catalog.GetBook(r.Context(), &catalogv1.GetBookRequest{Id: id})
    // Render edit form pre-populated with book data
}

func (s *Server) AdminBookUpdate(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) { return }
    // Parse form, call s.catalog.UpdateBook(), setFlash, redirect to /books/{id}
}

func (s *Server) AdminBookDelete(w http.ResponseWriter, r *http.Request) {
    if !s.requireAdmin(w, r) { return }
    id := r.PathValue("id")
    // Call s.catalog.DeleteBook(), setFlash, redirect to /books
}
```

`admin_book_new.html`:
```html
{{define "title"}}Add Book{{end}}
{{define "content"}}
<h1>Add New Book</h1>
{{if .Data.Error}}<p class="form-error">{{.Data.Error}}</p>{{end}}
<form method="POST" action="/admin/books" class="auth-form">
    <label for="title">Title</label>
    <input type="text" name="title" id="title" required>
    <label for="author">Author</label>
    <input type="text" name="author" id="author" required>
    <label for="isbn">ISBN</label>
    <input type="text" name="isbn" id="isbn" required>
    <label for="genre">Genre</label>
    <input type="text" name="genre" id="genre" required>
    <label for="description">Description</label>
    <textarea name="description" id="description"></textarea>
    <label for="published_year">Published Year</label>
    <input type="number" name="published_year" id="published_year" required>
    <label for="total_copies">Total Copies</label>
    <input type="number" name="total_copies" id="total_copies" required>
    <button type="submit">Create Book</button>
</form>
{{end}}
```

`admin_book_edit.html` — similar, pre-populated with existing values from `.Data`.

Tests:
- `TestAdminBookNew_RequiresAdmin` — no user in context → redirect to login
- `TestAdminBookNew_NonAdmin` — user role "member" → 403
- `TestAdminBookNew_Admin` — admin user → 200, form rendered
- `TestAdminBookCreate_Success` — mock CreateBook, verify redirect + flash
- `TestAdminBookEdit_LoadsBook` — mock GetBook, verify form pre-populated
- `TestAdminBookUpdate_Success` — mock UpdateBook, verify redirect
- `TestAdminBookDelete_Success` — mock DeleteBook, verify redirect

- [ ] **Step 1:** Create `templates/admin_book_new.html` and `templates/admin_book_edit.html`.
- [ ] **Step 2:** Add `requireAdmin` helper and all admin handlers to `catalog.go`.
- [ ] **Step 3:** Write admin test cases in `catalog_test.go`.
- [ ] **Step 4:** Run `go test ./...`.
- [ ] **Step 5:** Commit: `feat(gateway): add admin book CRUD handlers`

---

### Task 6: Main Wiring and Route Registration

**Context:** Wire everything together in `cmd/main.go` — create gRPC client connections, parse templates, construct `Server`, register routes, apply middleware, start HTTP server.

**Files:**
- Modify: `services/gateway/cmd/main.go` — complete rewrite
- Modify: `services/gateway/.air.toml` — add template/static watch patterns

**Details:**

`cmd/main.go`:
```go
package main

import (
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "path/filepath"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
    catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
    "github.com/fesoliveira014/library-system/services/gateway/internal/handler"
    "github.com/fesoliveira014/library-system/services/gateway/internal/middleware"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Fatal("JWT_SECRET is required")
    }

    // gRPC connections
    authAddr := os.Getenv("AUTH_GRPC_ADDR")
    if authAddr == "" {
        authAddr = "localhost:50051"
    }
    authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("connect to auth service: %v", err)
    }
    defer authConn.Close()

    catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
    if catalogAddr == "" {
        catalogAddr = "localhost:50052"
    }
    catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("connect to catalog service: %v", err)
    }
    defer catalogConn.Close()

    // Parse templates
    tmpl, err := template.ParseGlob(filepath.Join("templates", "*.html"))
    if err != nil {
        log.Fatalf("parse templates: %v", err)
    }
    tmpl, err = tmpl.ParseGlob(filepath.Join("templates", "partials", "*.html"))
    if err != nil {
        log.Fatalf("parse partials: %v", err)
    }

    // Create server
    authClient := authv1.NewAuthServiceClient(authConn)
    catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
    srv := handler.New(authClient, catalogClient, tmpl)

    // Routes
    mux := http.NewServeMux()
    mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    mux.HandleFunc("GET /healthz", srv.Health)
    mux.HandleFunc("GET /{$}", srv.Home)

    // Auth routes
    mux.HandleFunc("GET /login", srv.LoginPage)
    mux.HandleFunc("POST /login", srv.LoginSubmit)
    mux.HandleFunc("GET /register", srv.RegisterPage)
    mux.HandleFunc("POST /register", srv.RegisterSubmit)
    mux.HandleFunc("POST /logout", srv.Logout)
    mux.HandleFunc("GET /auth/oauth2/google", srv.OAuth2Start)
    mux.HandleFunc("GET /auth/oauth2/google/callback", srv.OAuth2Callback)

    // Catalog routes
    mux.HandleFunc("GET /books", srv.BookList)
    mux.HandleFunc("GET /books/{id}", srv.BookDetail)

    // Admin routes
    mux.HandleFunc("GET /admin/books/new", srv.AdminBookNew)
    mux.HandleFunc("POST /admin/books", srv.AdminBookCreate)
    mux.HandleFunc("GET /admin/books/{id}/edit", srv.AdminBookEdit)
    mux.HandleFunc("POST /admin/books/{id}", srv.AdminBookUpdate)
    mux.HandleFunc("POST /admin/books/{id}/delete", srv.AdminBookDelete)

    // Middleware chain
    var h http.Handler = mux
    h = middleware.Auth(h, jwtSecret)
    h = middleware.Logging(h)

    addr := fmt.Sprintf(":%s", port)
    log.Printf("gateway listening on %s", addr)
    if err := http.ListenAndServe(addr, h); err != nil {
        log.Fatalf("server failed: %v", err)
    }
}
```

`.air.toml` update — add `.html` and `.css` to the watch include list so template changes trigger a rebuild.

- [ ] **Step 1:** Rewrite `cmd/main.go` with full wiring.
- [ ] **Step 2:** Update `.air.toml` to watch template and static files.
- [ ] **Step 3:** Run `go build ./cmd/...` to verify compilation.
- [ ] **Step 4:** Commit: `feat(gateway): wire routes, middleware, and gRPC clients in main`

---

### Task 7: Docker and Compose Configuration

**Context:** Update Dockerfiles and compose configs so the gateway builds and runs with its new dependencies (gen, pkg/auth, templates, static files).

**Files:**
- Modify: `services/gateway/Dockerfile`
- Modify: `services/gateway/Dockerfile.dev`
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/docker-compose.dev.yml`
- Modify: `deploy/.env`

**Details:**

`Dockerfile` (production):
```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY gen/ gen/
COPY pkg/auth/ pkg/auth/
COPY services/gateway/ services/gateway/

WORKDIR /app/services/gateway
ENV GOWORK=off
RUN go build -o /gateway ./cmd

FROM alpine:3.21
COPY --from=builder /gateway /gateway
COPY services/gateway/templates/ /templates/
COPY services/gateway/static/ /static/
WORKDIR /
ENTRYPOINT ["/gateway"]
```

`Dockerfile.dev`:
```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

COPY gen/ gen/
COPY pkg/auth/ pkg/auth/
COPY services/gateway/ services/gateway/

WORKDIR /app/services/gateway
ENV GOWORK=off
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

`docker-compose.yml` — update existing gateway service block (keep `networks: [library-net]`):
```yaml
gateway:
    build:
      context: ..
      dockerfile: services/gateway/Dockerfile
    environment:
      PORT: "8080"
      AUTH_GRPC_ADDR: "auth:50051"
      CATALOG_GRPC_ADDR: "catalog:50052"
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}
    ports:
      - "${GATEWAY_PORT:-8080}:8080"
    depends_on:
      - auth
      - catalog
    networks:
      - library-net
```

`docker-compose.dev.yml` — update gateway dev override:
```yaml
gateway:
    build:
      context: ..
      dockerfile: services/gateway/Dockerfile.dev
    volumes:
      - ../services/gateway:/app/services/gateway
      - ../gen:/app/gen
      - ../pkg/auth:/app/pkg/auth
```

`.env` additions (`GATEWAY_PORT` already exists, only add the new ones):
```
AUTH_GRPC_ADDR=auth:50051
CATALOG_GRPC_ADDR=catalog:50052
```

- [ ] **Step 1:** Update `Dockerfile` with gen, pkg/auth copies and template/static copies to runtime.
- [ ] **Step 2:** Update `Dockerfile.dev` with gen, pkg/auth copies.
- [ ] **Step 3:** Update `docker-compose.yml` with gateway environment and depends_on.
- [ ] **Step 4:** Update `docker-compose.dev.yml` with volume mounts.
- [ ] **Step 5:** Add new variables to `.env`.
- [ ] **Step 6:** Run `docker compose -f deploy/docker-compose.yml build gateway` to verify the build.
- [ ] **Step 7:** Commit: `feat(gateway): update Docker and Compose for BFF gateway`

---

### Task 8: Chapter 5 Documentation

**Context:** Write the tutorial documentation for Chapter 5. Four sub-chapters covering the BFF pattern, templates + HTMX, session management, and admin CRUD. Update SUMMARY.md to add the new chapter entries.

**Files:**
- Modify: `docs/src/SUMMARY.md`
- Create: `docs/src/ch05/index.md`
- Create: `docs/src/ch05/bff-pattern.md`
- Create: `docs/src/ch05/templates-htmx.md`
- Create: `docs/src/ch05/session-management.md`
- Create: `docs/src/ch05/admin-crud.md`

**Details:**

Chapter structure:

**`index.md`** (~400 words) — Overview of Chapter 5. Mermaid architecture diagram showing browser → gateway → auth/catalog. What you'll build: BFF gateway with HTML templates, JWT sessions, HTMX filtering, admin CRUD.

**`bff-pattern.md`** (~1500 words) — Section 5.1:
- What is a BFF and why use one (vs. direct API calls from frontend)
- Go as a BFF language — `net/http`, `html/template`, stdlib routing
- The `Server` struct pattern — dependency injection in Go HTTP services
- Comparison to Spring `@Controller` + `@Service` injection
- Middleware chain — wrapping handlers, the `http.Handler` interface
- Code walkthrough: `server.go`, `middleware/auth.go`, `cmd/main.go` routing

**`templates-htmx.md`** (~1500 words) — Section 5.2:
- Go `html/template` basics — `{{define}}`, `{{template}}`, `{{block}}`
- Base layout + partials pattern (vs. Spring Thymeleaf layouts)
- Template parsing and caching at startup
- HTMX introduction — what it is, the 3 attributes (`hx-get`, `hx-target`, `hx-swap`)
- Building the catalog filter — detecting `HX-Request`, partial vs. full renders
- When to use HTMX vs. standard forms (targeted enhancement, not SPA)

**`session-management.md`** (~1500 words) — Section 5.3:
- JWT in cookies vs. localStorage (security tradeoffs)
- Cookie attributes: HttpOnly, SameSite, Secure, MaxAge
- Login/register flow: form POST → gRPC call → set cookie → redirect (PRG pattern)
- OAuth2 flow through the gateway (gateway as redirect orchestrator)
- Flash messages via cookies — simple, no server-side state
- Logout — clearing the cookie

**`admin-crud.md`** (~1200 words) — Section 5.4:
- Role-based access: `requireAdmin` helper pattern
- Form handling: parsing, validation, re-rendering with errors
- gRPC error mapping — translating backend errors to user-friendly pages
- Docker setup: multi-stage build with templates + static files
- Testing strategy: mock gRPC clients, `httptest`

Each chapter should include code snippets from the actual implementation, footnoted references to external sources, and exercises where appropriate.

- [ ] **Step 1:** Add Chapter 5 entries to `docs/src/SUMMARY.md`.
- [ ] **Step 2:** Create `docs/src/ch05/index.md` with overview and architecture diagram.
- [ ] **Step 3:** Create `docs/src/ch05/bff-pattern.md`.
- [ ] **Step 4:** Create `docs/src/ch05/templates-htmx.md`.
- [ ] **Step 5:** Create `docs/src/ch05/session-management.md`.
- [ ] **Step 6:** Create `docs/src/ch05/admin-crud.md`.
- [ ] **Step 7:** Run `mdbook build` from `docs/` to verify no broken links.
- [ ] **Step 8:** Commit: `docs: add Chapter 5 — Gateway & Frontend`
