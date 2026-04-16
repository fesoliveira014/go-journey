# 12.3 Application Manifests

<!-- [STRUCTURAL] Opening bridges from 12.2 (which focused on service code) to manifest work. Good transition. -->
<!-- [LINE EDIT] "With all five services containerized and the infrastructure layer (PostgreSQL, Kafka, Meilisearch) declared in the previous section" — but section 12.2 is "Preparing Services for Kubernetes" (code-level), not infrastructure manifests. The infrastructure manifests are covered in 12.4 (infra-manifests.md), which comes AFTER this section. This cross-reference is wrong. -->
<!-- [STRUCTURAL] Major issue: this section (12.3) references "the previous section" as having declared infrastructure layer, but 12.2 is about preparing services. 12.4 is the infra-manifests section. Either (a) reorder so 12.4 comes first, OR (b) rewrite this opening to not presume infrastructure has already been declared. The section numbering and ordering must be reconciled. -->
With all five services containerized and the infrastructure layer (PostgreSQL, Kafka, Meilisearch) declared in the previous section, we are ready to write the manifests for the application services themselves. Every service needs three resources: a Deployment that runs the container, a Service that gives it a stable DNS name inside the cluster, and a ConfigMap that injects non-sensitive configuration. Secrets are declared separately as placeholder objects that a local overlay will fill in with real values.

<!-- [LINE EDIT] "Keeping namespaces separate has two practical benefits: you can delete all application resources with a single `kubectl delete namespace library` during development without touching your databases, and RBAC policies (covered in Chapter 13) can grant service accounts namespace-scoped permissions rather than cluster-wide ones." — 40+ words; consider splitting. -->
All application resources live in the `library` namespace. Infrastructure resources live in `data` and `messaging`. Keeping namespaces separate has two practical benefits: you can delete all application resources with a single `kubectl delete namespace library` during development without touching your databases, and RBAC policies (covered in Chapter 13) can grant service accounts namespace-scoped permissions rather than cluster-wide ones.

---

## Namespaces

<!-- [LINE EDIT] "A Kubernetes namespace is a logical partition of the cluster's resource tree." — clean. -->
<!-- [COPY EDIT] "Objects in different namespaces can have the same name without collision." — good. -->
A Kubernetes namespace is a logical partition of the cluster's resource tree. Objects in different namespaces can have the same name without collision. Cluster-level objects — nodes, persistent volumes, storage classes — are not namespaced.

We use three namespaces:

| Namespace   | Contents                                        |
|-------------|-------------------------------------------------|
| `library`   | All five application services                   |
| `data`      | PostgreSQL StatefulSets and their Services      |
| `messaging` | Kafka StatefulSet and its Service               |

```yaml
# deploy/k8s/base/library/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: library
```

<!-- [STRUCTURAL] This cross-reference is to "section 12.2" but the namespace manifests for data and messaging actually belong to section 12.4 (infra-manifests). Same ordering issue as above. Fix terminology/ordering. -->
<!-- [LINE EDIT] "were declared in section 12.2" — wrong section number given current section ordering. Should be 12.4. -->
The `data` and `messaging` namespace manifests are identical in structure (just with different `name` fields) and were declared in section 12.2.

---

## Catalog Deployment — full walkthrough

<!-- [COPY EDIT] Heading case: "Catalog Deployment — full walkthrough" — mixes title case with sentence case after the em dash. Other H2 headings in this chapter use title case consistently. Normalize to either "Catalog Deployment — Full Walkthrough" (title case) or lowercase all H2s. CMOS 8.159. -->
<!-- [STRUCTURAL] Walking through a single canonical manifest in detail before repeating the pattern for other services is exactly right. This is the spine of the section. -->
The catalog service is a good template for all the others. Its manifest touches every field you need to understand, so we will walk through it in detail.

```yaml
# deploy/k8s/base/library/catalog-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: library
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catalog
  template:
    metadata:
      labels:
        app: catalog
    spec:
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
      containers:
        - name: catalog
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          image: library-system/catalog:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 50052
              name: grpc
          envFrom:
            - configMapRef:
                name: catalog-config
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-catalog-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: "host=postgres-catalog-0.postgres-catalog.data.svc.cluster.local port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=catalog sslmode=disable"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "250m"
              memory: "256Mi"
          livenessProbe:
            grpc:
              port: 50052
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            grpc:
              port: 50052
            initialDelaySeconds: 5
            periodSeconds: 10
```

<!-- [COPY EDIT] DATABASE_URL here is rendered as a single long string. Later in the auth deployment, the same pattern uses `value: >-` (YAML folded block scalar) for multi-line readability. Pick one style and apply consistently. Folded scalar is more readable. -->
<!-- [COPY EDIT] The catalog deployment shown here omits `failureThreshold: 3` on probes, but the later auth/reservation/search/gateway deployments include it. Also, section 12.2 examples showed `failureThreshold: 3`. Normalize. -->

### `apiVersion` and `kind`

<!-- [LINE EDIT] "`apiVersion: apps/v1` means this resource is defined in the `apps` API group, stable version 1." — good. -->
`apiVersion: apps/v1` means this resource is defined in the `apps` API group, stable version 1. The `kind: Deployment` tells the API server which controller should own this object. The Deployment controller is built into `kube-controller-manager` and runs on every standard cluster.

