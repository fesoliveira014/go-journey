# Changelog: go-basics.md

## Pass 1: Structural / Developmental
- 6 comments. Themes:
  - Heading level inconsistency: this file opens at H2 (`##`) while project-setup.md opens at H1 (`#`). Chapter-wide style decision required.
  - "No pointer arithmetic" note is orphaned at the end of "Types and Zero Values" — it belongs in the Pointers section.
  - Slices and Maps share one heading; the maps portion is shorter and feels tacked on. Consider splitting.
  - Error handling section could introduce sentinel-error idiom (`errors.New`) before the wrapping section to make the exercise solution feel continuous with the prose.
  - "Types" table cites "not `null`" — could stay in Go idiom ("not `nil`").
  - Exercise is well-calibrated and reinforces every subsection.

## Pass 2: Line Editing
- **Line ~48:** Parallelism fix for unmotivated fragment.
  - Before: "No pointer arithmetic. `p++` on a pointer is a compile error."
  - After: "**No pointer arithmetic.** `p++` on a pointer is a compile error."
  - Reason: Matches bold-lead pattern used for "**No implicit conversions.**" and "**Every type has a zero value.**" above.
- **Line ~142:** Awkward trailing phrase.
  - Before: "The compiler checks at the point of use."
  - After: "The compiler verifies satisfaction where the interface is used."
  - Reason: Clarifies what is checked; removes bare prepositional trailing phrase.
- **Line ~155:** Active-voice rewrite.
  - Before: "This pattern will come up constantly in this project."
  - After: "You will see this pattern constantly in this project."
  - Reason: Keeps author-reader direct address established elsewhere.
- **Line ~301:** Noun pile.
  - Before: "Go's are simpler: same concept, no arithmetic, automatic nil-safety enforcement via the runtime."
  - After: "Go's are simpler — same concept, no arithmetic, and the runtime enforces nil safety automatically."
  - Reason: Active verb replaces passive abstract noun pile "nil-safety enforcement".
- **Line ~423:** Long sentence split.
  - Before: "Section 1.3 builds on these types by wiring them into an HTTP server — you will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses."
  - After: "Section 1.3 wires these types into an HTTP server. You will see how Go's `net/http` package uses interfaces (specifically `http.Handler`) to compose request handling, and how the structs you defined here become JSON responses."
  - Reason: 40 words → two clearer sentences.
- **Line ~346:** Minor clarity tweak on exercise step 5 ("...for a genre that exists and one that does not" → "...for both a genre that exists and one that does not").

## Pass 3: Copy Editing
- **Line ~9:** `uint` listed but `uint8`, `uint16`, `uint32`, `uint64` omitted from primitives list; either drop `uint` or add the rest.
- **Line ~17:** Query: Verify the exact Go compiler error text; current phrasing may be slightly off.
- **Line ~35:** Consider "(empty string, not `nil`)" for Go-idiom consistency.
- **Line ~153:** "Postgres" — CLAUDE.md tech stack is PostgreSQL; prefer PostgreSQL in prose. Apply consistently across book.
- **Line ~164:** "unmarshalling" → "unmarshaling" (Go ecosystem + CMOS preference for American spelling with single `l`).
- **Line ~177:** CMOS 7.82 — adverbs ending in -ly are not hyphenated when forming compound adjectives. "dynamically-sized" → "dynamically sized".
- **Line ~209:** Query/clarification: C++ contrast on range copy semantics is technically inaccurate without the `auto&` qualifier; consider rewording to avoid misleading.
- **Line ~275:** Snippet references `*NotFoundError` and `ErrNotFound` that are not defined earlier; add a sentence naming them as illustrative sentinels.
- **Line ~309:** "zeroed T" → "zero-value T" for terminology consistency with the earlier Zero Values subsection.
- **Line ~313:** "vs" → "versus" or "vs." (CMOS). Apply chapter-wide.

## Pass 4: Final Polish
- **Line ~1:** Heading level — should be H1 if other section files use H1. Cold-read flags this as inconsistent.
- No typos, doubled words, or homophone errors detected.
