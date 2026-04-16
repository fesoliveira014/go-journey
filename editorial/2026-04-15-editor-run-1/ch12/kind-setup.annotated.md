# 12.1 Local Cluster with kind

<!-- [STRUCTURAL] Opening sets the stage well — quickly explains why kind matters before jumping into commands. -->
<!-- [COPY EDIT] "EKS, GKE, AKE" — AKE is not a standard acronym. Azure's managed Kubernetes is AKS (Azure Kubernetes Service). Please verify: this looks like a typo for AKS. -->
<!-- [LINE EDIT] "Cloud clusters (EKS, GKE, AKE) are the eventual target, but spinning one up just to test a YAML change is slow and expensive." → "Cloud clusters (EKS, GKE, AKS) are the eventual target, but spinning one up to test a YAML change is slow and expensive." (drop "just"). -->
Before writing a single Kubernetes manifest, you need somewhere to run it. Cloud clusters (EKS, GKE, AKE) are the eventual target, but spinning one up just to test a YAML change is slow and expensive. kind solves this by running a full Kubernetes cluster entirely inside Docker containers on your laptop.

---

## What kind Is

<!-- [COPY EDIT] Heading case: the book seems to use title case for section headings. "What kind Is" — "Is" capitalized is correct title case, but "kind" is a product name that upstream styles lowercase. That's the intended rendering. Keep. CMOS 8.159. -->
<!-- [LINE EDIT] "Each Kubernetes node — control plane or worker — is a Docker container running `containerd` and `kubelet` inside it." → "is a Docker container running `containerd` and `kubelet`." ("inside it" is redundant with "running"). -->
kind stands for **Kubernetes IN Docker**. Each Kubernetes node — control plane or worker — is a Docker container running `containerd` and `kubelet` inside it. The cluster looks and behaves like a real Kubernetes cluster from the outside: you interact with it through `kubectl`, pods schedule and run normally, Services and Ingresses work. But it takes seconds to create and seconds to destroy.

This makes kind the right tool for:

<!-- [LINE EDIT] List items are clean; no change. -->
- Iterating on Kubernetes manifests before committing them
- Running cluster-level integration tests in CI (the Earthfile targets from Chapter 10 use this)
- Experimenting with cluster configuration without touching a shared environment

<!-- [LINE EDIT] "kind is not a production tool." — good, blunt statement. -->
<!-- [COPY EDIT] "Use it where you use unit and integration tests — as a fast feedback loop." — check parallelism with previous sentence. OK. -->
kind is not a production tool. It does not support multi-node networking across machines, persistent volumes backed by real storage, or cloud provider integrations. Use it where you use unit and integration tests — as a fast feedback loop.

---

## Installation

You need two tools: kind itself and `kubectl`.

**kind:**

```bash
go install sigs.k8s.io/kind@latest
```

<!-- [LINE EDIT] "This places the `kind` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`." — good. -->
This places the `kind` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`. Alternatively, if you prefer a package manager:

<!-- [COPY EDIT] Please verify: kind v0.23.0 — the latest stable release at writing. kind has likely moved to newer versions (e.g., v0.24+). Pinning a specific version is good practice; note this may be outdated by publication. https://github.com/kubernetes-sigs/kind/releases -->
```bash
# macOS
brew install kind

# Linux (direct binary)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

**kubectl:**

```bash
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -sSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/kubectl
```

<!-- [COPY EDIT] Please verify: the `curl -sSL` on the inner dl.k8s.io/release/stable.txt endpoint returns the current stable kubectl version. This is the official pattern from Kubernetes docs. OK. -->
Verify both are available:

```bash
kind version
kubectl version --client
```

---

## Cluster Configuration

<!-- [STRUCTURAL] Good progression: command-line tools first, then cluster config. Order is logical. -->
<!-- [LINE EDIT] "A bare `kind create cluster` gives you a minimal single-node cluster." — good. -->
A bare `kind create cluster` gives you a minimal single-node cluster. For the library system you need two extras: a label on the control-plane node that signals it can accept Ingress traffic, and port mappings so that HTTP and HTTPS requests from `localhost` reach the cluster.

Create `kind-config.yaml` at the root of the repository:

