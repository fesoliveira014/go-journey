# 10.2 The Earthly Build System

<!-- [STRUCTURAL] Major section. Progression: hook → what Earthly is → concepts table → full walkthrough → variations → root Earthfile → local/cloud equivalence → caching → exercises. Logical, with clear pay-off. One concern: "Key Concepts" table (26 entries) and the full Earthfile walkthrough are very close together — consider splitting the walkthrough into collapsible subsections or moving the full file to an appendix and introducing each target separately. -->
<!-- [LINE EDIT] Opening sentence, 44 words. Split it. "Every team eventually ends up with a build script that works on the CI server but not on your laptop, or vice versa. The usual culprit is environmental differences: different versions of Go, different lint tool versions, missing environment variables, or system dependencies that someone forgot to document." → "Every team eventually ends up with a build script that works in CI but not locally — or vice versa. The culprit is almost always environmental drift: different Go versions, different lint tool versions, missing environment variables, undocumented system dependencies." -->
Every team eventually ends up with a build script that works on the CI server but not on your laptop, or vice versa. The usual culprit is environmental differences: different versions of Go, different lint tool versions, missing environment variables, or system dependencies that someone forgot to document. Earthly solves this problem by running every build step inside a container. If it builds on your laptop, it builds in CI -- and the outputs are bit-for-bit identical.

<!-- [COPY EDIT] "bit-for-bit identical" — compound adjective, hyphenated before noun per CMOS 7.81 (though "identical" is predicative here; acceptable either way). -->
<!-- [COPY EDIT] Convert all `--` to em dashes `—` throughout this file (CMOS 6.85). Repeated pattern — flagged once. -->
This section covers Earthly in depth, because it is the primary build and CI tool for this project. By the end you will understand how to read and write Earthfiles, how the caching model works, and how the root Earthfile orchestrates builds across all five services.

---

## What Earthly Is

<!-- [STRUCTURAL] Good framing. The three-bullet mental model is the right first frame. -->
Earthly is a build tool that combines Dockerfile syntax with Makefile-style named targets. The mental model is:

<!-- [COPY EDIT] "`FROM` base image" — backticks consistent. -->
- Every target runs inside its own container, derived from a `FROM` base image.
- Targets can depend on other targets. Earthly resolves the dependency graph and runs them in the right order.
- Outputs (files, Docker images) are explicitly declared with `SAVE ARTIFACT` and `SAVE IMAGE`. Nothing leaks out of a container unless you ask for it.

<!-- [LINE EDIT] "If you have used Gradle, think of each Earthly target as a Gradle task, except the task runs in a reproducible container rather than on the host JVM." (28 words) — good. -->
<!-- [COPY EDIT] "`:docker` depends on `:build`, which depends on `:src`, which depends on `:deps`" — Gradle uses colon-prefixed task paths; acceptable but unusual. Consider a more canonical Gradle example such as "`assemble` depends on `compileJava`, which depends on `processResources`". -->
If you have used Gradle, think of each Earthly target as a Gradle task, except the task runs in a reproducible container rather than on the host JVM. The dependency semantics are the same: `:docker` depends on `:build`, which depends on `:src`, which depends on `:deps`. Earthly evaluates that chain automatically.

<!-- [LINE EDIT] "The key difference from a raw Dockerfile is named targets with explicit inputs and outputs." — good; keep. -->
The key difference from a raw Dockerfile is named targets with explicit inputs and outputs. A Dockerfile is a linear script that produces one image. An Earthfile is a graph of targets, each producing artifacts that others can consume.

<!-- [LINE EDIT] "Earthly is also distinct from Docker Compose. Compose is for running services. Earthly is for building them." — short, punchy. Keep. -->
Earthly is also distinct from Docker Compose. Compose is for running services. Earthly is for building them. You run Earthly during development and CI to produce binaries and images; you run Compose (or Kubernetes) to run those images.

---

## Key Concepts

<!-- [STRUCTURAL] This table is useful reference. Consider anchoring each row with a link to the relevant section of the Earthly docs. -->
<!-- [COPY EDIT] Column header alignment `|---------|-------------|` — fine. -->
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