### `metadata`

<!-- [LINE EDIT] "If omitted, objects land in `default`, which is fine for experiments but undesirable in a real project." — good pragmatic note. -->
<!-- [COPY EDIT] "labels on the Deployment itself are for your own organization (filtering with `kubectl get -l app=catalog`). They are distinct from the labels on the Pod template." — but the catalog YAML above does NOT define labels on the Deployment metadata. Other deployments (auth, reservation, etc.) DO have `metadata.labels`. Mismatch: add `labels: { app: catalog }` to catalog's metadata for consistency, or note that labels on Deployment metadata are optional. -->
- `name: catalog` — the name of the Deployment object. This is also what appears in `kubectl get deployments`.
- `namespace: library` — places the object in our application namespace. If omitted, objects land in `default`, which is fine for experiments but undesirable in a real project.
- `labels` — key-value pairs attached to the object. Labels on the Deployment itself are for your own organization (filtering with `kubectl get -l app=catalog`). They are distinct from the labels on the Pod template.

### `spec.replicas`

<!-- [LINE EDIT] "Increasing this to 2+ enables rolling updates and basic availability during node maintenance" — "2+" is unusual for prose. Consider "two or more." CMOS 9.2 spells out small whole numbers in prose. -->
<!-- [COPY EDIT] CMOS 9.2–9.5: zero through one hundred spelled out in general prose; numerals for counts with units. Here "2+" is shorthand; prefer "two or more" in prose. -->
How many Pod copies the controller should maintain. We use 1 during development. Increasing this to 2+ enables rolling updates and basic availability during node maintenance, but requires that services handle multiple concurrent instances correctly — which ours do, since all state lives in PostgreSQL and Kafka.

### `spec.selector.matchLabels`

<!-- [LINE EDIT] "This is the link between the Deployment controller and the Pods it manages." — clear. -->
<!-- [LINE EDIT] "**The selector must match the labels in `spec.template.metadata.labels` exactly.** If they diverge, the controller cannot find its Pods and will continually create new ones." — good emphasis. -->
This is the link between the Deployment controller and the Pods it manages. The controller watches all Pods whose labels match this selector and reconciles toward the desired replica count. **The selector must match the labels in `spec.template.metadata.labels` exactly.** If they diverge, the controller cannot find its Pods and will continually create new ones.

<!-- [COPY EDIT] "Once a Deployment is created, `spec.selector` is immutable. To change it you must delete and recreate the Deployment." — please verify: since Kubernetes 1.8, `spec.selector` is immutable for Deployments. Correct. -->
Once a Deployment is created, `spec.selector` is immutable. To change it you must delete and recreate the Deployment.

### Pod template

<!-- [LINE EDIT] "It is a Pod spec, not a Deployment-specific construct — you could copy it into a standalone `kind: Pod` manifest and it would be valid." — good explanation. -->
Everything under `spec.template` describes the Pod that the Deployment creates. It is a Pod spec, not a Deployment-specific construct — you could copy it into a standalone `kind: Pod` manifest and it would be valid.

#### `image` and `imagePullPolicy`

<!-- [LINE EDIT] "references the image we built and loaded into kind in section 12.1" — good cross-reference. -->
<!-- [COPY EDIT] "The `latest` tag is normally discouraged in production because it makes rollbacks ambiguous, but it is fine for a local development cluster" — CMOS 6.19 serial comma — no list here, OK. -->
`image: library-system/catalog:latest` references the image we built and loaded into kind in section 12.1. The `latest` tag is normally discouraged in production because it makes rollbacks ambiguous, but it is fine for a local development cluster where we control exactly what is in the cache.

<!-- [LINE EDIT] "`imagePullPolicy: IfNotPresent` is critical for kind." — good strong statement. -->
<!-- [LINE EDIT] "the kubelet will still attempt a registry pull, which fails because `library-system/catalog:latest` does not exist in any public registry" — clear. -->
`imagePullPolicy: IfNotPresent` is critical for kind. By default, Kubernetes tries to pull images from a registry. kind loads images directly into its internal containerd cache via `kind load docker-image`. If the pull policy is `Always`, the kubelet will still attempt a registry pull, which fails because `library-system/catalog:latest` does not exist in any public registry. `IfNotPresent` tells the kubelet: if the image is already present in the local cache, use it. This is the correct policy for locally built images in a kind cluster.

#### `ports`

<!-- [STRUCTURAL] This paragraph is a useful gotcha (`containerPort` is documentation, not a binding). Keep. -->
<!-- [LINE EDIT] "`containerPort` is documentation — it does not actually open a port or affect networking." — the word "actually" is filler. Drop: "`containerPort` is documentation; it does not open a port or affect networking." -->
<!-- [COPY EDIT] Be careful with this claim. While `containerPort` does not directly affect pod networking (all ports are reachable), it IS used when Services use port names (`targetPort: grpc` referencing the port name in the containerPort section). So "does not actually open a port or affect networking" is slightly overstated. Consider: "It does not open the port — all container ports are reachable regardless. The declaration exists so that tooling…" -->
`containerPort` is documentation — it does not actually open a port or affect networking. Kubernetes networking makes all container ports reachable regardless of whether they are declared here. The convention exists so that tooling (`kubectl describe`, service mesh proxies, monitoring agents) can discover which ports a container uses.

