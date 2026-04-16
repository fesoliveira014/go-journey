# Chapter 2: First Microservice — Catalog Service

In this chapter, we build the first standalone microservice: the **Catalog service**. It manages the book registry with full CRUD operations, exposes a gRPC API, stores data in PostgreSQL via GORM, and manages schema with versioned SQL migrations.

## What You'll Learn

- Protocol Buffers and gRPC fundamentals
- PostgreSQL interaction via GORM with the repository pattern
- Database migrations with golang-migrate
- Layered service architecture (handler → service → repository)
- Testing strategies for each layer

## Architecture Overview

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

Each layer depends on the layer below it only through an interface, which keeps every layer independently testable.

## Sections

1. **[Protocol Buffers & gRPC](./protobuf-grpc.md)** — Define the service API using protobuf, generate Go code with buf
2. **[PostgreSQL & Migrations](./postgresql-migrations.md)** — Set up the database, write versioned migrations
3. **[Repository Pattern with GORM](./repository-pattern.md)** — Implement data access with GORM
4. **[Service Layer & Business Logic](./service-layer.md)** — Business rules, validation, interfaces
5. **[Wiring It All Together](./wiring.md)** — gRPC server setup, dependency injection, testing with grpcurl
