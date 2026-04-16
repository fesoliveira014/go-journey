# 14.2 TLS with ACM

<!-- [STRUCTURAL] Heading hierarchy change: index.md uses "14.1 — DNS with Route 53" (em dash) and "14.4 Kafka Encryption: MSK TLS" / "14.5 Applying the Changes" (no em dash). This section uses "14.2 TLS with ACM" (no em dash). Unify chapter-wide. Recommend "14.2 — TLS with ACM" for parity with 14.1 or drop em dash from 14.1 to match 14.2/14.4/14.5. -->
<!-- [COPY EDIT] Heading case here is Title Case ("TLS with ACM"); dns.md uses sentence case below heading ("How DNS works — a brief refresher"). tls.md uses Title Case in sub-headings ("How ACM Works", "TLS Termination at the ALB"). Inconsistent across sections. Pick one. Chicago allows either (CMOS 8.158) but requires consistency. -->

<!-- [STRUCTURAL] Opening paragraph names the concrete problems (browser warnings, OAuth2, session tokens). Good, but duplicates threat framing from index.md. Consider trimming since the reader has just read those arguments. -->
<!-- [LINE EDIT] "None of these are theoretical concerns — they are the default behavior of the tooling your users are already running." → keep; strong closer. -->
TLS is table stakes for any production web application. Browsers label plain HTTP connections as "Not Secure" in the address bar, OAuth2 providers refuse to complete login flows when the redirect URI is HTTP, and any session token or API response that crosses an unencrypted connection is readable by anyone on the same network path. None of these are theoretical concerns — they are the default behavior of the tooling your users are already running.

<!-- [LINE EDIT] "The good news is that there is no longer a cost reason to skip TLS." → "There is no longer a cost reason to skip TLS." (drop "The good news is that" — filler). -->
<!-- [COPY EDIT] "ALB, CloudFront, API Gateway" — needs serial comma; these are AWS service names (CloudFront, API Gateway are correct caps). → "ALB, CloudFront, or API Gateway." -->
<!-- [COPY EDIT] "roughly 30 days before" — numeral + "days" is correct per CMOS 9.7. Good. -->
<!-- [COPY EDIT] Please verify: ACM renewal timing — docs state ACM begins renewal attempts ~60 days before expiration for publicly-trusted certs, not 30 days. -->
The good news is that there is no longer a cost reason to skip TLS. AWS Certificate Manager issues and automatically renews certificates for free when they are used with AWS services — ALB, CloudFront, API Gateway. The certificate you provision in this section will never expire without intervention, because ACM handles renewal roughly 30 days before the current certificate's expiration. You do nothing. The certificate rotates.

<!-- [LINE EDIT] "The application pods will see plain HTTP, which is entirely correct — and the next section explains why." → "and the next section explains why" — but "the next section" is the TLS-Termination section immediately below, not a future chapter section. Verify referent. -->
By the end of this section, `https://library.example.com` will return a valid certificate issued by Amazon, and any request to `http://library.example.com` will receive a 301 redirect to the HTTPS URL. The application pods will see plain HTTP, which is entirely correct — and the next section explains why.

---

## How ACM Works

<!-- [COPY EDIT] "How ACM Works" — Title Case. Compare "How DNS works — a brief refresher" (dns.md) in sentence case. Pick one chapter-wide. -->
ACM is a managed certificate authority that integrates natively with AWS load balancers. The workflow has three steps:

<!-- [LINE EDIT] "it first needs to confirm that you control the domain" — precise. Keep. -->
1. **Request** — you request a certificate for a domain name (and optionally wildcard subdomains). ACM does not issue the certificate immediately; it first needs to confirm that you control the domain.

<!-- [COPY EDIT] "two to five minutes" — CMOS 9.3: spell out cardinal numbers zero–one hundred. Good. -->
<!-- [COPY EDIT] "DNS validation" — consistent capitalization. OK. -->
2. **Validate** — ACM offers two validation methods: DNS validation and email validation. DNS validation is the preferred method. ACM generates a CNAME record that you must add to your domain's DNS configuration. Once ACM detects that the record is present, validation completes and the certificate is issued. The whole process typically takes two to five minutes when the DNS is managed in Route 53.

