# 7.4 Reservation UI

<!-- [STRUCTURAL] Section scope: gateway routes → reserve flow → list flow → return flow → error handling → eventual consistency note → manual testing → exercises. Clean, predictable. Matches the "apply patterns we already know" framing. Keep. -->

The reservation UI extends the gateway (our BFF from Chapter 5) with three new capabilities: reserving a book, listing a user's reservations, and returning a book. No new architectural concepts -- just applying the patterns we already know to a new feature.

<!-- [LINE EDIT] Cut "just". "No new architectural concepts -- just applying the patterns we already know to a new feature." → "No new architectural concepts — we apply the patterns we already know to a new feature." -->
<!-- [COPY EDIT] "BFF" — expand on first use: "Backend for Frontend (BFF)". Readers who skipped Chapter 5 need the expansion. (CMOS 10.3) -->

---

## New Routes

The gateway registers three reservation-related routes:

```go
// services/gateway/cmd/main.go

mux.HandleFunc("POST /books/{id}/reserve", srv.ReserveBook)
mux.HandleFunc("GET /reservations", srv.MyReservations)
mux.HandleFunc("POST /reservations/{id}/return", srv.ReturnBook)
```

<!-- [COPY EDIT] The net/http method-pattern syntax (`"POST /books/{id}/reserve"`) is a Go 1.22+ feature. Consider noting this explicitly the first time the syntax appears — readers on older Go versions will be confused. If it was explained in Chapter 5, a cross-reference is fine: "See Chapter 5 for the Go 1.22+ method-pattern syntax." -->

All three require authentication. There are no `GET` forms for reserve or return -- these are actions triggered by buttons, not pages with their own URL. The `POST` handlers do their work and redirect.

<!-- [LINE EDIT] "There are no GET forms for reserve or return" → "There are no GET endpoints for reserve or return" — "Forms" is ambiguous (HTML `<form>` vs. "versions"). -->
<!-- [COPY EDIT] "not pages with their own URL" — "URL" should be plural to match "pages": "not pages with their own URLs." -->

The gateway also needs a new gRPC client. In `main.go`, the reservation service connection is set up alongside the catalog and auth connections:

```go
reservationConn, err := grpc.NewClient(reservationAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
// ...
reservationClient := reservationv1.NewReservationServiceClient(reservationConn)

srv := handler.New(authClient, catalogClient, reservationClient, searchClient, tmpl)
```

The `Server` struct now holds four gRPC clients:

```go
type Server struct {
    auth        authv1.AuthServiceClient
    catalog     catalogv1.CatalogServiceClient
    reservation reservationv1.ReservationServiceClient
    search      searchv1.SearchServiceClient
    tmpl        map[string]*template.Template
    baseTmpl    *template.Template
}
```

As the system grows, so does the gateway's dependency list. This is a characteristic of the BFF pattern -- it is the aggregation point for all backend services. If this gets unwieldy (10+ clients), it might be time to split the gateway into multiple BFFs or introduce an API gateway layer. For four clients, it is fine.

<!-- [STRUCTURAL] Good architectural callout tied to the struct definition. Keep. -->
<!-- [COPY EDIT] "10+ clients" — in formal prose, "ten or more clients" (CMOS 9.2). Technical acceptable as-is. House call. -->

---

## Reserve a Book

The reserve action lives on the book detail page. When a logged-in user views a book with available copies, they see a "Reserve This Book" button:

```html
<!-- services/gateway/templates/book.html -->

{{if and .User (gt .Data.AvailableCopies 0)}}
<form method="POST" action="/books/{{.Data.Id}}/reserve">
    <button type="submit">Reserve This Book</button>
</form>
{{end}}
```

The `{{if and .User (gt .Data.AvailableCopies 0)}}` template condition checks two things: the user is logged in (`.User` is non-nil) and the book has available copies. If either condition fails, the button does not render. This is a UI-level guard -- the backend enforces the same rules independently.

<!-- [STRUCTURAL] "The backend enforces the same rules independently" — good reminder that client checks are UX, not security. Keep. -->

The handler is minimal:

```go
// services/gateway/internal/handler/reservation.go

func (s *Server) ReserveBook(w http.ResponseWriter, r *http.Request) {
    if !s.requireAuth(w, r) {
        return
    }

    bookID := r.PathValue("id")
    _, err := s.reservation.CreateReservation(r.Context(), &reservationv1.CreateReservationRequest{
        BookId: bookID,
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to reserve book")
        return
    }

    setFlash(w, "Book reserved successfully")
    http.Redirect(w, r, "/reservations", http.StatusSeeOther)
}
```

