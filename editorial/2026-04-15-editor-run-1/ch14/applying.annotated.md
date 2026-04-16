# 14.5 Applying the Changes

<!-- [COPY EDIT] Heading: no em dash; compare 14.1 em dash. Chapter-wide unify. -->

<!-- [FINAL] "Sections 13.1 through 13.4 introduced four independent concerns" — typo: should be "Sections 14.1 through 14.4". Same chapter, not Chapter 13. -->
<!-- [COPY EDIT] Please verify: "Sections 13.1 through 13.4 introduced four independent concerns" — appears to be a chapter-number error. Should be 14.1 through 14.4. -->
Sections 13.1 through 13.4 introduced four independent concerns — Route 53 DNS, ACM certificate provisioning, External Secrets Operator, and MSK TLS. Each was presented in isolation so the concepts could be unpacked without the surrounding noise. Now you apply all four together.

<!-- [LINE EDIT] "This section is deliberately linear. You will run exactly these commands, in exactly this order, and verify exactly these outputs." → strong pedagogical framing. Keep. -->
<!-- [COPY EDIT] "If something goes wrong, the rollback notes at each step tell you how to recover without leaving the cluster in a broken state." — precise. Good. -->
This section is deliberately linear. You will run exactly these commands, in exactly this order, and verify exactly these outputs. If something goes wrong, the rollback notes at each step tell you how to recover without leaving the cluster in a broken state.

---

## Dependency Order

<!-- [STRUCTURAL] Dependency-order section motivates the command sequence. Good pre-reading for the steps that follow. Keep. -->
The four changes have dependencies on each other that are not obvious until you try to apply them out of order.

<!-- [LINE EDIT] "If the hosted zone does not exist yet, Terraform cannot write the validation record, and `terraform apply` will time out waiting for a certificate that can never be issued." → keep; concrete failure mode. -->
- **ACM needs Route 53** — the certificate validation method is DNS. ACM writes a CNAME into your hosted zone and polls for it. If the hosted zone does not exist yet, Terraform cannot write the validation record, and `terraform apply` will time out waiting for a certificate that can never be issued.

<!-- [COPY EDIT] "Kubernetes apply must come after Terraform apply." — precise and terse. Good. -->
- **The ALB needs the ACM certificate ARN** — the Ingress annotation `alb.ingress.kubernetes.io/certificate-arn` references the ARN that ACM assigns at issuance time. Terraform produces the ARN as an output, which the Kustomize overlay references. Kubernetes apply must come after Terraform apply.

<!-- [LINE EDIT] "ESO's IAM role also depends on the cluster's OIDC issuer URL, which Terraform creates." → keep. -->
- **ESO needs the cluster running** — the External Secrets Operator is installed as a Helm release via the Terraform Kubernetes and Helm providers, but the `ExternalSecret` and `SecretStore` Kubernetes resources are applied by `kubectl apply -k`. Both steps require a running EKS cluster. ESO's IAM role also depends on the cluster's OIDC issuer URL, which Terraform creates.

<!-- [LINE EDIT] "enabling the TLS listener on an MSK cluster triggers a rolling configuration update. Each broker restarts one at a time, which takes roughly 10–15 minutes." → keep; sets expectation. -->
<!-- [COPY EDIT] "10–15 minutes" — en dash for range. Good (CMOS 6.78). -->
- **MSK TLS requires a rolling broker restart** — enabling the TLS listener on an MSK cluster triggers a rolling configuration update. Each broker restarts one at a time, which takes roughly 10–15 minutes. The cluster remains available during the rolling restart (MSK is designed for this), but if you update the application's `KAFKA_BROKERS` environment variable to point at port 9094 before the TLS listener is live, services will fail to connect. Apply Terraform first; apply the Kustomize patch after Terraform confirms completion.

<!-- [LINE EDIT] "The correct order is therefore: Terraform first (DNS, ACM, ESO IAM, MSK listener, security group rules), then secrets creation in Secrets Manager, then `kubectl apply` (ESO Kubernetes resources, Ingress TLS annotations, MSK ConfigMap patch)." → keep; summary is useful. -->
The correct order is therefore: Terraform first (DNS, ACM, ESO IAM, MSK listener, security group rules), then secrets creation in Secrets Manager, then `kubectl apply` (ESO Kubernetes resources, Ingress TLS annotations, MSK ConfigMap patch).

