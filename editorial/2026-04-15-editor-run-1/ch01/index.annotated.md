<!-- [STRUCTURAL] Chapter landing page is adequate: it states what the reader will build, prerequisites, and sections. However, it skips a "why this order?" bridge connecting the four sections (setup → language → HTTP → testing). A single sentence of narrative would help the reader see the arc: we scaffold, learn the minimum language, build a thing, then verify it. Consider also noting the target branch/tag of the repo the snippets correspond to, since introduction.md warns that later chapters modify these files. -->
# Chapter 1: Go Foundations

<!-- [STRUCTURAL] Opening sentence is a bare enumeration. Readers skim chapter openers; a one-sentence narrative frame ("This chapter is the runway — you will not write business logic yet, but by the end you will have a repeatable build, a live HTTP endpoint, and tests running in CI") would orient the reader better. Low-cost addition. -->
<!-- [LINE EDIT] "In this chapter, you will set up the project, learn the Go essentials you need for the rest of the tutorial, build a basic HTTP server, and write your first tests." — consider tightening: "This chapter sets up the project, covers the Go essentials you'll need throughout the tutorial, builds a basic HTTP server, and introduces Go's testing conventions." Active framing, parallel verbs. -->
In this chapter, you will set up the project, learn the Go essentials you need for the rest of the tutorial, build a basic HTTP server, and write your first tests.

<!-- [LINE EDIT] "By the end of this chapter, you will have:" — fine, but the list items below mix deliverables of different granularity (a monorepo vs. a CI pipeline). Consider grouping: "Repository & build: ...", "Running code: ...". Judgment call; current list is fine. -->
By the end of this chapter, you will have:

<!-- [COPY EDIT] "Go's standard `testing` package" — "Go's" possessive is correct; ensure consistent throughout chapter. -->
- A Go monorepo with workspace support
- A running HTTP server with two endpoints
- Tests using Go's standard `testing` package
- An Earthfile for reproducible builds and test runs
- A GitHub Actions CI pipeline

## Prerequisites

Before starting, install:

<!-- [COPY EDIT] Please verify: "Go 1.26+" — Go 1.26 released on schedule? As of the book's stated date (2026-04-15) Go 1.26.1 is plausible, but confirm against go.dev/doc/devel/release. (Same version appears in project-setup.md `go 1.26.1`.) -->
- **Go 1.26+** — [go.dev/dl](https://go.dev/dl/)
<!-- [COPY EDIT] Please verify: Earthly download URL — canonical landing is https://earthly.dev/get-earthly; confirm it is still live and not redirected to `earthly.dev/download`. -->
- **Earthly** — [earthly.dev/get-earthly](https://earthly.dev/get-earthly)
- **Git** — [git-scm.com](https://git-scm.com/)
<!-- [LINE EDIT] "A code editor (VS Code with the Go extension is recommended)" → "A code editor; VS Code with the official Go extension is recommended." — tightens parenthetical into a stronger recommendation. -->
<!-- [COPY EDIT] "VS Code" is correct (CMOS: respect product capitalization); the "Go extension" is officially "Go for Visual Studio Code" — consider linking to marketplace. -->
- A code editor (VS Code with the Go extension is recommended)

Verify your installations:

```bash
go version    # go1.26.x or later
earthly --version
git --version
```

## Sections

<!-- [STRUCTURAL] The section list at the bottom duplicates what the opening paragraph said. Consider dropping the opening list items ("A Go monorepo...") or these section bullets — pick one. Right now the reader reads the same arc twice in 30 lines. -->
<!-- [COPY EDIT] Sections list items use em-dash with spaces; verify style consistency with CMOS 6.85 (em dash, no spaces). Current "— monorepo structure" uses spaces around em dash, which is a common house style but not strict CMOS. Pick one and apply consistently across the chapter — the rest of the chapter also mixes spaced em dashes. Flagging once here; recurring pattern in all section files. -->
1. [Project Setup](./project-setup.md) — monorepo structure, Go modules, and workspaces
<!-- [COPY EDIT] "types, structs, interfaces, error handling, slices, maps" — serial comma present after "error handling"? Yes: "slices, maps" has no Oxford comma because it is the final pair — correct per CMOS 6.19. Leave as-is. -->
2. [Go Language Essentials](./go-basics.md) — types, structs, interfaces, error handling, slices, maps
<!-- [COPY EDIT] "net/http, handlers, JSON responses, routing" — CMOS 6.19 serial comma; list of 4 needs comma before "and"? There is no "and" here (asyndetic list). Acceptable but inconsistent with item 1 which uses "and". Make parallel: all items should or shouldn't include the conjunction. -->
3. [Building an HTTP Server](./http-server.md) — net/http, handlers, JSON responses, routing
<!-- [COPY EDIT] Same parallelism issue: "table-driven tests, httptest, test coverage, running with Earthly" — no conjunction, inconsistent with item 1. Suggest: "table-driven tests, httptest, test coverage, and running with Earthly." -->
4. [Testing in Go](./testing.md) — table-driven tests, httptest, test coverage, running with Earthly
