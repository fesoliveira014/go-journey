# Chapter 5: Gateway & Frontend — Design Spec

## Overview

Transform the existing gateway service from a stub JSON API into a full BFF (Backend for Frontend) that serves server-rendered HTML pages, manages user sessions via JWT cookies, and communicates with Auth and Catalog services over gRPC. The frontend uses Go `html/template` with HTMX for targeted interactive enhancements.

## Goals

- Teach the BFF pattern: gateway owns the user experience, backends own business logic
- Demonstrate Go `html/template` with base layout + `{{block}}` partials
- Show gRPC client usage from a Go HTTP server
- Introduce session management with HTTP-only JWT cookies
- Use HTMX for filter/search UX without full-page reloads
- Maintain the project's existing patterns (Server struct DI, middleware chain, Docker multi-stage builds)

## Non-Goals

- No stubs for services not yet built (reservation, notification)
- No CSS framework — minimal hand-written CSS
- No client-side JavaScript beyond HTMX
- No server-side session store — JWT cookie is the session
- No integration tests — deferred to Chapter 8 (CI/CD)

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Browser                        │
│  HTML forms ──POST──▶  Gateway  ◀──HTMX swap──  │
└─────────────────┬───────────────────────────────┘
                  │ HTTP (:8080)
┌─────────────────▼───────────────────────────────┐
│              Gateway (BFF)                       │
│                                                  │
│  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │
│  │Middleware │→ │ Handlers  │→ │  Templates   │  │
│  │(auth,log)│  │(auth,cat) │  │(base+blocks) │  │
│  └──────────┘  └─────┬─────┘  └──────────────┘  │
│                      │                           │
│         ┌────────────┼────────────┐              │
│         ▼                         ▼              │
│  ┌─────────────┐          ┌─────────────┐        │
│  │ Auth gRPC   │          │Catalog gRPC │        │
│  │ Client      │          │ Client      │        │
│  └──────┬──────┘          └──────┬──────┘        │
└─────────┼────────────────────────┼───────────────┘
          │ gRPC (:50051)          │ gRPC (:50052)
    ┌─────▼─────┐            ┌─────▼─────┐
    │Auth Service│            │Catalog Svc│
    └───────────┘            └───────────┘
```

### Key Flows

- **Login/Register**: HTML form POST → gateway handler → `AuthService.Login`/`Register` gRPC → set JWT in HttpOnly cookie → redirect
- **Browse catalog**: GET `/books` → auth middleware reads cookie, injects user into context → `CatalogService.ListBooks` gRPC → render template
- **Admin CRUD**: POST form → require admin role → `CatalogService.CreateBook`/`UpdateBook`/`DeleteBook` → redirect with flash message
- **OAuth2**: GET `/auth/oauth2/google` → handler calls `AuthService.InitOAuth2` gRPC → redirect to Google → callback at `/auth/oauth2/google/callback` → handler calls `AuthService.CompleteOAuth2` → set cookie → redirect home

## File Structure

```
services/gateway/
├── cmd/main.go                  # DI wiring: gRPC clients, server, middleware, routes
├── internal/
│   ├── handler/
│   │   ├── server.go            # Server struct with gRPC clients, template cache, JWT secret
│   │   ├── render.go            # render(), renderPartial(), renderError(), PageData struct
│   │   ├── auth.go              # LoginPage, LoginSubmit, RegisterPage, RegisterSubmit, Logout, OAuth2Start, OAuth2Callback
│   │   ├── catalog.go           # Home, BookList, BookDetail, AdminBookNew, AdminBookCreate, AdminBookEdit, AdminBookUpdate, AdminBookDelete
│   │   └── health.go            # Health (existing, adapted to Server method)
│   └── middleware/
│       ├── auth.go              # Cookie → JWT validation → context injection
│       └── logging.go           # Request method, path, status, duration
├── templates/
│   ├── base.html                # HTML shell with {{block "title"}}, {{template "nav"}}, {{block "content"}}
│   ├── home.html                # Landing page
│   ├── login.html               # Login form
│   ├── register.html            # Registration form
│   ├── catalog.html             # Book list with HTMX filter
│   ├── book.html                # Book detail page
│   ├── error.html               # Error page (status code + message)
│   ├── admin_book_new.html      # Create book form
│   ├── admin_book_edit.html     # Edit book form
│   └── partials/
│       ├── nav.html             # Navigation bar (login/logout, admin link)
│       ├── book_card.html       # Single book card (HTMX swap target)
│       └── flash.html           # Flash message banner
├── static/
│   └── style.css                # Minimal layout, table, form styles
├── go.mod                       # Adds gen, pkg/auth (with replace directives to ../../gen and ../../pkg/auth), google.golang.org/grpc
├── Dockerfile                   # Multi-stage: build binary, copy templates + static to runtime
├── Dockerfile.dev               # Air hot-reload with volume mounts
└── .air.toml                    # Existing, may need template watch paths
```

## Component Details

### Server Struct (`internal/handler/server.go`)

```go
type Server struct {
    auth      authv1.AuthServiceClient
    catalog   catalogv1.CatalogServiceClient
    tmpl      *template.Template
}

