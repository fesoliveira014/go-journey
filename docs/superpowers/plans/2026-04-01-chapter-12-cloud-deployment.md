# Chapter 12: Cloud Deployment (EKS) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write Chapter 12 documentation (11 sections) and implement the Terraform infrastructure code, production Kustomize overlay, and CI/CD pipeline updates needed to deploy the library system to AWS EKS.

**Architecture:** Documentation sections first (12.1-12.11), then Terraform files (provider, VPC, ECR, RDS, MSK, EKS, CI/CD), then base restructuring (move local-only infra to separate directory), then production Kustomize overlay, then CI/CD workflow updates. Documentation tasks are independent and can be parallelized. Terraform file tasks are independent. Base restructuring must precede the production overlay. CI/CD workflow updates depend on the Terraform cicd.tf (for IAM role ARN references).

**Tech Stack:** Terraform (AWS provider), Amazon EKS, RDS, MSK, ECR, Kustomize, GitHub Actions, AWS Load Balancer Controller, OIDC federation

---

## File Structure

### Documentation (new)
- `docs/src/ch12/index.md` — From local to cloud: architecture diagram, concept mapping, cost awareness, prerequisites
- `docs/src/ch12/terraform-fundamentals.md` — Terraform primer: IaC concepts, HCL syntax, state, plan/apply lifecycle
- `docs/src/ch12/networking.md` — VPC, subnets, NAT gateway, security groups, subnet tagging
- `docs/src/ch12/ecr.md` — ECR repositories, lifecycle policies, image tagging strategy
- `docs/src/ch12/rds.md` — RDS for PostgreSQL: 3 instances, credentials, security groups
- `docs/src/ch12/msk.md` — Amazon MSK: KRaft mode, broker config, auto-create topics
- `docs/src/ch12/eks.md` — EKS cluster, managed node groups, AWS LB Controller, IRSA
- `docs/src/ch12/production-overlay.md` — Production Kustomize overlay: patches, images, ALB ingress
- `docs/src/ch12/cicd.md` — CI/CD pipeline: OIDC federation, ECR push, EKS deploy
- `docs/src/ch12/deploying.md` — Deploy walkthrough, verification, troubleshooting, teardown
- `docs/src/ch12/argocd.md` — GitOps alternative: ArgoCD concepts, pros/cons, architecture
- `docs/src/SUMMARY.md` — Add Chapter 12 entries (modify)

### Terraform (new)
- `terraform/main.tf` — Provider config (AWS, Helm, Kubernetes), backend config (local + S3 commented), data sources
- `terraform/variables.tf` — Input variables: region, project name, instance types, CIDR blocks
- `terraform/outputs.tf` — Output values: ECR URLs, RDS endpoints, MSK brokers, EKS cluster details
- `terraform/vpc.tf` — VPC module, public/private subnets, NAT, IGW, subnet tags
- `terraform/ecr.tf` — 5 ECR repositories with lifecycle policies
- `terraform/rds.tf` — DB subnet group, security group, 3 RDS instances
- `terraform/msk.tf` — MSK configuration, security group, MSK cluster
- `terraform/eks.tf` — EKS module, managed node group, EBS CSI driver, AWS LB Controller (Helm), IRSA
- `terraform/cicd.tf` — GitHub Actions OIDC provider, IAM role with trust policy, EKS access entry

### Kubernetes manifests (modified)
- `deploy/k8s/base/local-infra/kustomization.yaml` — New directory for local-only Postgres + Kafka resources
- `deploy/k8s/base/local-infra/data/` — Postgres StatefulSets/Services/ConfigMaps moved from `base/data/`
- `deploy/k8s/base/local-infra/messaging/` — Kafka StatefulSet/Service/ConfigMap moved from `base/messaging/`
- `deploy/k8s/base/data/kustomization.yaml` — Reduced to Meilisearch + namespace + secrets only
- `deploy/k8s/base/messaging/kustomization.yaml` — Reduced to namespace only
- `deploy/k8s/base/kustomization.yaml` — Unchanged (still references data, messaging, library)
- `deploy/k8s/overlays/local/kustomization.yaml` — Add `../../base/local-infra` to resources
- `deploy/k8s/overlays/production/kustomization.yaml` — Complete production overlay

### CI/CD (modified)
- `.github/workflows/main.yml` — Add deploy job: OIDC auth, ECR push, kustomize set image, kubectl apply
- `.github/workflows/pr.yml` — Add optional terraform plan step

---

## Context for All Tasks

**Writing style:** This is an educational book for an experienced engineer (7+ years C/C++/Kotlin/Java) learning Go and cloud-native tooling. Chapters use a narrative-driven style: open with the problem, map new concepts to familiar ones, walk through code with explanations, include comparison tables, and close with verification and troubleshooting. See `docs/src/ch11/index.md` for style reference.

**Existing infrastructure:** Chapter 11 deployed everything to a local `kind` cluster. The base K8s manifests are in `deploy/k8s/base/` across 3 namespaces: `library` (5 app Deployments), `data` (3 Postgres StatefulSets + Meilisearch), `messaging` (Kafka). The local overlay in `deploy/k8s/overlays/local/` provides secretGenerator with dev credentials.

**Key env var patterns in existing Deployments:**
- `DATABASE_URL` is a hardcoded `value` in the Deployment env block (not from ConfigMap), using `$(POSTGRES_PASSWORD)` substitution pointing to StatefulSet DNS names
- `KAFKA_BROKERS` is in ConfigMaps (`catalog-config`, `reservation-config`, `search-config`), pointing to `kafka-0.kafka.messaging.svc.cluster.local:9092`
- `MEILI_URL` is in `search-config` ConfigMap pointing to `meilisearch.data.svc.cluster.local:7700`
- Service-to-service gRPC addresses (`CATALOG_GRPC_ADDR`, etc.) are in ConfigMaps and remain unchanged (they use `library` namespace DNS which stays the same)

---

### Task 1: Write Section 12.1 — From Local to Cloud

**Files:**
- Create: `docs/src/ch12/index.md`

- [ ] **Step 1: Create the chapter index file**

Write `docs/src/ch12/index.md` (~150-200 lines) covering:

1. Opening paragraph: Chapter 11 deployed to a local kind cluster — containers running, probes passing, Ingress routing. But kind is a development tool, not a production platform. This chapter takes the same application to AWS.

2. Architecture diagram (Mermaid): Show the target state — EKS cluster containing 5 app Deployments + Meilisearch StatefulSet in the cluster, managed RDS (3 instances) and MSK outside the cluster connected via VPC networking, ALB in front routing to gateway, ECR as image source, GitHub Actions CI/CD pipeline pushing images.

3. Concept mapping table — map kind/local concepts to AWS equivalents:

| Local (kind) | AWS | Purpose |
|--------------|-----|---------|
| kind cluster | Amazon EKS | Kubernetes control plane + nodes |
| `kind load docker-image` | Amazon ECR | Container image storage |
| NGINX Ingress Controller | AWS Load Balancer Controller + ALB | External traffic routing |
| Postgres StatefulSets | Amazon RDS | Managed PostgreSQL |
| Kafka StatefulSet | Amazon MSK | Managed Kafka |
| `kubectl apply` from laptop | GitHub Actions + OIDC | Automated deployment |
| Kustomize local overlay | Kustomize production overlay | Environment-specific config |

4. What changes vs what stays the same: emphasize that the base K8s manifests from Chapter 11 are untouched. Only the overlay and infrastructure differ. This is the Kustomize payoff promised in section 11.5.

5. Cost awareness section: estimated costs per component:
   - EKS control plane: ~$0.10/hr (~$73/month)
   - 2x t3.medium nodes: ~$0.042/hr each (~$61/month total)
   - 3x RDS db.t3.micro: ~$0.018/hr each (~$39/month total)
   - MSK kafka.t3.small (2 brokers): ~$0.05/hr each (~$73/month total)
   - NAT Gateway: ~$0.045/hr + data (~$33/month)
   - **Total estimate: ~$280/month if left running**
   - Emphasize `terraform destroy` when done experimenting.

6. Prerequisites: AWS account with admin access, AWS CLI v2 installed and configured (`aws configure`), Terraform >= 1.5 installed, `kubectl` (already from Chapter 11).

- [ ] **Step 2: Verify the file renders correctly**

Run: `head -5 docs/src/ch12/index.md`
Expected: Chapter title and opening paragraph visible.

- [ ] **Step 3: Commit**

```bash
git add docs/src/ch12/index.md
git commit -m "docs: add Chapter 12 introduction — from local to cloud"
```

---

### Task 2: Write Section 12.2 — Terraform Fundamentals

**Files:**
- Create: `docs/src/ch12/terraform-fundamentals.md`

- [ ] **Step 1: Create the Terraform primer**

Write `docs/src/ch12/terraform-fundamentals.md` (~200-250 lines) covering:

