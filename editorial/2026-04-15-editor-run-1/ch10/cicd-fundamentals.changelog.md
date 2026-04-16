# Changelog: cicd-fundamentals.md

## Pass 1: Structural / Developmental

- 6 comments. Themes:
  - Progression CI → CD → Continuous Deployment → Feedback Loop → Reproducibility → Two-Tool → Exercises is logical and well paced.
  - The five-stages claim (pre-diagram) conflicts with the four-stage diagram. Recommend stating "four stages" or relabeling "push" as stage 0.
  - Factual inconsistency: text references `earthly +publish` (line 118) but no such target exists elsewhere in the chapter. The GHA workflow actually calls `earthly +ci` and `docker/build-push-action`. Must be fixed.
  - JVM analogies placed well; occasionally a little late in the paragraph — fine to leave.
  - Exercises are strong and varied for an experienced engineer.
  - Theory-only framing delivered; no code clutter, which matches the section's purpose.

## Pass 2: Line Editing

- **Line ~3:** Tighten opener.
  - Before: "Before we configure a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is and why the two-tool approach in this chapter works the way it does."
  - After: "Before configuring a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is — and why the two-tool approach in this chapter works the way it does."
  - Reason: Active participle; em dash splits the two ideas.

- **Line ~116:** Rewrite for tighter rhythm.
  - Before: "The practical benefit: when a CI build fails, you can reproduce it locally with `earthly +test`. You do not need to push a debug commit, wait 3 minutes for a runner to spin up, and read logs in a browser."
  - After: "The practical benefit: when a CI build fails, you reproduce it locally with `earthly +test`. No debug commit, no three-minute wait for a runner, no reading logs in a browser."
  - Reason: Cuts 6 words and reinforces the parallel "no … no … no" structure.

- **Line ~118:** Fix factual error.
  - Before: "It installs Earthly, calls `earthly +lint`, `earthly +test`, and `earthly +publish`."
  - After: "It installs Earthly and calls `earthly +ci`."
  - Reason: `+publish` target is not defined; pipeline actually calls `+ci` and then `docker/build-push-action`. Publish happens outside Earthly.

## Pass 3: Copy Editing

- **Throughout:** File uses `--` (two hyphens) as a dash. Convert to em dash `—` (no surrounding spaces) per CMOS 6.85, to match index.md and image-publishing.md.
- **Line 74:** "costs 5 minutes" / "costs 10 seconds" — CMOS 9.7 prefers spelled-out numbers under 100 in prose. Suggest "five minutes" / "ten seconds". (Retain numerals where paired with explicit units in technical contexts, e.g., "5-minute test suite" — compound adjective with unit.)
- **Line 91:** "Earthly, which we cover in section 10.2" — capitalize "Section" for cross-reference per internal style (image-publishing.md uses "Section 10.4"). CMOS 8.179 allows both; pick one.
- **Line 103:** Please verify: "Go 1.22 and 1.23" — book elsewhere (earthly.md line 49) uses `golang:1.26-alpine`. Inconsistency; update example to Go 1.25/1.26.
- **Line 84:** Please verify: golangci-lint versions `1.54` and `1.62` are both real published releases.
- **Line 116:** "wait 3 minutes" → "wait three minutes" (CMOS 9.7).
- **Line 130:** "file system" (two words) vs "filesystem" (one word, line 88). Choose one and apply consistently.
- Serial commas consistent throughout (CMOS 6.19). Good.
- Compound adjectives hyphenated correctly: "known-good", "low-cost", "quality-of-life" (CMOS 7.81). Good.
- Acronym definitions on first use (CI, CD, VM, SHA, EKS): all correct except VM is never defined (acceptable for this audience).
- References section: CMOS-compliant author-title-publisher citation for Humble & Farley (14.100). Good.

## Pass 4: Final Polish

- **Line ~118:** Factual error — `earthly +publish` does not exist. See Pass 2. Highest-priority fix.
- **Line ~48 / diagram:** "Our pipeline has five stages" precedes a four-arrow diagram. Either "four stages" or "five, including the push trigger" — pick one.
- No doubled words or typos detected.
- Homophones: none misused.
