# syntax=docker/dockerfile:1

# =============================================================================
# Builder stage
# =============================================================================
# Base images are pinned by digest only (Scorecard Pinned-Dependencies). The
# digest references the multi-arch manifest list, so builds stay reproducible
# while still resolving the correct image per platform (linux/amd64, linux/arm64, …).
# The tag is omitted deliberately: Docker ignores it when a digest is present,
# and pairing both trips SonarCloud docker:S7018.
# golang:1.27-alpine
FROM golang@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea AS builder

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
# alpine:3.20
FROM alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk add --no-cache ca-certificates tzdata curl postgresql-client \
    && adduser -D -H -u 1000 app

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /out/infra /app/infra

# Copy the docker-optimized default config
# Can be overridden at runtime with a volume mount
COPY infra-config-docker.yaml /app/infra-config.yaml

# Create a data directory (in case any local files are written)
RUN mkdir -p /app/data \
    && chown -R app:app /app

# Run as non-root to limit container privilege (docker:S6471)
USER app

EXPOSE 8100

# The binary expects infra-config.yaml in the current working directory
ENTRYPOINT ["/app/infra"]
