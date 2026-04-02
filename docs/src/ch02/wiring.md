# 2.5 Wiring It All Together

Every component you have built so far — the repository, the service, the handler — exists in isolation. The repository knows nothing about the service, and the service knows nothing about gRPC. That separation is the whole point, but at some point something has to plug them together. That's `main.go`. This section walks through how the pieces connect, why the connection is explicit rather than magical, and how to drive the running service with `grpcurl`.

---

## Constructor-Based Dependency Injection

In Spring you might write:

```kotlin
// Spring — framework magic
@Service
class CatalogService(@Autowired val repo: BookRepository)

@GrpcService
class CatalogHandler(@Autowired val svc: CatalogService)
```

The framework scans for annotations, builds a dependency graph, constructs objects in the right order, and wires them up for you. This is convenient until it isn't — when something goes wrong, the stack trace runs through several layers of reflection before reaching your code.

Go uses none of this. Dependency injection in Go is just function calls:

```go
// services/catalog/cmd/main.go

bookRepo    := repository.NewBookRepository(db)
catalogSvc  := service.NewCatalogService(bookRepo)
catalogHandler := handler.NewCatalogHandler(catalogSvc)
```

Three lines, three constructors, each returning a pointer to an initialized struct. The wiring is explicit, linear, and entirely visible. You can read it and immediately understand the dependency graph: `db → repo → svc → handler`. If a constructor requires a dependency you have not yet created, the compiler tells you — there is no runtime "bean not found" surprise.

Each constructor follows the same pattern. It takes what it needs as arguments and returns a concrete type:

```go
func NewBookRepository(db *gorm.DB) *BookRepository {
    return &BookRepository{db: db}
}

func NewCatalogService(repo BookRepository) *CatalogService {
    return &CatalogService{repo: repo}
}

func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler {
    return &CatalogHandler{svc: svc}
}
```

The service constructor accepts a `BookRepository` **interface**, not a concrete type. This is the key inversion: the service owns the interface definition and only knows that it has something to call `Create`, `GetByID`, and so on. In production, that turns out to be the GORM-backed struct. In tests it is the in-memory mock from the previous section. The service does not know or care which.

This pattern is sometimes called **manual dependency injection**. For projects up to moderate complexity it is the right approach — it scales well, is trivially debuggable, and produces zero "magic". Larger projects sometimes introduce a dependency injection framework (`google/wire` is the most common in Go), but that is code generation over the same pattern, not a fundamentally different approach.

---

## gRPC Server Setup

Once the dependencies are wired, setting up the gRPC server takes five lines:

```go
lis, err := net.Listen("tcp", ":"+grpcPort)
if err != nil {
    log.Fatalf("failed to listen: %v", err)
}

grpcServer := grpc.NewServer()
catalogv1.RegisterCatalogServiceServer(grpcServer, catalogHandler)
reflection.Register(grpcServer)

log.Printf("catalog service listening on :%s", grpcPort)
if err := grpcServer.Serve(lis); err != nil {
    log.Fatalf("failed to serve: %v", err)
}
```

Walk through each call:

- **`net.Listen("tcp", ":50052")`** — opens a TCP socket. The `:50052` form means "listen on all interfaces, port 50052". The `net.Listener` is a standard library type; gRPC does not control how the socket is opened.
- **`grpc.NewServer()`** — creates a gRPC server instance. At this point it has no services registered. Options for TLS, interceptors (middleware), and keepalives go here as variadic arguments — none are needed yet.
- **`catalogv1.RegisterCatalogServiceServer(...)`** — this is generated code from the protobuf toolchain. It binds your handler to the server's internal service registry, mapping each RPC name to its method. Without this call, the server runs but has no services.
- **`reflection.Register(grpcServer)`** — registers the gRPC server reflection protocol. This is what allows tools like `grpcurl` to query the server for its available services and method signatures at runtime, without needing the `.proto` files locally. You would disable this in production (it exposes your API surface), but it is invaluable during development.
- **`grpcServer.Serve(lis)`** — blocks, accepting connections and dispatching calls. This never returns unless the server is stopped.

---

## The Handler Layer

The `CatalogHandler` is the boundary between the gRPC transport and the domain model. Its job is narrow: translate proto types to domain types, call the service, translate the result back to proto, and map domain errors to gRPC status codes.

### Proto ↔ Domain Conversion

Proto types and domain types are deliberately separate. The generated `catalogv1.Book` is a transport struct — it carries wire format metadata, has protobuf-specific field types (`int32`, `*timestamppb.Timestamp`), and is tied to the serialization contract with external callers. The domain `model.Book` is a persistence struct with GORM tags and Go-native types (`uuid.UUID`, `time.Time`, `int`).

