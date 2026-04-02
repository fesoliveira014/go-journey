# README.md Design Spec

## Goal

Replace the 16-line stub `README.md` with a comprehensive project README aimed at tutorial readers. It should orient them in the codebase, get them running locally in minutes, and serve as a command reference card for build, test, and deploy operations.

## Audience

Tutorial readers following the book chapter-by-chapter who need to understand the project structure and get the code running at each stage.

## Sections (in order)

### 1. Title + Description

One-line: "A microservices-based library management system built in Go." Follow with 2-3 sentences explaining that this is a learning project covering microservices architecture, containerization, Kubernetes, observability, and CI/CD.

### 2. Architecture Diagram

Text-based (ASCII/Unicode) box diagram showing:
- Client at the top
- Gateway (:8080) receiving HTTP
- Auth (:50051), Catalog (:50052), Reservation (:50053) connected via gRPC
- Kafka connecting Catalog and Reservation to Search (:50054)
- Three Postgres instances (one per backend service)
- Meilisearch under Search

```
┌─────────────────────────────────────────────────────────┐
│                        Client                           │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP
                    ┌────▼────┐
                    │ Gateway │
                    │  :8080  │
                    └──┬──┬──┬┘
           gRPC ┌──────┘  │  └──────┐ gRPC
          ┌─────▼──┐  ┌───▼───┐  ┌──▼──────────┐
          │  Auth  │  │Catalog│  │ Reservation  │
          │ :50051 │  │:50052 │  │   :50053     │
          └───┬────┘  └─┬──┬──┘  └──┬───────────┘
              │         │  │        │
         Postgres    Postgres  Postgres
                       │        │
                       └──┬─────┘
                        Kafka
                          │
                    ┌─────▼────┐
                    │  Search  │
                    │  :50054  │
                    └────┬─────┘
                         │
                    Meilisearch
```

### 3. Project Structure

Annotated directory tree of top-level layout:

```
.
├── services/          # Go microservices
│   ├── auth/          #   email/password + OAuth2 authentication
│   ├── catalog/       #   book CRUD, Kafka event publishing
│   ├── gateway/       #   HTTP BFF, templates, HTMX
│   ├── reservation/   #   book reservations, Kafka consumer
│   └── search/        #   full-text search via Meilisearch
├── proto/             # Protobuf definitions (buf-managed)
├── gen/               # Generated Go code from proto/
├── pkg/               # Shared Go libraries
│   ├── auth/          #   JWT validation, gRPC interceptor
│   └── otel/          #   OpenTelemetry bootstrap helpers
├── deploy/            # Deployment configs
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yml
│   ├── k8s/           #   Kubernetes manifests (base + overlays)
│   └── ...            #   Grafana, Prometheus, Tempo, Loki configs
├── terraform/         # AWS infrastructure (EKS, RDS, MSK, ECR)
├── docs/              # Tutorial content (mdBook)
├── Earthfile          # Build system (lint, test, docker)
└── go.work            # Go workspace (7 modules)
```

### 4. Prerequisites

**Tiered approach:**

**Required (to run the quick start):**
- Go 1.26+
- Docker and Docker Compose

**Later chapters:**
- Earthly (Ch9 — CI/CD and reproducible builds)
- buf (Ch2 — protobuf code generation)
- grpcurl (Ch2 — testing gRPC services)
- kubectl (Ch11 — Kubernetes)
- kind (Ch11 — local K8s cluster)
- Terraform (Ch12 — cloud deployment)

### 5. Quick Start

Steps:
1. Clone the repo
2. `cd deploy && docker compose up --build`
3. Verify gateway: `curl http://localhost:8080/healthz`
4. Verify gRPC: `grpcurl -plaintext localhost:50052 catalog.v1.CatalogService/ListBooks`
5. Brief note about hot reload with `docker-compose.dev.yml`

### 6. Build & Test

Earthly commands in a compact format:

| Command | What it does |
|---------|-------------|
| `earthly +ci` | Lint + test all services |
| `earthly +lint` | golangci-lint all services |
| `earthly +test` | Unit tests all services |
| `earthly +integration-test` | Integration tests (Testcontainers) |
| `earthly +docker` | Build all Docker images |

Plus the per-service variant: `earthly ./services/catalog+test`

### 7. Kubernetes (Local)

```bash
kind create cluster --config deploy/k8s/kind-config.yaml
kubectl apply -k deploy/k8s/overlays/local
kubectl get pods -n library
```

Brief verification step. Link to Ch11 for details.

### 8. Cloud Deployment

2-3 sentences: the `terraform/` directory contains AWS infrastructure (VPC, EKS, RDS, MSK, ECR). The production Kustomize overlay at `deploy/k8s/overlays/production/` configures the app for AWS. Link to Ch12 and Ch13 for the full walkthrough.

### 9. Observability

Brief mention of the Grafana stack (Grafana, Prometheus, Tempo, Loki) included in Docker Compose. Access Grafana at `http://localhost:3000`. Link to Ch8.

### 10. Tutorial

Link to `docs/src/SUMMARY.md`. Compact chapter list:

| Chapter | Topic |
|---------|-------|
| 1 | Go Foundations |
| 2 | First Microservice — Catalog |
| 3 | Containerization |
| 4 | Authentication |
| 5 | Gateway & Frontend |
| 6 | Event-Driven Architecture |
| 7 | Full-Text Search |
| 8 | Observability |
| 9 | CI/CD |
| 10 | Testing Strategies |
| 11 | Kubernetes |
| 12 | Cloud Deployment |
| 13 | Production Hardening |

### 11. License

Include only if a LICENSE file exists in the repo. Otherwise omit.

## Constraints

- Target ~250-350 lines
- No teaching prose — just enough context to orient and run
- All commands must be accurate against actual file paths and tool names
- Do not duplicate tutorial content; link to it
