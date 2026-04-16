# Findings: Chapter 14

---

## index.md

### Summary
Reviewed ~130 lines. 0 structural, 1 line edit, 1 copy edit. 0 factual queries.

### Line Edits
- **L58:** "$0.50 per month, which includes standard query volumes typical of this project" — awkward. Route 53 charges $0.50/hosted zone plus per-query. → "$0.50 per month for the hosted zone; query charges are negligible at this traffic volume."

### Copy Edit & Polish
- **L121:** "the Go Kafka library" — vague. → "the Go Kafka client library."

---

## dns.md

### Summary
Reviewed ~200 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L200:** "`.com` domains cost around $12/year through Route 53" — Route 53 `.com` registration is $13/year as of 2025. → "around $13/year."

---

## tls.md

### Summary
Reviewed ~180 lines. 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L6:** "roughly 30 days before the current certificate's expiration" → "up to 60 days before the current certificate's expiration" — AWS documentation states renewal begins up to 60 days before expiry.
- **L98:** "Section 14.1 added the Route 53 alias annotation" — section 14.1 added the DNS record, not an annotation. The Ingress annotations were in section 13.7. → "Section 14.1 added the Route 53 DNS record."

---

## secrets.md

### Summary
Reviewed ~350 lines. 1 structural, 0 line edits, 0 copy edits. 1 factual query.

### Structural
- **L346:** "see `secret-store.yaml` for the second SecretStore definition" — but the listing at L198–222 only shows one SecretStore (in the `library` namespace). The second for the `data` namespace is referenced but never shown. Either add it or note the reader should create a duplicate.

### Factual Queries
- **L79:** IRSA module version `~> 5.34` — section 13.6 uses `~> 5.39` for the same module. The constraints are compatible, but the inconsistency is confusing. Unify.

---

## msk-tls.md

### Summary
Reviewed ~220 lines. 0 structural, 1 line edit, 0 copy edits. 1 factual query.

### Line Edits
- **L84:** "whitelist the source security group" → "allowlist the source security group" — "whitelist" is increasingly deprecated in technical writing.

### Factual Queries
- **L216:** "The library system's Dockerfiles already copy the CA bundle from Alpine" — verify this matches the Dockerfiles in earlier chapters. If the final stage uses `distroless` rather than Alpine, the CA bundle source differs.

---

## applying.md

### Summary
Reviewed ~350 lines. 0 structural, 0 line edits, 2 copy edits. 1 factual query.

### Copy Edit & Polish
- **L87:** "the `aws_rds_cluster` resource" → "the `aws_db_instance` resource" — section 13.4 defines standard RDS, not Aurora.
- **L349:** "Chapter 15 addresses this" — verify that Chapter 15 exists in the manuscript plan. SUMMARY.md ends at Chapter 14.

### Factual Queries
- **L55:** "with `TLS_PLAINTEXT` replaced by `TLS`" — section 13.5 uses `PLAINTEXT`, not `TLS_PLAINTEXT`. → "with `PLAINTEXT` replaced by `TLS`."
