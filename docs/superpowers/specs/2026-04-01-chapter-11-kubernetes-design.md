# Chapter 11: Kubernetes — Design Spec

## Goal

Take the reader from Docker Compose (Chapter 3) to a fully operational Kubernetes deployment on a local `kind` cluster. By the end, all 5 application services and their stateful infrastructure (3x Postgres, Kafka, Meilisearch) run in Kubernetes with health checks, graceful shutdown, resource limits, Ingress routing, and Kustomize-based environment management. The chapter also prepares the groundwork for Chapter 12's EKS deployment by stubbing a production overlay.

## Context

### What Exists

- **5 services** with multi-stage Dockerfiles: gateway (HTTP :8080), auth (:50051), catalog (:50052), reservation (:50053), search (:50054).
- **Docker Compose** (`deploy/docker-compose.yml`) orchestrates all services + Postgres (3 instances), Kafka (KRaft mode), Meilisearch, and the Grafana observability stack.
- **All services** read config from environment variables (`DATABASE_URL`, `GRPC_PORT`, `JWT_SECRET`, `KAFKA_BROKERS`, `OTEL_COLLECTOR_ENDPOINT`, etc.).
- **Earthly** builds Docker images (`+docker` target per service).
- **Gateway** has an HTTP health endpoint (`/healthz`). The 4 gRPC services have no health checks.
- **No graceful shutdown** — none of the services handle `SIGTERM`. They exit immediately when Docker Compose stops them. This is acceptable in Compose but breaks Kubernetes rolling updates.
- **No `deploy/k8s/` directory** — Kubernetes manifests do not exist yet.

### What's Missing

1. gRPC health checking protocol implementation in auth, catalog, reservation, search.
2. Graceful shutdown handling in all 5 services.
3. Kubernetes manifests for all services and infrastructure.
4. Kustomize structure for environment management (local vs production).
5. `kind` cluster configuration and deployment workflow.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Local cluster tool | kind | Lightweight, fast, first-class Ingress support, ships K8s 1.27+ |
| Manifest management | Kustomize (base + overlays) | Built into kubectl, no extra tools, teaches real-world environment management, sets up Chapter 12 |
| Health checks | gRPC native probes (K8s 1.24+) | Idiomatic, no sidecar needed, kind supports it |
| Graceful shutdown | `signal.NotifyContext` + `GracefulStop`/`Shutdown` | Required for rolling updates, small code change, good teaching moment |
| Ingress controller | NGINX Ingress Controller | Most common, well-documented for kind, maps to ALB in Chapter 12 |
| Infrastructure in K8s | Full StatefulSets with PVCs | Teaches core K8s concepts (StatefulSets, PVCs, headless Services) |
| Namespace layout | 3 namespaces: library, data, messaging | Matches arch spec, teaches namespace isolation |
| Kafka mode | KRaft (no Zookeeper) | Matches arch spec and Docker Compose setup, modern approach |
| Manifest walkthrough style | One service in full detail, rest concise | Avoids repetition, reader applies pattern |

## Chapter Structure

### 11.1 — Kubernetes Fundamentals

Conceptual introduction. No code.

**Content:**
- Map Docker Compose concepts to K8s equivalents: containers → Pods, `docker-compose.yml` → Deployments + Services, `depends_on` → readiness probes, `ports:` → Services + Ingress, `volumes:` → PVCs.
- Control plane components: API server, scheduler, etcd, controller manager.
- Node components: kubelet, kube-proxy.
- The declarative model: "desired state" vs Docker Compose's imperative "start these containers".
- Key resource types overview: Pod, Deployment, Service, StatefulSet, ConfigMap, Secret, Ingress, PersistentVolumeClaim.
- `kubectl` basics: `apply`, `get`, `describe`, `logs`, `delete`.

**Length:** ~150-200 lines.

### 11.2 — Local Cluster with kind

Hands-on setup.

**Content:**
- Install `kind` and verify.
- Create a cluster with a config file:
  - `extraPortMappings` on ports 80/443 for NGINX Ingress.
  - `node-labels` for `ingress-ready=true`.
- Install the NGINX Ingress Controller via the kind-specific manifest (`kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml`).
- Verify: `kubectl cluster-info`, `kubectl get nodes`, `kubectl get pods -n ingress-nginx`.
- Explain the `kind load docker-image` workflow — kind runs containers-in-containers, so it can't pull from the local Docker daemon. This is the #1 kind gotcha.
- Tie back to Chapter 9's Earthly `+docker` targets for building images.

**Length:** ~100-150 lines.

### 11.3 — Preparing Services for Kubernetes

