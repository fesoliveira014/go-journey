# Changelog: applying.md

## Pass 1: Structural / Developmental
- 3 comments. Themes:
  - "Dependency Order" section is the right gate before the command sequence.
  - "Rollback" as a dedicated section is a standout; preserves reader confidence during live deploys.
  - "What's Next" needs chapter-number correction (closes with "Chapter 14 addresses this" while this IS Chapter 14).

## Pass 2: Line Editing
- **Line ~59:** Split 59-word sentence in "Review the plan output" paragraph for pacing.
- Minor tightening opportunities throughout; otherwise prose is already tight.

## Pass 3: Copy Editing
- **Line ~1:** Heading "14.5 Applying the Changes" — no em dash; 14.1 uses em dash. Unify chapter-wide.
- **Line ~3:** "Sections 13.1 through 13.4" — likely a chapter-number typo; should be "14.1 through 14.4".
- **Line ~29:** Add `hcl` language hint to tfvars code block (optional).
- **Line ~46:** "sizeable" → "sizable" (CMOS 7.88).
- **Line ~55:** QUERY — "aws_msk_configuration with `TLS_PLAINTEXT` replaced by `TLS`" — section 14.4 specified `PLAINTEXT` (not `TLS_PLAINTEXT`) as the Ch. 13 state. Align.
- **Line ~72:** Output-name drift — tls.md uses `certificate_arn`; here `acm_certificate_arn`. msk-tls.md uses `msk_bootstrap_brokers_tls`; here `msk_tls_bootstrap`. Unify across files.
- **Line ~86:** QUERY — "`aws_rds_cluster` resource" — Ch. 13 repo uses `aws_db_instance` for PostgreSQL. Correct resource name or clarify.
- **Line ~91:** "Meilisearch API key" → consistent with secrets.md "master key". Unify to "master key".
- **Line ~99:** QUERY — JWT secret length mismatch. secrets.md recommends `openssl rand -hex 64` (128 hex chars); here `openssl rand -hex 32` (64 hex chars). Align.
- **Line ~253:** QUERY — ESO behavior on ExternalSecret deletion. With `creationPolicy: Owner`, owner ref triggers GC of the Secret when the ExternalSecret is deleted (per secrets.md). Phrasing here ("secrets are not removed on operator removal") may conflict. Reconcile.
- **Line ~287:** QUERY — `kubectl patch configmap library-config` — there is no `library-config` ConfigMap per msk-tls.md and step 4. The names are `catalog-config`, `reservation-config`, `search-config`. Fix.
- **Line ~295:** QUERY — `aws_acm_certificate_validation` default timeout value. Confirm current Terraform AWS provider default.
- **Line ~328:** QUERY — resource counts ("roughly 57 instead of 47"). Spot-check against actual state.

## Pass 4: Final Polish
- **Line ~3:** "Sections 13.1 through 13.4" — chapter-number typo (should be 14.1–14.4).
- **Line ~76:** `acm_certificate_arn` in output sample vs `certificate_arn` in tls.md. Unify.
- **Line ~76:** `msk_tls_bootstrap` here vs `msk_bootstrap_brokers_tls` in msk-tls.md. Unify.
- **Line ~162:** Sample `kubectl get externalsecrets -n library` places meilisearch in `data` namespace (self-consistent with spec). But secrets.md sample had it in `library`. Fix secrets.md to match this file.
- **Line ~252:** "MSK_BROKER" placeholder — consistent with prior sections. OK.
- **Line ~287:** `kubectl patch configmap library-config` — wrong ConfigMap name. Fix.
- **Line ~348:** "Chapter 14 addresses this" in the "What's Next" paragraph — should be "Chapter 15".
- Footnotes [^1]–[^4] are defined but none appear to be referenced in the body text. Add in-body markers or remove.
- No typos or doubled words detected otherwise.
