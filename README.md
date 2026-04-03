# go-journey: A Hands-On Guide to Go Microservices

A 14-chapter tutorial that teaches microservices architecture, containerization, orchestration, observability, and CI/CD by building a library management system in Go. The book walks through every layer of a production-grade system — from writing your first gRPC service to deploying on AWS with Terraform.

## The Book

The tutorial is written as an [mdBook](https://rust-lang.github.io/mdBook/). Each chapter builds on the previous one.

| # | Chapter | Topics |
|---|---------|--------|
| 1 | [Go Foundations](docs/src/ch01/index.md) | Project setup, language essentials, HTTP server, testing |
| 2 | [First Microservice](docs/src/ch02/index.md) | Protobuf, gRPC, PostgreSQL, repository pattern, service layer |
| 3 | [Containerization](docs/src/ch03/index.md) | Docker, Dockerfiles, Docker Compose, dev workflow |
| 4 | [Authentication](docs/src/ch04/index.md) | JWT, bcrypt, OAuth2 with Google, gRPC interceptors |
| 5 | [Gateway & Frontend](docs/src/ch05/index.md) | BFF pattern, HTML templates, HTMX, sessions, admin CRUD |
| 6 | [Admin & Developer Tooling](docs/src/ch06/index.md) | Admin CLI, admin dashboard, catalog seed CLI |
| 7 | [Event-Driven Architecture](docs/src/ch07/index.md) | Kafka, reservation service, event consumers |
| 8 | [Full-Text Search](docs/src/ch08/index.md) | Meilisearch, event-driven indexing, search UI |
| 9 | [Observability](docs/src/ch09/index.md) | OpenTelemetry, structured logging, Grafana stack |
| 10 | [CI/CD](docs/src/ch10/index.md) | Earthly, GitHub Actions, linting, image publishing |
| 11 | [Testing Strategies](docs/src/ch11/index.md) | Unit tests, Testcontainers, gRPC testing, e2e tests |
| 12 | [Kubernetes](docs/src/ch12/index.md) | kind, manifests, Kustomize, local deployment |
| 13 | [Cloud Deployment](docs/src/ch13/index.md) | Terraform, VPC, EKS, RDS, MSK, ECR, CI/CD pipeline |
| 14 | [Production Hardening](docs/src/ch14/index.md) | DNS, TLS, secrets management, Kafka encryption |

Full table of contents: [docs/src/SUMMARY.md](docs/src/SUMMARY.md)

### Building the Book

```bash
# Install mdbook and the mermaid preprocessor (requires Rust/Cargo)
cargo install mdbook mdbook-mermaid

# Build the book (output goes to site/)
cd docs
mdbook build

# Or serve with live reload at http://localhost:3001
mdbook serve --open
```

The book uses [Mermaid](https://mermaid.js.org/) for architecture diagrams. The `mdbook-mermaid` preprocessor renders them automatically during build.

## The Project

The companion source code is a fully working library management system: five Go microservices communicating over gRPC and Kafka, backed by PostgreSQL and Meilisearch, deployable via Docker Compose, Kubernetes, or Terraform on AWS.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the system design, service descriptions, and project structure.

### Prerequisites

#### Required

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.26+ | Build and run services |
| [Docker](https://docs.docker.com/get-docker/) | 20.10+ | Container runtime |
| [Docker Compose](https://docs.docker.com/compose/install/) | v2+ | Local development stack |

#### Later Chapters

| Tool | Chapter | Purpose |
|------|---------|---------|
| [buf](https://buf.build/docs/installation) | 2 | Protobuf code generation |
| [grpcurl](https://github.com/fullstorydev/grpcurl) | 2 | Testing gRPC services from the CLI |
| [Earthly](https://earthly.dev/get-earthly) | 10 | Reproducible builds (lint, test, docker) |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | 12 | Kubernetes CLI |
| [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) | 12 | Local Kubernetes cluster |
| [Terraform](https://developer.hashicorp.com/terraform/install) | 13 | AWS infrastructure provisioning |
| [mdbook](https://rust-lang.github.io/mdBook/) | -- | Build and serve the tutorial book |

### Quick Start

**1. Clone and start the stack:**

```bash
git clone https://github.com/fesoliveira014/go-journey.git
cd go-journey/deploy
docker compose up --build
```

This starts 16 containers: five application services, three PostgreSQL instances, Kafka, Meilisearch, and the Grafana observability stack (Grafana, Prometheus, Tempo, Loki, Promtail, OTel Collector).

**2. Verify the gateway:**

```bash
curl http://localhost:8080/healthz
```

Expected: `{"status":"ok"}`

**3. Create an admin account:**

```bash
DATABASE_URL="postgres://postgres:postgres@localhost:5434/auth?sslmode=disable" \
  go run services/auth/cmd/admin/main.go \
    --email admin@example.com --password secret --name "Admin"
```

**4. Seed the catalog with sample books:**

```bash
go run services/catalog/cmd/seed/main.go \
  --auth-addr localhost:50051 --catalog-addr localhost:50052 \
  --email admin@example.com --password secret
```

**5. Access the UI:**

Open [http://localhost:8080](http://localhost:8080) in your browser. Log in with `admin@example.com` / `secret` to access the admin dashboard at `/admin`.

**6. Verify gRPC (optional, requires grpcurl):**

```bash
grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks
```

### Hot Reload

For development with live reloading (via [Air](https://github.com/air-verse/air)):

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

Changes to Go files under `services/` will trigger automatic rebuilds.

### Build & Test

#### With Earthly (Chapter 10+)

| Command | Description |
|---------|-------------|
| `earthly +ci` | Lint + unit test all services |
| `earthly +lint` | Run golangci-lint on all services |
| `earthly +test` | Run unit tests for all services |
| `earthly +integration-test` | Run integration tests (uses Testcontainers) |
| `earthly +docker` | Build all Docker images |

Run targets for a single service:

```bash
earthly ./services/catalog+test
earthly ./services/auth+lint
earthly ./services/catalog+integration-test
```

#### With Go directly

```bash
# Run tests for a single service
cd services/catalog
go test ./...

# Run all tests via the workspace
go test ./services/catalog/... ./services/auth/... ./services/gateway/...
```

### Kubernetes (Local)

Create a local cluster with [kind](https://kind.sigs.k8s.io/) and deploy:

```bash
# Create cluster
kind create cluster --config deploy/k8s/kind-config.yaml

# Deploy the full stack
kubectl apply -k deploy/k8s/overlays/local

# Verify
kubectl get pods -n library
kubectl get pods -n data
kubectl get pods -n messaging
```

All pods should reach `Running` status within a few minutes. The gateway is exposed via an Ingress on port 80.

See [Chapter 12](docs/src/ch12/index.md) for the full walkthrough.

### Cloud Deployment (AWS)

The `terraform/` directory provisions production infrastructure on AWS:

- **VPC** with public/private subnets across 2 AZs
- **EKS** managed Kubernetes cluster
- **RDS** PostgreSQL instances (one per service)
- **MSK** managed Kafka cluster
- **ECR** container registry (one repo per service)
- **Route 53 + ACM** for DNS and TLS (Chapter 14)
- **External Secrets Operator** for secrets management (Chapter 14)

The production Kustomize overlay at `deploy/k8s/overlays/production/` configures the application for AWS (ECR images, RDS endpoints, MSK brokers, ALB ingress).

```bash
cd terraform
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

See [Chapter 13](docs/src/ch13/index.md) and [Chapter 14](docs/src/ch14/index.md) for the full walkthrough.

### Observability

The Docker Compose stack includes a full Grafana observability suite:

| Tool | URL | Purpose |
|------|-----|---------|
| Grafana | [localhost:3000](http://localhost:3000) | Dashboards, log/trace exploration |
| Prometheus | [localhost:9090](http://localhost:9090) | Metrics collection |
| Tempo | (internal) | Distributed tracing backend |
| Loki | (internal) | Log aggregation backend |

All services are instrumented with OpenTelemetry. Traces, metrics, and structured logs are correlated via trace IDs.

See [Chapter 9](docs/src/ch09/index.md) for details.

## Security

All Docker containers run as a non-root user (`app`). Kubernetes manifests include pod and container security contexts (`runAsNonRoot`, `readOnlyRootFilesystem`, `drop: ["ALL"]` capabilities). Secrets are managed via Kustomize `secretGenerator` (local) and External Secrets Operator with AWS Secrets Manager (production). See [Chapter 14](docs/src/ch14/index.md) for the full production hardening walkthrough.
