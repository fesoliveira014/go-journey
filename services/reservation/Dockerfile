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
COPY services/reservation/go.mod services/reservation/go.sum* ./services/reservation/

# 2. Download dependencies
WORKDIR /app/services/reservation
RUN go mod download

# 3. Copy source code
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY pkg/otel/ ./pkg/otel/
COPY services/reservation/ ./services/reservation/

# 4. Build static binary
WORKDIR /app/services/reservation
RUN CGO_ENABLED=0 go build -o /bin/reservation ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/reservation /usr/local/bin/reservation
EXPOSE 50053
ENTRYPOINT ["/usr/local/bin/reservation"]
