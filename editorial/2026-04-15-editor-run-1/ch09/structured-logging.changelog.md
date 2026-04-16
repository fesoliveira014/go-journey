# Changelog: structured-logging.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Opening framing ("Traces and metrics tell you what happened and how fast. Logs tell you why.") is a quotable line — keep.
  - Side-by-side "unstructured vs. structured" JSON contrast is the right teaching technique.
  - The four-step Handle() walkthrough is well-paced.
  - Section pivots (Why → slog package → TraceLogHandler → Testing → Migration → Context Flow) follow a natural learn-then-use sequence.
  - Two ASCII flow diagrams (request side / backend side) are complementary; verify the backend-side diagram's "GORM query" matches the actual database layer (see cross-file GORM/pgx concern).
  - Exercises ladder well.

## Pass 2: Line Editing
- **Line ~3:** tighten double "only if"
  - Before: "But only if you can find the right log lines, and only if they contain enough context to be useful."
  - After: "But only if you can find them and they carry enough context to be useful."
  - Reason: cuts "only if" repetition; stays within the same rhythm.
- **Line ~205:** split 50-word sentence
  - Before: "The test creates a real `TracerProvider` with an in-memory exporter (no network calls). It starts a span, logs through the handler, and checks that the JSON output contains the correct `trace_id` and `span_id`. The in-memory exporter (`tracetest.NewInMemoryExporter`) is part of the OTel SDK's test utilities -- use it whenever you need to verify tracing behavior in unit tests without a live Collector."
  - After: already multi-sentence; consider promoting the last clause to its own sentence for emphasis. Minor.
  - Reason: pacing.
- **Line ~149:** "From this point on"
  - Before: "From this point on, every `slog.InfoContext(ctx, ...)` call goes through our handler."
  - After: "From then on, every `slog.InfoContext(ctx, ...)` call goes through our handler."
  - Reason: marginally tighter; optional.

## Pass 3: Copy Editing
- **File-wide:** `--` double-hyphens should be em dashes without spaces (CMOS 6.85).
- **Line ~44:** Please verify — "Go 1.21 introduced `log/slog`". Confirmed (August 2023 release). OK.
- **Line ~84:** "pre-set" — hyphenated compound adjective (CMOS 7.81); Merriam-Webster lists "preset" as closed. Either acceptable; choose one and apply consistently.
- **Line ~159:** "thread-local map" — compound adjective before noun, correct hyphenation (CMOS 7.81).
- **Line ~162:** Arrow characters `→` are Unicode; used consistently through the chapter. OK.
- **Line ~205:** Please verify — `tracetest.NewInMemoryExporter` resides at `go.opentelemetry.io/otel/sdk/trace/tracetest`. Confirmed as current path.
- **Line ~247:** Please verify — "Compare this to the pre-slog version from Chapter 5". Verify that Chapter 5 actually contains the `log.Printf("%s %s %d %s", ...)` form. If Chapter 5 used a different log form, the cross-ref is misleading.
- **Line ~311:** Please verify — "GORM query  →  creates DB span from ctx". Same cross-file concern as 9.2: confirm GORM vs. pgx in the actual codebase. If pgx, replace GORM with "pgx / otelpgx".
- **Line ~217:** Serial comma in "(method, path, status, and duration)" — correct (CMOS 6.19).
- **Line ~337 (Ref [^4]):** Link text "OpenTelemetry Trace Context in Go" is slightly misleading — the URL anchors to `SpanFromContext`. Retitle "SpanFromContext (go.opentelemetry.io/otel/trace)".

## Pass 4: Final Polish
- **Line ~35:** "~" and "–" characters — none found out of place.
- **Line ~165:** "If you forget the context, the log line is still emitted — it just lacks trace fields." — clean; keep.
- **Line ~314:** "you can trace exactly where the context comes from by reading the code" — clean.
- No typos, doubled words, or homophone errors detected.
- Cross-refs to Chapter 5 flagged for verification (Pass 3).
