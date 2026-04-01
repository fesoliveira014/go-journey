# Chapter 9: CI/CD with GitHub Actions & Earthly — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up a complete CI/CD pipeline with Earthly for build logic and GitHub Actions for orchestration, plus write all Chapter 9 documentation.

**Architecture:** Earthly handles lint/test/build/docker across 5 services, orchestrated by a root Earthfile. GitHub Actions runs two workflows: PR checks (`earthly +ci`) and main-branch CI/CD (ci + matrix Docker image publish to GHCR). golangci-lint replaces `go vet` in all services.

**Tech Stack:** Earthly 0.8, GitHub Actions, golangci-lint v1.57.2, GHCR (ghcr.io), Docker

---

## File Structure

### New files
| File | Purpose |
|------|---------|
| `.golangci.yml` | Root linter config shared by all services |
| `services/auth/Earthfile` | Auth service Earthly build targets |
| `.github/workflows/pr.yml` | PR check workflow (earthly +ci) |
| `.github/workflows/main.yml` | Main CI/CD workflow (ci + build-and-push matrix) |
| `docs/src/ch09/index.md` | Chapter 9 index page |
| `docs/src/ch09/cicd-fundamentals.md` | Section 9.1 |
| `docs/src/ch09/earthly.md` | Section 9.2 |
| `docs/src/ch09/github-actions.md` | Section 9.3 |
| `docs/src/ch09/linting.md` | Section 9.4 |
| `docs/src/ch09/image-publishing.md` | Section 9.5 |

### Modified files
| File | Change |
|------|--------|
| `services/catalog/Earthfile` | Replace `go vet` with golangci-lint, copy `.golangci.yml` |
| `services/reservation/Earthfile` | Replace `go vet` with golangci-lint, copy `.golangci.yml` |
| `services/search/Earthfile` | Replace `go vet` with golangci-lint, copy `.golangci.yml` |
| `services/gateway/Earthfile` | Add templates/static to src+docker, copy `.golangci.yml` into lint |
| `Earthfile` (root) | Add `+docker` target |
| `docs/src/SUMMARY.md` | Add Chapter 9 entries |

### Deleted files
| File | Reason |
|------|--------|
| `.github/workflows/ci.yml` | Replaced by `pr.yml` and `main.yml` |

---

## Task 1: Create `.golangci.yml`

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create the linter config file**

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

- [ ] **Step 2: Verify the file is valid**

Run: `cat .golangci.yml`
Expected: YAML renders correctly, no syntax errors.

- [ ] **Step 3: Commit**

```bash
git add .golangci.yml
git commit -m "build: add golangci-lint configuration"
```

---

## Task 2: Create `services/auth/Earthfile`

**Files:**
- Create: `services/auth/Earthfile`

**Context:** Auth depends on `gen/` and `pkg/auth/` (via `replace` directives in `go.mod`). It does NOT depend on `pkg/otel`. The auth service has `cmd/`, `internal/` (handler, model, repository, service), and `migrations/` directories. Port is 50051. Follow the same pattern as `services/catalog/Earthfile`.

- [ ] **Step 1: Create the Earthfile**

```earthfile
VERSION 0.8

FROM golang:1.26-alpine

WORKDIR /app

deps:
    COPY go.mod go.sum* ./
    # Copy local module dependencies (replace directives)
    COPY ../../gen/go.mod ../../gen/go.sum* ../gen/
    COPY ../../pkg/auth/go.mod ../../pkg/auth/go.sum* ../pkg/auth/
    ENV GOWORK=off
    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

src:
    FROM +deps
    COPY --dir cmd internal migrations ./
    COPY ../../gen/ ../gen/
    COPY ../../pkg/auth/ ../pkg/auth/

lint:
    FROM +src
    COPY ../../.golangci.yml ./
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...

test:
    FROM +src
    RUN go test ./internal/service/... ./internal/handler/... -v -count=1

build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/auth ./cmd/
    SAVE ARTIFACT /bin/auth

docker:
    FROM alpine:3.19
    COPY +build/auth /usr/local/bin/auth
    EXPOSE 50051
    ENTRYPOINT ["/usr/local/bin/auth"]
    SAVE IMAGE auth:latest
```

- [ ] **Step 2: Verify the Earthfile parses**

