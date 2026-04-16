# Changelog: kind-setup.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Opening provides strong motivation before commands. Good.
  - "The `kind load` Gotcha" earns its dedicated H2 — common stumbling block, right call.
  - Chapter 10 cross-reference section reinforces the book's through-line.
  - Summary table is useful as a cheat sheet for returning readers.

## Pass 2: Line Editing
- **Line ~3:** drop filler
  - Before: "spinning one up just to test a YAML change"
  - After: "spinning one up to test a YAML change"
  - Reason: "just" softens a technical claim without adding meaning.
- **Line ~9:** redundant "inside it"
  - Before: "is a Docker container running `containerd` and `kubelet` inside it"
  - After: "is a Docker container running `containerd` and `kubelet`"
  - Reason: "running" already implies "inside"; "inside it" is tautological.

## Pass 3: Copy Editing
- **Line ~3:** "EKS, GKE, AKE" — AKE is not a standard acronym. Azure's managed Kubernetes is AKS. Query: is this a typo for AKS?
- **Line ~38:** `kind-linux-amd64` pinned to `v0.23.0`. Query: as of publication, newer kind versions may exist; this version was released June 2024. Note the pin is intentional but may warrant a "check for latest" aside.
- **Line ~115:** `v1.30.x` — query: verify kind v0.23.0 default Kubernetes version is 1.30.x (https://github.com/kubernetes-sigs/kind/releases/tag/v0.23.0). This is consistent with release notes.
- **Line ~129:** "Ingress Controller" — Kubernetes docs style "Ingress controller" (controller lowercase; only the Ingress API resource is capitalized). Normalize throughout section.
- **Line ~172:** "imagePullPolicy: Never" is used here but app-manifests.md uses "imagePullPolicy: IfNotPresent" for the same image. Query: standardize to one? Both work; the text already says "(or `IfNotPresent`)" here, and subsequent sections use `IfNotPresent`. Minor inconsistency; flag for author.

## Pass 4: Final Polish
- **Line ~91:** verify kind config apiVersion `kind.x-k8s.io/v1alpha4` is still current per https://kind.sigs.k8s.io/docs/user/configuration/ (the v1alpha4 API version has been stable for years).
- **Line ~132:** verify URL `https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml` resolves to a kind-specific deployment manifest. Matches official ingress-nginx documentation.
- No typos, doubled words, or broken cross-references detected beyond the AKE → AKS possibility.
