# Findings: Chapter 7

**Global issues for this chapter:**
- All files use ` -- ` for em dashes (some mix `--` and `—` within the same file). Batch-replace with `—`.
- "tradeoff" → "trade-off" (multiple instances).
- "serialises" → "serializes" (American English).

---

## index.md

### Summary
Reviewed ~40 lines. Clean file. No issues beyond the global em dash fix.

---

## event-driven-architecture.md

### Summary
Reviewed ~285 lines (~180 prose, ~80 code). 1 structural, 2 line edits, 3 copy edits. 2 factual queries.

### Structural
- **L108–110:** KRaft mode subsection is only three sentences. Consider folding into the preceding material as a callout or note rather than a standalone subsection.

### Line Edits
- **L106:** "which we will return to shortly" → "discussed below" — filler.
- **L152:** "tradeoff" → "trade-off."

### Copy Edit & Polish
- **L86:** "3 partitions and 2 consumers" → "three partitions and two consumers" — spell out per CMOS.
- **L152, L218:** "tradeoff" → "trade-off" — 2 instances.
- **L222:** "serialises" → "serializes" — British/American inconsistency.

### Factual Queries
- **L110:** "ZooKeeper support was removed entirely in Kafka 4.0" — verify this is the intended version (Kafka 4.0 released March 2025; correct).
- **L281:** Reference [^3] cites "Chapter 12" of *Designing Data-Intensive Applications*. Chapter 11 ("Stream Processing") may be the more relevant citation. Verify intended chapter.

---

## reservation-service.md

### Summary
Reviewed ~470 lines (~280 prose, ~160 code). 0 structural, 2 line edits, 2 copy edits. 1 factual query.

### Line Edits
- **L145:** "The ordering is the interesting bit" → "The ordering matters" or "The ordering is the key detail" — "bit" is informal.
- **L461:** "This is not exciting -- it is the point." → fix em dash.

### Copy Edit & Polish
- **L116, L203, L389, L461:** Mixed em dash styles (` -- ` and `—`) within the same file. Standardize all to `—`.
- **L222:** "serialises" → "serializes."

### Factual Queries
- **L414:** "FailedPrecondition becomes 412" — the official gRPC-to-HTTP mapping for `FailedPrecondition` is 400, not 412. The code deliberately maps to 412 in the gateway. Clarify this is a custom mapping, not the gRPC default.

---

## kafka-consumer.md

### Summary
Reviewed ~340 lines (~180 prose, ~140 code). 0 structural, 0 line edits, 0 copy edits. 0 factual queries.

File is clean beyond the global em dash fix.

---

## reservation-ui.md

### Summary
Reviewed ~305 lines (~170 prose, ~110 code). 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L18:** Remaining `--` instance — batch fix.
- **L191:** "`yyyy-MM-dd`" — add backticks since it is a format string: `` `yyyy-MM-dd` ``.
