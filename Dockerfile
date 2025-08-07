# Build stage
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Install UPX for binary compression
RUN wget -q https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-amd64_linux.tar.xz && \
    tar -xf upx-4.2.1-amd64_linux.tar.xz && \
    mv upx-4.2.1-amd64_linux/upx /usr/local/bin/ && \
    rm -rf upx-4.2.1-amd64_linux*

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o goproxlb ./cmd/main.go

# Compress binary with UPX
RUN upx --best --lzma goproxlb

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S goproxlb && \
    adduser -u 1001 -S goproxlb -G goproxlb

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/goproxlb .

# Copy sample config
COPY --from=builder /app/config.yaml ./config.yaml.example

# Change ownership to non-root user
RUN chown -R goproxlb:goproxlb /app

# Switch to non-root user
USER goproxlb

# Expose port (if needed for future web interface)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD goproxlb status --config /app/config.yaml || exit 1

# Labels
LABEL org.opencontainers.image.source="https://github.com/cblomart/GoProxLB"
LABEL org.opencontainers.image.description="GoProxLB - Load balancer for Proxmox clusters"
LABEL org.opencontainers.image.licenses="GPL-3.0"

# Default command
ENTRYPOINT ["./goproxlb"]

# Default arguments
CMD ["start", "--config", "/app/config.yaml"]
