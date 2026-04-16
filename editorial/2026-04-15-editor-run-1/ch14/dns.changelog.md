# Changelog: dns.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - DNS refresher is well-scoped to A/CNAME/alias; right level for audience.
  - Chicken-and-egg ordering section is a standout — honest, concrete.
  - "If you don't own a domain" is a thoughtful inclusion for learners.
  - Consider whether each section should redefine abbreviations (TLS, ACM) for readers arriving via direct link — chapter-wide decision.

## Pass 2: Line Editing
- **Line ~7:** Condense.
  - Before: "The Terraform is concise: one data source to look up your hosted zone, one data source to look up the ALB, and one record that points your domain at the load balancer using an AWS alias record."
  - After: "The Terraform is concise: one data source for the hosted zone, one for the ALB, and one record pointing your domain at the load balancer via an AWS alias."
- **Line ~19:** Tighten.
  - Before: "the presence of a CNAME at the apex would be technically invalid"
  - After: "a CNAME at the apex would be invalid"
- **Line ~21:** Plural.
  - Before: "returns the current A record of the target"
  - After: "returns the target's current A records"
  - Reason: ALB has multiple A records.
- **Line ~27:** Drop "open" (redundant with "any resolver worldwide").
  - Before: "serves DNS responses to the open internet — any resolver worldwide can query it"
  - After: "serves DNS responses to the internet — any resolver worldwide can query it"
- **Line ~33:** Drop "any".
  - Before: "returns all values for a name without any health-check or geographic logic"
  - After: "returns all values for a name without health-check or geographic logic"
- **Line ~101:** Tighten.
  - Before: "There is a sequencing dependency here that Terraform's dependency graph cannot automatically resolve."
  - After: "Terraform's dependency graph cannot resolve this sequencing automatically."

## Pass 3: Copy Editing
- **Line ~3:** "ACM" — expand on first use in this section: "AWS Certificate Manager (ACM)" (CMOS 10.3).
- **Line ~19:** "SOA and NS record" → "SOA and NS records" (plural; two record types).
- **Line ~29:** "externally-registered domain" → "externally registered domain" (CMOS 7.89 — no hyphen after "-ly" adverb).
- **Line ~93:** QUERY — Please verify: with `evaluate_target_health = true` on an ALB alias, Route 53 evaluates the ALB's target-group health rather than "probing health endpoints". Phrasing "probe the ALB's health endpoints" is imprecise.
- **Line ~162:** Consider a one-line clarifier: "Alias records return the target's TTL (typically 60 seconds for ALBs), not the 300-second default" — reconciles "TTL defaults to 300" with the "60" value in sample output.
- **Line ~202:** QUERY — Please verify: current `.com` registration price via Route 53 is $14/year (per AWS pricing 2025), not $12/year.
- **Line ~138:** Add `bash` language hint to shell code blocks for syntax-highlight consistency (optional; unify across chapter).
- **Line ~150:** "near-instantaneous" — preferred "nearly instantaneous" or leave hyphenated form for brevity (CMOS 7.81).

## Pass 4: Final Polish
- **Line ~107:** "null_resource" is sometimes in backticks, sometimes not. Verify and unify; current mix is acceptable but flag.
- No typos, doubled words, or broken cross-refs detected. Footnote superscripts all resolve.
