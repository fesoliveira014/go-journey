# 12.7 Production Kustomize Overlay

Chapter 11 introduced Kustomize and structured the manifests into a base and two overlays. The local overlay configured kind-specific settings: a `secretGenerator` with plaintext development credentials, a single replica per service, and `imagePullPolicy: IfNotPresent` so kind's locally loaded images are used without hitting a registry. That overlay is still correct for local development and remains untouched.

Now you write the production overlay. At this point you have an EKS cluster, three RDS instances, an MSK cluster, ECR repositories, and a working CI pipeline. The production overlay is the piece that wires those AWS resources into the Kubernetes manifests — without modifying a single file under `deploy/k8s/base/`.

---

## Restructuring the base for production

There is one structural change to make before writing the overlay: Postgres and Kafka StatefulSets have no place in a production cluster. Production uses RDS and MSK. Including the in-cluster StatefulSets in the base and then having the production overlay try to delete or ignore them creates unnecessary complexity and risk.

The clean solution is to move the local infrastructure manifests out of the base and into a `local-infra/` component that only the local overlay includes:

**Before:**

```
deploy/k8s/
├── base/
│   ├── kustomization.yaml       # includes data/ and messaging/
│   ├── data/                    # Postgres + Meilisearch StatefulSets
│   ├── messaging/               # Kafka StatefulSet
│   └── library/                 # application Deployments, Services, Ingress
└── overlays/
    ├── local/
    │   └── kustomization.yaml
    └── production/
        └── kustomization.yaml
```

**After:**

```
deploy/k8s/
├── base/
│   ├── kustomization.yaml       # includes data/, messaging/, library/
│   ├── data/                    # Meilisearch only (Postgres moved out)
│   ├── messaging/               # namespace only (Kafka moved out)
│   ├── local-infra/             # local-only infrastructure
│   │   ├── kustomization.yaml
│   │   ├── data/                # Postgres StatefulSets (moved from base/data/)
│   │   └── messaging/           # Kafka StatefulSet (moved from base/messaging/)
│   └── library/                 # unchanged
└── overlays/
    ├── local/
    │   └── kustomization.yaml   # now also includes ../../base/local-infra
    └── production/
        └── kustomization.yaml   # references only ../../base (no local-infra)
```

The base `kustomization.yaml` stays unchanged — it still references `data/`, `messaging/`, and `library/`. What changed is the *contents* of those directories: `data/` now contains only Meilisearch resources, and `messaging/` contains only the namespace manifest. Postgres and Kafka moved to `local-infra/`.

Update `deploy/k8s/base/data/kustomization.yaml` to remove the postgres entries, keeping only Meilisearch:

```yaml
# deploy/k8s/base/data/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - namespace.yaml
  - meilisearch-configmap.yaml
  - meilisearch-statefulset.yaml
  - meilisearch-service.yaml
```

Update `deploy/k8s/base/messaging/kustomization.yaml` to remove Kafka entries:

```yaml
# deploy/k8s/base/messaging/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: messaging

resources:
  - namespace.yaml
```

Add a `kustomization.yaml` to the new `local-infra/` directory:

```yaml
# deploy/k8s/base/local-infra/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - data
  - messaging
```

Update the local overlay to include both the base and the local infrastructure:

```yaml
# deploy/k8s/overlays/local/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base
  - ../../base/local-infra

secretGenerator:
  # ... same as before, unchanged ...
```

The production overlay references only `../../base`. It never sees a StatefulSet for Postgres or Kafka. Those resources simply do not exist in the production render.

---

## The production kustomization.yaml

Here is the complete production overlay. Each section is explained in detail below:

```yaml
# deploy/k8s/overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

# ---------------------------------------------------------------------------
# Image references: rewrite local names to ECR URIs.
# Kustomize matches on the image name field in every container spec across all
# Deployments in the base. All containers using library-system/catalog will
# get the ECR URI assigned here, without touching any base manifest.
# ---------------------------------------------------------------------------
images:
  - name: library-system/gateway
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/gateway
    newTag: latest
  - name: library-system/auth
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/auth
    newTag: latest
  - name: library-system/catalog
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/catalog
    newTag: latest
  - name: library-system/reservation
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/reservation
    newTag: latest
  - name: library-system/search
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/search
    newTag: latest

generatorOptions:
  disableNameSuffixHash: true

# ---------------------------------------------------------------------------
# Secret placeholders: Chapter 13 replaces these with External Secrets
# resources populated from AWS Secrets Manager. For now these stand in so the
# overlay renders without errors. Do not commit real values.
# ---------------------------------------------------------------------------
secretGenerator:
  - name: jwt-secret
    namespace: library
    literals:
      - JWT_SECRET=REPLACE_WITH_EXTERNAL_SECRET
  - name: postgres-catalog-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_EXTERNAL_SECRET
  - name: postgres-auth-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_EXTERNAL_SECRET
  - name: postgres-reservation-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_EXTERNAL_SECRET
  - name: meilisearch-secret
    namespace: library
    literals:
      - MEILI_MASTER_KEY=REPLACE_WITH_EXTERNAL_SECRET

patches:
  # -------------------------------------------------------------------------
  # Replica counts: run two replicas of every stateless service.
  # A single JSON patch targets all Deployments in the library namespace.
  # -------------------------------------------------------------------------
  - patch: |-
      - op: replace
        path: /spec/replicas
        value: 2
    target:
      kind: Deployment
      namespace: library

  # -------------------------------------------------------------------------
  # Resource limits: increase CPU and memory for production workloads.
  # -------------------------------------------------------------------------
  - path: patches/resources.yaml
    target:
      kind: Deployment
      namespace: library

  # -------------------------------------------------------------------------
  # imagePullPolicy: Always — required when pulling from ECR. kind can serve
  # IfNotPresent from its local image cache; ECR cannot.
  # -------------------------------------------------------------------------
  - patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/imagePullPolicy
        value: Always
    target:
      kind: Deployment
      namespace: library

  # -------------------------------------------------------------------------
  # DATABASE_URL patches: replace in-cluster Postgres hostnames with RDS
  # endpoints. These are strategic merge patches on individual Deployments.
  # -------------------------------------------------------------------------
  - path: patches/database-url-auth.yaml
  - path: patches/database-url-catalog.yaml
  - path: patches/database-url-reservation.yaml

  # -------------------------------------------------------------------------
  # ConfigMap patches: replace in-cluster Kafka bootstrap address with the
  # MSK bootstrap string for catalog, reservation, and search.
  # -------------------------------------------------------------------------
  - path: patches/configmap-catalog.yaml
  - path: patches/configmap-reservation.yaml
  - path: patches/configmap-search.yaml

  # -------------------------------------------------------------------------
  # Ingress: switch from NGINX to AWS Application Load Balancer.
  # -------------------------------------------------------------------------
  - path: patches/ingress-alb.yaml
```

---

## Patch files

The `kustomization.yaml` above references several patch files in a `patches/` subdirectory. Create that directory alongside `kustomization.yaml`.

### Resource limits

```yaml
# deploy/k8s/overlays/production/patches/resources.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: placeholder
  namespace: library
spec:
  template:
    spec:
      containers:
        - name: placeholder
          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
```

This patch doubles the base limits. The `target` selector in `kustomization.yaml` applies it to all Deployments in the `library` namespace. The `placeholder` names in `metadata.name` and the container `name` field are required YAML structure — the target selector is what actually routes the patch to the correct resources.

In practice, you will want per-service tuning once you observe real usage in production. This single patch is a sensible starting point and avoids fragility from hardcoding limits in the base.

### DATABASE_URL patches

Each database service uses a different RDS endpoint. The patch for `auth` illustrates the pattern; `catalog` and `reservation` follow the same shape.

There is a critical requirement here that trips people up: because the base `auth` Deployment has both `env` and `envFrom` blocks on the container, any strategic merge patch targeting that container **must include the `envFrom` block**. Kustomize merges containers by name, and if your patch omits `envFrom`, the rendered output will be missing the ConfigMap reference entirely. The original `envFrom` is not preserved — it is replaced by the patch container entry.

```yaml
# deploy/k8s/overlays/production/patches/database-url-auth.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth
  namespace: library
spec:
  template:
    spec:
      containers:
        - name: auth
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-auth-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: "host=auth-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=auth sslmode=require"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          envFrom:
            - configMapRef:
                name: auth-config
```

```yaml
# deploy/k8s/overlays/production/patches/database-url-catalog.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: library
spec:
  template:
    spec:
      containers:
        - name: catalog
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-catalog-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: "host=catalog-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=catalog sslmode=require"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          envFrom:
            - configMapRef:
                name: catalog-config
```

```yaml
# deploy/k8s/overlays/production/patches/database-url-reservation.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reservation
  namespace: library
spec:
  template:
    spec:
      containers:
        - name: reservation
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-reservation-secret
                  key: POSTGRES_PASSWORD
            - name: DATABASE_URL
              value: "host=reservation-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=reservation sslmode=require"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: jwt-secret
                  key: JWT_SECRET
          envFrom:
            - configMapRef:
                name: reservation-config
```

