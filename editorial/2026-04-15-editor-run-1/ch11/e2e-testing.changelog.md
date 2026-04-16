# Changelog: e2e-testing.md

## Pass 1: Structural / Developmental
- 4 STRUCTURAL comments. Themes:
  - Definition-first approach is strong. The diagram + blockquote definition + contrast-to-11.3 paragraph is textbook-quality.
  - "Earthfile integration" overlaps 11.2's Earthly section. Trim or cross-reference.
  - Root Earthfile aggregate here omits gateway service; 11.2 includes it. Align.
  - 11.5 preserves `t.Skip` in unit tests; 11.2 recommends replacing `t.Skip` with Testcontainers. Reconcile the advice — currently directly contradictory.

## Pass 2: Line Editing
- **Line ~380:** Split 42-word "Step 1 — Create" first sentence.
  - Before: "The gRPC request traverses the auth interceptor (which validates the JWT and extracts the caller's identity), reaches the handler, is validated by the service layer, and is persisted by the GORM repository via a real `INSERT` statement."
  - After: "The gRPC request traverses the auth interceptor, which validates the JWT and extracts the caller's identity. It then reaches the handler, is validated by the service layer, and is persisted by the GORM repository via a real `INSERT`."
  - Reason: Split 42-word sentence with parenthetical at two clauses for clarity.
- **Line ~551:** Split 43-word sentence in "Step 3 uses a helper".
  - Before: "This is the same consumer-side code path that the reservation service uses internally — you are verifying not just that a message was sent, but that it can be received and deserialized by the exact code path a downstream consumer would use."
  - After: "This is the same consumer-side path the reservation service uses internally. You are verifying not just that a message was sent, but that it can be received and deserialized by the exact code path a downstream consumer would use."
  - Reason: Long compound sentence reads better split.
- **Line ~704:** Drop "very".
  - Before: "Every file under `internal/e2e/` carries `//go:build integration` at the very top..."
  - After: "Every file under `internal/e2e/` carries `//go:build integration` at the top..."
  - Reason: Filler.

## Pass 3: Copy Editing
- **Line ~15:** "10.2 through 10.4" — typo; must be "11.2 through 11.4".
- **Line ~45:** "section 11.3" then "Section 11.3" (next line) — normalize case for cross-references throughout chapter. Recommend lowercase "section 11.x". (CMOS 8.180)
- **Line ~386:** "Step 5 and 6" → "Steps 5 and 6" (parallel with "Steps 2 and 3" above).
- **Line ~705:** "very top" → "top" (filler). (CMOS tight-prose guideline)
- **Line ~729:** "test result cache" → "test-result cache" (CMOS 7.81 compound adjective).
- **Line ~779:** "each with their own" — number/possessive agreement issue. "each ... its own" is more consistent with prior singular subjects. Consider "each with its own Docker-daemon scope" (and hyphenate "Docker-daemon scope" as compound adjective). (CMOS 5.36)
- **Line ~827:** "60 to 120 seconds" — reconcile with index.md's "30–120 s per scenario". Pick one range.
- **Line ~840:** "`request → DB → event`" — Unicode arrow. Earlier sections use ASCII `->`. Normalize one style.
- **Line ~190:** Please verify: `interceptor.NewAuthInterceptor(jwtSecret)` API vs 11.3's `pkgauth.UnaryAuthInterceptor(jwtSecret, nil)` — pick one or reconcile.
- **Line ~218:** Please verify: `grpc.DialContext` is deprecated in grpc-go 1.64+. Use `grpc.NewClient("passthrough:///bufnet", ...)` to match 11.3.
- **Line ~218:** Please verify: `grpc.ErrServerStopped` identifier currency.
- **Line ~742:** Please verify: `golang:1.22-alpine` base image — bump to current if applicable.
- **Line ~800:** Please verify: `earthly/actions-setup@v1` and pin version (avoid `version: latest`).
- **Line ~797:** Missing `--allow-privileged` flag that 11.2 notes is required; add or explain omission.
- **Line ~819:** Please verify: "BookCreated" event flow — 11.4 models reservation → catalog (availability); catalog → reservation for BookCreated would be a different direction. Align event nomenclature across chapter.
- **Line ~854:** Please verify: Pact citation — footnote currently points to Sam Newman's testing patterns page, not Pact's own docs. Consider adding dedicated Pact footnote.
- **Line ~856:** Please verify: footnote URLs still resolve.

## Pass 4: Final Polish
- **Line ~15:** "10.2 through 10.4" typo → "11.2 through 11.4".
- **Line ~190:** `setupPostgres` helper imports `"github.com/yourorg/library/services/catalog/internal/repository"` but the function body does not use it. Either remove or note it is consumed by other helpers in the same file.
- **Line ~167:** "Unlike PostgreSQL, where we used `GenericContainer`, Kafka uses the Testcontainers Kafka module..." — factually wrong: `setupPostgres` uses `tcpostgres.Run` (the Postgres module), not `GenericContainer`. Rewrite.
- **Line ~386:** "Step 5 and 6" — should be "Steps 5 and 6".
- **Line ~422:** `mockCatalogClient` embeds `catalogpb.UnimplementedCatalogServiceServer` but is used as a `catalogpb.CatalogServiceClient`. The client and server interfaces differ; code may not compile. Please verify and fix.
- **Line ~425:** `grpc.CallOption` is referenced but `"google.golang.org/grpc"` is not imported in this code block. Add import.
- **Line ~447:** `startReservationServer` helper is referenced but never defined in the chapter. Either add a parallel definition or cross-reference to `startCatalogServer` explicitly.
- **Line ~489:** `consumeOneEvent(t, brokers, "reservations")` is referenced but not defined. Either provide the helper or add a sidebar.
- **Line ~580:** `pkgauth` and `uuid` are used in the auth Step 6 but not imported in the shown import block. Add.
- **Line ~584:** `startAuthServer(t, svc)` — signature takes two args while `startCatalogServer(t, svc, jwtSecret)` takes three. Show `startAuthServer`'s definition or explain where JWT secret is bound.
- **Line ~696:** Direct contradiction with 11.2 — 11.2 recommends replacing `t.Skip`, 11.5 says leave it. Reconcile.
