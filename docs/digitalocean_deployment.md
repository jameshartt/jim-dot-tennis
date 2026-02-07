# DigitalOcean Deployment Guide

This guide explains how to deploy the Jim.Tennis application to a DigitalOcean droplet using Docker.

## Prerequisites

1. A DigitalOcean account
2. A droplet running Ubuntu (recommended: Ubuntu 20.04 LTS or newer)
3. SSH access to your droplet
4. (Optional) A domain name pointed to your droplet's IP address

## Deployment

We use a two-part deployment approach to ensure reliability:

1. A server setup script that runs on the droplet to install dependencies
2. A local deployment script that transfers files and configures the application

### Step 1: Configure the Deployment Script

Edit the configuration section at the top of `scripts/deploy-digitalocean.sh`:

```bash
# Configuration - Update these values
DROPLET_IP=""              # Your droplet's IP address
SSH_USER="root"            # SSH user (usually root for initial setup)
SSH_KEY_PATH=""            # Path to your SSH private key (leave empty for default)
DEPLOY_DIR="/opt/jim-dot-tennis" # Deployment directory on the server
APP_DOMAIN=""              # Optional: your domain if you have one
```

Make sure to set at least the `DROPLET_IP` value. The `SSH_KEY_PATH` can be left empty if your SSH key is in the default location.

### Step 2: Run the Deployment Script

Once you've configured the script, simply run:

```bash
./scripts/deploy-digitalocean.sh
```

The deployment script will:

1. Test the SSH connection to your droplet
2. Detect if this is a new deployment or an update
3. For new deployments:
   - Upload and run the server setup script
   - Install Docker, Docker Compose, and other dependencies
   - Configure firewall and security settings
4. Transfer the application files to the server
5. Configure HTTPS with Caddy if a domain is provided
6. Start the application using Docker Compose

## What's Included

The deployment sets up:

- Docker and Docker Compose
- UFW firewall with proper port settings
- Fail2ban for SSH protection
- A dedicated user account for the application
- HTTPS configuration with Caddy (if domain provided)
- Automatic database backups

### Services

The production deployment comprises multiple services orchestrated via Docker Compose:

| Service | Description | Compose File |
|---------|-------------|--------------|
| **jim-dot-tennis** (app) | Go 1.25 web application serving the league management UI | `docker-compose.yml` |
| **backup** | Alpine-based container performing daily SQLite backups with 30-day retention | `docker-compose.yml` |
| **import** | On-demand tools container for match card and club data imports | `docker-compose.yml` (tools profile) |
| **competition-factory-server** | CourtHive API server (Node.js) for tournament management | `docker-compose.courthive.yml` |
| **TMX frontend** | Static tournament management UI served via Caddy | `docker-compose.courthive.yml` |
| **courthive-public** | Public-facing CourtHive UI served via Caddy | `docker-compose.courthive.yml` |
| **Redis** | In-memory data store used by the CourtHive server (`redis:7-alpine`) | `docker-compose.courthive.yml` |
| **Caddy** | Reverse proxy handling SSL termination and routing for all services (`caddy:2-alpine`) | `docker-compose.courthive.yml` |

The base `docker-compose.yml` runs jim-dot-tennis standalone. The full CourtHive stack uses `docker-compose.courthive.yml`, which includes all of the above services on a shared `tennis-network` bridge network.

### CourtHive Stack Management

The Makefile provides dedicated targets for managing the CourtHive stack:

```bash
make courthive           # Build TMX frontend and start the full CourtHive stack
make courthive-up        # Start the CourtHive stack without rebuilding TMX
make courthive-down      # Stop the CourtHive stack
make courthive-restart   # Restart the CourtHive stack
make courthive-logs      # View CourtHive stack logs
make build-tmx           # Build TMX frontend only
```

### Admin Routes

The application exposes the following admin routes (all under `/admin/league/`, protected by session-based auth with admin role):