<!-- [COPY EDIT] Please verify: Earthly spec version `0.8` is the current recommended version as of 2026-04-15. -->

---

## The Catalog Earthfile: A Full Walkthrough

<!-- [STRUCTURAL] The full file shown first and then walked through target-by-target is a strong pedagogical choice — gives the reader a big picture before the details. Good. -->
The catalog service Earthfile is the canonical example. All five service Earthfiles follow the same pattern with minor variations. Here is the full file:

<!-- [COPY EDIT] Please verify: `golang:1.26-alpine` — Go 1.26 expected release timing; confirm this is a real, pullable tag at 2026-04-15. As of cutoff, Go 1.22 is current. -->
<!-- [COPY EDIT] Please verify: `alpine:3.19` still the recommended LTS-flavored base at 2026-04-15. Alpine 3.20/3.21 may be current. -->
<!-- [COPY EDIT] Please verify: `golangci-lint` version `v1.64.8` exists and is current. -->
```earthfile
VERSION 0.8

FROM golang:1.26-alpine

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

<!-- [LINE EDIT] "This target copies only the `go.mod` and `go.sum` files -- nothing else -- and then runs `go mod download`." — clean. -->
This target copies only the `go.mod` and `go.sum` files -- nothing else -- and then runs `go mod download`. The reason for this separation is layer caching.

<!-- [LINE EDIT] "By copying only the module files first and running `go mod download` before copying any application source, you ensure that dependency downloads only happen when dependencies actually change -- not every time you edit a `.go` file." (40 words) — tight; keep. -->
Docker (and Earthly) cache build layers. If the inputs to a step have not changed since the last run, the cached layer is reused. By copying only the module files first and running `go mod download` before copying any application source, you ensure that dependency downloads only happen when dependencies actually change -- not every time you edit a `.go` file.

<!-- [LINE EDIT] "If you have ever written a Java Dockerfile, you have done the same thing: copy `pom.xml` or `build.gradle` first, run the dependency download step, then copy the source." — good. -->
If you have ever written a Java Dockerfile, you have done the same thing: copy `pom.xml` or `build.gradle` first, run the dependency download step, then copy the source.

<!-- [LINE EDIT] "The `COPY ../../+gen-mod/gen ../gen` syntax is a cross-Earthfile artifact reference." (12 words) — fine. The following sentence is 43 words; consider breaking. "Instead of directly copying `../../gen/go.mod` (which would fail because Earthly's build context is scoped to the service directory), the service Earthfile references artifact targets defined in the root Earthfile." → "Instead of directly copying `../../gen/go.mod` — which would fail because Earthly scopes each service's build context to its own directory — the service Earthfile references an artifact target defined in the root Earthfile." -->
The `COPY ../../+gen-mod/gen ../gen` syntax is a cross-Earthfile artifact reference. Instead of directly copying `../../gen/go.mod` (which would fail because Earthly's build context is scoped to the service directory), the service Earthfile references artifact targets defined in the root Earthfile. The root `+gen-mod` target copies `gen/go.mod` and `gen/go.sum` into a `scratch` container and saves them as a named artifact. The service then pulls that artifact into its own container. This indirection is necessary in monorepos where local modules live outside the service's build context.

<!-- [COPY EDIT] "replace'd module" — non-standard apostrophe contraction. Suggest: "each `replace`d module" → "each module named in a `replace` directive" or "each replaced module". The apostrophe is informal (CMOS 7.70 on forming verbal forms of proper names). -->
The `go mod download` command is chained with subshell calls for each local module: `(cd ../gen && go mod download)`. Each replace'd module has its own `go.mod` with its own dependencies, and Go does not transitively download them. Without these subshells, the build would fail later when the compiler tries to resolve imports from the local modules.

<!-- [LINE EDIT] "The project uses Go workspaces for local development, but `GOWORK=off` disables that inside the container." — good. -->
<!-- [COPY EDIT] "the `replace` directives in `go.mod`" — backticks. Good. -->
The project uses Go workspaces for local development, but `GOWORK=off` disables that inside the container. Inside the container, the local modules (`gen`, `pkg/auth`, `pkg/otel`) are placed at paths that match the `replace` directives in `go.mod`, so Go resolves them locally without needing the workspace. This is a deliberate design: workspaces are a developer convenience, but the container build should be explicit about module resolution.

<!-- [STRUCTURAL] The `SAVE ARTIFACT ... AS LOCAL` trick is subtle and important. Readers from Gradle/Maven will wonder why a build step writes back to the repo. Consider a sentence on why this round-trip is useful. -->
<!-- [LINE EDIT] "`SAVE ARTIFACT go.mod AS LOCAL go.mod` writes the resolved `go.mod` back to your working directory. This is how `go mod tidy` updates (run inside the container) flow back out to your checkout." — clarify: it does not flow back out *unless* `go mod tidy` is run inside the container or `go mod download` causes a go.sum change. Consider: "This is how `go mod tidy` (run inside the container via another target) pushes updated files back to your checkout." -->
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

<!-- [LINE EDIT] "`FROM +deps` starts this target from the state of the `deps` target -- the base image with all Go modules already downloaded." — clean. -->
`FROM +deps` starts this target from the state of the `deps` target -- the base image with all Go modules already downloaded. Now we copy the actual source code.

The source copy uses the `+gen-src`, `+pkg-auth-src`, and `+pkg-otel-src` artifact targets from the root Earthfile, following the same cross-Earthfile pattern as `deps`. The `-src` variants copy the full source trees (not just `go.mod` files).

<!-- [LINE EDIT] "This target is invalidated whenever any `.go` file in `cmd/`, `internal/`, `migrations/`, `gen/`, `pkg/auth/`, or `pkg/otel/` changes." — good. -->
<!-- [COPY EDIT] "`cmd/`, `internal/`, `migrations/`, `gen/`, `pkg/auth/`, or `pkg/otel/`" — serial comma present (CMOS 6.19). Good. -->
This target is invalidated whenever any `.go` file in `cmd/`, `internal/`, `migrations/`, `gen/`, `pkg/auth/`, or `pkg/otel/` changes. That is fine, because copying source is fast. What matters is that the `deps` layer below it stays cached as long as dependencies have not changed.

<!-- [LINE EDIT] "Notice that `src` produces no `SAVE ARTIFACT`. It is an intermediate target. Its purpose is to give `lint`, `test`, and `build` a common starting point so that each of those targets does not need to repeat the COPY instructions." (43 words) — could split: "Notice that `src` produces no `SAVE ARTIFACT`; it is an intermediate target. Its job is to give `lint`, `test`, and `build` a common starting point, so none of them needs to repeat the `COPY` instructions." -->
Notice that `src` produces no `SAVE ARTIFACT`. It is an intermediate target. Its purpose is to give `lint`, `test`, and `build` a common starting point so that each of those targets does not need to repeat the COPY instructions.

<!-- [COPY EDIT] "the COPY instructions" — `COPY` should be in backticks for consistency with rest of text. -->
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

The linter is installed inside the container at a pinned version. This means every developer and every CI run uses exactly the same linter version -- no drift, no "works on my machine" lint failures.

<!-- [LINE EDIT] "The `.golangci.yml` configuration is copied from the root Earthfile's `+golangci-config` artifact target. It lives at the repository root (not inside the service directory) so all services share the same lint rules." — good. -->
The `.golangci.yml` configuration is copied from the root Earthfile's `+golangci-config` artifact target. It lives at the repository root (not inside the service directory) so all services share the same lint rules. If you added a new rule to `.golangci.yml`, all five services would pick it up the next time `+lint` ran.

<!-- [STRUCTURAL] Good explanation of why `RUN go build ./...` precedes golangci-lint. This kind of "here's the gotcha" detail is exactly what a tutor-voice book should flag. -->
<!-- [COPY EDIT] "replace'd local modules" — second instance; see earlier note. Suggest "`replace`d" or just "replaced". -->
The `RUN go build ./...` step before `golangci-lint run` is deliberate. Some linters (notably those based on `go/analysis`) need compiled export data for dependencies to perform type-aware checks. When the replace'd local modules (`gen`, `pkg/auth`, `pkg/otel`) have not been built, golangci-lint may report "could not load export data" errors. Running `go build` first populates the build cache with the necessary export data.

<!-- [LINE EDIT] "Note that `.golangci.yml` is copied after `+src`. This means a change to lint configuration invalidates the lint layer but not the source layer -- the compiled package cache is still reused." — good. -->
Note that `.golangci.yml` is copied after `+src`. This means a change to lint configuration invalidates the lint layer but not the source layer -- the compiled package cache is still reused.

### `test` — Unit Tests

```earthfile
test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1
```

<!-- [LINE EDIT] "Tests run against the same `+src` layer used by `lint` and `build`. The `-count=1` flag disables Go's test result cache, ensuring tests always run fresh." — good. -->
Tests run against the same `+src` layer used by `lint` and `build`. The `-count=1` flag disables Go's test result cache, ensuring tests always run fresh. The `-v` flag produces verbose output, which is useful in CI logs.

<!-- [STRUCTURAL] Good callout about scoping to service+handler to avoid integration tests. Consider forward-referencing the integration-test target by name so readers know where that shows up. -->
The test target deliberately scopes to `./internal/service/...` and `./internal/handler/...` rather than `./...`. This avoids running integration tests that require a live database or Kafka broker. Those belong in a separate target that accepts a running Docker network -- a topic covered in a later section.

<!-- [COPY EDIT] "a later section" — vague cross-reference. Name the chapter or section if possible. -->

### `build` — Compile

```earthfile
build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/
    SAVE ARTIFACT /bin/catalog