<!-- [COPY EDIT] "every 13 months[^1]" — "13 months" numeral + units OK. Footnote placement: per CMOS 14.26 superscript comes after punctuation if at end of sentence. Here it's mid-sentence (before the period); standard for footnote markers, OK. -->
<!-- [COPY EDIT] Please verify: "every 13 months" — AWS docs state ACM attempts renewal "well before" expiration; managed renewal begins around 60 days prior. "Every 13 months" conflates certificate validity (13 months) with renewal cadence. Clarify: certificate validity is 13 months; ACM renews ~60 days before expiry. -->
3. **Renew** — ACM checks the validation CNAME periodically. As long as the record is still present in Route 53 — and Terraform keeps it there — ACM renews the certificate automatically every 13 months[^1]. You receive no notifications, no renewal prompts, and no expiry pages.

<!-- [LINE EDIT] "The DNS validation method has an important advantage over email validation: it is fully automatable." → keep; good sentence. -->
The DNS validation method has an important advantage over email validation: it is fully automatable. Terraform can create the certificate request, write the validation CNAME to Route 53, and wait for ACM to report the certificate as `ISSUED` in a single `apply` run. Email validation requires a human to click a link, which breaks any infrastructure-as-code workflow.

<!-- [LINE EDIT] "cert-manager with Let's Encrypt is the standard path, and it is covered briefly at the end of this section." → keep; signals the later subsection. -->
<!-- [COPY EDIT] "Let's Encrypt" — proper noun with apostrophe. Correct. -->
Certificates issued by ACM are only usable within AWS. You cannot export the private key and install the certificate on an on-premises server or a non-AWS load balancer. If you ever move the application off AWS, you will need a replacement — cert-manager with Let's Encrypt is the standard path, and it is covered briefly at the end of this section.

---

## TLS Termination at the ALB

The ALB handles the TLS handshake with the client. Once the handshake completes, the ALB decrypts the request and forwards it over HTTP to the target pod. The pod never sees TLS.

```
Client → HTTPS (port 443) → ALB (TLS termination) → HTTP (port 80) → Pod
```

<!-- [COPY EDIT] "public internet" — lowercase "internet" (CMOS 8.190). Good. -->
<!-- [LINE EDIT] "This is called TLS termination at the edge, and it is the standard pattern for load-balanced applications." → keep; labels the term for the reader. -->
<!-- [COPY EDIT] "inbound HTTP leg" — precise term, good. -->
This is called TLS termination at the edge, and it is the standard pattern for load-balanced applications. The inbound HTTP leg between the ALB and the pod is private — it runs inside your VPC, across AWS's internal network fabric, and never touches the public internet. The security group on the worker nodes allows inbound port 80 only from the ALB security group, so no external traffic can reach the pod directly.

<!-- [LINE EDIT] "The complexity is real and the security benefit for intra-VPC traffic is marginal — you are protecting against an attacker who has already breached your VPC boundary, at which point you have larger problems." → 37 words; good rhythm. Keep. -->
<!-- [COPY EDIT] "end-to-end TLS" — hyphenated compound before noun (CMOS 7.81). Good. -->
<!-- [COPY EDIT] "cert-manager" — lowercase consistent with project name. Good. -->
<!-- [COPY EDIT] "intra-VPC" — hyphenated; consistent. Good. -->
The alternative is end-to-end TLS, where the ALB re-encrypts traffic before forwarding it to the pod. This requires the pod to terminate TLS too, which means deploying cert-manager into the cluster, rotating certificates inside Kubernetes, and configuring the application server (or a sidecar) to handle TLS. The complexity is real and the security benefit for intra-VPC traffic is marginal — you are protecting against an attacker who has already breached your VPC boundary, at which point you have larger problems. Edge termination is the right choice here[^2].

<!-- [COPY EDIT] "PCI DSS and some HIPAA interpretations fall into this category." — PCI DSS 4.0 does indeed require encryption in transit for CHD on public networks; "internal hops" coverage is interpretive. QUERY: verify phrasing of "PCI DSS mandates encryption for all internal hops" — this is a common misconception; PCI DSS requires strong cryptography for transmission over open/public networks. -->
<!-- [COPY EDIT] Please verify: PCI DSS does not mandate TLS on all internal hops (requirement 4 targets open/public networks). HIPAA is policy-driven rather than prescriptive. Soften "explicitly mandate" to "may require". -->
There is one situation where end-to-end TLS is worth considering: when compliance requirements explicitly mandate encryption for all hops, including internal ones. PCI DSS and some HIPAA interpretations fall into this category. For the library system, and for most applications that are not subject to those frameworks, edge termination at the ALB is both correct and sufficient.

---

## Terraform Configuration