func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, tmpl *template.Template) *Server
```

All handlers are methods on `Server`. Dependencies are injected at construction in `cmd/main.go`. Note: the `Server` does not hold `jwtSecret` — JWT validation is handled exclusively by the auth middleware, and the gateway never signs tokens (it receives them from the auth service).

### Template Rendering (`internal/handler/render.go`)

```go
type PageData struct {
    User  *UserInfo // nil if anonymous
    Flash string    // from flash cookie
    Data  any       // page-specific payload
}

type UserInfo struct {
    ID   string // Converted from uuid.UUID via .String() in the auth middleware
    Role string
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any)
func (s *Server) renderPartial(w http.ResponseWriter, name string, data any)
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string)
```

- `render()` builds `PageData` from context (user) and cookie (flash), executes named template
- `renderPartial()` executes a partial template without the base layout (for HTMX responses)
- `renderError()` renders `error.html` with status code and message

### Routing (`cmd/main.go`)

Go 1.22+ `ServeMux` with method patterns:

| Method + Path | Handler | Auth Required |
|---------------|---------|---------------|
| `GET /` | `Home` | No |
| `GET /healthz` | `Health` | No |
| `GET /login` | `LoginPage` | No |
| `POST /login` | `LoginSubmit` | No |
| `GET /register` | `RegisterPage` | No |
| `POST /register` | `RegisterSubmit` | No |
| `POST /logout` | `Logout` | No |
| `GET /auth/oauth2/google` | `OAuth2Start` | No |
| `GET /auth/oauth2/google/callback` | `OAuth2Callback` | No |
| `GET /books` | `BookList` | No (user optional) |
| `GET /books/{id}` | `BookDetail` | No (user optional) |
| `GET /admin/books/new` | `AdminBookNew` | Admin |
| `POST /admin/books` | `AdminBookCreate` | Admin |
| `GET /admin/books/{id}/edit` | `AdminBookEdit` | Admin |
| `POST /admin/books/{id}` | `AdminBookUpdate` | Admin |
| `POST /admin/books/{id}/delete` | `AdminBookDelete` | Admin |
| `GET /static/` | `FileServer` | No |

### Middleware (`internal/middleware/`)

**Auth middleware (`auth.go`):**
- Reads `session` cookie
- Validates JWT locally via `pkg/auth.ValidateToken()`
- On valid token: injects `UserID` and `Role` into request context
- On missing/invalid token: continues with no user in context (anonymous)
- Does NOT reject requests — handlers decide whether to require auth

**Logging middleware (`logging.go`):**
- Wraps `http.ResponseWriter` to capture status code
- Logs: method, path, status code, duration

**Chain order in main.go:**
```go
var h http.Handler = mux
h = middleware.Auth(h, jwtSecret) // jwtSecret is a string
h = middleware.Logging(h)
```

### Session Management

**Cookie settings:**

| Field | Value | Rationale |
|-------|-------|-----------|
| `Name` | `"session"` | Standard name |
| `HttpOnly` | `true` | Prevents XSS access to token |
| `SameSite` | `Lax` | CSRF protection, allows GET navigations |
| `Secure` | `false` (dev), `true` (prod) | HTTPS only in production |
| `Path` | `"/"` | Available to all routes |
| `MaxAge` | `86400` (24h) | Matches JWT expiry |

**Flash messages:** Separate `flash` cookie. The `render()` helper reads the flash value and immediately clears the cookie by setting `MaxAge: -1` in the same response. The cookie is also set with `MaxAge: 10` as a fallback expiry in case the redirect response is cached or the flash is never consumed.

**Login flow:** POST `/login` → `AuthService.Login` gRPC → set session cookie → redirect `/books`

**Logout flow:** POST `/logout` → clear session cookie (MaxAge: -1) → redirect `/`

**OAuth2 flow:** GET `/auth/oauth2/google` → `AuthService.InitOAuth2` → redirect to Google → callback with code+state → `AuthService.CompleteOAuth2` → set cookie → redirect `/books`

**OAuth2 state validation:** The CSRF `state` parameter is generated and validated by the auth service (which maintains an in-memory state map with TTL, as built in Chapter 4). The gateway simply forwards the `state` from the callback query parameter to `CompleteOAuth2`. The gateway does not need to store state.

### Admin Role Gating

Admin routes are protected at the handler level using a `requireAdmin()` helper in the handler package:

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
```

