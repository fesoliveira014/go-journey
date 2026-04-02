# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service, gen/, and pkg/auth/
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/

# 2. Download dependencies (cached unless go.mod changes)
WORKDIR /app/services/auth
RUN go mod download

# 3. Copy source code (invalidates cache only when source changes)
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY services/auth/ ./services/auth/

# 4. Build static binary
WORKDIR /app/services/auth
RUN CGO_ENABLED=0 go build -o /bin/auth ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
RUN addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/auth /usr/local/bin/auth
USER app
EXPOSE 50051
ENTRYPOINT ["/usr/local/bin/auth"]
