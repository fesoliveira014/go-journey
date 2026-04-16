# 12.5 Kustomize Environments

<!-- [STRUCTURAL] Good opening problem framing: "what works in kind breaks the moment you think about EKS." Motivates the whole chapter. -->
<!-- [LINE EDIT] "At the end of sections 11.3 and 11.4" — the chapter sections are 12.3 and 12.4, not 11.3 and 11.4. This is a factual chapter-number error — carried through from the previous chapter template. -->
<!-- [FINAL] Line 3: "sections 11.3 and 11.4" → "sections 12.3 and 12.4". Critical fix. -->
At the end of sections 11.3 and 11.4, the project has roughly thirty manifest files — Deployments, Services, ConfigMaps, StatefulSets, PersistentVolumeClaims, Ingress rules — organized across three namespaces. They work correctly in kind. The problem appears the moment you think about EKS.

<!-- [LINE EDIT] "In a real AWS cluster, secrets must come from AWS Secrets Manager or Kubernetes Secrets populated by the External Secrets Operator, not from literal values in YAML files." — 37 words; still readable. -->
<!-- [COPY EDIT] "ECR" — first use; expand on first mention (Amazon Elastic Container Registry) per CMOS 10.3. -->
In a real AWS cluster, secrets must come from AWS Secrets Manager or Kubernetes Secrets populated by the External Secrets Operator, not from literal values in YAML files. Resource limits should be larger to reflect actual capacity. Critical services should run as at least two replicas. Image references should carry explicit tags and always pull from ECR. None of those changes apply to your local kind cluster, where lightweight single-replica pods and embedded credentials are exactly right.

<!-- [LINE EDIT] "The naive solution is to copy the entire `deploy/k8s` directory into a second `deploy/k8s-production` tree and edit the differences." — good setup for why Kustomize is needed. -->
The naive solution is to copy the entire `deploy/k8s` directory into a second `deploy/k8s-production` tree and edit the differences. That works until both directories contain the same change — a new environment variable, a renamed label, a resource limit adjustment — and you have to apply it in two places. With five services across three namespaces and two environments, divergence becomes inevitable.

Kustomize solves this by separating what is shared from what varies.

---

## What Kustomize Is

<!-- [LINE EDIT] "Kustomize is a configuration management tool built directly into `kubectl` since version 1.14. No separate installation is required. You already have it." — crisp three-sentence intro. -->
<!-- [COPY EDIT] Please verify: Kustomize merged into kubectl in 1.14 (2019). Correct. -->
Kustomize is a configuration management tool built directly into `kubectl` since version 1.14. No separate installation is required. You already have it.

The core model is simple:

<!-- [LINE EDIT] "A **base** contains the canonical manifest files — Deployments, Services, StatefulSets, and so on. These describe the application without any environment-specific details." — clear. -->
- A **base** contains the canonical manifest files — Deployments, Services, StatefulSets, and so on. These describe the application without any environment-specific details.
<!-- [LINE EDIT] "An **overlay** references the base and applies patches, replacements, and generators on top of it." — serial comma; good. -->
- An **overlay** references the base and applies patches, replacements, and generators on top of it. Each environment gets its own overlay directory.
- `kubectl apply -k <overlay>` renders the base plus the overlay's patches into a single manifest stream and applies it.

<!-- [LINE EDIT] "Every overlay produces complete, valid Kubernetes YAML. Kustomize is not a templating engine — there are no `{{ }}` placeholders and no values files." — good direct contrast with Helm. -->
<!-- [COPY EDIT] "strategic merge patches" — lowercase; correct term. -->
<!-- [COPY EDIT] "A strategic merge patch for a Deployment, for example, knows to merge containers by name rather than replace the entire list." — good concrete example. -->
Every overlay produces complete, valid Kubernetes YAML. Kustomize is not a templating engine — there are no `{{ }}` placeholders and no values files. Instead, it uses structured JSON patch operations and strategic merge patches, both of which understand the shape of Kubernetes objects. A strategic merge patch for a Deployment, for example, knows to merge containers by name rather than replace the entire list.

