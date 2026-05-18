# syntax=docker/dockerfile:1

# =============================================================================
# Builder stage
# =============================================================================
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the infra server binary (static, small)
# Supports both amd64 and arm64 (Apple Silicon)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /out/infra \
    ./cmd/infra

# =============================================================================
# Runtime stage
# =============================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl postgresql-client

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /out/infra /app/infra

# Copy the docker-optimized default config
# Can be overridden at runtime with a volume mount
COPY infra-config-docker.yaml /app/infra-config.yaml

# Create a data directory (in case any local files are written)
RUN mkdir -p /app/data

# Run as root for development convenience (mounted config files, etc.)
# For production you can switch to a non-root user.
# USER 1000

EXPOSE 8100

# The binary expects infra-config.yaml in the current working directory
ENTRYPOINT ["/app/infra"]
