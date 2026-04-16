# 2.1 Protocol Buffers & gRPC

Before writing a single line of service logic, we need to define the contract between services. In this project, internal service-to-service communication uses **gRPC**, and the message format is **Protocol Buffers** (protobuf). This section explains both, walks through the actual proto file for the Catalog service, and introduces the `buf` toolchain that simplifies protobuf development.

---

## What Is Protocol Buffers?

Protocol Buffers is a language-neutral, platform-neutral binary serialization format developed by Google. You define your data structures and service interfaces in `.proto` files—a purpose-built interface definition language (IDL)—and then generate code for whatever language you're targeting.

### Comparison to JSON

| Property | JSON | Protobuf |
|---|---|---|
| Format | Human-readable text | Binary |
| Schema | Optional (JSON Schema) | Required (`.proto` file) |
| Type safety | Weak (everything is a string/number/bool/null) | Strong (int32, string, bool, enum, etc.) |
| Size | Larger | 3–10x smaller on average |
| Speed | Slower to parse | Faster to serialize/deserialize |
| Language support | Universal | Generated per language |

The trade-off is readability. You can't curl a gRPC endpoint and inspect the response the way you can with REST/JSON. This is acceptable for internal service calls—you'll add tooling like gRPC reflection and Postman/Insomnia gRPC support for debugging.

### Comparison to Java/Kotlin Serialization

If you've used Java's `Serializable` interface or Kotlin's `kotlinx.serialization`, protobuf occupies a similar conceptual space with some important differences:

- **Java `Serializable`** is tied to the JVM, fragile across class changes (serialVersionUID), and produces opaque binary that's hard to evolve. Protobuf's field numbering scheme makes forward/backward compatibility explicit and predictable.
- **`kotlinx.serialization`** is closer in spirit: it's explicit (you annotate what gets serialized), supports multiple formats (JSON, CBOR, etc.), and is type-safe. The key difference is that protobuf is *cross-language* by design. Your Go service and a future Python client can share the same `.proto` file and generate idiomatic code for each language.
- **Protobuf's most significant advantage** is that field identity is based on *numbers*, not names. If you rename `published_year` in your `.proto`, the wire format doesn't change—existing services don't break. Removing a field (but keeping its number reserved) also preserves compatibility. This is what makes it safe to evolve APIs in a microservices system where you can't do a big-bang redeploy.

---

## Why gRPC for Internal Services?

gRPC is an RPC framework built on top of HTTP/2 that uses protobuf as its default serialization format. The question isn't "why not REST"—it's "why use each where you do."

**Use REST for external APIs** (the public-facing gateway, webhooks, integrations). REST is universally understood, easy to consume from browsers, and simpler to document and test manually.

**Use gRPC for service-to-service calls** because:

- **HTTP/2 multiplexing**: Multiple RPC calls share a single TCP connection, reducing connection overhead.
- **Strong typing end-to-end**: The `.proto` file is the source of truth. Client and server code is generated from it. Type mismatches are caught at compile time, not at 2 a.m. in production.
- **Code generation**: You don't write HTTP clients, parse response bodies, or build URL paths. You call a method on a generated stub.
- **Bidirectional streaming**: gRPC supports four call types—unary (standard request/response), server streaming, client streaming, and bidirectional streaming. Most of our service calls are unary, but the option is there if you need to stream a large result set.
- **Built-in deadlines and cancellation**: gRPC propagates `context.Context` cancellation and deadlines across the network boundary automatically.

In our system, external HTTP requests reach the API Gateway (or, for some routes, a service directly). Internal calls—like the Reservation service asking the Catalog service to update book availability—use gRPC.

---

## Walking Through `catalog.proto`

The full file lives at `proto/catalog/v1/catalog.proto`. Let's go through it piece by piece.

```protobuf
syntax = "proto3";
```

This declares proto3 syntax. Proto3 is the pragmatic default for new projects. The Protobuf Editions system, announced in 2023, is the long-term successor, but proto3 remains widely supported and is what most tooling expects. The most notable change from proto2: all fields are optional by default and have zero-value defaults (empty string, 0, false). There's no `required` keyword.

```protobuf
package catalog.v1;
```

The package name defines the protobuf namespace. It prevents naming collisions when multiple proto files are compiled together. The `v1` suffix is an API versioning convention—when a breaking change is unavoidable, you'd create a `v2` package rather than silently changing the existing one.

```protobuf
option go_package = "github.com/fesoliveira014/library-system/gen/catalog/v1;catalogv1";
```

This is a Go-specific option. It tells the code generator two things:

- The import path for the generated package (`github.com/fesoliveira014/library-system/gen/catalog/v1`)
- The package name to use in the generated Go file (`catalogv1`, after the semicolon)

Without this, the generator picks a default path that's awkward to import.

```protobuf
import "google/protobuf/timestamp.proto";
```

Protobuf has a standard library of well-known types. `google.protobuf.Timestamp` is the canonical way to represent timestamps—it's a message with `seconds` (int64) and `nanos` (int32) fields. You import it the same way you'd import a dependency in any language.

