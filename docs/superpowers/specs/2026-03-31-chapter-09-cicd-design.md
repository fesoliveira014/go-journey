# Chapter 9: CI/CD with GitHub Actions & Earthly — Design Spec

## Goal

Teach the reader how to set up a complete CI/CD pipeline for the library management system using **Earthly** as the build system and **GitHub Actions** as the CI/CD orchestrator. The chapter takes a hybrid approach: Earthly handles the build logic (lint, test, build, Docker image), GitHub Actions handles workflow orchestration (triggers, secrets, image publishing). For each Earthly-based step, the pure GitHub Actions equivalent is shown inline, so the reader understands both tools.

## Prerequisites

- Chapters 1–8 completed (all 5 services running, Dockerfiles exist, Earthfiles partially exist)
- Docker installed locally
- Earthly CLI installed (referenced since Chapter 1)
- A GitHub repository (for Actions)

## Architecture Overview

```
Developer Push
       │
       ▼
┌─────────────────────────────┐
│  GitHub Actions Workflow    │
│                             │
│  PR → earthly +ci           │
│        (lint + test)        │
│                             │
│  main → earthly +ci         │
│          │                  │
│          ▼                  │
│  build-and-push (matrix)    │
│  ┌───────────────────────┐  │
│  │ auth    → ghcr.io     │  │
│  │ catalog → ghcr.io     │  │
│  │ gateway → ghcr.io     │  │
│  │ reservation → ghcr.io │  │
│  │ search  → ghcr.io     │  │
│  └───────────────────────┘  │
└─────────────────────────────┘
```

## Current State

### Earthfiles that exist

| Service | Earthfile | Lint Target | Notes |
|---------|-----------|-------------|-------|
| Root | `Earthfile` | Orchestrates `+ci`, `+lint`, `+test` | References `./services/auth+lint` but auth has no Earthfile — must be created |
| Catalog | `services/catalog/Earthfile` | `go vet` | Has `deps`, `src`, `lint`, `test`, `build`, `docker`. Needs golangci-lint |
| Gateway | `services/gateway/Earthfile` | `golangci-lint` | Already has golangci-lint. Missing `templates/` and `static/` in `docker` target |
| Reservation | `services/reservation/Earthfile` | `go vet` | Needs golangci-lint |
| Search | `services/search/Earthfile` | `go vet` | Needs golangci-lint. Does not depend on `pkg/otel` (correct as-is) |
| Auth | **Missing** | N/A | Must be created from scratch |

### GitHub Actions workflow that exists

`.github/workflows/ci.yml` — a combined push+PR workflow that runs `earthly +ci`. Pins Earthly v0.8.15. This file will be **deleted and replaced** by two separate workflows (`pr.yml` and `main.yml`) to teach the PR-gate vs. main-publish separation.

### What does not exist

- No `.golangci.yml` config
- No auth service Earthfile
- No image publishing pipeline

## Part 1: Chapter Sections

### Section 9.1: CI/CD Fundamentals

**Type:** Theory only — no code changes.

**Content:**
- Continuous Integration: merge frequently, run automated checks on every push
- Continuous Delivery: every green build is a releasable artifact (Docker image)
- Continuous Deployment: auto-deploy to production (not covered in this chapter, deferred to Kubernetes chapter)
- The feedback loop: push → lint → test → build → publish
- Build reproducibility: why "works on my machine" is insufficient, how containerized builds (Earthly) and CI environments (GitHub Actions) solve it
- Two-tool approach: Earthly for build logic (portable, local-friendly), GitHub Actions for orchestration (triggers, secrets, cloud integration)
- JVM comparison: Earthly ≈ Gradle with built-in Docker layer caching; GitHub Actions ≈ Jenkins/TeamCity/GitHub CI

### Section 9.2: Earthly Build System

**Type:** Theory + code changes.

**Content:**
- What Earthly is: Dockerfile syntax + Makefile target structure. Each target runs in a container.
- Key concepts: `VERSION`, `FROM`, targets, `COPY`, `RUN`, `SAVE ARTIFACT`, `SAVE IMAGE`
- Walk through the existing catalog Earthfile target by target:
  - `deps` — copy go.mod files (including local modules), download dependencies. Explain layer caching: dependencies change rarely, so this layer is cached.
  - `src` — copy source code on top of deps. Invalidated on any code change but deps layer stays cached.
  - `lint` — run linter on source
  - `test` — run tests
  - `build` — compile binary with `CGO_ENABLED=0`
  - `docker` — minimal alpine image with just the binary
- The root Earthfile as orchestrator: `+ci` triggers `+lint` and `+test` across all services in parallel
- `earthly +ci` locally = same as CI — demonstrate the "local = cloud" principle
- Earthly caching: automatic layer caching, `--push` flag for image push, remote cache with `--remote-cache`

**Code changes:**

1. **Create `services/auth/Earthfile`** — same pattern as catalog:
   - `deps`: copy `go.mod`, `gen/go.mod`, `pkg/auth/go.mod` (auth does not depend on `pkg/otel`)
   - `src`: `COPY --dir cmd internal migrations ./` plus `gen/` and `pkg/auth/`
   - `lint`: golangci-lint `@v1.57.2`
   - `test`: `go test ./internal/service/... ./internal/handler/... -v -count=1`
   - `build`: `CGO_ENABLED=0 go build -o /bin/auth ./cmd/`, then `SAVE ARTIFACT /bin/auth`
   - `docker`: alpine + binary, expose 50051, `SAVE IMAGE auth:latest`

2. **Update `services/catalog/Earthfile`** — replace `go vet` with golangci-lint `@v1.57.2` install + run

3. **Update `services/reservation/Earthfile`** — replace `go vet` with golangci-lint `@v1.57.2` install + run

4. **Update `services/search/Earthfile`** — replace `go vet` with golangci-lint `@v1.57.2` install + run (no other dependency changes needed — search does not import `pkg/otel`)

5. **Update `services/gateway/Earthfile`** — fix bug: add `templates/` and `static/` dirs to `src` and `docker` targets:
   - `src` target: `COPY --dir cmd internal templates static ./`, add `SAVE ARTIFACT ./templates` and `SAVE ARTIFACT ./static`
   - `docker` target: add `COPY +build/gateway /usr/local/bin/gateway` (already exists), plus `COPY +src/templates /app/templates` and `COPY +src/static /app/static`, set `WORKDIR /app`

6. **Update root `Earthfile`** — add `+docker` target:
   ```
   docker:
       BUILD ./services/auth+docker
       BUILD ./services/catalog+docker
       BUILD ./services/gateway+docker
       BUILD ./services/reservation+docker
       BUILD ./services/search+docker
   ```

7. **Copy `.golangci.yml` into Earthly containers** — each service's `lint` target must `COPY` the root `.golangci.yml` into the container so the linter config takes effect. Add `COPY ../../.golangci.yml ./` (or equivalent relative path) to each lint target before running `golangci-lint run`.

8. **Create `.golangci.yml`** at repo root:
   ```yaml
   run:
     timeout: 5m

   linters:
     enable:
       - govet
       - errcheck
       - staticcheck
       - unused
       - gosimple
       - ineffassign
       - typecheck

   issues:
     exclude-use-default: true
   ```

### Section 9.3: GitHub Actions Workflows

**Type:** Theory + code changes.

**Content:**

**GitHub Actions concepts taught:**
- Workflow files: YAML in `.github/workflows/`
- Triggers: `on.pull_request`, `on.push`
- Jobs, steps, runners (`ubuntu-latest`)
- Actions marketplace: `actions/checkout`, `docker/login-action`, `docker/build-push-action`
- Job dependencies: `needs`
- Permissions: `packages: write` for GHCR
- Matrix strategy for parallel builds
- GitHub context variables: `github.sha`, `github.ref`, `github.actor`

