# Chapter 1: Go Foundations

> **Chapter checkpoint**
> Start from: `git checkout chapter-01-start`
> End state: `git checkout chapter-01-end`
>
> Chapter snippets are point-in-time snapshots. Later chapters intentionally change the same files.

This chapter sets up the project, covers the Go essentials you'll need throughout the tutorial, builds a basic HTTP server, and introduces Go's testing conventions.

## Assumed Knowledge

You already know programming fundamentals: functions, structs or records, interfaces, HTTP basics, and automated tests. This chapter focuses on how Go expresses those ideas and where Go's tooling makes different trade-offs.

## What You Build

By the end of this chapter, you will have:

- A Go monorepo with workspace support
- A running HTTP server with two endpoints
- Tests using Go's standard `testing` package

## Why This Chapter Exists

The rest of the book assumes you can move through a Go workspace, read a `go.mod`, write a small HTTP handler, and run tests. Chapter 1 gives you that shared baseline without turning into a beginner's Go book.

## Skim Path

If you already write Go professionally, skim Section 1.1 for the repository layout and workspace conventions, then read the exercises in Sections 1.3 and 1.4. If you are comfortable with another language but new to Go, read the chapter in order. The syntax details in Section 1.2 pay off immediately in Chapter 2.

## Prerequisites

Before starting, install:

- **Go 1.26+**—[go.dev/dl](https://go.dev/dl/)
- **Git**—[git-scm.com](https://git-scm.com/)
- A code editor—VS Code with the official Go extension is recommended.

Verify your installations:

```bash
go version    # go1.26.x or later
git --version
```

Earthly and GitHub Actions are introduced in Chapter 10. Section 1.4 includes a short optional preview so you can recognize the build target when you see it later.

## Sections

1. [Project Setup](./project-setup.md)—monorepo structure, Go modules, and workspaces
2. [Go Language Essentials](./go-basics.md)—types, structs, interfaces, error handling, slices, maps
3. [Building an HTTP Server](./http-server.md)—net/http, handlers, JSON responses, routing
4. [Testing in Go](./testing.md)—table-driven tests, httptest, test coverage, optional Earthly preview
