# 8.5 The Sidecar Collector Pattern

Our system uses a single shared OTel Collector for all services. This is the simplest possible deployment. But as your system grows -- more services, more teams, different sampling needs -- you will encounter the sidecar and DaemonSet patterns. This section explains what they are, when to use them, and how they map to our architecture.

---

## Why Not Export Directly to Backends?

A natural question: why not skip the Collector and have each service export traces directly to Tempo and metrics directly to Prometheus?

You can. The OTel SDK supports direct export to many backends. But the Collector provides several benefits that matter in production:

**Buffering and retry.** If Tempo is temporarily unavailable, the Collector buffers spans in memory and retries. Without a Collector, the SDK's internal buffer fills up and drops spans silently. The Collector has a larger, configurable buffer and more sophisticated retry logic.

**Protocol translation.** Your services speak OTLP. Your backends might speak something else (Jaeger's Thrift protocol, Zipkin's JSON format, Prometheus's scrape protocol). The Collector handles the translation. When you switch backends, you change the Collector config -- not your application code.

**Sampling.** In production, you may not want to keep every trace. The Collector can apply probabilistic sampling (keep 10% of traces), tail-based sampling[^3] (keep 100% of slow or error traces, 1% of everything else), or rate-based sampling. These decisions require seeing the full trace, which the Collector can do after receiving all spans. The SDK can only make sampling decisions at span creation time, before the outcome is known.

**Credential management.** If your backend requires API keys or TLS certificates, the Collector holds them. Your services do not need access to backend credentials -- they just export to the Collector's local gRPC endpoint.

**Enrichment and filtering.** The Collector can add attributes (e.g., deployment environment, cluster name), remove sensitive attributes, or drop entire spans that match a filter. This is a central policy enforcement point.

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

All services export to a single Collector instance[^2]. This is what we have in `docker-compose.yml`.

**Pros:** Simple to set up and operate. One configuration file. Easy to reason about.

**Cons:** Single point of failure. If the Collector crashes, all services lose telemetry. The Collector must scale to handle the combined load of all services. Configuration is shared -- you cannot apply different sampling rules to different services without adding label-based routing logic.

### 2. Sidecar Collector (Per-Service)

```
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│  ┌────────┐         │  │  ┌────────┐         │  │  ┌────────────┐     │
│  │Gateway │─►Collector│  │  │Catalog │─►Collector│  │  │Reservation│─►Col│
│  └────────┘         │  │  └────────┘         │  │  └────────────┘     │
└─────────────────────┘  └─────────────────────┘  └─────────────────────┘
```

Each service gets its own Collector instance, running alongside it. In Docker Compose, this means additional containers. In Kubernetes, the Collector runs as a sidecar container in the same pod as the service.

In Docker Compose, this would look like:

```yaml
# Not implemented -- shown for illustration

catalog:
  image: library/catalog:latest
  environment:
    OTEL_COLLECTOR_ENDPOINT: otel-collector-catalog:4317

otel-collector-catalog:
  image: otel/opentelemetry-collector-contrib:0.115.0
  volumes:
    - ./otel-collector-catalog-config.yaml:/etc/otelcol-contrib/config.yaml
```

Each service's `OTEL_COLLECTOR_ENDPOINT` points to its own Collector, and each Collector can have its own configuration (different sampling rates, different exporters, different processors).

In Kubernetes, the sidecar pattern uses a multi-container pod:

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
          image: otel/opentelemetry-collector-contrib:0.115.0
          volumeMounts:
            - name: config
              mountPath: /etc/otelcol-contrib/config.yaml
              subPath: config.yaml
```

Because containers in the same pod share a network namespace, the service can reach the Collector at `localhost:4317`. No DNS resolution, no cross-node network hops.

**Pros:** Failure isolation -- one Collector crash affects only one service. Per-service configuration. Each Collector handles only its own service's load.

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

A Kubernetes DaemonSet[^4] runs one Collector per node. All services on that node export to their node-local Collector. Services find the Collector using the Kubernetes Downward API:

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
          image: otel/opentelemetry-collector-contrib:0.115.0
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

The service exports to `$(status.hostIP):4317`, which reaches the DaemonSet Collector on the same node.

**Pros:** Fewer Collector instances than sidecar (one per node, not one per pod). Network traffic stays node-local. Shared configuration across all services on the node.

**Cons:** A Collector crash affects all services on that node. The Collector must handle the combined load of all pods on the node. Shared configuration means less per-service flexibility than the sidecar model.

---

## Trade-offs at a Glance

| Aspect | Shared | Sidecar | DaemonSet |
|--------|--------|---------|-----------|
| **Instances** | 1 | 1 per service | 1 per node |
| **Failure blast radius** | All services | 1 service | All services on 1 node |
| **Config flexibility** | Shared | Per-service | Per-node (usually shared) |
| **Resource overhead** | Low | High | Medium |
| **Operational complexity** | Low | High | Medium |
| **Network hops** | Cross-container | localhost | Same node |
| **Best for** | Dev, small teams | Multi-team, strict isolation | Production Kubernetes |

The DaemonSet model is the most common in production Kubernetes deployments. It balances resource efficiency with locality. The sidecar model is used when teams need complete independence (different sampling, different backends). The shared model is fine for development and small production systems.

---

## Production Considerations

### Sampling Strategies

In development, we keep 100% of traces. In production, this is expensive. Common strategies:

**Head-based sampling** -- Decide at trace creation time whether to sample. The SDK's `ParentBasedSampler` with `TraceIDRatioBased(0.1)` keeps 10% of traces. Simple and efficient, but you might miss the one trace that matters.

**Tail-based sampling** -- Decide after the trace completes. The Collector's `tail_sampling` processor can keep 100% of error traces and 1% of successful traces. This requires the Collector to buffer complete traces before deciding, which increases memory usage.

**Rate-based sampling** -- Keep a fixed number of traces per second. Useful for high-traffic services where even 1% sampling produces too much data.

In practice, you combine these: head-based sampling in the SDK (to reduce export volume) plus tail-based sampling in the Collector (to keep interesting traces). The sidecar and DaemonSet patterns make tail-based sampling more practical because the Collector is closer to the service and sees fewer total traces.

### Resource Limits

The Collector consumes memory proportional to the telemetry volume. Set `memory_limiter` processor to cap memory usage:

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
```

If the limit is hit, the Collector drops data. This is intentional -- it is better to lose telemetry than to crash the Collector (and lose all telemetry).

### High Availability

For the shared model, run multiple Collector instances behind a load balancer. For the DaemonSet model, Kubernetes handles availability (if a node dies, its Collector dies with it, and pods are rescheduled to other nodes with their own Collectors).

---

## When to Switch Models

Start with the shared Collector (what we have). Switch when:

- **Team boundaries emerge.** If the catalog team wants different sampling than the reservation team, the sidecar model gives them independence.
- **Scale demands it.** If a single Collector cannot handle the load, either scale it horizontally (with a load balancer) or move to DaemonSet/sidecar.
- **Compliance requires isolation.** If one service handles PII and its telemetry must be processed separately (different exporters, different retention), a sidecar provides the necessary isolation.
- **You move to Kubernetes.** The DaemonSet model is natural in Kubernetes and is well-supported by the OpenTelemetry Operator[^1], which can automatically inject sidecar Collectors into pods.

---

## Exercises

1. **Draw the sidecar architecture.** Sketch the Docker Compose YAML for a sidecar deployment where each of the three services has its own Collector. How many containers does this add? What changes in each service's environment variables?

2. **Configure tail-based sampling.** Add a `tail_sampling` processor to the Collector config that keeps 100% of traces with any error span and 10% of all other traces. Test it by making both successful and failing requests.

3. **Calculate resource overhead.** If a single Collector uses 256MB of RAM, how much total RAM do you need for: (a) shared model with 10 services, (b) sidecar model with 10 services, (c) DaemonSet model with 10 services across 3 nodes? Which model is most efficient?

4. **Research the OpenTelemetry Operator.** Read the [OpenTelemetry Operator documentation](https://opentelemetry.io/docs/kubernetes/operator/). How does it automate sidecar injection? What CRDs (Custom Resource Definitions) does it introduce?

5. **Compare to Java infrastructure.** If you have used Datadog's Java agent, compare its architecture to the OTel Collector. Where does the Datadog agent run (sidecar, DaemonSet, or shared)? How does it differ from the OTel approach?

---

## References

[^1]: [OpenTelemetry Operator for Kubernetes](https://opentelemetry.io/docs/kubernetes/operator/) -- Automatic sidecar injection and Collector management via Kubernetes CRDs.
[^2]: [OTel Collector Deployment Patterns](https://opentelemetry.io/docs/collector/deployment/) -- Official documentation on no-collector, agent, and gateway deployment modes.
[^3]: [Tail-Based Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor) -- Configuration and policy options for tail-based sampling in the Collector.
[^4]: [Kubernetes DaemonSet Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) -- How DaemonSets ensure one pod per node.
