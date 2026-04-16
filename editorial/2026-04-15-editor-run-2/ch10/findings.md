# Findings: Chapter 10

**Global issue for this chapter:** Most files use ` -- ` for em dashes, though `index.md` and `cicd-fundamentals.md` use correct `—` in places. Standardize all to `—` (no spaces).

---

## index.md

### Summary
Reviewed ~40 lines. Clean file. Uses correct em dashes.

---

## cicd-fundamentals.md

### Summary
Reviewed ~140 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L136–138:** Reference descriptions use ` — ` (spaced em dash). Remove spaces: ` — ` → `—`.

---

## earthly.md

### Summary
Reviewed ~460 lines. 0 structural, 0 line edits, 0 copy edits. 2 factual queries.

### Factual Queries
- **L48:** `golang:1.26-alpine` — Go 1.26 does not exist. Verify at publication.
- **L75:** `golangci-lint@v1.64.8` — plausible for 2026 but verify at publication.

---

## github-actions.md

### Summary
Reviewed ~280 lines. 0 structural, 1 line edit, 1 copy edit. 0 factual queries.

### Line Edits
- **L229:** Missing comma after introductory phrase: "For a learning project**,** those features are optional."

### Copy Edit & Polish
- **L277–279:** Reference descriptions use ` — ` (spaced em dash). Remove spaces.

---

## linting.md

### Summary
Reviewed ~300 lines. 0 structural, 0 line edits, 0 copy edits. 1 factual query.

### Factual Queries
- **L87:** "`SA1006` catches `Printf` calls with no formatting verbs" — imprecise. SA1006 catches `Printf` with a dynamic format string and no arguments. Rephrase.

---

## image-publishing.md

### Summary
Reviewed ~250 lines. 0 structural, 1 line edit, 1 copy edit. 0 factual queries.

### Line Edits
- **L43:** Redundant "Semantic Versioning" expansion — the term is already defined on the same line. → "pushes the SemVer tag alongside the SHA tag."

### Copy Edit & Polish
- **L9:** Uses correct `—` (em dash), but the rest of the file uses ` -- `. Inconsistent. Batch-fix.
