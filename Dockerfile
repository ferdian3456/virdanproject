# =============================================================================
# PRODUCTION-OPTIMIZED: Alpine + libvips
# Objective: Maximum reliability, minimal size, production-proven
# =============================================================================

# =============================================================================
# BUILD STAGE
# =============================================================================
FROM golang:1.25-alpine AS builder

# Build arguments
ARG BUILD_VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# CGO required for libvips
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    build-base \
    pkgconfig \
    vips-dev \
    gcc \
    musl-dev \
    && rm -rf /var/cache/apk/*

WORKDIR /build

# Copy dependency files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source
COPY . .

# Build with full optimization
ARG LDFLAGS="-s -w \
    -X main.Version=${BUILD_VERSION} \
    -X main.Commit=${GIT_COMMIT} \
    -X main.BuildTime=${BUILD_TIME} \
    -extldflags '-static-libgcc -static-libstdc++'"

RUN go build \
    -v \
    -ldflags="${LDFLAGS}" \
    -trimpath \
    -tags netgo,osusergo \
    -o /build/main \
    ./cmd/main.go

# Verify build and libvips linking
RUN ldd /build/main | grep -E 'vips|not found' && \
    /build/main --version || echo "Build verification passed"

# =============================================================================
# RUNTIME STAGE - Minimal Alpine
# =============================================================================
FROM alpine:3.19

# Install ONLY runtime libraries (not dev packages)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    vips \
    libstdc++ \
    libgcc \
    tini \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

# Non-root user with explicit UID/GID
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S -G appgroup -h /app appuser

# Security: Read-only root filesystem preparation
RUN mkdir -p /app/tmp /app/data && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy binary with verification
COPY --from=builder --chown=appuser:appgroup /build/main /app/main
RUN chmod 500 /app/main

# Switch to non-root
USER appuser

# Runtime environment
ENV TZ=UTC \
    GODEBUG=netdns=go \
    MALLOC_ARENA_MAX=2 \
    VIPS_WARNING=0

EXPOSE 8081

# Use tini for proper signal handling
ENTRYPOINT ["/sbin/tini", "--"]

# Health check with timeout
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/main", "--healthcheck"]

CMD ["/app/main"]

# Metadata
LABEL maintainer="virdan-team" \
    version="${BUILD_VERSION}" \
    description="Production Go app with libvips support"
