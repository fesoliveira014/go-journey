# Changelog: eks.md

## Pass 1: Structural / Developmental
- 1 comment. Section is extensive; system-pod verification partially overlaps with deploying.md (13.9). Suggest delegating that to 13.9.

## Pass 2: Line Editing
- **Line ~5:** "your actual workloads" — drop "actual".
- Minor: several comments on wordy phrasing; most prose is already tight.

## Pass 3: Copy Editing
- **Heading:** "13.6 Kubernetes Cluster: EKS" vs index.md "13.6 — EKS Cluster". Unify.
- **Line ~3:** "the previous section" refers to 13.5 (MSK); VPC is 13.2. Clarify.
- **Line ~3:** "NAT Gateways" plural — networking.md uses one. Fix.
- **Line ~48:** EKS Pod Identity (the 2023 feature) not mentioned; worth footnote.
- **Line ~54:** "AWS maintains an official Terraform module" — inaccurate. `terraform-aws-modules/eks` is community-maintained. Correct.
- **Line ~110:** "EKS-optimised AMI" — UK spelling; use "optimized" for US consistency.
- **Line ~110:** QUERY — Amazon Linux 2 vs 2023 for K8s 1.29 default. AL2 default through 1.29; AL2023 is current default for newer.
- **Line ~110:** `local.cluster_name`, `local.common_tags` used but not shown declared.
- **Line ~120:** QUERY — "EKS supports roughly four versions" — confirm current AWS support matrix (standard + extended).
- **Line ~120:** Kubernetes 1.29 may be in extended support as of early 2026. Consider bumping or noting version currency.
- **Line ~133:** QUERY — `AmazonEKS_CNI_Policy` attached to node role vs IRSA. Both work; terraform-aws-modules/eks attaches to node role. Note direction.
- **Line ~140:** "signed JWT token" — "JWT token" is redundant (RAS-syndrome).
- **Line ~190:** "EKS Pod Identity webhook" — ambiguous. IRSA uses `pod-identity-webhook` (amazon-eks-pod-identity-webhook). 2023's EKS Pod Identity is a different non-OIDC mechanism. Clarify.
- **Line ~192:** "finalising" — UK; use "finalizing".
- **Line ~200:** `kubernetes.io/ingress.class: alb` deprecated; use `ingressClassName`.
- **Line ~212:** QUERY — aws-load-balancer-controller Helm chart version 1.7.2 (April 2024). As of early 2026 likely newer. Verify currency. Also note chart version vs controller image version differ.
- **Line ~212:** QUERY — Helm chart `set` arguments `clusterName`, `region`, `vpcId`, `serviceAccount.*` match current chart values.yaml. Verify.
- **Line ~257:** "admission controller running inside the API server" — IRSA webhook is a separate Deployment. Clarify.
- **Line ~268:** `providers.tf` here vs `main.tf`/`backend.tf` elsewhere. Unify filename.
- **Line ~289:** QUERY — `aws eks get-token` token TTL is 14 minutes (not 15). Verify.
- **Line ~322:** "should be in an encrypted S3 bucket" contradicts earlier "leave backend commented out".
- **Line ~385:** "The next section covers the ECR registries" — incorrect forward reference. ECR was 13.3; next is 13.7.

## Pass 4: Final Polish
- Footnotes uncited inline.