#### `envFrom` — ConfigMap injection

```yaml
envFrom:
  - configMapRef:
      name: catalog-config
```

`envFrom` injects every key from the named ConfigMap as an environment variable. This is how non-sensitive configuration (ports, broker addresses, service endpoints) reaches the container. We cover the ConfigMap itself later in this section.

#### `env` — individual variables and Secrets

<!-- [COPY EDIT] The snippet below uses `value: >-` (folded scalar) — inconsistent with the full manifest above, which uses single-line `value: "..."`. Pick one and apply throughout. -->
```yaml
env:
  - name: POSTGRES_PASSWORD
    valueFrom:
      secretKeyRef:
        name: postgres-catalog-secret
        key: POSTGRES_PASSWORD
  - name: DATABASE_URL
    value: >-
      host=postgres-catalog-0.postgres-catalog.data.svc.cluster.local
      port=5432
      user=postgres
      password=$(POSTGRES_PASSWORD)
      dbname=catalog
      sslmode=disable
```

<!-- [LINE EDIT] "Kubernetes resolves `$(VAR_NAME)` references in the `env` list at definition time, in the order the variables appear." — clear and correct. -->
<!-- [LINE EDIT] "If you place `DATABASE_URL` before `POSTGRES_PASSWORD`, the substitution produces a literal `$(POSTGRES_PASSWORD)` string and the connection will fail." — good warning; this is a common gotcha. -->
There is an important ordering rule here. Kubernetes resolves `$(VAR_NAME)` references in the `env` list at definition time, in the order the variables appear. `DATABASE_URL` uses `$(POSTGRES_PASSWORD)` in its value, so `POSTGRES_PASSWORD` must be defined first. If you place `DATABASE_URL` before `POSTGRES_PASSWORD`, the substitution produces a literal `$(POSTGRES_PASSWORD)` string and the connection will fail.

<!-- [LINE EDIT] "The value is decoded from base64 and injected as a plain string — the container sees it as a normal environment variable." — good. -->
`secretKeyRef` fetches a single key from a Secret object. The value is decoded from base64 and injected as a plain string — the container sees it as a normal environment variable. We declare the Secret placeholders at the end of this section.

#### Security context

```yaml
securityContext:                      # pod-level
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534
containers:
  - name: catalog
    securityContext:                   # container-level
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

<!-- [LINE EDIT] "The pod-level `securityContext` sets defaults for all containers in the pod." — good. -->
<!-- [COPY EDIT] "`runAsUser: 65534` runs as the `nobody` user — our Go binaries are statically linked and need no special user." — good. Verify: UID 65534 is conventionally `nobody`/`nogroup` on most Linux distributions. Correct. -->
The pod-level `securityContext` sets defaults for all containers in the pod. `runAsNonRoot: true` prevents the container from running as UID 0. `runAsUser: 65534` runs as the `nobody` user — our Go binaries are statically linked and need no special user.

<!-- [LINE EDIT] "The container-level `securityContext` tightens permissions further." — strong topic sentence. -->
<!-- [LINE EDIT] 5-sentence paragraph covers a lot; consider splitting at "This eliminates an entire class of attacks…" — a standalone sentence makes the security benefit more prominent. -->
<!-- [COPY EDIT] "`NET_RAW`, `SYS_ADMIN`, etc." — CMOS 6.43: "etc." preceded by comma. Correct. -->
<!-- [COPY EDIT] "e.g." / "etc." — check overall usage in the chapter for consistent handling. -->
The container-level `securityContext` tightens permissions further. `allowPrivilegeEscalation: false` prevents a process from gaining more privileges than its parent (blocks `setuid` binaries and `ptrace` exploits). `readOnlyRootFilesystem: true` makes the container's root filesystem immutable — any attempt to write to disk fails. This eliminates an entire class of attacks where a compromised process writes a malicious binary and executes it. Our Go services write nothing to disk; all state goes to PostgreSQL. `capabilities: drop: ["ALL"]` removes all Linux capabilities (the fine-grained root powers like `NET_RAW`, `SYS_ADMIN`, etc.). A Go gRPC server needs none of them.

<!-- [LINE EDIT] "These settings implement the principle of least privilege at the container level." — good summary. -->
<!-- [COPY EDIT] "Pod Security Admission controller" — this is the current name (replaced PodSecurityPolicy in 1.25). Please verify: https://kubernetes.io/docs/concepts/security/pod-security-admission/ — correct current terminology. -->
These settings implement the principle of least privilege at the container level. In a production cluster, a Pod Security Admission controller can enforce these as a baseline — but setting them explicitly in each manifest ensures compliance regardless of cluster policy.

#### Resource requests and limits

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "250m"
    memory: "256Mi"
```

<!-- [LINE EDIT] "A node with 200m of unallocated CPU can host a Pod requesting 100m." — good concrete example. -->
<!-- [LINE EDIT] "`limits` are enforced at runtime by the Linux kernel's cgroup subsystem: if the container exceeds its memory limit, the kernel OOM-kills it; if it exceeds its CPU limit, it is throttled." — slightly technical but appropriate for the audience. -->
<!-- [LINE EDIT] "Always set both — a Pod without requests cannot be scheduled intelligently, and a Pod without limits can starve its neighbors." — good imperative. -->
`requests` are what the scheduler uses to decide which node can host the Pod. A node with 200m of unallocated CPU can host a Pod requesting 100m. `limits` are enforced at runtime by the Linux kernel's cgroup subsystem: if the container exceeds its memory limit, the kernel OOM-kills it; if it exceeds its CPU limit, it is throttled. Always set both — a Pod without requests cannot be scheduled intelligently, and a Pod without limits can starve its neighbors.

