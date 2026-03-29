# Chapter 1: Go Foundations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up the monorepo scaffold, install tooling, build a minimal HTTP server in Go with tests, and produce the Chapter 1 tutorial content (Markdown + mdBook).

**Architecture:** Monorepo using Go workspaces (`go.work`). Chapter 1 builds a standalone HTTP server (not yet a microservice) to teach Go fundamentals. The server implements a basic in-memory book list endpoint as a preview of the domain. mdBook generates the static HTML site.

**Tech Stack:** Go 1.22+, Go workspaces, net/http (stdlib), encoding/json, testing (stdlib), mdBook, Earthly

**Spec reference:** `docs/superpowers/specs/2026-03-29-library-system-architecture-design.md`

**Scope note:** This plan covers Chapter 1 only. Chapters 2-12 each get their own plan after the previous chapter is complete.

---

## File Structure

```
library-system/
├── go.work
├── Earthfile                           # Root Earthfile (lint + test all)
├── .gitignore
├── README.md
├── services/
│   └── gateway/
│       ├── go.mod
│       ├── go.sum
│       ├── Earthfile                   # Service-level Earthfile
│       ├── cmd/
│       │   └── main.go                # Entry point — starts HTTP server
│       └── internal/
│           └── handler/
│               ├── health.go          # GET /healthz
│               ├── health_test.go
│               ├── books.go           # GET /books (in-memory list)
│               └── books_test.go
├── docs/
│   ├── book.toml                      # mdBook config
│   ├── src/
│   │   ├── SUMMARY.md                 # mdBook table of contents
│   │   └── ch01/
│   │       ├── index.md               # Chapter 1: Go Foundations
│   │       ├── project-setup.md       # 1.1: Project setup and tooling
│   │       ├── go-basics.md           # 1.2: Go language essentials
│   │       ├── http-server.md         # 1.3: Building an HTTP server
│   │       └── testing.md             # 1.4: Testing in Go
│   └── superpowers/
│       ├── specs/                     # (already exists)
│       └── plans/                     # (already exists)
└── .github/
    └── workflows/
        └── ci.yml                     # GitHub Actions — earthly +ci
```

---

## Task 1: Initialize Monorepo and Go Workspace

**Files:**
- Create: `library-system/go.work`
- Create: `library-system/services/gateway/go.mod`
- Create: `library-system/.gitignore`
- Create: `library-system/README.md`

- [ ] **Step 1: Create the project root directory**

```bash
mkdir -p library-system/services/gateway
cd library-system
```

- [ ] **Step 2: Initialize the Gateway Go module**

```bash
cd services/gateway
go mod init github.com/<user>/library-system/services/gateway
cd ../..
```

Replace `<user>` with your GitHub username.

- [ ] **Step 3: Create the Go workspace file**

Create `go.work` at the project root:

```go
go 1.22

use (
    ./services/gateway
)
```

- [ ] **Step 4: Verify the workspace resolves**

```bash
go work sync
```

Expected: no errors, `go.work.sum` may be created.

- [ ] **Step 5: Create .gitignore**

Create `.gitignore` at the project root:

```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/

# Test
*.test
*.out
coverage.html

# Go workspace
go.work.sum

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Superpowers
.superpowers/

# Environment
.env
.env.local
```

- [ ] **Step 6: Create README.md**

Create `README.md` at the project root:

```markdown
# Library Management System

A microservices-based library management system built in Go. This project serves as a hands-on tutorial covering microservices architecture, containerization, orchestration, observability, and CI/CD.

## Project Structure

- `services/` — Go microservices (gateway, auth, catalog, reservation, search)
- `proto/` — shared protobuf definitions
- `pkg/` — shared Go libraries
- `deploy/` — Docker Compose, Kubernetes manifests, Terraform
- `docs/` — tutorial content (viewable at GitHub Pages)

## Getting Started

See the [tutorial](docs/src/SUMMARY.md) for the step-by-step guide.
```

- [ ] **Step 7: Commit**

```bash
git add go.work services/gateway/go.mod .gitignore README.md
git commit -m "feat: initialize monorepo with Go workspace and gateway module"
```

---

## Task 2: Health Check Endpoint with TDD

**Files:**
- Create: `library-system/services/gateway/internal/handler/health.go`
- Create: `library-system/services/gateway/internal/handler/health_test.go`
- Create: `library-system/services/gateway/cmd/main.go`

- [ ] **Step 1: Create the directory structure**

```bash
mkdir -p services/gateway/internal/handler
mkdir -p services/gateway/cmd
```

- [ ] **Step 2: Write the failing test for the health handler**

