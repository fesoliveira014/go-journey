# Changelog: deploying.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Strong opening rhythm (five short confirmations).
  - Verification checklist is well-ordered: pods → logs → health → Ingress → end-to-end flow. Follows troubleshooting heuristics.
  - Troubleshooting table is well-structured (symptom/cause/fix). Keep as-is.
  - "Note on telemetry" callout is appropriately placed at the end of the checklist.

## Pass 2: Line Editing
- **Line ~111:** "startup probes" is misleading
  - Before: "The application services have startup probes and will restart if the database or broker is not yet accepting connections."
  - After: "The application services have liveness probes that may restart them if startup takes too long while the database or broker is not yet accepting connections."
  - Reason: the manifests in app-manifests.md configure liveness and readiness probes, not `startupProbe`. "Startup probes" is a specific Kubernetes feature (stable since 1.16); calling liveness probes "startup probes" is inaccurate.
- **Line ~243:** clarify kind re-creation timing
  - Before: "Re-creating the cluster from scratch with `kind create cluster --config kind-config.yaml --name library` takes the same 30 seconds as the first time (images are cached locally)."
  - After: "Re-creating the cluster takes about 30 seconds once the node image is cached locally (the first `kind create cluster` pulls ~700 MB; subsequent runs skip the pull)."
  - Reason: the current phrasing implies "first time = 30s," which contradicts kind-setup.md's accurate first-run timing.

## Pass 3: Copy Editing
- **Line ~11:** "Section 12.1" — capital S on cross-reference; normalize to lowercase per CMOS 8.178.
- **Line ~22:** `docker images | grep library-system` — works but relies on grep; consider the more portable `docker images --filter "reference=library-system/*"`. Optional.
- **Line ~40:** `crictl` described as "the `containerd` CLI" — `crictl` is actually the CRI (Container Runtime Interface) CLI, working with any CRI-compliant runtime. Minor imprecision. Suggest: "`crictl` is the CRI CLI bundled inside kind nodes."
- **Line ~70:** output block includes `secret/oauth-secret created` — but the kustomize.md secretGenerator list does not create an `oauth-secret`. Either (a) add it to the overlay's secretGenerator in kustomize.md, or (b) remove from the output here. Reconcile.
- **Line ~92:** output lists `ingress.networking.k8s.io/gateway created` — but the Ingress resource in app-manifests.md is named `library-ingress`, not `gateway`. Output should be `ingress.networking.k8s.io/library-ingress created`. Fix.
- **Line ~170:** "the Gateway or Ingress" — inconsistent capitalization; the gateway service is lowercase elsewhere. Normalize.
- **Line ~195:** `curl ... -d '{"title":"The Go Programming Language","author":"Donovan & Kernighan","isbn":"978-0134190440"}'` — ISBN 978-0134190440 verified (The Go Programming Language, 2015, Donovan & Kernighan). Correct.
- **Line ~119:** "30–60 seconds" — en dash for range per CMOS 9.58. Correct.
- **Line ~217:** kind default StorageClass `standard` backed by `rancher.io/local-path` — verified correct per kind documentation.
- **Line ~217:** "verify your PVC requests `standard`" — but infra-manifests.md PVC specs don't set `storageClassName` (they use default). Rephrase: "Run `kubectl get sc` and confirm `standard` is marked (default). Your PVCs use the default class implicitly."
- **Line ~250:** `kubectl delete namespace library data messaging` — namespace deletion is asynchronous; add a note that the subsequent `kubectl apply` may briefly error with "namespace X is terminating." Minor UX aside.

## Pass 4: Final Polish
- **Line ~92:** `ingress.networking.k8s.io/gateway` → should match the Ingress `metadata.name` from app-manifests.md (`library-ingress`).
- **Line ~70:** `secret/oauth-secret` — not produced by the kustomize.md secretGenerator; remove or add to overlay.
- **Line ~111:** "startup probes" — not configured in the manifests.
- No typos or doubled words detected; output-block factual errors are the priority fixes.
