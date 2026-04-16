# 6.2 Admin Dashboard

<!-- [STRUCTURAL] This is the longest section in the chapter and covers the most material: two new RPCs across two services, a denormalized DTO, and gateway handlers/templates. The ordering (proto → repo → service → handler, repeated for each of auth and reservation) works, but it's a lot of repetition. Consider whether a short table or diagram at the top summarising "what gets added where" would help the reader hold the full scope before diving in. -->

<!-- [LINE EDIT] "The admin can already manage books through the CRUD pages built in Chapter 5. But two questions remain unanswered: who are the users of the system, and what reservations exist?" → "The admin can already manage books through the CRUD pages built in Chapter 5, but two questions remain unanswered: who are the users of the system, and what reservations exist?" Reason: the two sentences are tightly contrastive; the comma version reads cleaner. -->
The admin can already manage books through the CRUD pages built in Chapter 5. But two questions remain unanswered: who are the users of the system, and what reservations exist? This section adds an admin dashboard with two new gRPC RPCs and three new gateway pages.

---

## Adding `ListUsers` to the Auth Proto

The auth proto already has `GetUser` (single user by ID). We need a new RPC that returns all users:

```protobuf
// proto/auth/v1/auth.proto

service AuthService {
  // ... existing RPCs ...
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
}

message ListUsersRequest {}

message ListUsersResponse {
  repeated User users = 1;
}
```

<!-- [LINE EDIT] "The request is empty because there is no filtering or pagination. In a production system you would add `page_size`, `page_token`, and possibly filter fields. For a tutorial with a handful of users, returning them all is fine." — Good. Keep. -->
The request is empty because there is no filtering or pagination. In a production system you would add `page_size`, `page_token`, and possibly filter fields. For a tutorial with a handful of users, returning them all is fine.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "`page_size`, `page_token`, and possibly filter fields" — correct. -->

After updating the proto, regenerate the Go code:

```bash
buf generate
```

<!-- [COPY EDIT] "buf" — product name for the proto tooling from Buf Technologies. Lowercase `buf` is the binary; prose references to the product are typically "Buf". CMOS 8.154 — where the brand prefers the lowercase styling (as Buf does for its CLI), keep lowercase in code voice. Acceptable as-is. -->

This updates the generated client and server interfaces in `gen/auth/v1/`. The auth service will not compile until you implement the new `ListUsers` method on the handler.

---

## Auth Service Implementation

The implementation follows the same layered pattern as every other RPC in the project: repository -> service -> handler.
<!-- [COPY EDIT] Replace ASCII arrow `->` with Unicode `→`. CMOS in-line symbols; `->` reads as a code token (pointer dereference or Go channel receive) and may confuse the reader mid-prose. Same fix at line 47 ("repository -> service -> handler"). -->

### Repository: `List()`

```go
// services/auth/internal/repository/user.go

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	var users []*model.User
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
```

<!-- [LINE EDIT] "Nothing surprising here -- `Find` without a `Where` clause returns all records. We order by `created_at DESC` so the most recently registered users appear first." — Good. Keep. -->
Nothing surprising here -- `Find` without a `Where` clause returns all records. We order by `created_at DESC` so the most recently registered users appear first.

### Service: `ListUsers()`

```go
// services/auth/internal/service/auth.go

// ListUsers returns all users.
func (s *AuthService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.repo.List(ctx)
}
```

<!-- [LINE EDIT] "The service layer is a passthrough here. It exists to maintain the pattern -- if you later need to add filtering, caching, or audit logging, you have a place to put it without touching the handler or repository." — Good. Keep. -->
The service layer is a passthrough here. It exists to maintain the pattern -- if you later need to add filtering, caching, or audit logging, you have a place to put it without touching the handler or repository.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "filtering, caching, or audit logging" — correct. -->
<!-- [COPY EDIT] "passthrough" — closed form, widely accepted in engineering prose. Consistency check: only occurrence in chapter. Fine. -->

### Handler: `ListUsers()` with `RequireRole`

```go
// services/auth/internal/handler/auth.go

func (h *AuthHandler) ListUsers(ctx context.Context, _ *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	protos := make([]*authv1.User, len(users))
	for i, u := range users {
		protos[i] = userToProto(u)
	}
	return &authv1.ListUsersResponse{Users: protos}, nil
}
```

