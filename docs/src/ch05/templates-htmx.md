# 5.2 Templates & HTMX

Go's `html/template` package is a server-side template engine that produces HTML. If you have used Thymeleaf with Spring, the concept is familiar: you write HTML files with special directives that the server evaluates before sending the response. The difference is that Go templates are not XML-attribute-based (like Thymeleaf's `th:text`) -- they use a `{{...}}` syntax embedded in the HTML.

This section covers the template system, explains a subtle gotcha that will bite you if you do not know about it, and introduces HTMX for progressive enhancement.

---

## Go Template Basics

A Go template is an HTML file with **actions** enclosed in double curly braces. The most important actions are:

| Action | Purpose | Thymeleaf equivalent |
|---|---|---|
| `{{.FieldName}}` | Print a field from the data passed to the template | `th:text="${obj.fieldName}"` |
| `{{if .Condition}}...{{end}}` | Conditional rendering | `th:if="${condition}"` |
| `{{range .Items}}...{{end}}` | Loop over a slice | `th:each="item : ${items}"` |
| `{{define "name"}}...{{end}}` | Define a named template block | Thymeleaf fragment |
| `{{template "name" .}}` | Include a named template, passing data | `th:replace="~{fragment}"` |
| `{{block "name" .}}...{{end}}` | Define a block with a default body (overridable) | Thymeleaf layout block |

The dot (`.`) is the **cursor** -- it refers to the current data context. Inside a `{{range}}`, the dot shifts to the current element. This is the single most confusing thing about Go templates for newcomers.

### Auto-Escaping

Go's `html/template` package (not `text/template`) automatically escapes values based on context. A string rendered inside an HTML element is HTML-escaped. A string inside a `<script>` tag is JavaScript-escaped. This prevents XSS attacks by default -- you do not need to remember to call an escape function on every variable.

---

## Base Layout and Partials

Most pages share common structure: a `<head>`, navigation bar, flash messages, and a `<main>` content area. We use a **base layout** that defines this structure, with `{{block}}` placeholders that individual pages override.

Here is `base.html`:

```html
<!-- services/gateway/templates/base.html -->

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Library System{{end}}</title>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    {{template "nav" .}}
    <main class="container">
        {{if .Flash}}{{template "flash" .}}{{end}}
        {{block "content" .}}{{end}}
    </main>
</body>
</html>
```

The `{{block "title" .}}Library System{{end}}` action defines a block named `"title"` with a default value of "Library System". A page template can override this:

```html
{{define "title"}}Login{{end}}
{{define "content"}}
<h1>Login</h1>
<!-- ... form ... -->
{{end}}
```

The `{{template "nav" .}}` action includes a **partial** -- a reusable fragment defined in a separate file:

```html
<!-- services/gateway/templates/partials/nav.html -->

{{define "nav"}}
<nav>
    <a href="/">Library System</a>
    <a href="/books">Catalog</a>
    {{if .User}}
        {{if eq .User.Role "admin"}}
            <a href="/admin/books/new">Add Book</a>
        {{end}}
        <form method="POST" action="/logout" style="display:inline">
            <button type="submit">Logout</button>
        </form>
    {{else}}
        <a href="/login">Login</a>
        <a href="/register">Register</a>
    {{end}}
</nav>
{{end}}
```

Notice that the nav template accesses `.User` -- this comes from the `PageData` struct that wraps every template rendering:

```go
type PageData struct {
    User  *UserInfo
    Flash string
    Data  any
}
```

Every page gets the current user, flash message, and page-specific data. Page templates access their data through `.Data` (e.g., `.Data.Books`, `.Data.SelectedGenre`).

---

## The Clone-per-Page Problem

Here is where Go templates get tricky. If you naively parse all your templates into a single set:

```go
// DON'T DO THIS
tmpl, err := template.ParseGlob("templates/*.html")
```

You will hit a problem: every page template defines `{{define "content"}}...{{end}}`. When parsed into a single set, the **last file parsed wins** -- only one "content" definition survives. All pages render the same content block.