Keeping them separate means a change to the proto schema does not cascade into the database layer, and a change to the domain model (adding a field, changing a type) does not automatically break the public API. The conversion functions are the explicit, auditable boundary between those two worlds.

`bookToProto` handles domain → proto:

```go
func bookToProto(b *model.Book) *catalogv1.Book {
    return &catalogv1.Book{
        Id:              b.ID.String(),
        Title:           b.Title,
        Author:          b.Author,
        Isbn:            b.ISBN,
        Genre:           b.Genre,
        Description:     b.Description,
        PublishedYear:   int32(b.PublishedYear),
        TotalCopies:     int32(b.TotalCopies),
        AvailableCopies: int32(b.AvailableCopies),
        CreatedAt:       timestamppb.New(b.CreatedAt),
        UpdatedAt:       timestamppb.New(b.UpdatedAt),
    }
}
```

There is no magic here — just explicit field assignment. The UUID becomes a string (proto has no UUID type), `time.Time` becomes a `*timestamppb.Timestamp` (proto's standard timestamp type), and Go's `int` becomes proto3's `int32`. Every conversion is visible and testable.

### Error Mapping

`toGRPCError` translates domain errors to gRPC status errors:

```go
func toGRPCError(err error) error {
    switch {
    case errors.Is(err, model.ErrBookNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, model.ErrDuplicateISBN):
        return status.Error(codes.AlreadyExists, err.Error())
    case errors.Is(err, model.ErrInvalidBook):
        return status.Error(codes.InvalidArgument, err.Error())
    default:
        return status.Error(codes.Internal, "internal error")
    }
}
```

A few things worth noting here:

- `errors.Is` handles wrapped errors — if the service returns `fmt.Errorf("%w: title is required", model.ErrInvalidBook)`, the `errors.Is(err, model.ErrInvalidBook)` branch still fires.
- The `default` case returns `codes.Internal` with a **generic message** — not `err.Error()`. That is deliberate: unexpected errors often contain internal implementation details (SQL query text, file paths, internal service names) that should not be sent to external callers. Logging the original error separately is the right pattern.
- The `status.Error(code, message)` call produces a gRPC status error — a type that the gRPC runtime knows how to serialize and send as a proper gRPC error response, including the status code that the client can inspect. [^3]

---

## Testing with grpcurl

`grpcurl` is a command-line tool for calling gRPC servers, analogous to `curl` for HTTP. Because you enabled `reflection.Register`, it can discover the service schema without needing the `.proto` files. [^2]

### Installation

```bash
# macOS
brew install grpcurl

# Linux (download from GitHub releases)
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### Start the Service

First, make sure PostgreSQL is running and the `catalog` database exists, then:

```bash
cd services/catalog
DATABASE_URL="host=localhost port=5432 user=postgres password=postgres dbname=catalog sslmode=disable" \
    go run ./cmd
```

You should see:

```
connected to PostgreSQL
migrations completed
catalog service listening on :50052
```

### Discover Available Services

```bash
grpcurl -plaintext localhost:50052 list
```

```
catalog.v1.CatalogService
grpc.reflection.v1alpha.ServerReflection
```

Drill into the methods on the service:

```bash
grpcurl -plaintext localhost:50052 list catalog.v1.CatalogService
```

```
catalog.v1.CatalogService.CreateBook
catalog.v1.CatalogService.DeleteBook
catalog.v1.CatalogService.GetBook
catalog.v1.CatalogService.ListBooks
catalog.v1.CatalogService.UpdateAvailability
catalog.v1.CatalogService.UpdateBook
```

Inspect the message schema for any RPC:

```bash
grpcurl -plaintext localhost:50052 describe catalog.v1.CatalogService.CreateBook
```

### Create a Book

```bash
grpcurl -plaintext -d '{
  "title": "The Go Programming Language",
  "author": "Donovan & Kernighan",
  "isbn": "9780134190440",
  "genre": "programming",
  "published_year": 2015,
  "total_copies": 3
}' localhost:50052 catalog.v1.CatalogService.CreateBook
```

The response includes the server-assigned UUID, `available_copies` (set equal to `total_copies` by the service layer), and timestamps:

```json
{
  "id": "a1b2c3d4-...",
  "title": "The Go Programming Language",
  "author": "Donovan & Kernighan",
  "isbn": "9780134190440",
  "genre": "programming",
  "publishedYear": 2015,
  "totalCopies": 3,
  "availableCopies": 3,
  "createdAt": "2026-03-29T10:00:00Z",
  "updatedAt": "2026-03-29T10:00:00Z"
}
```

### List Books with Filters

List all books (paginated, default page size 20):

```bash
grpcurl -plaintext -d '{}' localhost:50052 catalog.v1.CatalogService.ListBooks
```

Filter by genre:

```bash
grpcurl -plaintext -d '{"genre": "programming"}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks
```

Filter to show only books with available copies:

```bash
grpcurl -plaintext -d '{"available_only": true}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks
```

Paginate:

```bash
grpcurl -plaintext -d '{"page": 2, "page_size": 5}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks
```

### Update a Book

Use the ID returned from `CreateBook`:

```bash
grpcurl -plaintext -d '{
  "id": "a1b2c3d4-...",
  "title": "The Go Programming Language",
  "author": "Donovan & Kernighan",
  "total_copies": 5
}' localhost:50052 catalog.v1.CatalogService.UpdateBook
```

### Delete a Book

```bash
grpcurl -plaintext -d '{"id": "a1b2c3d4-..."}' \
    localhost:50052 catalog.v1.CatalogService.DeleteBook
