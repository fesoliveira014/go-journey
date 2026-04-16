<!-- [STRUCTURAL] Good opening hook: names three concrete problems, then maps each to a subsection. Keeps the chapter framing tight. Consider also telegraphing what the reader will have accomplished by chapter's end (a running, populated system usable for the rest of the book) so the motivation lands even harder. -->
# Chapter 6: Admin & Developer Tooling

<!-- [STRUCTURAL] Strong motivation paragraph, anchored in the "chicken-and-egg" image. Works well. -->
<!-- [LINE EDIT] "it all in the browser. But if you try" → "it all in the browser. If you try" (drop the "But" — the contrast is already implied by "But if you try to use it from scratch, you hit…" is fine, but the sentence reads cleaner without the conjunction). Leave as author's call. -->
At this point we have a working library system: authentication, a book catalog with CRUD, reservations, and a gateway that renders it all in the browser. But if you try to use it from scratch, you hit a chicken-and-egg problem:

<!-- [COPY EDIT] CMOS 6.85: prose uses two ASCII hyphens `--` for em dashes throughout the chapter. Standard practice in published work is the Unicode em dash (—) with no surrounding spaces. The author has already used a Unicode em dash once (inside the AutoMigrate blockquote in admin-cli.md, line 88), so the inconsistency is worth resolving chapter-wide. Recommend global pass: `--` → `—`. -->
- **No admin accounts.** The `POST /admin/books` route requires an admin user, but `Register` always creates users with the `"user"` role. There is no way to promote a user to admin through the UI or the gRPC API.
- **No books.** Even if you had an admin, the catalog is empty. Manually filling in the "Add Book" form 16 times is tedious and error-prone.
<!-- [COPY EDIT] CMOS 9.2: "16 times" — numerals for countable repetitions of a specific operation is acceptable; consistent with later "16 books". Keep. -->
- **No visibility.** An admin can manage books, but cannot see who is registered or what reservations exist across the system.
<!-- [COPY EDIT] CMOS 6.19 (serial comma): "who is registered or what reservations exist" — only two items joined by "or", no serial comma needed. Fine. -->

This chapter builds three tools that solve these problems and make the system usable for development:
<!-- [STRUCTURAL] Minor: you say "three tools" here, but the list immediately below has four items (the fourth is a walkthrough, not a tool). Consider "This chapter builds three tools — and then walks through them end-to-end:" to reconcile the count with the outline. -->
<!-- [LINE EDIT] "This chapter builds three tools that solve these problems and make the system usable for development:" → "This chapter builds three tools that solve these problems and make the system usable for development — plus an end-to-end walkthrough to tie them together:" -->

---

## What We Build
<!-- [COPY EDIT] CMOS 8.159 (headline-style capitalization): "What We Build" is fine. Consistent with the rest of the chapter's H2s. -->

<!-- [LINE EDIT] "A standalone Go binary that connects directly to the auth database and creates (or promotes) an admin account." — tight, good. -->
1. **Admin CLI** (Section 6.1) -- A standalone Go binary that connects directly to the auth database and creates (or promotes) an admin account. This bypasses the gRPC layer intentionally: it is a bootstrapping tool, not a feature.
<!-- [COPY EDIT] CMOS 6.85: em dash — "CLI** (Section 6.1) -- A" → "CLI** (Section 6.1) — A". Apply to items 2–4 as well. -->
<!-- [COPY EDIT] "CLI" — first use in this chapter. Expand on first use ("command-line interface (CLI)") since the chapter may be read standalone. If CLI was expanded earlier in the book, a cross-reference footnote is sufficient. -->

2. **Admin Dashboard** (Section 6.2) -- New gRPC RPCs (`ListUsers` on auth, `ListAllReservations` on reservation) and three gateway pages that display users and reservations to admins. This involves adding a new proto message (`ReservationDetail`) that denormalizes book titles and user emails into a single response.
<!-- [LINE EDIT] "This involves adding a new proto message (`ReservationDetail`) that denormalizes book titles and user emails into a single response." → "We also add a new proto message, `ReservationDetail`, that denormalizes book titles and user emails into a single response." Reason: active voice; removes "This involves adding" as a hollow pivot. -->

3. **Catalog Seed CLI** (Section 6.3) -- A Go binary that logs in as an admin via gRPC, reads a JSON fixture file, and creates books through the catalog service's `CreateBook` RPC. Unlike the admin CLI, this exercises the full stack: authentication, authorization, validation, and (when configured) Kafka event publishing.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "authentication, authorization, validation, and (when configured) Kafka event publishing." — correct. -->

4. **Putting It Together** (Section 6.4) -- An end-to-end walkthrough: start the stack, create an admin, seed the catalog, register a user, make a reservation, and verify everything in the admin dashboard.
<!-- [COPY EDIT] CMOS 7.89: "end-to-end" is a compound adjective before "walkthrough"; hyphenation correct. -->

---

## Section Outline
<!-- [STRUCTURAL] The table duplicates the numbered list above. Having both is reader-friendly for scanning (list for narrative, table for reference), but consider framing the table explicitly: "For quick reference, here is the section outline with the key source files you'll touch:" -->

| Section | Topic | Key Files |
|---------|-------|-----------|
| [6.1 Admin CLI](admin-cli.md) | Bootstrapping admin accounts via direct DB access | `services/auth/cmd/admin/main.go` |
<!-- [COPY EDIT] "DB" — abbreviation for "database". CMOS 10.3: spell out on first use unless the abbreviation is more recognizable. "DB" is ubiquitous in engineering prose; acceptable. Consistency: the body text uses both "database" and "DB" — pick one style. -->
| [6.2 Admin Dashboard](admin-dashboard.md) | New RPCs, denormalization, gateway pages | `proto/auth/v1/auth.proto`, `proto/reservation/v1/reservation.proto`, `services/gateway/internal/handler/admin.go` |
| [6.3 Catalog Seed CLI](seed-cli.md) | Populating the catalog through gRPC | `services/catalog/cmd/seed/main.go`, `services/catalog/cmd/seed/books.json` |
| [6.4 Putting It Together](putting-it-together.md) | End-to-end verification | All of the above |
<!-- [FINAL] No typos, doubled words, or broken cross-refs found. Relative links resolve to sibling files in the same directory. -->
