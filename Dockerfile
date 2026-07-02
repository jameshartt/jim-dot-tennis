FROM golang:1.25-alpine AS builder

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

# Build the application with SQLite compatibility flags.
# `-a` is intentionally omitted: static linking does not require rebuilding the
# whole stdlib, and the build cache mount makes incremental builds far faster on
# the 1-CPU server.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -ldflags '-extldflags "-static"' -tags 'sqlite_omit_load_extension' -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis

# Use a smaller image for the final application.
# Pinned to match the builder's Alpine release (golang:1.25-alpine == 3.23.x)
# so the runtime libc/sqlite-libs stay ABI-compatible with the CGO binary.
FROM alpine:3.23

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bin/jim-dot-tennis .

# Copy necessary files
COPY migrations/ ./migrations/
COPY templates/ ./templates/
COPY static/ ./static/

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

# Run the application
CMD ["./jim-dot-tennis"] 