Create a new file `terraform/acm.tf`. It contains three resources: the certificate request, the DNS validation records, and a validation waiter.

```hcl
# terraform/acm.tf

resource "aws_acm_certificate" "app" {
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.project_name}-cert" }
}

# Create the DNS validation records in Route 53
resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.app.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  ttl             = 60
  type            = each.value.type
  zone_id         = data.aws_route53_zone.main.zone_id
}

# Wait for the certificate to be validated
resource "aws_acm_certificate_validation" "app" {
  certificate_arn         = aws_acm_certificate.app.arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}
```

Walk through each resource:

<!-- [COPY EDIT] "Subject Alternative Name" — capitalized as a term of art. OK; but consider adding "(SAN)" parenthetical on first use. -->
<!-- [LINE EDIT] "Without it, the ALB would briefly have no valid certificate during the update" → "Without it, the ALB would briefly have no valid certificate" (drop "during the update"; tautology). -->
**`aws_acm_certificate.app`** submits the certificate request to ACM for `var.domain_name`. Setting `validation_method = "DNS"` tells ACM to generate a CNAME record for ownership verification rather than sending a validation email. The `lifecycle` block with `create_before_destroy = true` is important: when you update a certificate — for example, to add a Subject Alternative Name — Terraform provisions the new certificate before destroying the old one. Without it, the ALB would briefly have no valid certificate during the update[^3].

<!-- [LINE EDIT] "The expression keys the map by domain name so that adding domains later does not reorder or replace existing records." → keep; precise. -->
<!-- [COPY EDIT] "idempotent" — technical term, used correctly. Good. -->
**`aws_route53_record.cert_validation`** uses a `for_each` expression to iterate over `domain_validation_options` — the set of CNAME records ACM requires. For a single-domain certificate there is one record; for a certificate covering multiple domains or wildcards, there would be more. The expression keys the map by domain name so that adding domains later does not reorder or replace existing records. `allow_overwrite = true` makes the resource idempotent: if a previous `apply` already wrote the record and the state was lost, Terraform updates rather than errors.

<!-- [STRUCTURAL] This paragraph is information-dense and important — it explains the subtle "validation waiter" concept. Consider a small code-comment callout or highlight to draw the eye. -->
<!-- [LINE EDIT] "is not a real AWS resource — it is a Terraform construct that polls the ACM API" → "is a Terraform construct, not a real AWS resource. It polls the ACM API" (restructures for emphasis). -->
**`aws_acm_certificate_validation.app`** is not a real AWS resource — it is a Terraform construct that polls the ACM API and blocks until the certificate status changes from `PENDING_VALIDATION` to `ISSUED`. The `depends_on` is implicit here: because `validation_record_fqdns` references `aws_route53_record.cert_validation`, Terraform knows to create the DNS records before waiting for validation. The ALB listener attachment depends on `aws_acm_certificate_validation.app`, which forces the full validation chain to complete before the certificate is attached to traffic.

---

## Updating the ALB Annotations

<!-- [COPY EDIT] "ALB Annotations" heading Title Case; consistent with other section headings in this file. Keep. -->
The production Kustomize overlay contains an Ingress patch for the ALB. Section 14.1 added the Route 53 alias annotation. Now add three more annotations to the same patch:

<!-- [COPY EDIT] Comment "Ingress patch in deploy/k8s/overlays/production/kustomization.yaml" — file path. OK. -->
```yaml
# Ingress patch in deploy/k8s/overlays/production/kustomization.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: library-ingress
  namespace: library
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: ACM_CERTIFICATE_ARN
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}, {"HTTPS": 443}]'
    alb.ingress.kubernetes.io/ssl-redirect: "443"
```

<!-- [COPY EDIT] "Replace `ACM_CERTIFICATE_ARN` with the output from `terraform output certificate_arn` after applying `acm.tf`." — precise. Good. -->
Replace `ACM_CERTIFICATE_ARN` with the output from `terraform output certificate_arn` after applying `acm.tf`. The three new annotations work as follows:

<!-- [COPY EDIT] "`arn:aws:acm:us-east-1:123456789012:certificate/...`[^4]" — ARN format OK. Footnote placement OK. -->
**`certificate-arn`** attaches the ACM certificate to the ALB. The AWS Load Balancer Controller reads this annotation and calls the ELB API to associate the certificate with the HTTPS listener. Without this annotation, the ALB has no certificate and cannot terminate TLS. The value is the full ARN in the form `arn:aws:acm:us-east-1:123456789012:certificate/...`[^4].