- **Dashboard**: `/admin/league/dashboard`
- **Players**: `/admin/league/players`, `/admin/league/players/filter`
- **Fixtures**: `/admin/league/fixtures`, `/admin/league/fixtures/week-overview`
- **Teams**: `/admin/league/teams`
- **Clubs**: `/admin/league/clubs`
- **Divisions**: `/admin/league/divisions/` (division editing -- added in Sprint 003)
- **Users**: `/admin/league/users` (user CRUD management -- added in Sprint 003)
- **Sessions**: `/admin/league/sessions` (session management -- added in Sprint 003)
- **Seasons**: `/admin/league/seasons`, `/admin/league/seasons/set-active`, `/admin/league/seasons/setup`, `/admin/league/seasons/move-team`, `/admin/league/seasons/copy-from-previous`
- **Selection Overview**: `/admin/league/selection-overview`
- **Points Table**: `/admin/league/points-table`
- **Match Card Import**: `/admin/league/match-card-import`
- **Club Data Import**: `/admin/league/club-data-import`
- **Club Wrapped**: `/admin/league/wrapped` (also has public route at `/club/wrapped`)
- **Preferred Names**: `/admin/league/preferred-names` (approvals, history, approve/reject)

## Manual Server Setup (Optional)

If you prefer to set up the server manually or want to understand what the server setup script does, you can:

1. SSH into your DigitalOcean droplet:

```bash
ssh root@your-droplet-ip
```

2. Upload the server setup script:

```bash
scp scripts/digitalocean-server-setup.sh root@your-droplet-ip:/tmp/
```

3. Run the server setup script manually:

```bash
ssh root@your-droplet-ip "chmod +x /tmp/digitalocean-server-setup.sh && sudo /tmp/digitalocean-server-setup.sh"
```

## Setting Up HTTPS with Caddy

HTTPS is automatically configured if you provide a domain name in the deployment script. The deployment creates:

1. A `Caddyfile` with your domain configuration
2. A `docker-compose.override.yml` file that adds the Caddy service
3. Proper port mappings and volume configurations

## Managing Your Deployment

### Viewing Logs

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs -f"
```

### Stopping the Application

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down"
```

### Restarting the Application

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose restart"
```

### Updating the Application

Simply run the deployment script again:

```bash
./scripts/deploy-digitalocean.sh
```

The script will detect the existing installation and update only the necessary files.

## Backup Management

The deployment includes an automatic backup system that:

1. Creates daily backups within Docker
2. Exports backups to `/opt/jim-dot-tennis/external-backups`
3. Runs a cron job daily at 3 AM

### Manual Backup

To manually trigger a backup:

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker exec jim-dot-tennis-backup sh -c 'sqlite3 /data/tennis.db \".backup /backups/tennis-\$(date +%Y-%m-%d-%H%M%S)-manual.db\"'"
```

### Restoring from Backup

```bash
# Stop the application
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down"

# Restore the database from backup
ssh user@your-droplet-ip "cp /opt/jim-dot-tennis/external-backups/tennis-backup-file.db /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db"

# Start the application
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose up -d"
```

## Upgrading Go Version

When upgrading the Go version in the Dockerfile, follow these steps:

### Recent Upgrade: Go 1.24.1 -> 1.25 (Sprint 004)

This documents the upgrade from Go 1.24.1 to Go 1.25 completed in Sprint 004:

**Changes Made:**
- Updated `FROM golang:1.24.1-alpine AS builder` to `FROM golang:1.25-alpine AS builder`
- Updated runtime image remains `FROM alpine:latest`
- Updated Go tooling image in Makefile to `golang:1.25-alpine`
- Updated `go.mod` module directive to `go 1.25.0`

### Previous Upgrade: Go 1.18 -> 1.24.1 (June 2025)

This documents the successful upgrade from Go 1.18 to Go 1.24.1 that was completed:

**Changes Made:**
- Updated `FROM golang:1.18-alpine AS builder` to `FROM golang:1.24.1-alpine AS builder`
- Added SQLite compatibility dependencies: `sqlite-dev build-base`
- Added compatibility build flags: `CGO_CFLAGS="-D_LARGEFILE64_SOURCE"`
- Added static linking and SQLite optimization tags
- Resolved duplicate repository file conflicts during deployment

**Commands Used:**
```bash
# 1. Force rebuild after Dockerfile changes
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose down && docker-compose build --no-cache app && docker-compose up -d"

# 2. When build conflicts occurred, removed duplicate files
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && rm -f ./internal/repository/*_repository.go"

# 3. Verified successful deployment
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose ps"
ssh root@144.126.228.64 "docker images | grep jim-dot-tennis"
```

