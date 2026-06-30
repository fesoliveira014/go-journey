# Chapter 6: Admin & Developer Tooling

> **Chapter checkpoint**
> Start from: `git checkout chapter-06-start`
> End state: `git checkout chapter-06-end`
>
> Chapter snippets are point-in-time snapshots. Later chapters intentionally change the same files.

At this point we have authentication, a book catalog with CRUD, and a gateway that renders it in the browser. The system works, but it is still awkward to operate during development.

Even before reservations exist, the development workflow has a chicken-and-egg problem:

- **No admin accounts.** The `POST /admin/books` route requires an admin user, but `Register` always creates users with the `"user"` role. No path exists to promote a user to admin through the UI or gRPC API.
- **No books.** Even if you had an admin, the catalog is empty. Manually filling in the "Add Book" form sixteen times is tedious and error-prone.
- **Limited visibility.** An admin can manage books, but cannot see registered users. Chapter 7 extends the same dashboard pattern to reservations after the Reservation service exists.

This chapter builds three tools that solve these problems and make the system usable for development—plus an end-to-end walkthrough to tie them together:

---

## What We Build

1. **Admin CLI** (Section 6.1)—A standalone Go binary that connects directly to the auth database and creates (or promotes) an admin account. This bypasses the gRPC layer intentionally: it is a bootstrapping tool, not a feature.

2. **Admin Dashboard** (Section 6.2)—A new `ListUsers` RPC on auth and gateway pages for user visibility.

3. **Catalog Seed CLI** (Section 6.3)—A Go binary that logs in as an admin via gRPC, reads a JSON fixture file, and creates books through the Catalog Service's `CreateBook` RPC. Unlike the admin CLI, this exercises the full stack: authentication, authorization, validation, and (when configured) Kafka event publishing.

4. **Putting It Together** (Section 6.4)—A walkthrough for the checkpoint: start the stack, create an admin, seed the catalog, register a user, and verify the admin dashboard.

---

## Section Outline

| Section | Topic | Key Files |
|---------|-------|-----------|
| [6.1 Admin CLI](admin-cli.md) | Bootstrapping admin accounts via direct DB access | `services/auth/cmd/admin/main.go` |
| [6.2 Admin Dashboard](admin-dashboard.md) | User visibility and gateway admin pages | `proto/auth/v1/auth.proto`, `services/gateway/internal/handler/admin.go` |
| [6.3 Catalog Seed CLI](seed-cli.md) | Populating the catalog through gRPC | `services/catalog/cmd/seed/main.go`, `services/catalog/cmd/seed/books.json` |
| [6.4 Putting It Together](putting-it-together.md) | End-to-end verification | All of the above |
