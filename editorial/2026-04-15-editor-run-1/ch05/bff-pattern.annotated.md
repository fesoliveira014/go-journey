# 5.1 The BFF Pattern

<!-- [STRUCTURAL] Strong opening: the three-way fork (direct calls / API gateway / BFF) is the right frame. This delivers the heading. The only gap: after listing three options you jump straight to "Our gateway is a BFF" — a one-line "we pick option 3 because…" would make the rhetorical handoff more intentional. -->
<!-- [LINE EDIT] "there are three common approaches" → "three common approaches exist" or "engineers pick from three common approaches" — minor; the existential-there is acceptable. Leave if prose rhythm matters more. -->
When you have a microservices backend and need to serve a web frontend, there are three common approaches:

<!-- [COPY EDIT] "React, Angular, etc." — CMOS 6.43 requires a comma before "etc.". Correct as written. But see [^?] — "etc." in formal prose is sometimes discouraged; "React, Angular, or similar" is warmer. Style choice. -->
<!-- [LINE EDIT] "leaks internal service topology to the client" — good tight phrase. Keep. -->
1. **Direct API calls from the browser.** The frontend (React, Angular, etc.) calls each microservice directly. This leaks internal service topology to the client, requires CORS configuration for every service, and forces the browser to make multiple round trips to assemble a single page.

<!-- [COPY EDIT] "Kong, Envoy, AWS API Gateway" — product capitalizations correct. -->
2. **API gateway.** A general-purpose reverse proxy (Kong, Envoy, AWS API Gateway) sits in front of all services. It handles routing, rate limiting, and authentication -- but it does not render HTML or understand the needs of any particular client.

<!-- [COPY EDIT] "Backend-for-Frontend (BFF)" — first use in this file. Lock style across chapter (see index.md note). Sam Newman's original coinage is lowercase "backends for frontends"; the acronym is uppercase BFF regardless. -->
<!-- [LINE EDIT] "It speaks the client's language (HTTP + HTML for browsers, JSON for mobile apps) and translates requests into backend RPC calls." — active, concrete, good. -->
3. **Backend-for-Frontend (BFF).** A lightweight backend service built specifically for one client type. It speaks the client's language (HTTP + HTML for browsers, JSON for mobile apps) and translates requests into backend RPC calls. Each client type gets its own BFF, tailored to its needs.

<!-- [LINE EDIT] "It does not contain business logic -- it does not validate ISBNs or hash passwords." Consider: "It contains no business logic — it does not validate ISBNs or hash passwords." (active, no "does not contain"). -->
<!-- [COPY EDIT] "ISBNs" — uppercase proper noun. Correct. -->
Our gateway is a BFF. It serves HTML to the browser, issues gRPC calls to the Auth and Catalog services, and owns the presentation layer. It does not contain business logic -- it does not validate ISBNs or hash passwords. It validates form input just enough to give the user good error messages, then delegates to the backend.

<!-- [LINE EDIT] "If you have worked with Spring MVC" — clear, keeps the Java-bridge idiom the book uses consistently. -->
<!-- [COPY EDIT] "@Controller" and "@Service" — inline code formatting. Good. -->
If you have worked with Spring MVC, the BFF is analogous to a `@Controller` layer that calls `@Service` beans -- except the "services" live in separate processes and communicate over gRPC instead of local method calls.

---

## Go as a BFF Language

<!-- [STRUCTURAL] This section answers "why Go" in three bullets. Works. Consider adding one sentence on compile-to-single-binary / container-size advantage — a common BFF selling point when comparing with Node/Python. Optional. -->
<!-- [LINE EDIT] "Go is well-suited for this role." — acceptable. Could be tightened to "Go fits this role well." -->
Go is well-suited for this role. The standard library provides:

<!-- [COPY EDIT] "`net/http`" — stdlib path, inline code. Correct. -->
<!-- [COPY EDIT] "production-quality" — hyphenated compound adjective before "HTTP server" (CMOS 7.81). Correct. -->
- **`net/http`** -- a production-quality HTTP server and router (no framework needed)
- **`html/template`** -- a context-aware template engine with auto-escaping (prevents XSS by default)
- **`net/http/cookiejar`** and `http.Cookie` -- first-class cookie support

