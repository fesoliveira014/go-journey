# 2.1 Protocol Buffers & gRPC

<!-- [STRUCTURAL] Opening paragraph does triple duty — motivates the contract-first mindset, names the two technologies, and previews both the code walk-through and the toolchain. Good lede. -->
<!-- [LINE EDIT] "Before writing a single line of service logic, we need to define the contract between services." — consider ending with "between services." for punchier rhythm, or tighten to "Before writing service logic, we define the contract between services." to cut "a single line of" which is a cliché. -->
Before writing a single line of service logic, we need to define the contract between services. In this project, internal service-to-service communication uses **gRPC**, and the message format is **Protocol Buffers** (protobuf). This section explains both, walks through the actual proto file for the Catalog service, and introduces the `buf` toolchain that makes protobuf development manageable at scale.

---

<!-- [STRUCTURAL] "What Is Protocol Buffers?" heading — singular/plural agreement: "Protocol Buffers" is traditionally treated as singular (the product name), which the body text then contradicts ("Protocol Buffers is..."). Keep the singular; the heading is fine. Still, consider "What Are Protocol Buffers?" only if you find the singular jarring. -->
## What Is Protocol Buffers?

<!-- [COPY EDIT] "language-neutral, platform-neutral binary serialization format" — serial comma (CMOS 6.19) N/A (only two adjectives); compound adjectives "language-neutral" and "platform-neutral" correctly hyphenated before noun (CMOS 7.81). Good. -->
Protocol Buffers is a language-neutral, platform-neutral binary serialization format developed by Google. You define your data structures and service interfaces in `.proto` files — a purpose-built interface definition language (IDL) — and then generate code for whatever language you're targeting.

### Comparison to JSON

<!-- [STRUCTURAL] The comparison table is a strong pedagogical choice for an experienced engineer. Well-placed right before the tradeoff paragraph. -->
| Property | JSON | Protobuf |
|---|---|---|
| Format | Human-readable text | Binary |
| Schema | Optional (JSON Schema) | Required (`.proto` file) |
| Type safety | Weak (everything is a string/number/bool/null) | Strong (int32, string, bool, enum, etc.) |
| Size | Larger | 3–10x smaller on average |
| Speed | Slower to parse | Faster to serialize/deserialize |
| Language support | Universal | Generated per language |

<!-- [COPY EDIT] "3–10x smaller on average" — en dash for numeric range correct (CMOS 6.78). "x" as multiplier: CMOS accepts lowercase "x" in informal/technical contexts; consider the Unicode "×" for a book typeset, but "x" is fine in Markdown source. -->
<!-- [COPY EDIT] "Please verify: claim that protobuf is 3–10× smaller than JSON 'on average'. Figures vary by payload shape; consider citing a specific source or qualifying ('often'). -->

<!-- [LINE EDIT] "You can't curl a gRPC endpoint and inspect the response with naked eyes the way you can with REST/JSON." → "You can't curl a gRPC endpoint and inspect the response the way you can with REST/JSON." — "with naked eyes" is colourful but awkward; meaning is already carried. -->
<!-- [LINE EDIT] "gRPC reflection and Postman/Insomnia gRPC support" — Postman and Insomnia both gained gRPC support some time ago; consider "modern API clients (Postman, Insomnia) with gRPC support" to avoid reading as 2021-era guidance. -->
The tradeoff is readability. You can't curl a gRPC endpoint and inspect the response with naked eyes the way you can with REST/JSON. This is acceptable for internal service calls — you'll add tooling like gRPC reflection and Postman/Insomnia gRPC support for debugging.

### Comparison to Java/Kotlin Serialization

<!-- [STRUCTURAL] Good use of the audience's background knowledge. Three bullets cover Serializable, kotlinx.serialization, and the "killer feature" — appropriately scoped. -->
If you've used Java's `Serializable` interface or Kotlin's `kotlinx.serialization`, protobuf occupies a similar conceptual space with some important differences:

<!-- [COPY EDIT] "forward/backward compatibility" — CMOS prefers "forward and backward compatibility" but the slash form is acceptable in technical prose. Consistent usage throughout chapter. -->
- **Java `Serializable`** is tied to the JVM, fragile across class changes (serialVersionUID), and produces opaque binary that's hard to evolve. Protobuf's field numbering scheme makes forward/backward compatibility explicit and predictable.
- **`kotlinx.serialization`** is closer in spirit: it's explicit (you annotate what gets serialized), supports multiple formats (JSON, CBOR, etc.), and is type-safe. The key difference is that protobuf is *cross-language* by design. Your Go service and a future Python client can share the same `.proto` file and generate idiomatic code for each language.
<!-- [COPY EDIT] "e.g.," and "etc." — CMOS 6.43 requires comma after "e.g.,"; "etc." should be preceded by comma in a list. "JSON, CBOR, etc." is correct. Good. -->
- **Protobuf's killer feature** is that field identity is based on *numbers*, not names. If you rename `published_year` in your `.proto`, the wire format doesn't change — existing services don't break. Removing a field (but keeping its number reserved) also preserves compatibility. This is what makes it safe to evolve APIs in a microservices system where you can't do a big-bang redeploy.
<!-- [LINE EDIT] "big-bang redeploy" — informal and fine, but consider "coordinated redeploy" for a slightly more technical register. Judgment call. -->

---

## Why gRPC for Internal Services?

<!-- [LINE EDIT] "The question isn't 'why not REST' — it's 'why use each where you do.'" — rhetorically effective but slightly glib. Consider: "The question isn't REST versus gRPC — it's where each fits." -->
gRPC is an RPC framework built on top of HTTP/2 that uses protobuf as its default serialization format. The question isn't "why not REST" — it's "why use each where you do."

<!-- [STRUCTURAL] The REST-vs-gRPC framing is a useful structural move: it pre-empts the question the experienced reader will have already formed. Well placed. -->
**Use REST for external APIs** (the public-facing gateway, webhooks, integrations). REST is universally understood, easy to consume from browsers, and simpler to document and test manually.

**Use gRPC for service-to-service calls** because:

<!-- [COPY EDIT] Bulleted list: each item begins with bolded noun phrase, then colon — list parallelism is good. -->
- **HTTP/2 multiplexing**: Multiple RPC calls can fly over a single TCP connection, reducing connection overhead.
<!-- [LINE EDIT] "can fly over a single TCP connection" → "share a single TCP connection" — drops the colloquial "fly". -->
- **Strong typing end-to-end**: The `.proto` file is the source of truth. Client and server code is generated from it. Type mismatches are caught at compile time, not at 2am in production.
<!-- [COPY EDIT] "2am" → "2 a.m." (CMOS 9.38, time of day uses periods in lower-case "a.m./p.m."); or "2 AM" in technical contexts. Recommend "2 a.m." -->
- **Code generation**: You don't write HTTP clients, parse response bodies, or build URL paths. You call a method on a generated stub.
- **Bidirectional streaming**: gRPC supports four call types — unary (standard request/response), server streaming, client streaming, and bidirectional streaming. Most of our service calls are unary, but the option is there if you need to stream a large result set.
- **Built-in deadlines and cancellation**: gRPC propagates `context.Context` cancellation and deadlines across the network boundary automatically.

<!-- [LINE EDIT] "external HTTP requests from the browser hit the API Gateway (or directly the service, depending on routing)" — awkward parenthetical. Suggest: "External HTTP requests reach the API Gateway (or, for some routes, a service directly)." -->
In our system, external HTTP requests from the browser hit the API Gateway (or directly the service, depending on routing). Internal calls — like the Reservation service asking the Catalog service to update book availability — use gRPC.

---

## Walking Through `catalog.proto`

<!-- [STRUCTURAL] Good chunking — each language feature gets its own micro-section with the snippet it explains immediately above the prose. This is the right density for code-heavy tutorial material. -->
The full file lives at `proto/catalog/v1/catalog.proto`. Let's go through it piece by piece.

```protobuf
syntax = "proto3";
```

<!-- [LINE EDIT] "Proto3 is the current version and the one you should use for new projects." — slightly wordy. "Proto3 is current; use it for new projects." tightens. -->
<!-- [COPY EDIT] "proto3" vs "Proto3": the language spec uses lowercase "proto3" throughout. Recommend "proto3" (lowercase) consistently except at start of sentence, where it is already capitalized. -->
This declares proto3 syntax. Proto3 is the current version and the one you should use for new projects. The most notable change from proto2: all fields are optional by default and have zero-value defaults (empty string, 0, false). There's no `required` keyword.
<!-- [COPY EDIT] "There's no `required` keyword." — proto3 actually reintroduced explicit field presence (the `optional` keyword and, via editions, presence semantics) in recent releases. Please verify: is it still accurate to say "all fields are optional by default" without mentioning the reintroduced `optional` keyword (proto3.15+)? -->