```

<!-- [LINE EDIT] "`CGO_ENABLED=0` produces a statically linked binary. This is important because the binary will run inside an `alpine` container that may not have glibc." — good. -->
<!-- [COPY EDIT] "glibc" is lowercase by convention (project/library name). Good. -->
`CGO_ENABLED=0` produces a statically linked binary. This is important because the binary will run inside an `alpine` container that may not have glibc. A dynamically linked Go binary would fail to start with a "no such file or directory" error on the dynamic linker. Static binaries have no such dependency.

<!-- [COPY EDIT] Technical note: Alpine uses musl, not glibc. A CGO-linked binary built on Debian glibc would fail on Alpine. Within Alpine-to-Alpine the link would work. The current wording is accurate enough, but consider tightening: "Alpine uses musl, not glibc. A Go binary compiled against glibc on Debian would fail to start on Alpine. `CGO_ENABLED=0` sidesteps the problem by producing a statically linked binary with no libc dependency." -->
<!-- [LINE EDIT] "`SAVE ARTIFACT /bin/catalog` makes the compiled binary available to other targets. Without this declaration, the binary exists only inside this container and disappears when the target finishes. With it, any target can reference it as `+build/catalog`." — clean. -->
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

<!-- [STRUCTURAL] Explaining why `FROM alpine:3.19` (not `FROM +src`) is the key mental model shift. Good placement. -->
<!-- [COPY EDIT] "(typically under 20MB for a static Go binary on Alpine)" — CMOS 9.17 uses a thin space or no space between number and unit abbreviation; "20 MB" with space is CMOS-preferred in prose. Technical contexts sometimes drop the space — be consistent throughout. -->
This target starts fresh from `alpine:3.19` -- not from `+src`. The Go toolchain, module cache, and source code are not needed at runtime. The final image contains only the Alpine base and the compiled binary. This keeps the image small (typically under 20MB for a static Go binary on Alpine) and reduces attack surface.

`COPY +build/catalog /usr/local/bin/catalog` pulls the artifact saved by the `build` target into this container. This is Earthly's artifact reference syntax: `+targetname/path`.

<!-- [LINE EDIT] "`SAVE IMAGE catalog:latest` tags the container image. You can push it with `earthly --push +docker`, which runs the target and pushes the resulting image to a registry." — good. -->
`SAVE IMAGE catalog:latest` tags the container image. You can push it with `earthly --push +docker`, which runs the target and pushes the resulting image to a registry.

---

## Service Variations

<!-- [STRUCTURAL] Good section. Hits the "all five services" promise from the intro. Could benefit from a short summary table mapping service → port → pkg/otel yes/no → CGO yes/no. -->
All five Earthfiles share the same structure. The differences are small but worth noting.

### Auth Service

<!-- [LINE EDIT] "The auth Earthfile omits the `pkg/otel` dependency. The auth service uses structured logging and basic instrumentation but does not pull in the shared OpenTelemetry package directly." — good. -->
<!-- [STRUCTURAL] Stating that auth "does not pull in the shared OpenTelemetry package directly" raises an eyebrow for readers who assume all services should emit traces. Consider a one-line justification or a forward pointer to the observability chapter. -->
The auth Earthfile omits the `pkg/otel` dependency. The auth service uses structured logging and basic instrumentation but does not pull in the shared OpenTelemetry package directly. Both its `deps` and `src` targets copy only `gen` and `pkg/auth`, not `pkg/otel`.

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

<!-- [LINE EDIT] "The gateway's `test` target differs from the other services. The `-race` flag enables the Go race detector, which requires CGO. On Alpine, CGO needs `gcc` and `musl-dev` installed, plus `CGO_ENABLED=1` explicitly set." — good. -->
The gateway's `test` target differs from the other services. The `-race` flag enables the Go race detector, which requires CGO. On Alpine, CGO needs `gcc` and `musl-dev` installed, plus `CGO_ENABLED=1` explicitly set. The other services skip the race detector and run with `CGO_ENABLED=0` by default.

<!-- [STRUCTURAL] Implicit question: if the race detector is valuable for the gateway, why not for all services? Anticipate this and answer (e.g., "the gateway has the most concurrent request handlers; the others are I/O-bound and less prone to data races"). -->
The gateway image exposes port 8080 instead of a gRPC port. It also sets `WORKDIR /app` so that the templates and static directories are at predictable relative paths when the binary looks for them at runtime.

### Search Service

The search service, like auth, omits `pkg/otel`. It is otherwise identical to catalog in structure. The `docker` target exposes port 50054.

<!-- [STRUCTURAL] Same question as auth: why does search skip observability? Tie to the observability chapter or justify. -->

### Reservation Service

Reservation follows the same pattern as catalog, exposing port 50053. It depends on both `pkg/auth` (for JWT validation) and `pkg/otel`.

---

## The Root Earthfile

The root Earthfile serves two roles: it defines shared artifact targets that service Earthfiles depend on, and it orchestrates builds across all five services.

### Shared Artifact Targets

<!-- [LINE EDIT] "In a monorepo, service Earthfiles cannot directly `COPY` files from outside their directory. Earthly scopes each Earthfile's build context to its own directory, so `COPY ../../gen/go.mod` would fail." — good. -->
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

<!-- [LINE EDIT] "Each shared module has two artifact targets: a `-mod` target (just `go.mod` and `go.sum`, for the `deps` layer) and a `-src` target (the full source tree, for the `src` layer)." — good. -->
<!-- [COPY EDIT] "-mod" vs "`-mod`" — backticks consistent elsewhere; apply to both the `-mod` and `-src` mentions. -->
Each shared module has two artifact targets: a `-mod` target (just `go.mod` and `go.sum`, for the `deps` layer) and a `-src` target (the full source tree, for the `src` layer). This split preserves the layer caching strategy -- changes to source files do not invalidate the dependency download layer.

The `golangci-config` target does the same for the shared lint configuration. All service `+lint` targets reference `../../+golangci-config/.golangci.yml` instead of copying the file directly.

<!-- [LINE EDIT] "Service Earthfiles reference these artifacts with the syntax `COPY ../../+gen-mod/gen ../gen`. The `../../` points to the root Earthfile's directory, `+gen-mod` is the target name, and `/gen` is the artifact name declared in `SAVE ARTIFACT`." — 39 words; fine. -->
<!-- [COPY EDIT] Inconsistency: earlier examples show `COPY ../../+gen-mod/gen ../gen` (no leading slash on "gen"), and `SAVE ARTIFACT /gen gen` makes `/gen` the source and `gen` the *artifact name*. The explanation says "`/gen` is the artifact name" — it should be "`gen` (without slash) is the artifact name, declared via `SAVE ARTIFACT /gen gen`." Please verify and correct. -->
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

<!-- [LINE EDIT] "The `+ci` target runs lint and test across all five services. Earthly executes `BUILD` directives in parallel by default -- all ten targets run concurrently, subject to resource limits." — clean. -->
<!-- [COPY EDIT] "eight cores" — CMOS 9.7 spells out numbers below 100 in prose. Good. -->
The `+ci` target runs lint and test across all five services. Earthly executes `BUILD` directives in parallel by default -- all ten targets run concurrently, subject to resource limits. On a developer workstation with eight cores, the wall-clock time for `earthly +ci` is roughly the time for the slowest single service, not the sum of all services.

`+lint` and `+test` are convenience targets for running one phase across all services. You would use `earthly +lint` after editing `.golangci.yml` to confirm no regressions.

`+docker` builds all five images. You run this before pushing to a registry or before starting the local Docker Compose stack.

<!-- [LINE EDIT] "The cross-directory reference syntax `./services/catalog+lint` is how Earthly addresses targets in other Earthfiles. The path before the `+` is the directory containing the Earthfile; the part after `+` is the target name." — good. -->
The cross-directory reference syntax `./services/catalog+lint` is how Earthly addresses targets in other Earthfiles. The path before the `+` is the directory containing the Earthfile; the part after `+` is the target name.

---

## Local = Cloud

<!-- [STRUCTURAL] Section heading "Local = Cloud" is catchy. Payoff well delivered. -->
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

<!-- [LINE EDIT] "In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` -- the same command you run locally. There is no CI-specific build script to maintain and no category of 'it worked locally but failed in CI' failures caused by environment differences." (44 words) — split: "In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` — the same command you run locally. There is no CI-specific build script to maintain. The category of 'it worked locally but failed in CI' failures from environment drift disappears." -->
In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` -- the same command you run locally. There is no CI-specific build script to maintain and no category of "it worked locally but failed in CI" failures caused by environment differences.