---

## Directory Structure

Add a `base` directory and an `overlays` directory alongside the existing namespace directories:

<!-- [STRUCTURAL] Directory-tree diagram is the right tool here. -->
```
deploy/k8s/
├── base/
│   ├── kustomization.yaml
│   ├── library/
│   │   ├── namespace.yaml
│   │   ├── auth-configmap.yaml
│   │   ├── auth-deployment.yaml
│   │   ├── auth-service.yaml
│   │   ├── catalog-configmap.yaml
│   │   ├── catalog-deployment.yaml
│   │   ├── catalog-service.yaml
│   │   ├── reservation-configmap.yaml
│   │   ├── reservation-deployment.yaml
│   │   ├── reservation-service.yaml
│   │   ├── search-configmap.yaml
│   │   ├── search-deployment.yaml
│   │   ├── search-service.yaml
│   │   ├── gateway-configmap.yaml
│   │   ├── gateway-deployment.yaml
│   │   ├── gateway-service.yaml
│   │   ├── ingress.yaml
│   │   └── kustomization.yaml
│   ├── data/
│   │   ├── namespace.yaml
│   │   ├── meilisearch-configmap.yaml
│   │   ├── meilisearch-service.yaml
│   │   ├── meilisearch-statefulset.yaml
│   │   ├── postgres-auth-configmap.yaml
│   │   ├── postgres-auth-service.yaml
│   │   ├── postgres-auth-statefulset.yaml
│   │   ├── postgres-catalog-configmap.yaml
│   │   ├── postgres-catalog-service.yaml
│   │   ├── postgres-catalog-statefulset.yaml
│   │   ├── postgres-reservation-configmap.yaml
│   │   ├── postgres-reservation-service.yaml
│   │   ├── postgres-reservation-statefulset.yaml
│   │   └── kustomization.yaml
│   └── messaging/
│       ├── namespace.yaml
│       ├── kafka-configmap.yaml
│       ├── kafka-service.yaml
│       ├── kafka-statefulset.yaml
│       └── kustomization.yaml
└── overlays/
    ├── local/
    │   └── kustomization.yaml
    └── production/
        └── kustomization.yaml
```

<!-- [COPY EDIT] Filenames here use `-configmap.yaml`, `-service.yaml`, `-statefulset.yaml` — inconsistent with infra-manifests.md's `-cm.yaml`, `-svc.yaml`, `-sts.yaml`. Pick one. See also infra-manifests.changelog.md. -->
<!-- [LINE EDIT] "The manifest files in `base/` are the ones you wrote in sections 11.3 and 11.4, moved into this structure." — again, wrong section numbers (should be 12.3 and 12.4). -->
The manifest files in `base/` are the ones you wrote in sections 11.3 and 11.4, moved into this structure. They contain no environment-specific values — no credentials, no replica counts, no resource limits that differ between environments. Those are the overlay's responsibility.

<!-- [COPY EDIT] But app-manifests.md and infra-manifests.md both bake values that differ between environments into the base (e.g., MEILI_ENV=development, resource limits of 250m CPU, replicas=1). The claim that the base "contains no environment-specific values" contradicts what the manifests actually show. Either (a) the earlier sections should omit these from the base (move to overlay), or (b) this paragraph should acknowledge the base has sensible dev defaults that the production overlay patches. -->

---

## Base `kustomization.yaml`

The base directory needs a `kustomization.yaml` that tells Kustomize which resources belong to it:

```yaml
# deploy/k8s/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - data
  - messaging
  - library
```

<!-- [LINE EDIT] "Each entry is a directory that contains its own `kustomization.yaml`." — good. -->
<!-- [COPY EDIT] Please verify: apiVersion `kustomize.config.k8s.io/v1beta1` — correct and current. -->
Each entry is a directory that contains its own `kustomization.yaml`. Kustomize recurses into each subdirectory and assembles all resources declared there. The base itself is never applied directly; overlays reference it.

---

## Local Overlay

