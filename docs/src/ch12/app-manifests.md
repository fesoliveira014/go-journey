# 12.3 Application Manifests

With all five services containerized and the infrastructure layer (PostgreSQL, Kafka, Meilisearch) declared in the previous section, we are ready to write the manifests for the application services themselves. Every service needs three resources: a Deployment that runs the container, a Service that gives it a stable DNS name inside the cluster, and a ConfigMap that injects non-sensitive configuration. Secrets are declared separately as placeholder objects that a local overlay will fill in with real values.

All application resources live in the `library` namespace. Infrastructure resources live in `data` and `messaging`. Keeping namespaces separate has two practical benefits: you can delete all application resources with a single `kubectl delete namespace library` during development without touching your databases, and RBAC policies (covered in Chapter 13) can grant service accounts namespace-scoped permissions rather than cluster-wide ones.

---

## Namespaces

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

The `data` and `messaging` namespace manifests are identical in structure (just with different `name` fields) and were declared in section 12.2.

---

## Catalog Deployment — full walkthrough

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

### `apiVersion` and `kind`

`apiVersion: apps/v1` means this resource is defined in the `apps` API group, stable version 1. The `kind: Deployment` tells the API server which controller should own this object. The Deployment controller is built into `kube-controller-manager` and runs on every standard cluster.

### `metadata`

- `name: catalog` — the name of the Deployment object. This is also what appears in `kubectl get deployments`.
- `namespace: library` — places the object in our application namespace. If omitted, objects land in `default`, which is fine for experiments but undesirable in a real project.
- `labels` — key-value pairs attached to the object. Labels on the Deployment itself are for your own organization (filtering with `kubectl get -l app=catalog`). They are distinct from the labels on the Pod template.

### `spec.replicas`

How many Pod copies the controller should maintain. We use 1 during development. Increasing this to 2+ enables rolling updates and basic availability during node maintenance, but requires that services handle multiple concurrent instances correctly — which ours do, since all state lives in PostgreSQL and Kafka.

### `spec.selector.matchLabels`

This is the link between the Deployment controller and the Pods it manages. The controller watches all Pods whose labels match this selector and reconciles toward the desired replica count. **The selector must match the labels in `spec.template.metadata.labels` exactly.** If they diverge, the controller cannot find its Pods and will continually create new ones.

Once a Deployment is created, `spec.selector` is immutable. To change it you must delete and recreate the Deployment.

### Pod template

Everything under `spec.template` describes the Pod that the Deployment creates. It is a Pod spec, not a Deployment-specific construct — you could copy it into a standalone `kind: Pod` manifest and it would be valid.

#### `image` and `imagePullPolicy`

`image: library-system/catalog:latest` references the image we built and loaded into kind in section 12.1. The `latest` tag is normally discouraged in production because it makes rollbacks ambiguous, but it is fine for a local development cluster where we control exactly what is in the cache.

`imagePullPolicy: IfNotPresent` is critical for kind. By default, Kubernetes tries to pull images from a registry. kind loads images directly into its internal containerd cache via `kind load docker-image`. If the pull policy is `Always`, the kubelet will still attempt a registry pull, which fails because `library-system/catalog:latest` does not exist in any public registry. `IfNotPresent` tells the kubelet: if the image is already present in the local cache, use it. This is the correct policy for locally built images in a kind cluster.

#### `ports`

`containerPort` is documentation — it does not actually open a port or affect networking. Kubernetes networking makes all container ports reachable regardless of whether they are declared here. The convention exists so that tooling (`kubectl describe`, service mesh proxies, monitoring agents) can discover which ports a container uses.

#### `envFrom` — ConfigMap injection

```yaml
envFrom:
  - configMapRef:
      name: catalog-config
```

`envFrom` injects every key from the named ConfigMap as an environment variable. This is how non-sensitive configuration (ports, broker addresses, service endpoints) reaches the container. We cover the ConfigMap itself later in this section.

#### `env` — individual variables and Secrets

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

There is an important ordering rule here. Kubernetes resolves `$(VAR_NAME)` references in the `env` list at definition time, in the order the variables appear. `DATABASE_URL` uses `$(POSTGRES_PASSWORD)` in its value, so `POSTGRES_PASSWORD` must be defined first. If you place `DATABASE_URL` before `POSTGRES_PASSWORD`, the substitution produces a literal `$(POSTGRES_PASSWORD)` string and the connection will fail.

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

The pod-level `securityContext` sets defaults for all containers in the pod. `runAsNonRoot: true` prevents the container from running as UID 0. `runAsUser: 65534` runs as the `nobody` user — our Go binaries are statically linked and need no special user.