```

An empty `{}` response means success. Attempting to delete the same ID again returns a `NotFound` status code.

---

## Implementation Notes

### Proto3 and Partial Updates

The `UpdateBook` RPC has a subtle limitation: proto3 does not distinguish between "field not set" and "field set to its zero value". If you send:

```json
{"id": "...", "total_copies": 0}
```

The handler cannot tell whether you intentionally want to set `total_copies` to zero or whether you just omitted the field. Both cases arrive as `req.GetTotalCopies() == 0`. The current implementation treats this as a full replacement — whatever fields you send overwrite the stored values, and zero-value fields overwrite with zero.

The idiomatic proto3 solution is `google.protobuf.FieldMask` for partial updates, or using wrapper types (`google.protobuf.Int32Value`) which can represent explicit null. Both add complexity that is not warranted for this learning stage. For now, understand the limitation: `UpdateBook` requires you to resend all fields you want to retain, not just the ones you want to change.

### UpdateAvailability as a Forward Reference

You may notice `UpdateAvailability` in the handler and service — it adjusts `available_copies` by a signed delta. This RPC is not driven by catalog management; it is driven by the reservations service. When a user checks out a book, the reservations service will call `UpdateAvailability(id, -1)` on the catalog service. When the book is returned, it calls `UpdateAvailability(id, +1)`.

This is part of the inter-service communication pattern covered in Chapter 7. The RPC exists now because the catalog service needs to own the availability invariant — no other service should directly manipulate `available_copies`. For now, you can exercise it via `grpcurl`:

```bash
grpcurl -plaintext -d '{"id": "a1b2c3d4-...", "delta": -1}' \
    localhost:50052 catalog.v1.CatalogService.UpdateAvailability
```

---

## Exercise: Drive the Catalog with grpcurl

Start the catalog service locally and work through the following scenario entirely using `grpcurl`.

**Setup:** Create 5 books across at least 2-3 genres. Suggested data:

| Title | Author | Genre | Copies |
|---|---|---|---|
| Clean Code | Robert Martin | programming | 2 |
| The Pragmatic Programmer | Hunt & Thomas | programming | 3 |
| Dune | Frank Herbert | sci-fi | 4 |
| Foundation | Isaac Asimov | sci-fi | 2 |
| The Design of Everyday Things | Don Norman | design | 1 |

**Tasks:**

1. Create all 5 books using `CreateBook`. Record the IDs returned.
2. List all books using `ListBooks` with `{}`. Verify all 5 appear in `total_count`.
3. Filter by `genre: "programming"` — only Clean Code and The Pragmatic Programmer should appear.
4. Filter by `available_only: true` — all 5 should appear since nothing is checked out yet.
5. Update Clean Code to have `total_copies: 4` using `UpdateBook`. Check the returned `available_copies` — it should remain at 2 (the service only updates total, not available, on a manual update).
6. Call `UpdateAvailability` on Dune with `delta: -2`. Verify the returned `available_copies` is 2.
7. Now filter `available_only: true` again. Dune should still appear (2 copies available). Call `UpdateAvailability` with `delta: -2` again. Dune should now have 0 available copies.
8. Filter `available_only: true` one more time — Dune should no longer appear.
9. Delete Foundation. Attempt to `GetBook` with its ID and confirm you receive a `NotFound` error.

<details>
<summary>Solution: complete grpcurl command sequence</summary>

```bash
# Step 1: Create all 5 books (capture IDs from output)
grpcurl -plaintext -d '{
  "title": "Clean Code",
  "author": "Robert Martin",
  "isbn": "9780132350884",
  "genre": "programming",
  "published_year": 2008,
  "total_copies": 2
}' localhost:50052 catalog.v1.CatalogService.CreateBook

