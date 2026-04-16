# Chapter 6: Admin & Developer Tooling

At this point we have a working library system: authentication, a book catalog with CRUD, reservations, and a gateway that renders it all in the browser. But if you try to use it from scratch, you hit a chicken-and-egg problem:

- **No admin accounts.** The `POST /admin/books` route requires an admin user, but `Register` always creates users with the `"user"` role. No path exists to promote a user to admin through the UI or gRPC API.
- **No books.** Even if you had an admin, the catalog is empty. Manually filling in the "Add Book" form sixteen times is tedious and error-prone.
- **No visibility.** An admin can manage books, but cannot see who is registered or what reservations exist across the system.

This chapter builds three tools that solve these problems and make the system usable for development—plus an end-to-end walkthrough to tie them together:

---

## What We Build

1. **Admin CLI** (Section 6.1)—A standalone Go binary that connects directly to the auth database and creates (or promotes) an admin account. This bypasses the gRPC layer intentionally: it is a bootstrapping tool, not a feature.

2. **Admin Dashboard** (Section 6.2)—New gRPC RPCs (`ListUsers` on auth, `ListAllReservations` on reservation) and three gateway pages that display users and reservations to admins. We also add a new proto message, `ReservationDetail`, that denormalizes book titles and user emails into a single response.

3. **Catalog Seed CLI** (Section 6.3)—A Go binary that logs in as an admin via gRPC, reads a JSON fixture file, and creates books through the Catalog Service's `CreateBook` RPC. Unlike the admin CLI, this exercises the full stack: authentication, authorization, validation, and (when configured) Kafka event publishing.

4. **Putting It Together** (Section 6.4)—An end-to-end walkthrough: start the stack, create an admin, seed the catalog, register a user, make a reservation, and verify everything in the admin dashboard.

---

## Section Outline

| Section | Topic | Key Files |
|---------|-------|-----------|
| [6.1 Admin CLI](admin-cli.md) | Bootstrapping admin accounts via direct DB access | `services/auth/cmd/admin/main.go` |
| [6.2 Admin Dashboard](admin-dashboard.md) | New RPCs, denormalization, gateway pages | `proto/auth/v1/auth.proto`, `proto/reservation/v1/reservation.proto`, `services/gateway/internal/handler/admin.go` |
| [6.3 Catalog Seed CLI](seed-cli.md) | Populating the catalog through gRPC | `services/catalog/cmd/seed/main.go`, `services/catalog/cmd/seed/books.json` |
| [6.4 Putting It Together](putting-it-together.md) | End-to-end verification | All of the above |
