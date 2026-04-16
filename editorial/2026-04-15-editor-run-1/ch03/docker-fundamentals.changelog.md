# Changelog: docker-fundamentals.md

## Pass 1: Structural / Developmental
- 5 comments. Themes: JVM analogy is well-maintained and serves the target reader; pedagogical arc (what → images vs containers → layers → multi-stage → why) is sound. Flagged — but did not mandate — reconsidering the position of "Why Containerize Go?" (motivation after mechanics). Placement is defensible; noted as author's call.

## Pass 2: Line Editing
- **Line ~3:** Drop filler.
  - Before: "what containers actually are"
  - After: "what containers are"
  - Reason: "actually" is an empty intensifier.
- **Line ~9:** Replace vague noun.
  - Before: "Containers do the same thing"
  - After: "Containers do the same job"
  - Reason: "thing" is imprecise.
- **Line ~11:** Drop "fundamentally."
  - Before: "This is fundamentally different from a virtual machine"
  - After: "This is different from a virtual machine"
  - Reason: "fundamentally" rarely carries weight; delete filler.
- **Line ~53:** Tighten.
  - Before: "This distinction trips up many newcomers."
  - After: "This distinction matters:" (optional).
  - Reason: Softer, but either works.
- **Line ~55:** Tighten OO framing.
  - Before: "To use object-oriented terms you already know:"
  - After: "In object-oriented terms:"
  - Reason: Shorter; no loss of meaning.
- **Line ~99:** Active voice.
  - Before: "Here, `go.mod` and `go.sum` are copied first and dependencies are downloaded."
  - After: "Here we copy `go.mod` and `go.sum` first, then download dependencies."
  - Reason: Passive → active; also clarifies the sequencing.
- **Line ~164:** Drop "just."
  - Before: "You *could* just `scp` the binary..."
  - After: "You *could* `scp` the binary..."
  - Reason: "just" flagged as filler in editorial rubric.

## Pass 3: Copy Editing
- **Throughout:** Replace `--` with em dash `—` (no spaces) per CMOS 6.85.
- **Line ~143:** Range formatting.
  - Before: "15-20MB instead of 300+MB"
  - After: "15–20 MB instead of more than 300 MB" (en dash for range, spell out "+" per CMOS 6.78, 9.16)
- **Lines ~34, 122, 143, 151–156:** Units should have a (non-breaking) space between number and unit (CMOS 9.16): `~300 MB`, `~5 MB`, `~15 MB`, `~20 MB`. Current concatenated style is common in tech writing but nonstandard per CMOS; author's style call.
- **Line ~144:** "non-root" hyphenated — correct (CMOS 7.89).
- **Line ~156:** "CVEs" — expand on first use: "Common Vulnerabilities and Exposures (CVEs)."
- **Line ~66:** "AWS ECR" — consider expanding to "AWS Elastic Container Registry (ECR)" on first use.
- **Line ~168:** "It works on my machine" — confirm comma placement per CMOS 6.9.
- **Queries (please verify):**
  - `golang:1.26-alpine` timing and release.
  - `Alpine Linux 3.19` — pin vs. latest (3.20/3.21 now exist).
  - "GORM's PostgreSQL driver uses pure Go" — project appears to use `pgx` directly in Chapter 2; this line may have drifted.
  - Java `SecurityManager` is deprecated in Java 17+ (JEP 411). Analogy still works but may confuse modern Java readers.
  - Modern BuildKit does not create a layer for every Dockerfile instruction; consider softening to "most build instructions create a new layer."
  - Reference URLs (all four) — verify canonical Docker docs paths.

## Pass 4: Final Polish
- No typos, doubled words, or homophones detected. All internal references intact.
