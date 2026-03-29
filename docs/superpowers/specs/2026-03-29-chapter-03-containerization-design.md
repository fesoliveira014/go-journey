# Chapter 3: Containerization — Design Spec

## Overview

Containerize the existing services (gateway and catalog) with Docker, orchestrate them with Docker Compose, and set up a hot-reload development workflow using air. This chapter teaches Docker fundamentals through the lens of the library system, building on the services created in Chapters 1–2.

## Goals

- Teach Docker fundamentals (images, containers, layers, multi-stage builds)
- Teach Docker Compose for multi-container orchestration
- Set up a complete local development environment with hot-reload
- Handle the monorepo build context challenge (go.work, gen/, multiple services)
- Produce working Docker Compose config that starts gateway + catalog + PostgreSQL

## Scope

**In scope:** Gateway, Catalog, and their PostgreSQL database. Only services that exist get containerized.

**Out of scope:** Auth, Reservation, and Search services (future chapters). Kafka and Meilisearch containers (added when those services are built). Kubernetes (Chapter 11). CI/CD Docker integration (Chapter 10).

## Key Design Decisions

- **Incremental containerization:** Only gateway + catalog + PostgreSQL. Future chapters add services to the compose file as they're built.
- **Dev Dockerfiles with air:** Each service gets a `Dockerfile.dev` with air for hot-reload inside containers, teaching the full Docker dev workflow.
- **Deploy directory:** Docker Compose files live in `deploy/`, following the architecture spec. Keeps deployment config grouped with future K8s manifests and Terraform.
- **Multi-stage COPY for monorepo:** Dockerfiles selectively copy `gen/` and their own `services/<name>/` directory with `GOWORK=off`. Combined with a root `.dockerignore` to keep build context small.

## Docker Compose Architecture

### Containers

| Container | Image | Ports | Purpose |
|---|---|---|---|
| `postgres-catalog` | `postgres:16-alpine` | 5433:5432 | Catalog service database |
| `catalog` | built from `services/catalog/Dockerfile` | 50052:50052 | gRPC Catalog service |
| `gateway` | built from `services/gateway/Dockerfile` | 8080:8080 | HTTP Gateway |

### Networking

