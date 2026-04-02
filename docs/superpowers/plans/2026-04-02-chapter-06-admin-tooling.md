# Chapter 6: Admin & Developer Tooling — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin dashboard (user/reservation listing), admin CLI, catalog seed CLI, and a new Chapter 6 in the tutorial book — renumbering existing Ch.6-13 to Ch.7-14.

**Architecture:** Two CLI tools (admin account creator, catalog seeder) + two new gRPC RPCs (ListUsers, ListAllReservations) + three new gateway pages. The admin CLI connects directly to PostgreSQL; the seed CLI authenticates via gRPC and calls CreateBook. The reservation service gains an auth gRPC client to resolve user emails for the admin view.

**Tech Stack:** Go, gRPC/protobuf (buf), GORM, HTML templates, HTMX, Docker Compose

**Spec:** `docs/superpowers/specs/2026-04-02-chapter-06-admin-tooling-design.md`

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `services/auth/cmd/admin/main.go` | CLI to create admin accounts directly in PostgreSQL |
| `services/catalog/cmd/seed/main.go` | CLI to seed catalog via gRPC (login + CreateBook) |
| `services/catalog/cmd/seed/books.json` | Fixture data: ~15-20 sample books |
| `services/gateway/internal/handler/admin.go` | Gateway handlers: AdminDashboard, AdminUserList, AdminReservationList |
| `services/gateway/templates/admin_dashboard.html` | Admin landing page with links |
| `services/gateway/templates/admin_users.html` | User list table |
| `services/gateway/templates/admin_reservations.html` | All-reservations table |
| `docs/src/ch06/index.md` | Chapter 6 index page |
| `docs/src/ch06/admin-cli.md` | Section 6.1: Admin CLI |
| `docs/src/ch06/admin-dashboard.md` | Section 6.2: Admin Dashboard |
| `docs/src/ch06/seed-cli.md` | Section 6.3: Catalog Seed CLI |
| `docs/src/ch06/putting-it-together.md` | Section 6.4: End-to-end walkthrough |

### Modified Files
| File | Change |
|------|--------|
| `proto/auth/v1/auth.proto` | Add `ListUsers` RPC + request/response messages |
| `proto/reservation/v1/reservation.proto` | Add `ListAllReservations` RPC + `ReservationDetail` message |
| `gen/` (regenerated) | Run `buf generate` after proto changes |
| `services/auth/internal/repository/user.go` | Add `List()` method |
| `services/auth/internal/service/auth.go` | Add `List()` to interface, add `ListUsers()` method |
| `services/auth/internal/handler/auth.go` | Add `ListUsers` RPC handler |
| `services/auth/internal/handler/auth_test.go` | Add `TestAuthHandler_ListUsers_*` tests |
| `services/auth/internal/service/auth_test.go` | Add `TestAuthService_ListUsers` test |
| `services/reservation/internal/repository/repository.go` | Add `ListAll()` method |
| `services/reservation/internal/service/service.go` | Add `ListAll()` to interface, add `ListAllReservations()` method, add auth client |
| `services/reservation/internal/handler/handler.go` | Add `ListAllReservations` handler, update Service interface |
| `services/reservation/internal/handler/handler_test.go` | Add `TestReservationHandler_ListAllReservations_*` tests |
| `services/reservation/internal/service/service_test.go` | Add `TestReservationService_ListAllReservations` test |
| `services/reservation/cmd/main.go` | Add auth gRPC client, pass to service |
| `services/gateway/cmd/main.go` | Add 3 admin routes |
| `services/gateway/templates/partials/nav.html` | Add Admin link for admin users |
| `deploy/docker-compose.yml` | Add `AUTH_GRPC_ADDR` to reservation service |
| `deploy/.env` | Add `AUTH_GRPC_ADDR` default |
| `docs/src/SUMMARY.md` | Add Ch.6 entries, renumber Ch.7-14 |
| `docs/src/ch04/interceptors.md` | Add forward reference to Ch.6 |
| `docs/src/ch05/admin-crud.md` | Add closing note referencing Ch.6 |
| `docs/src/ch06/` → `docs/src/ch07/` | Rename directory (Event-Driven Architecture) |
| `docs/src/ch07/` → `docs/src/ch08/` | Rename directory (Full-Text Search) |
| `docs/src/ch08/` → `docs/src/ch09/` | Rename directory (Observability) |
| `docs/src/ch09/` → `docs/src/ch10/` | Rename directory (CI/CD) |
| `docs/src/ch10/` → `docs/src/ch11/` | Rename directory (Testing) |
| `docs/src/ch11/` → `docs/src/ch12/` | Rename directory (Kubernetes) |
| `docs/src/ch12/` → `docs/src/ch13/` | Rename directory (Cloud Deployment) |
| `docs/src/ch13/` → `docs/src/ch14/` | Rename directory (Production Hardening) |

---

## Task 1: Proto Definitions & Code Generation

Add `ListUsers` and `ListAllReservations` RPCs to the proto files and regenerate Go code.

**Files:**
- Modify: `proto/auth/v1/auth.proto`
- Modify: `proto/reservation/v1/reservation.proto`
- Regenerate: `gen/`

- [ ] **Step 1: Add ListUsers RPC to auth proto**

In `proto/auth/v1/auth.proto`, add the `ListUsers` RPC to the service block (after `CompleteOAuth2` on line 15), and add the request/response messages at the end of the file:

```protobuf
// Inside service AuthService block, add after CompleteOAuth2:
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);

// At end of file, add:
message ListUsersRequest {}

message ListUsersResponse {
  repeated User users = 1;
}
```

- [ ] **Step 2: Add ListAllReservations RPC to reservation proto**

In `proto/reservation/v1/reservation.proto`, add the `ListAllReservations` RPC to the service block (after `ListUserReservations` on line 12), and add the request/response messages and `ReservationDetail` message at the end of the file:

