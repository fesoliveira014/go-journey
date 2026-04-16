# Findings: Chapter 6

**Global issue for this chapter:** All files use ` -- ` for em dashes. Batch-replace with `—` (no spaces).

---

## index.md

### Summary
Reviewed ~35 lines. 0 structural, 1 line edit, 0 copy edits. 0 factual queries.

### Line Edits
- **L6:** "16 times" → "sixteen times" — spell out per CMOS.

---

## admin-cli.md

### Summary
Reviewed ~185 lines. 1 structural, 0 line edits, 0 copy edits. 1 factual query.

### Structural
- **L80–84:** Code uses `pkgdb.Open(dsn, pkgdb.Config{})` but the import block (L36–47) does not include a `pkgdb` import. Add the import or note the abbreviation.

### Factual Queries
- **L36–47 vs L80:** Missing `pkgdb` import in code listing.

---

## admin-dashboard.md

### Summary
Reviewed ~410 lines. 0 structural, 0 line edits, 0 copy edits. 0 factual queries. Clean file.

---

## seed-cli.md

### Summary
Reviewed ~160 lines. 0 structural, 0 line edits, 3 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L131:** "all 16 books" → "all sixteen books."
- **L139:** "contains 16 books" → "contains sixteen books."
- **L149:** "from 2 to 5" → "from two to five."

---

## putting-it-together.md

### Summary
Reviewed ~120 lines. 0 structural, 1 line edit, 0 copy edits. 0 factual queries.

### Line Edits
- **L47:** "You should see 16 books created." → "You should see sixteen books created."
