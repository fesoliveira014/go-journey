# Changelog: kafka-consumer.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - **Nuance about log-and-continue skipping behavior.** The text correctly says a failed message is not marked, but does not mention that marking a *later* message in the same partition advances the committed offset past the failed one — so the failed message is silently skipped on restart, not replayed. Add one clarifying sentence.
  - **Concurrent-decrement claim.** Section 7.1's key-based partitioning guarantees events for the same book land in the same partition, so Kafka-path decrements for one book are serialized by design. The SQL guard's real value is protecting the race between the synchronous `UpdateAvailability` in 7.2 and the consumer. Worth stating explicitly.
  - **Idempotency wording here is more precise than 7.1's.** 7.1 says "a duplicate increment is harmless"; this file correctly says "could increment the count beyond total_copies." Reconcile 7.1 with this text.
  - Add a sentence to the "Running the Consumer" section noting that horizontal scaling triggers Kafka rebalance across replicas.
  - Java snippet is self-contained enough; no change needed.

## Pass 2: Line Editing
- **Line ~3:** Remove filler.
  - Before: "how the catalog service actually consumes reservation events"
  - After: "how the catalog service consumes reservation events"
  - Reason: "Actually" adds no information.
- **Line ~5:** Redundant "there".
  - Before: "If you have used Spring Kafka, consumer setup there is a matter of annotating a method"
  - After: "If you have used Spring Kafka, consumer setup is a matter of annotating a method"
  - Reason: "There" already implied by the conditional.
- **Line ~85:** Drop tutorial preamble.
  - Before: "Let us unpack the configuration:"
  - After: "The configuration, piece by piece:"
  - Reason: Keeps the tutor voice without the "let us" scaffolding.
- **Line ~89:** Correct limiting-modifier placement.
  - Before: "skips all existing messages and only reads new ones"
  - After: "skips existing messages and reads only new ones"
  - Reason: Places "only" next to "new". (CMOS 5.184)
- **Line ~180:** Same — placement of "only".
  - Before: "but it only includes the fields the consumer needs"
  - After: "but it includes only the fields the consumer needs"
  - Reason: CMOS 5.184.
- **Line ~191:** Drop "simple".
  - Before: "The routing logic is a simple switch:"
  - After: "The routing logic is a switch:"
  - Reason: The code shows it's a switch; "simple" is subjective.
- **Line ~229:** Colon over parentheses.
  - Before: "treats both the same way (silently does nothing)"
  - After: "treats both the same way: silently does nothing."
  - Reason: Cleaner punctuation.
- **Line ~313:** Slight word strength upgrade.
  - Before: "every aspect of the behavior is visible in your code"
  - After: "every aspect of the behavior lives in your code"
  - Reason: "Lives" suggests ownership over passive inspection.

## Pass 3: Copy Editing
- **Global (dash style):** ` -- ` (spaced double hyphens) throughout; normalize chapter-wide. (CMOS 6.85)
- **Global (spelling):** "cancelled" (UK) — if US house style is used, change to "canceled" throughout (appears at ~91, ~127, ~282). (CMOS 7.4)
- **Line ~87:** Please verify — Sarama API surface. `sarama.NewBalanceStrategyRoundRobin()` is the factory function; older Sarama versions exposed the strategy as a variable. Confirm the code matches the repo's Sarama version pin.
- **Line ~108:** Please verify — OpenTelemetry Go package alias. The snippets use `otelgo`; most repos use `otel` (from `go.opentelemetry.io/otel`). Confirm and normalize across 7.1, 7.2, and 7.3.
- **Line ~134:** Please verify — default `Consumer.Offsets.AutoCommit.Interval` is 1 second in Sarama. Confirm against current Sarama docs.
- **Line ~193:** ASCII arrow `->` in prose; consider Unicode `→` to match diagrams. Style consistency only.
- **Line ~325:** Please verify — the Exercise 4 hint says `ConsumerGroupSession.HighWaterMarkOffset()`. Sarama exposes `HighWaterMarkOffset()` on `ConsumerGroupClaim`, not `ConsumerGroupSession`. Correct the hint.
- **Line ~319:** "1s, 2s, 4s" — numerals-with-units fine in technical prose; alternative "1, 2, and 4 seconds" is more formal. House call.
- **References:** " -- " in footnote entries — standardize to em dash or period. Verify each URL against current upstream sources (links to confluent.io/blog and opentelemetry.io have changed frequently).

## Pass 4: Final Polish
- **Line ~32:** File path comment `services/catalog/internal/consumer/consumer.go` — verify against current repo layout.
- **Line ~208:** File path comment `services/catalog/internal/repository/book.go` — verify.
- **Line ~293:** Java snippet — `private final CatalogService catalogService;` with no constructor. Readers will assume Lombok or a Spring pattern; consider adding a one-line comment to avoid the question.
- **Line ~250 block:** `consumerHeaderCarrier.Set(key, value string)` — the parameters are unused; Go style would rename them `_ string, _ string` or add a `//nolint:unused` if lint complains. Minor.
