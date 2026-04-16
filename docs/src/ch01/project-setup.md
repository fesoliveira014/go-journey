## 1.1 Project Setup

Every Go project starts by answering one question: *how does the toolchain find, version, and build your code?* The answer lives in two files—`go.mod` for each service, and `go.work` at the monorepo root. This section explains both, then reconstructs the choices made when setting up this project.

---

### Go Modules

If you come from the JVM world, a Go module resembles a Maven `pom.xml` or Gradle `build.gradle` file. It declares the project name, the minimum Go version, and all external dependencies. Unlike Maven or Gradle, Go bakes module support directly into the toolchain (`go mod`) with no plugin layer.

A module is a directory tree whose root contains a `go.mod` file. Every `.go` source file in that tree belongs to exactly one module.

#### The `go.mod` File

The Gateway Service's module file looks like this:

```
module github.com/fesoliveira014/library-system/services/gateway

go 1.22
```

Three things worth unpacking:

**`module`—the module path.** This is the canonical import path prefix for every package inside this module. Any file anywhere in the tree can import a package from this module using that prefix. The convention is to use the repository URL as the module path—for example, `github.com/<user>/<repo>/<optional-subdirectory>`. This is not just a convention for readability; it is how `go get` and the Go proxy infrastructure locate and download modules.

Contrast this with Java packages, where the reverse-domain convention (`com.example.myapp`) is separate from the artifact coordinates in `pom.xml`. In Go, they are the same string.

**`go`—the minimum language version.** This pins language semantics. The toolchain refuses to build if the installed Go version is older than what is declared here.

**Dependencies** appear as `require` directives added automatically by `go get` or `go mod tidy`, following [Semantic Versioning](https://semver.org/). One Go-specific rule worth knowing early: If a module releases a v2 or higher, the major version must appear in the module path itself (`github.com/foo/bar/v2`). This lets two incompatible major versions coexist in a single binary without conflict—something that has historically been painful in Maven/Gradle projects.

#### Creating a Module

```bash
mkdir -p services/gateway
cd services/gateway
go mod init github.com/fesoliveira014/library-system/services/gateway
```

That single command writes `go.mod`; nothing else is required to start writing Go.

---

### Go Workspaces

This project is a monorepo: multiple independently deployable services live inside a single repository, each with its own `go.mod`. That raises a problem—how does the toolchain resolve an import from a sibling service during local development without first publishing it to a registry?

The answer is `go.work`, introduced in Go 1.18. A workspace file at the repository root tells the toolchain to resolve specific module paths to local directories instead of fetching them from the network. It does not replace `go.mod`—each module retains its own dependency graph and builds independently. The workspace is a development-time overlay.

```
go 1.22

use ./services/gateway
```

The `use` directive registers a local module. When any code in the workspace imports a package whose path matches a registered module, the toolchain resolves it locally. This is analogous to Gradle's multi-project build, with one key advantage: Each `go.mod` remains self-contained. Running `go build ./...` from inside `services/gateway` works exactly as if the workspace did not exist.

#### Workspace Commands

```bash
# Initialize a workspace at the repo root
go work init

# Register an existing module
go work use ./services/gateway

# After adding or removing modules, resync workspace metadata
go work sync
```

`go work sync` downloads any dependencies that are listed in a module's `go.mod` but not yet reflected in the workspace's lock state. The command is idempotent and safe to run at any time.

---

### Project Structure Conventions

Go has no mandated project layout, but the community has converged on conventions that most open-source projects follow. There is no `src/` directory—that is a Java/Maven convention Go does not use.

#### `cmd/`

The `cmd/` directory holds executable entry points. Each subdirectory under `cmd/` corresponds to one binary. If a module produces a single binary, it is common to place `main.go` directly in `cmd/` without a further subdirectory. The Gateway Service does exactly this:

```
services/gateway/
└── cmd/
    └── main.go
```

#### `internal/`

The `internal/` directory enforces package visibility at the toolchain level. Any package path containing `/internal/` can only be imported by code rooted at the parent of `internal/`. `services/gateway/internal/handler` is importable by `services/gateway/cmd/main.go` but by nothing outside the gateway module—not even another service in the same repository. This is a stronger boundary than Java's package-private: it is enforced by the compiler across module boundaries.

#### The Full Gateway Layout

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

### Walkthrough: Recreating the Setup

The modules and workspaces commands shown above produced the current project state. After running `go work init`, `go mod init`, and `go work use` as described, create the directory structure:

```bash
mkdir -p services/gateway/cmd
mkdir -p services/gateway/internal/handler
```

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

There is no classpath, no build descriptor listing source roots—the module path is the single source of truth for all import resolution. Verify the build compiles cleanly:

```bash
# From the repo root (workspace active)
go build ./services/gateway/...
```

---

### Exercise

Add a second service module and integrate it with the workspace.

1. Create the directory `services/auth/` and initialize a new Go module inside it. Use the module path `github.com/fesoliveira014/library-system/services/auth`.

2. Create `services/auth/cmd/main.go` with a minimal `main` function that prints `"auth service starting"` to standard output and exits.

3. Register the new module with the workspace using `go work use`.

4. Run `go work sync` from the repository root and confirm it exits cleanly.

5. Verify that `go build ./services/auth/...` succeeds from the repository root.

<details>
<summary>Hints</summary>

- `go mod init` must be run from inside the target directory, or pass the path explicitly.
- A minimal `main.go` only needs `package main` and a `func main()` body—no imports required if you use the built-in `println` instead of `fmt.Println`.
- The `go.work` file is updated by `go work use`, not by editing it manually (though manual edits are valid Go syntax and will work).

</details>

---

### References

[^1]: Go Modules Reference—the authoritative specification for `go.mod`, version selection, and the module proxy protocol. <https://go.dev/ref/mod>

[^2]: Tutorial: Getting started with multi-module workspaces—the official walkthrough for `go.work` and workspace commands. <https://go.dev/doc/tutorial/workspaces>

[^3]: Organizing a Go module—Go team guidance on `cmd/`, `internal/`, and package layout decisions. <https://go.dev/doc/modules/layout>