Each admin handler calls `if !s.requireAdmin(w, r) { return }` as its first line. This is handler-level authorization (not middleware), matching the design principle that the auth middleware only populates context and handlers decide what level of auth they require. This is analogous to calling `@PreAuthorize("hasRole('ADMIN')")` at the controller method level in Spring Security.

### HTMX Usage

Targeted swaps only — standard form POST + redirect for all mutations:

- **Catalog filter:** `<select>` with `hx-get="/books"` + `hx-target="#book-list"` + `hx-swap="innerHTML"` swaps book cards without full-page reload
- **Query parameters:** `?genre=Fiction` maps to `ListBooksRequest.Genre`. Only genre filtering is implemented in this chapter. The `author`, `available_only`, `page`, and `page_size` fields exist in the proto but are not wired in the UI yet — pagination and additional filters are deferred to a later enhancement.
- **Detection:** Handler checks `r.Header.Get("HX-Request")` — if present, renders partial (book cards only); otherwise renders full page
- **No HTMX for:** login, register, CRUD forms, page navigation — all use standard HTML forms with POST-Redirect-GET pattern

### Error Handling

**gRPC error mapping** in `handleGRPCError()` (located in `render.go` alongside other rendering helpers):

| gRPC Code | HTTP Status | Behavior |
|-----------|-------------|----------|
| `NotFound` | 404 | Render error page |
| `InvalidArgument` | 400 | Render error with gRPC message |
| `AlreadyExists` | 409 | Render error with gRPC message |
| `Unauthenticated` | — | Redirect to `/login` |
| `PermissionDenied` | 403 | Render "Access denied" |
| Default | 500 | Render generic error |

**Form validation errors:** Re-render the form with error message in template data. No redirect on validation failure — user input is preserved.

**Backend unavailable:** gRPC clients created with dial timeout. If backend is down at startup, gateway still starts. Failed gRPC calls at runtime render 502 "Service temporarily unavailable".

### Templates

