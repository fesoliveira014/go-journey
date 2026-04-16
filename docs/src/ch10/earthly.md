# 10.2 The Earthly Build System

Every team eventually ends up with a build script that works in CI but not locally---or vice versa. The culprit is almost always environmental drift: different Go versions, different lint tool versions, missing environment variables, undocumented system dependencies. Earthly solves this problem by running every build step inside a container. If it builds on your laptop, it builds in CI — and the outputs are bit-for-bit identical.

This section covers Earthly in depth, because it is the primary build and CI tool for this project. By the end you will understand how to read and write Earthfiles, how the caching model works, and how the root Earthfile orchestrates builds across all five services.

---

## What Earthly Is

Earthly is a build tool that combines Dockerfile syntax with Makefile-style named targets. The mental model is:

- Every target runs inside its own container, derived from a `FROM` base image.
- Targets can depend on other targets; Earthly resolves the dependency graph and runs them in the correct order.
- Outputs (files, Docker images) are explicitly declared with `SAVE ARTIFACT` and `SAVE IMAGE`. Nothing leaks out of a container unless you ask for it.

If you have used Gradle, think of each Earthly target as a Gradle task, except the task runs in a reproducible container rather than on the host JVM. The dependency semantics are the same: `:docker` depends on `:build`, which depends on `:src`, which depends on `:deps`. Earthly evaluates that chain automatically.

The key difference from a raw Dockerfile is named targets with explicit inputs and outputs. A Dockerfile is a linear script that produces one image. An Earthfile is a graph of targets, each producing artifacts that others can consume.

Earthly is also distinct from Docker Compose. Compose is for running services. Earthly is for building them. You run Earthly during development and CI to produce binaries and images; you run Compose (or Kubernetes) to run those images.

---

## Key Concepts

| Concept | What it does |
|---------|-------------|
| `VERSION` | Declares the Earthfile spec version. Use `0.8`. |
| `FROM` | Sets the base image for a target (or globally at the top of the file). |
| Target | A named build step, like `deps:` or `docker:`. Invoked with `earthly +targetname`. |
| `COPY` | Copies files into the container. Supports `--dir` for directories. |
| `RUN` | Executes a shell command inside the container. |
| `SAVE ARTIFACT` | Exports a file or directory from the container so other targets can use it. |
| `SAVE IMAGE` | Tags and saves the container as a Docker image. |
| `BUILD` | Triggers another target (in the same or a different Earthfile). Used for orchestration. |
| `ARG` | Declares a build argument, similar to Dockerfile `ARG`. |
| `+targetname` | Reference syntax for a local target. `./services/catalog+docker` references a target in another directory. |

---

## Local = Cloud

The core value proposition of Earthly is that your local build and your CI build are the same command. From the repository root:

```bash
# Run the full CI pipeline locally
earthly +ci

# Build Docker images
earthly +docker

# Build and lint just one service
earthly ./services/catalog+lint

# Run tests for one service
earthly ./services/gateway+test
```

In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` — the same command you run locally. There is no CI-specific build script to maintain. The category of "it worked locally but failed in CI" failures from environment drift disappears.

If a lint failure appears in CI, you run `earthly ./services/catalog+lint` locally and see the exact same output. If a test fails in CI, you run `earthly ./services/catalog+test` and reproduce it immediately.

This is a meaningful shift from the traditional model where developers run `go test ./...` locally (with whatever version of Go happens to be installed) and CI runs a slightly different set of steps in a managed environment. With Earthly, the container is the environment, and it is the same container everywhere.

---

## The Catalog Earthfile: A Full Walkthrough

The Catalog Service Earthfile is the canonical example. All five service Earthfiles follow the same pattern with minor variations. Here is the full file:

```earthfile
VERSION 0.8

FROM golang:1.22-alpine

WORKDIR /app

deps:
    COPY go.mod go.sum* ./
    # Copy local module go.mod files via root Earthfile artifact targets
    COPY ../../+gen-mod/gen ../gen
    COPY ../../+pkg-auth-mod/pkg-auth ../pkg/auth
    COPY ../../+pkg-otel-mod/pkg-otel ../pkg/otel
    ENV GOWORK=off
    RUN go mod download && (cd ../gen && go mod download) && (cd ../pkg/auth && go mod download) && (cd ../pkg/otel && go mod download)
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