<!-- [LINE EDIT] "If a lint failure appears in CI, you run `earthly ./services/catalog+lint` locally and see the exact same output. If a test fails in CI, you run `earthly ./services/catalog+test` and reproduce it immediately." — good parallel structure. -->
If a lint failure appears in CI, you run `earthly ./services/catalog+lint` locally and see the exact same output. If a test fails in CI, you run `earthly ./services/catalog+test` and reproduce it immediately.

<!-- [LINE EDIT] "This is a meaningful shift from the traditional model where developers run `go test ./...` locally (with whatever version of Go happens to be installed) and CI runs a slightly different set of steps in a managed environment." (38 words) — good. -->
This is a meaningful shift from the traditional model where developers run `go test ./...` locally (with whatever version of Go happens to be installed) and CI runs a slightly different set of steps in a managed environment. With Earthly, the container is the environment, and it is the same container everywhere.

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

<!-- [LINE EDIT] "On a typical development iteration -- editing a `.go` file and running `earthly +test` -- the `deps` layer is served from cache, only `src` and `test` actually execute. Module downloads do not happen again." (35 words) — comma splice before "only". Fix: "On a typical development iteration — editing a `.go` file and running `earthly +test` — the `deps` layer is served from cache; only `src` and `test` actually execute. Module downloads do not happen again." -->
<!-- [COPY EDIT] Comma splice "served from cache, only `src`" — replace comma with semicolon or period (CMOS 6.22). -->
On a typical development iteration -- editing a `.go` file and running `earthly +test` -- the `deps` layer is served from cache, only `src` and `test` actually execute. Module downloads do not happen again.

