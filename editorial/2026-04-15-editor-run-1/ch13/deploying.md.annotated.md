# 13.9 Deploying and Verifying

<!-- [STRUCTURAL] Operational checklist style, appropriate for this section. Flow: provision → kubectl config → verify AWS → retrieve creds → push images → deploy → checklist → troubleshooting → teardown → summary. Well-ordered. Note: the "Push Images to ECR" section duplicates cicd.md's promotion step — if CI already pushed, this manual push is redundant. Decide whether this chapter is for "first manual apply" OR "CI-driven". -->
<!-- [COPY EDIT] Heading "13.9 Deploying and Verifying" vs index.md "13.9 — Deploying and Verifying". Unify dash style. -->

This section is the payoff for everything in Chapter 13. The Terraform modules are written, the production Kustomize overlay is configured, and the ECR repositories, RDS cluster, and MSK broker are defined in code. Now you actually run it.
<!-- [COPY EDIT] "RDS cluster" — rds.md provisions three `aws_db_instance` resources, NOT an RDS Aurora cluster (`aws_rds_cluster`). Similarly below "Multi-AZ RDS Aurora cluster" — terminology collision. Use "RDS instances". -->

**This costs real money.** The infrastructure spun up here — an EKS cluster with managed node groups, a Multi-AZ RDS Aurora cluster, an MSK broker, a NAT Gateway, and an Application Load Balancer — runs roughly $8–15 per day. Tear everything down with `terraform destroy` when you are finished. The teardown section at the end of this chapter shows the exact steps.
<!-- [COPY EDIT] "Multi-AZ RDS Aurora cluster" — the chapter provisions single-AZ db.t3.micro RDS instances via `aws_db_instance`, not Aurora. Fix. -->
<!-- [COPY EDIT] Cost estimate "$8–15/day" — index.md's monthly table sums to ~$295/month ≈ $9.75/day. OK — within range. -->

If you prefer not to deploy, the verification and troubleshooting sections describe the expected outputs at each step so you can follow along without incurring costs.

---

## Provision Infrastructure with Terraform
<!-- [COPY EDIT] Heading title case; sentence case elsewhere. Normalize. -->

Start in the `terraform/` directory.
<!-- [COPY EDIT] `terraform/` here; cicd.md says `infrastructure/`; networking.md says `infra/terraform`. Unify. -->

**Initialize the working directory:**

```bash
terraform init
```

This downloads the AWS, Kubernetes, and Helm providers defined in `versions.tf`, initializes the S3 backend for remote state, and sets up the module cache. Expected output ends with:
<!-- [COPY EDIT] "initializes the S3 backend for remote state" — terraform-fundamentals.md explicitly tells the reader to leave S3 backend commented out. Contradiction. -->
<!-- [COPY EDIT] `versions.tf` not shown elsewhere in chapter. terraform-fundamentals.md uses `main.tf` for the `terraform { required_providers {} }` block. Unify file names. -->

```
Terraform has been successfully initialized!
```

**Preview the plan:**

```bash
terraform plan -out=tfplan
```

Terraform will print a summary of every resource it intends to create. On first apply this is a long list — VPC, subnets, security groups, IAM roles, EKS cluster, node group, RDS cluster, MSK cluster, ECR repositories, and the ALB controller Helm release. Review the summary. If anything looks unexpected, stop here.
<!-- [LINE EDIT] "Review the summary. If anything looks unexpected, stop here." — good imperative. -->

**Apply:**

```bash
terraform apply tfplan
```

This takes approximately 15–20 minutes. The EKS control plane alone takes 10–12 minutes; RDS and MSK initialization add several more. The terminal will stream resource creation events as they complete. Expected final output:

```
Apply complete! Resources: 47 added, 0 changed, 0 destroyed.

Outputs:

ecr_repository_urls = {
  "auth"        = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/auth"
  "catalog"     = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/catalog"
  ...
}
cluster_name             = "library-system"
msk_bootstrap_brokers_tls = "b-1.xxxxx.kafka.us-east-1.amazonaws.com:9094,..."
rds_endpoints            = {
  "auth"        = "library-system-auth.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com:5432"
  "catalog"     = "library-system-catalog.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com:5432"
  "reservation" = "library-system-reservation.xxxxxxxxxxxx.us-east-1.rds.amazonaws.com:5432"
}
```
<!-- [COPY EDIT] RDS instance name prefix here `library-system-*`; rds.md uses `${var.project_name}-${each.key}` = `library-*`. Pick one prefix. -->
<!-- [COPY EDIT] "47 added" here vs. "23 added" in terraform-fundamentals.md. Pick one canonical count. -->