1. Opening: Kubernetes tells us *what* to run (Deployments, Services). But *where* does the cluster itself run? Who creates the VPC, the database, the load balancer? Infrastructure as Code (IaC) tools answer this — they let you describe cloud resources in files, version-control them, and apply changes reproducibly.

2. What Terraform is: HashiCorp's open-source IaC tool. Declarative — you describe the desired state in `.tf` files (HCL syntax), and Terraform figures out how to get there. Analogous to Kubernetes' declarative model but for infrastructure rather than workloads.

3. Core concepts, each with a brief code example:
   - **Providers:** Plugins that talk to cloud APIs. `provider "aws" { region = var.region }`.
   - **Resources:** Individual infrastructure objects. `resource "aws_vpc" "main" { cidr_block = "10.0.0.0/16" }`.
   - **Data sources:** Read-only queries. `data "aws_caller_identity" "current" {}`.
   - **Variables:** Input parameters. `variable "region" { default = "us-east-1" }`.
   - **Outputs:** Values exported for other tools. `output "vpc_id" { value = aws_vpc.main.id }`.
   - **Modules:** Reusable packages of resources. `module "vpc" { source = "terraform-aws-modules/vpc/aws" }`.
   - **State:** Terraform's record of what it created — a JSON file that maps HCL resources to real infrastructure. Explain the plan/apply/destroy lifecycle.

4. State management:
   - Local state: `terraform.tfstate` file in the working directory. Fine for learning.
   - Remote state (S3 + DynamoDB): show the backend config but comment it out. Explain why it matters for teams (locking, shared state, crash recovery). We use local state in this chapter.

5. Project structure for this chapter (match the spec):
   ```
   terraform/
     main.tf          # provider config, backend
     variables.tf     # input variables
     outputs.tf       # values needed by Kustomize/CI
     vpc.tf           # networking
     ecr.tf           # container registry
     rds.tf           # Postgres instances
     msk.tf           # Kafka cluster
     eks.tf           # EKS cluster + node group
     cicd.tf          # GitHub Actions OIDC provider + IAM role
   ```

6. The Terraform workflow:
   - `terraform init` — downloads providers and modules.
   - `terraform plan` — shows what Terraform will create/change/destroy. Always review before applying.
   - `terraform apply` — creates the infrastructure. Takes ~15-20 minutes for our full stack.
   - `terraform destroy` — tears everything down. **Always run this when done to avoid ongoing charges.**

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/terraform-fundamentals.md
git commit -m "docs: add Terraform fundamentals section"
```

---

### Task 3: Write Section 12.3 — Networking

**Files:**
- Create: `docs/src/ch12/networking.md`

- [ ] **Step 1: Create the networking section**

Write `docs/src/ch12/networking.md` (~200-250 lines) covering:

1. Opening: Every AWS resource in this chapter lives inside a Virtual Private Cloud (VPC). A VPC is your own isolated network within AWS — think of it as the private LAN your laptop is connected to, but in the cloud. EKS nodes, RDS databases, and MSK brokers all need IP addresses, subnets, and routing rules.

2. VPC design diagram (Mermaid): Show the VPC with 2 AZs, each containing 1 public + 1 private subnet. Internet Gateway on public side, NAT Gateway in public subnet, EKS nodes + RDS + MSK in private subnets, ALB in public subnets.

3. Design rationale:
   - 2 Availability Zones — minimum for RDS Multi-AZ and MSK. Not 3 to keep costs down for a learning project.
   - Public subnets: ALB and NAT Gateway live here. ALB routes external traffic to pods. NAT Gateway lets nodes in private subnets reach the internet (to pull images from ECR, reach AWS APIs).
   - Private subnets: EKS nodes, RDS instances, MSK brokers. Not directly reachable from the internet.
   - CIDR: `10.0.0.0/16` — gives 65,536 IPs, more than enough.

4. Full `terraform/vpc.tf` code using the `terraform-aws-modules/vpc/aws` module. Explain every parameter:
   - `name`, `cidr`, `azs` (use `data.aws_availability_zones`).
   - `private_subnets`, `public_subnets` CIDR blocks.
   - `enable_nat_gateway = true`, `single_nat_gateway = true` (cost saving — one NAT gateway instead of one per AZ).
   - `enable_dns_hostnames = true`, `enable_dns_support = true` (required for EKS).
   - `public_subnet_tags` with `"kubernetes.io/role/elb" = 1` — tells the AWS LB Controller to place ALBs in these subnets.
   - `private_subnet_tags` with `"kubernetes.io/role/internal-elb" = 1` — for internal load balancers.
   - Tags with `"kubernetes.io/cluster/${var.cluster_name}" = "shared"`.

5. Security groups section:
   - RDS security group: `aws_security_group` + `aws_security_group_rule` allowing inbound TCP 5432 from the EKS node security group.
   - MSK security group: same pattern, allowing inbound TCP 9092 from EKS node security group. Note: port 9094 for TLS added in Chapter 13.
   - Explain the pattern: infrastructure is only reachable from inside the cluster, never directly from the internet.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/networking.md
git commit -m "docs: add VPC and networking section"
```

---

### Task 4: Write Section 12.4 — ECR

**Files:**
- Create: `docs/src/ch12/ecr.md`

- [ ] **Step 1: Create the ECR section**

Write `docs/src/ch12/ecr.md` (~100-150 lines) covering:

1. Opening: In Chapter 9, CI pushed images to GitHub Container Registry (GHCR). For EKS, we use Amazon ECR — AWS's native container registry. EKS nodes authenticate to ECR automatically via their IAM role, so there are no image pull secrets to manage. It is the path of least resistance for an AWS-native workflow.

2. Full `terraform/ecr.tf` code:
   - Use a `locals` block with a list of service names: `["auth", "catalog", "gateway", "reservation", "search"]`.
   - `aws_ecr_repository` resource using `for_each = toset(local.services)`:
     - `name = "library-system/${each.key}"` — matches the image names in base K8s manifests.
     - `image_tag_mutability = "MUTABLE"` — allows overwriting `:latest` tag.
     - `image_scanning_configuration { scan_on_push = true }` — free vulnerability scanning.
   - `aws_ecr_lifecycle_policy` per repository: expire untagged images after 14 days, keep only the last 20 tagged images. Prevents unbounded storage costs.

3. Image tagging strategy:
   - CI pushes two tags per image: `sha-<commit-hash>` (immutable, for traceability) and `latest` (convenience).
   - Full URI: `<account-id>.dkr.ecr.<region>.amazonaws.com/library-system/catalog:sha-abc1234`.
   - The Kustomize production overlay uses the `images` transformer to rewrite `library-system/catalog:latest` → the ECR URI.

4. Outputs: ECR repository URLs exported for use in CI/CD and Kustomize. Show the `outputs.tf` entries.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/ecr.md
git commit -m "docs: add ECR container registry section"
```

---

### Task 5: Write Section 12.5 — RDS

**Files:**
- Create: `docs/src/ch12/rds.md`

- [ ] **Step 1: Create the RDS section**

Write `docs/src/ch12/rds.md` (~200-250 lines) covering:

1. Opening: Chapter 11 ran PostgreSQL as StatefulSets inside the cluster. That taught us StatefulSet concepts — stable network identities, persistent volumes, headless Services. For production, we hand database operations to AWS. Amazon RDS for PostgreSQL provides automated backups, patching, monitoring, and optional Multi-AZ failover — things you would otherwise have to build and maintain yourself.

2. Full `terraform/rds.tf` code:
   - `aws_db_subnet_group` using private subnets from the VPC module.
   - Use a `locals` block with service database names: `{ catalog = "catalog", auth = "auth", reservation = "reservation" }`.
   - `aws_db_instance` resource using `for_each`:
     - `identifier = "library-${each.key}"`.
     - `engine = "postgres"`, `engine_version = "16.4"`.
     - `instance_class = "db.t3.micro"` — free tier eligible.
     - `allocated_storage = 20`, `storage_type = "gp3"`.
     - `db_name = each.value` — creates the database automatically.
     - `username = "postgres"`.
     - `manage_master_user_password = true` — AWS creates and rotates the password in Secrets Manager. Explain this is the modern approach: no password in Terraform state, automatic rotation, retrieval via `aws secretsmanager get-secret-value`.
     - `vpc_security_group_ids` referencing the RDS security group.
     - `db_subnet_group_name`.
     - `skip_final_snapshot = true` — **learning project only**. In production, you always want a final snapshot before deletion. Call this out prominently.
     - `backup_retention_period = 0` — disables automated backups to reduce cost for learning. Note the production default (7 days).
   - Explain the security group reference: RDS instances accept connections only from the EKS node security group, which is created by the EKS module.

3. Key differences from Chapter 11:
   - `DATABASE_URL` changes from `host=postgres-catalog-0.postgres-catalog.data.svc.cluster.local` to `host=library-catalog.xxxx.us-east-1.rds.amazonaws.com`. The production overlay patches this via a Deployment strategic merge patch.
   - Password management: instead of a Kustomize `secretGenerator` literal, the password lives in AWS Secrets Manager. For now, the reader retrieves it manually; Chapter 13 automates this with external-secrets operator.

4. Outputs: RDS endpoints and Secrets Manager ARNs. Show the `outputs.tf` entries.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/rds.md
git commit -m "docs: add RDS for PostgreSQL section"
```

