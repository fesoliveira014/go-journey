# 13.1 Terraform Fundamentals

<!-- [STRUCTURAL] The section introduces concepts in logical order: providers → resources → data sources → variables → outputs → modules → state. That's the right progression. Consider a brief "what this section does not cover" note (e.g., workspaces, imports, refactoring with `moved` blocks) so readers know the scope. -->

Chapter 12 ended with every service running in a Kubernetes cluster on your laptop. Kubernetes answered the question of *what* to run: which containers, how many replicas, what configuration, which volumes. But it left another question open: *where* does the cluster itself run when you move to production?

<!-- [LINE EDIT] "Assembling all of that by clicking through the AWS console works once, but it does not scale and it does not survive a new team member, a disaster recovery drill, or a cost-cutting experiment in a second region." — 38 words, borderline long but lively. Keep. -->
The answer is cloud infrastructure — virtual machines for nodes, a managed control plane, a VPC with subnets and routing tables, a container registry, a managed database, a managed Kafka cluster, IAM roles for service accounts. Assembling all of that by clicking through the AWS console works once, but it does not scale and it does not survive a new team member, a disaster recovery drill, or a cost-cutting experiment in a second region. You need a way to describe infrastructure in files, version-control those files alongside your application code, and apply changes reproducibly.
<!-- [COPY EDIT] "version-control those files" — "version-control" as a verb is hyphenated when used attributively (compound verb). Acceptable. -->

That is exactly what Infrastructure as Code tools do. Terraform is the most widely adopted of them.
<!-- [LINE EDIT] "That is exactly what" → "That is what" (cut "exactly"). -->
<!-- [COPY EDIT] "Infrastructure as Code" — per CMOS 8.1 title-case proper-noun policy, "Infrastructure as Code" (IaC) is acceptable as a coined term. Subsequent uses often go to "IaC"; define at first use (done). -->
<!-- [COPY EDIT] "Terraform is the most widely adopted of them." — Please verify: With HashiCorp's 2023 BSL license change and the OpenTofu fork (2024), "most widely adopted" is still accurate but worth a footnote mention. -->

---

## What Terraform is

Terraform is an open-source IaC tool created by HashiCorp. You write configuration in **HCL** (HashiCorp Configuration Language) — a declarative syntax designed to be readable by humans and parseable by machines — and Terraform figures out how to make the real world match what you described.
<!-- [COPY EDIT] "open-source" compound adjective before noun — correct hyphenation (CMOS 7.81). -->
<!-- [COPY EDIT] "Terraform is an open-source IaC tool" — since 2023, Terraform ships under the BSL license, not OSI-approved open source. Consider "source-available IaC tool" or footnote. -->

The mental model is deliberately similar to Kubernetes. In Chapter 12 you wrote a Deployment manifest that said "I want three replicas of the catalog service." Kubernetes reconciled reality toward that desired state. In Terraform you write a resource block that says "I want a VPC with this CIDR block." Terraform computes a plan — the diff between what exists and what you described — and applies it. The difference is that Kubernetes manages workloads inside a cluster; Terraform manages the cloud primitives the cluster itself sits on.

Terraform is not tied to AWS. The same tool drives resources in GCP, Azure, Cloudflare, GitHub, Datadog, and hundreds of other providers. Everything in this chapter uses the AWS provider, but the concepts transfer directly.

---

## Core concepts

### Providers

