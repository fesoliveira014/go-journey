# 14.4 Kafka Encryption: MSK TLS

<!-- [STRUCTURAL] Heading "14.4 Kafka Encryption: MSK TLS" — colon-style. Other sections: 14.1 em dash, 14.2 no dash, 14.3 no dash, 14.5 no dash. Unify chapter-wide. -->
<!-- [COPY EDIT] Heading capitalization after colon: "MSK TLS" — all caps OK as a technical abbreviation (CMOS 8.159 permits abbreviations in title-case headings). -->

<!-- [STRUCTURAL] Opening paragraph nicely recalls Ch. 13 state and sets the threat model. Keep. -->
Chapter 13 deployed the MSK cluster with `client_broker = "PLAINTEXT"` and opened port 9092 in the security group. The comment in the Terraform file acknowledged the gap: port 9094 and TLS were deferred to this chapter. This section closes that gap.

<!-- [LINE EDIT] "The usual objection to encrypting intra-VPC traffic is that the VPC boundary already provides isolation — an attacker on the public internet cannot reach port 9092, so what is there to protect?" — rhetorical question. Good pedagogically. Keep. -->
<!-- [COPY EDIT] "public internet" — lowercase per CMOS 8.190. Good. -->
<!-- [COPY EDIT] "AES-NI" — acronym not defined; acceptable for technical audience. Could expand: "AES-NI (Advanced Encryption Standard New Instructions)". Optional. -->
<!-- [COPY EDIT] "at line rate using AES-NI hardware acceleration" — accurate for most x86-64 and Graviton CPUs. Good. -->
The usual objection to encrypting intra-VPC traffic is that the VPC boundary already provides isolation — an attacker on the public internet cannot reach port 9092, so what is there to protect? The objection holds for the public internet, but the VPC boundary is not the only threat model. A compromised pod inside the cluster, a misconfigured VPC peering attachment, or a broad security group rule opened during debugging all create paths for lateral movement. Any process on the same VPC that can reach the MSK security group could read every Kafka message in transit if those messages are unencrypted. Enabling TLS closes that exposure with near-zero performance impact — modern server CPUs handle TLS at line rate using AES-NI hardware acceleration, and MSK brokers are no different[^1].

<!-- [COPY EDIT] "SOC 2 Type II, PCI DSS, and most HIPAA interpretations" — serial comma OK. Product capitalizations OK. -->
<!-- [COPY EDIT] Please verify: "SOC 2 Type II... require[s] encryption in transit for all data" — SOC 2 is a framework of trust service criteria; encryption specifics are implementation choices, not prescribed at the framework level. Soften to "SOC 2 Type II audits typically expect encryption in transit". -->
<!-- [LINE EDIT] "A finding that reads 'Kafka traffic between EKS nodes and MSK is unencrypted' will fail an audit." → concrete example, strong. Keep. -->
The change is also required for compliance. SOC 2 Type II, PCI DSS, and most HIPAA interpretations require encryption in transit for all data. A finding that reads "Kafka traffic between EKS nodes and MSK is unencrypted" will fail an audit. Fixing it is a one-line Terraform change.

<!-- [LINE EDIT] "No application code changes are needed. The Go Kafka client libraries handle TLS transparently when pointed at the TLS listener." → keep; strong. -->
<!-- [COPY EDIT] "broker address changes from port 9092 to port 9094" — precise. -->
No application code changes are needed. The Go Kafka client libraries handle TLS transparently when pointed at the TLS listener. The broker address changes from port 9092 to port 9094; everything above that is identical.

---

## What Changes in Terraform

<!-- [COPY EDIT] Heading "What Changes in Terraform" — Title Case; consistent within this file. Other files mix; chapter-wide unify. -->
### `terraform/msk.tf` — Enabling the TLS Listener

<!-- [STRUCTURAL] Sub-heading pattern (file names as section headings) is a nice UX for readers following along. Keep. -->
<!-- [COPY EDIT] "Enabling the TLS Listener" — Title Case. Good. -->

The `encryption_in_transit` block in `aws_msk_cluster.main` controls which listeners MSK activates. Update it from `PLAINTEXT` to `TLS`:

```hcl
# terraform/msk.tf

resource "aws_msk_cluster" "main" {
  cluster_name           = "${var.project_name}-kafka"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 2

  broker_node_group_info {
    instance_type  = "kafka.t3.small"
    client_subnets = module.vpc.private_subnets

    security_groups = [aws_security_group.msk.id]

    storage_info {
      ebs_storage_info {
        volume_size = 10
      }
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.main.arn
    revision = aws_msk_configuration.main.latest_revision
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }
}
```

