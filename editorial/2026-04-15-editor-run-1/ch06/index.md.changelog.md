# Changelog: index.md

## Pass 1: Structural / Developmental
- 3 comments. Themes:
  - Opening motivation is strong; consider adding a one-line "by the end of this chapter you will have X" promise.
  - Count mismatch: prose says "three tools" but the list has four items (the fourth is a walkthrough, not a tool).
  - The section-outline table duplicates the numbered list; suggest framing the table as a quick-reference so the two feel complementary rather than redundant.

## Pass 2: Line Editing
- **Line ~17:** tighten hollow pivot
  - Before: "This involves adding a new proto message (`ReservationDetail`) that denormalizes book titles and user emails into a single response."
  - After: "We also add a new proto message, `ReservationDetail`, that denormalizes book titles and user emails into a single response."
  - Reason: active voice; removes the "This involves adding" scaffold.
- **Line ~9:** optional tightening of leading conjunction (flagged, not prescribed)
  - Before: "…it all in the browser. But if you try to use it from scratch…"
  - After: "…it all in the browser. If you try to use it from scratch…"
  - Reason: sentence-initial "But" is acceptable; here the "But"/"if" pairing is slightly redundant.
- **Line ~9:** reconcile "three tools" with four-item list
  - Before: "This chapter builds three tools that solve these problems and make the system usable for development:"
  - After: "This chapter builds three tools that solve these problems and make the system usable for development — plus an end-to-end walkthrough to tie them together:"
  - Reason: aligns the head-count with the numbered list.

## Pass 3: Copy Editing
- **Chapter-wide:** `--` → `—` (em dash, no surrounding spaces). CMOS 6.85. The author already uses a true em dash once in admin-cli.md line 88, so the `--` spellings are an inconsistency rather than a deliberate style choice.
- **Line ~15:** CLI — first use in chapter. Expand on first mention as "command-line interface (CLI)" or cross-reference the prior chapter where it was defined. CMOS 10.3.
- **Line ~5:** "16 times" — numerals fine for a repeated operation; consistent with "16 books" later. CMOS 9.2.
- **Line ~29 (table):** "DB" vs "database" — the body text mixes both; choose one. Recommend "database" for prose, reserve "DB" for labels only.

## Pass 4: Final Polish
- No typos, doubled words, or missing words found.
- All four relative links (`admin-cli.md`, `admin-dashboard.md`, `seed-cli.md`, `putting-it-together.md`) resolve to sibling files.
