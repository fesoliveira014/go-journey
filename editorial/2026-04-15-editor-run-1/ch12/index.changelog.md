# Changelog: index.md

## Pass 1: Structural / Developmental
- 7 comments. Themes:
  - Strong motivation and bridging from Compose → Kubernetes; opening paragraph earns its keep.
  - "Key resource types" is long-winded reference content that interrupts narrative flow — acceptable because it seeds shared vocabulary for the remaining sections, but each entry could be tightened by ~20%.
  - Roadmap uses "development" overlay name that contradicts later sections (which use "local"). Terminology mismatch flagged.
  - Diagram commentary ("The diagram shows…") is redundant given adjacent positioning.

## Pass 2: Line Editing
- **Line ~3:** tighten idiom stacking
  - Before: "For development and demonstration that is exactly right."
  - After: "For development and demonstration that is the right tool."
  - Reason: "exactly right" stacks with "that is" — weaker than a direct noun phrase.
- **Line ~27:** shorten "does not have" construction
  - Before: "Kubernetes does not have startup ordering."
  - After: "Kubernetes has no startup ordering."
  - Reason: tighter; removes expletive "does not have."
- **Line ~31:** drop filler "notably"
  - Before: "PersistentVolumeClaims are notably heavier than Compose volumes."
  - After: "PersistentVolumeClaims are heavier than Compose volumes."
  - Reason: "notably" adds nothing; the comparison is explicit.
- **Line ~59:** redundant "The diagram shows"
  - Before: "The diagram shows that everything flows through the API server."
  - After: "Everything flows through the API server."
  - Reason: the diagram is directly above; the verb "shows" is redundant.
- **Line ~81:** "fundamentally" → "far"
  - Before: "this model is fundamentally more reliable"
  - After: "this model is far more reliable"
  - Reason: "fundamentally" is a weaker intensifier than "far" in this comparative; "far" also reads cleaner.
- **Line ~91:** parenthetical naming
  - Before: "the multi-container pattern (called a sidecar)"
  - After: "the multi-container pattern (the sidecar pattern)"
  - Reason: "sidecar" is the name of the pattern, not the configuration.
- **Line ~95:** Pods description
  - Before: "they are created and destroyed as replicas scale up and down, and each gets a new IP address when it starts"
  - After: "created and destroyed as replicas scale, each assigned a fresh IP at start"
  - Reason: removes two nominal phrases.
- **Line ~101:** parenthetical clarity
  - Before: "Secrets are base64-encoded (not encrypted by default at rest, though encryption can be enabled)"
  - After: "Secrets are base64-encoded and, by default, stored unencrypted at rest (etcd encryption can be enabled separately)"
  - Reason: the "though" clause was misparseable; readers conflate base64 encoding with etcd encryption.
- **Line ~169:** "not just organizational"
  - Before: "This separation is not just organizational."
  - After: "This separation is more than organizational."
  - Reason: positive framing is crisper.

## Pass 3: Copy Editing
- **Line ~45:** "API Server" → "API server" (Kubernetes docs style; CMOS 8.152 — match upstream product usage). Verify against kubernetes.io/docs.
- **Line ~79:** "256 MB" — mismatches YAML "256Mi" used later in chapter. Prefer "256 MiB" for consistency with manifests. CMOS 10.51 (binary prefixes).
- **Line ~79:** "100m of CPU" — add parenthetical "(= 0.1 CPU)" for first-time readers; retain lowercase "m" (Kubernetes convention, not an SI unit).
- **Line ~95:** "The default type, ClusterIP, is only reachable inside the cluster." → "…is reachable only inside the cluster." CMOS 5.186 (dangling "only").
- **Line ~97:** "(postgres-0, postgres-1)" → wrap in backticks: "(`postgres-0`, `postgres-1`)". Consistency with other inline identifiers.
- **Line ~103:** "nginx or Traefik" — the NGINX product uses all-caps. CMOS follows trademark owner's style. Query: the chapter mixes "nginx" and "NGINX"; normalize.
- **Lines ~116, 123, 131, 138, 146, 153:** code fences missing language tag. Add ```bash for shell examples (consistency with kind-setup.md).
- **Line ~219:** roadmap says "two overlays — development and production" but later sections consistently say "local and production." Align to "local and production" per CMOS consistency principle.
- **Line ~225:** comma splice: "Kubernetes is not a replacement for that workflow, it is the production target…" → replace comma with semicolon. CMOS 6.57.

## Pass 4: Final Polish
- **Line ~45:** "goes through the API server" — verify against https://kubernetes.io/docs/concepts/overview/components/ that "API server" (lowercase s) matches upstream.
- **Line ~225:** "every service you have built will be running… self-healing, namespace-isolated, and configured exactly as they would be…" — "they" refers to "every service" (grammatically singular collective). Acceptable by CMOS 5.61 for notional plural, but a reader may stumble. No change required; note only.
- No typos, doubled words, or broken cross-references detected. Footnote URLs appear valid at time of review (not confirmed via fetch).