<!-- [LINE EDIT] "The values here are intentionally small." — good. -->
<!-- [COPY EDIT] "well under 50 MiB" — CMOS 9.3 uses numerals for technical measurements. Good. -->
The values here are intentionally small. The catalog service at rest uses almost no CPU and well under 50 MiB of RAM. Give your local cluster room to run all five services simultaneously.

#### Liveness and readiness probes

<!-- [COPY EDIT] This snippet adds `failureThreshold: 3` but the full manifest above omits it. Sync them. -->
```yaml
livenessProbe:
  grpc:
    port: 50052
  initialDelaySeconds: 10
  periodSeconds: 15
  failureThreshold: 3
readinessProbe:
  grpc:
    port: 50052
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

<!-- [LINE EDIT] "Kubernetes supports `grpc` probes natively as of v1.24." — consistent with section 12.2; good. -->
<!-- [COPY EDIT] "Your gRPC servers must register this handler — if they do not, the probe will fail and the Pod will be restarted or never receive traffic." — good warning. -->
Kubernetes supports `grpc` probes natively as of v1.24. The kubelet sends a gRPC `Check` request to the standard `grpc.health.v1.Health` service. Your gRPC servers must register this handler — if they do not, the probe will fail and the Pod will be restarted or never receive traffic.

The difference between the two probe types:

<!-- [LINE EDIT] This bullet list duplicates content from section 12.2. Consider a brief reference back to 12.2 instead of the full re-explanation. E.g., "Section 12.2 covered the difference between liveness and readiness probes; the same rules apply here." -->
- **Liveness** answers "is this container alive?" If it fails `failureThreshold` times consecutively, the kubelet restarts the container. Use it to catch deadlocks or fatal errors that do not crash the process.
- **Readiness** answers "should this container receive traffic?" If it fails, the Pod is removed from the Service's endpoint list. Use it to delay traffic until the service has finished startup (database migrations, cache warming).

`initialDelaySeconds` gives the container time to start before the first probe fires. Set it to slightly less than your service's typical cold-start time.

#### `terminationGracePeriodSeconds`

<!-- [LINE EDIT] "Our services should catch `SIGTERM`, stop accepting new requests, drain in-flight work, and exit. gRPC's `GracefulStop` handles this correctly." — good cross-reference to 12.2. -->
<!-- [COPY EDIT] "30 seconds is generous — most services drain in under 5 seconds." — CMOS 9.3 numerals for measurements. OK. -->
When a Pod is deleted, Kubernetes sends `SIGTERM` to the main container process and waits up to `terminationGracePeriodSeconds` (30 seconds here) for it to exit cleanly before sending `SIGKILL`. Our services should catch `SIGTERM`, stop accepting new requests, drain in-flight work, and exit. gRPC's `GracefulStop` handles this correctly. 30 seconds is generous — most services drain in under 5 seconds.

---

## Catalog Service

<!-- [COPY EDIT] Heading: "Catalog Service" — a reader may mistake this for "description of the catalog service as software." In context it clearly means "the Service manifest for catalog." Consider "Catalog — Service manifest" or renaming to "Catalog Service manifest" for clarity. -->
```yaml
# deploy/k8s/base/library/catalog-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: catalog
  namespace: library
  labels:
    app: catalog
spec:
  selector:
    app: catalog
  ports:
    - name: grpc
      port: 50052
      targetPort: 50052
      protocol: TCP
```

<!-- [LINE EDIT] "A Service[^2] gives a set of Pods a stable virtual IP address and DNS name." — clear. -->
A Service[^2] gives a set of Pods a stable virtual IP address and DNS name. The `selector` field matches Pods by label — in this case any Pod in the `library` namespace with `app: catalog`. When a Pod is added or removed, the Endpoints controller updates the Service's endpoint list automatically.

<!-- [LINE EDIT] "Within the same namespace, `catalog` alone resolves correctly." — good. -->
The DNS name for this Service is `catalog.library.svc.cluster.local`. Within the same namespace, `catalog` alone resolves correctly. From other namespaces you need the full name: `catalog.library.svc.cluster.local`.

<!-- [LINE EDIT] "That is exactly what we want" — fine; minor filler but tutor-appropriate. -->
The default Service type is `ClusterIP`, which means the virtual IP is only reachable from inside the cluster. That is exactly what we want — gRPC services should not be directly exposed externally. Only the gateway Service is accessible from outside, via the Ingress.

<!-- [LINE EDIT] "`port` is what clients connect to. `targetPort` is the port on the container. They are equal here but do not have to be — you could expose port 80 on the Service and forward to port 50052 on the container." — good practical example. -->
`port` is what clients connect to. `targetPort` is the port on the container. They are equal here but do not have to be — you could expose port 80 on the Service and forward to port 50052 on the container.

---

## Catalog ConfigMap

```yaml
# deploy/k8s/base/library/catalog-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalog-config
  namespace: library
data:
  GRPC_PORT: "50052"
  KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"
  OTEL_COLLECTOR_ENDPOINT: ""