Create `services/gateway/internal/handler/health_test.go`:

```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/<user>/library-system/services/gateway/internal/handler"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// json.Encode appends a trailing newline
	expected := "{\"status\":\"ok\"}\n"
	if body := rec.Body.String(); body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestHealthHandler_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
cd services/gateway && go test ./internal/handler/... -v
```

Expected: compilation error — `handler.Health` not defined.

- [ ] **Step 4: Write the minimal implementation**

Create `services/gateway/internal/handler/health.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
)

func Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(response)
}
```

- [ ] **Step 5: Run the test to verify it passes**

```bash
cd services/gateway && go test ./internal/handler/... -v
```

Expected: both tests PASS.

- [ ] **Step 6: Write the main.go entry point**

Create `services/gateway/cmd/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/<user>/library-system/services/gateway/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

- [ ] **Step 7: Run the server manually and verify**

```bash
cd services/gateway && go run ./cmd/
```

In another terminal:
```bash
curl -s http://localhost:8080/healthz | jq .
```

Expected: `{"status": "ok"}`

Kill the server with Ctrl+C.

- [ ] **Step 8: Commit**

```bash
git add services/gateway/
git commit -m "feat(gateway): add health check endpoint with tests"
```

---

## Task 3: Books List Endpoint with TDD

**Files:**
- Create: `library-system/services/gateway/internal/handler/books.go`
- Create: `library-system/services/gateway/internal/handler/books_test.go`
- Modify: `library-system/services/gateway/cmd/main.go`

- [ ] **Step 1: Write the failing test for the books handler**

Create `services/gateway/internal/handler/books_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/<user>/library-system/services/gateway/internal/handler"
)

func TestBooksHandler_ReturnsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	rec := httptest.NewRecorder()

	handler.Books(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
	}

	var books []handler.Book
	if err := json.NewDecoder(rec.Body).Decode(&books); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(books) == 0 {
		t.Error("expected at least one book in the list")
	}

	first := books[0]
	if first.Title == "" {
		t.Error("expected book to have a title")
	}
	if first.Author == "" {
		t.Error("expected book to have an author")
	}
}

func TestBooksHandler_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/books", nil)
	rec := httptest.NewRecorder()

	handler.Books(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd services/gateway && go test ./internal/handler/... -v
```

Expected: compilation error — `handler.Books` and `handler.Book` not defined.

- [ ] **Step 3: Write the minimal implementation**

Create `services/gateway/internal/handler/books.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
)

// Book represents a book in the library catalog.
// In later chapters, this will be replaced by the Catalog service's protobuf type.
type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Genre  string `json:"genre"`
	Year   int    `json:"year"`
}

// sampleBooks is a hardcoded list for Chapter 1. It will be replaced
// by gRPC calls to the Catalog service in Chapter 5.
var sampleBooks = []Book{
	{ID: "1", Title: "The Go Programming Language", Author: "Alan Donovan & Brian Kernighan", Genre: "Programming", Year: 2015},
	{ID: "2", Title: "Designing Data-Intensive Applications", Author: "Martin Kleppmann", Genre: "Distributed Systems", Year: 2017},
	{ID: "3", Title: "Building Microservices", Author: "Sam Newman", Genre: "Architecture", Year: 2021},
}

func Books(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sampleBooks)
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd services/gateway && go test ./internal/handler/... -v
```

Expected: all 4 tests PASS (2 health + 2 books).

- [ ] **Step 5: Register the route in main.go**

Modify `services/gateway/cmd/main.go` — add this line after the health route:

```go
mux.HandleFunc("/books", handler.Books)
```

- [ ] **Step 6: Run the server and verify**

```bash
cd services/gateway && go run ./cmd/
```

In another terminal:
```bash
curl -s http://localhost:8080/books | jq .
```

Expected: JSON array with 3 books.

- [ ] **Step 7: Commit**

```bash
git add services/gateway/
git commit -m "feat(gateway): add books list endpoint with in-memory data"
```

---

## Task 4: Root Earthfile for Lint and Test

**Files:**
- Create: `library-system/Earthfile`
- Create: `library-system/services/gateway/Earthfile`

- [ ] **Step 1: Create the service-level Earthfile**

Create `services/gateway/Earthfile`:

```earthly
VERSION 0.8

FROM golang:1.22-alpine

WORKDIR /app

deps:
    COPY go.mod go.sum* ./
    RUN go mod download

src:
    FROM +deps
    COPY --dir cmd internal ./

lint:
    FROM +src
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.2
    RUN golangci-lint run ./...

test:
    FROM +src
    RUN go test -v -race -cover ./...

