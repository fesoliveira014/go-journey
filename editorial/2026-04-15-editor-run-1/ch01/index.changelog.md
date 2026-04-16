# Changelog: index.md

## Pass 1: Structural / Developmental
- 2 comments. Themes:
  - Landing page lists deliverables twice (opening bullets and Sections list) without a narrative bridge explaining the order of the four sections.
  - Suggested a single-sentence framing to orient an experienced reader on why this chapter exists before the microservices content begins.

## Pass 2: Line Editing
- **Line ~3:** Tighten opening sentence.
  - Before: "In this chapter, you will set up the project, learn the Go essentials you need for the rest of the tutorial, build a basic HTTP server, and write your first tests."
  - After: "This chapter sets up the project, covers the Go essentials you'll need throughout the tutorial, builds a basic HTTP server, and introduces Go's testing conventions."
  - Reason: Active framing, parallel verb tense, removes "for the rest of the tutorial" throat-clearing.
- **Line ~20:** Reshape the editor-tooling line.
  - Before: "A code editor (VS Code with the Go extension is recommended)"
  - After: "A code editor; VS Code with the official Go extension is recommended."
  - Reason: Parenthetical-buried recommendation becomes a standalone clause; clearer.

## Pass 3: Copy Editing
- **Line ~17:** Factual query — verify "Go 1.26+" is the current stable at book date (2026-04-15). CMOS N/A.
- **Line ~18:** Factual query — confirm earthly.dev/get-earthly URL still resolves (CI docs sometimes point at /download).
- **Line ~32–35:** Parallelism across section descriptions: item 1 uses "and" before last element; items 2, 3, 4 are asyndetic. Make all parallel (prefer inserting "and" before the final element — CMOS 6.19 serial comma).
- **General:** Chapter-wide note — em-dash style inconsistent (spaced em dashes appear throughout; CMOS 6.85 prescribes no spaces). Flag once here; apply house-style decision consistently through all five files.

## Pass 4: Final Polish
- None. File is clean; no typos, doubled words, or broken cross-refs detected.