<!-- [COPY EDIT] Please verify: `kafka_version = "3.6.0"` — a 2026-era book should likely use a more recent MSK version. Verify current MSK supported versions and update as appropriate. -->
The `client_broker` field accepts three values:

<!-- [COPY EDIT] Table — consistent formatting. "TLS only — Chapter 14 target state" → hyphen as em dash? Current chars are plain hyphen-minus inside table cells. CMOS 6.85 prefers em dashes; tables sometimes use hyphens for readability. OK. -->
| Value | Port 9092 open | Port 9094 open | Notes |
|-------|---------------|---------------|-------|
| `PLAINTEXT` | Yes | No | Chapter 13 state |
| `TLS_PLAINTEXT` | Yes | Yes | Both listeners active — useful during migration |
| `TLS` | No | Yes | TLS only — Chapter 14 target state |

<!-- [LINE EDIT] "For a fresh deployment — which is our case — go straight to `TLS`." → keep; sets expectation. -->
<!-- [COPY EDIT] "which is our case" — direct reader address. Consistent with book voice. Good. -->
`TLS_PLAINTEXT` is the recommended setting during a migration on a cluster that is already serving traffic: it activates the TLS listener without disabling the plaintext one, so you can deploy the updated client configuration and verify it is working before removing the fallback. For a fresh deployment — which is our case — go straight to `TLS`. There are no existing consumers to migrate and no compatibility window to maintain.

<!-- [COPY EDIT] "`in_cluster = true` encrypts replication traffic between the two broker nodes." — precise. Good. -->
<!-- [LINE EDIT] "This setting is independent of `client_broker` and was already enabled implicitly in Chapter 13 (it is the default). It is set explicitly here for clarity[^2]." → keep. -->
`in_cluster = true` encrypts replication traffic between the two broker nodes. This setting is independent of `client_broker` and was already enabled implicitly in Chapter 13 (it is the default). It is set explicitly here for clarity[^2].

### `terraform/vpc.tf` — Adding the TLS Security Group Rule

<!-- [LINE EDIT] "The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = \"TLS\"` is applied — MSK will stop listening on 9092 — but instead of removing the old rule immediately, it is cleaner to add the new one first, apply both, verify the TLS connection, and then remove the plaintext rule in a follow-up apply." → 62 words. Split: "The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = \"TLS\"` is applied; MSK stops listening on 9092. Even so, it is cleaner to add the TLS rule first, verify the connection, and then remove the plaintext rule in a follow-up apply." -->
The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = "TLS"` is applied — MSK will stop listening on 9092 — but instead of removing the old rule immediately, it is cleaner to add the new one first, apply both, verify the TLS connection, and then remove the plaintext rule in a follow-up apply. For a fresh deployment you can add only the TLS rule:

```hcl
# terraform/vpc.tf

