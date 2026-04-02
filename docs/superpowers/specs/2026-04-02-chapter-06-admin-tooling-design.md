# Chapter 6: Admin & Developer Tooling — Design Spec

## Goal

Add an admin dashboard (view-only user and reservation management), admin account creation CLI, and catalog seed CLI. Insert these as a new Chapter 6, renumbering existing Ch.6-13 to Ch.7-14.

## Context

After Chapter 5 (Gateway & Frontend), readers have a working gateway with admin book CRUD routes, but:
- No way to create admin accounts except manual SQL
- No way to populate the catalog with sample data
- No admin visibility into users or reservations

This chapter fills those gaps before the event-driven chapters, so readers enter Ch.7 (Kafka) with a populated catalog and a working admin account.

## Scope

### In Scope
- Admin CLI tool to create admin accounts directly in the database
- New `ListUsers` RPC on the auth service (admin-only)
- New `ListAllReservations` RPC on the reservation service (admin-only, with denormalized user email and book title)
- Admin dashboard in the gateway (landing page, user list, reservation list)
- Catalog seed CLI that logs in via auth gRPC, then calls `CreateBook` on the catalog service
- JSON fixture file with ~15-20 sample books
- New Chapter 6 documentation (4 sections)
- Chapter renumbering (Ch.6-13 become Ch.7-14)
- Cross-reference updates in earlier and later chapters

### Out of Scope
- User management actions (banning, role promotion via UI)
- Reservation management actions (canceling reservations on behalf of users)
- Pagination on admin list endpoints
- Admin audit logging

---

## 1. Admin CLI Tool

**Location:** `services/auth/cmd/admin/main.go`

**Module:** Lives in the auth service module to reuse `model.User`, `repository.UserRepository`, and bcrypt password hashing.

**Behavior:**
- Connects directly to PostgreSQL using the `DATABASE_URL` environment variable
- Accepts `--email`, `--password`, and `--name` flags
- Hashes the password with bcrypt (reusing existing auth service logic)
- Inserts a user with `role = "admin"` via the existing GORM repository
- If the email already exists, updates the role to `"admin"` (idempotent)
- Runs GORM AutoMigrate to ensure the schema exists
- No gRPC calls, no JWT, no service layer — direct DB access

**Usage:**
```bash
DATABASE_URL="host=localhost port=5434 user=postgres password=postgres dbname=auth sslmode=disable" \
  go run services/auth/cmd/admin/main.go \
    --email admin@example.com \
    --password secret \
    --name "Admin User"
```

**No new dependencies.** Uses `flag` from the standard library and existing auth module packages.

---

## 2. New Proto RPCs

### 2.1 Auth Service — `ListUsers`

**File:** `proto/auth/v1/auth.proto`

```protobuf
rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);

message ListUsersRequest {}

message ListUsersResponse {
  repeated User users = 1;
}
```

The existing `User` message already has `id`, `email`, `name`, `role`, `created_at` fields.

**Auth:** Requires admin role. Enforced by adding `ListUsers` to the interceptor's protected methods (same pattern as catalog's `CreateBook`). The handler calls `RequireRole(ctx, "admin")` before executing.

**Implementation layers:**
- `repository.UserRepository.List() ([]model.User, error)` — new method, `SELECT * FROM users ORDER BY created_at DESC`
- `service.AuthService.ListUsers(ctx) ([]model.User, error)` — calls repository, checks admin role
- `handler.AuthHandler.ListUsers(ctx, req) (*ListUsersResponse, error)` — calls service, maps to proto

### 2.2 Reservation Service — `ListAllReservations`

**File:** `proto/reservation/v1/reservation.proto`

```protobuf
rpc ListAllReservations(ListAllReservationsRequest) returns (ListAllReservationsResponse);

message ListAllReservationsRequest {}

message ListAllReservationsResponse {
  repeated ReservationDetail reservations = 1;
}

message ReservationDetail {
  string id = 1;
  string book_id = 2;
  string user_id = 3;
  string status = 4;
  string book_title = 5;
  string user_email = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp returned_at = 8;
}
```

**Auth:** Requires admin role. Same `RequireRole(ctx, "admin")` pattern.

**Denormalization:** The reservation handler resolves `book_title` by calling the catalog service's `GetBook` RPC (client already exists — used for availability checks). It resolves `user_email` by calling the auth service's `GetUser` RPC. This adds a new gRPC client dependency: reservation service needs an auth service client.

