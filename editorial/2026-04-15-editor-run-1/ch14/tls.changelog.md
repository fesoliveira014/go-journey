# Changelog: tls.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Opening risk framing (browser warnings, OAuth2, session tokens) duplicates index.md — trim to avoid redundancy.
  - Heading hierarchy and case inconsistent vs other sections (Title Case here, sentence case in dns.md).
  - "Applying the Changes" is also the title of 14.5; rename this sub-heading to avoid TOC collision.
  - cert-manager alternative section is appropriately brief.

## Pass 2: Line Editing
- **Line ~5:** Drop filler.
  - Before: "The good news is that there is no longer a cost reason to skip TLS."
  - After: "There is no longer a cost reason to skip TLS."
- **Line ~37 / acm block:** Drop tautology.
  - Before: "Without it, the ALB would briefly have no valid certificate during the update"
  - After: "Without it, the ALB would briefly have no valid certificate"
- **Line ~92:** Restructure for emphasis.
  - Before: "is not a real AWS resource — it is a Terraform construct that polls the ACM API"
  - After: "is a Terraform construct, not a real AWS resource. It polls the ACM API"
- **Line ~119:** Tighten.
  - Before: "Both must be present for the redirect to work; the redirect is applied to the HTTP listener, and it needs somewhere to redirect to."
  - After: "Both listeners must exist for the redirect to work: the redirect is applied to the HTTP listener and needs a target."

## Pass 3: Copy Editing
- **Line ~1:** Heading uses "14.2 TLS with ACM" (no em dash) while 14.1 uses em dash. Unify.
- **Line ~5:** "ALB, CloudFront, API Gateway" → add serial "or" and Oxford comma: "ALB, CloudFront, or API Gateway" (CMOS 6.19).
- **Line ~5:** QUERY — ACM renewal timing: "roughly 30 days before" — AWS docs indicate renewal attempts begin ~60 days before expiration. Please verify.
- **Line ~19:** QUERY — "every 13 months" conflates certificate validity (~395 days) with renewal cadence. ACM renews before expiry; "every 13 months" is imprecise.
- **Line ~39:** QUERY — "PCI DSS and some HIPAA interpretations fall into this category" — PCI DSS v4.0 requirement 4 targets transmission over open/public networks, not all internal hops. Consider softening.
- **Line ~88:** Consider expanding "Subject Alternative Name" → "Subject Alternative Name (SAN)" on first use (CMOS 10.3).
- **Line ~148:** QUERY — File name inconsistency: section intro references "in `kustomization.yaml`" but apply step references pasting into `ingress-patch.yaml`. Unify to the actual project layout.
- **Line ~174:** QUERY — "HTTP/2 which requires TLS" — spec allows h2c cleartext; browsers and ALB require TLS. Clarify to avoid spec-level inaccuracy.
- **Line ~197:** "Amazon's intermediate CA" — expand CA on first use in file: "Certificate Authority (CA)".
- **Line ~212:** "Chapter 14.5" → "Section 14.5" to match style used elsewhere.
- **Line ~22:** "cert-manager with Let's Encrypt" — "Let's Encrypt" capitalization correct (CMOS 8.87). Good.

## Pass 4: Final Polish
- **Line ~7:** Referent check: "the next section explains why" — verify that the following subsection ("TLS Termination at the ALB") actually explains the ALB-to-pod-sees-plain-HTTP rationale; it does. OK.
- No typos or doubled words detected. Footnote superscripts resolve.
- Heading case: Title Case throughout this file; dns.md uses sentence case. Unify chapter-wide in a final pass.
