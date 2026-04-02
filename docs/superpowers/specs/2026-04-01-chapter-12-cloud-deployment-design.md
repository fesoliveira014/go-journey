# Chapter 12: Cloud Deployment (EKS) — Design Spec

## Goal

Take the reader from a local kind cluster (Chapter 11) to a production-grade AWS deployment. By the end, the reader has complete, runnable Terraform for EKS + RDS + MSK + ECR, a production Kustomize overlay, and a CI/CD pipeline that builds, pushes, and deploys to EKS. Running the infrastructure is optional — every resource is explained in detail so the reader can follow along without an AWS account.

## Context

### What Exists

- **Chapter 11 Kubernetes manifests** in `deploy/k8s/base/` — 3 namespaces (library, data, messaging), 5 application Deployments, 3 Postgres StatefulSets, Kafka StatefulSet, Meilisearch StatefulSet, NGINX Ingress, ConfigMaps, Secrets.
- **Kustomize structure** — base + local overlay (kind-specific). Production overlay is a stub (`deploy/k8s/overlays/production/kustomization.yaml`) with comments listing what Chapter 12 fills in.
- **CI/CD** — GitHub Actions (`main.yml`) runs `earthly +ci`, builds images, pushes to GHCR (`ghcr.io/<owner>/<repo>/<service>:sha-<commit>` and `:latest`). `pr.yml` runs `earthly +ci` only.
- **Docker images** — 5 services (`library-system/auth`, `library-system/catalog`, `library-system/reservation`, `library-system/search`, `library-system/gateway`), multi-stage builds (golang:1.26-alpine → alpine:3.19).
- **All services** read config from environment variables (`DATABASE_URL`, `GRPC_PORT`, `JWT_SECRET`, `KAFKA_BROKERS`, etc.) via ConfigMaps and Secrets.
- **No Terraform, no AWS config** — `terraform/` directory does not exist.

### What's Missing

1. AWS infrastructure: VPC, EKS cluster, RDS instances, MSK cluster, ECR repositories.
2. Terraform code to provision all infrastructure.
3. Production Kustomize overlay replacing local-only resources.
4. CI/CD pipeline updates to push to ECR and deploy to EKS.
5. AWS Load Balancer Controller for ALB-based Ingress.
6. OIDC federation for GitHub Actions → AWS authentication.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| IaC tool | Terraform | On the project's tech stack roadmap, broadly transferable skill, full control over all AWS resources in one tool |
| Container registry | Amazon ECR | AWS-native, EKS nodes authenticate via IAM (no image pull secrets), Terraform can provision repos |
| Postgres | Amazon RDS (3 instances) | Managed backups, patching, failover. Replaces 3 in-cluster StatefulSets |
| Kafka | Amazon MSK | Managed Kafka, same KRaft mode. Replaces in-cluster StatefulSet |
| Meilisearch | Stays as StatefulSet in EKS | No managed AWS equivalent. Runs in the `data` namespace with EBS PVC |
| EKS node type | Managed Node Groups | Standard approach, teaches transferable EC2/ASG concepts, no StatefulSet limitations |
| Ingress | AWS Load Balancer Controller + ALB | Replaces NGINX Ingress from Chapter 11, AWS-native, provisions ALBs from Ingress resources |
| CI/CD deployment | GitHub Actions with `kubectl apply` | Builds on existing Chapter 9 workflows, OIDC federation for auth, simple and direct |
| AWS authentication from CI | OIDC federation | No long-lived credentials, modern best practice, scoped to the repo |
| Terraform state | Local by default, S3 backend explained | Local state works for learning; S3+DynamoDB locking explained for teams |
| VPC module | `terraform-aws-modules/vpc/aws` | Standard community module, avoids 100+ lines of raw resource definitions |
| EKS module | `terraform-aws-modules/eks/aws` | Standard community module, handles IAM roles, OIDC provider, add-ons |
| Hands-on execution | Optional with cost warnings | Complete runnable code, but reader can follow along without an AWS account |

## Chapter Structure

### 12.1 — From Local to Cloud

Conceptual introduction. No code.

