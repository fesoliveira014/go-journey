# Changelog: admin-dashboard.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Longest and most complex section in the chapter; covers two proto changes, repo/service/handler implementations on two services, gateway handlers, and templates. Consider a short "map" (diagram or summary table) at the top listing what gets added where.
  - N+1 discussion is excellent teaching — you show the naive version, name it, justify it at this scale, and point to Chapter 7 for the proper fix.
  - Templates are claimed ("renders a table with email, name, role, and join date") but only the landing page is shown. Consider including 8–10 lines of `admin_users.html` so the reader can verify the binding.
  - The shape of the section (proto → repo → service → handler twice) is repetitive — minor restructuring could factor the second pass into "same shape, these diffs".

## Pass 2: Line Editing
- **Line ~3:** tighten contrastive pair
  - Before: "The admin can already manage books through the CRUD pages built in Chapter 5. But two questions remain unanswered: who are the users of the system, and what reservations exist?"
  - After: "The admin can already manage books through the CRUD pages built in Chapter 5, but two questions remain unanswered: who are the users of the system, and what reservations exist?"
  - Reason: contrastive clause; single sentence reads cleaner.
- **Line ~191:** tighten
  - Before: "This requires a corresponding update to `deploy/docker-compose.yml` to pass the auth service address to the reservation container:"
  - After: "Passing the address requires a matching update to `deploy/docker-compose.yml`:"
  - Reason: removes "corresponding" scaffold; active voice.
- **Line ~239:** remove filler
  - Before: "The admin dashboard still works; it just shows less-readable data."
  - After: "The admin dashboard still works; it shows less-readable data."
  - Reason: "just" is in the cut-list.

## Pass 3: Copy Editing
- **Lines ~40, ~47:** ASCII arrow `->` → Unicode `→`. Rationale: `->` reads as a code token mid-prose (Go channel receive, Rust arrow function, C `->` pointer access).
- **Chapter-wide:** `--` → `—` em dash, no surrounding spaces. CMOS 6.85.
- **Query — line ~395:** please verify that the reservation service is exposed on port 50053. `seed-cli.md` and `putting-it-together.md` consistently reference ports 50051 (auth) and 50052 (catalog); 50053 appears only in this section's `grpcurl` example. Confirm against `deploy/docker-compose.yml`.
- **Line ~109:** please verify the forward reference to "the gateway's `requireAdmin` helper from Chapter 5" — confirm Chapter 5 covers the helper and consider linking explicitly.
- **Line ~247:** please verify Chapter 7 covers the catalog/auth → reservation denormalization via Kafka events (the chapter is promised here).
- **Line ~233:** "**N+1**" — industry-standard spelling without spaces is fine; keep consistent across chapter.
- Multiple serial-comma lists (lines ~26, ~69, ~162, ~348, ~364) — all correct per CMOS 6.19.
- Multiple compound adjectives ("role-based", "human-readable", "three-step", "less-readable", "event-driven", "cross-service") — all hyphenated correctly before the noun (CMOS 7.89).

## Pass 4: Final Polish
- No typos, doubled words, or missing words found.
- No broken cross-references detected within this file; confirm Chapter 5 (`requireAdmin`) and Chapter 7 (event-driven sync) forward-refs once adjacent chapters are reviewed.
- Port-number inconsistency (50053 only here) is the single most important factual check before the chapter ships.