In Thymeleaf, this is not an issue because each template file is an independent document that Thymeleaf processes separately. Go's template system works differently: all `{{define}}` blocks in a parsed set share a single namespace.

### The Solution: Clone Per Page

The fix is to parse the base layout and partials once, then **clone** the set for each page template. Each clone gets its own copy of the namespace, so each page's `{{define "content"}}` overrides the base without affecting other pages.

Here is the actual `ParseTemplates` function:

```go
// services/gateway/internal/handler/server.go

func ParseTemplates(templateDir string) (map[string]*template.Template, error) {
    baseFile := filepath.Join(templateDir, "base.html")
    partialsGlob := filepath.Join(templateDir, "partials", "*.html")

    partials, err := filepath.Glob(partialsGlob)
    if err != nil {
        return nil, err
    }

    // Build the list of files that make up the base set.
    baseFiles := append([]string{baseFile}, partials...)

    // Parse the base set once; we'll clone it for each page.
    baseSet, err := template.ParseFiles(baseFiles...)
    if err != nil {
        return nil, err
    }

    // Find all page templates (direct children of templateDir, not partials).
    entries, err := os.ReadDir(templateDir)
    if err != nil {
        return nil, err
    }

    m := make(map[string]*template.Template)
    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".html" {
            continue
        }
        name := e.Name() // e.g. "home.html"
        if name == "base.html" {
            continue
        }
        pageFile := filepath.Join(templateDir, name)

        // Clone the base set and parse the page file into it.
        clone, err := baseSet.Clone()
        if err != nil {
            return nil, err
        }
        if _, err = clone.ParseFiles(pageFile); err != nil {
            return nil, err
        }
        m[name] = clone
    }

    return m, nil
}
```

The algorithm:

1. Parse `base.html` + all `partials/*.html` into a base `*template.Template` set.
2. For each page file (e.g., `catalog.html`, `login.html`), clone the base set.
3. Parse the page file into the clone. Its `{{define "content"}}` and `{{define "title"}}` override the base `{{block}}` defaults.
4. Store each clone in a map keyed by filename.

At render time, the handler looks up the template by name and executes it:

```go
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
    pd := PageData{
        User:  userFromContext(r.Context()),
        Flash: consumeFlash(w, r),
        Data:  data,
    }
    tmpl, ok := s.tmpl[name]
    if !ok {
        slog.ErrorContext(r.Context(), "template not found", "name", name)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
        slog.ErrorContext(r.Context(), "template error", "err", err)
    }
}
```

The key call is `tmpl.ExecuteTemplate(w, "base.html", pd)` -- it starts rendering from `base.html`, which pulls in the page's overridden `"content"` and `"title"` blocks.

---

## HTMX: Hypermedia-Driven Interactivity

HTMX[^1] is a small JavaScript library (14KB) that lets you add dynamic behavior to server-rendered HTML without writing JavaScript. Its philosophy is that the server should return **HTML fragments**, not JSON -- the browser swaps fragments into the DOM. This is the hypermedia approach, and it is a natural fit for a Go BFF.

### The Three Key Attributes

| Attribute | Purpose | Example |
|---|---|---|
| `hx-get` / `hx-post` | Issue an AJAX request | `hx-get="/books"` |
| `hx-target` | Which DOM element receives the response | `hx-target="#book-list"` |
| `hx-swap` | How to insert the response into the target | `hx-swap="innerHTML"` |

That is the core of HTMX. There are more attributes for triggers, indicators, and history, but these three cover most use cases.

### Building the Catalog Filter

Our catalog page uses HTMX for genre filtering. When the user selects a genre from a dropdown, HTMX issues a GET request to `/books?genre=Programming` and swaps the response into the book list -- no full page reload.

