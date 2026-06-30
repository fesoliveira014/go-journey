# Chapter 2: First Microservice—Catalog Service

> **Chapter checkpoint**
> Start from: `git checkout chapter-02-start`
> End state: `git checkout chapter-02-end`
>
> Chapter snippets are point-in-time snapshots. Later chapters intentionally change the same files.

In this chapter, we build the first standalone microservice: the **Catalog service**. It manages the book registry with full CRUD operations and exposes a gRPC API. Data is stored in PostgreSQL via GORM, and the schema is managed with versioned SQL migrations.

## Assumed Knowledge

You should be comfortable reading HTTP handlers and Go structs from Chapter 1. You do not need prior gRPC, protobuf, GORM, or PostgreSQL migration experience; those are introduced here.

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

## Skim Path

If you have already built gRPC services, skim Section 2.1 for this project's proto conventions and code-generation commands. If you know PostgreSQL migrations, skim Section 2.2 but read the `embed` and startup-migration notes. If you have used an ORM before, focus on Section 2.3's repository boundary and error translation rather than the CRUD mechanics.

## Sections

1. **[Protocol Buffers & gRPC](./protobuf-grpc.md)**—Define the service API using protobuf, generate Go code with buf
2. **[PostgreSQL & Migrations](./postgresql-migrations.md)**—Set up the database, write versioned migrations
3. **[Repository Pattern with GORM](./repository-pattern.md)**—Implement data access with GORM
4. **[Service Layer & Business Logic](./service-layer.md)**—Business rules, validation, interfaces
5. **[Wiring It All Together](./wiring.md)**—gRPC server setup, dependency injection, testing with grpcurl
