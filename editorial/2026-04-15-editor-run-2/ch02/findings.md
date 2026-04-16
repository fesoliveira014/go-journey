# Findings: Chapter 2

---

## index.md

### Summary
Reviewed 47 lines (15 prose, 20 diagram, 12 structure). 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L3:** "manages" appears twice in the same sentence ("It manages the book registry" and "manages schema"). Vary: "and handles schema changes with versioned SQL migrations."

---

## protobuf-grpc.md

### Summary
Reviewed 333 lines (~180 prose, ~130 code). 1 structural, 3 line edits, 3 copy edits. 1 factual query.

### Structural
- **L3:** Front-loads a walkthrough of a concrete `.proto` file before conceptual grounding, but the "What Is Protocol Buffers?" section immediately below provides that grounding. Acceptable — no change needed.

### Line Edits
- **L3:** "makes protobuf development manageable at scale" → "simplifies protobuf development" — vague.
- **L22:** "tradeoff" → "trade-off" — CMOS hyphenated noun form.
- **L29:** "killer feature" → "most significant advantage" — informal for a technical book.

### Copy Edit & Polish
- **L22:** "tradeoff" → "trade-off" — 2 instances in file.
- **L36:** Nested quotation marks read awkwardly. Consider: "The question is not 'why not REST?'—it is 'where does each fit?'"
- **L7:** "What Is Protocol Buffers?" — subject-verb agreement. Google treats the product name as singular, so this is acceptable, but flag for author's preferred treatment.

### Factual Queries
- **L60–61:** "Proto3 is current; use it for new projects." — Protobuf Editions (starting with Edition 2023) is the successor to proto3. Confirm whether to mention Editions or note proto3 remains the pragmatic default.

---

## postgresql-migrations.md

### Summary
Reviewed 305 lines (~185 prose, ~80 code). 1 structural, 4 line edits, 2 copy edits. 2 factual queries.

### Structural
- **L155–182:** "Embedding Migrations in Go" appears before "Running Migrations Programmatically." A brief forward reference at the end of the embedding section would improve flow.

### Line Edits
- **L46:** "it's worth understanding" → "consider" — throat-clearing.
- **L59:** "genuinely useful" → "useful" — "genuinely" is filler.
- **L122:** "out of the box" → "natively" — informal.
- **L181:** "tradeoff" → "trade-off" — 2 instances in paragraph.

### Copy Edit & Polish
- **L137:** Missing comma after introductory clause: "With an auto-generated name**,** you'd see..." — 2 instances.
- **L181:** "tradeoff" → "trade-off" — batch fix.

### Factual Queries
- **L124:** `gen_random_uuid()` was added in PostgreSQL 13 — correct. Consider noting `pgcrypto` for earlier versions.
- **L258:** "atomic operation" is imprecise for `ALTER TABLE ADD COLUMN ... DEFAULT` in PostgreSQL 11+. The operation avoids a table rewrite but "atomic" has a different database meaning. Consider "a single-statement operation" or "does not require a table rewrite."

---

## repository-pattern.md

### Summary
Reviewed 448 lines (~280 prose, ~140 code). 0 structural, 4 line edits, 1 copy edit. 1 factual query.

### Line Edits
- **L4:** "gets tedious fast" → "becomes tedious quickly" — informal.
- **L47:** "A few things worth calling out:" → "Notable details:" or cut — throat-clearing.
- **L117:** "tells you what's happening" → "communicates intent immediately" — vague.
- **L248:** "done wrong" → "implemented incorrectly" — informal.

### Copy Edit & Polish
- **L439:** "PostgreSQL error messages are translated" → "PostgreSQL error codes are translated" — the code uses SQLSTATE codes, not messages.

### Factual Queries
- **L191:** SQL clause ordering shown as `SELECT * FROM books WHERE id = ? LIMIT 1 ORDER BY id` — `LIMIT` before `ORDER BY` is non-standard. GORM generates `ORDER BY ... LIMIT 1`. Verify and correct.

---

## service-layer.md

### Summary
Reviewed 288 lines (~175 prose, ~90 code). 0 structural, 2 line edits, 0 copy edits. 0 factual queries.

### Line Edits
- **L5:** "it sees both only through interfaces it owns or is handed" → "it interacts with both only through interfaces it defines or receives" — awkward phrasing.
- **L72:** "There's no business logic here worth a dedicated layer for." → "There is no business logic here that warrants a dedicated layer." — dangling preposition, informal.

---

## wiring.md

### Summary
Reviewed 458 lines (~210 prose, ~210 code). 1 structural, 4 line edits, 4 copy edits. 1 factual query.

### Structural
- **L290–315:** "Implementation Notes" section could be confused with the exercise section that follows. Consider a horizontal rule or clearer transition.

### Line Edits
- **L3:** "at some point something has to plug them together" → "something must eventually plug them together" — vague.
- **L21:** "This is convenient until it isn't" → "This is convenient—until something goes wrong and the stack trace runs through several layers of reflection before reaching your code." — cliche.
- **L52:** Missing comma: "For projects up to moderate complexity**,** it is the right approach."
- **L81:** "You would disable this in production" → "Disable this in production" — recommendation presented as fact.

### Copy Edit & Polish
- **L78:** Period placement with closing quotation mark: `50052".` → `50052."` — CMOS 6.9.
- **L141:** After colon, complete sentence begins lowercase. "unexpected errors" → "Unexpected errors" — CMOS 6.63.
- **L443:** "tradeoff" → "trade-off."
- **L52:** "produces zero "magic"" → consider backticks: "produces zero `magic`" — matches code-term convention.

### Factual Queries
- **L186:** `grpc.reflection.v1alpha.ServerReflection` has been superseded by `grpc.reflection.v1.ServerReflection` in newer grpc-go versions. Verify which reflection version the project produces.
