# 14.4—Kafka Encryption: MSK TLS

Chapter 13 deployed the MSK cluster with `client_broker = "PLAINTEXT"` and opened port 9092 in the security group. The comment in the Terraform file acknowledged the gap: port 9094 and TLS were deferred to this chapter. This section closes that gap.

The usual objection to encrypting intra-VPC traffic is that the VPC boundary already provides isolation—an attacker on the public internet cannot reach port 9092, so what is there to protect? The objection holds for the public internet, but the VPC boundary is not the only threat model. A compromised pod inside the cluster, a misconfigured VPC peering attachment, or a broad security group rule opened during debugging all create paths for lateral movement. Any process in the same VPC that can reach the MSK security group can read every Kafka message in transit if those messages are unencrypted. Enabling TLS closes that exposure with near-zero performance impact—modern server CPUs handle TLS at line rate using AES-NI hardware acceleration, and MSK brokers are no different[^1].

The change is also required for compliance. SOC 2 Type II, PCI DSS, and most HIPAA interpretations require encryption in transit for all data. A finding that reads "Kafka traffic between EKS nodes and MSK is unencrypted" will fail an audit. Fixing the broker listener is a small Terraform change, but the application must also tell Sarama to use TLS. Pointing a plaintext client at port 9094 will fail during the Kafka protocol handshake.

---

## What Changes in Terraform

### `terraform/msk.tf`—Enabling the TLS Listener

The `encryption_in_transit` block in `aws_msk_cluster.library` controls which listeners MSK activates. Update it from `PLAINTEXT` to `TLS`:

```hcl
# terraform/msk.tf

resource "aws_msk_cluster" "library" {
  cluster_name           = "library"
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
    arn      = aws_msk_configuration.library.arn
    revision = aws_msk_configuration.library.latest_revision
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }
}
```

The `client_broker` field accepts three values:

| Value | Port 9092 open | Port 9094 open | Notes |
|-------|---------------|---------------|-------|
| `PLAINTEXT` | Yes | No | Chapter 13 state |
| `TLS_PLAINTEXT` | Yes | Yes | Both listeners active—useful during migration |
| `TLS` | No | Yes | TLS only—Chapter 14 target state |

`TLS_PLAINTEXT` is the recommended setting during a migration on a cluster that is already serving traffic: it activates the TLS listener without disabling the plaintext one, so you can deploy the updated client configuration and verify it is working before removing the fallback. For a fresh deployment—which is the case here—go straight to `TLS`. There are no existing consumers to migrate and no compatibility window to maintain.

`in_cluster = true` encrypts replication traffic between the two broker nodes. This setting is independent of `client_broker` and was already enabled implicitly in Chapter 13 (it is the default). It is set explicitly here for clarity[^2].

### `terraform/vpc.tf`—Adding the TLS Security Group Rule

The current security group allows inbound TCP on port 9092. That rule is no longer needed once `client_broker = "TLS"` is applied; MSK stops listening on 9092. Even so, it is cleaner to add the TLS rule first, verify the connection, and then remove the plaintext rule in a follow-up apply. For a fresh deployment you can add only the TLS rule:

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

The `source_security_group_id` references the EKS managed node group security group, which is the same approach used by the `rds_ingress` rule for PostgreSQL. Only traffic originating from EKS worker nodes is permitted—no CIDR block, no `0.0.0.0/0`. This is the correct pattern for intra-VPC service-to-service access: allowlist the source security group, not a broad IP range.

### `terraform/outputs.tf`—Adding the TLS Bootstrap Output

MSK exposes two attributes for bootstrap broker strings: `bootstrap_brokers` for the plaintext listener and `bootstrap_brokers_tls` for the TLS listener. Chapter 13's `outputs.tf` already exported the plaintext string. Add the TLS equivalent:

```hcl
output "msk_bootstrap_brokers_tls" {
  description = "MSK bootstrap broker string (TLS)"
  value       = aws_msk_cluster.library.bootstrap_brokers_tls
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

The production Kustomize overlay in `deploy/k8s/overlays/production/kustomization.yaml` contains three ConfigMap patches that set `KAFKA_BROKERS` to the MSK bootstrap string. In the final repository, those patches use the placeholder `MSK_BOOTSTRAP_BROKERS_TLS`. When you apply this hardening step, replace that placeholder with the actual TLS bootstrap string on port 9094.

The patches live in the `patches:` block of `kustomization.yaml`. Two values change: `KAFKA_BROKERS` shifts from the plaintext bootstrap output to `bootstrap_brokers_tls`, and `KAFKA_TLS` is set to `"true"` so the Sarama clients use TLS:

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
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        KAFKA_TLS: "true"

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
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        KAFKA_TLS: "true"

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
        KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094"
        KAFKA_TLS: "true"
```

