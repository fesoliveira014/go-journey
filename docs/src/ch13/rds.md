# 13.4 Database: RDS for PostgreSQL

Chapter 12 ran three PostgreSQL instances as StatefulSets inside the cluster. That approach works and is reproducible, but it puts the operational burden of a database on you: you are responsible for backups, point-in-time recovery, engine patching, storage scaling, and high-availability failover. For a production workload, that is a significant ongoing commitment.

Amazon Relational Database Service (RDS) shifts that burden to AWS. It manages backups automatically, applies minor version patches during configurable maintenance windows, handles failover to a standby replica in a different Availability Zone, and exposes CloudWatch metrics without additional instrumentation. Your cluster stops running databases entirely — it connects to RDS endpoints the same way any application connects to a remote database.

This section replaces all three StatefulSets with RDS instances provisioned by Terraform. The application services do not change; only their `DATABASE_URL` environment variable changes.

---

## Why RDS Over StatefulSets in Production

StatefulSets are the right tool for running databases in environments where managed services are not available — on-premises clusters, air-gapped environments, cost-constrained setups. For AWS, the tradeoffs shift:

**Operational overhead.** A StatefulSet PostgreSQL instance requires you to manage `pg_basebackup` schedules, WAL archiving, backup retention policies, and restore procedures. RDS handles all of this with a few Terraform parameters.

**Durability.** RDS Multi-AZ keeps a synchronous standby replica in a second Availability Zone. If the primary instance fails or its AZ becomes unavailable, RDS promotes the standby automatically, typically within 60 to 120 seconds. Achieving the same durability with a StatefulSet requires replication configuration, health probes, and a failover controller such as Patroni.

**Patching.** Minor PostgreSQL releases (16.4 → 16.5) include security and bug fixes. RDS can apply these automatically during a configured maintenance window. Patching a StatefulSet means updating the image tag, triggering a rolling restart, and verifying replication state before and after.

**Storage.** RDS `gp3` volumes can be scaled independently of compute. A StatefulSet's PersistentVolumeClaim can be expanded on most StorageClasses, but the database must be restarted in some cases and the operation is less predictable.

The cost tradeoff runs the other way: a `db.t3.micro` RDS instance costs more per month than the compute and storage a StatefulSet would consume on a single worker node. For production systems handling real data, the operational savings almost always justify the cost difference.

---

## Terraform Configuration

All three databases are provisioned in a single file. A `for_each` on a locals map keeps the code DRY — adding a fourth database requires one line in `locals`, not another block of repeated HCL.

```hcl
# terraform/rds.tf

locals {
  databases = {
    catalog     = "catalog"
    auth        = "auth"
    reservation = "reservation"
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnet-group"
  subnet_ids = module.vpc.private_subnets

  tags = {
    Name    = "${var.project_name}-db-subnet-group"
    Project = var.project_name
  }
}

resource "aws_db_instance" "main" {
  for_each = local.databases

  identifier     = "${var.project_name}-${each.key}"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  db_name  = each.value
  username = "postgres"

  manage_master_user_password = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  backup_retention_period = 0
  skip_final_snapshot     = true

  tags = {
    Name    = "${var.project_name}-${each.key}"
    Project = var.project_name
  }
}
```

Walk through each parameter:

**`for_each = local.databases`** iterates over the map. Each iteration binds `each.key` to the logical name (`catalog`, `auth`, `reservation`) and `each.value` to the database name — which happens to be the same string here, but they are kept separate in the locals map to make that distinction explicit.

**`identifier`** is the RDS instance identifier visible in the AWS console and in DNS hostnames. The convention `${project_name}-${each.key}` produces `library-catalog`, `library-auth`, `library-reservation`.

**`engine_version = "16.4"`** pins the PostgreSQL minor version. RDS can be configured to auto-upgrade minor versions, but pinning avoids unexpected behavior during a deployment. Upgrade by updating this value and running `terraform apply`.