Two things change from the base `DATABASE_URL` values. First, `host=` points to an RDS endpoint instead of a Kubernetes DNS name. The `xxxxxxxxxxxx` placeholder is replaced with the actual RDS instance identifier once Terraform has provisioned the instances — section 12.4 captures those values as Terraform outputs. Second, `sslmode=require` replaces `sslmode=disable`. RDS enforces TLS; the local Postgres StatefulSet does not.

The `$(POSTGRES_PASSWORD)` substitution still works exactly as in the base. Kubernetes resolves env variable references within a container's `env` list in declaration order. `POSTGRES_PASSWORD` is declared before `DATABASE_URL`, so the substitution is valid.

### ConfigMap patches for Kafka

ConfigMap patches are simpler than Deployment patches because ConfigMaps have no containers with `envFrom` complexity. Each patch is a partial update that merges by key.

```yaml
# deploy/k8s/overlays/production/patches/configmap-catalog.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalog-config
  namespace: library
data:
  KAFKA_BROKERS: "b-1.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092,b-2.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092"
```

```yaml
# deploy/k8s/overlays/production/patches/configmap-reservation.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: reservation-config
  namespace: library
data:
  KAFKA_BROKERS: "b-1.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092,b-2.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092"
```

```yaml
# deploy/k8s/overlays/production/patches/configmap-search.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: search-config
  namespace: library
data:
  KAFKA_BROKERS: "b-1.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092,b-2.library-msk.xxxxxxxx.c1.kafka.us-east-1.amazonaws.com:9092"
```

These patches specify only the keys that change. `GRPC_PORT`, `CATALOG_GRPC_ADDR`, `MEILI_URL`, `MAX_ACTIVE_RESERVATIONS`, and any other keys in each ConfigMap are preserved from the base.

The MSK bootstrap string lists both brokers. Kafka clients use the bootstrap list to discover the full cluster membership; a single entry works for connection, but listing both gives the client a fallback if one broker is temporarily unavailable during startup.

Service-to-service gRPC addresses — `CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"` and similar — require no changes. All application services run in the same EKS cluster and the same `library` namespace. Kubernetes DNS works identically whether the cluster is kind or EKS. Only the dependencies that moved out of the cluster (Postgres, Kafka) need their addresses updated.

### Ingress patch

The base Ingress uses NGINX as its controller. EKS uses the AWS Load Balancer Controller, which provisions an Application Load Balancer in response to Ingress resources annotated for ALB.

```yaml
# deploy/k8s/overlays/production/patches/ingress-alb.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: library-ingress
  namespace: library
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:us-east-1:123456789012:certificate/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}, {"HTTPS": 443}]'
    alb.ingress.kubernetes.io/ssl-redirect: "443"
spec:
  ingressClassName: alb
```

The annotations drive the ALB Controller:

- `scheme: internet-facing` creates a public-facing ALB. Use `internal` for one accessible only within the VPC.
- `target-type: ip` registers pod IPs directly with the ALB target group instead of going through node ports. This is the recommended mode for EKS and removes an unnecessary hop through kube-proxy.
- `certificate-arn` attaches the ACM certificate for TLS termination at the load balancer. Replace the placeholder ARN with the certificate provisioned by Terraform in section 12.1.
- `ssl-redirect: "443"` instructs the ALB to issue a 301 redirect for plain HTTP requests, enforcing HTTPS without any application code change.

The `spec.rules` block is inherited from the base manifest unchanged. The patch adds `ingressClassName: alb` to the spec and the full annotation set to `metadata.annotations`, leaving the routing rules untouched.

---

## Meilisearch and persistent storage

Meilisearch remains a StatefulSet inside EKS — there is no AWS-managed equivalent for its API. Its PersistentVolumeClaim works without modification.

In kind, the default StorageClass provisions volumes through the `rancher.io/local-path` provisioner using the host filesystem. In EKS, the default StorageClass is backed by the Amazon EBS CSI driver and provisions `gp2` EBS volumes automatically.

The PVC declaration in the base manifest:

```yaml
volumeClaimTemplates:
  - metadata:
      name: meilisearch-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 5Gi
```

This is storage-class agnostic. It requests `ReadWriteOnce` access and 5 GiB. On kind it binds to a local path. On EKS it binds to a `gp2` EBS volume. No patch is needed.

If you want `gp3` instead — lower cost per GiB, better baseline throughput than `gp2` — add a StorageClass resource to the production overlay and mark it as the cluster default:

```yaml
# deploy/k8s/overlays/production/storage-class.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
parameters:
  type: gp3
  encrypted: "true"
```

`WaitForFirstConsumer` delays volume provisioning until the pod that claims it is scheduled to a node, which lets EBS create the volume in the correct Availability Zone. Using `Immediate` binding can create a volume in a different AZ than the pod lands in, causing a mount failure.

