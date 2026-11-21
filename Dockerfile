FROM golang:1.26-alpine AS builder

# Install build dependencies including newer build tools
RUN apk add --no-cache gcc musl-dev sqlite-dev build-base

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies (module cache persists across builds via BuildKit cache mount)
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build identity, shown in the admin footer, at /version and by
# `jim-dot-tennis version`. Pass with --build-arg; plain builds say "dev".
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the application with SQLite compatibility flags.
# `-a` is intentionally omitted: static linking does not require rebuilding the
# whole stdlib, and the build cache mount makes incremental builds far faster on
# the 1-CPU server.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -ldflags "-extldflags '-static' -X jim-dot-tennis/internal/version.Version=${VERSION} -X jim-dot-tennis/internal/version.Commit=${COMMIT} -X jim-dot-tennis/internal/version.Date=${BUILD_DATE}" -tags 'sqlite_omit_load_extension' -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis

# Use a smaller image for the final application.
# Pinned to match the builder's Alpine release (golang:1.26-alpine == 3.24.x)
# so the runtime libc/sqlite-libs stay ABI-compatible with the CGO binary.
FROM alpine:3.24

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bin/jim-dot-tennis .

# Templates, static files and migrations are built into the binary (embed.go),
# so nothing else is copied. Development can bind-mount templates/ and static/
# over /app and set TEMPLATE_RELOAD=true to use the files on disk instead.

# Create a directory for the SQLite database
RUN mkdir -p /app/data

# Set environment variables
ENV PORT=8080
ENV DB_TYPE=sqlite3
ENV DB_PATH=/app/data/tennis.db

# Expose the application port
EXPOSE 8080

# Create a non-root user and set appropriate permissions
RUN adduser -D appuser \
    && chown -R appuser:appuser /app \
    && chown -R appuser:appuser /app/data

# Switch to non-root user
USER appuser

# Healthy only when the database answers and migrations are current
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

# Run the application
CMD ["./jim-dot-tennis"] 