---

### Task 6: Write Section 12.6 — MSK

**Files:**
- Create: `docs/src/ch12/msk.md`

- [ ] **Step 1: Create the MSK section**

Write `docs/src/ch12/msk.md` (~150-200 lines) covering:

1. Opening: Like PostgreSQL, Kafka moves from an in-cluster StatefulSet to a managed service. Amazon MSK (Managed Streaming for Apache Kafka) runs the same Kafka we used in Docker Compose and kind — same protocol, same consumer group mechanics, same topic model. The difference is AWS handles broker provisioning, storage scaling, patching, and KRaft controller management.

2. MSK configuration resource:
   - `aws_msk_configuration` with `server_properties`:
     - `auto.create.topics.enable = true` — our services auto-create topics on startup.
     - `default.replication.factor = 2` — match the 2-broker setup.
     - `min.insync.replicas = 1` — learning project; production would use 2.
     - `num.partitions = 3` — default partition count.

3. Full `terraform/msk.tf` code:
   - `aws_msk_cluster` resource:
     - `cluster_name = "${var.project_name}-kafka"`.
     - `kafka_version = "3.6.0"` — or latest supported version.
     - `number_of_broker_nodes = 2` — one per AZ.
     - `broker_node_group_info`: `instance_type = "kafka.t3.small"`, `ebs_volume_size = 10`, `client_subnets` from VPC private subnets, `security_groups` referencing MSK SG.
     - `configuration_info` referencing the MSK configuration resource.
     - `encryption_info`: `encryption_in_transit { client_broker = "PLAINTEXT" }` — plaintext for local parity. Add comment: "Chapter 13 changes this to TLS and updates service configs to use port 9094."
   - Explain the security group: inbound 9092 from EKS node SG (port 9094 for TLS deferred to Chapter 13).

4. Key difference from Chapter 11:
   - `KAFKA_BROKERS` in ConfigMaps changes from `kafka-0.kafka.messaging.svc.cluster.local:9092` to the MSK bootstrap broker string (e.g., `b-1.library-kafka.xxxx.kafka.us-east-1.amazonaws.com:9092,b-2.library-kafka.xxxx.kafka.us-east-1.amazonaws.com:9092`). The production overlay patches the ConfigMaps.

5. Outputs: bootstrap broker strings (plaintext). Show `outputs.tf` entries.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/msk.md
git commit -m "docs: add Amazon MSK section"
```

---

### Task 7: Write Section 12.7 — EKS

**Files:**
- Create: `docs/src/ch12/eks.md`

- [ ] **Step 1: Create the EKS section**

Write `docs/src/ch12/eks.md` (~300-350 lines) covering:

1. Opening: This is the core resource — the Kubernetes cluster itself. Amazon EKS manages the control plane (API server, etcd, scheduler, controller manager). You provide the worker nodes via managed node groups — EC2 instances that EKS automatically registers, monitors, and can update.

2. EKS architecture diagram (Mermaid): AWS-managed control plane on the left, your VPC with managed node group on the right, connected by ENIs. Show the OIDC provider connecting to IAM for IRSA.

3. Full `terraform/eks.tf` code using `terraform-aws-modules/eks/aws` module:
   - `cluster_name = var.cluster_name`, `cluster_version = "1.29"`.
   - `vpc_id = module.vpc.vpc_id`, `subnet_ids = module.vpc.private_subnets`.
   - `cluster_endpoint_public_access = true` (for `kubectl` from developer's machine).
   - `cluster_endpoint_private_access = true` (for node-to-API-server communication).
   - `cluster_addons`: `coredns`, `kube-proxy`, `vpc-cni`, `aws-ebs-csi-driver` (explain each — EBS CSI is needed for Meilisearch PVC with `gp2` default StorageClass).
   - `eks_managed_node_groups` block:
     - `name = "default"`.
     - `instance_types = ["t3.medium"]` — 2 vCPU, 4GB RAM.
     - `min_size = 1`, `max_size = 3`, `desired_size = 2`.
     - Explain: the EKS module automatically attaches the standard node IAM policies (`AmazonEKSWorkerNodePolicy`, `AmazonEKS_CNI_Policy`, `AmazonEC2ContainerRegistryReadOnly`). List them for educational clarity.
   - `access_entries` block: configure EKS access entries for the current IAM user/role (the modern API, preferred over the legacy `aws-auth` ConfigMap). Explain: access entries are Terraform-managed, auditable, and don't require modifying a ConfigMap inside the cluster.

4. AWS Load Balancer Controller section:
   - What it does: watches Ingress resources and provisions ALBs in AWS. Replaces the NGINX Ingress Controller from Chapter 11.
   - IRSA (IAM Roles for Service Accounts): the controller runs as a pod but needs AWS permissions to create ALBs. IRSA lets you bind an IAM role to a Kubernetes ServiceAccount without giving every pod on the node those permissions.
   - Show the Helm release for the LB controller:
     - `helm_release` resource with `repository = "https://aws.github.io/eks-charts"`, `chart = "aws-load-balancer-controller"`.
     - ServiceAccount annotation with the IRSA role ARN.
   - IRSA IAM role: `aws_iam_role` with OIDC trust policy, `aws_iam_role_policy_attachment` for the LB controller policy.

5. Connecting to the cluster:
   - `aws eks update-kubeconfig --name <cluster-name> --region <region>`.
   - Verify: `kubectl get nodes` — should show 2 `t3.medium` nodes in `Ready` state.

6. Outputs: cluster name, endpoint, CA certificate, OIDC provider ARN, OIDC provider URL.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/eks.md
git commit -m "docs: add EKS cluster section"
```

---

### Task 8: Write Section 12.8 — Production Overlay

**Files:**
- Create: `docs/src/ch12/production-overlay.md`

- [ ] **Step 1: Create the production overlay section**

Write `docs/src/ch12/production-overlay.md` (~300-350 lines) covering:

1. Opening: Chapter 11 introduced Kustomize's base/overlay pattern. The base manifests describe *what* runs. Overlays describe *where* and *how*. The local overlay configured kind-specific settings — `imagePullPolicy: IfNotPresent`, dev secrets, single replicas. Now we write the production overlay for EKS, and the base manifests remain untouched. This is the payoff of the design decision made in section 11.5.

2. Base restructuring explanation:
   - Problem: the base includes Postgres and Kafka StatefulSets that production doesn't need (RDS and MSK replace them). We don't want to delete-patch resources — that's fragile and hard to read.
   - Solution: move local-only infrastructure (Postgres StatefulSets/Services/ConfigMaps, Kafka StatefulSet/Service/ConfigMap) to a new `deploy/k8s/base/local-infra/` directory. The local overlay includes this directory as a resource; the production overlay does not.
   - Show the before/after directory structure.
   - The `data` namespace directory keeps: `namespace.yaml`, Meilisearch (StatefulSet, Service, ConfigMap), `secrets.yaml`.
   - The `messaging` namespace directory keeps: `namespace.yaml` only.
   - The `local-infra` directory gets: Postgres (3x StatefulSet, Service, ConfigMap) under `local-infra/data/`, Kafka (StatefulSet, Service, ConfigMap) under `local-infra/messaging/`.