src:
    FROM +deps
    COPY --dir cmd internal migrations ./
    COPY ../../+gen-src/gen ../gen
    COPY ../../+pkg-auth-src/pkg-auth ../pkg/auth
    COPY ../../+pkg-otel-src/pkg-otel ../pkg/otel

lint:
    FROM +src
    COPY ../../+golangci-config/.golangci.yml ./
    RUN go build ./...
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
    RUN golangci-lint run ./...

test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1

build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/
    SAVE ARTIFACT /bin/catalog

docker:
    FROM alpine:3.19
    COPY +build/catalog /usr/local/bin/catalog
    EXPOSE 50052
    ENTRYPOINT ["/usr/local/bin/catalog"]
    SAVE IMAGE catalog:latest
```

### `deps` — Dependency Layer

```earthfile
deps:
    COPY go.mod go.sum* ./
    # Copy local module go.mod files via root Earthfile artifact targets
    COPY ../../+gen-mod/gen ../gen
    COPY ../../+pkg-auth-mod/pkg-auth ../pkg/auth
    COPY ../../+pkg-otel-mod/pkg-otel ../pkg/otel
    ENV GOWORK=off
    RUN go mod download && (cd ../gen && go mod download) && (cd ../pkg/auth && go mod download) && (cd ../pkg/otel && go mod download)
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum
```

This target copies only the `go.mod` and `go.sum` files — nothing else — and then runs `go mod download`. The reason for this separation is layer caching.

Docker (and Earthly) cache build layers. If the inputs to a step have not changed since the last run, the cached layer is reused. By copying only the module files first and running `go mod download` before copying any application source, you ensure that dependency downloads only happen when dependencies actually change — not every time you edit a `.go` file.

If you have ever written a Java Dockerfile, you have done the same thing: copy `pom.xml` or `build.gradle` first, run the dependency download step, then copy the source.

The `COPY ../../+gen-mod/gen ../gen` syntax is a cross-Earthfile artifact reference. Instead of directly copying `../../gen/go.mod` — which would fail because Earthly scopes each service's build context to its own directory — the service Earthfile references an artifact target defined in the root Earthfile. The root `+gen-mod` target copies `gen/go.mod` and `gen/go.sum` into a `scratch` container and saves them as a named artifact. The service then pulls that artifact into its own container. This indirection is necessary in monorepos where local modules live outside the service's build context.

The `go mod download` command is chained with subshell calls for each local module: `(cd ../gen && go mod download)`. Each local module has its own `go.mod` with its own dependencies, and Go does not download them transitively. Without these subshells, the build would fail later when the compiler tries to resolve imports from the local modules.

The project uses Go workspaces for local development, but `GOWORK=off` disables that inside the container. Inside the container, the local modules (`gen`, `pkg/auth`, `pkg/otel`) are placed at paths that match the `replace` directives in `go.mod`, so Go resolves them locally without needing the workspace. This is a deliberate design: workspaces are a developer convenience, but the container build should be explicit about module resolution.

`SAVE ARTIFACT go.mod AS LOCAL go.mod` writes the resolved `go.mod` back to your working directory. This is how `go mod tidy` updates (run inside the container) flow back out to your checkout.

### `src` — Source Layer

```earthfile
src:
    FROM +deps
    COPY --dir cmd internal migrations ./
    COPY ../../+gen-src/gen ../gen
    COPY ../../+pkg-auth-src/pkg-auth ../pkg/auth
    COPY ../../+pkg-otel-src/pkg-otel ../pkg/otel
```

`FROM +deps` starts this target from the state of the `deps` target — the base image with all Go modules already downloaded. Now we copy the actual source code.

The source copy uses the `+gen-src`, `+pkg-auth-src`, and `+pkg-otel-src` artifact targets from the root Earthfile, following the same cross-Earthfile pattern as `deps`. The `-src` variants copy the full source trees (not just `go.mod` files).

This target is invalidated whenever any `.go` file in `cmd/`, `internal/`, `migrations/`, `gen/`, `pkg/auth/`, or `pkg/otel/` changes. That is fine, because copying source is fast. What matters is that the `deps` layer below it stays cached as long as dependencies have not changed.

Notice that `src` produces no `SAVE ARTIFACT`; it is an intermediate target. Its job is to give `lint`, `test`, and `build` a common starting point, so none of them needs to repeat the `COPY` instructions.

In Gradle terms, this is like a `compileClasspath` configuration that `test` and `assemble` both inherit from.

### `lint` — Static Analysis

```earthfile
lint:
    FROM +src
    COPY ../../+golangci-config/.golangci.yml ./
    RUN go build ./...
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
    RUN golangci-lint run ./...
