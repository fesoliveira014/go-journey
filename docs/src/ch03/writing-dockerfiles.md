# 3.2 Writing Dockerfiles

Now that you understand how layers, caching, and multi-stage builds work, let's walk through the actual Dockerfiles in this project. We have two services to containerize: the Catalog service (which depends on generated protobuf code in `gen/`) and the Gateway (which is self-contained). Each presents different challenges.

---

## The Catalog Dockerfile

Here is `services/catalog/Dockerfile` in its entirety:

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

Let's go through this line by line.

### Base Image and Workspace

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV GOWORK=off
```

We start from `golang:1.26-alpine`, the official Go image based on Alpine Linux. Alpine is a minimal Linux distribution (~5MB) -- using it as a base instead of the full Debian-based `golang:1.26` saves about 400MB in the builder stage.

`GOWORK=off` is the most important line in this Dockerfile, and it requires context.

**Go workspace mode** (`go.work`) lets you develop multiple modules in a monorepo without publishing them to a registry. Our project root has a `go.work` file that links `services/catalog`, `services/gateway`, `gen/`, and other modules together. During local development, this is convenient -- `go build` resolves inter-module imports by following the workspace configuration.

Inside Docker, workspace mode is a problem. We do not copy the *entire* repository into the image (that would defeat the purpose of isolated builds). We copy only `gen/` and `services/catalog/`. Without `go.work` present, Go would try to fetch the `gen` module from the internet (it has a `github.com/...` import path). With `GOWORK=off`, Go ignores any workspace file and relies solely on `go.mod` for dependency resolution.

But wait -- if workspace mode is off and the module isn't published, how does the Catalog service find the `gen` module? The answer is in `services/catalog/go.mod`:

```go
replace github.com/fesoliveira014/library-system/gen => ../../gen
```

This `replace` directive tells Go: "when you encounter an import of `github.com/fesoliveira014/library-system/gen`, don't fetch it from GitHub -- use the local directory `../../gen` instead." Since we copy `gen/` into `/app/gen/` and the catalog service lives at `/app/services/catalog/`, the relative path `../../gen` resolves correctly to `/app/gen`.

### Two-Phase COPY for Cache Efficiency

```dockerfile
COPY gen/go.mod gen/go.sum* ./gen/
COPY services/catalog/go.mod services/catalog/go.sum* ./services/catalog/
```

We copy only the Go module files first. The `*` glob on `go.sum` is a safeguard -- the `gen` module might not have a `go.sum` file yet, and Docker's `COPY` would fail on a missing file without the glob.

```dockerfile
WORKDIR /app/services/catalog
RUN go mod download
```

Dependencies are downloaded while only the module files are in the image. This layer is cached as long as `go.mod` and `go.sum` remain unchanged. Adding a new handler, fixing a bug, refactoring code -- none of these invalidate this layer.

### Source Copy and Build

```dockerfile
WORKDIR /app
COPY gen/ ./gen/
COPY services/catalog/ ./services/catalog/

WORKDIR /app/services/catalog
RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/
```

Now we copy the full source. Note that `gen/` contains the protobuf-generated Go code that the Catalog service imports. The build produces a static binary at `/bin/catalog`.

### Runtime Stage

```dockerfile
FROM alpine:3.19
COPY --from=builder /bin/catalog /usr/local/bin/catalog
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/catalog"]
```

The runtime stage starts fresh from `alpine:3.19`. Only the compiled binary is copied in. `EXPOSE 50052` documents the gRPC port (it does not publish the port -- that's done at runtime with `-p` or in Compose). `ENTRYPOINT` sets the default command.

---

## The Monorepo Build Context Challenge

Both Dockerfiles are located inside their service directories, but the Docker build context must be the repository root. Why?

The Catalog Dockerfile needs files from two directories:
- `gen/` (generated protobuf code)
- `services/catalog/` (the service itself)

Docker can only access files within the build context -- the directory you pass to `docker build`. If we ran `docker build .` from `services/catalog/`, the build context would be `services/catalog/` and `COPY gen/ ./gen/` would fail because `gen/` is outside the context.

The solution: run the build from the repository root and specify the Dockerfile path:

```bash
docker build -f services/catalog/Dockerfile -t catalog:latest .
```

Or equivalently, in `docker-compose.yml`:

```yaml
catalog:
  build:
    context: ../..            # repo root
    dockerfile: services/catalog/Dockerfile
