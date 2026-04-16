# Findings: Chapter 3

---

## index.md

### Summary
Reviewed 53 lines. 0 structural, 0 line edits, 0 copy edits. 0 factual queries. Clean file.

---

## docker-fundamentals.md

### Summary
Reviewed ~200 lines. 0 structural, 1 line edit, 0 copy edits. 2 factual queries.

### Line Edits
- **L3:** "skim through and make sure" → "review it and confirm" — slightly informal.

### Factual Queries
- **L79:** `golang:1.26-alpine` — Go 1.26 does not exist. Verify at publication.
- **L136:** `alpine:3.19` — Alpine 3.21+ available. Verify at publication.

---

## writing-dockerfiles.md

### Summary
Reviewed ~340 lines. 1 structural, 0 line edits, 1 copy edit. 3 factual queries.

### Structural
- **L93–103:** Simplified code block omits `pkg/auth/` and `pkg/otel/` COPY lines present in the full Dockerfile above. A brief "abbreviated for clarity" note would prevent confusion.

### Copy Edit & Polish
- **L328:** "25MB" → "25 MB" — missing space before unit (inconsistent with "~5 MB" on L60 and "~300 MB" elsewhere).

### Factual Queries
- **L12:** `golang:1.26-alpine` — same Go version concern.
- **L42:** `alpine:3.19` — same Alpine version concern.
- **L116:** `-S` flag for Alpine `adduser` — parenthetical says "no home directory," but Alpine's `adduser -S` creates a home directory unless `-H` is also passed. Verify whether `-H` is needed or correct the parenthetical.

---

## docker-compose.md

### Summary
Reviewed ~290 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L50:** Backtick-quoted `docker-compose.yml` in heading. CMOS generally discourages code formatting in headings. Consider: "The Compose File Structure."

---

## dev-workflow.md

### Summary
Reviewed ~200 lines. 0 structural, 0 line edits, 0 copy edits. 1 factual query.

### Factual Queries
- **L97:** `github.com/air-verse/air` — verify this is the current canonical import path (transferred from `cosmtrek/air`). Appears correct as of 2024.
- **L120–135:** Gateway's `Dockerfile.dev` lacks `gen/`, `pkg/auth/`, and `pkg/otel/` copies, but the production Dockerfile includes them and `docker-compose.dev.yml` only mounts `../services/gateway`. If Gateway imports from `gen/` or `pkg/`, the dev build would fail. Verify.
