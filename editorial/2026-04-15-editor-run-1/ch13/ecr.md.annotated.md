# 13.3 ECR — Container Registry for EKS

<!-- [STRUCTURAL] Section scope is clear and tight. Good motivation via GHCR vs ECR contrast. "Why ECR over GHCR for EKS" delivers on its heading. Recommend one structural addition: a note on ECR pricing (storage $/GB/mo + data out) — fits the chapter's cost-awareness theme. -->
<!-- [COPY EDIT] Heading style inconsistent with index.md roadmap ("13.3 — Amazon ECR and the Build Pipeline"). Here: "13.3 ECR — Container Registry for EKS". Unify. -->

Chapter 10 built a CI pipeline that publishes five Docker images to the GitHub Container Registry (GHCR) on every push to `main`. GHCR is the right choice for a project hosted on GitHub — authentication is automatic via `GITHUB_TOKEN`, and the registry lives next to the source. When running on EKS, though, GHCR is an external registry. Every node that pulls an image needs credentials, and those credentials need to be rotated, distributed, and kept out of your manifests.

Amazon Elastic Container Registry (ECR) is the AWS-native alternative.[^1] It stores images in the same AWS account as the EKS cluster, and authentication between the two is handled by IAM. A node role with the right policy can pull images from ECR without a `Secret`, without `imagePullSecrets` in every pod spec, and without managing credentials at all. For workloads running inside AWS, ECR is the path of least resistance.
<!-- [LINE EDIT] "path of least resistance" — mild cliché (same as index.md). Consider: "the simplest integration". -->

---

## Why ECR over GHCR for EKS

The difference is not about capability — both registries store OCI-compatible images. The difference is about where authentication lives.

With GHCR and Kubernetes, you need a `kubernetes.io/dockerconfigjson` Secret in every namespace that pulls images. You create it once with `kubectl create secret docker-registry`, but you also need to rotate it when the token expires, distribute it to every new namespace, and reference it in every `Deployment` manifest under `imagePullSecrets`. In a multi-namespace cluster, this becomes operational overhead.
<!-- [COPY EDIT] Please verify: the Kubernetes dockerconfigjson secret type is `kubernetes.io/dockerconfigjson` (lowercase). Correct. -->
<!-- [LINE EDIT] "In a multi-namespace cluster, this becomes operational overhead." — "this" refers to the whole chain; acceptable. -->

With ECR and EKS, the EKS node group has an IAM instance role. Attaching the `AmazonEC2ContainerRegistryReadOnly` managed policy to that role allows every pod on the node to pull from ECR without any in-cluster credential machinery. The kubelet on each node calls the ECR API using the instance role, receives a temporary token valid for twelve hours, and caches it. No Secrets, no `imagePullSecrets`, no rotation scripts.
<!-- [COPY EDIT] Please verify: ECR authorization token TTL is 12 hours. Confirmed per AWS docs. OK. -->
<!-- [COPY EDIT] "twelve hours" — CMOS 9.3: spell out zero–ninety-nine in prose; "12 hours" also acceptable when paired with units. Either works. -->

This is the IAM-native authentication pattern that runs through all AWS service integrations: instead of distributing credentials, you attach policies to roles that the compute nodes already hold.

---

## ecr.tf

<!-- [COPY EDIT] Heading "ecr.tf" lowercase; other sections use title case ("Security groups", "Outputs"). Consider "The `ecr.tf` file". -->

Create `terraform/ecr.tf`. This file provisions one ECR repository per service and attaches a lifecycle policy to each.

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

  tags = {
    Project = "library-system"
    Service = each.key
  }
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each   = aws_ecr_repository.services
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
        description  = "Keep last 20 tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["sha-", "latest"]
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
<!-- [COPY EDIT] Please verify: lifecycle policy JSON structure (`rulePriority`, `selection.tagStatus`, `selection.countType`, `selection.countUnit`, `selection.countNumber`, `action.type`) matches AWS ECR lifecycle policy syntax. Confirmed per AWS ECR docs. OK. -->
<!-- [COPY EDIT] Please verify: `aws_ecr_lifecycle_policy` resource name and `repository` argument. Correct per Terraform AWS provider docs. -->

A few things worth noting in this configuration.
<!-- [LINE EDIT] "A few things worth noting" — fragment. Add "are". -->

`for_each = toset(local.services)` iterates over the services list, treating it as a set to guarantee uniqueness. Terraform creates one `aws_ecr_repository` resource per service, accessible as `aws_ecr_repository.services["catalog"]`, `aws_ecr_repository.services["auth"]`, and so on. Adding a new service to `local.services` and running `terraform apply` creates the new repository and its lifecycle policy automatically.

`image_tag_mutability = "MUTABLE"` allows overwriting existing tags. This is needed because the `latest` tag is pushed on every build. If you set this to `IMMUTABLE`, every push must use a unique tag — `latest` would fail after the first push. The `sha-<commit>` tags are effectively immutable in practice (you would never rebuild the same commit), but enforcing immutability at the registry level makes `latest` unusable.

