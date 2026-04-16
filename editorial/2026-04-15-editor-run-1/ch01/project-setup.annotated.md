# 1.1 — Project Setup

<!-- [STRUCTURAL] Strong opening frame — the "how does the toolchain find, version, and build your code?" question is well posed and motivates both files. Good tutor voice. -->
<!-- [LINE EDIT] Consider ending the opening paragraph with a single sentence previewing the section arc: "We cover modules, then workspaces, then the conventional project layout, and close with a walkthrough." Saves the reader from scrolling to orient. Judgment call. -->
Every Go project starts by answering one question: *how does the toolchain find, version, and build your code?* The answer lives in two files — `go.mod` for each service, and `go.work` at the monorepo root. This section explains both, then reconstructs the choices made when setting up this project.

---

## Go Modules

<!-- [STRUCTURAL] The JVM analogy lands well for the target reader. However, the paragraph compresses three ideas (module name, min Go version, dependencies) into one sentence before the subsection breaks them out. Consider introducing only the concept here and letting "The `go.mod` File" do the enumeration. -->
<!-- [LINE EDIT] "If you are coming from the JVM world, a Go module is roughly the equivalent of a Maven `pom.xml` or a Gradle `build.gradle`" — tighten: "If you come from the JVM world, a Go module is roughly the equivalent of a Maven `pom.xml` or Gradle `build.gradle` file." Present tense is less tentative; "a Maven … or a Gradle …" is mildly redundant. -->
<!-- [COPY EDIT] CMOS 7.81 — compound adjective "JVM world" is fine unhyphenated here (noun phrase, not attributive compound). No change. -->
If you are coming from the JVM world, a Go module is roughly the equivalent of a Maven `pom.xml` or a Gradle `build.gradle` — it declares the name of the project, the minimum Go version required, and all external dependencies. Unlike Maven or Gradle, Go bakes module support directly into the toolchain (`go mod`) with no plugin layer.

A module is a directory tree whose root contains a `go.mod` file. Every `.go` source file in that tree belongs to exactly one module.

### The `go.mod` File

<!-- [LINE EDIT] "The gateway service's module file looks like this:" — fine. Could be slightly tightened to "Here is the gateway service's module file:" but both work. -->
The gateway service's module file looks like this:

<!-- [COPY EDIT] The fenced block has no language hint. Add ```go-module``` or leave unfenced-language; for consistency with rest of chapter consider a comment. Many Markdown renderers will not highlight go.mod anyway, so plain ``` is fine; flagging only for consistency. -->
```
module github.com/fesoliveira014/library-system/services/gateway

go 1.26.1
```

Three things worth unpacking:

<!-- [LINE EDIT] "**`module` — the module path.** This is the canonical import path prefix for every package inside this module." — reads well. -->
<!-- [COPY EDIT] "every package inside this module" — "this" has no near referent after the heading; "the module" is clearer. Minor. -->
**`module` — the module path.** This is the canonical import path prefix for every package inside this module. Any file anywhere in the tree can import a package from this module using that prefix. The convention is to use the repository URL as the module path — for example, `github.com/<user>/<repo>/<optional-subdirectory>`. This is not just convention for readability; it is how `go get` and the Go proxy infrastructure locate and download modules.

<!-- [LINE EDIT] "Contrast this with Java packages, where the reverse-domain convention (`com.example.myapp`) is separate from the artifact coordinates in `pom.xml`. In Go, they are the same string." — good sentence. Keep. -->
Contrast this with Java packages, where the reverse-domain convention (`com.example.myapp`) is separate from the artifact coordinates in `pom.xml`. In Go, they are the same string.

**`go` — the minimum language version.** This pins language semantics. The toolchain refuses to build if the installed Go version is older than what is declared here.

<!-- [COPY EDIT] Please verify: the `go` directive in go.mod semantics changed in Go 1.21 — it is now the **minimum required** Go toolchain version, not merely a hint. The phrasing "pins language semantics" is accurate but may merit a pointer to `toolchain` directive (Go 1.21+). Flag for author accuracy. -->
<!-- [LINE EDIT] "**Dependencies** appear as `require` directives added automatically by `go get` or `go mod tidy`" — the "added automatically" is passive and noun-first. Consider: "`go get` and `go mod tidy` add **dependencies** as `require` directives, following [Semantic Versioning](https://semver.org/)." Reads more direct. Judgment call — current phrasing is fine. -->
**Dependencies** appear as `require` directives added automatically by `go get` or `go mod tidy`, following [Semantic Versioning](https://semver.org/). One Go-specific rule worth knowing early: if a module releases a v2 or higher, the major version must appear in the module path itself (`github.com/foo/bar/v2`). This lets two incompatible major versions coexist in a single binary without conflict — something that has historically been painful in Maven/Gradle projects.

### Creating a Module

```bash
mkdir -p services/gateway
cd services/gateway
go mod init github.com/fesoliveira014/library-system/services/gateway
```

<!-- [LINE EDIT] "That single command writes `go.mod`. Nothing else is required to start writing Go code in that directory." — two sentences that could be one: "That single command writes `go.mod`; nothing else is required to start writing Go." -->
That single command writes `go.mod`. Nothing else is required to start writing Go code in that directory.

---

## Go Workspaces

<!-- [STRUCTURAL] Good motivation — you state the monorepo problem before introducing the solution. This is the right order. -->
<!-- [LINE EDIT] "how does the toolchain resolve an import from a sibling service during local development, without publishing it to a registry first?" — the comma before "without" is unnecessary (restrictive clause). Consider: "how does the toolchain resolve an import from a sibling service during local development without first publishing it to a registry?" -->
This project is a monorepo: multiple independently deployable services live inside a single repository, each with its own `go.mod`. That raises a problem — how does the toolchain resolve an import from a sibling service during local development, without publishing it to a registry first?

<!-- [COPY EDIT] "introduced in Go 1.18" — factual, correct (workspaces landed in Go 1.18, March 2022). No change. -->
<!-- [LINE EDIT] "The workspace is a development-time overlay." — crisp, keep. -->
The answer is `go.work`, introduced in Go 1.18. A workspace file at the repository root tells the toolchain to resolve specific module paths to local directories instead of fetching them from the network. It does not replace `go.mod` — each module retains its own dependency graph and builds independently. The workspace is a development-time overlay.

```
go 1.26.1

