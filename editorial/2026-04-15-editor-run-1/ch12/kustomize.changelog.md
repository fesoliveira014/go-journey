# Changelog: kustomize.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Opening "what works in kind breaks for EKS" framing is effective.
  - Directory-tree diagram is the right tool for setup-style sections; keep.
  - "Kustomize Concepts in Practice" at the end is a useful reference; its placement (after examples) is defensible.
  - Local overlay's reasoning for `disableNameSuffixHash` is somewhat inaccurate — Kustomize's secretGenerator rewrites references automatically; the real pain in dev is noisy kubectl output and debugging, not manual updates.
  - Production overlay stub uses commented-out YAML heavily; annotations per block would help.

## Pass 2: Line Editing
- **Line ~3 & ~82:** fix chapter numbers
  - Before: "sections 11.3 and 11.4"
  - After: "sections 12.3 and 12.4"
  - Reason: the surrounding chapter is Chapter 12; the references are internal cross-references to earlier sections in the same chapter.
- **Line ~173:** clarify why hash suffixes are annoying in dev
  - Before: "If the name changes every time the secret content changes, you would have to update every Deployment that references it."
  - After: "If the name changes every time the secret content changes, `kubectl get secret postgres-catalog-secret` stops working; you have to remember the hashed name. Kustomize does rewrite `secretKeyRef.name` references automatically in pods it manages, but ad-hoc debugging becomes harder."
  - Reason: current wording implies manual updates of Deployments would be needed, which is not true for Kustomize-managed secrets.
- **Line ~210:** "all ~30 manifests"
  - Before: "all ~30 manifests"
  - After: "all thirty or so manifests"
  - Reason: CMOS 9.4 — avoid tilde in general prose.

## Pass 3: Copy Editing
- **Line ~3:** "sections 11.3 and 11.4" — wrong chapter number (should be 12.3/12.4). Fix.
- **Line ~82:** same — "sections 11.3 and 11.4" → "sections 12.3 and 12.4".
- **Line ~82:** "They contain no environment-specific values — no credentials, no replica counts, no resource limits that differ between environments." — but the base (per app-manifests.md and infra-manifests.md) DOES contain replica counts (`replicas: 1`) and resource limits (`250m`, `256Mi`). The base contains dev-appropriate defaults; the production overlay overrides them. This contradiction deserves acknowledgement: the base contains sensible defaults, not zero environment-specific values.
- **Line ~17:** Verify: "since version 1.14" — Kustomize merged into kubectl in v1.14 (Feb 2019). Correct.
- **Line ~190:** `commonLabels` — deprecated in Kustomize v5.0 in favor of `labels:` transformer. Query: the book might want to update to use `labels:`; alternatively note the deprecation.
- **Line ~188:** "`namePrefix` / `nameSuffix`" introduced as "appear in the production overlay comments but are not used in the local one" — but neither prefix/suffix actually appears in the stub YAML. Reword: "Two additional primitives worth knowing, though not shown in either overlay:".
- **Line ~218:** `kubectl get all` — known misnomer (does not include ConfigMaps, Secrets, PVCs, Ingresses). Add a brief clarifying note.
- **Line ~282:** "JSON 6902 patches" — the term is "RFC 6902 JSON Patch" or "JSON 6902 patches." Both are used; consider "RFC 6902 (JSON Patch) operations" for first-time clarity.
- **Line ~190:** "Chapter 13" — capital C fine when preceded by "in." CMOS 8.178 allows either; keep capital-C style if book uses it consistently.

## Pass 4: Final Polish
- **Line ~3:** chapter number typo (11 → 12) — critical fix.
- **Line ~82:** chapter number typo (11 → 12) — critical fix.
- **Line ~17:** "Kustomize is a configuration management tool built directly into `kubectl` since version 1.14." — factually correct.
- **Line ~94:** apiVersion `kustomize.config.k8s.io/v1beta1` — current and correct.
- No typos or doubled words detected; only the chapter-number issues warrant priority fixes.