```protobuf
package catalog.v1;
```

<!-- [LINE EDIT] "when a breaking change is unavoidable, you'd create a `v2` package rather than silently changing the existing one" — shift from "you'd" (conditional) is fine, but consider "you create" for imperative register matching surrounding prose. -->
The package name defines the protobuf namespace. It prevents naming collisions when multiple proto files are compiled together. The `v1` suffix is an API versioning convention — when a breaking change is unavoidable, you'd create a `v2` package rather than silently changing the existing one.

```protobuf
option go_package = "github.com/fesoliveira014/library-system/gen/catalog/v1;catalogv1";
```

This is a Go-specific option. It tells the code generator two things:

- The import path for the generated package (`github.com/fesoliveira014/library-system/gen/catalog/v1`)
- The package name to use in the generated Go file (`catalogv1`, after the semicolon)

<!-- [LINE EDIT] "Without this, the generated code would be placed in a location that's harder to import correctly." → "Without this, the generator picks a default path that's awkward to import." — more active. -->
Without this, the generated code would be placed in a location that's harder to import correctly.

```protobuf
import "google/protobuf/timestamp.proto";
```

<!-- [COPY EDIT] "canonical way" — fine. "google.protobuf.Timestamp" in body text should be in code font or italicized; it is already in code font. Good. -->
Protobuf has a standard library of well-known types. `google.protobuf.Timestamp` is the canonical way to represent timestamps — it's a message with `seconds` (int64) and `nanos` (int32) fields. You import it the same way you'd import a dependency in any language.

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

<!-- [LINE EDIT] "A `service` block defines the RPC interface — equivalent to a Java `interface` or a Kotlin `interface` annotated with some RPC framework." — "annotated with some RPC framework" is vague. Suggest: "equivalent to a Java or Kotlin interface annotated for an RPC framework (e.g., gRPC's @GrpcService)." -->
A `service` block defines the RPC interface — equivalent to a Java `interface` or a Kotlin `interface` annotated with some RPC framework. Each `rpc` line is one method: its name, request message type, and response message type.

<!-- [LINE EDIT] "Notice that even `DeleteBook` returns a message (`DeleteBookResponse`) instead of nothing." — the word "nothing" is a bit informal; consider "rather than a void/Empty type." -->
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

<!-- [COPY EDIT] "= 1, = 2, etc. are field numbers" — "etc." should have a comma before it in American style when it ends a list mid-sentence (CMOS 6.20). The sentence is fine as written. -->
The `= 1`, `= 2`, etc. are **field numbers**, not default values. They are how protobuf identifies fields on the wire. Field 1 is encoded differently than field 11 — and crucially, field 1 will always mean `id` for any client that speaks this protocol, regardless of what you name the field in the future.

<!-- [COPY EDIT] "Field numbers 1–15 use one byte of encoding overhead; fields 16–2047 use two bytes." — en dash for number ranges, correct (CMOS 6.78). Number literals "1", "15", "16", "2047" properly numeric (technical, CMOS 9.2). Good. -->
<!-- [COPY EDIT] "Please verify: 'Field numbers 1–15 use one byte of encoding overhead; fields 16–2047 use two bytes.' — the protobuf encoding reference gives 1–15 = 1 byte and 16–2047 = 2 bytes for the tag (field number + wire type), but subsequent ranges exist (2048–262143 use 3 bytes, etc.). Accurate for the ranges stated; just confirming. -->
Field numbers 1–15 use one byte of encoding overhead; fields 16–2047 use two bytes. Put your most-frequently-used fields in the lower numbers. Field numbers must be unique within a message and must never be reused once a field has been removed (use `reserved` to prevent accidental reuse).
<!-- [COPY EDIT] "most-frequently-used fields" — triple-compound adjective before noun: hyphenate all three elements (CMOS 7.81). "most-frequently-used" is correctly hyphenated. -->

### Repeated Fields

```protobuf
message ListBooksResponse {
  repeated Book books = 1;
  int32 total_count = 2;
}
```