The pattern is: authenticate, extract path parameters, call gRPC, handle errors, set a flash message, redirect. This is the same pattern as every other mutation in the gateway.

<!-- [LINE EDIT] "This is the same pattern as every other mutation in the gateway." → "Every mutation in the gateway follows this shape." — More direct. Optional. -->

`r.PathValue("id")` extracts the `{id}` segment from the URL (Go 1.22+). The value is passed directly to the gRPC request as a string -- UUID validation happens in the reservation service's handler layer, not in the gateway. The gateway's job is to relay, not to validate business data.

<!-- [STRUCTURAL] Good delineation of responsibilities. Keep. -->

`http.StatusSeeOther` (303) is the correct status code for a POST-redirect-GET flow. It tells the browser to follow the redirect with a GET request, preventing the "resubmit form?" dialog if the user refreshes.

<!-- [COPY EDIT] "POST-redirect-GET" — hyphenated as a compound, industry-standard acronym (sometimes PRG). Correct. -->

### Flash Messages

`setFlash` stores a success message in a short-lived cookie:

```go
func setFlash(w http.ResponseWriter, message string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "flash",
        Value:    message,
        Path:     "/",
        MaxAge:   10,
        HttpOnly: true,
    })
}
```

The `MaxAge: 10` means the cookie expires after 10 seconds -- long enough for the redirect to complete and the next page to read it, short enough that it does not linger. The `consumeFlash` function reads the cookie and immediately deletes it (by setting `MaxAge: -1`), ensuring each flash message is shown exactly once.

<!-- [COPY EDIT] "MaxAge: 10 means the cookie expires after 10 seconds" — the value is in seconds per the net/http.Cookie docs. Correct. Consider inlining the unit in the explanation for readers who don't know: "`MaxAge: 10` (seconds) means the cookie expires after 10 seconds..." Minor. -->
<!-- [STRUCTURAL] The flash cookie implementation lacks signing — a user can craft arbitrary flash messages by setting the cookie. For a learning-project caveat, this is fine but worth acknowledging briefly. Consider: "Note: this cookie is unsigned, so a determined user could set their own flash message. For a learning project that is acceptable; in production you would sign or encrypt the value." -->
<!-- [COPY EDIT] Please verify: the cookie has no `Secure` attribute. For HTTPS-only production, `Secure: true` is required. Consider noting this for completeness. -->
<!-- [COPY EDIT] "SameSite" is not set. Default in modern Go is `SameSiteDefaultMode` (no attribute emitted) which varies by browser. Consider explicitly setting `SameSite: http.SameSiteLaxMode` for CSRF defense-in-depth (which exercise 5 addresses anyway). -->

This is a standard server-rendered application pattern. In Spring MVC, the equivalent is `RedirectAttributes.addFlashAttribute()`, which uses the session instead of a cookie.

<!-- [STRUCTURAL] Good Java analogy. Keep. -->

---

## List Reservations

The "My Reservations" page shows all of a user's reservations:

```go
func (s *Server) MyReservations(w http.ResponseWriter, r *http.Request) {
    if !s.requireAuth(w, r) {
        return
    }

    resp, err := s.reservation.ListUserReservations(r.Context(),
        &reservationv1.ListUserReservationsRequest{})
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to load reservations")
        return
    }

    s.render(w, r, "reservations.html", map[string]any{
        "Reservations": resp.Reservations,
    })
}
```

The handler calls the reservation service, gets back a list of protobuf `Reservation` messages, and passes them to the template. Note that the template receives protobuf types directly -- there is no conversion to a gateway-specific view model. This is a simplification. In a larger system, you might define gateway-specific structs to decouple the template from the protobuf schema.

<!-- [STRUCTURAL] Honest "simplification" callout — good. Keep. -->

The template renders the list as an HTML table:

