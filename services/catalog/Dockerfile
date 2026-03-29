# Build stage
# NOTE: This Dockerfile requires a build context that includes the gen/ module.
# Build from the project root: docker build -f services/catalog/Dockerfile .
# Standalone docker build from services/catalog/ will fail because go.mod
# references ../../gen via a replace directive. This is addressed properly
# in Chapter 3 (Docker Compose) with a root-level build context.
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/catalog ./cmd/

# Runtime stage
FROM alpine:3.19
COPY --from=builder /bin/catalog /usr/local/bin/catalog
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/catalog"]