**Content:**
- Architecture diagram: target state — EKS cluster with 5 app Deployments, Meilisearch StatefulSet, managed RDS (3 instances) and MSK outside the cluster, ALB Ingress, ECR as image source.
- Map kind concepts to AWS equivalents: kind cluster → EKS, local Docker images → ECR, NGINX Ingress → ALB, in-cluster Postgres → RDS, in-cluster Kafka → MSK, `kubectl apply` → CI/CD pipeline.
- What changes vs what stays the same: base K8s manifests untouched, only overlay and infrastructure differ. This is the Kustomize payoff.
- Cost awareness: estimated hourly/monthly costs for each component (EKS control plane ~$0.10/hr, t3.medium nodes, RDS db.t3.micro, MSK kafka.t3.small). Emphasize `terraform destroy`.
- Prerequisites: AWS account, AWS CLI configured, Terraform installed.

**Length:** ~150-200 lines.

### 12.2 — Terraform Fundamentals

Brief Terraform primer.

**Content:**
- What Terraform is: declarative IaC, HCL syntax, state management, plan/apply/destroy lifecycle.
- Core concepts: providers, resources, data sources, variables, outputs, modules.
- State file: what it is, why it matters, S3 + DynamoDB remote backend for teams (shown but local state used for learning).
- Project structure:
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
  ```
- `terraform init`, `terraform plan`, `terraform apply`, `terraform destroy` walkthrough.

**Length:** ~200-250 lines.

### 12.3 — Networking: VPC and Subnets

First Terraform resources.

**Content:**
- Why EKS needs a VPC: nodes are EC2 instances, RDS and MSK also live in the VPC.
- VPC design: 1 VPC (`10.0.0.0/16`), 2 public subnets (ALB, NAT), 2 private subnets (nodes, RDS, MSK) across 2 AZs.
- Internet Gateway (public subnets), NAT Gateway (private subnet egress).
- `terraform-aws-modules/vpc/aws` module — explain what it creates, full `vpc.tf` with every parameter.
- Subnet tagging for EKS: `kubernetes.io/role/elb` (public), `kubernetes.io/role/internal-elb` (private).
- Security groups: RDS and MSK SGs allow inbound from EKS node SG only.

**Length:** ~200-250 lines.

### 12.4 — Container Registry: ECR

Short section.

**Content:**
- One ECR repository per service (5 total).
- `ecr.tf` with `aws_ecr_repository` resources + lifecycle policy (expire untagged images after 14 days).
- Image tagging strategy: `<account-id>.dkr.ecr.<region>.amazonaws.com/library-system/<service>:sha-<commit>` and `:latest`.
- IAM: EKS nodes pull from ECR via node IAM role (no image pull secrets).
- `outputs.tf` entries for ECR repository URLs.

**Length:** ~100-150 lines.

### 12.5 — Database: RDS for PostgreSQL

Replaces 3 in-cluster Postgres StatefulSets.

**Content:**
- Why managed Postgres: automated backups, patching, failover, monitoring.
- 3 RDS instances (catalog, auth, reservation) — one per service, matching existing isolation.
- Instance config: `db.t3.micro`, PostgreSQL engine, 20GB gp3 storage.
- `rds.tf` with `aws_db_subnet_group` + 3x `aws_db_instance`.
- Security group: inbound 5432 from EKS node SG only.
- Credentials: `manage_master_user_password = true` (AWS Secrets Manager manages rotation). Manual retrieval for now; Chapter 13 wires in external-secrets operator.
- `skip_final_snapshot = true` for learning (with warning about production).
- `outputs.tf` for RDS endpoints — replace StatefulSet DNS names in overlay.
- Key difference: `DATABASE_URL` points to `<instance>.xxx.<region>.rds.amazonaws.com`.

**Length:** ~200-250 lines.

### 12.6 — Message Broker: Amazon MSK

Replaces in-cluster Kafka StatefulSet.

**Content:**
- Why managed Kafka: broker patching, storage scaling, KRaft management handled by AWS.
- MSK config: `kafka.t3.small`, 2 brokers across 2 AZs, 10GB EBS per broker.
- `msk.tf` with `aws_msk_cluster`. Plaintext listener for local parity; TLS noted as a production hardening step for Chapter 13.
- Security group: inbound 9092 from EKS node SG.
- `aws_msk_configuration` for `auto.create.topics.enable`.
- `outputs.tf` for bootstrap broker strings — replace `kafka-0.kafka.messaging.svc.cluster.local:9092` in overlay.

**Length:** ~150-200 lines.

### 12.7 — Kubernetes Cluster: EKS

Core infrastructure section.

**Content:**
- EKS architecture: AWS-managed control plane, self-managed worker nodes via managed node groups.
- `eks.tf` using `terraform-aws-modules/eks/aws` module.
- Cluster config: K8s 1.29+, private + public API endpoints, add-ons (coredns, kube-proxy, vpc-cni, aws-ebs-csi-driver).
- Managed node group: `t3.medium`, desired 2 / min 1 / max 3, IAM policies (`AmazonEKSWorkerNodePolicy`, `AmazonEKS_CNI_Policy`, `AmazonEC2ContainerRegistryReadOnly`).
- AWS Load Balancer Controller: installed via Helm provider or manual `kubectl apply`. IRSA (IAM Roles for Service Accounts) for the controller — explain IRSA as the AWS pattern for pod-level AWS permissions.
- `outputs.tf`: cluster name, endpoint, CA, OIDC provider ARN.
- `kubeconfig` setup: `aws eks update-kubeconfig`.

**Length:** ~300-350 lines.

### 12.8 — Production Kustomize Overlay

Fills in the Chapter 11 stub.

**Content:**
- Complete `deploy/k8s/overlays/production/kustomization.yaml`:
  - `images` transformer: `library-system/<service>:latest` → ECR URIs with tags.
  - Resource limit patches: 250m-500m CPU, 256Mi-512Mi memory.
  - Replica patches: 2 replicas per app Deployment.
  - ConfigMap patches: `DATABASE_URL` → RDS endpoints, `KAFKA_BROKERS` → MSK brokers, `OTEL_COLLECTOR_ENDPOINT` → empty.
  - `imagePullPolicy: Always`.
- Removing local-only resources: Postgres StatefulSets, Kafka StatefulSet, their Services/ConfigMaps from data and messaging namespaces. Discussion of approaches: Kustomize delete patches vs restructuring base.
- What stays: Meilisearch StatefulSet, all app Deployments/Services, namespaces.
- Ingress annotations for ALB: `alb.ingress.kubernetes.io/scheme: internet-facing`, `alb.ingress.kubernetes.io/target-type: ip`, `ingressClassName: alb`.
- Secrets: `secretGenerator` with placeholder values + comment that Chapter 13 replaces with external-secrets. Reader manually substitutes real values.

**Length:** ~300-350 lines.

### 12.9 — CI/CD Pipeline: GitHub Actions to EKS

Extends existing workflows.

**Content:**
- OIDC federation: Terraform resource for IAM OIDC identity provider + IAM role with trust policy scoped to the repo. Why this is better than stored AWS credentials.
- Updated `main.yml`:
  1. `earthly +ci` (unchanged).
  2. `aws-actions/configure-aws-credentials` with OIDC.
  3. `aws-actions/amazon-ecr-login`.
  4. Build + push to ECR (tag: `sha-${{ github.sha }}` and `latest`).
  5. `kustomize edit set image` in production overlay.
  6. `kubectl apply -k deploy/k8s/overlays/production`.
  7. `kubectl rollout status` per service.
- IAM role permissions: ECR push, EKS access via `aws-auth` ConfigMap or EKS access entries.
- `pr.yml`: optional `terraform plan` step that comments on the PR.
- Rollback: `kubectl rollout undo`, previous images still in ECR.

**Length:** ~250-300 lines.

### 12.10 — Deploying and Verifying

End-to-end walkthrough.

**Content:**
- Step-by-step (for readers who run it):
  1. `terraform init` → `plan` → `apply` (~15-20 min).
  2. `aws eks update-kubeconfig`.
  3. Verify infrastructure: RDS available, MSK active, ECR repos exist.
  4. Push images to ECR (manual first time).
  5. `kubectl apply -k deploy/k8s/overlays/production`.
  6. `kubectl get pods` — all Running/Ready.
- Verification:
  - `kubectl get ingress -n library` — ALB provisioned.
  - `curl http://<alb-dns-name>/healthz` — gateway responds.
  - Create a book, verify search (full event flow through MSK).