Replace the placeholder broker addresses with the actual output from `terraform output msk_bootstrap_brokers_tls`. As in Chapter 13, you can automate this in the CI/CD pipeline by writing the TLS bootstrap string to SSM Parameter Store during the Terraform apply and reading it back before running `kubectl apply`.

The base ConfigMaps under `deploy/k8s/base/library/` are not touched. They continue to use the in-cluster address on port 9092, which is correct for local development against the Kafka StatefulSet in kind. The overlay handles the production-specific difference.

---

## Go Client TLS Configuration

Sarama does not infer TLS from the broker port. The library system uses a small shared helper in `pkg/kafka` so producers and consumers all get the same TLS behavior:

```go
func NewSaramaConfig(tlsEnabled bool) *sarama.Config {
    cfg := sarama.NewConfig()
    if tlsEnabled {
        cfg.Net.TLS.Enable = true
        cfg.Net.TLS.Config = &tls.Config{MinVersion: tls.VersionTLS12}
    }
    return cfg
}
```

The Catalog publisher, Reservation publisher, Catalog reservation-event observer, and Search consumer all call this helper. Local development leaves `KAFKA_TLS=false`; the production overlay sets `KAFKA_TLS=true`.

MSK TLS certificates are issued by Amazon Trust Services, Amazon's public certificate authority. Go's `crypto/tls` package uses the system trust store by default when `RootCAs` is nil. That means the application does not need a custom CA bundle or `InsecureSkipVerify`; it does need a container image that actually contains the system CA bundle.

The runtime images therefore install `ca-certificates` before switching to the non-root app user:

```dockerfile
FROM alpine:3.19
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/catalog /usr/local/bin/catalog
```

If you use `scratch` or another stripped-down runtime image in a different project, copy `/etc/ssl/certs/ca-certificates.crt` into the image yourself. Otherwise the TLS handshake can fail with `x509: certificate signed by unknown authority` even when the Sarama TLS config is correct.

---

## Migration Strategy

The approach above describes a fresh deployment—no data in the existing cluster, no consumers to migrate. If you apply this chapter's changes to a cluster that is already handling production traffic, the steps are slightly different.

**Step 1: Switch to `TLS_PLAINTEXT`.** This activates the TLS listener without disabling the plaintext one. Both ports are open. Apply the Terraform change and wait for MSK to complete the broker configuration update. MSK applies this change as a rolling restart—brokers are restarted one at a time, so the cluster remains available, though with reduced redundancy during the restart window. Expect one to two minutes per broker.

**Step 2: Deploy with port 9094 and `KAFKA_TLS=true`.** Update the ConfigMap patches to use the TLS bootstrap string, set `KAFKA_TLS=true`, and run `kubectl apply -k deploy/k8s/overlays/production/`. The pods restart and begin connecting on port 9094. The plaintext listener is still active, so any pod that has not yet restarted continues to work on port 9092 during the rolling update.

**Step 3: Verify consumers.** Check that all consumer groups are making progress. The MSK console shows per-group lag under Monitoring. Alternatively, use `kcat` from inside a pod:

```bash
kcat -b b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9094 \
  -X security.protocol=ssl \
  -L
```

If the broker list comes back without errors, the TLS connection is working. If consumer lag is zero or decreasing normally, the switch was successful.

**Step 4: Switch to `TLS` only.** Update `client_broker = "TLS"` in `msk.tf` and apply again. Remove the port 9092 security group rule. MSK performs another rolling restart to disable the plaintext listener.

For a fresh deployment—the case in this chapter—skip directly to `TLS`. There is no traffic to migrate and no compatibility window needed.

---

## Verification

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

`Verify return code: 0 (ok)` means the certificate chain validated against a trusted root in the system store. Any non-zero return code indicates a certificate validation failure and needs investigation before proceeding.

Also confirm that the pods are connecting by checking the application logs:

```bash
kubectl logs -n library -l app=catalog --tail=20
```

You should see the Kafka consumer group join log lines without any TLS or connection errors. If the service previously logged `connected to broker b-1...` on startup, that line should still appear, now against port 9094.

---

[^1]: MSK encryption in transit documentation: https://docs.aws.amazon.com/msk/latest/developerguide/msk-encryption.html
[^2]: `aws_msk_cluster` Terraform resource reference—`encryption_info`: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/msk_cluster#encryption_info
[^3]: Amazon Trust Services root CA information: https://www.amazontrust.com/repository/
[^4]: Go `crypto/tls` package—system certificate pool behavior: https://pkg.go.dev/crypto/tls#Config