<!-- [LINE EDIT] "Both must be present for the redirect to work; the redirect is applied to the HTTP listener, and it needs somewhere to redirect to." → tighter: "Both listeners must exist for the redirect to work: the redirect is applied to the HTTP listener and needs a target." -->
**`listen-ports`** declares which ports the ALB should open. The default is `[{"HTTP": 80}]` — a single HTTP listener. Overriding it to include both port 80 and port 443 causes the controller to create two listeners on the ALB: one for plain HTTP and one for HTTPS. Both must be present for the redirect to work; the redirect is applied to the HTTP listener, and it needs somewhere to redirect to.

<!-- [COPY EDIT] "`ssl-redirect: \"443\"`" — consistent formatting of annotation value. Good. -->
<!-- [LINE EDIT] "Any request arriving at `http://library.example.com/books` is redirected to `https://library.example.com/books`." → strong concrete example. Keep. -->
**`ssl-redirect: "443"`** configures the HTTP listener to return a 301 redirect to the same URL on port 443. Any request arriving at `http://library.example.com/books` is redirected to `https://library.example.com/books`. The redirect preserves the full path and query string. This is a property of the ALB listener rule, not an application-level redirect — the pod never sees the HTTP request at all.

<!-- [COPY EDIT] "The AWS Load Balancer Controller parses it as an integer internally; the quotes are required for valid YAML." — correct and helpful. Keep. -->
The value `"443"` is a string, not an integer, because Kubernetes annotation values are always strings. The AWS Load Balancer Controller parses it as an integer internally; the quotes are required for valid YAML.

---

## Applying the Changes

<!-- [COPY EDIT] "Applying the Changes" — same heading appears as 14.5 title. Consider renaming here to "Running the apply" or "Deploy" to avoid duplicate scent in TOC. -->
Run `terraform apply` from the `terraform/` directory. Terraform will create three resources in order: the certificate request, the validation DNS records, and the validation waiter. The waiter typically completes within two to five minutes:

```
aws_acm_certificate.app: Creating...
aws_acm_certificate.app: Creation complete after 3s
aws_route53_record.cert_validation["library.example.com"]: Creating...
aws_route53_record.cert_validation["library.example.com"]: Creation complete after 8s
aws_acm_certificate_validation.app: Creating...
aws_acm_certificate_validation.app: Still creating... [1m0s elapsed]
aws_acm_certificate_validation.app: Still creating... [2m0s elapsed]
aws_acm_certificate_validation.app: Creation complete after 2m14s
```

After the apply completes, retrieve the certificate ARN:

```bash
terraform output certificate_arn
```

<!-- [COPY EDIT] "`ingress-patch.yaml`" — earlier the patch was described as being inside `kustomization.yaml`. File name inconsistency. QUERY: verify whether the patch lives in its own file or inline in kustomization.yaml, and unify. -->
<!-- [COPY EDIT] Please verify: Ingress patch file structure. Section intro says "Ingress patch in deploy/k8s/overlays/production/kustomization.yaml" but this instruction says "Paste the ARN value into `ingress-patch.yaml`". Pick one. -->
Paste the ARN value into `ingress-patch.yaml`, replacing `ACM_CERTIFICATE_ARN`. Then apply the Kustomize overlay:

```bash
kubectl apply -k deploy/k8s/overlays/production/
```

<!-- [LINE EDIT] "The AWS Load Balancer Controller will update the ALB within about 30 seconds. The existing ALB is modified in place — a new HTTPS listener is added and the HTTP listener rule is updated to redirect. No downtime occurs." → keep; good reassurance. -->
The AWS Load Balancer Controller will update the ALB within about 30 seconds. The existing ALB is modified in place — a new HTTPS listener is added and the HTTP listener rule is updated to redirect. No downtime occurs.

---

## Verification

Confirm that HTTPS is working and that the certificate is valid:

```bash
curl -I https://library.example.com
```

Expected output:

```
HTTP/2 200
content-type: application/json
...
```

<!-- [COPY EDIT] "`HTTP/2` prefix" — more precisely, HTTP/2 is the status line. Minor. -->
<!-- [COPY EDIT] "HTTP/2 (which requires TLS — plain HTTP/1.1 connections cannot be upgraded to HTTP/2 at this layer)" — technically HTTP/2 does not require TLS per spec (h2c exists), but browsers only negotiate HTTP/2 over TLS, and ALB requires TLS for HTTP/2. Phrasing is OK for audience but worth a QUERY. -->
<!-- [COPY EDIT] Please verify: statement that HTTP/2 requires TLS — spec permits cleartext h2c; however browsers and AWS ALB only support HTTP/2 over TLS. Clarify phrasing to avoid spec-level inaccuracy. -->
The `HTTP/2` prefix confirms the ALB accepted the TLS connection and negotiated HTTP/2 (which requires TLS — plain HTTP/1.1 connections cannot be upgraded to HTTP/2 at this layer). A `200` status on the health endpoint confirms the traffic reached a pod.

