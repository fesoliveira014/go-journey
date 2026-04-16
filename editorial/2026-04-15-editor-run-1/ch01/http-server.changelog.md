# Changelog: http-server.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Heading level inconsistency — opens at H2 like go-basics.md (not H1 like project-setup.md). Chapter-wide decision needed.
  - main.go duplication across §1.1 and §1.3: legitimate (this section is where the lines are explained), but a cross-reference would reduce reader friction.
  - `sampleBooks` IDs ("1", "2", "3") inconsistent with the ISBN-style IDs in go-basics.md's Book example.
  - Line-by-line annotations for the Health handler are excellent — consider applying the same treatment to `main.go` for symmetry. (Currently you do this partially via sub-headings Environment Configuration / Starting the Server — acceptable.)
  - gRPC is mentioned in passing ("Kafka brokers, gRPC addresses") without prior definition; first mention in chapter should either define or forward-reference.

## Pass 2: Line Editing
- **Line ~3:** Small preposition fix.
  - Before: "no application server to deploy into"
  - After: "no application server to deploy to"
  - Reason: "deploy to" is standard; "deploy into" is awkward here.
- **Line ~58:** Merge for rhythm.
  - Before: "Once you call `Write`, Go automatically sends a `200 OK` if you have not called `WriteHeader` yet. Calling `WriteHeader` after `Write` has no effect and produces a warning in the logs."
  - After: "Once you call `Write`, Go automatically sends `200 OK` if `WriteHeader` has not already been called; calling it afterwards is a no-op that logs a warning."
  - Reason: Two-sentence sequence compresses well and avoids double pronoun "you...you".
- **Line ~94:** More accurate framing for backtick explanation.
  - Before: "The backtick syntax is Go's raw string literal for struct tags."
  - After: "Struct tags live inside Go's raw-string literal (backticks), which is why backslashes in them are literal rather than escaped."
  - Reason: The original phrasing suggests backticks exist for struct tags; they're general-purpose raw strings that happen to suit tags.
- **Line ~172:** Trailing phrase fix.
  - Before: "The encoder handles slices, structs, maps, and primitives — no configuration needed."
  - After: "The encoder handles slices, structs, maps, and primitives out of the box."
  - Reason: More idiomatic than "no configuration needed".
- **Line ~216:** Soften overstatement.
  - Before: "The explicit fallback to `\"8080\"` is the standard Go idiom"
  - After: "The explicit fallback to `\"8080\"` is a common Go idiom"
  - Reason: Multiple idioms exist (envconfig, viper, flag); avoid absolute claim.
- **Line ~226:** Java analogy fix.
  - Before: "...equivalent to Java's try-with-resources pattern for the variable scoping benefit."
  - After: "(Java programmers will recognise the scoping idea from try-with-resources, though the purpose is different.)"
  - Reason: try-with-resources handles automatic resource closing; the analogy overstates the parallel.

## Pass 3: Copy Editing
- **Line ~17:** `javax.servlet.Servlet` — Jakarta EE renamed to `jakarta.servlet.Servlet` (2019). Optional modernization.
- **Line ~127:** "the constant `405`" → "the integer constant 405" for precision (optional).
- **Line ~130:** Query: confirm default `json.NewEncoder` output has no whitespace (correct by default).
- **Line ~155:** Query: "Building Microservices" by Sam Newman, 2021 — 2nd edition is 2021. OK; just flagged.
- **Line ~188:** gofmt/goimports grouping of handler import should match §1.1 snippet (blank line between stdlib and third-party/local). Confirm and align.
- **Line ~235:** Prefer `go run ./services/gateway/cmd` (package path) over passing a single file.
- **General:** Footnote style inconsistency with project-setup.md (link-only here vs descriptor + URL there). Pick one.
- **General:** Confirm em-dash style (with/without spaces) matches chapter house decision.

## Pass 4: Final Polish
- No typos, doubled words, or broken homophones detected.
- Heading hierarchy inconsistency noted (H2 vs H1 across files) — cold read flags this clearly.
