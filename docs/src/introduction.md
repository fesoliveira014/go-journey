# Building Microservices in Go

This hands-on guide walks you through building a complete microservices application in Go. By the end, you will have built a library-management system that covers:

- **Go**—the language, project structure, testing, and idioms
- **Microservices**—decomposition, gRPC, and event-driven architecture with Kafka
- **Databases**—PostgreSQL with migrations and the repository pattern
- **Containers**—Docker, multi-stage builds, and Docker Compose
- **Orchestration**—Kubernetes (kind locally, EKS in production)
- **Infrastructure as Code**—Terraform for AWS (VPC, EKS, RDS)
- **Observability**—OpenTelemetry, Tempo, Prometheus, Grafana, Loki
- **CI/CD**—GitHub Actions and Earthly
- **Authentication**—JWT, bcrypt, OAuth2 with Google

## Who This Is For

You are an experienced software engineer who is new to Go, cloud-native tooling, or both. The guide assumes strong programming fundamentals and explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch.

## The Project

We will build a **library-management system** where:

- Admins manage the book catalog (CRUD operations)
- Users browse, search, reserve, and return books
- Authentication supports email/password and Google OAuth2

The system decomposes into five microservices: Gateway, Auth, Catalog, Reservation, and Search.

## How to Use This Guide

Each chapter builds on the previous one, so follow them in order. **The code snippets in each chapter show the codebase as it exists at that point in the journey.** Later chapters modify and extend these files, so a snippet from Chapter 2 will look different in the final repository. Expect additions from Chapters 3–9 (Kafka integration, OpenTelemetry, structured logging, etc.). This is intentional: each chapter teaches one layer at a time.

Every chapter includes:

- **Theory**—why each decision matters
- **Implementation**—complete, runnable code
- **Exercises**—problems that reinforce the chapter
- **References**—links to official documentation and further reading