```html
<!-- services/gateway/templates/reservations.html -->

{{define "content"}}
<h1>My Reservations</h1>
{{if not .Data.Reservations}}
    <p>You have no reservations.</p>
{{else}}
    <table>
        <thead>
            <tr>
                <th>Book ID</th>
                <th>Status</th>
                <th>Reserved</th>
                <th>Due</th>
                <th>Action</th>
            </tr>
        </thead>
        <tbody>
            {{range .Data.Reservations}}
            <tr>
                <td><a href="/books/{{.BookId}}">{{.BookId}}</a></td>
                <td>{{.Status}}</td>
                <td>{{.ReservedAt.AsTime.Format "2006-01-02"}}</td>
                <td>{{.DueAt.AsTime.Format "2006-01-02"}}</td>
                <td>
                    {{if eq .Status "active"}}
                    <form method="POST" action="/reservations/{{.Id}}/return"
                          style="display:inline">
                        <button type="submit">Return</button>
                    </form>
                    {{else}}
                        {{.Status}}
                    {{end}}
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
{{end}}
{{end}}
```

A few things to notice:

**`.ReservedAt.AsTime.Format "2006-01-02"`** -- The protobuf `Timestamp` type has an `AsTime()` method that returns a Go `time.Time`. We then call `Format` with Go's reference time layout. The string `"2006-01-02"` is not arbitrary -- it is Go's reference date (January 2, 2006). Where other languages use `yyyy-MM-dd`, Go uses a specific moment in time as the format specification. This is one of Go's most surprising design choices for newcomers.

<!-- [STRUCTURAL] Great teaching aside for a Java/Kotlin reader who will instinctively reach for `yyyy-MM-dd`. Keep. -->
<!-- [COPY EDIT] "Go's reference date (January 2, 2006)" — worth completing: the full reference time is `Mon Jan 2 15:04:05 MST 2006` (mnemonic: 1, 2, 3, 4, 5, 6, 7 for month/day/hour/minute/second/year/timezone offset = -0700). A one-line mnemonic would aid retention. -->

**Conditional return button.** The "Return" button only appears for active reservations. Returned and expired reservations display their status as plain text. This mirrors the backend rule that only active reservations can be returned.

<!-- [LINE EDIT] "only appears for active reservations" — placement of "only"; technically correct here because "only" modifies the verb phrase. Not worth changing. -->

**Book ID as link.** The book ID links to the book detail page. In a polished UI, you would display the book title instead of the raw UUID. That would require either embedding the book title in the reservation (denormalization) or making an additional gRPC call per reservation to fetch book details (N+1 query). For a learning project, the UUID link is fine.

<!-- [COPY EDIT] "N+1 query" — standard term, no change needed. -->

---

## Return a Book

The return handler follows the same pattern as reserve:

```go
func (s *Server) ReturnBook(w http.ResponseWriter, r *http.Request) {
    if !s.requireAuth(w, r) {
        return
    }

    resID := r.PathValue("id")
    _, err := s.reservation.ReturnBook(r.Context(), &reservationv1.ReturnBookRequest{
        ReservationId: resID,
    })
    if err != nil {
        s.handleGRPCError(w, r, err, "Failed to return book")
        return
    }

    setFlash(w, "Book returned successfully")
    http.Redirect(w, r, "/reservations", http.StatusSeeOther)
}
```

Authenticate, extract path parameter, call gRPC, handle errors, flash, redirect. The reservation service handles all validation (ownership, status checks). The gateway just relays.

<!-- [LINE EDIT] Cut "just". "The gateway just relays." → "The gateway relays." -->

---

## Error Handling in the Gateway

The `handleGRPCError` function maps gRPC status codes to HTTP responses:

```go
func (s *Server) handleGRPCError(w http.ResponseWriter, r *http.Request, err error, fallbackMsg string) {
    st, ok := status.FromError(err)
    if !ok {
        s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
        return
    }
    switch st.Code() {
    case codes.NotFound:
        s.renderError(w, r, http.StatusNotFound, "Not found")
    case codes.ResourceExhausted:
        s.renderError(w, r, http.StatusTooManyRequests,
            "You have reached the maximum number of active reservations")
    case codes.FailedPrecondition:
        s.renderError(w, r, http.StatusPreconditionFailed, st.Message())
    case codes.PermissionDenied:
        s.renderError(w, r, http.StatusForbidden, "Access denied")
    // ... other cases
    }
}
```

This is the translation layer between the backend's error domain (gRPC status codes) and the frontend's error domain (HTTP status codes with human-readable messages). The reservation service returns `codes.ResourceExhausted` when the user has too many active reservations -- the gateway turns that into a 429 with a friendly message. The reservation service returns `codes.FailedPrecondition` when the book has no available copies or the reservation is already returned -- the gateway passes through the service's message.

