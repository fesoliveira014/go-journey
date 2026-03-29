# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy only go.mod first for dependency caching
COPY services/gateway/go.mod services/gateway/go.sum* ./services/gateway/

# Download dependencies (currently none, but pattern is correct for future)
WORKDIR /app/services/gateway
RUN go mod download

# Copy source code
WORKDIR /app
COPY services/gateway/ ./services/gateway/

# Build static binary
WORKDIR /app/services/gateway
RUN CGO_ENABLED=0 go build -o /bin/gateway ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
COPY --from=builder /bin/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