**Implementation layers:**
- `repository.ReservationRepository.ListAll() ([]model.Reservation, error)` — new method, `SELECT * FROM reservations ORDER BY created_at DESC`
- `service.ReservationService.ListAllReservations(ctx) ([]ReservationDetail, error)` — calls repository, resolves names via gRPC clients
- `handler.ReservationHandler.ListAllReservations(ctx, req) (*ListAllReservationsResponse, error)` — calls service, maps to proto

**New dependency:** The reservation service's `cmd/main.go` needs a gRPC client connection to the auth service. Add `AUTH_GRPC_ADDR` env var (defaulting to `auth:50051`), matching the pattern used for `CATALOG_GRPC_ADDR`.

**Docker Compose update:** Add `AUTH_GRPC_ADDR` to the reservation service's environment in `deploy/docker-compose.yml`.

---

## 3. Admin Dashboard (Gateway)

### 3.1 New Routes

Added to `services/gateway/cmd/main.go`:

```go
mux.HandleFunc("GET /admin", srv.AdminDashboard)
mux.HandleFunc("GET /admin/users", srv.AdminUserList)
mux.HandleFunc("GET /admin/reservations", srv.AdminReservationList)
```

All use the existing `requireAdmin` guard.

### 3.2 New Handlers

**File:** `services/gateway/internal/handler/admin.go` (new file)

- `AdminDashboard` — renders `admin_dashboard.html`, no gRPC calls
- `AdminUserList` — calls `auth.ListUsers`, renders `admin_users.html`
- `AdminReservationList` — calls `reservation.ListAllReservations`, renders `admin_reservations.html`

### 3.3 New Templates

- `admin_dashboard.html` — card links to Users, Reservations, and Books management
- `admin_users.html` — table: email, name, role, joined date
- `admin_reservations.html` — table: user email, book title, status, created/returned dates

All extend the existing `base.html` layout.

### 3.4 Navigation Update

Update the base template's nav partial (`templates/partials/nav.html` or equivalent) to add an "Admin" link visible only when the user has admin role. Points to `/admin`.

---

## 4. Catalog Seed CLI

**Location:** `services/catalog/cmd/seed/main.go`

**Module:** Lives in the catalog service module.

**Behavior:**
1. Reads `--auth-addr` (default `localhost:50051`), `--catalog-addr` (default `localhost:50052`), `--email`, `--password` flags
2. Calls `auth.Login` via gRPC to obtain a JWT token
3. Reads the fixture file `services/catalog/cmd/seed/books.json`
4. For each book, calls `catalog.CreateBook` with the JWT in gRPC metadata (using `metadata.AppendToOutgoingContext`)
5. If a book already exists (`AlreadyExists` error), logs "skipped" and continues
6. Logs a summary (N created, M skipped)

**Fixture file:** `services/catalog/cmd/seed/books.json` — ~15-20 books with title, author, isbn, genre, description, published_year. Spanning fiction, science, history, and technology genres.

**Why gRPC instead of direct DB:** Exercises the full system path. Each `CreateBook` call triggers a Kafka `book.created` event, which the search service consumes to index the book. No separate search seeding needed.

**Usage:**
```bash
go run services/catalog/cmd/seed/main.go \
  --auth-addr localhost:50051 \
  --catalog-addr localhost:50052 \
  --email admin@example.com \
  --password secret
```

---

## 5. Documentation

### 5.1 New Chapter 6: "Admin & Developer Tooling"

**Sections:**
- **6.1 Admin CLI** — motivation (chicken-and-egg problem), building `cmd/admin`, direct DB access pattern, bcrypt reuse, testing it
- **6.2 Admin Dashboard** — new proto RPCs (`ListUsers`, `ListAllReservations`), buf generate, repository/service/handler layers, gateway routes, templates, denormalization trade-offs
- **6.3 Catalog Seed CLI** — building `cmd/seed`, gRPC client auth pattern, JSON fixture design, idempotency, verifying Kafka events flow through
- **6.4 Putting It Together** — end-to-end walkthrough: create admin, seed catalog, browse the UI, check admin dashboard

### 5.2 Chapter Renumbering