<!-- [LINE EDIT] "On the wire it encodes as a sequence of values for that field number." — consider "On the wire, it encodes as a sequence of values tagged with that field number." for precision. -->
`repeated` is protobuf's equivalent of a list/array. On the wire it encodes as a sequence of values for that field number. In the generated Go code, it becomes a slice: `Books []*Book`.

---

## The buf Toolchain

<!-- [STRUCTURAL] Good motivation for buf — you pose the problem before selling the solution. This is exactly the right teaching pattern. -->
<!-- [LINE EDIT] "In theory you can use the raw `protoc` compiler with downloaded plugins to generate code." — "In theory" is acceptable, but consider starting with the negative claim more directly: "You can use the raw `protoc` compiler with downloaded plugins. In practice, its dependency management is painful:" -->
In theory you can use the raw `protoc` compiler with downloaded plugins to generate code. In practice, `protoc` dependency management is painful:

<!-- [COPY EDIT] Bullet parallelism: four items all begin with "You" or "There's" — mildly mixed. The third bullet "There's no built-in linting..." breaks the "You..." pattern. Consider reflowing or accept as varied parallel structure. -->
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

<!-- [COPY EDIT] "This catches things like: inconsistent field naming conventions, missing comments on public RPCs, package name mismatches, and more." — colon after "like" is borderline (CMOS 6.63 allows colon before list); fine. Serial comma present before "and more" (CMOS 6.19). Good. -->
- `lint.use: [STANDARD]` enables buf's standard lint ruleset. This catches things like: inconsistent field naming conventions, missing comments on public RPCs, package name mismatches, and more.
<!-- [LINE EDIT] "we have some RPCs (like `CreateBook`) that return `Book` directly, which is valid design" — "which is valid design" is weak. Suggest: "which is an intentional design choice." -->
- The two `except` entries suppress rules we've deliberately relaxed. `RPC_RESPONSE_STANDARD_NAME` would require every response type to be named `<RpcName>Response` — we have some RPCs (like `CreateBook`) that return `Book` directly, which is valid design.
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

- `buf.build/protocolbuffers/go` — generates the message structs (`catalog.pb.go`)
- `buf.build/grpc/go` — generates the client and server interfaces (`catalog_grpc.pb.go`)

<!-- [COPY EDIT] "paths=source_relative" — verify the flag name is current (it is, but confirm for buf v2 plugin config). -->
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

<!-- [LINE EDIT] "If `buf lint` passes silently, your proto design meets the standard rules. If it fails, you'll get actionable messages pointing to the exact line." — fine. Consider merging: "`buf lint` is silent on success and prints file:line violations on failure." -->
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

<!-- [COPY EDIT] "PascalCase" and "snake_case" — common technical terms; no styling change needed. -->
Notice that field names are PascalCase in Go (Go convention) even though they're snake_case in the proto file. The struct tags encode both the wire format information and JSON field names — the generated structs work with `encoding/json` as well as protobuf encoding.

<!-- [LINE EDIT] "Do not manually edit these files. The `// Code generated by protoc-gen-go. DO NOT EDIT.` header at the top means exactly what it says — any edits you make will be overwritten the next time you run `buf generate`." — consider splitting for rhythm: "Do not edit these files. The `// Code generated...DO NOT EDIT.` header is load-bearing — `buf generate` will overwrite anything you change." -->
Do not manually edit these files. The `// Code generated by protoc-gen-go. DO NOT EDIT.` header at the top means exactly what it says — any edits you make will be overwritten the next time you run `buf generate`.

### `gen/catalog/v1/catalog_grpc.pb.go`

This file contains the server interface you must implement and the client stub you'll call from other services.

**The server interface** — this is what you implement in the Catalog service:

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