```

The linter is installed inside the container at a pinned version. This means every developer and every CI run uses exactly the same linter version — no drift, no "works on my machine" lint failures.

The `.golangci.yml` configuration is copied from the root Earthfile's `+golangci-config` artifact target. It lives at the repository root (not inside the service directory) so all services share the same lint rules. If you added a new rule to `.golangci.yml`, all five services would pick it up the next time `+lint` ran.

The `RUN go build ./...` step before `golangci-lint run` is deliberate. Some linters (notably those based on `go/analysis`) need compiled export data for dependencies to perform type-aware checks. When the replaced local modules (`gen`, `pkg/auth`, `pkg/otel`) have not been built, golangci-lint may report "could not load export data" errors. Running `go build` first populates the build cache with the necessary export data.

Note that `.golangci.yml` is copied after `+src`. This means a change to lint configuration invalidates the lint layer but not the source layer — the compiled package cache is still reused.

### `test` — Unit Tests

```earthfile
test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1
```

Tests run against the same `+src` layer used by `lint` and `build`. The `-count=1` flag disables Go's test result cache, ensuring tests always run fresh. The `-v` flag produces verbose output, which is useful in CI logs.

The test target deliberately scopes to `./internal/service/...` and `./internal/handler/...` rather than `./...`. This avoids running integration tests that require a live database or Kafka broker. Those belong in a separate target that accepts a running Docker network — a topic covered in a later section.

### `build` — Compile

```earthfile
build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/
    SAVE ARTIFACT /bin/catalog
```

`CGO_ENABLED=0` produces a statically linked binary. This is important because the binary will run inside an `alpine` container that may not have glibc. A dynamically linked Go binary would fail to start with a "no such file or directory" error because the dynamic linker is absent. Static binaries have no such dependency.

`SAVE ARTIFACT /bin/catalog` makes the compiled binary available to other targets. Without this declaration, the binary exists only inside this container and disappears when the target finishes. With it, any target can reference it as `+build/catalog`.

### `docker` — Container Image

```earthfile
docker:
    FROM alpine:3.19
    COPY +build/catalog /usr/local/bin/catalog
    EXPOSE 50052
    ENTRYPOINT ["/usr/local/bin/catalog"]
    SAVE IMAGE catalog:latest
```

This target starts fresh from `alpine:3.19` — not from `+src`. The Go toolchain, module cache, and source code are not needed at runtime. The final image contains only the Alpine base and the compiled binary. This keeps the image small (typically under 20 MB for a static Go binary on Alpine) and reduces the attack surface.

`COPY +build/catalog /usr/local/bin/catalog` pulls the artifact saved by the `build` target into this container. This is Earthly's artifact reference syntax: `+targetname/path`.

`SAVE IMAGE catalog:latest` tags the container image. You can push it with `earthly --push +docker`, which runs the target and pushes the resulting image to a registry.

---

## Service Variations

All five Earthfiles share the same structure. The differences are small but worth noting.

### Auth Service

The auth Earthfile omits the `pkg/otel` dependency. The Auth Service uses structured logging and basic instrumentation but does not pull in the shared OpenTelemetry package directly. Both its `deps` and `src` targets copy only `gen` and `pkg/auth`, not `pkg/otel`.

```earthfile
deps:
    COPY go.mod go.sum* ./
    COPY ../../+gen-mod/gen ../gen
    COPY ../../+pkg-auth-mod/pkg-auth ../pkg/auth
    # No pkg/otel here
    ENV GOWORK=off
    RUN go mod download && (cd ../gen && go mod download) && (cd ../pkg/auth && go mod download)
```

The rest of the targets (`lint`, `test`, `build`, `docker`) are identical in structure to catalog.

### Gateway Service

The gateway serves HTTP and includes HTML templates and static assets alongside the compiled binary. Its `src` target saves these assets as artifacts, and the `docker` target copies them in alongside the binary:

```earthfile
src:
    FROM +deps
    COPY --dir cmd internal templates static ./
    COPY ../../+gen-src/gen ../gen
    COPY ../../+pkg-auth-src/pkg-auth ../pkg/auth
    COPY ../../+pkg-otel-src/pkg-otel ../pkg/otel
    SAVE ARTIFACT ./templates
    SAVE ARTIFACT ./static

