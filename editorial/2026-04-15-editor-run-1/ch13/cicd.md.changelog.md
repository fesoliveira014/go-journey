# Changelog: cicd.md

## Pass 1: Structural / Developmental
- 2 comments. Strong OIDC explainer with sequence diagram. The commented-out PR workflow (~50 lines) is hard to read; consider an appendix or a GitHub link.

## Pass 2: Line Editing
- **Line ~5:** "A failed rollout rolls back automatically" — overclaim. `kubectl rollout status --timeout` fails the job but does not auto-rollback. Fix.
- **Line ~37:** "useless before an attacker can act on it" — soften to "narrow exploitation window".
- **Line ~431:** "deployed by a pipeline, with no credentials stored anywhere that do not expire" — double negation; simplify to "that stores no long-lived credentials".

## Pass 3: Copy Editing
- **Heading:** "13.8 CI/CD Pipeline" vs index.md "13.8 — Continuous Deployment with GitHub Actions". Unify.
- **Line ~13:** QUERY — GitHub Actions OIDC launched Oct 2021 GA; confirmed.
- **Line ~37:** QUERY — STS temp credentials TTL default. aws-actions/configure-aws-credentials defaults to 1 hour, not 15 minutes. Correct.
- **Line ~61:** QUERY — `thumbprint_list` requirement for well-known OIDC providers: AWS relaxed the requirement in 2023. Worth note.
- **Line ~61:** QUERY — `data "tls_certificate"` returning `[0].sha1_fingerprint` — leaf vs root cert. Verify current behavior.
- **Line ~132:** QUERY — `aws_eks_access_entry` and `aws_eks_access_policy_association` — confirmed valid.
- **Line ~142:** QUERY — `AmazonEKSEditPolicy` ARN format — confirmed.
- **Line ~159:** QUERY — EKS access entries API released ~Dec 2023.
- **Line ~164:** Directory `infrastructure` inconsistent with `terraform/`, `infra/terraform`.
- **Line ~165:** `-target` discouraged; note this is limited bootstrap use.
- **Line ~289:** `kustomize edit set image "library/${SERVICE}=..."` source name inconsistent with production-overlay.md's `library-system/${SERVICE}`.
- **Line ~289:** `--name library-cluster` inconsistent with deploying.md's `library-production` and eks.md's `local.cluster_name`.
- **Line ~279:** QUERY — `imranismail/setup-kustomize@v2` currency; consider the official kustomize release action.
- **Line ~250:** QUERY — ECR auth token TTL 12 hours: confirmed.
- **Line ~283:** ECR path `ECR_REGISTRY/library/${SERVICE}` vs ecr.md's `library-system/${SERVICE}`. Unify.
- **Line ~380:** Commented PR workflow's `fs.readFileSync('terraform/plan.txt')` conflicts with `working-directory: infrastructure`.
- **Line ~395:** QUERY — `revisionHistoryLimit` default 10 — confirmed.

## Pass 4: Final Polish
- Footnotes uncited inline.
