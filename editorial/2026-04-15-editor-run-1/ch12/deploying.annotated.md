# 12.6 Deploying and Verifying

<!-- [STRUCTURAL] Strong, punchy opening. The "everything is in place" setup earns the confident tone — the reader has indeed done the work. -->
<!-- [LINE EDIT] "Everything is in place. The code changes that make services cluster-aware are committed. The Kubernetes manifests — Deployments, Services, ConfigMaps, PersistentVolumeClaims — are written. Kustomize knows about the `local` overlay. The kind cluster is running with NGINX installed." — five short sentences; rhythmic. Keep. -->
<!-- [COPY EDIT] "PersistentVolumeClaims" — consistent capitalization. Good. -->
Everything is in place. The code changes that make services cluster-aware are committed. The Kubernetes manifests — Deployments, Services, ConfigMaps, PersistentVolumeClaims — are written. Kustomize knows about the `local` overlay. The kind cluster is running with NGINX installed.

<!-- [LINE EDIT] "This section walks through the final steps: building images, loading them into the cluster, applying the manifests, and confirming that the system actually works." — serial comma; good. -->
This section walks through the final steps: building images, loading them into the cluster, applying the manifests, and confirming that the system actually works.

---

## Build and Load Images

<!-- [LINE EDIT] "As covered in Section 12.1" — capital S on "Section"; other references in this chapter (and book?) use lowercase. Normalize. CMOS 8.178. -->
The first step is getting fresh images into the kind node. As covered in Section 12.1, kind nodes run their own `containerd` daemon and cannot see images in the host Docker daemon. You must build and then explicitly load.

**Build all service images with Earthly:**

```bash
earthly +docker
```

<!-- [LINE EDIT] "After it completes, verify they exist locally:" — clear step-by-step. -->
This runs the `+docker` target defined in the root `Earthfile`, which builds each service image and tags them under `library-system/`. After it completes, verify they exist locally:

<!-- [COPY EDIT] Using `docker images | grep library-system` — suggest a more portable alternative: `docker images --filter "reference=library-system/*"`. Current form works but relies on grep. Minor preference. -->
```bash
docker images | grep library-system
```

**Load each image into the kind node:**

```bash
kind load docker-image library-system/gateway:latest     --name library
kind load docker-image library-system/auth:latest        --name library
kind load docker-image library-system/catalog:latest     --name library
kind load docker-image library-system/reservation:latest --name library
kind load docker-image library-system/search:latest      --name library
```

<!-- [LINE EDIT] "This is a local copy operation — no registry push, no internet access required." — good reassurance. -->
Each `kind load` command copies the image tarball from the host Docker daemon into the `containerd` image store inside the `library-control-plane` container. This is a local copy operation — no registry push, no internet access required.

**Confirm the images are visible inside the node:**

```bash
docker exec library-control-plane crictl images | grep library-system
```

<!-- [LINE EDIT] "`crictl` is the `containerd` CLI bundled inside kind nodes." — correct and useful. -->
<!-- [COPY EDIT] "crictl" is actually the CRI (Container Runtime Interface) CLI, not a containerd-specific tool. It works with any CRI-compliant runtime. Minor imprecision; consider "`crictl` is the CRI CLI (Container Runtime Interface) bundled inside kind nodes." -->
<!-- [LINE EDIT] "Attempting to deploy with a missing image results in `ImagePullBackOff`, because Kubernetes will attempt a registry pull that will fail." — clean warning. -->
`crictl` is the `containerd` CLI bundled inside kind nodes. You should see all five images listed. If one is missing, re-run the corresponding `kind load` command before proceeding. Attempting to deploy with a missing image results in `ImagePullBackOff`, because Kubernetes will attempt a registry pull that will fail.

---

## Deploy

With the images in place, apply the Kustomize overlay:

```bash
kubectl apply -k deploy/k8s/overlays/local
```