---

## Step 1: Add the Domain Variable

Open `terraform/terraform.tfvars` and add the domain name variable introduced in section 14.1:

<!-- [COPY EDIT] Code block without language hint for an HCL/tfvars snippet. Add `hcl` for parity. -->
```
domain_name = "library.example.com"
```

<!-- [LINE EDIT] "Replace `library.example.com` with the domain or subdomain you are using." → keep. -->
<!-- [COPY EDIT] "If you are delegating a subdomain from an external registrar rather than using Route 53 as the registrar, this is still the correct value — the hosted zone is created for whatever name you provide here." — clear. -->
Replace `library.example.com` with the domain or subdomain you are using. If you are delegating a subdomain from an external registrar rather than using Route 53 as the registrar, this is still the correct value — the hosted zone is created for whatever name you provide here.

---

## Step 2: Apply Terraform

Run the plan from the `terraform/` directory:

```bash
cd terraform
terraform plan -out=tfplan
```

The plan output will include a sizeable diff. The new resources are:

<!-- [COPY EDIT] "sizeable" — variant spelling; CMOS 7.88 prefers "sizable". Minor. -->
- `data.aws_route53_zone` — reads your existing hosted zone (you must have already created the zone and delegated DNS to it)
- `aws_route53_record` (alias) — the A-record alias pointing your domain at the ALB
- `aws_acm_certificate` — the certificate request
- `aws_route53_record` (validation) — the CNAME that ACM uses to prove domain control
- `aws_acm_certificate_validation` — a Terraform resource that blocks until ACM reports `ISSUED`
- `aws_iam_role` and `aws_iam_policy` — the IRSA role that External Secrets Operator uses to call Secrets Manager
- `helm_release` (external-secrets) — the ESO Helm chart in the `external-secrets` namespace
<!-- [COPY EDIT] "a new MSK configuration with `TLS_PLAINTEXT` replaced by `TLS`" — but section 14.4 said for fresh deploys go straight to `TLS`. "Replaced by" implies a prior state of `TLS_PLAINTEXT` in the config. Phrasing inconsistency. QUERY. -->
<!-- [COPY EDIT] Please verify: "aws_msk_configuration with `TLS_PLAINTEXT` replaced by `TLS`" — section 14.4 specified `PLAINTEXT` (not `TLS_PLAINTEXT`) as the Ch. 13 state and `TLS` as target. Align wording. -->
- `aws_msk_configuration` — a new MSK configuration with `TLS_PLAINTEXT` replaced by `TLS`
- `aws_msk_cluster` (update) — the cluster update that applies the new configuration
- `aws_security_group_rule` — inbound allow on port 9094 from the EKS node security group

<!-- [LINE EDIT] "Review the plan output. If you see resources listed as destroyed unexpectedly — particularly the MSK cluster itself rather than a configuration change — stop and check that the MSK resource block in `terraform/msk.tf` is using `aws_msk_configuration` rather than an inline `configuration_info` block that Terraform would treat as requiring replacement." → 59 words. Split: "Review the plan. If resources are listed as destroyed unexpectedly — particularly the MSK cluster itself, not just a configuration change — stop. Check that the MSK resource block in `terraform/msk.tf` uses `aws_msk_configuration` rather than an inline `configuration_info` block; Terraform treats the latter as requiring replacement." -->
Review the plan output. If you see resources listed as destroyed unexpectedly — particularly the MSK cluster itself rather than a configuration change — stop and check that the MSK resource block in `terraform/msk.tf` is using `aws_msk_configuration` rather than an inline `configuration_info` block that Terraform would treat as requiring replacement.

When satisfied, apply:

```bash
terraform apply tfplan
```

<!-- [COPY EDIT] "10–15 minutes" — en dash range. Good. -->
<!-- [COPY EDIT] "typically 2–5 minutes" and "typically 8–12 minutes" — en dashes. Good. -->
This takes roughly 10–15 minutes. The ACM validation step waits for Route 53 propagation (typically 2–5 minutes), and the MSK configuration change triggers a rolling broker restart (typically 8–12 minutes). The terminal streams each resource completion as it happens. Expected final lines:

```
Apply complete! Resources: 10 added, 2 changed, 0 destroyed.

Outputs:

acm_certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
eso_role_arn            = "arn:aws:iam::123456789012:role/library-eso-role"
msk_tls_bootstrap       = "b-1.library.xxxxx.kafka.us-east-1.amazonaws.com:9094,b-2.library.xxxxx.kafka.us-east-1.amazonaws.com:9094"
route53_zone_id         = "Z1XXXXXXXXXXXXXXXXX"
```

<!-- [FINAL] Output variable name inconsistency: tls.md defines output as `certificate_arn`; here it is `acm_certificate_arn`. msk-tls.md defines `msk_bootstrap_brokers_tls`; here `msk_tls_bootstrap`. Align names across sections. -->
<!-- [COPY EDIT] Please verify: output names. tls.md → `certificate_arn`; here → `acm_certificate_arn`. msk-tls.md → `msk_bootstrap_brokers_tls`; here → `msk_tls_bootstrap`. Unify. -->
Note the `:9094` port in `msk_tls_bootstrap`. Save these outputs — you will use the certificate ARN and TLS bootstrap string when verifying the overlay in later steps. You can always retrieve them with `terraform output`.

---

## Step 3: Create Non-RDS Secrets in Secrets Manager

<!-- [COPY EDIT] Heading: "Non-RDS" — hyphenated compound; correct (CMOS 7.89). -->

<!-- [FINAL] "Terraform created the Secrets Manager entries for the RDS credentials in section 14.3 (those are generated by the `aws_rds_cluster` resource and stored automatically)." — but section 14.3 and section 14.4 use `aws_db_instance`, not `aws_rds_cluster`. Ch. 13 setup. Verify resource name. -->
<!-- [COPY EDIT] Please verify: "the `aws_rds_cluster` resource" — Ch. 13 used `aws_db_instance` (single-AZ PostgreSQL) per the repo. `aws_rds_cluster` is for Aurora. Correct the resource name or note the distinction. -->
Terraform created the Secrets Manager entries for the RDS credentials in section 14.3 (those are generated by the `aws_rds_cluster` resource and stored automatically). The JWT signing secret and the Meilisearch API key are not generated by AWS infrastructure — you must create them manually with values you choose.

<!-- [COPY EDIT] "Meilisearch API key" — elsewhere in secrets.md referred to as "master key". Unify. -->
```bash
aws secretsmanager create-secret \
  --name library-system/jwt-secret \
  --secret-string '{"secret":"GENERATE_A_STRONG_SECRET"}'

aws secretsmanager create-secret \
  --name library-system/meilisearch-key \
  --secret-string '{"key":"GENERATE_A_STRONG_KEY"}'
```

<!-- [COPY EDIT] "A 64-character hex string is appropriate for a JWT secret" — but the prior `openssl rand -hex 32` produces 64 hex chars. Good. -->
<!-- [LINE EDIT] "Replace the placeholder strings with real random values." → keep. -->
Replace the placeholder strings with real random values. A 64-character hex string is appropriate for a JWT secret:

```bash
openssl rand -hex 32
```

<!-- [FINAL] Conflict: secrets.md (section 14.3) uses `openssl rand -hex 64` (128 chars) for the JWT secret; here `openssl rand -hex 32` (64 chars). Inconsistency. Also note the comment here says "A 64-character hex string is appropriate for a JWT secret" while secrets.md said 128. -->
<!-- [COPY EDIT] Please verify: JWT secret length recommendation. secrets.md recommends 64 bytes / 128 hex chars; here 32 bytes / 64 hex chars. Align. -->

Run that command twice — once for each secret — and substitute the output for the placeholder values before running the `create-secret` commands. These values do not need to be memorized or stored anywhere outside Secrets Manager; the External Secrets Operator will sync them into the cluster automatically from this point forward.

<!-- [LINE EDIT] "If you are re-running this step (for example, after a `terraform destroy` and re-apply cycle), use `put-secret-value` instead of `create-secret` to avoid a 'secret already exists' error" → keep; actionable. -->
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

<!-- [LINE EDIT] "This applies the full overlay diff from Chapter 14 in one shot: the `SecretStore` and `ExternalSecret` resources from section 14.3, the Ingress patch with TLS annotations and the ACM certificate ARN from section 14.2, and the ConfigMap patch with the `KAFKA_BROKERS` value updated to port 9094 from section 14.4." → 52 words; good rhythm. Keep. -->
<!-- [COPY EDIT] "Kubernetes `apply` is declarative — it reconciles, not replaces" — precise. Good. -->
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

