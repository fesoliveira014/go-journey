# Changelog: putting-it-together.md

## Pass 1: Structural / Developmental
- 3 comments. Themes:
  - Capstone section is well-shaped: numbered, prescriptive, each step ends with a verifiable outcome. Pacing is excellent.
  - "Wait for all services to be healthy" — readers unfamiliar with Docker Compose health checks may not know what to look for. A one-line aside pointing at `docker compose ps` or a specific log line would help.
  - "But the services are mostly isolated" — reads oppositely to author's intent; the services are in fact tightly coupled via synchronous gRPC. This is the weakest sentence in an otherwise strong close.
  - Forward reference to Chapter 7 is well-calibrated: names what's coming without overselling.

## Pass 2: Line Editing
- **Line ~76:** remove filler and tighten
  - Before: "You should see a table with at least one row: the admin account you just created, with role `admin`."
  - After: "You should see a table with one row: the admin account you just created, with role `admin`."
  - Reason: "just" is filler; "at least one" hedges when there is exactly one.
- **Line ~113:** correct the semantic framing
  - Before: "But the services are mostly isolated -- the reservation service calls catalog and auth synchronously via gRPC, and the search service is not yet connected."
  - After: "But the services are tightly coupled — the reservation service calls catalog and auth synchronously via gRPC, and the search service is not yet connected."
  - Reason: "isolated" reads as the opposite of the author's point (the problem is coupling, not isolation).

## Pass 3: Copy Editing
- **Chapter-wide:** `--` → `—` em dash, no surrounding spaces. CMOS 6.85.
- **Line ~13:** "Postgres" vs. "PostgreSQL" — chapter mixes both forms. Recommend "PostgreSQL" on first mention per chapter, "Postgres" thereafter; or standardize on one.
- Port numbers (50051, 50052, 5434) consistent with seed-cli.md and admin-cli.md; no conflicts in this file.
- Multiple serial-comma lists throughout (steps 1, 5, 7, "What's Next") — all correct per CMOS 6.19.
- "end-to-end" (line 3), "event-driven" (line 115), "Log In" as phrasal verb (step 4), "Log out" (step 7) — all correctly styled.
- CMOS 6.43: "(e.g., *Dune*)" — correct.
- CMOS 8.171: book titles in italics — correct throughout.

## Pass 4: Final Polish
- No typos, doubled words, or missing words found.
- No broken cross-references within this file. Forward reference to Chapter 7 is appropriate.
- Title rendering: "*Dune*" appears in both italic (prose, line 90, 102) and non-italic (nowhere in this file) forms — consistent here. (Cross-file: seed-cli.md also italicizes "*Dune*"; consistent.)