<!-- [LINE EDIT] "The `-k` flag tells `kubectl` to run Kustomize against that directory before applying." — good. -->
<!-- [LINE EDIT] "Kustomize expands the overlay — merging the base resources with the local patches and the namespace declarations — and streams the result to the Kubernetes API server." — accurate. -->
The `-k` flag tells `kubectl` to run Kustomize against that directory before applying. Kustomize expands the overlay — merging the base resources with the local patches and the namespace declarations — and streams the result to the Kubernetes API server. You will see output like:

<!-- [COPY EDIT] The output block shows "secret/oauth-secret created" — but kustomize.md's secretGenerator does NOT create an oauth-secret. The auth ConfigMap (app-manifests.md) only references GOOGLE_CLIENT_ID/GOOGLE_REDIRECT_URL; GOOGLE_CLIENT_SECRET is mentioned as `optional: true`. Either (a) the overlay is supposed to generate oauth-secret for completeness, but kustomize.md doesn't show it, OR (b) this output is wrong. Reconcile with the kustomize.md secretGenerator list. -->
```
namespace/library created
namespace/data created
namespace/messaging created
configmap/auth-config created
configmap/catalog-config created
configmap/gateway-config created
configmap/reservation-config created
configmap/search-config created
secret/jwt-secret created
secret/postgres-auth-secret created
secret/postgres-catalog-secret created
secret/postgres-reservation-secret created
secret/meilisearch-secret created
secret/oauth-secret created
statefulset.apps/postgres-auth created
statefulset.apps/postgres-catalog created
statefulset.apps/postgres-reservation created
service/postgres-auth created
service/postgres-catalog created
service/postgres-reservation created
statefulset.apps/kafka created
service/kafka created
statefulset.apps/meilisearch created
service/meilisearch created
deployment.apps/auth created
service/auth created
deployment.apps/catalog created
service/catalog created
deployment.apps/reservation created
service/reservation created
deployment.apps/search created
service/search created
deployment.apps/gateway created
service/gateway created
ingress.networking.k8s.io/gateway created
```

<!-- [COPY EDIT] The output shows "ingress.networking.k8s.io/gateway created" — but the ingress was defined with `metadata.name: library-ingress` in app-manifests.md. It should show as `ingress.networking.k8s.io/library-ingress created`. Fix. -->
<!-- [COPY EDIT] Output shows "secret/meilisearch-secret created" once — but kustomize.md's secretGenerator creates it in both `library` and `data` namespaces. Since kubectl output typically shows namespace-scoped resources as `secret/<name>` with a separate `-n` column (elided here), the output could be ambiguous. Minor. -->
<!-- [COPY EDIT] Output lists "secret/postgres-catalog-secret created" once — again, kustomize.md creates it twice (library + data namespaces). Same observation. -->
<!-- [LINE EDIT] "Kubernetes creates the namespaces first (ordering matters — resources that reference a namespace will fail if it does not exist yet), then the infrastructure workloads, then the application services." — good. -->
<!-- [COPY EDIT] Technically, the user cannot control apply ordering from a flat kustomize output unless the Kubernetes API server handles dependencies. The parenthetical "ordering matters" implies Kustomize/kubectl orders them; actually, kubectl apply streams them roughly in the order they appear, and some namespaces-first behavior comes from kubectl's pre-sort. Consider clarifying. -->
Kubernetes creates the namespaces first (ordering matters — resources that reference a namespace will fail if it does not exist yet), then the infrastructure workloads, then the application services. The apply is declarative: running the same command again is safe and idempotent.

**Wait for infrastructure pods first:**

```bash
kubectl wait --namespace data \
  --for=condition=ready pod \
  --selector=app=postgres-catalog \
  --timeout=120s

kubectl wait --namespace messaging \
  --for=condition=ready pod \
  --selector=app=kafka \
  --timeout=120s
```

<!-- [LINE EDIT] "The application services have startup probes and will restart if the database or broker is not yet accepting connections." — but the manifests in app-manifests.md use livenessProbe and readinessProbe, NOT startupProbe. Startup probes are a distinct Kubernetes feature (since 1.16 stable). The text says "startup probes" but the probes configured are liveness/readiness. Correct to: "The application services have liveness probes that may restart them if startup takes too long while the database or broker is not yet accepting connections." -->
<!-- [LINE EDIT] "Waiting explicitly here avoids a wave of early `CrashLoopBackOff` restarts that can obscure real errors." — good rationale. -->
The application services have startup probes and will restart if the database or broker is not yet accepting connections. Waiting explicitly here avoids a wave of early `CrashLoopBackOff` restarts that can obscure real errors.

**Watch the application pods come up:**

```bash
kubectl get pods -n library --watch
```

<!-- [LINE EDIT] "Within 30–60 seconds all pods should transition from `ContainerCreating` to `Running`." — CMOS 9.58 en dash for ranges. Good. -->
<!-- [COPY EDIT] "Press `Ctrl+C` once they stabilize." — good practical tip. -->
Within 30–60 seconds all pods should transition from `ContainerCreating` to `Running`. Press `Ctrl+C` once they stabilize.

---

## Verification Checklist

Work through these steps in order. Each one builds on the previous.

### 1. All pods running

```bash
kubectl get pods -A
```

<!-- [LINE EDIT] "Expected state: every pod shows `Running` with restarts at 0 or 1 (one early restart for services that briefly beat the DB readiness probe is acceptable)." — good nuance about the "1 restart is ok" case. -->
<!-- [COPY EDIT] CMOS 9.2 "zero or 1" — use numerals when the series contains both a low and higher number: "0 or 1" is actually fine per CMOS 9.7 (numerals for technical contexts). Keep. -->
<!-- [LINE EDIT] "Pods stuck in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` need attention — see the troubleshooting table below." — serial comma; good. -->
Expected state: every pod shows `Running` with restarts at 0 or 1 (one early restart for services that briefly beat the DB readiness probe is acceptable). Pods stuck in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` need attention — see the troubleshooting table below.

### 2. Catalog logs clean

<!-- [LINE EDIT] "Pick one service and confirm it started without errors" — good. -->
Pick one service and confirm it started without errors:

```bash
kubectl logs -n library deployment/catalog
```

<!-- [LINE EDIT] "You should see the service log its startup message and indicate that it has connected to PostgreSQL and registered with Kafka. No `ERROR` lines, no stack traces, no repeated connection failures." — clear expectations. -->
You should see the service log its startup message and indicate that it has connected to PostgreSQL and registered with Kafka. No `ERROR` lines, no stack traces, no repeated connection failures.

### 3. Health check via port-forward and grpcurl

The services expose a gRPC health check endpoint. Test catalog directly, bypassing the Ingress:

<!-- [COPY EDIT] "kubectl port-forward ... &" — running port-forward in the background with `&` works but requires `kill %1` later. Fine for educational purposes but note: if readers run this in a non-interactive shell or CI context, the `&` backgrounding may behave differently. Minor. -->
```bash
kubectl port-forward -n library svc/catalog 50052:50052 &

grpcurl -plaintext localhost:50052 grpc.health.v1.Health/Check
```

Expected response:

```json
{
  "status": "SERVING"
}
```

Kill the port-forward when done:

```bash
kill %1
```

<!-- [LINE EDIT] "This verifies that the pod is alive and the gRPC server is responding before you involve the Gateway or Ingress." — clear causal chain. -->
<!-- [COPY EDIT] "the Gateway or Ingress" — "Gateway" capitalized here and "gateway" lowercase earlier in app-manifests.md. The gateway is the service name (lowercase); "Gateway" in casual reference is OK but inconsistent. Normalize. -->
This verifies that the pod is alive and the gRPC server is responding before you involve the Gateway or Ingress.

### 4. Gateway via Ingress

<!-- [LINE EDIT] "The Ingress routes `http://library.local` to the Gateway service." — good. -->
The Ingress routes `http://library.local` to the Gateway service. For this to work, `library.local` must resolve to `127.0.0.1` — add it to `/etc/hosts` if you have not already:

```
127.0.0.1 library.local
```

Then test the HTTP API:

```bash
curl http://library.local/healthz
```

<!-- [LINE EDIT] "A 200 response confirms the full path: DNS resolution, NGINX Ingress, Gateway Service, Gateway pod." — punchy; good. -->
<!-- [COPY EDIT] "If you get a 404 or connection refused, check the troubleshooting table." — good pointer. -->
A 200 response confirms the full path: DNS resolution, NGINX Ingress, Gateway Service, Gateway pod. If you get a 404 or connection refused, check the troubleshooting table.

### 5. End-to-end flow: create a book, verify in search

Create a book through the catalog API:

```bash
curl -s -X POST http://library.local/api/catalog/books \
  -H "Content-Type: application/json" \
  -d '{"title":"The Go Programming Language","author":"Donovan & Kernighan","isbn":"978-0134190440"}' \
  | jq .
```

<!-- [COPY EDIT] "Donovan & Kernighan" — for a real book title, the authors are Alan A. A. Donovan and Brian W. Kernighan. Using "&" as a shorthand is unusual for a data payload; typically it would be "Donovan and Kernighan" or comma-separated. Minor. Also, ISBN `978-0134190440` is the actual ISBN for "The Go Programming Language" (2015). Please verify. Correct. -->
<!-- [LINE EDIT] "The response should include the assigned `id`. Then verify it appears in search:" — good step-by-step. -->
The response should include the assigned `id`. Then verify it appears in search:

```bash
curl -s "http://library.local/api/search?q=Go+Programming" | jq .
```

<!-- [LINE EDIT] "The search result should include the book you just created. This exercises: Gateway routing, Catalog service, PostgreSQL persistence, and the Search service (which is populated via a Kafka event emitted when the book was created)." — 40+ words; useful summary though. -->
The search result should include the book you just created. This exercises: Gateway routing, Catalog service, PostgreSQL persistence, and the Search service (which is populated via a Kafka event emitted when the book was created).

<!-- [STRUCTURAL] The "Note on telemetry" block is a callout-style paragraph. Good placement at end of checklist. -->
<!-- [LINE EDIT] "Full observability is configured in the Docker Compose stack (Chapter 9); the kind cluster omits it for simplicity." — good. -->
**Note on telemetry:** `OTEL_COLLECTOR_ENDPOINT` is intentionally empty in the `local` overlay — the kind cluster does not run an OpenTelemetry Collector. Traces and metrics are not collected in this environment. The services log a warning at startup but continue normally. Full observability is configured in the Docker Compose stack (Chapter 9); the kind cluster omits it for simplicity.

---

## Troubleshooting Guide

<!-- [STRUCTURAL] Troubleshooting table is well-structured: symptom → likely cause → fix. Keep. -->
| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| `ImagePullBackOff` | Image not loaded into kind | Re-run `kind load docker-image <image> --name library`. Confirm with `docker exec library-control-plane crictl images`. |
| `CrashLoopBackOff` | Bad env var, missing secret, or DB not ready | `kubectl logs <pod> -n library`. Check previous run with `kubectl logs <pod> -n library --previous`. |
<!-- [COPY EDIT] "kind ships with a `standard` StorageClass backed by `rancher.io/local-path`" — verify: kind's default StorageClass is indeed `standard` with the `rancher.io/local-path` provisioner. Correct. -->
| Pod stuck in `Pending` | PVC cannot bind — no matching StorageClass | kind ships with a `standard` StorageClass backed by `rancher.io/local-path`. Run `kubectl get sc` and verify your PVC requests `standard`. |
<!-- [COPY EDIT] But the infra-manifests.md PVC specs do NOT specify a storageClassName, which means they use the cluster default. kind sets `standard` as default. Minor: the "verify your PVC requests `standard`" advice is oddly phrased if the PVC doesn't set the field at all. Clarify. -->
| DNS resolution failure inside a pod | Service FQDN truncated or wrong namespace | Use the full FQDN: `<service>.<namespace>.svc.cluster.local`. Run `kubectl exec -it <pod> -- nslookup <service>.<namespace>` to test. |
| Ingress returns 404 | NGINX controller not ready, or wrong `ingressClassName` | Check `kubectl get pods -n ingress-nginx`. Confirm the Ingress resource has `ingressClassName: nginx`. Verify the `Host` header in your request matches the Ingress `host` rule. |
| gRPC health check times out | Port-forward not established, or wrong port | Confirm the pod is `Running` first. Check the Service's `targetPort` matches the port the container is actually listening on. |

