# ============================================
# Build stage
# ============================================
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install ca-certificates for HTTPS calls
RUN apk add --no-cache ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary (no CGO, pure Go SQLite)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o link-shortener .

# ============================================
# Runtime stage
# ============================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S app && adduser -S app -G app

# Create data directory
RUN mkdir -p /data && chown app:app /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/link-shortener .

# Use non-root user
USER app

# Expose port
EXPOSE 3100

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:3100/health || exit 1

# Volume for persistent database
VOLUME ["/data"]

ENTRYPOINT ["./link-shortener"]