Verify that the HTTP-to-HTTPS redirect is working:

```bash
curl -I http://library.example.com
```

Expected output:

```
HTTP/1.1 301 Moved Permanently
Location: https://library.example.com/
```

The `301` comes from the ALB listener rule before any traffic reaches a pod.

For certificate chain inspection, use `openssl s_client`:

```bash
openssl s_client -connect library.example.com:443 -servername library.example.com
```

<!-- [LINE EDIT] "The output includes the full certificate chain. Look for `Verify return code: 0 (ok)` at the end, which confirms that the certificate chains to a trusted root in OpenSSL's certificate store." → keep; reads well. -->
<!-- [COPY EDIT] "Amazon's intermediate CA" — CA on first use in this file — expand once to "Certificate Authority (CA)". -->
The output includes the full certificate chain. Look for `Verify return code: 0 (ok)` at the end, which confirms that the certificate chains to a trusted root in OpenSSL's certificate store. You will see two certificates in the chain: the domain certificate issued by ACM and Amazon's intermediate CA.

---

## Outputs

Add a certificate ARN output to `terraform/outputs.tf`:

```hcl
output "certificate_arn" {
  description = "ACM certificate ARN"
  value       = aws_acm_certificate.app.arn
}
```

<!-- [COPY EDIT] "Chapter 14.5" — compare "Section 14.5" used elsewhere. Unify chapter-wide. "Section 14.5" is clearer. -->
This output is needed in two places: manually, to paste into the Ingress patch file as shown above, and potentially by a CI/CD step that generates the overlay automatically via `terraform output`. Chapter 14.5 revisits this when running the full end-to-end apply sequence.

---

## cert-manager as an Alternative

<!-- [STRUCTURAL] Alternative section is appropriate — signals portability. Length is right. Keep. -->
<!-- [COPY EDIT] "cert-manager" — project canonical name; keep lowercase. Good. -->
<!-- [COPY EDIT] "GKE, AKS, a bare-metal cluster" — serial comma needed before "a bare-metal cluster, or any environment where ACM is not available". Current punctuation OK with the trailing "or". -->
If you move this application off AWS — to GKE, AKS, a bare-metal cluster, or any environment where ACM is not available — cert-manager is the standard replacement. cert-manager is an open-source Kubernetes controller that integrates with ACME providers (Let's Encrypt being the most common) to issue and renew certificates automatically.

<!-- [COPY EDIT] "`Certificate` resource" — backticks for K8s CRD name. Good. -->
<!-- [COPY EDIT] "`Issuer` (or `ClusterIssuer`)" — backticks consistent. Good. -->
The workflow is similar: you create a `Certificate` resource in Kubernetes that references an `Issuer` (or `ClusterIssuer`) pointing at Let's Encrypt, and cert-manager handles the ACME challenge, certificate issuance, and renewal. The certificate ends up in a Kubernetes Secret that the Ingress references by name.

<!-- [LINE EDIT] "The reason to prefer ACM on AWS is integration depth." → keep; strong topic sentence. -->
<!-- [LINE EDIT] "When you are already using Route 53 and an ALB, ACM is the lower-complexity path." → keep. -->
The reason to prefer ACM on AWS is integration depth. ACM certificates are stored in AWS, renewed by AWS, and attached to ALBs directly — there is no Kubernetes Secret containing a private key, no cert-manager Deployment consuming cluster resources, and no renewal webhook to maintain. When you are already using Route 53 and an ALB, ACM is the lower-complexity path. When you are not on AWS, cert-manager is the right answer and the process is well-documented.

---

[^1]: AWS Certificate Manager renewal behavior: https://docs.aws.amazon.com/acm/latest/userguide/managed-renewal.html
[^2]: AWS documentation on ALB TLS termination: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-listeners.html#listener-rules
[^3]: Terraform `aws_acm_certificate` resource reference: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/acm_certificate
[^4]: AWS Load Balancer Controller annotation reference — TLS: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/ingress/annotations/#ssl
