# Changelog: otel-fundamentals.md

## Pass 1: Structural / Developmental
- 8 comments. Themes:
  - Attribution of the "ask arbitrary questions" framing (Majors/Honeycomb).
  - "What OpenTelemetry Is" and "The OTel API/SDK Split" subsections duplicate the SLF4J/Logback analogy; consolidate.
  - Section ordering: "How OTel Fits in the CNCF Landscape" would read more naturally BEFORE "OTel Architecture in Our System", not after.
  - Table row conflates Gauge and UpDownCounter — technically inaccurate; OTel treats them as distinct instruments.
  - Metrics feedback-loop synthesis paragraph could be repositioned as the takeaway at the end of Three Pillars.
  - Exercises ladder is good; could add one bridging to 9.2.

## Pass 2: Line Editing
- **Line ~3:** replace dash with semicolon OR normalize to em dash (see Pass 3)
  - Before: "is not about installing a library -- it is about designing your system"
  - After: "is not about installing a library; it is about designing your system"
  - Reason: reduces dash density; readers with screen readers benefit from punctuation variety.
- **Line ~13:** cut redundancy
  - Before: "Metrics are aggregated numerical measurements"
  - After: "Metrics are aggregated measurements"
  - Reason: "numerical" is implied by "measurements" in the list that follows.
- **Line ~15:** loosen the noun
  - Before: "Logs are timestamped text records emitted by your application."
  - After: "Logs are timestamped records emitted by your application."
  - Reason: structured logs may be JSON or protobuf, not "text".
- **Line ~27:** anchor the vague time reference
  - Before: "(as of recently) logs"
  - After: "(as of the OTel 1.x SDK series) logs"
  - Reason: "recently" ages; version anchor survives.
- **Line ~67:** tighten the sentence; add SI spacing
  - Before: "The total request took 45ms, and you can see that 12ms of that was the database query."
  - After: "The request took 45 ms total — 12 ms of which was the database query."
  - Reason: active voice, fewer words, correct SI unit spacing.
- **Line ~87:** split long sentence
  - Before: "In the Java world, Spring Cloud Sleuth (now Micrometer Tracing) handled this transparently via servlet filters and RestTemplate interceptors. OTel in Go works the same way -- the contrib packages (otelhttp, otelgrpc) inject and extract context without manual intervention."
  - After: two sentences broken at "OTel in Go is similar:"
  - Reason: 42 words; splitting improves scanability.
- **Line ~101:** precision word swap
  - Before: "the concepts are identical"
  - After: "the semantics are identical"
  - Reason: "semantics" is the technical term when comparing instruments.
- **Line ~105:** cut phatic opener
  - Before: "A design decision worth understanding:"
  - After: "The OTel API/SDK split is worth understanding."
  - Reason: the header already signals this; the colon-lead feels stagey.
- **Line ~165:** split long sentence
  - Before: "Our stack uses the Grafana family (Tempo, Loki, Grafana) plus Prometheus. This is a common open-source choice. In production, you might use managed equivalents..."
  - After: merge first two sentences with em dash: "... plus Prometheus — a common open-source choice."
  - Reason: 55-word compound reduces to 2 tighter sentences.
- **Line ~167:** invert passive
  - Before: "The fact that OTel is vendor-neutral is its defining feature."
  - After: "OTel's defining feature is that it is vendor-neutral."
  - Reason: cuts "The fact that" filler.

## Pass 3: Copy Editing
- **File-wide:** all `--` double-hyphen sequences should be em dashes without spaces (CMOS 6.85). Recurring, not individually enumerated.
- **Line ~11:** "sub-operations" — CMOS 7.89 closes "sub-" prefixes; consider "suboperations". Flagged, not changed, for author preference.
- **Line ~17:** "catalog-to-PostgreSQL" — hyphenated compound adjective before "span", correct per CMOS 7.81.
- **Line ~19:** MDC first-mention — consider gloss "MDC (Mapped Diagnostic Context)" for readers who used Logback but not by name.
- **Line ~25:** CNCF first-mention — consider spelling out "Cloud Native Computing Foundation (CNCF)".
- **Line ~27:** Please verify — OTel logs signal stability: Go SDK log API (`go.opentelemetry.io/otel/log`) status at publication.
- **Line ~49–56:** numerals + SI units: "128-bit", "64-bit" correctly hyphenated (CMOS 7.81).
- **Line ~67:** 45ms, 12ms etc. — SI recommends space between numeral and unit (CMOS 9.17 / SI brochure). Recommend "45 ms".
- **Line ~96:** Table conflates Gauge and UpDownCounter. Please verify and split into two rows — Gauge is observed; UpDownCounter is incremented.
- **Line ~99:** Please verify — metric name `http_server_request_duration_seconds`. Current otelhttp emits `http.server.request.duration` in seconds; Prometheus exporter converts to underscore form. Clarify the layer being named.
- **Line ~109:** Please verify — `go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/exporters/...` package paths are current.
- **Line ~117:** Footnote placement inconsistent — [^5] placed after "specification" term, but [^1] follows the sentence. Normalize: all footnotes go at the end of the sentence.
- **Line ~157:** Please verify claim — "second most active CNCF project after Kubernetes". True per 2023–2024 CNCF velocity; confirm for 2026.
- **Line ~157:** Please verify — "'Graduated' status for tracing and metrics, with logging in 'Stable' as of late 2024".
- **Line ~165:** "open-source" — hyphenated as compound adjective; correct (CMOS 7.81).
- **Line ~189 (Ref [^3]):** Please verify URL `https://opentelemetry.io/docs/languages/go/` resolves to the current getting-started page.

## Pass 4: Final Polish
- **Line ~69:** "Spans also carry a status." — minor repetition of earlier bullet. Reframe suggested in Pass 1.
- **Line ~175 (Exercise 1):** "the `01` flags field" — "flags" singular usage is mildly awkward; "flags byte" or "flags field (`01`)" reads better. Minor.
- No typos, doubled words, or broken cross-refs detected. Cross-ref to section 9.2 ("Kafka is the exception ... section 9.2") and section 9.5 resolve to existing files.
