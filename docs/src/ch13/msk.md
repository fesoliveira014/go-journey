# 13.5 Amazon MSK

Chapter 12 ran Kafka as a StatefulSet in the `messaging` namespace. That approach required you to manage KRaft quorum configuration, ordinal pod naming for stable advertised listeners, and persistent volume sizing yourself. Just as section 13.4 replaced the self-managed PostgreSQL StatefulSets with RDS, it is time to apply the same trade-off to Kafka: hand off the operational burden to a managed service and keep your focus on application logic.

Amazon Managed Streaming for Apache Kafka (MSK) is AWS's hosted Kafka offering. It runs the same Apache Kafka protocol — the same producer and consumer APIs, the same topic and partition model, the same consumer group semantics — so nothing in your application code changes. The difference is entirely operational: AWS provisions the broker nodes, handles KRaft controller elections, applies security patches, manages storage expansion, and exposes CloudWatch metrics without any configuration on your part.

---

## Why MSK Instead of a StatefulSet

A self-managed Kafka StatefulSet on EKS is viable, but it carries costs that compound over time.

**KRaft management.** Since Kafka 3.3, KRaft mode replaces ZooKeeper. Configuring a KRaft quorum inside Kubernetes requires coordinating controller and broker roles across multiple pods, ensuring stable network identities survive pod restarts, and managing the metadata topic manually. MSK handles all of this invisibly.

**Storage.** Kafka's log segments grow indefinitely unless retention policies purge them. On a StatefulSet, storage expansion means editing `volumeClaimTemplates`, which Kubernetes does not support in-place — you must delete and recreate the StatefulSet, during which time the cluster is unavailable unless you have replicas. MSK supports storage auto-scaling.

**Patching and upgrades.** Upgrading Kafka on a StatefulSet is a rolling-restart exercise with careful coordination between controller and broker nodes. MSK supports in-place rolling upgrades with no manifest changes.

For a learning system, the complexity of a production-quality Kafka StatefulSet exceeds what is worth writing yourself. MSK lets this chapter stay focused on what matters: configuring the cluster, connecting the services, and understanding the difference in how the bootstrap address changes.

---

## MSK Configuration Resource

MSK exposes a subset of Kafka server configuration through a separate `aws_msk_configuration` resource. This is distinct from the cluster resource itself and can be reused across clusters.

```hcl
# terraform/msk.tf

resource "aws_msk_configuration" "library" {
  name           = "library-kafka-config"
  kafka_versions = ["3.6.0"]
  description    = "Library system Kafka broker configuration"

  server_properties = <<-EOT
    auto.create.topics.enable=true
    default.replication.factor=2
    min.insync.replicas=1
    num.partitions=3
    log.retention.hours=168
  EOT
}
```

Each property is worth understanding before you apply it.

**`auto.create.topics.enable=true`** lets producers and consumers create topics on first use without an explicit admin call. This is convenient here because the application code controls which topics are accessed. In a multi-team environment you would disable this and manage topics through a topic registry or Terraform's `aws_msk_topic` resource.

**`default.replication.factor=2`** means every new topic will have two replicas — one on each broker. With two brokers, this is the maximum replication factor available. Replication protects against broker failure: if one broker goes down, the other holds a full copy of every partition.

**`min.insync.replicas=1`** sets the minimum number of replicas that must acknowledge a write before the producer considers it committed (when the producer uses `acks=all`). Setting this to 1 with two brokers means a single broker failure does not stall producers. The trade-off is that you lose the strict durability guarantee of requiring both replicas to confirm. For this system, where losing a small number of events on broker failure is acceptable, 1 is appropriate.

**`num.partitions=3`** is the default partition count for auto-created topics. Three partitions with two brokers means partitions are distributed across both brokers. Partition count sets the upper bound on consumer parallelism per consumer group — a consumer group with three members can process all three partitions in parallel.

**`log.retention.hours=168`** retains log segments for seven days. Events older than one week are eligible for deletion. This is the MSK default and suits the library system well: events are consumed by the search service within seconds of publication; seven days provides ample replay window for any catch-up scenario.

---

## The MSK Cluster Resource

```hcl
resource "aws_msk_cluster" "library" {
  cluster_name           = "library"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 2

  broker_node_group_info {
    instance_type   = "kafka.t3.small"
    client_subnets  = module.vpc.private_subnets
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
      client_broker = "PLAINTEXT"
      in_cluster    = true
    }
  }

  tags = {
    Project     = "library"
    Environment = "production"
  }
}
```


**`kafka.t3.small`** is the smallest available MSK instance type. It provides 2 GiB of memory per broker, which is sufficient for the library system's modest throughput. For a production workload you would size up to `kafka.m5.large` or larger and configure CloudWatch alarms on `KafkaDataLogsDiskUsed` and `MemoryUsed`.

**`number_of_broker_nodes = 2`** with two private subnets means MSK places one broker in each Availability Zone. This gives AZ-level fault tolerance: if one AZ becomes unavailable, the surviving broker continues serving producers and consumers, and replication catches up once the AZ recovers.

**`client_subnets = module.vpc.private_subnets`** places the brokers in private subnets with no internet-facing access. Only resources inside the VPC can reach the MSK cluster. Your EKS worker nodes are also in private subnets, so the traffic stays within the VPC.

**`client_broker = "PLAINTEXT"`** means traffic between your EKS pods and the MSK brokers is unencrypted on the wire. This is intentional for Chapter 13 — TLS between clients and brokers requires certificate management that is covered in Chapter 14. The `in_cluster = true` field, which is the default, encrypts replication traffic between brokers regardless of the client setting.

