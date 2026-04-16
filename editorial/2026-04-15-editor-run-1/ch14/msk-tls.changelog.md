# Changelog: msk-tls.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Opening threat model (intra-VPC lateral movement) is concrete — avoids hand-wavy "TLS is best practice" framing.
  - "Go Client TLS Configuration" is a standout — addresses the common Java/Kotlin reader's JKS-truststore expectation.
  - Three ConfigMap patch blocks in series risk eye fatigue; consider 1 worked example + table for remaining two.
  - Migration Strategy section usefully complements the fresh-deploy main path.

## Pass 2: Line Editing
- **Line ~68:** Split 62-word sentence.
  - Before: "The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = \"TLS\"` is applied — MSK will stop listening on 9092 — but instead of removing the old rule immediately, it is cleaner to add the new one first, apply both, verify the TLS connection, and then remove the plaintext rule in a follow-up apply."
  - After: "The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = \"TLS\"` is applied; MSK stops listening on 9092. Even so, it is cleaner to add the TLS rule first, verify the connection, and then remove the plaintext rule in a follow-up apply."
- **Line ~88:** Drop "separate".
  - Before: "MSK exposes two separate attributes for bootstrap broker strings"
  - After: "MSK exposes two attributes for bootstrap broker strings"
- **Line ~180:** Tighten.
  - Before: "you probably do not need to change the application code at all, and you certainly do not need to bundle a custom CA certificate."
  - After: "you probably do not need to change the application code, and you certainly do not need a custom CA bundle."
- **Line ~216:** Drop "truly".
  - Before: "for example, a truly empty `scratch` image"
  - After: "for example, an empty `scratch` image"

## Pass 3: Copy Editing
- **Line ~1:** Heading style: "14.4 Kafka Encryption: MSK TLS" (colon) vs 14.1 em dash, 14.2 no dash, 14.3 no dash, 14.5 no dash. Unify chapter-wide.
- **Line ~5:** QUERY — "AES-NI" acronym not expanded on first use; consider "AES-NI (AES New Instructions)" (CMOS 10.3). Acceptable for the target audience.
- **Line ~7:** QUERY — "SOC 2 Type II... require[s] encryption in transit for all data". SOC 2 is a trust-services framework, not a prescriptive standard. Soften.
- **Line ~25:** QUERY — "kafka_version = \"3.6.0\"". For a 2026 book, verify whether newer MSK-supported Kafka versions are appropriate.
- **Line ~174:** "in kind" — project name is lowercase "kind" (KIND). Consider `in kind` (backticked) for disambiguation mid-sentence.
- **Line ~182:** QUERY — "scratch-based images built on top of Alpine or distroless" is technically imprecise; scratch is an independent zero-content base image, separate from Alpine/distroless. Reword to separate build-stage base from final base.
- **Line ~186:** QUERY — `sarama` guidance: the repo has noted Sarama is in maintenance mode (per recent commit note). Consider mentioning franz-go or confluent-kafka-go as preferred forward path, or a pointer to the maintenance note.
- **Line ~219:** QUERY — `alpine:3.20` is likely outdated by April 2026 (Alpine 3.21+ available). Consider floating tag `alpine:3` or update.
- **Line ~84:** "whitelist the source security group" — some style guides prefer "allow-list" over "whitelist". Optional.

## Pass 4: Final Polish
- **Line ~305:** Footnote [^4] (Go crypto/tls) is defined but never referenced in body. Add an inline reference (e.g., after the "zero-value `tls.Config`" paragraph) or remove.
- No typos or doubled words detected. Heading case Title Case throughout — consistent within file.
- Tables use plain hyphen-minus inside cells for dashes; chapter-wide unify to em dash or leave as is.
