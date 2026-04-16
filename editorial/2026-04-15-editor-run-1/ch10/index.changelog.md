# Changelog: index.md

## Pass 1: Structural / Developmental

- 4 comments. Themes:
  - Chapter opener is concise and well scoped; learning objectives map 1:1 to sibling sections.
  - Diagram contradicts later text: `+build-and-push` is shown as an Earthly sub-target but in 10.5 it is a GHA job using `docker/build-push-action`. Recommend fixing the diagram.
  - Consider a one-line forward pointer at the end naming the five services or linking to their introduction (earlier chapter), and previewing the section order.
  - "production-grade" may over-claim for a learning project lacking signing enforcement, secret rotation, promotion gates.

## Pass 2: Line Editing

- **Line ~3:** Tighten the opening sentence.
  - Before: "Rather than scripting builds directly in YAML, we use Earthly to encapsulate all build logic in portable, cacheable targets — and GitHub Actions purely as the trigger and orchestration layer."
  - After: "Rather than scripting builds directly in YAML, we put all build logic in Earthly targets — portable and cacheable — and use GitHub Actions purely as trigger and orchestration."
  - Reason: "encapsulate" is filler; the revised version is ~5 words shorter and reads more actively.

- **Line ~35:** Minor tightening.
  - Before: "Every step that runs in CI is an Earthly target..."
  - After: "Every CI step is an Earthly target..."
  - Reason: Trim the relative clause.

## Pass 3: Copy Editing

- **Line 1:** Heading uses ampersand ("&"). CMOS 6.14 permits ampersands in titles; verify cross-chapter consistency (other H1s should follow the same convention).
- **Line 3:** "CI/CD" used before definition. CMOS 10.3: spell out abbreviations on first use. The definition lives in 10.1, but the chapter opener precedes it. Consider glossing it here ("continuous integration and continuous delivery (CI/CD)").
- **Line 7:** Serial comma present ("the feedback loop, fast failure, and why reproducibility matters") — CMOS 6.19. Good.
- **Line 8:** Compound adjective "`COPY`-based" hyphenated before noun — CMOS 7.81. Good.
- **Line 11:** Acronym GHCR defined on first use — CMOS 10.3. Good.
- **Line 12:** Em dashes used with no spaces throughout — CMOS 6.85. Good.

## Pass 4: Final Polish

- **Line 35:** Closing paragraph ends abruptly; recommend a one-line forward pointer to the next section.
- No typos, doubled words, or homophones detected.