<!-- [LINE EDIT] Filler cut candidate: "You do not need Gin, Echo, Chi, or any other framework." — fine. "any other framework" is clean. -->
<!-- [COPY EDIT] "Gin, Echo, Chi" — all are project names; Chi is often styled "chi" in its own docs. Please verify: Chi vs. chi project capitalization (https://github.com/go-chi/chi). -->
<!-- [COPY EDIT] "Go 1.22 stdlib router added method-based pattern matching" — Please verify: Go 1.22 is the correct version that introduced enhanced ServeMux patterns. Confirmed by footnote [^1]. -->
You do not need Gin, Echo, Chi, or any other framework. The Go 1.22 stdlib router added method-based pattern matching, which covers everything we need. We will use the stdlib exclusively.

---

## The `Server` Struct

<!-- [STRUCTURAL] Struct-first, then constructor, then DI commentary. Good order. -->
The gateway's core type is a `Server` struct that holds its dependencies: gRPC clients for the backend services and a map of parsed HTML templates.

```go
// services/gateway/internal/handler/server.go

type Server struct {
    auth    authv1.AuthServiceClient
    catalog catalogv1.CatalogServiceClient
    tmpl    map[string]*template.Template
    baseTmpl *template.Template // base set for rendering partials
}

func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, tmpl map[string]*template.Template) *Server {
    // Pick any entry for partial rendering — all share the same partial definitions.
    var base *template.Template
    for _, t := range tmpl {
        base = t
        break
    }
    return &Server{auth: auth, catalog: catalog, tmpl: tmpl, baseTmpl: base}
}
```

<!-- [STRUCTURAL] The "any entry works" picking pattern deserves a one-line forward-reference: readers will ask "but why is any entry good enough?" and the answer comes in §5.2's clone-per-page section. A forward reference tightens the arc. -->
<!-- [LINE EDIT] "This is Go's dependency injection pattern" → "This is dependency injection, the Go way" or "This is manual dependency injection." "Go's dependency injection pattern" suggests one canonical pattern, which oversells. -->
<!-- [LINE EDIT] Filler "You wire things up explicitly in `main()`." — strong and correct; keep. -->
This is Go's dependency injection pattern: construct your dependencies outside the struct and pass them in through the constructor. There is no annotation magic, no DI container, no classpath scanning. You wire things up explicitly in `main()`.

<!-- [STRUCTURAL] Spring comparison works, but is ~15 lines of Java for a small point. Consider trimming to the constructor signature only to save space; the `@Autowired` constructor point is the payload. -->
In Spring, the equivalent would be:

```java
@Controller
public class GatewayController {
    private final AuthServiceGrpc.AuthServiceBlockingStub authClient;
    private final CatalogServiceGrpc.CatalogServiceBlockingStub catalogClient;

    @Autowired
    public GatewayController(AuthServiceBlockingStub authClient,
                             CatalogServiceBlockingStub catalogClient) {
        this.authClient = authClient;
        this.catalogClient = catalogClient;
    }
}
```

<!-- [LINE EDIT] "the wiring is visible in one place (`main.go`), not scattered across annotations that require understanding Spring's component scanning rules" — good, opinionated, accurate. Keep. -->
<!-- [COPY EDIT] "component scanning" — no hyphen needed as noun phrase (CMOS 7.81; only hyphenate before a noun). Correct. -->
The Go version is more explicit but achieves the same result: the `Server` owns its dependencies, they are injected at construction time, and the struct's methods (handlers) can use them freely. The advantage of Go's approach is that the wiring is visible in one place (`main.go`), not scattered across annotations that require understanding Spring's component scanning rules.

---

## Go 1.22+ Stdlib Routing

<!-- [STRUCTURAL] Good setup: pre-1.22 limitation → 1.22 change → concrete pattern examples. Hierarchy clear. -->
<!-- [LINE EDIT] "the stdlib `http.ServeMux` could only match URL paths -- not HTTP methods. You had to check `r.Method` inside the handler or use a third-party router." — tight. Keep. -->
Before Go 1.22, the stdlib `http.ServeMux` could only match URL paths -- not HTTP methods. You had to check `r.Method` inside the handler or use a third-party router. Go 1.22 changed this with **method patterns**[^1]:

```go
mux.HandleFunc("GET /books", srv.BookList)
mux.HandleFunc("GET /books/{id}", srv.BookDetail)
mux.HandleFunc("POST /admin/books", srv.AdminBookCreate)
```

<!-- [COPY EDIT] "Path parameters use curly braces: `{id}` is extracted in the handler with `r.PathValue(\"id\")`." — colon + partial list; grammar OK. -->
<!-- [COPY EDIT] "The special pattern `\"GET /{$}\"` matches only the root path exactly" — Please verify: `{$}` terminator syntax for ServeMux. Per Go 1.22 release notes, confirmed. -->
<!-- [LINE EDIT] "without the `{$}`, `GET /` would match all `GET` requests as a prefix" — factually correct. Good. -->
The method comes first, separated from the path by a space. Path parameters use curly braces: `{id}` is extracted in the handler with `r.PathValue("id")`. The special pattern `"GET /{$}"` matches only the root path exactly (without the `{$}`, `GET /` would match all `GET` requests as a prefix).

Compare this to Spring:

```java
@GetMapping("/books")
public String bookList(Model model) { ... }

@GetMapping("/books/{id}")
public String bookDetail(@PathVariable String id, Model model) { ... }

@PostMapping("/admin/books")
public String adminBookCreate(@ModelAttribute BookForm form) { ... }
```

<!-- [LINE EDIT] "The Go version is more verbose (no annotation shorthand) but conceptually identical." — "more verbose" is arguable; character-counting, the Go registrations are shorter than Spring's annotation+method signature. Consider: "The Go version is more explicit (no annotations) but conceptually identical." -->
The Go version is more verbose (no annotation shorthand) but conceptually identical. One notable difference: Spring uses separate annotations for each method (`@GetMapping`, `@PostMapping`), while Go uses a single registration function with the method in the pattern string.

<!-- [STRUCTURAL] Big route table comes next. Useful, but it is a wall of registrations that could be split by section (auth / catalog / admin) with one-line intros. Currently the inline comments do that, so it's borderline OK. -->
Here is the complete route registration from `main.go`:

```go
// services/gateway/cmd/main.go

mux := http.NewServeMux()
mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

mux.HandleFunc("GET /healthz", srv.Health)
mux.HandleFunc("GET /{$}", srv.Home)

// Auth routes
mux.HandleFunc("GET /login", srv.LoginPage)
mux.HandleFunc("POST /login", srv.LoginSubmit)
mux.HandleFunc("GET /register", srv.RegisterPage)
mux.HandleFunc("POST /register", srv.RegisterSubmit)
mux.HandleFunc("POST /logout", srv.Logout)
mux.HandleFunc("GET /auth/oauth2/google", srv.OAuth2Start)
mux.HandleFunc("GET /auth/oauth2/google/callback", srv.OAuth2Callback)

// Catalog routes
mux.HandleFunc("GET /books", srv.BookList)
mux.HandleFunc("GET /books/{id}", srv.BookDetail)

// Admin routes
mux.HandleFunc("GET /admin/books/new", srv.AdminBookNew)
mux.HandleFunc("POST /admin/books", srv.AdminBookCreate)
mux.HandleFunc("GET /admin/books/{id}/edit", srv.AdminBookEdit)
mux.HandleFunc("POST /admin/books/{id}", srv.AdminBookUpdate)
mux.HandleFunc("POST /admin/books/{id}/delete", srv.AdminBookDelete)
```

<!-- [LINE EDIT] "Each route maps to a method on the `Server` struct." — tight. Keep. -->
<!-- [LINE EDIT] "The pattern is RESTful" → "The route style is RESTful". "Pattern" is already the ServeMux term in this section; reusing it muddies. -->
<!-- [COPY EDIT] "HTML forms can only submit `GET` and `POST`" — accurate per HTML5 spec; true for `<form method>` on browsers. Keep. -->
<!-- [LINE EDIT] "This is a standard pattern in server-rendered applications." — good, natural closer. -->
Each route maps to a method on the `Server` struct. The pattern is RESTful: `GET` for reads, `POST` for mutations. Notice that `POST /admin/books/{id}/delete` uses a POST, not a `DELETE` method -- HTML forms can only submit `GET` and `POST`, so we use a URL suffix to distinguish the action. This is a standard pattern in server-rendered applications.

---

## Middleware

<!-- [STRUCTURAL] Good order: wire diagram → chain semantics → code. -->
The gateway uses two middleware functions, applied as a chain around the `ServeMux`:

```go
// services/gateway/cmd/main.go

var h http.Handler = mux
h = middleware.Auth(h, jwtSecret)
h = middleware.Logging(h)
```

<!-- [LINE EDIT] "Middleware in Go wraps an `http.Handler` and returns a new `http.Handler`." — good, canonical statement. -->
<!-- [COPY EDIT] "inside-out" — compound adjective, hyphenated correctly (CMOS 7.81). -->
<!-- [LINE EDIT] "`Logging` is the outermost layer (runs first)" — accurate given the wrapping order shown. -->
Middleware in Go wraps an `http.Handler` and returns a new `http.Handler`. The chain is applied inside-out: `Logging` is the outermost layer (runs first), then `Auth`, then the `mux` router dispatches to the actual handler.

The logging middleware captures the response status code and request duration:

```go
// services/gateway/internal/middleware/logging.go

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(sw, r)
        slog.InfoContext(r.Context(), "http request",
            "method", r.Method,
            "path", r.URL.Path,
            "status", sw.status,
            "duration", time.Since(start),
        )
    })
}
```

<!-- [LINE EDIT] "The `statusWriter` trick is a common Go pattern" — "trick" is conversational; fine for tutor tone. Keep. -->
<!-- [LINE EDIT] "Without this wrapper, the middleware has no way to inspect the response -- `http.ResponseWriter` is a write-only interface." — tight and informative. -->
<!-- [COPY EDIT] "`http.ResponseWriter` is a write-only interface" — technically it exposes Header() which is read-write for the header map. "Write-only" is a minor inaccuracy. Consider: "`http.ResponseWriter` does not expose response status or body reads." -->
The `statusWriter` trick is a common Go pattern: embed `http.ResponseWriter` to inherit all its methods, then override `WriteHeader` to capture the status code. Without this wrapper, the middleware has no way to inspect the response -- `http.ResponseWriter` is a write-only interface.

<!-- [LINE EDIT] "The auth middleware is covered in detail in section 5.3." — good signpost. -->
The auth middleware is covered in detail in section 5.3.

<!-- [LINE EDIT] "In Spring terms, these middleware are the equivalent of" → "In Spring terms, these middleware functions are equivalent to". "middleware" is a mass noun sometimes but "these middleware are" reads awkwardly. -->
<!-- [COPY EDIT] "`HandlerInterceptor` or servlet `Filter` chains" — product names and Java types, inline code OK. -->
In Spring terms, these middleware are the equivalent of `HandlerInterceptor` or servlet `Filter` chains. The key difference is that Spring manages the chain through configuration, while in Go you compose it explicitly with function calls.

---

## Wiring It All Together

<!-- [STRUCTURAL] Good capstone section. Ties the piece together. -->
<!-- [LINE EDIT] "The `main.go` function ties everything together" — `main.go` is a file, not a function; the function is `main()`. Fix to: "The `main` function ties everything together:". -->
<!-- [LINE EDIT] Sentence 1 is a list, but the next sentence "There is no framework bootstrap, no YAML configuration, no classpath scanning. Everything is explicit." is almost identical to the Server-struct section's "no annotation magic, no DI container, no classpath scanning." Minor redundancy — trim one. -->
The `main.go` function ties everything together: environment variables, gRPC connections, template parsing, server construction, route registration, and middleware application. There is no framework bootstrap, no YAML configuration, no classpath scanning. Everything is explicit.

```go
// Create gRPC clients
authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// ...
catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// ...

// Parse templates using the clone-per-page pattern
tmpl, err := handler.ParseTemplates("templates")
// ...

// Create server with injected dependencies
authClient := authv1.NewAuthServiceClient(authConn)
catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
srv := handler.New(authClient, catalogClient, tmpl)
```

<!-- [LINE EDIT] "This is the \"explicit wiring\" philosophy of Go. It is more lines of code than `@SpringBootApplication`, but every dependency is visible and traceable." — strong closer. Keep. -->
<!-- [COPY EDIT] `"explicit wiring"` uses straight quotes; CMOS 6.9 wants smart quotes for prose ("explicit wiring"). If the book's Markdown pipeline typographically converts, leave; otherwise change. -->
This is the "explicit wiring" philosophy of Go. It is more lines of code than `@SpringBootApplication`, but every dependency is visible and traceable. If you want to know what the gateway depends on, read `main.go` -- it is all there.

---

## References

<!-- [COPY EDIT] Please verify: the Go 1.22 release notes URL anchor `#enhanced_routing_patterns` (https://go.dev/doc/go1.22#enhanced_routing_patterns) still exists. -->
<!-- [COPY EDIT] Please verify: Sam Newman BFF article URL still resolves (https://samnewman.io/patterns/architectural/bff/). -->
<!-- [COPY EDIT] "*Building Microservices*" — book title italicized. Correct per CMOS 8.168. -->
[^1]: [Go 1.22 release notes -- Enhanced routing patterns](https://go.dev/doc/go1.22#enhanced_routing_patterns) -- Official documentation for the new `ServeMux` routing syntax.
[^2]: [Sam Newman -- Backends for Frontends](https://samnewman.io/patterns/architectural/bff/) -- The original description of the BFF pattern by the author of *Building Microservices*.
[^3]: [net/http package documentation](https://pkg.go.dev/net/http) -- Go standard library HTTP server reference.
