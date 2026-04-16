# 14.3 Secrets Management with External Secrets Operator

<!-- [STRUCTURAL] Heading style: "14.3 Secrets Management with External Secrets Operator" — no em dash, Title Case. Consistent with tls.md, inconsistent with dns.md (em dash, sentence case). Unify chapter-wide. -->
<!-- [STRUCTURAL] Opening quotes the Ch. 13 comment verbatim — excellent way to establish the gap being closed. Very effective. Keep. -->

Chapter 13's production overlay left a comment in the `secretGenerator` block that was easy to skim past:

```yaml
# Substitute real values from `terraform output` and
# `aws secretsmanager get-secret-value` before deploying.
secretGenerator:
  - name: postgres-catalog-secret
    namespace: library
    literals:
      - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
```

<!-- [LINE EDIT] "That comment describes a manual process: you run an AWS CLI command to retrieve the password, copy the output, paste it into the kustomization file or a shell command, run `kubectl apply`, and then delete the plaintext value from your terminal." → 46 words. Consider splitting: "That comment describes a manual process. You run an AWS CLI command to retrieve the password, copy the output, paste it into the kustomization file or a shell command, run `kubectl apply`, then delete the plaintext value from your terminal." -->
That comment describes a manual process: you run an AWS CLI command to retrieve the password, copy the output, paste it into the kustomization file or a shell command, run `kubectl apply`, and then delete the plaintext value from your terminal. It works once. It does not work well at any scale, and it introduces several problems that compound over time.

---

## The problem with manual secret management

<!-- [STRUCTURAL] Four bolded lead-ins ("Shell history", "Rotation without downtime", "Error-prone during re-deployments", "The principle of least manual intervention") work well as a scanning structure. Keep. -->
<!-- [COPY EDIT] "Shell history." — label followed by period, then sentence. Consistent formatting across the four. Good. -->

<!-- [COPY EDIT] "`~/.zsh_history` or `~/.bash_history`" — backticked file paths. Good. -->
<!-- [LINE EDIT] "That file is typically world-readable on developer laptops, synced to dotfile repositories, and occasionally included in support bundles." → "world-readable" is strong; historically shell history files are mode 600. Verify. QUERY. -->
<!-- [COPY EDIT] Please verify: claim that shell history files are "typically world-readable on developer laptops". Default mode for `~/.zsh_history` and `~/.bash_history` is usually 0600 (owner only). They are personally readable but not world-readable. Soften phrasing to "readable by any process running as your user" or similar. -->
**Shell history.** Every `aws secretsmanager get-secret-value` invocation that you pipe into a `kubectl create secret` call or paste into an environment variable leaves a copy of the plaintext credential in `~/.zsh_history` or `~/.bash_history`. That file is typically world-readable on developer laptops, synced to dotfile repositories, and occasionally included in support bundles. You are also trusting that every operator who has ever run this step on a shared bastion host did not leave similar traces.

<!-- [COPY EDIT] "roughly every 30 days" — numeral + unit fine. CMOS 9.7. Good. -->
<!-- [COPY EDIT] "rotation" — would benefit from a concrete noting that RDS's managed-password rotation is Lambda-driven. Minor. -->
**Rotation without downtime.** AWS Secrets Manager can rotate the RDS master password on a schedule — roughly every 30 days is a common policy. When it does, the old password stops working immediately. If your Kubernetes Secret still holds the old value, every pod that restarts or that reads the credential at connection-establishment time will fail with an authentication error. Manual rotation means someone has to notice the failure, retrieve the new password, update the Secret, and trigger a rolling restart — all while the application is broken.

<!-- [COPY EDIT] "re-deployments" vs "redeployments" — CMOS 7.89 allows either; unify chapter-wide. -->
<!-- [LINE EDIT] "Placeholder values are sticky." → short punchy opener. Keep. -->
<!-- [COPY EDIT] "at least one value gets wrong" → "at least one value is wrong". "gets wrong" is awkward phrasing. -->
**Error-prone during re-deployments.** Placeholder values are sticky. If you run `kustomize build overlays/production | kubectl apply -f -` after pulling the latest git state on a fresh machine, the Secrets you created manually are already in the cluster — so nothing breaks immediately. But the day you run `terraform destroy` and `terraform apply` to rebuild the cluster for a new region or disaster recovery, you have to reconstruct every Secret by hand. The list of secrets grows over time, documentation for the process drifts, and at least one value gets wrong.