<!-- [COPY EDIT] Please verify: apiVersion `kind.x-k8s.io/v1alpha4` — kind's config API is still v1alpha4 as of the latest releases. Confirm via https://kind.sigs.k8s.io/docs/user/configuration/ -->
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
```

What each part does:

<!-- [LINE EDIT] "the NGINX Ingress Controller that you install in the next step uses a node selector targeting this label" — good sentence. -->
<!-- [COPY EDIT] "NGINX Ingress Controller" — product capitalization; CMOS 8.152 match upstream style. NGINX uses all-caps. Good. -->
- `node-labels: "ingress-ready=true"` — the NGINX Ingress Controller that you install in the next step uses a node selector targeting this label. Without the label, the Ingress Controller pod will not schedule.
- `extraPortMappings` — Docker normally isolates the container's network. These mappings punch through: traffic arriving on `localhost:80` or `localhost:443` on your machine is forwarded into the cluster node. This is what makes `http://localhost/api/catalog` work from your browser.

---

## Creating the Cluster

```bash
kind create cluster --config kind-config.yaml --name library
```

<!-- [LINE EDIT] "kind pulls the node image on first run (around 700 MB)" — verify size claim; 700 MB is approximately right for a recent kind node image. -->
<!-- [COPY EDIT] "kubeadm" lowercase is correct. -->
kind pulls the node image on first run (around 700 MB), creates the Docker container, bootstraps kubeadm inside it, and writes a kubeconfig entry named `kind-library`. Subsequent runs skip the image pull and take about 30 seconds.

Verify the cluster is reachable:

```bash
kubectl cluster-info --context kind-library
kubectl get nodes
```

Expected output:

<!-- [COPY EDIT] "v1.30.x" — uses .x as a floating version. Pinning a major.minor like this is fine for documentation. Please verify: kind v0.23.0 ships with Kubernetes 1.30.x by default, which is accurate. https://github.com/kubernetes-sigs/kind/releases/tag/v0.23.0 -->
```
NAME                    STATUS   ROLES           AGE   VERSION
library-control-plane   Ready    control-plane   1m    v1.30.x
```

If you have multiple clusters or contexts, be explicit about which one you are targeting:

```bash
kubectl --context kind-library get pods -A
```

---

## Installing the NGINX Ingress Controller

<!-- [LINE EDIT] "Kubernetes itself does not route external HTTP traffic — that is the job of an Ingress Controller." — fine. -->
<!-- [COPY EDIT] "Ingress Controller" — capitalization inconsistency. Kubernetes docs typically lowercase "Ingress controller" (only "Ingress" the API resource is capitalized). Consider normalizing to "Ingress controller" throughout. -->
Kubernetes itself does not route external HTTP traffic — that is the job of an Ingress Controller. For kind, the NGINX project ships a ready-made manifest tuned for this exact setup[^2]:

<!-- [COPY EDIT] Please verify URL: https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml — this is the canonical URL from the ingress-nginx project's kind guide. -->
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```

<!-- [LINE EDIT] "The manifest creates a `NodePort` Service that binds to the ports you mapped above." — good. -->
The manifest creates a `NodePort` Service that binds to the ports you mapped above. Wait for the controller pod to become ready before proceeding:

```bash
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

`kubectl wait` blocks until the condition is met or the timeout expires. If the pod is not ready after 90 seconds, check the pod events:

```bash
kubectl -n ingress-nginx describe pod -l app.kubernetes.io/component=controller
```

---

## The `kind load` Gotcha

<!-- [STRUCTURAL] Well-earned H2: this is the #1 source of confusion for newcomers. Dedicated section is right. -->
<!-- [LINE EDIT] "This trips up almost everyone the first time." — good conversational opener; tutor tone. -->
<!-- [COPY EDIT] "VMs" — abbreviation for "virtual machines," commonly understood; no expansion needed. -->
This trips up almost everyone the first time. kind nodes are Docker containers, not VMs — they run `containerd` internally and do not share the Docker daemon on your host. When you build an image with `docker build` or `earthly +docker`, that image lives in your host Docker daemon's image store. The kind node cannot see it.

