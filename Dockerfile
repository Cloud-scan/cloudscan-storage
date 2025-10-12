# Dockerfile for CloudScan Storage Service
# Expects pre-built binary from make linux
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 cloudscan && \
    adduser -D -u 1000 -G cloudscan cloudscan

WORKDIR /app

# Copy pre-built binary (expects cloudscan-storage-amd64 or cloudscan-storage-arm64)
ARG TARGETARCH
COPY cloudscan-storage-${TARGETARCH} ./cloudscan-storage

# Create storage directories with proper permissions
RUN mkdir -p /app/storage /app/cache /tmp && \
    chown -R cloudscan:cloudscan /app /tmp

# Switch to non-root user
USER cloudscan

# Expose HTTP port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the binary
ENTRYPOINT ["/app/cloudscan-storage"]