# 13.7 Production Kustomize Overlay

<!-- [STRUCTURAL] This is the section that ties all previous Terraform outputs to the Kubernetes manifests. Structure: base restructuring → overlay kustomization → patch files → Meilisearch storage → verify → Chapter 14 look-ahead → summary. Very thorough. One suggestion: the "Restructuring the base for production" subsection is a mid-chapter refactor; consider moving it to a sidebar or a Chapter 12 addendum so 13.7 can focus on the overlay. -->
<!-- [COPY EDIT] Heading: "13.7 Production Kustomize Overlay" matches index.md "13.7 — Production Kustomize Overlay" with variance in dash/space. OK. -->

Chapter 12 introduced Kustomize and structured the manifests into a base and two overlays. The local overlay configured kind-specific settings: a `secretGenerator` with plaintext development credentials, a single replica per service, and `imagePullPolicy: IfNotPresent` so kind's locally loaded images are used without hitting a registry. That overlay is still correct for local development and remains untouched.

Now you write the production overlay. At this point you have an EKS cluster, three RDS instances, an MSK cluster, ECR repositories, and a working CI pipeline. The production overlay is the piece that wires those AWS resources into the Kubernetes manifests — without modifying a single file under `deploy/k8s/base/`.
<!-- [COPY EDIT] "a working CI pipeline" — but cicd.md (13.8) is the NEXT section. You don't have the CI pipeline yet at 13.7. Fix forward reference. -->

---

## Restructuring the base for production

There is one structural change to make before writing the overlay: Postgres and Kafka StatefulSets have no place in a production cluster. Production uses RDS and MSK. Including the in-cluster StatefulSets in the base and then having the production overlay try to delete or ignore them creates unnecessary complexity and risk.
<!-- [LINE EDIT] "There is one structural change to make before writing the overlay" → "One structural change comes first" (tighter). -->
<!-- [COPY EDIT] "Postgres" — informal. Chapter elsewhere uses "PostgreSQL" for the product and "postgres" only in HCL. CMOS 8.152 prefers official name. -->

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
<!-- [STRUCTURAL] The ASCII trees before/after are the right way to show restructuring. Well done. -->

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
<!-- [COPY EDIT] The local-infra kustomization references `data` and `messaging` as relative dirs — but note that sibling `data/` and `messaging/` in base/ already reference themselves. Avoid naming collisions. -->

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
<!-- [LINE EDIT] "simply do not exist" — "simply" is filler. → "do not exist". -->

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
# Secret placeholders: Chapter 14 replaces these with External Secrets
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
<!-- [COPY EDIT] "newTag: latest" — conflicts with the practice of using `sha-<commit>` tags established in ecr.md and cicd.md. Overlay committed to git should use a fixed sha tag, not latest. Either show placeholder that CI replaces, or explain. -->
<!-- [COPY EDIT] Image name `library-system/catalog` here; ecr.md shows `name: catalog` (shorter). Kustomize matches on exact name — the base manifests need to declare images by `library-system/<svc>`. Unify. -->
<!-- [COPY EDIT] JSON6902 path `/spec/template/spec/containers/0/imagePullPolicy` assumes containers[0] is the service container. If base has init containers or multi-container pods this breaks. Flag as brittle. -->

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
<!-- [COPY EDIT] Please verify: strategic merge patches in Kustomize with `target` selector still require `metadata.name` to match — OR the `placeholder` name is a convention only when combined with patches that use `target`. Confirm Kustomize behavior; `placeholder` name works only because `target.kind` overrides matching. -->
<!-- [COPY EDIT] The container `name: placeholder` would NOT match any real container (base containers are `catalog`, `auth`, etc.). Kustomize strategic merge patches match containers by name — a `placeholder` name would create a new container, not patch the existing one. This pattern likely breaks; verify. Usually one uses a JSON6902 patch or wildcard for this. -->

In practice, you will want per-service tuning once you observe real usage in production. This single patch is a sensible starting point and avoids fragility from hardcoding limits in the base.

### DATABASE_URL patches

Each database service uses a different RDS endpoint. The patch for `auth` illustrates the pattern; `catalog` and `reservation` follow the same shape.

