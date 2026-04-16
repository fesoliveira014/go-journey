# Changelog: index.md

## Pass 1: Structural / Developmental
- 4 comments. Themes: good navigational clarity; bullets and lists are parallel; recommend one-line forward/back arc tie-in to situate the chapter between Chapter 2 and whatever follows.

## Pass 2: Line Editing
- **Line ~3:** Keep opener; strong.
- **Line ~16:** Prerequisite parenthetical is friendly; optional rephrase suggested, not required.
  - Before: "Basic terminal comfort (you've been building Go services, so this is a given)"
  - After: "Basic terminal comfort (a given, since you've been building Go services)"
  - Reason: Trims filler and tightens the aside.
- **Line ~28:** Remove intro-to-diagram phrasing.
  - Before: "Here is the container architecture we are building:"
  - After: "The container architecture looks like this:" (or delete entirely)
  - Reason: Filler intro; the diagram stands on its own.
- **Line ~44:** Prefer active voice.
  - Before: "PostgreSQL data is persisted in a named Docker volume so it survives container restarts."
  - After: "PostgreSQL data persists in a named Docker volume so it survives container restarts."
  - Reason: Needless passive.

## Pass 3: Copy Editing
- **Lines ~3, 15, 28, 44, 46, 50–53:** Replace `--` with em dashes (`—`, no spaces) per CMOS 6.85.
- **Throughout:** Verify "hot-reload" (compound adjective, hyphenated) vs. "hot reload" (noun, open) usage is consistent across the chapter.
- **Line 23:** "healthchecks" — confirm one-word usage is consistent with Docker documentation and with the rest of the chapter.

## Pass 4: Final Polish
- No typos, doubled words, or broken cross-references detected. Links to the four sibling files are valid relative paths.
