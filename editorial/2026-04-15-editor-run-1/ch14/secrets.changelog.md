# Changelog: secrets.md

## Pass 1: Structural / Developmental
- 6 comments. Themes:
  - Strong opening (quoting the Ch. 13 placeholder comment) — establishes the gap concretely.
  - Heading case inconsistent vs dns.md (sentence case) / tls.md (Title Case).
  - Five ExternalSecret YAML blocks in series is heavy for the reader; consider collapsing to 1–2 worked examples plus a reference table for the remaining entries.
  - SecretStore manifest section shows only the `library`-namespace SecretStore; later text notes a second SecretStore in the `data` namespace is also needed. Show both.
  - Sample `kubectl get externalsecrets -n library` output lists `meilisearch-secret` in the `library` namespace, contradicting the `data`-namespace placement.

## Pass 2: Line Editing
- **Line ~14:** Consider splitting 46-word sentence describing the manual process into two sentences for pacing.
- **Line ~25:** Tighten.
  - Before: "at least one value gets wrong"
  - After: "at least one value is wrong"
- **Line ~28:** Punctuation option.
  - Before: "Infrastructure-as-code exists to make system state reproducible without human memory."
  - After: "Infrastructure as code exists to make system state reproducible without human memory."
  - Reason: CMOS 7.85 — hyphenate only when used as adjective; here the phrase is the subject (noun).

## Pass 3: Copy Editing
- **Line ~1:** Heading: "14.3 Secrets Management with External Secrets Operator" — no em dash while dns.md uses em dash. Unify.
- **Line ~21:** QUERY — "That file is typically world-readable on developer laptops" — shell history files default to mode 0600 (owner only). Soften to "readable by any process running as your user".
- **Line ~35:** "GCP Secret Manager" → "Google Secret Manager" (Google dropped "GCP" branding).
- **Line ~35:** "kubectl command" → "`kubectl` command" for code-style consistency.
- **Line ~62:** "IRSA" — on first use in this file, consider expanding: "IAM Roles for Service Accounts (IRSA)".
- **Line ~66:** QUERY — Verify naming pattern `rds!db-<identifier>` for RDS-managed secrets on DB instances (vs clusters). AWS may use different patterns.
- **Line ~113:** QUERY — ESO Helm chart version `0.9.13`. As of April 2026 the chart is at v0.10+. Consider a pin update or a note to consult upstream for current version.
- **Line ~138:** Note: `terraform apply -target=` is officially discouraged for normal workflows per HashiCorp docs. Consider adding a one-line caveat that this is bootstrapping, not routine.
- **Line ~176:** QUERY — Meilisearch master-key behavior: docs state changing the master key invalidates existing API keys but does not require re-indexing. Verify and clarify "cannot be changed after the index is populated without a full re-index".
- **Line ~202:** QUERY — ESO API version: `external-secrets.io/v1beta1`. Since ESO 0.10+ the stable group is `v1`. Update or footnote.
- **Line ~346:** "the meilisearch ExternalSecret" → "the Meilisearch ExternalSecret" (product name capitalization).
- **Line ~346:** QUERY — Align example. SecretStore block shows only a `library`-namespace SecretStore but referring text says "see `secret-store.yaml` for the second SecretStore definition". Add the `data`-namespace SecretStore to the YAML example.
- **Line ~350:** QUERY — `creationPolicy: Merge` behavior: verify that it requires a pre-existing Secret.
- **Line ~418:** QUERY — sample `kubectl get externalsecrets -n library` output includes `meilisearch-secret`, but manifest places it in `data` namespace. Fix inconsistency.
- **Line ~440:** Same inconsistency in `kubectl get secrets -n library` sample (meilisearch-secret listed).

## Pass 4: Final Polish
- **Line ~517:** Footnotes [^3] (RDS managed passwords) and [^4] (EKS IRSA) are defined but never referenced in body. Add in-body references or remove.
- Cosmetic: `kubectl get externalsecrets` sample output has inconsistent column spacing on the `postgres-reservation-secret` row (two spaces vs three for REFRESH INTERVAL).
- No doubled words or broken cross-refs detected in prose.
- "re-deployments" / "re-apply" / "Re-deploy" — mixed hyphenation. CMOS 7.89 permits either; unify chapter-wide.
