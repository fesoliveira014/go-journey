# Changelog: unit-testing-patterns.md

## Pass 1: Structural / Developmental
- 5 STRUCTURAL comments. Themes:
  - "Subtests with `t.Run`" H2 grows too long once it absorbs parallel-execution material. Consider splitting into "Subtests with t.Run" and a separate H2 "Parallelism with t.Parallel".
  - Heading-case inconsistency: H2s use title case but H3s mix title and sentence case. Normalize.
  - Section promises (via index.md) to cover gomock and testify/mock, but the text does not mention them. Either add a brief pointer or adjust the index promise.
  - Final paragraph promises "the next section introduces mock objects" — 11.2 is actually about PostgreSQL/Testcontainers. Fix cross-reference.
  - Closing "Summary" is adequate.

## Pass 2: Line Editing
- **Line ~13:** Minor refinement on "ceremony" phrasing.
  - Before: "The anonymous struct type is defined inline, so there is no ceremony of naming a type you will never use elsewhere."
  - After: "The anonymous struct type is defined inline, so you avoid naming a type you will never use elsewhere."
  - Reason: Tighter and less editorial.
- **Line ~184:** Split 51-word sentence.
  - Before: "For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed: a package with thirty independent tests, each doing a bit of setup and one assertion, finishes in the time of the slowest single test rather than the sum of all of them."
  - After: "For unit tests that use mocks or in-memory fakes built inside each test function, this is essentially free speed. A package with thirty independent tests — each doing a bit of setup and one assertion — finishes in the time of the slowest single test rather than the sum of all of them."
  - Reason: Sentence over 40 words; easier to parse in two.
- **Line ~191:** Active voice on -race outcome.
  - Before: "... the race detector will fail the build if any test does accidentally share state across goroutines."
  - After: "... the race detector fails the build if any test accidentally shares state across goroutines."
  - Reason: Active voice; remove "does accidentally" stutter.
- **Line ~222:** Simplify "mentally navigate".
  - Before: "You have to mentally navigate from the helper back to the caller to understand what was being set up."
  - After: "You have to trace back from the helper to the caller to understand what was being set up."
  - Reason: Precise verb; shorter.

## Pass 3: Copy Editing
- **Line ~16:** "Google Test" → "GoogleTest" (product canonical name). Please verify.
- **Line ~95 and ~98:** Heading-case mix: "Why a slice of anonymous structs?" and "Worked example: `CreateBook` validation" are sentence case; "Table-Driven Tests" and "Subtests with `t.Run`" are title case. Normalize to title case throughout. (CMOS 8.159)
- **Line ~232:** `rand.Intn(1e10)` — compile issue. `rand.Intn` takes `int`, and `1e10` is a float constant. Needs `rand.Int63n(1e10)` (or explicit cast). Please verify / fix.
- **Line ~274:** "behaviour" → "behavior" (US spelling; chapter elsewhere uses US). (CMOS 11.11, regional consistency)
- **Line ~124:** Please verify: `-run` flag semantics. Per `go help testflag`, `-run regexp` matches slash-separated levels independently, not the full path as one regexp. Current phrasing may mislead.
- **Line ~354:** Please verify: footnote URLs (go.dev/blog/subtests and the GopherCon YouTube link) still resolve.
- **Line ~195:** "e.g.," — comma after correct per CMOS 6.43.
- **Line ~188:** "thirty" — spelled out per CMOS 9.2. Correct.

## Pass 4: Final Polish
- **Line ~348:** Closing sentence promises mock-object content in the next section, but 11.2 covers Testcontainers/PostgreSQL. Rewrite to: "The next section shifts from mocks to real infrastructure, using Testcontainers to run repository tests against a real PostgreSQL instance." (or similar).
- **Line ~232:** `rand.Intn(1e10)` compile/runtime risk flagged above; could be a bug in example code.
- **Line ~133:** Loop-variable capture comment says "required before Go 1.22" — check correctness. (Go 1.22 made per-iteration loop variables the default.) Confirm phrasing once.