3. Full production overlay `kustomization.yaml`:
   - `resources: [../../base]` — does NOT include `../../base/local-infra`.
   - `images` transformer block: one entry per service rewriting `library-system/<svc>` → ECR URI. Explain that CI dynamically sets the `newTag` via `kustomize edit set image`.
   - `patches` — strategic merge patches:
     - **Replica patches:** 2 replicas for each of the 5 app Deployments. Show one JSON patch targeting `spec.replicas`.
     - **Resource limit patches:** increase CPU to 250m-500m and memory to 256Mi-512Mi. Show one strategic merge patch.
     - **`imagePullPolicy` patch:** set `Always` for all containers (ECR images should always be pulled fresh to get latest tag).
     - **`DATABASE_URL` patches:** one per database-backed service (catalog, auth, reservation). Strategic merge patch on the Deployment's `env` block, replacing the `value` field with the RDS endpoint. Show the full patch for catalog:
       ```yaml
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
                     value: "host=${RDS_CATALOG_ENDPOINT} port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=catalog sslmode=require"
       ```
       Note: `${RDS_CATALOG_ENDPOINT}` is a placeholder the reader substitutes with the Terraform output. `sslmode=require` because RDS enforces SSL by default.
     - **ConfigMap patches:** patch `KAFKA_BROKERS` in catalog-config, reservation-config, search-config to use the MSK bootstrap string.
   - **Ingress patch:** replace `ingressClassName: nginx` with `ingressClassName: alb` and add ALB annotations:
     ```yaml
     alb.ingress.kubernetes.io/scheme: internet-facing
     alb.ingress.kubernetes.io/target-type: ip
     alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}]'
     ```
     Remove the `host: library.local` rule (use ALB DNS name directly). Note: TLS on the ALB is covered in Chapter 13.
   - **Secrets:** `secretGenerator` with placeholder values for `jwt-secret`, `postgres-*-secret`, `meilisearch-secret` in the `library` namespace. Prominent comment that Chapter 13 replaces this with external-secrets operator. No `data`-namespace secrets needed (RDS manages its own credentials; Meilisearch secret only needed in `library` for the search service's `MEILI_MASTER_KEY`).
   - `generatorOptions: disableNameSuffixHash: true`.

4. StorageClass note: EKS with the `aws-ebs-csi-driver` add-on provides a default `gp2` StorageClass. The Meilisearch StatefulSet's PVC works without changes. For production, `gp3` is cheaper and faster — can be set via a StorageClass resource in the overlay, but deferred for simplicity.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/production-overlay.md
git commit -m "docs: add production Kustomize overlay section"
```

---

### Task 9: Write Section 12.9 — CI/CD Pipeline

**Files:**
- Create: `docs/src/ch12/cicd.md`

- [ ] **Step 1: Create the CI/CD section**

Write `docs/src/ch12/cicd.md` (~250-300 lines) covering:

1. Opening: Chapter 9 built a CI/CD pipeline that tests code and pushes images to GHCR. Now we extend it: images go to ECR, and successful builds deploy to EKS. The key challenge is authentication — GitHub Actions needs AWS permissions without storing long-lived credentials.

2. OIDC federation explanation:
   - What it is: GitHub Actions can present a short-lived JWT (OIDC token) proving "I am a workflow run in repo X on branch Y." AWS verifies this token against a registered OIDC provider and grants temporary credentials scoped to an IAM role.
   - Why it's better than secrets: no `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` stored in GitHub settings. Tokens are short-lived, scoped to a single workflow run, and cannot be exfiltrated.
   - Diagram: GitHub Actions → OIDC token → AWS STS AssumeRoleWithWebIdentity → temporary credentials → ECR/EKS access.

3. Full `terraform/cicd.tf` code:
   - `aws_iam_openid_connect_provider` for `token.actions.githubusercontent.com`.
   - `aws_iam_role` with trust policy: `Principal` is the OIDC provider, `Condition` restricts to `repo:<owner>/<repo>:ref:refs/heads/main`.
   - `aws_iam_role_policy` with inline policy allowing:
     - `ecr:GetAuthorizationToken`, `ecr:BatchCheckLayerAvailability`, `ecr:PutImage`, etc.
     - `eks:DescribeCluster`, `eks:AccessKubernetesApi` (for kubectl).
   - `aws_eks_access_entry` + `aws_eks_access_policy_association` for the CI/CD role — grants the role `AmazonEKSClusterAdminPolicy` access to the cluster via the modern EKS access entries API.

4. Updated `main.yml` workflow:
   - Show the complete updated file. Key changes:
     - Add `id-token: write` to permissions (required for OIDC).
     - New `deploy` job (runs after `build-and-push`):
       1. `aws-actions/configure-aws-credentials@v4` with `role-to-assume`, `aws-region`.
       2. `aws-actions/amazon-ecr-login@v2`.
       3. Build + push to ECR (same `docker/build-push-action` but targeting ECR URIs).
       4. `kustomize edit set image` for each service in the production overlay.
       5. `kubectl apply -k deploy/k8s/overlays/production`.
       6. `kubectl rollout status deployment/<svc> -n library --timeout=300s` for each service.
   - Note: the existing GHCR push job can remain for backwards compatibility, or be removed. Keep it for now.

5. Optional `pr.yml` addition: `terraform plan` step that outputs the plan as a PR comment. Show the workflow snippet but mark it as optional — requires the OIDC role to have read-only Terraform permissions.

6. Rollback strategy:
   - Manual: `kubectl rollout undo deployment/catalog -n library` reverts to the previous ReplicaSet.
   - Image-based: previous images with `sha-<commit>` tags are still in ECR. Re-deploy by setting the old tag.
   - Note: ArgoCD (section 12.11) provides more sophisticated rollback via `git revert`.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/cicd.md
git commit -m "docs: add CI/CD pipeline section"
```

---

### Task 10: Write Section 12.10 — Deploying and Verifying

**Files:**
- Create: `docs/src/ch12/deploying.md`

- [ ] **Step 1: Create the deploy walkthrough**

Write `docs/src/ch12/deploying.md` (~200-250 lines) covering:

1. Opening: This section walks through deploying the full stack to AWS. If you have an AWS account and want to try it, follow along. If not, read through — the expected outputs at each step are shown so you can understand the flow without running it.

2. Step-by-step deployment:

   **Step 1: Initialize and apply Terraform (~15-20 minutes)**
   ```bash
   cd terraform
   terraform init
   terraform plan -out=tfplan
   # Review the plan output — it should show ~30-40 resources to create
   terraform apply tfplan
   ```
   Show example Terraform output with resource counts.

   **Step 2: Configure kubectl**
   ```bash
   aws eks update-kubeconfig --name library-system --region us-east-1
   kubectl get nodes
   ```
   Expected: 2 nodes in `Ready` state.

   **Step 3: Verify infrastructure**
   ```bash
   # ECR repos exist
   aws ecr describe-repositories --query 'repositories[].repositoryName'
   # RDS instances available
   aws rds describe-db-instances --query 'DBInstances[].[DBInstanceIdentifier,DBInstanceStatus]'
   # MSK cluster active
   aws kafka list-clusters --query 'ClusterInfoList[].[ClusterName,State]'
   ```

   **Step 4: Retrieve RDS credentials**
   ```bash
   # Get the Secrets Manager ARN from Terraform output
   terraform output rds_master_password_secret_arns
   # Retrieve the password
   aws secretsmanager get-secret-value --secret-id <arn> --query 'SecretString' --output text
   ```
   Substitute the password into `deploy/k8s/overlays/production/kustomization.yaml`'s secretGenerator. Note: Chapter 13 automates this with external-secrets operator.

   **Step 5: Push images to ECR**
   ```bash
   # Login to ECR
   aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <account-id>.dkr.ecr.us-east-1.amazonaws.com
   # Tag and push each service
   for svc in auth catalog gateway reservation search; do
     docker tag library-system/$svc:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/library-system/$svc:latest
     docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/library-system/$svc:latest
   done
   ```

   **Step 6: Deploy to EKS**
   ```bash
   kubectl apply -k deploy/k8s/overlays/production
   kubectl get pods -n library -w
   kubectl get pods -n data -w  # Meilisearch only
   ```

3. Verification checklist:
   - `kubectl get pods -A` — all pods Running/Ready.
   - `kubectl get ingress -n library` — ALB DNS name assigned (takes 2-3 minutes).
   - `curl http://<alb-dns-name>/healthz` — gateway responds.
   - `kubectl logs -n library deployment/catalog` — clean startup, connected to RDS, Kafka consumer joined.
   - Create a book via the API, verify it appears in search (proves full flow: gateway → catalog → MSK → search → Meilisearch).

4. Troubleshooting table:

   | Symptom | Cause | Fix |
   |---------|-------|-----|
   | `ImagePullBackOff` | ECR permissions or wrong image URI | Check node IAM role has `AmazonEC2ContainerRegistryReadOnly`, verify URI matches ECR repo name |
   | `CrashLoopBackOff` | RDS/MSK unreachable | Check security groups allow traffic from EKS node SG, verify endpoints in overlay |
   | Ingress has no `ADDRESS` | ALB controller not ready | `kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-load-balancer-controller`, check subnet tags |
   | Pods stuck `Pending` | Insufficient node capacity | `kubectl describe pod <name>` for events, check node group scaling limits |
   | RDS connection refused | Security group misconfigured | Verify RDS SG allows inbound 5432 from the EKS node SG, not the cluster SG |
   | MSK connection timeout | Wrong bootstrap string | Check `terraform output msk_bootstrap_brokers`, verify ConfigMap patches match |

5. Teardown:
   ```bash
   kubectl delete -k deploy/k8s/overlays/production
   cd terraform
   terraform destroy
   ```
   Verify: `aws eks list-clusters`, `aws rds describe-db-instances` — both empty. Check for orphaned EBS volumes (`aws ec2 describe-volumes --filters Name=status,Values=available`).

6. For non-deployers: screenshot-style text summaries of expected output at each verification step.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/deploying.md
git commit -m "docs: add deploying and verifying section"
```

---

### Task 11: Write Section 12.11 — ArgoCD

**Files:**
- Create: `docs/src/ch12/argocd.md`

- [ ] **Step 1: Create the ArgoCD discussion**

Write `docs/src/ch12/argocd.md` (~200-250 lines) covering:

1. Opening: Section 12.9 deployed to EKS by running `kubectl apply` from a GitHub Actions workflow. This works, but it has a fundamental property: the CI pipeline *pushes* changes into the cluster. The cluster has no opinion about what should be running — it receives whatever the pipeline sends. GitOps inverts this relationship.

2. What is GitOps:
   - The cluster's desired state is defined in a Git repository (which we already have — `deploy/k8s/overlays/production/`).
   - A controller running *inside* the cluster continuously watches that repository.
   - When a commit changes the manifests, the controller detects the diff and applies it.
   - The cluster *pulls* its own state from Git, rather than having state pushed into it.

3. ArgoCD overview:
   - Open-source GitOps controller for Kubernetes. Installs as a set of controllers and a web UI.
   - Core CRDs: `Application` (maps a Git repo path to a cluster namespace) and `AppProject` (groups applications with access controls).
   - Web UI: shows a live view of every Kubernetes resource managed by the Application, highlights out-of-sync resources, provides one-click sync and rollback.

4. How ArgoCD would replace section 12.9 (architecture diagram):
   ```
   Developer → git push → GitHub
     → CI (GitHub Actions): test, build, push to ECR
     → CI commits new image tag to deploy/k8s/overlays/production/
     → ArgoCD detects commit, diffs against cluster state
     → ArgoCD applies changes to EKS
   ```
   Key change: the CI pipeline no longer runs `kubectl apply`. It only updates the image tag in the overlay (via a commit) and ArgoCD handles the rest.

5. Pros vs direct `kubectl apply`:
   - **Drift detection:** if someone manually `kubectl edit`s a Deployment, ArgoCD detects the drift and can auto-revert it. With `kubectl apply` from CI, manual changes persist silently.
   - **Audit trail:** every deployment is a Git commit with author, timestamp, and diff. `kubectl apply` from CI logs exist but are spread across workflow runs.
   - **Multi-environment:** ArgoCD can watch multiple overlays (dev, staging, production) as separate Applications. Promotion is a PR from one overlay directory to another.
   - **Rollback:** `git revert` the tag-update commit. ArgoCD detects the revert and rolls back the cluster. With `kubectl apply`, rollback requires re-running a pipeline or manual `kubectl rollout undo`.

6. Cons:
   - **Additional component:** ArgoCD itself runs in the cluster (multiple pods, CRDs, RBAC). It needs to be installed, configured, upgraded, and monitored.
   - **Learning curve:** Application CRDs, sync policies (auto-sync, self-heal, prune), SSO configuration for the UI, multi-cluster setup.
   - **Chicken-and-egg:** Who deploys ArgoCD? Typically Terraform or a bootstrap script, before ArgoCD can manage itself (the "App of Apps" pattern).
   - **Overkill for small teams:** For a single-environment, single-team project, the `kubectl apply` approach from 12.9 is simpler and sufficient.

7. When to adopt:
   - Team size > 2-3 developers deploying independently.
   - Multiple environments (dev/staging/production) with promotion workflows.
   - Compliance requirements for deployment audit trails.
   - Desire for a deployment UI and visibility beyond `kubectl get pods`.

8. Closing: ArgoCD is not implemented in this chapter, but the foundation is already in place. The Kustomize overlay structure and Git-tracked manifests are exactly what ArgoCD watches. Adopting GitOps later would mean installing ArgoCD and pointing it at `deploy/k8s/overlays/production/` — no manifest restructuring needed.

- [ ] **Step 2: Commit**

```bash
git add docs/src/ch12/argocd.md
git commit -m "docs: add ArgoCD GitOps alternative section"
```

---

### Task 12: Update SUMMARY.md

**Files:**
- Modify: `docs/src/SUMMARY.md`

- [ ] **Step 1: Add Chapter 12 entries**

Add the following lines after the Chapter 11 entries in `docs/src/SUMMARY.md`:

```markdown
- [Chapter 12: Cloud Deployment](./ch12/index.md)
  - [12.2 Terraform Fundamentals](./ch12/terraform-fundamentals.md)
  - [12.3 Networking: VPC and Subnets](./ch12/networking.md)
  - [12.4 Container Registry: ECR](./ch12/ecr.md)
  - [12.5 Database: RDS for PostgreSQL](./ch12/rds.md)
  - [12.6 Message Broker: Amazon MSK](./ch12/msk.md)
  - [12.7 Kubernetes Cluster: EKS](./ch12/eks.md)
  - [12.8 Production Kustomize Overlay](./ch12/production-overlay.md)
  - [12.9 CI/CD Pipeline](./ch12/cicd.md)
  - [12.10 Deploying and Verifying](./ch12/deploying.md)
  - [12.11 GitOps Alternative: ArgoCD](./ch12/argocd.md)
```

Note: the chapter index covers section 12.1 (From Local to Cloud) directly. Sidebar entries start at 12.2 to match the spec's section numbering.

- [ ] **Step 2: Commit**

```bash
git add docs/src/SUMMARY.md
git commit -m "docs: add Chapter 12 to SUMMARY.md"
```

---

### Task 13: Create Terraform provider and variables

**Files:**
- Create: `terraform/main.tf`
- Create: `terraform/variables.tf`
- Create: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/main.tf`**

```hcl
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.40"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.27"
    }
  }

  # Uncomment for team use with remote state:
  # backend "s3" {
  #   bucket         = "library-system-terraform-state"
  #   key            = "infrastructure/terraform.tfstate"
  #   region         = "us-east-1"
  #   dynamodb_table = "terraform-state-lock"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = var.project_name
      ManagedBy   = "terraform"
      Environment = "production"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {
  state = "available"
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
    }
  }
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
  }
}
```

- [ ] **Step 2: Create `terraform/variables.tf`**

```hcl
variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used as prefix for all resources"
  type        = string
  default     = "library-system"
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "library-system"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "github_repo" {
  description = "GitHub repository in owner/repo format for OIDC federation"
  type        = string
}
```

- [ ] **Step 3: Create initial `terraform/outputs.tf`**

Create an empty outputs file with a header comment. Outputs will be added by subsequent tasks as resources are created.

```hcl
# Outputs are populated by individual resource files (ecr.tf, rds.tf, msk.tf, eks.tf).
# Run `terraform output` after apply to see all values.
```

- [ ] **Step 4: Verify Terraform syntax**

Run: `cd terraform && terraform fmt -check && cd ..`
Expected: no output (files are already formatted).

Note: `terraform init` is NOT expected to succeed yet because `module.eks` is referenced in providers but `eks.tf` doesn't exist. This is fine — init will work once all files are in place.

- [ ] **Step 5: Commit**

```bash
git add terraform/main.tf terraform/variables.tf terraform/outputs.tf
git commit -m "feat: add Terraform provider config and variables"
```

---

### Task 14: Create VPC Terraform configuration

**Files:**
- Create: `terraform/vpc.tf`

- [ ] **Step 1: Create `terraform/vpc.tf`**

```hcl
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.5"

  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  azs             = slice(data.aws_availability_zones.available.names, 0, 2)
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb"                    = 1
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"           = 1
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# Security group for RDS — allows inbound PostgreSQL from EKS nodes only
resource "aws_security_group" "rds" {
  name_prefix = "${var.project_name}-rds-"
  description = "Allow PostgreSQL access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  tags = { Name = "${var.project_name}-rds" }
}

resource "aws_security_group_rule" "rds_ingress" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  security_group_id        = aws_security_group.rds.id
  source_security_group_id = module.eks.node_security_group_id
  description              = "PostgreSQL from EKS nodes"
}

# Security group for MSK — allows inbound Kafka from EKS nodes only
resource "aws_security_group" "msk" {
  name_prefix = "${var.project_name}-msk-"
  description = "Allow Kafka access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  tags = { Name = "${var.project_name}-msk" }
}

resource "aws_security_group_rule" "msk_ingress_plaintext" {
  type                     = "ingress"
  from_port                = 9092
  to_port                  = 9092
  protocol                 = "tcp"
  security_group_id        = aws_security_group.msk.id
  source_security_group_id = module.eks.node_security_group_id
  description              = "Kafka plaintext from EKS nodes"
}
```

Note: The security group rules reference `module.eks.node_security_group_id` which doesn't exist yet. Terraform handles this via dependency resolution — it will create the EKS cluster (and its node SG) before creating these rules. This is why `terraform plan` works even with forward references.

- [ ] **Step 2: Run `terraform fmt`**

Run: `cd terraform && terraform fmt vpc.tf && cd ..`

- [ ] **Step 3: Commit**

```bash
git add terraform/vpc.tf
git commit -m "feat: add VPC and security group Terraform config"
```

---

### Task 15: Create ECR Terraform configuration

**Files:**
- Create: `terraform/ecr.tf`
- Modify: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/ecr.tf`**

```hcl
locals {
  services = ["auth", "catalog", "gateway", "reservation", "search"]
}

resource "aws_ecr_repository" "services" {
  for_each = toset(local.services)

  name                 = "library-system/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each = aws_ecr_repository.services

  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images after 14 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 14
        }
        action = {
          type = "expire"
        }
      },
      {
        rulePriority = 2
        description  = "Keep only last 20 tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["sha-"]
          countType     = "imageCountMoreThan"
          countNumber   = 20
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
```

- [ ] **Step 2: Add ECR outputs to `terraform/outputs.tf`**

Append to `terraform/outputs.tf`:

```hcl
output "ecr_repository_urls" {
  description = "ECR repository URLs per service"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}
```

- [ ] **Step 3: Run `terraform fmt`**

Run: `cd terraform && terraform fmt ecr.tf outputs.tf && cd ..`

- [ ] **Step 4: Commit**

```bash
git add terraform/ecr.tf terraform/outputs.tf
git commit -m "feat: add ECR repository Terraform config"
```

---

### Task 16: Create RDS Terraform configuration

**Files:**
- Modify: `terraform/rds.tf`
- Modify: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/rds.tf`**

```hcl
locals {
  databases = {
    catalog     = "catalog"
    auth        = "auth"
    reservation = "reservation"
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db"
  subnet_ids = module.vpc.private_subnets

  tags = { Name = "${var.project_name}-db-subnet-group" }
}

resource "aws_db_instance" "databases" {
  for_each = local.databases

  identifier = "${var.project_name}-${each.key}"

  engine         = "postgres"
  engine_version = "16.4"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  db_name  = each.value
  username = "postgres"

  manage_master_user_password = true

  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name

  skip_final_snapshot     = true # Learning project only — NEVER in production
  backup_retention_period = 0    # Disable backups to reduce cost — production default is 7

  tags = { Name = "${var.project_name}-${each.key}" }
}
```

- [ ] **Step 2: Add RDS outputs to `terraform/outputs.tf`**

Append to `terraform/outputs.tf`:

```hcl
output "rds_endpoints" {
  description = "RDS instance endpoints per service"
  value       = { for k, v in aws_db_instance.databases : k => v.endpoint }
}

output "rds_master_password_secret_arns" {
  description = "Secrets Manager ARNs for RDS master passwords"
  value       = { for k, v in aws_db_instance.databases : k => v.master_user_secret[0].secret_arn }
}
```

- [ ] **Step 3: Run `terraform fmt`**

Run: `cd terraform && terraform fmt rds.tf outputs.tf && cd ..`

- [ ] **Step 4: Commit**

```bash
git add terraform/rds.tf terraform/outputs.tf
git commit -m "feat: add RDS PostgreSQL Terraform config"
```

---

### Task 17: Create MSK Terraform configuration

**Files:**
- Create: `terraform/msk.tf`
- Modify: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/msk.tf`**

```hcl
resource "aws_msk_configuration" "main" {
  name              = "${var.project_name}-kafka-config"
  kafka_versions    = ["3.6.0"]

  server_properties = <<-EOT
    auto.create.topics.enable=true
    default.replication.factor=2
    min.insync.replicas=1
    num.partitions=3
    log.retention.hours=168
  EOT
}

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
      client_broker = "PLAINTEXT"
      # Chapter 13 changes this to "TLS" and updates service configs to use port 9094
    }
  }
}
```

- [ ] **Step 2: Add MSK outputs to `terraform/outputs.tf`**

Append to `terraform/outputs.tf`:

```hcl
output "msk_bootstrap_brokers" {
  description = "MSK bootstrap broker string (plaintext)"
  value       = aws_msk_cluster.main.bootstrap_brokers
}
```

- [ ] **Step 3: Run `terraform fmt`**

Run: `cd terraform && terraform fmt msk.tf outputs.tf && cd ..`

- [ ] **Step 4: Commit**

```bash
git add terraform/msk.tf terraform/outputs.tf
git commit -m "feat: add MSK Kafka Terraform config"
```

---

### Task 18: Create EKS Terraform configuration

**Files:**
- Create: `terraform/eks.tf`
- Modify: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/eks.tf`**