<!-- [COPY EDIT] "Infrastructure-as-code" — hyphenated noun phrase; elsewhere in the book "infrastructure as code" and "infrastructure-as-code" appear. CMOS 7.85: hyphenate when used as adjective. Here it is the subject of the sentence (noun). Preferred: "Infrastructure as code exists to..." -->
<!-- [LINE EDIT] "A manual secret-pasting step is a gap in that reproducibility — a step that is not captured in any file that `git blame` can show you, not gated by any code review, and not executable by your CI pipeline without introducing its own secret-management problem." → 41 words; good rhythm but long. Consider retaining. -->
**The principle of least manual intervention.** Infrastructure-as-code exists to make system state reproducible without human memory. A manual secret-pasting step is a gap in that reproducibility — a step that is not captured in any file that `git blame` can show you, not gated by any code review, and not executable by your CI pipeline without introducing its own secret-management problem.

<!-- [LINE EDIT] "The right answer is to treat secrets the same way you treat infrastructure: define them declaratively, let an automated system reconcile the desired state with the actual state, and audit the process rather than the operator." → strong summary sentence. Keep. -->
The right answer is to treat secrets the same way you treat infrastructure: define them declaratively, let an automated system reconcile the desired state with the actual state, and audit the process rather than the operator.

---

## How External Secrets Operator works

<!-- [COPY EDIT] Heading sentence case here; other sections use Title Case. Unify. -->
**External Secrets Operator** (ESO) is a Kubernetes controller that bridges external secret stores — AWS Secrets Manager, HashiCorp Vault, GCP Secret Manager, Azure Key Vault, and others — with native Kubernetes Secrets.[^1] Instead of running a kubectl command, you declare two custom resources:

<!-- [COPY EDIT] "GCP Secret Manager" — current product is "Google Secret Manager" (the "GCP" prefix is no longer official branding). QUERY. -->
<!-- [COPY EDIT] Please verify: current product name — "Google Secret Manager" vs "GCP Secret Manager". Google dropped "GCP" branding; prefer "Google Secret Manager". -->
<!-- [COPY EDIT] "kubectl command" → "`kubectl` command" for code styling consistency. -->
- A **`SecretStore`** that tells ESO how to connect to the external store. For AWS Secrets Manager, this specifies the region and the IAM authentication method.
- An **`ExternalSecret`** that tells ESO which value to fetch and what Kubernetes Secret to create from it. You specify the remote key name, the field within that key's JSON payload, and the local Secret key to populate.

ESO polls the external store on a configurable `refreshInterval`. When it fetches a value, it creates or updates the corresponding Kubernetes Secret. If the value changes in Secrets Manager — because RDS rotated the password — ESO writes the new value to the Secret on the next poll cycle.

```mermaid
flowchart LR
    ASM["AWS Secrets Manager\n(rds!db-library-system-catalog)"]
    ESO["ESO Controller\n(external-secrets namespace)"]
    KS["Kubernetes Secret\n(postgres-catalog-secret)"]
    POD["Pod env var\n(POSTGRES_PASSWORD)"]

    ASM -->|poll every 1h via IRSA| ESO
    ESO -->|create / update| KS
    KS -->|secretKeyRef| POD
```

<!-- [COPY EDIT] "a human with a clipboard" — voice marker, good. Keep. -->
From the pod's perspective, nothing changes. The Deployment still references `postgres-catalog-secret` via `secretKeyRef`. The difference is that ESO creates and maintains that Secret instead of a human with a clipboard.

<!-- [LINE EDIT] "ESO does not require any changes to application code, Dockerfiles, or the base Kustomize manifests. The interface between the operator and the application is the Kubernetes Secret object — a stable API that both sides already speak." → excellent summary; keep. -->
ESO does not require any changes to application code, Dockerfiles, or the base Kustomize manifests. The interface between the operator and the application is the Kubernetes Secret object — a stable API that both sides already speak.

---

## IRSA for ESO

<!-- [COPY EDIT] "IRSA" — first use in this file. Expand: "IAM Roles for Service Accounts (IRSA)". Even if defined in Ch. 13, safer to re-expand on first use per chapter. -->
ESO needs permission to call `GetSecretValue` on the Secrets Manager secrets it is responsible for. In an EKS cluster, the correct way to grant AWS permissions to a pod is IRSA — the same pattern you used for the EBS CSI driver and the Load Balancer Controller in Chapter 13.

