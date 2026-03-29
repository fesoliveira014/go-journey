# Chapter 3: Containerization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Containerize gateway and catalog services with Docker, orchestrate them with Docker Compose, and set up a hot-reload development workflow using air.

**Architecture:** Multi-stage Dockerfiles per service, Docker Compose for orchestration (production-like and dev override), air for hot-reload inside containers. The monorepo build challenge is solved with `GOWORK=off` and selective COPY — each service builds independently using its `go.mod` `replace` directive. Gateway is simpler (no `gen/` dependency); catalog needs the full monorepo-aware pattern.

**Tech Stack:** Docker, Docker Compose v2, air (Go hot-reload), PostgreSQL 16

**Spec:** `docs/superpowers/specs/2026-03-29-chapter-03-containerization-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `.dockerignore` | Create | Exclude non-build files from Docker build context |
| `services/catalog/Dockerfile` | Rewrite | Multi-stage production build with `GOWORK=off`, two-phase COPY for cache efficiency |
| `services/gateway/Dockerfile` | Create | Simple multi-stage production build (no `gen/` dependency) |
| `services/catalog/.air.toml` | Create | Air hot-reload config for catalog |
| `services/gateway/.air.toml` | Create | Air hot-reload config for gateway |
| `services/catalog/Dockerfile.dev` | Create | Dev image with air, `GOWORK=off`, mounts gen/ |
| `services/gateway/Dockerfile.dev` | Create | Dev image with air (simpler, no gen/) |
| `deploy/.env` | Create | Default environment variables for Compose |
| `deploy/docker-compose.yml` | Create | Production-like orchestration: postgres-catalog + catalog + gateway |
| `deploy/docker-compose.dev.yml` | Create | Dev override: air + volume mounts |
| `docs/src/SUMMARY.md` | Modify | Add Chapter 3 entries |
| `docs/src/ch03/index.md` | Create | Chapter 3 overview |
| `docs/src/ch03/docker-fundamentals.md` | Create | Section 3.1: Docker concepts |
| `docs/src/ch03/writing-dockerfiles.md` | Create | Section 3.2: Dockerfile walkthrough |
| `docs/src/ch03/docker-compose.md` | Create | Section 3.3: Compose orchestration |
| `docs/src/ch03/dev-workflow.md` | Create | Section 3.4: Hot-reload dev workflow |

---

### Task 1: Root .dockerignore and Catalog Production Dockerfile

Create the `.dockerignore` at the project root, then rewrite the catalog's `Dockerfile` with the monorepo-aware multi-stage build pattern. Verify the catalog image builds and runs correctly.

**Files:**
- Create: `.dockerignore`
- Rewrite: `services/catalog/Dockerfile`

**Context:** The existing `services/catalog/Dockerfile` does `COPY . .` which only works if the build context is the service directory itself. The new Dockerfile uses the project root as build context and selectively copies `gen/` and `services/catalog/` with `GOWORK=off`. It also uses a two-phase COPY pattern (go.mod first, then source) for Docker layer cache efficiency.

- [ ] **Step 1: Create `.dockerignore` at project root**

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

- [ ] **Step 2: Rewrite `services/catalog/Dockerfile`**

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service and gen/, not all
# workspace members. The replace directive in go.mod handles the gen/ import.
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY services/catalog/go.mod services/catalog/go.sum* ./services/catalog/

# 2. Download dependencies (cached unless go.mod changes)
WORKDIR /app/services/catalog
RUN go mod download

# 3. Copy source code (invalidates cache only when source changes)
WORKDIR /app
COPY gen/ ./gen/
COPY services/catalog/ ./services/catalog/

# 4. Build static binary
WORKDIR /app/services/catalog
RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/catalog /usr/local/bin/catalog
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/catalog"]
```

- [ ] **Step 3: Build the catalog Docker image from project root**

Run from project root:
```bash
docker build -f services/catalog/Dockerfile -t catalog:latest .
```

Expected: Image builds successfully. The `GOWORK=off` ensures Go resolves `gen/` via the `replace` directive in `services/catalog/go.mod` without needing the full workspace.

- [ ] **Step 4: Verify the image runs (will fail on DB, but binary should start)**

```bash
docker run --rm catalog:latest --help 2>&1 || true
docker run --rm -e DATABASE_URL="host=localhost" -e GRPC_PORT="50052" catalog:latest &
sleep 2
docker ps --filter ancestor=catalog:latest
docker stop $(docker ps -q --filter ancestor=catalog:latest) 2>/dev/null || true
```

Expected: The container starts (it will log a database connection error since there's no PostgreSQL — that's fine). The binary is present and executable.

- [ ] **Step 5: Commit**

```bash
git add .dockerignore services/catalog/Dockerfile
git commit -m "feat(docker): add .dockerignore and rewrite catalog Dockerfile

Multi-stage build with GOWORK=off for monorepo isolation. Two-phase COPY
pattern for Docker layer cache efficiency (go.mod first, then source)."
```

---

### Task 2: Gateway Production Dockerfile

Create the gateway's production Dockerfile. The gateway is simpler — no `gen/` dependency, no `GOWORK=off` needed.

**Files:**
- Create: `services/gateway/Dockerfile`

**Context:** The gateway has no dependencies beyond the Go standard library (`services/gateway/go.mod` has no `require` directives). Its Dockerfile is a straightforward multi-stage build without the monorepo-aware pattern. This contrast with the catalog Dockerfile is pedagogically useful.

- [ ] **Step 1: Create `services/gateway/Dockerfile`**

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy only go.mod first for dependency caching
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/

# Download dependencies (currently none, but pattern is correct for future)
WORKDIR /app/services/gateway
RUN go mod download

# Copy source code
WORKDIR /app
COPY services/gateway/ ./services/gateway/

# Build static binary
WORKDIR /app/services/gateway
RUN CGO_ENABLED=0 go build -o /bin/gateway ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
```

- [ ] **Step 2: Build the gateway Docker image from project root**

```bash
docker build -f services/gateway/Dockerfile -t gateway:latest .
```

Expected: Image builds successfully.

- [ ] **Step 3: Verify the gateway image runs and responds**

```bash
docker run --rm -d -p 8080:8080 -e PORT=8080 --name gateway-test gateway:latest
sleep 1
curl -s http://localhost:8080/healthz
docker stop gateway-test
```

Expected: `curl` returns `{"status":"ok"}` (or the gateway's health response).

- [ ] **Step 4: Commit**

```bash
git add services/gateway/Dockerfile
git commit -m "feat(docker): add gateway production Dockerfile

Simple multi-stage build — no gen/ dependency, no GOWORK=off needed."
```

---

### Task 3: Air Configuration and Dev Dockerfiles

Create `.air.toml` configs and `Dockerfile.dev` files for both services. Air provides hot-reload: it watches for `.go` file changes and rebuilds the binary inside the container.

**Files:**
- Create: `services/catalog/.air.toml`
- Create: `services/gateway/.air.toml`
- Create: `services/catalog/Dockerfile.dev`
- Create: `services/gateway/Dockerfile.dev`

**Context:** Dev Dockerfiles are single-stage (no multi-stage needed — we want the full Go toolchain at runtime). Docker Compose will mount local source as volumes, overriding the COPY'd files. Air detects changes and rebuilds. Catalog needs `GOWORK=off` and `gen/`; gateway does not.

- [ ] **Step 1: Create `services/catalog/.air.toml`**

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

- [ ] **Step 2: Create `services/gateway/.air.toml`**

Same content as catalog's `.air.toml` — identical config works for both services since both use the `cmd/` directory structure.

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

- [ ] **Step 3: Create `services/catalog/Dockerfile.dev`**

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

- [ ] **Step 4: Create `services/gateway/Dockerfile.dev`**

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app
COPY services/gateway/ ./services/gateway/

WORKDIR /app/services/gateway
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

- [ ] **Step 5: Build both dev images to verify they compile**

```bash
docker build -f services/catalog/Dockerfile.dev -t catalog:dev .
docker build -f services/gateway/Dockerfile.dev -t gateway:dev .
```

Expected: Both images build successfully. Air is installed and available.

- [ ] **Step 6: Commit**

```bash
git add services/catalog/.air.toml services/gateway/.air.toml \
      services/catalog/Dockerfile.dev services/gateway/Dockerfile.dev
git commit -m "feat(docker): add air configs and dev Dockerfiles for hot-reload

Catalog dev image uses GOWORK=off with gen/ for monorepo builds.
Gateway dev image is simpler (no gen/ dependency)."
```

---

### Task 4: Docker Compose — Production-Like Stack

Create the `deploy/` directory with `docker-compose.yml` and `.env`. This orchestrates PostgreSQL, catalog, and gateway into a working stack.

**Files:**
- Create: `deploy/.env`
- Create: `deploy/docker-compose.yml`

**Context:** The compose file uses the project root as build context (`context: ../..`) so Dockerfiles can reach `gen/`. PostgreSQL maps to port 5433 externally (avoiding conflicts with local PostgreSQL). The catalog service depends on PostgreSQL with a healthcheck. Catalog runs migrations automatically on startup.

- [ ] **Step 1: Create `deploy/.env`**

```env
POSTGRES_CATALOG_PORT=5433
POSTGRES_CATALOG_USER=postgres
POSTGRES_CATALOG_PASSWORD=postgres
POSTGRES_CATALOG_DB=catalog

GATEWAY_PORT=8080
CATALOG_GRPC_PORT=50052
```

- [ ] **Step 2: Create `deploy/docker-compose.yml`**

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

- [ ] **Step 3: Start the full stack and smoke test**

```bash
docker compose -f deploy/docker-compose.yml up --build -d
```

Wait for all containers to be healthy, then test:

```bash
# Wait for startup
sleep 10

# Gateway health check
curl -s http://localhost:8080/healthz

# Catalog gRPC check (requires grpcurl — skip if not installed)
grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks 2>/dev/null || echo "grpcurl not available, skipping"

# Check all containers are running
docker compose -f deploy/docker-compose.yml ps
```

Expected: Three containers running (postgres-catalog, catalog, gateway). Gateway returns health response on `:8080/healthz`. Catalog is connected to PostgreSQL and serving gRPC on `:50052`.

- [ ] **Step 4: Tear down**

```bash
docker compose -f deploy/docker-compose.yml down -v
```

- [ ] **Step 5: Commit**

```bash
git add deploy/.env deploy/docker-compose.yml
git commit -m "feat(docker): add Docker Compose stack with PostgreSQL, catalog, and gateway

Production-like orchestration with healthcheck-based startup ordering.
PostgreSQL on external port 5433, catalog on 50052, gateway on 8080."
```

---

### Task 5: Docker Compose — Dev Override with Hot-Reload

Create the development override file that swaps production images for dev images with air and volume mounts.

**Files:**
- Create: `deploy/docker-compose.dev.yml`

**Context:** The dev override file uses Docker Compose's merge behavior — it only overrides `build` (to use `Dockerfile.dev`) and adds `volumes` (to mount local source). All other settings (ports, environment, depends_on, networks) are inherited from the base `docker-compose.yml`.

- [ ] **Step 1: Create `deploy/docker-compose.dev.yml`**

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

- [ ] **Step 2: Start the dev stack**

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up --build -d
```

Wait for startup, then verify:

```bash
sleep 15

# Gateway should respond
curl -s http://localhost:8080/healthz

# All containers running
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml ps

# Check air is running inside gateway container
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml logs gateway | head -20
```

Expected: Air is running inside both service containers. Gateway responds to health check. Logs show air watching for file changes.

- [ ] **Step 3: Test hot-reload (gateway)**

Make a trivial change to the gateway's health handler, verify air detects it and rebuilds:

```bash
# Check current response
curl -s http://localhost:8080/healthz

# Make a small change to trigger rebuild (add a comment)
echo '// hot-reload test' >> services/gateway/internal/handler/health.go

# Wait for air to rebuild
sleep 5

# Check air detected the change
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml logs --tail=10 gateway

# Revert the change
git checkout services/gateway/internal/handler/health.go
```

Expected: Air logs show a file change detected and a rebuild triggered.

- [ ] **Step 4: Tear down**

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml down -v
```

- [ ] **Step 5: Commit**

```bash
git add deploy/docker-compose.dev.yml
git commit -m "feat(docker): add dev override with air hot-reload and volume mounts

Usage: docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up --build"
```

---

### Task 6: Tutorial Documentation — Chapter 3

Write the tutorial content for Chapter 3. This covers Docker fundamentals, writing Dockerfiles, Docker Compose, and the development workflow.

**Files:**
- Modify: `docs/src/SUMMARY.md`
- Create: `docs/src/ch03/index.md`
- Create: `docs/src/ch03/docker-fundamentals.md`
- Create: `docs/src/ch03/writing-dockerfiles.md`
- Create: `docs/src/ch03/docker-compose.md`
- Create: `docs/src/ch03/dev-workflow.md`

**Context:** The user is an experienced engineer (7+ years, C/C++, Kotlin, Java) learning Go and cloud-native tooling. The spec says to compare containers to JVMs for the Java dev. Content should be thorough but not condescending. Each section should be ~1000-1500 words with code blocks, exercises, and footnoted references. Follow the style established by Chapters 1-2 (check `docs/src/ch02/` for tone and formatting). Content is Markdown for mdBook (GitHub Pages).

Reference spec section: **Tutorial Chapter Outline** (lines 305-313 of spec).

- [ ] **Step 1: Update `docs/src/SUMMARY.md`**

Add Chapter 3 entries after the Chapter 2 block. Match the indentation style used by Chapter 2 entries:

```markdown
- [Chapter 3: Containerization](./ch03/index.md)
  - [3.1 Docker Fundamentals](./ch03/docker-fundamentals.md)
  - [3.2 Writing Dockerfiles](./ch03/writing-dockerfiles.md)
  - [3.3 Docker Compose](./ch03/docker-compose.md)
  - [3.4 Development Workflow](./ch03/dev-workflow.md)
