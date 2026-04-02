# Chapter 13: Production Hardening

Chapter 12 deployed the library system to real infrastructure: an EKS cluster running five services, three RDS instances holding persistent state, an MSK cluster brokering Kafka events, and a GitHub Actions pipeline shipping code on every push to `main`. That is a genuine production deployment — the application is reachable, the data survives pod restarts, and nobody has to run `kubectl apply` from a laptop. But "reachable" and "production-ready" are not the same thing. Three gaps remained when Chapter 12 ended, each one deferred in the name of getting the system running before addressing hardening. This chapter closes all three. None of them require changes to application code, Dockerfiles, or Earthfiles — everything that needs to change lives in Terraform and the production Kustomize overlay.

---

## The three gaps

| Gap | Chapter 12 state | Chapter 13 target |
|-----|-----------------|-------------------|
| DNS + TLS | HTTP via ALB hostname, no custom domain | HTTPS with a custom domain via Route 53 + ACM |
| Secrets | Placeholder values in the production overlay's `secretGenerator` | External Secrets Operator syncing live values from AWS Secrets Manager |
| Kafka encryption | Plaintext connections on port 9092 | TLS connections on port 9094 |

Each row is a separate concern with its own tools and its own section in this chapter. They are also independent of each other — you can apply them in any order, or apply just one if that is what your situation calls for. The sections below treat them sequentially because that matches the natural dependency order when you are also setting up DNS for the first time.

---

## Why these gaps matter

It is tempting to think of these as polish — things you would fix before a real public launch but can ignore while learning. That framing undersells the risk. Each gap is an audit failure in any environment subject to compliance frameworks, and the risks are concrete even if you are running a personal project.

Serving HTTP without TLS means every request between the browser and your ALB travels in plaintext. That includes session tokens, API responses, and any user data in query strings or response bodies. The ALB terminates TLS at the edge in Chapter 13's target state — traffic inside the VPC between the ALB and the pods can remain HTTP, which is a common and acceptable pattern — but the public-facing leg must be encrypted. Modern browsers actively warn users about non-HTTPS sites; more practically, OAuth2 providers including Google will refuse to complete a login flow if the redirect URI is HTTP.

Pasting database passwords manually into a Kustomize `secretGenerator` creates several problems at once. The secrets are almost certainly committed to git at some point — either accidentally or because someone thinks "I'll fix it later." Even if you avoid git, the values exist in someone's terminal history, in CI logs if you print them for debugging, and in every K8s Secret object that was ever created with the wrong value and then deleted. Proper secrets management means the application retrieves credentials from a single authoritative source — AWS Secrets Manager in this case — and the Kubernetes Secret is populated automatically and rotated without human intervention.

Kafka's plaintext listener (port 9092) transmits all broker traffic unencrypted inside the VPC. VPCs are not the internet, and an attacker who has not already breached your network boundary cannot read that traffic — but that is a weaker guarantee than it sounds. Lateral movement from a compromised pod, misconfigured security groups, or VPC peering arrangements can all expose plaintext traffic to unintended readers. MSK supports TLS-only listener configuration; enabling it costs nothing and closes the exposure entirely.

---

## What stays the same

The Kustomize layering from Chapter 11 and the CI/CD pipeline from Chapter 12 are not touched. The base manifests under `k8s/base/` remain unchanged — a deliberate constraint that demonstrates the value of the overlay pattern. Application services do not need to know anything about where TLS terminates, how secrets reach the pod's environment, or what port the Kafka broker is listening on. They connect to hostnames and read environment variables; the infrastructure layer handles everything else.

The Earthfile CI targets — `+lint`, `+test`, `+build`, `+integration-test` — run exactly as they did in Chapter 12. The GitHub Actions pipeline that calls them is unchanged. The only files that change in this chapter are:

- Terraform modules under `infra/` — for the Route 53 hosted zone, ACM certificate, and MSK listener configuration
- The production Kustomize overlay under `k8s/overlays/production/` — for the External Secrets Operator configuration and the updated Kafka broker address
- A new Terraform module for the External Secrets Operator IAM role and its associated Kubernetes resources

If you have been following the "what changes and what stays the same" framing from earlier chapters, the pattern holds here too.

---

## Cost impact

All three changes are either free or negligible.

| Addition | Monthly cost |
|----------|-------------|
| Route 53 hosted zone | $0.50 |
| ACM certificate | Free |
| MSK TLS listener | No additional cost |
| External Secrets Operator | No additional cost (open-source operator running on existing nodes) |

The only new line item is the Route 53 hosted zone, which is billed at fifty cents per month regardless of query volume up to one billion queries. ACM certificates for domains managed in Route 53 are issued and renewed automatically at no charge. MSK supports TLS as a configuration flag on the existing brokers — there is no separate TLS broker tier. External Secrets Operator runs as a Deployment in your EKS cluster, consuming a small amount of CPU and memory on nodes you are already paying for.

If you already own a domain and it is managed elsewhere — GoDaddy, Namecheap, Cloudflare — you have two options: transfer the domain to Route 53 (a one-time process that preserves your existing records) or keep the domain where it is and create an NS delegation for a subdomain. The sections below assume you are creating a new hosted zone; the delegation path is noted where it matters.

---

## Target architecture

