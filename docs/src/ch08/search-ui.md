# 8.4 Search UI

The search feature surfaces in the gateway through two routes: a full search page with filters, and an autocomplete endpoint that powers the nav bar's instant suggestions. Both use HTMX for interactive behavior with minimal JavaScript.

If you are coming from Spring MVC + Thymeleaf, the architecture is familiar: the gateway renders HTML templates server-side and sends complete (or partial) HTML to the browser. The difference is that HTMX lets us do partial page updates—fetching and swapping HTML fragments—without building a single-page application or writing a REST API that returns JSON.

---

## Gateway Search Routes

The gateway's `Server` struct holds a gRPC client for the Search Service, alongside the existing catalog and auth clients:

```go
// services/gateway/internal/handler/server.go

type Server struct {
    auth        authv1.AuthServiceClient
    catalog     catalogv1.CatalogServiceClient
    reservation reservationv1.ReservationServiceClient
    search      searchv1.SearchServiceClient
    tmpl        map[string]*template.Template
    baseTmpl    *template.Template
}
```

The search page handler reads query parameters, calls the Search Service via gRPC, and renders the results:

```go
// services/gateway/internal/handler/search.go

func (s *Server) SearchPage(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    genre := r.URL.Query().Get("genre")
    author := r.URL.Query().Get("author")
    available := r.URL.Query().Get("available")
    pageStr := r.URL.Query().Get("page")

    page, _ := strconv.Atoi(pageStr)
    if page <= 0 {
        page = 1
    }

    data := map[string]any{
        "Query":     query,
        "Genre":     genre,
        "Author":    author,
        "Available": available == "on",
        "Page":      page,
    }

    if query == "" {
        s.render(w, r, "search.html", data)
        return
    }

    resp, err := s.search.Search(r.Context(), &searchv1.SearchRequest{
        Query:         query,
        Genre:         genre,
        Author:        author,
        AvailableOnly: available == "on",
        Page:          int32(page),
        PageSize:      20,
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Search failed")
        return
    }

    data["Books"] = resp.Books
    data["TotalHits"] = resp.TotalHits
    data["QueryTimeMs"] = resp.QueryTimeMs
    data["HasResults"] = len(resp.Books) > 0

    s.render(w, r, "search.html", data)
}
```

The handler follows a pattern you have seen throughout the gateway: extract parameters, build a template data map, call a backend service, add response data to the map, render. When the query is empty, it renders the search page without results—just the form.

Notice `available == "on"`. HTML checkboxes send the value `"on"` when checked and send nothing when unchecked. This is a web platform quirk that every server-rendered application must handle.

---

## The Search Page Template

The template renders both the search form and the results:

```html
<!-- services/gateway/templates/search.html -->

{{define "title"}}Search{{end}}
{{define "content"}}
<h1>Search Books</h1>
<form method="GET" action="/search">
    <input type="text" name="q" value="{{.Data.Query}}" placeholder="Search books...">
    <input type="text" name="genre" value="{{.Data.Genre}}" placeholder="Genre">
    <input type="text" name="author" value="{{.Data.Author}}" placeholder="Author">
    <label>
        <input type="checkbox" name="available" {{if .Data.Available}}checked{{end}}>
        Available only
    </label>
    <button type="submit">Search</button>
</form>

{{if .Data.HasResults}}
<p>{{.Data.TotalHits}} results in {{.Data.QueryTimeMs}}ms</p>
<table>
    <thead>
        <tr>
            <th>Title</th>
            <th>Author</th>
            <th>Genre</th>
            <th>Available</th>
        </tr>
    </thead>
    <tbody>
        {{range .Data.Books}}
        <tr>
            <td><a href="/books/{{.Id}}">{{.Title}}</a></td>
            <td>{{.Author}}</td>
            <td>{{.Genre}}</td>
            <td>{{.AvailableCopies}} / {{.TotalCopies}}</td>
        </tr>
        {{end}}
    </tbody>
</table>
{{else if .Data.Query}}
<p>No results found for "{{.Data.Query}}".</p>
{{end}}
{{end}}
```

The form uses `GET`—search is a read operation, and using GET means the URL contains the full query state (`/search?q=golang&genre=programming`). Users can bookmark, share, and use browser back/forward to navigate search results. This is standard web practice and something you lose when building search as a POST-based SPA.