**Base layout (`base.html`):**
```html
<!DOCTYPE html>
<html>
<head>
    <title>{{block "title" .}}Library System{{end}}</title>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    {{template "nav" .}}
    <main>
        {{if .Flash}}<div class="flash">{{.Flash}}</div>{{end}}
        {{block "content" .}}{{end}}
    </main>
</body>
</html>
```

**Page templates** override `"title"` and `"content"` blocks.

**Nav partial** shows: site title, catalog link, login/register (anonymous) or username + logout (authenticated), admin link (admin role only).

**Template parsing:** All templates parsed once at startup. Go's `ParseGlob` does not recurse into subdirectories, so partials require a second call:
```go
tmpl, err := template.ParseGlob("templates/*.html")
tmpl, err = tmpl.ParseGlob("templates/partials/*.html")
```
The resulting `*template.Template` is cached in `Server.tmpl`.

## Testing Strategy

**Unit tests for handlers:**
- Mock gRPC clients implementing the generated interfaces
- Use `httptest.NewRequest` + `httptest.NewRecorder`
- Verify: HTTP status, response body contains expected HTML fragments, cookie set/cleared

**Middleware tests:**
- Valid cookie → context has UserID + Role
- No cookie → handler called, no user in context
- Expired/invalid token → no user in context

**Template tests:**
- Verify templates parse without error
- Render with test data, check output contains key elements

## Docker & Compose

**Dockerfile** (multi-stage):
- Build stage: copies `gen/`, `pkg/auth/`, `services/gateway/`, builds with `GOWORK=off`
- Runtime stage: copies binary + `templates/` + `static/` directories

**docker-compose.yml additions:**
```yaml
gateway:
    build:
      context: ..
      dockerfile: services/gateway/Dockerfile
    ports:
      - "${GATEWAY_PORT:-8080}:8080"
    environment:
      - PORT=8080
      - AUTH_GRPC_ADDR=auth:50051
      - CATALOG_GRPC_ADDR=catalog:50052
      - JWT_SECRET=${JWT_SECRET}
    depends_on:
      - auth
      - catalog
```

**Dev compose override:** Mounts `services/gateway/`, `gen/`, `pkg/auth/` as volumes. Air watches `.go`, `.html`, `.css` files for hot reload.

**New .env variables:**
- `GATEWAY_PORT` (default 8080)
- `AUTH_GRPC_ADDR` (default `auth:50051`)
- `CATALOG_GRPC_ADDR` (default `catalog:50052`)

## gRPC Client RPCs Used

**AuthService:**
- `Login(LoginRequest)` → `AuthResponse` (token + user)
- `Register(RegisterRequest)` → `AuthResponse` (token + user)
- `InitOAuth2(InitOAuth2Request)` → `InitOAuth2Response` (redirect_url)
- `CompleteOAuth2(CompleteOAuth2Request)` → `AuthResponse` (token + user)

**CatalogService:**
- `ListBooks(ListBooksRequest)` → `ListBooksResponse` (books + total_count)
- `GetBook(GetBookRequest)` → `Book`
- `CreateBook(CreateBookRequest)` → `Book`
- `UpdateBook(UpdateBookRequest)` → `Book`
- `DeleteBook(DeleteBookRequest)` → `DeleteBookResponse`

Note: `ValidateToken`, `GetUser`, and `UpdateAvailability` are not used by the gateway — JWT validation is local via `pkg/auth`, and availability updates come from the reservation service (future chapter).

## Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| HTTP router | Go 1.22+ stdlib `ServeMux` | No external deps, method patterns are sufficient |
| Template org | Base layout + `{{block}}` partials | Standard Go pattern, easy to teach |
| Session validation | Local JWT via `pkg/auth` | No network call per request, consistent with auth interceptor pattern |
| HTMX scope | Targeted swaps only (catalog filter) | Progressive enhancement, not SPA-lite |
| Middleware strategy | Single chain, handlers opt-in to auth | Simpler than route groups for 2 auth levels |
| Backend scope | Auth + Catalog only | No stubs for unbuilt services |