build:
    FROM +src
    RUN CGO_ENABLED=0 go build -o /bin/gateway ./cmd/
    SAVE ARTIFACT /bin/gateway

docker:
    FROM alpine:3.19
    COPY +build/gateway /usr/local/bin/gateway
    EXPOSE 8080
    ENTRYPOINT ["/usr/local/bin/gateway"]
    SAVE IMAGE gateway:latest
```

- [ ] **Step 2: Create the root Earthfile**

Create `Earthfile` at the project root:

```earthly
VERSION 0.8

ci:
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test

lint:
    BUILD ./services/gateway+lint

test:
    BUILD ./services/gateway+test
```

- [ ] **Step 3: Verify Earthly lint works**

```bash
earthly +lint
```

Expected: passes with no lint errors. If `golangci-lint` flags issues, fix them before proceeding.

- [ ] **Step 4: Verify Earthly test works**

```bash
earthly +test
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add Earthfile services/gateway/Earthfile
git commit -m "feat: add Earthfiles for lint and test targets"
```

---

## Task 5: GitHub Actions CI Workflow

**Files:**
- Create: `library-system/.github/workflows/ci.yml`

- [ ] **Step 1: Create the CI workflow**

```bash
mkdir -p .github/workflows
```

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
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

- [ ] **Step 2: Commit**

```bash
git add .github/
git commit -m "feat: add GitHub Actions CI workflow with Earthly"
```

---

## Task 6: mdBook Setup and Chapter 1 Structure

**Files:**
- Create: `library-system/docs/book.toml`
- Create: `library-system/docs/src/SUMMARY.md`
- Create: `library-system/docs/src/ch01/index.md`

- [ ] **Step 1: Install mdBook** (if not already installed)

```bash
# On Linux
curl -sSL https://github.com/rust-lang/mdBook/releases/download/v0.4.37/mdbook-v0.4.37-x86_64-unknown-linux-gnu.tar.gz | tar -xz -C /usr/local/bin

# Or via cargo
cargo install mdbook
```

Verify: `mdbook --version`

- [ ] **Step 2: Create mdBook configuration**

Create `docs/book.toml`:

```toml
[book]
title = "Building Microservices in Go"
authors = ["Your Name"]
language = "en"
multilingual = false
src = "src"

[build]
build-dir = "../site"

[output.html]
default-theme = "navy"
preferred-dark-theme = "navy"
git-repository-url = "https://github.com/<user>/library-system"
```

- [ ] **Step 3: Create the table of contents**

Create `docs/src/SUMMARY.md`:

```markdown
# Summary

- [Introduction](./introduction.md)
- [Chapter 1: Go Foundations](./ch01/index.md)
    - [1.1 Project Setup](./ch01/project-setup.md)
    - [1.2 Go Language Essentials](./ch01/go-basics.md)
    - [1.3 Building an HTTP Server](./ch01/http-server.md)
    - [1.4 Testing in Go](./ch01/testing.md)
```

- [ ] **Step 4: Create the introduction page**

Create `docs/src/introduction.md`:

```markdown
# Building Microservices in Go

Welcome to this hands-on guide to building a complete microservices application in Go. By the end of this tutorial, you will have built a library management system covering:

- **Go** — the language, project structure, testing, and idioms
- **Microservices** — service decomposition, gRPC, event-driven architecture with Kafka
- **Databases** — PostgreSQL with migrations and the repository pattern
- **Containers** — Docker, multi-stage builds, Docker Compose
- **Orchestration** — Kubernetes (kind locally, EKS in production)
- **Infrastructure as Code** — Terraform for AWS (VPC, EKS, RDS)
- **Observability** — OpenTelemetry, Jaeger, Prometheus, Grafana, Loki
- **CI/CD** — GitHub Actions and Earthly
- **Authentication** — JWT, bcrypt, OAuth2 with Gmail

## Who This Is For

You are an experienced software engineer who knows how to program but is new to Go and/or cloud-native tooling. The guide assumes strong programming fundamentals but explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch.

## The Project

We are building a **library management system** where:

- Admins manage the book catalog (CRUD operations)
- Users browse, search, reserve, and return books
- Authentication supports email/password and Google OAuth2

The system is decomposed into 5 microservices: Gateway, Auth, Catalog, Reservation, and Search.

## How to Use This Guide

Each chapter builds on the previous one. Follow them in order. Every chapter includes:

- **Theory** — why we are making each decision
- **Implementation** — complete, runnable code
- **Exercises** — practice problems to test your understanding
- **References** — links to official docs and further reading
```

- [ ] **Step 5: Create the Chapter 1 index page**

Create `docs/src/ch01/index.md`:

```markdown
# Chapter 1: Go Foundations

