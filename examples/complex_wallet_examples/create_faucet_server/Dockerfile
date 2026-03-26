# Multi-stage build for smaller final image
FROM golang:1.26-alpine AS builder

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
FROM alpine:latest

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
