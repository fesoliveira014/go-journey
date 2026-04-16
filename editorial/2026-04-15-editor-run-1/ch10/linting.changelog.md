# Changelog: linting.md

## Pass 1: Structural / Developmental

- 7 comments. Themes:
  - Progression (go vet limits → meta-linter → per-linter walkthrough → issues → running → patterns → JVM comparison → exercises) is logical.
  - "Why errcheck matters for Go newcomers" framing is pitch-perfect for the target reader.
  - gosimple example #2 (`result := input` as the "simplification") is technically risky because it aliases the backing array — gosimple would not actually suggest it. Replace with a canonical gosimple case.
  - Section 3 heading ("Simplifiable boolean expression (gosimple)") describes a string comparison, not a boolean comparison — rename or replace.
  - Consider mentioning that `exclude-use-default: true` is already the default in golangci-lint and the explicit setting is documentation rather than a behavioral change.
  - `typecheck` as an explicitly enabled linter is slightly misleading; verify that listing it in `linters.enable` has any effect in v1.64.x.
  - JVM comparison table mentions "SpotBugs + FindBugs" — FindBugs is deprecated; SpotBugs is the successor.

## Pass 2: Line Editing

- **Line ~15:** Tighten bullet.
  - Before: "**Unreachable code** -- code after a `return` statement in a branch."
  - After: "**Unreachable code** — statements after an unconditional `return`."
  - Reason: Removes the "code ... code" repetition.

- **Line ~297:** Split 65-word sentence on IDE-plugin contrast.
  - Before: "The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, vim, VS Code), but it is designed to be run as part of the build pipeline -- not just to produce squiggly lines in an editor. This makes it straightforward to block a CI pipeline on lint failures, which is how this project uses it."
  - After: "The key difference from the JVM world is that `golangci-lint` is a command-line tool, not an IDE plugin. It integrates with editors (LSP, Vim, VS Code), but it is designed to be run as part of the build pipeline — not just to produce squiggly lines in the editor. That makes it straightforward to block a CI pipeline on lint failures, as this project does."
  - Reason: Minor tightening; capitalizes "Vim" per vendor convention.

## Pass 3: Copy Editing

- **Throughout:** `--` → em dash `—` (CMOS 6.85).
- **Line 15:** "calling `t.Fatal` from a goroutine, which panics instead of failing the test" — technically imprecise. `t.Fatal` from a non-test goroutine calls `runtime.Goexit`, not panic, and does not fail the parent test. Please verify and refine wording.
- **Line 93:** "SpotBugs and FindBugs" — FindBugs is deprecated (superseded by SpotBugs ~2016). Recommend: "closest to SpotBugs (the successor to FindBugs)".
- **Line 122–124 (second gosimple example):** `result := input` does not preserve slice isolation. Either swap in a canonical gosimple case (e.g., `S1011` append-spread; `S1008` if/else with boolean return) or add a note: "use `slices.Clone` if you need to avoid aliasing." The current example is misleading.
- **Line 187:** "`go install ...@latest`" — conflicts with chapter's pinning theme. Suggest `@v1.64.8`.
- **Line 199:** "tradeoff" — earlier in chapter "trade-off" is used. Prefer hyphenated form for consistency.
- **Line 250 (heading):** "Simplifiable boolean expression (gosimple)" — the code compares a string, not a bool; the rule flagged is `if cond { return true } else { return false }` (S1008). Rename: "Unnecessary if/else around a boolean return (S1008)".
- **Line 291 (JVM table):** "`@CheckReturnValue` + `MustBeClosed`" — second should be "`@MustBeClosed`".
- **Line 297:** "vim" → "Vim" (proper noun).
- **Line 42–60 (config YAML):** Please verify golangci-lint v1.64.x schema:
  - `run.timeout: 5m` ✓
  - `linters.enable: [govet, errcheck, staticcheck, unused, gosimple, ineffassign, typecheck]` — all valid v1 names
  - `issues.exclude-use-default: true` — please verify the exact key name in v1.64 (vs v2 schema changes)
- **Line 162:** `exclude-use-default: true` is default behavior. Consider adding a clarifying parenthetical.
- Serial commas consistent (CMOS 6.19). Good.
- Compound adjectives hyphenated correctly (byte-for-byte, open-source). Good.
- References format: Markdown link form consistent with index/cicd-fundamentals. Good.

## Pass 4: Final Polish

- **Line 15:** Technical nuance on `t.Fatal` panics vs Goexit (see Pass 3).
- **Line 93:** FindBugs vs SpotBugs (see Pass 3).
- **Lines 122–124:** gosimple example (see Pass 3) — highest priority for a technical book.
- **Line 250:** Heading-body mismatch (see Pass 3).
- No doubled words, typos, or broken cross-references detected.