Code changes to the Go services. Two additions.

**Graceful shutdown:**
- Add `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)` to each service's `main.go`.
- gRPC services: `grpcServer.GracefulStop()` drains in-flight RPCs.
- Gateway: `httpServer.Shutdown(ctx)` drains HTTP connections.
- Kafka consumers: context cancellation triggers `ConsumerGroup.Close()`, committing offsets and leaving the group.
- Explain the Kubernetes pod termination lifecycle: `SIGTERM` → `terminationGracePeriodSeconds` (default 30s) → `SIGKILL`.
- Show one service (catalog) in full detail with the diff, then summarize the pattern for others.

**gRPC health checks:**
- Add `google.golang.org/grpc/health` and `google.golang.org/grpc/health/healthpb` to each gRPC service.
- Register the health server: `healthServer := health.NewServer()` + `healthpb.RegisterHealthServer(grpcServer, healthServer)`.
- Set status to `SERVING` after startup, `NOT_SERVING` during shutdown (before `GracefulStop`).
- Explain the gRPC Health Checking Protocol (standard `grpc.health.v1.Health/Check`).
- Show how Kubernetes uses the native `grpc` probe type (available since K8s 1.24).

**Files modified:**
- `services/catalog/cmd/main.go` — full diff shown
- `services/auth/cmd/main.go` — same pattern
- `services/reservation/cmd/main.go` — same pattern
- `services/search/cmd/main.go` — same pattern
- `services/gateway/cmd/main.go` — HTTP variant (already has `/healthz`, add graceful shutdown)

**Length:** ~200-250 lines.

### 11.4 — Kubernetes Manifests: Application Services

Core manifest-writing section. All resources in the `library` namespace.

**Per service (Deployment + ClusterIP Service):**
- Container image, `imagePullPolicy: IfNotPresent` (for kind).
- Environment variables via `envFrom` (ConfigMap) and `env[].valueFrom.secretKeyRef` (Secret).
- Resource requests and limits (`cpu`, `memory`).
- Liveness probe: gRPC health check for backend services, HTTP GET `/healthz` for gateway.
- Readiness probe: same endpoints, lower `initialDelaySeconds`.
- `terminationGracePeriodSeconds: 30` aligned with graceful shutdown code.
- Single replica (Kustomize production overlay can scale up).

**ConfigMaps:**
- One per service for non-sensitive config: `GRPC_PORT`, `KAFKA_BROKERS` (pointing to `kafka-0.kafka.messaging.svc.cluster.local:9092`), `CATALOG_GRPC_ADDR` (pointing to `catalog.library.svc.cluster.local:50052`), `OTEL_COLLECTOR_ENDPOINT`, etc.
- Cross-namespace DNS names for infrastructure services.

**Secrets:**
- Shared `jwt-secret` for `JWT_SECRET` (used by auth, catalog, reservation, gateway — all use `pkgauth.UnaryAuthInterceptor`).
- Per-service database Secrets (`POSTGRES_USER`, `POSTGRES_PASSWORD`).
- `meilisearch-secret` for `MEILI_MASTER_KEY`.
- Teaching point: K8s Secrets are base64-encoded, not encrypted. Real encryption requires encryption-at-rest or external secret managers (deferred to Chapter 12).
- Note: base manifests define Secret resources as placeholders. The local overlay uses `secretGenerator` with literal values to replace them, which is the Kustomize-idiomatic approach. The base `secrets.yaml` exists so the Deployments can reference Secret names consistently.

**Ingress:**
- NGINX Ingress resource routing `library.local` → gateway Service on port 8080.
- `ingressClassName: nginx`.
- Reader adds `127.0.0.1 library.local` to `/etc/hosts`.

**Walkthrough style:** Full detail for catalog (every field explained with why), then concise manifests for the remaining 4 services since they follow the same pattern.

**File structure:**
```
deploy/k8s/base/library/
  namespace.yaml
  catalog-deployment.yaml
  catalog-service.yaml
  catalog-configmap.yaml
  auth-deployment.yaml
  auth-service.yaml
  auth-configmap.yaml
  reservation-deployment.yaml
  reservation-service.yaml
  reservation-configmap.yaml
  search-deployment.yaml
  search-service.yaml
  search-configmap.yaml
  gateway-deployment.yaml
  gateway-service.yaml
  gateway-configmap.yaml
  secrets.yaml
  ingress.yaml
  kustomization.yaml
```

**Length:** ~400-500 lines.

### 11.5 — Kubernetes Manifests: Stateful Infrastructure

The `data` and `messaging` namespaces. Teaches StatefulSets.

