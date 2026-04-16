# Changelog: sidecar-pattern.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Section heading ("9.5 The Sidecar Collector Pattern") undersells the content; the section actually surveys three deployment models. Suggest broader title.
  - Strong opening anchors new material in the reader's existing setup; preview is well-placed.
  - Bullet/bold-led benefit paragraphs in "Why Not Export Directly" provide good scannability and an appropriate code-to-prose ratio for a conceptual section.
  - The Sidecar ASCII diagram has truncated right-edge labels ("Col"); worth tightening for visual polish.
  - The "Java world" analogy is well-targeted to the audience.
  - "Sampling Strategies" subsection partially overlaps with the "Sampling" benefit bullet earlier; suggest cross-reference rather than full restatement.
  - Comparison table and decision-rule list are well-structured pedagogical anchors.
  - Exercises balance research, hands-on YAML, and conceptual estimation — keep as-is.

## Pass 2: Line Editing
- **Line ~3:** Cut filler in opening paragraph
  - Before: "the simplest possible deployment"
  - After: "the simplest deployment"
  - Reason: "possible" is filler.
- **Line ~3:** Sharpen verb
  - Before: "you will encounter the sidecar and DaemonSet patterns"
  - After: "you will need the sidecar and DaemonSet patterns"
  - Reason: "encounter" is passive/vague; "need" frames the motivation.
- **Line ~9:** Remove redundant adverb
  - Before: "export traces directly to Tempo and metrics directly to Prometheus"
  - After: "export traces directly to Tempo and metrics to Prometheus"
  - Reason: second "directly" is redundant.
- **Line ~17:** Tighten passive
  - Before: "The SDK can only make sampling decisions at span creation time, before the outcome is known."
  - After: "The SDK must decide at span creation time, before the outcome is known."
  - Reason: more direct, removes "can only make … decisions" stack.
- **Line ~19:** Cut filler word
  - Before: "they just export to the Collector's local gRPC endpoint"
  - After: "they export to the Collector's local gRPC endpoint"
  - Reason: "just" is the dictionary case for cut.
- **Line ~48:** Restructure for active voice and concision
  - Before: "Configuration is shared -- you cannot apply different sampling rules to different services without adding label-based routing logic."
  - After: "Configuration is shared: applying different sampling rules per service requires label-based routing logic."
  - Reason: removes negation and passive frame.
- **Line ~179:** Sharper verb
  - Before: "The DaemonSet model is the most common in production Kubernetes deployments."
  - After: "The DaemonSet model dominates production Kubernetes deployments."
  - Reason: cuts auxiliary stack.
- **Line ~179:** Sharper verb
  - Before: "The shared model is fine for development and small production systems."
  - After: "The shared model suits development and small production systems."
  - Reason: replaces vague "is fine for".
- **Line ~209:** Active voice
  - Before: "If the limit is hit, the Collector drops data."
  - After: "If the Collector hits the limit, it drops data."
  - Reason: passive → active, clearer subject.
- **Line ~213:** Split long parenthetical
  - Before: "Kubernetes handles availability (if a node dies, its Collector dies with it, and pods are rescheduled to other nodes with their own Collectors)."
  - After: "Kubernetes handles availability. If a node dies, its Collector dies with it; Kubernetes reschedules pods to other nodes with their own Collectors."
  - Reason: 22-word parenthetical is hard to parse; splitting clarifies.

## Pass 3: Copy Editing
- **Throughout:** Replace " -- " body-text separators with em dash "—" without surrounding spaces (CMOS 6.85). Multiple instances on lines 3, 15, 19, 48, 108, 110, 209.
- **Line ~15:** Verify capitalization of "Jaeger's Thrift", "Zipkin's JSON", "Prometheus's scrape protocol" — all correct (CMOS 8 product names).
- **Line ~17:** "tail-based sampling", "head-based sampling", "rate-based sampling" — correctly hyphenated as compound modifiers (CMOS 7.81).
- **Line ~21:** "e.g.," — comma after "e.g." is correct (CMOS 6.43).
- **Line ~23:** Serial comma "buffering, retry, and translation" — correct (CMOS 6.19).
- **Line ~60, 80, 99, 142:** Please verify: image tag `otel/opentelemetry-collector-contrib:0.149.0` exists and is current.
- **Line ~99–104:** Please verify: K8s sidecar YAML references `volumeMounts` for `name: config` but no `volumes:` block is defined; intentional pedagogical omission or oversight?
- **Line ~128:** Please verify: phrasing "Services find the Collector using the Kubernetes Downward API" — the Downward API supplies host IP; the service then dials it. Phrasing slightly imprecise.
- **Line ~144–146:** Please verify: `hostPort: 4317` is functional but generally discouraged in production Kubernetes; worth noting as a trade-off.
- **Line ~159:** "$(status.hostIP):4317" mixes shell-style syntax with K8s env-var value; clarify that the service dials the env var directly.
- **Line ~189:** Please verify: exact Go SDK identifiers — typical form is `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))`, not bare `ParentBasedSampler` and `TraceIDRatioBased`.
- **Line ~191:** "tail_sampling" processor name — confirmed correct in collector-contrib.
- **Line ~199–207:** Please verify: `memory_limiter` processor fields `check_interval`, `limit_mib`, `spike_limit_mib` — correct in current OTel Collector schema.
- **Line ~223:** Consider expanding "PII" → "personally identifiable information (PII)" if not introduced earlier in ch09.
- **Line ~232:** "tail_sampling" — already in backticks, good.
- **Line ~234:** "256MB" → "256 MB" with space per CMOS 9.16 / SI conventions.
- **Line ~236:** "CRDs (Custom Resource Definitions)" — capitalization correct for K8s.

## Pass 4: Final Polish
- **Throughout:** No doubled words detected.
- **Footnotes:** [^1]–[^4] all defined and referenced; no broken cross-refs.
- **Capitalization consistency:** "OTel Collector" on first reference, "Collector" thereafter — consistent. Good.
- **Throughout:** Confirm whether " -- " in body text renders as intended em dash in the static HTML pipeline; if not, replace globally with "—".
- No typos, missing words, or homophones identified in cold read.