The template has three states:
1. **No query**—just the form.
2. **Query with results**—form + result count + table.
3. **Query with no results**—form + "no results" message.

The `{{range .Data.Books}}` iterates over the protobuf `BookResult` messages directly. Go's `html/template` can access protobuf message fields by name, just like any other struct. The `.Id`, `.Title`, `.Author` etc. are the generated Go field names from the proto definition.

---

## HTMX-Powered Autocomplete

The nav bar contains a search input that provides instant suggestions as the user types. This is powered by HTMX—no JavaScript handlers, no fetch calls, no JSON parsing:

```html
<!-- services/gateway/templates/partials/nav.html -->

<div style="display:inline;position:relative">
    <form method="GET" action="/search" style="display:inline">
        <input type="hidden" name="q" id="nav-search-q">
        <input type="text" name="prefix" placeholder="Search..."
               hx-get="/search/suggest"
               hx-trigger="keyup changed delay:300ms[this.value.length >= 2]"
               hx-target="#suggestions"
               hx-swap="innerHTML"
               autocomplete="off"
               onchange="document.getElementById('nav-search-q').value=this.value"
               onkeydown="if(event.key==='Enter'){
                   document.getElementById('nav-search-q').value=this.value
               }">
    </form>
    <div id="suggestions" style="position:absolute;background:white;z-index:10"></div>
</div>
```

Let us break down each HTMX attribute:

### `hx-get="/search/suggest"`

When triggered, HTMX sends a GET request to `/search/suggest`. It automatically includes the input's current value as a query parameter: `/search/suggest?prefix=gol`. HTMX does this because the input has a `name` attribute (`prefix`), and HTMX includes named form elements in requests by default.

### `hx-trigger="keyup changed delay:300ms[this.value.length >= 2]"`

This is a compound trigger with several modifiers:

- **`keyup`**—Fire on key release.
- **`changed`**—Only if the value actually changed (ignores arrow keys, shift, etc.).
- **`delay:300ms`**—Debounce: wait 300ms after the last keyup before firing. If the user types another character within 300ms, the timer resets. This prevents flooding the server with requests on fast typing.
- **`[this.value.length >= 2]`**—A JavaScript condition: only fire if the input has at least 2 characters. Single-character searches are too broad to be useful and expensive to run.

In a React or Angular application, you would implement this debounce with `rxjs` operators (`debounceTime`, `filter`, `switchMap`) or a custom hook. HTMX packs the same behavior into a declarative attribute string.

### `hx-target="#suggestions"` and `hx-swap="innerHTML"`

The server's response (an HTML fragment) replaces the innerHTML of the `#suggestions` div. HTMX does not parse JSON or render templates on the client—the server sends ready-to-display HTML.

### The Form Submission Trick

The nav bar search also doubles as a form that navigates to the full search page. When the user presses Enter, the form submits as a regular GET to `/search?q=...`. The hidden input `nav-search-q` is synchronized with the text input via `onchange` and `onkeydown` handlers. This is necessary because the text input sends `prefix` (for the suggest endpoint), but the search page expects `q`.

---

## The Suggest Endpoint

The server-side handler for autocomplete is minimal:

```go
// services/gateway/internal/handler/search.go

func (s *Server) SearchSuggest(w http.ResponseWriter, r *http.Request) {
    prefix := r.URL.Query().Get("prefix")
    if len(prefix) < 2 {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        return
    }

    resp, err := s.search.Suggest(r.Context(), &searchv1.SuggestRequest{
        Prefix: prefix,
        Limit:  5,
    })
    if err != nil {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        return
    }

    s.renderPartial(w, "suggestions.html", resp.Suggestions)
}
```

Two things to note:

1. **Short-circuit for short prefixes.** If the prefix is less than 2 characters, the handler returns an empty response with the correct content type. This renders as empty HTML in the suggestions div, effectively clearing any previous suggestions. The HTMX trigger already filters these out, but the server-side check is defense in depth.

2. **Error handling returns empty HTML.** If the Search Service is down, the user sees no suggestions—not an error message. This is appropriate for autocomplete: it is a progressive enhancement, not a critical feature. The user can still type their query and press Enter to use the full search page.

The `renderPartial` method renders a named template without the full page layout (no `<html>`, no nav, no footer). The suggestions template is a list of links:

```html
<!-- services/gateway/templates/partials/suggestions.html -->

{{define "suggestions.html"}}
{{range .}}
<a href="/books/{{.BookId}}" class="suggestion">
    <strong>{{.Title}}</strong> — {{.Author}}
</a>
{{end}}
{{end}}
```

Each suggestion is an anchor tag linking directly to the book detail page. The user can click a suggestion to navigate, or keep typing to refine. The `.` in `{{range .}}` is the slice of `*searchv1.Suggestion` messages passed from the handler.

---

## The Data Flow

Here is the complete path of a search query, from keypress to rendered suggestions:

```
1. User types "gol" in the nav bar search input
2. HTMX waits 300ms (debounce), then sends GET /search/suggest?prefix=gol
3. Gateway handler calls search.Suggest(prefix="gol", limit=5) via gRPC
4. Search service delegates to MeilisearchIndex.Suggest("gol", 5)
5. Meilisearch returns matching documents (e.g., "Go in Action", "Golang Patterns")
6. Search service maps documents to Suggestion structs
7. gRPC returns SuggestResponse to the gateway
8. Gateway renders suggestions.html partial with the suggestions
9. HTMX replaces #suggestions div innerHTML with the HTML fragment
10. User sees a dropdown with clickable book links
```

The entire round trip—gateway to Search Service to Meilisearch and back—typically completes in under 50 ms. The 300 ms debounce is the dominant latency from the user's perspective.

---

## Eventual Consistency in the UI

There is an important user experience consideration: the search index is eventually consistent with the catalog. When an admin creates a new book, the sequence is:

1. Catalog writes to PostgreSQL (immediate).
2. Catalog publishes `book.created` to Kafka (milliseconds).
3. Search consumer receives the event and upserts into Meilisearch (milliseconds to seconds).
4. Meilisearch processes the task and makes the document searchable (milliseconds).

In total, there is a window of roughly one to five seconds where the book exists in the catalog but is not yet searchable. This is rarely noticeable in practice, but it can be confusing during development when you create a book and immediately search for it.

The gateway handles this gracefully: the catalog browse page (`/books`) always reads from PostgreSQL through the Catalog Service, so new books appear there immediately. The search page reads from Meilisearch, so it reflects the slightly-delayed index. Both views are correct for their data source.

---

## Exercises

1. **Add pagination to search results.** The handler already reads a `page` parameter. Add "Previous" and "Next" links below the results table that link to `/search?q=...&page=2` etc. Use the `TotalHits` value to calculate whether a next page exists.

2. **Clear suggestions on blur.** Currently, clicking outside the suggestions dropdown does not hide it. Add a small inline script or HTMX attribute that clears the `#suggestions` div when the input loses focus. Consider the timing: if the user clicks a suggestion link, the blur fires before the click—how do you handle that?

3. **Add keyboard navigation to suggestions.** Using HTMX's `hx-on` attribute or a small script, let the user press arrow keys to highlight suggestions and Enter to navigate to the selected one. This is a common autocomplete UX pattern.

4. **Display Meilisearch query time.** The search response includes `QueryTimeMs`. Show it in the results header (e.g., "42 results in 3 ms"). This is already partially implemented in the template—verify it works end-to-end.

5. **Compare HTMX autocomplete to a React implementation.** Sketch (on paper or in pseudocode) how you would implement the same autocomplete with React: a `useState` hook for the input, a `useEffect` with debounce for the fetch, a JSON response, and a component to render suggestions. Compare the amount of code, the number of concepts involved, and where the rendering happens (client vs. server).

---

## References

[^1]: [HTMX documentation—Triggers](https://htmx.org/docs/#triggers)—Full reference for `hx-trigger` syntax, including debounce, conditions, and event filters.
[^2]: [HTMX documentation—Swapping](https://htmx.org/docs/#swapping)—How `hx-swap` controls where and how the server response is inserted into the DOM.
[^3]: [Go html/template package](https://pkg.go.dev/html/template)—Reference for Go's template engine, including context-aware escaping and the `{{range}}` action.
[^4]: [HTMX—Active Search pattern](https://htmx.org/examples/active-search/)—The official HTMX example for live search, which our implementation closely follows.
[^5]: [Carson Gross—Hypermedia Systems](https://hypermedia.systems/)—The book by the HTMX author that explains the philosophy behind server-rendered HTML with hypermedia controls, and why it is a viable alternative to JSON APIs + SPAs.
