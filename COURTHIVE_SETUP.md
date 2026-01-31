# CourtHive Integration Setup

This document describes the environment configuration needed for the CourtHive integration.

## Overview

The integration consists of:
- **jim-dot-tennis** (main app) - serves league admin at `/admin/league`
- **competition-factory-server** - CourtHive API at `/api/courthive`
- **TMX** - Tournament admin interface at `/tournaments`
- **courthive-public** - Public viewer at `/public`
- **Redis** - Cache for CourtHive
- **Caddy** - Reverse proxy routing all services

## Environment Files Required

### 1. jim-dot-tennis/.env

```bash
# Jim-dot-tennis Configuration
WRAPPED_ACCESS_PASSWORD=welovestanns

# CourtHive JWT Secret (must match competition-factory-server)
COURTHIVE_JWT_SECRET=<generate-with-openssl-rand-base64-48>
```

### 2. competition-factory-server/.env

```bash
# Application Configuration
APP_STORAGE=fileSystem
APP_NAME=Competition Factory Server
APP_MODE=development
APP_PORT=8383

# JWT Authentication (must match jim-dot-tennis COURTHIVE_JWT_SECRET)
JWT_SECRET=<same-as-jim-dot-tennis-COURTHIVE_JWT_SECRET>
JWT_VALIDITY=2h

# Cache Configuration
TRACKER_CACHE=cache

# Redis Configuration
REDIS_TTL=28800000
REDIS_HOST=localhost
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_PORT=6379

# Database Configuration (optional for LevelDB)
DB_HOST=localhost
DB_PORT=3838
DB_USER=admin
DB_PASS=adminpass

# Email Configuration (optional)
MAILGUN_API_KEY=
MAILGUN_HOST=api.eu.mailgun.net
MAILGUN_DOMAIN=
```

### 3. TMX/.env.production

```bash
SERVER=https://jim.tennis/api/courthive
ENVIRONMENT=production
BASE_URL=tournaments
```

### 4. TMX/.env.local (for local testing)

```bash
SERVER=http://localhost/api/courthive
ENVIRONMENT=development
BASE_URL=tournaments
```

### 5. courthive-public/.env.production

```bash
VITE_SERVER=https://jim.tennis/api/courthive
ENVIRONMENT=production
BASE_URL=public
```

### 6. courthive-public/.env.local (for local testing)

```bash
VITE_SERVER=http://localhost/api/courthive
ENVIRONMENT=development
BASE_URL=public
```

## Setup Steps

### 1. Generate JWT Secret

```bash
openssl rand -base64 48
```

Use this value for both `COURTHIVE_JWT_SECRET` in jim-dot-tennis/.env and `JWT_SECRET` in competition-factory-server/.env

### 2. Create .env Files

Create all the .env files listed above in their respective directories.

### 3. Build Frontend Components

```bash
# Build TMX (requires Node 24)
cd /path/to/TMX
nvm use 24  # or use your Node version manager
pnpm install
pnpm build

# Build courthive-public (requires Node 20)
cd /path/to/courthive-public
nvm use 20
pnpm install
pnpm build
```

### 4. Start Docker Services

```bash
cd /path/to/jim-dot-tennis
docker compose -f docker-compose.courthive.yml up -d
```

### 5. Verify Services

All containers should show as "healthy":
```bash
docker compose -f docker-compose.courthive.yml ps
```

Test endpoints:
- http://localhost/ - Landing page
- http://localhost/tournaments - TMX admin
- http://localhost/public - Public viewer
- http://localhost/admin/league - League admin
- http://localhost/api/courthive/ - CourtHive API

## Production Deployment

### Update Caddyfile

Uncomment the production domain configuration in `Caddyfile.courthive`:

```caddyfile
jim.tennis {
  # Import all the :80 configuration
  import :80

  # Additional HTTPS security headers
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
  }
}
```

### Point DNS

Point `jim.tennis` A record to your server IP.

### Deploy

```bash
docker compose -f docker-compose.courthive.yml up -d
```

Caddy will automatically obtain Let's Encrypt SSL certificates.

## Troubleshooting

### Frontend assets not loading

Ensure frontends were built with the correct BASE_URL:
- TMX: `BASE_URL=tournaments`
- courthive-public: `BASE_URL=public`

Rebuild if needed:
```bash
cd TMX
rm -rf dist
pnpm build
```

### CourtHive API errors

Check that JWT secrets match between jim-dot-tennis and competition-factory-server.

### Health check failures

Check container logs:
```bash
docker logs courthive-server
docker logs jim-dot-tennis
```

## Repository Branches

- **jim-dot-tennis**: `courthive-integration` branch
- **competition-factory-server**: `docker-integration` branch
- **TMX**: main branch (no changes needed)
- **courthive-public**: main branch (no changes needed)

## Notes

- .env files are **not** tracked in git for security
- Frontend dist/ folders are mounted as volumes, not copied into images
- Redis data persists in Docker volume `courthive-redis-data`
- CourtHive tournament data persists in `courthive-data` volume
- jim-dot-tennis database auto-backed up daily to `jim-dot-tennis-backups` volume
