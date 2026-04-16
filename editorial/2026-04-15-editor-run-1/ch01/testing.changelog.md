# Changelog: testing.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Heading level inconsistency (H2) — same chapter-wide issue as 1.2/1.3.
  - Opens strong but makes an absolute claim ("the standard library alone is ... what you will use here") that should be scoped: "here" = chapter 1, or whole book?
  - Test snippets use escaped quotes where a raw string (backticks) would read better.
  - `t.Parallel()` omitted. Given author values parallelism (recent commits add `t.Parallel()`), a forward-reference would tie the arc.
  - Earthly section jumps from stdlib to containers without a transition.
  - `+lint` target referenced but never defined — needs forward-reference or definition.
  - Exercise depends on §1.3 exercise; make dependency explicit.

## Pass 2: Line Editing
- **Line ~17:** Clarify ambiguous parenthetical.
  - Before: "the next character must be uppercase (or end of name)"
  - After: "the next character must be uppercase"
  - Reason: The "(or end of name)" parenthetical is technically true (`func Test(t *testing.T)` is valid) but confusing at first introduction and not actionable.
- **Line ~25:** Pin ambiguous "This".
  - Before: "This is the only assertion mechanism you get by default."
  - After: "These three methods are the only assertion mechanism you get by default."
  - Reason: Antecedent of "This" was ambiguous between the table and `t.Errorf` specifically.
- **Line ~44:** Sharpen claim about the race detector.
  - Before: "It has a runtime cost (~2–20x slowdown) but catches real bugs. You should run it in CI even if not locally every time."
  - After: "It has a runtime cost (roughly 2–20× slowdown) but catches genuine concurrency bugs that are otherwise nearly impossible to reproduce. Run it in CI, even if you skip it locally."
  - Reason: "catches real bugs" is vague; explicit mention of concurrency bugs is more informative.
- **Line ~66:** Split 44-word sentence.
  - Before: "No ports, no goroutines, no teardown. This is the standard Go pattern — if you have ever used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical, but the implementation is lighter because the handler is already just a function."
  - After: "No ports, no goroutines, no teardown. This is the standard Go pattern. If you've used Spring's `MockMvc` or Ktor's `testApplication`, the motivation is identical — but the implementation is lighter because the handler is already just a function."
  - Reason: Splits into two sentences; uses em dash for contrast.
- **Line ~194:** Soften "standard workflow" claim.
  - Before: "This is the standard Go workflow — there is no plugin or external tool required."
  - After: "This is the built-in Go workflow — no plugin or external tool required."
  - Reason: "standard" overstates; there are third-party coverage tools. "built-in" is accurate.

## Pass 3: Copy Editing
- **Line ~19 (table header):** "Behaviour" → "Behavior" (AmE consistency).
- **Line ~44:** "synchronised" → "synchronized" (AmE).
- **Line ~44:** "~2–20x slowdown" — use × (multiplication sign) instead of x; en dash for range is correct.
- **Line ~46:** CMOS 6.9 — period inside closing quotation marks in AmE: "below it". → "below it."
- **Line ~52:** "afterwards" → "afterward" (AmE).
- **Line ~132:** `go test -run TestHealthHandler/GET_returns_200` — actual subtest name is "GET returns 200 with ok body" which becomes "GET_returns_200_with_ok_body". Either use the full name or explain space-to-underscore mangling in the prose.
- **Line ~171:** Code snippet uses `json.NewDecoder` but its corresponding import block (the previous test's imports) doesn't include `encoding/json`. Either add the import or show the imports block for this test separately.
- **Line ~193:** "colour-coding" → "color-coding" (AmE).
- **Line ~203:** "behavioural" → "behavioral" (AmE).
- **Line ~217:** Query: `golang:1.22-alpine` image with prerequisites stating Go 1.26+ and go.mod `go 1.26.1`. Real inconsistency — please verify and align.
- **Line ~233:** "e.g." → "e.g.," (CMOS 6.43). Two instances in the exercise list.
- **Line ~222:** `+lint` target referenced without definition or forward-reference. Add.
- **General:** Code snippets use escaped quotes; raw strings (backticks) would improve readability. Optional.

## Pass 4: Final Polish
- **Line ~1:** Heading H2 vs H1 — inconsistent with project-setup.md.
- **Line ~25:** "what you will use here" — cold-read ambiguity (chapter vs book).
- No typos, doubled words, or homophone errors detected.