### The Service Definition

```protobuf
service CatalogService {
  rpc CreateBook(CreateBookRequest) returns (Book);
  rpc GetBook(GetBookRequest) returns (Book);
  rpc UpdateBook(UpdateBookRequest) returns (Book);
  rpc DeleteBook(DeleteBookRequest) returns (DeleteBookResponse);
  rpc ListBooks(ListBooksRequest) returns (ListBooksResponse);
  rpc UpdateAvailability(UpdateAvailabilityRequest) returns (UpdateAvailabilityResponse);
}
```

A `service` block defines the RPC interface—equivalent to a Java or Kotlin interface annotated for an RPC framework. (For example, in Spring with the `grpc-spring` integration library, `@GrpcService` marks a bean as a gRPC service implementation; the annotation comes from that library, not from gRPC itself.) Each `rpc` line is one method: its name, request message type, and response message type.

Notice that even `DeleteBook` returns a message (`DeleteBookResponse`) instead of nothing. This is a best practice: returning an empty message rather than `void` leaves room to add fields to the response later without a breaking change.

### Message Types and Field Numbers

```protobuf
message Book {
  string id = 1;
  string title = 2;
  string author = 3;
  string isbn = 4;
  string genre = 5;
  string description = 6;
  int32 published_year = 7;
  int32 total_copies = 8;
  int32 available_copies = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}
```

The `= 1`, `= 2`, etc. are **field numbers**, not default values. They are how protobuf identifies fields on the wire. Field 1 is encoded differently than field 11—and crucially, field 1 will always mean `id` for any client that speaks this protocol, regardless of what you name the field in the future.

Field numbers 1–15 use one byte of encoding overhead; fields 16–2047 use two bytes. Put your most-frequently-used fields in the lower numbers. Field numbers must be unique within a message and must never be reused once a field has been removed (use `reserved` to prevent accidental reuse).

### Repeated Fields

```protobuf
message ListBooksResponse {
  repeated Book books = 1;
  int32 total_count = 2;
}
```

`repeated` is protobuf's equivalent of a list/array. On the wire, it encodes as a sequence of values tagged with that field number. In the generated Go code, it becomes a slice: `Books []*Book`.

---

## The buf Toolchain

You can use the raw `protoc` compiler with downloaded plugins. In practice, its dependency management is painful:

- You manually download and version `protoc` itself and each language plugin
- You manage `.proto` imports by hand (where does `google/protobuf/timestamp.proto` live on your filesystem?)
- There's no built-in linting for API design consistency
- Breaking change detection requires external tooling

`buf` solves all of this. It's a modern build tool for protobuf that handles dependencies, enforces style rules, and detects breaking changes.

### `buf.yaml`

Located at `proto/buf.yaml`, this is the module configuration file:

```yaml
version: v2
lint:
  use:
    - STANDARD
  except:
    - RPC_RESPONSE_STANDARD_NAME
    - RPC_REQUEST_RESPONSE_UNIQUE
breaking:
  use:
    - FILE
```

- `lint.use: [STANDARD]` enables buf's standard lint ruleset. This catches things like: inconsistent field naming conventions, missing comments on public RPCs, package name mismatches, and more.
- The two `except` entries suppress rules we've deliberately relaxed. `RPC_RESPONSE_STANDARD_NAME` would require every response type to be named `<RpcName>Response`—we have some RPCs (like `CreateBook`) that return `Book` directly, which is an intentional design choice.
- `breaking.use: [FILE]` enables file-level breaking change detection. This means buf will error if you remove a field, change a field type, or make any other change that would break existing clients.

### `buf.gen.yaml`

Located at `proto/buf.gen.yaml`, this tells buf how to generate code:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: ../gen
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: ../gen
    opt: paths=source_relative
```

Two plugins are invoked:

- `buf.build/protocolbuffers/go`—generates the message structs (`catalog.pb.go`)
- `buf.build/grpc/go`—generates the client and server interfaces (`catalog_grpc.pb.go`)

Both write output to `../gen` (relative to `proto/`), so the generated code lands in `gen/catalog/v1/`. The `paths=source_relative` option tells the generator to mirror the proto directory structure in the output.

### Running buf

```bash
# From the proto/ directory

# Lint your proto files
buf lint

# Check for breaking changes against the previous git commit
buf breaking --against '.git#branch=main'

# Generate Go code
buf generate
```

If `buf lint` passes silently, your proto design meets the standard rules. If it fails, you'll get actionable messages pointing to the exact line.

---

## The Generated Code

Running `buf generate` produces two files per proto file. For the Catalog service:

### `gen/catalog/v1/catalog.pb.go`

This file contains all the message structs. The `Book` struct looks like this (simplified):

```go
type Book struct {
    Id              string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
    Title           string                 `protobuf:"bytes,2,opt,name=title,proto3" json:"title,omitempty"`
    Author          string                 `protobuf:"bytes,3,opt,name=author,proto3" json:"author,omitempty"`
    // ... remaining fields
    CreatedAt       *timestamppb.Timestamp `protobuf:"bytes,10,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
    UpdatedAt       *timestamppb.Timestamp `protobuf:"bytes,11,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
    // unexported internal fields omitted
}
```

Notice that field names are PascalCase in Go (Go convention) even though they're snake_case in the proto file. The struct tags encode both the wire format information and JSON field names—the generated structs work with `encoding/json` as well as protobuf encoding.

Do not manually edit these files. The `// Code generated...DO NOT EDIT.` header is load-bearing—`buf generate` will overwrite anything you change.

