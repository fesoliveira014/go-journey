# Library Management System — Architecture Design

## Overview

A microservices-based library management system built in Go, serving as a learning project that covers microservices architecture, containerization, orchestration, observability, and CI/CD. The system is also a tutorial — each component maps to a chapter in a step-by-step guide delivered as Markdown and static HTML (GitHub Pages).

## Goals

- Teach microservices architecture through a realistic but manageable domain
- Cover the full stack: Go services, PostgreSQL, Kafka, gRPC, Docker, Kubernetes, Terraform, OpenTelemetry, GitHub Actions + Earthly
- Produce a hostable tutorial with chapters, exercises, diagrams, and external references
- Deploy locally first (kind), then to AWS EKS as the final chapter

## Architecture Approach

**Event-driven first.** Services communicate primarily through Kafka events. gRPC is used only for synchronous queries where an immediate response is required (e.g., token validation, availability checks). This maximizes learning coverage of async patterns while keeping synchronous paths where they naturally belong.

## Services

### 1. Gateway Service

**Role:** The only public-facing service. Serves the HTMX + Go templates frontend, manages sessions, and routes requests to backend services via gRPC.

**Responsibilities:**
- Serve HTML pages using Go `html/template` + HTMX for interactivity
- Cookie-based session management (JWT tokens validated via Auth service)
- Route and aggregate requests to internal services
- Rate limiting and request validation

**HTTP endpoints (external):**
- `GET /` — homepage
- `GET /login`, `GET /register`, `POST /login`, `POST /register` — auth flows
- `GET /oauth2/callback` — Gmail OAuth2 callback
- `GET /catalog` — browse books
- `GET /search?q=` — search
- `POST /books/{id}/reserve`, `POST /books/{id}/return` — reservation actions
- `GET /admin/books`, `POST /admin/books` — admin CRUD

**No database.** Stateless except for session cookies.

### 2. Auth Service

**Role:** Handles all authentication and user management. Issues and validates JWTs.

**Responsibilities:**
- User registration and login (email/password)
- OAuth2 flow with Gmail
- JWT issuance and validation
- User profile and role management (admin vs user)
- Password hashing (bcrypt)

**gRPC API:**
- `Register(email, password) → token`
- `Login(email, password) → token`
- `ValidateToken(token) → user_id, role`
- `GetUser(user_id) → user profile`
- `InitOAuth2() → redirect_url`
- `CompleteOAuth2(code) → token`

**Data model (PostgreSQL):**

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| email | VARCHAR UNIQUE | |
| password | VARCHAR | Nullable — OAuth-only users have no password |
| name | VARCHAR | |
| role | ENUM('user', 'admin') | Default 'user' |
| oauth_provider | VARCHAR | 'google' or null |
| oauth_id | VARCHAR | Provider's user ID |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**Publishes:** `auth.users.created` — consumed by future Notification service.

### 3. Catalog Service

**Role:** Manages the book registry. Owns all book metadata and availability counts.

**Responsibilities:**
- Book CRUD (admin operations)
- Book metadata management
- Availability tracking (total copies, available copies)
- Genre/category management

**gRPC API:**
- `CreateBook(book) → book`
- `UpdateBook(id, book) → book`
- `DeleteBook(id) → ok`
- `GetBook(id) → book`
- `ListBooks(filter, page) → books[]`
- `UpdateAvailability(id, delta) → count`

**Data model (PostgreSQL):**

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| title | VARCHAR | |
| author | VARCHAR | |
| isbn | VARCHAR UNIQUE | |
| genre | VARCHAR | |
| description | TEXT | |
| published_year | INTEGER | |
| total_copies | INTEGER | Default 1 |
| available_copies | INTEGER | Default 1 |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**Publishes:** `catalog.books.changed` — consumed by Search service.
**Consumes:** `reservations.created`, `reservations.returned` — updates available_copies.

### 4. Reservation Service

**Role:** Manages book reservations, returns, and lease extensions.

**Responsibilities:**
- Book reservation (create lease)
- Book return
- Lease extension
- User reservation history
- Overdue tracking

**gRPC API:**
- `ReserveBook(user_id, book_id) → reservation`
- `ReturnBook(reservation_id) → ok`
- `ExtendLease(reservation_id, days) → reservation`
- `GetReservation(id) → reservation`
- `ListUserReservations(user_id) → reservations[]`

**Data model (PostgreSQL):**

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID | |
| book_id | UUID | |
| status | ENUM('active', 'returned', 'overdue', 'rejected') | |
| reserved_at | TIMESTAMP | |
| due_at | TIMESTAMP | |
| returned_at | TIMESTAMP | Nullable |
| extensions | INTEGER | Default 0, max 2 |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**Publishes:** `reservations.created`, `reservations.returned`.
**Calls:** `Catalog.GetBook` (gRPC) — verifies book exists before reserving.