<!-- [LINE EDIT] "it references the base, and it generates the Secrets that the application services read from environment variables" — good description. -->
The local overlay handles two things: it references the base, and it generates the Secrets that the application services read from environment variables.

```yaml
# deploy/k8s/overlays/local/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

# Generate secrets with actual values for local development.
# Secrets referenced by library-namespace Deployments must exist in the library
# namespace; secrets referenced by data-namespace StatefulSets must exist in the
# data namespace. Some secret names appear twice (one per namespace).
secretGenerator:
  # library namespace secrets (used by Deployments)
  - name: jwt-secret
    namespace: library
    literals:
      - JWT_SECRET=dev-secret-change-in-production
  - name: postgres-catalog-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: postgres-auth-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: postgres-reservation-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=postgres
  - name: meilisearch-secret
    namespace: library
    literals:
      - MEILI_MASTER_KEY=dev-master-key-change-in-production
  # data namespace secrets (used by StatefulSets)
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

### `secretGenerator`

<!-- [LINE EDIT] "`secretGenerator` instructs Kustomize to create Secret objects from the provided literals." — clear. -->
<!-- [LINE EDIT] "Each entry in `literals` becomes a key-value pair in the Secret's `data` field." — good. -->
<!-- [COPY EDIT] "base64-encoded" — compound adjective; hyphenated; CMOS 7.81 correct. -->
`secretGenerator` instructs Kustomize to create Secret objects from the provided literals. Each entry in `literals` becomes a key-value pair in the Secret's `data` field. The values are base64-encoded by Kustomize automatically — you write the plaintext, the rendered YAML contains the encoded form.

The key names — `JWT_SECRET`, `POSTGRES_PASSWORD`, `MEILI_MASTER_KEY` — must match exactly what the Deployment manifests reference in their `secretKeyRef.key` fields.

<!-- [LINE EDIT] "Several secrets appear twice in the generator: once for the `library` namespace and once for the `data` namespace." — clearly explained. -->
<!-- [LINE EDIT] "The application service Deployments (in `library`) read passwords at runtime via `secretKeyRef`, while the PostgreSQL StatefulSets (in `data`) need the same values to initialize the database." — good rationale. -->
<!-- [COPY EDIT] "Both copies must exist." — blunt; good. -->
Several secrets appear twice in the generator: once for the `library` namespace and once for the `data` namespace. This is required because Kubernetes Secrets are namespace-scoped — a Secret in `data` is invisible to a Pod in `library` and vice versa. The application service Deployments (in `library`) read passwords at runtime via `secretKeyRef`, while the PostgreSQL StatefulSets (in `data`) need the same values to initialize the database. Both copies must exist.

<!-- [LINE EDIT] "By default, Kustomize appends a content hash to each generated Secret's name — `postgres-catalog-secret-8m6fk2t` instead of `postgres-catalog-secret`." — good example. -->
<!-- [COPY EDIT] "This is intentional and useful" — slightly weak phrasing. Consider: "This is deliberate and useful:" (colon for explanation). -->
<!-- [LINE EDIT] "Without the hash, updating a Secret's value does not restart pods, so running pods continue using the old value until they are manually restarted." — important warning. Keep. -->
By default, Kustomize appends a content hash to each generated Secret's name — `postgres-catalog-secret-8m6fk2t` instead of `postgres-catalog-secret`. This is intentional and useful: when the secret content changes, the hash changes, and any Deployment that references the secret by name (which now includes the new hash) triggers an automatic pod rollout. Without the hash, updating a Secret's value does not restart pods, so running pods continue using the old value until they are manually restarted.

### `disableNameSuffixHash: true`

<!-- [LINE EDIT] "For local development, the hash suffix is more annoying than useful." — good colloquial tutor tone. -->
For local development, the hash suffix is more annoying than useful. Deployments reference secrets by name in `env` blocks:

```yaml
env:
  - name: POSTGRES_PASSWORD
    valueFrom:
      secretKeyRef:
        name: postgres-catalog-secret
        key: POSTGRES_PASSWORD