Run: `cd services/auth && earthly ls`
Expected: Lists targets: `deps`, `src`, `lint`, `test`, `build`, `docker`

- [ ] **Step 3: Commit**

```bash
git add services/auth/Earthfile
git commit -m "build: add Earthfile for auth service"
```

---

## Task 3: Update `services/catalog/Earthfile` — golangci-lint

**Files:**
- Modify: `services/catalog/Earthfile` (the `lint` target, lines ~21-23)

**Context:** Currently the lint target is:
```
lint:
    FROM +src
    RUN go vet ./...
```
Replace with golangci-lint, copying the root `.golangci.yml` into the container.

- [ ] **Step 1: Update the lint target**

Replace the existing lint target with:
```earthfile
lint:
    FROM +src
    COPY ../../.golangci.yml ./
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...
```

- [ ] **Step 2: Commit**

```bash
git add services/catalog/Earthfile
git commit -m "build(catalog): replace go vet with golangci-lint"
```

---

## Task 4: Update `services/reservation/Earthfile` — golangci-lint

**Files:**
- Modify: `services/reservation/Earthfile` (the `lint` target, lines ~21-23)

**Context:** Same change as catalog — replace `go vet` with golangci-lint.

- [ ] **Step 1: Update the lint target**

Replace the existing lint target with:
```earthfile
lint:
    FROM +src
    COPY ../../.golangci.yml ./
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...
```

- [ ] **Step 2: Commit**

```bash
git add services/reservation/Earthfile
git commit -m "build(reservation): replace go vet with golangci-lint"
```

---

## Task 5: Update `services/search/Earthfile` — golangci-lint

**Files:**
- Modify: `services/search/Earthfile` (the `lint` target, lines ~17-19)

**Context:** Same change as catalog/reservation — replace `go vet` with golangci-lint. Search does NOT depend on `pkg/otel` (correct as-is, do not add it).

- [ ] **Step 1: Update the lint target**

Replace the existing lint target with:
```earthfile
lint:
    FROM +src
    COPY ../../.golangci.yml ./
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...
```

- [ ] **Step 2: Commit**

```bash
git add services/search/Earthfile
git commit -m "build(search): replace go vet with golangci-lint"
```

---

## Task 6: Update `services/gateway/Earthfile` — templates/static + golangci-lint config

**Files:**
- Modify: `services/gateway/Earthfile` (the `src`, `lint`, and `docker` targets)

**Context:** The gateway Earthfile already uses golangci-lint, but:
1. The `lint` target does NOT copy `.golangci.yml` — add the COPY
2. The `src` target is missing `templates/` and `static/` directories — add them
3. The `src` target needs `SAVE ARTIFACT` for templates and static so the `docker` target can reference them
4. The `docker` target must copy templates and static into the image, set `WORKDIR /app`

Current `src` target:
```
src:
    FROM +deps
    COPY --dir cmd internal ./
    COPY ../../gen/ ../gen/
    COPY ../../pkg/auth/ ../pkg/auth/
    COPY ../../pkg/otel/ ../pkg/otel/
```

Current `docker` target:
```
docker:
    FROM alpine:3.19
    COPY +build/gateway /usr/local/bin/gateway
    EXPOSE 8080
    ENTRYPOINT ["/usr/local/bin/gateway"]
    SAVE IMAGE gateway:latest
```

- [ ] **Step 1: Update the `src` target**

Replace the src target with:
```earthfile
src:
    FROM +deps
    COPY --dir cmd internal templates static ./
    COPY ../../gen/ ../gen/
    COPY ../../pkg/auth/ ../pkg/auth/
    COPY ../../pkg/otel/ ../pkg/otel/
    SAVE ARTIFACT ./templates
    SAVE ARTIFACT ./static
```

- [ ] **Step 2: Add `.golangci.yml` COPY to the `lint` target**

The gateway lint target already has `golangci-lint` installed. Add the config COPY before the `RUN golangci-lint` line:
```earthfile
lint:
    FROM +src
    COPY ../../.golangci.yml ./
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...
```

- [ ] **Step 3: Update the `docker` target**