<!-- [COPY EDIT] "human-readable" — hyphenated compound modifier (CMOS 7.81). Correct. -->
<!-- [COPY EDIT] "the user has too many active reservations" — consistent with the ResourceExhausted mapping in 7.2. Good. -->
<!-- [STRUCTURAL] Note that 7.2 also states FailedPrecondition "becomes 412". This snippet confirms that — `http.StatusPreconditionFailed` is 412. Consistent. -->

This mapping exists in one place and is shared across all handlers. If you add a new error case in a backend service, you add one line here.

<!-- [STRUCTURAL] "DRY error mapping" is worth noting — the payoff of centralization is one-line changes. Keep. -->

---

## Eventual Consistency in the UI

There is a subtle UX issue. When a user reserves a book:

1. The reservation is created (instant).
2. The user is redirected to `/reservations`.
3. The `reservation.created` event is published to Kafka.
4. The catalog consumer processes the event and decrements `available_copies`.

<!-- [STRUCTURAL] CRITICAL inconsistency with 7.2. Section 7.2 establishes that `available_copies` is decremented *synchronously* (via `catalog.UpdateAvailability(-1)`) *before* the reservation row is created — that is the whole point of the TOCTOU fix. In that design, there is no eventual-consistency window on create: by the time the reservation row exists, the catalog count is already decremented. The four-step flow above (and the "stale read between steps 2 and 4" example) only holds if the decrement happens through Kafka, which contradicts 7.2. Options: (a) rewrite this section to describe the *return* or *expiration* flow (which IS eventual-consistency via Kafka) as the motivating example; (b) remove the section; (c) reconcile by describing the real flow where the *reservation list page* could briefly show a reservation before the consumer has processed an expire event for it. This is the single most important edit in 7.4. -->

If the user navigates back to the book detail page between steps 2 and 4, they might see `Available: 3 / 3` even though they just reserved a copy. The availability update has not happened yet.

<!-- [STRUCTURAL] This specific example is incorrect given 7.2's design. If decrement is synchronous, the count decreases before step 2 completes, so the user would see `Available: 2 / 3`. A correct analogous example would be: "If the user returns a book (which publishes `reservation.returned` async) and navigates back to the book detail page, they might briefly see the old (lower) availability count until the consumer processes the event." -->

This is eventual consistency at work. The data will converge -- usually within milliseconds to seconds -- but there is a window where the UI shows stale data. For a library system, this is acceptable. For a financial trading system, it would not be.

There are several ways to handle this in the UI, if needed:

- **Optimistic updates.** The gateway could subtract 1 from the displayed count after a successful reservation, without waiting for the event to process.
- **Cache invalidation.** The reservation service could notify the gateway to invalidate its cache for that book (if the gateway had a cache).
- **Polling.** The book detail page could poll for updated availability.

<!-- [STRUCTURAL] The three remediations all assume an async-on-create design. If the section is rewritten to focus on return/expire flows, these mitigations still apply (with minor wording tweaks). Keep the list, just update the anchoring scenario. -->

We do none of these. The simplest approach is often the right one: accept the brief inconsistency and let the system converge naturally.

<!-- [LINE EDIT] "We do none of these." → "We do none of these — accept the brief inconsistency and let the system converge." — Joins the two short sentences. Optional. -->

---

## Testing the Full Flow

To test the complete reserve-and-return flow locally, you need all the infrastructure running: PostgreSQL (for both services), Kafka, the catalog service, the reservation service, and the gateway. Docker Compose handles this.

<!-- [COPY EDIT] "PostgreSQL (for both services)" — accurate; the catalog and reservation services each have their own DB. Consider clarifying "two PostgreSQL databases, one per service" for readers who might assume a shared schema. -->
<!-- [STRUCTURAL] The index.md advertises this section as "Docker setup" but there is no actual Docker Compose walkthrough here — only a passing mention. Either add a mini-walkthrough (compose up, verify logs, hit the URL) or update the index.md bullet to "manual end-to-end testing". Cross-file consistency. -->

A manual test sequence:

1. Register a user and log in through the gateway.
2. Navigate to the book catalog and pick a book with available copies.
3. Click "Reserve This Book."
4. Verify you are redirected to `/reservations` and see the new reservation.
5. Navigate back to the book detail page and verify the availability count decreased.
6. On the reservations page, click "Return."
7. Navigate to the book detail page and verify the count increased.