### Remote Cache

For CI environments where no persistent local cache exists, Earthly supports a remote cache backed by a container registry:

```bash
# CI: push cache layers to the registry after building
earthly --push --remote-cache=ghcr.io/yourorg/library-cache +ci

# CI (next run): pull cache layers before building
earthly --remote-cache=ghcr.io/yourorg/library-cache +ci
```

<!-- [COPY EDIT] Please verify: The `--remote-cache` flag syntax and semantics on Earthly 0.8.x. Confirm that the flag is still `--remote-cache=<registry>` and not a different form (e.g., subcommand or environment variable). -->
<!-- [LINE EDIT] "When remote cache is configured, CI agents share cache across runs and across branches. A `deps` layer cached from main is reused on a feature branch as long as `go.mod` has not changed." — good. -->
When remote cache is configured, CI agents share cache across runs and across branches. A `deps` layer cached from main is reused on a feature branch as long as `go.mod` has not changed. This can cut CI times substantially on large projects.

<!-- [LINE EDIT] "The `--push` flag does two things: it pushes `SAVE IMAGE` outputs to their registries, and it updates the remote cache. You typically use `--push` only on the main branch, to avoid polluting the shared cache with experimental branches." (40 words) — good. -->
The `--push` flag does two things: it pushes `SAVE IMAGE` outputs to their registries, and it updates the remote cache. You typically use `--push` only on the main branch, to avoid polluting the shared cache with experimental branches.