**Result:** Successfully upgraded to Go 1.24.1 with application running healthy at https://jim.tennis

### General Upgrade Process

When upgrading the Go version in the Dockerfile, follow these steps:

### Step 1: Update Dockerfile

Edit the Go version in the Dockerfile:

```dockerfile
# Change from:
FROM golang:1.25-alpine AS builder

# To (example):
FROM golang:1.26-alpine AS builder
```

Also update the Go tooling image in the Makefile:

```makefile
GO_IMAGE = golang:1.26-alpine
```

### Step 2: Handle SQLite Compatibility (if using CGO)

For Go versions 1.20+ with SQLite, you may need to add compatibility fixes:

```dockerfile
# Install additional build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev build-base

# Use compatibility build flags
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -a -ldflags '-extldflags "-static"' -tags 'sqlite_omit_load_extension' -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis
```

### Step 3: Force Rebuild and Deploy

```bash
# Connect to server and force rebuild
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && docker-compose build --no-cache app && docker-compose up -d"
```

### Step 4: Verify Upgrade

```bash
# Check container status
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose ps"

# Check application logs
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs app | tail -10"

# Verify new image was built
ssh root@your-droplet-ip "docker images | grep jim-dot-tennis"
```

## Go Tooling (Docker-based)

Sprint 004 added 9 Makefile targets that run Go tooling inside Docker containers, so no local Go installation is required. These use the `golang:1.25-alpine` image to match the Dockerfile builder stage.

### Read-only Checks

```bash
make vet          # Run go vet (static analysis, requires CGO for SQLite)
make fmt          # Check formatting (list unformatted files)
make lint         # Run golangci-lint (comprehensive linting, requires CGO)
make deadcode     # Run dead code detection (requires CGO)
make check        # Run all read-only checks (vet + fmt + lint + deadcode)
```

### Auto-fix Targets

```bash
make fmt-fix      # Fix formatting in-place
make imports      # Check import ordering
make imports-fix  # Fix import ordering in-place
```

### Module Management

```bash
make tidy         # Run go mod tidy (requires CGO for SQLite dependency resolution)
```

Tools that need to compile code with SQLite (vet, deadcode, lint, tidy) run with CGO enabled and install `gcc musl-dev sqlite-dev build-base` inside the container. Text-only tools (fmt, imports) run without CGO for speed.

## Useful Management Commands

### Application Health Checks

```bash
# Check running containers and their status
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose ps"

# View real-time application logs
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs -f app"

# Check last 50 log lines
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs --tail=50 app"

# Test application health endpoint (if available)
ssh root@your-droplet-ip "curl -f http://localhost:8080/ || echo 'App not responding'"
```

### Docker Management

```bash
# View Docker images and sizes
ssh root@your-droplet-ip "docker images | grep jim-dot-tennis"

# Remove old/unused Docker images to free space
ssh root@your-droplet-ip "docker image prune -f"

# View Docker system usage
ssh root@your-droplet-ip "docker system df"

# Force rebuild specific service
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose build --no-cache app"

# Restart specific service
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose restart app"
```

### File System Management

```bash
# Check deployment directory contents
ssh root@your-droplet-ip "ls -la /opt/jim-dot-tennis"

# Check for duplicate or conflicting files
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && find . -name '*_repository.go' -type f"

# Remove duplicate files (if build conflicts occur)
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && rm -f ./internal/repository/*_repository.go"

# Check disk usage
ssh root@your-droplet-ip "df -h"

# Check application data directory
ssh root@your-droplet-ip "docker exec jim-dot-tennis ls -la /app/data"
```

### Database Operations

```bash
# Access database directly
ssh root@your-droplet-ip "docker exec -it jim-dot-tennis sqlite3 /app/data/tennis.db"

# Check database file size
ssh root@your-droplet-ip "docker exec jim-dot-tennis ls -lh /app/data/tennis.db"

# Run database integrity check
ssh root@your-droplet-ip "docker exec jim-dot-tennis sqlite3 /app/data/tennis.db 'PRAGMA integrity_check;'"
```

