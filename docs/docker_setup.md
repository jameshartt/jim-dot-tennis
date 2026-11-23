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

### Packaged deployment (`deploy/docker-compose.yml`)

The packaged deployment (`deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/.env.example`) is what a club runs on its server. Every image is published; nothing is built there. By default it runs two services:

1. **app** - jim-dot-tennis, from `ghcr.io/jameshartt/jim-dot-tennis`
2. **caddy** - `caddy:2-alpine`, the only service with open ports (80 and 443), with automatic HTTPS

With tournaments turned on (see [Tournaments (optional)](#tournaments-optional)) it also runs:

3. **courthive-server** - the tournament server (CourtHive), 512 MB memory limit
4. **courthive-hiveid** - the identity service that signs tournament sign-in tokens, 256 MB
5. **postgres** - `postgres:18-alpine`, the tournament database, 384 MB
6. **redis** - `redis:7-alpine`, the tournament server's cache, 64 MB
7. **tmx-content**, **public-content** - copy TMX and the public tournament pages into volumes for Caddy, then stop
8. **courthive-service-account** - makes sure the account the app signs in to the tournament stack with exists, then stops

jim.tennis itself still runs an older, source-built layout; see "jim.tennis until it adopts the bundle" in `docs/PRODUCTION_QUICK_REFERENCE.md`.

## Docker Volumes

The core setup uses one Docker volume:

1. `jim-dot-tennis-data` - The SQLite database, uploaded logos (`branding/`) and the app's own backups (`backups/`)

The packaged deployment adds:

2. `caddy-data` - Caddy TLS certificates and data
3. `caddy-config` - Caddy configuration

With tournaments on:

4. `courthive-pg-data` - the tournament database (all tournament data)
5. `courthive-redis-data` - Redis persistence
6. `courthive-archives` - the tournament server's provider archives
7. `courthive-tmx-files`, `courthive-public-files` - the two front ends, refilled from their images on every `docker compose up`

Turning tournaments off keeps these volumes and their data.

## Tournaments (optional)

Tournaments are off until you turn them on. With them on the server needs
2 GB of memory (or 1 GB with 2 GB of swap); see
`docs/courthive_bundle_design.md` section 3.

### Turn tournaments on

On the server, in the `deploy` folder (where `.env` is):

1. Make the secrets. This is done once and is safe to repeat:

   ```bash
   ./init-tournaments.sh
   ```

   It needs `openssl` (installed on almost every Linux server). It adds seven
   variables to the end of `.env`, never changes a line that is already
   there, prints no secret, and makes `.env` readable by you only. If it says
   a variable "has no real value", delete that line from `.env` and run it
   again (a `.env` copied from release 0.1.0 has two `change-me` lines).

2. Turn the profile on and start everything:

   ```bash
   echo COMPOSE_PROFILES=tournaments >> .env
   docker compose up -d
   ```

   The first start downloads about 1.3 GB of images and takes a minute or
   two. Compose starts the services in order and waits for each to be
   healthy. Use the `.env` line, not `docker compose --profile tournaments`:
   the compose file reads `COMPOSE_PROFILES` to decide what the app and Caddy
   are told.

3. Sign in to the app as an administrator and switch on **Tournaments** on the
   Club Settings page.

### Check it

```bash
docker compose ps -a
```

`app`, `caddy`, `courthive-server`, `courthive-hiveid`, `postgres` and `redis`
should be "healthy" or "running". `tmx-content`, `public-content` and
`courthive-service-account` should say "Exited (0)": they do their job and
stop. "Exited (1)" for `courthive-service-account` means the service account
could not be made; run `docker compose up -d` again, and if it still fails
look at `docker compose logs courthive-server`. Then open
`https://<your domain>/tournaments/` and `https://<your domain>/public/`.

If compose stops with "COURTHIVE_... is missing from deploy/.env", run
`./init-tournaments.sh`.

### Turn tournaments off

Remove (or comment out) the `COMPOSE_PROFILES=tournaments` line in `.env`,
then:

```bash
docker compose --profile tournaments down   # stops everything, keeps all data
docker compose up -d                        # starts the app and Caddy again
```

The `down` is needed because `docker compose up -d` leaves running services
of a profile that is off as they are. The site is unavailable for the few
seconds in between. Tournament data stays in its volumes; turning tournaments
on again brings it back. Keep the secrets in `.env`: they are needed to read
that data again.

### What Caddy allows

With tournaments off, `/tournaments`, `/public`, `/api/courthive` and
`/socket.io` answer with a plain 404. With them on, `/api/courthive` and
`/socket.io` go to the tournament stack with the `Cookie` header removed, and
the tournament stack's own sign-in, account, administration and internal
routes answer 404 (people sign in to jim-dot-tennis only). The exact rules are
at the top of `deploy/Caddyfile` and in `docs/courthive_bundle_design.md`
section 4. `/api/rankings` and `/epixodic` at the site root always answer 404.

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

The Makefile has shortcuts that run `deploy/docker-compose.yml` with the
tournaments profile on (they need `deploy/.env`, with the secrets from
`deploy/init-tournaments.sh`):

```bash
make courthive-up      # Start the packaged stack with tournaments on
make courthive-down    # Stop it (all services; data is kept)
make courthive-restart # Stop and start it
make courthive-logs    # Follow its logs
make build-tmx         # Build a TMX source checkout in ../TMX (development only; the stack uses the published image)
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
