# ============================================================
# Stage 1: Build
# ============================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

# Build a statically linked binary
# -ldflags: -s (strip debug info), -w (omit DWARF table)
# CGO_ENABLED=0 for pure static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/server ./cmd/server

# ============================================================
# Stage 2: Run (distroless for minimal attack surface)
# ============================================================
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="URL Shortener"
LABEL org.opencontainers.image.description="A high-performance URL shortening service with analytics"
LABEL org.opencontainers.image.version="1.0.0"

WORKDIR /app

# Copy only the binary and necessary runtime files
COPY --from=builder /build/server .
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/templates ./templates

# Use non-root user (distroless nonroot is uid 65532)
USER 65532:65532

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/server", "-healthcheck"] || exit 1

ENTRYPOINT ["/app/server"]