- Troubleshooting table:
  - `ImagePullBackOff` → ECR permissions or wrong URI.
  - `CrashLoopBackOff` → RDS/MSK unreachable (security groups).
  - Ingress no address → ALB controller, missing annotations/tags.
  - Pods pending → node capacity, check scaling.
- Cost reminder + teardown: `terraform destroy`, verify all deleted, check for orphans.
- For non-deployers: summary of expected output at each step.

**Length:** ~200-250 lines.

### 12.11 — GitOps Alternative: ArgoCD

Discussion section. No implementation.

**Content:**
- What is GitOps: desired state in Git, controller reconciles.
- ArgoCD overview: installs in cluster, watches repo, auto-syncs, web UI for visualization.
- How it replaces 12.9's approach: CI stops after ECR push, ArgoCD watches overlay, image tag updates via Git commit.
- Pros vs direct `kubectl apply`: drift detection, audit trail, multi-environment management, rollback is `git revert`.
- Cons: additional component, learning curve (CRDs: Application, AppProject), chicken-and-egg deployment, overkill for single-team projects.
- Architecture diagram: Developer → Git → CI → ECR + tag commit → ArgoCD → EKS.
- When to adopt: team size > 2-3, multiple environments, compliance requirements.

**Length:** ~200-250 lines.

