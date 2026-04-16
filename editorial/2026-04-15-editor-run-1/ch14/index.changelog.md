# Changelog: index.md

## Pass 1: Structural / Developmental
- 6 comments. Themes:
  - Opening recap from Ch. 13 is strong and concrete; one long 140-word paragraph could be split.
  - "Why these gaps matter" covers two ideas per paragraph in places — consider splitting.
  - Mermaid diagram placement is correct (post-scope, pre-roadmap).
  - Chapter roadmap mirrors section files; good navigation.
  - Closing paragraph lands the stakes without overstating.

## Pass 2: Line Editing
- **Line ~15:** Trim redundant clause.
  - Before: "They are also independent of each other — you can apply them in any order, or apply just one if that is what your situation calls for."
  - After: "They are also independent — apply them in any order, or apply just one."
  - Reason: "of each other" redundant with "independent"; "if that is what your situation calls for" is filler.
- **Line ~21:** Simplify.
  - Before: "It is tempting to think of these as polish — things you would fix before a real public launch but can ignore while learning."
  - After: "It is tempting to think of these as polish: things you would fix before a public launch but can ignore while learning."
  - Reason: "real" modifies "public launch" redundantly.
- **Line ~25:** Drop "manually" (implied) and "at once" (filler).
  - Before: "Pasting database passwords manually into a Kustomize `secretGenerator` creates several problems at once."
  - After: "Pasting database passwords into a Kustomize `secretGenerator` creates several problems."
- **Line ~27:** Tighten.
  - Before: "and an attacker who has not already breached your network boundary cannot read that traffic"
  - After: "and an attacker who hasn't breached your network boundary cannot read it"
- **Line ~33:** Tighten.
  - Before: "The Kustomize layering from Chapter 12 and the CI/CD pipeline from Chapter 13 are not touched."
  - After: "The Kustomize layering from Chapter 12 and the CI/CD pipeline from Chapter 13 are untouched."
- **Line ~35:** Drop qualifier.
  - Before: "run exactly as they did in Chapter 13"
  - After: "run as they did in Chapter 13"
- **Line ~56:** Soften cost claim.
  - Before: "...billed at fifty cents per month regardless of query volume up to one billion queries."
  - After: "...$0.50 per month, which includes standard query volumes typical of this project."
  - Reason: Queries are billed separately at $0.40/million; "up to one billion free" is not accurate.
- **Line ~125:** Condense closing sentence from 48 to ~30 words.
  - Before: "These are not exotic hardening measures — they are the baseline that any production system deployed to a regulated environment is expected to meet, and they are the baseline that a careful engineer expects even in environments that are not formally regulated."
  - After: "These are not exotic hardening measures. They are the baseline that any production system in a regulated environment must meet — and that a careful engineer expects everywhere else too."
- **Line ~127:** Tighten.
  - Before: "Before moving to section 14.1, confirm that your Chapter 13 cluster is still running and healthy"
  - After: "Before section 14.1, confirm your Chapter 13 cluster is running and healthy"

## Pass 3: Copy Editing
- **Line ~23:** "TLS" — expand on first use in prose to "TLS (Transport Layer Security)" (CMOS 10.3).
- **Line ~23:** "OAuth2 providers including Google" — CMOS 6.49 prefers comma: "OAuth2 providers, including Google". Also consider consistent "OAuth 2.0" per RFC 6749. QUERY: confirm book-wide style.
- **Line ~25:** "K8s Secret object" → "Kubernetes Secret object" for consistency with surrounding prose. (CMOS 10.11 abbrev consistency.)
- **Line ~58:** "GoDaddy, Namecheap, Cloudflare" → "GoDaddy, Namecheap, or Cloudflare" (serial comma + conjunction; CMOS 6.19).
- **Line ~58:** "NS delegation" — on first prose use, expand to "name server (NS) delegation".
- **Line ~56:** Please verify: Route 53 hosted zone pricing ($0.50/month) and the "up to one billion queries" claim — AWS docs bill standard queries at $0.40/million, not included.
- **Line ~121:** Please verify: Ch. 13 actually enables AWS Security Hub (conditional phrasing assumes prior-chapter action).
- **Line ~127:** "re-apply" vs "reapply" — unify across the chapter. CMOS 7.89 permits both; pick one.
- Table header case: "Chapter 13 state / Chapter 13 target / Monthly cost" — mix of sentence case and variant; make consistent (recommend sentence case across all tables in ch. 14).

## Pass 4: Final Polish
- **Line ~3:** Split 140-word opening paragraph after "from a laptop." for pacing.
- No typos, doubled words, or broken cross-refs detected. Footnote superscripts all point to entries in the references list.