```

<!-- [LINE EDIT] "All values must be strings — note the quotes around `"50052"`." — good. -->
<!-- [COPY EDIT] "loaded via `envFrom.configMapRef`" — backtick formatting is correct. -->
ConfigMaps[^4] store non-sensitive key-value data. All values must be strings — note the quotes around `"50052"`. The keys map directly to environment variable names when loaded via `envFrom.configMapRef`.

<!-- [LINE EDIT] "A regular Service DNS name (`kafka.messaging.svc.cluster.local`) would also work but would route through the cluster's load balancer rather than directly to the pod — for Kafka, connecting directly to broker pods by their stable identity is the standard approach." — long but the explanation is worth it; the reader benefits from knowing the alternative. -->
<!-- [COPY EDIT] "the cluster's load balancer" — technically, ClusterIP Services use kube-proxy with iptables/IPVS rules rather than a load balancer. Minor imprecision. Consider "would route through kube-proxy load balancing" or "would be load-balanced across broker pods." -->
`KAFKA_BROKERS` uses the cross-namespace DNS name for the Kafka StatefulSet Pod. StatefulSet Pods get stable DNS names in the form `<pod-name>.<service-name>.<namespace>.svc.cluster.local`. The `kafka-0` pod is in the `messaging` namespace, so its address is `kafka-0.kafka.messaging.svc.cluster.local:9092`. A regular Service DNS name (`kafka.messaging.svc.cluster.local`) would also work but would route through the cluster's load balancer rather than directly to the pod — for Kafka, connecting directly to broker pods by their stable identity is the standard approach.

<!-- [LINE EDIT] "The Docker Compose stack (Chapter 9) includes a full collector; the kind cluster omits it for simplicity." — the second half of the previous sentence already said this. Slight redundancy with the new independent clause. Consider keeping only one version. -->
<!-- [COPY EDIT] "is part of the observability stack in Docker Compose" — "observability stack" is informal terminology; since Chapter 9 is referenced, it's understood. OK. -->
`OTEL_COLLECTOR_ENDPOINT` is empty. The OTel collector is not deployed in the kind cluster (it is part of the observability stack in Docker Compose). Leaving this empty causes the services to skip exporting traces. The Docker Compose stack (Chapter 9) includes a full collector; the kind cluster omits it for simplicity.

---

## Remaining services

<!-- [STRUCTURAL] Good signposting — the author is explicit about minimal commentary on the remaining services, which is appropriate since the template was fully explained. -->
The auth, reservation, search, and gateway services follow the same three-resource pattern. Commentary is minimal — refer back to the catalog walkthrough for field explanations.

### Auth service

<!-- [LINE EDIT] "Auth runs on port 50051, uses its own PostgreSQL instance, handles JWT and OAuth2 configuration, and does not connect to Kafka." — clean one-sentence summary. -->
Auth runs on port 50051, uses its own PostgreSQL instance, handles JWT and OAuth2 configuration, and does not connect to Kafka.

```yaml
# deploy/k8s/base/library/auth-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth
  namespace: library
  labels:
    app: auth
spec:
  replicas: 1
  selector:
    matchLabels:
      app: auth
  template:
    metadata:
      labels:
        app: auth
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: auth
          image: library-system/auth:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: grpc
              containerPort: 50051
              protocol: TCP
          envFrom:
            - configMapRef:
                name: auth-config
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-auth-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: >-
                host=postgres-auth-0.postgres-auth.data.svc.cluster.local
                port=5432
                user=postgres
                password=$(POSTGRES_PASSWORD)
                dbname=auth
                sslmode=disable
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "128Mi"
          livenessProbe:
            grpc:
              port: 50051
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            grpc:
              port: 50051
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
```

<!-- [COPY EDIT] Auth deployment lacks the securityContext blocks (pod-level and container-level) that were emphasised as important for catalog. If the pattern is meant to apply to all services, include it here. If omitted for brevity, add a comment in the YAML or a prose note: "auth, reservation, search, gateway follow the same securityContext settings shown above; omitted here for brevity." -->

```yaml
# deploy/k8s/base/library/auth-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: auth
  namespace: library
  labels:
    app: auth
spec:
  selector:
    app: auth
  ports:
    - name: grpc
      port: 50051
      targetPort: 50051
      protocol: TCP
```

```yaml
# deploy/k8s/base/library/auth-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: auth-config
  namespace: library
data:
  GRPC_PORT: "50051"
  JWT_EXPIRY: "24h"
  GOOGLE_CLIENT_ID: ""
  GOOGLE_REDIRECT_URL: ""
