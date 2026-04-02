# 14.1 — DNS with Route 53

At the end of Chapter 13, the library system was reachable — but only via a hostname that looks something like `k8s-library-ingress-a1b2c3d4e5-987654321.us-east-1.elb.amazonaws.com`. That URL works, but it is not something you would print on a business card, give to a user, or embed in a mobile app. It is also not a hostname you can obtain a TLS certificate for in any meaningful way — a certificate for `*.us-east-1.elb.amazonaws.com` is not yours to request, and ACM (which you will use in section 14.2) requires that you demonstrate control over the domain you are issuing a certificate for.

A custom domain solves three problems at once: it gives users a stable, human-readable address; it gives you a hostname you can anchor a TLS certificate to; and it decouples your application's public identity from whatever AWS generates internally. When your ALB is replaced — during a cluster migration, a region failover, or a blue/green deployment — the DNS record is the only thing that changes. Clients see no difference.

This section wires up a custom domain using **Route 53**, AWS's managed DNS service. The Terraform is concise: one data source to look up your hosted zone, one data source to look up the ALB, and one record that points your domain at the load balancer using an AWS alias record.

---

## How DNS works — a brief refresher

When a browser navigates to `library.example.com`, it asks a DNS resolver (typically provided by your ISP or a public resolver like `8.8.8.8`) to translate the name to an IP address. The resolver walks a tree of authoritative name servers: first the root (`.`), then the TLD (`.com`), then `example.com`'s authoritative name server — Route 53, in your case. Route 53 returns a record, and the resolver hands the IP to the browser.

There are three record types relevant here.

**A records** map a hostname directly to one or more IPv4 addresses. They are the simplest and most common record type. The problem with using a plain A record for an ALB is that the ALB's IP addresses are not static — AWS changes them as the load balancer scales or fails over across Availability Zones. Hardcoding an IP in an A record and hoping it never changes is not a strategy you want to rely on.

**CNAME records** map one hostname to another hostname, and the resolver follows the chain until it reaches an A record. You could in theory create a CNAME pointing `library.example.com` to the ALB's generated hostname. However, CNAMEs cannot be placed at the **zone apex** — the root of the domain itself (e.g., `example.com` without a subdomain). This is a constraint in the DNS protocol: a zone apex must always contain an SOA and NS record, and the presence of a CNAME at the apex would be technically invalid. If your application lives at the root of a domain rather than a subdomain, CNAMEs do not work.

**AWS alias records** are Route 53's solution to both problems.[^1] An alias record behaves like a CNAME in that it points to another DNS name, but Route 53 resolves it internally and returns the current A record of the target — the actual IP addresses of the ALB at query time. Because Route 53 resolves the alias within AWS, there is no extra DNS hop from the client's perspective. Alias records are free (Route 53 does not charge per query for alias records pointing to AWS resources), they work at the zone apex, and they support health evaluation: Route 53 can automatically stop returning the record if the target resource is unhealthy.

---

## Route 53 concepts

**Hosted zones** are Route 53's container for DNS records for a single domain. A **public hosted zone** serves DNS responses to the open internet — any resolver worldwide can query it for `library.example.com`. A **private hosted zone** is visible only within one or more VPCs and is used for internal service discovery. You need a public hosted zone.