```hcl
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.8"

  cluster_name    = var.cluster_name
  cluster_version = "1.29"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access  = true
  cluster_endpoint_private_access = true

  # EKS access entries (modern API, replaces aws-auth ConfigMap)
  enable_cluster_creator_admin_permissions = true

  cluster_addons = {
    coredns = {
      most_recent = true
    }
    kube-proxy = {
      most_recent = true
    }
    vpc-cni = {
      most_recent = true
    }
    aws-ebs-csi-driver = {
      most_recent              = true
      service_account_role_arn = module.ebs_csi_irsa.iam_role_arn
    }
  }

  eks_managed_node_groups = {
    default = {
      instance_types = ["t3.medium"]
      min_size       = 1
      max_size       = 3
      desired_size   = 2
    }
  }
}

# IRSA role for EBS CSI driver
module "ebs_csi_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.34"

  role_name             = "${var.project_name}-ebs-csi"
  attach_ebs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }
}

# IRSA role for AWS Load Balancer Controller
module "lb_controller_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.34"

  role_name                              = "${var.project_name}-lb-controller"
  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }
}

# AWS Load Balancer Controller via Helm
resource "helm_release" "lb_controller" {
  name       = "aws-load-balancer-controller"
  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  namespace  = "kube-system"
  version    = "1.7.1"

  set {
    name  = "clusterName"
    value = module.eks.cluster_name
  }

  set {
    name  = "serviceAccount.create"
    value = "true"
  }

  set {
    name  = "serviceAccount.name"
    value = "aws-load-balancer-controller"
  }

  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.lb_controller_irsa.iam_role_arn
  }

  set {
    name  = "vpcId"
    value = module.vpc.vpc_id
  }

  depends_on = [module.eks]
}
```

