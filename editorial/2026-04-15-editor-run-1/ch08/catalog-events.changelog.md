# Changelog: catalog-events.md

## Pass 1: Structural / Developmental
- 5 comments. Themes: opening motivates pattern well; sections flow logically (struct → interface → CRUD → publisher → topic/keying/tracing → exercises); one possible fact-order bug flagged in the Trace Propagation snippet (Inject before Start span). The `UpdateAvailability` mention at end of "Publishing Events from CRUD Operations" could either be shown in code or left as a bridge — kept as bridge.

## Pass 2: Line Editing
- **Line ~3:** Suggested trim (not applied to preserve voice): "other parts of the system need to know about it" → "other parts of the system need to know." Minor.
- **Line ~37:** Suggested dropping "potentially" from "wasteful and potentially misleading". Voice-preserving; optional.
- **Line ~110:** Flagged 48-word sentence ("The alternative -- using a transactional outbox pattern ...") for optional split into two sentences.
- **Line ~223:** Flagged 57-word sentence in "Topic Naming" section. Suggested split at "simpler:" — kept as a note for author.

## Pass 3: Copy Editing
- **Line ~3, ~146, ~156, ~178, ~184:** Flagged mixed capitalization of "Catalog service" vs "catalog service" across chapter. Recommend lowercase common-noun form in prose (CMOS 8.1). No change applied.
- **Line ~39:** Flagged factual imprecision — "Go's UUID package" does not exist; `github.com/google/uuid` is the community standard. Recommend "a UUID package" or naming the specific import.
- **Throughout:** Flagged `--` usage. Toolchain likely renders as em dash; if not, CMOS 6.85 calls for em dashes (—) with no spaces. Consistent within chapter.
- **Line ~184:** Fact-check note — "Sarama, the most widely used Go Kafka client library" is historically true but contested today (`franz-go` is increasingly preferred). The repo's own recent commit (37c217a) recommends evaluating franz-go. Consider softening to "the long-standing and widely used Go Kafka client library."
- **Line ~211:** "in-sync replicas" correctly compound-hyphenated before noun (CMOS 7.81). No change.
- **Line ~250:** "end-to-end" correctly hyphenated (CMOS 7.81). No change.

## Pass 4: Final Polish
- **Line ~244-247:** Flag a possible source-order issue: the `Inject(ctx, ...)` line appears before `ctx, span := ...Start(...)`. If this mirrors the real file, it is an observability bug (injecting the parent trace context rather than the publish span's). Needs fact-check against `services/catalog/internal/kafka/publisher.go`. Query, not a change.
- No typos, doubled words, missing words, or broken cross-refs found. Cross-reference to section 8.3 for bootstrap verified.
