# Changelog: admin-crud.md

## Pass 1: Structural / Developmental
- 6 comments. Themes: (1) opener lists four topics that mirror section headings — good roadmap; (2) `requireAdmin` uses motivation-first pedagogy (extract-after-pain); (3) `AdminBookCreate` code block is long; trimming the second re-render block to a comment would save ~8 lines; (4) gRPC error mapping uses function-then-table — correct authoritative ordering; (5) Testing Strategy is thin (~6 lines) for a "Strategy" heading — given the Java reader's expectation, a table-driven httptest example would strengthen; (6) Exercises are scoped well and all achievable with patterns just taught.

## Pass 2: Line Editing
- **Line ~9:** Tighten.
  - Before: "Every admin handler needs to verify two things:"
  - After: "Every admin handler verifies two things:"
  - Reason: "needs to verify" is wordy; the action description is stronger than the obligation.
- **Line ~111:** Potential bug / stale snippet (see copy-edit).
  - Before: `setFlash(w, "Book created")`
  - After: `s.setFlash(w, "Book created")` (if `setFlash` is actually a `*Server` method per §5.3)
  - Reason: §5.3 shows `s.setFlash(w, ...)`; the bare call here either shadows a package-level helper or is an oversight. Query.
- **Line ~181:** Awkward compound modifier.
  - Before: "`ResourceExhausted` uses a hardcoded user-friendly message for the reservation limit."
  - After: "`ResourceExhausted` uses a fixed, user-friendly message for the reservation limit."
  - Reason: "hardcoded user-friendly" reads as two competing adjectives; a comma after "fixed" separates them.
- **Line ~251:** Contraction consistency.
  - Before: "we don't yet have an admin account or sample books"
  - After: "we do not yet have an admin account or sample books"
  - Reason: chapter uses uncontracted forms; lock one register.
- **Line ~251:** Contraction consistency.
  - Before: "we'll build CLI tools"
  - After: "we will build CLI tools"
  - Reason: same as above.

## Pass 3: Copy Editing
- **Line ~3:** "CRUD" — consider expanding on first use ("create, read, update, delete (CRUD)") per CMOS 10.2. Optional.
- **Line ~39:** "tradeoff" — see session-management.md note; lock "trade-off" (CMOS 7.89) or "tradeoff" chapter-wide.
- **Line ~111:** Please verify: `setFlash` (package-level) vs. `s.setFlash` (method) — determine which is correct for this handler and align with §5.3.
- **Line ~193:** Please verify: `golang:1.26-alpine` is a valid Docker tag as of April 2026. Based on Go's six-month release cadence (1.22 Feb 2024, 1.23 Aug 2024, 1.24 Feb 2025, 1.25 Aug 2025, 1.26 Feb 2026), this is likely accurate — confirm at Docker Hub (https://hub.docker.com/_/golang).
- **Line ~213:** Please verify: `alpine:3.19` is still supported as of April 2026. Alpine 3.19 was released Dec 2023; by April 2026 it is near or past end of support. Recommend bumping to the current stable Alpine tag (likely 3.22 or 3.23) for consistency with the chapter's "current best practice" tone.
- **Line ~243:** Exercise 3 uses `page` and `page_size` in snake_case; matches protobuf field-name convention. Correct.
- **Line ~247:** Consider adding `embed.FS` forward-reference for production template embedding. Optional.
- **References:** Please verify all four URLs resolve.

## Pass 4: Final Polish
- No typos, doubled words, or homophones found. Code fences balanced. Footnote markers [^1]–[^4] contiguous. Cross-reference to "section 5.3" correct; cross-reference to "Chapter 1" (for httptest) and "Chapter 3" (for Dockerfile pattern) consistent with chapter's earlier style. Inline-code backticks balanced.
