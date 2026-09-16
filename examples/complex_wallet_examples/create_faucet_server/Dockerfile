# Multi-stage build for smaller final image
# Base images are pinned by digest only (Scorecard Pinned-Dependencies). Digests
# reference the multi-arch manifest list, so builds resolve the correct image
# per platform (linux/amd64, linux/arm64, …) and stay reproducible. The tag is
# omitted on purpose (Docker ignores it alongside a digest, and using both trips
# SonarCloud docker:S7018).
# golang:1.27-alpine
FROM golang@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea AS builder

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
# alpine:3.20
FROM alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

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
