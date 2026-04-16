# Changelog: kafka-testing.md

## Pass 1: Structural / Developmental
- 1 STRUCTURAL comment (top of section). Opening four-gap structure parallels the chapter index's three-failure structure — good symmetry.
- Section flow is strong: motivate → setup → producer test → consumer test → capturing indexer → gotchas → summary.
- One content gap: does not mention Sarama's maintenance status (ch10 was recently updated to note this). Add a short footnote or aside pointing to franz-go.

## Pass 2: Line Editing
- **Line ~11 (Bullet 3):** Tighten the "Header propagation" bullet.
  - Before: "**Header propagation.** The reservation publisher injects OpenTelemetry trace context into Kafka message headers via `headerCarrier`. The catalog consumer extracts that context in `ConsumeClaim` before starting a new span. If the header key encoding changes — for example, if sarama serializes header keys differently than expected — the trace is broken and monitoring silently degrades. Only a test that goes through the real serialization path can catch this."
  - After: "**Header propagation.** The reservation publisher injects OpenTelemetry trace context into Kafka headers via `headerCarrier`; the catalog consumer extracts it in `ConsumeClaim`. If sarama ever serializes header keys differently than the consumer expects, traces break and monitoring silently degrades. Only a test through the real serialization path catches this."
  - Reason: Two 50-word sentences merged into three shorter ones.
- **Line ~481:** Split 42-word sentence about OffsetNewest/timing.
  - Before: "If you start the consumer first and produce later, you need `OffsetNewest` — but then the test is sensitive to timing between the goroutine that runs the consumer and the goroutine (your test function) that produces the message."
  - After: "If you start the consumer first and produce later, you need `OffsetNewest`. The test is then sensitive to timing between the consumer goroutine and the producer (your test function)."
  - Reason: Reads more cleanly after split.

## Pass 3: Copy Editing
- **Line ~97:** Introduce "OpenTelemetry (OTel)" on first use; later references to "OTel" then flow naturally. (CMOS 10.6)
- **Line ~196:** "OTel SDK" — acronym introduced above is fine.
- **Line ~199:** "trace ID" — two words; consistent throughout chapter.
- **Line ~216:** "section 11.2" vs 11.3's "Section 11.2" — pick one case convention. Recommend lowercase "section 11.2" throughout (consistent with 11.2's internal cross-references; CMOS 8.180 allows lowercase).
- **Line ~469:** "non-trivial" → "nontrivial" (CMOS 7.85).
- **Line ~516:** Please verify: `testcontainers.WithReuseFlag()` vs `testcontainers.WithReuse()` — current API is `WithReuse()`. Update if confirmed.
- **Line ~41:** Please verify: `confluentinc/confluent-local:7.6.0` image tag — confirm currency as of 2026-04.
- **Line ~521:** "cancelled"/"cancelling" → "canceled"/"canceling" (US).
- **Line ~445:** Heading: "Setting up a Kafka testcontainer" — "testcontainer" reads as common noun but conflicts with chapter's "Testcontainers" brand usage. Consider "Setting up a Kafka container with Testcontainers".
- **Line ~21:** "Confluent local Kafka image" — capitalize: "Confluent Local Kafka image" (Confluent Local is the product name).
- **Line ~540:** "consumer group heartbeat interval" → "consumer-group heartbeat interval" (CMOS 7.81).
- **Line ~539:** "offset state collisions" → "offset-state collisions" (CMOS 7.81) — minor.
- **Line ~545:** Please verify: footnote URLs (golang.testcontainers.org/modules/kafka/, github.com/IBM/sarama) still resolve.
- **Line ~128:** Bullet 3 "OTel" usage — first introduction needed earlier per CMOS 10.6.

## Pass 4: Final Polish
- **Line ~84:** `setup := &testing.T{}` — bare zero-value `*testing.T` is not a supported pattern. `t.Fatalf` on a bare T calls `runtime.Goexit` on an unexpected goroutine, and `t.Cleanup` registers on a T that never gets finalized. Replace with a helper that returns `(brokers, cleanup, error)` and in TestMain use plain `log.Fatalf` + explicit `container.Terminate`. Add text caution: this pattern is a footgun.
- **Line ~267:** `service.NewCatalogService(repo)` is called with one argument, but 11.1 and 11.3 use two (`repo, pub`). Resolve: did `NewCatalogService` signature change, or is this an inconsistency? Please verify.
- **Line ~266:** `sharedDB` is referenced but never declared in any shown `TestMain` block. Either add to the earlier TestMain example or note in prose that a Postgres TestMain sets it up.
- **Line ~447:** `upserted := append([]any(nil), idx.upserted...)` — may not compile because `idx.upserted` is `[]model.BookDocument`, not `[]any`. Use `append([]model.BookDocument(nil), idx.upserted...)` or `slices.Clone(idx.upserted)`. The subsequent `upserted[0].(model.BookDocument)` type assertion becomes unnecessary after the fix.
- **Line ~247:** `payload, _ := json.Marshal(event)` silently discards the error — chapter elsewhere uses explicit checks. Align.