### SSL/HTTPS Management

```bash
# Check Caddy configuration
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && cat Caddyfile"

# View Caddy logs
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs caddy"

# Force SSL certificate renewal
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose exec caddy caddy reload --config /etc/caddy/Caddyfile"
```

## Troubleshooting

### Connection Issues

If you're experiencing SSH connection issues:

1. Check that your SSH key is correctly set up in DigitalOcean
2. Verify the droplet's IP address and your network connectivity
3. Try increasing the connection timeout in the deployment script
4. Ensure the SSH port (22) is open in the droplet's firewall

### Build Conflicts

If you encounter duplicate declaration errors during build:

```bash
# Check for duplicate repository files
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && find . -name '*_repository.go' -type f"

# Remove duplicate files
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && rm -f ./internal/repository/*_repository.go"

# Force clean rebuild
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && docker-compose build --no-cache app"
```

### Docker Compose Errors

If Docker Compose fails to start the application:

1. Check the application logs for specific errors:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs"
   ```

2. Verify that all required files were transferred correctly:
   ```bash
   ssh root@your-droplet-ip "ls -la /opt/jim-dot-tennis"
   ```

3. Check if containers are running:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose ps"
   ```

4. Force restart all services:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && docker-compose up -d"
   ```

### SQLite/CGO Build Issues

If you encounter SQLite compilation errors with newer Go versions:

1. Ensure build dependencies are installed:
   ```dockerfile
   RUN apk add --no-cache gcc musl-dev sqlite-dev build-base
   ```

2. Use compatibility flags:
   ```dockerfile
   RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -a -ldflags '-extldflags "-static"' -tags 'sqlite_omit_load_extension' -o /app/bin/jim-dot-tennis ./cmd/jim-dot-tennis
   ```

### Application Not Responding

If the application is not responding:

1. Check if the container is running:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose ps"
   ```

2. Check application logs:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs app"
   ```

3. Check if port 8080 is accessible:
   ```bash
   ssh root@your-droplet-ip "netstat -tlnp | grep 8080"
   ```

4. Test local connectivity:
   ```bash
   ssh root@your-droplet-ip "curl -f http://localhost:8080/ || echo 'Local connection failed'"
   ```

### Database Issues

If you're experiencing database problems:

1. Check if the database exists:
   ```bash
   ssh root@your-droplet-ip "docker exec jim-dot-tennis ls -la /app/data"
   ```

2. Verify database integrity:
   ```bash
   ssh root@your-droplet-ip "docker exec jim-dot-tennis sqlite3 /app/data/tennis.db 'PRAGMA integrity_check;'"
   ```

3. If the database is missing or corrupted, restore from a backup as described above

### Performance Issues

If the application is running slowly:

1. Check system resources:
   ```bash
   ssh root@your-droplet-ip "htop"  # or "top" if htop not available
   ssh root@your-droplet-ip "free -h"
   ssh root@your-droplet-ip "df -h"
   ```

2. Check Docker container resource usage:
   ```bash
   ssh root@your-droplet-ip "docker stats"
   ```

3. Check application-specific logs for errors:
   ```bash
   ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs app | grep -i error"
   ```

## Quick Reference Commands

### Most Common Operations

```bash
# Quick status check
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose ps && docker images | grep jim-dot-tennis"

# View current logs
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs -f app"

# Quick restart
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose restart app"

# Full restart (stops and starts all services)
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && docker-compose up -d"

# Force rebuild and deploy
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && docker-compose build --no-cache app && docker-compose up -d"
```

### Emergency Commands

```bash
# If app won't start due to build conflicts
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && rm -f ./internal/repository/*_repository.go && docker-compose build --no-cache app"

# If running out of disk space
ssh root@your-droplet-ip "docker system prune -af"

# If need to restore from backup (replace backup-file.db with actual backup)
ssh root@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down && cp external-backups/backup-file.db /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db && docker-compose up -d"
```

### Health Check URLs

- **Application**: https://jim.tennis
- **Direct IP**: http://144.126.228.64:8080 (if HTTPS fails)
- **Local on server**: `curl http://localhost:8080/`
