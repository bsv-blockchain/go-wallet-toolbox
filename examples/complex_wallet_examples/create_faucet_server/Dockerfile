# Multi-stage build for smaller final image
# Base images are pinned by tag + digest (Scorecard Pinned-Dependencies). Digests
# reference the multi-arch manifest list, so builds resolve the correct image per
# platform and stay reproducible; the tag is kept so Dependabot preserves the
# -alpine variant on bumps (a digest alone is variant-ambiguous and can flip the
# builder to Debian, which has no `apk`).
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

# Install git, ca-certificates, and SQLite development libraries
RUN apk add --no-cache git ca-certificates sqlite-dev gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
# Remove local replace to parent repo (not available in container context)
RUN sed -i '/^replace github.com\/bsv-blockchain\/go-wallet-toolbox/d' go.mod
RUN go mod download

# Copy only required source code
COPY cmd/ ./cmd
COPY internal/ ./internal

# Tidy modules after adding source
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o faucet-server ./cmd/main.go

# Final stage
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S faucet && \
    adduser -u 1001 -S faucet -G faucet

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/faucet-server .

# No example env file to copy in this context

# Change ownership to non-root user
RUN chown -R faucet:faucet /app

# Switch to non-root user
USER faucet

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/info || exit 1

# Run the application
CMD ["./faucet-server"]