resource "aws_security_group_rule" "msk_ingress_tls" {
  type                     = "ingress"
  from_port                = 9094
  to_port                  = 9094
  protocol                 = "tcp"
  security_group_id        = aws_security_group.msk.id
  source_security_group_id = module.eks.node_security_group_id
  description              = "Kafka TLS from EKS nodes"
}
```

<!-- [COPY EDIT] "intra-VPC service-to-service access" — hyphenated compounds. Good. -->
<!-- [COPY EDIT] "`0.0.0.0/0`" — backticks OK. -->
<!-- [LINE EDIT] "whitelist the source security group, not a broad IP range" — "whitelist" verb is fine but some style guides prefer "allow-list". Optional. -->
The `source_security_group_id` references the EKS managed node group security group, which is the same approach used by the `rds_ingress` rule for PostgreSQL. Only traffic originating from EKS worker nodes is permitted — no CIDR block, no `0.0.0.0/0`. This is the correct pattern for intra-VPC service-to-service access: whitelist the source security group, not a broad IP range.

### `terraform/outputs.tf` — Adding the TLS Bootstrap Output

<!-- [LINE EDIT] "MSK exposes two separate attributes for bootstrap broker strings" → "MSK exposes two attributes for bootstrap broker strings" (drop "separate"; implied). -->
MSK exposes two separate attributes for bootstrap broker strings: `bootstrap_brokers` for the plaintext listener and `bootstrap_brokers_tls` for the TLS listener. Chapter 13's `outputs.tf` already exported the plaintext string. Add the TLS equivalent:

```hcl
output "msk_bootstrap_brokers_tls" {
  description = "MSK bootstrap broker string (TLS)"
  value       = aws_msk_cluster.main.bootstrap_brokers_tls
}
```

After `terraform apply`, retrieve it:

```bash
terraform output msk_bootstrap_brokers_tls
```

The output is a comma-separated string of broker addresses on port 9094:

```
"b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
```

This string replaces the port 9092 addresses in the production Kustomize overlay.

---

## Updating the Production ConfigMap Patches

<!-- [STRUCTURAL] Three near-identical ConfigMap patches shown verbatim — consider same compression note as secrets.md (show 1, table the remaining two). Reader eye-fatigue risk. -->
<!-- [COPY EDIT] "the placeholder `MSK_BOOTSTRAP_BROKERS`" — consistent with Ch. 13 pattern. Good. -->
The production Kustomize overlay in `deploy/k8s/overlays/production/kustomization.yaml` contains three ConfigMap patches that set `KAFKA_BROKERS` to the MSK bootstrap string. Each currently uses the placeholder `MSK_BOOTSTRAP_BROKERS` pointing at port 9092. Update all three to use the TLS bootstrap string on port 9094.

<!-- [LINE EDIT] "The only value that changes is `KAFKA_BROKERS` — the port number shifts from 9092 to 9094, and the addresses come from `bootstrap_brokers_tls` rather than `bootstrap_brokers`" → keep; crisp. -->
The patches live in the `patches:` block of `kustomization.yaml`. The only value that changes is `KAFKA_BROKERS` — the port number shifts from 9092 to 9094, and the addresses come from `bootstrap_brokers_tls` rather than `bootstrap_brokers`:

```yaml
# deploy/k8s/overlays/production/kustomization.yaml (excerpt)

  # --- ConfigMap patches (Kafka brokers → MSK TLS) ---
  - target:
      kind: ConfigMap
      name: catalog-config
      namespace: library
    patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: catalog-config
        namespace: library
      data:
        GRPC_PORT: "50052"
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        OTEL_COLLECTOR_ENDPOINT: ""

  - target:
      kind: ConfigMap
      name: reservation-config
      namespace: library
    patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: reservation-config
        namespace: library
      data:
        GRPC_PORT: "50053"
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"
        MAX_ACTIVE_RESERVATIONS: "5"
        OTEL_COLLECTOR_ENDPOINT: ""

  - target:
      kind: ConfigMap
      name: search-config
      namespace: library
    patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: search-config
        namespace: library
      data:
        GRPC_PORT: "50054"
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        MEILI_URL: "http://meilisearch.data.svc.cluster.local:7700"
        CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"