**`db_name`** is the name of the initial database created inside the instance. This corresponds to the `POSTGRES_DB` environment variable from the Chapter 12 ConfigMaps.

**`manage_master_user_password = true`** delegates password management to AWS Secrets Manager. This is covered in detail in the next section.

**`backup_retention_period = 0`** disables automated backups. For this project that is acceptable; for any real production system, set this to at least 7 days.

**`skip_final_snapshot = true`** allows `terraform destroy` to delete the instance without requiring a final snapshot. This is intentional for a learning project where you may tear down and rebuild frequently.

> **Warning:** `skip_final_snapshot = true` and `backup_retention_period = 0` are appropriate for development and experimentation. In production, set `backup_retention_period` to 7 or more and remove `skip_final_snapshot` (or set it to `false`) so that destroying the instance requires a deliberate extra step. Accidentally destroying a production database with no snapshot and no automated backups means permanent data loss.

**`db_subnet_group_name`** places the instances in the private subnets of the VPC created by the `vpc` module. RDS instances should never be in public subnets — they are only reachable from the EKS worker nodes in the same VPC.

**`vpc_security_group_ids`** restricts inbound connections to port 5432. The `rds` security group (defined in `security-groups.tf`) allows inbound traffic on port 5432 from the EKS worker node security group and nothing else.

---

## `manage_master_user_password` and AWS Secrets Manager

When `manage_master_user_password = true`, RDS does not accept a `password` argument and does not embed credentials in the Terraform state file. Instead, it generates a random password and stores it in AWS Secrets Manager as a JSON secret with the following structure:

```json
{
  "username": "postgres",
  "password": "...",
  "engine": "postgres",
  "host": "library-catalog.xxxx.us-east-1.rds.amazonaws.com",
  "port": 5432,
  "dbInstanceIdentifier": "library-catalog"
}
```

Secrets Manager can automatically rotate this password on a schedule you define, regenerating it and updating the stored value without downtime. Rotation is handled by a Lambda function that RDS integrates with directly[^3].

This is the modern approach to database credentials on AWS. The older pattern — writing a password into `terraform.tfvars` and passing it through `var.db_password` — embeds the credential in Terraform state, which is stored in S3. State is not encrypted by default, and it is accessible to anyone with S3 read permissions on the bucket. Secrets Manager keeps the credential out of state entirely.

**Retrieving the password manually** (for initial testing or debugging):

```bash
# List the secret ARN for the catalog database
aws secretsmanager list-secrets \
  --query "SecretList[?contains(Name,'library-catalog')].ARN" \
  --output text

# Retrieve and decode the secret value
aws secretsmanager get-secret-value \
  --secret-id <ARN> \
  --query SecretString \
  --output text | jq .
```

For now, manual retrieval is sufficient to verify connectivity. Chapter 14 removes this manual step entirely: the External Secrets Operator will watch Secrets Manager and automatically sync credentials into Kubernetes Secrets that the Deployment pods can reference directly.

---

## Updating `DATABASE_URL` for RDS

In Chapter 12, each application service's ConfigMap contained a `DATABASE_URL` pointing to the StatefulSet pod via cluster-internal DNS:

```
postgresql://postgres:$(POSTGRES_PASSWORD)@postgres-catalog-0.postgres-catalog.data.svc.cluster.local:5432/catalog?sslmode=disable
```

There are two things that must change for RDS:

1. **The hostname** is now the RDS endpoint, which looks like `library-catalog.xxxx.us-east-1.rds.amazonaws.com`. The exact value is a Terraform output, covered below.

2. **`sslmode=disable` becomes `sslmode=require`.** RDS enforces TLS for all connections by default. Attempting to connect without TLS results in a connection error. The `sslmode=require` parameter instructs the PostgreSQL driver to use TLS but skip certificate verification — sufficient for RDS, which presents a valid certificate from an AWS CA.

The production overlay in `deploy/k8s/overlays/production/` applies these changes using strategic merge patches on the relevant Deployments. A strategic merge patch for the catalog service looks like this:

