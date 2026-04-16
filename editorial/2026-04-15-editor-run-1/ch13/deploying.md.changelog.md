# Changelog: deploying.md

## Pass 1: Structural / Developmental
- 1 comment. The "Push Images to ECR" subsection duplicates cicd.md's image promotion. Decide whether 13.9 is "first manual apply" or "after CI runs". Currently mixed.

## Pass 2: Line Editing
- Few line edits beyond the major copy-edit issues.

## Pass 3: Copy Editing
- **Heading:** "13.9 Deploying and Verifying" matches index.md but uses different dash. Unify.
- **Multiple headings** title cased: "Provision Infrastructure with Terraform", "Verify AWS Resources", "Retrieve RDS Credentials", "Verification Checklist", "Troubleshooting Guide", "Expected Outputs for Non-Deployers", "What's Next". Normalize.
- **Line ~5:** "Multi-AZ RDS Aurora cluster" — chapter uses single-AZ db.t3.micro `aws_db_instance`, not Aurora. Multiple references through file. Fix throughout.
- **Line ~5:** "RDS cluster" wording confuses Aurora cluster vs RDS instances. Use "RDS instances".
- **Line ~21:** "initializes the S3 backend for remote state" — terraform-fundamentals.md leaves backend commented out. Contradiction.
- **Line ~21:** `versions.tf` — chapter uses `main.tf`. Unify.
- **Line ~13:** `terraform/` directory — inconsistent with `infrastructure/` and `infra/terraform`.
- **Line ~44:** "47 added" vs terraform-fundamentals.md's "23 added". Unify.
- **Line ~52-58:** RDS endpoint prefix `library-system-*` here vs rds.md's `library-*`. Unify.
- **Line ~73:** Cluster name `library-production` vs `library-cluster` (index.md, cicd.md) vs `local.cluster_name` (eks.md). Unify across all files.
- **Line ~103:** ECR repo names `library/<svc>` vs ecr.md's `library-system/<svc>`. Unify.
- **Line ~133:** `--secret-id library/rds/master` — chapter uses `manage_master_user_password = true`, generating `rds!db-...` secret name automatically. Friendly name not configured. Fix.
- **Line ~145:** Aurora-style endpoint `library-cluster.cluster-xxxx.rds.amazonaws.com` vs RDS instance `<id>.<random>.<region>.rds.amazonaws.com`. Fix.
- **Line ~143:** `"username": "library_master"` vs rds.md's `username = "postgres"`. Unify.
- **Line ~151:** `secrets.env` not used by production-overlay.md (which uses `secretGenerator` literals). Inconsistent flow.
- **Line ~163:** `terraform output -raw ecr_registry` — no such output declared in ecr.md.
- **Line ~182:** ECR push path `library/<svc>` vs ecr.md's `library-system/<svc>`. Unify.
- **Line ~182:** `latest` tag only — conflicts with sha-tag practice.
- **Line ~189:** "the Go standard library" — Go stdlib is statically linked, not a layer. Fix.
- **Line ~200:** "uses `StorageClass: gp3`" — production-overlay.md treats gp3 as optional; default is gp2.
- **Line ~204-223:** Resource names mismatch production-overlay.md (namespace `infra`, configmap `library-config`, secret `library-secrets`, ingress `gateway`). Unify.
- **Line ~258:** Ingress name `gateway` vs production-overlay.md's `library-ingress`.
- **Line ~310:** `terraform output msk_bootstrap` — msk.md exports `msk_bootstrap_brokers_plaintext` / `msk_bootstrap_brokers_tls`. Fix.
- **Troubleshooting table line ~319:** "port 9094 (MSK TLS)" — chapter uses 9092 plaintext. Fix.
- **Troubleshooting table line ~323:** `msk_bootstrap_brokers_tls` — chapter uses plaintext. Fix.
- **Troubleshooting table line ~323:** `library-config` ConfigMap not used in 13.7. Fix.
- **Line ~378:** `aws rds describe-db-clusters` queries Aurora; for `aws_db_instance` use `describe-db-instances`. Fix.
- **Line ~404:** EBS price $0.08/GB-mo — gp3 price; gp2 is $0.10. Specify or pick.
- **Line ~298:** ISBN 978-0134190440 verified.

## Pass 4: Final Polish
- Footnotes ARE cited inline. Good.
- "What's Next" smart-quote: verify HTML build preserves it.
