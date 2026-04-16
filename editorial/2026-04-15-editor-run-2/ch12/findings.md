# Findings: Chapter 12

---

## index.md

### Summary
Reviewed ~230 lines. 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L4:** "AKE" → "AKS" — Azure Kubernetes Service is "AKS," not "AKE."
- **L91:** "the multi-container pattern (the sidecar pattern)" → "multi-container patterns (such as the sidecar pattern)" — sidecar is one variant; not equivalent.

---

## kind-setup.md

### Summary
Reviewed ~200 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L4:** "AKE" → "AKS" — same fix.

---

## preparing-services.md

### Summary
Reviewed ~300 lines. 0 structural, 0 line edits, 2 copy edits. 0 factual queries.

### Copy Edit & Polish
- **L162:** "initialises" → "initializes" — British/American inconsistency.
- **L229:** "Since Kubernetes 1.24 it also" → "Since Kubernetes 1.24, it also" — comma after introductory phrase.

---

## app-manifests.md

### Summary
Reviewed ~760 lines. 1 structural, 0 line edits, 0 copy edits. 1 factual query.

### Structural
- **L1–4:** "the infrastructure layer declared in the previous section" — infrastructure manifests are in section 12.4 (after this section 12.3). → "declared in the next section."

### Factual Queries
- **L325–374:** Auth Deployment omits `securityContext` (both pod-level and container-level), unlike the catalog Deployment which includes it. Either add security contexts to all services in the base manifests or note the omission.

---

## infra-manifests.md

### Summary
Reviewed ~385 lines. 0 structural, 0 line edits, 0 copy edits. 2 factual queries.

### Factual Queries
- **L255:** `apache/kafka:3.9` — plausible for 2026 but verify at publication.
- **L329:** `getmeili/meilisearch:v1.12` — plausible. Verify at publication.

---

## kustomize.md

### Summary
Reviewed ~295 lines. 0 structural, 0 line edits, 1 copy edit. 0 factual queries.

### Copy Edit & Polish
- **L290:** "JSON 6902 patches" → "JSON Patch (RFC 6902) operations" — informal.

---

## deploying.md

### Summary
Reviewed ~260 lines. Clean file. No issues.