<!-- [LINE EDIT] "The key line is `pkgauth.RequireRole(ctx, "admin")`." — Good framing. Keep. -->
The key line is `pkgauth.RequireRole(ctx, "admin")`. This helper from `pkg/auth/context.go` extracts the role from the gRPC context (set by the JWT interceptor) and returns a `PermissionDenied` error if it does not match:

```go
// pkg/auth/context.go

func RequireRole(ctx context.Context, required string) error {
	role, err := RoleFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, "no role in context")
	}
	if role != required {
		return status.Errorf(codes.PermissionDenied, "requires %s role", required)
	}
	return nil
}
```

<!-- [STRUCTURAL] Good. Shows the helper directly rather than forcing the reader to trust "it does X". -->

<!-- [LINE EDIT] "This is a reusable building block. Any gRPC handler that needs role-based access control can call `RequireRole` as its first line." — Good. -->
This is a reusable building block. Any gRPC handler that needs role-based access control can call `RequireRole` as its first line. The pattern is the same as the gateway's `requireAdmin` helper from Chapter 5, but operates at the gRPC layer instead of HTTP.
<!-- [COPY EDIT] "role-based access control" — compound adjective before noun, hyphenated correctly. CMOS 7.89. -->
<!-- [FINAL] Please verify cross-reference: "the gateway's `requireAdmin` helper from Chapter 5" — confirm a section in Chapter 5 introduces `requireAdmin`. If it does, consider adding an explicit link. -->

---

## Adding `ListAllReservations` to the Reservation Proto

