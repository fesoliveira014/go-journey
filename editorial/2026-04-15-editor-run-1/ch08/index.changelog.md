# Changelog: index.md

## Pass 1: Structural / Developmental
- 2 comments. Themes: landing page is fit-for-purpose; minor suggestion to consider a Prerequisites note and to label arrows in the ASCII diagram. No restructuring required — this is a short overview.

## Pass 2: Line Editing
- **Line ~3:** Suggested (not applied) "In this chapter we add" → "This chapter adds" for directness. Author voice preserved; kept as optional note.

## Pass 3: Copy Editing
- **Line ~3:** Flagged "gRPC RPCs" as redundant ("RPC" = remote procedure call). Conventional in gRPC community; acceptable. No change.
- **Lines ~3, ~17, ~19:** Flagged inconsistent capitalization of "service" after proper-noun service names ("Catalog service" lowercase vs "Catalog Service" in diagram). Recommend consistency: lowercase "service" after the name in running prose; capitalize only when part of a formal proper-noun label. CMOS 8.1.
- **Line ~9:** "meilisearch-go" lowercase kept — matches library name on GitHub.

## Pass 4: Final Polish
- **Line ~27:** Cross-reference drift. The description for 8.4 says "Gateway search page, autocomplete, Docker setup" but `search-ui.md` does not cover Docker. Either update this line to remove "Docker setup" or add a Docker Compose subsection to 8.4. Recommend removing the phrase unless the omitted Docker content is planned.