```

- [ ] **Step 2: Create `docs/src/ch03/index.md`**

Chapter overview page. Include:
- What this chapter covers (containerization of existing services)
- Prerequisites (Docker installed, Chapters 1-2 complete)
- What you'll build (Compose stack with PostgreSQL + catalog + gateway, dev workflow with hot-reload)
- Architecture diagram (Mermaid) showing the three containers and their connections

- [ ] **Step 3: Create `docs/src/ch03/docker-fundamentals.md`**

Section 3.1 content (~1200-1500 words). Cover:
- What containers are — compare to JVMs for the Java dev (spec line 307): "If you're coming from Java, think of a container as a standardized JVM runtime — but for any language. Where a JVM isolates your app from the OS, a container isolates your entire runtime environment."
- Images vs containers (blueprint vs running instance)
- Layers and caching — how Docker builds images layer by layer, why order matters
- Multi-stage builds — why we use them, how they produce small final images
- Why containerize Go? (Go produces static binaries, so why bother?) Answer: consistency, dependency isolation, deployment uniformity, matching prod environments
- No exercises in this section — it's conceptual foundation

- [ ] **Step 4: Create `docs/src/ch03/writing-dockerfiles.md`**

Section 3.2 content (~1200-1500 words). Cover:
- Walk through the catalog production Dockerfile line by line
- The monorepo build context challenge: why `COPY . .` doesn't work when your service imports from `gen/` but lives in `services/catalog/`
- `GOWORK=off` explained: what Go workspace mode is, why it breaks in Docker, how `replace` directives save us
- Selective COPY and why we only copy what the service needs
- Two-phase COPY for cache efficiency (go.mod first, then source)
- The `.dockerignore` file and what it excludes
- Contrast with the gateway Dockerfile (simpler, no gen/ dependency)
- Building and running a single container manually (`docker build`, `docker run`)
- **Exercise:** Build both images manually, run the gateway, test with curl

- [ ] **Step 5: Create `docs/src/ch03/docker-compose.md`**

Section 3.3 content (~1200-1500 words). Cover:
- What Compose solves (multi-container orchestration — "imagine starting 3 terminals, running docker run in each with the right flags, and hoping you got the network right")
- The `docker-compose.yml` structure: services, networks, volumes
- Walk through each service definition (postgres-catalog, catalog, gateway)
- Environment variables with `.env` defaults and `${VAR:-default}` syntax
- Healthchecks: what they do, `pg_isready`, `depends_on` with `condition: service_healthy`
- The `library-net` bridge network and container DNS (services reference each other by name)
- Port mapping explained (external:internal, why PostgreSQL is 5433 externally)
- Named volumes for PostgreSQL data persistence
- Starting the stack: `docker compose up`, viewing logs, stopping
- **Exercise:** Start the stack, create a book via grpcurl, verify it persists after `docker compose down` + `up` (volume persistence)

- [ ] **Step 6: Create `docs/src/ch03/dev-workflow.md`**

Section 3.4 content (~1200-1500 words). Cover:
- The dev override file pattern: what it is, why not modify the base compose file
- Docker Compose file merging behavior
- Air for hot-reload: what it does, `.air.toml` configuration explained
- Volume mounts: how they make hot-reload work (overriding COPY'd files)
- The full dev command: `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up --build`
- Debugging tips: `docker compose logs -f <service>`, `docker compose exec <service> sh`, inspecting networks, diagnosing port conflicts
- When to rebuild vs restart (dependency changes → rebuild; source changes → air handles it)
- **Exercise:** Start the dev stack, modify a handler to change the health response, watch air rebuild, verify the change with curl

- [ ] **Step 7: Commit**

```bash
git add docs/src/SUMMARY.md docs/src/ch03/
git commit -m "docs(ch03): write Chapter 3 tutorial — Docker, Compose, and dev workflow

Four sections covering Docker fundamentals, writing Dockerfiles for Go
monorepos, Docker Compose orchestration, and hot-reload development with air."
```