<!-- [LINE EDIT] "If you try to deploy a pod referencing an image that only exists on the host, Kubernetes will attempt to pull it from a registry and fail (or use a cached version that is out of date)." — well-explained failure mode. -->
<!-- [COPY EDIT] "`ImagePullBackOff`" — inline code is correct (it's a Kubernetes pod status value). -->
If you try to deploy a pod referencing an image that only exists on the host, Kubernetes will attempt to pull it from a registry and fail (or use a cached version that is out of date). The symptom is pods stuck in `ImagePullBackOff`.

The fix is `kind load`:

```bash
kind load docker-image library-system/catalog:latest --name library
```

This copies the image from your host Docker daemon into the containerd store inside the kind node. After this command, any pod in the cluster can use `library-system/catalog:latest` without a registry pull.

<!-- [LINE EDIT] "You must also set `imagePullPolicy: Never` (or `IfNotPresent`) in your pod spec" — clear and correct. -->
You must also set `imagePullPolicy: Never` (or `IfNotPresent`) in your pod spec, otherwise Kubernetes will try to pull from a registry anyway:

```yaml
containers:
  - name: catalog
    image: library-system/catalog:latest
    imagePullPolicy: Never
```

<!-- [COPY EDIT] Note: app-manifests.md uses `IfNotPresent`, not `Never`, for the same image. Both work; cross-reference or standardize to one. Minor. -->
Run `kind load` again whenever you rebuild an image. There is no automatic sync.

---

## Tying Back to Chapter 10: Earthly Builds + kind Load

<!-- [COPY EDIT] Heading: "Tying Back to Chapter 10: Earthly Builds + kind Load" — CMOS 8.159 capitalize after colon only if it's a complete sentence. "Earthly Builds + kind Load" is a subtitle/phrase, so lowercase "kind Load" would be CMOS-compliant if we treat post-colon as subtitle in title case, which is what's happening here. Style is fine. -->
<!-- [STRUCTURAL] Useful cross-reference to Chapter 10; reinforces the book's build-and-deploy arc. Keep. -->
In Chapter 10 you built container images with `earthly +docker`. That target produces tagged Docker images in the host daemon. The workflow for local Kubernetes development is:

<!-- [COPY EDIT] Comment-style formatting consistent within code block. -->
```bash
# 1. Build all service images
earthly +docker

# 2. Load each into the kind cluster
kind load docker-image library-system/auth:latest      --name library
kind load docker-image library-system/catalog:latest   --name library
kind load docker-image library-system/reservation:latest --name library
kind load docker-image library-system/search:latest     --name library
kind load docker-image library-system/gateway:latest   --name library

# 3. Apply manifests (covered in upcoming sections)
kubectl apply -f k8s/
```

<!-- [LINE EDIT] "This sequence — build, load, apply — replaces the inner loop you would otherwise spend waiting for a CI pipeline or a remote registry push." — clear; keep. -->
<!-- [FINAL] "on a laptop with warm Earthly caches and a running cluster, the full cycle takes under a minute." — good closer, motivates the reader. -->
This sequence — build, load, apply — replaces the inner loop you would otherwise spend waiting for a CI pipeline or a remote registry push. On a laptop with warm Earthly caches and a running cluster, the full cycle takes under a minute.

---

## Cleanup

When you are done, delete the cluster entirely:

```bash
kind delete cluster --name library
```

<!-- [LINE EDIT] "This removes the Docker container, all pods, all persistent volumes, and the kubeconfig entry. There is no partial teardown." — good. -->
<!-- [COPY EDIT] "30 seconds" — consistent with the earlier claim. Good. -->
This removes the Docker container, all pods, all persistent volumes, and the kubeconfig entry. There is no partial teardown. Re-creating takes the same 30 seconds as the original bootstrap.

---

## Summary

<!-- [STRUCTURAL] Summary table is a nice reference stub. Keep. -->
| Command | What it does |
|---|---|
| `kind create cluster --config kind-config.yaml --name library` | Bootstrap a new cluster |
| `kubectl cluster-info --context kind-library` | Verify the cluster is reachable |
| `kubectl apply -f ingress-nginx/...` | Install the NGINX Ingress Controller |
| `kind load docker-image <image> --name library` | Copy a local image into the cluster |
| `kind delete cluster --name library` | Tear down and remove everything |

<!-- [LINE EDIT] "The cluster you just created is where everything in the rest of this chapter will run." — good pointer to next section. -->
The cluster you just created is where everything in the rest of this chapter will run. The next section introduces Kubernetes manifests — Deployments, Services, and ConfigMaps — and how to write them for the library services.

---

[^1]: kind Quick Start: https://kind.sigs.k8s.io/docs/user/quick-start/
[^2]: kind Ingress: https://kind.sigs.k8s.io/docs/user/ingress/
[^3]: kind Loading Images: https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster
