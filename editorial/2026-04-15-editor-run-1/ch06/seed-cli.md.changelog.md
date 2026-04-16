# Changelog: seed-cli.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Section mirrors 6.1's Problem → Design → Walkthrough → Usage → Takeaways arc — consistent with the chapter's pedagogical rhythm.
  - "Struct and Imports" heading shows only the struct; either include imports or rename to "The Seed Struct".
  - Technical correction worth a query: the prose implies `Login` fails for a non-admin user; in practice, a non-admin can log in successfully and will only be rejected at `CreateBook`. Worth a sentence of clarification.
  - The "non-default ports" example shows the default port numbers; consider using an actual non-default value to make the example demonstrate the feature.

## Pass 2: Line Editing
- **Line ~17:** remove filler
  - Before: "The seed CLI is just an automated version of that workflow."
  - After: "The seed CLI is an automated version of that workflow."
  - Reason: "just" is in the cut-list.
- **Line ~90:** clarify failure mode
  - Before: "If login fails (wrong password, non-existent user, user is not admin), the process exits with a clear error."
  - After: "If login fails (wrong password or non-existent user), or if the user is not admin, the first `CreateBook` call will fail — in either case the process exits with a clear error."
  - Reason: factually tighter; `Login` itself does not check admin role — only the subsequent `CreateBook` does.

## Pass 3: Copy Editing
- **Chapter-wide:** `--` → `—` em dash, no surrounding spaces. CMOS 6.85.
- **Line ~42:** "int32" in prose — wrap in backticks for consistency with code voice: `int32`.
- **Line ~236:** "CI" — define on first use: "CI pipelines" → "continuous integration (CI) pipelines". CMOS 10.3.
- **Query — line ~74:** please verify `grpc.NewClient` vs. `grpc.Dial`. `grpc.NewClient` was promoted to the canonical constructor in `google.golang.org/grpc` v1.63 (2024). Confirm the project's `go.mod` pins v1.63 or later; if it uses an earlier pin, the code should use `grpc.Dial`.
- **Query — lines ~149, ~196:** "Sapiens" vs. "Sapiens: A Brief History of Humankind" — the fixture-file table lists the short form; the expected-output block uses the long form. Reconcile so the two blocks agree.
- **Query — line ~169:** please verify the Kafka topic name `catalog.books.changed` matches what Chapter 7 will introduce. This is the forward reference most likely to drift.
- **Line ~26:** CMOS 6.19 serial comma in "browse, reserve, and test against" — correct.
- Various compound adjectives hyphenated correctly before nouns; no fixes.
- Quotation punctuation on "no copies available" (line ~149) — correct per CMOS 6.9.

## Pass 4: Final Polish
- **Line ~196:** title inconsistency (Sapiens short vs. long form). Single source of truth fix needed.
- **Lines ~220–228:** the "non-default ports" example uses default ports — copy-paste friendly but pedagogically confusing.
- No typos, doubled words, or missing words found.
