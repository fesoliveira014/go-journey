<!-- [STRUCTURAL] Index is brief and scannable — appropriate for a chapter TOC. Consider adding a one-line "prerequisites" note (e.g., "assumes familiarity with Go modules and Docker from Chapter 1") to set the floor. -->
# Chapter 2: First Microservice — Catalog Service

<!-- [STRUCTURAL] Lede paragraph is compact and does the job. The final clause "uses versioned SQL migrations" is slightly orphaned — consider rolling it into a parallel list with gRPC and GORM, or integrating with a stronger verb. -->
<!-- [LINE EDIT] "uses versioned SQL migrations" → "and manages schema with versioned SQL migrations" — parallels the preceding active-verb clauses. -->
In this chapter, we build the first standalone microservice: the **Catalog service**. It manages the book registry with full CRUD operations, exposes a gRPC API, stores data in PostgreSQL via GORM, and uses versioned SQL migrations.

## What You'll Learn

<!-- [STRUCTURAL] Five-item learning-outcomes list maps cleanly onto the five sections below — good alignment. -->
<!-- [COPY EDIT] "You'll" heading: CMOS 8.159 allows contractions in headings when the voice is conversational; consistent across the chapter, so leave as-is. -->
- Protocol Buffers and gRPC fundamentals
- PostgreSQL interaction via GORM with the repository pattern
- Database migrations with golang-migrate
- Layered service architecture (handler → service → repository)
- Testing strategies for each layer

## Architecture Overview

<!-- [STRUCTURAL] ASCII diagram is the right call for a plaintext manuscript, but the diagram shows handler → service → repository → PostgreSQL while the Sections list below starts with Protobuf/gRPC and ends with wiring. Briefly note that sections are presented in build order (bottom-up), not call order, to avoid cognitive whiplash. -->
The Catalog service follows a clean layered architecture:

```
gRPC Request
    ↓
┌─────────────────────────┐
│   Handler Layer         │  Protobuf ↔ Domain conversion
│   (internal/handler/)   │  Error mapping to gRPC codes
└─────────┬───────────────┘
          ↓
┌─────────────────────────┐
│   Service Layer         │  Business logic & validation
│   (internal/service/)   │  Defines repository interface
└─────────┬───────────────┘
          ↓
┌─────────────────────────┐
│   Repository Layer      │  GORM queries & error translation
│   (internal/repository/)│  Implements repository interface
└─────────┬───────────────┘
          ↓
    PostgreSQL
```

<!-- [LINE EDIT] "Each layer depends only on the layer below it through interfaces, making the code testable and maintainable." → "Each layer depends on the layer below it only through an interface, which keeps every layer independently testable." — tightens and drops the generic "maintainable" filler. -->
Each layer depends only on the layer below it through interfaces, making the code testable and maintainable.

## Sections

<!-- [STRUCTURAL] Section descriptions are parallel and well-scoped. Consider marking approximate reading time or difficulty per section for the self-paced learner. -->
<!-- [COPY EDIT] Use em dashes without spaces per CMOS 6.85 — the current form already does this. Consistent. -->
1. **[Protocol Buffers & gRPC](./protobuf-grpc.md)** — Define the service API using protobuf, generate Go code with buf
2. **[PostgreSQL & Migrations](./postgresql-migrations.md)** — Set up the database, write versioned migrations
3. **[The Repository Pattern with GORM](./repository-pattern.md)** — Implement data access with GORM
<!-- [COPY EDIT] "The Repository Pattern with GORM" uses title case with leading article; items 1, 2, 4, 5 omit articles. Recommend "Repository Pattern with GORM" for parallelism. -->
4. **[Service Layer & Business Logic](./service-layer.md)** — Business rules, validation, interfaces
5. **[Wiring It All Together](./wiring.md)** — gRPC server setup, dependency injection, testing with grpcurl
