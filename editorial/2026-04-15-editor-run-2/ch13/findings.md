# Findings: Chapter 13

**Global issues for this chapter:**
- Filler preamble "A few details/things/decisions are worth noting/explaining" — delete and proceed directly (5 instances).
- Heading capitalization inconsistent (title case vs sentence case). Standardize to sentence case.
- Variable naming: `var.region` (section 13.1) vs `var.aws_region` (sections 13.2, 13.6, 13.8). Unify.
- `aws_rds_cluster` vs `aws_db_instance` confusion in deploying.md. Section 13.4 defines `aws_db_instance` but later sections reference Aurora/`aws_rds_cluster`. Correct all to `aws_db_instance`.

---

## index.md

### Summary
Reviewed ~135 lines. 0 structural, 2 line edits, 0 copy edits. 0 factual queries.

### Line Edits
- **L46:** "A few details are worth noting." → delete, proceed directly to "Meilisearch remains..."
- **L3:** "which is the right tool" — relative pronoun "which" ambiguous (could refer to control plane or kind). Split into two sentences.

---

## terraform-fundamentals.md

### Summary
Reviewed ~230 lines. 0 structural, 0 line edits, 1 copy edit. 1 factual query.

### Copy Edit & Polish
- **L8:** "That is what IaC tools do." → "That is what Infrastructure as Code (IaC) tools do." — expand abbreviation on first use in this section.

### Factual Queries
- **L228:** "RDS instance" (singular) but the chapter provisions multiple RDS instances. → "RDS instances."

---

## networking.md

### Summary
Reviewed ~205 lines. 0 structural, 1 line edit, 0 copy edits. 1 factual query.

### Line Edits
- **L59:** "A few details are worth noting." → delete.

### Factual Queries
- **L155–161:** Security groups reference `module.eks.node_security_group_id` but EKS module is not defined until section 13.6. These resources cannot be applied until `eks.tf` is in place. Add a note.

---

## ecr.md

### Summary
Reviewed ~100 lines. 0 structural, 1 line edit, 1 copy edit. 1 factual query.

### Line Edits
- **L83:** "A few things are worth noting in this configuration." → delete or replace: "Several configuration choices deserve explanation."

### Copy Edit & Polish
- **L95:** "Image Tagging Strategy" → "Image tagging strategy" — heading capitalization inconsistency.

### Factual Queries
- **L89:** "built-in vulnerability scanning using Amazon Inspector" — `scan_on_push = true` alone uses basic scanning (Clair), not Inspector. Enhanced scanning requires separate configuration. Verify or correct.

---

## rds.md

### Summary
Reviewed ~260 lines. 1 structural, 0 line edits, 2 copy edits. 1 factual query.

### Structural
- **L101:** "The `rds` security group (defined in `security-groups.tf`)" — earlier in the chapter (section 13.2), security groups are defined in `vpc.tf`. File name inconsistency. → "defined in `vpc.tf`."

### Copy Edit & Polish
- **L9:** "Why RDS Over StatefulSets in Production" → "Why RDS over StatefulSets in production" — heading case.
- **L254:** "What Changed" → "What changed."

### Factual Queries
- **L122:** "State is not encrypted by default" — S3 server-side encryption is enabled by default for all new buckets since January 2023. → "State may be accessible to anyone with S3 read permissions on the bucket."

---

## msk.md

### Summary
Reviewed ~230 lines. 1 structural, 1 line edit, 1 copy edit. 1 factual query.

### Structural
- **L114–149:** MSK security group appears in both `networking.md` and `msk.md` with different source references. Clarify which file owns the security group and remove the duplicate.

### Line Edits
- **L98–99:** "A few decisions in this configuration are deliberate and worth noting." → delete.

### Copy Edit & Polish
- **L227:** "What Changes and What Does Not" → "What changes and what does not."

### Factual Queries
- **L47:** `aws_msk_topic` — no official Terraform resource by this name exists in the AWS provider. Topic management is typically done via Kafka admin APIs. Verify or correct.

---

## eks.md

### Summary
Reviewed ~390 lines. 0 structural, 2 line edits, 0 copy edits. 1 factual query.

### Line Edits
- **L117:** "A few decisions are worth explaining." → delete.
- **L387:** "The next section covers the ECR registries" — incorrect forward reference. ECR was section 13.3; the next section is 13.7 (Production Kustomize Overlay). → "The next section writes the production Kustomize overlay that wires these AWS resources into the Kubernetes manifests."

### Factual Queries
- **L120:** `cluster_version = "1.29"` — EKS 1.29 GA January 2024, still supported in 2026 but 1.30/1.31 also available. Consider noting readers should check current supported versions.

---

## production-overlay.md

### Summary
Reviewed ~450 lines. 0 structural, 0 line edits, 1 copy edit. 1 factual query.

### Copy Edit & Polish
- **L432:** "Replace the placeholder ARN with the certificate provisioned by Terraform in section 13.1." — the certificate is provisioned in section 14.2, not 13.1. → "in section 14.2."

### Factual Queries
- **L443:** "default StorageClass is backed by... `gp2` EBS volumes" — EKS 1.28+ may default to `gp3` in some configurations. Verify.

---

## cicd.md

### Summary
Reviewed ~385 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L381:** PR workflow reads from `'terraform/plan.txt'` but the plan step writes with `working-directory: infrastructure`. Path inconsistency. → `'infrastructure/plan.txt'`.

---

## deploying.md

### Summary
Reviewed ~430 lines. 2 structural, 0 line edits, 0 copy edits. 2 factual queries.

### Structural
- **L4–5:** "a Multi-AZ RDS Aurora cluster" — section 13.4 provisions `aws_db_instance` (standard RDS), not Aurora. → "RDS instances."
- **L87:** "Terraform created the Secrets Manager entries for the RDS credentials in section 14.3" — this is section 13.9; referencing section 14.3 (not yet reached) is a forward reference that confuses the deployment sequence.

### Factual Queries
- **L377–380:** Teardown uses `aws rds describe-db-clusters` — should be `aws rds describe-db-instances` (standard RDS, not Aurora). Also `DBClusters[*].DBClusterIdentifier` → `DBInstances[*].DBInstanceIdentifier`.
- **L425:** "What's Next" — heading uses contraction; others do not. → "What comes next" or "Next steps."

---

## argocd.md

### Summary
Reviewed ~190 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L183:** Weaveworks blog URL (`weave.works`) — Weaveworks ceased operations in 2024. The URL may no longer resolve. Consider using a Wayback Machine archive link.