```

<!-- [LINE EDIT] "As in Chapter 13, you can automate this in the CI/CD pipeline by writing the TLS bootstrap string to SSM Parameter Store during the Terraform apply and reading it back before running `kubectl apply`." → keep; actionable. -->
Replace the placeholder broker addresses with the actual output from `terraform output msk_bootstrap_brokers_tls`. As in Chapter 13, you can automate this in the CI/CD pipeline by writing the TLS bootstrap string to SSM Parameter Store during the Terraform apply and reading it back before running `kubectl apply`.

<!-- [COPY EDIT] "in kind" — referring to the Kubernetes-in-Docker tool. Project name is "kind" (lowercase) — OK, but confusing at sentence start or mid-sentence; consider backticks: "in `kind`" for disambiguation. -->
The base ConfigMaps under `deploy/k8s/base/library/` are not touched. They continue to use the in-cluster address on port 9092, which is correct for local development against the Kafka StatefulSet in kind. The overlay handles the production-specific difference.

---

## Go Client TLS Configuration

<!-- [STRUCTURAL] Standout section — distinguishes this book from generic MSK tutorials. The "no code change needed" framing helps Java/Kotlin readers who expect JKS truststore dance. Keep. -->
<!-- [LINE EDIT] "Here is the part that surprises engineers coming from other ecosystems: you probably do not need to change the application code at all, and you certainly do not need to bundle a custom CA certificate." → "Here is the part that surprises engineers coming from other ecosystems: you probably do not need to change the application code, and you certainly do not need a custom CA bundle." (tightens). -->
Here is the part that surprises engineers coming from other ecosystems: you probably do not need to change the application code at all, and you certainly do not need to bundle a custom CA certificate.

<!-- [COPY EDIT] "Amazon Trust Services, Amazon's public certificate authority" — "Certificate Authority" capitalization? Lowercase "certificate authority" is standard when not referring to a specific named entity. Good as written. -->
<!-- [COPY EDIT] "scratch-based images built on top of Alpine or distroless" — slightly confusing; scratch is its own base (zero-content), whereas Alpine/distroless are separate minimal bases. Reword: "the minimal base images used by the library services (Alpine-based or distroless)" — scratch would not contain a CA bundle by default. -->
<!-- [COPY EDIT] Please verify: phrasing "scratch-based images built on top of Alpine or distroless" is technically imprecise; scratch is independent of Alpine/distroless. Reword to clarify the build-stage vs final-image distinction. -->
MSK TLS certificates are issued by Amazon Trust Services, Amazon's public certificate authority. The root certificates for Amazon Trust Services are included in the system trust store on every major Linux distribution, including the base images used by the library services (scratch-based images built on top of Alpine or distroless)[^3]. Go's `crypto/tls` package uses the system trust store by default. When the Kafka client connects to port 9094, it performs a standard TLS handshake, verifies the broker's certificate against the system roots, and proceeds. No custom CA bundle, no `InsecureSkipVerify`, no manual certificate distribution.

<!-- [COPY EDIT] "If the services use `sarama`:" — backticks on library name. Good. -->
<!-- [COPY EDIT] Please verify: `sarama` recommendation. Earlier commit in this repo noted Sarama is in maintenance mode and franz-go/confluent-kafka-go are preferred. Consider aligning this section with that note (mention franz-go as a recommended alternative). -->
The only code change is enabling TLS in the client configuration. If the services use `sarama`:

```go
config := sarama.NewConfig()
config.Net.TLS.Enable = true
// config.Net.TLS.Config is nil by default, which causes sarama to use
// a zero-value tls.Config. A zero-value tls.Config uses the system root
// certificate pool — exactly what we need for MSK.
```

If the services use `segmentio/kafka-go`:

```go
dialer := &kafka.Dialer{
    Timeout:   10 * time.Second,
    DualStack: true,
    TLS:       &tls.Config{}, // empty config, system roots apply
}
```

<!-- [COPY EDIT] "In both cases, passing an empty `tls.Config{}` is equivalent to passing `nil` for the CA pool" — the two libraries handle it differently; statement is close but segmentio/kafka-go expects non-nil `TLS`. Phrasing OK. -->
<!-- [COPY EDIT] "standard library falls back to the system root store, which trusts Amazon Trust Services" — accurate. Good. -->
In both cases, passing an empty `tls.Config{}` is equivalent to passing `nil` for the CA pool — the standard library falls back to the system root store, which trusts Amazon Trust Services, which signed the MSK broker certificate. The connection succeeds without any further configuration.

<!-- [COPY EDIT] "confluent-kafka-go" — backticks. OK. -->
If the services use `confluent-kafka-go`, TLS is configured through the `security.protocol` property:

```go
producer, err := kafka.NewProducer(&kafka.ConfigMap{
    "bootstrap.servers": brokers,
    "security.protocol": "ssl",
    // ssl.ca.location is not needed — librdkafka uses the system CA store
})
```

<!-- [LINE EDIT] "The one exception to 'no code change needed' arises if your container images use a stripped-down base that does not include a CA bundle — for example, a truly empty `scratch` image." → "truly empty" is redundant with "empty `scratch`". Drop "truly". -->
<!-- [COPY EDIT] "x509: certificate signed by unknown authority" — Go error text backticked. Good. -->
The one exception to "no code change needed" arises if your container images use a stripped-down base that does not include a CA bundle — for example, a truly empty `scratch` image. In that case, Go's `crypto/tls` cannot find any system roots and the TLS handshake will fail with an error like `x509: certificate signed by unknown authority`. The fix is to copy the CA bundle into the image during the Docker build:

```dockerfile
FROM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/service /service
ENTRYPOINT ["/service"]
```

<!-- [COPY EDIT] Please verify: `alpine:3.20` — the latest Alpine at April 2026 is 3.21 or later. Consider updating to match current stable or using a floating tag like `alpine:3` for the CA-source stage. -->
<!-- [LINE EDIT] "The library system's Dockerfiles already copy the CA bundle from Alpine, so this is handled." → keep; clear. -->
The library system's Dockerfiles already copy the CA bundle from Alpine, so this is handled. The point is worth understanding so that the error message — if you ever encounter it in a different project — does not send you down a rabbit hole of debugging TLS configuration when the actual fix is two Dockerfile lines.

---

## Migration Strategy

<!-- [STRUCTURAL] Migration strategy is good even though the chapter's concrete path is fresh-deploy. Shows the production-grade path. Keep. -->
The approach above describes a fresh deployment — no data in the existing cluster, no consumers to migrate. If you apply this chapter's changes to a cluster that is already handling production traffic, the steps are slightly different.

<!-- [COPY EDIT] "rolling restart — brokers are restarted one at a time" — precise. -->
<!-- [COPY EDIT] "one to two minutes per broker" — CMOS 9.3. OK. -->
**Step 1: Switch to `TLS_PLAINTEXT`.** This activates the TLS listener without disabling the plaintext one. Both ports are open. Apply the Terraform change and wait for MSK to complete the broker configuration update. MSK applies this change as a rolling restart — brokers are restarted one at a time, so the cluster remains available, though with reduced redundancy during the restart window. Expect one to two minutes per broker.

**Step 2: Deploy with port 9094.** Update the ConfigMap patches to use the TLS bootstrap string and run `kubectl apply -k deploy/k8s/overlays/production/`. The pods restart and begin connecting on port 9094. The plaintext listener is still active, so any pod that has not yet restarted continues to work on port 9092 during the rolling update.

<!-- [COPY EDIT] "The MSK console shows per-group lag under Monitoring." — "Monitoring" capitalized (UI label); OK. -->
**Step 3: Verify consumers.** Check that all consumer groups are making progress. The MSK console shows per-group lag under Monitoring. Alternatively, use `kcat` from inside a pod:

```bash
kcat -b b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094 \
  -X security.protocol=ssl \
  -L