**PR workflow** (`.github/workflows/pr.yml`):
```yaml
name: PR Check
on:
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci
```

Inline callout showing the pure GHA equivalent:
```yaml
# Alternative: without Earthly
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with:
      go-version: '1.26'
  - uses: golangci/golangci-lint-action@v6
  - run: go test ./...
```

**Main workflow** (`.github/workflows/main.yml`):
```yaml
name: CI/CD
on:
  push:
    branches: [main]

permissions:
  contents: read
  packages: write

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci

  build-and-push:
    needs: ci
    runs-on: ubuntu-latest
    strategy:
      matrix:
        service: [auth, catalog, gateway, reservation, search]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: services/${{ matrix.service }}/Dockerfile
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:sha-${{ github.sha }}
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:latest
```

Inline callout showing the Earthly-push equivalent:
```yaml
# Alternative: using Earthly for push
- run: |
    earthly --push ./services/${{ matrix.service }}+docker
```

**Design decision: Dockerfiles for publishing, Earthly for CI.** The `build-and-push` job uses `docker/build-push-action` with the existing Dockerfiles rather than `earthly --push`. Rationale: `docker/build-push-action` integrates natively with GHA's OIDC, caching, and provenance features, and is the standard pattern for GHCR publishing. The Earthly `+docker` target is used for local image building and verification. The chapter explicitly discusses this trade-off in Section 9.5.

**Code changes:**
1. Delete `.github/workflows/ci.yml`
2. Create `.github/workflows/pr.yml`
3. Create `.github/workflows/main.yml`

### Section 9.4: Linting & Code Quality

**Type:** Theory + code fixes.

**Content:**
- Why `go vet` is not enough: catches correctness bugs but misses style, unused code, unchecked errors
- What `golangci-lint` is: meta-linter that runs many linters in parallel, sharing AST parsing
- Walk through `.golangci.yml` — explain each enabled linter:
  - `govet` — correctness (printf format strings, struct tag validity, atomic operations)
  - `errcheck` — unchecked error returns (the most common Go bug)
  - `staticcheck` — broad static analysis suite (SA checks for bugs, S for simplifications, ST for style)
  - `unused` — dead code detection
  - `gosimple` — code that can be simplified
  - `ineffassign` — assignments to variables that are never read
  - `typecheck` — compilation errors (catches issues before other linters run)
- Running locally: `golangci-lint run ./...` from a service directory, or `earthly +lint` from anywhere
- Fixing violations: work through any real issues found when running against the codebase
- JVM comparison: golangci-lint ≈ Checkstyle + SpotBugs + PMD + ErrorProne combined into one tool; `.golangci.yml` ≈ `checkstyle.xml`

**Code changes:**
- Fix any lint violations discovered when running `golangci-lint` against the codebase

### Section 9.5: Image Publishing & Versioning

**Type:** Theory — deep dive into the `build-and-push` job from 9.3.

**Content:**
- Image tagging strategy:
  - `latest` — mutable, points to most recent build. Useful for dev, dangerous for production (which version is running?)
  - `sha-<commit>` — immutable, tied to a specific commit. Traceable: given a running container, you can find the exact code that built it
  - In production, you would also add semantic version tags (`v1.2.3`), but that requires a release process beyond this chapter's scope
- GHCR (GitHub Container Registry):
  - Images live at `ghcr.io/<owner>/<repo>/<service>`
  - Authenticated via `GITHUB_TOKEN` — no extra secrets needed
  - Package visibility defaults to private; can be made public in repo settings
- The `build-and-push` job walkthrough:
  - `docker/login-action` — authenticates with GHCR using the workflow's automatic `GITHUB_TOKEN`
  - `docker/build-push-action` — builds using the service's Dockerfile, pushes with two tags
  - Matrix strategy — runs 5 jobs in parallel, one per service. GitHub Actions parallelizes these across runners.