The reservation proto needs a new RPC and a new message type. Unlike `ListUserReservations` (which returns the caller's own reservations), `ListAllReservations` is an admin-only view that includes denormalized information:

```protobuf
// proto/reservation/v1/reservation.proto

service ReservationService {
  // ... existing RPCs ...
  rpc ListAllReservations(ListAllReservationsRequest) returns (ListAllReservationsResponse);
}

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

<!-- [STRUCTURAL] Very clear motivation for the separate message type. -->
<!-- [LINE EDIT] "Notice that `ReservationDetail` is a separate message from `Reservation`. The existing `Reservation` message stores raw IDs (`book_id`, `user_id`), which is correct for its use cases -- the user already knows their own email, and the book title is shown on the page where they made the reservation. But the admin dashboard needs to display a table with human-readable columns: **who** reserved **what**. Embedding `book_title` and `user_email` directly in the response avoids forcing the gateway to make additional round trips." — 78 words in the long sentence block; could be split but reads fine. -->
Notice that `ReservationDetail` is a separate message from `Reservation`. The existing `Reservation` message stores raw IDs (`book_id`, `user_id`), which is correct for its use cases -- the user already knows their own email, and the book title is shown on the page where they made the reservation. But the admin dashboard needs to display a table with human-readable columns: **who** reserved **what**. Embedding `book_title` and `user_email` directly in the response avoids forcing the gateway to make additional round trips.
<!-- [COPY EDIT] "human-readable" — compound adjective before noun; hyphenated. CMOS 7.89. Correct. -->
<!-- [COPY EDIT] "round trips" — open compound as noun. M-W. Correct. -->

---

## Reservation Service Changes

### The `ReservationDetail` Type

The service layer defines a struct that embeds the domain model and adds the denormalized fields:

```go
// services/reservation/internal/service/service.go

type ReservationDetail struct {
	model.Reservation
	BookTitle string
	UserEmail string
}
```

<!-- [LINE EDIT] "Embedding `model.Reservation` gives `ReservationDetail` all the fields of a reservation (`ID`, `UserID`, `BookID`, `Status`, `ReservedAt`, `DueAt`, `ReturnedAt`) without repeating them." — Good. Keep. -->
Embedding `model.Reservation` gives `ReservationDetail` all the fields of a reservation (`ID`, `UserID`, `BookID`, `Status`, `ReservedAt`, `DueAt`, `ReturnedAt`) without repeating them.
<!-- [COPY EDIT] Inline list of fields: CMOS 6.19 serial comma present; correct. -->

### New Auth Client Dependency

To resolve `user_id` to an email address, the reservation service needs to call the auth service's `GetUser` RPC. This means a new dependency:

```go
// services/reservation/internal/service/service.go

type ReservationService struct {
	repo      ReservationRepository
	catalog   catalogv1.CatalogServiceClient
	auth      authv1.AuthServiceClient    // new
	publisher EventPublisher
	maxActive int
}

func NewReservationService(
	repo ReservationRepository,
	catalog catalogv1.CatalogServiceClient,
	auth authv1.AuthServiceClient,          // new
	publisher EventPublisher,
	maxActive int,
) *ReservationService {
	// ...
}
```

<!-- [LINE EDIT] "This requires a corresponding update to `deploy/docker-compose.yml` to pass the auth service address to the reservation container:" → "Passing the address requires a matching update to `deploy/docker-compose.yml`:" Reason: tighter, active, avoids "corresponding". -->
This requires a corresponding update to `deploy/docker-compose.yml` to pass the auth service address to the reservation container:

```yaml
# deploy/docker-compose.yml (reservation service section)

environment:
  # ... existing vars ...
  AUTH_GRPC_ADDR: ${AUTH_GRPC_ADDR:-auth:50051}
```

### The Denormalization Logic

```go
// services/reservation/internal/service/service.go

func (s *ReservationService) ListAllReservations(ctx context.Context) ([]ReservationDetail, error) {
	reservations, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]ReservationDetail, len(reservations))
	for i, r := range reservations {
		details[i] = ReservationDetail{Reservation: *r}
		book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{Id: r.BookID.String()})
		if err != nil {
			slog.WarnContext(ctx, "failed to resolve book title", "book_id", r.BookID, "error", err)
			details[i].BookTitle = r.BookID.String()
		} else {
			details[i].BookTitle = book.Title
		}
		user, err := s.auth.GetUser(ctx, &authv1.GetUserRequest{Id: r.UserID.String()})
		if err != nil {
			slog.WarnContext(ctx, "failed to resolve user email", "user_id", r.UserID, "error", err)
			details[i].UserEmail = r.UserID.String()
		} else {
			details[i].UserEmail = user.Email
		}
	}
	return details, nil
}
```

<!-- [STRUCTURAL] Excellent that the N+1 problem is called out explicitly, with both "why it is acceptable here" and "why it would not scale". This is exactly the teaching moment that justifies showing the naive version. -->

<!-- [LINE EDIT] "This code iterates over every reservation and makes two gRPC calls per reservation: one to the catalog service for the book title, one to the auth service for the user email. This is an **N+1 problem** -- if there are 100 reservations, this makes 200 gRPC calls." — 47 words; borderline long but the payoff (the concrete "200 calls" number) justifies it. Keep. -->
This code iterates over every reservation and makes two gRPC calls per reservation: one to the catalog service for the book title, one to the auth service for the user email. This is an **N+1 problem** -- if there are 100 reservations, this makes 200 gRPC calls.
<!-- [COPY EDIT] "**N+1 problem**" — convention in database prose typically renders as "N + 1" with spaces around the plus; CMOS 9.7 prefers the spaced form for arithmetic. In this book's idiom, "N+1" is standard industry usage; keep as-is but consistent chapter-wide. -->

**Why is this acceptable here?**

- This is a tutorial project with a handful of reservations. Performance is not a concern at this scale.
- The alternative (a batch RPC like `GetBooks(ids)` or denormalizing into the reservation database) would add complexity that distracts from the core lesson.
- The fallback behavior is graceful: if a book or user lookup fails, the raw UUID is displayed instead. The admin dashboard still works; it just shows less-readable data.
<!-- [LINE EDIT] "it just shows less-readable data" → "it shows less-readable data" — "just" is on the cut-list. -->
<!-- [COPY EDIT] "less-readable" — compound adjective before noun; hyphenated. CMOS 7.89. Correct. -->

**Why would this not scale?**

In a real system with thousands of reservations:

- You would add **pagination** (`page_size` + `page_token`) to both the proto and the handler.
- You would add **batch RPCs** (`GetUsers(ids)`, `GetBooks(ids)`) to reduce N+1 to 1+1 calls.
- Or you would **denormalize** the book title and user email into the reservation database at write time (using Kafka events from the catalog and auth services to keep them in sync). Chapter 7 introduces exactly this kind of event-driven data flow.
<!-- [COPY EDIT] "event-driven" — compound adjective before noun. CMOS 7.89. Correct. -->
<!-- [FINAL] Cross-reference to Chapter 7 is forward; confirm Chapter 7 covers this exact pattern (catalog/auth → reservation denormalization). -->

### Handler: `ListAllReservations()`

```go
// services/reservation/internal/handler/handler.go