### 5. Search Service

**Role:** Provides full-text search over the book catalog via Meilisearch.

**Responsibilities:**
- Full-text search over book catalog
- Faceted filtering (genre, author, availability)
- Keep Meilisearch index in sync via Kafka
- Search suggestions / autocomplete

**gRPC API:**
- `Search(query, filters, page) → results[]`
- `Suggest(prefix) → suggestions[]`

**No database.** Meilisearch is the only data store. If the index is lost, it can be rebuilt by replaying Kafka or requesting a full dump from Catalog.

**Consumes:** `catalog.books.changed` — indexes/removes books in Meilisearch.

## Communication Patterns

### Synchronous (gRPC)

Used when the caller needs an immediate response:

- Gateway → Auth (token validation on every request)
- Gateway → Catalog (book queries, CRUD)
- Gateway → Reservation (reserve/return)
- Gateway → Search (search queries)
- Reservation → Catalog (verify book exists and is available before reserving)

### Asynchronous (Kafka)

Used for cross-service state propagation where eventual consistency is acceptable:

- Catalog → Search (book created/updated/deleted)
- Reservation → Catalog (availability changes)
- Auth → * (user created — future Notification service)
- Reservation → * (lease expiring — future Notification service)

## Kafka Topics

### `catalog.books.changed`
- **Key:** book_id
- **Producer:** Catalog
- **Consumers:** Search
- **Payload:** `{ event: "book.created|book.updated|book.deleted", book_id, title, author, genre, available_copies, timestamp }`

### `reservations.created`
- **Key:** book_id
- **Producer:** Reservation
- **Consumers:** Catalog
- **Payload:** `{ event: "reservation.created", reservation_id, book_id, user_id, due_at, timestamp }`

### `reservations.returned`
- **Key:** book_id
- **Producer:** Reservation
- **Consumers:** Catalog
- **Payload:** `{ event: "reservation.returned", reservation_id, book_id, returned_at, timestamp }`

### `auth.users.created`
- **Key:** user_id
- **Producer:** Auth
- **Consumers:** None yet (future: Notification)
- **Payload:** `{ event: "user.created", user_id, email, name, timestamp }`

## Key Design Decisions

**Message keys = entity IDs.** All events for the same book land in the same Kafka partition (keyed by book_id). This guarantees ordering per book.

**Events carry full state.** The `catalog.books.changed` event includes all book fields, not just the book_id. Search can index directly from the event without calling back to Catalog.

**Eventual consistency is acceptable.** Search results may be slightly stale (sub-second). The reservation flow validates availability via gRPC before confirming, so correctness is maintained.

**Race condition handling.** Two users could attempt to reserve the last copy simultaneously. Catalog handles this — when it processes a reservation event and available_copies would go below 0, it publishes a `reservations.rejected` event (payload: `{ reservation_id, book_id, reason: "unavailable" }`). The Reservation service consumes this event and updates the reservation status to `rejected`. The Gateway can then inform the user. This is an explicit teaching point in the tutorial.

## Event Flows

### Flow 1: Admin Creates a Book

1. Gateway → Catalog (gRPC: CreateBook)
2. Catalog saves to PostgreSQL, returns response to Gateway
3. Catalog publishes `catalog.books.changed` (event: book.created)
4. Search consumes event, indexes book in Meilisearch

### Flow 2: User Reserves a Book

1. Gateway → Reservation (gRPC: ReserveBook)
2. Reservation → Catalog (gRPC: GetBook — verify exists and available)
3. Reservation saves reservation to PostgreSQL
4. Reservation publishes `reservations.created`
5. Catalog consumes event, decrements available_copies
6. Catalog publishes `catalog.books.changed` (updated availability)
7. Search consumes event, re-indexes book with new availability

## Infrastructure

### Local Development (Docker Compose)

**Application containers:**
- gateway (:8080), auth-service (:50051), catalog-service (:50052), reservation-service (:50053), search-service (:50054)

**Infrastructure containers:**
- postgres-auth (:5432), postgres-catalog (:5433), postgres-reservation (:5434)
- kafka (:9092), zookeeper (:2181)
- meilisearch (:7700)

Hot-reload via `air` (Go live reload). Each service has a multi-stage Dockerfile. Docker Compose uses Zookeeper for Kafka because it is simpler with existing images; Kubernetes uses KRaft mode to avoid the Zookeeper dependency — this difference is covered in the Kafka chapter.

### Kubernetes

