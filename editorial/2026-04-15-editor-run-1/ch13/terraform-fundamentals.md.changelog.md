# Changelog: terraform-fundamentals.md

## Pass 1: Structural / Developmental
- 2 structural notes. Progression of concepts is sound. Suggest adding a scope note (what's *not* covered: workspaces, imports, `moved` blocks).

## Pass 2: Line Editing
- **Line ~7:** "That is exactly what Infrastructure as Code tools do." — "exactly" is filler. Suggest: "That is what IaC tools do."
- **Line ~65:** "and only makes changes if something diverged" → "making changes only when something has diverged" (mild active).
- **Line ~99:** "The `.tfvars` files holding real secrets should be in `.gitignore`" → "belong in `.gitignore`" (more direct).
- **Line ~140:** "saves hundreds of lines of raw resource blocks and encodes years of community best practices" — acceptable; minor alternative "bakes in community best practices".
- **Line ~182:** "is just a directory" — drop filler "just".
- **Line ~206:** "almost everything you will do" → "the core of what you will do".

## Pass 3: Copy Editing
- **Line ~7:** QUERY — "Terraform is the most widely adopted" should acknowledge 2023 BSL license change and OpenTofu fork (footnote).
- **Line ~13:** "open-source IaC tool" — since 2023 Terraform is source-available (BSL), not OSI open source. Clarify or footnote.
- **Line ~42:** Confirmed `~> 5.0` means `>= 5.0, < 6.0`.
- **Line ~42:** QUERY — hashicorp/aws current major is 5.x; 6.x may be released. Confirm pin is still valid as of early 2026.
- **Line ~51:** `aws_vpc` arguments verified.
- **Line ~72:** `aws_caller_identity`, `aws_availability_zones` verified.
- **Line ~99:** "e.g.," with comma per CMOS 6.43 — OK.
- **Line ~126:** `version = "5.1.1"` here, `version = "~> 5.0"` in networking.md — pin-strategy inconsistent.
- **Line ~156:** QUERY — S3 backend native locking (Terraform 1.10, 2024) makes DynamoDB optional. Mention.
- **Line ~176:** "Section 13.7 revisits remote state" — production-overlay.md does not. Remove or redirect.
- **Line ~192:** `rds.tf # RDS PostgreSQL instance` singular; rds.md provisions three. Fix.
- **Line ~185:** Directory is `terraform/` here; `infrastructure/` in cicd.md. Unify.
- **Line ~228:** "not idempotent in the same sense as `kubectl apply`" — technically inaccurate. Both are idempotent; some Terraform resources trigger destructive replacement. Rephrase.
- **Line ~239:** "23 added" vs. deploying.md "47 added". Pick one canonical number or label as illustrative.
- **Line ~228:** Use en dash for ranges ("15–20 minutes") per CMOS 6.78; body uses "15 to 20". Either form acceptable; apply consistently.

## Pass 4: Final Polish
- **Footnotes:** `[^1]`–`[^5]` defined but not cited inline.