```yaml
# deploy/k8s/overlays/production/catalog-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: library
spec:
  template:
    spec:
      containers:
        - name: catalog
          env:
            - name: DATABASE_URL
              value: "postgresql://postgres:$(POSTGRES_PASSWORD)@library-catalog.xxxx.us-east-1.rds.amazonaws.com:5432/catalog?sslmode=require"
```

The patch references the container by name (`name: catalog`). Kubernetes strategic merge patch semantics merge containers by name rather than replacing the entire list, so all other container fields — image, ports, resource limits, other environment variables — are preserved from the base[^4]. Only `DATABASE_URL` is overridden.

In the production overlay's `kustomization.yaml`:

```yaml
# deploy/k8s/overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

patches:
  - path: catalog-patch.yaml
  - path: auth-patch.yaml
  - path: reservation-patch.yaml
```

The RDS hostname contains a random identifier component generated by AWS. Terraform outputs (covered in the next section) give you the exact value to substitute into the patch files after the first `terraform apply`.

---

## Outputs

Add outputs to expose the RDS endpoints and the Secrets Manager ARNs. Both are needed when writing the Kustomize production overlays.

```hcl
# terraform/outputs.tf (additions)

output "rds_endpoints" {
  description = "RDS instance endpoints by service name"
  value = {
    for k, instance in aws_db_instance.main :
    k => instance.endpoint
  }
}

output "rds_secret_arns" {
  description = "Secrets Manager ARNs for RDS master user credentials"
  value = {
    for k, instance in aws_db_instance.main :
    k => instance.master_user_secret[0].secret_arn
  }
}
```

After applying, retrieve the outputs:

```bash
terraform output -json rds_endpoints
```

```json
{
  "auth": "library-auth.xxxx.us-east-1.rds.amazonaws.com:5432",
  "catalog": "library-catalog.xxxx.us-east-1.rds.amazonaws.com:5432",
  "reservation": "library-reservation.xxxx.us-east-1.rds.amazonaws.com:5432"
}
```

```bash
terraform output -json rds_secret_arns
```

```json
{
  "auth": "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-...",
  "catalog": "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-...",
  "reservation": "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-..."
}
```

The `rds_endpoints` values go into the `DATABASE_URL` environment variable in the production Kustomize patches. The `rds_secret_arns` values go into the External Secrets resources in Chapter 14, which automate credential injection.

Note that `endpoint` in the Terraform `aws_db_instance` resource includes the port (`hostname:5432`). Depending on how your `DATABASE_URL` is assembled, you may want to use `address` (hostname only) instead. The `address` attribute is also available on `aws_db_instance` and omits the port suffix[^2].

---

## What Changed

| Component | Chapter 12 | Chapter 13 |
|-----------|-----------|-----------|
| Database host | `postgres-catalog-0.postgres-catalog.data.svc.cluster.local` | `library-catalog.xxxx.us-east-1.rds.amazonaws.com` |
| Database process | StatefulSet pod in `data` namespace | RDS managed instance |
| Credentials | Kubernetes Secret (plaintext in Kustomize overlay) | AWS Secrets Manager (managed rotation) |
| Backups | Manual or not configured | RDS automated (when `backup_retention_period > 0`) |
| Failover | Manual | RDS Multi-AZ (when `multi_az = true`) |
| SSL | `sslmode=disable` | `sslmode=require` |

The application code changes nothing. The Go database clients in each service — using `pgx` or `database/sql` — connect to a PostgreSQL endpoint. Whether that endpoint is a StatefulSet pod or an RDS instance is opaque to the application layer; only the connection string differs.

---

[^1]: AWS RDS PostgreSQL engine versions: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_PostgreSQL.html
[^2]: Terraform `aws_db_instance` resource reference: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/db_instance
[^3]: AWS Secrets Manager automatic rotation for RDS: https://docs.aws.amazon.com/secretsmanager/latest/userguide/rotate-secrets_turn-on-for-db.html
[^4]: Kustomize strategic merge patches: https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/
