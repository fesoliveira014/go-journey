# Changelog: app-manifests.md

## Pass 1: Structural / Developmental
- 8 comments. Themes:
  - **Critical ordering issue.** Opening refers to "the previous section" having declared infrastructure manifests, and notes that namespace manifests for `data` and `messaging` "were declared in section 12.2." But 12.2 is about preparing services (code-level), and 12.4 is the actual infrastructure manifests. The cross-references need reconciliation. Either reorder sections (12.3 ↔ 12.4) or rewrite openings to match current ordering.
  - The canonical catalog walkthrough is an effective spine; repeats for the remaining services are appropriately compressed.
  - The Kustomization subsection at the end assumes familiarity with Kustomize, which is formally introduced in 12.5. Either defer or add a "we'll cover this in 12.5" lead-in.
  - Final paragraph's forward reference ("Section 12.4 assembles the top-level `kustomization.yaml`…") appears inaccurate given that 12.4 is infra-manifests.
  - Secrets subsection is well-placed and appropriately emphatic ("Base64 is not encryption.").

## Pass 2: Line Editing
- **Line ~116:** "2+" → "two or more"
  - Before: "Increasing this to 2+ enables rolling updates"
  - After: "Increasing this to two or more enables rolling updates"
  - Reason: CMOS 9.2 — spell out small whole numbers in prose.
- **Line ~137:** drop filler "actually"
  - Before: "`containerPort` is documentation — it does not actually open a port or affect networking."
  - After: "`containerPort` is documentation — it does not open a port or affect networking."
  - Reason: "actually" is filler; removes weak emphasis.
- **Line ~291:** tighten "would route through the cluster's load balancer"
  - Before: "would route through the cluster's load balancer rather than directly to the pod"
  - After: "would be load-balanced across broker pods by kube-proxy rather than routed directly to one pod"
  - Reason: a ClusterIP Service uses kube-proxy load balancing, not a "load balancer" in the LoadBalancer Service sense; the original wording blurs terminology.
- **Line ~520:** clarify StatefulSet vs Deployment-backed Services
  - Before: "Application Services (as opposed to StatefulSets) are load-balanced by default, so using the Service name is correct."
  - After: "The catalog Service is a regular ClusterIP Service backed by a Deployment, so it load-balances across all catalog pods. StatefulSet pods use headless Services and are addressed by pod name (as with Kafka above)."
  - Reason: the original conflates a network object (Service) with a workload controller (StatefulSet).
- **Line ~757:** soften "undefined"
  - Before: "Without this field, if multiple Ingress controllers are installed, the behavior is undefined."
  - After: "Without this field, if multiple Ingress controllers are installed, the behavior is nondeterministic — whichever controller claims the Ingress first handles it."
  - Reason: "undefined" is too strong for documented behavior (multiple controllers claim-race).
- **Line ~814:** "retrying dependencies that are not yet ready"
  - Before: "`kubectl apply` handles resource creation order internally, retrying dependencies that are not yet ready"
  - After: "`kubectl apply` submits resources in a single request, and the API server plus controllers reconcile dependency readiness through normal retry loops"
  - Reason: `kubectl apply` itself does not retry; controllers do.

## Pass 3: Copy Editing
- **Line ~29:** "were declared in section 12.2" — factually incorrect; infrastructure manifests (incl. data/messaging namespaces) are in section 12.4. Fix to "will be declared in section 12.4" or reorder sections.
- **Line ~35:** Heading "Catalog Deployment — full walkthrough" mixes title case with sentence case. Normalize to "Catalog Deployment — Full Walkthrough" per the surrounding H2 style, or lowercase all H2s consistently.
- **Line ~81:** DATABASE_URL uses single-line `value: "..."` in full manifest but later snippet at line ~160 uses folded scalar `value: >-`. Standardize to one — recommend folded scalar for readability.
- **Lines ~94–103:** catalog probe spec omits `failureThreshold: 3`, but lines ~214–226 (same-section snippet) and auth/reservation/search/gateway manifests include it. Sync.
- **Line ~113:** catalog deployment lacks `metadata.labels: { app: catalog }`. Auth, reservation, search, gateway deployments include this. Add for consistency.
- **Lines ~325–374:** Auth deployment (and reservation/search/gateway) omit the securityContext blocks (pod-level and container-level) emphasised for catalog. Either include them in all services' YAML or add a prose note: "The same `securityContext` settings from catalog apply; omitted here for brevity."
- **Line ~409:** prose says auth Deployment references `GOOGLE_CLIENT_SECRET` via `secretKeyRef` with `optional: true`. The auth YAML does NOT include this env var. Either add it to the YAML or remove/revise the prose.
- **Line ~733:** "an Ingress controller to be running" — lowercase "controller" is Kubernetes docs style. Chapter also uses "Ingress Controller" in kind-setup.md. Normalize to lowercase throughout.
- **Line ~737:** verify `apiVersion: networking.k8s.io/v1` — stable since Kubernetes 1.19. Correct.
- **Line ~828:** forward reference "Section 12.4 assembles the top-level `kustomization.yaml`" contradicts 12.4's actual content (infra-manifests). Fix to correct section number (12.5) or reword.
- Throughout: "Ingress controller" / "Ingress Controller" / "Ingress" (the API resource) — be consistent. API resource capitalized; the reverse-proxy software lowercase.

## Pass 4: Final Polish
- **Line ~29:** factual error — "section 12.2" where it should reference section 12.4 (or resequence).
- **Line ~828:** factual error — "Section 12.4 assembles the top-level `kustomization.yaml`" but section 12.4 is infra-manifests, and section 12.5 is where Kustomize overlays are introduced.
- **Line ~189:** "runs as the `nobody` user" — UID 65534 is conventionally `nobody` on most Linux distributions. Accurate.
- No typos, doubled words, or broken cross-references detected beyond the two section-number issues above.
