# 6.2 Admin Dashboard

The admin can already manage books through the CRUD pages built in Chapter 5, but one operational question is still unanswered: who are the registered users? This section adds a small admin dashboard with a user list. The dashboard will grow in Chapter 7 after reservations exist.

The important boundary is authorization. The gateway hides the page from regular users, but the Auth Service must still enforce the admin role at the gRPC layer. UI checks are useful for navigation; backend checks are the security boundary.

---

## Adding `ListUsers` to the Auth Proto

The auth proto already has `GetUser` for looking up one user by ID. We need a new RPC that returns all users:

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

The request is empty because this endpoint has no filtering or pagination. In a production system you would add `page_size`, `page_token`, and filter fields. For this checkpoint, the system has a handful of users, so returning them all keeps the example focused on the authorization and gateway flow.

After updating the proto, regenerate the Go code:

```bash
buf generate
```

This updates the generated client and server interfaces in `gen/auth/v1/`. The Auth Service will not compile until you implement the new `ListUsers` method on the handler.

---

## Auth Service Implementation

The implementation follows the same layered pattern as every other RPC in the project: repository -> service -> handler.

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

`Find` without a `Where` clause returns all records. We order by `created_at DESC` so the most recently registered users appear first.

### Service: `ListUsers()`

```go
// services/auth/internal/service/auth.go

// ListUsers returns all users.
func (s *AuthService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.repo.List(ctx)
}
```

The service layer is a passthrough here. It exists to maintain the pattern: if you later add filtering, caching, or audit logging, you have a place to put it without touching the handler or repository.

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

The key line is `pkgauth.RequireRole(ctx, "admin")`. This helper from `pkg/auth/context.go` extracts the role from the gRPC context, which the JWT interceptor populated earlier in the request. If the caller is missing a token or does not have the required role, the helper returns a gRPC auth error before the repository is touched:

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

This is defense in depth. The gateway also checks the admin role before rendering admin pages, but direct gRPC callers still have to pass the service-level authorization check.

---

## Gateway: Admin Dashboard Pages

The gateway gets a small dashboard landing page and a user-list page.

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
```

Each handler follows the same three-step pattern: check admin role, call gRPC, render a template. This file is separate from `catalog.go` because these views are cross-cutting admin pages rather than catalog CRUD handlers.

### Routes

Register the dashboard and user-list routes alongside the existing admin book CRUD routes:

```go
// services/gateway/cmd/main.go

mux.HandleFunc("GET /admin", srv.AdminDashboard)
mux.HandleFunc("GET /admin/users", srv.AdminUserList)
```

### Templates

The dashboard landing page (`admin_dashboard.html`) links to user visibility and the existing book-management pages:

```html
{{define "content"}}
<h1>Admin Dashboard</h1>
<div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-top: 1rem;">
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Users</h3>
    <p>View registered users and their roles.</p>
    <a href="/admin/users">View Users &rarr;</a>
  </div>
  <div style="border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; flex: 1; min-width: 200px;">
    <h3>Books</h3>
    <p>Add and edit catalog records.</p>
    <a href="/admin/books/new">Add Book &rarr;</a>
  </div>
</div>
{{end}}
```

The users template (`admin_users.html`) renders a table with email, name, role, and join date.

### Navigation Update

The navigation partial conditionally shows the "Admin" link for admin users:

```html
<!-- services/gateway/templates/partials/nav.html -->

{{if eq .User.Role "admin"}}
    <a href="/admin">Admin</a>
{{end}}
```

Regular users never see this link. Even if they manually navigate to `/admin`, the `requireAdmin` check in the handler returns a 403 error.

---

## Testing the New RPC

You can test the new RPC directly with `grpcurl`:

```bash
# List users (requires admin JWT)
grpcurl -plaintext \
  -H "authorization: Bearer $ADMIN_TOKEN" \
  localhost:50051 auth.v1.AuthService/ListUsers
```

Non-admin tokens receive `PERMISSION_DENIED`. Unauthenticated requests receive `UNAUTHENTICATED`.

---

## Key Takeaways

- **`RequireRole` at the gRPC layer** provides defense in depth. Even if someone bypasses the gateway, the backend service enforces authorization.
- **Admin tooling starts narrow.** Chapter 6 needs admin bootstrapping, catalog seeding, and user visibility. Reservation-wide visibility belongs after the Reservation service exists.
- **Separate handler files by responsibility.** `admin.go` handles cross-cutting admin views; `catalog.go` handles catalog CRUD. This keeps files focused and navigable.