grpcurl -plaintext -d '{
  "title": "The Pragmatic Programmer",
  "author": "Hunt & Thomas",
  "isbn": "9780135957059",
  "genre": "programming",
  "published_year": 2019,
  "total_copies": 3
}' localhost:50052 catalog.v1.CatalogService.CreateBook

grpcurl -plaintext -d '{
  "title": "Dune",
  "author": "Frank Herbert",
  "isbn": "9780441013593",
  "genre": "sci-fi",
  "published_year": 1965,
  "total_copies": 4
}' localhost:50052 catalog.v1.CatalogService.CreateBook

grpcurl -plaintext -d '{
  "title": "Foundation",
  "author": "Isaac Asimov",
  "isbn": "9780553293357",
  "genre": "sci-fi",
  "published_year": 1951,
  "total_copies": 2
}' localhost:50052 catalog.v1.CatalogService.CreateBook

grpcurl -plaintext -d '{
  "title": "The Design of Everyday Things",
  "author": "Don Norman",
  "isbn": "9780465050659",
  "genre": "design",
  "published_year": 2013,
  "total_copies": 1
}' localhost:50052 catalog.v1.CatalogService.CreateBook

# Step 2: List all books
grpcurl -plaintext -d '{}' localhost:50052 catalog.v1.CatalogService.ListBooks

# Step 3: Filter by genre
grpcurl -plaintext -d '{"genre": "programming"}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks

# Step 4: Filter available_only (all 5 should appear)
grpcurl -plaintext -d '{"available_only": true}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks

# Step 5: Update Clean Code total_copies to 4
# Note: you must re-send all fields you want to keep
grpcurl -plaintext -d '{
  "id": "<clean-code-id>",
  "title": "Clean Code",
  "author": "Robert Martin",
  "isbn": "9780132350884",
  "genre": "programming",
  "published_year": 2008,
  "total_copies": 4
}' localhost:50052 catalog.v1.CatalogService.UpdateBook
# available_copies stays at 2 — UpdateBook does not touch it

# Step 6: Check out 2 copies of Dune
grpcurl -plaintext -d '{"id": "<dune-id>", "delta": -2}' \
    localhost:50052 catalog.v1.CatalogService.UpdateAvailability
# Response: available_copies: 2

# Step 7: Check out the remaining 2 copies of Dune
grpcurl -plaintext -d '{"id": "<dune-id>", "delta": -2}' \
    localhost:50052 catalog.v1.CatalogService.UpdateAvailability
# Response: available_copies: 0

# Step 8: available_only filter — Dune should be gone
grpcurl -plaintext -d '{"available_only": true}' \
    localhost:50052 catalog.v1.CatalogService.ListBooks

# Step 9: Delete Foundation
grpcurl -plaintext -d '{"id": "<foundation-id>"}' \
    localhost:50052 catalog.v1.CatalogService.DeleteBook

# Verify it's gone
grpcurl -plaintext -d '{"id": "<foundation-id>"}' \
    localhost:50052 catalog.v1.CatalogService.GetBook
# Expected: ERROR: Code: NotFound — book not found
```

**Observation on step 5:** The `available_copies` field is not updated by `UpdateBook`. The service layer enforces that `available_copies` starts equal to `total_copies` only on creation. A subsequent manual update to `total_copies` does not automatically adjust `available_copies`. This is a real-world trade-off: it prevents a race condition where a concurrent checkout might be undone by an admin updating the copy count, but it means administrators who increase total copies also need to call `UpdateAvailability` to reflect new stock. In a production system you would address this with a more explicit inventory management workflow.

</details>

---

## What Comes Next

The catalog service is complete — domain model, repository, service, gRPC handler, and server wiring. The next chapter introduces the gateway service, which sits between external HTTP clients and the internal gRPC services. It is where REST-to-gRPC translation, authentication middleware, and request validation will live.

---

[^1]: [gRPC Go basics](https://grpc.io/docs/languages/go/basics/)
[^2]: [grpcurl documentation](https://github.com/fullstorydev/grpcurl)
[^3]: [gRPC status codes](https://grpc.github.io/grpc/core/md_doc_statuscodes.html)
