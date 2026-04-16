# Findings: Chapter 11

**Global issues for this chapter:**
- Import path inconsistency: some files use `fesoliveira014/library-system`, ch11/e2e-testing.md uses `yourorg/library`. Standardize.
- `grpc.DialContext` (deprecated) in e2e-testing.md vs `grpc.NewClient` in grpc-testing.md. Use `grpc.NewClient` throughout.

---

## index.md

### Summary
Reviewed ~145 lines. 0 structural, 2 line edits, 1 copy edit. 0 factual queries.

### Line Edits
- **L3:** "Along the way you wrote" → "Along the way, you wrote" — comma after introductory phrase.
- **L128:** "It is worth calling out explicitly because it shapes" → "It shapes" — flab.

### Copy Edit & Polish
- **L98:** "Maven's Failsafe plugin" → "Maven Failsafe Plugin" — capitalize plugin name.

---

## unit-testing-patterns.md

### Summary
Reviewed ~355 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L348:** "These four patterns cover" → "These five patterns cover" — the summary table lists five rows (table-driven, `t.Run`, `t.Helper()`, `testdata/`, `t.Parallel()`).

---

## integration-testing-postgres.md

### Summary
Reviewed ~365 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L145:** Error values silently discarded in helper code (`sqlDB, _ := db.DB()` etc.), but the equivalent helper in section 11.5 checks them with `t.Fatalf`. Mention that production-quality test helpers should check these errors, or update the code.

---

## grpc-testing.md

### Summary
Reviewed ~270 lines. 0 structural, 0 line edits, 0 copy edits. 1 factual query.

### Factual Queries
- **L88:** `grpc.NewClient("passthrough:///bufconn", ...)` uses the non-deprecated API (grpc-go v1.63.0+). But L218 in e2e-testing.md uses the deprecated `grpc.DialContext`. Standardize to `grpc.NewClient` throughout.

---

## kafka-testing.md

### Summary
Reviewed ~535 lines. 0 structural, 0 line edits, 1 copy edit. 1 factual query.

### Copy Edit & Polish
- **L99:** "does not require group rebalance" → "does not require a group rebalance" — missing article.

### Factual Queries
- **L81–88:** `TestMain` creates a `&testing.T{}` literal to pass to `setupKafka`. A manually constructed `*testing.T` will cause `t.Helper()`, `t.Fatalf()`, and `t.Cleanup()` to panic or misbehave. Better approach: inline the container setup in `TestMain` without `*testing.T`, using `log.Fatal` on error and `defer container.Terminate()`.

---

## e2e-testing.md

### Summary
Reviewed ~855 lines. 1 structural, 0 line edits, 1 copy edit. 2 factual queries.

### Structural
- **L1–4:** "the infrastructure layer declared in the previous section" — but infrastructure manifests are in section 12.4, which comes after section 12.3. This is a forward reference presented as backward. → "declared in the next section" (if this cross-reference exists in this file; otherwise, note for ch12).

### Copy Edit & Polish
- **L98:** Import path uses `yourorg/library` while other chapter files use `fesoliveira014/library-system`. Standardize.

### Factual Queries
- **L655:** `GenerateToken(uuid.New(), "user", "test-jwt-secret", -1*time.Second)` — generating a token with negative duration is unusual. Works because `time.Now().Add(-1*time.Second)` produces a past timestamp, but some JWT libraries reject negative durations at generation. Worth a brief note.
- **L799:** GitHub Actions step uses `earthly/actions-setup@v1` with `version: latest`. Pinning to `latest` in CI is fragile (noted in Ch10). Specify a version.
