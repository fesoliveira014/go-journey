<!-- [STRUCTURAL] Heading "9.5 The Sidecar Collector Pattern" is slightly misleading: the section also covers shared and DaemonSet patterns. Consider "Collector Deployment Patterns" or "Sidecar, DaemonSet, and Shared Collectors". -->
# 9.5 The Sidecar Collector Pattern

<!-- [STRUCTURAL] Strong opening: anchors the new material in the reader's current setup and previews the section. Keep. -->
<!-- [LINE EDIT] "the simplest possible deployment" → "the simplest deployment" (cut filler) -->
<!-- [LINE EDIT] "you will encounter the sidecar and DaemonSet patterns" → "you will need the sidecar and DaemonSet patterns" (more precise; encountering vs. needing) -->
<!-- [COPY EDIT] Replace " -- " (double hyphen) with em dashes "—" without spaces per CMOS 6.85. Three instances in this paragraph. -->
Our system uses a single shared OTel Collector for all services. This is the simplest possible deployment. But as your system grows -- more services, more teams, different sampling needs -- you will encounter the sidecar and DaemonSet patterns. This section explains what they are, when to use them, and how they map to our architecture.

---

## Why Not Export Directly to Backends?

<!-- [LINE EDIT] "A natural question: why not skip the Collector and have each service export traces directly to Tempo and metrics directly to Prometheus?" → "A natural question arises: why not skip the Collector and have each service export traces directly to Tempo and metrics to Prometheus?" (cut redundant "directly") -->
A natural question: why not skip the Collector and have each service export traces directly to Tempo and metrics directly to Prometheus?

<!-- [LINE EDIT] "You can." → keep as one-word punch sentence; effective. -->
You can. The OTel SDK supports direct export to many backends. But the Collector provides several benefits that matter in production:

<!-- [STRUCTURAL] Five bold-led benefit paragraphs work well as a scannable list. Good code-to-prose balance for a conceptual section (no code yet, appropriate). -->
**Buffering and retry.** If Tempo is temporarily unavailable, the Collector buffers spans in memory and retries. Without a Collector, the SDK's internal buffer fills up and drops spans silently. The Collector has a larger, configurable buffer and more sophisticated retry logic.