<!-- [STRUCTURAL] The explanation of `mustEmbedUnimplementedCatalogServiceServer` and forward-compat semantics is excellent — this is one of those Go-idiom details that confuses Java/Kotlin-trained developers, and the rationale is given. -->
<!-- [LINE EDIT] "This is a forward-compatibility mechanism: when new RPC methods are added to the proto, your service won't fail to compile — the embedded `Unimplemented...` struct provides default implementations that return `codes.Unimplemented`. You can then add the real implementations incrementally." — sentence is 47 words and chains three clauses. Consider: "This is a forward-compatibility mechanism. When new RPC methods are added to the proto, your service still compiles — the embedded `Unimplemented...` struct returns `codes.Unimplemented` for unknown methods, letting you add real implementations incrementally." -->
The `mustEmbedUnimplementedCatalogServiceServer()` unexported method forces you to embed `UnimplementedCatalogServiceServer` in your implementation struct. This is a forward-compatibility mechanism: when new RPC methods are added to the proto, your service won't fail to compile — the embedded `Unimplemented...` struct provides default implementations that return `codes.Unimplemented`. You can then add the real implementations incrementally.

**The client stub** — this is what other services use to call Catalog:

```go
type CatalogServiceClient interface {
    CreateBook(ctx context.Context, in *CreateBookRequest, opts ...grpc.CallOption) (*Book, error)
    GetBook(ctx context.Context, in *GetBookRequest, opts ...grpc.CallOption) (*Book, error)
    // ...
}

func NewCatalogServiceClient(cc grpc.ClientConnInterface) CatalogServiceClient { ... }
```

<!-- [LINE EDIT] "as if it were a local function call" — standard RPC framing; fine. -->
The Reservation service, for example, will call `NewCatalogServiceClient(conn)` with a gRPC connection and then call `client.UpdateAvailability(ctx, req)` as if it were a local function call. The network transport is entirely hidden.

---

## Exercise: Add a `SearchBooks` RPC

<!-- [STRUCTURAL] Exercise scope is tight (proto-only, no Go implementation) and the success criteria are explicit. Good. -->
The system will eventually have a Search service, but the Catalog service needs to expose a search endpoint first. Your task is to add a `SearchBooks` RPC to `proto/catalog/v1/catalog.proto`.

Requirements:
- The request message should accept a `query` string (the search term)
- The response should reuse the existing `ListBooksResponse` (it already has `repeated Book books` and `int32 total_count`)
- Run `buf lint` from the `proto/` directory to verify there are no style violations
- Run `buf generate` to regenerate the Go code

<!-- [LINE EDIT] "This is proto-only work — don't implement the Go handler yet. The point is to practice the full proto edit → lint → generate cycle." — good; leave. -->
This is proto-only work — don't implement the Go handler yet. The point is to practice the full proto edit → lint → generate cycle.

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

<!-- [LINE EDIT] "will include `SearchBooks` in both the `CatalogServiceServer` interface and the `CatalogServiceClient` interface" — redundant "interface". Tighten: "will include `SearchBooks` in both the server and client interfaces". -->
After regeneration, `catalog_grpc.pb.go` will include `SearchBooks` in both the `CatalogServiceServer` interface and the `CatalogServiceClient` interface. The `UnimplementedCatalogServiceServer` will have a default implementation returning `codes.Unimplemented`, so the service compiles without changes.

Note: `buf breaking` would flag this as safe — adding a new RPC is not a breaking change. Removing one would be.

</details>

---

## Summary

<!-- [STRUCTURAL] Summary bullets mirror the section structure — useful for spaced repetition. Good. -->
- Protocol Buffers is a binary IDL-driven serialization format. `.proto` files define the schema; code generators produce language-specific implementations.
- gRPC is the RPC framework of choice for internal service calls: type-safe, fast, with built-in support for streaming and deadline propagation.
- Field numbers — not names — define wire identity. Never reuse them after removal.
- `buf` replaces raw `protoc` with a dependency-managed, linting, breaking-change-detecting build tool.
<!-- [LINE EDIT] "dependency-managed, linting, breaking-change-detecting build tool" — three compound modifiers stacked becomes hard to parse. Suggest: "`buf` replaces raw `protoc` with a build tool that manages dependencies, lints proto files, and detects breaking changes." -->
- Generated code lives in `gen/` and must not be manually edited. The server interface is what you implement; the client stub is what callers use.

---

## References

[^1]: [Protocol Buffers Language Guide (proto3)](https://protobuf.dev/programming-guides/proto3/)
[^2]: [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
[^3]: [buf documentation](https://buf.build/docs/)
<!-- [COPY EDIT] Please verify: URLs for all three footnotes are live and canonical. `https://buf.build/docs/` is valid but buf has been reorganizing their docs — check whether a more specific landing page is now preferred. -->
