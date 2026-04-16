# Findings: Chapter 5

**Global issue for this chapter:** All files use ` -- ` for em dashes. Batch-replace with `—` (no spaces).

---

## index.md

### Summary
Reviewed ~53 lines. Clean file. No issues beyond the global em dash fix.

---

## bff-pattern.md

### Summary
Reviewed ~220 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L25:** "We will use the stdlib exclusively." → "We use the stdlib exclusively." — present tense for consistency.

---

## templates-htmx.md

### Summary
Reviewed ~325 lines. 0 structural, 0 line edits, 2 copy edits. 1 factual query.

### Copy Edit & Polish
- **L217:** "14KB" → "14 KB" — CMOS requires space between numeral and unit.
- **L325:** "This is progressive enhancement: the page works without JavaScript..." → capitalize "The" after colon (complete sentence, CMOS 6.63).

### Factual Queries
- **L45:** `htmx.org@2.0.4` with SRI hash — verify integrity hash matches the published hash for 2.0.4 before print.

---

## session-management.md

### Summary
Reviewed ~290 lines. 0 structural, 0 line edits, 2 copy edits. 1 factual query.

### Copy Edit & Polish
- **L44:** "`Strict` would also block" — consider backtick formatting for `Strict` for consistency with other cookie attribute references.
- **L280:** `"https://yoursite.com/logout"` → use `"https://example.com/logout"` per RFC 2606.

### Factual Queries
- **L188:** "OAuth2Start uses 302 Found (not 303) because we want the browser to follow the redirect with the same method" — for GET requests, 302 and 303 behave identically. The method-preservation rationale is misleading. Simplify: "OAuth2Start uses 302 Found, the standard status code for OAuth2 authorization redirects."

---

## admin-crud.md

### Summary
Reviewed ~250 lines. 1 structural, 0 line edits, 1 copy edit. 2 factual queries.

### Structural
- **L110–112:** `setFlash(w, "Book created")` is called as a bare function, but in session-management.md L199 it is a method on `Server` (`s.setFlash`). One is inconsistent with the codebase. Verify.

### Copy Edit & Polish
- **L187:** "with one addition: it copies the `templates/` and `static/` directories" → capitalize "It" after colon (complete sentence, CMOS 6.63).

### Factual Queries
- **L110:** `setFlash` vs `s.setFlash` — verify which is correct in actual code.
- **L193:** `golang:1.26-alpine` — Go 1.26 does not exist. Verify at publication.