<!-- [COPY EDIT] "Jaeger's Thrift protocol, Zipkin's JSON format, Prometheus's scrape protocol" — verify capitalization: Jaeger, Zipkin, Prometheus all correct. -->
<!-- [COPY EDIT] "Collector config -- not your application code." → use em dash without spaces (CMOS 6.85). -->
**Protocol translation.** Your services speak OTLP. Your backends might speak something else (Jaeger's Thrift protocol, Zipkin's JSON format, Prometheus's scrape protocol). The Collector handles the translation. When you switch backends, you change the Collector config -- not your application code.

<!-- [COPY EDIT] "10%" and "100%" and "1%" — numerals with % are correct in technical contexts (CMOS 9.18). -->
<!-- [COPY EDIT] "tail-based sampling" — hyphenated compound modifier before noun, correct (CMOS 7.81). -->
<!-- [LINE EDIT] Sentence "These decisions require seeing the full trace, which the Collector can do after receiving all spans." (16 words) — fine. Following sentence "The SDK can only make sampling decisions at span creation time, before the outcome is known." → consider "The SDK must decide at span creation time, before the outcome is known." (active, tighter) -->
**Sampling.** In production, you may not want to keep every trace. The Collector can apply probabilistic sampling (keep 10% of traces), tail-based sampling[^3] (keep 100% of slow or error traces, 1% of everything else), or rate-based sampling. These decisions require seeing the full trace, which the Collector can do after receiving all spans. The SDK can only make sampling decisions at span creation time, before the outcome is known.

<!-- [COPY EDIT] "API keys or TLS certificates" — correct caps. -->
<!-- [COPY EDIT] "Collector -- they just export" — em dash without spaces; also "just" is filler. -->
<!-- [LINE EDIT] "they just export to the Collector's local gRPC endpoint" → "they export to the Collector's local gRPC endpoint" (cut "just") -->
**Credential management.** If your backend requires API keys or TLS certificates, the Collector holds them. Your services do not need access to backend credentials -- they just export to the Collector's local gRPC endpoint.

<!-- [COPY EDIT] "e.g.," — comma after "e.g." correct (CMOS 6.43). -->
**Enrichment and filtering.** The Collector can add attributes (e.g., deployment environment, cluster name), remove sensitive attributes, or drop entire spans that match a filter. This is a central policy enforcement point.

<!-- [STRUCTURAL] The "Java world" paragraph is a strong analogy for the target audience. Keep. -->
<!-- [LINE EDIT] "In the Java world, this pattern is common. Datadog Agent, Elastic APM Server, and New Relic's infrastructure agent all serve the same role: a local process that receives telemetry from your application and forwards it to the backend, handling the complexity of buffering, retry, and translation." → consider splitting the long final sentence; 38 words is borderline acceptable but dense. -->
<!-- [COPY EDIT] Serial comma in "buffering, retry, and translation" — correct (CMOS 6.19). -->
In the Java world, this pattern is common. Datadog Agent, Elastic APM Server, and New Relic's infrastructure agent all serve the same role: a local process that receives telemetry from your application and forwards it to the backend, handling the complexity of buffering, retry, and translation.

---

## Three Deployment Models

### 1. Shared Collector (Our Current Setup)

```
┌──────────┐  ┌──────────┐  ┌──────────────┐
│ Gateway  │  │ Catalog  │  │ Reservation  │
└────┬─────┘  └────┬─────┘  └──────┬───────┘
     │             │               │
     └─────────────┼───────────────┘
                   ▼
          ┌────────────────┐
          │ OTel Collector │
          │  (1 instance)  │
          └────────────────┘
```

<!-- [COPY EDIT] "single Collector instance" — capitalization "Collector" is consistent throughout, good. -->
All services export to a single Collector instance[^2]. This is what we have in `docker-compose.yml`.

**Pros:** Simple to set up and operate. One configuration file. Easy to reason about.

<!-- [COPY EDIT] "shared -- you cannot" → em dash without spaces. -->
<!-- [LINE EDIT] "Configuration is shared -- you cannot apply different sampling rules to different services without adding label-based routing logic." → "Configuration is shared: applying different sampling rules per service requires label-based routing logic." (tighter, active) -->
**Cons:** Single point of failure. If the Collector crashes, all services lose telemetry. The Collector must scale to handle the combined load of all services. Configuration is shared -- you cannot apply different sampling rules to different services without adding label-based routing logic.

### 2. Sidecar Collector (Per-Service)

<!-- [STRUCTURAL] ASCII diagram is informative but right-edge "Collector" labels are truncated ("Col"). Consider adjusting widths or using full names on a second line. -->
```
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│  ┌────────┐         │  │  ┌────────┐         │  │  ┌────────────┐     │
│  │Gateway │─►Collector│  │  │Catalog │─►Collector│  │  │Reservation│─►Col│
│  └────────┘         │  │  └────────┘         │  │  └────────────┘     │
└─────────────────────┘  └─────────────────────┘  └─────────────────────┘
```

<!-- [COPY EDIT] "pod" should be capitalized "Pod" per Kubernetes API conventions when referring to the resource (CMOS deference to project terminology). However, lowercase "pod" is also widely accepted in prose. Be consistent: file uses lowercase "pod" throughout — acceptable, but verify against ch09 sibling files for consistency. -->
Each service gets its own Collector instance, running alongside it. In Docker Compose, this means additional containers. In Kubernetes, the Collector runs as a sidecar container in the same pod as the service.

In Docker Compose, this would look like:

<!-- [COPY EDIT] Please verify: image tag `otel/opentelemetry-collector-contrib:0.149.0` — confirm 0.149.0 exists/is current. As of late 2025, current contrib releases are around 0.110+; 0.149.0 may not exist yet. If using a future-dated example, ensure consistency with other ch09 files. -->
```yaml
# Not implemented -- shown for illustration

catalog:
  image: library/catalog:latest
  environment:
    OTEL_COLLECTOR_ENDPOINT: otel-collector-catalog:4317

otel-collector-catalog:
  image: otel/opentelemetry-collector-contrib:0.149.0
  volumes:
    - ./otel-collector-catalog-config.yaml:/etc/otelcol-contrib/config.yaml
```

Each service's `OTEL_COLLECTOR_ENDPOINT` points to its own Collector, and each Collector can have its own configuration (different sampling rates, different exporters, different processors).

In Kubernetes, the sidecar pattern uses a multi-container pod:

<!-- [COPY EDIT] Please verify: K8s `apiVersion: apps/v1`, `kind: Deployment` snippet — `volumeMounts` references "config" but no `volumes:` entry is shown. For a pedagogical snippet, this omission may confuse readers expecting copy-paste-ready YAML. Consider adding `# volume defined elsewhere` comment or include the volumes block. -->
```yaml
# Kubernetes sidecar example -- not implemented

apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
spec:
  template:
    spec:
      containers:
        - name: catalog
          image: library/catalog:latest
          env:
            - name: OTEL_COLLECTOR_ENDPOINT
              value: "localhost:4317"
        - name: otel-collector
          image: otel/opentelemetry-collector-contrib:0.149.0
          volumeMounts:
            - name: config
              mountPath: /etc/otelcol-contrib/config.yaml
              subPath: config.yaml
```

Because containers in the same pod share a network namespace, the service can reach the Collector at `localhost:4317`. No DNS resolution, no cross-node network hops.

<!-- [COPY EDIT] "isolation -- one Collector crash" → em dash without spaces. -->
**Pros:** Failure isolation -- one Collector crash affects only one service. Per-service configuration. Each Collector handles only its own service's load.

<!-- [COPY EDIT] "templating" — fine. "duplication (or templating to avoid it)" — clear. -->
<!-- [COPY EDIT] "usage -- each Collector" → em dash without spaces. -->
**Cons:** More containers to manage. Configuration duplication (or templating to avoid it). Higher total resource usage -- each Collector consumes CPU and memory even when idle.

### 3. DaemonSet Collector (Per-Node)

```
┌─ Node 1 ──────────────────────────────────┐
│  ┌────────┐  ┌────────┐                   │
│  │Gateway │  │Catalog │  ──► OTel Collector│
│  └────────┘  └────────┘      (1 per node)  │
└────────────────────────────────────────────┘

┌─ Node 2 ──────────────────────────────────┐
│  ┌────────────┐                            │
│  │Reservation │  ──► OTel Collector        │
│  └────────────┘      (1 per node)          │
└────────────────────────────────────────────┘
```

<!-- [COPY EDIT] Please verify: "Services find the Collector using the Kubernetes Downward API" — strictly, Downward API exposes pod/node info as env or files; it does not "find" the Collector but supplies the host IP that the service then dials. Phrasing is acceptable but slightly imprecise. -->
A Kubernetes DaemonSet[^4] runs one Collector per node. All services on that node export to their node-local Collector. Services find the Collector using the Kubernetes Downward API:

<!-- [COPY EDIT] Please verify: `hostPort: 4317` — works but is generally discouraged in production K8s (port conflicts, security). Consider noting this trade-off. -->
```yaml
# Kubernetes DaemonSet example -- not implemented

apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel-collector
spec:
  template:
    spec:
      containers:
        - name: otel-collector
          image: otel/opentelemetry-collector-contrib:0.149.0
          ports:
            - containerPort: 4317
              hostPort: 4317
              protocol: TCP
```

Services use the node's IP (via the Downward API) to reach the Collector:

```yaml
env:
  - name: OTEL_COLLECTOR_ENDPOINT
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
```

<!-- [LINE EDIT] "The service exports to `$(status.hostIP):4317`, which reaches the DaemonSet Collector on the same node." — fine, but `$(status.hostIP)` mixes shell-style syntax with the actual K8s env-var value. Real services would dial the env var directly. Worth a brief clarifying note. -->
The service exports to `$(status.hostIP):4317`, which reaches the DaemonSet Collector on the same node.

**Pros:** Fewer Collector instances than sidecar (one per node, not one per pod). Network traffic stays node-local. Shared configuration across all services on the node.

**Cons:** A Collector crash affects all services on that node. The Collector must handle the combined load of all pods on the node. Shared configuration means less per-service flexibility than the sidecar model.

---

## Trade-offs at a Glance

<!-- [STRUCTURAL] Excellent comparison table — concise and complete. Strong pedagogical anchor. -->
| Aspect | Shared | Sidecar | DaemonSet |
|--------|--------|---------|-----------|
| **Instances** | 1 | 1 per service | 1 per node |
| **Failure blast radius** | All services | 1 service | All services on 1 node |
| **Config flexibility** | Shared | Per-service | Per-node (usually shared) |
| **Resource overhead** | Low | High | Medium |
| **Operational complexity** | Low | High | Medium |
| **Network hops** | Cross-container | localhost | Same node |
| **Best for** | Dev, small teams | Multi-team, strict isolation | Production Kubernetes |

<!-- [LINE EDIT] "The DaemonSet model is the most common in production Kubernetes deployments." → "The DaemonSet model dominates production Kubernetes deployments." (more direct) -->
<!-- [LINE EDIT] "The shared model is fine for development and small production systems." → "The shared model suits development and small production systems." -->
The DaemonSet model is the most common in production Kubernetes deployments. It balances resource efficiency with locality. The sidecar model is used when teams need complete independence (different sampling, different backends). The shared model is fine for development and small production systems.

---

## Production Considerations

### Sampling Strategies

<!-- [STRUCTURAL] Sampling subsection partially overlaps with the "Sampling" bullet earlier in the chapter. Consider either trimming the earlier mention or cross-referencing here ("see §X.Y for the head/tail-based distinction"). Some redundancy is acceptable for a reference section, but flag it. -->
In development, we keep 100% of traces. In production, this is expensive. Common strategies:

<!-- [COPY EDIT] "ParentBasedSampler" and "TraceIDRatioBased(0.1)" — verify exact OTel Go SDK identifiers. Common forms: `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))`. The cased identifiers as written may not match exact API names. -->
<!-- [COPY EDIT] "Head-based" / "Tail-based" — hyphenated compound modifiers, correct (CMOS 7.81). -->
**Head-based sampling** -- Decide at trace creation time whether to sample. The SDK's `ParentBasedSampler` with `TraceIDRatioBased(0.1)` keeps 10% of traces. Simple and efficient, but you might miss the one trace that matters.

<!-- [COPY EDIT] "tail_sampling" processor — verify exact processor name; in collector-contrib it is `tail_sampling`. Correct. -->
**Tail-based sampling** -- Decide after the trace completes. The Collector's `tail_sampling` processor can keep 100% of error traces and 1% of successful traces. This requires the Collector to buffer complete traces before deciding, which increases memory usage.

**Rate-based sampling** -- Keep a fixed number of traces per second. Useful for high-traffic services where even 1% sampling produces too much data.

<!-- [LINE EDIT] "In practice, you combine these: head-based sampling in the SDK (to reduce export volume) plus tail-based sampling in the Collector (to keep interesting traces)." → fine. -->
<!-- [LINE EDIT] "The sidecar and DaemonSet patterns make tail-based sampling more practical because the Collector is closer to the service and sees fewer total traces." (28 words) — acceptable. -->
In practice, you combine these: head-based sampling in the SDK (to reduce export volume) plus tail-based sampling in the Collector (to keep interesting traces). The sidecar and DaemonSet patterns make tail-based sampling more practical because the Collector is closer to the service and sees fewer total traces.

### Resource Limits

<!-- [COPY EDIT] "memory_limiter" — correct processor name; lowercase as in OTel config. -->
The Collector consumes memory proportional to the telemetry volume. Set `memory_limiter` processor to cap memory usage:

<!-- [COPY EDIT] Please verify: `memory_limiter` processor accepts `check_interval`, `limit_mib`, `spike_limit_mib`. Correct as of recent OTel collector versions. -->
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
```

<!-- [COPY EDIT] "intentional -- it is better to lose telemetry than to crash the Collector (and lose all telemetry)." → em dash without spaces. -->
<!-- [LINE EDIT] "If the limit is hit, the Collector drops data." → "If the Collector hits the limit, it drops data." (active voice) -->
If the limit is hit, the Collector drops data. This is intentional -- it is better to lose telemetry than to crash the Collector (and lose all telemetry).

### High Availability

<!-- [LINE EDIT] "For the shared model, run multiple Collector instances behind a load balancer." → fine. -->
<!-- [LINE EDIT] Long parenthetical "(if a node dies, its Collector dies with it, and pods are rescheduled to other nodes with their own Collectors)" — 22 words inside parens; consider splitting into a separate sentence: "If a node dies, its Collector dies with it; Kubernetes reschedules pods to other nodes with their own Collectors." -->
For the shared model, run multiple Collector instances behind a load balancer. For the DaemonSet model, Kubernetes handles availability (if a node dies, its Collector dies with it, and pods are rescheduled to other nodes with their own Collectors).

---

## When to Switch Models

<!-- [STRUCTURAL] Decision-rule list is well-targeted. Keep. -->
Start with the shared Collector (what we have). Switch when:

<!-- [COPY EDIT] "PII" — initialism, OK. Consider expanding on first mention: "personally identifiable information (PII)". Verify whether PII is introduced earlier in ch09. -->
- **Team boundaries emerge.** If the catalog team wants different sampling than the reservation team, the sidecar model gives them independence.
- **Scale demands it.** If a single Collector cannot handle the load, either scale it horizontally (with a load balancer) or move to DaemonSet/sidecar.
- **Compliance requires isolation.** If one service handles PII and its telemetry must be processed separately (different exporters, different retention), a sidecar provides the necessary isolation.
- **You move to Kubernetes.** The DaemonSet model is natural in Kubernetes and is well-supported by the OpenTelemetry Operator[^1], which can automatically inject sidecar Collectors into pods.

---

## Exercises

<!-- [STRUCTURAL] Exercises mix research, hands-on config, and conceptual calculation — well-balanced. -->
1. **Draw the sidecar architecture.** Sketch the Docker Compose YAML for a sidecar deployment where each of the three services has its own Collector. How many containers does this add? What changes in each service's environment variables?

<!-- [COPY EDIT] "tail_sampling" — backticks would be helpful for code-named processor: `tail_sampling`. -->
2. **Configure tail-based sampling.** Add a `tail_sampling` processor to the Collector config that keeps 100% of traces with any error span and 10% of all other traces. Test it by making both successful and failing requests.

<!-- [COPY EDIT] "256MB" → per CMOS 9.16/SI: prefer "256 MB" with a thin or regular space. -->
<!-- [LINE EDIT] "If a single Collector uses 256MB of RAM, how much total RAM do you need for: (a) shared model with 10 services, (b) sidecar model with 10 services, (c) DaemonSet model with 10 services across 3 nodes? Which model is most efficient?" — long but list-style; acceptable. -->
3. **Calculate resource overhead.** If a single Collector uses 256MB of RAM, how much total RAM do you need for: (a) shared model with 10 services, (b) sidecar model with 10 services, (c) DaemonSet model with 10 services across 3 nodes? Which model is most efficient?

<!-- [COPY EDIT] "Custom Resource Definitions" — capitalization correct for K8s acronym. -->
4. **Research the OpenTelemetry Operator.** Read the [OpenTelemetry Operator documentation](https://opentelemetry.io/docs/kubernetes/operator/). How does it automate sidecar injection? What CRDs (Custom Resource Definitions) does it introduce?

<!-- [LINE EDIT] "If you have used Datadog's Java agent, compare its architecture to the OTel Collector." → fine. -->
5. **Compare to Java infrastructure.** If you have used Datadog's Java agent, compare its architecture to the OTel Collector. Where does the Datadog agent run (sidecar, DaemonSet, or shared)? How does it differ from the OTel approach?

---

## References

<!-- [COPY EDIT] All footnote URLs are well-formed; verify they remain live. -->
[^1]: [OpenTelemetry Operator for Kubernetes](https://opentelemetry.io/docs/kubernetes/operator/) -- Automatic sidecar injection and Collector management via Kubernetes CRDs.
[^2]: [OTel Collector Deployment Patterns](https://opentelemetry.io/docs/collector/deployment/) -- Official documentation on no-collector, agent, and gateway deployment modes.
[^3]: [Tail-Based Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor) -- Configuration and policy options for tail-based sampling in the Collector.
[^4]: [Kubernetes DaemonSet Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) -- How DaemonSets ensure one pod per node.

<!-- [FINAL] Doubled words check: none found. -->
<!-- [FINAL] Cross-reference check: footnotes [^1]–[^4] all defined and used. -->
<!-- [FINAL] Throughout the file, the en-dash-style separator " -- " is used in body text and in footnote definitions. Per CMOS 6.85, prefer the em dash "—" without surrounding spaces, or rendered as "--" only in code blocks. The Markdown source does not render "--" as an em dash; consider a sweeping replace in revision. -->
<!-- [FINAL] "OTel Collector" vs "Collector" — both used; consistent: "OTel Collector" on first reference, "Collector" thereafter. Good. -->
