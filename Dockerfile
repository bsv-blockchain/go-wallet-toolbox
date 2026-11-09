# syntax=docker/dockerfile:1

# =============================================================================
# Builder stage
# =============================================================================
# Base images are pinned by tag + digest. The digest keeps builds reproducible
# and satisfies the Scorecard Pinned-Dependencies check (it references the
# multi-arch manifest list, resolving the right image per platform). The tag is
# kept so Dependabot preserves the -alpine variant on bumps: given a digest alone
# it cannot tell -alpine from the default Debian image and silently flips the
# builder to Debian, which has no `apk` and breaks the build.
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

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
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

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