Replace the docker target with:
```earthfile
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

- [ ] **Step 4: Commit**

```bash
git add services/gateway/Earthfile
git commit -m "build(gateway): add templates/static to docker, copy golangci config"
```

---

## Task 7: Update root `Earthfile` — add `+docker` target

**Files:**
- Modify: `Earthfile` (root)

**Context:** The root Earthfile currently has `ci`, `lint`, and `test` targets. Add a `docker` target that builds all 5 service images.

- [ ] **Step 1: Add the `+docker` target**

Append to the end of the root `Earthfile`:
```earthfile

docker:
    BUILD ./services/auth+docker
    BUILD ./services/catalog+docker
    BUILD ./services/gateway+docker
    BUILD ./services/reservation+docker
    BUILD ./services/search+docker
```

- [ ] **Step 2: Commit**

```bash
git add Earthfile
git commit -m "build: add root +docker target to build all service images"
```

---

## Task 8: Run `earthly +lint` and fix violations

**Files:**
- Potentially modify: any source files with lint violations

**Context:** With golangci-lint now configured across all services, run the linter and fix any violations it finds. This is the first time `errcheck`, `staticcheck`, `unused`, `gosimple`, and `ineffassign` are being run — expect some findings.

- [ ] **Step 1: Run linting across all services**

Run: `earthly +lint`
Expected: May fail with lint violations. Record all violations.

- [ ] **Step 2: Fix all lint violations**

Fix each violation in the source code. Common fixes:
- `errcheck`: add `if err != nil` checks or explicitly ignore with `_ =`
- `unused`: remove dead code
- `ineffassign`: remove or use the assigned variable
- `gosimple`: simplify the flagged code

- [ ] **Step 3: Re-run lint to verify all clean**

Run: `earthly +lint`
Expected: All 5 services pass.

- [ ] **Step 4: Run tests to ensure fixes didn't break anything**

Run: `earthly +test`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "fix: resolve golangci-lint violations across all services"
```

---

## Task 9: Run `earthly +docker` to verify all images build

**Files:** None (verification only)

- [ ] **Step 1: Build all Docker images via Earthly**

Run: `earthly +docker`
Expected: All 5 images build successfully: `auth:latest`, `catalog:latest`, `gateway:latest`, `reservation:latest`, `search:latest`

- [ ] **Step 2: Verify images exist**

Run: `docker images | grep -E "^(auth|catalog|gateway|reservation|search)"`
Expected: All 5 images listed.

---

## Task 10: Replace GitHub Actions workflows

**Files:**
- Delete: `.github/workflows/ci.yml`
- Create: `.github/workflows/pr.yml`
- Create: `.github/workflows/main.yml`

- [ ] **Step 1: Delete the old workflow**

```bash
rm .github/workflows/ci.yml
```

- [ ] **Step 2: Create `.github/workflows/pr.yml`**

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

- [ ] **Step 3: Create `.github/workflows/main.yml`**

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

- [ ] **Step 4: Commit**

```bash
git rm .github/workflows/ci.yml
git add .github/workflows/pr.yml .github/workflows/main.yml
git commit -m "ci: replace single workflow with PR check and main CI/CD workflows"
```

---

## Task 11: Update `docs/src/SUMMARY.md` with Chapter 9

**Files:**
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Add Chapter 9 entries**

Append after the Chapter 8 block (after the `- [8.5 Sidecar Collector Pattern]` line):
```markdown
- [Chapter 9: CI/CD with GitHub Actions & Earthly](./ch09/index.md)
  - [9.1 CI/CD Fundamentals](./ch09/cicd-fundamentals.md)
  - [9.2 The Earthly Build System](./ch09/earthly.md)
  - [9.3 GitHub Actions Workflows](./ch09/github-actions.md)
  - [9.4 Linting & Code Quality](./ch09/linting.md)
  - [9.5 Image Publishing & Versioning](./ch09/image-publishing.md)
```

- [ ] **Step 2: Create `docs/src/ch09/` directory**

```bash
mkdir -p docs/src/ch09
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/SUMMARY.md
git commit -m "docs: add Chapter 9 entries to SUMMARY.md"
```

---

## Task 12: Write `docs/src/ch09/index.md`

**Files:**
- Create: `docs/src/ch09/index.md`

**Context:** Chapter index page. Include:
- Chapter overview — what you'll learn
- Architecture diagram (the pipeline ASCII diagram from the spec)
- Prerequisites (Docker, Earthly CLI, GitHub repo)
- Chapter structure overview
- Follow the established pattern from previous chapter indexes (e.g., `docs/src/ch08/index.md`)