use ./services/gateway
```

<!-- [LINE EDIT] "When any code in the workspace imports a package whose path matches a registered module, the toolchain resolves it locally." — 24 words; fine. -->
<!-- [COPY EDIT] "This is analogous to Gradle's multi-project build, with one key advantage: each `go.mod` remains self-contained." — CMOS 6.63: after a colon, capitalize only if what follows is a complete sentence. "each `go.mod` remains self-contained" is a complete sentence, so capitalize: "Each `go.mod` remains self-contained." Same applies if the author prefers lowercase (some house styles keep lowercase after colon even for full sentences). Pick one and apply consistently. -->
The `use` directive registers a local module. When any code in the workspace imports a package whose path matches a registered module, the toolchain resolves it locally. This is analogous to Gradle's multi-project build, with one key advantage: each `go.mod` remains self-contained. Running `go build ./...` from inside `services/gateway` works exactly as if the workspace did not exist.

### Workspace Commands

```bash
# Initialize a workspace at the repo root
go work init

# Register an existing module
go work use ./services/gateway

# After adding or removing modules, resync workspace metadata
go work sync
```

<!-- [COPY EDIT] Please verify: "`go work sync` downloads any dependencies that are listed in a module's `go.mod` but not yet reflected in the workspace's lock state." — Per Go docs, `go work sync` syncs the workspace's build list back to each module's `go.mod` (pushes versions outward), not the reverse. The direction may be inverted in this description. Author should double-check: https://go.dev/ref/mod#go-work-sync -->
<!-- [LINE EDIT] "It is safe to run at any time and idempotent." → "It is safe to run at any time; the command is idempotent." or drop "and idempotent" as redundant given "safe to run at any time". -->
`go work sync` downloads any dependencies that are listed in a module's `go.mod` but not yet reflected in the workspace's lock state. It is safe to run at any time and idempotent.

---

## Project Structure Conventions

<!-- [STRUCTURAL] The `cmd/` → `internal/` → full layout flow is clean. Consider adding a one-line note on where `pkg/` fits (or explicitly: "we do not use `pkg/` in this project and here's why") — experienced readers will expect it and wonder at its absence. -->
Go has no mandated project layout, but the community has converged on conventions that most open-source projects follow. There is no `src/` directory — that is a Java/Maven artifact Go does not need.

### `cmd/`

<!-- [LINE EDIT] "The `cmd/` directory holds the entry points for executables." — could be: "The `cmd/` directory holds executable entry points." -->
The `cmd/` directory holds the entry points for executables. Each subdirectory under `cmd/` corresponds to one binary. If a module produces a single binary, it is common to place `main.go` directly in `cmd/` without a further subdirectory. The gateway service does exactly this:

```
services/gateway/
└── cmd/
    └── main.go