test:
    FROM +src
    RUN apk add --no-cache gcc musl-dev
    RUN CGO_ENABLED=1 go test -v -race -cover ./...

docker:
    FROM alpine:3.19
    WORKDIR /app
    COPY +build/gateway /usr/local/bin/gateway
    COPY +src/templates /app/templates
    COPY +src/static /app/static
    EXPOSE 8080
    ENTRYPOINT ["/usr/local/bin/gateway"]
    SAVE IMAGE gateway:latest
```

The gateway's `test` target differs from the other services. The `-race` flag enables the Go race detector, which requires CGO. On Alpine, CGO needs `gcc` and `musl-dev` installed, plus `CGO_ENABLED=1` explicitly set. The other services skip the race detector and run with `CGO_ENABLED=0` by default.

The gateway image exposes port 8080 instead of a gRPC port. It also sets `WORKDIR /app` so that the templates and static directories are at predictable relative paths when the binary looks for them at runtime.

### Search Service

The Search Service, like auth, omits `pkg/otel`. It is otherwise identical to catalog in structure. The `docker` target exposes port 50054.

### Reservation Service

Reservation follows the same pattern as catalog, exposing port 50053. It depends on both `pkg/auth` (for JWT validation) and `pkg/otel`.

---

## The Root Earthfile

The root Earthfile serves two roles: it defines shared artifact targets that service Earthfiles depend on, and it orchestrates builds across all five services.

### Shared Artifact Targets

In a monorepo, service Earthfiles cannot directly `COPY` files from outside their directory. Earthly scopes each Earthfile's build context to its own directory, so `COPY ../../gen/go.mod` would fail. The solution is to define artifact targets in the root Earthfile that package shared modules and make them available via cross-Earthfile references:

```earthfile
VERSION 0.8

# Shared module artifacts — service Earthfiles COPY from these targets
# instead of using ../../ relative paths (which exceed the build context).

gen-mod:
    FROM scratch
    COPY gen/go.mod gen/go.sum* /gen/
    SAVE ARTIFACT /gen gen

gen-src:
    FROM scratch
    COPY gen/ /gen/
    SAVE ARTIFACT /gen gen

pkg-auth-mod:
    FROM scratch
    COPY pkg/auth/go.mod pkg/auth/go.sum* /pkg-auth/
    SAVE ARTIFACT /pkg-auth pkg-auth

pkg-auth-src:
    FROM scratch
    COPY pkg/auth/ /pkg-auth/
    SAVE ARTIFACT /pkg-auth pkg-auth

pkg-otel-mod:
    FROM scratch
    COPY pkg/otel/go.mod pkg/otel/go.sum* /pkg-otel/
    SAVE ARTIFACT /pkg-otel pkg-otel

pkg-otel-src:
    FROM scratch
    COPY pkg/otel/ /pkg-otel/
    SAVE ARTIFACT /pkg-otel pkg-otel

golangci-config:
    FROM scratch
    COPY .golangci.yml /
    SAVE ARTIFACT /.golangci.yml
```

Each shared module has two artifact targets: a `-mod` target (just `go.mod` and `go.sum`, for the `deps` layer) and a `-src` target (the full source tree, for the `src` layer). This split preserves the layer caching strategy — changes to source files do not invalidate the dependency download layer.

The `golangci-config` target does the same for the shared lint configuration. All service `+lint` targets reference `../../+golangci-config/.golangci.yml` instead of copying the file directly.

Service Earthfiles reference these artifacts with the syntax `COPY ../../+gen-mod/gen ../gen`. The `../../` points to the root Earthfile's directory, `+gen-mod` is the target name, and `/gen` is the artifact name declared in `SAVE ARTIFACT`.

### Orchestration Targets

The remaining targets delegate to the service Earthfiles using `BUILD` directives with cross-directory target references:

```earthfile
ci:
    BUILD ./services/auth+lint
    BUILD ./services/auth+test
    BUILD ./services/catalog+lint
    BUILD ./services/catalog+test
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
    BUILD ./services/reservation+lint
    BUILD ./services/reservation+test
    BUILD ./services/search+lint
    BUILD ./services/search+test

