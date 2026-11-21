# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X github.com/14mdzk/goscratch/internal/platform/version.Version=${VERSION}" -o /app/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X github.com/14mdzk/goscratch/internal/platform/version.Version=${VERSION}" -o /app/worker ./cmd/worker

# Runtime stage
FROM alpine:3.24

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy binary and config
COPY --from=builder /app/api .
COPY --from=builder /app/worker .
COPY --from=builder /app/config/config.default.json ./config/

# Change ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose default API port (matches config/config.default.json; override via SERVER_PORT)
EXPOSE 8080

# Health check (uses /healthz/live and honors SERVER_PORT)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD sh -c 'wget --no-verbose --tries=1 --spider "http://localhost:${SERVER_PORT:-8080}/healthz/live"' || exit 1

# Run
CMD ["./api"]