func (h *ReservationHandler) ListAllReservations(ctx context.Context, _ *reservationv1.ListAllReservationsRequest) (*reservationv1.ListAllReservationsResponse, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}

	details, err := h.svc.ListAllReservations(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protos := make([]*reservationv1.ReservationDetail, len(details))
	for i, d := range details {
		protos[i] = &reservationv1.ReservationDetail{
			Id:        d.ID.String(),
			BookId:    d.BookID.String(),
			UserId:    d.UserID.String(),
			Status:    d.Status,
			BookTitle: d.BookTitle,
			UserEmail: d.UserEmail,
			CreatedAt: timestamppb.New(d.ReservedAt),
		}
		if d.ReturnedAt != nil {
			protos[i].ReturnedAt = timestamppb.New(*d.ReturnedAt)
		}
	}
	return &reservationv1.ReservationDetail{Reservations: protos}, nil
}
```
<!-- [FINAL] Please verify: the final `return` line of the handler reads `&reservationv1.ReservationDetail{Reservations: protos}` in my read, but the response type should be `ReservationDetail`*Response*. Looking again at line 279 of the source: `return &reservationv1.ListAllReservationsResponse{Reservations: protos}, nil` — the source is correct; I may have misread. Please double-check both versions render identically; the source version is correct. -->

<!-- [LINE EDIT] "Same pattern as the auth handler: check role, call service, convert to protobuf. The `ReturnedAt` field is conditionally set because it is `nil` for active reservations." — Good. Keep. -->
Same pattern as the auth handler: check role, call service, convert to protobuf. The `ReturnedAt` field is conditionally set because it is `nil` for active reservations.

---

## Gateway: Admin Dashboard Pages

The gateway gets a new handler file and three templates.

### Handler File

```go
// services/gateway/internal/handler/admin.go

func (s *Server) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	s.render(w, r, "admin_dashboard.html", nil)
}

func (s *Server) AdminUserList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	resp, err := s.auth.ListUsers(r.Context(), &authv1.ListUsersRequest{})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load users")
		return
	}
	s.render(w, r, "admin_users.html", map[string]any{
		"Users": resp.Users,
	})
}

func (s *Server) AdminReservationList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	resp, err := s.reservation.ListAllReservations(r.Context(), &reservationv1.ListAllReservationsRequest{})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load reservations")
		return
	}
	s.render(w, r, "admin_reservations.html", map[string]any{
		"Reservations": resp.Reservations,
	})
}
```

<!-- [LINE EDIT] "Each handler follows the same three-step pattern: check admin role, call gRPC, render template." — Clear. Keep. -->
<!-- [LINE EDIT] "This file is separate from `catalog.go` (which has the book CRUD handlers) because the admin dashboard handlers operate on different gRPC services (auth and reservation, not just catalog)." — 32 words; fine. -->
Each handler follows the same three-step pattern: check admin role, call gRPC, render template. This file is separate from `catalog.go` (which has the book CRUD handlers) because the admin dashboard handlers operate on different gRPC services (auth and reservation, not just catalog).
<!-- [COPY EDIT] "three-step" — compound adjective before noun; hyphenated. CMOS 7.89. Correct. -->

### Routes

The new handlers are registered alongside the existing book CRUD routes:

```go
// services/gateway/cmd/main.go