The diagram below shows the system after all three changes are applied. Compare it to the Chapter 12 diagram: the public-facing edge now terminates HTTPS, the pod environment variables arrive via External Secrets, and the Kafka connections use the TLS listener.

```mermaid
graph TD
    Internet([Internet]) --> R53[Route 53\nDNS]
    R53 --> ALB[AWS ALB\nHTTPS :443\nACM Certificate]

    subgraph VPC [AWS VPC]
        ALB -->|HTTP :80 internal| GW[gateway]

        subgraph EKS [Amazon EKS Cluster]
            subgraph library [Namespace: library]
                GW --> AUTH[auth]
                GW --> CAT[catalog]
                GW --> RES[reservation]
                GW --> SRC[search]
            end

            subgraph ops [Namespace: external-secrets]
                ESO[External Secrets\nOperator]
            end
        end

        ESO -->|sync| ASM[AWS Secrets Manager]
        ASM -.->|K8s Secrets| AUTH
        ASM -.->|K8s Secrets| CAT
        ASM -.->|K8s Secrets| RES

        AUTH --> RDS_AUTH[(RDS: auth-db)]
        CAT --> RDS_CAT[(RDS: catalog-db)]
        RES --> RDS_RES[(RDS: reservation-db)]

        CAT -->|TLS :9094| MSK[Amazon MSK\nKafka TLS]
        RES -->|TLS :9094| MSK
        SRC -->|TLS :9094| MSK

        SRC --> MEI[(Meilisearch\nStatefulSet)]
    end

    ECR[Amazon ECR] -.->|pull images| EKS
    GHA[GitHub Actions\nOIDC] -.->|kubectl apply| EKS
```

The changes touch three edges in this diagram: the public entry point gains TLS termination at the ALB, the secret values gain a managed sync path through External Secrets Operator, and the Kafka edges switch from port 9092 to port 9094. Everything else — the service topology, the RDS connections, the Meilisearch StatefulSet, the ECR image pulls, the OIDC-authenticated deployments — is carried forward from Chapter 12 without modification.

---

## Chapter roadmap

**13.2 — DNS with Route 53** creates a hosted zone for your domain and adds the A-record alias that points your domain's apex (or a subdomain) at the ALB. You will use Terraform's `aws_route53_zone` and `aws_route53_record` resources. By the end of this section, the application is reachable at a human-readable URL — still over HTTP, but at the right address.

**13.3 — TLS with ACM** provisions an ACM certificate for your domain and attaches it to the ALB. ACM handles certificate issuance via DNS validation — it writes a CNAME record to your hosted zone and polls for it, which Terraform orchestrates in a single `apply`. You will update the Ingress annotations in the production overlay to reference the certificate ARN and redirect HTTP to HTTPS. After this section, the application is reachable at `https://yourdomain.com`.

**13.4 — Secrets Management with External Secrets Operator** installs the External Secrets Operator into the cluster via Helm, creates the IAM role and policy that allow it to read from Secrets Manager, and writes the `ExternalSecret` and `SecretStore` resources that define which secrets to sync and how often. You will also write a small Terraform module that creates the Secrets Manager entries for each service's database credentials, replacing the placeholder values in the `secretGenerator`.

**13.5 — Kafka Encryption (MSK TLS)** enables the TLS listener on the MSK cluster and disables the plaintext listener, updates the security group rules to allow port 9094 instead of 9092, and patches the `KAFKA_BROKERS` environment variable in the production overlay to use the TLS bootstrap server addresses. The application-level Kafka client configuration requires no changes — the Go Kafka library picks up the TLS requirement from the broker address scheme.

**13.6 — Applying the Changes** walks through the full `terraform apply` and `kubectl apply` sequence, verifies each gap is closed, and confirms the integration tests from Chapter 10 still pass against the hardened cluster. You will also review the AWS Security Hub findings — if you enabled it in Chapter 12 — to confirm that the three controls that were previously failing now show as passed.

---

By the end of this chapter, the library system will pass a basic security review: encrypted traffic on every public-facing edge, secrets sourced from a managed store rather than committed configuration, and encrypted broker connections inside the cluster. These are not exotic hardening measures — they are the baseline that any production system deployed to a regulated environment is expected to meet, and they are the baseline that a careful engineer expects even in environments that are not formally regulated. Getting comfortable applying them in a learning project means they will not be unfamiliar when the stakes are higher.

Before moving to section 13.2, confirm that your Chapter 12 cluster is still running and healthy: `kubectl get pods -n library` should show all pods in the `Running` state. If you have run `terraform destroy` since Chapter 12, re-apply the Chapter 12 Terraform before continuing — the changes in this chapter build on top of the existing infrastructure rather than replacing it.

---

[^1]: AWS Route 53 Documentation: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/Welcome.html
[^2]: AWS Certificate Manager Documentation: https://docs.aws.amazon.com/acm/latest/userguide/acm-overview.html
[^3]: External Secrets Operator Documentation: https://external-secrets.io/latest/
[^4]: AWS Secrets Manager Documentation: https://docs.aws.amazon.com/secretsmanager/latest/userguide/intro.html
[^5]: Amazon MSK TLS Encryption: https://docs.aws.amazon.com/msk/latest/developerguide/msk-encryption.html
[^6]: AWS Load Balancer Controller — TLS: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/ingress/annotations/#ssl
[^7]: Route 53 Pricing: https://aws.amazon.com/route53/pricing/
