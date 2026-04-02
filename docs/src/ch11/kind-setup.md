# 11.2 Local Cluster with kind

Before writing a single Kubernetes manifest, you need somewhere to run it. Cloud clusters (EKS, GKE, AKE) are the eventual target, but spinning one up just to test a YAML change is slow and expensive. kind solves this by running a full Kubernetes cluster entirely inside Docker containers on your laptop.

---

## What kind Is

kind stands for **Kubernetes IN Docker**. Each Kubernetes node — control plane or worker — is a Docker container running `containerd` and `kubelet` inside it. The cluster looks and behaves like a real Kubernetes cluster from the outside: you interact with it through `kubectl`, pods schedule and run normally, Services and Ingresses work. But it takes seconds to create and seconds to destroy.

This makes kind the right tool for:

- Iterating on Kubernetes manifests before committing them
- Running cluster-level integration tests in CI (the Earthfile targets from Chapter 9 use this)
- Experimenting with cluster configuration without touching a shared environment

kind is not a production tool. It does not support multi-node networking across machines, persistent volumes backed by real storage, or cloud provider integrations. Use it where you use unit and integration tests — as a fast feedback loop.

---

## Installation

You need two tools: kind itself and `kubectl`.

**kind:**

```bash
go install sigs.k8s.io/kind@latest
```

This places the `kind` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`. Alternatively, if you prefer a package manager:

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

Verify both are available:

```bash
kind version
kubectl version --client
```

---

## Cluster Configuration

A bare `kind create cluster` gives you a minimal single-node cluster. For the library system you need two extras: a label on the control-plane node that signals it can accept Ingress traffic, and port mappings so that HTTP and HTTPS requests from `localhost` reach the cluster.

Create `kind-config.yaml` at the root of the repository:

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

- `node-labels: "ingress-ready=true"` — the NGINX Ingress Controller that you install in the next step uses a node selector targeting this label. Without the label, the Ingress Controller pod will not schedule.
- `extraPortMappings` — Docker normally isolates the container's network. These mappings punch through: traffic arriving on `localhost:80` or `localhost:443` on your machine is forwarded into the cluster node. This is what makes `http://localhost/api/catalog` work from your browser.

---

## Creating the Cluster

```bash
kind create cluster --config kind-config.yaml --name library
```

kind pulls the node image on first run (around 700 MB), creates the Docker container, bootstraps kubeadm inside it, and writes a kubeconfig entry named `kind-library`. Subsequent runs skip the image pull and take about 30 seconds.

Verify the cluster is reachable:

```bash
kubectl cluster-info --context kind-library
kubectl get nodes
```

Expected output:

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

Kubernetes itself does not route external HTTP traffic — that is the job of an Ingress Controller. For kind, the NGINX project ships a ready-made manifest tuned for this exact setup[^2]:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```

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

This trips up almost everyone the first time. kind nodes are Docker containers, not VMs — they run `containerd` internally and do not share the Docker daemon on your host. When you build an image with `docker build` or `earthly +docker`, that image lives in your host Docker daemon's image store. The kind node cannot see it.

If you try to deploy a pod referencing an image that only exists on the host, Kubernetes will attempt to pull it from a registry and fail (or use a cached version that is out of date). The symptom is pods stuck in `ImagePullBackOff`.

The fix is `kind load`:

```bash
kind load docker-image library-system/catalog:latest --name library
```

This copies the image from your host Docker daemon into the containerd store inside the kind node. After this command, any pod in the cluster can use `library-system/catalog:latest` without a registry pull.

You must also set `imagePullPolicy: Never` (or `IfNotPresent`) in your pod spec, otherwise Kubernetes will try to pull from a registry anyway:

```yaml
containers:
  - name: catalog
    image: library-system/catalog:latest
    imagePullPolicy: Never
```

Run `kind load` again whenever you rebuild an image. There is no automatic sync.

---

## Tying Back to Chapter 9: Earthly Builds + kind Load

In Chapter 9 you built container images with `earthly +docker`. That target produces tagged Docker images in the host daemon. The workflow for local Kubernetes development is:

```bash
# 1. Build all service images
earthly +docker

# 2. Load each into the kind cluster
kind load docker-image library-system/auth:latest      --name library
kind load docker-image library-system/catalog:latest   --name library
kind load docker-image library-system/reservation:latest --name library
kind load docker-image library-system/user:latest      --name library
kind load docker-image library-system/gateway:latest   --name library

# 3. Apply manifests (covered in upcoming sections)
kubectl apply -f k8s/
```

This sequence — build, load, apply — replaces the inner loop you would otherwise spend waiting for a CI pipeline or a remote registry push. On a laptop with warm Earthly caches and a running cluster, the full cycle takes under a minute.

---

## Cleanup

When you are done, delete the cluster entirely:

```bash
kind delete cluster --name library
```

This removes the Docker container, all pods, all persistent volumes, and the kubeconfig entry. There is no partial teardown. Re-creating takes the same 30 seconds as the original bootstrap.

---

## Summary

| Command | What it does |
|---|---|
| `kind create cluster --config kind-config.yaml --name library` | Bootstrap a new cluster |
| `kubectl cluster-info --context kind-library` | Verify the cluster is reachable |
| `kubectl apply -f ingress-nginx/...` | Install the NGINX Ingress Controller |
| `kind load docker-image <image> --name library` | Copy a local image into the cluster |
| `kind delete cluster --name library` | Tear down and remove everything |

The cluster you just created is where everything in the rest of this chapter will run. The next section introduces Kubernetes manifests — Deployments, Services, and ConfigMaps — and how to write them for the library services.

---

[^1]: kind Quick Start: https://kind.sigs.k8s.io/docs/user/quick-start/
[^2]: kind Ingress: https://kind.sigs.k8s.io/docs/user/ingress/
[^3]: kind Loading Images: https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster
