# Findings: Chapter 1

---

## index.md

### Summary
Reviewed 36 lines (36 prose). 0 structural, 0 line edits, 0 copy edits. 1 factual query.

### Factual Queries
- **L17, L25:** "Go 1.26+" — Go 1.26 does not exist as of May 2025 (latest stable: 1.24). Verify this is a deliberate forward-looking pin or correct to current version.

---

## project-setup.md

### Summary
Reviewed 206 lines (~120 prose, ~86 code). 1 structural, 3 line edits, 2 copy edits. 0 factual queries.

### Structural
- **L23:** "Three things worth unpacking:" introduces three bold-headed paragraphs without numbering or bullets. Consider numbering them for scanability.

### Line Edits
- **L9:** "is roughly the equivalent of a Maven" → "resembles a Maven" — wordy.
- **L72:** "It is safe to run at any time; the command is idempotent." → "The command is idempotent and safe to run at any time." — combines into one sentence.

### Copy Edit & Polish
- **L25:** "This is not just convention for readability" → "This is not just a convention for readability" — missing article.
- **L78:** "that is a Java/Maven artifact Go does not need" → "that is a Java/Maven convention Go does not use" — "artifact" is ambiguous in a Maven context.

---

## go-basics.md

### Summary
Reviewed 433 lines (~230 prose, ~203 code). 0 structural, 2 line edits, 0 copy edits. 0 factual queries.

### Line Edits
- **L300:** "Go's are simpler" → "Go pointers are simpler" — clearer antecedent.
- **L328:** "which interface a given receiver set satisfies" → "which interfaces the type satisfies" — less awkward.

---

## http-server.md

### Summary
Reviewed 263 lines (~145 prose, ~118 code). 1 structural, 1 line edit, 2 copy edits. 1 factual query.

### Structural
- **L260–262:** Footnote [^1] is defined but never referenced in the prose. Either add a reference or remove the footnote.

### Line Edits
- **L226:** Punctuation error and British spelling in one sentence.
  Before: "in one line, (Java programmers will recognise the scoping idea"
  After: "in one line. (Java programmers will recognize the scoping idea"

### Copy Edit & Polish
- **L226:** "recognise" → "recognize" — American English consistency. 1 instance.

### Factual Queries
- **L217:** Go version inconsistency — ch01/index.md requires "Go 1.26+" but the Earthfile uses `golang:1.22-alpine`. Reconcile.

---

## testing.md

### Summary
Reviewed 247 lines (~140 prose, ~107 code). 1 structural, 2 line edits, 1 copy edit. 2 factual queries.

### Structural
- **L136–181:** The import block at L79–85 does not include `"encoding/json"`, but `TestBooksHandler` at L171 calls `json.NewDecoder`. This would cause a compilation error.

### Line Edits
- **L9:** "by their name: any file" → "by their name: Any file" — capitalize after colon (complete sentence follows, CMOS 6.63).
- **L25:** "but the standard library alone is sufficient and is what you will use here." → "but the standard library suffices and is what this project uses." — tighter.

### Copy Edit & Polish
- **L227:** "no Maven settings XML" → "no Maven `settings.xml`" — backticks for filename.

### Factual Queries
- **L217:** `golang:1.22-alpine` in Earthfile contradicts "Go 1.26+" prerequisite. Reconcile.
- **L79–85:** Missing `"encoding/json"` import. Code would not compile as written. Add the import or show a separate complete import block.
