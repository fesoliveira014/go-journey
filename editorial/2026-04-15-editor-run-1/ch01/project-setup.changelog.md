# Changelog: project-setup.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Heading and opening hook are strong; good tutor voice.
  - Minor redundancy with section 1.3: the `main.go` snippet is shown here before the reader knows what `ServeMux`, `HandleFunc`, or `ListenAndServe` are, then re-explained in 1.3. Either shorten the snippet here or cross-reference.
  - `pkg/` is conspicuously absent from the layout conventions discussion — experienced readers will expect it called out, even as "we don't use it, here's why."
  - Walkthrough sequencing is correct; exercise is well-scoped and covers every command introduced.
  - Consider a preview sentence in the opening paragraph listing the sub-sections.

## Pass 2: Line Editing
- **Line ~9:** Tighten JVM-analogy sentence.
  - Before: "If you are coming from the JVM world, a Go module is roughly the equivalent of a Maven `pom.xml` or a Gradle `build.gradle`"
  - After: "If you come from the JVM world, a Go module is roughly the equivalent of a Maven `pom.xml` or Gradle `build.gradle` file"
  - Reason: Present tense; remove second indefinite article for parallel series.
- **Line ~41:** Merge two short sentences.
  - Before: "That single command writes `go.mod`. Nothing else is required to start writing Go code in that directory."
  - After: "That single command writes `go.mod`; nothing else is required to start writing Go."
  - Reason: Combines related statements; cuts tail filler ("in that directory" is clear from context).
- **Line ~47:** Remove comma in restrictive clause.
  - Before: "...during local development, without publishing it to a registry first?"
  - After: "...during local development without first publishing it to a registry?"
  - Reason: "without" clause is restrictive; moved "first" for rhythm.
- **Line ~72:** Redundant phrasing.
  - Before: "It is safe to run at any time and idempotent."
  - After: "It is safe to run at any time; the command is idempotent."
  - Reason: Split the two claims; "safe to run at any time AND idempotent" reads awkwardly as parallel predicates with different subjects implied.
- **Line ~82:** Tighten description of cmd.
  - Before: "The `cmd/` directory holds the entry points for executables."
  - After: "The `cmd/` directory holds executable entry points."
  - Reason: Removes two helper words without loss.

## Pass 3: Copy Editing
- **Line ~29:** Query: `go` directive semantics (post-Go 1.21 toolchain directive) — author should confirm phrasing still accurate and optionally mention `toolchain`.
- **Line ~49:** CMOS 6.63 — capitalize after colon when what follows is a full sentence. "each `go.mod` remains self-contained" is a full sentence. Apply a single house style (either capitalize or not) across the chapter.
- **Line ~72:** Factual query — `go work sync` direction: per Go reference, it syncs workspace build list INTO each module's go.mod (push), not fetch missing deps. Author should verify and, if necessary, correct the description.
- **General:** Em-dash style with surrounding spaces is consistent within the file; confirm matches house style decision (CMOS prefers no spaces, 6.85).
- **General:** Bare ``` code fences for go.mod content — acceptable; flag for consistency if highlighter is desired.

## Pass 4: Final Polish
- None. No typos, doubled words, or homophone errors detected. Cross-references (to 1.3) are implicit; consider making explicit.
