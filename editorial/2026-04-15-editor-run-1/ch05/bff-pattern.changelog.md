# Changelog: bff-pattern.md

## Pass 1: Structural / Developmental
- 7 comments. Themes: (1) good three-way framing at the opening; add an explicit "we pick option 3" handoff; (2) "Go as a BFF Language" could add one sentence on single-binary / small-image advantage; (3) Spring DI example is ~15 lines of Java for a small point — consider trimming; (4) the big route registration block is a wall of code; inline comments split it sufficiently; (5) the `baseTmpl` "pick any entry" trick warrants a forward reference to §5.2's clone-per-page explanation; (6) middleware section ordering (wire → chain semantics → code) works; (7) "Wiring It All Together" has minor redundancy with the earlier "no annotation magic, no DI container" bullet.

## Pass 2: Line Editing
- **Line ~3:** Tighten existential-there opener (optional).
  - Before: "there are three common approaches"
  - After: "three common approaches exist" (or keep — rhythm is acceptable)
  - Reason: existential-there is weak; mention only.
- **Line ~11:** Active-voice tighten.
  - Before: "It does not contain business logic -- it does not validate ISBNs or hash passwords."
  - After: "It contains no business logic — it does not validate ISBNs or hash passwords."
  - Reason: negative construction reads better in active form.
- **Line ~19:** Mild tighten.
  - Before: "Go is well-suited for this role."
  - After: "Go fits this role well."
  - Reason: removes passive flavor; optional.
- **Line ~54:** Overclaim.
  - Before: "This is Go's dependency injection pattern:"
  - After: "This is manual dependency injection, Go-style:"
  - Reason: avoids implying one canonical Go DI pattern.
- **Line ~102:** Wording overlap with "method patterns" above.
  - Before: "The pattern is RESTful: `GET` for reads, `POST` for mutations."
  - After: "The style is RESTful: `GET` for reads, `POST` for mutations."
  - Reason: avoids reusing "pattern" so close to the ServeMux "method patterns" phrase.
- **Line ~184:** Factual tighten.
  - Before: "`http.ResponseWriter` is a write-only interface."
  - After: "`http.ResponseWriter` does not expose response status or body reads."
  - Reason: ResponseWriter's Header() is read-write for the header map, so "write-only" is not strictly correct.
- **Line ~188:** Subject-verb agreement.
  - Before: "these middleware are the equivalent of"
  - After: "these middleware functions are equivalent to"
  - Reason: "middleware" as a count noun with "these" and the repeated "of" sounds off.
- **Line ~194:** Factual correction.
  - Before: "The `main.go` function ties everything together"
  - After: "The `main` function ties everything together"
  - Reason: `main.go` is a file, not a function.

## Pass 3: Copy Editing
- **Line ~5:** "React, Angular, etc." — CMOS 6.43 serial comma before "etc." is present. Optional: replace "etc." with "or similar" for warmer prose.
- **Line ~9:** Lock "Backend-for-Frontend (BFF)" form chapter-wide (see index.md note).
- **Line ~25:** Please verify Chi's canonical casing (go-chi/chi README typically uses lowercase "chi"; many articles use "Chi"). Consistency optional.
- **Line ~25:** Please verify: Go 1.22 introduced method-in-pattern ServeMux syntax. Confirmed per [^1].
- **Line ~87:** Please verify: `{$}` exact-match terminator in ServeMux patterns is documented in Go 1.22 release notes.
- **Line ~213:** "explicit wiring" uses straight quotes in source; confirm Markdown pipeline converts to smart quotes (CMOS 6.9).
- **Line ~219:** Please verify Go 1.22 release notes anchor `#enhanced_routing_patterns` still resolves.
- **Line ~220:** Please verify Sam Newman BFF URL still resolves.

## Pass 4: Final Polish
- No typos, doubled words, or homophones found. Inline code backticks consistent. Cross-reference to §5.3 correct. Footnote numbering [^1]–[^3] contiguous.
