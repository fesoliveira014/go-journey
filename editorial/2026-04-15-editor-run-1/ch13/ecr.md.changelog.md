# Changelog: ecr.md

## Pass 1: Structural / Developmental
- 1 comment. Clean section; suggest adding a short cost note (storage/transfer pricing) consistent with chapter's cost-awareness theme.

## Pass 2: Line Editing
- **Line ~5:** "path of least resistance" — cliché. Suggest "simplest integration".
- **Line ~83:** "A few things worth noting" — fragment; add "are".

## Pass 3: Copy Editing
- **Heading:** "13.3 ECR — Container Registry for EKS" conflicts with index.md roadmap entry "13.3 — Amazon ECR and the Build Pipeline". Unify.
- **Line ~15:** QUERY — ECR authorization token TTL is 12 hours. Confirmed.
- **Line ~15:** "twelve hours" vs. "12 hours" — CMOS 9.3 allows either; prefer numerals with units.
- **Line ~46:** `aws_ecr_lifecycle_policy` JSON structure verified.
- **Line ~89:** QUERY — `scan_on_push = true` triggers *basic* scanning (Clair-based), not Amazon Inspector. Enhanced scanning (Inspector) requires registry-level configuration. Correction needed. Also affects the "free, zero-configuration" line.
- **Line ~21:** Heading "ecr.tf" (lowercase file name as section heading) inconsistent with other sentence-case headings.
- **Line ~95:** "Image Tagging Strategy" title-case; other headings sentence case. Pick one.
- **Line ~106:** "GitOps repository" — forward references 13.10. Use "Git repository" or define GitOps earlier.
- **Line ~111:** Kustomize `name: catalog` here; production-overlay.md uses `library-system/catalog`. Inconsistent source-name matching.
- **Line ~116:** "committing the change to trigger a GitOps reconciliation" describes 13.10 flow, not 13.8 push pipeline. Fix.
- **Line ~125:** Output name `ecr_repository_urls` (map); deploying.md references `ecr_registry` (scalar). Reconcile.
- **Line ~3:** "GHCR is the right choice for a project hosted on GitHub" — OK but technically debatable with Docker Hub/Quay. Keep.

## Pass 4: Final Polish
- Footnotes use markdown-link style; other files use bare URLs. Unify.
- Footnotes `[^3]` and `[^4]` defined but not cited inline.