Three namespaces:
- **library** — Deployments for all 5 services, ClusterIP Services, Ingress pointing to Gateway
- **data** — StatefulSets for PostgreSQL instances and Meilisearch, with PVCs
- **messaging** — StatefulSet for Kafka in KRaft mode (no Zookeeper), with PVC

Progression: `kind` locally → EKS in production. Same manifests for both.

### Terraform (AWS)

Modules: `vpc`, `eks`, `rds` (PostgreSQL), `ecr` (container registry).
Environments: `dev` (small instances), `prod` (full setup).

In production, PostgreSQL moves to RDS. Kafka and Meilisearch remain self-hosted on EKS to keep costs manageable for a learning project.

### Observability

All services export telemetry via OTLP to an OpenTelemetry Collector, which routes to:
- **Traces:** Jaeger — distributed tracing across gRPC calls and Kafka events
- **Metrics:** Prometheus + Grafana — request latency, error rates, Kafka consumer lag, DB connection pool
- **Logs:** Loki + Grafana — structured JSON logs correlated with trace IDs

Key principle: every log line includes a trace ID. Traces, metrics, and logs are cross-linked in Grafana.

### CI/CD (GitHub Actions + Earthly)

Pipeline stages:
1. `earthly +lint` — golangci-lint on all services
2. `earthly +test` — unit + integration tests (testcontainers)
3. `earthly +build` — multi-stage Docker builds per service
4. `earthly +push` — push images to ECR (main branch only)
5. `kubectl apply` — rolling update on EKS (main branch only)

Each service has its own Earthfile. The root Earthfile orchestrates all services.

## Project Structure

```
library-system/
  services/
    gateway/
    auth/
    catalog/
    reservation/
    search/
  proto/                    # shared .proto files
  pkg/                      # shared Go libraries
    otel/                   # OpenTelemetry init
    kafka/                  # producer/consumer wrappers
    grpcutil/               # common interceptors
    config/                 # env-based config loading
  deploy/
    docker-compose.yml
    k8s/                    # Kubernetes manifests
    terraform/              # IaC modules
  docs/                     # tutorial content
  Earthfile
  go.work                   # Go workspace
```

Each service follows:
```
services/<name>/
  cmd/
    main.go
  internal/
    handler/                # gRPC handlers
    service/                # business logic
    repository/             # DB access
    kafka/                  # producer/consumer
    model/                  # domain types
  migrations/               # SQL migrations
  Dockerfile
  Earthfile
  go.mod
```

Monorepo with Go workspaces (`go.work`). Each service has its own `go.mod` but shares proto files and common libraries.

## Tutorial Chapter Outline

1. **Go Foundations** — project setup, modules, error handling, interfaces, testing basics, minimal HTTP server
2. **First Microservice: Catalog** — PostgreSQL, sqlc/pgx, migrations, repository pattern, gRPC server, protobuf
3. **Containerization** — multi-stage Dockerfile, Docker Compose, networking, hot-reload dev workflow
4. **Auth Service** — JWT, bcrypt, OAuth2 with Gmail, role-based access, gRPC auth interceptors
5. **Gateway & Frontend** — HTMX + Go templates, session management, BFF pattern
6. **Event-Driven Architecture with Kafka** — Kafka fundamentals, Go producers/consumers, event schemas, idempotency
7. **Reservation Service & Search** — reservation logic, Meilisearch, Kafka-driven indexing, eventual consistency
8. **Observability** — OpenTelemetry SDK, distributed traces, Prometheus metrics, structured logging, Grafana dashboards
9. **Testing Strategies** — unit tests, integration tests (testcontainers), gRPC testing, Kafka testing, e2e tests
10. **CI/CD with Earthly & GitHub Actions** — Earthfiles, GitHub Actions workflow, automated testing and image builds
11. **Kubernetes** — K8s concepts, kind, manifests (Deployments, Services, Ingress, StatefulSets, ConfigMaps, Secrets)
12. **Terraform & Cloud Deployment** — Terraform fundamentals, AWS modules, deploying to EKS, cost management

## Deliverable Format

**Markdown (docs/):** One file per chapter with code blocks, exercises (collapsible solutions), Mermaid diagrams, and footnoted references.

**Static HTML (GitHub Pages):** Generated from Markdown via mdBook (simpler setup, built-in sidebar navigation, Mermaid support via plugin). Sidebar navigation, syntax highlighting, rendered Mermaid diagrams, mobile-responsive layout.

## Future Additions (Not in Initial Scope)

- **Notification Service** — Kafka consumer for lease expiry reminders, reservation confirmations. Clean addition since it subscribes to existing topics.
- **Rate limiting / circuit breakers** — could be added to Gateway and gRPC clients.
- **Horizontal scaling** — Kafka consumer groups, multiple service replicas.