Save these output values — you will need them in the next steps. You can always retrieve them later with `terraform output`.

---

## Configure kubectl

Connect your local `kubectl` to the new EKS cluster:

```bash
aws eks update-kubeconfig \
  --region us-east-1 \
  --name library-production
```
<!-- [COPY EDIT] `library-production` here, `library-cluster` in index.md and cicd.md, `local.cluster_name` in eks.md. Pick one. -->

This writes a new context to `~/.kube/config` and sets it as the active context. Verify the connection:

```bash
kubectl cluster-info
```

Expected output:

```
Kubernetes control plane is running at https://XXXXXXXXXXXX.gr7.us-east-1.eks.amazonaws.com
CoreDNS is running at https://XXXXXXXXXXXX.gr7.us-east-1.eks.amazonaws.com/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
```

---

## Verify AWS Resources
<!-- [COPY EDIT] Title case. -->

Before deploying the application, confirm the three managed services came up correctly.

**ECR repositories:**

```bash
aws ecr describe-repositories \
  --query 'repositories[*].repositoryName' \
  --output table
```

You should see entries for `library/gateway`, `library/auth`, `library/catalog`, `library/reservation`, and `library/search`.
<!-- [COPY EDIT] `library/<svc>` here; ecr.md uses `library-system/<svc>`. Pick one. -->

**RDS instances:**

```bash
aws rds describe-db-instances \
  --query 'DBInstances[?starts_with(DBInstanceIdentifier,`library-system`)].{Identifier:DBInstanceIdentifier,Status:DBInstanceStatus,Endpoint:Endpoint.Address}' \
  --output table
```

You should see three rows — one per service database — with `Status` reading `available`.

**MSK cluster:**

```bash
aws kafka list-clusters \
  --query 'ClusterInfoList[*].{Name:ClusterName,State:State}' \
  --output table
```

The `State` column should read `ACTIVE`.

---

## Retrieve RDS Credentials
<!-- [COPY EDIT] Title case. -->

Terraform stored the RDS master password in AWS Secrets Manager. Retrieve it now — you will need it to create the per-service database users and to populate the Kubernetes secrets in the production overlay.
<!-- [COPY EDIT] "create the per-service database users" — not covered elsewhere. The chapter uses the RDS master user directly in DATABASE_URL. Remove this clause or add content. -->

```bash
aws secretsmanager get-secret-value \
  --secret-id library/rds/master \
  --query SecretString \
  --output text | jq .
```
<!-- [COPY EDIT] `--secret-id library/rds/master` — rds.md uses `manage_master_user_password = true`, which generates a secret with ARN pattern `rds!db-...`, not a friendly name you chose. The friendly name `library/rds/master` would require an explicit `aws_secretsmanager_secret` that the chapter does not create. Fix example. -->

Expected output:

```json
{
  "username": "library_master",
  "password": "GENERATED_PASSWORD",
  "engine": "postgres",
  "host": "library-cluster.cluster-xxxxxxxxxxxx.us-east-1.rds.amazonaws.com",
  "port": 5432,
  "dbname": "library"
}
```
<!-- [COPY EDIT] "host": "library-cluster.cluster-..." pattern is Aurora endpoint format (contains "cluster-"). The chapter provisions non-Aurora RDS. Actual RDS instance endpoint format is `<identifier>.<random>.<region>.rds.amazonaws.com`. Fix. -->
<!-- [COPY EDIT] "username": "library_master" — rds.md sets `username = "postgres"`. Inconsistent. -->

Use these values to update `deploy/k8s/overlays/production/secrets.env` before pushing to ECR and deploying. Do not commit the password to git — the overlay reads it from a local file that is listed in `.gitignore`.
<!-- [COPY EDIT] `secrets.env` not mentioned in production-overlay.md, which uses `secretGenerator` with inline literals. Inconsistent secret flow. -->

---

## Push Images to ECR

Authenticate Docker to ECR, then build and push each service image.

**Authenticate:**

