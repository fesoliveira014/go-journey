# Changelog: networking.md

## Pass 1: Structural / Developmental
- 2 comments. Section structure (topology → code → SG → summary) is excellent. Main issue: the MSK security group is also defined in msk.md — pick one home and cross-reference.

## Pass 2: Line Editing
- **Line ~3:** Long sentence can be trimmed: "need subnets whose routing prevents direct reachability from the internet".
- **Line ~15:** "it is itself a managed AWS resource" → "itself a managed AWS resource".
- **Line ~59:** "A few details worth noting." — sentence fragment; add "are".
- **Line ~119:** "constrains it to one, shared NAT Gateway" — unnecessary comma. → "one shared NAT Gateway".

## Pass 3: Copy Editing
- **Heading:** "13.2 — VPC and Networking" — em dash in heading inconsistent with sibling sections. CMOS 6.85.
- **Line ~11:** "Availability Zones" — AWS capitalizes; subsequent prose uses "availability zone" lowercase. Unify (prefer AWS form).
- **Line ~11:** QUERY — AWS defines an AZ as "one or more discrete data centers" (not strictly one). Soften.
- **Line ~15:** "Internet Gateway" — capitalized (AWS product); "the internet" lowercase (CMOS 8.190). OK, but verify consistency.
- **Line ~59:** QUERY — "~$65/month each" for NAT Gateway contradicts index.md's $33/month. 730h × $0.045 ≈ $33. Fix.
- **Line ~113:** `var.aws_region` here; `var.region` in terraform-fundamentals.md. Unify.
- **Line ~136:** QUERY — The `kubernetes.io/cluster/...` tag: EKS managed clusters since 2020 do not strictly require it, but AWS LB Controller does discover via it. Clarify.
- **Line ~144:** DUPLICATE RESOURCE: `aws_security_group.msk` declared here AND in msk.md — Terraform will fail. Remove one.
- **Line ~158:** Both files use different upstream SG references (`module.eks.node_security_group_id` here; `aws_security_group.eks_nodes.id` in msk.md). Unify.
- **Line ~144:** Circular dependency risk: `aws_security_group.msk` in `vpc.tf` references `module.eks.node_security_group_id`. Worth discussion or restructuring via `aws_security_group_rule`.
- **Line ~209:** "mutual TLS is configured" — MSK's TLS mode is server-auth, not mTLS by default. Adjust.
- **Line ~220:** QUERY — route table count from terraform-aws-modules/vpc. Verify "4" is accurate.
- **Line ~227:** Directory `infra/terraform` — inconsistent with `terraform/` and `infrastructure/` elsewhere.

## Pass 4: Final Polish
- Footnotes uncited inline.
