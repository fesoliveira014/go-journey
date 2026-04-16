# Changelog: index.md

## Pass 1: Structural / Developmental
- 4 comments. Themes: good opening hook and architecture diagram; chapter roadmap is strong but has inconsistencies with what the body sections actually deliver (schema migration Jobs, smoke tests, parameter groups, automated backups); consider adding a "why this order" note explaining dependencies between sections.

## Pass 2: Line Editing
- **Line ~3:** "exactly the right tool" — mild filler. Suggest: "the right tool".
- **Line ~46:** "A few details worth noting." — sentence fragment, missing verb. Suggest: "A few details are worth noting."
- **Line ~68:** "The right column is not a replacement in the 'throw out the old thing' sense." — wordy. Suggest: "The right column is not a wholesale replacement."
- **Line ~82:** "tighter CPU and memory bounds appropriate for a paid compute environment" — awkward. Suggest: "CPU and memory bounds tuned for a paid compute environment".
- **Line ~109:** "run `terraform destroy` when you are done for the day" — suggest "when you finish for the day" (crisper).
- **Line ~111:** "You will lose the high-availability story" — "story" is vague jargon. Suggest: "You will lose high availability".
- **Line ~119:** "the path of least resistance" — mild cliché; consider "the simplest path".
- **Line ~173:** "the same fundamentals" — vague; consider "the same core mechanics".

## Pass 3: Copy Editing
- **Line ~3:** "multi-AZ" vs "Multi-AZ" — inconsistent in chapter. AWS canonical form is "Multi-AZ" (CMOS 7.89, technical abbreviation convention).
- **Line ~48:** Verify "AWS Load Balancer Controller" is canonical; avoid "AWS LoadBalancer Controller" typo.
- **Line ~48:** QUERY — Is AWS LB Controller installed as an EKS managed add-on or via Helm? eks.md uses Helm. Unify.
- **Line ~61:** Table row parallelism: "`kind load docker-image`" (command) vs. "NGINX Ingress Controller" (product). Consider "kind image cache" for parallelism.
- **Line ~79:** Environment variable named `DB_HOST` contradicts rds.md and production-overlay.md, which use `DATABASE_URL`. Resolve.
- **Line ~83:** `kubernetes.io/ingress.class` annotation is deprecated; prefer `ingressClassName`. Cross-reference production-overlay.md which uses the modern form.
- **Line ~98:** Table: "2x t3.medium" — CMOS prefers "2×" (Unicode ×) or spelled "two". Apply consistently.
- **Lines ~97-104:** QUERY — Verify individual AWS cost claims (EKS $73, t3.medium $61, db.t3.micro $13, kafka.t3.small $73, NAT $33, ALB $16) against current us-east-1 pricing. MSK claim at $73 for 2× kafka.t3.small may be high (t3.small ≈ $0.0456/hr × 2 × 730 ≈ $66.58).
- **Line ~109:** "you will configure an S3 backend in section 13.1" — terraform-fundamentals.md defers S3. Soften to "once you enable remote state (see section 13.1)".
- **Line ~111:** QUERY — Verify $160/month claim for cost-saving configuration; quick math suggests closer to $255.
- **Line ~109:** "git" — CMOS 8.1 convention would be "Git" when referring to the tool. Accept project style but be consistent.
- **Line ~121:** QUERY — "AWS CLI v1 will not work" is weak. v1 is end-of-support (2024); state that more directly.
- **Line ~130:** QUERY — Terraform 1.5 and `check` blocks: confirmed introduced May 2023.
- **Line ~130:** ">=" → "≥" or "1.5 or later".
- **Line ~140:** Cluster name inconsistency: `library-cluster` vs. `library-production` (deploying.md) vs. `local.cluster_name` (eks.md). Unify.
- **Line ~151:** Roadmap says 13.1 covers "remote state in S3"; body defers. Fix.
- **Line ~153:** Roadmap says "NAT gateways" (plural); networking.md uses one. Fix.
- **Line ~155:** Roadmap claims 13.3 "wires up the GitHub Actions workflow"; ecr.md does not.
- **Line ~157:** Roadmap claims 13.4 covers "automated backups" and "parameter groups" and "schema migrations as Jobs"; rds.md delivers none of these.
- **Line ~165:** Roadmap claims 13.8 contains a smoke-test job; cicd.md does not.

## Pass 4: Final Polish
- **Line ~46:** "A few details worth noting." — sentence fragment; either add "are" or repunctuate as a heading.
- **Line ~184 (footnotes):** Footnotes `[^1]`…`[^7]` are defined but never referenced in body prose. Either cite inline or convert to "Further reading". CMOS 14.24.