```bash
ECR_REGISTRY=$(terraform output -raw ecr_registry)

aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin "$ECR_REGISTRY"
```
<!-- [COPY EDIT] `terraform output -raw ecr_registry` — no such output is declared in ecr.md (which declares `ecr_repository_urls` map). Either add the output or change this command. -->

Expected output: `Login Succeeded`

**Build images with Earthly:**

```bash
earthly +docker
```

**Tag and push each image:**

```bash
SERVICES=(gateway auth catalog reservation search)

for svc in "${SERVICES[@]}"; do
  docker tag "library-system/${svc}:latest" "${ECR_REGISTRY}/library/${svc}:latest"
  docker push "${ECR_REGISTRY}/library/${svc}:latest"
  echo "Pushed ${svc}"
done
```
<!-- [COPY EDIT] Destination path `${ECR_REGISTRY}/library/${svc}` vs. ecr.md's `library-system/${svc}`. Unify. -->
<!-- [COPY EDIT] "latest" tag only — conflicts with sha-tag practice. Consistency issue. -->

Each push uploads the image layers to the corresponding ECR repository. Layers are deduplicated across pushes — common base layers (the distroless runtime, the Go standard library) are only uploaded once. On a fast connection this completes in under two minutes.
<!-- [COPY EDIT] "the Go standard library" — Go programs statically link stdlib into the binary. It's not a separate layer. Rephrase: "shared base layers like the distroless runtime". -->

---

## Deploy to EKS

Apply the production Kustomize overlay:

```bash
kubectl apply -k deploy/k8s/overlays/production
```

The production overlay references the ECR image URIs rather than local names, and uses `StorageClass: gp3` (backed by EBS) for persistent volumes. Expected output:
<!-- [COPY EDIT] "uses `StorageClass: gp3`" — production-overlay.md treats gp3 StorageClass as OPTIONAL. The default is gp2. Contradiction. -->

```
namespace/library created
namespace/infra created
serviceaccount/catalog created
serviceaccount/reservation created
serviceaccount/auth created
serviceaccount/search created
serviceaccount/gateway created
configmap/library-config created
secret/library-secrets created
deployment.apps/auth created
service/auth created
deployment.apps/catalog created
service/catalog created
deployment.apps/reservation created
service/reservation created
deployment.apps/search created
service/search created
deployment.apps/gateway created
service/gateway created
ingress.networking.k8s.io/gateway created
```
<!-- [COPY EDIT] "namespace/infra" — not referenced anywhere else. production-overlay.md uses `library`, `data`, `messaging`. Inconsistent. -->
<!-- [COPY EDIT] "configmap/library-config" — production-overlay.md uses per-service ConfigMaps (`catalog-config`, `auth-config`, etc.). Inconsistent. -->
<!-- [COPY EDIT] "secret/library-secrets" — production-overlay.md defines `jwt-secret`, `postgres-<svc>-secret`, etc. Inconsistent. -->
<!-- [COPY EDIT] "ingress.networking.k8s.io/gateway" — production-overlay.md patches an Ingress named `library-ingress`. Inconsistent. -->

Note that the production overlay does not deploy Kafka or PostgreSQL as in-cluster workloads — those are replaced by MSK and RDS. The `infra` namespace is created for consistency but remains empty in production.

**Watch the pods come up:**

```bash
kubectl get pods -n library --watch
```

The ALB controller provisions the load balancer asynchronously after the Ingress resource is created — this can take 2–3 minutes. The pods themselves should reach `Running` within 60 seconds.

---

## Verification Checklist
<!-- [COPY EDIT] Title case. -->

Work through these steps in order.

### 1. All pods running

```bash
kubectl get pods -A
```

Expected state: every pod in the `library` namespace shows `Running` and `1/1` or `2/2` READY. The `kube-system` namespace should show CoreDNS, the AWS node daemon, and the ALB controller all running.

### 2. Ingress has an ADDRESS

```bash
kubectl get ingress -n library
```

Expected output:

```
NAME      CLASS   HOSTS   ADDRESS                                                        PORTS   AGE
gateway   alb     *       k8s-library-gateway-xxxxxxxxxxxx.us-east-1.elb.amazonaws.com   80      4m
```
<!-- [COPY EDIT] Ingress name `gateway` here; production-overlay.md patches `library-ingress`. Unify. -->