When you register a domain through Route 53 (or point an externally-registered domain at Route 53 by updating the domain's NS records with your registrar), Route 53 creates the hosted zone and becomes the authoritative name server for that domain. Every record you add to the hosted zone is served by Route 53's globally distributed DNS infrastructure.

**Record sets** are the individual DNS entries within a hosted zone. Each record has a name, a type, a TTL, and a value (or, for alias records, a target resource). You can have multiple records with the same name but different types.

**Routing policies** control how Route 53 selects which record to return when there are multiple records with the same name. For this section you only need **simple routing**, which returns all values for a name without any health-check or geographic logic. The other policies — weighted, latency-based, failover, geolocation — are relevant for multi-region deployments and are outside the scope of this chapter.

---

## Terraform: `terraform/dns.tf`

Create the file `terraform/dns.tf`:

```hcl
variable "domain_name" {
  description = "Domain name for the application (e.g., library.example.com)"
  type        = string
}

# Option 1: Use existing hosted zone (if you already own the domain)
data "aws_route53_zone" "main" {
  name         = var.domain_name
  private_zone = false
}

# Option 2: Create a new hosted zone (uncomment if registering fresh)
# resource "aws_route53_zone" "main" {
#   name = var.domain_name
# }

# Alias record pointing the domain to the ALB
resource "aws_route53_record" "app" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = data.aws_lb.ingress.dns_name
    zone_id                = data.aws_lb.ingress.zone_id
    evaluate_target_health = true
  }
}

# Look up the ALB created by the AWS Load Balancer Controller.
# The LB is created asynchronously by the controller after the Ingress
# resource is applied — this data source reads it by tag.
data "aws_lb" "ingress" {
  tags = {
    "elbv2.k8s.aws/cluster" = var.cluster_name
  }
}
```

Walk through each block.

**`variable "domain_name"`** is a new input variable. You will set it in `terraform.tfvars` alongside `cluster_name` and `github_repo`. It should be the full domain name you are pointing at the application: `library.example.com` if you are creating a subdomain record, or `example.com` if you are using the zone apex.

**`data "aws_route53_zone" "main"`** looks up an existing hosted zone by name. The `private_zone = false` filter ensures you only match a public hosted zone, not a private one that might share the same name. If you have not yet created the hosted zone — for example, you just registered a fresh domain — you would comment out this data source and uncomment the `aws_route53_zone` resource block instead. The resource block creates the hosted zone; you would then need to copy the NS records it assigns and update your domain registrar to delegate to Route 53.[^2]

**`resource "aws_route53_record" "app"`** creates the alias A record. The key fields are:

- `zone_id` — the ID of the hosted zone that owns this record.
- `name` — the fully qualified domain name the record answers for. Setting it to `var.domain_name` means the record is at the zone apex or at the exact subdomain you specified.
- `type = "A"` — even though this is an alias, the DNS record type is still `A`. Route 53 resolves the alias internally and returns A records to callers.
- `alias.name` and `alias.zone_id` — the ALB's DNS name and its canonical hosted zone ID. Both are read from the `data.aws_lb.ingress` data source described next. Note that the `zone_id` inside the `alias` block is the ALB's hosted zone ID — a static identifier assigned by AWS per region, distinct from your Route 53 hosted zone ID.
- `evaluate_target_health = true` — instructs Route 53 to probe the ALB's health endpoints before including it in DNS responses. If the ALB becomes unhealthy, Route 53 can stop returning the record and let any failover configuration take effect.

**`data "aws_lb" "ingress"`** looks up the Application Load Balancer by the tag that the AWS Load Balancer Controller applies when it provisions the ALB on behalf of the Kubernetes Ingress resource.[^3] When you apply the Ingress manifest, the controller creates the ALB and tags it with `elbv2.k8s.aws/cluster = <cluster-name>`. The data source reads those tags and exposes the ALB's attributes — `dns_name` and `zone_id` — for use in the alias record.

---

## The chicken-and-egg ordering problem

There is a sequencing dependency here that Terraform's dependency graph cannot automatically resolve. The `data "aws_lb" "ingress"` data source reads an ALB that does not exist in AWS until after you deploy the Kubernetes Ingress resource — and the Ingress resource is deployed with `kubectl apply`, outside of Terraform. If you run `terraform apply` before deploying the application, the data source will fail with "No load balancers found."

Two approaches handle this.

**Approach (a): apply Terraform after deploying the app.** Deploy the Kubernetes manifests first — including the Ingress resource — wait for the AWS Load Balancer Controller to finish provisioning the ALB (typically 30–60 seconds), then run `terraform apply`. The data source will find the ALB, and everything proceeds in a single apply. This is the recommended approach for this project. It matches the natural deployment order: infrastructure comes up first, then the application, then DNS and TLS are layered on top.

**Approach (b): `depends_on` with a `null_resource`.** Terraform has a `null_resource` that can depend on external commands via a `local-exec` provisioner. You could write a `null_resource` that runs `kubectl apply -f deploy/` and then make the `aws_lb` data source implicitly depend on it by referencing something in that null_resource. This works, but it couples your Terraform to your `kubectl` configuration and makes the apply non-idempotent. Avoid it unless you have a specific reason to manage the full deployment sequence inside a single `terraform apply`.

For the remainder of this chapter, use approach (a): deploy the Kubernetes manifests first, confirm the ALB is provisioned, then apply the Terraform in this section.

---

## Outputs

Add the following to `terraform/outputs.tf`:

```hcl
output "app_domain" {
  description = "Application domain name"
  value       = var.domain_name
}
```

After `terraform apply`, running `terraform output app_domain` will echo the configured domain — useful in CI scripts that need to pass the URL downstream for smoke tests or notifications.

---

## Applying the configuration

Before running `terraform apply`, add the new variable to `terraform.tfvars`:

```hcl
domain_name = "library.example.com"
```

Then apply:

```
cd terraform
terraform plan
terraform apply
```

The plan will show one new resource (`aws_route53_record.app`) and two new data sources. The apply typically completes in under ten seconds — Route 53 record creation is fast.

---

## Verification

DNS propagation from Route 53 is usually near-instantaneous because Route 53 serves as the authoritative name server and the TTL for newly created records defaults to 300 seconds. However, your local resolver may cache previous responses (including NXDOMAIN, "no such record") for up to the negative TTL of the zone's SOA record.

Use `dig` to query Route 53 directly, bypassing any local cache:

```
dig library.example.com @8.8.8.8
```

Expected output (abbreviated):

```
;; ANSWER SECTION:
library.example.com.    60    IN    A    52.4.25.11
library.example.com.    60    IN    A    54.165.42.87
```

The IP addresses returned will be the current IP addresses of the ALB. They will change over time as the ALB scales, which is precisely why you used an alias record rather than a hardcoded A record.

Alternatively, use `nslookup`:

```
nslookup library.example.com 8.8.8.8
```

```
Server:   8.8.8.8
Address:  8.8.8.8#53

Non-authoritative answer:
Name:     library.example.com
Address:  52.4.25.11
Address:  54.165.42.87
```

Once DNS resolves correctly, try an HTTP request to confirm the ALB routes traffic:

```
curl -v http://library.example.com/healthz
```

At this point, the connection will succeed over plain HTTP. Section 14.2 will add ACM and HTTPS termination at the ALB, after which port 80 will redirect to port 443.

---

## If you don't own a domain

If you do not own a domain, you have two options.

First, you can follow this section conceptually without applying the Terraform. The code is straightforward and does not depend on anything you cannot reason about without a real domain. Skip the verification steps and revisit when you have a domain.

Second, services like **FreeDNS** (freedns.afraid.org) offer free subdomains under community-owned domains. You can register `library.yourname.mooo.com` and point it at your ALB. The catch is that FreeDNS is not Route 53, so you would use their web interface to create the CNAME record rather than the Terraform code in this section. ACM validation — covered in the next section — requires that you have DNS control, which FreeDNS does provide (you can add TXT records for validation), but the process is manual rather than automated through Terraform.

The cleanest path is to register a domain. `.com` domains cost around $12/year through Route 53 itself, and the registration can be done entirely in the AWS console or via the `aws_route53domains_registered_domain` Terraform resource.[^4] The domain pays for itself in the time you save debugging certificate validation issues.

---

[^1]: AWS Route 53 alias records: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html
[^2]: Configuring Route 53 as your DNS service: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/migrate-dns-domain-in-use.html
[^3]: AWS Load Balancer Controller — how it tags ALBs: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/ingress/annotations/
[^4]: Registering a domain with Route 53: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/domain-register.html