```

<!-- [LINE EDIT] "If the name changes every time the secret content changes, you would have to update every Deployment that references it." — clear. -->
<!-- [COPY EDIT] Actually, Kustomize handles this via `secretGenerator` name transformations — it automatically rewrites `secretKeyRef.name` references to the hashed name. The prose here implies the user would have to update them manually, which is inaccurate. The real problem with hash suffixes in dev is: noisy `kubectl get secret` output, debugging is harder, and you can't easily `kubectl describe secret postgres-catalog-secret`. Rephrase. -->
If the name changes every time the secret content changes, you would have to update every Deployment that references it. During active development — where you change a secret value frequently — this friction is counterproductive.

<!-- [COPY EDIT] "is a Chapter 13 topic" — capital C on "Chapter 13" in prose; CMOS 8.178 prefers lowercase ("chapter 13") unless starting a sentence. Check book-wide style. -->
<!-- [LINE EDIT] "For production, consider leaving the hash enabled and using `replacements` to propagate the generated name into the Deployment specs automatically. That is a Chapter 13 topic." — good forward reference. -->
<!-- [COPY EDIT] Note: Kustomize's `replacements` feature is a specific primitive (replaces fields at specific JSON paths). For this use case, Kustomize already rewrites `secretKeyRef.name` automatically when the secret is generated by `secretGenerator` in the same kustomization — no `replacements` needed. The claim in this paragraph overstates the mechanism. Please verify. -->
`disableNameSuffixHash: true` keeps the name predictable. For production, consider leaving the hash enabled and using `replacements` to propagate the generated name into the Deployment specs automatically. That is a Chapter 13 topic.

---

## Deploying with the Local Overlay

To preview what Kustomize will render without applying it, use `kubectl kustomize`:

```bash
kubectl kustomize deploy/k8s/overlays/local
```

<!-- [LINE EDIT] "Review it to confirm the secrets are present and the manifests look correct before touching the cluster." — good practice advice. -->
This prints the full rendered YAML to stdout — every resource from the base plus the generated Secrets. Review it to confirm the secrets are present and the manifests look correct before touching the cluster.

To apply everything in one command:

```bash
kubectl apply -k deploy/k8s/overlays/local
```

<!-- [LINE EDIT] "This is the local equivalent of `docker compose up`." — effective analogy. -->
<!-- [COPY EDIT] "all ~30 manifests" — CMOS 9.4: use "~" sparingly in prose; consider "all thirty or so manifests". -->
This is the local equivalent of `docker compose up`. It submits all ~30 manifests plus the generated Secrets to the API server in a single operation, creating or updating each resource as needed.

To verify the result:

```bash
kubectl get all -n library
kubectl get all -n data
kubectl get all -n messaging
kubectl get secrets -n library
kubectl get secrets -n data
```

<!-- [COPY EDIT] "kubectl get all" — this does NOT list all resource types (it misses ConfigMaps, Secrets, Ingresses, PVCs, etc.). Per Kubernetes community convention, `kubectl get all` is a known misnomer. Consider noting this: "kubectl get all is a common shorthand — note it doesn't literally list all resources; for example, ConfigMaps and Secrets must be queried separately, which is why we add `kubectl get secrets` below." -->

---

## Production Overlay (Stub)

<!-- [LINE EDIT] "For now, it documents what needs to change and why, serving as a checklist" — good framing. -->
The production overlay will be fleshed out in Chapter 13 when you provision an EKS cluster. For now, it documents what needs to change and why, serving as a checklist:

```yaml
# deploy/k8s/overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base
  # ExternalSecret resources (populated by AWS Secrets Manager via ESO)
  # - external-secrets/jwt-secret.yaml
  # - external-secrets/postgres-credentials.yaml
  # - external-secrets/meilisearch-credentials.yaml

# Larger resource limits for production workloads:
# patches:
#   - path: patches/catalog-resources.yaml
#     target:
#       kind: Deployment
#       name: catalog

# Multiple replicas for high-availability services:
# patches:
#   - path: patches/replicas.yaml