The ADDRESS column contains the ALB DNS name. If it is empty after five minutes, check the troubleshooting table below.

### 3. Gateway health check

```bash
ALB=$(kubectl get ingress -n library gateway \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

curl -s "http://${ALB}/healthz"
```

Expected response:

```json
{"status":"ok"}
```

This confirms: ALB routing, target group registration, and the Gateway pod responding to HTTP.

### 4. Catalog logs clean

```bash
kubectl logs -n library deployment/catalog
```

Look for the startup sequence: database connection established, Kafka producer initialized, gRPC server listening. No `ERROR` lines, no connection refused, no repeated retry loops.

### 5. End-to-end flow: create a book, verify in search

```bash
ALB=$(kubectl get ingress -n library gateway \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

# Create a book
curl -s -X POST "http://${ALB}/api/catalog/books" \
  -H "Content-Type: application/json" \
  -d '{"title":"The Go Programming Language","author":"Donovan & Kernighan","isbn":"978-0134190440"}' \
  | jq .
```
<!-- [COPY EDIT] "The Go Programming Language","author":"Donovan & Kernighan","isbn":"978-0134190440" — real ISBN; nice touch. Verify ISBN checksum if exact accuracy matters: 978-0134190440 is the correct ISBN-13. OK. -->

The response includes the assigned `id`. Then verify it is indexed in search — this exercises the full MSK event path:

```bash
sleep 3  # allow the Kafka consumer to process the event

curl -s "http://${ALB}/api/search?q=Go+Programming" | jq .
```

The result should contain the book you just created. If the catalog write succeeds but search returns empty, check the MSK bootstrap string in the application ConfigMap against `terraform output msk_bootstrap`.
<!-- [COPY EDIT] `terraform output msk_bootstrap` — msk.md declares outputs `msk_bootstrap_brokers_plaintext` and `msk_bootstrap_brokers_tls`. No `msk_bootstrap`. Fix. -->

---

## Troubleshooting Guide
<!-- [COPY EDIT] Title case. -->

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| `ImagePullBackOff` | ECR permissions or wrong image URI | Verify the node IAM role has `AmazonEC2ContainerRegistryReadOnly`. Check `kubectl describe pod <pod>` for the exact URI Kubernetes tried to pull. Compare with `aws ecr describe-repositories`. |
| `CrashLoopBackOff` | RDS or MSK unreachable at startup | Check security group rules: the EKS node security group must be allowed inbound on port 5432 (RDS) and 9094 (MSK TLS). Run `kubectl logs <pod> -n library --previous` for the actual error. |
<!-- [COPY EDIT] "port 9094 (MSK TLS)" — this chapter uses plaintext on 9092, not TLS on 9094. See msk.md. Troubleshooting entry wrong for the config actually deployed in 13.5. -->
| Ingress no ADDRESS | ALB controller not running or subnet tags missing | Check `kubectl get pods -n kube-system | grep aws-load-balancer`. Public subnets need `kubernetes.io/role/elb: 1` tag; private subnets need `kubernetes.io/role/internal-elb: 1`. |
| Pods `Pending` | Node capacity exhausted | Check `kubectl describe pod <pod>` for `Insufficient cpu` or `Insufficient memory`. Review the node group scaling limits in `terraform/eks.tf` and increase `max_size`. |
| RDS connection refused | RDS security group misconfigured | Verify the RDS security group has an inbound rule allowing the EKS node security group on port 5432. Check with `aws ec2 describe-security-groups --group-ids <rds-sg-id>`. |
| MSK timeout on startup | Wrong bootstrap string in ConfigMap | Run `terraform output msk_bootstrap_brokers_tls` and compare with the value in `kubectl get configmap library-config -n library -o yaml`. Update and re-apply if they differ. |
<!-- [COPY EDIT] "msk_bootstrap_brokers_tls" — 13.5 uses plaintext; production-overlay.md's ConfigMap uses plaintext. Troubleshooting check should use `msk_bootstrap_brokers_plaintext`. -->
<!-- [COPY EDIT] "library-config" ConfigMap — 13.7 uses `catalog-config`/`reservation-config`/`search-config`. No `library-config`. -->

For deeper inspection use the standard describe commands:

```bash
# Full pod event history and resource state
kubectl describe pod <pod-name> -n library

# Service endpoint registration
kubectl describe svc <service-name> -n library
```

