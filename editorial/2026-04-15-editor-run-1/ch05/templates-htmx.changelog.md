# Changelog: templates-htmx.md

## Pass 1: Structural / Developmental
- 6 comments. Themes: (1) Thymeleaf analog is well-motivated for Java-background reader; (2) "subtle gotcha" teaser + payoff in Clone-per-Page section works; (3) three-attribute HTMX table is ideal first-exposure pedagogy; (4) motivated example (user story before code) for catalog filter; (5) dual-mode handler (full page vs fragment) is the canonical HTMX server pattern; (6) fit / not-fit pairing at the end gives honest risk framing.

## Pass 2: Line Editing
- **Line ~1:** Mild filler in opener.
  - Before: "a server-side template engine that produces HTML"
  - After: "a server-side template engine for HTML output"
  - Reason: "that produces" is throat-clearing.
- **Line ~5:** Filler phrase.
  - Before: "explains a subtle gotcha that will bite you if you do not know about it"
  - After: "explains a subtle gotcha"
  - Reason: "that will bite you if you do not know about it" is padding.
- **Line ~22:** Florid superlative.
  - Before: "This is the single most confusing thing about Go templates for newcomers."
  - After: "This is the most common source of confusion for newcomers."
  - Reason: measured, less breathless.
- **Line ~26:** Filler.
  - Before: "you do not need to remember to call an escape function on every variable"
  - After: "no per-variable escape call is needed"
  - Reason: cuts "need to remember to" filler.
- **Line ~325:** Cut filler per style rules.
  - Before: "the page works without JavaScript (you just get full-page reloads on filter changes)"
  - After: "the page works without JavaScript (you get full-page reloads on filter changes)"
  - Reason: "just" flagged in pass-2 style rules.

## Pass 3: Copy Editing
- **Line ~26:** "XSS" — consider spelling out "cross-site scripting (XSS)" on first use (CMOS 10.2).
- **Line ~45–47:** Please verify: HTMX 2.0.4 is a published release; confirm https://unpkg.com/htmx.org@2.0.4 resolves and the `integrity` SRI hash matches the hash published at htmx.org/docs/#installing for that version.
- **Line ~60:** Contractions "doesn't", "you'd" — chapter otherwise uses uncontracted forms ("do not", "is not"). Lock one register. Recommend uncontracted for consistency.
- **Line ~106:** "e.g.," — comma correctly follows (CMOS 6.43).
- **Line ~124:** "The Clone-per-Page Problem" heading vs. "clone-per-page pattern" in prose — lock one form (all lowercase-hyphenated in headings per chapter style, or title-case; currently mixed).
- **Line ~217:** "14KB" — space between number and unit preferred (CMOS 9.16). Recommend "14 KB".
- **Line ~217:** Please verify: HTMX minified+gzipped is ~14 KB per current htmx.org marketing.
- **Line ~223–225:** Please verify: `hx-get`, `hx-post`, `hx-target`, `hx-swap` are the exact HTMX attribute names as documented at htmx.org/docs. Confirmed.
- **Line ~283:** Please verify: HTMX sends `HX-Request: true` header on AJAX requests. Confirmed per HTMX docs.
- **Line ~325:** "SPA" — spell out "single-page application (SPA)" on first use (CMOS 10.2).
- **References:** Please verify URLs resolve (htmx.org/docs/, pkg.go.dev/html/template, hypermedia.systems, gowebexamples.com/templates/).

## Pass 4: Final Polish
- No typos, doubled words, or missing words found. Code fences balanced, inline-code backticks balanced. Footnote marker `[^1]` in prose matches reference list. Cross-reference "section 5.3" in neighbor file — check that this section does not forward-reference beyond what is delivered (none found).
