# Summary

<!-- [STRUCTURAL] As an mdBook TOC this is functional, but consider whether a top-level "Part" grouping (e.g., Part I: Foundations [Ch 1-2], Part II: Containers & Auth [Ch 3-5], Part III: Async & Search [Ch 6-8], Part IV: Production [Ch 9-14]) would help readers see the arc. mdBook supports `# Part Title` headings between list items. -->
<!-- [STRUCTURAL] Indentation is inconsistent: Chapter 1's sub-items use 4 spaces (lines 5-8) while Chapters 2-14 use 2 spaces. mdBook tolerates both but mixed nesting can produce subtle render differences. Pick one (mdBook docs use 2 spaces) and apply uniformly. -->
<!-- [STRUCTURAL] Chapter title pattern is inconsistent: most chapters are bare nouns ("Containerization", "Authentication", "Kubernetes") while a few embed scope qualifiers ("Observability with OpenTelemetry", "CI/CD with GitHub Actions & Earthly"). Decide whether tool names belong in chapter titles or only in section titles. -->

- [Introduction](./introduction.md)
- [Chapter 1: Go Foundations](./ch01/index.md)
    - [1.1 Project Setup](./ch01/project-setup.md)
    - [1.2 Go Language Essentials](./ch01/go-basics.md)
    - [1.3 Building an HTTP Server](./ch01/http-server.md)
    - [1.4 Testing in Go](./ch01/testing.md)
<!-- [COPY EDIT] Lines 5-8 use 4-space indent; all subsequent chapters use 2-space. Normalize to 2 spaces. -->
<!-- [LINE EDIT] Section parallelism: 1.1 (noun), 1.2 (noun), 1.3 (gerund), 1.4 (gerund). Choose one form across the chapter. -->
- [Chapter 2: First Microservice — Catalog](./ch02/index.md)
<!-- [COPY EDIT] CMOS 6.85: em dash with no spaces. Spaced em dash is a typographic variant; flag for project-wide consistency. -->
  - [2.1 Protocol Buffers & gRPC](./ch02/protobuf-grpc.md)
<!-- [COPY EDIT] Ampersand "&" in titles (2.1, 2.2, 2.4, Ch 5/6 titles, 5.2, 10.4, 10.5). CMOS 10.10 prefers "and" in titles unless the ampersand is part of a brand. -->
  - [2.2 PostgreSQL & Migrations](./ch02/postgresql-migrations.md)
  - [2.3 The Repository Pattern with GORM](./ch02/repository-pattern.md)
  - [2.4 Service Layer & Business Logic](./ch02/service-layer.md)
  - [2.5 Wiring It All Together](./ch02/wiring.md)
<!-- [LINE EDIT] "Wiring It All Together" is colloquial. Consider "Assembling the Service" or keep deliberately. -->
- [Chapter 3: Containerization](./ch03/index.md)
  - [3.1 Docker Fundamentals](./ch03/docker-fundamentals.md)
  - [3.2 Writing Dockerfiles](./ch03/writing-dockerfiles.md)
  - [3.3 Docker Compose](./ch03/docker-compose.md)
  - [3.4 Development Workflow](./ch03/dev-workflow.md)
<!-- [STRUCTURAL] 3.1/3.3/3.4 are nouns, 3.2 is gerund. Rename 3.2 to "Dockerfile Authoring" or shift others to gerunds. -->
- [Chapter 4: Authentication](./ch04/index.md)
  - [4.1 Authentication Fundamentals](./ch04/auth-fundamentals.md)
  - [4.2 The Auth Service](./ch04/auth-service.md)
  - [4.3 OAuth2 with Google](./ch04/oauth2.md)
<!-- [COPY EDIT] CLAUDE.md and introduction.md say "Gmail"; TOC says "Google". Gmail is the mail product; Google is the OAuth2 provider. "Google" is correct. -->
  - [4.4 Protecting Services with Interceptors](./ch04/interceptors.md)
- [Chapter 5: Gateway & Frontend](./ch05/index.md)
  - [5.1 The BFF Pattern](./ch05/bff-pattern.md)
<!-- [STRUCTURAL] Expand jargon on first appearance: "5.1 The Backend-for-Frontend (BFF) Pattern". -->
  - [5.2 Templates & HTMX](./ch05/templates-htmx.md)
  - [5.3 Session Management](./ch05/session-management.md)
  - [5.4 Admin CRUD](./ch05/admin-crud.md)
<!-- [STRUCTURAL] Confirm scope split with Ch 6 (admin tooling). If 5.4 = web, retitle "Admin Web Pages". -->
- [Chapter 6: Admin & Developer Tooling](./ch06/index.md)
  - [6.1 Admin CLI](./ch06/admin-cli.md)
  - [6.2 Admin Dashboard](./ch06/admin-dashboard.md)
  - [6.3 Catalog Seed CLI](./ch06/seed-cli.md)
  - [6.4 Putting It Together](./ch06/putting-it-together.md)
<!-- [LINE EDIT] Mirrors 2.5 "Wiring It All Together". Pick one phrase. -->
- [Chapter 7: Event-Driven Architecture](./ch07/index.md)
  - [7.1 Event-Driven Architecture](./ch07/event-driven-architecture.md)