Read `docs/src/ch08/index.md` for the tone, structure, and format to follow.

- [ ] **Step 1: Write the index page**

The page should cover:
- What CI/CD means and why it matters
- The two-tool approach: Earthly for build logic, GitHub Actions for orchestration
- The pipeline architecture diagram showing PR → lint+test, main → lint+test+build+push
- What each section covers
- JVM comparison: Earthly ≈ Gradle, GitHub Actions ≈ Jenkins/TeamCity

Target length: ~100-150 lines.

- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`
Expected: No errors for ch09/index.md

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/index.md
git commit -m "docs: add Chapter 9 index page"
```

---

## Task 13: Write `docs/src/ch09/cicd-fundamentals.md`

**Files:**
- Create: `docs/src/ch09/cicd-fundamentals.md`

**Context:** Section 9.1 — theory only, no code changes. Cover:
- Continuous Integration: merge frequently, automated checks on every push
- Continuous Delivery: every green build produces a releasable artifact (Docker image)
- Continuous Deployment: auto-deploy (not covered here, deferred to Kubernetes chapter)
- The feedback loop: push → lint → test → build → publish
- Build reproducibility: why "works on my machine" fails, how containerized builds solve it
- Two-tool approach: Earthly for portable local builds, GitHub Actions for cloud orchestration
- JVM comparison: Earthly ≈ Gradle with Docker caching; GitHub Actions ≈ Jenkins/TeamCity

Read `docs/src/ch08/otel-fundamentals.md` for the writing style and structure to follow: theory sections, cross-language comparisons, exercises, and footnoted references.

Target length: ~150-200 lines.

- [ ] **Step 1: Write the fundamentals section**
- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/cicd-fundamentals.md
git commit -m "docs: write Section 9.1 CI/CD Fundamentals"
```

---

## Task 14: Write `docs/src/ch09/earthly.md`

**Files:**
- Create: `docs/src/ch09/earthly.md`

**Context:** Section 9.2 — theory + code walkthrough. Cover:
- What Earthly is: Dockerfile syntax + Makefile target structure. Each target runs in a container.
- Key concepts: `VERSION`, `FROM`, targets, `COPY`, `RUN`, `SAVE ARTIFACT`, `SAVE IMAGE`
- Walk through the catalog Earthfile target-by-target (deps, src, lint, test, build, docker) — explain layer caching
- The root Earthfile as orchestrator: `+ci` runs lint+test across all services in parallel
- `earthly +ci` locally = same as CI (the "local = cloud" principle)
- The auth Earthfile creation — explain the auth-specific dependency graph (gen + pkg/auth, no pkg/otel)
- golangci-lint integration: installing in lint targets, copying `.golangci.yml`
- The gateway fix: adding templates/static to src and docker, `SAVE ARTIFACT` pattern
- The `+docker` target: building all service images
- Earthly caching, `--push` flag, remote cache
- JVM comparison: Earthly ≈ Gradle with built-in Docker layer caching

Show actual code from the Earthfiles (the versions created in Tasks 2-7). Include the catalog Earthfile in full, then explain differences for other services.

Read `docs/src/ch08/instrumentation.md` for the pattern of showing code then explaining it.

Target length: ~300-400 lines.

- [ ] **Step 1: Write the Earthly section**
- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/earthly.md
git commit -m "docs: write Section 9.2 The Earthly Build System"
```

---

## Task 15: Write `docs/src/ch09/github-actions.md`

**Files:**
- Create: `docs/src/ch09/github-actions.md`

**Context:** Section 9.3 — theory + code walkthrough. Cover:
- GitHub Actions concepts: workflow files, triggers (`on.pull_request`, `on.push`), jobs, steps, runners
- Actions marketplace: `actions/checkout`, `docker/login-action`, `docker/build-push-action`
- Job dependencies with `needs`
- Permissions: `packages: write` for GHCR
- Matrix strategy for parallel builds
- GitHub context variables: `github.sha`, `github.ref`, `github.actor`
- Walk through `pr.yml` — explain each line
- Walk through `main.yml` — explain ci job, then build-and-push job with matrix
- Inline callout: pure GHA equivalent (without Earthly) using `actions/setup-go` + `golangci/golangci-lint-action`
- Inline callout: Earthly-push alternative using `earthly --push`
- Why we deleted `ci.yml` and split into two workflows (PR-gate vs main-publish separation)