- Earthly alternative: `earthly --push +docker` pushes the image defined by `SAVE IMAGE --push`. Simpler syntax but less control over registry auth and tags.
- When to use which:
  - Earthly push: local development, simple setups, single registry
  - GHA docker actions: production pipelines (better secret management, build provenance, multi-platform builds with QEMU)
- JVM comparison: this is the equivalent of publishing JARs to Maven Central / Artifactory, but the artifact is a container image and the registry is GHCR

## Part 2: File Inventory

### New files

| File | Purpose |
|------|---------|
| `.golangci.yml` | Linter configuration |
| `.github/workflows/pr.yml` | PR check workflow |
| `.github/workflows/main.yml` | Main CI/CD workflow |
| `services/auth/Earthfile` | Auth service build targets |
| `docs/src/ch09/index.md` | Chapter 9 index page |
| `docs/src/ch09/cicd-fundamentals.md` | Section 9.1 |
| `docs/src/ch09/earthly.md` | Section 9.2 |
| `docs/src/ch09/github-actions.md` | Section 9.3 |
| `docs/src/ch09/linting.md` | Section 9.4 |
| `docs/src/ch09/image-publishing.md` | Section 9.5 |

### Deleted files

| File | Reason |
|------|--------|
| `.github/workflows/ci.yml` | Replaced by `pr.yml` and `main.yml` |

### Modified files

| File | Change |
|------|--------|
| `services/catalog/Earthfile` | Replace `go vet` with `golangci-lint`, copy `.golangci.yml` |
| `services/reservation/Earthfile` | Replace `go vet` with `golangci-lint`, copy `.golangci.yml` |
| `services/search/Earthfile` | Replace `go vet` with `golangci-lint`, copy `.golangci.yml` |
| `services/gateway/Earthfile` | Add `templates/`+`static/` to `src` and `docker`, copy `.golangci.yml` into lint |
| `Earthfile` (root) | Add `+docker` target |
| `docs/src/SUMMARY.md` | Add Chapter 9 entries |

## Part 3: Testing & Verification

1. `earthly +lint` — all 5 services pass golangci-lint
2. `earthly +test` — all existing tests pass
3. `earthly +ci` — full pipeline succeeds locally
4. `earthly +docker` — all 5 images build successfully
5. GitHub Actions workflows are syntactically valid YAML (can be validated with `actionlint` if available)
6. Documentation renders with `mdbook build`

## Part 4: Documentation Structure

```
Chapter 9: CI/CD with GitHub Actions & Earthly
├── index.md          — overview, architecture diagram, what you'll learn
├── cicd-fundamentals.md — CI vs CD, feedback loops, reproducibility
├── earthly.md        — Earthfile anatomy, targets, caching, golangci-lint
├── github-actions.md — workflows, triggers, jobs, matrix, secrets
├── linting.md        — golangci-lint deep dive, .golangci.yml config
└── image-publishing.md — GHCR, tagging strategy, Earthly vs GHA push
```

Each section follows the established pattern: theory → implementation → exercises → references. Cross-language comparisons to JVM/Gradle/Jenkins throughout.

**SUMMARY.md entries to add:**
```markdown
- [Chapter 9: CI/CD with GitHub Actions & Earthly](./ch09/index.md)
  - [9.1 CI/CD Fundamentals](./ch09/cicd-fundamentals.md)
  - [9.2 The Earthly Build System](./ch09/earthly.md)
  - [9.3 GitHub Actions Workflows](./ch09/github-actions.md)
  - [9.4 Linting & Code Quality](./ch09/linting.md)
  - [9.5 Image Publishing & Versioning](./ch09/image-publishing.md)
```

## Out of Scope

- Deployment to Kubernetes (Chapter 10)
- Multi-platform builds (arm64/amd64)
- Release management (semantic versioning, changelogs)
- Branch protection rules and required status checks (mentioned briefly, not configured)
- Self-hosted runners
- Earthly Satellites (remote runners)