<!-- [LINE EDIT] "Kubernetes provides two indispensable debugging commands" — good. -->
For deeper investigation, Kubernetes provides two indispensable debugging commands[^1]:

```bash
# Pod-level events and status
kubectl describe pod <pod-name> -n library

# Service routing and endpoint registration
kubectl describe svc <service-name> -n library
```

<!-- [LINE EDIT] "If a Service has no `Endpoints` listed in `kubectl describe svc`, no pod matched the label selector — this is one of the most common sources of 503 errors through an Ingress[^2]." — great specific debugging tip. -->
If a Service has no `Endpoints` listed in `kubectl describe svc`, no pod matched the label selector — this is one of the most common sources of 503 errors through an Ingress[^2].

---

## Cleanup

When you are finished experimenting, delete the cluster entirely:

```bash
kind delete cluster --name library
```

<!-- [LINE EDIT] "This removes the Docker container, all pods, all persistent data, and the kubeconfig context. Nothing persists." — clear. -->
<!-- [COPY EDIT] "Re-creating the cluster from scratch with `kind create cluster --config kind-config.yaml --name library` takes the same 30 seconds as the first time (images are cached locally)." — "first time" but earlier in kind-setup.md we said first-run pulls the ~700MB image. Reconcile: the claim here is that re-creation is as fast as the FIRST time, which is only true if the node image is cached. Rephrase: "Re-creating takes about 30 seconds once the node image is cached locally (which happens on the very first `kind create cluster`)." -->
This removes the Docker container, all pods, all persistent data, and the kubeconfig context. Nothing persists. Re-creating the cluster from scratch with `kind create cluster --config kind-config.yaml --name library` takes the same 30 seconds as the first time (images are cached locally).

<!-- [LINE EDIT] "If you only want to reset the application state without tearing down the cluster, you can delete the namespaces:" — good alternative workflow. -->
If you only want to reset the application state without tearing down the cluster, you can delete the namespaces:

```bash
kubectl delete namespace library data messaging
kubectl apply -k deploy/k8s/overlays/local
```

<!-- [COPY EDIT] "kubectl delete namespace library data messaging" — this deletes all three namespaces in one command. Note: deletion is asynchronous; a fast `kubectl apply` afterward may hit "namespace is terminating" errors. Consider adding `--wait=true` note or advising the user to wait. -->

---

## What's Next

<!-- [STRUCTURAL] Forward-looking closer; good book-level continuity. -->
<!-- [LINE EDIT] "EKS introduces real load balancers, persistent volume classes backed by EBS, IAM-based secrets management, and a Route 53 DNS name instead of `/etc/hosts`." — serial comma; good. -->
<!-- [COPY EDIT] "Route 53" — AWS product name; correct capitalization. -->
The library system is now running locally on Kubernetes. The same manifests — with a different Kustomize overlay — will be deployed to AWS EKS in Chapter 13. EKS introduces real load balancers, persistent volume classes backed by EBS, IAM-based secrets management, and a Route 53 DNS name instead of `/etc/hosts`. The workflow stays the same; the infrastructure layer underneath changes.

<!-- [FINAL] "The workflow stays the same; the infrastructure layer underneath changes." — satisfying closer; confirms the narrative arc. -->

---

[^1]: Debugging Pods: https://kubernetes.io/docs/tasks/debug/debug-application/debug-pods/
[^2]: Debugging Services: https://kubernetes.io/docs/tasks/debug/debug-application/debug-service/