<!-- [COPY EDIT] "terraform-aws-modules/iam" — consistent casing. Good. -->
<!-- [COPY EDIT] "`secretsmanager:GetSecretValue`, `secretsmanager:DescribeSecret`, and `secretsmanager:ListSecretVersionIds`" — three permissions, serial comma OK. Good. -->
The `terraform-aws-modules/iam` module has a built-in preset for External Secrets Operator. Setting `attach_external_secrets_policy = true` creates a policy with the minimum required permissions: `secretsmanager:GetSecretValue`, `secretsmanager:DescribeSecret`, and `secretsmanager:ListSecretVersionIds`.[^2] You scope it to a resource ARN pattern to limit access to only the secrets this cluster owns.

<!-- [COPY EDIT] "`manage_master_user_password = true`" — consistent backtick formatting. Good. -->
<!-- [LINE EDIT] "The automatically created secrets follow the naming pattern `rds!db-<identifier>`." → concise and informative. Keep. -->
<!-- [COPY EDIT] Please verify: pattern `rds!db-<identifier>` — AWS docs show `rds!<cluster-id>` or `rds!db-<identifier>-<suffix>`. Confirm the exact pattern used for RDS non-cluster instances with `manage_master_user_password`. -->
RDS creates Secrets Manager entries automatically when `manage_master_user_password = true` is set on the `aws_db_instance` resource — which your Chapter 13 Terraform already does for all three databases. The automatically created secrets follow the naming pattern `rds!db-<identifier>`. The `*` wildcard in the ARN below matches the random suffix that Secrets Manager appends to prevent enumeration.

Create `terraform/eso.tf`:

```hcl
# terraform/eso.tf

# --- IRSA role for External Secrets Operator ---
# ESO needs GetSecretValue access to read RDS credentials and the
# manually created JWT and Meilisearch secrets.
module "eso_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.34"

  role_name = "${var.project_name}-external-secrets"

  attach_external_secrets_policy        = true
  external_secrets_secrets_manager_arns = [
    # RDS-managed secrets use the rds! prefix. The suffix after the secret
    # name is a random ID appended by Secrets Manager; the wildcard covers it.
    "arn:aws:secretsmanager:${var.region}:${data.aws_caller_identity.current.account_id}:secret:rds!*",
    # Manually created secrets for JWT and Meilisearch use a path prefix.
    "arn:aws:secretsmanager:${var.region}:${data.aws_caller_identity.current.account_id}:secret:library-system/*",
  ]

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      # ESO's service account lives in the external-secrets namespace.
      # The trust policy allows only this exact namespace/serviceaccount pair
      # to assume the role.
      namespace_service_accounts = ["external-secrets:external-secrets"]
    }
  }

  tags = local.common_tags
}

# --- Install ESO via Helm ---
# The Helm chart installs the controller Deployment, the CRDs for
# SecretStore and ExternalSecret, and the RBAC rules ESO needs to
# create and update Kubernetes Secrets.
resource "helm_release" "external_secrets" {
  name       = "external-secrets"
  repository = "https://charts.external-secrets.io"
  chart      = "external-secrets"
  namespace  = "external-secrets"
  version    = "0.9.13"

  create_namespace = true

  # Annotate the service account with the role ARN so the OIDC webhook
  # can inject the web identity token at pod startup.
  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.eso_irsa.iam_role_arn
  }

  # The controller must be running before any ExternalSecret resources
  # can be reconciled; the CRDs need to exist before Kubernetes will
  # accept ExternalSecret or SecretStore objects.
  depends_on = [module.eks]
}
```

<!-- [COPY EDIT] Please verify: ESO Helm chart version "0.9.13" — as of April 2026, chart is at v0.10+. For a learning book published 2026, consider pinning to a recent stable; or add a note that readers should consult the project for the latest. -->
<!-- [LINE EDIT] "The `namespace_service_accounts = [\"external-secrets:external-secrets\"]` entry is the binding between the Kubernetes identity and the IAM role." → keep; clear. -->
<!-- [COPY EDIT] "`AssumeRoleWithWebIdentity`" — AWS API name, correctly styled. Good. -->
The `namespace_service_accounts = ["external-secrets:external-secrets"]` entry is the binding between the Kubernetes identity and the IAM role. The trust policy it generates says: only tokens issued to the service account named `external-secrets` in the `external-secrets` namespace on this specific OIDC provider may call `AssumeRoleWithWebIdentity` for this role. A token issued to any other pod — even one in the same cluster — is denied.

Apply this before moving to the Kubernetes manifests:

```bash
cd terraform
terraform apply -target=module.eso_irsa -target=helm_release.external_secrets
```