```protobuf
// Inside service ReservationService block, add after ListUserReservations:
  rpc ListAllReservations(ListAllReservationsRequest) returns (ListAllReservationsResponse);

// At end of file, add:
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

- [ ] **Step 3: Regenerate Go code**

Run: `cd proto && buf generate`

Verify the generated files exist:
- `gen/auth/v1/auth_grpc.pb.go` should contain `ListUsers` method on the interface
- `gen/reservation/v1/reservation_grpc.pb.go` should contain `ListAllReservations` method

- [ ] **Step 4: Commit**

```bash
git add proto/ gen/
git commit -m "feat: add ListUsers and ListAllReservations proto RPCs"
```

---

## Task 2: Auth Service — ListUsers Implementation

Add the `List()` repository method, service method, and gRPC handler for `ListUsers`.

**Files:**
- Modify: `services/auth/internal/repository/user.go`
- Modify: `services/auth/internal/service/auth.go`
- Modify: `services/auth/internal/handler/auth.go`
- Modify: `services/auth/internal/handler/auth_test.go`
- Modify: `services/auth/internal/service/auth_test.go`

- [ ] **Step 1: Add List() to repository**

In `services/auth/internal/repository/user.go`, add after the `Update` method:

```go
// List returns all users ordered by creation date (newest first).
func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	var users []*model.User
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
```

- [ ] **Step 2: Add List to the UserRepository interface in the service layer**

In `services/auth/internal/service/auth.go`, add `List` to the `UserRepository` interface (after the `Update` method, around line 29):

```go
	List(ctx context.Context) ([]*model.User, error)
```

- [ ] **Step 3: Add ListUsers method to AuthService**

In `services/auth/internal/service/auth.go`, add after the `GetUser` method:

```go
// ListUsers returns all registered users. Caller must verify admin role.
func (s *AuthService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.repo.List(ctx)
}
```

- [ ] **Step 4: Add ListUsers handler**

In `services/auth/internal/handler/auth.go`, add after the `GetUser` method:

```go
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

Note: Import `pkgauth "github.com/fesoliveira014/library-system/pkg/auth"` if not already imported. Check the existing imports in the file.

- [ ] **Step 5: Add List to the in-memory repo in handler tests**

In `services/auth/internal/handler/auth_test.go`, add a `List` method to the `inMemoryRepo` struct:

```go
func (r *inMemoryRepo) List(_ context.Context) ([]*model.User, error) {
	users := make([]*model.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	return users, nil
}
```

- [ ] **Step 6: Add handler test for ListUsers**

In `services/auth/internal/handler/auth_test.go`, add:

```go
func TestAuthHandler_ListUsers_Success(t *testing.T) {
	repo := newInMemoryRepo()
	svc := service.NewAuthService(repo, "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	// Register two users first
	_, err := h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "alice@example.com", Password: "password123", Name: "Alice",
	})
	require.NoError(t, err)
	_, err = h.Register(context.Background(), &authv1.RegisterRequest{
		Email: "bob@example.com", Password: "password123", Name: "Bob",
	})
	require.NoError(t, err)

	// ListUsers as admin
	ctx := pkgauth.ContextWithUser(context.Background(), uuid.New(), "admin")
	resp, err := h.ListUsers(ctx, &authv1.ListUsersRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Users, 2)
}

func TestAuthHandler_ListUsers_NonAdmin(t *testing.T) {
	svc := service.NewAuthService(newInMemoryRepo(), "test-secret", "24h")
	h := handler.NewAuthHandler(svc)

	ctx := pkgauth.ContextWithUser(context.Background(), uuid.New(), "user")
	_, err := h.ListUsers(ctx, &authv1.ListUsersRequest{})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}
```

Ensure imports include `pkgauth "github.com/fesoliveira014/library-system/pkg/auth"`, `"google.golang.org/grpc/codes"`, `"google.golang.org/grpc/status"`, `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"`.

- [ ] **Step 7: Add List to the mock repo in service tests**

In `services/auth/internal/service/auth_test.go`, add a `List` method to the `mockUserRepo`:

```go
func (r *mockUserRepo) List(_ context.Context) ([]*model.User, error) {
	users := make([]*model.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	return users, nil
}
```

- [ ] **Step 8: Add service test for ListUsers**

In `services/auth/internal/service/auth_test.go`, add:

```go
func TestAuthService_ListUsers(t *testing.T) {
	svc := service.NewAuthService(newMockRepo(), "test-secret", "24h")

	// Register two users
	_, _, err := svc.Register(context.Background(), "alice@example.com", "pass123", "Alice")
	require.NoError(t, err)
	_, _, err = svc.Register(context.Background(), "bob@example.com", "pass123", "Bob")
	require.NoError(t, err)

	users, err := svc.ListUsers(context.Background())
	require.NoError(t, err)
	assert.Len(t, users, 2)
}
```

- [ ] **Step 9: Run tests**

Run: `cd services/auth && go test ./...`
Expected: All tests pass

- [ ] **Step 10: Commit**

```bash
git add services/auth/
git commit -m "feat(auth): add ListUsers RPC for admin user listing"
```

---

## Task 3: Reservation Service — ListAllReservations Implementation

Add `ListAll()` repository method, update service with auth client dependency, and add `ListAllReservations` gRPC handler with denormalized user/book info.

**Files:**
- Modify: `services/reservation/internal/repository/repository.go`
- Modify: `services/reservation/internal/service/service.go`
- Modify: `services/reservation/internal/handler/handler.go`
- Modify: `services/reservation/internal/handler/handler_test.go`
- Modify: `services/reservation/internal/service/service_test.go`
- Modify: `services/reservation/cmd/main.go`
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/.env`

- [ ] **Step 1: Add ListAll() to repository**

In `services/reservation/internal/repository/repository.go`, add after the `Update` method:

```go
// ListAll returns all reservations ordered by reserved_at (newest first).
func (r *ReservationRepository) ListAll(ctx context.Context) ([]*model.Reservation, error) {
	var reservations []*model.Reservation
	if err := r.db.WithContext(ctx).Order("reserved_at DESC").Find(&reservations).Error; err != nil {
		return nil, err
	}
	return reservations, err
}
```

- [ ] **Step 2: Add ListAll to the ReservationRepository interface**

In `services/reservation/internal/service/service.go`, add to the `ReservationRepository` interface (after `Update`, around line 23):

```go
	ListAll(ctx context.Context) ([]*model.Reservation, error)
