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
FROM golang@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS builder

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
FROM alpine@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

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
