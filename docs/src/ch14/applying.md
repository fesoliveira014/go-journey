# 14.5 Applying the Changes

Sections 13.1 through 13.4 introduced four independent concerns — Route 53 DNS, ACM certificate provisioning, External Secrets Operator, and MSK TLS. Each was presented in isolation so the concepts could be unpacked without the surrounding noise. Now you apply all four together.

This section is deliberately linear. You will run exactly these commands, in exactly this order, and verify exactly these outputs. If something goes wrong, the rollback notes at each step tell you how to recover without leaving the cluster in a broken state.

---

## Dependency Order

The four changes have dependencies on each other that are not obvious until you try to apply them out of order.

- **ACM needs Route 53** — the certificate validation method is DNS. ACM writes a CNAME into your hosted zone and polls for it. If the hosted zone does not exist yet, Terraform cannot write the validation record, and `terraform apply` will time out waiting for a certificate that can never be issued.

- **The ALB needs the ACM certificate ARN** — the Ingress annotation `alb.ingress.kubernetes.io/certificate-arn` references the ARN that ACM assigns at issuance time. Terraform produces the ARN as an output, which the Kustomize overlay references. Kubernetes apply must come after Terraform apply.

- **ESO needs the cluster running** — the External Secrets Operator is installed as a Helm release via the Terraform Kubernetes and Helm providers, but the `ExternalSecret` and `SecretStore` Kubernetes resources are applied by `kubectl apply -k`. Both steps require a running EKS cluster. ESO's IAM role also depends on the cluster's OIDC issuer URL, which Terraform creates.

- **MSK TLS requires a rolling broker restart** — enabling the TLS listener on an MSK cluster triggers a rolling configuration update. Each broker restarts one at a time, which takes roughly 10–15 minutes. The cluster remains available during the rolling restart (MSK is designed for this), but if you update the application's `KAFKA_BROKERS` environment variable to point at port 9094 before the TLS listener is live, services will fail to connect. Apply Terraform first; apply the Kustomize patch after Terraform confirms completion.

The correct order is therefore: Terraform first (DNS, ACM, ESO IAM, MSK listener, security group rules), then secrets creation in Secrets Manager, then `kubectl apply` (ESO Kubernetes resources, Ingress TLS annotations, MSK ConfigMap patch).

---

## Step 1: Add the Domain Variable

Open `terraform/terraform.tfvars` and add the domain name variable introduced in section 14.1:

```
domain_name = "library.example.com"
```

Replace `library.example.com` with the domain or subdomain you are using. If you are delegating a subdomain from an external registrar rather than using Route 53 as the registrar, this is still the correct value — the hosted zone is created for whatever name you provide here.

---

## Step 2: Apply Terraform

Run the plan from the `terraform/` directory:

```bash
cd terraform
terraform plan -out=tfplan
```

The plan output will include a sizeable diff. The new resources are:

- `data.aws_route53_zone` — reads your existing hosted zone (you must have already created the zone and delegated DNS to it)
- `aws_route53_record` (alias) — the A-record alias pointing your domain at the ALB
- `aws_acm_certificate` — the certificate request
- `aws_route53_record` (validation) — the CNAME that ACM uses to prove domain control
- `aws_acm_certificate_validation` — a Terraform resource that blocks until ACM reports `ISSUED`
- `aws_iam_role` and `aws_iam_policy` — the IRSA role that External Secrets Operator uses to call Secrets Manager
- `helm_release` (external-secrets) — the ESO Helm chart in the `external-secrets` namespace
- `aws_msk_configuration` — a new MSK configuration with `TLS_PLAINTEXT` replaced by `TLS`
- `aws_msk_cluster` (update) — the cluster update that applies the new configuration
- `aws_security_group_rule` — inbound allow on port 9094 from the EKS node security group

Review the plan output. If you see resources listed as destroyed unexpectedly — particularly the MSK cluster itself rather than a configuration change — stop and check that the MSK resource block in `terraform/msk.tf` is using `aws_msk_configuration` rather than an inline `configuration_info` block that Terraform would treat as requiring replacement.

When satisfied, apply:

```bash
terraform apply tfplan
```

This takes roughly 10–15 minutes. The ACM validation step waits for Route 53 propagation (typically 2–5 minutes), and the MSK configuration change triggers a rolling broker restart (typically 8–12 minutes). The terminal streams each resource completion as it happens. Expected final lines:

```
Apply complete! Resources: 10 added, 2 changed, 0 destroyed.

Outputs:

acm_certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
eso_role_arn            = "arn:aws:iam::123456789012:role/library-eso-role"
msk_tls_bootstrap       = "b-1.library.xxxxx.kafka.us-east-1.amazonaws.com:9094,b-2.library.xxxxx.kafka.us-east-1.amazonaws.com:9094"
route53_zone_id         = "Z1XXXXXXXXXXXXXXXXX"
```

Note the `:9094` port in `msk_tls_bootstrap`. Save these outputs — you will use the certificate ARN and TLS bootstrap string when verifying the overlay in later steps. You can always retrieve them with `terraform output`.

---

## Step 3: Create Non-RDS Secrets in Secrets Manager

Terraform created the Secrets Manager entries for the RDS credentials in section 14.3 (those are generated by the `aws_rds_cluster` resource and stored automatically). The JWT signing secret and the Meilisearch API key are not generated by AWS infrastructure — you must create them manually with values you choose.

```bash
aws secretsmanager create-secret \
  --name library-system/jwt-secret \
  --secret-string '{"secret":"GENERATE_A_STRONG_SECRET"}'

aws secretsmanager create-secret \
  --name library-system/meilisearch-key \
  --secret-string '{"key":"GENERATE_A_STRONG_KEY"}'
```

Replace the placeholder strings with real random values. A 64-character hex string is appropriate for a JWT secret:

```bash
openssl rand -hex 32
```

Run that command twice — once for each secret — and substitute the output for the placeholder values before running the `create-secret` commands. These values do not need to be memorized or stored anywhere outside Secrets Manager; the External Secrets Operator will sync them into the cluster automatically from this point forward.

If you are re-running this step (for example, after a `terraform destroy` and re-apply cycle), use `put-secret-value` instead of `create-secret` to avoid a "secret already exists" error:

```bash
aws secretsmanager put-secret-value \
  --secret-id library-system/jwt-secret \
  --secret-string '{"secret":"YOUR_GENERATED_VALUE"}'
```

---

## Step 4: Apply the Updated Production Overlay

With Terraform complete and secrets present in Secrets Manager, apply the updated production Kustomize overlay:

```bash
kubectl apply -k deploy/k8s/overlays/production
```

This applies the full overlay diff from Chapter 14 in one shot: the `SecretStore` and `ExternalSecret` resources from section 14.3, the Ingress patch with TLS annotations and the ACM certificate ARN from section 14.2, and the ConfigMap patch with the `KAFKA_BROKERS` value updated to port 9094 from section 14.4. Resources that already exist and have not changed are left untouched (Kubernetes `apply` is declarative — it reconciles, not replaces).

Expected additions in the output:

```
secretstore.external-secrets.io/aws-secrets-manager created  (library namespace)
secretstore.external-secrets.io/aws-secrets-manager created  (data namespace)
externalsecret.external-secrets.io/postgres-auth-secret created
externalsecret.external-secrets.io/postgres-catalog-secret created
externalsecret.external-secrets.io/postgres-reservation-secret created
externalsecret.external-secrets.io/jwt-secret created
externalsecret.external-secrets.io/meilisearch-secret created
ingress.networking.k8s.io/library-ingress configured
configmap/catalog-config configured
configmap/reservation-config configured
configmap/search-config configured
```

The Deployments for `auth`, `catalog`, and `reservation` will be updated with the new `KAFKA_BROKERS` value. Kubernetes will perform a rolling update for each — old pods are terminated only after new pods are running and passing their readiness probes. The cluster remains available throughout.

---

## Step 5: Verify Secrets Sync

External Secrets Operator reconciles `ExternalSecret` resources and reports sync status. Check that all secrets synced successfully:

```bash
kubectl get externalsecrets -n library
kubectl get externalsecrets -n data
```

Expected output (library namespace):

```
NAME                         STORE                  REFRESH INTERVAL   STATUS         READY
postgres-auth-secret         aws-secrets-manager    1h                 SecretSynced   True
postgres-catalog-secret      aws-secrets-manager    1h                 SecretSynced   True
postgres-reservation-secret  aws-secrets-manager    1h                 SecretSynced   True
jwt-secret                   aws-secrets-manager    1h                 SecretSynced   True
```

Expected output (data namespace):

```
NAME                STORE                  REFRESH INTERVAL   STATUS         READY
meilisearch-secret  aws-secrets-manager    1h                 SecretSynced   True
```

All entries must show `SecretSynced` in the STATUS column and `True` in READY. If any show `SecretSyncedError`, inspect the ESO operator logs:

```bash
kubectl logs -n external-secrets deployment/external-secrets
```

The most common error at this stage is an IAM permissions problem — the IRSA role exists but the trust policy does not match the service account or namespace. Compare the role's trust policy in the IAM console against the service account annotation on the ESO deployment:

```bash
kubectl get sa -n external-secrets external-secrets \
  -o jsonpath='{.metadata.annotations}'
```

The annotation value should be the `eso_role_arn` from the Terraform output.

---

## Step 6: Verify TLS

With DNS propagated and the ACM certificate attached to the ALB, the application should now be reachable over HTTPS at your custom domain:

```bash
curl -I https://library.example.com
```

Expected output:

```
HTTP/2 200
content-type: application/json
...
```

The `HTTP/2` status line confirms that TLS negotiation succeeded and the ALB returned a valid response. The certificate should be issued by Amazon:

```bash
curl -sv https://library.example.com 2>&1 | grep -A3 "SSL connection"
```

Expected:

```
* SSL connection using TLSv1.3 / TLS_AES_128_GCM_SHA256
* Server certificate:
*  subject: CN=library.example.com
*  issuer: CN=Amazon RSA 2048 M02,O=Amazon,C=US
```

Also verify that the HTTP-to-HTTPS redirect is working:

```bash
curl -I http://library.example.com
```

Expected:

```
HTTP/1.1 301 Moved Permanently
location: https://library.example.com/
```

If `curl -I https://...` returns a `Could not resolve host` error, DNS has not propagated yet. This can take anywhere from a few seconds to several minutes depending on your resolver's cache TTL. Verify the Route 53 record exists before waiting:

```bash
aws route53 list-resource-record-sets \
  --hosted-zone-id $(terraform output -raw route53_zone_id) \
  --query "ResourceRecordSets[?Name=='library.example.com.']"
```

---

## Step 7: Verify MSK TLS

Confirm that the services are connecting to Kafka over TLS by opening a TLS connection to the broker from inside the cluster:

```bash
kubectl exec -it deploy/catalog -n library -- sh -c \
  "echo | openssl s_client -connect MSK_BROKER:9094 2>/dev/null | head -5"
```

Replace `MSK_BROKER` with one of the broker hostnames from `terraform output msk_tls_bootstrap` (the hostname portion only, without the port). Expected output:

```
CONNECTED(00000003)
depth=2 C = US, O = Amazon, CN = Amazon Root CA 1
verify return:1
depth=1 C = US, O = Amazon, CN = Amazon RSA 2048 M02
verify return:1
```

The `CONNECTED` line confirms TCP connectivity on port 9094. The `verify return:1` lines confirm that the broker's certificate chain validated successfully against the system CA bundle in the container. If you see `verify error:num=` instead, the certificate chain is incomplete — check that the MSK cluster configuration has `TLS` set as the client broker encryption rather than `TLS_PLAINTEXT`, which would allow both but not enforce TLS.

Also check the catalog service logs to confirm no Kafka connection errors since the rolling restart:

```bash
kubectl logs -n library deployment/catalog --since=10m | grep -i kafka
```

Expected: connection established messages, no timeout or authentication errors.

---

## Rollback

If something goes wrong at any step, here is how to recover.

**If Terraform apply fails mid-run** — run `terraform apply` again with the same plan file, or re-plan with `terraform plan -out=tfplan`. Terraform is idempotent; resources that were already created will show as unchanged and the partially-applied set will converge. If the failure was in the MSK configuration change specifically, check the MSK cluster state in the AWS console — a rolling restart that was interrupted may leave the cluster in an `UPDATING` state for several minutes before it becomes available again.

**If `kubectl apply` fails** — revert the production overlay to the Chapter 13 state by checking out the previous version of the overlay and re-applying. ESO `ExternalSecret` resources that were already created but then deleted will leave the K8s `Secret` objects they created in place — pods that are already running will continue to use those secrets until they are restarted or deleted. This is deliberate behavior in ESO: secrets are not removed on operator removal to prevent accidental data loss.

**If MSK TLS breaks Kafka connectivity** — the fastest recovery path is to revert the MSK cluster to `TLS_PLAINTEXT` mode (which allows both port 9092 plaintext and port 9094 TLS) rather than `TLS`-only. Update the MSK configuration resource in Terraform and re-apply. This restores port 9092 connectivity immediately, giving you time to debug the TLS configuration without an outage. Then patch the ConfigMap back to port 9092 and apply:

```bash
kubectl patch configmap library-config -n library \
  --type merge \
  -p '{"data":{"KAFKA_BROKERS":"b-1.library.xxxxx.kafka.us-east-1.amazonaws.com:9092,..."}}'
```

Pods will pick up the change on their next restart (or you can force a rollout with `kubectl rollout restart deployment/catalog -n library`).

**If ACM certificate issuance times out** — the `aws_acm_certificate_validation` resource in Terraform has a default timeout of 45 minutes. A timeout usually means the validation CNAME was not found by ACM, which means the Route 53 record was not created or has not propagated. Check with:

```bash
aws acm describe-certificate \
  --certificate-arn $(terraform output -raw acm_certificate_arn) \
  --query 'Certificate.DomainValidationOptions'
```

The output shows the CNAME name and value that ACM expects. Compare against what is in Route 53. If the record is missing, `terraform apply` again — the Route 53 record and certificate validation resources are often created together and a transient API error on the first run can leave one without the other.

---

## Final State

| Component | Before (Chapter 13) | After (Chapter 14) |
|-----------|---------------------|---------------------|
| ALB | HTTP only, auto-generated hostname | HTTPS + HTTP-to-HTTPS redirect, custom domain |
| Domain | None | Route 53 alias to ALB, managed hosted zone |
| TLS certificate | None | ACM certificate, auto-renewing |
| Application secrets | Placeholder values in `secretGenerator` | Auto-synced from Secrets Manager via ESO |
| Kafka connection | Plaintext, port 9092 | TLS-encrypted, port 9094 |

Every row in this table represents a gap that would be flagged in a security review of the Chapter 13 deployment. After this chapter, the system meets the baseline security requirements for a production deployment: encrypted public-facing traffic, managed secrets with no human-visible credential values in Kubernetes manifests or git history, and encrypted broker connections inside the cluster.

---

## Teardown

When you are done, the same `terraform destroy` command from Chapter 13 handles everything. Terraform manages the Route 53 hosted zone, the ACM certificate, the ESO Helm release and IRSA role, and the MSK configuration — all are destroyed in the correct dependency order when you run:

```bash
kubectl delete -k deploy/k8s/overlays/production
terraform destroy
```

Run `kubectl delete` first to allow the ALB controller to deprovision the load balancer before Terraform destroys the VPC. The sequence is the same as Chapter 13; the resource count will be higher (roughly 57 instead of 47) to account for the Chapter 14 additions.

The Secrets Manager entries for `library-system/jwt-secret` and `library-system/meilisearch-key` are not managed by Terraform — you created them manually in Step 3. Delete them separately to avoid the 7-day recovery window that AWS applies to all deleted secrets:

```bash
aws secretsmanager delete-secret \
  --secret-id library-system/jwt-secret \
  --force-delete-without-recovery

aws secretsmanager delete-secret \
  --secret-id library-system/meilisearch-key \
  --force-delete-without-recovery
```

The `--force-delete-without-recovery` flag bypasses the 7-day scheduled deletion and removes the secret immediately. Use it here because these are development secrets — in a real system you might prefer the recovery window.

---

## What's Next

The library system is now production-grade from a security standpoint. Encrypted traffic, managed secrets, and encrypted broker connections are not exotic hardening measures — they are the baseline that any production deployment is expected to meet, and the baseline you would apply on day one in a regulated environment.

The remaining gap is observability. The application emits structured logs, but there is no distributed tracing, no metrics aggregation, and no dashboards showing request latency or error rates across services. Chapter 14 addresses this by deploying an OpenTelemetry Collector to the cluster, instrumenting the Go services with the OpenTelemetry SDK, and wiring the collected telemetry to Grafana for visualization. The deployment workflow stays identical — a new Kustomize overlay, a Terraform module for the monitoring stack, and no changes to application business logic.

---

[^1]: AWS Certificate Manager — Managed Renewal: https://docs.aws.amazon.com/acm/latest/userguide/managed-renewal.html
[^2]: External Secrets Operator — Secret Deletion Behavior: https://external-secrets.io/latest/introduction/faq/#what-happens-to-the-target-secret-when-i-delete-the-externalsecret
[^3]: Amazon MSK — Updating Broker Encryption: https://docs.aws.amazon.com/msk/latest/developerguide/msk-update-security.html
[^4]: AWS Secrets Manager — Deleting Secrets: https://docs.aws.amazon.com/secretsmanager/latest/userguide/manage_delete-secret.html
