# 7.5 Reservation Admin Dashboard

Chapter 6 created an admin dashboard with user visibility and links to catalog management. Now that the Reservation service exists, the dashboard can show activity across the system: which user reserved which book, the reservation status, and whether the book has been returned.

The new view crosses service boundaries. Reservation owns reservation records, Catalog owns book titles, and Auth owns user emails. The implementation in this section deliberately keeps Reservation as the API owner for the admin view, then has Reservation resolve the display fields it needs from the owning services.

---

## Adding `ListAllReservations` to the Reservation Proto

The reservation proto needs a new admin-only RPC and a response type with human-readable display fields. Unlike `ListUserReservations`, which returns the caller's own reservations, `ListAllReservations` returns system-wide rows for admin use:

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

`ReservationDetail` is separate from `Reservation`. The normal user-facing reservation response stores raw IDs, which is correct for its use case. The admin dashboard needs readable table columns: **who** reserved **what**. Embedding `book_title` and `user_email` directly in the response avoids forcing the gateway to coordinate extra calls to Catalog and Auth.

---

## Reservation Service Changes

### The `ReservationDetail` Type

The service layer defines a struct that embeds the reservation model and adds the denormalized fields:

```go
// services/reservation/internal/service/service.go

type ReservationDetail struct {
	model.Reservation
	BookTitle string
	UserEmail string
}
```

Embedding `model.Reservation` gives `ReservationDetail` all reservation fields (`ID`, `UserID`, `BookID`, `Status`, `ReservedAt`, `DueAt`, `ReturnedAt`) without repeating them.

### New Auth Client Dependency

To resolve `user_id` to an email address, the Reservation Service needs to call the Auth Service's `GetUser` RPC. This adds one dependency:

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

The reservation service's `main.go` establishes this gRPC connection alongside the existing catalog connection. Docker Compose also needs the Auth address in the reservation service environment:

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

This code iterates over every reservation and makes two gRPC calls per reservation: one to Catalog for the book title, one to Auth for the user email. This is an **N+1 problem**. If there are 100 reservations, this makes 200 gRPC calls.

For this tutorial checkpoint, the trade-off is acceptable:

- The local dataset is small.
- Batch RPCs such as `GetBooks(ids)` and `GetUsers(ids)` would add more API surface than this section needs.
- The fallback behavior is graceful: if a lookup fails, the raw UUID is displayed and the admin page still loads.

In a larger system, use pagination, batch RPCs, or a read model that denormalizes book titles and user emails into a reservation projection. Kafka gives you a natural path to that projection: Catalog and Auth publish facts, and a read-model consumer keeps the admin view current.

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
	return &reservationv1.ListAllReservationsResponse{Reservations: protos}, nil
}
```

The handler checks the admin role, calls the service, and converts the service model into protobuf messages. `ReturnedAt` is conditionally set because it is `nil` for active reservations.

---

## Gateway: Reservation Admin Page

Chapter 6 already added `AdminDashboard` and `AdminUserList`. Now add the reservation list handler to the same `admin.go` file:

```go
// services/gateway/internal/handler/admin.go

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

Register the route with the other admin routes:

```go
// services/gateway/cmd/main.go

mux.HandleFunc("GET /admin/reservations", srv.AdminReservationList)
```

Update `admin_dashboard.html` with a third card that links to `/admin/reservations`, and add `admin_reservations.html` to render user email, book title, status, reserved date, and returned date.

The gateway does not join data itself. It calls one reservation-owned RPC and renders the response. That keeps the cross-service composition decision in the backend service that owns the reservation use case.

---

## Testing the New RPC

You can test the admin RPC directly with `grpcurl`:

```bash
# List all reservations (requires admin JWT)
grpcurl -plaintext \
  -H "authorization: Bearer $ADMIN_TOKEN" \
  localhost:50053 reservation.v1.ReservationService/ListAllReservations
```

Non-admin tokens receive `PERMISSION_DENIED`. Unauthenticated requests receive `UNAUTHENTICATED`.

For a browser-level check:

1. Create an admin and seed the catalog using Chapter 6.
2. Register a regular user.
3. Reserve a book through the normal user flow from Section 7.4.
4. Log back in as the admin.
5. Open `/admin/reservations` and confirm the row shows the user email, book title, status, reserved date, and blank returned date.

---

## Key Takeaways

- **Admin views often need read models.** The reservation owner returns a display-oriented response instead of making the gateway join Auth, Catalog, and Reservation data.
- **N+1 calls are a conscious trade-off here.** The tutorial implementation is readable and correct for a tiny dataset; production systems should add pagination, batch RPCs, or an event-fed projection.
- **Authorization stays at both layers.** The gateway hides admin pages from regular users, and the Reservation Service still enforces `RequireRole("admin")`.