<!-- [LINE EDIT] "Kubernetes will perform a rolling update for each — old pods are terminated only after new pods are running and passing their readiness probes." → keep; correct. -->
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

<!-- [FINAL] This sample (library namespace, no meilisearch) disagrees with secrets.md's sample (library namespace including meilisearch). This file is self-consistent with meilisearch in `data`, but contradicts secrets.md. Reconcile across the two files. -->
Expected output (data namespace):

```
NAME                STORE                  REFRESH INTERVAL   STATUS         READY
meilisearch-secret  aws-secrets-manager    1h                 SecretSynced   True
```

<!-- [LINE EDIT] "All entries must show `SecretSynced` in the STATUS column and `True` in READY." → keep. -->
All entries must show `SecretSynced` in the STATUS column and `True` in READY. If any show `SecretSyncedError`, inspect the ESO operator logs:

```bash
kubectl logs -n external-secrets deployment/external-secrets
```

<!-- [LINE EDIT] "The most common error at this stage is an IAM permissions problem — the IRSA role exists but the trust policy does not match the service account or namespace." → keep; diagnostic. -->
The most common error at this stage is an IAM permissions problem — the IRSA role exists but the trust policy does not match the service account or namespace. Compare the role's trust policy in the IAM console against the service account annotation on the ESO deployment:

```bash
kubectl get sa -n external-secrets external-secrets \
  -o jsonpath='{.metadata.annotations}'
```

The annotation value should be the `eso_role_arn` from the Terraform output.

---

## Step 6: Verify TLS

<!-- [LINE EDIT] "With DNS propagated and the ACM certificate attached to the ALB, the application should now be reachable over HTTPS at your custom domain" → keep; clear. -->
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

<!-- [LINE EDIT] "If `curl -I https://...` returns a `Could not resolve host` error, DNS has not propagated yet. This can take anywhere from a few seconds to several minutes depending on your resolver's cache TTL." → keep. -->
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

<!-- [FINAL] Output reference: "from `terraform output msk_tls_bootstrap`" — but msk-tls.md defined the output name as `msk_bootstrap_brokers_tls`. Align. -->
Replace `MSK_BROKER` with one of the broker hostnames from `terraform output msk_tls_bootstrap` (the hostname portion only, without the port). Expected output:

```
CONNECTED(00000003)
depth=2 C = US, O = Amazon, CN = Amazon Root CA 1
verify return:1
depth=1 C = US, O = Amazon, CN = Amazon RSA 2048 M02
verify return:1
```

<!-- [LINE EDIT] "The `CONNECTED` line confirms TCP connectivity on port 9094. The `verify return:1` lines confirm that the broker's certificate chain validated successfully against the system CA bundle in the container." → keep. -->
<!-- [COPY EDIT] "check that the MSK cluster configuration has `TLS` set as the client broker encryption rather than `TLS_PLAINTEXT`, which would allow both but not enforce TLS." → "would allow both but not enforce TLS" — precise phrasing; good. -->
The `CONNECTED` line confirms TCP connectivity on port 9094. The `verify return:1` lines confirm that the broker's certificate chain validated successfully against the system CA bundle in the container. If you see `verify error:num=` instead, the certificate chain is incomplete — check that the MSK cluster configuration has `TLS` set as the client broker encryption rather than `TLS_PLAINTEXT`, which would allow both but not enforce TLS.

Also check the catalog service logs to confirm no Kafka connection errors since the rolling restart:

```bash
kubectl logs -n library deployment/catalog --since=10m | grep -i kafka
```

Expected: connection established messages, no timeout or authentication errors.

---

## Rollback

<!-- [STRUCTURAL] Rollback as a full section is excellent — shows respect for the reader's potential pain. Keep. -->
If something goes wrong at any step, here is how to recover.

<!-- [LINE EDIT] "Terraform is idempotent; resources that were already created will show as unchanged and the partially-applied set will converge." → "partially-applied" has hyphen as compound modifier; CMOS 7.81 — good. -->
<!-- [COPY EDIT] "`UPDATING` state for several minutes before it becomes available again" — OK. -->
**If Terraform apply fails mid-run** — run `terraform apply` again with the same plan file, or re-plan with `terraform plan -out=tfplan`. Terraform is idempotent; resources that were already created will show as unchanged and the partially-applied set will converge. If the failure was in the MSK configuration change specifically, check the MSK cluster state in the AWS console — a rolling restart that was interrupted may leave the cluster in an `UPDATING` state for several minutes before it becomes available again.

