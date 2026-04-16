# Changelog: index.md

## Pass 1: Structural / Developmental
- 3 comments. Themes:
  - Good chapter progression (theory → service → consumer → UI) and scope.
  - Two factual mismatches between the sections list and the linked files: 7.2 advertises "TDD" but the section does not teach TDD; 7.4 advertises "Docker setup" but the section only briefly mentions Docker Compose in a "Testing the Full Flow" block. Both bullets should be reworded or the content added.
  - Suggestion to convert the ASCII-art architecture diagram into a proper figure in the HTML build, with sync/async flows labelled directly.

## Pass 2: Line Editing
- **Line ~3:** Tighten Tip sentence.
  - Before: "Having sample books in the catalog will make it easier to see events flowing through the system."
  - After: "Sample books in the catalog make it easier to watch events flow through the system."
  - Reason: Remove gerund opener and "will make" filler; switch "flowing" → "flow" for active feel.
- **Line ~27:** Minor word order tweak.
  - Before: "state changes (created, returned, expired) are published as events to Kafka"
  - After: "state changes (created, returned, expired) are published to Kafka as events"
  - Reason: More natural flow; target (Kafka) closer to the verb.

## Pass 3: Copy Editing
- **Line ~3:** Capitalization inconsistency. "Reservation service" here vs. "Reservation Service" in the H1 and sibling headings. Standardize: lowercase in running prose, title case in headings. (CMOS 8.1)
- **Line ~10:** "sarama" → "Sarama" in prose (proper product name); keep lowercase only for code-font `sarama` import identifier. (CMOS 8.153)
- **Lines ~11, 27:** "tradeoffs" — acceptable, but confirm whole-chapter consistency (not mixing "tradeoffs" / "trade-offs").
- **Lines ~25, 27:** Em dash style — CMOS 6.85 prefers unspaced em dash. Note that 7.1 source uses `--` double hyphens (likely rendering as en dash) while this file uses true em dash. Pick one style chapter-wide.

## Pass 4: Final Polish
- **Line ~5:** Cross-reference verify: "CLI tools from Chapter 6" — confirm Chapter 6 covers the admin-seed CLI.
- **Line ~31:** Sections list bullet for 7.2 says "TDD" — not covered in the linked file. Remove or add content.
- **Line ~34:** Sections list bullet for 7.4 says "Docker setup" — not the focus of the linked file. Consider "manual end-to-end testing".
