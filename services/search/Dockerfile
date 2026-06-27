# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Disable workspace mode — we only copy this service and gen/, not all
# workspace members. The replace directive in go.mod handles the gen/ import.
ENV GOWORK=off

# 1. Copy only go.mod/go.sum for dependency caching
COPY gen/go.mod gen/go.sum* ./gen/
COPY pkg/auth/go.mod pkg/auth/go.sum* ./pkg/auth/
COPY pkg/kafka/go.mod pkg/kafka/go.sum* ./pkg/kafka/
COPY services/search/go.mod services/search/go.sum* ./services/search/

# 2. Download dependencies
WORKDIR /app/services/search
RUN go mod download

# 3. Copy source code
WORKDIR /app
COPY gen/ ./gen/
COPY pkg/auth/ ./pkg/auth/
COPY pkg/kafka/ ./pkg/kafka/
COPY services/search/ ./services/search/

# 4. Build static binary
WORKDIR /app/services/search
RUN CGO_ENABLED=0 go build -o /bin/search ./cmd/

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/search /usr/local/bin/search
USER app
EXPOSE 50054
ENTRYPOINT ["/usr/local/bin/search"]
