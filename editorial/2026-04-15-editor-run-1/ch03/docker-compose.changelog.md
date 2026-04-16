# Changelog: docker-compose.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Strong rhetorical structure (motivate with tedious manual flow → one-liner → YAML dissection).
  - Consider promoting "Port Mapping" from a subsection of Networking to its own top-level section, since it's a distinct concern from inter-container networking.
  - Suggest one clarifying line near the dual `volumes:` YAML snippet to keep less-experienced YAML readers from conflating the top-level declaration with the per-service reference.

## Pass 2: Line Editing
- **Line ~40:** Fix parallelism.
  - Before: "That is six commands, error-prone, and doesn't handle startup ordering."
  - After: "That's six error-prone commands with no startup ordering."
  - Reason: items weren't parallel (count vs. adjective vs. verb phrase); also corrects the count (see Pass 3 query).
- **Line ~108:** Tighten.
  - Before: "Let's walk through each section."
  - After: "Walk through it:" (optional)
- **Line ~132:** Active voice.
  - Before: "its value is used"
  - After: "Compose uses its value"
- **Line ~237:** Drop intensifier.
  - Before: "To completely reset the database"
  - After: "To reset the database"
- **Line ~279:** More directive.
  - Before: "You should see PostgreSQL's healthcheck pass..."
  - After: "Expect to see PostgreSQL's healthcheck pass..."
- **Line ~302:** Drop filler.
  - Before: "Now stop with `docker compose down -v`..."
  - After: "Stop with `docker compose down -v`..."
- **Line ~307:** Fix awkward list construction in solution.
  - Before: "...with a generated `id`, `created_at`, and `updated_at` timestamp."
  - After: "...with a generated `id` and `created_at`/`updated_at` timestamps."
  - Reason: `id` isn't a timestamp; the singular "timestamp" at the end attached to a mixed list.

## Pass 3: Copy Editing
- **Throughout:** Replace `--` with em dash `—` (no spaces) per CMOS 6.85.
- **Line ~40:** Command count — recount: `network create`, `run postgres`, `run catalog`, `run gateway` = 4 commands (the "wait" line is a comment). Prose claims "six commands." Either recount or reconcile.
- **Line ~121:** "pre-built" — CMOS 7.89 accepts "prebuilt" (closed). Keep consistent chapter-wide.
- **Line ~158 area:** "5 seconds" / "every 5 seconds" — numerals for technical measurements (CMOS 9.16). Correct.
- **Line ~160:** Quoted state words "healthy." / "unhealthy." — periods inside double quotes (CMOS 6.9). Correct.
- **Line ~209 and ~332:** "inter-container" / "inter-module" — style consistency across chapter.
- **Queries (please verify):**
  - `postgres:16-alpine` — version pin currency (PostgreSQL 17 released Sep 2024; 18 expected 2026).
  - ISBN "978-0134190440" for *The Go Programming Language* — verify ISBN-13 correctness.
  - `go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest` — canonical install path, verify.
  - Reference URLs (all four).

## Pass 4: Final Polish
- No typos or doubled words detected. One numeric nit (command count) flagged above. YAML snippet with two `volumes:` keys is flagged for a light prose clarifier in Pass 1.
