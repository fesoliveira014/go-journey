# Changelog: event-driven-architecture.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - **Cross-section consistency with 7.2 (critical).** The "Our Event Flow" numbered list describes step 2 as a "synchronous read" (GetBook-style). Section 7.2 explicitly calls this pattern wrong and documents the real implementation as a *guarded decrement* via `catalog.UpdateAvailability(-1)` (the "TOCTOU trap"). Readers will notice. Recommend rewriting step 2 to reflect the decrement-first pattern and adding a forward reference from 7.1 to the TOCTOU discussion.
  - **"Eventual consistency" framing is weaker than stated.** Since the decrement is synchronous on create, the only eventual-consistency window on write is for returns/expirations. Tighten the claim.
  - **"Neither service directly modifies the other's database"** (Commands vs. Events) is not quite accurate given the sync `UpdateAvailability` path. Rephrase to "Neither service reaches into the other's database — writes go through APIs or events the owning service controls."
  - **Idempotency claim is imprecise.** "A duplicate increment is harmless" contradicts the more precise statement in 7.3 that a duplicate `reservation.returned` can push the count above `total_copies`. Reconcile.
  - Motivation, analogies (Spring @TransactionalEventListener, @KafkaListener), and the Sarama-vs-franz-go sidebar are all well-placed; keep.
  - Exercises have a good gradient. Optional: add a schema-evolution exercise tied to `reservation.extended` to reinforce the tolerant-reader pattern.

## Pass 2: Line Editing
- **Line ~36:** Tighter opener for Commands vs. Events.
  - Before: "Two terms get used loosely in messaging systems, and it is worth distinguishing them:"
  - After: "Two terms get used loosely in messaging systems; the distinction matters."
  - Reason: Cuts filler and breaks the compound sentence.
- **Line ~58:** Redundant adverb.
  - Before: "it is clearly a notification of something that already happened."
  - After: "it is clearly a notification of something that happened."
  - Reason: "Already" is redundant with past tense.
- **Line ~216:** Parallelism and gerund removal.
  - Before: "The availability count being slightly off is less harmful than losing the reservation."
  - After: "A slightly stale availability count is less harmful than a lost reservation."
  - Reason: Parallels the two noun phrases; removes the "-ing" clause.
- **Line ~121:** Minor smoothness.
  - Before: "Better performance, but requires a C toolchain for building."
  - After: "Better performance, but it requires a C toolchain to build."
  - Reason: Adds the implied subject and uses the infinitive.

## Pass 3: Copy Editing
- **Global (dash style):** File uses ` -- ` (spaced double-hyphen, rendering as spaced en dash). CMOS 6.85 prefers unspaced em dash. Choose a chapter-wide house style. (CMOS 6.85)
- **Lines ~28, ~192:** "side-effect" used as a standalone noun — should be "side effect" (open compound). Hyphenate only when used attributively before a noun. (CMOS 7.81, 7.89)
- **Line ~64:** "Unlike traditional message queues (RabbitMQ, ActiveMQ) where messages are consumed and deleted, Kafka is..." — add a comma after the closing parenthesis: "...(RabbitMQ, ActiveMQ), where messages are consumed and deleted, Kafka is..." (CMOS 6.27, non-restrictive clause.)
- **Line ~96:** "re-delivers" → "redelivers" (closed prefix). (CMOS 7.89)
- **Line ~98:** Accuracy — "commits offsets explicitly by calling session.MarkMessage" slightly overstates what MarkMessage does. Suggest: "marks offsets explicitly … Sarama batches and commits the marks in the background."
- **Line ~110:** Please verify — "Since Kafka 3.3" (KRaft GA for new clusters) and "ZooKeeper support was removed entirely in Kafka 4.0" (verify against the 4.0 release notes). 
- **Line ~124:** Please verify — "Sarama is in maintenance mode. IBM still takes security patches and critical fixes." Confirm a citable source (IBM/sarama README status block or recent release note).
- **Line ~128:** Please verify — anchor `#comparisons` in https://github.com/twmb/franz-go#comparisons.
- **Line ~152:** Precision — "all in-sync replicas have written the message" → "...have acknowledged the write" (matches ISR acknowledgment semantics).
- **Line ~192:** "Steps 1-4" — use en dash for the number range: "Steps 1–4". Same for any other hyphen-for-range usage. (CMOS 6.78)
- **Line ~247:** Please verify — OpenTelemetry Go package alias. The snippets use `otelgo`; most repos alias the package as `otel` (`go.opentelemetry.io/otel`). Confirm against the actual code and normalize.
- **Heading case:** H3 "What happens when publishing fails?" uses sentence case while sibling H3s ("Topics and Partitions", "Consumer Groups") use title case. Standardize.
- **References block:** " -- " between author and title in footnote entries — standardize to em dash (or a period) for cleaner typesetting.

## Pass 4: Final Polish
- **Line ~23:** Consider moving/bolding "This is where event-driven architecture earns its keep." as a pull quote in the HTML build — it's the thesis of the section.
- **Line ~46:** Code comment path `services/reservation/internal/service/service.go` — verify still accurate vs. current repo layout.
- **Line ~227:** Code comment path `services/catalog/internal/repository/book.go` — verify still accurate.
- **Line ~271:** Exercise 4 references `reservationEvent` (lowercase `r`) — this is the consumer-side struct; confirm this is intentional (otherwise use `ReservationEvent`).