<!-- [COPY EDIT] Using `-target` in `terraform apply` is discouraged for normal workflows (per Terraform docs — "intended only for exceptional situations"). Consider a QUERY or note acknowledging this trade-off. -->
<!-- [COPY EDIT] Please verify phrasing — HashiCorp's official position is that `-target` is for exceptional recovery, not routine apply. Consider adding a brief note that this is a bootstrapping apply, not a normal workflow step. -->
Wait until the ESO pods are healthy before continuing:

<!-- [COPY EDIT] Sample kubectl output alignment uses mixed spaces; cosmetic only — keep verbatim if this matches the real tool output. -->
```
$ kubectl get pods -n external-secrets

NAME                                                READY   STATUS    RESTARTS   AGE
external-secrets-6d8b9f7c4-xk9pz                   1/1     Running   0          90s
external-secrets-cert-controller-77b6d9b8c-m2jql   1/1     Running   0          90s
external-secrets-webhook-5c7f8d9b4-v8nrw            1/1     Running   0          90s
```

<!-- [LINE EDIT] "Three pods: the main controller that reconciles `ExternalSecret` resources, a cert-controller that manages the webhook TLS certificates, and an admission webhook that validates your `ExternalSecret` and `SecretStore` definitions at apply time." → fragment as sentence — works. Keep. -->
Three pods: the main controller that reconciles `ExternalSecret` resources, a cert-controller that manages the webhook TLS certificates, and an admission webhook that validates your `ExternalSecret` and `SecretStore` definitions at apply time. All three need to be `Running` before you create the Kubernetes resources.

---

## Creating the non-RDS secrets

<!-- [COPY EDIT] "non-RDS" — hyphenated compound prefix; CMOS 7.89. Good. -->
RDS secrets are created automatically by Secrets Manager when you set `manage_master_user_password = true`. The JWT secret and the Meilisearch master key are not database credentials and have no equivalent automated source — you need to create them once:

<!-- [COPY EDIT] "64-byte" — "openssl rand -hex 64" produces 128 hex chars = 64 bytes. Comment is accurate. Good. -->
<!-- [COPY EDIT] "cryptographically random 64-byte JWT secret" — good phrasing. -->
<!-- [LINE EDIT] "long enough that brute force is not a practical concern" → keep; pedagogical reassurance. -->
```bash
# Generate a cryptographically random 64-byte JWT secret and store it.
# `openssl rand -hex 64` produces 128 hex characters — long enough that
# brute force is not a practical concern.
aws secretsmanager create-secret \
  --name library-system/jwt-secret \
  --description "JWT signing secret for the library system auth service" \
  --secret-string "{\"secret\":\"$(openssl rand -hex 64)\"}"

# Generate a random Meilisearch master key. Meilisearch requires this
# to be present at first startup; it cannot be changed after the index
# is populated without a full re-index.
aws secretsmanager create-secret \
  --name library-system/meilisearch-key \
  --description "Meilisearch master key for the library system search service" \
  --secret-string "{\"key\":\"$(openssl rand -hex 32)\"}"
```

<!-- [COPY EDIT] Please verify: Meilisearch master key claim — "cannot be changed after the index is populated without a full re-index". Meilisearch docs indicate you can change the master key but it invalidates existing API keys; not strictly re-index. Clarify. -->
<!-- [LINE EDIT] "The JSON wrapper (`{\"secret\": \"...\"}`) is deliberate." → keep. -->
<!-- [COPY EDIT] "Secrets Manager's own rotation framework expects" — precise. Good. -->
The JSON wrapper (`{"secret": "..."}`) is deliberate. ESO's `remoteRef.property` field extracts a specific key from a JSON-formatted secret value. If you store a bare string instead, you must omit the `property` field and ESO will use the entire value — which works, but JSON-formatted secrets are easier to extend later (if you need to add a `rotation_lambda_arn` field, for example) and are the format Secrets Manager's own rotation framework expects.

Verify both secrets exist:

```bash
aws secretsmanager list-secrets --query "SecretList[?starts_with(Name, 'library-system')].Name"
```

Expected output:

```json
[
    "library-system/jwt-secret",
    "library-system/meilisearch-key"
]
```

---

## The SecretStore manifest

<!-- [STRUCTURAL] SecretStore/ExternalSecret/Update-overlay sequence is logical. Maybe too many YAML blocks in series — reader's eyes glaze. Consider collapsing the five ExternalSecret examples to one annotated sample plus a table listing the others. Alternatively, present in a folded/summarized form. See line 230 comment. -->
A `SecretStore` is a namespaced resource that tells ESO how to reach the external secrets backend. Create `deploy/k8s/overlays/production/secret-store.yaml`:

```yaml
# deploy/k8s/overlays/production/secret-store.yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets-manager
  namespace: library
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      # Use the JWT auth method: ESO's service account carries an OIDC
      # token issued by EKS. The serviceAccountRef points to the ESO
      # service account in the external-secrets namespace, which has the
      # IRSA annotation that maps to the IAM role you created above.
      auth:
        jwt:
          serviceAccountRef:
            name: external-secrets
            namespace: external-secrets
```

<!-- [COPY EDIT] "v1beta1" — ESO current API is `external-secrets.io/v1`. QUERY: confirm whether v1beta1 is still the recommended API or whether v1 has been promoted. -->
<!-- [COPY EDIT] Please verify: ESO API version. As of ESO 0.10+, v1 is the stable API. v1beta1 may still be supported but deprecated. -->
The `auth.jwt.serviceAccountRef` field is the cross-namespace reference that ties this `SecretStore` to ESO's IAM permissions. When ESO reconciles an `ExternalSecret` that references this `SecretStore`, it uses the JWT token from the `external-secrets` service account in the `external-secrets` namespace to call STS and assume the IRSA role. The ESO controller itself runs in `external-secrets`, so it can read that service account's token directly.

<!-- [LINE EDIT] "A `SecretStore` is namespaced — it is only usable by `ExternalSecret` resources in the same namespace." → keep; clear. -->
<!-- [COPY EDIT] "`ClusterSecretStore`" — backticked, consistent. -->
A `SecretStore` is namespaced — it is only usable by `ExternalSecret` resources in the same namespace. If you had multiple application namespaces, you would either create a `SecretStore` in each one or use a `ClusterSecretStore` (a cluster-scoped variant). For this system, which has a single `library` namespace, the namespaced variant is the right choice.

<!-- [STRUCTURAL] Note: below the reader learns the meilisearch ExternalSecret targets a `data` namespace — implying there needs to be a second SecretStore in the `data` namespace too. The SecretStore here only defines one for `library`. The inline note later refers to it but may confuse on first read. Consider adding the second SecretStore block here. -->

---

## The ExternalSecret manifests

Create `deploy/k8s/overlays/production/external-secrets.yaml`. This file defines one `ExternalSecret` per secret the application needs:

<!-- [STRUCTURAL] ~110 lines of YAML in a single block with five very similar ExternalSecret entries. Strong candidate for collapsing: show 1–2 worked examples (RDS-managed + the JWT one), then list the rest in a table with remoteRef.key / property / secretKey / target.namespace. Reduces eye fatigue and makes edit-points obvious. -->
```yaml
# deploy/k8s/overlays/production/external-secrets.yaml

# --- Catalog database password ---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: postgres-catalog-secret
  namespace: library
spec:
  # Poll Secrets Manager every hour. If the RDS password rotates,
  # the Kubernetes Secret will be updated within one hour.
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    # name must match the secretKeyRef.name in the Deployment patch.
    name: postgres-catalog-secret
    # Owner means ESO sets itself as the owner of this Secret.
    # If you delete the ExternalSecret, the Secret is deleted too.
    creationPolicy: Owner
  data:
    - secretKey: POSTGRES_PASSWORD
      remoteRef:
        # RDS-managed secrets use the rds! prefix followed by the
        # db-instance identifier.
        key: rds!db-library-system-catalog
        # The RDS-managed secret is JSON: {"username": "...", "password": "..."}.
        # The property field extracts only the password field.
        property: password
---
# --- Auth database password ---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: postgres-auth-secret
  namespace: library
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: postgres-auth-secret
    creationPolicy: Owner
  data:
    - secretKey: POSTGRES_PASSWORD
      remoteRef:
        key: rds!db-library-system-auth
        property: password
---
# --- Reservation database password ---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: postgres-reservation-secret
  namespace: library
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: postgres-reservation-secret
    creationPolicy: Owner
  data:
    - secretKey: POSTGRES_PASSWORD
      remoteRef:
        key: rds!db-library-system-reservation
        property: password
---
# --- JWT signing secret ---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: jwt-secret
  namespace: library
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: jwt-secret
    creationPolicy: Owner
  data:
    - secretKey: JWT_SECRET
      remoteRef:
        key: library-system/jwt-secret
        property: secret
---
# --- Meilisearch master key ---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: meilisearch-secret
  namespace: data          # Meilisearch runs in the data namespace, not library
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: meilisearch-secret
    creationPolicy: Owner
  data:
    - secretKey: MEILI_MASTER_KEY
      remoteRef:
        key: library-system/meilisearch-key
        property: key
```

