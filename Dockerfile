FROM golang:1.24.1-alpine AS builder

# Install build dependencies including newer build tools
RUN apk add --no-cache gcc musl-dev sqlite-dev build-base

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with SQLite compatibility flags
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -a -ldflags '-extldflags "-static"' -tags 'sqlite_omit_load_extension' -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis

# Use a smaller image for the final application
FROM alpine:latest

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