<!-- [COPY EDIT] "This is deliberate behavior in ESO: secrets are not removed on operator removal to prevent accidental data loss." — well-stated; but verify this matches ESO's actual default: `creationPolicy: Owner` sets owner reference, which does garbage-collect the Secret on ExternalSecret deletion. QUERY. -->
<!-- [COPY EDIT] Please verify: ESO behavior on ExternalSecret deletion. With `creationPolicy: Owner`, the Kubernetes owner reference is set, so deleting the ExternalSecret DOES garbage-collect the target Secret. Statement "secrets are not removed on operator removal" may conflict with the earlier secrets.md claim that "if you delete the ExternalSecret, the Secret is deleted too". Reconcile. -->
**If `kubectl apply` fails** — revert the production overlay to the Chapter 13 state by checking out the previous version of the overlay and re-applying. ESO `ExternalSecret` resources that were already created but then deleted will leave the K8s `Secret` objects they created in place — pods that are already running will continue to use those secrets until they are restarted or deleted. This is deliberate behavior in ESO: secrets are not removed on operator removal to prevent accidental data loss.

<!-- [LINE EDIT] "the fastest recovery path is to revert the MSK cluster to `TLS_PLAINTEXT` mode (which allows both port 9092 plaintext and port 9094 TLS) rather than `TLS`-only" → keep; good recovery guidance. -->
**If MSK TLS breaks Kafka connectivity** — the fastest recovery path is to revert the MSK cluster to `TLS_PLAINTEXT` mode (which allows both port 9092 plaintext and port 9094 TLS) rather than `TLS`-only. Update the MSK configuration resource in Terraform and re-apply. This restores port 9092 connectivity immediately, giving you time to debug the TLS configuration without an outage. Then patch the ConfigMap back to port 9092 and apply:

```bash
kubectl patch configmap library-config -n library \
  --type merge \
  -p '{"data":{"KAFKA_BROKERS":"b-1.library.xxxxx.kafka.us-east-1.amazonaws.com:9092,..."}}'
```

<!-- [FINAL] Patching "library-config" but earlier sections updated `catalog-config`, `reservation-config`, `search-config` (three separate ConfigMaps). There is no single `library-config`. Command target is incorrect. -->
<!-- [COPY EDIT] Please verify: ConfigMap name in rollback patch. msk-tls.md and step 4 reference `catalog-config`, `reservation-config`, `search-config`. There is no `library-config` ConfigMap. Either fix name(s) or clarify that `library-config` is a different aggregate. -->
Pods will pick up the change on their next restart (or you can force a rollout with `kubectl rollout restart deployment/catalog -n library`).

<!-- [COPY EDIT] "45 minutes" — numeral + unit OK. -->
<!-- [COPY EDIT] Please verify: "`aws_acm_certificate_validation` resource in Terraform has a default timeout of 45 minutes" — confirm current default (Terraform AWS provider). Default is 45m but has been changed across provider versions. -->
**If ACM certificate issuance times out** — the `aws_acm_certificate_validation` resource in Terraform has a default timeout of 45 minutes. A timeout usually means the validation CNAME was not found by ACM, which means the Route 53 record was not created or has not propagated. Check with:

```bash
aws acm describe-certificate \
  --certificate-arn $(terraform output -raw acm_certificate_arn) \
  --query 'Certificate.DomainValidationOptions'
```

<!-- [FINAL] "acm_certificate_arn" — tls.md defined the output as `certificate_arn`. Align output names. -->
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

<!-- [LINE EDIT] "Every row in this table represents a gap that would be flagged in a security review of the Chapter 13 deployment." → keep; strong closer. -->
<!-- [LINE EDIT] "After this chapter, the system meets the baseline security requirements for a production deployment: encrypted public-facing traffic, managed secrets with no human-visible credential values in Kubernetes manifests or git history, and encrypted broker connections inside the cluster." → 39 words. Good rhythm. Keep. -->
Every row in this table represents a gap that would be flagged in a security review of the Chapter 13 deployment. After this chapter, the system meets the baseline security requirements for a production deployment: encrypted public-facing traffic, managed secrets with no human-visible credential values in Kubernetes manifests or git history, and encrypted broker connections inside the cluster.