| Current | New | Title |
|---------|-----|-------|
| — | Ch.6 | Admin & Developer Tooling (NEW) |
| Ch.6 | Ch.7 | Event-Driven Architecture |
| Ch.7 | Ch.8 | Full-Text Search |
| Ch.8 | Ch.9 | Observability |
| Ch.9 | Ch.10 | CI/CD |
| Ch.10 | Ch.11 | Testing Strategies |
| Ch.11 | Ch.12 | Kubernetes |
| Ch.12 | Ch.13 | Cloud Deployment |
| Ch.13 | Ch.14 | Production Hardening |

**Affected files:**
- `docs/src/SUMMARY.md` — renumber all entries, add Ch.6 sections
- All section files in `docs/src/ch06/` through `docs/src/ch13/` — rename directories to `ch07/`-`ch14/`, update internal cross-references
- All cross-chapter references throughout the book

### 5.3 Earlier Chapter Updates

- **Ch.4 (Interceptors, `docs/src/ch04/interceptors.md`):** Replace the bare "promote via SQL" instruction with: "We'll build a proper admin CLI for this in Chapter 6. For now, you can promote manually with SQL."
- **Ch.5 (Admin CRUD, `docs/src/ch05/admin-crud.md`):** Add closing note: "The admin routes are ready, but we don't yet have an admin account or sample books. Chapter 6 builds CLI tools to solve both."

### 5.4 Later Chapter Updates

- **Ch.7 (was Ch.6, Kafka):** Add note: "If you haven't already, seed the catalog with the seed CLI from Chapter 6 to generate events."
- **Ch.8-14:** Section number references only — no content changes beyond renumbering.

---

## Architecture Summary

```
                    ┌──────────────────┐
                    │   Admin CLI      │
                    │ cmd/admin/main.go│
                    └────────┬─────────┘
                             │ direct DB (GORM)
                        ┌────▼────┐
                   ┌────│  Auth   │◄────────┐
                   │    │ :50051  │          │
                   │    └────┬────┘          │
                   │         │               │
              ListUsers   GetUser      Login (seed CLI)
                   │         │               │
              ┌────▼─────────▼───┐    ┌──────▼──────────┐
              │     Gateway      │    │   Seed CLI       │
              │     :8080        │    │ cmd/seed/main.go │
              │  /admin          │    └──────┬───────────┘
              │  /admin/users    │           │ CreateBook (gRPC)
              │  /admin/reserv.  │      ┌────▼────┐
              └────────┬─────────┘      │ Catalog │
                       │                │ :50052  │
              ListAllReservations       └────┬────┘
                       │                     │
                  ┌────▼────────┐       book.created
                  │ Reservation │        (Kafka)
                  │   :50053    │            │
                  └─────────────┘       ┌────▼────┐
                                        │ Search  │
                                        │ :50054  │
                                        └─────────┘
```

## File Changes Summary

**New files:**
- `services/auth/cmd/admin/main.go`
- `services/catalog/cmd/seed/main.go`
- `services/catalog/cmd/seed/books.json`
- `services/gateway/internal/handler/admin.go`
- `services/gateway/templates/admin_dashboard.html`
- `services/gateway/templates/admin_users.html`
- `services/gateway/templates/admin_reservations.html`
- `docs/src/ch06/` (new directory, 4 section files + index)

**Modified files:**
- `proto/auth/v1/auth.proto` — add `ListUsers` RPC + messages
- `proto/reservation/v1/reservation.proto` — add `ListAllReservations` RPC + messages
- `gen/` — regenerated from proto changes
- `services/auth/internal/repository/user.go` — add `List()` method
- `services/auth/internal/service/auth.go` — add `ListUsers()` method
- `services/auth/internal/handler/auth.go` — add `ListUsers` handler
- `services/reservation/internal/repository/reservation.go` — add `ListAll()` method
- `services/reservation/internal/service/reservation.go` — add `ListAllReservations()` method
- `services/reservation/internal/handler/reservation.go` — add `ListAllReservations` handler
- `services/reservation/cmd/main.go` — add auth gRPC client connection
- `services/gateway/cmd/main.go` — add 3 new routes
- `services/gateway/templates/partials/nav.html` — add Admin link
- `deploy/docker-compose.yml` — add `AUTH_GRPC_ADDR` to reservation service
- `docs/src/SUMMARY.md` — renumber, add Ch.6 entries
- `docs/src/ch04/interceptors.md` — add forward reference
- `docs/src/ch05/admin-crud.md` — add closing note
- `docs/src/ch06/` through `docs/src/ch13/` — rename to `ch07/`-`ch14/`, update cross-refs