```

<!-- [LINE EDIT] "`GOOGLE_CLIENT_ID` and `GOOGLE_REDIRECT_URL` are non-sensitive configuration — the client ID is public, and the redirect URL is a route, not a credential." — good justification. -->
<!-- [STRUCTURAL] The auth deployment above does NOT include a `GOOGLE_CLIENT_SECRET` secretKeyRef in its env block. The prose here claims "The auth Deployment references it via `secretKeyRef` with `optional: true`" — but the manifest does not show this. Either (a) add it to the YAML, or (b) drop the claim from prose. This is a factual discrepancy between code and narrative. -->
<!-- [LINE EDIT] "For production, the External Secrets Operator injects the real value from AWS Secrets Manager." — forward-references Chapter 14. Good. -->
`GOOGLE_CLIENT_ID` and `GOOGLE_REDIRECT_URL` are non-sensitive configuration — the client ID is public, and the redirect URL is a route, not a credential. Both are left empty in the base ConfigMap. `GOOGLE_CLIENT_SECRET`, on the other hand, is a credential and belongs in a Secret object. The auth Deployment references it via `secretKeyRef` with `optional: true`, so the pod starts normally when the Secret does not exist (as in the local overlay where OAuth2 is not used). For production, the External Secrets Operator injects the real value from AWS Secrets Manager.

### Reservation service

<!-- [LINE EDIT] "Reservation uses port 50053, connects to Kafka, and calls the catalog service over gRPC to validate book availability before creating a reservation." — clear summary. -->
Reservation uses port 50053, connects to Kafka, and calls the catalog service over gRPC to validate book availability before creating a reservation.

```yaml
# deploy/k8s/base/library/reservation-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reservation
  namespace: library
  labels:
    app: reservation
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reservation
  template:
    metadata:
      labels:
        app: reservation
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: reservation
          image: library-system/reservation:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: grpc
              containerPort: 50053
              protocol: TCP
          envFrom:
            - configMapRef:
                name: reservation-config
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-reservation-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: >-
                host=postgres-reservation-0.postgres-reservation.data.svc.cluster.local
                port=5432
                user=postgres
                password=$(POSTGRES_PASSWORD)
                dbname=reservation
                sslmode=disable
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "128Mi"
          livenessProbe:
            grpc:
              port: 50053
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            grpc:
              port: 50053
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
```

```yaml
# deploy/k8s/base/library/reservation-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: reservation
  namespace: library
  labels:
    app: reservation
spec:
  selector:
    app: reservation
  ports:
    - name: grpc
      port: 50053
      targetPort: 50053
      protocol: TCP
```

```yaml
# deploy/k8s/base/library/reservation-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: reservation-config
  namespace: library
data:
  GRPC_PORT: "50053"
  KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"
  CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"
  MAX_ACTIVE_RESERVATIONS: "5"
  OTEL_COLLECTOR_ENDPOINT: ""
```

<!-- [LINE EDIT] "`CATALOG_GRPC_ADDR` uses the Service DNS name (`catalog.library.svc.cluster.local`) rather than a StatefulSet pod name. Application Services (as opposed to StatefulSets) are load-balanced by default, so using the Service name is correct." — good clarification. -->
<!-- [COPY EDIT] "Application Services (as opposed to StatefulSets)" — mixes two distinct concepts. A Service is a network object; a StatefulSet is a workload controller. The intended contrast is between Services backed by a Deployment (load-balanced) and headless Services backed by a StatefulSet (per-pod DNS). Reword: "The catalog Service is a regular ClusterIP Service backed by a Deployment — load-balanced by default — so using the Service name is correct. StatefulSet pods use headless Services and are addressed by pod name instead (as with Kafka above)." -->
`CATALOG_GRPC_ADDR` uses the Service DNS name (`catalog.library.svc.cluster.local`) rather than a StatefulSet pod name. Application Services (as opposed to StatefulSets) are load-balanced by default, so using the Service name is correct.

### Search service

<!-- [LINE EDIT] "It has no database of its own — Meilisearch is its persistence layer." — good. -->
Search uses port 50054, connects to Kafka and Meilisearch, and calls catalog over gRPC for initial index population. It has no database of its own — Meilisearch is its persistence layer.

```yaml
# deploy/k8s/base/library/search-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: search
  namespace: library
  labels:
    app: search
spec:
  replicas: 1
  selector:
    matchLabels:
      app: search
  template:
    metadata:
      labels:
        app: search
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: search
          image: library-system/search:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: grpc
              containerPort: 50054
              protocol: TCP
          envFrom:
            - configMapRef:
                name: search-config
          env:
            - name: MEILI_MASTER_KEY
              valueFrom:
                secretKeyRef:
                  name: meilisearch-secret
                  key: MEILI_MASTER_KEY
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "128Mi"
          livenessProbe:
            grpc:
              port: 50054
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            grpc:
              port: 50054
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
```

```yaml
# deploy/k8s/base/library/search-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: search
  namespace: library
  labels:
    app: search
spec:
  selector:
    app: search
  ports:
    - name: grpc
      port: 50054
      targetPort: 50054
      protocol: TCP
```

```yaml
# deploy/k8s/base/library/search-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: search-config
  namespace: library
data:
  GRPC_PORT: "50054"
  MEILI_URL: "http://meilisearch.data.svc.cluster.local:7700"
  CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"
  KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"
  OTEL_COLLECTOR_ENDPOINT: ""
```

### Gateway service

<!-- [LINE EDIT] "Its liveness probe uses an HTTP GET on `/healthz` rather than a gRPC check." — clear. -->
Gateway is the only HTTP service. Its liveness probe uses an HTTP GET on `/healthz` rather than a gRPC check. It holds `*_GRPC_ADDR` references for all four backend services, plus `JWT_SECRET` for validating tokens on incoming requests.

```yaml
# deploy/k8s/base/library/gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: library
  labels:
    app: gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: gateway
          image: library-system/gateway:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          envFrom:
            - configMapRef:
                name: gateway-config
          env:
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "128Mi"
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
```

```yaml
# deploy/k8s/base/library/gateway-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: library
  labels:
    app: gateway
