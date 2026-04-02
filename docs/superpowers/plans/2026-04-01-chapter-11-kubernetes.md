# Chapter 11: Kubernetes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write Chapter 11 documentation (7 sections) and implement the code changes (graceful shutdown, gRPC health checks) and Kubernetes manifests (Kustomize base + overlays) needed to deploy the library system to a local `kind` cluster.

**Architecture:** Documentation sections first (11.1-11.7), then code changes to services (graceful shutdown + health checks), then Kubernetes manifests (base per namespace), then Kustomize overlays. Documentation tasks are independent and can be parallelized. Code change tasks are independent per service. Manifest tasks depend on understanding the code changes (for probe config) but not on the code being committed first.

**Tech Stack:** Kubernetes (kind), Kustomize, NGINX Ingress Controller, gRPC health checking protocol (`google.golang.org/grpc/health`), `signal.NotifyContext`

---

## File Structure

### Documentation (new)
- `docs/src/ch11/index.md` — Kubernetes fundamentals, Compose-to-K8s mapping
- `docs/src/ch11/kind-setup.md` — kind installation, cluster creation, NGINX Ingress
- `docs/src/ch11/preparing-services.md` — Graceful shutdown + gRPC health checks
- `docs/src/ch11/app-manifests.md` — Deployments, Services, ConfigMaps, Secrets, Ingress
- `docs/src/ch11/infra-manifests.md` — StatefulSets for Postgres, Kafka, Meilisearch
- `docs/src/ch11/kustomize.md` — Kustomize base/overlay structure
- `docs/src/ch11/deploying.md` — Full deployment walkthrough and troubleshooting
- `docs/src/SUMMARY.md` — Add Chapter 11 entries

### Service code (modified)
- `services/catalog/cmd/main.go` — Add gRPC health server + graceful shutdown wiring
- `services/auth/cmd/main.go` — Add signal handling, gRPC health server, graceful shutdown
- `services/reservation/cmd/main.go` — Add signal handling, gRPC health server, graceful shutdown
- `services/search/cmd/main.go` — Add gRPC health server + graceful shutdown wiring
- `services/gateway/cmd/main.go` — Add signal handling + `http.Server` graceful shutdown

### Kubernetes manifests (new)
- `deploy/k8s/base/library/` — namespace, 5x Deployment+Service+ConfigMap, secrets, ingress, kustomization
- `deploy/k8s/base/data/` — namespace, 3x Postgres StatefulSet+Service+ConfigMap, Meilisearch StatefulSet+Service+ConfigMap, secrets, kustomization
- `deploy/k8s/base/messaging/` — namespace, Kafka StatefulSet+Service+ConfigMap, kustomization
- `deploy/k8s/base/kustomization.yaml` — Root base referencing all three namespaces
- `deploy/k8s/overlays/local/kustomization.yaml` — kind-specific patches and secretGenerator
- `deploy/k8s/overlays/production/kustomization.yaml` — EKS stub for Chapter 12

---

### Task 1: Write Section 11.1 — Kubernetes Fundamentals

**Files:**
- Create: `docs/src/ch11/index.md`
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Create the chapter index file**

Write `docs/src/ch11/index.md` covering:

1. Opening: Docker Compose (Chapter 3) got us running locally — but it's a single-machine orchestrator. Kubernetes is the production-grade answer: self-healing, rolling updates, service discovery, declarative infrastructure.
2. Compose-to-K8s concept mapping table:

| Docker Compose | Kubernetes | Purpose |
|----------------|------------|---------|
| `container` | Pod | Smallest deployable unit |
| `services:` block | Deployment | Manages replica sets and rolling updates |
| `ports:` | Service (ClusterIP/NodePort) | Internal service discovery and load balancing |
| `depends_on` | Readiness probes | Health-based dependency management |
| `volumes:` | PersistentVolumeClaim | Persistent storage |
| `docker-compose.yml` | Manifests (YAML) | Declarative desired state |
| Port mapping to host | Ingress | External traffic routing |

3. Control plane components (brief — API server, scheduler, etcd, controller manager) with a Mermaid diagram showing the relationship.
4. Node components: kubelet (runs pods), kube-proxy (networking).
5. The declarative model: "I want 3 replicas of catalog" vs "start catalog". Kubernetes reconciles continuously.
6. Key resource types overview (one paragraph each): Pod, Deployment, Service, StatefulSet, ConfigMap, Secret, Ingress, PersistentVolumeClaim.
7. `kubectl` basics: `apply -f`, `get`, `describe`, `logs`, `delete`, `port-forward`. Show example commands.
8. What we're building: a 3-namespace architecture diagram (library, data, messaging).

Target length: ~150-200 lines. No code in this section.

Include references:
- [^1]: Kubernetes Documentation: https://kubernetes.io/docs/home/
- [^2]: Kubernetes Components: https://kubernetes.io/docs/concepts/overview/components/
- [^3]: kubectl Cheat Sheet: https://kubernetes.io/docs/reference/kubectl/cheatsheet/

- [ ] **Step 2: Update SUMMARY.md**

Add Chapter 11 entries to `docs/src/SUMMARY.md` after the Chapter 10 section:

```markdown
- [Chapter 11: Kubernetes](./ch11/index.md)
  - [11.1 Local Cluster with kind](./ch11/kind-setup.md)
  - [11.2 Preparing Services for Kubernetes](./ch11/preparing-services.md)
  - [11.3 Application Manifests](./ch11/app-manifests.md)
  - [11.4 Infrastructure Manifests](./ch11/infra-manifests.md)
  - [11.5 Kustomize Environments](./ch11/kustomize.md)
  - [11.6 Deploying and Verifying](./ch11/deploying.md)
```

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch11/index.md docs/src/SUMMARY.md
git commit -m "docs: add Chapter 11 index and Kubernetes fundamentals"
```

---

### Task 2: Write Section 11.2 — Local Cluster with kind

**Files:**
- Create: `docs/src/ch11/kind-setup.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/kind-setup.md` covering:

1. What kind is: "Kubernetes IN Docker" — runs K8s nodes as Docker containers. Fast to create/destroy, perfect for local development.
2. Installation: `go install sigs.k8s.io/kind@latest` or package manager. Also need `kubectl`.
3. Cluster config file (`kind-config.yaml`):

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
```

