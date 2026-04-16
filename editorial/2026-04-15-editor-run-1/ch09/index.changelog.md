# Changelog: index.md

## Pass 1: Structural / Developmental
- 3 comments. Themes: missing motivation/"why now" sentence; missing section-list/"Sections" navigation; scope bullet list lacks a sidecar/Collector bullet that mirrors the actual 9.5 section.

## Pass 2: Line Editing
- **Line ~3:** tighten listing preposition
  - Before: "add end-to-end observability to the library system using OpenTelemetry and the Grafana stack"
  - After: "add end-to-end observability to the library system with OpenTelemetry and the Grafana stack"
  - Reason: "with" reads cleaner when enumerating tools.
- **Line ~8:** sharpen verb for active tone
  - Before: "Setting up OpenTelemetry in Go microservices"
  - After: "Wiring OpenTelemetry into Go microservices"
  - Reason: "Setting up" is generic; "wiring ... into" is both more active and more technically specific to dependency injection / SDK registration.
- **Line ~12:** add concrete payoff
  - Before: "Correlating traces with logs for debugging"
  - After: "Correlating traces with logs to debug production incidents"
  - Reason: Ties the skill to an outcome the reader cares about.

## Pass 3: Copy Editing
- **Line ~5:** Heading "What You'll Learn" — headline-style title case (CMOS 8.159); OK.
- **Line ~7–12:** Serial commas present throughout (CMOS 6.19). OK.
- **Line ~9:** "HTTP, gRPC, Kafka, and PostgreSQL" — product capitalization correct per style sheet. OK.
- **Line ~10:** `slog` in inline code — correct per style rule. OK.
- **Line ~3:** "end-to-end" as compound adjective before noun, correctly hyphenated (CMOS 7.81). OK.

## Pass 4: Final Polish
- **End of file:** no typos found; flagged absence of a "Sections" navigation block as a structural, not copy, issue.
