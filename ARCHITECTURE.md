# Architecture

## System Overview

```
┌───────────────────────────────────────────────────────────┐
│                         Client                            │
└────────────────────────┬──────────────────────────────────┘
                         │ HTTP
                    ┌────▼────┐
                    │ Gateway │
                    │  :8080  │
                    └┬───┬───┬┘
          gRPC ┌─────┘   │   └─────┐ gRPC
         ┌─────▼──┐  ┌───▼───┐  ┌──▼───────────┐
         │  Auth  │  │Catalog│  │  Reservation  │
         │ :50051 │  │:50052 │  │    :50053     │
         └───┬────┘  └─┬───┬─┘  └──┬────────────┘
             │         │   │       │
        Postgres    Postgres   Postgres
                       │       │
                       └───┬───┘
                         Kafka
                           │
                     ┌─────▼────┐
                     │  Search  │
                     │  :50054  │
                     └────┬─────┘
                          │
                     Meilisearch
```

## Services

**Gateway** is the HTTP entry point (BFF pattern). It proxies requests to backend services over gRPC, renders HTML templates with HTMX, and manages user sessions.

**Auth** handles email/password registration, login, JWT issuance, and OAuth2 via Google.

**Catalog** manages the book registry (CRUD). It publishes `book.created`, `book.updated`, and `book.deleted` events to Kafka.

**Reservation** handles book reservations and returns. It consumes Kafka events to track book availability.

**Search** provides full-text search over the book catalog. It consumes Kafka events to keep a Meilisearch index in sync.

## Project Structure

```
.
├── services/              # Go microservices
│   ├── auth/              #   authentication (gRPC :50051)
│   ├── catalog/           #   book registry (gRPC :50052)
│   ├── gateway/           #   HTTP BFF (HTTP :8080)
│   ├── reservation/       #   reservations (gRPC :50053)
│   └── search/            #   full-text search (gRPC :50054)
├── proto/                 # Protobuf definitions (buf-managed)
├── gen/                   # Generated Go code from proto/
├── pkg/                   # Shared Go libraries
│   ├── auth/              #   JWT validation, gRPC auth interceptor
│   └── otel/              #   OpenTelemetry bootstrap helpers
├── deploy/                # Deployment configuration
│   ├── docker-compose.yml #   full local stack (16 containers)
│   ├── docker-compose.dev.yml  # hot-reload overrides
│   ├── k8s/               #   Kubernetes manifests
│   │   ├── base/          #     shared resources
│   │   └── overlays/      #     local / production variants
│   ├── grafana/           #   dashboards and datasource config
│   └── *.yaml             #   Prometheus, Tempo, Loki, OTel configs
├── terraform/             # AWS infrastructure (VPC, EKS, RDS, MSK, ECR)
├── docs/                  # Tutorial content (mdBook, 14 chapters)
├── Earthfile              # Build system (lint, test, integration-test, docker)
├── .github/workflows/     # CI/CD (pr.yml, main.yml)
└── go.work                # Go workspace (8 modules)
```

Each service follows the same internal layout:

```
services/<name>/
├── cmd/main.go                # entry point
├── internal/
│   ├── handler/               # gRPC handler (or HTTP for gateway)
│   ├── service/               # business logic
│   ├── repository/            # database access (GORM)
│   ├── model/                 # domain types
│   ├── kafka/                 # Kafka publisher/consumer (where applicable)
│   └── e2e/                   # end-to-end tests
├── migrations/                # SQL migrations (golang-migrate)
├── Dockerfile
├── Dockerfile.dev
├── Earthfile
└── go.mod
```

## Tech Stack

| Category | Technology |
|----------|------------|
| Language | Go |
| Architecture | Microservices, event-driven |
| RPC | gRPC + Protobuf |
| Database | PostgreSQL (one per service) |
| Message Broker | Apache Kafka (KRaft mode) |
| Search | Meilisearch |
| Containers | Docker, Docker Compose |
| Orchestration | Kubernetes (kind locally, EKS in production) |
| IaC | Terraform |
| Observability | OpenTelemetry, Grafana, Prometheus, Tempo, Loki |
| CI/CD | GitHub Actions + Earthly |
| Auth | JWT, OAuth2 (Google) |