- [ ] **Step 2: Add EKS outputs to `terraform/outputs.tf`**

Append to `terraform/outputs.tf`:

```hcl
output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS cluster API endpoint"
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority" {
  description = "EKS cluster CA certificate (base64)"
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "oidc_provider_arn" {
  description = "EKS OIDC provider ARN (for IRSA)"
  value       = module.eks.oidc_provider_arn
}
```

- [ ] **Step 3: Run `terraform fmt`**

Run: `cd terraform && terraform fmt eks.tf outputs.tf && cd ..`

- [ ] **Step 4: Commit**

```bash
git add terraform/eks.tf terraform/outputs.tf
git commit -m "feat: add EKS cluster and LB controller Terraform config"
```

---

### Task 19: Create CI/CD Terraform configuration

**Files:**
- Create: `terraform/cicd.tf`
- Modify: `terraform/outputs.tf`

- [ ] **Step 1: Create `terraform/cicd.tf`**

```hcl
# GitHub Actions OIDC provider
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

# IAM role for GitHub Actions
resource "aws_iam_role" "github_actions" {
  name = "${var.project_name}-github-actions"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = aws_iam_openid_connect_provider.github.arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
          }
          StringLike = {
            "token.actions.githubusercontent.com:sub" = "repo:${var.github_repo}:ref:refs/heads/main"
          }
        }
      }
    ]
  })
}

# ECR push permissions
resource "aws_iam_role_policy" "github_actions_ecr" {
  name = "ecr-push"
  role = aws_iam_role.github_actions.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:PutImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload"
        ]
        Resource = [for repo in aws_ecr_repository.services : repo.arn]
      }
    ]
  })
}

# EKS access for GitHub Actions deployer
resource "aws_iam_role_policy" "github_actions_eks" {
  name = "eks-access"
  role = aws_iam_role.github_actions.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "eks:DescribeCluster"
        ]
        Resource = module.eks.cluster_arn
      }
    ]
  })
}

# EKS access entry for GitHub Actions role
resource "aws_eks_access_entry" "github_actions" {
  cluster_name  = module.eks.cluster_name
  principal_arn = aws_iam_role.github_actions.arn
  type          = "STANDARD"
}

resource "aws_eks_access_policy_association" "github_actions" {
  cluster_name  = module.eks.cluster_name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = aws_iam_role.github_actions.arn

  access_scope {
    type = "cluster"
  }
}
```

- [ ] **Step 2: Add CI/CD outputs to `terraform/outputs.tf`**

Append to `terraform/outputs.tf`:

```hcl
output "github_actions_role_arn" {
  description = "IAM role ARN for GitHub Actions OIDC federation"
  value       = aws_iam_role.github_actions.arn
}
```

- [ ] **Step 3: Run `terraform fmt`**

Run: `cd terraform && terraform fmt cicd.tf outputs.tf && cd ..`

- [ ] **Step 4: Verify all Terraform files parse**

Run: `cd terraform && terraform fmt -check -recursive && cd ..`
Expected: no output (all files formatted correctly).

- [ ] **Step 5: Commit**