**Contrast with Deployments:** Stable network identities (ordinal naming), ordered pod management, `volumeClaimTemplates` for per-pod storage, headless Services for DNS.

**PostgreSQL (3 instances in `data` namespace):**
- StatefulSet with `volumeClaimTemplates` for persistent data.
- Headless Service for stable DNS (`postgres-catalog-0.postgres-catalog.data.svc.cluster.local`).
- ConfigMap for `POSTGRES_DB`, `POSTGRES_USER`.
- Secret for `POSTGRES_PASSWORD`.
- Readiness probe: `exec` with `pg_isready`.
- Walk through catalog Postgres fully, then show auth and reservation as variations (different DB names).

**Kafka (single-node KRaft in `messaging` namespace):**
- StatefulSet with PVC for log segments.
- KRaft mode — no Zookeeper, same as Docker Compose. Note that both environments use KRaft; the key difference is the networking and advertised listener configuration.
- ConfigMap for broker configuration: `KAFKA_NODE_ID`, `KAFKA_PROCESS_ROLES`, `KAFKA_CONTROLLER_QUORUM_VOTERS`, `KAFKA_LISTENERS`, `KAFKA_ADVERTISED_LISTENERS`.
- Headless Service.
- `KAFKA_ADVERTISED_LISTENERS` must use the in-cluster DNS name (`kafka-0.kafka.messaging.svc.cluster.local:9092`), not `kafka:9092` from Docker Compose.

**Meilisearch (in `data` namespace):**
- StatefulSet with PVC for index data.
- ConfigMap for `MEILI_ENV`, `MEILI_NO_ANALYTICS`.
- Secret for `MEILI_MASTER_KEY`.
- HTTP readiness probe on `/health`.
- Simpler than Postgres/Kafka — good for the reader to apply the learned pattern.

**Cross-namespace service discovery:**
- How services in `library` reach infrastructure in `data` and `messaging`.
- Fully qualified DNS: `<pod>.<headless-svc>.<namespace>.svc.cluster.local`.
- Short form: `<service>.<namespace>.svc.cluster.local` (for Services, not individual pods).
- ConfigMaps in `library` namespace reference these DNS names.

**File structure:**
```
deploy/k8s/base/data/
  namespace.yaml
  postgres-catalog-statefulset.yaml
  postgres-catalog-service.yaml
  postgres-catalog-configmap.yaml
  postgres-auth-statefulset.yaml
  postgres-auth-service.yaml
  postgres-auth-configmap.yaml
  postgres-reservation-statefulset.yaml
  postgres-reservation-service.yaml
  postgres-reservation-configmap.yaml
  meilisearch-statefulset.yaml
  meilisearch-service.yaml
  meilisearch-configmap.yaml
  secrets.yaml
  kustomization.yaml

deploy/k8s/base/messaging/
  namespace.yaml
  kafka-statefulset.yaml
  kafka-service.yaml
  kafka-configmap.yaml
  kustomization.yaml
```

**Length:** ~350-400 lines.

### 11.6 — Kustomize: Managing Environments

Introduces Kustomize for environment management.

**Directory structure:**
```
deploy/k8s/
  base/
    library/
    data/
    messaging/
    kustomization.yaml       # references all three namespace kustomizations
  overlays/
    local/
      kustomization.yaml     # kind-specific patches
    production/
      kustomization.yaml     # EKS stub for Chapter 12
```

**Base `kustomization.yaml`:** Lists all resources from the three namespace directories.

**Local overlay patches:**
- `imagePullPolicy: IfNotPresent` (images loaded via `kind load`).
- Small resource limits (suitable for a laptop: 128Mi-256Mi memory, 100m-250m CPU).
- Single-replica StatefulSets.
- Development-mode env vars (`MEILI_ENV: development`).
- `secretGenerator` with literal values (acceptable for local dev, not for production).

**Production overlay (stub):**
- Skeleton `kustomization.yaml` with comments explaining what Chapter 12 fills in.
- Larger resource limits.
- Real Secrets via external-secrets or sealed-secrets.
- RDS endpoints replacing Postgres StatefulSets.
- Image tags from CI pipeline.
- Multiple replicas for application services.

**Usage:** `kubectl apply -k deploy/k8s/overlays/local` deploys everything.

**Kustomize concepts taught:**
- `resources` — referencing base manifests.
- `patches` — strategic merge patches and JSON patches.
- `secretGenerator` / `configMapGenerator` — generating resources with hash suffixes.
- `namePrefix` / `commonLabels` — optional but mentioned.

**Length:** ~200-250 lines.