Add this file to the `resources` list in `kustomization.yaml` and also remove the default annotation from the existing `gp2` StorageClass. This is an optional optimization — the system functions correctly with `gp2`.

---

## Verifying the render

Before applying to the cluster, render the overlay locally to confirm it produces valid output:

```bash
kubectl kustomize deploy/k8s/overlays/production
```

Scan the output and confirm:

1. Every Deployment shows `replicas: 2`.
2. Every container image references the ECR URI, not `library-system/<svc>:latest`.
3. Every container shows `imagePullPolicy: Always`.
4. The `auth`, `catalog`, and `reservation` Deployments show RDS hostnames in `DATABASE_URL` with `sslmode=require`.
5. The `catalog-config`, `reservation-config`, and `search-config` ConfigMaps show the MSK bootstrap string in `KAFKA_BROKERS`.
6. The `catalog`, `reservation`, and `auth` Deployments still have the `envFrom` ConfigMap reference alongside the `env` block.
7. The Ingress shows `ingressClassName: alb` and the full annotation set.
8. No Postgres StatefulSets or Kafka StatefulSets appear anywhere in the output.

If the render looks correct, apply it:

```bash
kubectl apply -k deploy/k8s/overlays/production
```

Watch the rollout for all five services:

```bash
kubectl rollout status deployment/gateway -n library
kubectl rollout status deployment/auth -n library
kubectl rollout status deployment/catalog -n library
kubectl rollout status deployment/reservation -n library
kubectl rollout status deployment/search -n library
```

If a Deployment stalls, `kubectl describe deployment/<name> -n library` shows events including image pull errors and probe failures. `kubectl logs deployment/<name> -n library` shows application startup logs. The most common failure modes at this point: an RDS security group not opened for the EKS node CIDR, an ECR pull failing because the node IAM role is missing `ecr:GetAuthorizationToken`, or a missing secret because the placeholder values were not replaced before applying.

---

## What remains for Chapter 13

The `secretGenerator` in this overlay writes placeholder strings into Kubernetes Secrets. That is a deliberate simplification — real credentials belong in AWS Secrets Manager, not in a `kustomization.yaml` committed to a repository.

Chapter 13 replaces the `secretGenerator` block entirely with ExternalSecret resources. The External Secrets Operator (ESO) watches ExternalSecret objects, fetches the value from AWS Secrets Manager, and creates or updates a native Kubernetes Secret. The Deployments do not change — they continue reading from `postgres-auth-secret` and `jwt-secret` by name. Only the source of those secrets changes.

Until that work is complete, populate secrets out-of-band after applying the overlay:

```bash
kubectl create secret generic jwt-secret \
  --from-literal=JWT_SECRET="$(aws secretsmanager get-secret-value \
    --secret-id library/jwt-secret --query SecretString --output text)" \
  --namespace library --dry-run=client -o yaml | kubectl apply -f -
```

This is operational friction, not a permanent solution. It is preferable to committing credentials to git.

---

## Summary

The production overlay adds seven things to the base without touching a single base manifest:

1. **Image rewrites** via the `images` transformer — ECR URIs replace local image names across all Deployments.
2. **Replica patches** — all Deployments run two replicas for availability.
3. **Resource patches** — CPU and memory limits are doubled to reflect real production capacity (250m–500m CPU, 256Mi–512Mi memory).
4. **`imagePullPolicy: Always`** — pods always pull from ECR, ensuring the image on disk matches the declared tag.
5. **DATABASE_URL patches** — RDS endpoints replace in-cluster Postgres DNS names; `sslmode=require` replaces `sslmode=disable`; `envFrom` is preserved to retain ConfigMap references.
6. **ConfigMap patches** — MSK bootstrap addresses replace the in-cluster Kafka DNS name in `catalog-config`, `reservation-config`, and `search-config`.
7. **Ingress patch** — ALB controller annotations and `ingressClassName: alb` replace the NGINX configuration.

The base manifests under `deploy/k8s/base/library/` are unchanged from Chapter 11. The local overlay continues to work identically for development. The production overlay encodes every environment difference in one directory. The property established in Chapter 11 — environment differences are explicit and isolated — holds at production scale.

---

[^1]: Kustomize Strategic Merge Patch: https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/
[^2]: AWS Load Balancer Controller Ingress Annotations: https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.7/guide/ingress/annotations/
[^3]: Amazon EBS CSI Driver: https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html
[^4]: Kustomize ConfigMap Merge Behavior: https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/configmapgenerator/
[^5]: External Secrets Operator: https://external-secrets.io/latest/