```

<!-- [LINE EDIT] "If the broker list comes back without errors, the TLS connection is working. If consumer lag is zero or decreasing normally, the switch was successful." → keep. -->
If the broker list comes back without errors, the TLS connection is working. If consumer lag is zero or decreasing normally, the switch was successful.

**Step 4: Switch to `TLS` only.** Update `client_broker = "TLS"` in `msk.tf` and apply again. Remove the port 9092 security group rule. MSK performs another rolling restart to disable the plaintext listener.

For a fresh deployment — the case in this chapter — skip directly to `TLS`. There is no traffic to migrate and no compatibility window needed.

---

## Verification

<!-- [STRUCTURAL] Verification section pairs openssl s_client with pod logs — two orthogonal signals. Good. Keep. -->
After applying both the Terraform changes and the Kustomize overlay, confirm that the TLS listener is reachable from inside the cluster. Run an ephemeral pod in the `library` namespace:

```bash
kubectl run -it --rm tls-check \
  --image=alpine \
  --restart=Never \
  -n library \
  -- sh
```

Inside the pod, install `openssl` and connect to one of the MSK brokers:

```bash
apk add --no-cache openssl
openssl s_client -connect b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094
```

Look for two things in the output. First, the certificate chain should include Amazon's CA:

```
depth=2 C=US, O=Amazon, CN=Amazon Root CA 1
depth=1 C=US, O=Amazon, CN=Amazon RSA 2048 M01
depth=0 CN=*.kafka.us-east-1.amazonaws.com
```

Second, the handshake should complete successfully:

```
SSL handshake has read 4135 bytes and written 415 bytes
Verify return code: 0 (ok)
```

<!-- [LINE EDIT] "Any non-zero return code indicates a certificate validation failure and needs investigation before proceeding." → keep; actionable. -->
`Verify return code: 0 (ok)` means the certificate chain validated against a trusted root in the system store. Any non-zero return code indicates a certificate validation failure and needs investigation before proceeding.

<!-- [LINE EDIT] "Also confirm that the pods are connecting by checking the application logs" → keep. -->
Also confirm that the pods are connecting by checking the application logs:

```bash
kubectl logs -n library -l app=catalog --tail=20
```

You should see the Kafka consumer group join log lines without any TLS or connection errors. If the service previously logged `connected to broker b-1...` on startup, that line should still appear, now against port 9094.

---

[^1]: MSK encryption in transit documentation: https://docs.aws.amazon.com/msk/latest/developerguide/msk-encryption.html
[^2]: `aws_msk_cluster` Terraform resource reference — `encryption_info`: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/msk_cluster#encryption_info
[^3]: Amazon Trust Services root CA information: https://www.amazontrust.com/repository/
[^4]: Go `crypto/tls` package — system certificate pool behavior: https://pkg.go.dev/crypto/tls#Config

<!-- [FINAL] Footnote [^4] is defined but never referenced in body. Add an in-body reference or remove. -->