A **provider** is a plugin that translates Terraform resource definitions into API calls for a specific platform. Before Terraform can create any AWS resource, it needs the AWS provider configured with credentials and a target region.

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}
```

The `terraform` block pins the provider version — a critical practice, because provider updates can introduce breaking changes. The `~> 5.0` constraint allows patch and minor releases within the 5.x series but blocks a jump to 6.x. The `provider` block supplies runtime configuration; `var.region` references a variable defined elsewhere (covered below).
<!-- [COPY EDIT] "the 5.x series but blocks a jump to 6.x" — Please verify: `~> 5.0` in HCL actually constrains to `>= 5.0, < 6.0` (Pessimistic constraint). Correct. -->
<!-- [COPY EDIT] Please verify: current aws provider version — plan output example below shows v5.43.0. AWS provider 5.x is current; 6.x released in 2024. Confirm "~> 5.0" still pins correctly against 5.x. -->

Terraform downloads providers when you run `terraform init`. They are stored in a `.terraform/` directory local to your project and should not be committed to source control.

### Resources

A **resource** is the fundamental unit in Terraform — a single piece of infrastructure that Terraform creates, updates, and destroys.

```hcl
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name    = "library-vpc"
    Project = "library"
  }
}
```

<!-- [COPY EDIT] Please verify: `aws_vpc` resource arguments `cidr_block`, `enable_dns_support`, `enable_dns_hostnames`, `tags` — all valid per hashicorp/aws docs. OK. -->

The first argument to `resource` is the type (`aws_vpc`), which is defined by the provider. The second is a local name (`main`) used to reference this resource elsewhere in the configuration. Together they form the address `aws_vpc.main`, which you will see in plan output and in cross-resource references like `aws_vpc.main.id`.

After `terraform apply` runs, Terraform stores the resource's real-world identifiers — the VPC ID AWS assigned, its state, its ARN — in state. On the next `apply`, it compares the configuration to state and to the live resource, and only makes changes if something diverged.
<!-- [LINE EDIT] "and only makes changes if something diverged" → "making changes only when something has diverged" (or keep; original is acceptable). -->

### Data sources

A **data source** reads existing infrastructure rather than creating it. This is useful for referencing resources managed outside Terraform — shared resources, resources created by another team's configuration, or resources that predate your IaC adoption.

```hcl
data "aws_caller_identity" "current" {}

data "aws_availability_zones" "available" {
  state = "available"
}
```
<!-- [COPY EDIT] Please verify: `aws_caller_identity` and `aws_availability_zones` data sources — both valid. -->

`aws_caller_identity` returns the AWS account ID and ARN of the current caller. `aws_availability_zones` returns the list of AZs available in the configured region. Both are referenced elsewhere as `data.aws_caller_identity.current.account_id` and `data.aws_availability_zones.available.names`.

### Variables

**Variables** parameterize configurations so the same Terraform code can deploy to different environments or regions without edits.

```hcl
variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "db_password" {
  description = "Password for the RDS PostgreSQL instance"
  type        = string
  sensitive   = true
}
```

The `sensitive = true` flag tells Terraform to redact the value from plan and apply output. Variable values can be supplied on the command line (`-var="region=us-west-2"`), in a `.tfvars` file, or via environment variables prefixed with `TF_VAR_` (e.g., `TF_VAR_db_password`). The `.tfvars` files holding real secrets should be in `.gitignore`.
<!-- [COPY EDIT] "e.g.," — CMOS 6.43 requires comma after "e.g." (present). OK. -->
<!-- [LINE EDIT] "should be in `.gitignore`" → "belong in `.gitignore`" (more direct). -->

### Outputs

**Outputs** expose values from your configuration for use by other systems — a CI/CD pipeline, a downstream Terraform module, or a human reading the apply output.

```hcl
output "vpc_id" {
  description = "ID of the primary VPC"
  value       = aws_vpc.main.id
}

