# Changelog: rds.md

## Pass 1: Structural / Developmental
- 2 comments. Section is well structured. Flag: the claim about strategic merge patch env list behavior (line ~176) contradicts production-overlay.md (13.7) — resolve.

## Pass 2: Line Editing
- **Line ~29:** "keeps the code DRY" — jargon acronym; expand or remove.
- **Line ~150:** "There are two things that must change" → "Two things must change" (cut expletive).

## Pass 3: Copy Editing
- **Heading:** "13.4 Database: RDS for PostgreSQL" vs. index.md "13.4 — Amazon RDS". Unify.
- **Multiple headings:** "Why RDS Over StatefulSets in Production", "Terraform Configuration", "What Changed" — all title case; other sections use sentence case. Normalize.
- **Line ~14:** "tradeoffs" vs "trade-offs" — pick one form.
- **Line ~17:** QUERY — "60 to 120 seconds" failover, confirmed. Use en dash "60–120 seconds" per CMOS 6.78.
- **Line ~57:** QUERY — `engine_version = "16.4"` validity as of early 2026. Check current minor.
- **Line ~55:** `var.project_name` — not declared in terraform-fundamentals.md. Explain origin.
- **Line ~93:** "at least 7 days" — fine; index.md roadmap claims section covers automated backups, but this file disables them. Align narrative.
- **Line ~101:** "defined in `security-groups.tf`" contradicts networking.md which puts the SG in `vpc.tf`. Unify file placement.
- **Line ~120:** QUERY — "State is not encrypted by default" — true for local state; S3 backend typically sets `encrypt = true`. Clarify.
- **Line ~155:** QUERY — RDS default `rds.force_ssl` behavior; confirm TLS enforced by default on PG 15+.
- **Line ~155:** Stronger `sslmode=verify-full` is worth mentioning.
- **Line ~173:** URL form here vs. `host=... port=...` form in production-overlay.md. Pick one canonical DATABASE_URL format.
- **Line ~176:** FACTUAL CONFLICT — "Only `DATABASE_URL` is overridden" vs. production-overlay.md's "must include the `envFrom` block, or ConfigMap reference is lost". Kustomize env list merge by name should merge individual items but not envFrom. Verify with actual kustomize build and align both sections.
- **Line ~218:** `master_user_secret` attribute verified as list(object).
- **Line ~120:** "[^3]" is cited inline — good pattern; most other files don't cite footnotes inline.

## Pass 4: Final Polish
- `[^1]` defined but not cited inline.
