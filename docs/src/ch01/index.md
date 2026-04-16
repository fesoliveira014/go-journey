# Chapter 1: Go Foundations

This chapter sets up the project, covers the Go essentials you'll need throughout the tutorial, builds a basic HTTP server, and introduces Go's testing conventions.

By the end of this chapter, you will have:

- A Go monorepo with workspace support
- A running HTTP server with two endpoints
- Tests using Go's standard `testing` package
- An Earthfile for reproducible builds and test runs
- A GitHub Actions CI pipeline

## Prerequisites

Before starting, install:

- **Go 1.22+**—[go.dev/dl](https://go.dev/dl/)
- **Earthly**—[earthly.dev/get-earthly](https://earthly.dev/get-earthly)
- **Git**—[git-scm.com](https://git-scm.com/)
- A code editor—VS Code with the official Go extension is recommended.

Verify your installations:

```bash
go version    # go1.22.x or later
earthly --version
git --version
```

## Sections

1. [Project Setup](./project-setup.md)—monorepo structure, Go modules, and workspaces
2. [Go Language Essentials](./go-basics.md)—types, structs, interfaces, error handling, slices, maps
3. [Building an HTTP Server](./http-server.md)—net/http, handlers, JSON responses, routing
4. [Testing in Go](./testing.md)—table-driven tests, httptest, test coverage, running with Earthly