---

## Teardown

<!-- [LINE EDIT] "When you are done, the same `terraform destroy` command from Chapter 13 handles everything." → keep; reassuring. -->
When you are done, the same `terraform destroy` command from Chapter 13 handles everything. Terraform manages the Route 53 hosted zone, the ACM certificate, the ESO Helm release and IRSA role, and the MSK configuration — all are destroyed in the correct dependency order when you run:

```bash
kubectl delete -k deploy/k8s/overlays/production
terraform destroy
```

<!-- [COPY EDIT] "roughly 57 instead of 47" — specific counts; verify they match the project's actual resource counts. QUERY. -->
<!-- [COPY EDIT] Please verify: resource counts "roughly 57 instead of 47" — spot-check against the actual Terraform state size. -->
Run `kubectl delete` first to allow the ALB controller to deprovision the load balancer before Terraform destroys the VPC. The sequence is the same as Chapter 13; the resource count will be higher (roughly 57 instead of 47) to account for the Chapter 14 additions.

<!-- [COPY EDIT] "the 7-day recovery window that AWS applies to all deleted secrets" — CMOS 9.3: small numbers in prose usually spelled; "7-day" as compound adjective with unit is acceptable as numerals (CMOS 9.13). Good. -->
The Secrets Manager entries for `library-system/jwt-secret` and `library-system/meilisearch-key` are not managed by Terraform — you created them manually in Step 3. Delete them separately to avoid the 7-day recovery window that AWS applies to all deleted secrets:

```bash
aws secretsmanager delete-secret \
  --secret-id library-system/jwt-secret \
  --force-delete-without-recovery

aws secretsmanager delete-secret \
  --secret-id library-system/meilisearch-key \
  --force-delete-without-recovery
```

<!-- [LINE EDIT] "The `--force-delete-without-recovery` flag bypasses the 7-day scheduled deletion and removes the secret immediately. Use it here because these are development secrets — in a real system you might prefer the recovery window." → good warning flag. Keep. -->
The `--force-delete-without-recovery` flag bypasses the 7-day scheduled deletion and removes the secret immediately. Use it here because these are development secrets — in a real system you might prefer the recovery window.

---

## What's Next

<!-- [STRUCTURAL] "What's Next" is well-placed. Names the remaining observability gap and sets up Chapter 15 — but see next comment. -->
<!-- [FINAL] "Chapter 14 addresses this by deploying an OpenTelemetry Collector..." — but this IS Chapter 14. The observability work is next chapter (15). Chapter-number error in the closing. -->
<!-- [COPY EDIT] Please verify: text says "Chapter 14 addresses this" but this is the closing of Chapter 14 itself; the observability discussion is forward-looking. Should read "Chapter 15 addresses this". -->
The library system is now production-grade from a security standpoint. Encrypted traffic, managed secrets, and encrypted broker connections are not exotic hardening measures — they are the baseline that any production deployment is expected to meet, and the baseline you would apply on day one in a regulated environment.

The remaining gap is observability. The application emits structured logs, but there is no distributed tracing, no metrics aggregation, and no dashboards showing request latency or error rates across services. Chapter 14 addresses this by deploying an OpenTelemetry Collector to the cluster, instrumenting the Go services with the OpenTelemetry SDK, and wiring the collected telemetry to Grafana for visualization. The deployment workflow stays identical — a new Kustomize overlay, a Terraform module for the monitoring stack, and no changes to application business logic.

---

[^1]: AWS Certificate Manager — Managed Renewal: https://docs.aws.amazon.com/acm/latest/userguide/managed-renewal.html
[^2]: External Secrets Operator — Secret Deletion Behavior: https://external-secrets.io/latest/introduction/faq/#what-happens-to-the-target-secret-when-i-delete-the-externalsecret
[^3]: Amazon MSK — Updating Broker Encryption: https://docs.aws.amazon.com/msk/latest/developerguide/msk-update-security.html
[^4]: AWS Secrets Manager — Deleting Secrets: https://docs.aws.amazon.com/secretsmanager/latest/userguide/manage_delete-secret.html

<!-- [FINAL] None of footnotes [^1]–[^4] appear to be referenced in the body of this section. Add in-body references or remove. -->