In this chapter, you will set up the project, learn the Go essentials you need for the rest of the tutorial, build a basic HTTP server, and write your first tests.

By the end of this chapter, you will have:

- A Go monorepo with workspace support
- A running HTTP server with two endpoints
- Tests using Go's standard `testing` package
- An Earthfile for reproducible builds and test runs
- A GitHub Actions CI pipeline

## Prerequisites

Before starting, install:

- **Go 1.22+** — [go.dev/dl](https://go.dev/dl/)
- **Earthly** — [earthly.dev/get-earthly](https://earthly.dev/get-earthly)
- **Git** — [git-scm.com](https://git-scm.com/)
- A code editor (VS Code with the Go extension is recommended)

Verify your installations:

\```bash
go version    # go1.22.x or later
earthly --version
git --version
\```

## Sections

1. [Project Setup](./project-setup.md) — monorepo structure, Go modules, and workspaces
2. [Go Language Essentials](./go-basics.md) — types, structs, interfaces, error handling, slices, maps
3. [Building an HTTP Server](./http-server.md) — net/http, handlers, JSON responses, routing
4. [Testing in Go](./testing.md) — table-driven tests, httptest, test coverage, running with Earthly
```

- [ ] **Step 6: Verify mdBook builds**

```bash
cd docs && mdbook build
```

Expected: builds to `site/` directory with no errors. Open `site/index.html` in a browser to verify.

- [ ] **Step 7: Add site/ to .gitignore**

Append to `.gitignore`:

```gitignore
# mdBook output
/site/
```

- [ ] **Step 8: Commit**

```bash
git add docs/book.toml docs/src/ .gitignore
git commit -m "feat(docs): set up mdBook with Chapter 1 structure"
```

---

## Task 7: Write Chapter 1 Tutorial Content — Project Setup (1.1)

**Files:**
- Create: `library-system/docs/src/ch01/project-setup.md`

- [ ] **Step 1: Write section 1.1**

Create `docs/src/ch01/project-setup.md`. This section covers:

- **What is a Go module** — `go.mod`, module paths, semantic versioning
- **Go workspaces** — `go.work`, why monorepos use them, how `use` directives work
- **Project structure conventions** — `cmd/`, `internal/`, why Go doesn't have `src/`
- **Walkthrough** — step-by-step recreation of what Task 1 built, with explanations of each decision
- **Exercise:** Create a second module (`services/auth/`) with an empty `main.go`, add it to `go.work`, verify `go work sync` works

Include footnoted references:
1. [Go Modules Reference](https://go.dev/ref/mod)
2. [Go Workspaces](https://go.dev/doc/tutorial/workspaces)
3. [Standard Go Project Layout discussion](https://go.dev/doc/modules/layout)

Content length target: 800-1200 words. Include code blocks that mirror the actual project code.

- [ ] **Step 2: Verify mdBook builds with the new content**

```bash
cd docs && mdbook build
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch01/project-setup.md
git commit -m "docs(ch01): write section 1.1 — project setup and Go workspaces"
```

---

## Task 8: Write Chapter 1 Tutorial Content — Go Basics (1.2)

**Files:**
- Create: `library-system/docs/src/ch01/go-basics.md`

- [ ] **Step 1: Write section 1.2**

Create `docs/src/ch01/go-basics.md`. This section covers:

- **Types and zero values** — int, string, bool, and their defaults. Coming from C/C++: no implicit conversions, no pointer arithmetic.
- **Structs** — defining, initializing, embedding. Compare to Java/Kotlin classes: no inheritance, composition via embedding.
- **Interfaces** — implicit satisfaction (no `implements` keyword). Compare to C++ pure virtual / Java interfaces. The empty interface (`any`). Why this matters for testing.
- **Slices and maps** — creation, append, iteration, nil vs empty. Compare to C++ vectors/maps, Java ArrayList/HashMap.
- **Error handling** — `error` interface, returning errors, `fmt.Errorf` with `%w` for wrapping, `errors.Is`/`errors.As`. Contrast with exceptions (Java/Kotlin) and error codes (C).
- **Pointers** — basics only (coming from C/C++, this should be quick). No pointer arithmetic. When to use pointers vs values.
- **Exercise:** Write a `Book` struct with a `String()` method (implementing `fmt.Stringer`), create a slice of books, write a function that filters books by genre, handle the case where no books match by returning an error.

Include footnoted references:
1. [A Tour of Go](https://go.dev/tour/)
2. [Effective Go](https://go.dev/doc/effective_go)
3. [Go Blog: Error handling](https://go.dev/blog/go1.13-errors)

Content length target: 1500-2000 words. Heavy on code examples, light on prose.

- [ ] **Step 2: Verify mdBook builds**

```bash
cd docs && mdbook build
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch01/go-basics.md
git commit -m "docs(ch01): write section 1.2 — Go language essentials"
```

---

## Task 9: Write Chapter 1 Tutorial Content — HTTP Server (1.3)

**Files:**
- Create: `library-system/docs/src/ch01/http-server.md`

- [ ] **Step 1: Write section 1.3**

Create `docs/src/ch01/http-server.md`. This section covers:

- **net/http basics** — `http.Handler` interface, `http.HandlerFunc`, `http.ServeMux`
- **Writing handlers** — request/response model, reading method/path/headers, writing status codes and bodies
- **JSON responses** — `encoding/json`, `json.NewEncoder`, struct tags (`json:"name"`)
- **Walkthrough** — build the health and books handlers from Tasks 2-3, explaining each line
- **Environment configuration** — reading `PORT` from env with a default (pattern used throughout)
- **Exercise:** Add a `GET /books/{id}` endpoint that returns a single book by ID from the in-memory list, or 404 if not found. (Hint: use `r.PathValue("id")` with Go 1.22's enhanced routing.)

Include footnoted references:
1. [net/http package docs](https://pkg.go.dev/net/http)
2. [Go 1.22 enhanced routing](https://go.dev/blog/routing-enhancements)
3. [encoding/json package docs](https://pkg.go.dev/encoding/json)

Content length target: 1000-1500 words.

- [ ] **Step 2: Verify mdBook builds**

```bash
cd docs && mdbook build
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch01/http-server.md
git commit -m "docs(ch01): write section 1.3 — building an HTTP server"
```

---

## Task 10: Write Chapter 1 Tutorial Content — Testing (1.4)

**Files:**
- Create: `library-system/docs/src/ch01/testing.md`

- [ ] **Step 1: Write section 1.4**

Create `docs/src/ch01/testing.md`. This section covers:

- **Go testing basics** — `_test.go` convention, `testing.T`, `go test`, `-v`, `-race`, `-cover`
- **httptest package** — `httptest.NewRequest`, `httptest.NewRecorder`, testing handlers without a running server
- **Table-driven tests** — the Go idiom for parameterized tests. Rewrite the health and books tests as table-driven.
- **Test coverage** — `go test -coverprofile`, viewing in browser with `go tool cover -html`
- **Testing with Earthly** — why reproducible test environments matter, running `earthly +test`
- **Exercise:** Write table-driven tests for the `GET /books/{id}` endpoint from the previous exercise. Test cases: valid ID returns 200 + book, invalid ID returns 404, non-GET method returns 405.

Include footnoted references:
1. [Go testing package](https://pkg.go.dev/testing)
2. [Go Blog: Table-driven tests](https://go.dev/wiki/TableDrivenTests)
3. [httptest package](https://pkg.go.dev/net/http/httptest)
4. [Earthly docs](https://docs.earthly.dev/)

Content length target: 1000-1500 words.

- [ ] **Step 2: Verify the full mdBook builds cleanly**

```bash
cd docs && mdbook build
```

Open `site/index.html` and navigate through all Chapter 1 sections to verify links and rendering.

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch01/testing.md
git commit -m "docs(ch01): write section 1.4 — testing in Go"
```

---

## Task 11: Final Verification and Chapter 1 Complete Commit

**Files:**
- No new files — verification only

- [ ] **Step 1: Run all tests**

```bash
cd services/gateway && go test -v -race -cover ./...
```

Expected: all tests pass, coverage reported.

- [ ] **Step 2: Run Earthly CI**

```bash
earthly +ci
```

Expected: lint passes, all tests pass.

- [ ] **Step 3: Build and verify mdBook**

```bash
cd docs && mdbook build
```

Open `site/index.html`, verify:
- Sidebar shows all Chapter 1 sections
- All code blocks render with syntax highlighting
- Exercise sections are present
- Footnote references link correctly

- [ ] **Step 4: Run the server one final time**

```bash
cd services/gateway && go run ./cmd/
```

Verify in another terminal:
```bash
curl -s http://localhost:8080/healthz | jq .
curl -s http://localhost:8080/books | jq .
```

- [ ] **Step 5: Commit if any fixes were needed**

```bash
git add -A
git commit -m "fix(ch01): address issues found during final verification"
```

Only commit if changes were made. Skip if everything was clean.