```

- [ ] **Step 3: Add auth client to ReservationService**

In `services/reservation/internal/service/service.go`, update the struct and constructor:

Add import for `authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"`.

Update the struct (around line 38):
```go
type ReservationService struct {
	repo      ReservationRepository
	catalog   catalogv1.CatalogServiceClient
	auth      authv1.AuthServiceClient
	publisher EventPublisher
	maxActive int
}
```

Update the constructor:
```go
func NewReservationService(
	repo ReservationRepository,
	catalog catalogv1.CatalogServiceClient,
	auth authv1.AuthServiceClient,
	publisher EventPublisher,
	maxActive int,
) *ReservationService {
	return &ReservationService{
		repo:      repo,
		catalog:   catalog,
		auth:      auth,
		publisher: publisher,
		maxActive: maxActive,
	}
}
```

- [ ] **Step 4: Add ListAllReservations service method**

In `services/reservation/internal/service/service.go`, add a `ReservationDetail` type and the method:

```go
// ReservationDetail is an enriched reservation with denormalized user/book info.
type ReservationDetail struct {
	model.Reservation
	BookTitle string
	UserEmail string
}

// ListAllReservations returns all reservations with denormalized book titles and user emails.
// Caller must verify admin role.
func (s *ReservationService) ListAllReservations(ctx context.Context) ([]ReservationDetail, error) {
	reservations, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	details := make([]ReservationDetail, len(reservations))
	for i, r := range reservations {
		details[i] = ReservationDetail{Reservation: *r}

		// Resolve book title
		book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{Id: r.BookID.String()})
		if err != nil {
			slog.WarnContext(ctx, "failed to resolve book title", "book_id", r.BookID, "error", err)
			details[i].BookTitle = r.BookID.String()
		} else {
			details[i].BookTitle = book.Title
		}

		// Resolve user email
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

Add `"log/slog"` to imports if not present.

- [ ] **Step 5: Update the handler Service interface**

In `services/reservation/internal/handler/handler.go`, add to the `Service` interface:

```go
	ListAllReservations(ctx context.Context) ([]service.ReservationDetail, error)
```

Add import for `"github.com/fesoliveira014/library-system/services/reservation/internal/service"` if not present.

- [ ] **Step 6: Add ListAllReservations handler**

In `services/reservation/internal/handler/handler.go`, add:

```go
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
		pd := &reservationv1.ReservationDetail{
			Id:        d.ID.String(),
			BookId:    d.BookID.String(),
			UserId:    d.UserID.String(),
			Status:    d.Status,
			BookTitle: d.BookTitle,
			UserEmail: d.UserEmail,
			CreatedAt: timestamppb.New(d.ReservedAt),
		}
		if d.ReturnedAt != nil {
			pd.ReturnedAt = timestamppb.New(*d.ReturnedAt)
		}
		protos[i] = pd
	}
	return &reservationv1.ListAllReservationsResponse{Reservations: protos}, nil
}
```

Ensure `pkgauth "github.com/fesoliveira014/library-system/pkg/auth"` and `"google.golang.org/protobuf/types/known/timestamppb"` are imported.

- [ ] **Step 7: Update reservation service main.go**

In `services/reservation/cmd/main.go`:

Add auth gRPC client setup (follow the existing pattern for catalogConn). Add after the `catalogConn` setup:

```go
	authAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authAddr == "" {
		authAddr = "localhost:50051"
	}
	authConn, err := grpc.NewClient(authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("connect to auth service", "error", err)
		os.Exit(1)
	}
	defer authConn.Close()
	authClient := authv1.NewAuthServiceClient(authConn)
```

Update the `NewReservationService` call to pass `authClient`:

```go
reservationSvc := service.NewReservationService(repo, catalogClient, authClient, publisher, maxActive)
```

Add import: `authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"`.

- [ ] **Step 8: Update Docker Compose**

In `deploy/docker-compose.yml`, add `AUTH_GRPC_ADDR` to the reservation service environment (around line 128):

```yaml
      AUTH_GRPC_ADDR: ${AUTH_GRPC_ADDR:-auth:50051}
```

In `deploy/.env`, add:

```
AUTH_GRPC_ADDR=auth:50051
```

- [ ] **Step 9: Update handler test mock**

In `services/reservation/internal/handler/handler_test.go`, add to the `mockService` struct:

```go
	listAllFn func(ctx context.Context) ([]service.ReservationDetail, error)
```

And add the method:

```go
func (m *mockService) ListAllReservations(ctx context.Context) ([]service.ReservationDetail, error) {
	if m.listAllFn != nil {
		return m.listAllFn(ctx)
	}
	return nil, nil
}
```

- [ ] **Step 10: Add handler tests**

In `services/reservation/internal/handler/handler_test.go`, add:

```go
func TestReservationHandler_ListAllReservations_Success(t *testing.T) {
	now := time.Now()
	bookID := uuid.New()
	userID := uuid.New()

	mock := &mockService{
		listAllFn: func(_ context.Context) ([]service.ReservationDetail, error) {
			return []service.ReservationDetail{
				{
					Reservation: model.Reservation{
						ID: uuid.New(), UserID: userID, BookID: bookID,
						Status: "active", ReservedAt: now, DueAt: now.Add(14 * 24 * time.Hour),
					},
					BookTitle: "Test Book",
					UserEmail: "alice@example.com",
				},
			}, nil
		},
	}
	h := handler.NewReservationHandler(mock)
	ctx := pkgauth.ContextWithUser(context.Background(), uuid.New(), "admin")
	resp, err := h.ListAllReservations(ctx, &reservationv1.ListAllReservationsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Reservations, 1)
	assert.Equal(t, "Test Book", resp.Reservations[0].BookTitle)
	assert.Equal(t, "alice@example.com", resp.Reservations[0].UserEmail)
}

func TestReservationHandler_ListAllReservations_NonAdmin(t *testing.T) {
	h := handler.NewReservationHandler(&mockService{})
	ctx := pkgauth.ContextWithUser(context.Background(), uuid.New(), "user")
	_, err := h.ListAllReservations(ctx, &reservationv1.ListAllReservationsRequest{})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}
```

Ensure imports include `"time"`, `"google.golang.org/grpc/codes"`, `"google.golang.org/grpc/status"`, `service "github.com/fesoliveira014/library-system/services/reservation/internal/service"`, and `model "github.com/fesoliveira014/library-system/services/reservation/internal/model"`.

- [ ] **Step 11: Update service test mock repo**

In `services/reservation/internal/service/service_test.go`, add to the `mockRepo` struct:

```go
	listAllFn func(ctx context.Context) ([]*model.Reservation, error)
```

And add the method:

```go
func (m *mockRepo) ListAll(ctx context.Context) ([]*model.Reservation, error) {
	if m.listAllFn != nil {
		return m.listAllFn(ctx)
	}
	return nil, nil
}
```

- [ ] **Step 12: Add mock auth client for service tests**

In `services/reservation/internal/service/service_test.go`, add a mock auth client:

```go
type mockAuthClient struct {
	getUserFn func(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error)
}

func (m *mockAuthClient) Register(_ context.Context, _ *authv1.RegisterRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) Login(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, in, opts...)
	}
	return &authv1.User{Email: "unknown@example.com"}, nil
}
func (m *mockAuthClient) InitOAuth2(_ context.Context, _ *authv1.InitOAuth2Request, _ ...grpc.CallOption) (*authv1.InitOAuth2Response, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) CompleteOAuth2(_ context.Context, _ *authv1.CompleteOAuth2Request, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) ListUsers(_ context.Context, _ *authv1.ListUsersRequest, _ ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
```

Add imports: `authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"`.

- [ ] **Step 13: Update all existing NewReservationService calls in tests**