There is a critical requirement here that trips people up: because the base `auth` Deployment has both `env` and `envFrom` blocks on the container, any strategic merge patch targeting that container **must include the `envFrom` block**. Kustomize merges containers by name, and if your patch omits `envFrom`, the rendered output will be missing the ConfigMap reference entirely. The original `envFrom` is not preserved — it is replaced by the patch container entry.
<!-- [COPY EDIT] FACTUAL CONFLICT WITH rds.md: rds.md (13.4) says "Only `DATABASE_URL` is overridden" (i.e., full merge). This section says `envFrom` must be re-declared or it's lost. Please verify Kustomize strategic merge patch behavior with actual `kustomize build` output. Typically, envFrom is a list without merge key — so it gets replaced. env items with `name` key merge. Reconcile with rds.md. -->

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
<!-- [COPY EDIT] DATABASE_URL uses libpq keyword=value format here; rds.md uses URL form (postgresql://...). Both work with pgx but you should pick one canonical across files. -->
<!-- [COPY EDIT] RDS identifier format `auth-db.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com` — rds.md uses `library-catalog.xxxx.us-east-1.rds.amazonaws.com` (with product prefix, only 4 x's). Normalize. -->

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

Two things change from the base `DATABASE_URL` values. First, `host=` points to an RDS endpoint instead of a Kubernetes DNS name. The `xxxxxxxxxxxx` placeholder is replaced with the actual RDS instance identifier once Terraform has provisioned the instances — section 13.4 captures those values as Terraform outputs. Second, `sslmode=require` replaces `sslmode=disable`. RDS enforces TLS; the local Postgres StatefulSet does not.
<!-- [LINE EDIT] "once Terraform has provisioned the instances" — "has" optional. Keep. -->

The `$(POSTGRES_PASSWORD)` substitution still works exactly as in the base. Kubernetes resolves env variable references within a container's `env` list in declaration order. `POSTGRES_PASSWORD` is declared before `DATABASE_URL`, so the substitution is valid.
<!-- [COPY EDIT] Please verify: Kubernetes `$(VAR)` substitution resolves forward (later env values can reference earlier), order-dependent. Confirmed per K8s docs. OK. -->

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
<!-- [COPY EDIT] MSK hostname format differs from msk.md example (`b-1.library.abc123.c2.kafka...` vs `b-1.library-msk.xxxxxxxx.c1.kafka...`). Normalize. -->

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
<!-- [COPY EDIT] `kubernetes.io/ingress.class: alb` annotation is deprecated. `spec.ingressClassName: alb` (shown below in same manifest) is the modern form. Both appearing is confusing — remove the annotation; keep the spec field. AWS LB Controller picks up `ingressClassName`. -->
<!-- [COPY EDIT] Please verify: `alb.ingress.kubernetes.io/certificate-arn` annotation key. Confirmed valid. -->
<!-- [COPY EDIT] ACM certificate ARN is a placeholder but it is not provisioned anywhere in this chapter. index.md says Chapter 14 handles ACM/Route53. production-overlay.md references the ARN as if it exists. Flag as forward-dependent. -->

The annotations drive the ALB Controller:

- `scheme: internet-facing` creates a public-facing ALB. Use `internal` for one accessible only within the VPC.
- `target-type: ip` registers pod IPs directly with the ALB target group instead of going through node ports. This is the recommended mode for EKS and removes an unnecessary hop through kube-proxy.
- `certificate-arn` attaches the ACM certificate for TLS termination at the load balancer. Replace the placeholder ARN with the certificate provisioned by Terraform in section 13.1.
<!-- [COPY EDIT] "provisioned by Terraform in section 13.1" — section 13.1 (terraform-fundamentals.md) does NOT provision ACM certificates. Fix cross-reference. -->
- `ssl-redirect: "443"` instructs the ALB to issue a 301 redirect for plain HTTP requests, enforcing HTTPS without any application code change.
<!-- [COPY EDIT] Please verify: `ssl-redirect` issues 301 (permanent) redirects. AWS LB Controller docs state this; confirm. -->

The `spec.rules` block is inherited from the base manifest unchanged. The patch adds `ingressClassName: alb` to the spec and the full annotation set to `metadata.annotations`, leaving the routing rules untouched.

---

## Meilisearch and persistent storage

Meilisearch remains a StatefulSet inside EKS — there is no AWS-managed equivalent for its API. Its PersistentVolumeClaim works without modification.

In kind, the default StorageClass provisions volumes through the `rancher.io/local-path` provisioner using the host filesystem. In EKS, the default StorageClass is backed by the Amazon EBS CSI driver and provisions `gp2` EBS volumes automatically.
<!-- [COPY EDIT] Please verify: EKS default StorageClass as of 2024/2025 — EKS now ships `gp2` as the default from legacy in-tree provisioner; `ebs.csi.aws.com` StorageClass is optional. Newer EKS clusters (1.29+) should prefer `ebs.csi.aws.com` with gp3 baseline. Clarify. -->

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
<!-- [LINE EDIT] "also remove the default annotation from the existing `gp2` StorageClass" — how? Kustomize can patch cluster-scoped resources that existed before the overlay applied; this is non-obvious. Mention `kubectl patch` or explain. -->

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

## What remains for Chapter 14

The `secretGenerator` in this overlay writes placeholder strings into Kubernetes Secrets. That is a deliberate simplification — real credentials belong in AWS Secrets Manager, not in a `kustomization.yaml` committed to a repository.

Chapter 14 replaces the `secretGenerator` block entirely with ExternalSecret resources. The External Secrets Operator (ESO) watches ExternalSecret objects, fetches the value from AWS Secrets Manager, and creates or updates a native Kubernetes Secret. The Deployments do not change — they continue reading from `postgres-auth-secret` and `jwt-secret` by name. Only the source of those secrets changes.

Until that work is complete, populate secrets out-of-band after applying the overlay:

```bash
kubectl create secret generic jwt-secret \
  --from-literal=JWT_SECRET="$(aws secretsmanager get-secret-value \
    --secret-id library/jwt-secret --query SecretString --output text)" \
  --namespace library --dry-run=client -o yaml | kubectl apply -f -
```

This is operational friction, not a permanent solution. It is preferable to committing credentials to git.
<!-- [LINE EDIT] "operational friction, not a permanent solution" — good. Keep. -->

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

The base manifests under `deploy/k8s/base/library/` are unchanged from Chapter 12. The local overlay continues to work identically for development. The production overlay encodes every environment difference in one directory. The property established in Chapter 12 — environment differences are explicit and isolated — holds at production scale.

---

[^1]: Kustomize Strategic Merge Patch: https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/
[^2]: AWS Load Balancer Controller Ingress Annotations: https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.7/guide/ingress/annotations/
[^3]: Amazon EBS CSI Driver: https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html
[^4]: Kustomize ConfigMap Merge Behavior: https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/configmapgenerator/
[^5]: External Secrets Operator: https://external-secrets.io/latest/
<!-- [FINAL] Footnotes uncited inline. -->
<!-- [FINAL] "AWS Load Balancer Controller v2.7" URL — pin may drift; consider "/latest/guide/ingress/annotations/". -->
