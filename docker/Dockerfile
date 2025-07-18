# Multi-stage Dockerfile for Hotel Reviews Microservice

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev

# Set build environment
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Create app directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a \
    -installsuffix cgo \
    -o hotel-reviews-api \
    ./cmd/api

# Verify the binary
RUN file hotel-reviews-api
RUN ldd hotel-reviews-api || true

# Stage 2: Security scanning (optional, can be enabled in CI/CD)
FROM alpine:3.18 AS security-scan
RUN apk add --no-cache \
    ca-certificates \
    curl
# Copy the binary for security scanning
COPY --from=builder /build/hotel-reviews-api /tmp/hotel-reviews-api
# Placeholder for security scanning tools
# RUN security-scan /tmp/hotel-reviews-api

# Stage 3: Final runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    dumb-init

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/configs /app/logs /app/tmp && \
    chown -R appuser:appgroup /app

# Set timezone
ENV TZ=UTC

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from builder stage
COPY --from=builder /build/hotel-reviews-api /app/hotel-reviews-api

# Copy configuration files
COPY --chown=appuser:appgroup configs/ /app/configs/

# Create example environment file
RUN echo '# Hotel Reviews Microservice Environment Variables' > /app/.env.example && \
    echo 'HOTEL_REVIEWS_DATABASE_HOST=localhost' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_DATABASE_PORT=5432' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_DATABASE_USER=postgres' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_DATABASE_PASSWORD=postgres' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_DATABASE_NAME=hotel_reviews' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_S3_REGION=us-east-1' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_S3_BUCKET=hotel-reviews-files' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_LOG_LEVEL=info' >> /app/.env.example && \
    echo 'HOTEL_REVIEWS_LOG_FORMAT=json' >> /app/.env.example && \
    chown appuser:appgroup /app/.env.example

# Set proper permissions
RUN chmod +x /app/hotel-reviews-api && \
    chmod 755 /app && \
    chmod -R 755 /app/configs && \
    chmod -R 755 /app/logs && \
    chmod -R 755 /app/tmp

# Set working directory
WORKDIR /app

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

# Expose port
EXPOSE 8080

# Set default environment variables
ENV HOTEL_REVIEWS_SERVER_HOST=0.0.0.0 \
    HOTEL_REVIEWS_SERVER_PORT=8080 \
    HOTEL_REVIEWS_LOG_LEVEL=info \
    HOTEL_REVIEWS_LOG_FORMAT=json \
    HOTEL_REVIEWS_LOG_OUTPUT=stdout

# Labels for metadata
LABEL maintainer="Hotel Reviews Team <team@example.com>" \
      version="1.0.0" \
      description="Hotel Reviews Microservice" \
      org.opencontainers.image.title="hotel-reviews-microservice" \
      org.opencontainers.image.description="A microservice for processing hotel review data" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.vendor="Hotel Reviews Inc." \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.url="https://github.com/gkbiswas/hotel-reviews-microservice" \
      org.opencontainers.image.source="https://github.com/gkbiswas/hotel-reviews-microservice" \
      org.opencontainers.image.documentation="https://github.com/gkbiswas/hotel-reviews-microservice/README.md"

# Use dumb-init as PID 1 for proper signal handling
ENTRYPOINT ["/usr/bin/dumb-init", "--"]

# Default command
CMD ["./hotel-reviews-api", "-mode", "server"]