<!-- [FINAL] "the meilisearch ExternalSecret targets" → "the Meilisearch ExternalSecret targets" (capital M — product name). -->
<!-- [COPY EDIT] Please verify: cross-namespace reference. "This also requires a SecretStore in the `data` namespace; see `secret-store.yaml` for the second SecretStore definition." — but the secret-store.yaml example above shows only one SecretStore in `library`. Either add the second SecretStore to the YAML example, or update the referring text. -->
Note that the meilisearch ExternalSecret targets the `data` namespace — not `library` — because the Meilisearch StatefulSet runs in the `data` namespace. This also requires a SecretStore in the `data` namespace; see `secret-store.yaml` for the second SecretStore definition.

<!-- [LINE EDIT] "Each `ExternalSecret` maps a single field from a Secrets Manager secret to a single key in a Kubernetes Secret." → keep; good concise framing. -->
Each `ExternalSecret` maps a single field from a Secrets Manager secret to a single key in a Kubernetes Secret. The `target.name` matches exactly the name used in the StatefulSet or Deployment's `secretKeyRef.name` — `postgres-catalog-secret`, `postgres-auth-secret`, `postgres-reservation-secret`, `jwt-secret`, and `meilisearch-secret`. No application manifest changes are required.

<!-- [LINE EDIT] "The `creationPolicy: Owner` setting is worth understanding." → keep as lead-in. -->
<!-- [COPY EDIT] "`creationPolicy: Merge`" — value "Merge" requires a pre-existing Secret with ESO patching in keys. QUERY: verify that ESO's `Merge` policy requires the target Secret to already exist (that is the project's documented behavior). -->
<!-- [COPY EDIT] Please verify: `creationPolicy: Merge` semantics per ESO docs — merges into an existing Secret without managing its lifecycle. Confirm the "useful when you want to add ESO-managed keys into an existing Secret" phrasing is precise. -->
The `creationPolicy: Owner` setting is worth understanding. It means ESO creates the Kubernetes Secret and marks itself as the owner via a Kubernetes owner reference. If you run `kubectl delete externalsecret postgres-catalog-secret -n library`, Kubernetes will garbage-collect the associated Secret automatically. This is the right policy for production: the Secret should not outlive its source of truth. The alternative, `creationPolicy: Merge`, is useful when you want to add ESO-managed keys into an existing Secret that is partially managed by other means — not the case here.

---

## Updating the production overlay

<!-- [STRUCTURAL] Removal of secretGenerator + addition of resources is the right narrative arc. Keep. -->
With ESO managing the Secrets, the `secretGenerator` block in the production overlay is no longer needed. Remove it entirely from `deploy/k8s/overlays/production/kustomization.yaml`, along with the `generatorOptions` block:

```yaml
# Remove these lines from kustomization.yaml:

# secretGenerator:
#   - name: jwt-secret
#     namespace: library
#     literals:
#       - JWT_SECRET=REPLACE_WITH_PRODUCTION_SECRET
#   - name: postgres-catalog-secret
#     namespace: library
#     literals:
#       - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
#   - name: postgres-auth-secret
#     namespace: library
#     literals:
#       - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
#   - name: postgres-reservation-secret
#     namespace: library
#     literals:
#       - POSTGRES_PASSWORD=REPLACE_WITH_RDS_PASSWORD
#   - name: meilisearch-secret
#     namespace: library
#     literals:
#       - MEILI_MASTER_KEY=REPLACE_WITH_PRODUCTION_KEY
#
# generatorOptions:
#   disableNameSuffixHash: true
```

Add the two new manifest files to the `resources` list instead:

```yaml
resources:
  - ../../base
  - secret-store.yaml
  - external-secrets.yaml
```

<!-- [LINE EDIT] "The Deployment patches that reference `postgres-catalog-secret`, `jwt-secret`, and so on are unchanged." → keep. -->
<!-- [COPY EDIT] "Kustomize's `secretGenerator` is replaced by ESO" — correct. -->
The Deployment patches that reference `postgres-catalog-secret`, `jwt-secret`, and so on are unchanged. They were already written to consume a Kubernetes Secret by name. The only thing that changes is who creates those Secrets — Kustomize's `secretGenerator` is replaced by ESO.

Apply the updated overlay:

```bash
kubectl apply -k deploy/k8s/overlays/production/
```

Kustomize will apply the `SecretStore` and all five `ExternalSecret` objects. ESO's controller detects the new `ExternalSecret` resources almost immediately and begins reconciling.

---

## Verification

Check that ESO has reconciled each secret:

<!-- [FINAL] Output formatting: "postgres-reservation-secret aws-secrets-manager  1h ..." — "REFRESH INTERVAL" column alignment uses two spaces here and three elsewhere. Cosmetic only. -->
```
$ kubectl get externalsecrets -n library

NAME                       STORE                 REFRESH INTERVAL   STATUS         READY
jwt-secret                 aws-secrets-manager   1h                 SecretSynced   True
meilisearch-secret         aws-secrets-manager   1h                 SecretSynced   True
postgres-auth-secret       aws-secrets-manager   1h                 SecretSynced   True
postgres-catalog-secret    aws-secrets-manager   1h                 SecretSynced   True
postgres-reservation-secret aws-secrets-manager  1h                 SecretSynced   True
```

<!-- [FINAL] Note: above output shows "meilisearch-secret" in the library namespace, but the ExternalSecret manifest puts meilisearch-secret in the `data` namespace. Contradicts previous block. -->
<!-- [COPY EDIT] Please verify: sample `kubectl get externalsecrets -n library` output includes "meilisearch-secret", but the manifest earlier explicitly places it in the `data` namespace. Correct one or the other. -->

The `STATUS` column shows `SecretSynced` and `READY` is `True` for each resource when ESO has successfully fetched the value from Secrets Manager and written it to the cluster. If any row shows `SecretSyncedError`, inspect the status condition for detail:

```bash
kubectl describe externalsecret postgres-catalog-secret -n library
```

The `Status.Conditions` section will contain a message. Common causes:

<!-- [COPY EDIT] "`unauthorized`" — quoted code style; consistent. Good. -->
<!-- [LINE EDIT] "Verify with `kubectl describe serviceaccount external-secrets -n external-secrets` and confirm the `eks.amazonaws.com/role-arn` annotation matches the Terraform output." → keep; actionable. -->
- **`unauthorized`** — the IRSA role ARN is wrong, or the `namespace_service_accounts` field in the Terraform module does not exactly match `external-secrets:external-secrets`. Verify with `kubectl describe serviceaccount external-secrets -n external-secrets` and confirm the `eks.amazonaws.com/role-arn` annotation matches the Terraform output.
- **`ResourceNotFoundException`** — the secret key (e.g., `rds!db-library-system-catalog`) does not exist in Secrets Manager. The RDS identifier in Terraform and the key name in the `ExternalSecret` must match. Run `aws secretsmanager list-secrets` to see the actual names.
- **`InvalidParameterException`** — the `property` field does not match a key in the JSON payload. Verify the secret format with `aws secretsmanager get-secret-value --secret-id rds!db-library-system-catalog`.

Once all five resources are `SecretSynced`, confirm the Kubernetes Secrets exist:

```
$ kubectl get secrets -n library

NAME                         TYPE     DATA   AGE
jwt-secret                   Opaque   1      45s
meilisearch-secret           Opaque   1      45s
postgres-auth-secret         Opaque   1      45s
postgres-catalog-secret      Opaque   1      45s
postgres-reservation-secret  Opaque   1      45s
```

<!-- [FINAL] Again: `meilisearch-secret` is shown in the library namespace in this sample, contradicting the earlier statement that Meilisearch is in the `data` namespace. Fix one way or the other. -->

You can verify a specific value without exposing it in terminal history by decoding the base64-encoded data:

<!-- [COPY EDIT] "base64-encoded data" — hyphenated compound adjective before noun (CMOS 7.81). Good. -->
```bash
kubectl get secret postgres-catalog-secret -n library \
  -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d
```

<!-- [LINE EDIT] Sentence combines two retrievals into one long pipeline reference. Consider splitting the prose around it so the reader parses both shell commands. Minor. -->
The decoded value should match what `aws secretsmanager get-secret-value --secret-id rds!db-library-system-catalog --query SecretString --output text | jq -r '.password'` returns.

---

## Secret rotation

<!-- [STRUCTURAL] Rotation gap is important and well explained. Keep all four numbered steps. -->
When RDS rotates the master password — either on a schedule you configure in Secrets Manager or triggered manually — the sequence is:

1. Secrets Manager stores the new password and marks the old version as `AWSPREVIOUS`.
2. ESO polls Secrets Manager at the next `refreshInterval` boundary (up to one hour later with the configuration above).
3. ESO fetches the new value and overwrites the Kubernetes Secret's `data` field.
4. Pods that have already started and cached the old value in memory will fail on the next database operation that requires re-authentication.