spec:
  selector:
    app: gateway
  ports:
    - name: http
      port: 8080
      targetPort: 8080
      protocol: TCP
```

```yaml
# deploy/k8s/base/library/gateway-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
  namespace: library
data:
  PORT: "8080"
  AUTH_GRPC_ADDR: "auth.library.svc.cluster.local:50051"
  CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"
  RESERVATION_GRPC_ADDR: "reservation.library.svc.cluster.local:50053"
  SEARCH_GRPC_ADDR: "search.library.svc.cluster.local:50054"
  OTEL_COLLECTOR_ENDPOINT: ""
```

---

## Secrets

<!-- [LINE EDIT] "Notice that the base directory does **not** include a `secrets.yaml` file. This is intentional — secret values should never exist in the base, even as empty placeholders." — strong and correct. -->
<!-- [STRUCTURAL] Good dedicated subsection on secrets. Keeps security concerns explicit. -->
The Deployment manifests reference several Secrets by name (`jwt-secret`, `postgres-catalog-secret`, `postgres-auth-secret`, `postgres-reservation-secret`, `meilisearch-secret`). Notice that the base directory does **not** include a `secrets.yaml` file. This is intentional — secret values should never exist in the base, even as empty placeholders. Instead, each overlay is responsible for creating the Secrets that its environment needs.

<!-- [LINE EDIT] "The local overlay (section 12.5) uses Kustomize's `secretGenerator` to create all required Secrets with development values." — good forward-reference. -->
<!-- [COPY EDIT] "Chapter 14" — forward-reference; confirm this is chapter 14 in the outline. -->
The local overlay (section 12.5) uses Kustomize's `secretGenerator` to create all required Secrets with development values. The production overlay uses the External Secrets Operator to sync values from AWS Secrets Manager (Chapter 14). This pattern keeps credentials out of version control entirely.

<!-- [LINE EDIT] "The key names populated by the overlay (`JWT_SECRET`, `POSTGRES_PASSWORD`, `MEILI_MASTER_KEY`) match the `secretKeyRef.key` values in the Deployment manifests exactly." — clear. -->
The key names populated by the overlay (`JWT_SECRET`, `POSTGRES_PASSWORD`, `MEILI_MASTER_KEY`) match the `secretKeyRef.key` values in the Deployment manifests exactly. The OAuth2 client secret (`GOOGLE_CLIENT_SECRET`) is referenced by the auth Deployment with `optional: true`, so it only needs to be provided in environments where Google OAuth2 is configured.

<!-- [STRUCTURAL] "**Base64 is not encryption.**" paragraph — great. Bold opener drives the point home. -->
<!-- [LINE EDIT] "Secrets are only marginally better than ConfigMaps at rest (in etcd) unless you enable etcd encryption, and they are only as secure as your RBAC policy." — good summary. -->
<!-- [COPY EDIT] "The canonical solution for production is an external secret store (HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager) synced to Kubernetes Secrets by an operator." — serial comma; good. -->
**Base64 is not encryption.** Anyone with read access to a Secret object — via `kubectl get secret jwt-secret -o yaml` — can decode the value with `base64 -d`. Secrets are only marginally better than ConfigMaps at rest (in etcd) unless you enable etcd encryption, and they are only as secure as your RBAC policy. The canonical solution for production is an external secret store (HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager) synced to Kubernetes Secrets by an operator. We cover this in Chapter 14. For the local cluster, the local overlay's `secretGenerator` provides concrete values without putting them in version control.

---

## Ingress

<!-- [LINE EDIT] "It requires an Ingress controller to be running — we installed NGINX Ingress as part of the kind cluster setup in section 12.1." — good. -->
<!-- [COPY EDIT] "Ingress controller" vs earlier "Ingress Controller" in kind-setup.md. Normalize across chapter. Suggest lowercase "controller" (kubernetes.io docs style). -->
An Ingress[^3] exposes HTTP and HTTPS routes from outside the cluster to Services inside it. It requires an Ingress controller to be running — we installed NGINX Ingress as part of the kind cluster setup in section 12.1.

```yaml
# deploy/k8s/base/library/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: library-ingress
  namespace: library
spec:
  ingressClassName: nginx
  rules:
    - host: library.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: gateway
                port:
                  number: 8080
```

<!-- [COPY EDIT] Please verify: `apiVersion: networking.k8s.io/v1` — stable since Kubernetes 1.19. Correct. -->
<!-- [LINE EDIT] "Without this field, if multiple Ingress controllers are installed, the behavior is undefined." — "undefined" is slightly strong; "ambiguous" or "nondeterministic" is more accurate. Or: "the behavior depends on which controller claims the Ingress first." -->
`ingressClassName: nginx` selects the NGINX controller installed in the cluster. Without this field, if multiple Ingress controllers are installed, the behavior is undefined.

<!-- [LINE EDIT] "The gateway handles all routing internally — it owns the URL tree and proxies to the appropriate gRPC backend." — clear. -->
All HTTP traffic to `library.local` routes to the gateway Service on port 8080. The gateway handles all routing internally — it owns the URL tree and proxies to the appropriate gRPC backend.

<!-- [LINE EDIT] "To reach `library.local` from your development machine, add the following line to `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows):" — good cross-platform instruction. -->
To reach `library.local` from your development machine, add the following line to `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows):

```
127.0.0.1 library.local
```

<!-- [LINE EDIT] "With the hosts entry in place, `http://library.local` resolves to `127.0.0.1:80`, which kind forwards to the NGINX controller, which routes to the gateway Pod." — good causal chain. -->
The kind cluster exposes port 80 on `localhost` via the `extraPortMappings` configured in section 12.1. With the hosts entry in place, `http://library.local` resolves to `127.0.0.1:80`, which kind forwards to the NGINX controller, which routes to the gateway Pod.

