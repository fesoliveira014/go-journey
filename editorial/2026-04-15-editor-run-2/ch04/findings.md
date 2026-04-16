# Findings: Chapter 4

**Global issue for this chapter:** All files use `--` (double hyphen with spaces) for em dashes, while Chapter 3 uses correct `—`. Batch-replace all ` -- ` with `—` (no spaces) across all Chapter 4 files. Keep `--` for en dash ranges (e.g., "Chapters 1–3").

---

## index.md

### Summary
Reviewed 65 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L47:** "Chapters 1--3" — en dash for range. Verify renderer converts `--` to en dash; if not, use Unicode `–` directly.

---

## auth-fundamentals.md

### Summary
Reviewed ~230 lines. 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L163:** Footnote marker `[^1]` precedes the period. Per CMOS 14.21, place after: "not a theoretical concern.[^1]"
- **L223:** "RFC 7519 -- JSON Web Token" — reference separator should be em dash: "RFC 7519—JSON Web Token" or use a colon.

---

## auth-service.md

### Summary
Reviewed ~335 lines. 1 structural, 0 line edits, 0 copy edits. 2 factual queries.

### Structural
- **L283–284:** Long blockquote reads as authorial aside, not a quotation. Consider using `> **Note:**` admonition to distinguish from quoted material.

### Factual Queries
- **L44:** `uuid-ossp` extension — PostgreSQL 13+ includes `gen_random_uuid()` natively. Consider mentioning the modern alternative or noting why `uuid-ossp` was chosen.
- **L56:** `valid_role CHECK` constraint restricts to `'user'` or `'admin'`. If more roles are added later (e.g., `'employee'`), this constraint needs updating. Note as a deliberate simplification.

---

## oauth2.md

### Summary
Reviewed ~265 lines. 0 structural, 0 line edits, 3 copy edits. 1 factual query.

### Copy Edit & Polish
- **L100:** CSRF not expanded on first use. → "a **cross-site request forgery (CSRF) attack**"
- **L103:** "a 5-minute TTL" → "a five-minute TTL" — spell out number, hyphenated compound adjective.
- **L232:** "at most 5 minutes" → "at most five minutes."

### Factual Queries
- **L164:** `googleapis.com/oauth2/v2/userinfo` — Google recommends v3 or the OpenID Connect endpoint. Verify v2 is intentional.

---

## interceptors.md

### Summary
Reviewed ~350 lines. 1 structural, 0 line edits, 2 copy edits. 1 factual query.

### Structural
- **L48–49:** "a Go community convention" for `pkg/` layout overstates consensus. The `golang-standards/project-layout` repo is not officially endorsed by the Go team. Soften to "a common Go convention."

### Copy Edit & Polish
- **L111:** Mixed dash styles in same sentence: `--` and `—` with spaces. Standardize to `—` (no spaces).
- **L246:** "except explicit rather than declarative" → "except that it is explicit rather than declarative" — grammatically incomplete.

### Factual Queries
- **L48:** "Go community convention" for `pkg/` — debated. Go blog and official docs do not endorse this layout. Soften claim.