```bash
git add terraform/cicd.tf terraform/outputs.tf
git commit -m "feat: add GitHub Actions OIDC and IAM Terraform config"
```

---

### Task 20: Restructure base K8s manifests

Move local-only infrastructure (Postgres StatefulSets/Services/ConfigMaps, Kafka resources) to `deploy/k8s/base/local-infra/` so the production overlay can exclude them.

**Files:**
- Create: `deploy/k8s/base/local-infra/kustomization.yaml`
- Create: `deploy/k8s/base/local-infra/data/kustomization.yaml`
- Create: `deploy/k8s/base/local-infra/messaging/kustomization.yaml`
- Move: `deploy/k8s/base/data/postgres-*` files → `deploy/k8s/base/local-infra/data/`
- Move: `deploy/k8s/base/messaging/kafka-*` files → `deploy/k8s/base/local-infra/messaging/`
- Modify: `deploy/k8s/base/data/kustomization.yaml` — remove postgres entries
- Modify: `deploy/k8s/base/messaging/kustomization.yaml` — remove kafka entries
- Modify: `deploy/k8s/overlays/local/kustomization.yaml` — add `../../base/local-infra` to resources

- [ ] **Step 1: Create the local-infra directory structure**

Create `deploy/k8s/base/local-infra/data/` and `deploy/k8s/base/local-infra/messaging/` directories.

- [ ] **Step 2: Move Postgres files to local-infra/data/**

```bash
git mv deploy/k8s/base/data/postgres-catalog-configmap.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-catalog-statefulset.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-catalog-service.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-auth-configmap.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-auth-statefulset.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-auth-service.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-reservation-configmap.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-reservation-statefulset.yaml deploy/k8s/base/local-infra/data/
git mv deploy/k8s/base/data/postgres-reservation-service.yaml deploy/k8s/base/local-infra/data/
```

- [ ] **Step 3: Move Kafka files to local-infra/messaging/**

```bash
git mv deploy/k8s/base/messaging/kafka-statefulset.yaml deploy/k8s/base/local-infra/messaging/
git mv deploy/k8s/base/messaging/kafka-service.yaml deploy/k8s/base/local-infra/messaging/
git mv deploy/k8s/base/messaging/kafka-configmap.yaml deploy/k8s/base/local-infra/messaging/
```

- [ ] **Step 4: Create `deploy/k8s/base/local-infra/data/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - secrets.yaml
  - postgres-catalog-configmap.yaml
  - postgres-catalog-statefulset.yaml
  - postgres-catalog-service.yaml
  - postgres-auth-configmap.yaml
  - postgres-auth-statefulset.yaml
  - postgres-auth-service.yaml
  - postgres-reservation-configmap.yaml
  - postgres-reservation-statefulset.yaml
  - postgres-reservation-service.yaml
```

- [ ] **Step 5: Create `deploy/k8s/base/local-infra/messaging/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: messaging

resources:
  - kafka-statefulset.yaml
  - kafka-service.yaml
  - kafka-configmap.yaml
```

Note: The `namespace: messaging` directive matches the original `deploy/k8s/base/messaging/kustomization.yaml` pattern.

- [ ] **Step 6: Create `deploy/k8s/base/local-infra/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - data
  - messaging
```

- [ ] **Step 7: Move `deploy/k8s/base/data/secrets.yaml` to local-infra and create a production-safe replacement**

The existing `secrets.yaml` contains empty placeholder Secrets for all 3 Postgres instances and Meilisearch in the `data` namespace. In production, Postgres runs on RDS (no `data`-namespace Secrets needed), and Meilisearch's `MEILI_MASTER_KEY` is read by the search service from a `library`-namespace Secret (not `data`). Move the full file to `local-infra/data/` and remove `secrets.yaml` from the shared base entirely:

```bash
git mv deploy/k8s/base/data/secrets.yaml deploy/k8s/base/local-infra/data/secrets.yaml
```

Then update `deploy/k8s/base/local-infra/data/kustomization.yaml` to include it (add `- secrets.yaml` to the resources list).

- [ ] **Step 8: Update `deploy/k8s/base/data/kustomization.yaml`**

Remove all postgres entries and secrets, keeping only Meilisearch and the namespace:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - namespace.yaml
  - meilisearch-configmap.yaml
  - meilisearch-statefulset.yaml
  - meilisearch-service.yaml
```

- [ ] **Step 9: Update `deploy/k8s/base/messaging/kustomization.yaml`**

Remove all kafka entries, keeping only the namespace:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: messaging

resources:
  - namespace.yaml
```

- [ ] **Step 10: Update `deploy/k8s/overlays/local/kustomization.yaml`**

Add `../../base/local-infra` to the resources list:

```yaml
resources:
  - ../../base
  - ../../base/local-infra
```

The rest of the file (secretGenerator, generatorOptions) remains unchanged.

- [ ] **Step 11: Verify local overlay still renders correctly**

Run: `kubectl kustomize deploy/k8s/overlays/local | grep -c 'kind:'`
Expected: a count of all resources (should match the count before restructuring — same resources, just organized differently).

To verify specific resources:
```bash
kubectl kustomize deploy/k8s/overlays/local | grep 'name: postgres-catalog$' | head -1
kubectl kustomize deploy/k8s/overlays/local | grep 'name: kafka$' | head -1
```
Expected: both should appear (local overlay includes local-infra).

- [ ] **Step 12: Verify production overlay excludes local-infra**

Run: `kubectl kustomize deploy/k8s/overlays/production | grep 'name: postgres-catalog$'`
Expected: no output (production overlay does NOT include local-infra).

Run: `kubectl kustomize deploy/k8s/overlays/production | grep 'name: kafka$'`
Expected: no output.

Run: `kubectl kustomize deploy/k8s/overlays/production | grep 'name: meilisearch$'`
Expected: output present (Meilisearch stays in production).

- [ ] **Step 13: Commit**

```bash
git add deploy/k8s/base/ deploy/k8s/overlays/local/kustomization.yaml
git commit -m "refactor: restructure K8s base to separate local-only infrastructure

Move Postgres StatefulSets/Services/ConfigMaps and Kafka resources to
deploy/k8s/base/local-infra/. Local overlay includes this directory;
production overlay does not. Meilisearch stays in the shared base."
```

---

### Task 21: Create production Kustomize overlay

**Files:**
- Modify: `deploy/k8s/overlays/production/kustomization.yaml`

- [ ] **Step 1: Write the complete production overlay**

Replace the stub in `deploy/k8s/overlays/production/kustomization.yaml` with:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base
  # NOTE: ../../base/local-infra is intentionally NOT included.
  # Production uses RDS for PostgreSQL and MSK for Kafka instead of
  # in-cluster StatefulSets. Meilisearch stays in the base (no managed
  # AWS equivalent).

# Image references — CI updates tags via `kustomize edit set image`
images:
  - name: library-system/auth
    newName: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/library-system/auth
    newTag: latest
  - name: library-system/catalog
    newName: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/library-system/catalog
    newTag: latest
  - name: library-system/gateway
    newName: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/library-system/gateway
    newTag: latest
  - name: library-system/reservation
    newName: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/library-system/reservation
    newTag: latest
  - name: library-system/search
    newName: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/library-system/search
    newTag: latest

patches:
  # --- Replica patches ---
  - target:
      kind: Deployment
      namespace: library
    patch: |
      - op: replace
        path: /spec/replicas
        value: 2

  # --- imagePullPolicy patches ---
  - target:
      kind: Deployment
      namespace: library
    patch: |
      - op: replace
        path: /spec/template/spec/containers/0/imagePullPolicy
        value: Always

  # --- Resource limit patches ---
  - target:
      kind: Deployment
      namespace: library
    patch: |
      - op: replace
        path: /spec/template/spec/containers/0/resources
        value:
          requests:
            cpu: "250m"
            memory: "256Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"

  # --- DATABASE_URL patches (RDS endpoints) ---
  # Replace StatefulSet DNS names with RDS endpoints.
  # Substitute ACCOUNT_ID, REGION, and RDS endpoints from `terraform output`.
  - target:
      kind: Deployment
      name: catalog
      namespace: library
    patch: |
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
                envFrom:
                  - configMapRef:
                      name: catalog-config
                env:
                  - name: POSTGRES_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: postgres-catalog-secret
                        key: POSTGRES_PASSWORD
                  - name: DATABASE_URL
                    value: "host=RDS_CATALOG_ENDPOINT port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=catalog sslmode=require"
                  - name: JWT_SECRET
                    valueFrom:
                      secretKeyRef:
                        name: jwt-secret
                        key: JWT_SECRET

  - target:
      kind: Deployment
      name: auth
      namespace: library
    patch: |
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: auth
        namespace: library
      spec:
        template:
          spec:
            containers:
              - name: auth
                envFrom:
                  - configMapRef:
                      name: auth-config
                env:
                  - name: POSTGRES_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: postgres-auth-secret
                        key: POSTGRES_PASSWORD
                  - name: DATABASE_URL
                    value: "host=RDS_AUTH_ENDPOINT port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=auth sslmode=require"
                  - name: JWT_SECRET
                    valueFrom:
                      secretKeyRef:
                        name: jwt-secret
                        key: JWT_SECRET

  - target:
      kind: Deployment
      name: reservation
      namespace: library
    patch: |
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: reservation
        namespace: library
      spec:
        template:
          spec:
            containers:
              - name: reservation
                envFrom:
                  - configMapRef:
                      name: reservation-config
                env:
                  - name: POSTGRES_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: postgres-reservation-secret
                        key: POSTGRES_PASSWORD
                  - name: DATABASE_URL
                    value: "host=RDS_RESERVATION_ENDPOINT port=5432 user=postgres password=$(POSTGRES_PASSWORD) dbname=reservation sslmode=require"
                  - name: JWT_SECRET
                    valueFrom:
                      secretKeyRef:
                        name: jwt-secret
                        key: JWT_SECRET

  # --- Ingress patch (NGINX → ALB) ---
  - target:
      kind: Ingress
      name: library-ingress
      namespace: library
    patch: |
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: library-ingress
        namespace: library
        annotations:
          alb.ingress.kubernetes.io/scheme: internet-facing
          alb.ingress.kubernetes.io/target-type: ip
          alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}]'
      spec:
        ingressClassName: alb
        rules:
          - http:
              paths:
                - path: /
                  pathType: Prefix
                  backend:
                    service:
                      name: gateway
                      port:
                        number: 8080

  # --- ConfigMap patches (Kafka brokers → MSK) ---
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
        KAFKA_BROKERS: "MSK_BOOTSTRAP_BROKERS"
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
        KAFKA_BROKERS: "MSK_BOOTSTRAP_BROKERS"
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
        KAFKA_BROKERS: "MSK_BOOTSTRAP_BROKERS"
        MEILI_URL: "http://meilisearch.data.svc.cluster.local:7700"
        CATALOG_GRPC_ADDR: "catalog.library.svc.cluster.local:50052"

# Secrets — placeholder values. Chapter 13 replaces these with
# external-secrets operator syncing from AWS Secrets Manager.
# Substitute real values from `terraform output` and
# `aws secretsmanager get-secret-value` before deploying.
secretGenerator:
  - name: jwt-secret
    namespace: library
    literals:
      - JWT_SECRET=REPLACE_WITH_PRODUCTION_SECRET
  - name: postgres-catalog-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
  - name: postgres-auth-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
  - name: postgres-reservation-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
  - name: meilisearch-secret
    namespace: library
    literals:
      - MEILI_MASTER_KEY=REPLACE_WITH_PRODUCTION_KEY

generatorOptions:
  disableNameSuffixHash: true
```

- [ ] **Step 2: Verify overlay renders**

Run: `kubectl kustomize deploy/k8s/overlays/production | head -50`
Expected: rendered YAML with ECR image references and ALB ingress annotations. No Postgres or Kafka StatefulSets.

- [ ] **Step 3: Commit**

```bash
git add deploy/k8s/overlays/production/kustomization.yaml
git commit -m "feat: complete production Kustomize overlay for EKS

Adds ECR image references, ALB ingress annotations, RDS endpoint
patches, MSK broker patches, production resource limits, 2 replicas,
and placeholder secrets for manual substitution."
```

---

### Task 22: Update CI/CD workflows

**Files:**
- Modify: `.github/workflows/main.yml`
- Modify: `.github/workflows/pr.yml`

- [ ] **Step 1: Update `.github/workflows/main.yml`**

Replace the existing content with the updated workflow that adds ECR push and EKS deploy:

```yaml
name: CI/CD
on:
  push:
    branches: [main]

permissions:
  contents: read
  packages: write
  id-token: write  # Required for OIDC federation with AWS

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci

  build-and-push:
    needs: ci
    runs-on: ubuntu-latest
    strategy:
      matrix:
        service: [auth, catalog, gateway, reservation, search]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: services/${{ matrix.service }}/Dockerfile
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:sha-${{ github.sha }}
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:latest

  deploy:
    needs: build-and-push
    runs-on: ubuntu-latest
    # Only deploy when all images are pushed successfully
    if: github.ref == 'refs/heads/main'
    environment: production
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_DEPLOY_ROLE_ARN }}
          aws-region: ${{ vars.AWS_REGION }}

      - name: Login to Amazon ECR
        id: ecr-login
        uses: aws-actions/amazon-ecr-login@v2

      - name: Pull from GHCR and push to ECR
        env:
          ECR_REGISTRY: ${{ steps.ecr-login.outputs.registry }}
          GHCR_REGISTRY: ghcr.io/${{ github.repository }}
        run: |
          # Pull images already built by the build-and-push job, re-tag for ECR
          echo ${{ secrets.GITHUB_TOKEN }} | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          for service in auth catalog gateway reservation search; do
            docker pull $GHCR_REGISTRY/$service:sha-${{ github.sha }}
            docker tag $GHCR_REGISTRY/$service:sha-${{ github.sha }} \
                       $ECR_REGISTRY/library-system/$service:sha-${{ github.sha }}
            docker tag $GHCR_REGISTRY/$service:sha-${{ github.sha }} \
                       $ECR_REGISTRY/library-system/$service:latest
            docker push $ECR_REGISTRY/library-system/$service:sha-${{ github.sha }}
            docker push $ECR_REGISTRY/library-system/$service:latest
          done

      - name: Update Kustomize image tags
        env:
          REGISTRY: ${{ steps.ecr-login.outputs.registry }}
        run: |
          cd deploy/k8s/overlays/production
          for service in auth catalog gateway reservation search; do
            kustomize edit set image \
              library-system/$service=$REGISTRY/library-system/$service:sha-${{ github.sha }}
          done

      - name: Deploy to EKS
        run: |
          aws eks update-kubeconfig --name ${{ vars.EKS_CLUSTER_NAME }} --region ${{ vars.AWS_REGION }}
          kubectl apply -k deploy/k8s/overlays/production

      - name: Wait for rollout
        run: |
          for service in auth catalog gateway reservation search; do
            kubectl rollout status deployment/$service -n library --timeout=300s
          done
```

- [ ] **Step 2: Update `.github/workflows/pr.yml`**

Add an optional Terraform plan step:

```yaml
name: PR Check
on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  pull-requests: write
  id-token: write  # Required for Terraform plan with OIDC

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci

  # Optional: uncomment to run terraform plan on PRs that modify infrastructure
  # terraform-plan:
  #   runs-on: ubuntu-latest
  #   if: contains(github.event.pull_request.changed_files, 'terraform/')
  #   steps:
  #     - uses: actions/checkout@v4
  #     - uses: aws-actions/configure-aws-credentials@v4
  #       with:
  #         role-to-assume: ${{ vars.AWS_DEPLOY_ROLE_ARN }}
  #         aws-region: ${{ vars.AWS_REGION }}
  #     - uses: hashicorp/setup-terraform@v3
  #     - name: Terraform Plan
  #       run: |
  #         cd terraform
  #         terraform init
  #         terraform plan -no-color
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/main.yml .github/workflows/pr.yml
git commit -m "feat: add EKS deploy job and ECR push to CI/CD pipeline

Adds OIDC federation, ECR login/push, kustomize image tag updates,
and kubectl apply to the main branch workflow. Adds optional terraform
plan step to PR workflow."
```

---

### Task 23: Add .gitignore for Terraform

**Files:**
- Modify: `.gitignore` (or create `terraform/.gitignore`)

- [ ] **Step 1: Create `terraform/.gitignore`**

```
# Terraform state and provider cache
.terraform/
*.tfstate
*.tfstate.backup
*.tfplan
.terraform.lock.hcl
```

- [ ] **Step 2: Commit**

```bash
git add terraform/.gitignore
git commit -m "chore: add Terraform .gitignore"
```

---

## Task Dependencies and Parallelization

**Parallel group 1 (documentation — all independent):**
Tasks 1-11 can run in parallel. Each creates an independent markdown file.

**Sequential group 2 (Terraform):**
Tasks 13-19 must run sequentially. Task 13 creates the base files (`main.tf`, `variables.tf`, `outputs.tf`); tasks 14-19 each append outputs to `outputs.tf`, which requires the previous task's appends to be present.

**Sequential group:**
- Task 20 (base restructuring) must run before Task 21 (production overlay) — the overlay depends on the restructured base.
- Task 21 must run before Task 22 (CI/CD) — the workflow references the overlay.
- Task 23 (gitignore) can run anytime.

**Recommended execution order:**
1. Tasks 1-12 in parallel (all documentation + SUMMARY.md)
2. Task 23 (gitignore — quick)
3. Tasks 13-19 sequentially (Terraform files — each appends to outputs.tf)
4. Task 20 (base restructuring)
5. Task 21 (production overlay)
6. Task 22 (CI/CD workflows)