output "eks_cluster_endpoint" {
  description = "Endpoint for the EKS API server"
  value       = module.eks.cluster_endpoint
}
```

After `terraform apply` completes, all defined outputs are printed to stdout. You can also retrieve them later with `terraform output vpc_id`.

### Modules

A **module** is a directory of Terraform files that can be instantiated as a unit — the equivalent of a function in an imperative language. Modules promote reuse and encapsulation: complex resources with many interdependencies can be wrapped in a module with a clean input/output interface.

```hcl
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.1.1"

  name = "library-vpc"
  cidr = "10.0.0.0/16"

  azs             = data.aws_availability_zones.available.names
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway = true
  single_nat_gateway = true
}
```

<!-- [COPY EDIT] "version = '5.1.1'" — networking.md uses `"~> 5.0"`. Inconsistent pinning strategy across examples. Mention both or unify. -->

`source` can be a local path (`./modules/vpc`), a Terraform Registry address (as above), or a GitHub URL. The `terraform-aws-modules/vpc/aws` module from the public registry is one of the most widely used in the ecosystem — it encapsulates a VPC, subnets, route tables, internet gateways, and NAT gateways into a few dozen input variables. Using it saves hundreds of lines of raw resource blocks and encodes years of community best practices.
<!-- [LINE EDIT] "saves hundreds of lines of raw resource blocks and encodes years of community best practices" → "saves hundreds of lines of raw resource blocks and bakes in community best practices". (Minor; "encodes years" is fine.) -->

### State

**State** is Terraform's record of the infrastructure it manages. It is a JSON file — `terraform.tfstate` by default — that maps each resource block in your configuration to the corresponding real-world object, including all of its attributes as of the last apply.

State is what enables Terraform to compute a diff. On each `plan` or `apply`, Terraform reads the state, calls the provider APIs to refresh the actual attributes, compares both against your configuration, and produces a list of additions, changes, and deletions.

State is also Terraform's most sensitive artifact. It contains every attribute of every managed resource — including secrets, private keys, and database passwords — in plaintext. Treat `terraform.tfstate` with the same care you give production credentials.
<!-- [LINE EDIT] "Treat `terraform.tfstate` with the same care you give production credentials." — good line. Keep. -->

---

## State management

<!-- [STRUCTURAL] This subsection correctly flags the bootstrap problem but then tells readers to stay local. Good pragmatic call for a learning book. Could add a forward reference ("you'll revisit this in 13.x") — already done at end. -->

For learning purposes, local state is fine. Terraform writes `terraform.tfstate` to the project directory, and nothing else is required. The risk is real: if you lose the file, Terraform loses track of what it created and you must either import resources manually or destroy and recreate them.

For any shared or long-lived environment, **remote state** is the standard. AWS's canonical setup uses an S3 bucket for storage and a DynamoDB table for locking — the lock prevents two operators from running `apply` simultaneously and producing conflicting state.
<!-- [COPY EDIT] Please verify: as of Terraform 1.10 (2024), S3 backend supports native lockfile-based locking via `use_lockfile = true`, making DynamoDB optional. Worth a footnote. -->

The backend configuration goes in a `backend.tf` file. The resources it references (the S3 bucket and DynamoDB table) must exist before you can configure them as a backend, which creates a bootstrapping problem: those resources are often created by a separate, minimal Terraform configuration sometimes called a "bootstrap" or "foundation" layer.

```hcl
# backend.tf
# Uncomment this block after creating the S3 bucket and DynamoDB table
# (see terraform/bootstrap/ for the configuration that creates them).

# terraform {
#   backend "s3" {
#     bucket         = "library-terraform-state"
#     key            = "prod/terraform.tfstate"
#     region         = "us-east-1"
#     encrypt        = true
#     dynamodb_table = "library-terraform-locks"
#   }
# }
```

<!-- [LINE EDIT] "While working through this chapter, leave the backend block commented out." — good. -->
While working through this chapter, leave the backend block commented out. The local state file is sufficient, and it avoids the prerequisite of creating AWS resources just to store Terraform's bookkeeping. Section 13.7 revisits remote state when discussing production hardening.
<!-- [COPY EDIT] "Section 13.7 revisits remote state" — production-overlay.md does NOT discuss remote state. Verify/remove. -->

---

## Project structure

A Terraform project is just a directory of `.tf` files. Terraform loads all files in the directory as a single configuration, so there is no semantic significance to how you split content across files. By convention, the standard split used in this chapter groups resources by concern:
<!-- [LINE EDIT] "is just a directory" — "just" is filler. → "is a directory of `.tf` files." -->

```
terraform/
  main.tf        # Provider configuration and terraform block
  variables.tf   # All input variable declarations
  outputs.tf     # All output declarations
  backend.tf     # Remote state backend (commented out initially)
  vpc.tf         # VPC, subnets, routing, security groups
  ecr.tf         # Elastic Container Registry repositories
  rds.tf         # RDS PostgreSQL instance
  msk.tf         # Amazon MSK (managed Kafka) cluster
  eks.tf         # EKS cluster and node groups
  cicd.tf        # IAM roles and policies for GitHub Actions