---

## MSK Security Group

The MSK cluster needs a security group that permits inbound connections on the Kafka plaintext port from the EKS worker nodes.

```hcl
resource "aws_security_group" "msk" {
  name        = "library-msk"
  description = "Allow Kafka access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Kafka plaintext from EKS nodes"
    from_port       = 9092
    to_port         = 9092
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
  }

  # Chapter 14 will add:
  # ingress {
  #   description     = "Kafka TLS from EKS nodes"
  #   from_port       = 9094
  #   to_port         = 9094
  #   protocol        = "tcp"
  #   security_groups = [aws_security_group.eks_nodes.id]
  # }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name    = "library-msk"
    Project = "library"
  }
}
```

Port 9092 is the Kafka plaintext listener. Port 9094 is the TLS listener that Chapter 14 will enable. The commented-out block is there to make the addition explicit when you reach that chapter. Keeping the two ports separate rather than opening both now avoids ambiguity about which listener your application is actually using.

---

## Outputs

```hcl
output "msk_bootstrap_brokers_plaintext" {
  description = "MSK plaintext bootstrap broker string"
  value       = aws_msk_cluster.library.bootstrap_brokers
}

output "msk_bootstrap_brokers_tls" {
  description = "MSK TLS bootstrap broker string (for Chapter 14)"
  value       = aws_msk_cluster.library.bootstrap_brokers_tls
}
```

`bootstrap_brokers` returns a comma-separated list of broker addresses on port 9092. After `terraform apply` completes you can inspect it directly:

```
$ terraform output msk_bootstrap_brokers_plaintext
"b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9092,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9092"
```

This string replaces the in-cluster address `kafka-0.kafka.messaging.svc.cluster.local:9092` that your ConfigMaps used in Chapter 12. The format is different — MSK uses hostname-based addressing rather than Kubernetes DNS — but from the perspective of the Kafka client library, both are bootstrap strings: the client connects to any listed broker to fetch the full cluster metadata, then routes subsequent requests accordingly. The application code does not change.

---

## Updating the ConfigMaps

In Chapter 12, every service that produces or consumes Kafka events read its broker address from an environment variable set in a Kubernetes ConfigMap:

```yaml
# base ConfigMap entry (Chapter 12)
KAFKA_BROKERS: kafka-0.kafka.messaging.svc.cluster.local:9092
```

The production overlay must patch this value. The Kustomize production overlay you built in Section 12.4 is the right place. Add a strategic merge patch for each service that uses Kafka:

```yaml
# deploy/k8s/overlays/production/catalog-configmap-patch.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalog-config
  namespace: library
data:
  KAFKA_BROKERS: "b-1.library.abc123.c2.kafka.us-east-1.amazonaws.com:9092,b-2.library.abc123.c2.kafka.us-east-1.amazonaws.com:9092"
```

Rather than hard-coding the bootstrap string — which changes every time you recreate the MSK cluster — drive it from a Terraform output. Write the bootstrap string to AWS Systems Manager Parameter Store as part of the Terraform apply:

```hcl
resource "aws_ssm_parameter" "kafka_brokers" {
  name  = "/library/production/KAFKA_BROKERS"
  type  = "String"
  value = aws_msk_cluster.library.bootstrap_brokers
}
```

Your CI/CD pipeline then reads the parameter and injects it into the Kustomize patch before applying the overlay:

```bash
KAFKA_BROKERS=$(aws ssm get-parameter \
  --name "/library/production/KAFKA_BROKERS" \
  --query "Parameter.Value" \
  --output text)

kubectl patch configmap catalog-config -n library \
  --patch "{\"data\":{\"KAFKA_BROKERS\":\"${KAFKA_BROKERS}\"}}"
```

The base manifests remain unchanged and continue to work in kind with the in-cluster address. Only the production overlay carries the MSK bootstrap string. Local development against kind keeps the Kafka StatefulSet; the MSK cluster is only reachable from inside the AWS VPC.

---

## What Changes and What Does Not

The move from a Kafka StatefulSet to MSK changes exactly one thing for your application code: the value of `KAFKA_BROKERS`. The Kafka client library does not know or care whether the bootstrap address is a Kubernetes pod FQDN or an MSK hostname. The protocol is identical. Consumer groups work the same way. Topic creation, message production, and offset management all behave the same way.

What changes is everything outside the application boundary. You no longer have `kubectl exec` access to a Kafka pod to run `kafka-topics.sh`. Instead, you use the AWS Management Console, the AWS CLI's `kafka` subcommand, or a client tool like `kcat` running inside a pod in the same VPC. MSK also publishes per-broker and per-topic metrics to CloudWatch automatically, without any JMX exporter configuration on your part.

The self-managed Kafka StatefulSet in `k8s/messaging/` can be removed from the production overlay once MSK is provisioned and the services are verified to be connecting successfully. Keep it in the base for local development — kind does not have access to your AWS VPC, and a lightweight in-cluster Kafka remains the right choice for `make dev`.

---

[^1]: MSK Kafka version support and broker instance types: https://docs.aws.amazon.com/msk/latest/developerguide/supported-kafka-versions.html
[^2]: `aws_msk_cluster` Terraform resource reference: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/msk_cluster
[^3]: `aws_msk_configuration` Terraform resource reference: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/msk_configuration
[^4]: MSK networking and security group requirements: https://docs.aws.amazon.com/msk/latest/developerguide/msk-vpc-inbound-traffic.html
[^5]: Kafka `min.insync.replicas` and producer durability semantics: https://kafka.apache.org/documentation/#replication