<!-- [LINE EDIT] "If you later find that NGINX terminates long-running requests prematurely, you can add timeout annotations like `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"`." — useful practical note. -->
If you later find that NGINX terminates long-running requests prematurely, you can add timeout annotations like `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"`. For our use case — short HTTP requests proxied to gRPC backends — the defaults are sufficient.

---

## Kustomization

<!-- [STRUCTURAL] The Kustomization file here is a preview of Kustomize, which is formally introduced in section 12.5. Either remove this section (defer entirely to 12.5) or add a brief "Kustomize is covered in section 12.5; for now, here's the file that ties the manifests together." The current presentation assumes the reader already knows Kustomize. -->
<!-- [LINE EDIT] "Kustomize assembles resources from a list of files." — OK but assumes knowledge. -->
Kustomize assembles resources from a list of files. The `kustomization.yaml` for the `library` namespace lists everything declared in this section:

```yaml
# deploy/k8s/base/library/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - namespace.yaml
  - auth-configmap.yaml
  - auth-deployment.yaml
  - auth-service.yaml
  - catalog-configmap.yaml
  - catalog-deployment.yaml
  - catalog-service.yaml
  - reservation-configmap.yaml
  - reservation-deployment.yaml
  - reservation-service.yaml
  - search-configmap.yaml
  - search-deployment.yaml
  - search-service.yaml
  - gateway-configmap.yaml
  - gateway-deployment.yaml
  - gateway-service.yaml
  - ingress.yaml
```

Apply the whole namespace with:

```bash
kubectl apply -k deploy/k8s/base/library/
```

Or render the assembled YAML without applying (useful for review):

```bash
kubectl kustomize deploy/k8s/base/library/
```

<!-- [LINE EDIT] "The order of `resources` matters for readability but not for correctness — `kubectl apply` handles resource creation order internally, retrying dependencies that are not yet ready." — slight overstatement: `kubectl apply` doesn't "retry" per se, but the API server is eventually consistent and controllers handle dependency readiness. Consider: "…the API server and controllers handle dependency ordering through eventual consistency." -->
The order of `resources` matters for readability but not for correctness — `kubectl apply` handles resource creation order internally, retrying dependencies that are not yet ready.

---

## Summary

<!-- [LINE EDIT] "a Deployment that runs the container with probes, resource bounds, and environment injection; a ClusterIP Service for stable in-cluster DNS; and a ConfigMap for non-sensitive configuration." — serial comma with semicolons; good CMOS 6.60. -->
You now have complete Kubernetes manifests for all five application services. The pattern is consistent: a Deployment that runs the container with probes, resource bounds, and environment injection; a ClusterIP Service for stable in-cluster DNS; and a ConfigMap for non-sensitive configuration. Secrets are declared as placeholders — the local overlay in section 12.5 substitutes real values via `secretGenerator`.

Three things to carry forward:

<!-- [COPY EDIT] Numbered list: CMOS 6.132 — periods after each item (since they're full sentences). Correctly punctuated. -->
1. `imagePullPolicy: IfNotPresent` is mandatory for images loaded into kind locally. Without it, every pod start fails with `ErrImagePull`.
<!-- [COPY EDIT] Item 2 states "The ordering is sequential, not lexicographic." — terse and accurate. -->
2. In the `env` list, variables that reference other variables via `$(VAR_NAME)` must be declared after the variables they reference. The ordering is sequential, not lexicographic.
<!-- [COPY EDIT] "Base64 encoding is not protection." — strong and correct. -->
3. Base64 encoding is not protection. Treat Secret manifests as sensitive files — never commit real values to source control.

<!-- [STRUCTURAL] "Section 12.4 assembles the top-level `kustomization.yaml` that ties all three namespaces together, and section 12.5 adds the local overlay with image name patches and `secretGenerator` entries." — but based on the file list, 12.4 is infra-manifests (StatefulSets for databases, Kafka, Meilisearch), not a top-level Kustomize assembly. The forward reference is wrong. Reconcile with actual chapter outline. -->
<!-- [FINAL] Forward reference appears factually incorrect — verify against table of contents / index.md. -->
Section 12.4 assembles the top-level `kustomization.yaml` that ties all three namespaces together, and section 12.5 adds the local overlay with image name patches and `secretGenerator` entries.

---

[^1]: Deployments: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
[^2]: Services: https://kubernetes.io/docs/concepts/services-networking/service/
[^3]: Ingress: https://kubernetes.io/docs/concepts/services-networking/ingress/
[^4]: ConfigMaps: https://kubernetes.io/docs/concepts/configuration/configmap/
[^5]: Secrets: https://kubernetes.io/docs/concepts/configuration/secret/
