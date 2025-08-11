# Docker Setup for Jim.Tennis

This document explains how to use Docker for developing, testing, and deploying the Jim.Tennis application.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

## Quick Start

The simplest way to get started is using our Makefile:

```bash
# Build and start the application
make

# Or separately:
make build   # Build the Docker images
make run     # Start the containers
```

The application will be available at http://localhost:8080

## Docker Components

The Docker setup includes:

1. **Application Container** - Runs the Go web application
2. **Backup Container** - Automatically backs up the SQLite database

## Docker Volumes

The setup uses two Docker volumes:

1. `jim-dot-tennis-data` - Stores the SQLite database
2. `jim-dot-tennis-backups` - Stores database backups

## Common Operations

The Makefile provides shortcuts for common operations:

```bash
# View logs
make logs       # All logs
make app-logs   # App container logs
make backup-logs # Backup container logs

# Manage application
make stop       # Stop the application
make restart    # Restart the application
make clean      # Stop and remove volumes (CAUTION: Deletes data)

# Manage backups
make backup         # Create a manual backup
make export-backup  # Export latest backup to ./exported-backups
```

## Development Workflow

For development, you can use the hot-reload feature by mounting your local templates and static files:

```bash
# The docker-compose.yml already includes volume mounts for development
# Just start the application
make dev
```

The containers are configured to watch for changes in the templates and static files.

## Backup Strategy

The Docker setup includes an automatic backup system:

1. **Daily Backups** - The backup container creates a daily backup of the database
2. **Backup Retention** - Backups older than 30 days are automatically deleted from the container
3. **External Backup Script** - Use `scripts/backup-manager.sh` to export backups to external storage

### External Backup Configuration

Edit `scripts/backup-manager.sh` to configure:

1. The external storage location
2. Retention policy
3. Optional cloud storage uploads (supports AWS S3 and Backblaze B2)

### Setting Up Scheduled External Backups

Add to your crontab:

```bash
# Example: Run external backup daily at 3 AM
0 3 * * * /path/to/jim-dot-tennis/scripts/backup-manager.sh
```

## Customizing Configuration

Environment variables can be modified in the `docker-compose.yml` file:

```yaml
environment:
  - PORT=8080             # Web server port
  - DB_TYPE=sqlite3       # Database type
  - DB_PATH=/app/data/tennis.db # Database file location
```

## Deployment

For production deployment:

1. Build the image locally or set up CI/CD to build it
2. Push the image to a Docker registry
3. On your server, pull the image and run with Docker Compose

Example deployment script:

```bash
#!/bin/bash
# Basic deployment script
ssh user@your-server "cd /path/to/app && \
  git pull && \
  docker-compose down && \
  docker-compose up -d && \
  docker image prune -f"
```

## Troubleshooting

### Accessing the Container Shell

```bash
make shell
```

### Inspecting Database

```bash
docker exec -it jim-dot-tennis /bin/sh
sqlite3 /app/data/tennis.db
```

### Checking Volume Contents

```bash
# List data volume contents
docker run --rm -v jim-dot-tennis-data:/data alpine ls -la /data

# List backup volume contents
docker run --rm -v jim-dot-tennis-backups:/backups alpine ls -la /backups
``` 

## Environment Variables (.env)

You can configure the application using environment variables. For local development, create a `.env` file in the project root (this file is already gitignored) or export the values in your shell.

Example `.env`:

```
PORT=8080
DB_TYPE=sqlite3
DB_PATH=./tennis.db
WRAPPED_ACCESS_PASSWORD=example
```

Docker Compose can reference variables using `${VAR}` syntax. The `docker-compose.yml` is set up to read `WRAPPED_ACCESS_PASSWORD` from your environment so you don't commit secrets.

Alternatively, you can export variables for a session:

```
export WRAPPED_ACCESS_PASSWORD="st.anns.2025"
./scripts/run.sh
```