```

This pattern is common in monorepos. The tradeoff is that the entire repository is the build context, which means Docker sends everything to the daemon. This is where `.dockerignore` becomes essential.

---

## The `.dockerignore` File

Located at the repository root, `.dockerignore` tells Docker what to exclude from the build context:

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

This excludes:
- `.git/` -- the Git history (can be hundreds of MB)
- `.worktrees/` -- Git worktree data
- `docs/` -- the tutorial documentation you are reading right now
- `deploy/` -- Compose files and environment configs (not needed inside the image)
- `**/*.md` -- Markdown files at any depth
- `.env*` -- environment files (should never end up in images -- they may contain secrets)
- `**/.air.toml` -- development-only config
- `**/tmp/` -- temporary build artifacts from Air

Without `.dockerignore`, the build context would include all of these, slowing down every build and potentially leaking sensitive files into the image.

---

## The Gateway Dockerfile

Here is `services/gateway/Dockerfile`:

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

The structure is identical to the Catalog Dockerfile, but simpler:

- **No `GOWORK=off`**: The Gateway has no cross-module dependencies (it doesn't import `gen/`), so workspace mode is irrelevant.
- **No `gen/` copy**: The Gateway is self-contained -- it serves HTTP and will eventually proxy to backend services, but currently has no protobuf dependency.
- **Port 8080**: The Gateway is an HTTP service, not gRPC.

As the Gateway evolves to call backend services via gRPC, it will need to import `gen/` and the Dockerfile will need the same `GOWORK=off` and `gen/` copy treatment as the Catalog Dockerfile.

---

## Building and Running Manually

Before using Compose, it is useful to understand manual image building and container execution.

Build both images from the repository root:

```bash
# Build the catalog image
docker build -f services/catalog/Dockerfile -t catalog:latest .

# Build the gateway image
docker build -f services/gateway/Dockerfile -t gateway:latest .
```

Run the gateway standalone:

```bash
docker run --rm -p 8080:8080 gateway:latest
```

`--rm` removes the container when it exits (cleanup). `-p 8080:8080` maps host port 8080 to container port 8080. You should see the Gateway's startup log. Test it:

```bash
curl http://localhost:8080/health
```

The Catalog service is harder to run standalone because it needs PostgreSQL. That is exactly what Compose solves -- covered in the next section.

---

## Exercise: Build and Run

1. From the repository root, build both Docker images:
   ```bash
   docker build -f services/catalog/Dockerfile -t catalog:latest .
   docker build -f services/gateway/Dockerfile -t gateway:latest .
   ```

2. Check the image sizes:
   ```bash
   docker images | grep -E 'catalog|gateway'
   ```

3. Run the gateway container and test it with `curl`:
   ```bash
   docker run --rm -p 8080:8080 gateway:latest
   # In another terminal:
   curl http://localhost:8080/health
   ```

4. Try running the catalog container. What happens and why?

<details>
<summary>Solution</summary>

The image sizes should be approximately 15-25MB each, depending on the binary size. Compare this to the `golang:1.26-alpine` base image (~300MB) -- the multi-stage build cut the image size by over 90%.

```bash
$ docker images | grep -E 'catalog|gateway'
catalog   latest   abc123   15MB
gateway   latest   def456   12MB
```

Running the gateway:

```bash
$ docker run --rm -p 8080:8080 gateway:latest
# Gateway starts and listens on :8080
$ curl http://localhost:8080/health
# Returns 200 OK (or whatever the health endpoint returns)
```

Running the catalog:

```bash
$ docker run --rm -p 50052:50052 catalog:latest
# The service will fail to start (or start and crash) because it cannot
# connect to PostgreSQL. The DATABASE_URL environment variable is not set,
# and even if it were, there is no PostgreSQL server reachable from the
# container's isolated network.
```

This demonstrates why multi-container orchestration (Docker Compose) is necessary. A service that depends on a database cannot be tested in isolation without providing that database.

</details>

---

## Summary

- The Catalog Dockerfile uses `GOWORK=off` to disable Go workspace mode inside Docker, relying on `replace` directives in `go.mod` for local module resolution.
- Two-phase COPY (module files first, then source) keeps the dependency download layer cached across most builds.
- The build context must be the repository root in a monorepo so that all required directories (`gen/`, `services/catalog/`) are accessible.
- `.dockerignore` excludes large and sensitive files from the build context.
- The Gateway Dockerfile follows the same pattern but is simpler because it has no cross-module dependencies.
- Multi-stage builds produce final images under 25MB for Go services.

---

## References

[^1]: [Dockerfile reference](https://docs.docker.com/reference/dockerfile/) -- complete syntax documentation for all Dockerfile instructions.
[^2]: [Go modules reference: replace directive](https://go.dev/ref/mod#go-mod-file-replace) -- how `replace` directives work in `go.mod`.
[^3]: [.dockerignore file](https://docs.docker.com/build/building/context/#dockerignore-files) -- syntax and behavior of the Docker ignore file.
[^4]: [Go workspaces](https://go.dev/doc/tutorial/workspaces) -- official tutorial on Go workspace mode and `go.work`.