### Forcing a Clean Build

If you suspect a corrupted cache:

```bash
# Bypass the cache for this run
earthly --no-cache +ci
```

This re-executes every layer from scratch. You rarely need it, but it is the escape hatch when something unexplained is happening.

<!-- [COPY EDIT] "something unexplained is happening" — fine; could tighten: "something you cannot explain". -->

---

## Exercises

<!-- [STRUCTURAL] Four exercises hitting: extending Earthfile, benchmarking cache, understanding parallel BUILD, adding a migration target. Good variety. -->
1. **Add a `proto` target.** Add a new target to the catalog Earthfile that installs `protoc` and the Go protobuf plugins, copies the `.proto` files, and generates the gRPC stubs. Save the generated files as artifacts. Consider where this target fits in the dependency graph.

<!-- [COPY EDIT] "wall-clock" — compound adjective, hyphenated before noun (CMOS 7.81). Good. -->
2. **Benchmark the cache.** Run `earthly ./services/catalog+test` twice in a row. Note the wall-clock times. Then modify a single line in `internal/service/catalog.go` and run it again. Which layers were cached on the third run? Which were not?

<!-- [LINE EDIT] "The current `+ci` target runs `BUILD` directives for lint and test sequentially within each service listing. Earthly actually runs independent `BUILD` directives in parallel automatically -- confirm this by looking at the Earthly output and identifying which targets ran concurrently." (41 words) — split: "The current `+ci` target lists `BUILD` directives for lint and test service-by-service. In fact, Earthly runs independent `BUILD` directives in parallel automatically — confirm this by reading the Earthly output and identifying which targets ran concurrently." -->
<!-- [STRUCTURAL] Exercise 3 is interesting but a little tangled. The earlier text already claims "Earthly executes `BUILD` directives in parallel by default". Rephrase as "Prove it: run `+ci` with `--verbose` (or `--interactive-debug`) and identify which targets ran concurrently." -->
3. **Parallelize the root `+ci` target.** The current `+ci` target runs `BUILD` directives for lint and test sequentially within each service listing. Earthly actually runs independent `BUILD` directives in parallel automatically -- confirm this by looking at the Earthly output and identifying which targets ran concurrently. How does this compare to `./gradlew check` on a multi-module Gradle project?

<!-- [COPY EDIT] "golang-migrate" — tool name, render as inline code `golang-migrate`. The GitHub org/repo is `golang-migrate/migrate`; consider linking. -->
4. **Add a `migrate` target.** Add a target to the catalog Earthfile that runs the database migrations using `golang-migrate`. The target should accept the database DSN as an `ARG`. Think about what base image is appropriate for this target and whether it should run in CI or only during deployment.

---

## References

<!-- [COPY EDIT] References use plain URL form rather than Markdown link form. Other files (index.md, cicd-fundamentals.md) use `[Title](URL)` form. Be consistent. Suggest: `[Earthly Earthfile reference](https://docs.earthly.dev/docs/earthfile)`. -->
<!-- [COPY EDIT] Please verify URLs: https://docs.earthly.dev/docs/earthfile ; https://docs.earthly.dev/docs/caching/caching-via-registry ; https://docs.earthly.dev/ci-integration -->
[^1]: Earthly documentation — Earthfile reference: https://docs.earthly.dev/docs/earthfile
[^2]: Earthly documentation — Remote caching: https://docs.earthly.dev/docs/caching/caching-via-registry
[^3]: Earthly documentation — Earthly CI integration overview: https://docs.earthly.dev/ci-integration
