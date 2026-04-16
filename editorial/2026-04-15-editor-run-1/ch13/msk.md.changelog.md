# Changelog: msk.md

## Pass 1: Structural / Developmental
- 3 comments. Structure mirrors rds.md well. Problems: (1) the SSM Parameter Store → kubectl patch flow introduced here is imperative and conflicts with the declarative Kustomize flow in production-overlay.md (13.7) and cicd.md (13.8); (2) partial ConfigMap-patch material overlaps with 13.7.

## Pass 2: Line Editing
- **Line ~3:** "Just as Chapter 13 replaced" — reader is IN Chapter 13. Reference section 13.4.
- **Line ~19:** "what is worth building" → "what is worth writing yourself".
- **Line ~220:** kubectl patch on ConfigMap conflicts with declarative flow; call out as problematic pattern or remove.

## Pass 3: Copy Editing
- Multiple headings in title case: "Why MSK Instead of a StatefulSet", "MSK Configuration Resource", "The MSK Cluster Resource", "MSK Security Group", "Updating the ConfigMaps", "What Changes and What Does Not". Normalize to sentence case.
- **Line ~5:** QUERY — MSK + KRaft status; verify 3.6.0 on MSK uses KRaft or ZooKeeper.
- **Line ~13:** QUERY — Kafka 3.3 KRaft production readiness: confirmed.
- **Line ~15:** QUERY — Kubernetes `volumeClaimTemplates` expansion status (GA vs alpha). Check against K8s 1.29+.
- **Line ~33:** QUERY — `aws_msk_configuration` arguments verified.
- **Line ~33:** QUERY — MSK Kafka 3.6.0 availability.
- **Line ~47:** FACTUAL ERROR — `aws_msk_topic` resource does NOT exist in Terraform AWS provider. Replace with a correct alternative (Strimzi KafkaTopic or `Mongey/kafka` provider).
- **Line ~53:** QUERY — MSK default log.retention.hours: confirmed 168.
- **Line ~100:** QUERY — kafka.t3.small memory: 2 GiB confirmed.
- **Line ~100:** FACTUAL ERROR — CloudWatch metric name `KafkaBrokerDiskSpaceUsed` does not exist; correct is `KafkaDataLogsDiskUsed`. Fix.
- **Line ~106:** QUERY — `encryption_in_transit.client_broker` default — AWS default is TLS. Setting PLAINTEXT is explicit. OK as written.
- **Line ~125:** DUPLICATE RESOURCE: `aws_security_group.msk` declared here AND in networking.md. One must go.
- **Line ~125:** References `aws_security_group.eks_nodes.id` which is not declared anywhere in the chapter. networking.md uses `module.eks.node_security_group_id`. Unify.
- **Line ~151:** QUERY — MSK TLS listener 9094; confirmed. Also: IAM-auth on 9098, SCRAM on 9096 (for future reference).
- **Line ~162:** QUERY — `bootstrap_brokers` vs `bootstrap_brokers_tls` attribute names — verified.
- **Line ~189:** "Section 12.4" cross-reference — verify.
- **Line ~232:** QUERY — `aws kafka` CLI scope clarification: does not create topics.
- **Line ~234:** Base-layout claim ("k8s/messaging/") contradicts production-overlay.md's `base/local-infra/` restructuring. Align.

## Pass 4: Final Polish
- Footnotes not cited inline.