lint:
    BUILD ./services/auth+lint
    BUILD ./services/catalog+lint
    BUILD ./services/gateway+lint
    BUILD ./services/reservation+lint
    BUILD ./services/search+lint

test:
    BUILD ./services/auth+test
    BUILD ./services/catalog+test
    BUILD ./services/gateway+test
    BUILD ./services/reservation+test
    BUILD ./services/search+test

docker:
    BUILD ./services/auth+docker
    BUILD ./services/catalog+docker
    BUILD ./services/gateway+docker
    BUILD ./services/reservation+docker
    BUILD ./services/search+docker
```

The `+ci` target runs lint and test across all five services. Earthly executes `BUILD` directives in parallel by default — all ten targets run concurrently, subject to resource limits. On a developer workstation with eight cores, the wall-clock time for `earthly +ci` is roughly the time for the slowest single service, not the sum of all services.

`+lint` and `+test` are convenience targets for running one phase across all services. You would use `earthly +lint` after editing `.golangci.yml` to confirm no regressions.

`+docker` builds all five images. You run this before pushing to a registry or before starting the local Docker Compose stack.

The cross-directory reference syntax `./services/catalog+lint` is how Earthly addresses targets in other Earthfiles. The path before the `+` is the directory containing the Earthfile; the part after `+` is the target name.

---

## Caching

Earthly's caching model is layer-based, like Docker. Each `RUN`, `COPY`, and `FROM` instruction produces a layer. If the inputs to a layer have not changed since the last run, the cached result is reused.

The dependency structure of the catalog Earthfile is designed to maximize cache hits:

```
deps  (changes when go.mod changes)
  └── src  (changes when any source file changes)
        ├── lint  (changes when .golangci.yml changes)
        ├── test  (changes with src)
        └── build  (changes with src)
              └── docker  (changes when binary changes)
```

On a typical development iteration — editing a `.go` file and running `earthly +test` — the `deps` layer is served from cache; only `src` and `test` actually execute. Module downloads do not happen again.

### Remote Cache

For CI environments where no persistent local cache exists, Earthly supports a remote cache backed by a container registry:

```bash
# CI: push cache layers to the registry after building
earthly --push --remote-cache=ghcr.io/yourorg/library-cache +ci

# CI (next run): pull cache layers before building
earthly --remote-cache=ghcr.io/yourorg/library-cache +ci
```

When remote cache is configured, CI agents share cache across runs and across branches. A `deps` layer cached from main is reused on a feature branch as long as `go.mod` has not changed. This can cut CI times substantially on large projects.

The `--push` flag does two things: it pushes `SAVE IMAGE` outputs to their registries, and it updates the remote cache. You typically use `--push` only on the main branch, to avoid polluting the shared cache with experimental branches.

### Forcing a Clean Build

If you suspect a corrupted cache:

```bash
# Bypass the cache for this run
earthly --no-cache +ci
```

This re-executes every layer from scratch. You rarely need it, but it is the escape hatch when something unexplained is happening.

---

## Exercises

1. **Add a `proto` target.** Add a new target to the catalog Earthfile that installs `protoc` and the Go protobuf plugins, copies the `.proto` files, and generates the gRPC stubs. Save the generated files as artifacts. Consider where this target fits in the dependency graph.

2. **Benchmark the cache.** Run `earthly ./services/catalog+test` twice in a row. Note the wall-clock times. Then modify a single line in `internal/service/catalog.go` and run it again. Which layers were cached on the third run? Which were not?

3. **Parallelize the root `+ci` target.** The current `+ci` target runs `BUILD` directives for lint and test sequentially within each service listing. Earthly actually runs independent `BUILD` directives in parallel automatically — confirm this by looking at the Earthly output and identifying which targets ran concurrently. How does this compare to `./gradlew check` on a multi-module Gradle project?

4. **Add a `migrate` target.** Add a target to the catalog Earthfile that runs the database migrations using `golang-migrate`. The target should accept the database DSN as an `ARG`. Think about what base image is appropriate for this target and whether it should run in CI or only during deployment.

---

## References

[^1]: [Earthly documentation — Earthfile reference](https://docs.earthly.dev/docs/earthfile)
[^2]: [Earthly documentation — Remote caching](https://docs.earthly.dev/docs/caching/caching-via-registry)
[^3]: [Earthly documentation — Earthly CI integration overview](https://docs.earthly.dev/ci-integration)
