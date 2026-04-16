# Changelog: postgresql-migrations.md

## Pass 1: Structural / Developmental
- 8 comments. Themes:
  - Opening promises four things; consider splitting into a "we will" list or narrowing scope.
  - The two-alternatives (external vs embedded) and the "Why GORM AutoMigrate is dangerous" framings are strong pedagogical moves; keep.
  - Missing sidebar on startup-migration race conditions at replica scale — worth at least a one-line footnote/forward-reference.
  - Consider explicit recommendation of `gen_random_uuid()` for new projects on PG 13+, noting why you chose `uuid-ossp`.
  - Pedagogical asymmetry in "down migrations should undo the up migration completely" — the extension is left in place; flag this as an intentional exception.

## Pass 2: Line Editing
- **Line ~9:** Remove banned "just".
  - Before: "just one command:"
  - After: "a single command does it:"
- **Line ~33:** "Useful commands to remember" → "Commands worth knowing" — trims filler.
- **Line ~63:** Tighten adverb-heavy sentence about environment drift.
  - Before: "Every environment is potentially in a different state with no way to tell."
  - After: "Every environment drifts silently with no way to tell."
- **Line ~137:** Break and tighten the constraint-naming paragraph for rhythm; see annotated.
- **Line ~141:** Second conjunct inconsistency in index discussion.
  - Before: "`CREATE INDEX idx_books_genre ON books(genre)` and `idx_books_author ON books(author)` exist because..."
  - After: "The two index statements exist because..."
- **Line ~149:** Reframe the "we don't reverse the extension" caveat so the asymmetry with "undo ... completely" is acknowledged.
- **Line ~173:** Split long sentence on `//go:embed` directive semantics; see annotated for suggestion.
- **Line ~225:** Step 3 wording — "within the embedded FS" → "inside the embedded filesystem".

## Pass 3: Copy Editing
- **Multiple H3s:** Heading-case inconsistency — mix of title and sentence case across H3s in this file (e.g. "Writing Migrations" title case; "The up migration" sentence case). Standardize to sentence case for H3s, title case for H2s.
- **Line ~128:** "schema in production, staging, and local dev gradually diverge" — subject-verb agreement is awkward because "schema" is singular; consider "schemas" for cleaner plural agreement.
- **Line ~173:** "new in Go 1.16" → "introduced in Go 1.16" — tense-neutral; the feature is not new in 2026.
- **Line ~211:** `err != migrate.ErrNoChange` — idiomatic modern Go would prefer `errors.Is`. Query rather than silent fix since the source may be returning an unwrapped sentinel (in which case `!=` is fine).
- **Line ~258:** "PostgreSQL 11+" — PG 11 is past community EOL; rephrase or soften.
- Serial-comma and numeric-unit spellings verified (CMOS 6.19, 9.2); hyphenation of "timezone-correct" and "frequently-queried" conforms to CMOS 7.81.
- Footnote URLs — verify (query).

### Factual queries (Please verify)
- **Line ~18:** `postgres:16` — is this the intended major version pin in a 2026 publication? PG 17 is current.
- **Line ~124:** "PostgreSQL 13+ ships `gen_random_uuid()` as a built-in" — correct, but consider upgrading recommendation language.
- **Line ~211:** Does `migrate.Up()` in current golang-migrate versions return `ErrNoChange` wrapped or unwrapped? If wrapped, the `!=` comparison is subtly wrong.
- **Line ~258:** `ADD COLUMN ... DEFAULT` atomic in PG 11+ — verify still the current cutoff (it is; introduced in PG 11, mid-2018).
- **References:** Confirm footnote URLs live and canonical.

## Pass 4: Final Polish
- No typos, doubled words, homophone errors, or broken cross-references found.
