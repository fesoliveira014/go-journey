# Changelog: production-overlay.md

## Pass 1: Structural / Developmental
- 2 comments. The "Restructuring the base" subsection is a mid-chapter base refactor — consider moving to a Chapter 12 addendum.

## Pass 2: Line Editing
- **Line ~11:** "There is one structural change to make before writing the overlay" → "One structural change comes first".
- **Line ~106:** "simply do not exist" — cut "simply".

## Pass 3: Copy Editing
- **Line ~5:** "a working CI pipeline" — 13.7 precedes cicd.md (13.8). Forward reference.
- **Line ~11:** "Postgres" informal; chapter uses "PostgreSQL". Normalize.
- **Line ~85:** local-infra's `data` / `messaging` subdirs share names with base's `data/` and `messaging/`. Avoid collisions.
- **Line ~130+:** `newTag: latest` conflicts with sha-tag practice established in ecr.md / cicd.md. Use placeholder CI replaces.
- **Line ~130+:** Image name `library-system/<svc>` vs. ecr.md's `catalog` short name. Unify.
- **Line ~200:** JSON6902 `containers/0/imagePullPolicy` brittle for multi-container pods. Flag.
- **Line ~249:** QUERY — Kustomize strategic merge patch with `target` selector: does `placeholder` container name work? Containers in strategic merge match by `name`; patching with a nonexistent name appends rather than replaces. Verify by running `kustomize build` and revise if broken.
- **Line ~267:** FACTUAL CONFLICT with rds.md (13.4) — rds.md claims full merge ("only DATABASE_URL is overridden"); this file says envFrom must be re-declared. Verify actual Kustomize behavior; unify.
- **Line ~288:** DATABASE_URL libpq keyword=value format here vs. URL format in rds.md. Pick one.
- **Line ~288:** RDS hostname format `auth-db.xxxxxxxxxxxx...` here vs. `library-catalog.xxxx...` in rds.md. Normalize.
- **Line ~361:** QUERY — `$(VAR)` env substitution order-dependent: confirmed.
- **Line ~375:** MSK hostname format `b-1.library-msk.xxxxxxxx.c1.kafka...` here vs. `b-1.library.abc123.c2.kafka...` in msk.md. Normalize.
- **Line ~418:** Ingress has both deprecated `kubernetes.io/ingress.class: alb` annotation AND modern `spec.ingressClassName: alb`. Remove the annotation.
- **Line ~421:** QUERY — `alb.ingress.kubernetes.io/certificate-arn` annotation: confirmed valid.
- **Line ~421:** ACM certificate not provisioned in this chapter; index.md says Chapter 14 adds it. Flag forward dependency.
- **Line ~432:** "provisioned by Terraform in section 13.1" — 13.1 does NOT provision ACM. Fix.
- **Line ~433:** QUERY — `ssl-redirect` produces 301 permanent redirect. Verify against AWS LB Controller docs.
- **Line ~443:** QUERY — EKS default StorageClass on 1.29: gp2 (in-tree) vs ebs.csi.aws.com (default in newer). Clarify.
- **Line ~479:** "remove the default annotation from the existing gp2 StorageClass" — mechanism not shown (Kustomize can't patch a resource it doesn't own). Explain.

## Pass 4: Final Polish
- Footnotes uncited inline.
- AWS LB Controller URL pinned to v2.7; prefer /latest/.
