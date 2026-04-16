# Findings: Chapter 8

**Global issues for this chapter:**
- All files use ` -- ` for em dashes. Batch-replace with `—`.
- "tradeoff" → "trade-off."

---

## index.md

### Summary
Reviewed ~30 lines. Clean file. No issues.

---

## catalog-events.md

### Summary
Reviewed ~305 lines (~180 prose, ~100 code). 0 structural, 0 line edits, 1 copy edit. 1 factual query.

### Copy Edit & Polish
- **L218:** "tradeoff" → "trade-off."

### Factual Queries
- **L184:** "The most widely used Go Kafka client library" — Sarama is in maintenance mode (IBM). Section 7.1 already acknowledges this. Qualify: "historically the most popular" or "most widely known."

---

## search-service.md

### Summary
Reviewed ~395 lines (~240 prose, ~130 code). 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L206:** "`@Autowired` an interface" — `@Autowired` is an annotation, not a verb. → "inject via an `@Autowired` interface."
- **L315:** Missing serial comma: "no framework, no annotation, no proxy generation" → "no framework, no annotation, and no proxy generation" (CMOS 6.19).

---

## meilisearch.md

### Summary
Reviewed ~545 lines (~300 prose, ~200 code). 0 structural, 2 line edits, 1 copy edit. 0 factual queries.

### Line Edits
- **L3:** "out of the box" → "by default" or "without configuration" — cliche.
- **L390:** "belt-and-suspenders" → "defense-in-depth" or "redundant" — informal idiom.

### Copy Edit & Polish
- **L228:** "`interface{}`" in prose should be in backticks for consistency: `` `interface{}` ``.

---

## search-ui.md

### Summary
Reviewed ~305 lines (~170 prose, ~110 code). 0 structural, 2 line edits, 1 copy edit. 0 factual queries.

### Line Edits
- **L4:** "without writing any JavaScript" — the nav bar code includes inline JS handlers. → "with minimal JavaScript" or "without a JavaScript framework."
- **L184:** "In a React or Angular application, you would implement this debounce with rxjs operators" — RxJS is more associated with Angular than React. → "In Angular, with RxJS operators (`debounceTime`, `filter`, `switchMap`); in React, with a custom hook or `useDebouncedCallback`."

### Copy Edit & Polish
- **L181:** "`debounceTime, filter, switchMap`" — add backticks around each: `` `debounceTime` ``, `` `filter` ``, `` `switchMap` ``.
