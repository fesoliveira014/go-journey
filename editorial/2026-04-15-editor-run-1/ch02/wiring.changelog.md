# Changelog: wiring.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Spring-vs-Go DI framing is on target for this audience.
  - Missing production-hardening sidebar: graceful shutdown (SIGTERM handling, `grpcServer.GracefulStop()`). Reader will hit this immediately.
  - "Accept interfaces, return concrete types" idiom is almost made explicit; worth a one-line callout.
  - Error-mapping discussion says "log the original error separately" but the code snippet does not log. Either show the log call or mark as exercise.
  - Capstone exercise (9 steps, all RPCs) is the right way to close the chapter — keep verbatim.
  - Consider mentioning proto3's `optional` keyword alongside FieldMask/wrappers for partial-update workaround.

## Pass 2: Line Editing
- **Line ~22:** Remove banned "just".
  - Before: "Dependency injection in Go is just function calls:"
  - After: "Dependency injection in Go is function calls:"
- **Line ~50:** "key inversion" — buzzwordy.
  - Before: "This is the key inversion:"
  - After: "This is the important detail:"
- **Line ~52:** "trivially debuggable" — overstated.
  - Before: "is trivially debuggable"
  - After: "is easy to debug"
- **Line ~94:** Split 50-word sentence on proto↔domain separation.
  - Before: "Keeping them separate means a change to the proto schema does not cascade into the database layer, and a change to the domain model (adding a field, changing a type) does not automatically break the public API. The conversion functions are the explicit, auditable boundary between those two worlds."
  - After: Split into two shorter sentences; see annotated.
- **Line ~116:** "There is no magic here — just explicit field assignment." — banned "just".
  - Before: "There is no magic here — just explicit field assignment."
  - After: "The function is explicit field assignment, nothing more."
- **Line ~302:** Remove banned "just".
  - Before: "whether you just omitted the field"
  - After: "whether you omitted the field"
- **Line ~443:** Restructure 70+-word "Observation on step 5" paragraph; see annotated.

## Pass 3: Copy Editing
- **Line ~27:** Code-block vertical alignment (`bookRepo    :=` etc.) will be reformatted by `gofmt` — flag that reader should not copy the visual alignment if they want canonical Go style.
- **Line ~81:** "it exposes your API surface" — sharpen to "lets anyone introspect the schema without authentication."
- **Line ~141, 147, 150:** Leading space before `[^3]`, `[^2]` footnote markers (same pattern as service-layer.md). Remove leading spaces.
- **Line ~156:** Installation comment mismatches command (`# Linux (download from GitHub releases)` vs `go install`). Fix labels.
- **Line ~184:** `grpc.reflection.v1alpha.ServerReflection` — verify current protocol version registered by grpc-go.
- **Line ~323:** "2-3 genres" → "2–3 genres" (en dash per CMOS 6.78). Also consider spelling out in prose.
- **Line ~443:** "trade-off" vs "tradeoff" — standardize. Recommend "tradeoff" (closed) for chapter consistency (other files use closed form).

### Factual queries (Please verify)
- **Line ~156:** Is `@latest` the right install convention for a book, or should we pin `grpcurl@v1.9.1`?
- **Line ~184:** Does current grpc-go register `grpc.reflection.v1.ServerReflection` (v1) or `v1alpha`?
- **Line ~212 and step 1 solution:** ISBNs — confirm each maps to the cited edition.
- **References:** Canonical URL for gRPC status codes doc.

## Pass 4: Final Polish
- No typos or doubled words. Consistent em dash usage.
- Cross-reference to "Chapter 7" for inter-service communication is explicit — good.
