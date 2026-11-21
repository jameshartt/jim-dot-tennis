FROM golang:1.26-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies (module cache persists across builds via BuildKit cache mount)
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build a static binary with no C toolchain: SQLite is the pure-Go
# modernc.org/sqlite driver, so CGO is off. The build cache mount makes
# incremental builds far faster on the 1-CPU server.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis

# Use a smaller image for the final application. The binary is static and
# links no C library, so the runtime image needs no SQLite package.
FROM alpine:3.24

# Install runtime dependencies (TLS roots for outbound HTTPS, zone data)
RUN apk add --no-cache ca-certificates tzdata

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