### 11.7 — Deploying and Verifying

End-to-end walkthrough. The payoff.

**Build and load images:**
- `earthly +docker` for all 5 services.
- `kind load docker-image <image>` for each.
- Verify with `docker exec kind-control-plane crictl images`.

**Deploy in order:**
1. `kubectl apply -k deploy/k8s/overlays/local` — Kustomize applies everything.
2. Wait for infrastructure: `kubectl wait --for=condition=ready pod -l app=postgres-catalog -n data --timeout=120s`.
3. Verify application pods come up: `kubectl get pods -n library`.
4. Explain: readiness probes handle the startup race. Pods restart until dependencies are ready. Kubernetes handles this; `depends_on` is not needed.

**Verification checklist:**
- `kubectl get pods -A` — all pods Running/Ready.
- `kubectl logs -n library deployment/catalog` — clean startup, migrations ran, Kafka consumer joined group.
- `curl http://library.local` through Ingress — gateway responds.
- `kubectl port-forward -n library svc/catalog 50052:50052` + `grpcurl -plaintext localhost:50052 grpc.health.v1.Health/Check` — responds SERVING.
- Create a book via the UI at `http://library.local`, verify it appears in search (proves full event flow: gateway → catalog → Kafka → search → Meilisearch).

**Troubleshooting guide:**
- `ImagePullBackOff` — forgot `kind load docker-image`.
- `CrashLoopBackOff` — wrong env var or database not ready yet (check logs).
- `Pending` PVC — storage class mismatch (kind uses `standard` by default).
- DNS resolution failures — wrong namespace in service address, missing `.svc.cluster.local` suffix.
- Ingress not responding — NGINX controller not ready, missing `ingressClassName`, `/etc/hosts` not updated.

**Length:** ~200-250 lines.

## Non-Goals

- **Helm charts** — Kustomize is sufficient for this project's complexity. Helm could be a future chapter.
- **Multi-replica application services** — single replica in local, scaling discussed conceptually but not deployed.
- **TLS/cert-manager** — deferred to Chapter 12 with real DNS.
- **Horizontal Pod Autoscaling** — mentioned as a future topic, not implemented.
- **Observability stack in K8s** — the Grafana stack (Tempo, Prometheus, Loki, Grafana) from Chapter 8 stays in Docker Compose for now. Moving it to K8s is a significant effort and not the focus. The OTel Collector could be deployed as a DaemonSet but this is deferred. For the kind deployment, `OTEL_COLLECTOR_ENDPOINT` in ConfigMaps should be left empty or set to a no-op value — services already handle missing collector endpoints gracefully (Chapter 8). The verification section should note that telemetry is not collected in the kind cluster.
- **Network policies** — mentioned conceptually (namespace isolation) but not implemented.

## Dependencies

- **Chapter 3** — Dockerfiles and Docker Compose understanding.
- **Chapter 6** — Kafka concepts (KRaft vs Zookeeper).
- **Chapter 8** — OTel collector endpoint config.
- **Chapter 9** — Earthly `+docker` targets for building images.

## Code Changes Summary

| Service | Change | Scope |
|---------|--------|-------|
| catalog | Graceful shutdown + gRPC health server | `cmd/main.go` (~15 lines) |
| auth | Graceful shutdown + gRPC health server | `cmd/main.go` (~15 lines) |
| reservation | Graceful shutdown + gRPC health server | `cmd/main.go` (~15 lines) |
| search | Graceful shutdown + gRPC health server | `cmd/main.go` (~15 lines) |
| gateway | Graceful shutdown (already has `/healthz`) | `cmd/main.go` (~10 lines) |

## New Files Summary

| Path | Type | Description |
|------|------|-------------|
| `docs/src/ch11/index.md` | Documentation | K8s fundamentals |
| `docs/src/ch11/kind-setup.md` | Documentation | Local cluster setup |
| `docs/src/ch11/preparing-services.md` | Documentation | Graceful shutdown + health checks |
| `docs/src/ch11/app-manifests.md` | Documentation | Application service manifests |
| `docs/src/ch11/infra-manifests.md` | Documentation | StatefulSet infrastructure manifests |
| `docs/src/ch11/kustomize.md` | Documentation | Kustomize environments |
| `docs/src/ch11/deploying.md` | Documentation | Deploy and verify walkthrough |
| `deploy/k8s/base/**` | K8s manifests | All base manifests (~25 files) |
| `deploy/k8s/overlays/local/**` | K8s manifests | Local overlay patches |
| `deploy/k8s/overlays/production/**` | K8s manifests | Production stub |
