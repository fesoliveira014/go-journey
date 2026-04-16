# Changelog: earthly.md

## Pass 1: Structural / Developmental

- 10 comments. Themes:
  - File is long; walkthrough structure (show full file, then target-by-target) is sound. Consider collapsing shared sub-targets in "Service Variations" behind a single summary table.
  - "Auth omits pkg/otel" and "Search omits pkg/otel" decisions surprise a reader who expects every service to emit traces. Add a one-sentence justification or a forward pointer to the observability chapter.
  - Gateway race-detector rationale deserves a sentence ("gateway has the most concurrent handlers; others are primarily I/O-bound, where races are less common") — otherwise the choice looks arbitrary.
  - Forward references like "a topic covered in a later section" should name the section.
  - Exercise 3 text fights the earlier claim that Earthly parallelizes BUILD by default; rephrase as "confirm it" rather than "parallelize it".
  - The `SAVE ARTIFACT … AS LOCAL` pattern is subtle — a sentence on why you want updates to flow back to your checkout (dependency bumps, `go mod tidy` inside the container) would help JVM-background readers.

## Pass 2: Line Editing

- **Line ~3:** Split long opener.
  - Before: "Every team eventually ends up with a build script that works on the CI server but not on your laptop, or vice versa. The usual culprit is environmental differences: different versions of Go, different lint tool versions, missing environment variables, or system dependencies that someone forgot to document."
  - After: "Every team eventually ends up with a build script that works in CI but not locally — or vice versa. The culprit is almost always environmental drift: different Go versions, different lint tool versions, missing environment variables, undocumented system dependencies."
  - Reason: Cuts 17 words; ties "drift" to the later reproducibility theme.

- **Line ~116:** Split a 43-word sentence.
  - Before: "Instead of directly copying `../../gen/go.mod` (which would fail because Earthly's build context is scoped to the service directory), the service Earthfile references artifact targets defined in the root Earthfile."
  - After: "Instead of directly copying `../../gen/go.mod` — which would fail because Earthly scopes each service's build context to its own directory — the service Earthfile references an artifact target defined in the root Earthfile."
  - Reason: Dashes replace the parenthetical for a cleaner read.

- **Line ~141:** Split and tighten.
  - Before: "Notice that `src` produces no `SAVE ARTIFACT`. It is an intermediate target. Its purpose is to give `lint`, `test`, and `build` a common starting point so that each of those targets does not need to repeat the COPY instructions."
  - After: "Notice that `src` produces no `SAVE ARTIFACT`; it is an intermediate target. Its job is to give `lint`, `test`, and `build` a common starting point, so none of them needs to repeat the `COPY` instructions."
  - Reason: Semicolon merges two related thoughts; backticks added for consistency.

- **Line ~395:** Tighten CI-reproducibility sentence.
  - Before: "In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` -- the same command you run locally. There is no CI-specific build script to maintain and no category of 'it worked locally but failed in CI' failures caused by environment differences."
  - After: "In GitHub Actions, the CI job installs Earthly and then runs `earthly +ci` — the same command you run locally. There is no CI-specific build script to maintain. The category of 'it worked locally but failed in CI' failures from environment drift disappears."
  - Reason: Three sentences rather than two long ones; final clause names the payoff.

- **Line ~418:** Fix comma splice.
  - Before: "On a typical development iteration -- editing a `.go` file and running `earthly +test` -- the `deps` layer is served from cache, only `src` and `test` actually execute."
  - After: "On a typical development iteration — editing a `.go` file and running `earthly +test` — the `deps` layer is served from cache; only `src` and `test` actually execute."
  - Reason: Comma splice replaced by semicolon (CMOS 6.22).

## Pass 3: Copy Editing

- **Throughout:** `--` used as a dash; convert to em dash `—` (CMOS 6.85).
- **Line 118:** "Each replace'd module" — non-standard possessive/contraction. Suggest "`replace`d module" with backticks, or "each replaced module". (CMOS 7.70 on coined verbal forms.) Same issue in the `lint` section.
- **Line 141:** "the COPY instructions" — inline `COPY` in backticks for consistency.
- **Line 200:** "typically under 20MB" — CMOS 9.17 prefers "20 MB" with thin space. Technical exception: acceptable without space if used consistently project-wide; verify consistency.
- **Line 322:** "a `-mod` target … `-src` target" — backticks inconsistent earlier ("-mod" without backticks). Apply backticks throughout.
- **Line 326:** Technical correction. Text says `/gen` is the artifact name, but in the `SAVE ARTIFACT /gen gen` syntax the artifact name is the second argument, `gen` (no slash). Please verify and correct.
- **Line 367:** "eight cores" — spelled out correctly (CMOS 9.7).
- **Line 29 (table):** Please verify: Earthly spec `VERSION 0.8` is current recommended spec version at 2026-04-15.
- **Line 49:** Please verify: `golang:1.26-alpine` is a pullable tag at publication date.
- **Line 75, 88, 152, 193:** Please verify: `alpine:3.19` is current; `v1.64.8` for golangci-lint is current.
- **Line 430:** Please verify: Earthly `--remote-cache=<registry>` flag form is current.
- **Exercise 4:** "golang-migrate" → inline-code `golang-migrate` (tool name).
- **References:** Convert bare URL form to Markdown link form for consistency with the rest of the chapter.
- Serial commas consistent (CMOS 6.19). Good.
- Compound adjectives: "layer-based", "bit-for-bit", "wall-clock", "cross-Earthfile", "known-good", "quality-of-life" — all hyphenated correctly before nouns.

## Pass 4: Final Polish

- **Line 326:** Possible factual error on artifact-name syntax (see Pass 3). Needs verification with Earthly docs.
- **Line 258:** "On Alpine, CGO needs `gcc` and `musl-dev` installed, plus `CGO_ENABLED=1` explicitly set." — no issues; "explicitly set" is idiomatic.
- **Line 185:** Technical clarification opportunity on "may not have glibc": Alpine uses musl, not glibc. Current wording is accurate but imprecise; see annotated suggestion.
- No doubled words, typos, or broken cross-references detected in the file itself (forward references are vague; see Pass 1).