Show the actual workflow YAML from Tasks 10. Use fenced code blocks with `yaml` highlighting.

Target length: ~300-400 lines.

- [ ] **Step 1: Write the GitHub Actions section**
- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/github-actions.md
git commit -m "docs: write Section 9.3 GitHub Actions Workflows"
```

---

## Task 16: Write `docs/src/ch09/linting.md`

**Files:**
- Create: `docs/src/ch09/linting.md`

**Context:** Section 9.4 — theory + explanation. Cover:
- Why `go vet` alone is insufficient (catches correctness but misses style, unused code, unchecked errors)
- What `golangci-lint` is: meta-linter that runs many linters in parallel, sharing AST parsing
- Walk through `.golangci.yml` — explain each enabled linter:
  - `govet` — correctness (printf format strings, struct tag validity, atomic operations)
  - `errcheck` — unchecked error returns (the most common Go bug)
  - `staticcheck` — broad static analysis suite (SA/S/ST checks)
  - `unused` — dead code detection
  - `gosimple` — simplifiable code
  - `ineffassign` — write-only variables
  - `typecheck` — compilation errors
- Running locally: `golangci-lint run ./...` from a service dir, or `earthly +lint` from anywhere
- Fixing violations: examples of common fixes (errcheck, unused, gosimple)
- JVM comparison: golangci-lint ≈ Checkstyle + SpotBugs + PMD + ErrorProne combined; `.golangci.yml` ≈ `checkstyle.xml`

Target length: ~200-300 lines.

- [ ] **Step 1: Write the linting section**
- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/linting.md
git commit -m "docs: write Section 9.4 Linting & Code Quality"
```

---

## Task 17: Write `docs/src/ch09/image-publishing.md`

**Files:**
- Create: `docs/src/ch09/image-publishing.md`

**Context:** Section 9.5 — theory, deep dive into the `build-and-push` job. Cover:
- Image tagging strategy:
  - `latest` — mutable, useful for dev, dangerous for production
  - `sha-<commit>` — immutable, traceable to exact code
  - Semantic versioning mentioned but deferred (requires release process)
- GHCR overview:
  - Images at `ghcr.io/<owner>/<repo>/<service>`
  - `GITHUB_TOKEN` authentication — no extra secrets
  - Package visibility defaults
- The `build-and-push` job walkthrough:
  - `docker/login-action` — GHCR auth with workflow token
  - `docker/build-push-action` — builds from Dockerfile, pushes with two tags
  - Matrix strategy — 5 parallel jobs, one per service
- Design decision: Dockerfiles for publishing, Earthly for CI:
  - `docker/build-push-action` integrates natively with GHA's OIDC, caching, provenance
  - Earthly `+docker` is for local building and verification
  - Earthly alternative: `earthly --push +docker`
  - When to use which (simple vs production)
- JVM comparison: equivalent to publishing JARs to Maven Central/Artifactory, but artifacts are container images

Target length: ~200-300 lines.

- [ ] **Step 1: Write the image publishing section**
- [ ] **Step 2: Verify it renders**

Run: `cd docs && mdbook build 2>&1 | tail -5`

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch09/image-publishing.md
git commit -m "docs: write Section 9.5 Image Publishing & Versioning"
```

---

## Task 18: Final verification

**Files:** None (verification only)

- [ ] **Step 1: Run full CI pipeline**

Run: `earthly +ci`
Expected: All lint and test targets pass across all 5 services.

- [ ] **Step 2: Build all Docker images**

Run: `earthly +docker`
Expected: All 5 images build successfully.

- [ ] **Step 3: Verify documentation renders**

Run: `cd docs && mdbook build 2>&1 | grep -i error`
Expected: No errors.

- [ ] **Step 4: Verify GitHub Actions YAML is valid**

Run: `python3 -c "import yaml; [yaml.safe_load(open(f)) for f in ['.github/workflows/pr.yml', '.github/workflows/main.yml']]" && echo "Valid YAML"`
Expected: "Valid YAML"