A single `library-net` bridge network. Services reference each other by container name:
- Catalog connects to `postgres-catalog:5432`
- Gateway will eventually call catalog at `catalog:50052` (not wired yet — gateway doesn't call catalog until later chapters)

### Startup Order

`catalog` depends on `postgres-catalog` with a healthcheck:

```yaml
postgres-catalog:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U postgres"]
    interval: 5s
    timeout: 5s
    retries: 5

catalog:
  depends_on:
    postgres-catalog:
      condition: service_healthy
```

Gateway has no database dependency and starts independently.

### Environment Variables

Managed via `deploy/.env`:

```env
POSTGRES_CATALOG_PORT=5433
POSTGRES_CATALOG_USER=postgres
POSTGRES_CATALOG_PASSWORD=postgres
POSTGRES_CATALOG_DB=catalog

GATEWAY_PORT=8080
CATALOG_GRPC_PORT=50052
```

Services read configuration from environment variables (already implemented in catalog's `main.go` with `DATABASE_URL` and `GRPC_PORT`).

## Dockerfile Strategy

### Production Dockerfiles (`services/<name>/Dockerfile`)

Multi-stage builds for minimal final images:

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service and gen/, not all
# workspace members. The replace directive in go.mod handles the gen/ import.
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY services/<name>/go.mod services/<name>/go.sum* ./services/<name>/

# 2. Download dependencies (cached unless go.mod changes)
WORKDIR /app/services/<name>
RUN go mod download

# 3. Copy source code (invalidates cache only when source changes)
WORKDIR /app
COPY gen/ ./gen/
COPY services/<name>/ ./services/<name>/

# 4. Build static binary
WORKDIR /app/services/<name>
RUN CGO_ENABLED=0 go build -o /bin/<name> ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/<name> /usr/local/bin/<name>
EXPOSE <port>
ENTRYPOINT ["/usr/local/bin/<name>"]
```

Key points:
- **Root build context:** Docker Compose sets `context: ../..` (project root) so COPY can reach `gen/`
- **`GOWORK=off`:** Disables Go workspace mode inside the container. The `go.work` file lists all workspace members, but we only copy one service. With workspace mode off, the `replace` directive in each service's `go.mod` resolves the `gen/` import. This is a key monorepo Docker pattern — the build must be self-contained per service.
- **Selective COPY:** Only copies what the service needs, not the entire monorepo
- **Two-phase COPY for cache efficiency:** First copy only `go.mod`/`go.sum` and download dependencies. Then copy source code. This means source code changes don't invalidate the dependency download cache — a standard Go Docker optimization.
- **Static binary:** `CGO_ENABLED=0` produces a self-contained binary that runs on alpine without glibc

The catalog Dockerfile uses this pattern exactly. The gateway Dockerfile is simpler — it does not depend on the `gen/` module today (no gRPC client calls yet), so its COPY steps omit `gen/` and `GOWORK=off`. When future chapters add gRPC client calls to the gateway, its Dockerfile will be updated to match the full pattern. This difference is a teaching opportunity: not every service needs the monorepo-aware build.

### Dev Dockerfiles (`services/<name>/Dockerfile.dev`)

Single-stage with air for hot-reload. Like the production Dockerfiles, services that depend on `gen/` (catalog) use `GOWORK=off` and copy the shared module. The gateway dev Dockerfile is simpler since it has no `gen/` dependency.

**Catalog dev Dockerfile:**

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Disable workspace mode — same reason as production
ENV GOWORK=off

# Copy shared modules and service source
COPY gen/ ./gen/
COPY services/catalog/ ./services/catalog/

WORKDIR /app/services/catalog
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

**Gateway dev Dockerfile** (simpler — no gen/ dependency):

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app
COPY services/gateway/ ./services/gateway/

WORKDIR /app/services/gateway
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

At runtime, Docker Compose mounts the local source directories as volumes, overriding the COPY'd files. When you edit code locally, air detects the change and rebuilds inside the container.

### Air Configuration (`services/<name>/.air.toml`)

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor"]
  include_ext = ["go"]
  kill_delay = "0s"

[log]
  time = false

[misc]
  clean_on_exit = true
```

### `.dockerignore`

At the project root, excludes non-build files from the Docker build context:

```
.git/
.worktrees/
docs/
deploy/
**/*.md
.env*
**/.air.toml
**/tmp/
```

Note: `**/*.md` excludes Markdown files recursively. No `go.work` exception is needed — both production and dev Dockerfiles use `GOWORK=off` and rely on each service's `go.mod` `replace` directive instead.

## Docker Compose Files

### `deploy/docker-compose.yml` (production-like)

```yaml
services:
  postgres-catalog:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_CATALOG_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_CATALOG_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_CATALOG_DB:-catalog}
    ports:
      - "${POSTGRES_CATALOG_PORT:-5433}:5432"
    volumes:
      - catalog-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - library-net

  catalog:
    build:
      context: ../..
      dockerfile: services/catalog/Dockerfile
    environment:
      DATABASE_URL: "host=postgres-catalog port=5432 user=${POSTGRES_CATALOG_USER:-postgres} password=${POSTGRES_CATALOG_PASSWORD:-postgres} dbname=${POSTGRES_CATALOG_DB:-catalog} sslmode=disable"
      GRPC_PORT: "50052"
    ports:
      - "${CATALOG_GRPC_PORT:-50052}:50052"
    depends_on:
      postgres-catalog:
        condition: service_healthy
    networks:
      - library-net

  gateway:
    build:
      context: ../..
      dockerfile: services/gateway/Dockerfile
    environment:
      PORT: "8080"
    ports:
      - "${GATEWAY_PORT:-8080}:8080"
    networks:
      - library-net

volumes:
  catalog-data:

networks:
  library-net:
    driver: bridge
```

### `deploy/docker-compose.dev.yml` (development override)

```yaml
services:
  catalog:
    build:
      context: ../..
      dockerfile: services/catalog/Dockerfile.dev
    volumes:
      - ../../services/catalog:/app/services/catalog
      - ../../gen:/app/gen

  gateway:
    build:
      context: ../..
      dockerfile: services/gateway/Dockerfile.dev
    volumes:
      - ../../services/gateway:/app/services/gateway
```

Usage:
```bash
# Production-like (built images)
docker compose -f deploy/docker-compose.yml up --build

# Development (hot-reload)
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up --build
```

## Testing Strategy

- **Smoke test:** `docker compose up --build` starts all three containers without errors
- **Health check:** `curl http://localhost:8080/healthz` returns `{"status":"ok"}` from gateway
- **gRPC test:** `grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks` returns from catalog (through Docker networking to PostgreSQL)
- **Hot-reload test:** Edit a Go file, verify air rebuilds and the change takes effect without restarting containers
- **Integration tests:** Repository tests can now target the Docker PostgreSQL at `localhost:5433`

## Tutorial Chapter Outline

1. **3.1 Docker Fundamentals** — What containers are (compare to JVMs for the Java dev), images vs containers, layers and caching, multi-stage builds. Why containerize Go when it already produces static binaries (consistency, dependency isolation, deployment uniformity, matching prod environments).

2. **3.2 Writing Dockerfiles** — Walk through the production Dockerfile line by line. The monorepo build context challenge and how selective COPY solves it. Layer ordering for cache efficiency (dependencies before source). The `.dockerignore` file. Building and running a single container manually with `docker build` and `docker run`.

3. **3.3 Docker Compose** — What Compose solves (multi-container orchestration). The `docker-compose.yml` structure: services, networks, volumes. Service definitions, environment variables with `.env` defaults. Healthchecks and `depends_on`. Starting the stack with `docker compose up`, viewing logs, stopping.

4. **3.4 Development Workflow** — The dev override file pattern (`docker-compose.dev.yml`). Air for hot-reload: what it does, `.air.toml` configuration, volume mounts that make it work. The full dev command. Debugging tips: `docker compose logs -f`, `docker compose exec`, inspecting networks, port conflicts.

## File Structure

```
deploy/
├── docker-compose.yml              # Production-like: built images
├── docker-compose.dev.yml          # Dev override: air + volumes
└── .env                            # Default env vars

services/gateway/
├── Dockerfile                      # NEW: multi-stage production build
├── Dockerfile.dev                  # NEW: air hot-reload
└── .air.toml                       # NEW: air config

services/catalog/
├── Dockerfile                      # REWRITE: proper monorepo COPY
├── Dockerfile.dev                  # NEW: air hot-reload
└── .air.toml                       # NEW: air config

.dockerignore                       # NEW: root-level

docs/src/
├── SUMMARY.md                      # UPDATE: add Chapter 3 entries
└── ch03/
    ├── index.md
    ├── docker-fundamentals.md      # 3.1
    ├── writing-dockerfiles.md      # 3.2
    ├── docker-compose.md           # 3.3
    └── dev-workflow.md             # 3.4
```

## Dependencies

- Docker Engine 24+ (or Docker Desktop)
- Docker Compose v2 (included with Docker Desktop, or standalone `docker-compose-plugin`)
- air (installed inside dev containers, not required on host)

## Implementation Notes

- **GOWORK=off for services depending on gen/:** Dockerfiles for services that import from the `gen/` module (currently catalog) set `ENV GOWORK=off` to disable Go workspace mode. The `go.work` file references all workspace members, but each image only contains one service. With workspace mode off, the service's `go.mod` `replace` directive resolves the `gen/` import independently. This is a key monorepo Docker pattern. The gateway does not need `GOWORK=off` today since it has no `gen/` dependency.
- **Volume mount gotcha:** In dev mode, volume mounts override the COPY'd source. For services that depend on `gen/` (catalog), both `gen/` and the service directory are mounted so that local changes to proto definitions or service code are reflected without rebuilding. Services without `gen/` dependency (gateway) only mount their own service directory.
- **Port mapping:** PostgreSQL uses 5433 externally (to avoid conflicting with a local PostgreSQL on 5432) but 5432 internally. Services inside the Docker network connect on 5432.
- **Catalog DATABASE_URL:** The catalog service already reads `DATABASE_URL` from the environment (implemented in Chapter 2). Docker Compose sets this to point at `postgres-catalog` by container name.
- **Migrations on first startup:** The catalog service runs golang-migrate on startup (implemented in Chapter 2). On the first `docker compose up`, PostgreSQL starts, passes its healthcheck, then catalog starts and automatically applies migrations to create the books table. No manual migration step is needed.
- **Gateway does not need gen/:** The gateway currently has no gRPC client dependencies on the `gen/` module. Its Dockerfile is simpler — no `COPY gen/` or `GOWORK=off`. When future chapters add gRPC client calls, its Dockerfile will be updated.

## What This Chapter Does NOT Include

- Kubernetes (Chapter 11)
- Kafka or Meilisearch containers (added when those services are built)
- Auth, Reservation, or Search services (future chapters)
- CI/CD Docker integration (Chapter 10)
- Production deployment (Chapter 12)
- Container registries or image pushing