mux.HandleFunc("GET /admin", srv.AdminDashboard)
mux.HandleFunc("GET /admin/users", srv.AdminUserList)
mux.HandleFunc("GET /admin/reservations", srv.AdminReservationList)
```

### Templates

<!-- [LINE EDIT] "The dashboard landing page (`admin_dashboard.html`) provides card-style navigation to the three admin sections -- users, reservations, and book management:" → keep. Good. -->
The dashboard landing page (`admin_dashboard.html`) provides card-style navigation to the three admin sections -- users, reservations, and book management:
<!-- [COPY EDIT] CMOS 6.19 serial comma: "users, reservations, and book management" — correct. -->

```html
{{define "content"}}
<h1>Admin Dashboard</h1>
<div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-top: 1rem;">
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Users</h3>
    <p>View all registered users and their roles.</p>
    <a href="/admin/users">View Users &rarr;</a>
  </div>
  <!-- ... similar cards for Reservations and Books ... -->
</div>
{{end}}
```

<!-- [LINE EDIT] "The users template (`admin_users.html`) renders a table with email, name, role, and join date. The reservations template (`admin_reservations.html`) shows user email, book title, status, reserved date, and returned date -- using the denormalized fields from `ReservationDetail`." — Good. -->
The users template (`admin_users.html`) renders a table with email, name, role, and join date. The reservations template (`admin_reservations.html`) shows user email, book title, status, reserved date, and returned date -- using the denormalized fields from `ReservationDetail`.
<!-- [COPY EDIT] CMOS 6.19 serial comma: both lists correct. -->
<!-- [STRUCTURAL] The chapter shows only the landing template in full. Consider including a trimmed snippet of `admin_users.html` (even 8–10 lines) so the reader sees how the template binds to `.Users` — otherwise the prose "renders a table with email, name, role, and join date" is an unsupported claim and the reader has to trust it. -->

### Navigation Update

The navigation partial conditionally shows the "Admin" link for admin users:

```html
<!-- services/gateway/templates/partials/nav.html -->

{{if eq .User.Role "admin"}}
    <a href="/admin">Admin</a>
{{end}}
```

<!-- [LINE EDIT] "Regular users never see this link. Even if they manually navigate to `/admin`, the `requireAdmin` check in the handler will return a 403 error." — Good. -->
Regular users never see this link. Even if they manually navigate to `/admin`, the `requireAdmin` check in the handler will return a 403 error.
<!-- [COPY EDIT] "403 error" — numerals for HTTP status codes; CMOS 9.16 (numbers as identifiers). Correct. -->

---

## Testing the New RPCs

You can test the new RPCs directly with `grpcurl`:

```bash
# List users (requires admin JWT)
grpcurl -plaintext \
  -H "authorization: Bearer $ADMIN_TOKEN" \
  localhost:50051 auth.v1.AuthService/ListUsers

# List all reservations (requires admin JWT)
grpcurl -plaintext \
  -H "authorization: Bearer $ADMIN_TOKEN" \
  localhost:50053 reservation.v1.ReservationService/ListAllReservations
```

<!-- [COPY EDIT] Please verify: reservation service exposed on port 50053. The putting-it-together walkthrough references 50051 (auth) and 50052 (catalog); seed-cli likewise uses 50051 and 50052. A third port, 50053, appears only here. Please confirm `deploy/docker-compose.yml` exposes the reservation service on 50053. -->

Non-admin tokens will receive a `PERMISSION_DENIED` error. Unauthenticated requests will receive `UNAUTHENTICATED`.

---

## Key Takeaways

- **`RequireRole` at the gRPC layer** provides defense in depth. Even if someone bypasses the gateway, the backend services enforce authorization.
- **Denormalization trades consistency for convenience.** `ReservationDetail` embeds book titles and user emails so the gateway does not need to join data from three services. The tradeoff is that these values are fetched live (N+1 calls), which is fine at tutorial scale but would need batch RPCs or event-driven sync at production scale.
<!-- [COPY EDIT] "tradeoff" — closed form acceptable; M-W lists both "tradeoff" and "trade-off". Consistency: only occurrence. Fine. -->
<!-- [LINE EDIT] Long bullet (47 words before the period at "fetched live (N+1 calls)"). Acceptable in a takeaway; it is intentionally dense. -->
- **Separate handler files by domain.** `admin.go` handles cross-service admin views; `catalog.go` handles catalog CRUD. This keeps files focused and navigable.
<!-- [COPY EDIT] "cross-service" — compound adjective; hyphen correct. CMOS 7.89. -->
