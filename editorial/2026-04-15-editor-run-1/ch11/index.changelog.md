# Changelog: index.md

## Pass 1: Structural / Developmental
- 6 STRUCTURAL comments. Themes:
  - Chapter roadmap uses wrong numbering (10.x instead of 11.x) — five sub-item typos.
  - Roadmap description of 11.5 contradicts what 11.5 actually covers (roadmap says "drives the scenario through the gateway's HTTP API" for a multi-service flow; 11.5 is a service-level test that explicitly excludes multi-service flows).
  - Opening lists "five-service system" then names only four services; audit whether a fifth service exists.
  - "Tooling overview" mixes libraries and a compiler convention in one flat list — minor taxonomy awkwardness.
  - "The cost model" heading is oversold for the content; consider merging with "The testing pyramid."
  - "Build tags for test separation" duplicates material in 11.2 and 11.4; evaluate whether to keep principle-only here and push examples forward.

## Pass 2: Line Editing
- **Line ~7:** Tighten opening of "This chapter is about..."
  - Before: "This chapter is about the failures that unit tests miss, why they miss them, and what you need to do instead."
  - After: "This chapter covers the failures unit tests miss, why they miss them, and what to do instead."
  - Reason: Remove "is about" and "you need to" filler.
- **Line ~23:** Combine short sentences.
  - Before: "Your unit tests inject a hand-written mock that satisfies the interface. The service logic is thoroughly exercised."
  - After: "Your unit tests inject a hand-written mock that satisfies the interface, thoroughly exercising the service logic."
  - Reason: Improve flow; eliminate passive voice.
- **Line ~25:** Use colon to link.
  - Before: "Now consider the gRPC layer. The `CatalogServer` is tested by..."
  - After: "Now consider the gRPC layer: the `CatalogServer` is tested by..."
  - Reason: Tighter rhetorical link.
- **Line ~39:** Tighten "is a model that describes".
  - Before: "The testing pyramid[^1] is a model that describes the recommended distribution of tests across three levels."
  - After: "The testing pyramid[^1] describes a recommended distribution of tests across three levels."
  - Reason: Remove redundant relative clause.
- **Line ~56:** Split the 41-word closing sentence of Chapter Roadmap summary.
  - Before: "By the end of this chapter, your test suite will be stratified, fast where it needs to be fast, thorough where thoroughness matters, and organized so that CI can run the right level of testing at the right time."
  - After: "By the end of this chapter your test suite will be stratified. It will be fast where speed matters, thorough where thoroughness matters, and organized so CI can run the right level at the right time."
  - Reason: Sentence over 40 words; easier to parse in two.

## Pass 3: Copy Editing
- **Line ~27:** "Protobuf" — standardize capitalization (Google style uses initial cap). (CMOS 7.77)
- **Line ~54:** "state machine transitions" → "state-machine transitions" (CMOS 7.81 compound adjective before noun).
- **Line ~56:** Trailing period after `?".` — CMOS 6.124; delete the extra period.
- **Line ~58:** Introduce "End-to-end (E2E)" on first technical use; standardize later references as E2E (prose) / e2e (code/files). (CMOS 10.6, 10.50)
- **Line ~60:** "fully-deployed application" — hyphen correct before noun; retain (CMOS 7.81).
- **Line ~74:** "Testcontainers-go" vs "testcontainers-go": use "Testcontainers" in prose, `testcontainers-go` in monospace for module name. (CMOS 7.77)
- **Line ~74:** "5–8 seconds" — acceptable with en dash; check consistency with "10–20 seconds" (both OK).
- **Line ~98:** "failsafe" → "Failsafe" (Maven plugin proper noun when in prose).
- **Line ~108:** "testcontainers-java" — either monospace it or rename "Testcontainers for Java". Pick one.
- **Line ~126:** "operating system network stack" → "operating-system network stack" (CMOS 7.81).
- **Line ~152:** Queries for URL/API:
  - Please verify: `testcontainers.GenericContainer` example compiles against current testcontainers-go; the newer API uses per-module `Run` functions.
  - Please verify: `github.com/docker/docker/client` as direct vs transitive import of testcontainers-go.
  - Please verify: URLs in footnotes still resolve.
- **Line ~144:** "BookReserved" event — please verify actual event name used in earlier chapters; ch11/kafka-testing.md uses "reservation.created". Align across sections.

## Pass 4: Final Polish
- **Line ~136–144:** Section numbers 10.1–10.5 must be 11.1–11.5 (chapter is 11). Five instances plus one "from 10.2" reference.
- **Line ~144:** Roadmap paragraph for 11.5 describes a multi-service/gateway flow that contradicts the actual content of 11.5 (service-level, single service). Rewrite to match.
- **Line ~3:** "a working five-service system: catalog, auth, reservation, search, and a gateway front-end" lists four services + gateway = five total if the gateway counts as a service. Reader may miscount. Confirm or clarify.