<!-- [COPY EDIT] Period inside quotation marks: 'Click "Reserve This Book."' — correct per CMOS 6.9. Good. -->
<!-- [STRUCTURAL] Step 5 "verify the availability count decreased" — given 7.2's synchronous decrement, this should be immediate. Step 7 "verify the count increased" is the eventual-consistency step (Kafka consumer increments on `reservation.returned`). Consider annotating the steps: "5. Navigate back to the book detail page — the availability count decreased immediately (synchronous decrement). 6. … 7. Navigate to the book detail page — the count increased, usually within a second (async increment via Kafka consumer)." That would also reinforce the earlier Eventual Consistency discussion and correct its anchoring scenario. -->

If step 5 still shows the old count, wait a second and refresh. The Kafka consumer may not have processed the event yet. If it *never* updates, check the catalog service logs for consumer errors.

<!-- [STRUCTURAL] This caveat is wrong for step 5 given 7.2's design: step 5 is synchronous, so a stale count at step 5 is a *bug*, not an eventual-consistency artifact. The caveat would apply correctly to step 7 (the async increment). Move/rewrite. -->

---

## Exercises

1. **Add a "Reserve" button to the catalog list.** Currently, the reserve button only appears on the book detail page. Add it to each row in the book list page (`books.html`), but only for logged-in users and books with available copies. Consider the UX tradeoffs -- is it better to have the button on the list, or force users to view the detail page first?

<!-- [COPY EDIT] "currently" — mildly weak opener; "The reserve button only appears on the book detail page today. Add it..." Optional. -->

2. **Display book title in reservations.** The reservations table shows the book UUID, which is not user-friendly. Modify `MyReservations` to fetch book details for each reservation and pass the titles to the template. Consider the performance implications (N+1 gRPC calls). How would you batch this?

<!-- [STRUCTURAL] Good exercise — pulls the reader toward designing a batch RPC. Consider prompting for the proto shape: "Hint: what would a `BatchGetBooks(ids []string)` RPC look like?" -->

3. **Extend reservation from the UI.** Add a POST route `POST /reservations/{id}/extend` and an "Extend" button on the reservations page (only for active reservations). This requires the `ExtendReservation` RPC to exist in the reservation service (see exercise 1 in section 7.2).

4. **Error message UX.** Try to reserve a book when you already have the maximum number of active reservations. What error page do you see? Modify the flow so that instead of showing an error page, the user is redirected back to the book detail page with a flash message explaining the problem.

<!-- [STRUCTURAL] Good concrete UX exercise. Keep. -->

5. **CSRF protection.** Our POST forms have no CSRF tokens. Explain why this is a security risk. Describe how you would add CSRF protection: where would the token be generated, how would it be embedded in the form, and where would it be validated? (Hint: the `gorilla/csrf` package is a common choice, but you can also implement it with the stdlib.)

<!-- [COPY EDIT] "gorilla/csrf" — the gorilla project was archived in 2022 but has since resumed maintenance (2023+) under a new team. Please verify the current status and consider also suggesting `github.com/justinas/nosurf` as an alternative. -->
<!-- [COPY EDIT] "stdlib" — informal. Spell as "standard library" in prose. -->

---

## References

[^1]: [Go html/template package](https://pkg.go.dev/html/template) -- Template engine documentation, including the auto-escaping security model.
[^2]: [Go time.Format reference date](https://pkg.go.dev/time#pkg-constants) -- Explanation of Go's reference time `Mon Jan 2 15:04:05 MST 2006`.
[^3]: [MDN -- HTTP 303 See Other](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/303) -- When and why to use 303 for POST-redirect-GET.
[^4]: [OWASP -- Cross-Site Request Forgery](https://owasp.org/www-community/attacks/csrf) -- CSRF attack description and prevention techniques.
[^5]: [Sam Newman -- Building Microservices, Chapter 13](https://samnewman.io/books/building_microservices_2nd_edition/) -- Discussion of eventual consistency in user interfaces.

<!-- [COPY EDIT] Please verify: URL https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/303 — MDN previously used `/Web/HTTP/Status/303` (no `Reference/` segment). Confirm the canonical path under MDN's current IA. -->
<!-- [COPY EDIT] " -- " in reference entries — standardize to em dash or period. (CMOS 6.85) -->
<!-- [FINAL] Verify file-path comments: `services/gateway/cmd/main.go`, `services/gateway/internal/handler/reservation.go`, `services/gateway/templates/book.html`, `services/gateway/templates/reservations.html` — all against current repo layout. -->
