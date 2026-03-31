# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service and gen/, not all
# workspace members. The replace directive in go.mod handles the gen/ import.
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY pkg/otel/go.mod pkg/otel/go.sum* ./pkg/otel/
COPY services/catalog/go.mod services/catalog/go.sum* ./services/catalog/

# 2. Download dependencies (cached unless go.mod changes)
WORKDIR /app/services/catalog
RUN go mod download

# 3. Copy source code (invalidates cache only when source changes)
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY pkg/otel/ ./pkg/otel/
COPY services/catalog/ ./services/catalog/

# 4. Build static binary
WORKDIR /app/services/catalog
RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/catalog /usr/local/bin/catalog
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/catalog"]