```
<!-- [COPY EDIT] "RDS PostgreSQL instance" (singular) — rds.md provisions three. → "RDS PostgreSQL instances". -->
<!-- [COPY EDIT] The directory layout shown here is `terraform/`; cicd.md uses `infrastructure/`. Unify. -->

This is not the only valid structure. Large configurations are sometimes organized into subdirectories as separate modules with explicit composition. For a project of this size — a handful of major AWS services — flat files grouped by resource domain are readable and maintainable.

Each subsequent section in this chapter adds one file to this directory and walks through its content in detail. By the end, `terraform apply` will provision the complete AWS environment the library system runs in.

---

## The Terraform workflow

Four commands cover almost everything you will do in this chapter.
<!-- [LINE EDIT] "almost everything" — OK but slightly hedged. Consider "cover the core of what you will do". -->

**`terraform init`** initializes the working directory. It downloads the providers declared in the `terraform` block, initializes the backend (local or remote), and prepares the `.terraform/` directory. You run this once when setting up a new project, and again whenever you add or change providers or modules.

```
$ terraform init

Initializing the backend...
Initializing provider plugins...
- Finding hashicorp/aws versions matching "~> 5.0"...
- Installing hashicorp/aws v5.43.0...
Terraform has been successfully initialized!
```

**`terraform plan`** computes the difference between your configuration and the current state (refreshed from AWS). It prints every resource that will be created, changed, or destroyed — the exact changes, attribute by attribute — without making any real-world modifications. Reading the plan carefully before applying is the single most important habit to build. A misplaced resource block that would delete a production database shows up here, not after the fact.

```
$ terraform plan

Plan: 23 to add, 0 to change, 0 to destroy.
```

<!-- [COPY EDIT] "15 to 20 minutes" — deploying.md also says "15–20 minutes". Use en dash (CMOS 6.78) for ranges. Apply consistently. Here "15 to 20 minutes" is acceptable in running prose. -->
**`terraform apply`** executes the plan. It prompts for confirmation (type `yes`) unless you pass `-auto-approve`. Creating the full infrastructure in this chapter — VPC, ECR repositories, RDS instance, MSK cluster, EKS cluster — takes roughly 15 to 20 minutes, dominated by EKS cluster creation (10 minutes) and MSK cluster creation (5 minutes). This is normal; AWS is provisioning real infrastructure.

```
$ terraform apply

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes

Apply complete! Resources: 23 added, 0 changed, 0 destroyed.
```
<!-- [COPY EDIT] Resource count "23" contradicts deploying.md's "47 added". Pick one canonical number or note that this is an illustrative example. -->

**`terraform destroy`** tears down every resource in state. It prints the same kind of plan output (all deletions) and prompts for confirmation. Use this at the end of a learning session to avoid ongoing AWS charges. Managed services like RDS, MSK, and EKS accrue cost even when idle.

```
$ terraform destroy

Plan: 0 to add, 0 to change, 23 to destroy.

Do you really want to destroy all resources?
  Terraform will destroy all your managed infrastructure.
  There is no undo. Only 'yes' will be accepted to confirm.

  Enter a value: yes
```

<!-- [LINE EDIT] "Always read the plan output before typing `yes`" — good imperative. Keep. -->
The workflow mirrors what you know from `kubectl`: describe the desired state, preview the changes, apply them. The key difference is that `terraform apply` is not idempotent in the same sense as `kubectl apply` — some changes (resizing an RDS instance, modifying a security group rule) require brief downtime or trigger resource replacement. Always read the plan output before typing `yes`, and pay particular attention to any line marked `forces replacement`.
<!-- [COPY EDIT] "not idempotent in the same sense as `kubectl apply`" — this is technically imprecise. Both `apply` commands are idempotent in the reconciliation sense. What differs is that some Terraform resource changes cause destructive replacement. Rephrase: "the key difference is that some Terraform changes trigger destructive replacement of the underlying resource". -->

---

The remaining sections build out each `.tf` file in turn, starting with the network foundation in `vpc.tf` and working up to the EKS cluster and the CI/CD IAM configuration.

---

[^1]: Terraform Documentation: https://developer.hashicorp.com/terraform/docs
[^2]: Terraform AWS Provider: https://registry.terraform.io/providers/hashicorp/aws/latest/docs
[^3]: terraform-aws-modules/vpc: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/latest
[^4]: Terraform Language Reference: https://developer.hashicorp.com/terraform/language
[^5]: S3 Backend Configuration: https://developer.hashicorp.com/terraform/language/settings/backends/s3
<!-- [FINAL] Footnotes defined but not cited inline. See same issue in index.md. -->