```html
<!-- services/gateway/templates/catalog.html -->

{{define "title"}}Catalog{{end}}
{{define "content"}}
<h1>Book Catalog</h1>
<div class="filter-bar">
    <label for="genre">Filter by genre:</label>
    <select name="genre" id="genre"
            hx-get="/books"
            hx-target="#book-list"
            hx-swap="innerHTML">
        <option value="">All Genres</option>
        <option value="Programming"
            {{if eq .Data.SelectedGenre "Programming"}}selected{{end}}>Programming</option>
        <option value="Architecture"
            {{if eq .Data.SelectedGenre "Architecture"}}selected{{end}}>Architecture</option>
        <option value="Distributed Systems"
            {{if eq .Data.SelectedGenre "Distributed Systems"}}selected{{end}}>Distributed Systems</option>
    </select>
</div>
<div id="book-list" class="book-list">
    {{range .Data.Books}}
        {{template "book_card" .}}
    {{end}}
</div>
{{end}}
```

When the `<select>` changes, HTMX reads the `hx-get` attribute, appends the `<select>`'s value as a query parameter (`/books?genre=Programming`), and sends a GET request. The server response replaces the `innerHTML` of `#book-list`.

The `book_card` partial renders each book:

```html
<!-- services/gateway/templates/partials/book_card.html -->

{{define "book_card"}}
<div class="book-card">
    <h3><a href="/books/{{.Id}}">{{.Title}}</a></h3>
    <p class="meta">{{.Author}} · {{.PublishedYear}}</p>
    <p class="meta">{{.Genre}} · {{.AvailableCopies}}/{{.TotalCopies}} available</p>
</div>
{{end}}
{{define "book_cards"}}
{{range .}}{{template "book_card" .}}{{end}}
{{end}}
```

### The Handler: Detecting HTMX Requests

The `BookList` handler serves both the full page (initial load) and the HTMX fragment (filter updates). It distinguishes between the two by checking the `HX-Request` header, which HTMX sends on every AJAX request:

```go
// services/gateway/internal/handler/catalog.go

func (s *Server) BookList(w http.ResponseWriter, r *http.Request) {
    genre := r.URL.Query().Get("genre")
    resp, err := s.catalog.ListBooks(r.Context(), &catalogv1.ListBooksRequest{
        Genre: genre,
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to load books")
        return
    }
    if r.Header.Get("HX-Request") != "" {
        s.renderPartial(w, "book_cards", resp.Books)
        return
    }
    s.render(w, r, "catalog.html", map[string]any{
        "Books":         resp.Books,
        "SelectedGenre": genre,
    })
}
```

For an HTMX request, the handler renders only the `"book_cards"` partial -- a bare list of `<div>` elements with no `<html>`, `<head>`, or layout. HTMX swaps this HTML fragment directly into the `#book-list` container. For a full page load (or a browser with JavaScript disabled), the handler renders the complete `catalog.html` page.

The `renderPartial` method executes a named template from the base set without the page layout:

```go
func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if s.baseTmpl == nil {
        slog.Error("no templates loaded; cannot render partial", "name", name)
        return
    }
    if err := s.baseTmpl.ExecuteTemplate(w, name, data); err != nil {
        slog.Error("template error", "err", err)
    }
}
```

This is progressive enhancement: the page works without JavaScript (you just get full-page reloads on filter changes), but with HTMX it feels like an SPA.

---

## When to Use HTMX

HTMX is a good fit when:

- You want to add interactivity to specific parts of a server-rendered page (filters, search-as-you-type, inline editing)
- Your backend is already rendering HTML
- You do not need a full client-side application state

HTMX is *not* a good fit when:

- You are building a complex, highly interactive UI (a spreadsheet, a drag-and-drop kanban board)
- You need offline support
- Your team already has deep React/Vue/Angular expertise and the application demands it

For our library system -- catalog browsing with filters and admin CRUD forms -- HTMX is the right tool. It adds interactivity with zero JavaScript in our codebase and keeps all rendering logic on the server.

---

## References

[^1]: [HTMX documentation](https://htmx.org/docs/) -- Official HTMX reference and examples.
[^2]: [Go html/template package](https://pkg.go.dev/html/template) -- Standard library template documentation.
[^3]: [Hypermedia Systems (book)](https://hypermedia.systems/) -- Free online book on hypermedia-driven application architecture by the HTMX authors.
[^4]: [Go template tutorial](https://gowebexamples.com/templates/) -- Practical examples of Go's template system.
