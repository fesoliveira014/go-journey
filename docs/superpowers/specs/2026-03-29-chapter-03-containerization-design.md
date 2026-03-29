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
- **Multi-stage COPY for monorepo:** Dockerfiles explicitly copy `go.work`, `gen/`, and their own `services/<name>/` directory. Combined with a root `.dockerignore` to keep build context small.

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

# Copy workspace and shared modules first (cache-friendly layer ordering)
COPY go.work go.work.sum* ./
COPY gen/ ./gen/
COPY services/<name>/ ./services/<name>/

WORKDIR /app/services/<name>
RUN go mod download
RUN CGO_ENABLED=0 go build -o /bin/<name> ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/<name> /usr/local/bin/<name>
EXPOSE <port>
ENTRYPOINT ["/usr/local/bin/<name>"]
```

Key points:
- **Root build context:** Docker Compose sets `context: ../..` (project root) so COPY can reach `go.work` and `gen/`
- **Selective COPY:** Only copies what the service needs, not the entire monorepo
- **Layer ordering:** Workspace files and dependencies before source code, maximizing cache hits
- **Static binary:** `CGO_ENABLED=0` produces a self-contained binary that runs on alpine without glibc

Both gateway and catalog get this pattern. The existing catalog Dockerfile is rewritten. Gateway gets a new Dockerfile.

### Dev Dockerfiles (`services/<name>/Dockerfile.dev`)

Single-stage with air for hot-reload:

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copy workspace and shared modules
COPY go.work go.work.sum* ./
COPY gen/ ./gen/
COPY services/<name>/ ./services/<name>/

WORKDIR /app/services/<name>
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
  include_ext = ["go", "proto"]
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
*.md
!go.work
.env*
.air.toml
tmp/
```

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

- **go.work in Docker:** The `go.work` file is critical for the monorepo build. Dockerfiles copy it to `/app/go.work` so that `go build` resolves cross-module imports (e.g., catalog importing from gen/).
- **Volume mount gotcha:** In dev mode, the volume mount overrides the COPY'd source. But `go.work` and `gen/` are also mounted so that local changes to proto definitions are reflected without rebuilding the container.
- **Port mapping:** PostgreSQL uses 5433 externally (to avoid conflicting with a local PostgreSQL on 5432) but 5432 internally. Services inside the Docker network connect on 5432.
- **go.work.sum:** The `COPY go.work go.work.sum* ./` wildcard handles the case where `go.work.sum` doesn't exist yet.
- **Catalog DATABASE_URL:** The catalog service already reads `DATABASE_URL` from the environment (implemented in Chapter 2). Docker Compose sets this to point at `postgres-catalog` by container name.

## What This Chapter Does NOT Include

- Kubernetes (Chapter 11)
- Kafka or Meilisearch containers (added when those services are built)
- Auth, Reservation, or Search services (future chapters)
- CI/CD Docker integration (Chapter 10)
- Production deployment (Chapter 12)
- Container registries or image pushing
