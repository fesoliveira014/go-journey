# Changelog: instrumentation.md

## Pass 1: Structural / Developmental
- 10 comments. Themes:
  - Major factual concern: GORM plugin section may be inconsistent with the codebase's actual database driver. Recent commits reference pgx (see ch04 update commit a902f3d). If services use pgx/sqlx, the GORM section must be rewritten for `otelpgx`. This is the single most impactful concern in the chapter.
  - Section ordering is solid (shared init → auto-instrumentation → manual Kafka → custom metrics → test isolation).
  - The reservation-service parenthetical saves a duplicated code block — good tutor economy.
  - The custom-metric section could back-reference 9.1's Gauge/UpDownCounter table to reinforce the distinction.
  - The "Package Naming Collision" subsection earns its place as a Go-specific pitfall.
  - Opening sentence is 43 words — split recommended.
  - Walkthrough ordering (Resource → TracerProvider → MeterProvider → TextMapPropagator → Collision → Shutdown) could be foreshadowed above the code block.

## Pass 2: Line Editing
- **Line ~3:** split 43-word opening sentence
  - Before: "With the theory from section 9.1 in hand, we now instrument three services: the gateway (HTTP), the catalog (gRPC + Kafka + PostgreSQL), and the reservation service (gRPC + Kafka + PostgreSQL). The pattern is the same each time: initialize OTel once in `main()`, attach auto-instrumentation to transports, and add manual instrumentation where auto-instrumentation does not reach."
  - After: break after "The pattern is the same each time." into a new sentence.
  - Reason: improves scanability at the chapter opener.
- **Line ~82:** cause/effect clarity
  - Before: "new spans are dropped silently. Your application keeps running."
  - After: "new spans are dropped silently, but your application keeps running."
  - Reason: comma + "but" connects the drop behavior to the non-blocking guarantee.
- **Line ~89:** precision verb
  - Before: "flushes metric data every 30 seconds"
  - After: "periodically exports metric data (every 30 seconds)"
  - Reason: "flushes" is informal; "exports" is the OTel term of art.
- **Line ~117:** conversational tightening
  - Before: "without modifying business logic"
  - After: "without touching business logic"
  - Reason: slightly more colloquial and active.
- **Line ~302:** cut hedging opener
  - Before: "Note the `Set` method is a no-op."
  - After: "The `Set` method is a no-op."
  - Reason: "Note" is filler; the sentence is already declarative.

## Pass 3: Copy Editing
- **File-wide:** `--` double-hyphens should be em dashes (no spaces) per CMOS 6.85.
- **Line ~76:** Footnote placement — "[^1]" after "Semantic Conventions" rather than at end of sentence. Normalize to end-of-sentence placement.
- **Line ~76:** Please verify — `semconv.ServiceNameKey.String(serviceName)` vs. newer helper `semconv.ServiceName(serviceName)` in current semconv package versions. Match whichever the actual code uses.
- **Line ~112:** Please verify — `errors.Join` introduced in Go 1.20 (confirmed; OK).
- **Line ~136:** Please verify — current OTel HTTP semantic conventions use `http.request.method`, `http.route`, `http.response.status_code`. The listed attributes (`http.method`, `http.status_code`) are from the pre-stable convention. If the project pins a recent otelhttp version, these should be updated. Please verify against the installed `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` version.
- **Line ~138:** Please verify — metric name `http_server_request_duration_seconds`. In current OTel HTTP conventions the metric is `http.server.request.duration` (in seconds); Prometheus exporter converts to the underscore form. Clarify layer.
- **Line ~171:** "`StatsHandler`, not interceptors" — appositive comma correct (CMOS 6.28).
- **Line ~178:** Please verify — this project's database layer. Recent commits indicate pgx usage (ch04 typed pgx error). GORM + `tracing.NewPlugin()` may be inconsistent. Flag for author confirmation.
- **Line ~185:** Please verify — actual GORM OTel plugin span names. Some versions emit `gorm.query`, others emit operation-specific names like `gorm.Find`, `gorm.Create`.
- **Line ~193:** Please verify — `otelsarama` is at `go.opentelemetry.io/contrib/instrumentation/github.com/IBM/sarama/otelsarama`. The commit history notes Sarama's maintenance status; consider a footnote recommending franz-go migration consistent with prior ch08 guidance.
- **Line ~262:** "`traceparent`" — code font, correct.
- **Line ~340:** "`Int64UpDownCounter`" — correct OTel Go type name.
- **Line ~344:** Please verify — instrument rebinding semantics when the global MeterProvider is set after package init. In Go OTel, global providers use a proxy mechanism that allows post-init rebinding; confirm for the Int64UpDownCounter path.
- **Line ~364:** "push vs. pull" — "vs." acceptable in technical prose.
- **Line ~382 (Exercise 1):** "(services/auth/cmd/main.go)" — inconsistent code-font usage; wrap in backticks for consistency with surrounding paths.
- **Line ~384:** "Meilisearch" — product capitalization (one word, capital M). OK.
- **Line ~388, ~390:** "`docker compose`" (new CLI) vs. legacy `docker-compose` — modern form. OK.
- **Line ~399 (Ref [^4]):** Please verify — link may need replacement with `otelpgx` if pgx is the actual driver.
- **Line ~400 (Ref [^5]):** Misleading link text: "W3C Trace Context in Kafka" points to the generic OTel propagator spec, not Kafka-specific. Retitle to "OTel Context Propagation Specification".

## Pass 4: Final Polish
- **Line ~86:** "Libraries that depend on the OTel API will automatically use it." — clear, no change.
- **Line ~108:** "This is a defensive pattern" — consider "This is defensive" (tighter) but keep for voice.
- **Line ~187:** "The difference is that Go does not have a universal instrumentation agent" — clear statement; keep.
- No typos or doubled words found.
- Cross-refs to 9.1, 9.5 verified.
