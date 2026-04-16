# Changelog: argocd.md

## Pass 1: Structural / Developmental
- 1 comment. Excellent discussion-style close to the chapter. Structure works as-is.

## Pass 2: Line Editing
- **Line ~3:** "perfectly reasonable" — drop "perfectly".
- **Line ~64:** "are how you enforce" → "AppProjects enforce".
- **Line ~64:** "a single button click" → "a single click".
- **Line ~93:** "configuration … are removed" — subject-verb agreement.
- **Line ~120:** "the pipeline should be configured to skip" → "configure the pipeline to skip".
- **Line ~143:** "is not trivial" — double-negative; "is non-trivial" or "is substantial".
- **Line ~147:** "ArgoCD cannot deploy itself" — slight imprecision; can manage itself after bootstrap (App-of-Apps). Reword "cannot install itself the first time."
- **Line ~151:** "actually exist" — drop "actually". Consider active voice.

## Pass 3: Copy Editing
- **Throughout:** "ArgoCD" vs official "Argo CD" (with space). Pick canonical. Apply consistently.
- **Line ~13:** QUERY — GitOps coined by Weaveworks 2017: confirmed.
- **Line ~13:** OpenGitOps's four principles: declarative, versioned/immutable, pulled, continuously reconciled. Body text paraphrases; consider footnoting.
- **Line ~29:** QUERY — Argo CD CNCF graduation Dec 2022: confirmed.
- **Line ~37:** QUERY — `argoproj.io/v1alpha1` current as of 2026: confirmed (no v1beta1 yet at time of writing).
- **Line ~46:** Path `k8s/overlays/production` here vs `deploy/k8s/overlays/production` elsewhere in chapter. Unify.
- **Line ~66:** QUERY — Flux CNCF graduation Nov 2022: confirmed.
- **Line ~91:** QUERY — ArgoCD default polling interval 3 minutes: confirmed.
- **Line ~91:** "image-tags.yaml" — non-canonical filename; image refs live in `kustomization.yaml`. Reconcile with later "kustomize edit set image" example.
- **Line ~111:** Path `k8s/overlays/production` and source `library/catalog` vs `library-system/catalog` used elsewhere. Unify.
- **Line ~123:** "Argo CD Image Updater" capitalization.
- **Line ~133:** No `staging` overlay built in this chapter; soften reference.
- **Line ~165:** "post-mortems" — CMOS 7.85 prefers closed compound "postmortems".
- **Line ~171:** `k8s/overlays/staging/` — staging not in chapter. Qualify.
- **Line ~175:** `k8s/overlays/production` vs `deploy/k8s/overlays/production` again. Unify.

## Pass 4: Final Polish
- Footnotes uncited inline.
