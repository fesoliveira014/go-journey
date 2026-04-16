# Changelog: integration-testing-postgres.md

## Pass 1: Structural / Developmental
- 3 STRUCTURAL comments. Themes:
  - Container-startup figures disagree across sections: index.md says 5–8 s; here says "three or more seconds" and "two to four seconds". Pick one baseline.
  - Earthly section overlaps somewhat with 11.5's Earthly section. Audit for duplication.
  - The helper code's bulk error-swallowing with `_` contradicts 11.5's helper which surfaces every error via `t.Fatalf`. Decide on one convention for error-handling strictness in helpers.

## Pass 2: Line Editing
- **Line ~3:** Split 44-word opener.
  - Before: "Unit tests with mocks are fast, isolated, and deterministic. But they can only verify logic that your mock correctly models. As soon as the real behavior of a dependency — a database constraint, a transaction rollback, an index scan — differs from your mock's assumptions, the discrepancy is invisible. This section closes that gap by showing how to run repository tests against a real PostgreSQL instance that is spun up on demand, requires no external setup, and is torn down automatically after the test run."
  - After: (split as suggested in annotation; four shorter sentences with colon-led rephrasing for "This section closes that gap: we run...")
  - Reason: Long fourth sentence; colon tightens.
- **Line ~163:** Drop "very".
  - Before: "The build constraint must be on the very first line..."
  - After: "The build constraint must be on the first line..."
  - Reason: Filler.
- **Line ~179:** Split 51-word sentence.
  - Before: "Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional: `ctx` was created at the start of `setupPostgres` and is no longer in scope when the cleanup runs. Using a fresh context ensures that a cancelled parent context does not prevent the container from being cleaned up."
  - After: "Note that `container.Terminate` takes a fresh `context.Background()` rather than `ctx`. This is intentional. By the time cleanup runs the original `ctx` may be cancelled; a fresh context ensures the container is always torn down."
  - Reason: Split, clarify technical mechanism (closure scope vs cancellation).
- **Line ~183:** Remove doubled "both".
  - Before: "...would otherwise both be referred to as `postgres` in their package identifiers."
  - After: "...would otherwise both be referred to as `postgres` in their package identifiers." (keep 'both' once; delete "both" before "be referred to") — revised: "because the `modules/postgres` import and the GORM Postgres driver would otherwise both use `postgres` as their package identifier."
  - Reason: "both ... both" is a redundancy.
- **Line ~183:** Trim defensive "not `gorm.io/driver/pg`" aside.
  - Before: "GORM's driver is imported from `gorm.io/driver/postgres` — not `gorm.io/driver/pg`, which does not exist."
  - After: "GORM's driver is imported from `gorm.io/driver/postgres`."
  - Reason: Readers are unlikely to invent `pg`; the aside is defensive.

## Pass 3: Copy Editing
- **Line ~65:** `docker-compose up` → `docker compose up` (Compose v2 convention; v1 deprecated). Please verify.
- **Line ~83:** "sub-module" → "submodule" (CMOS 7.85 closed compound).
- **Line ~85:** "test-scope" → "test scope" (noun, no hyphen).
- **Line ~54:** "Postgres" vs "PostgreSQL" inconsistency; pick one rule (first use: PostgreSQL; thereafter: Postgres).
- **Line ~173:** "initialising" / "initialisation" → "initializing" / "initialization" (US spelling).
- **Line ~179:** "cancelled" → "canceled" (US spelling).
- **Line ~264:** "behaviour" → "behavior" (US).
- **Line ~273:** "serialisation" → "serialization" (US).
- **Line ~273:** "non-deterministic" → "nondeterministic" (CMOS 7.85).
- **Line ~267:** "unique violation error code" — consider "unique-violation error code" (compound adjective, CMOS 7.81) for consistency with "error-translation code".
- **Line ~265:** "error translation" → "error-translation" before noun (CMOS 7.81).
- **Line ~163:** Please verify: Go build-constraint placement rule. Go 1.17+ requires a blank line between `//go:build` and `package`. Current prose ("before any blank lines or comments") is inaccurate.
- **Line ~167:** Please verify: "`testcontainers-go`" module path and API. `postgres.Run(ctx, image, opts...)` is current (v0.32+).
- **Line ~265:** Please verify: `pq.Error` example — ch04 was recently updated to use pgx-typed error. Align.
- **Line ~324:** Please verify: Earthly `--allow-privileged` flag against current Earthly 0.8+ docs.
- **Line ~366:** Please verify: all three footnote URLs still resolve.

## Pass 4: Final Polish
- **Line ~146–152:** Helper migration code discards errors via `_`. On failure the following call will panic or silently succeed. Align with 11.5 error-handling style (explicit `t.Fatalf` for each step).
- **Line ~235, 362, and index.md:** Container startup time is inconsistent (index says 5–8 s; here says two to four and three or more). Choose a canonical range.
