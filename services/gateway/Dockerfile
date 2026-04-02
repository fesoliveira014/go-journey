# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

ENV GOWORK=off

# Copy go.mod files first for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY pkg/otel/go.mod pkg/otel/go.sum* ./pkg/otel/
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/

WORKDIR /app/services/gateway
RUN go mod download

# Copy full source
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY pkg/otel/ ./pkg/otel/
COPY services/gateway/ ./services/gateway/

# Build static binary
WORKDIR /app/services/gateway
RUN CGO_ENABLED=0 go build -o /bin/gateway ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
RUN addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/gateway /usr/local/bin/gateway
COPY --from=builder /app/services/gateway/templates/ /app/templates/
COPY --from=builder /app/services/gateway/static/ /app/static/
WORKDIR /app
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