<!-- [LINE EDIT] "Step 4 is the gap that naive rotation handling leaves open." → keep; strong lead. -->
Step 4 is the gap that naive rotation handling leaves open. A connection pool that has established connections will not re-read environment variables while it is running. The connection remains authenticated until either the old password is invalidated by the next rotation cycle or the pod restarts.

There are two approaches to closing this gap:

<!-- [COPY EDIT] "Rolling restart via a controller." — bolded lead, consistent. -->
<!-- [COPY EDIT] "three lines of Helm" → "three lines of Helm commands" for precision. -->
**Rolling restart via a controller.** The [Reloader](https://github.com/stakater/Reloader) project watches Kubernetes Secrets and ConfigMaps for changes and triggers a rolling restart of any Deployment that references them.[^5] Installing it is three lines of Helm:

```bash
helm repo add stakater https://stakater.github.io/stakater-charts
helm repo update
helm install reloader stakater/reloader -n kube-system
```

Once installed, annotate any Deployment you want to be restarted automatically when its referenced Secret changes:

```yaml
annotations:
  reloader.stakater.com/auto: "true"
```

With Reloader in place, the rotation sequence becomes: ESO writes the new Secret → Reloader detects the change → pods restart in a rolling fashion, pulling fresh environment variables → the new password is in effect within a few minutes.

<!-- [LINE EDIT] "**Application-level reconnection.** The cleaner long-term solution is to write database connection code that catches authentication errors and re-reads the password from the environment or from a filesystem mount before retrying." → good. -->
<!-- [COPY EDIT] "Reloader is the pragmatic choice; for a high-availability system where rolling restarts have a meaningful cost, application-level reconnection is worth implementing." — clear trade-off, good. -->
**Application-level reconnection.** The cleaner long-term solution is to write database connection code that catches authentication errors and re-reads the password from the environment or from a filesystem mount before retrying. This is more work but eliminates the dependency on an external restart controller. For a learning project, Reloader is the pragmatic choice; for a high-availability system where rolling restarts have a meaningful cost, application-level reconnection is worth implementing.

<!-- [LINE EDIT] "For this system, adding Reloader and the annotation to the three database-connected Deployments (auth, catalog, reservation) gives you complete automation" → keep. "complete automation" is a claim; justified. -->
For this system, adding Reloader and the annotation to the three database-connected Deployments (auth, catalog, reservation) gives you complete automation: a password rotation in Secrets Manager results in zero manual intervention and a seamless rolling update within minutes.

---

## What you have now

<!-- [STRUCTURAL] Good section closer with before/after summary table. Keep. -->
<!-- [LINE EDIT] "The `secretGenerator` block is gone. No placeholder values exist anywhere in the git repository." → strong punch opening. Keep. -->
The `secretGenerator` block is gone. No placeholder values exist anywhere in the git repository. The five Kubernetes Secrets that the application depends on are created and maintained by ESO, which reads live values from AWS Secrets Manager using a scoped IAM role that requires no static credentials. RDS passwords can rotate on a schedule without requiring any human action — the only question is whether you want automatic pod restarts (Reloader) or manual rolling restarts.

The shape of what changed:

| Before | After |
|--------|-------|
| `secretGenerator` with `REPLACE_WITH_*` literals | ESO `ExternalSecret` resources |
| Manual `aws secretsmanager get-secret-value` + paste | Automated poll every hour |
| Credentials in shell history on every deploy | No credentials in any shell command |
| Rotation requires manual Secret update + restart | Rotation is automatic; restart is optional via Reloader |
| Re-deploy to a new cluster requires reconstructing secrets | Re-deploy applies `external-secrets.yaml`; ESO fetches values |

<!-- [FINAL] "The next section addresses..." — good transition to 14.4. -->
The next section addresses the remaining production gap: Kafka connections currently use the plaintext listener on port 9092. Section 14.4 enables the TLS listener on MSK and updates the `KAFKA_BROKERS` environment variable in the production overlay to use port 9094.

---

[^1]: External Secrets Operator documentation: https://external-secrets.io/latest/
[^2]: terraform-aws-modules/iam IRSA submodule — External Secrets preset: https://registry.terraform.io/modules/terraform-aws-modules/iam/aws/latest/submodules/iam-role-for-service-accounts-eks
[^3]: AWS Secrets Manager — RDS managed passwords: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/rds-secrets-manager.html
[^4]: EKS IRSA documentation: https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html
[^5]: Stakater Reloader: https://github.com/stakater/Reloader

<!-- [FINAL] Footnote [^3] and [^4] are defined but not referenced in the body. [^1], [^2], [^5] are referenced. Add in-body references for [^3] and [^4] or remove them. -->