4. Create the cluster: `kind create cluster --config kind-config.yaml --name library`
5. Verify: `kubectl cluster-info --context kind-library`, `kubectl get nodes`
6. Install NGINX Ingress Controller for kind:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

7. The `kind load` gotcha: kind nodes are Docker containers, so they can't pull from your local Docker daemon. You must explicitly load images:

```bash
kind load docker-image library-system/catalog:latest --name library
```

8. Tie back to Chapter 9: `earthly +docker` builds the images, then `kind load` makes them available in the cluster.
9. Cleanup: `kind delete cluster --name library`

Target length: ~100-150 lines.

Include references:
- [^1]: kind Quick Start: https://kind.sigs.k8s.io/docs/user/quick-start/
- [^2]: kind Ingress: https://kind.sigs.k8s.io/docs/user/ingress/
- [^3]: kind Loading Images: https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/kind-setup.md
git commit -m "docs: write Section 11.2 local cluster with kind"
```

---

### Task 3: Write Section 11.3 — Preparing Services for Kubernetes

**Files:**
- Create: `docs/src/ch11/preparing-services.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/preparing-services.md` covering:

1. Opening: before writing K8s manifests, two code changes are needed — graceful shutdown and health checks. Docker Compose was forgiving about both; Kubernetes is not.

2. **Graceful shutdown** subsection:
   - The Kubernetes pod termination lifecycle (Mermaid sequence diagram):
     1. Pod marked for deletion
     2. Endpoints removed from Service (no new traffic)
     3. `preStop` hook runs (if any)
     4. `SIGTERM` sent to PID 1
     5. `terminationGracePeriodSeconds` countdown (default 30s)
     6. `SIGKILL` if still running
   - Why it matters: during rolling updates, old pods get `SIGTERM`. Without handling, in-flight RPCs are dropped, Kafka consumers don't commit offsets, DB connections leak.
   - Pattern: `signal.NotifyContext` creates a context that cancels on `SIGTERM`/`SIGINT`. Pass this context to all long-running goroutines.
   - Full catalog `main.go` diff showing:
     - Existing `signal.NotifyContext` (already present at line 99) — good, we just need to wire it to shutdown
     - Add goroutine that waits for context cancellation, then calls `grpcServer.GracefulStop()`
     - The server's `Serve` call returns when `GracefulStop` is called
   - Auth diff: add `signal.NotifyContext`, add shutdown goroutine (auth currently has no signal handling)
   - Reservation diff: same pattern as auth
   - Search diff: already has `signal.NotifyContext` (line 53), add shutdown goroutine
   - Gateway diff: add `signal.NotifyContext`, create `http.Server{}` struct, replace `http.ListenAndServe` with `server.ListenAndServe`, add `server.Shutdown(ctx)` on signal

3. **gRPC health checks** subsection:
   - The gRPC Health Checking Protocol: standard `grpc.health.v1.Health/Check` RPC. Kubernetes 1.24+ has native support via `grpc` probe type.
   - Add `google.golang.org/grpc/health` package (already in the grpc module, no new dependency).
   - Pattern for each gRPC service:
     ```go
     import (
         "google.golang.org/grpc/health"
         healthpb "google.golang.org/grpc/health/healthpb"
     )
     
     healthServer := health.NewServer()
     healthpb.RegisterHealthServer(grpcServer, healthServer)
     healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
     // In shutdown goroutine, before GracefulStop:
     healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
     ```
   - Show how K8s uses it in a Deployment spec:
     ```yaml
     livenessProbe:
       grpc:
         port: 50052
       initialDelaySeconds: 5
       periodSeconds: 10
     readinessProbe:
       grpc:
         port: 50052
       initialDelaySeconds: 2
       periodSeconds: 5
     ```
   - Gateway already has `/healthz` — show HTTP probe equivalent.

4. Testing the changes: `go build ./services/*/cmd/` to verify compilation, `grpcurl` to test health endpoint locally.

Target length: ~200-250 lines.

Include references:
- [^1]: Kubernetes Pod Lifecycle: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/
- [^2]: gRPC Health Checking Protocol: https://github.com/grpc/grpc/blob/master/doc/health-checking.md
- [^3]: Configure Liveness, Readiness Probes: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/preparing-services.md
git commit -m "docs: write Section 11.3 preparing services for Kubernetes"
```

---

### Task 4: Write Section 11.4 — Application Manifests

**Files:**
- Create: `docs/src/ch11/app-manifests.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/app-manifests.md` covering:

1. Opening: with services ready for K8s, we write the manifests. All application services go in the `library` namespace. Each service needs a Deployment, a Service, and a ConfigMap.

2. **Namespace** — explain what namespaces are and why we use three. Show `namespace.yaml`:
   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: library
   ```

3. **Catalog Deployment — full walkthrough** (every field explained):
   - `apiVersion: apps/v1`, `kind: Deployment`
   - `metadata.name`, `metadata.namespace`, `metadata.labels`
   - `spec.replicas: 1`
   - `spec.selector.matchLabels` — must match pod template labels
   - Pod template: container spec with `image`, `imagePullPolicy: IfNotPresent`, `ports`, `envFrom` (ConfigMap), `env` (Secrets), resource requests/limits, liveness/readiness probes (gRPC), `terminationGracePeriodSeconds: 30`
   - Teaching point: `imagePullPolicy: IfNotPresent` is critical for kind — images are loaded locally, not pulled from a registry.

4. **Catalog Service** — ClusterIP (default), `selector` matching Deployment labels, port mapping (50052 → 50052). Explain: Services provide stable DNS names within the cluster.

5. **Catalog ConfigMap** — non-sensitive config: `GRPC_PORT: "50052"`, `KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"`, `OTEL_COLLECTOR_ENDPOINT: ""` (telemetry not collected in kind cluster). Explain cross-namespace DNS.

6. **Remaining 4 services** — show manifests concisely (less commentary, same pattern):
   - auth: Deployment+Service+ConfigMap (port 50051, DATABASE_URL points to `postgres-auth-0.postgres-auth.data.svc.cluster.local`)
   - reservation: Deployment+Service+ConfigMap (port 50053, needs CATALOG_GRPC_ADDR, KAFKA_BROKERS, MAX_ACTIVE_RESERVATIONS)
   - search: Deployment+Service+ConfigMap (port 50054, needs MEILI_URL, CATALOG_GRPC_ADDR, KAFKA_BROKERS)
   - gateway: Deployment+Service+ConfigMap (port 8080, HTTP liveness probe on `/healthz`, needs all `*_GRPC_ADDR` env vars)

7. **Secrets** — `secrets.yaml` as placeholder:
   - `jwt-secret` with `JWT_SECRET` key
   - `postgres-catalog-secret`, `postgres-auth-secret`, `postgres-reservation-secret` with `POSTGRES_PASSWORD`
   - `meilisearch-secret` with `MEILI_MASTER_KEY`
   - Teaching point: base64 encoding is not encryption. Real secret management in Chapter 12.
   - Note: local overlay will use `secretGenerator` to provide actual values.

8. **Ingress** — NGINX Ingress routing `library.local` → gateway:8080:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: gateway-ingress
     namespace: library
     annotations:
       nginx.ingress.kubernetes.io/rewrite-target: /
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
   - Reader adds `127.0.0.1 library.local` to `/etc/hosts`.

9. **Kustomization** for this namespace — `kustomization.yaml` listing all resources.

Target length: ~400-500 lines.

Include references:
- [^1]: Deployments: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
- [^2]: Services: https://kubernetes.io/docs/concepts/services-networking/service/
- [^3]: Ingress: https://kubernetes.io/docs/concepts/services-networking/ingress/
- [^4]: ConfigMaps: https://kubernetes.io/docs/concepts/configuration/configmap/
- [^5]: Secrets: https://kubernetes.io/docs/concepts/configuration/secret/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/app-manifests.md
git commit -m "docs: write Section 11.4 application service manifests"
```

---

### Task 5: Write Section 11.5 — Infrastructure Manifests

**Files:**
- Create: `docs/src/ch11/infra-manifests.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/infra-manifests.md` covering:

1. Opening: application services are stateless — they can be replaced freely. Infrastructure (Postgres, Kafka, Meilisearch) is stateful — data must survive pod restarts. Kubernetes has a dedicated resource for this: StatefulSet.

2. **StatefulSet vs Deployment** comparison:
   - Stable network identity: pods get predictable names (`postgres-catalog-0`, not `postgres-catalog-7b4f9-xk2p`)
   - Ordered startup/shutdown: pod-0 must be ready before pod-1 starts
   - `volumeClaimTemplates`: each pod gets its own PVC (not shared)
   - Headless Service: no load balancing, DNS resolves directly to pod IPs

3. **PostgreSQL — Catalog (full walkthrough)**:
   - Headless Service (`clusterIP: None`) with port 5432. Explain: this gives us `postgres-catalog-0.postgres-catalog.data.svc.cluster.local`.
   - StatefulSet: single replica, `postgres:16-alpine` image, `volumeClaimTemplates` for `/var/lib/postgresql/data` (1Gi `ReadWriteOnce`), env from ConfigMap + Secret, readiness probe with `exec: pg_isready -U postgres`.
   - ConfigMap: `POSTGRES_DB: catalog`, `POSTGRES_USER: postgres`.
   - Secret reference: `POSTGRES_PASSWORD` from `postgres-catalog-secret`.

4. **PostgreSQL — Auth and Reservation**: show concisely as variations (different DB names, same pattern).

5. **Kafka (messaging namespace)**:
   - Headless Service for `kafka.messaging.svc.cluster.local`.
   - StatefulSet with `apache/kafka:3.9` image. Single replica, KRaft mode.
   - ConfigMap with broker config:
     ```
     KAFKA_NODE_ID: "1"
     KAFKA_PROCESS_ROLES: "broker,controller"
     KAFKA_LISTENERS: "PLAINTEXT://:9092,CONTROLLER://:9093"
     KAFKA_ADVERTISED_LISTENERS: "PLAINTEXT://kafka-0.kafka.messaging.svc.cluster.local:9092"
     KAFKA_CONTROLLER_QUORUM_VOTERS: "1@kafka-0.kafka.messaging.svc.cluster.local:9093"
     KAFKA_CONTROLLER_LISTENER_NAMES: "CONTROLLER"
     KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
     KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
     ```
   - Key teaching point: `KAFKA_ADVERTISED_LISTENERS` must use the FQDN, not just `kafka:9092`. Clients in the `library` namespace connect cross-namespace.
   - PVC for `/var/lib/kafka/data`.
   - Compare with Docker Compose: same KRaft config, different networking.

6. **Meilisearch (data namespace)**:
   - Simpler StatefulSet. `getmeili/meilisearch:v1.12`, PVC for `/meili_data`, HTTP readiness probe on `/health`.
   - ConfigMap: `MEILI_ENV: development`, `MEILI_NO_ANALYTICS: "true"`.
   - Secret: `MEILI_MASTER_KEY` from `meilisearch-secret`.

7. **Cross-namespace service discovery** summary:
   - Application ConfigMaps use FQDNs: `kafka-0.kafka.messaging.svc.cluster.local:9092`, `postgres-catalog-0.postgres-catalog.data.svc.cluster.local:5432`
   - Mermaid diagram showing DNS resolution flow.

8. **Kustomization files** for `data/` and `messaging/` namespaces.

Target length: ~350-400 lines.

Include references:
- [^1]: StatefulSets: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/
- [^2]: Headless Services: https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
- [^3]: Persistent Volumes: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
- [^4]: DNS for Services and Pods: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/infra-manifests.md
git commit -m "docs: write Section 11.5 infrastructure StatefulSet manifests"
```

---

### Task 6: Write Section 11.6 — Kustomize Environments

**Files:**
- Create: `docs/src/ch11/kustomize.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/kustomize.md` covering:

1. Opening: we have ~30 manifest files. They work for kind, but EKS needs different values (real secrets, bigger resources, multiple replicas). Copy-pasting manifests per environment is a maintenance nightmare. Kustomize solves this.

2. **What Kustomize is**: built into `kubectl` (no install needed). A base contains shared manifests, overlays patch them per environment.

3. **Directory structure** (show the tree):
   ```
   deploy/k8s/
   ├── base/
   │   ├── kustomization.yaml      # references library/, data/, messaging/
   │   ├── library/
   │   │   ├── kustomization.yaml
   │   │   └── *.yaml
   │   ├── data/
   │   │   ├── kustomization.yaml
   │   │   └── *.yaml
   │   └── messaging/
   │       ├── kustomization.yaml
   │       └── *.yaml
   └── overlays/
       ├── local/
       │   └── kustomization.yaml
       └── production/
           └── kustomization.yaml
   ```

4. **Base `kustomization.yaml`**: simple — just references the three namespace directories.

5. **Local overlay** — what it patches and how:
   - `secretGenerator` with literal values (teaching: generates Secrets with content hash suffix, triggers pod rollout on secret changes):
     ```yaml
     secretGenerator:
       - name: jwt-secret
         namespace: library
         literals:
           - JWT_SECRET=dev-secret-change-in-production
       - name: postgres-catalog-secret
         namespace: data
         literals:
           - POSTGRES_PASSWORD=postgres
     # ... etc
     ```
   - Strategic merge patches for resource limits, imagePullPolicy.
   - Explain: `kubectl apply -k deploy/k8s/overlays/local` applies base + local patches.

6. **Production overlay (stub)** — commented skeleton showing what Chapter 12 fills in:
   - Larger resource limits
   - `imagePullPolicy: Always` with tagged images from CI
   - Multiple replicas for Deployments
   - External secrets (reference to external-secrets operator)
   - RDS endpoints replacing Postgres StatefulSets

7. **Kustomize concepts taught**:
   - `resources` — referencing other kustomizations
   - `patches` — strategic merge patches
   - `secretGenerator` / `configMapGenerator` — generating resources with hash suffixes
   - Preview: `kubectl kustomize deploy/k8s/overlays/local` to see rendered YAML

Target length: ~200-250 lines.

Include references:
- [^1]: Kustomize: https://kustomize.io/
- [^2]: Kustomize Built-in Reference: https://kubectl.docs.kubernetes.io/references/kustomize/
- [^3]: Managing Secrets with Kustomize: https://kubernetes.io/docs/tasks/configmap-secret/managing-secret-using-kustomize/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/kustomize.md
git commit -m "docs: write Section 11.6 Kustomize environment management"
```

---

### Task 7: Write Section 11.7 — Deploying and Verifying

**Files:**
- Create: `docs/src/ch11/deploying.md`

- [ ] **Step 1: Write the section**

Write `docs/src/ch11/deploying.md` covering:

1. Opening: everything is ready — code changes committed, manifests written, Kustomize configured. Time to deploy.

2. **Build and load images**:
   ```bash
   # Build all service images with Earthly
   earthly +docker
   
   # Load into kind cluster
   kind load docker-image library-system/gateway:latest --name library
   kind load docker-image library-system/auth:latest --name library
   kind load docker-image library-system/catalog:latest --name library
   kind load docker-image library-system/reservation:latest --name library
   kind load docker-image library-system/search:latest --name library
   
   # Verify images are available
   docker exec library-control-plane crictl images | grep library-system
   ```

3. **Deploy**:
   ```bash
   kubectl apply -k deploy/k8s/overlays/local
   ```
   - Explain: Kustomize creates namespaces, applies all resources. Kubernetes handles ordering — StatefulSets start, pods retry until dependencies are ready.
   - Wait for infrastructure:
   ```bash
   kubectl wait --for=condition=ready pod -l app=postgres-catalog -n data --timeout=120s
   kubectl wait --for=condition=ready pod -l app=kafka -n messaging --timeout=120s
   ```
   - Watch application pods:
   ```bash
   kubectl get pods -n library -w
   ```

4. **Verification checklist** (numbered steps the reader follows):
   1. All pods running: `kubectl get pods -A`
   2. Catalog logs clean: `kubectl logs -n library deployment/catalog`
   3. Health check working: `kubectl port-forward -n library svc/catalog 50052:50052` then `grpcurl -plaintext localhost:50052 grpc.health.v1.Health/Check`
   4. Gateway accessible via Ingress: `curl http://library.local` (after adding `/etc/hosts` entry)
   5. End-to-end flow: create a book via the UI, verify it appears in search

5. **Troubleshooting guide** — common issues as a table:

| Symptom | Cause | Fix |
|---------|-------|-----|
| `ImagePullBackOff` | Image not loaded into kind | `kind load docker-image <image> --name library` |
| `CrashLoopBackOff` | Bad env var or DB not ready | `kubectl logs <pod>` to find the error |
| `Pending` PVC | No matching StorageClass | kind uses `standard` by default; check `kubectl get sc` |
| DNS resolution failure | Wrong namespace in address | Use FQDN: `service.namespace.svc.cluster.local` |
| Ingress 404 | NGINX not ready or wrong config | `kubectl get pods -n ingress-nginx`; check `ingressClassName` |
| Pod stuck in `Init` | Init container failing | `kubectl describe pod <pod>` to see init container status |

6. **Cleanup**:
   ```bash
   kind delete cluster --name library
   ```

7. **What's next**: Chapter 12 takes these same manifests to AWS EKS. The production Kustomize overlay replaces kind-specific values with cloud resources (RDS, real secrets, autoscaling).

Target length: ~200-250 lines.

Include references:
- [^1]: Debugging Pods: https://kubernetes.io/docs/tasks/debug/debug-application/debug-pods/
- [^2]: Debugging Services: https://kubernetes.io/docs/tasks/debug/debug-application/debug-service/

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch11/deploying.md
git commit -m "docs: write Section 11.7 deploying and verifying"
```

---

### Task 8: Add graceful shutdown and gRPC health checks to catalog service

**Files:**
- Modify: `services/catalog/cmd/main.go`

- [ ] **Step 1: Add gRPC health import and server**

In `services/catalog/cmd/main.go`, add the health imports:

```go
"google.golang.org/grpc/health"
healthpb "google.golang.org/grpc/health/healthpb"
```

After `reflection.Register(grpcServer)` (line 129), add:

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

- [ ] **Step 2: Add graceful shutdown goroutine**

The catalog service already has `signal.NotifyContext` at line 99 with `ctx` and `cancel`. Add a shutdown goroutine before the `Serve` call. Replace the final section (from `slog.Info("catalog service listening"...` to end of `main`) with:

```go
go func() {
    <-ctx.Done()
    slog.Info("shutting down catalog service")
    healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
    grpcServer.GracefulStop()
}()

slog.Info("catalog service listening", "port", grpcPort)
if err := grpcServer.Serve(lis); err != nil {
    slog.Error("failed to serve", "error", err)
    os.Exit(1)
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./services/catalog/cmd/`
Expected: compiles without errors.

- [ ] **Step 4: Verify existing tests still pass**

Run: `go test ./services/catalog/...`
Expected: all existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/catalog/cmd/main.go
git commit -m "feat: add graceful shutdown and gRPC health checks to catalog"
```

---

### Task 9: Add graceful shutdown and gRPC health checks to auth service

**Files:**
- Modify: `services/auth/cmd/main.go`

- [ ] **Step 1: Add signal handling, health imports**

The auth service currently has no signal handling and uses `log` not `slog`. Add these imports:

```go
"context"
"os/signal"
"syscall"
"google.golang.org/grpc/health"
healthpb "google.golang.org/grpc/health/healthpb"
```

- [ ] **Step 2: Add signal context**

After the dependency wiring section (after line 64, `authHandler := handler.NewAuthHandlerWithOAuth(...)`), add:

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
```

- [ ] **Step 3: Register health server**

After `reflection.Register(grpcServer)` (line 83), add:

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

- [ ] **Step 4: Add graceful shutdown goroutine**

Replace the final serve block with:

```go
go func() {
    <-ctx.Done()
    log.Println("shutting down auth service")
    healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
    grpcServer.GracefulStop()
}()

log.Printf("auth service listening on :%s", grpcPort)
if err := grpcServer.Serve(lis); err != nil {
    log.Fatalf("failed to serve: %v", err)
}
```

- [ ] **Step 5: Verify compilation and tests**

Run: `go build ./services/auth/cmd/ && go test ./services/auth/...`
Expected: compiles and all tests pass.

- [ ] **Step 6: Commit**

```bash
git add services/auth/cmd/main.go
git commit -m "feat: add graceful shutdown and gRPC health checks to auth"
```

---

### Task 10: Add graceful shutdown and gRPC health checks to reservation service

**Files:**
- Modify: `services/reservation/cmd/main.go`

- [ ] **Step 1: Add health imports and signal handling**

The reservation service currently has no signal handling. Add imports:

```go
"os/signal"
"syscall"
"google.golang.org/grpc/health"
healthpb "google.golang.org/grpc/health/healthpb"
```

Add signal context after the dependency wiring (after line 110):

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
```

- [ ] **Step 2: Register health server**

After `reflection.Register(grpcServer)` (line 125), add:

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

- [ ] **Step 3: Add graceful shutdown goroutine**

Replace the final serve block with:

```go
go func() {
    <-ctx.Done()
    slog.Info("shutting down reservation service")
    healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
    grpcServer.GracefulStop()
}()

slog.Info("reservation service listening", "port", grpcPort)
if err := grpcServer.Serve(lis); err != nil {
    slog.Error("failed to serve", "error", err)
    os.Exit(1)
}
```

- [ ] **Step 4: Verify compilation and tests**

Run: `go build ./services/reservation/cmd/ && go test ./services/reservation/...`
Expected: compiles and all tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/reservation/cmd/main.go
git commit -m "feat: add graceful shutdown and gRPC health checks to reservation"
```

---

### Task 11: Add graceful shutdown and gRPC health checks to search service

**Files:**
- Modify: `services/search/cmd/main.go`

- [ ] **Step 1: Add health imports**

The search service already has `signal.NotifyContext` (line 53). Add health imports:

```go
"google.golang.org/grpc/health"
healthpb "google.golang.org/grpc/health/healthpb"
```

- [ ] **Step 2: Register health server**

After `reflection.Register(grpcServer)` (line 81), add:

```go
healthServer := health.NewServer()
healthpb.RegisterHealthServer(grpcServer, healthServer)
healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

- [ ] **Step 3: Add graceful shutdown goroutine**

Replace the final serve block with:

```go
go func() {
    <-ctx.Done()
    log.Println("shutting down search service")
    healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
    grpcServer.GracefulStop()
}()

log.Printf("search service listening on :%s", grpcPort)
if err := grpcServer.Serve(lis); err != nil {
    log.Fatalf("failed to serve: %v", err)
}
```

- [ ] **Step 4: Verify compilation and tests**

Run: `go build ./services/search/cmd/ && go test ./services/search/...`
Expected: compiles and all tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/search/cmd/main.go
git commit -m "feat: add graceful shutdown and gRPC health checks to search"
```

---

### Task 12: Add graceful shutdown to gateway service

**Files:**
- Modify: `services/gateway/cmd/main.go`

- [ ] **Step 1: Add signal handling imports**

Add to imports:

```go
"context"
"os/signal"
"syscall"
```

- [ ] **Step 2: Add signal context and replace ListenAndServe**

Before the `addr := fmt.Sprintf(...)` line (line 148), add:

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
```

Replace `http.ListenAndServe(addr, h)` with an explicit `http.Server` and graceful shutdown:

```go
server := &http.Server{
    Addr:    addr,
    Handler: h,
}

go func() {
    <-ctx.Done()
    slog.Info("shutting down gateway")
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer shutdownCancel()
    if err := server.Shutdown(shutdownCtx); err != nil {
        slog.Error("gateway shutdown error", "error", err)
    }
}()

slog.Info("gateway listening", "addr", addr)
if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
    slog.Error("server failed", "error", err)
    os.Exit(1)
}
```

Add `"time"` to imports.

- [ ] **Step 3: Verify compilation and tests**

Run: `go build ./services/gateway/cmd/ && go test ./services/gateway/...`
Expected: compiles and all tests pass.

- [ ] **Step 4: Commit**

```bash
git add services/gateway/cmd/main.go
git commit -m "feat: add graceful shutdown to gateway"
```

---

### Task 13: Create Kubernetes manifests — library namespace (application services)

**Files:**
- Create: `deploy/k8s/base/library/namespace.yaml`
- Create: `deploy/k8s/base/library/catalog-deployment.yaml`
- Create: `deploy/k8s/base/library/catalog-service.yaml`
- Create: `deploy/k8s/base/library/catalog-configmap.yaml`
- Create: `deploy/k8s/base/library/auth-deployment.yaml`
- Create: `deploy/k8s/base/library/auth-service.yaml`
- Create: `deploy/k8s/base/library/auth-configmap.yaml`
- Create: `deploy/k8s/base/library/reservation-deployment.yaml`
- Create: `deploy/k8s/base/library/reservation-service.yaml`
- Create: `deploy/k8s/base/library/reservation-configmap.yaml`
- Create: `deploy/k8s/base/library/search-deployment.yaml`
- Create: `deploy/k8s/base/library/search-service.yaml`
- Create: `deploy/k8s/base/library/search-configmap.yaml`
- Create: `deploy/k8s/base/library/gateway-deployment.yaml`
- Create: `deploy/k8s/base/library/gateway-service.yaml`
- Create: `deploy/k8s/base/library/gateway-configmap.yaml`
- Create: `deploy/k8s/base/library/secrets.yaml`
- Create: `deploy/k8s/base/library/ingress.yaml`
- Create: `deploy/k8s/base/library/kustomization.yaml`

- [ ] **Step 1: Create namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: library
```

- [ ] **Step 2: Create catalog manifests**

`catalog-deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: library
  labels:
    app: catalog
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
      containers:
        - name: catalog
          image: library-system/catalog:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 50052
              protocol: TCP
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
          livenessProbe:
            grpc:
              port: 50052
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            grpc:
              port: 50052
            initialDelaySeconds: 2
            periodSeconds: 5
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 250m
              memory: 256Mi
```

`catalog-service.yaml`:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: catalog
  namespace: library
spec:
  selector:
    app: catalog
  ports:
    - port: 50052
      targetPort: 50052
      protocol: TCP
```

`catalog-configmap.yaml`:
```yaml
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

- [ ] **Step 3: Create auth manifests**

Same pattern as catalog. **Important:** In all Deployment manifests that use `$(POSTGRES_PASSWORD)` in `DATABASE_URL`, the `POSTGRES_PASSWORD` env entry must be listed *before* `DATABASE_URL` — Kubernetes resolves `$(VAR_NAME)` references in definition order. Key differences:
- Image: `library-system/auth:latest`
- Port: 50051
- ConfigMap: `GRPC_PORT: "50051"`, no `KAFKA_BROKERS` (auth doesn't use Kafka), `JWT_EXPIRY: "24h"`
- Additional env vars: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL` (empty defaults for local, OAuth2 won't work in kind without real credentials)
- DATABASE_URL: points to `postgres-auth-0.postgres-auth.data.svc.cluster.local`

- [ ] **Step 4: Create reservation manifests**

Key differences:
- Image: `library-system/reservation:latest`
- Port: 50053
- ConfigMap: `GRPC_PORT: "50053"`, `KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"`, `CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"`, `MAX_ACTIVE_RESERVATIONS: "5"`, `OTEL_COLLECTOR_ENDPOINT: ""`
- DATABASE_URL: points to `postgres-reservation-0.postgres-reservation.data.svc.cluster.local`

- [ ] **Step 5: Create search manifests**

Key differences:
- Image: `library-system/search:latest`
- Port: 50054
- ConfigMap: `GRPC_PORT: "50054"`, `KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092"`, `MEILI_URL: "http://meilisearch.data.svc.cluster.local:7700"`, `CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"`
- env: `MEILI_MASTER_KEY` from `meilisearch-secret`
- No database — no DATABASE_URL or postgres secret

- [ ] **Step 6: Create gateway manifests**

Key differences:
- Image: `library-system/gateway:latest`
- Port: 8080
- HTTP liveness/readiness probes on `/healthz` instead of gRPC
- ConfigMap: `PORT: "8080"`, `AUTH_GRPC_ADDR: "auth.library.svc.cluster.local:50051"`, `CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"`, `RESERVATION_GRPC_ADDR: "reservation.library.svc.cluster.local:50053"`, `SEARCH_GRPC_ADDR: "search.library.svc.cluster.local:50054"`, `OTEL_COLLECTOR_ENDPOINT: ""`
- Needs `JWT_SECRET` from secret

- [ ] **Step 7: Create secrets.yaml (placeholders)**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: jwt-secret
  namespace: library
type: Opaque
data:
  JWT_SECRET: ""  # Populated by overlay secretGenerator
```

Note: include all secret placeholders (jwt-secret, postgres-catalog-secret, postgres-auth-secret, postgres-reservation-secret, meilisearch-secret). The local overlay's `secretGenerator` will replace these.

- [ ] **Step 8: Create ingress.yaml**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
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

- [ ] **Step 9: Create kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: library
resources:
  - namespace.yaml
  - catalog-deployment.yaml
  - catalog-service.yaml
  - catalog-configmap.yaml
  - auth-deployment.yaml
  - auth-service.yaml
  - auth-configmap.yaml
  - reservation-deployment.yaml
  - reservation-service.yaml
  - reservation-configmap.yaml
  - search-deployment.yaml
  - search-service.yaml
  - search-configmap.yaml
  - gateway-deployment.yaml
  - gateway-service.yaml
  - gateway-configmap.yaml
  - secrets.yaml
  - ingress.yaml
```

- [ ] **Step 10: Validate manifests**

Run: `kubectl kustomize deploy/k8s/base/library/`
Expected: renders all resources without errors. If kubectl is not available, use `ls deploy/k8s/base/library/` to verify all files exist.

- [ ] **Step 11: Commit**

```bash
git add deploy/k8s/base/library/
git commit -m "feat: add Kubernetes manifests for application services"
```

---

### Task 14: Create Kubernetes manifests — data namespace (Postgres + Meilisearch)

**Files:**
- Create: `deploy/k8s/base/data/namespace.yaml`
- Create: `deploy/k8s/base/data/postgres-catalog-statefulset.yaml`
- Create: `deploy/k8s/base/data/postgres-catalog-service.yaml`
- Create: `deploy/k8s/base/data/postgres-catalog-configmap.yaml`
- Create: `deploy/k8s/base/data/postgres-auth-statefulset.yaml`
- Create: `deploy/k8s/base/data/postgres-auth-service.yaml`
- Create: `deploy/k8s/base/data/postgres-auth-configmap.yaml`
- Create: `deploy/k8s/base/data/postgres-reservation-statefulset.yaml`
- Create: `deploy/k8s/base/data/postgres-reservation-service.yaml`
- Create: `deploy/k8s/base/data/postgres-reservation-configmap.yaml`
- Create: `deploy/k8s/base/data/meilisearch-statefulset.yaml`
- Create: `deploy/k8s/base/data/meilisearch-service.yaml`
- Create: `deploy/k8s/base/data/meilisearch-configmap.yaml`
- Create: `deploy/k8s/base/data/secrets.yaml`
- Create: `deploy/k8s/base/data/kustomization.yaml`

- [ ] **Step 1: Create namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: data
```

- [ ] **Step 2: Create postgres-catalog StatefulSet**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-catalog
  namespace: data
spec:
  serviceName: postgres-catalog
  replicas: 1
  selector:
    matchLabels:
      app: postgres-catalog
  template:
    metadata:
      labels:
        app: postgres-catalog
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports:
            - containerPort: 5432
          envFrom:
            - configMapRef:
                name: postgres-catalog-config
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-catalog-secret
                  key: POSTGRES_PASSWORD
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "postgres"]
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

Headless service:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres-catalog
  namespace: data
spec:
  clusterIP: None
  selector:
    app: postgres-catalog
  ports:
    - port: 5432
      targetPort: 5432
```

ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-catalog-config
  namespace: data
data:
  POSTGRES_DB: catalog
  POSTGRES_USER: postgres
```

- [ ] **Step 3: Create postgres-auth and postgres-reservation StatefulSets**

Same pattern, different names and database names:
- `postgres-auth`: `POSTGRES_DB: auth`
- `postgres-reservation`: `POSTGRES_DB: reservation`

- [ ] **Step 4: Create Meilisearch StatefulSet**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: meilisearch
  namespace: data
spec:
  serviceName: meilisearch
  replicas: 1
  selector:
    matchLabels:
      app: meilisearch
  template:
    metadata:
      labels:
        app: meilisearch
    spec:
      containers:
        - name: meilisearch
          image: getmeili/meilisearch:v1.12
          ports:
            - containerPort: 7700
          envFrom:
            - configMapRef:
                name: meilisearch-config
          env:
            - name: MEILI_MASTER_KEY
              valueFrom:
                secretKeyRef:
                  name: meilisearch-secret
                  key: MEILI_MASTER_KEY
          readinessProbe:
            httpGet:
              path: /health
              port: 7700
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: data
              mountPath: /meili_data
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

Meilisearch Service (headless):
```yaml
apiVersion: v1
kind: Service
metadata:
  name: meilisearch
  namespace: data
spec:
  clusterIP: None
  selector:
    app: meilisearch
  ports:
    - port: 7700
      targetPort: 7700
```

Meilisearch ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: meilisearch-config
  namespace: data
data:
  MEILI_ENV: development
  MEILI_NO_ANALYTICS: "true"
```

- [ ] **Step 5: Create secrets.yaml (placeholders)**

Secret placeholders for postgres-catalog-secret, postgres-auth-secret, postgres-reservation-secret, meilisearch-secret. All with empty data — populated by overlay `secretGenerator`.

- [ ] **Step 6: Create kustomization.yaml**

List all resources in the data namespace.

- [ ] **Step 7: Validate and commit**

Run: `kubectl kustomize deploy/k8s/base/data/` (or `ls` to verify files).

```bash
git add deploy/k8s/base/data/
git commit -m "feat: add Kubernetes manifests for data namespace (Postgres + Meilisearch)"
```

---

### Task 15: Create Kubernetes manifests — messaging namespace (Kafka)

**Files:**
- Create: `deploy/k8s/base/messaging/namespace.yaml`
- Create: `deploy/k8s/base/messaging/kafka-statefulset.yaml`
- Create: `deploy/k8s/base/messaging/kafka-service.yaml`
- Create: `deploy/k8s/base/messaging/kafka-configmap.yaml`
- Create: `deploy/k8s/base/messaging/kustomization.yaml`

- [ ] **Step 1: Create namespace.yaml**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: messaging
```

- [ ] **Step 2: Create Kafka StatefulSet**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kafka
  namespace: messaging
spec:
  serviceName: kafka
  replicas: 1
  selector:
    matchLabels:
      app: kafka
  template:
    metadata:
      labels:
        app: kafka
    spec:
      containers:
        - name: kafka
          image: apache/kafka:3.9
          ports:
            - containerPort: 9092
              name: broker
            - containerPort: 9093
              name: controller
          envFrom:
            - configMapRef:
                name: kafka-config
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
            initialDelaySeconds: 30
            periodSeconds: 10
            timeoutSeconds: 5
          volumeMounts:
            - name: data
              mountPath: /var/lib/kafka/data
          resources:
            requests:
              cpu: 200m
              memory: 512Mi
            limits:
              cpu: 500m
              memory: 1Gi
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 2Gi
```

- [ ] **Step 3: Create Kafka headless Service**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kafka
  namespace: messaging
spec:
  clusterIP: None
  selector:
    app: kafka
  ports:
    - port: 9092
      targetPort: 9092
      name: broker
    - port: 9093
      targetPort: 9093
      name: controller
```

- [ ] **Step 4: Create Kafka ConfigMap**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kafka-config
  namespace: messaging
data:
  KAFKA_NODE_ID: "1"
  KAFKA_PROCESS_ROLES: "broker,controller"
  KAFKA_LISTENERS: "PLAINTEXT://:9092,CONTROLLER://:9093"
  KAFKA_ADVERTISED_LISTENERS: "PLAINTEXT://kafka-0.kafka.messaging.svc.cluster.local:9092"
  KAFKA_CONTROLLER_QUORUM_VOTERS: "1@kafka-0.kafka.messaging.svc.cluster.local:9093"
  KAFKA_CONTROLLER_LISTENER_NAMES: "CONTROLLER"
  KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
  KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
```

- [ ] **Step 5: Create kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: messaging
resources:
  - namespace.yaml
  - kafka-statefulset.yaml
  - kafka-service.yaml
  - kafka-configmap.yaml
```

- [ ] **Step 6: Validate and commit**

Run: `kubectl kustomize deploy/k8s/base/messaging/` (or `ls` to verify files).

```bash
git add deploy/k8s/base/messaging/
git commit -m "feat: add Kubernetes manifests for messaging namespace (Kafka)"
```

---

### Task 16: Create Kustomize base and overlays

**Files:**
- Create: `deploy/k8s/base/kustomization.yaml`
- Create: `deploy/k8s/overlays/local/kustomization.yaml`
- Create: `deploy/k8s/overlays/production/kustomization.yaml`

- [ ] **Step 1: Create root base kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - data
  - messaging
  - library
```

Note: `data` and `messaging` listed before `library` so infrastructure namespaces are created first (though Kustomize doesn't guarantee ordering, the convention helps readability).

- [ ] **Step 2: Create local overlay kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

# Generate secrets with actual values for local development
secretGenerator:
  - name: jwt-secret
    namespace: library
    literals:
      - JWT_SECRET=dev-secret-change-in-production
  - name: postgres-catalog-secret
    namespace: data
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: postgres-auth-secret
    namespace: data
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: postgres-reservation-secret
    namespace: data
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: meilisearch-secret
    namespace: data
    literals:
      - MEILI_MASTER_KEY=dev-master-key-change-in-production

generatorOptions:
  disableNameSuffixHash: true
```

Note: `disableNameSuffixHash: true` keeps secret names predictable so Deployment `secretKeyRef` names match. For production, you'd want the hash suffix for automatic rollouts.

- [ ] **Step 3: Create production overlay stub**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

# Chapter 12 fills this in with:
# - Larger resource limits
# - imagePullPolicy: Always with tagged images from ECR
# - Multiple replicas for application Deployments
# - Real Secrets via external-secrets operator
# - RDS endpoints replacing Postgres StatefulSets (remove from resources, patch ConfigMaps)
# - ALB Ingress Controller annotations
```

- [ ] **Step 4: Validate full Kustomize build**

Run: `kubectl kustomize deploy/k8s/overlays/local/`
Expected: renders all resources from all three namespaces with secrets populated.

- [ ] **Step 5: Commit**

```bash
git add deploy/k8s/base/kustomization.yaml deploy/k8s/overlays/
git commit -m "feat: add Kustomize base and local/production overlays"
```

---

### Task 17: Create kind cluster config

**Files:**
- Create: `deploy/k8s/kind-config.yaml`

- [ ] **Step 1: Create kind config**

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
```

- [ ] **Step 2: Commit**

```bash
git add deploy/k8s/kind-config.yaml
git commit -m "feat: add kind cluster configuration"
```

---

### Task 18: Final verification

- [ ] **Step 1: Verify all services compile**

Run: `go build ./services/catalog/cmd/ && go build ./services/auth/cmd/ && go build ./services/reservation/cmd/ && go build ./services/search/cmd/ && go build ./services/gateway/cmd/`
Expected: all compile without errors.

- [ ] **Step 2: Verify all existing tests pass**

Run: `earthly +test`
Expected: all unit tests pass.

- [ ] **Step 3: Verify lint passes**

Run: `earthly +lint`
Expected: no lint errors.

- [ ] **Step 4: Verify Kustomize renders correctly**

Run: `kubectl kustomize deploy/k8s/overlays/local/` (if kubectl available)
Expected: clean YAML output with all resources.

- [ ] **Step 5: Verify all docs files exist**

Run: `ls docs/src/ch11/`
Expected: `index.md`, `kind-setup.md`, `preparing-services.md`, `app-manifests.md`, `infra-manifests.md`, `kustomize.md`, `deploying.md`