# Force fresh image pulls on every rollout:
# patches:
#   - patch: |-
#       - op: replace
#         path: /spec/template/spec/containers/0/imagePullPolicy
#         value: Always
#     target:
#       kind: Deployment

# RDS endpoint instead of in-cluster PostgreSQL StatefulSet:
# patches:
#   - path: patches/postgres-endpoint.yaml

# Images from ECR with explicit tags:
# images:
#   - name: library-system/catalog
#     newName: 123456789.dkr.ecr.eu-west-1.amazonaws.com/library-system/catalog
#     newTag: v1.2.0
```

<!-- [COPY EDIT] The commented YAML block is useful but dense. Consider annotating each block with a brief intent line (e.g., "# 1. External secrets from AWS Secrets Manager" as a header comment). -->
Each commented section maps to a concrete problem:

<!-- [LINE EDIT] Good bulleted summary mapping YAML comments to their rationale. -->
<!-- [COPY EDIT] "External Secrets Operator (ESO)" — first expansion. Good. -->
- **External secrets** — credentials must not live in Git. The External Secrets Operator (ESO) reads from AWS Secrets Manager and creates native Kubernetes Secrets. Chapter 14 sets this up.
- **Resource patches** — production pods need real CPU and memory limits. A strategic merge patch on a Deployment's `resources` block adds these without duplicating the whole manifest.
<!-- [LINE EDIT] "a JSON patch on `spec.replicas` sets each service's replica count independently" — good. -->
- **Replica patches** — a JSON patch on `spec.replicas` sets each service's replica count independently.
<!-- [COPY EDIT] "`IfNotPresent` is correct for kind (which loads images locally); `Always` is correct for ECR (which holds canonical tagged releases)" — clear justification. -->
- **imagePullPolicy** — `IfNotPresent` is correct for kind (which loads images locally); `Always` is correct for ECR (which holds canonical tagged releases).
<!-- [LINE EDIT] "managed PostgreSQL replaces the in-cluster StatefulSet in production" — good. -->
<!-- [COPY EDIT] "A patch replaces the Service with an ExternalName Service pointing to the RDS endpoint." — accurate; ExternalName is the right tool for this. -->
- **RDS endpoint** — managed PostgreSQL replaces the in-cluster StatefulSet in production. A patch replaces the Service with an ExternalName Service pointing to the RDS endpoint.
- **Image references** — the `images` block rewrites image names and tags without touching the base manifests. All Deployments referencing `library-system/catalog` will use the ECR URI and the release tag.

---

## Kustomize Concepts in Practice

<!-- [STRUCTURAL] Reference-style section at the end is useful — treats readers as adults who can look up terms. Consider whether it should come earlier, before the overlay examples, so readers have vocabulary when reading the code. Trade-off is that abstract definitions are easier to grasp after concrete examples. Current placement (after examples) is defensible. -->
The overlay model introduces four Kustomize primitives worth knowing explicitly.

<!-- [LINE EDIT] "`resources` is the list of inputs: paths to YAML files or directories containing a `kustomization.yaml`." — clear. -->
**`resources`** is the list of inputs: paths to YAML files or directories containing a `kustomization.yaml`. Overlays use it to reference the base; the base uses it to enumerate its manifests.

<!-- [LINE EDIT] "A patch for a Deployment that specifies only `spec.replicas: 3` leaves all other fields unchanged. A patch that specifies a container by name merges into that container without affecting others. This is the most readable patching mechanism for simple changes." — well-chosen examples. -->
**Strategic merge patches** apply a partial YAML document on top of an existing resource, using the Kubernetes API schema to decide how to merge fields. A patch for a Deployment that specifies only `spec.replicas: 3` leaves all other fields unchanged. A patch that specifies a container by name merges into that container without affecting others. This is the most readable patching mechanism for simple changes.

<!-- [LINE EDIT] "JSON 6902 patches are precise surgical operations using RFC 6902 patch operations (`add`, `remove`, `replace`, `move`, `copy`)." — OK. -->
<!-- [COPY EDIT] "RFC 6902" / "JSON 6902 patches" — the common term is "JSON Patch" or "RFC 6902 patches". "JSON 6902 patches" is unusual; Kustomize docs use "JSON patches" or "JSON 6902". Consider the clearer "RFC 6902 (JSON Patch) operations." -->
**JSON 6902 patches** are precise surgical operations using RFC 6902 patch operations (`add`, `remove`, `replace`, `move`, `copy`). They address fields by JSON pointer path (`/spec/template/spec/containers/0/imagePullPolicy`). Use these when a strategic merge patch cannot express what you need — for example, changing a single element in a list identified by index.

<!-- [LINE EDIT] "They keep credential values out of base manifests and provide the hash-suffix rollout mechanism described earlier." — good recap. -->
**`secretGenerator` and `configMapGenerator`** create Kubernetes Secrets and ConfigMaps from literal values, files, or environment files. They keep credential values out of base manifests and provide the hash-suffix rollout mechanism described earlier.

Two additional primitives appear in the production overlay comments but are not used in the local one:

<!-- [COPY EDIT] But `namePrefix` / `nameSuffix` do NOT appear in the production overlay stub above. The stub only shows patches and images. Either add a namePrefix example to the stub, or rephrase: "Two additional primitives worth knowing, though not used in either overlay here:" -->
<!-- [LINE EDIT] "`namePrefix: team-a-` ensures there are no name collisions without modifying base manifests" — good example. -->
**`namePrefix` / `nameSuffix`** prepend or append a string to every resource name in the kustomization. Useful for multi-tenant clusters where teams share a namespace: `namePrefix: team-a-` ensures there are no name collisions without modifying base manifests.

<!-- [LINE EDIT] "A standard practice is to add `app.kubernetes.io/managed-by: kustomize` and an environment label so you can filter all local resources with a single `kubectl get` selector." — useful tip. -->
<!-- [COPY EDIT] Please verify: `commonLabels` was deprecated in Kustomize v5.0 (2023) in favor of `labels` (with transformers). The book says "commonLabels" — may still work but is deprecated. Consider noting: "Note: Kustomize v5+ prefers the newer `labels:` transformer block; `commonLabels` is deprecated but still functional." -->
**`commonLabels`** adds a set of labels to every resource. A standard practice is to add `app.kubernetes.io/managed-by: kustomize` and an environment label so you can filter all local resources with a single `kubectl get` selector.

---

## Summary

<!-- [LINE EDIT] "The manifests written in sections 12.3 and 12.4 live unchanged in `deploy/k8s/base/`." — good; note section numbers are correct here (12.3, 12.4). The opening and "The manifest files in `base/`…" paragraph use the wrong numbers (11.3, 11.4). Fix those. -->
<!-- [STRUCTURAL] Summary restates the key insight cleanly. -->
The manifests written in sections 12.3 and 12.4 live unchanged in `deploy/k8s/base/`. The local overlay adds generated Secrets and applies via `kubectl apply -k deploy/k8s/overlays/local`. The production overlay is a documented stub that Chapter 13 fills in with real patches, external secrets, and ECR image references.

<!-- [LINE EDIT] "Looking at `overlays/local/kustomization.yaml` tells you exactly how local differs from the base. Looking at `overlays/production/kustomization.yaml` tells you exactly how production differs." — good parallel structure. -->
<!-- [FINAL] "Adding a third environment — staging, or a CI-specific overlay — means adding one directory and one file without touching anything that already exists." — satisfying closer. -->
The important property is that environment differences are **explicit and isolated**. Looking at `overlays/local/kustomization.yaml` tells you exactly how local differs from the base. Looking at `overlays/production/kustomization.yaml` tells you exactly how production differs. The base contains no environment-specific assumptions. Adding a third environment — staging, or a CI-specific overlay — means adding one directory and one file without touching anything that already exists.

---

[^1]: Kustomize: https://kustomize.io/
[^2]: Kustomize Built-in Reference: https://kubectl.docs.kubernetes.io/references/kustomize/
[^3]: Managing Secrets with Kustomize: https://kubernetes.io/docs/tasks/configmap-secret/managing-secret-using-kustomize/