```

### `internal/`

<!-- [LINE EDIT] "The `internal/` directory enforces package visibility at the toolchain level." — direct and good. -->
<!-- [COPY EDIT] "Java's package-private" — correct capitalization; hyphenate "package-private" as a compound modifier. Currently hyphenated; good. -->
The `internal/` directory enforces package visibility at the toolchain level. Any package path containing `/internal/` can only be imported by code rooted at the parent of `internal/`. `services/gateway/internal/handler` is importable by `services/gateway/cmd/main.go` but by nothing outside the gateway module — not even another service in the same repository. This is a stronger boundary than Java's package-private: it is enforced by the compiler across module boundaries.

### The Full Gateway Layout

<!-- [COPY EDIT] The arrows in the ASCII tree use "←" (U+2190). Not all terminal fonts render this consistently; acceptable in Markdown/HTML output. No change. -->
```
services/gateway/
├── go.mod
├── cmd/
│   └── main.go          ← package main, the binary entry point
└── internal/
    └── handler/
        ├── health.go    ← GET /healthz
        ├── health_test.go
        ├── books.go     ← GET /books
        └── books_test.go
```

---

## Walkthrough: Recreating the Setup

<!-- [STRUCTURAL] Good — the walkthrough ties theory to practice. The sequencing is correct: workspace init, then module init, then use. -->
Here is the exact sequence of commands that produced the current project state.

```bash
# 1. Initialize the workspace at the repository root
go work init

# 2. Create and initialize the gateway module
mkdir -p services/gateway
cd services/gateway
go mod init github.com/fesoliveira014/library-system/services/gateway

# 3. Register the module with the workspace (run from the repo root)
go work use ./services/gateway

# 4. Create the directory structure
mkdir -p services/gateway/cmd
mkdir -p services/gateway/internal/handler
```

<!-- [STRUCTURAL] The main.go snippet appears here before the reader has been introduced to handlers, ServeMux, or ListenAndServe. This overlaps with section 1.3 (HTTP Server) which walks through the same code in detail. Either (a) shorten this snippet to just show the package/import structure, noting "we'll unpack this in section 1.3", or (b) cross-reference explicitly. As written, it's a minor redundancy and a jarring jump in abstraction (the reader is still in layout mode). -->
With the scaffolding in place, the entry point imports the handler package using its full module-qualified path:

```go
// services/gateway/cmd/main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/books", handler.Books)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

<!-- [LINE EDIT] "There is no classpath, no build descriptor listing source roots — the module path is the single source of truth for all import resolution." — strong sentence; keep. -->
There is no classpath, no build descriptor listing source roots — the module path is the single source of truth for all import resolution. Verify the build compiles cleanly:

```bash
# From the repo root (workspace active)
go build ./services/gateway/...
```

---

## Exercise

Add a second service module and integrate it with the workspace.

<!-- [STRUCTURAL] Well-scoped exercise — builds on the walkthrough and requires the reader to exercise every command just introduced. Good. -->
1. Create the directory `services/auth/` and initialize a new Go module inside it. Use the module path `github.com/fesoliveira014/library-system/services/auth`.

<!-- [LINE EDIT] "Create `services/auth/cmd/main.go` with a minimal `main` function that prints `"auth service starting"` to standard output and exits." — fine. -->
<!-- [COPY EDIT] "standard output" — lowercase is correct. -->
2. Create `services/auth/cmd/main.go` with a minimal `main` function that prints `"auth service starting"` to standard output and exits.

3. Register the new module with the workspace using `go work use`.

4. Run `go work sync` from the repository root and confirm it exits cleanly.

5. Verify that `go build ./services/auth/...` succeeds from the repository root.

<details>
<summary>Hints</summary>

<!-- [LINE EDIT] "A minimal `main.go` only needs `package main` and a `func main()` body — no imports required if you use the built-in `println` instead of `fmt.Println`." — note: `println` is a builtin but is commonly discouraged for production code. The hint is acceptable because this is an exercise minimum; perhaps add ", but prefer `fmt.Println` in real code". Judgment call. -->
- `go mod init` must be run from inside the target directory, or pass the path explicitly.
- A minimal `main.go` only needs `package main` and a `func main()` body — no imports required if you use the built-in `println` instead of `fmt.Println`.
<!-- [LINE EDIT] "though manual edits are valid Go syntax and will work" → "manual edits are valid and will work". Less defensive. -->
- The `go.work` file is updated by `go work use`, not by editing it manually (though manual edits are valid Go syntax and will work).

</details>

---

## References

<!-- [COPY EDIT] Footnote formatting is consistent: author/title descriptor, then URL. Keep style across chapter. -->
[^1]: Go Modules Reference — the authoritative specification for `go.mod`, version selection, and the module proxy protocol. <https://go.dev/ref/mod>

[^2]: Tutorial: Getting started with multi-module workspaces — the official walkthrough for `go.work` and workspace commands. <https://go.dev/doc/tutorial/workspaces>

[^3]: Organizing a Go module — Go team guidance on `cmd/`, `internal/`, and package layout decisions. <https://go.dev/doc/modules/layout>
