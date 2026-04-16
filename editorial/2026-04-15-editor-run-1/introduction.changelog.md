# Changelog: introduction.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - **Missing prerequisites section.** Reader has no stated baseline (Go version, OS, local tooling, disk/RAM, AWS account).
  - **Missing "What This Is Not" / scope-exclusion section.** Sets expectations and prevents drop-off.
  - **No time-investment estimate.** Standard for tutorials.
  - **No conventions section.** Code blocks, callouts, file-path notation, diff representation, prompts.
  - **No companion repository pointer.** Critical, since the "How to Use This Guide" section relies on a code-snapshot model that implicitly assumes a tagged repo.
  - **No errata / feedback channel.**
  - **No architecture diagram** despite stating five services with rich interactions; CLAUDE.md commits to producing diagrams.
  - **Snapshot-model paragraph buried** mid-paragraph instead of being promoted to an admonition / callout.
  - **No closing transition** to Chapter 1.

## Pass 2: Line Editing
- **Line ~3:** "Welcome to this hands-on guide to building..." 
  - Before: "Welcome to this hands-on guide to building a complete microservices application in Go."
  - After: "This hands-on guide walks you through building a complete microservices application in Go."
  - Reason: Cuts the empty "Welcome to" opener; leads with the action verb.
- **Line ~3:** "By the end of this tutorial, you will have built a library management system covering:"
  - Before: as written.
  - After: "By the end, you will have built a library-management system that covers:"
  - Reason: "of this tutorial" is redundant; "covering" dangles — replaced with "that covers".
- **Line ~6:** Microservices bullet
  - Before: "Microservices — service decomposition, gRPC, event-driven architecture with Kafka"
  - After: "Microservices — decomposition, gRPC, and event-driven architecture with Kafka"
  - Reason: Removes the redundant "service" before "decomposition"; adds serial comma + "and".
- **Line ~8:** Containers bullet
  - Before: "Containers — Docker, multi-stage builds, Docker Compose"
  - After: "Containers — Docker, multi-stage builds, and Docker Compose"
  - Reason: Adds "and" before final item for parallelism with other bullets.
- **Line ~17:** Reader profile
  - Before: "You are an experienced software engineer who knows how to program but is new to Go and/or cloud-native tooling."
  - After: "You are an experienced software engineer who is new to Go, cloud-native tooling, or both."
  - Reason: "knows how to program" is redundant after "experienced software engineer"; "and/or" is awkward (CMOS 5.250).
- **Line ~17:** Same paragraph
  - Before: "...assumes strong programming fundamentals but explains..."
  - After: "...assumes strong programming fundamentals and explains..."
  - Reason: "but" implies contradiction where none exists.
- **Line ~21:** Project section opener
  - Before: "We are building a **library management system** where:"
  - After: "We will build a **library-management system** where:"
  - Reason: Aligns tense with "you will have built" above.
- **Line ~27:** Architecture statement
  - Before: "The system is decomposed into 5 microservices"
  - After: "The system decomposes into five microservices"
  - Reason: Active voice; CMOS 9.2 number rule.
- **Line ~31:** Snapshot model
  - Before: "Each chapter builds on the previous one. Follow them in order."
  - After: "Each chapter builds on the previous one, so follow them in order."
  - Reason: Combines two short sentences; smoother rhythm.
- **Line ~31:** Comparison sentence
  - Before: "Later chapters modify and extend these files -- so if you compare a snippet from Chapter 2 to the final repository, you will see additions from Chapters 3-9"
  - After: "Later chapters modify and extend these files, so a snippet from Chapter 2 will look different in the final repository — you'll see additions from Chapters 3-9"
  - Reason: Active framing; the comparison happens implicitly.
- **Line ~35:** Theory bullet
  - Before: "Theory — why we are making each decision"
  - After: "Theory — why each decision matters"
  - Reason: Concise; removes weak progressive verb.
- **Line ~37:** Exercises bullet
  - Before: "Exercises — practice problems to test your understanding"
  - After: "Exercises — problems that reinforce the chapter"
  - Reason: Less self-referential; tighter.
- **Line ~38:** References bullet
  - Before: "References — links to official docs and further reading"
  - After: "References — links to official documentation and further reading"
  - Reason: CMOS 5.220 — avoid casual abbreviation in formal prose.

## Pass 3: Copy Editing
- **Line ~3:** "library management system" → "library-management system" — CMOS 7.89, hyphenate compound modifier before noun. Applies on lines 3, 21.
- **Lines ~5-13:** Em dashes are spaced; CMOS 6.85 prefers unspaced. Project-level decision needed; flag for consistency with TOC (which also uses spaced em dashes — internally consistent).
- **Line ~6:** Serial comma — present throughout bullet list. Correct (CMOS 6.19).
- **Line ~13:** "OAuth2 with Gmail" → "OAuth2 with Google". Gmail is a product, Google is the OAuth2 identity provider. Matches TOC section 4.3 ("OAuth2 with Google").
- **Line ~9:** "kind" — consider backticks (`kind`) to signal a tool name. Same for `slog` later.
- **Line ~17:** "and/or" → "or both" (CMOS 5.250).
- **Line ~25:** "email/password" — acceptable per CMOS 6.106 for established pairings; alternative "email and password".
- **Line ~25:** "(CRUD operations)" — CRUD is undefined jargon at first appearance; spell out on first use or add to a glossary.
- **Line ~27:** "5 microservices" → "five microservices" — CMOS 9.2 (spell out whole numbers under 100 in narrative prose).
- **Line ~31:** "--" → "—" — true em dash (CMOS 6.85).
- **Line ~31:** "Chapters 3-9" → "Chapters 3–9" — en dash for inclusive ranges (CMOS 6.78).
- **Line ~31:** "Chapter 2" — uppercase chapter+number is correct (CMOS 8.179).
- **Lines ~6-13, ~35-38:** Bold-em-dash bullet construction is consistent (good).

## Pass 4: Final Polish
- **Line ~31:** Typographic: replace "--" with "—".
- **Line ~31:** Typographic: replace "Chapters 3-9" with "Chapters 3–9".
- **Line ~27:** Number style: replace "5" with "five".
- **Line ~13:** Factual: replace "Gmail" with "Google" (matches TOC 4.3 and is technically accurate).
- No doubled words, no missing words, no homophone errors detected.
- Trailing newline present.
