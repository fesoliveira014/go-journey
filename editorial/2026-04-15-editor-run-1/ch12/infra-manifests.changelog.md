# Changelog: infra-manifests.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Clean opening contrast (stateless = Deployment, stateful = StatefulSet).
  - Good pattern of documenting only the diffs for auth/reservation databases.
  - Kafka networking section is pedagogically effective — names the problem before showing the fix.
  - Meilisearch Service YAML is omitted but referenced in kustomize.md's directory listing. Add for completeness.
  - Kustomization subsection uses Kustomize terminology before 12.5 formally introduces it — add a brief pointer.
  - MEILI_ENV=development in the base ConfigMap contradicts the book's own principle (environment values belong in overlays, not base).
  - Cross-namespace MEILI_URL inconsistency: app-manifests.md uses the Service name (`meilisearch.data.svc...`); infra-manifests.md's FQDN list uses the pod name (`meilisearch-0.meilisearch.data.svc...`). Reconcile.

## Pass 2: Line Editing
- **Line ~13:** "databases that embed their own hostname"
  - Before: "This is critical for databases that embed their own hostname in configuration (Kafka's `KAFKA_ADVERTISED_LISTENERS` is the canonical example)."
  - After: "This is critical for stateful systems that embed their own hostname in configuration (Kafka's `KAFKA_ADVERTISED_LISTENERS` is the canonical example)."
  - Reason: Kafka is not a database; "stateful systems" subsumes both.
- **Line ~363:** show Meilisearch Service
  - Before: "Meilisearch's headless Service follows the same pattern as the others, with port 7700."
  - After: add the actual YAML snippet for the Service (3-line addition) so the reader can copy it without guessing.
  - Reason: every other infra service in this section includes its Service manifest explicitly.
- **Line ~446:** "single transaction"
  - Before: "Kustomize processes all listed resources as a single transaction"
  - After: "Kustomize processes all listed resources as a single apply operation"
  - Reason: Kubernetes does not support multi-resource transactions; "transaction" misleads readers.
- **Line ~461:** "next section covers Secrets management"
  - Before: "The next section covers Secrets management — how to create the `postgres-catalog-secret`, `meilisearch-secret`, and similar objects without committing credentials to the repository."
  - After: "The next section introduces Kustomize environments — including the overlay that generates these Secrets for local development without committing credentials to the repository."
  - Reason: 12.5 is Kustomize environments; secrets are a part of it, not the whole topic.

## Pass 3: Copy Editing
- **Line ~15:** "Pod-0" and "pod-0" — case mixing within a paragraph. Pick one. Suggest lowercase "pod-0" throughout (matching Kubernetes docs style for ordinal pod names).
- **Line ~24:** heading "StatefulSet vs Deployment" — no period after "vs." per typical title/heading conventions (CMOS 7.59 allows either). Consistent.
- **Line ~26:** "Section 12.5" — capital S; other cross-references in the chapter use lowercase "section 12.1". Normalize to lowercase per CMOS 8.178.
- **Lines ~30, 57, 73, 184, 213, 234, 294, 310, 404, 423:** comment paths `k8s/...` — but app-manifests.md and kustomize.md use `deploy/k8s/base/...`. Pick one prefix throughout the chapter.
- **Line ~92:** StatefulSet spec lacks `securityContext` blocks that app-manifests.md emphasised. For StatefulSets running databases, readOnlyRootFilesystem breaks Postgres — but runAsNonRoot and a defined runAsUser are still possible. Add a brief note justifying the absence or include baseline settings.
- **Lines ~254, 329:** verify image tags:
  - `apache/kafka:3.9` — Apache Kafka 3.9 released Oct 2024; tag should exist. Verify via https://hub.docker.com/r/apache/kafka/tags.
  - `getmeili/meilisearch:v1.12` — Meilisearch 1.12 released late 2024; verify via https://hub.docker.com/r/getmeili/meilisearch/tags.
- **Line ~199:** "KAFKA_AUTO_CREATE_TOPICS_ENABLE: 'true'" — convenient for dev; consider brief aside about production practice of disabling auto-create.
- **Line ~381:** "kube-dns" — Kubernetes clusters since ~1.13 use CoreDNS as the DNS server; "kube-dns" survives as the Service name. For technical precision in the diagram/prose, consider using "CoreDNS" or clarifying. Flag as query.
- **Lines ~409–432:** filename suffixes `-svc.yaml`, `-cm.yaml`, `-sts.yaml` conflict with kustomize.md's `-service.yaml`, `-configmap.yaml`, `-statefulset.yaml`. Pick one convention.
- **Line ~375:** "meilisearch-0.meilisearch.data.svc.cluster.local:7700" vs `MEILI_URL: "http://meilisearch.data.svc.cluster.local:7700"` in search-configmap (app-manifests.md). Reconcile: does the app reach Meilisearch by pod name or by Service name? Single-replica StatefulSet makes either work, but the convention should be consistent.

## Pass 4: Final Polish
- **Line ~92:** verify `postgres:16-alpine` — image exists and is actively maintained. Correct.
- **Line ~446:** "Ordering within the apply is handled by the Kubernetes API server, which creates namespaced resources after the namespace object exists." — but this kustomization.yaml does NOT include the namespace. Clarify that the namespace is assumed pre-existing (created by the parent base kustomization).
- No typos, doubled words, or broken cross-references detected beyond the terminology issues above.
