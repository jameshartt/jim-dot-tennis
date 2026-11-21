# Docker Setup for Jim.Tennis

This document explains how to use Docker for developing, testing, and deploying the Jim.Tennis application.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

No local Go installation is required. All Go tooling (building, formatting, linting, static analysis) runs inside Docker containers using the `golang:1.25-alpine` image.

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

### Core Stack (`docker-compose.yml`)

1. **Application Container** - Runs the Go 1.25 web application (multi-stage build: `golang:1.25-alpine` builder, `alpine:latest` runtime)
2. **Import Container** - On-demand container for running import utilities (profile: `tools`)

Backups are made by the app itself (no separate container): see [Backup Strategy](#backup-strategy).

### CourtHive Stack (`deploy/docker-compose.yml`)

The reference deployment (`deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/.env.example`) is the stack production runs. It extends the deployment with additional services for tournament management:

1. **jim-dot-tennis app** - The core Go application
2. **competition-factory-server** - CourtHive API server (NestJS)
3. **TMX Frontend** - Tournament management interface (served via Caddy from pre-built static files)
4. **courthive-public** - Public-facing CourtHive site (served via Caddy)
5. **Redis** and **Postgres** - `redis:7-alpine` and `postgres:18-alpine` for CourtHive
6. **Caddy** - `caddy:2-alpine` reverse proxy with automatic SSL

## Docker Volumes

The core setup uses one Docker volume:

1. `jim-dot-tennis-data` - The SQLite database, uploaded logos (`branding/`) and the app's own backups (`backups/`)

The CourtHive stack adds additional volumes:

2. `courthive-redis-data` - Redis persistence
3. `courthive-data` - CourtHive server data
4. `courthive-cache` - CourtHive tracker cache
5. `caddy-data` - Caddy TLS certificates and data
6. `caddy-config` - Caddy configuration

## Common Operations

The Makefile provides shortcuts for common operations:

```bash
# View logs
make logs       # All logs
make app-logs   # App container logs

# Manage application
make stop       # Stop the application
make restart    # Restart the application
make clean      # Stop and remove volumes (CAUTION: Deletes data)

# Manage backups
make backup         # Make a backup now (jim-dot-tennis backup in the app container)
make export-backup  # Copy the newest backup out to ./exported-backups
```

## Go Tooling (Docker-based)

All Go tooling runs inside Docker containers using the `golang:1.25-alpine` image, so no local Go installation is required. The SQLite driver is pure Go (`modernc.org/sqlite`), so every tool runs with `CGO_ENABLED=0` and no C toolchain is installed.

### Code Quality Checks

```bash
make vet         # Run go vet static analysis
make fmt         # Check formatting (lists unformatted files)
make fmt-fix     # Fix formatting in-place
make imports     # Check import ordering (uses goimports)
make imports-fix # Fix import ordering in-place
make lint        # Run golangci-lint (11 linters configured in .golangci.yml)
make deadcode    # Run dead code detection
make tidy        # Run go mod tidy
make check       # Run all read-only checks (vet, fmt, lint, deadcode)
```

The `make lint` target runs golangci-lint with 11 linters enabled: errcheck, govet, staticcheck, ineffassign, unused, gosimple, typecheck, misspell, goconst, gofmt, and goimports. Configuration is in `.golangci.yml`.

## CourtHive Operations

The Makefile provides targets for managing the full CourtHive stack:

```bash
make build-tmx        # Build TMX frontend (requires pnpm in ../TMX)
make courthive        # Build TMX and start the full CourtHive stack
make courthive-up     # Start the CourtHive stack without rebuilding TMX
make courthive-down   # Stop the CourtHive stack
make courthive-restart # Restart the CourtHive stack
make courthive-logs   # View CourtHive stack logs
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

The app backs itself up; there is no backup container and no cloud upload.

1. **Daily backups** - the server makes one archive a day (`BACKUP_INTERVAL`, default `24h`, or `off`) in `/app/data/backups` (`BACKUP_DIR`) and keeps the newest 14 (`BACKUP_KEEP`). Each archive holds a consistent copy of the database, the uploaded logo and a manifest with checksums.
2. **Before upgrades** - a backup is taken before any migration changes the database; if it cannot be made, the app does not upgrade or start.
3. **On demand** - `make backup` (or `docker exec jim-dot-tennis ./jim-dot-tennis backup`), or **Download backup** on the admin Backups page.
4. **Restore** - `jim-dot-tennis restore <file>`, only while the app is stopped, in a one-off container on the same volume.

These copies are on the same disk as the app. Copying them somewhere else, and restoring, are described step by step in `docs/PRODUCTION_QUICK_REFERENCE.md` section 9. A backup holds personal data and password hashes: keep copies private.

## Customizing Configuration

Environment variables can be modified in the `docker-compose.yml` file or in a `.env` file:

```yaml
environment:
  - HOME_CLUB_ID=10             # Database ID of the home club (required, or use HOME_CLUB_NAME)
  - HOME_CLUB_NAME=St Ann's     # Fallback: club name for fuzzy lookup
  - HOME_CLUB_LOGO_PATH=/static/my-club.jpg  # Optional: overrides the logo uploaded on the Club Settings page
  - BHPLTA_CLUB_CODE=what-three-words  # Optional: overrides the BHPLTA club code stored on the Club Settings page
  - PORT=8080                   # Web server port
  - DB_TYPE=sqlite3             # Database type
  - DB_PATH=/app/data/tennis.db # Database file location
```

### Using your own club logo

Upload the logo on **Admin Dashboard → Club Settings** (PNG, JPEG or WebP, up to 2 MB). It is stored in `/app/data/branding/`, on the data volume next to the database, so it survives image upgrades. The app's backups include it.

Alternatively, set `HOME_CLUB_LOGO_PATH` to the URL path of an image you serve yourself (for example from the reverse proxy); that takes precedence over an uploaded logo. Static files are built into the binary, so an image mounted into `/app/static` is not served (except in development with `TEMPLATE_RELOAD=true`).

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

# List the app's backups
docker run --rm -v jim-dot-tennis-data:/data alpine ls -la /data/backups
```

## Environment Variables (.env)

You can configure the application using environment variables. For local development, create a `.env` file in the project root (this file is already gitignored) or export the values in your shell.

Example `.env`:

```
HOME_CLUB_ID=10
BHPLTA_CLUB_CODE=what-three-words
PORT=8080
DB_TYPE=sqlite3
DB_PATH=./tennis.db
# Optional: WRAPPED_ACCESS_PASSWORD=<a code of your own>. Leave it out and the app
# generates a code for this instance (shown on the admin Club Settings page).
```

Docker Compose can reference variables using `${VAR}` syntax. The `docker-compose.yml` is set up to read `HOME_CLUB_ID`, `BHPLTA_CLUB_CODE`, and `WRAPPED_ACCESS_PASSWORD` from your environment so you don't commit secrets.

Alternatively, you can export variables for a session:

```
export HOME_CLUB_ID=10
export BHPLTA_CLUB_CODE="what-three-words"
./scripts/run.sh
```