The container-level `securityContext` tightens permissions further. `allowPrivilegeEscalation: false` prevents a process from gaining more privileges than its parent (blocks `setuid` binaries and `ptrace` exploits). `readOnlyRootFilesystem: true` makes the container's root filesystem immutable — any attempt to write to disk fails. This eliminates an entire class of attacks where a compromised process writes a malicious binary and executes it. Our Go services write nothing to disk; all state goes to PostgreSQL. `capabilities: drop: ["ALL"]` removes all Linux capabilities (the fine-grained root powers like `NET_RAW`, `SYS_ADMIN`, etc.). A Go gRPC server needs none of them.

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

`requests` are what the scheduler uses to decide which node can host the Pod. A node with 200m of unallocated CPU can host a Pod requesting 100m. `limits` are enforced at runtime by the Linux kernel's cgroup subsystem: if the container exceeds its memory limit, the kernel OOM-kills it; if it exceeds its CPU limit, it is throttled. Always set both — a Pod without requests cannot be scheduled intelligently, and a Pod without limits can starve its neighbors.

The values here are intentionally small. The catalog service at rest uses almost no CPU and well under 50 MiB of RAM. Give your local cluster room to run all five services simultaneously.

#### Liveness and readiness probes

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

Kubernetes supports `grpc` probes natively as of v1.24. The kubelet sends a gRPC `Check` request to the standard `grpc.health.v1.Health` service. Your gRPC servers must register this handler — if they do not, the probe will fail and the Pod will be restarted or never receive traffic.

The difference between the two probe types:

- **Liveness** answers "is this container alive?" If it fails `failureThreshold` times consecutively, the kubelet restarts the container. Use it to catch deadlocks or fatal errors that do not crash the process.
- **Readiness** answers "should this container receive traffic?" If it fails, the Pod is removed from the Service's endpoint list. Use it to delay traffic until the service has finished startup (database migrations, cache warming).

`initialDelaySeconds` gives the container time to start before the first probe fires. Set it to slightly less than your service's typical cold-start time.

#### `terminationGracePeriodSeconds`

When a Pod is deleted, Kubernetes sends `SIGTERM` to the main container process and waits up to `terminationGracePeriodSeconds` (30 seconds here) for it to exit cleanly before sending `SIGKILL`. Our services should catch `SIGTERM`, stop accepting new requests, drain in-flight work, and exit. gRPC's `GracefulStop` handles this correctly. 30 seconds is generous — most services drain in under 5 seconds.

---

## Catalog Service

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

A Service[^2] gives a set of Pods a stable virtual IP address and DNS name. The `selector` field matches Pods by label — in this case any Pod in the `library` namespace with `app: catalog`. When a Pod is added or removed, the Endpoints controller updates the Service's endpoint list automatically.

The DNS name for this Service is `catalog.library.svc.cluster.local`. Within the same namespace, `catalog` alone resolves correctly. From other namespaces you need the full name: `catalog.library.svc.cluster.local`.

The default Service type is `ClusterIP`, which means the virtual IP is only reachable from inside the cluster. That is exactly what we want — gRPC services should not be directly exposed externally. Only the gateway Service is accessible from outside, via the Ingress.

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

ConfigMaps[^4] store non-sensitive key-value data. All values must be strings — note the quotes around `"50052"`. The keys map directly to environment variable names when loaded via `envFrom.configMapRef`.

`KAFKA_BROKERS` uses the cross-namespace DNS name for the Kafka StatefulSet Pod. StatefulSet Pods get stable DNS names in the form `<pod-name>.<service-name>.<namespace>.svc.cluster.local`. The `kafka-0` pod is in the `messaging` namespace, so its address is `kafka-0.kafka.messaging.svc.cluster.local:9092`. A regular Service DNS name (`kafka.messaging.svc.cluster.local`) would also work but would route through the cluster's load balancer rather than directly to the pod — for Kafka, connecting directly to broker pods by their stable identity is the standard approach.

`OTEL_COLLECTOR_ENDPOINT` is empty. The OTel collector is not deployed in the kind cluster (it is part of the observability stack in Docker Compose). Leaving this empty causes the services to skip exporting traces. The Docker Compose stack (Chapter 9) includes a full collector; the kind cluster omits it for simplicity.

---

## Remaining services

The auth, reservation, search, and gateway services follow the same three-resource pattern. Commentary is minimal — refer back to the catalog walkthrough for field explanations.

### Auth service

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