### `gen/catalog/v1/catalog_grpc.pb.go`

This file contains the server interface you must implement and the client stub you'll call from other services.

**The server interface**—this is what you implement in the Catalog service:

```go
type CatalogServiceServer interface {
    CreateBook(context.Context, *CreateBookRequest) (*Book, error)
    GetBook(context.Context, *GetBookRequest) (*Book, error)
    UpdateBook(context.Context, *UpdateBookRequest) (*Book, error)
    DeleteBook(context.Context, *DeleteBookRequest) (*DeleteBookResponse, error)
    ListBooks(context.Context, *ListBooksRequest) (*ListBooksResponse, error)
    UpdateAvailability(context.Context, *UpdateAvailabilityRequest) (*UpdateAvailabilityResponse, error)
    mustEmbedUnimplementedCatalogServiceServer()
}
```

The `mustEmbedUnimplementedCatalogServiceServer()` unexported method forces you to embed `UnimplementedCatalogServiceServer` in your implementation struct. This is a forward-compatibility mechanism: when new RPC methods are added to the proto, your service won't fail to compile—the embedded `Unimplemented...` struct provides default implementations that return `codes.Unimplemented`. You can then add the real implementations incrementally.

**The client stub**—this is what other services use to call Catalog:

```go
type CatalogServiceClient interface {
    CreateBook(ctx context.Context, in *CreateBookRequest, opts ...grpc.CallOption) (*Book, error)
    GetBook(ctx context.Context, in *GetBookRequest, opts ...grpc.CallOption) (*Book, error)
    // ...
}

func NewCatalogServiceClient(cc grpc.ClientConnInterface) CatalogServiceClient { ... }
```

The Reservation service, for example, will call `NewCatalogServiceClient(conn)` with a gRPC connection and then call `client.UpdateAvailability(ctx, req)` as if it were a local function call. The network transport is entirely hidden.

---

## Exercise: Add a `SearchBooks` RPC

The system will eventually have a Search service, but the Catalog service needs to expose a search endpoint first. Your task is to add a `SearchBooks` RPC to `proto/catalog/v1/catalog.proto`.

Requirements:
- The request message should accept a `query` string (the search term)
- The response should reuse the existing `ListBooksResponse` (it already has `repeated Book books` and `int32 total_count`)
- Run `buf lint` from the `proto/` directory to verify there are no style violations
- Run `buf generate` to regenerate the Go code

This is proto-only work—don't implement the Go handler yet. The point is to practice the full proto edit → lint → generate cycle.

<details>
<summary>Solution</summary>

Add the RPC to the service definition and a new request message:

```protobuf
service CatalogService {
  rpc CreateBook(CreateBookRequest) returns (Book);
  rpc GetBook(GetBookRequest) returns (Book);
  rpc UpdateBook(UpdateBookRequest) returns (Book);
  rpc DeleteBook(DeleteBookRequest) returns (DeleteBookResponse);
  rpc ListBooks(ListBooksRequest) returns (ListBooksResponse);
  rpc UpdateAvailability(UpdateAvailabilityRequest) returns (UpdateAvailabilityResponse);
  rpc SearchBooks(SearchBooksRequest) returns (ListBooksResponse);  // add this
}

// Add this new message alongside the others
message SearchBooksRequest {
  string query = 1;
}
```

Then from the `proto/` directory:

```bash
buf lint      # should pass (or show only the suppressed rules)
buf generate  # regenerates gen/catalog/v1/
```

After regeneration, `catalog_grpc.pb.go` will include `SearchBooks` in both the server and client interfaces. The `UnimplementedCatalogServiceServer` will have a default implementation returning `codes.Unimplemented`, so the service compiles without changes.

Note: `buf breaking` would flag this as safe—adding a new RPC is not a breaking change. Removing one would be.

</details>

---

## Summary

- Protocol Buffers is a binary IDL-driven serialization format. `.proto` files define the schema; code generators produce language-specific implementations.
- gRPC is the RPC framework of choice for internal service calls: type-safe, fast, with built-in support for streaming and deadline propagation.
- Field numbers—not names—define wire identity. Never reuse them after removal.
- `buf` replaces raw `protoc` with a dependency-managed, linting, breaking-change-detecting build tool.
- Generated code lives in `gen/` and must not be manually edited. The server interface is what you implement; the client stub is what callers use.

---

## References

[^1]: [Protocol Buffers Language Guide (proto3)](https://protobuf.dev/programming-guides/proto3/)
[^2]: [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
[^3]: [buf documentation](https://buf.build/docs/)
