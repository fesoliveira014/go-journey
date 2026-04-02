# Building Microservices in Go

Welcome to this hands-on guide to building a complete microservices application in Go. By the end of this tutorial, you will have built a library management system covering:

- **Go** — the language, project structure, testing, and idioms
- **Microservices** — service decomposition, gRPC, event-driven architecture with Kafka
- **Databases** — PostgreSQL with migrations and the repository pattern
- **Containers** — Docker, multi-stage builds, Docker Compose
- **Orchestration** — Kubernetes (kind locally, EKS in production)
- **Infrastructure as Code** — Terraform for AWS (VPC, EKS, RDS)
- **Observability** — OpenTelemetry, Tempo, Prometheus, Grafana, Loki
- **CI/CD** — GitHub Actions and Earthly
- **Authentication** — JWT, bcrypt, OAuth2 with Gmail

## Who This Is For

You are an experienced software engineer who knows how to program but is new to Go and/or cloud-native tooling. The guide assumes strong programming fundamentals but explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch.

## The Project

We are building a **library management system** where:

- Admins manage the book catalog (CRUD operations)
- Users browse, search, reserve, and return books
- Authentication supports email/password and Google OAuth2

The system is decomposed into 5 microservices: Gateway, Auth, Catalog, Reservation, and Search.

## How to Use This Guide

Each chapter builds on the previous one. Follow them in order. **The code snippets in each chapter show the codebase as it exists at that point in the journey.** Later chapters modify and extend these files -- so if you compare a snippet from Chapter 2 to the final repository, you will see additions from Chapters 3-9 (Kafka integration, OpenTelemetry, structured logging, etc.). This is intentional: each chapter teaches one layer at a time.

Every chapter includes:

- **Theory** — why we are making each decision
- **Implementation** — complete, runnable code
- **Exercises** — practice problems to test your understanding
- **References** — links to official docs and further reading