## Non-Goals

- **DNS and TLS** — Route 53, cert-manager, ACM. Deferred to Chapter 13.
- **Secrets management** — External-secrets operator, AWS Secrets Manager integration. Deferred to Chapter 13. Production overlay uses placeholder secretGenerator values with comments.
- **Observability in EKS** — OTel Collector DaemonSet, CloudWatch Container Insights. `OTEL_COLLECTOR_ENDPOINT` left empty in production overlay, same as Chapter 11.
- **Multi-region or multi-cluster** — Single region, single cluster.
- **Spot instances or Graviton** — Cost optimization strategies mentioned briefly but not implemented.
- **Network policies** — Mentioned in Chapter 11 as deferred, remains deferred.
- **ArgoCD implementation** — Discussed conceptually in 12.11, not deployed.
- **Terraform modules abstraction** — Resources kept in flat files for readability. No custom module wrapping.

## Dependencies

- **Chapter 11** — Kubernetes manifests, Kustomize base/overlay structure, all service health checks and graceful shutdown.
- **Chapter 9** — GitHub Actions CI/CD workflows, Earthly build targets.
- **Chapter 3** — Dockerfiles and multi-stage builds.

## New Files Summary

| Path | Type | Description |
|------|------|-------------|
| `docs/src/ch12/index.md` | Documentation | Local to cloud introduction |
| `docs/src/ch12/terraform-fundamentals.md` | Documentation | Terraform primer |
| `docs/src/ch12/networking.md` | Documentation | VPC and subnets |
| `docs/src/ch12/ecr.md` | Documentation | Container registry |
| `docs/src/ch12/rds.md` | Documentation | RDS for PostgreSQL |
| `docs/src/ch12/msk.md` | Documentation | Amazon MSK |
| `docs/src/ch12/eks.md` | Documentation | EKS cluster |
| `docs/src/ch12/production-overlay.md` | Documentation | Production Kustomize overlay |
| `docs/src/ch12/cicd.md` | Documentation | CI/CD pipeline updates |
| `docs/src/ch12/deploying.md` | Documentation | Deploy and verify walkthrough |
| `docs/src/ch12/argocd.md` | Documentation | GitOps alternative discussion |
| `terraform/main.tf` | Terraform | Provider config, backend |
| `terraform/variables.tf` | Terraform | Input variables |
| `terraform/outputs.tf` | Terraform | Output values for Kustomize/CI |
| `terraform/vpc.tf` | Terraform | VPC, subnets, NAT, IGW |
| `terraform/ecr.tf` | Terraform | ECR repositories |
| `terraform/rds.tf` | Terraform | 3x RDS instances |
| `terraform/msk.tf` | Terraform | MSK cluster |
| `terraform/eks.tf` | Terraform | EKS cluster + node group + LB controller |

## Modified Files Summary

| Path | Change |
|------|--------|
| `deploy/k8s/overlays/production/kustomization.yaml` | Fill in stub with complete production overlay |
| `.github/workflows/main.yml` | Add ECR push + EKS deploy steps |
| `.github/workflows/pr.yml` | Optional: add `terraform plan` comment step |
| `docs/src/SUMMARY.md` | Add Chapter 12 entries |

## Chapter 13 Roadmap (Deferred)

Chapter 13 will cover production hardening:
- **DNS + TLS:** Route 53 domain, ACM certificate, cert-manager or ALB-native TLS termination.
- **Secrets management:** External-secrets operator syncing from AWS Secrets Manager to K8s Secrets. Replaces placeholder `secretGenerator` in production overlay.
- **MSK TLS:** Enabling in-transit encryption for Kafka client connections.
- Any other production hardening topics identified during Chapter 12 development.
