# Changelog: index.md

## Pass 1: Structural / Developmental
- 4 comments. Themes: (1) lead could add a one-line "why not SPA?" forward-reference to tighten the chapter arc; (2) BFF acronym is introduced inside a dense sentence and could be its own sentence; (3) section list is well-scoped; (4) architecture diagram placement is correct.

## Pass 2: Line Editing
- **Line ~3:** Soften mild hedge in the opener.
  - Before: "That is fine for service-to-service communication"
  - After: "That works for service-to-service communication"
  - Reason: "is fine" is weaker than "works"; removes filler.
- **Line ~33:** Collapse redundant sentences about the gateway's statelessness.
  - Before: "The gateway has no database of its own. It holds no business state -- it is a translation layer between HTTP/HTML and gRPC."
  - After: "The gateway has no database and holds no business state; it is a translation layer between HTTP/HTML and gRPC."
  - Reason: two sentences say the same thing; merging tightens without losing nuance.

## Pass 3: Copy Editing
- **Line ~3:** Lock one form for "Backend-for-Frontend (BFF)" across the chapter. Industry-standard is lowercase "backend for frontend (BFF)" (per Sam Newman). Current chapter mixes "Backend-for-Frontend", "Backend for Frontend", and bare "BFF". CMOS style-choice note only — consistency is required.
- **Line ~44:** "POST-redirect-GET" vs. §5.3's "POST-Redirect-GET" — lock one casing. Suggest "POST-Redirect-GET" as the chapter-wide form (matches Wikipedia title casing referenced in 5.3's footnote).
- **Line ~48:** "Chapters 1--4" — verify the book's Markdown renderer converts `--` to an en dash (CMOS 6.78). If it renders as em dash, change to `Chapters 1-4` or spell out.

## Pass 4: Final Polish
- No typos, doubled words, or homophones found. Cross-links to sibling files verified present (`./bff-pattern.md`, `./templates-htmx.md`, `./session-management.md`, `./admin-crud.md`).