Search for all `service.NewReservationService(` calls in test files under `services/reservation/`. Each call needs the new `auth` parameter added after `catalog`. The new parameter should be `&mockAuthClient{}` (or `nil` if the test doesn't use ListAllReservations — but using the mock is safer).

Update each call from:
```go
service.NewReservationService(repo, catalog, publisher, maxActive)
```
to:
```go
service.NewReservationService(repo, catalog, &mockAuthClient{}, publisher, maxActive)
```

Check these files:
- `services/reservation/internal/service/service_test.go`
- `services/reservation/internal/handler/grpc_integration_test.go` (if exists)
- `services/reservation/internal/e2e/reservation_e2e_test.go` (if exists)

- [ ] **Step 14: Add service test for ListAllReservations**

In `services/reservation/internal/service/service_test.go`, add:

```go
func TestReservationService_ListAllReservations(t *testing.T) {
	bookID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	repo := &mockRepo{
		listAllFn: func(_ context.Context) ([]*model.Reservation, error) {
			return []*model.Reservation{
				{ID: uuid.New(), UserID: userID, BookID: bookID, Status: "active", ReservedAt: now, DueAt: now.Add(14 * 24 * time.Hour)},
			}, nil
		},
	}
	catalog := &mockCatalog{
		getBookFn: func(_ context.Context, req *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			return &catalogv1.Book{Id: req.Id, Title: "Test Book"}, nil
		},
	}
	auth := &mockAuthClient{
		getUserFn: func(_ context.Context, req *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.User, error) {
			return &authv1.User{Id: req.Id, Email: "alice@example.com"}, nil
		},
	}
	svc := service.NewReservationService(repo, catalog, auth, &mockPublisher{}, 5)
	details, err := svc.ListAllReservations(context.Background())
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.Equal(t, "Test Book", details[0].BookTitle)
	assert.Equal(t, "alice@example.com", details[0].UserEmail)
}
```

- [ ] **Step 15: Run tests**

Run: `cd services/reservation && go test ./...`
Expected: All tests pass (ignore integration tests behind build tags)

- [ ] **Step 16: Commit**

```bash
git add services/reservation/ deploy/docker-compose.yml deploy/.env
git commit -m "feat(reservation): add ListAllReservations RPC with denormalized user/book info"
```

---

## Task 4: Admin CLI Tool

Create the CLI that inserts admin accounts directly into PostgreSQL.

**Files:**
- Create: `services/auth/cmd/admin/main.go`

- [ ] **Step 1: Create the admin CLI**

Create `services/auth/cmd/admin/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
)

func main() {
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	name := flag.String("name", "", "admin display name (required)")
	flag.Parse()

	if *email == "" || *password == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "Usage: admin --email EMAIL --password PASSWORD --name NAME")
		fmt.Fprintln(os.Stderr, "Requires DATABASE_URL environment variable")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&model.User{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	hashStr := string(hash)

	var existing model.User
	result := db.Where("email = ?", *email).First(&existing)
	if result.Error == nil {
		// User exists — promote to admin
		existing.Role = "admin"
		existing.PasswordHash = &hashStr
		existing.Name = *name
		if err := db.Save(&existing).Error; err != nil {
			log.Fatalf("failed to update user: %v", err)
		}
		fmt.Printf("Updated existing user %s to admin role\n", *email)
		return
	}

	user := model.User{
		Email:        *email,
		PasswordHash: &hashStr,
		Name:         *name,
		Role:         "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}
	fmt.Printf("Created admin user: %s (%s)\n", *email, user.ID)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd services/auth && go build ./cmd/admin/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add services/auth/cmd/admin/
git commit -m "feat(auth): add admin CLI for creating admin accounts"
```

---

## Task 5: Catalog Seed CLI & Fixture Data

Create the seed CLI and JSON fixture file.

**Files:**
- Create: `services/catalog/cmd/seed/main.go`
- Create: `services/catalog/cmd/seed/books.json`

- [ ] **Step 1: Create the books.json fixture**

Create `services/catalog/cmd/seed/books.json`:

```json
[
  {
    "title": "The Go Programming Language",
    "author": "Alan Donovan & Brian Kernighan",
    "isbn": "9780134190440",
    "genre": "Technology",
    "description": "The authoritative resource for learning Go, covering the language's foundations and practical application.",
    "published_year": 2015,
    "total_copies": 3
  },
  {
    "title": "Designing Data-Intensive Applications",
    "author": "Martin Kleppmann",
    "isbn": "9781449373320",
    "genre": "Technology",
    "description": "A deep dive into the principles and practicalities of data systems, from databases to stream processing.",
    "published_year": 2017,
    "total_copies": 2
  },
  {
    "title": "Clean Code",
    "author": "Robert C. Martin",
    "isbn": "9780132350884",
    "genre": "Technology",
    "description": "A handbook of agile software craftsmanship with principles, patterns, and practices for writing clean code.",
    "published_year": 2008,
    "total_copies": 4
  },
  {
    "title": "To Kill a Mockingbird",
    "author": "Harper Lee",
    "isbn": "9780061120084",
    "genre": "Fiction",
    "description": "A classic novel of racial injustice and childhood innocence in the American South.",
    "published_year": 1960,
    "total_copies": 5
  },
  {
    "title": "1984",
    "author": "George Orwell",
    "isbn": "9780451524935",
    "genre": "Fiction",
    "description": "A dystopian novel exploring totalitarianism, surveillance, and the manipulation of truth.",
    "published_year": 1949,
    "total_copies": 4
  },
  {
    "title": "Dune",
    "author": "Frank Herbert",
    "isbn": "9780441013593",
    "genre": "Science Fiction",
    "description": "An epic tale of politics, religion, and ecology on the desert planet Arrakis.",
    "published_year": 1965,
    "total_copies": 3
  },
  {
    "title": "A Brief History of Time",
    "author": "Stephen Hawking",
    "isbn": "9780553380163",
    "genre": "Science",
    "description": "An accessible exploration of cosmology, black holes, and the nature of time.",
    "published_year": 1988,
    "total_copies": 2
  },
  {
    "title": "Sapiens: A Brief History of Humankind",
    "author": "Yuval Noah Harari",
    "isbn": "9780062316110",
    "genre": "History",
    "description": "A sweeping narrative of human history from the Stone Age to the twenty-first century.",
    "published_year": 2011,
    "total_copies": 3
  },
  {
    "title": "The Pragmatic Programmer",
    "author": "David Thomas & Andrew Hunt",
    "isbn": "9780135957059",
    "genre": "Technology",
    "description": "Timeless advice for software developers on craftsmanship, career growth, and pragmatic thinking.",
    "published_year": 2019,
    "total_copies": 3
  },
  {
    "title": "The Great Gatsby",
    "author": "F. Scott Fitzgerald",
    "isbn": "9780743273565",
    "genre": "Fiction",
    "description": "A portrait of the Jazz Age and the American Dream through the eyes of narrator Nick Carraway.",
    "published_year": 1925,
    "total_copies": 4
  },
  {
    "title": "Cosmos",
    "author": "Carl Sagan",
    "isbn": "9780345539434",
    "genre": "Science",
    "description": "A journey through the universe exploring the origins of life, the cosmos, and our place within it.",
    "published_year": 1980,
    "total_copies": 2
  },
  {
    "title": "The Art of War",
    "author": "Sun Tzu",
    "isbn": "9781590302255",
    "genre": "History",
    "description": "An ancient Chinese military treatise on strategy, tactics, and the philosophy of conflict.",
    "published_year": -500,
    "total_copies": 3
  },
  {
    "title": "Neuromancer",
    "author": "William Gibson",
    "isbn": "9780441569595",
    "genre": "Science Fiction",
    "description": "The pioneering cyberpunk novel that defined a genre and predicted the networked future.",
    "published_year": 1984,
    "total_copies": 2
  },
  {
    "title": "Site Reliability Engineering",
    "author": "Betsy Beyer et al.",
    "isbn": "9781491929124",
    "genre": "Technology",
    "description": "How Google runs production systems — principles and practices for large-scale reliability.",
    "published_year": 2016,
    "total_copies": 2
  },
  {
    "title": "Pride and Prejudice",
    "author": "Jane Austen",
    "isbn": "9780141439518",
    "genre": "Fiction",
    "description": "A witty exploration of manners, morality, and marriage in Regency-era England.",
    "published_year": 1813,
    "total_copies": 3
  },
  {
    "title": "The Selfish Gene",
    "author": "Richard Dawkins",
    "isbn": "9780198788607",
    "genre": "Science",
    "description": "A groundbreaking look at evolution from the gene's perspective, introducing the concept of memes.",
    "published_year": 1976,
    "total_copies": 2
  }
]
```

- [ ] **Step 2: Create the seed CLI**

Create `services/catalog/cmd/seed/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

type seedBook struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	Genre         string `json:"genre"`
	Description   string `json:"description"`
	PublishedYear int32  `json:"published_year"`
	TotalCopies   int32  `json:"total_copies"`
}

func main() {
	authAddr := flag.String("auth-addr", "localhost:50051", "auth service gRPC address")
	catalogAddr := flag.String("catalog-addr", "localhost:50052", "catalog service gRPC address")
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	booksFile := flag.String("books", "", "path to books JSON file (default: books.json next to this binary)")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: seed --email EMAIL --password PASSWORD [--auth-addr ADDR] [--catalog-addr ADDR] [--books FILE]")
		os.Exit(1)
	}

	if *booksFile == "" {
		*booksFile = "services/catalog/cmd/seed/books.json"
	}

	// 1. Read fixture data
	data, err := os.ReadFile(*booksFile)
	if err != nil {
		log.Fatalf("failed to read books file: %v", err)
	}
	var books []seedBook
	if err := json.Unmarshal(data, &books); err != nil {
		log.Fatalf("failed to parse books JSON: %v", err)
	}

	// 2. Connect to auth service and log in
	authConn, err := grpc.NewClient(*authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to auth service: %v", err)
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)
	loginResp, err := authClient.Login(context.Background(), &authv1.LoginRequest{
		Email:    *email,
		Password: *password,
	})
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	token := loginResp.Token
	fmt.Println("Logged in successfully")

	// 3. Connect to catalog service
	catalogConn, err := grpc.NewClient(*catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)

	// 4. Create books with auth token in metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	created, skipped := 0, 0
	for _, b := range books {
		_, err := catalogClient.CreateBook(ctx, &catalogv1.CreateBookRequest{
			Title:         b.Title,
			Author:        b.Author,
			Isbn:          b.ISBN,
			Genre:         b.Genre,
			Description:   b.Description,
			PublishedYear: b.PublishedYear,
			TotalCopies:   b.TotalCopies,
		})
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.AlreadyExists {
				fmt.Printf("  skipped (exists): %s\n", b.Title)
				skipped++
				continue
			}
			log.Fatalf("failed to create book %q: %v", b.Title, err)
		}
		fmt.Printf("  created: %s\n", b.Title)
		created++
	}

	fmt.Printf("\nDone: %d created, %d skipped\n", created, skipped)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd services/catalog && go build ./cmd/seed/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add services/catalog/cmd/seed/
git commit -m "feat(catalog): add seed CLI with sample book fixtures"
```

---

## Task 6: Gateway Admin Dashboard

Add admin dashboard handlers, templates, routes, and navigation link.

**Files:**
- Create: `services/gateway/internal/handler/admin.go`
- Create: `services/gateway/templates/admin_dashboard.html`
- Create: `services/gateway/templates/admin_users.html`
- Create: `services/gateway/templates/admin_reservations.html`
- Modify: `services/gateway/cmd/main.go`
- Modify: `services/gateway/templates/partials/nav.html`

- [ ] **Step 1: Create admin handler file**

Create `services/gateway/internal/handler/admin.go`:

```go
package handler

import (
	"net/http"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
)

// AdminDashboard renders the admin landing page.
func (s *Server) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	s.render(w, r, "admin_dashboard.html", nil)
}

// AdminUserList shows all registered users.
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

// AdminReservationList shows all reservations across all users.
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

- [ ] **Step 2: Create admin_dashboard.html template**

Create `services/gateway/templates/admin_dashboard.html`:

```html
{{define "title"}}Admin Dashboard{{end}}
{{define "content"}}
<h1>Admin Dashboard</h1>
<div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-top: 1rem;">
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Users</h3>
    <p>View all registered users and their roles.</p>
    <a href="/admin/users">View Users &rarr;</a>
  </div>
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Reservations</h3>
    <p>View all book reservations across all users.</p>
    <a href="/admin/reservations">View Reservations &rarr;</a>
  </div>
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Books</h3>
    <p>Add, edit, or remove books from the catalog.</p>
    <a href="/admin/books/new">Add Book &rarr;</a>
  </div>
</div>
{{end}}
```

- [ ] **Step 3: Create admin_users.html template**

Create `services/gateway/templates/admin_users.html`:

```html
{{define "title"}}Manage Users{{end}}
{{define "content"}}
<h1>Users</h1>
<p><a href="/admin">&larr; Admin Dashboard</a></p>
<table style="width: 100%; border-collapse: collapse; margin-top: 1rem;">
  <thead>
    <tr style="border-bottom: 2px solid #ddd; text-align: left;">
      <th style="padding: 0.5rem;">Email</th>
      <th style="padding: 0.5rem;">Name</th>
      <th style="padding: 0.5rem;">Role</th>
      <th style="padding: 0.5rem;">Joined</th>
    </tr>
  </thead>
  <tbody>
    {{range .Data.Users}}
    <tr style="border-bottom: 1px solid #eee;">
      <td style="padding: 0.5rem;">{{.Email}}</td>
      <td style="padding: 0.5rem;">{{.Name}}</td>
      <td style="padding: 0.5rem;">{{.Role}}</td>
      <td style="padding: 0.5rem;">{{.CreatedAt.AsTime.Format "2006-01-02"}}</td>
    </tr>
    {{else}}
    <tr><td colspan="4" style="padding: 0.5rem;">No users found.</td></tr>
    {{end}}
  </tbody>
</table>
{{end}}
```

- [ ] **Step 4: Create admin_reservations.html template**

Create `services/gateway/templates/admin_reservations.html`:

```html
{{define "title"}}Manage Reservations{{end}}
{{define "content"}}
<h1>All Reservations</h1>
<p><a href="/admin">&larr; Admin Dashboard</a></p>
<table style="width: 100%; border-collapse: collapse; margin-top: 1rem;">
  <thead>
    <tr style="border-bottom: 2px solid #ddd; text-align: left;">
      <th style="padding: 0.5rem;">User</th>
      <th style="padding: 0.5rem;">Book</th>
      <th style="padding: 0.5rem;">Status</th>
      <th style="padding: 0.5rem;">Reserved</th>
      <th style="padding: 0.5rem;">Returned</th>
    </tr>
  </thead>
  <tbody>
    {{range .Data.Reservations}}
    <tr style="border-bottom: 1px solid #eee;">
      <td style="padding: 0.5rem;">{{.UserEmail}}</td>
      <td style="padding: 0.5rem;">{{.BookTitle}}</td>
      <td style="padding: 0.5rem;">{{.Status}}</td>
      <td style="padding: 0.5rem;">{{.CreatedAt.AsTime.Format "2006-01-02"}}</td>
      <td style="padding: 0.5rem;">{{if .ReturnedAt}}{{.ReturnedAt.AsTime.Format "2006-01-02"}}{{else}}—{{end}}</td>
    </tr>
    {{else}}
    <tr><td colspan="5" style="padding: 0.5rem;">No reservations found.</td></tr>
    {{end}}
  </tbody>
</table>
{{end}}
```

- [ ] **Step 5: Add admin routes to gateway main.go**

In `services/gateway/cmd/main.go`, add the three admin routes after the existing admin book routes (after line 148, after `AdminBookDelete`):

```go
	mux.HandleFunc("GET /admin", srv.AdminDashboard)
	mux.HandleFunc("GET /admin/users", srv.AdminUserList)
	mux.HandleFunc("GET /admin/reservations", srv.AdminReservationList)
```

**Important:** These must be placed BEFORE the existing `GET /admin/books/new` route in the mux, otherwise the more specific `/admin/books/new` pattern will never match if Go's mux tries `/admin` first. Actually, with Go 1.22+ method-aware routing, more specific patterns take precedence regardless of order. But for readability, place these new routes right before the existing admin book routes.

- [ ] **Step 6: Update nav partial**

In `services/gateway/templates/partials/nav.html`, find the admin role check section (around line 20-21 where it shows the "Add Book" link). Replace or extend the admin section to include dashboard link:

Find the existing admin link block (inside `{{if eq .User.Role "admin"}}`) and add the dashboard link. The exact edit depends on the current nav structure — add `<a href="/admin">Admin</a>` alongside or replacing the existing `<a href="/admin/books/new">Add Book</a>` link.

- [ ] **Step 7: Verify gateway compiles**

Run: `cd services/gateway && go build ./cmd/...`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add services/gateway/
git commit -m "feat(gateway): add admin dashboard with user and reservation views"
```

---

## Task 7: Chapter Renumbering — Rename Directories

Rename existing chapter directories from ch06-ch13 to ch07-ch14 (must be done in reverse order to avoid collisions).

**Files:**
- Rename: `docs/src/ch06/` → `docs/src/ch07/`
- Rename: `docs/src/ch07/` → `docs/src/ch08/`
- Rename: `docs/src/ch08/` → `docs/src/ch09/`
- Rename: `docs/src/ch09/` → `docs/src/ch10/`
- Rename: `docs/src/ch10/` → `docs/src/ch11/`
- Rename: `docs/src/ch11/` → `docs/src/ch12/`
- Rename: `docs/src/ch12/` → `docs/src/ch13/`
- Rename: `docs/src/ch13/` → `docs/src/ch14/`

- [ ] **Step 1: Rename directories in reverse order**

```bash
cd docs/src
git mv ch13 ch14
git mv ch12 ch13
git mv ch11 ch12
git mv ch10 ch11
git mv ch09 ch10
git mv ch08 ch09
git mv ch07 ch08
git mv ch06 ch07
```

- [ ] **Step 2: Update SUMMARY.md**

Replace all chapter references in `docs/src/SUMMARY.md`:
- `./ch06/` → `./ch07/` and "Chapter 6:" → "Chapter 7:"
- `./ch07/` → `./ch08/` and "Chapter 7:" → "Chapter 8:"
- `./ch08/` → `./ch09/` and "Chapter 8:" → "Chapter 9:"
- `./ch09/` → `./ch10/` and "Chapter 9:" → "Chapter 10:"
- `./ch10/` → `./ch11/` and "Chapter 10:" → "Chapter 11:"
- `./ch11/` → `./ch12/` and "Chapter 11:" → "Chapter 12:"
- `./ch12/` → `./ch13/` and "Chapter 12:" → "Chapter 13:"
- `./ch13/` → `./ch14/` and "Chapter 13:" → "Chapter 14:"

Also update section number prefixes (e.g., `6.1` → `7.1`, `7.1` → `8.1`, etc.).

- [ ] **Step 3: Add new Chapter 6 entries to SUMMARY.md**

Insert after the Chapter 5 entries:

```markdown
- [Chapter 6: Admin & Developer Tooling](./ch06/index.md)
  - [6.1 Admin CLI](./ch06/admin-cli.md)
  - [6.2 Admin Dashboard](./ch06/admin-dashboard.md)
  - [6.3 Catalog Seed CLI](./ch06/seed-cli.md)
  - [6.4 Putting It Together](./ch06/putting-it-together.md)
```

- [ ] **Step 4: Update cross-chapter references in all chapter files**

Search through all `.md` files under `docs/src/` for references like "Chapter 6", "Chapter 7", etc., and update them to the new numbering. Also update any `ch06/`, `ch07/` path references within the chapter content.

Key patterns to search and replace:
- "Chapter 6" → "Chapter 7" (in files other than the new ch06)
- "Chapter 7" → "Chapter 8"
- "Chapter 8" → "Chapter 9"
- "Chapter 9" → "Chapter 10"
- "Chapter 10" → "Chapter 11"
- "Chapter 11" → "Chapter 12"
- "Chapter 12" → "Chapter 13"
- "Chapter 13" → "Chapter 14"

And section numbers: "6.1" → "7.1", "7.1" → "8.1", etc.

**Important:** Do this in reverse order (13→14 first, then 12→13, etc.) to avoid double-renumbering.

- [ ] **Step 5: Update forward references in earlier chapters**

In `docs/src/ch04/interceptors.md`: find the "promote via SQL" instruction and add a note: "We'll build a proper CLI for this in Chapter 6. For now, promote manually with SQL:"

In `docs/src/ch05/admin-crud.md`: add at the end of the chapter: "The admin routes are ready, but we don't yet have an admin account or sample books. Chapter 6 builds CLI tools to solve both."

- [ ] **Step 6: Add note to new Ch.7 (was Ch.6, Kafka)**

In `docs/src/ch07/index.md` (the renamed event-driven architecture chapter), add a note near the top: "If you haven't already, create an admin account and seed the catalog using the CLI tools from Chapter 6."

- [ ] **Step 7: Commit**

```bash
git add docs/src/
git commit -m "docs: renumber chapters 6-13 to 7-14, add Chapter 6 placeholder"
```

---

## Task 8: Write Chapter 6 Documentation

Write the four sections of the new Chapter 6.

**Files:**
- Create: `docs/src/ch06/index.md`
- Create: `docs/src/ch06/admin-cli.md`
- Create: `docs/src/ch06/admin-dashboard.md`
- Create: `docs/src/ch06/seed-cli.md`
- Create: `docs/src/ch06/putting-it-together.md`

- [ ] **Step 1: Create chapter index**

Create `docs/src/ch06/index.md` with an overview of the chapter: motivation (chicken-and-egg problem with admin accounts and empty catalogs), what will be built (admin CLI, admin dashboard with new RPCs, seed CLI), and a brief section outline.

- [ ] **Step 2: Write section 6.1 — Admin CLI**

Create `docs/src/ch06/admin-cli.md` covering:
- The problem: no admin accounts exist after a fresh deployment
- Design decision: direct DB access vs. gRPC (and why direct DB is appropriate here)
- Code walkthrough: connecting to PostgreSQL with GORM, bcrypt hashing, idempotent upsert
- Usage instructions with exact commands
- Testing it: run the CLI, then verify via SQL or login through the UI

- [ ] **Step 3: Write section 6.2 — Admin Dashboard**

Create `docs/src/ch06/admin-dashboard.md` covering:
- Adding `ListUsers` proto RPC and implementation (repository → service → handler)
- Adding `ListAllReservations` proto RPC with denormalization trade-off discussion
- New auth client dependency in reservation service
- Gateway handlers and templates
- Navigation update
- Running `buf generate` and verifying

- [ ] **Step 4: Write section 6.3 — Catalog Seed CLI**

Create `docs/src/ch06/seed-cli.md` covering:
- The problem: empty catalog is useless for development and testing
- Design: gRPC-based seeding (exercises auth + catalog + triggers Kafka events later)
- Code walkthrough: login, read JSON, iterate with CreateBook, handle AlreadyExists
- The fixture file design: diverse genres, realistic data
- Idempotency: safe to re-run

- [ ] **Step 5: Write section 6.4 — Putting It Together**

Create `docs/src/ch06/putting-it-together.md` covering:
- End-to-end walkthrough:
  1. Start the stack with `docker compose up --build`
  2. Create admin account with the CLI
  3. Seed the catalog
  4. Log in as admin, browse the catalog
  5. Check the admin dashboard (users, reservations)
  6. Register a regular user, make a reservation, see it in admin view
- What's next: Chapter 7 introduces Kafka events

- [ ] **Step 6: Commit**

```bash
git add docs/src/ch06/
git commit -m "docs: write Chapter 6 — Admin & Developer Tooling"
```

---

## Task 9: Integration Verification

Run all tests, verify Docker Compose works, and do a manual smoke test.

- [ ] **Step 1: Run auth service tests**

Run: `cd services/auth && go test ./...`
Expected: All tests pass

- [ ] **Step 2: Run reservation service tests**

Run: `cd services/reservation && go test ./...`
Expected: All tests pass (unit tests only, integration tests behind build tags)

- [ ] **Step 3: Run gateway tests**

Run: `cd services/gateway && go test ./...`
Expected: All tests pass

- [ ] **Step 4: Run full CI**

Run: `earthly +ci`
Expected: All lint + test targets pass

- [ ] **Step 5: Docker Compose smoke test**

```bash
cd deploy && docker compose up --build -d
```

Wait for all services to be healthy, then:

```bash
# Create admin
DATABASE_URL="host=localhost port=5434 user=postgres password=postgres dbname=auth sslmode=disable" \
  go run services/auth/cmd/admin/main.go \
    --email admin@example.com --password secret --name "Admin"

# Seed catalog
go run services/catalog/cmd/seed/main.go \
  --auth-addr localhost:50051 --catalog-addr localhost:50052 \
  --email admin@example.com --password secret
```

Verify:
- Login as admin at http://localhost:8080/login
- Navigate to http://localhost:8080/admin — should see dashboard
- Navigate to http://localhost:8080/admin/users — should see user list
- Navigate to http://localhost:8080/admin/reservations — should see (empty) reservation list
- Navigate to http://localhost:8080/books — should see seeded books
- Register a new user, reserve a book, check admin reservations page shows it

- [ ] **Step 6: Tear down**

```bash
cd deploy && docker compose down
```

- [ ] **Step 7: Final commit if any fixes needed**

If smoke testing revealed issues, fix and commit.

---

## Task 10: Update README

Update the root `README.md` to reflect the new chapter numbering and mention the admin/seed CLI tools.

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update tutorial table**

In `README.md`, update the chapter table to reflect new numbering (Ch.6-13 → Ch.7-14) and add the new Chapter 6 entry.

- [ ] **Step 2: Add admin/seed CLI to Quick Start**

In the Quick Start section, after "Verify the gateway", add:

```markdown
**3. Create an admin account:**

\`\`\`bash
DATABASE_URL="host=localhost port=5434 user=postgres password=postgres dbname=auth sslmode=disable" \
  go run services/auth/cmd/admin/main.go \
    --email admin@example.com --password secret --name "Admin"
\`\`\`

**4. Seed the catalog:**

\`\`\`bash
go run services/catalog/cmd/seed/main.go \
  --auth-addr localhost:50051 --catalog-addr localhost:50052 \
  --email admin@example.com --password secret
\`\`\`
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README with new chapter numbering and admin/seed CLI instructions"
```