<!-- [STRUCTURAL] 7.1 duplicates the chapter title verbatim. Rename: "7.1 Event-Driven Fundamentals" or "7.1 Why Events?". -->
  - [7.2 Reservation Service](./ch07/reservation-service.md)
  - [7.3 Kafka Consumer](./ch07/kafka-consumer.md)
  - [7.4 Reservation UI](./ch07/reservation-ui.md)
- [Chapter 8: Full-Text Search](./ch08/index.md)
<!-- [COPY EDIT] CMOS 7.89: "full-text" hyphenated before noun — correct. -->
  - [8.1 Catalog Event Publishing](./ch08/catalog-events.md)
  - [8.2 Search Service](./ch08/search-service.md)
  - [8.3 Meilisearch Integration](./ch08/meilisearch.md)
<!-- [COPY EDIT] "Meilisearch" — vendor canonical (one capital, one word). Correct. -->
  - [8.4 Search UI](./ch08/search-ui.md)
- [Chapter 9: Observability with OpenTelemetry](./ch09/index.md)
  - [9.1 OpenTelemetry Fundamentals](./ch09/otel-fundamentals.md)
  - [9.2 Instrumenting Go Services](./ch09/instrumentation.md)
  - [9.3 Structured Logging with slog](./ch09/structured-logging.md)
<!-- [COPY EDIT] `slog` is a Go package identifier; consider backticks: "Structured Logging with `slog`". -->
  - [9.4 The Grafana Stack](./ch09/grafana-stack.md)
  - [9.5 Sidecar Collector Pattern](./ch09/sidecar-pattern.md)
- [Chapter 10: CI/CD with GitHub Actions & Earthly](./ch10/index.md)
<!-- [COPY EDIT] "CI/CD" slash acceptable (CMOS 6.106). -->
  - [10.1 CI/CD Fundamentals](./ch10/cicd-fundamentals.md)
  - [10.2 The Earthly Build System](./ch10/earthly.md)
  - [10.3 GitHub Actions Workflows](./ch10/github-actions.md)
  - [10.4 Linting & Code Quality](./ch10/linting.md)
  - [10.5 Image Publishing & Versioning](./ch10/image-publishing.md)
- [Chapter 11: Testing Strategies](./ch11/index.md)
  - [11.1 Unit Testing Patterns](./ch11/unit-testing-patterns.md)
  - [11.2 Integration Testing with Testcontainers](./ch11/integration-testing-postgres.md)
<!-- [COPY EDIT] Filename `integration-testing-postgres.md` diverges from title "Testcontainers". Rename file or title. -->
  - [11.3 gRPC Testing with bufconn](./ch11/grpc-testing.md)
  - [11.4 Kafka Testing](./ch11/kafka-testing.md)
  - [11.5 Service-Level End-to-End Tests](./ch11/e2e-testing.md)
<!-- [COPY EDIT] CMOS 7.89: "End-to-End" — correct. -->
- [Chapter 12: Kubernetes](./ch12/index.md)
  - [12.1 Local Cluster with kind](./ch12/kind-setup.md)
<!-- [COPY EDIT] "kind" lowercase intentional; consider backticks. -->
  - [12.2 Preparing Services for Kubernetes](./ch12/preparing-services.md)
  - [12.3 Application Manifests](./ch12/app-manifests.md)
  - [12.4 Infrastructure Manifests](./ch12/infra-manifests.md)
  - [12.5 Kustomize Environments](./ch12/kustomize.md)
  - [12.6 Deploying and Verifying](./ch12/deploying.md)
- [Chapter 13: Cloud Deployment](./ch13/index.md)
  - [13.1 Terraform Fundamentals](./ch13/terraform-fundamentals.md)
  - [13.2 Networking: VPC and Subnets](./ch13/networking.md)
<!-- [STRUCTURAL] 13.2-13.6 use "Topic: Implementation" colon pattern. 13.1, 13.7-13.10 do not. Apply throughout AWS sections or drop. -->
  - [13.3 Container Registry: ECR](./ch13/ecr.md)
  - [13.4 Database: RDS for PostgreSQL](./ch13/rds.md)
  - [13.5 Message Broker: Amazon MSK](./ch13/msk.md)
  - [13.6 Kubernetes Cluster: EKS](./ch13/eks.md)
  - [13.7 Production Kustomize Overlay](./ch13/production-overlay.md)
  - [13.8 CI/CD Pipeline](./ch13/cicd.md)
  - [13.9 Deploying and Verifying](./ch13/deploying.md)
<!-- [COPY EDIT] Duplicates 12.6 verbatim. Disambiguate: "13.9 Deploying to AWS". -->
  - [13.10 GitOps Alternative: ArgoCD](./ch13/argocd.md)
<!-- [COPY EDIT] Vendor canonical "Argo CD" (two words). -->
- [Chapter 14: Production Hardening](./ch14/index.md)
  - [14.1 DNS with Route 53](./ch14/dns.md)
<!-- [COPY EDIT] AWS canonical "Route 53" — correct. -->
  - [14.2 TLS with ACM](./ch14/tls.md)
  - [14.3 Secrets Management](./ch14/secrets.md)
  - [14.4 Kafka Encryption: MSK TLS](./ch14/msk-tls.md)
  - [14.5 Applying the Changes](./ch14/applying.md)
<!-- [LINE EDIT] Generic. Consider "14.5 Rolling Out the Hardening". -->
<!-- [FINAL] No typos detected. Trailing newline present. -->