`GOOGLE_CLIENT_ID` and `GOOGLE_REDIRECT_URL` are non-sensitive configuration — the client ID is public, and the redirect URL is a route, not a credential. Both are left empty in the base ConfigMap. `GOOGLE_CLIENT_SECRET`, on the other hand, is a credential and belongs in a Secret object. The auth Deployment references it via `secretKeyRef` with `optional: true`, so the pod starts normally when the Secret does not exist (as in the local overlay where OAuth2 is not used). For production, the External Secrets Operator injects the real value from AWS Secrets Manager.

### Reservation service

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

`CATALOG_GRPC_ADDR` uses the Service DNS name (`catalog.library.svc.cluster.local`) rather than a StatefulSet pod name. Application Services (as opposed to StatefulSets) are load-balanced by default, so using the Service name is correct.

### Search service

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

The Deployment manifests reference several Secrets by name (`jwt-secret`, `postgres-catalog-secret`, `postgres-auth-secret`, `postgres-reservation-secret`, `meilisearch-secret`). Notice that the base directory does **not** include a `secrets.yaml` file. This is intentional — secret values should never exist in the base, even as empty placeholders. Instead, each overlay is responsible for creating the Secrets that its environment needs.

The local overlay (section 12.5) uses Kustomize's `secretGenerator` to create all required Secrets with development values. The production overlay uses the External Secrets Operator to sync values from AWS Secrets Manager (Chapter 14). This pattern keeps credentials out of version control entirely.

The key names populated by the overlay (`JWT_SECRET`, `POSTGRES_PASSWORD`, `MEILI_MASTER_KEY`) match the `secretKeyRef.key` values in the Deployment manifests exactly. The OAuth2 client secret (`GOOGLE_CLIENT_SECRET`) is referenced by the auth Deployment with `optional: true`, so it only needs to be provided in environments where Google OAuth2 is configured.

**Base64 is not encryption.** Anyone with read access to a Secret object — via `kubectl get secret jwt-secret -o yaml` — can decode the value with `base64 -d`. Secrets are only marginally better than ConfigMaps at rest (in etcd) unless you enable etcd encryption, and they are only as secure as your RBAC policy. The canonical solution for production is an external secret store (HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager) synced to Kubernetes Secrets by an operator. We cover this in Chapter 14. For the local cluster, the local overlay's `secretGenerator` provides concrete values without putting them in version control.

---

## Ingress

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

`ingressClassName: nginx` selects the NGINX controller installed in the cluster. Without this field, if multiple Ingress controllers are installed, the behavior is undefined.

All HTTP traffic to `library.local` routes to the gateway Service on port 8080. The gateway handles all routing internally — it owns the URL tree and proxies to the appropriate gRPC backend.

To reach `library.local` from your development machine, add the following line to `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows):

```
127.0.0.1 library.local
```

The kind cluster exposes port 80 on `localhost` via the `extraPortMappings` configured in section 12.1. With the hosts entry in place, `http://library.local` resolves to `127.0.0.1:80`, which kind forwards to the NGINX controller, which routes to the gateway Pod.

If you later find that NGINX terminates long-running requests prematurely, you can add timeout annotations like `nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"`. For our use case — short HTTP requests proxied to gRPC backends — the defaults are sufficient.

---

## Kustomization

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

The order of `resources` matters for readability but not for correctness — `kubectl apply` handles resource creation order internally, retrying dependencies that are not yet ready.

---

## Summary

You now have complete Kubernetes manifests for all five application services. The pattern is consistent: a Deployment that runs the container with probes, resource bounds, and environment injection; a ClusterIP Service for stable in-cluster DNS; and a ConfigMap for non-sensitive configuration. Secrets are declared as placeholders — the local overlay in section 12.5 substitutes real values via `secretGenerator`.

Three things to carry forward:

1. `imagePullPolicy: IfNotPresent` is mandatory for images loaded into kind locally. Without it, every pod start fails with `ErrImagePull`.
2. In the `env` list, variables that reference other variables via `$(VAR_NAME)` must be declared after the variables they reference. The ordering is sequential, not lexicographic.
3. Base64 encoding is not protection. Treat Secret manifests as sensitive files — never commit real values to source control.

Section 12.4 assembles the top-level `kustomization.yaml` that ties all three namespaces together, and section 12.5 adds the local overlay with image name patches and `secretGenerator` entries.

---

[^1]: Deployments: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
[^2]: Services: https://kubernetes.io/docs/concepts/services-networking/service/
[^3]: Ingress: https://kubernetes.io/docs/concepts/services-networking/ingress/
[^4]: ConfigMaps: https://kubernetes.io/docs/concepts/configuration/configmap/
[^5]: Secrets: https://kubernetes.io/docs/concepts/configuration/secret/
