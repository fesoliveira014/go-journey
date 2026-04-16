# 12.6 Deploying and Verifying

Everything is in place. The code changes that make services cluster-aware are committed. The Kubernetes manifests — Deployments, Services, ConfigMaps, PersistentVolumeClaims — are written. Kustomize knows about the `local` overlay. The kind cluster is running with NGINX installed.

This section walks through the final steps: building images, loading them into the cluster, applying the manifests, and confirming that the system actually works.

---

## Build and Load Images

The first step is getting fresh images into the kind node. As covered in section 12.1, kind nodes run their own `containerd` daemon and cannot see images in the host Docker daemon. You must build and then explicitly load.

**Build all service images with Earthly:**

```bash
earthly +docker
```

This runs the `+docker` target defined in the root `Earthfile`, which builds each service image and tags them under `library-system/`. After it completes, verify they exist locally:

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

Each `kind load` command copies the image tarball from the host Docker daemon into the `containerd` image store inside the `library-control-plane` container. This is a local copy operation — no registry push, no internet access required.

**Confirm the images are visible inside the node:**

```bash
docker exec library-control-plane crictl images | grep library-system
```

`crictl` is the `containerd` CLI bundled inside kind nodes. You should see all five images listed. If one is missing, re-run the corresponding `kind load` command before proceeding. Attempting to deploy with a missing image results in `ImagePullBackOff`, because Kubernetes will attempt a registry pull that will fail.

---

## Deploy

With the images in place, apply the Kustomize overlay:

```bash
kubectl apply -k deploy/k8s/overlays/local
```

The `-k` flag tells `kubectl` to run Kustomize against that directory before applying. Kustomize expands the overlay — merging the base resources with the local patches and the namespace declarations — and streams the result to the Kubernetes API server. You will see output like:

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
ingress.networking.k8s.io/library-ingress created
```

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

The application services have liveness probes that may restart them if startup takes too long while the database or broker is not yet accepting connections. Waiting explicitly here avoids a wave of early `CrashLoopBackOff` restarts that can obscure real errors.

**Watch the application pods come up:**

```bash
kubectl get pods -n library --watch
```

Within 30–60 seconds all pods should transition from `ContainerCreating` to `Running`. Press `Ctrl+C` once they stabilize.

---

## Verification Checklist

Work through these steps in order. Each one builds on the previous.

### 1. All pods running

```bash
kubectl get pods -A
```

Expected state: every pod shows `Running` with restarts at 0 or 1 (one early restart for services that briefly beat the DB readiness probe is acceptable). Pods stuck in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` need attention — see the troubleshooting table below.

### 2. Catalog logs clean

Pick one service and confirm it started without errors:

```bash
kubectl logs -n library deployment/catalog
```

You should see the service log its startup message and indicate that it has connected to PostgreSQL and registered with Kafka. No `ERROR` lines, no stack traces, no repeated connection failures.

### 3. Health check via port-forward and grpcurl

The services expose a gRPC health check endpoint. Test catalog directly, bypassing the Ingress:

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

This verifies that the pod is alive and the gRPC server is responding before you involve the Gateway or Ingress.

### 4. Gateway via Ingress

The Ingress routes `http://library.local` to the Gateway service. For this to work, `library.local` must resolve to `127.0.0.1` — add it to `/etc/hosts` if you have not already:

```
127.0.0.1 library.local
```

Then test the HTTP API:

```bash
curl http://library.local/healthz
```

A 200 response confirms the full path: DNS resolution, NGINX Ingress, Gateway Service, Gateway pod. If you get a 404 or connection refused, check the troubleshooting table.

### 5. End-to-end flow: create a book, verify in search

Create a book through the catalog API:

```bash
curl -s -X POST http://library.local/api/catalog/books \
  -H "Content-Type: application/json" \
  -d '{"title":"The Go Programming Language","author":"Donovan & Kernighan","isbn":"978-0134190440"}' \
  | jq .
```

The response should include the assigned `id`. Then verify it appears in search:

```bash
curl -s "http://library.local/api/search?q=Go+Programming" | jq .
```

The search result should include the book you just created. This exercises: Gateway routing, Catalog service, PostgreSQL persistence, and the Search service (which is populated via a Kafka event emitted when the book was created).

**Note on telemetry:** `OTEL_COLLECTOR_ENDPOINT` is intentionally empty in the `local` overlay — the kind cluster does not run an OpenTelemetry Collector. Traces and metrics are not collected in this environment. The services log a warning at startup but continue normally. Full observability is configured in the Docker Compose stack (Chapter 9); the kind cluster omits it for simplicity.

---

## Troubleshooting Guide

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| `ImagePullBackOff` | Image not loaded into kind | Re-run `kind load docker-image <image> --name library`. Confirm with `docker exec library-control-plane crictl images`. |
| `CrashLoopBackOff` | Bad env var, missing secret, or DB not ready | `kubectl logs <pod> -n library`. Check previous run with `kubectl logs <pod> -n library --previous`. |
| Pod stuck in `Pending` | PVC cannot bind — no matching StorageClass | kind ships with a `standard` StorageClass backed by `rancher.io/local-path`. Run `kubectl get sc` and verify your PVC requests `standard`. |
| DNS resolution failure inside a pod | Service FQDN truncated or wrong namespace | Use the full FQDN: `<service>.<namespace>.svc.cluster.local`. Run `kubectl exec -it <pod> -- nslookup <service>.<namespace>` to test. |
| Ingress returns 404 | NGINX controller not ready, or wrong `ingressClassName` | Check `kubectl get pods -n ingress-nginx`. Confirm the Ingress resource has `ingressClassName: nginx`. Verify the `Host` header in your request matches the Ingress `host` rule. |
| gRPC health check times out | Port-forward not established, or wrong port | Confirm the pod is `Running` first. Check the Service's `targetPort` matches the port the container is actually listening on. |

For deeper investigation, Kubernetes provides two indispensable debugging commands[^1]:

```bash
# Pod-level events and status
kubectl describe pod <pod-name> -n library

# Service routing and endpoint registration
kubectl describe svc <service-name> -n library
```

If a Service has no `Endpoints` listed in `kubectl describe svc`, no pod matched the label selector — this is one of the most common sources of 503 errors through an Ingress[^2].

---

## Cleanup

When you are finished experimenting, delete the cluster entirely:

```bash
kind delete cluster --name library
```

This removes the Docker container, all pods, all persistent data, and the kubeconfig context. Nothing persists. Re-creating the cluster takes about 30 seconds once the node image is cached locally (the first `kind create cluster` pulls ~700 MB; subsequent runs skip the pull).

If you only want to reset the application state without tearing down the cluster, you can delete the namespaces:

```bash
kubectl delete namespace library data messaging
kubectl apply -k deploy/k8s/overlays/local
```

---

## What's Next

The library system is now running locally on Kubernetes. The same manifests — with a different Kustomize overlay — will be deployed to AWS EKS in Chapter 13. EKS introduces real load balancers, persistent volume classes backed by EBS, IAM-based secrets management, and a Route 53 DNS name instead of `/etc/hosts`. The workflow stays the same; the infrastructure layer underneath changes.

---

[^1]: Debugging Pods: https://kubernetes.io/docs/tasks/debug/debug-application/debug-pods/
[^2]: Debugging Services: https://kubernetes.io/docs/tasks/debug/debug-application/debug-service/