`scan_on_push = true` triggers ECR's built-in vulnerability scanning using Amazon Inspector every time an image is pushed.[^2] Scan results appear in the ECR console and can be retrieved via the AWS CLI. This is a free, zero-configuration security baseline. It does not block deployments on findings — that would require a separate policy or CI gate — but it gives you visibility without additional tooling.
<!-- [COPY EDIT] Please verify: "scan_on_push = true" triggers *basic* scanning, which uses Clair (not Inspector). *Enhanced* scanning uses Amazon Inspector and requires enabling at the registry level separately. Correct this or clarify. -->
<!-- [COPY EDIT] "free, zero-configuration security baseline" — Basic scanning is free; enhanced scanning is billed per image scanned. If body actually means basic, the "free" claim holds; if enhanced, it does not. -->

The lifecycle policy has two rules, evaluated in priority order. Rule 1 expires untagged images after 14 days. Untagged images accumulate whenever a push overwrites the `latest` tag — the previous image loses its tag but its layers remain. Without a lifecycle policy, these accumulate indefinitely and appear on your ECR bill. Rule 2 keeps the last 20 images tagged with `sha-` or `latest` prefixes. On an active project with multiple deployments per day, this retains roughly two to three weeks of history, which is enough to roll back to any recent deployment.
<!-- [COPY EDIT] "two to three weeks" — CMOS 9.56: consider "2 to 3 weeks" only for technical numerics; spelled form is fine here. OK. -->
<!-- [LINE EDIT] "appear on your ECR bill" — light. Good. Keep. -->

---

## Image Tagging Strategy

<!-- [COPY EDIT] Heading "Image Tagging Strategy" uses title case; others in this file use sentence case ("Why ECR over GHCR for EKS", "Outputs"). Pick one and apply consistently (CMOS 8.158 allows either if consistent). -->

ECR uses the same two-tag strategy from Chapter 10: `sha-<commit>` and `latest`. The full image URI changes to reflect the ECR endpoint:

```
<account-id>.dkr.ecr.<region>.amazonaws.com/library-system/catalog:sha-abc1234
<account-id>.dkr.ecr.<region>.amazonaws.com/library-system/catalog:latest
```

The account ID and region make ECR URIs more verbose than GHCR equivalents, but the semantics are identical. `sha-<commit>` is immutable and production-safe; `latest` is mutable and convenient for local pulls.

In Kubernetes manifests committed to the GitOps repository, always use `sha-<commit>` tags. The Kustomize `images` transformer in the production overlay rewrites the base image name to the full ECR URI before apply:
<!-- [COPY EDIT] "GitOps repository" — this section predates the ArgoCD discussion (13.10). Either define GitOps briefly here or use "Git repository" to avoid forward reference. -->

```yaml
# k8s/overlays/production/kustomization.yaml
images:
  - name: catalog
    newName: 123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/catalog
    newTag: sha-abc1234f
```
<!-- [COPY EDIT] "catalog" as the source name here; production-overlay.md uses `library-system/catalog`. Inconsistent. Kustomize matches on exact image name in base; verify base manifest names. -->

This keeps the base manifests portable — they reference short names like `catalog` — while the production overlay injects the environment-specific registry and tag. A CI job updates `newTag` in this file as part of the deploy pipeline, committing the change to trigger a GitOps reconciliation.
<!-- [COPY EDIT] "committing the change to trigger a GitOps reconciliation" — cicd.md does NOT commit the change; it runs `kubectl apply -k` directly (push pipeline). ArgoCD commit pattern is only in 13.10. Adjust to match 13.8. -->

---

## Outputs

Add an `outputs.tf` block (or extend an existing one) to expose the repository URLs for use in CI and Kustomize:

```hcl
output "ecr_repository_urls" {
  description = "ECR repository URLs keyed by service name"
  value = {
    for k, repo in aws_ecr_repository.services : k => repo.repository_url
  }
}
```
<!-- [COPY EDIT] deploying.md uses `terraform output -raw ecr_registry` (a scalar registry URL). This map is different. The body of deploying.md needs a matching output named `ecr_registry` OR this one will be used differently. Add a second output or reconcile. -->

After `terraform apply`, the output looks like:

```
ecr_repository_urls = {
  "auth"        = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/auth"
  "catalog"     = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/catalog"
  "gateway"     = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/gateway"
  "reservation" = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/reservation"
  "search"      = "123456789012.dkr.ecr.us-east-1.amazonaws.com/library-system/search"
}
```

The GitHub Actions push workflow reads these URLs with `terraform output -json ecr_repository_urls` and substitutes them into the `docker/build-push-action` tags. The Kustomize overlay consumes the same values when constructing the `newName` field in the `images` transformer.

---

## References

[^1]: [Amazon ECR documentation](https://docs.aws.amazon.com/AmazonECR/latest/userguide/what-is-ecr.html) — Overview of ECR concepts, authentication methods, and registry types (private, public, pull-through cache).
[^2]: [ECR image scanning with Amazon Inspector](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-scanning.html) — Configuring enhanced scanning, interpreting findings, and setting up EventBridge notifications for critical vulnerabilities.
[^3]: [Kustomize images transformer](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/images/) — Reference for the `images` field in `kustomization.yaml`, covering `newName`, `newTag`, and `digest` overrides.
[^4]: [Amazon ECR lifecycle policies](https://docs.aws.amazon.com/AmazonECR/latest/userguide/LifecyclePolicies.html) — Full lifecycle policy syntax, rule evaluation order, and examples for common retention patterns.
<!-- [FINAL] This file uses markdown links in footnotes; other files use bare URLs. Style inconsistency. -->
<!-- [FINAL] Footnotes `[^3]` and `[^4]` are defined but not cited inline. -->