If a Service shows no `Endpoints` in `kubectl describe svc`, no pod is matching its label selector — a common source of 502 errors from the ALB[^1].

---

## Teardown

When you are done, clean up in reverse order to avoid dependency conflicts.

**Delete the Kubernetes application resources:**

```bash
kubectl delete -k deploy/k8s/overlays/production
```

Wait for the Ingress deletion to complete before running `terraform destroy`. The ALB controller deprovisions the load balancer when the Ingress resource is deleted. If Terraform runs first, it may fail trying to delete the VPC while the ALB still holds an ENI in one of its subnets.
<!-- [STRUCTURAL] Good practical warning; readers often hit this. Keep. -->

```bash
kubectl get ingress -n library --watch
```

Wait until the Ingress disappears from the output (Ctrl+C when gone).

**Destroy all Terraform-managed infrastructure:**

```bash
terraform destroy
```

Type `yes` at the prompt. This takes 15–20 minutes. Expected final line:

```
Destroy complete! Resources: 47 destroyed.
```

**Verify cleanup:**

```bash
# Confirm no EKS clusters remain
aws eks list-clusters

# Confirm no RDS clusters remain
aws rds describe-db-clusters \
  --query 'DBClusters[*].DBClusterIdentifier'
```
<!-- [COPY EDIT] `describe-db-clusters` queries Aurora (RDS cluster) resources. Non-Aurora RDS instances require `describe-db-instances`. Since chapter uses `aws_db_instance`, use the latter. -->

```bash
# Confirm no MSK clusters remain
aws kafka list-clusters \
  --query 'ClusterInfoList[*].ClusterName'
```

All three should return empty lists.

**Check for orphaned EBS volumes:**

EBS volumes provisioned by the EKS storage driver (for PersistentVolumeClaims) are sometimes not deleted automatically when the cluster is torn down. Check for leftover volumes:

```bash
aws ec2 describe-volumes \
  --filters "Name=status,Values=available" \
            "Name=tag-key,Values=kubernetes.io/cluster/library-production" \
  --query 'Volumes[*].{ID:VolumeId,Size:Size,AZ:AvailabilityZone}' \
  --output table
```

Delete any listed volumes manually:

```bash
aws ec2 delete-volume --volume-id <volume-id>
```

An orphaned volume costs roughly $0.08 per GB per month[^2]. It is small but it accumulates silently — always check.
<!-- [COPY EDIT] Please verify: gp2 $/GB-month in us-east-1 is $0.10. gp3 is $0.08. Specify which. -->

---

## Expected Outputs for Non-Deployers
<!-- [COPY EDIT] Title case. -->

If you are following along without running the infrastructure, here is a summary of what a successful deployment looks like at each milestone:

| Step | Expected terminal output |
|------|--------------------------|
| `terraform apply` complete | `Apply complete! Resources: 47 added, 0 changed, 0 destroyed.` |
| `aws eks update-kubeconfig` | `Updated context arn:aws:eks:us-east-1:...:cluster/library-production in ~/.kube/config` |
| `kubectl get pods -A` | All `library` pods `Running 1/1`, restarts 0 |
| `kubectl get ingress -n library` | ADDRESS column populated with an ELB hostname |
| `curl .../healthz` | `{"status":"ok"}` with HTTP 200 |
| `curl .../api/catalog/books` (POST) | JSON body with `"id"` field |
| `curl .../api/search?q=...` | JSON array containing the created book |
| `terraform destroy` complete | `Destroy complete! Resources: 47 destroyed.` |

---

## What's Next
<!-- [COPY EDIT] "What's Next" uses apostrophe "'s" — fine; verify smart-quote preservation in static HTML build. -->

The library system is running on AWS with managed database, message broker, and load balancer infrastructure. Chapter 14 hardens the deployment for production: configuring DNS with Route 53 and TLS with ACM, managing secrets with External Secrets Operator, and encrypting Kafka traffic. None of these changes touch application code — everything lives in Terraform and the production Kustomize overlay.

---

[^1]: Debugging Kubernetes Services: https://kubernetes.io/docs/tasks/debug/debug-application/debug-service/
[^2]: AWS EBS Pricing: https://aws.amazon.com/ebs/pricing/
<!-- [FINAL] These footnotes ARE cited inline (line ~335, line ~404). First file in chapter with proper citations. Good. -->
