# CourtHive Deployment Planning Document

## Executive Summary

This document provides a comprehensive, step-by-step plan to deploy the CourtHive tournament management system alongside the existing jim-dot-tennis application on a single DigitalOcean droplet. The deployment will integrate three CourtHive TypeScript/Node.js components with the existing Go-based calendar and team management system.

**Status:** Phase 1 completed and tested locally on 2026-01-18

---

## Implementation Status

### Phase 1: COMPLETED ✅ (2026-01-18)

Phase 1 (Local Development Environment) has been successfully completed with all services running and verified. See **[Implementation Notes](#implementation-notes)** section below for details on actual implementation vs original plan, including errors encountered and solutions.

**Key Deliverables:**
- ✅ jim-dot-tennis routes updated from `/admin` to `/admin/league`
- ✅ competition-factory-server Dockerized and running
- ✅ TMX admin frontend built and accessible at `/tournaments`
- ✅ courthive-public built and accessible at `/public`
- ✅ Docker Compose configuration created (`docker-compose.courthive.yml`)
- ✅ Caddy reverse proxy configured (`Caddyfile.courthive`)
- ✅ All services healthy and endpoints tested
- ✅ Documentation created (`COURTHIVE_SETUP.md`)

**Git Branches:**
- jim-dot-tennis: `courthive-integration` branch (3 commits)
- competition-factory-server: `docker-integration` branch (1 commit)
- TMX: main branch (no code changes, dist/ built)
- courthive-public: main branch (no code changes, dist/ built)

### Phase 2-5: Pending

Phases 2-5 (deployment to production) are ready to proceed pending decision to deploy.

---

## Table of Contents

0. [Implementation Notes](#implementation-notes) **← Phase 1 Actual Implementation Details**
1. [Current State Analysis](#current-state-analysis)
2. [CourtHive Components Overview](#courthive-components-overview)
3. [Architecture Design](#architecture-design)
4. [Routing Strategy](#routing-strategy)
5. [Step-by-Step Implementation Plan](#step-by-step-implementation-plan)
6. [Docker Configuration](#docker-configuration)
7. [Nginx/Caddy Reverse Proxy Setup](#nginx-caddy-reverse-proxy-setup)
8. [Authentication & Authorization Integration](#authentication-authorization-integration)
9. [Deployment Process](#deployment-process)
10. [Testing & Validation](#testing-validation)
11. [Rollback Strategy](#rollback-strategy)
12. [Future Considerations](#future-considerations)

---

## Implementation Notes

**This section documents the actual Phase 1 implementation completed on 2026-01-18.**

### Overview

Phase 1 was successfully completed with all services running locally. The implementation followed the plan outlined in Section 5 with some important modifications and lessons learned.

### Key Differences from Original Plan

#### 1. Environment Variable Configuration

**Original Plan:** Suggested using `VITE_API_URL` and `VITE_SOCKET_URL` for both frontends.

**Actual Implementation:**
- **TMX (.env.production):**
  ```bash
  SERVER=https://jim.tennis/api/courthive
  ENVIRONMENT=production
  BASE_URL=tournaments
  ```

- **courthive-public (.env.production):**
  ```bash
  VITE_SERVER=https://jim.tennis/api/courthive
  ENVIRONMENT=production
  BASE_URL=public
  ```

**Reason:** The actual vite.config.ts files in each project expected different environment variable names. We read the actual config files to determine the correct variables.

**Critical Addition:** `BASE_URL` was not in the original plan but was essential for correct asset path generation in the built frontends. Without this, assets were referenced as `./assets/` which failed after being served from `/tournaments` and `/public` routes.

#### 2. Dockerfile for competition-factory-server

**Original Plan:** Used `--prod` flag for production dependencies only.

**Actual Implementation:**
```dockerfile
# Production stage
FROM node:24-alpine

RUN corepack enable && corepack prepare pnpm@latest --activate && \
    apk add --no-cache curl

WORKDIR /app

COPY package.json pnpm-lock.yaml ./
COPY pnpm-workspace.yaml ./

# Install ALL dependencies, not just production
RUN pnpm install --frozen-lockfile --ignore-scripts

COPY --from=builder /app/build ./build

RUN mkdir -p /app/data /app/cache && chown -R node:node /app

USER node
EXPOSE 8383

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD curl -f http://localhost:8383/ || exit 1

CMD ["node", "build/main.js"]
```

**Changes:**
1. **Dependencies:** Used `pnpm install --frozen-lockfile --ignore-scripts` instead of `--prod` because the `compression` module was needed at runtime but was in devDependencies
2. **Health Check:** Changed from `wget` to `curl` because curl worked more reliably in testing
3. **Curl Installation:** Added `apk add --no-cache curl` for health checks
4. **Build Output:** Used `build/` directory instead of `dist/` (actual build output location)

#### 3. docker-compose.courthive.yml Modifications

**Critical Addition:** `NODE_ENV=production` environment variable was required for courthive-server validation checks:

```yaml
courthive-server:
  environment:
    - NODE_ENV=production  # REQUIRED - server validation checks this
    - JWT_SECRET=${COURTHIVE_JWT_SECRET}
    # ... other vars
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8383/"]  # Changed from wget
```

**Volume Mounts:** Changed from `../TMX/dist` to absolute paths relative to docker-compose location:
```yaml
caddy:
  volumes:
    - ./Caddyfile.courthive:/etc/caddy/Caddyfile
    - ../TMX/dist:/srv/tournaments
    - ../courthive-public/dist:/srv/public
```

#### 4. Caddyfile Configuration

**Original Plan:** Used `uri strip_prefix` with `handle` directive.

**Actual Implementation:** Used `handle_path` for SPA routes:

```caddyfile
:80 {
  # API endpoints - strip prefix as planned
  handle /api/courthive/* {
    uri strip_prefix /api/courthive
    reverse_proxy courthive-server:8383
  }

  # SPA routes - use handle_path instead of handle + strip_prefix
  handle_path /tournaments* {
    root * /srv/tournaments
    try_files {path} /index.html
    file_server
  }

  handle_path /public* {
    root * /srv/public
    try_files {path} /index.html
    file_server
  }

  # ... rest of config
}
```

**Reason:** `handle_path` automatically strips the path prefix for the `root` directive, preventing double-stripping issues with SPA assets.

**Local Testing:** Configuration starts with `:80` for local testing. Production section is commented out and can be enabled by uncommenting the `jim.tennis { import :80 }` block.

#### 5. Landing Page (index.html)

**Original Plan:** No changes to landing page.

**Actual Implementation:** Complete redesign of `templates/index.html`:

**Issue:** Original index.html used Go template inheritance with `{{define "content"}}` blocks that conflicted with login.html template blocks, causing the admin login page to appear on the homepage.

**Solution:** Made index.html standalone without using layout template inheritance:

```html
{{define "index.html"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <title>Jim.Tennis - Tournament Management</title>
    <!-- ... full head section ... -->
</head>
<body>
    <!-- Clean landing page with link to /public tournaments -->
    <h1 style="font-size: 3rem;">🎾 Jim.Tennis</h1>
    <a href="/public" class="btn btn-primary">View Public Tournaments →</a>
</body>
</html>
{{end}}
```

#### 6. Environment File Security

**Original Plan:** Create .env files and commit to git.

**Actual Implementation:** Created comprehensive `COURTHIVE_SETUP.md` documentation instead of committing .env files.

**Reason:** Security best practice - never commit secrets to git. The COURTHIVE_SETUP.md documents all required .env files with placeholders and instructions for generating secrets.

**Location:** `/home/jameshartt/Development/Tennis/jim-dot-tennis/COURTHIVE_SETUP.md`

### Errors Encountered and Solutions

#### Error 1: Module 'compression' Not Found

**Symptom:** courthive-server container failed to start with error: `Cannot find module 'compression'`

**Root Cause:** Using `pnpm install --prod` in Dockerfile only installed production dependencies, but `compression` was listed in devDependencies yet needed at runtime.

**Solution:** Changed Dockerfile to use `pnpm install --frozen-lockfile --ignore-scripts` to install all dependencies.

**Files Modified:** `competition-factory-server/Dockerfile`

#### Error 2: Health Check Failures - Missing NODE_ENV

**Symptom:** courthive-server health checks continuously failed even though server was running.

**Root Cause:** The server's environment validation required `NODE_ENV` to be set, but it wasn't in the docker-compose environment variables.

**Solution:** Added `NODE_ENV=production` to docker-compose.courthive.yml environment section.

**Files Modified:** `docker-compose.courthive.yml`

#### Error 3: Health Check Command Not Working

**Symptom:** Health checks using `wget` returned "connection refused" but `curl` worked.

**Root Cause:** Initial health check used Node.js HTTP request and then `wget`, but the container didn't have `wget` installed and the Node approach was unreliable.

**Solution:**
1. Installed `curl` in Dockerfile: `apk add --no-cache curl`
2. Changed health check to: `CMD curl -f http://localhost:8383/ || exit 1`

**Files Modified:** `competition-factory-server/Dockerfile`, `docker-compose.courthive.yml`

#### Error 4: Frontend Assets Not Loading (Blank Pages)

**Symptom:** Navigating to `/tournaments` and `/public` showed blank HTML pages with no assets loaded. Browser console showed 404 errors for CSS/JS files.

**Root Cause:** Vite builds were using relative asset paths (`./assets/file.js`). When served from `/tournaments`, the browser looked for assets at `/assets/file.js` instead of `/tournaments/assets/file.js`.

**Solution:** Added `BASE_URL` environment variable to .env files and rebuilt frontends:
- TMX: `BASE_URL=tournaments`
- courthive-public: `BASE_URL=public`

This made Vite generate absolute paths like `/tournaments/assets/file.js`.

**Additional Fix:** Changed Caddyfile from `handle /tournaments/* { uri strip_prefix }` to `handle_path /tournaments*` to properly serve the SPA.

**Files Modified:** `TMX/.env.production`, `courthive-public/.env.production`, `Caddyfile.courthive`

#### Error 5: Homepage Showing Admin Login

**Symptom:** Visiting `http://localhost/` showed the admin login page instead of the landing page.

**Root Cause:** Go template block naming conflict. Both `templates/index.html` and `templates/login.html` defined blocks named "head" and "content" within the shared layout. The login template was overriding the index template.

**Solution:** Made `templates/index.html` completely standalone without using template inheritance. Removed `{{define "content"}}` blocks and created a full HTML document with `{{define "index.html"}}`.

**Files Modified:** `templates/index.html`

#### Error 6: Git Operations in Wrong Directory

**Symptom:** Git commands failed because they were executed in the parent `Tennis/` directory which isn't a git repository.

**Root Cause:** Initial commands like `git checkout -b docker-integration` were run from the wrong directory.

**Solution:** Always `cd` into the specific repository directory before running git commands:
```bash
cd /home/jameshartt/Development/Tennis/competition-factory-server
git checkout -b docker-integration
```

**Affected Repositories:** competition-factory-server

#### Error 7: Wrong Environment Variable Names

**Symptom:** Frontend builds completed but weren't using correct API endpoints.

**Root Cause:** Initially used `VITE_API_URL` and `VITE_SOCKET_URL` for all frontends based on common Vite conventions.

**Solution:** Read actual `vite.config.ts` files to determine correct variable names:
- TMX expects: `SERVER`, `ENVIRONMENT`, `BASE_URL`
- courthive-public expects: `VITE_SERVER`, `ENVIRONMENT`, `BASE_URL`

**Files Modified:** All .env.production and .env.local files

#### Error 8: Production User Seeding Required

**Symptom:** Cannot login with test user `axel@castle.com` in production, getting 401 Unauthorized.

**Root Cause:** The test user is only available when `APP_MODE=development`. The code in `users.service.ts` shows:

```typescript
async findOne(email: string) {
  const mode = this.configService.get('APP')?.mode;
  const devModeTestUser = mode === 'development' && (await this.testUsers.find((user) => user.email === email));
  if (devModeTestUser) return devModeTestUser;
  return await netLevel.get(BASE_USER, { key: email });
}
```

**Solution:** Create a production-safe admin user seeding script.

**Created:** `/home/jameshartt/Development/Tennis/competition-factory-server/seed-admin.js`

The script includes multiple security layers:
1. **File permissions:** 700 (owner-only access)
2. **In .gitignore:** Won't be committed to repository
3. **Interactive confirmation:** Requires typing "yes" to proceed
4. **Database credentials required:** Must know DB_USER and DB_PASS
5. **Container-only access:** Can only run from inside Docker container or with direct server access
6. **Duplicate prevention:** Checks if user already exists before creating
7. **Password validation:** Minimum 8 characters required

**Usage:**

```bash
# Copy script to container
docker cp seed-admin.js courthive-server:/app/

# Run interactively
docker exec -it courthive-server node /app/seed-admin.js user@example.com 'SecurePassword123'

# Or with environment variables
docker exec -it courthive-server sh -c "ADMIN_EMAIL=user@example.com ADMIN_PASSWORD='SecurePassword123' node /app/seed-admin.js"
```

**Important Notes:**
- Uses `bcryptjs` (not `bcrypt`) - the version in package.json
- Uses `@gridspace/net-level-client` which automatically handles JSON serialization
- Must pass object to `db.put()`, NOT `JSON.stringify(object)`
- Requires `net-level-server` to be running on port 3838
- Creates user with all roles: SUPER_ADMIN, ADMIN, DEVELOPER, CLIENT, SCORE

**Files Modified:** Created `competition-factory-server/seed-admin.js`, updated `competition-factory-server/.gitignore`

### Actual File Structure Created

```
jim-dot-tennis/
├── .env                              # NOT in git
├── docker-compose.courthive.yml      # NEW - orchestration
├── Caddyfile.courthive              # NEW - reverse proxy config
├── COURTHIVE_SETUP.md               # NEW - environment documentation
├── templates/index.html              # MODIFIED - standalone landing page
├── internal/admin/
│   ├── handler.go                    # MODIFIED - routes to /admin/league
│   ├── teams.go                      # MODIFIED - route updates
│   ├── fixtures.go                   # MODIFIED - route updates
│   ├── players.go                    # MODIFIED - route updates
│   └── ...                           # All admin files updated
└── cmd/jim-dot-tennis/main.go       # MODIFIED - auth redirect path

competition-factory-server/
├── .env                              # NOT in git
├── Dockerfile                        # NEW
├── seed-admin.js                     # NEW - production user seeding (NOT in git)
└── [existing files]

TMX/
├── .env.production                   # NOT in git
├── .env.local                        # NOT in git
└── dist/                             # BUILT output

courthive-public/
├── .env.production                   # NOT in git
├── .env.local                        # NOT in git
└── dist/                             # BUILT output
```

### Build Commands Used

```bash
# Install Node versions with nvm
nvm install 24
nvm install 20

# Build TMX (requires Node 24)
cd /home/jameshartt/Development/Tennis/TMX
nvm use 24
pnpm install
pnpm build

# Build courthive-public (requires Node 20)
cd /home/jameshartt/Development/Tennis/courthive-public
nvm use 20
pnpm install
pnpm build

# Start all services
cd /home/jameshartt/Development/Tennis/jim-dot-tennis
docker compose -f docker-compose.courthive.yml up -d

# Verify health
docker compose -f docker-compose.courthive.yml ps
```

### Testing Results

All endpoints tested successfully on 2026-01-18:

```bash
# Jim-dot-tennis home
curl -I http://localhost/
# ✅ 200 OK - Landing page with link to /public

# League admin (redirects to login)
curl -I http://localhost/admin/league
# ✅ 302 Found - Redirects to login

# TMX admin interface
curl -I http://localhost/tournaments
# ✅ 200 OK - TMX loads with assets

# Public tournament viewer
curl -I http://localhost/public
# ✅ 200 OK - Public viewer loads

# CourtHive API
curl http://localhost/api/courthive/
# ✅ 200 OK - API responds

# WebSocket endpoint
curl -I http://localhost/socket.io/
# ✅ 400 Bad Request - Expected (needs upgrade headers)
```

**Docker Health Checks:**
- ✅ jim-dot-tennis: healthy
- ✅ courthive-redis: healthy
- ✅ courthive-server: healthy
- ✅ tennis-caddy: running
- ✅ jim-dot-tennis-backup: running

### Lessons Learned

1. **Always read actual config files** instead of assuming standard conventions (e.g., environment variable names)

2. **BASE_URL is critical for SPAs** served from subdirectories - this wasn't in the original plan but was essential

3. **Health checks need proper tools** - ensure curl/wget are installed in containers that use them

4. **Environment validation matters** - NODE_ENV was required by the application even though not documented

5. **Template inheritance can cause conflicts** - standalone templates are sometimes simpler than shared layouts

6. **Never commit .env files** - use documentation with placeholders instead

7. **Docker Compose depends_on with conditions** - Using `depends_on: service_name: condition: service_healthy` ensures proper startup order

8. **Vite asset paths** - Using relative paths breaks when served from subdirectories; always set BASE_URL

9. **Testing locally first is essential** - Caught all issues before they would have affected production

10. **Git branch strategy** - Keep infrastructure changes (Dockerfile) in separate branches per repository

11. **Production requires initial user seeding** - The test user (axel@castle.com) only works in development mode; production needs a proper admin user creation script

12. **LevelDB is required for authentication** - Even with fileSystem storage, user/provider management requires net-level-server running on port 3838

13. **@gridspace/net-level-client handles JSON automatically** - When storing objects, use the object directly, not JSON.stringify()

14. **bcryptjs vs bcrypt** - The project uses bcryptjs (pure JS), not native bcrypt

### References

For complete environment setup instructions, see: `COURTHIVE_SETUP.md`

For actual working configurations:
- Docker orchestration: `docker-compose.courthive.yml`
- Reverse proxy: `Caddyfile.courthive`
- Dockerfile: `../competition-factory-server/Dockerfile`

---

## 1. Current State Analysis

### jim-dot-tennis (Existing Application)

**Technology Stack:**
- **Language:** Go 1.24.1
- **Database:** SQLite3 (with file-based storage)
- **Web Framework:** Chi/Gin for HTTP routing
- **Templating:** Server-side HTML templates
- **Frontend:** HTMX + vanilla JavaScript
- **Deployment:** Docker + Docker Compose
- **Reverse Proxy:** Caddy (for HTTPS/SSL)

**Current Routes:**
- `/` - Public home/login
- `/admin/*` - Admin interfaces for fixtures, teams, players
- `/players/*` - Player availability management
- `/static/*` - Static assets (CSS, JS, images)

**Current Port:** 8080 (behind Caddy on 80/443)

**Key Files:**
- `Dockerfile` - Go application container
- `docker-compose.yml` - Orchestration with backup container
- Deployment script at `scripts/deploy-digitalocean.sh`

---

## 2. CourtHive Components Overview

### A. competition-factory-server (Backend API)

**Technology Stack:**
- **Framework:** NestJS (Node.js/TypeScript)
- **Port:** 8383 (configurable)
- **Database:** Redis + LevelDB or file system storage
- **Authentication:** JWT-based
- **Key Dependencies:**
  - Redis (required)
  - Optional: net-level-server for LevelDB
  - Mailgun for email notifications
  - Socket.io for real-time updates

**Build Requirements:**
- Node.js 20+
- pnpm package manager
- Redis server
- Environment configuration (.env file)

**Key Features:**
- Tournament data persistence
- Competition factory mutations (scheduling, draw management)
- WebSocket support for real-time updates
- User/provider authentication system

### B. TMX (Admin Frontend)

**Technology Stack:**
- **Framework:** Vite + TypeScript (SPA)
- **UI Library:** Bulma CSS framework
- **Build Output:** Static files in `dist/`
- **Key Dependencies:**
  - tods-competition-factory (business logic)
  - courthive-components
  - Tabulator tables
  - Event Calendar
  - Socket.io client

**Build Requirements:**
- Node.js 24
- pnpm
- Build command: `pnpm build` → produces `dist/` folder

**Features:**
- Tournament creation and management
- Draw generation and visualization
- Player/participant management
- Schedule creation
- Results entry

### C. courthive-public (Public Tournament Viewer)

**Technology Stack:**
- **Framework:** Vite + TypeScript (SPA)
- **UI Library:** Tabulator + custom CSS
- **Build Output:** Static files in `dist/`
- **Routing:** Client-side routing with Navigo

**Build Requirements:**
- Node.js 20+
- pnpm
- Build command: `pnpm build` → produces `dist/`

**Features:**
- Public tournament viewing
- Real-time match results
- Schedule display
- Participant/team information
- No authentication required

---

## 3. Architecture Design

### Proposed Architecture

```
                              ┌──────────────────────────────────┐
                              │    DigitalOcean Droplet          │
                              │    (144.126.228.64)              │
                              └──────────────────────────────────┘
                                              │
                                              │ Port 80/443
                                              ▼
                              ┌──────────────────────────────────┐
                              │         Caddy/Nginx              │
                              │      (Reverse Proxy/SSL)         │
                              └──────────────────────────────────┘
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    │                         │                         │
                    ▼                         ▼                         ▼
        ┌────────────────────┐   ┌────────────────────┐   ┌────────────────────┐
        │  jim-dot-tennis    │   │ courthive-server   │   │   Redis Server     │
        │  (Go - Port 8080)  │   │ (NestJS - 8383)    │   │   (Port 6379)      │
        │                    │   │                    │   │                    │
        │  Routes:           │   │  Routes:           │   └────────────────────┘
        │  /calendar/*       │   │  /api/courthive/*  │
        │  /teams/*          │   │  /socket.io/*      │
        │  /admin/league/*   │   └────────────────────┘
        └────────────────────┘
                    │
                    ▼
        ┌────────────────────┐
        │  SQLite Database   │
        │  (League Data)     │
        └────────────────────┘

Static File Serving (via Caddy/Nginx):
  - /tournaments/*     → TMX Admin Frontend (built static files)
  - /public/*          → courthive-public Frontend (built static files)
  - /static/*          → jim-dot-tennis static assets
```

### Key Design Decisions

1. **All Services Run in Docker:** Each component gets its own container
2. **Shared Network:** Docker network for inter-service communication
3. **Single Entry Point:** Caddy/Nginx handles all external traffic
4. **Separate Data Stores:**
   - jim-dot-tennis: SQLite for league data
   - CourtHive: Redis + filesystem for tournament data
5. **Static File Serving:** Built frontend apps served directly by reverse proxy

---

## 4. Routing Strategy

### Updated URL Structure

| Route Pattern | Service | Description |
|--------------|---------|-------------|
| `/` | jim-dot-tennis | Landing/login page |
| `/calendar/*` | jim-dot-tennis | Calendar and availability |
| `/teams/*` | jim-dot-tennis | Team management |
| `/admin/league/*` | jim-dot-tennis | League administration |
| `/players/*` | jim-dot-tennis | Player management |
| `/static/*` | jim-dot-tennis | Static assets for league app |
| `/tournaments/*` | TMX (static) | Tournament admin interface |
| `/public/*` | courthive-public (static) | Public tournament viewer |
| `/api/courthive/*` | competition-factory-server | CourtHive API endpoints |
| `/socket.io/*` | competition-factory-server | WebSocket connections |

### Route Migration Plan

**Current jim-dot-tennis routes to update:**
- `/admin` → `/admin/league` (namespace admin under league context)
- Keep `/static/*` as-is
- All other routes remain unchanged

---

## 5. Step-by-Step Implementation Plan

### Phase 1: Prepare Local Development Environment ✅ COMPLETED (2026-01-18)

**Objective:** Get all four applications running locally with the new routing structure

**Status:** ✅ COMPLETED - See [Implementation Notes](#implementation-notes) for actual implementation details, errors encountered, and solutions.

**What Was Actually Implemented:** The steps below represent the original plan. For what actually happened, including all errors and fixes, see the Implementation Notes section at the top of this document.

#### Step 1.1: Update jim-dot-tennis Routes (1-2 hours)

1. **Backup current code:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis
   git checkout -b courthive-integration
   ```

2. **Update routing in Go application:**
   - Locate main router setup (likely in `cmd/jim-dot-tennis/main.go` or `internal/`)
   - Change admin route prefix from `/admin` to `/admin/league`
   - Update all template references to use new paths
   - Update any hardcoded links in HTML templates

3. **Test locally:**
   ```bash
   make clean-local
   make local
   # Verify routes at http://localhost:8080/admin/league
   ```

#### Step 1.2: Build CourtHive Components (2-3 hours)

1. **Setup competition-factory-server:**
   ```bash
   cd /home/jameshartt/Development/Tennis/competition-factory-server
   
   # Install Redis locally
   sudo apt-get update && sudo apt-get install redis-server -y
   sudo systemctl start redis
   
   # Install dependencies
   pnpm install
   
   # Create .env file
   cat > .env << 'EOF'
   APP_STORAGE='fileSystem'
   APP_NAME='Competition Factory Server'
   APP_MODE='development'
   APP_PORT=8383
   
   JWT_SECRET='change-this-in-production-use-random-string-here'
   JWT_VALIDITY=2h
   
   TRACKER_CACHE='cache'
   
   REDIS_TTL=28800000
   REDIS_HOST='localhost'
   REDIS_USERNAME=''
   REDIS_PASSWORD=''
   REDIS_PORT=6379
   
   DB_HOST=localhost
   DB_PORT=3838
   DB_USER=admin
   DB_PASS=adminpass
   
   MAILGUN_API_KEY='optional-for-email'
   MAILGUN_HOST='api.eu.mailgun.net'
   MAILGUN_DOMAIN='m.your.domain'
   EOF
   
   # Build and start
   pnpm build
   pnpm start
   ```

2. **Build TMX Admin Frontend:**
   ```bash
   cd /home/jameshartt/Development/Tennis/TMX
   
   # Install dependencies
   pnpm install
   
   # Update .env.production for API endpoint
   cat > .env.production << 'EOF'
   VITE_API_URL=https://jim.tennis/api/courthive
   VITE_SOCKET_URL=https://jim.tennis
   EOF
   
   # Build for production
   pnpm build
   
   # Output will be in dist/ folder
   ls -la dist/
   ```

3. **Build courthive-public:**
   ```bash
   cd /home/jameshartt/Development/Tennis/courthive-public
   
   # Install dependencies
   pnpm install
   
   # Update .env.production
   cat > .env.production << 'EOF'
   VITE_API_URL=https://jim.tennis/api/courthive
   EOF
   
   # Build for production
   pnpm build
   
   # Output will be in dist/
   ls -la dist/
   ```

#### Step 1.3: Create Dockerfiles for CourtHive Components (1-2 hours)

1. **Create Dockerfile for competition-factory-server:**
   ```bash
   cd /home/jameshartt/Development/Tennis/competition-factory-server
   ```
   
   Create `Dockerfile`:
   ```dockerfile
   # Build stage
   FROM node:24-alpine AS builder
   
   # Install pnpm
   RUN npm install -g pnpm
   
   WORKDIR /app
   
   # Copy package files
   COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
   
   # Install dependencies
   RUN pnpm install --frozen-lockfile
   
   # Copy source code
   COPY . .
   
   # Build application
   RUN pnpm build
   
   # Production stage
   FROM node:24-alpine
   
   # Install pnpm
   RUN npm install -g pnpm
   
   WORKDIR /app
   
   # Copy package files and install production dependencies only
   COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
   RUN pnpm install --prod --frozen-lockfile
   
   # Copy built application from builder
   COPY --from=builder /app/dist ./dist
   COPY --from=builder /app/build ./build
   
   # Create directories for data storage
   RUN mkdir -p /app/data /app/cache
   
   # Expose port
   EXPOSE 8383
   
   # Start application
   CMD ["node", "dist/main.js"]
   ```

2. **No Dockerfile needed for TMX and courthive-public** (static files served by Caddy)

#### Step 1.4: Create Docker Compose Configuration (2-3 hours)

1. **Create new docker-compose file structure:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis
   ```

2. **Create `docker-compose.courthive.yml`:**
   ```yaml
   version: '3.8'
   
   services:
     # Existing jim-dot-tennis app
     app:
       build:
         context: .
         dockerfile: Dockerfile
       container_name: jim-dot-tennis
       restart: unless-stopped
       volumes:
         - tennis-data:/app/data
         - ./templates:/app/templates
         - ./static:/app/static
       environment:
         - PORT=8080
         - DB_TYPE=sqlite3
         - DB_PATH=/app/data/tennis.db
         - WRAPPED_ACCESS_PASSWORD=${WRAPPED_ACCESS_PASSWORD}
       networks:
         - tennis-network
       healthcheck:
         test: ["CMD", "wget", "-qO-", "http://localhost:8080/"]
         interval: 30s
         timeout: 10s
         retries: 3
   
     # Redis for CourtHive
     redis:
       image: redis:7-alpine
       container_name: courthive-redis
       restart: unless-stopped
       command: redis-server --appendonly yes
       volumes:
         - redis-data:/data
       networks:
         - tennis-network
       healthcheck:
         test: ["CMD", "redis-cli", "ping"]
         interval: 30s
         timeout: 10s
         retries: 3
   
     # CourtHive API Server
     courthive-server:
       build:
         context: ../competition-factory-server
         dockerfile: Dockerfile
       container_name: courthive-server
       restart: unless-stopped
       depends_on:
         - redis
       volumes:
         - courthive-data:/app/data
         - courthive-cache:/app/cache
       environment:
         - APP_STORAGE=fileSystem
         - APP_NAME=Competition Factory Server
         - APP_MODE=production
         - APP_PORT=8383
         - JWT_SECRET=${COURTHIVE_JWT_SECRET}
         - JWT_VALIDITY=2h
         - TRACKER_CACHE=/app/cache
         - REDIS_TTL=28800000
         - REDIS_HOST=redis
         - REDIS_USERNAME=
         - REDIS_PASSWORD=
         - REDIS_PORT=6379
         - DB_HOST=localhost
         - DB_PORT=3838
         - DB_USER=admin
         - DB_PASS=adminpass
       networks:
         - tennis-network
       healthcheck:
         test: ["CMD", "wget", "-qO-", "http://localhost:8383/"]
         interval: 30s
         timeout: 10s
         retries: 3
   
     # Caddy reverse proxy with SSL
     caddy:
       image: caddy:2-alpine
       container_name: tennis-caddy
       restart: unless-stopped
       depends_on:
         - app
         - courthive-server
       ports:
         - "80:80"
         - "443:443"
       volumes:
         - ./Caddyfile:/etc/caddy/Caddyfile
         - ./TMX/dist:/srv/tournaments
         - ./courthive-public/dist:/srv/public
         - caddy-data:/data
         - caddy-config:/config
       networks:
         - tennis-network
   
     # Backup service for jim-dot-tennis database
     backup:
       image: alpine:latest
       container_name: jim-dot-tennis-backup
       restart: unless-stopped
       volumes:
         - tennis-data:/data:ro
         - tennis-backups:/backups
       depends_on:
         - app
       environment:
         - BACKUP_INTERVAL=86400
         - BACKUP_RETENTION=30
       command: >
         sh -c '
           apk add --no-cache sqlite tzdata &&
           while true; do
             DATE=$$(date +%Y-%m-%d-%H%M%S) &&
             echo "Creating backup at $${DATE}" &&
             mkdir -p /backups &&
             sqlite3 /data/tennis.db ".backup /backups/tennis-$${DATE}.db" &&
             echo "Backup completed" &&
             find /backups -name "tennis-*.db" -type f -mtime +$${BACKUP_RETENTION} -delete &&
             sleep $${BACKUP_INTERVAL};
           done
         '
       networks:
         - tennis-network
   
   networks:
     tennis-network:
       driver: bridge
   
   volumes:
     tennis-data:
       name: jim-dot-tennis-data
     tennis-backups:
       name: jim-dot-tennis-backups
     redis-data:
       name: courthive-redis-data
     courthive-data:
       name: courthive-data
     courthive-cache:
       name: courthive-cache
     caddy-data:
       name: caddy-data
     caddy-config:
       name: caddy-config
   ```

3. **Create comprehensive Caddyfile:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis
   ```
   
   Create `Caddyfile.courthive`:
   ```caddyfile
   jim.tennis {
     # CourtHive API endpoints
     handle /api/courthive/* {
       uri strip_prefix /api/courthive
       reverse_proxy courthive-server:8383
     }
     
     # Socket.io for real-time updates
     handle /socket.io/* {
       reverse_proxy courthive-server:8383
     }
     
     # Tournament admin interface (TMX)
     handle_path /tournaments/* {
       root * /srv/tournaments
       try_files {path} /index.html
       file_server
     }
     
     # Public tournament viewer
     handle_path /public/* {
       root * /srv/public
       try_files {path} /index.html
       file_server
     }
     
     # Jim-dot-tennis application (catch-all)
     handle {
       reverse_proxy app:8080 {
         header_up Service-Worker-Allowed {http.response.header.Service-Worker-Allowed}
       }
     }
     
     # Enable compression
     encode gzip
     
     # Security headers
     header {
       # Enable HSTS
       Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
       # Prevent clickjacking
       X-Frame-Options "SAMEORIGIN"
       # Prevent MIME sniffing
       X-Content-Type-Options "nosniff"
       # Enable XSS protection
       X-XSS-Protection "1; mode=block"
     }
   }
   ```

### Phase 2: Local Testing (2-4 hours)

#### Step 2.1: Test Integrated Stack Locally

1. **Update local environment variables:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis
   
   # Create .env file
   cat > .env << 'EOF'
   WRAPPED_ACCESS_PASSWORD=your-password-here
   COURTHIVE_JWT_SECRET=generate-a-very-long-random-string-here-use-pwgen-or-openssl
   EOF
   ```

2. **Build all containers:**
   ```bash
   docker-compose -f docker-compose.courthive.yml build
   ```

3. **Start all services:**
   ```bash
   docker-compose -f docker-compose.courthive.yml up -d
   ```

4. **Verify all services are running:**
   ```bash
   docker-compose -f docker-compose.courthive.yml ps
   docker-compose -f docker-compose.courthive.yml logs -f
   ```

5. **Test each endpoint:**
   - http://localhost/ - jim-dot-tennis home
   - http://localhost/admin/league - League admin
   - http://localhost/tournaments - TMX admin interface
   - http://localhost/public - Public tournament viewer
   - http://localhost/api/courthive/health - API health check

#### Step 2.2: Setup CourtHive Initial User

1. **Access TMX interface at http://localhost/tournaments**
2. **Login with test credentials:**
   - Username: `axel@castle.com`
   - Password: `castle`
3. **Create a provider** (top right user icon → Create Provider)
4. **Create your admin user** with that provider
5. **Logout and login with new user**
6. **Test tournament creation and persistence**

### Phase 3: Prepare Deployment Scripts (2-3 hours)

#### Step 3.1: Update Deployment Script

1. **Create new deployment script:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis/scripts
   cp deploy-digitalocean.sh deploy-courthive-integrated.sh
   ```

2. **Edit `deploy-courthive-integrated.sh`:**
   ```bash
   #!/bin/bash
   set -e
   
   # Jim.Tennis + CourtHive Deployment Script
   
   DROPLET_IP="144.126.228.64"
   SSH_USER="root"
   DEPLOY_DIR="/opt/jim-dot-tennis"
   APP_DOMAIN="jim.tennis"
   
   echo "========================================================"
   echo "Jim.Tennis + CourtHive Integrated Deployment"
   echo "========================================================"
   
   # Function to run remote commands
   function remote_command() {
     echo "Running: $1"
     ssh $SSH_USER@$DROPLET_IP "$1"
   }
   
   # Step 1: Test SSH connection
   echo "Testing SSH connection..."
   ssh -q $SSH_USER@$DROPLET_IP exit || {
     echo "Error: Cannot connect to server"
     exit 1
   }
   
   # Step 2: Build frontend applications locally
   echo "Building TMX frontend..."
   cd ../TMX
   pnpm install
   pnpm build
   
   echo "Building courthive-public frontend..."
   cd ../courthive-public
   pnpm install
   pnpm build
   
   cd ../jim-dot-tennis
   
   # Step 3: Create deployment archive including all projects
   echo "Creating deployment archive..."
   tar --exclude='.git' \
       --exclude='.vscode' \
       --exclude='node_modules' \
       --exclude='.cursor' \
       --exclude='tennis.db' \
       --exclude='.DS_Store' \
       -czf /tmp/tennis-integrated.tar.gz \
       -C .. \
       jim-dot-tennis \
       competition-factory-server \
       TMX/dist \
       courthive-public/dist
   
   # Step 4: Upload to server
   echo "Uploading deployment archive..."
   scp /tmp/tennis-integrated.tar.gz $SSH_USER@$DROPLET_IP:/tmp/
   
   # Step 5: Extract and setup on server
   echo "Extracting on server..."
   remote_command "cd /opt && tar -xzf /tmp/tennis-integrated.tar.gz && rm /tmp/tennis-integrated.tar.gz"
   
   # Step 6: Copy docker-compose and Caddyfile
   echo "Setting up Docker configuration..."
   scp docker-compose.courthive.yml $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/docker-compose.yml
   scp Caddyfile.courthive $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/Caddyfile
   
   # Step 7: Setup environment variables on server
   echo "Setting up environment variables..."
   remote_command "cd $DEPLOY_DIR && cat > .env << 'ENVEOF'
   WRAPPED_ACCESS_PASSWORD=\${WRAPPED_ACCESS_PASSWORD}
   COURTHIVE_JWT_SECRET=\${COURTHIVE_JWT_SECRET}
   ENVEOF"
   
   # Step 8: Install Redis if not present
   echo "Ensuring Redis is available..."
   remote_command "docker pull redis:7-alpine"
   
   # Step 9: Deploy with Docker Compose
   echo "Starting services with Docker Compose..."
   remote_command "cd $DEPLOY_DIR && docker-compose down && docker-compose pull && docker-compose up -d --build"
   
   # Step 10: Wait and verify
   echo "Waiting for services to start..."
   sleep 10
   
   echo "Checking service status..."
   remote_command "cd $DEPLOY_DIR && docker-compose ps"
   
   echo "========================================================"
   echo "Deployment completed!"
   echo "========================================================"
   echo "League Management: https://jim.tennis/admin/league"
   echo "Tournament Admin: https://jim.tennis/tournaments"
   echo "Public Tournaments: https://jim.tennis/public"
   echo "API Health: https://jim.tennis/api/courthive/health"
   
   # Cleanup
   rm /tmp/tennis-integrated.tar.gz
   ```

3. **Make script executable:**
   ```bash
   chmod +x scripts/deploy-courthive-integrated.sh
   ```

#### Step 3.2: Create Server Preparation Script

1. **Create `scripts/prepare-server-courthive.sh`:**
   ```bash
   #!/bin/bash
   set -e
   
   echo "Preparing server for CourtHive integration..."
   
   # Install system dependencies
   apt-get update
   apt-get install -y \
     docker.io \
     docker-compose \
     git \
     curl \
     wget \
     rsync
   
   # Start and enable Docker
   systemctl start docker
   systemctl enable docker
   
   # Create deployment directory
   mkdir -p /opt/jim-dot-tennis
   
   # Create system user for running services
   if ! id -u jimtennis > /dev/null 2>&1; then
     useradd -r -s /bin/false jimtennis
   fi
   
   # Add jimtennis user to docker group
   usermod -aG docker jimtennis
   
   # Set ownership
   chown -R jimtennis:jimtennis /opt/jim-dot-tennis
   
   echo "Server preparation complete!"
   ```

2. **Make executable:**
   ```bash
   chmod +x scripts/prepare-server-courthive.sh
   ```

### Phase 4: Server Deployment (2-4 hours)

#### Step 4.1: Backup Current Production System

1. **SSH into server:**
   ```bash
   ssh root@144.126.228.64
   ```

2. **Create backup:**
   ```bash
   cd /opt/jim-dot-tennis
   docker-compose exec app sqlite3 /app/data/tennis.db ".backup /app/data/tennis-pre-courthive-$(date +%Y%m%d).db"
   docker cp jim-dot-tennis:/app/data/tennis-pre-courthive-*.db ~/backups/
   
   # Also backup current config
   tar -czf ~/backups/jim-tennis-config-$(date +%Y%m%d).tar.gz \
     docker-compose.yml \
     Caddyfile \
     .env
   ```

#### Step 4.2: Prepare Server (if needed)

1. **Run preparation script:**
   ```bash
   scp scripts/prepare-server-courthive.sh root@144.126.228.64:/tmp/
   ssh root@144.126.228.64 "chmod +x /tmp/prepare-server-courthive.sh && /tmp/prepare-server-courthive.sh"
   ```

#### Step 4.3: Deploy Integrated System

1. **Set environment variables locally:**
   ```bash
   export WRAPPED_ACCESS_PASSWORD="your-password"
   export COURTHIVE_JWT_SECRET="$(openssl rand -base64 48)"
   ```

2. **Run deployment:**
   ```bash
   cd /home/jameshartt/Development/Tennis/jim-dot-tennis
   ./scripts/deploy-courthive-integrated.sh
   ```

3. **SSH to server and verify:**
   ```bash
   ssh root@144.126.228.64
   cd /opt/jim-dot-tennis
   docker-compose ps
   docker-compose logs -f courthive-server
   ```

#### Step 4.4: Configure DNS (if not already done)

Ensure your domain `jim.tennis` points to `144.126.228.64`:

```bash
# Check DNS propagation
dig jim.tennis
nslookup jim.tennis
```

### Phase 5: Post-Deployment Configuration (1-2 hours)

#### Step 5.1: Seed Production Admin User

**CRITICAL:** The test user (`axel@castle.com`) only works when `APP_MODE=development`. For production, you must create an admin user using the seed script.

1. **Ensure seed-admin.js is in the competition-factory-server directory:**

   The script should already exist at `/home/jameshartt/Development/Tennis/competition-factory-server/seed-admin.js`. If deploying fresh, create it from the template in the Implementation Notes section above.

2. **Copy seed script to production server:**
   ```bash
   # From local machine
   scp /path/to/competition-factory-server/seed-admin.js root@144.126.228.64:/tmp/
   ```

3. **Copy script into the running container:**
   ```bash
   ssh root@144.126.228.64
   docker cp /tmp/seed-admin.js courthive-server:/app/
   rm /tmp/seed-admin.js  # Clean up
   ```

4. **Create the admin user:**
   ```bash
   # Interactive method (recommended)
   docker exec -it courthive-server node /app/seed-admin.js admin@yourdomain.com 'YourSecurePassword123'

   # You will be prompted:
   # ⚠️  WARNING: This will create a SUPER_ADMIN user with full system access.
   # Email: admin@yourdomain.com
   # Are you sure you want to continue? (yes/no): yes
   ```

   **Expected output:**
   ```
   === CourtHive Admin User Creation ===
   ⚠️  This script requires LevelDB access (DB_USER, DB_PASS)

   Connecting to LevelDB...
   Checking if user already exists...
   Hashing password...
   Creating user: admin@yourdomain.com
   ✅ Admin user created successfully!
   Email: admin@yourdomain.com
   Roles: SUPER_ADMIN, ADMIN, DEVELOPER, CLIENT, SCORE
   ```

5. **Test login:**
   ```bash
   curl -X POST https://jim.tennis/api/courthive/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"admin@yourdomain.com","password":"YourSecurePassword123"}'

   # Should return: {"token":"eyJhbGc..."}
   ```

6. **Access TMX interface:**
   - Navigate to https://jim.tennis/tournaments
   - Login with your new admin credentials
   - You should now have access to create tournaments

7. **Create a provider (required for tournament persistence):**
   - Click user icon in top right → "Create Provider"
   - Fill in provider details (organization name, abbreviation, etc.)
   - This associates tournaments with your organization

8. **Test tournament creation and persistence:**
   - Create a test tournament
   - Restart the container:
     ```bash
     docker-compose restart courthive-server
     ```
   - Refresh the page - tournament should still exist

**Security Notes:**
- The seed script requires knowledge of DB_USER and DB_PASS (from docker-compose environment)
- Can only be run from inside the container or with direct server access
- Script includes interactive confirmation to prevent accidents
- Remove seed-admin.js from container after use: `docker exec courthive-server rm /app/seed-admin.js`

#### Step 5.2: Setup Monitoring

1. **Create monitoring script `scripts/health-check.sh`:**
   ```bash
   #!/bin/bash
   
   echo "=== Health Check for Jim.Tennis + CourtHive ==="
   echo ""
   
   echo "1. Jim-dot-tennis (League Management):"
   curl -s -o /dev/null -w "  Status: %{http_code}\n" https://jim.tennis/
   
   echo "2. Tournament Admin (TMX):"
   curl -s -o /dev/null -w "  Status: %{http_code}\n" https://jim.tennis/tournaments
   
   echo "3. Public Tournament Viewer:"
   curl -s -o /dev/null -w "  Status: %{http_code}\n" https://jim.tennis/public
   
   echo "4. CourtHive API:"
   curl -s -o /dev/null -w "  Status: %{http_code}\n" https://jim.tennis/api/courthive/
   
   echo ""
   echo "=== Docker Container Status ==="
   docker-compose ps
   
   echo ""
   echo "=== Redis Status ==="
   docker-compose exec redis redis-cli ping
   ```

2. **Add cron job for monitoring:**
   ```bash
   # On server, add to crontab
   */15 * * * * /opt/jim-dot-tennis/scripts/health-check.sh >> /var/log/tennis-health.log 2>&1
   ```

---

## 6. Docker Configuration

### Detailed Service Configuration

#### jim-dot-tennis Container
- **Image:** Custom (built from Golang Dockerfile)
- **Exposed internally:** 8080
- **Volumes:**
  - `tennis-data:/app/data` - SQLite database
  - Template and static file mounts for development
- **Networks:** tennis-network
- **Dependencies:** None (standalone)

#### Redis Container
- **Image:** `redis:7-alpine`
- **Exposed internally:** 6379
- **Volumes:** `redis-data:/data` for persistence
- **Purpose:** Cache and session storage for CourtHive
- **Configuration:** Append-only file (AOF) for durability

#### courthive-server Container
- **Image:** Custom (built from NestJS application)
- **Exposed internally:** 8383
- **Volumes:**
  - `courthive-data:/app/data` - Tournament data storage
  - `courthive-cache:/app/cache` - Cache directory
- **Dependencies:** Redis must be healthy
- **Environment:** Production mode with file system storage

#### Caddy Container
- **Image:** `caddy:2-alpine`
- **Exposed externally:** 80, 443
- **Volumes:**
  - Caddyfile for configuration
  - Built TMX and courthive-public static files
  - Persistent data for SSL certificates
- **Purpose:** Reverse proxy, SSL termination, static file serving
- **Dependencies:** All backend services

### Volume Management

```bash
# List all volumes
docker volume ls | grep tennis

# Backup volumes
docker run --rm -v jim-dot-tennis-data:/data -v $(pwd):/backup alpine tar czf /backup/tennis-data-backup.tar.gz /data

# Inspect volume
docker volume inspect jim-dot-tennis-data
```

---

## 7. Nginx/Caddy Reverse Proxy Setup

### Why Caddy?

- Automatic HTTPS with Let's Encrypt
- Simpler configuration than Nginx
- Built-in certificate management
- WebSocket support out of the box

### Caddyfile Breakdown

```caddyfile
jim.tennis {
  # API routing - strip /api/courthive prefix before proxying
  handle /api/courthive/* {
    uri strip_prefix /api/courthive
    reverse_proxy courthive-server:8383
  }
  
  # WebSocket support for real-time updates
  handle /socket.io/* {
    reverse_proxy courthive-server:8383
  }
  
  # SPA routing for TMX - always serve index.html for client-side routing
  handle_path /tournaments/* {
    root * /srv/tournaments
    try_files {path} /index.html
    file_server
  }
  
  # SPA routing for public viewer
  handle_path /public/* {
    root * /srv/public
    try_files {path} /index.html
    file_server
  }
  
  # Default to jim-dot-tennis for all other routes
  handle {
    reverse_proxy app:8080
  }
}
```

### Alternative: Nginx Configuration

If you prefer Nginx, here's an equivalent configuration:

```nginx
upstream jim_tennis {
    server app:8080;
}

upstream courthive_api {
    server courthive-server:8383;
}

server {
    listen 80;
    server_name jim.tennis;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name jim.tennis;
    
    ssl_certificate /etc/letsencrypt/live/jim.tennis/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/jim.tennis/privkey.pem;
    
    # CourtHive API
    location /api/courthive/ {
        rewrite ^/api/courthive/(.*) /$1 break;
        proxy_pass http://courthive_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # Socket.io
    location /socket.io/ {
        proxy_pass http://courthive_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
    
    # TMX Admin
    location /tournaments/ {
        alias /srv/tournaments/;
        try_files $uri $uri/ /tournaments/index.html;
    }
    
    # Public Viewer
    location /public/ {
        alias /srv/public/;
        try_files $uri $uri/ /public/index.html;
    }
    
    # Jim-dot-tennis
    location / {
        proxy_pass http://jim_tennis;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 8. Authentication & Authorization Integration

### Current State

**jim-dot-tennis:**
- Cookie-based session authentication
- Captains and players with different permissions
- Database-backed user system

**CourtHive:**
- JWT-based authentication
- Provider/user hierarchy
- Tournament ownership model

### Integration Options

#### Option A: Completely Separate (Recommended for Initial Deployment)

**Pros:**
- Fastest to implement
- No risk of breaking existing authentication
- Clean separation of concerns
- Users manage separate logins

**Cons:**
- Users need two accounts
- No single sign-on
- Duplicate user management

**Implementation:**
- No changes needed
- Each system maintains its own auth
- Link between systems via UI only

**Timeline:** 0 hours (already done)

#### Option B: Shared JWT Tokens (Medium Complexity)

**Approach:**
- jim-dot-tennis generates JWT tokens after login
- CourtHive accepts these tokens for API calls
- Shared secret between services

**Pros:**
- Single login for users
- jim-dot-tennis remains source of truth for users
- CourtHive APIs become accessible from league app

**Cons:**
- Requires modifying both codebases
- Need to map jim-dot-tennis users to CourtHive permissions
- More complex testing

**Implementation Steps:**

1. **Update jim-dot-tennis to generate JWTs:**
   ```go
   // Add to jim-dot-tennis
   import "github.com/golang-jwt/jwt/v5"
   
   func generateJWT(userID string, email string) (string, error) {
       token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
           "sub": userID,
           "email": email,
           "exp": time.Now().Add(time.Hour * 2).Unix(),
       })
       
       return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
   }
   ```

2. **Update CourtHive to accept external JWTs:**
   ```typescript
   // In competition-factory-server/src/auth/jwt.strategy.ts
   async validate(payload: any) {
       // Accept JWTs from jim-dot-tennis
       if (payload.source === 'jim-tennis') {
           return {
               userId: payload.sub,
               email: payload.email,
               provider: 'external',
           };
       }
       // ... existing validation
   }
   ```

3. **Add middleware to attach JWT to TMX requests:**
   ```typescript
   // In TMX frontend
   axios.interceptors.request.use((config) => {
       const token = localStorage.getItem('jim_tennis_token');
       if (token) {
           config.headers.Authorization = `Bearer ${token}`;
       }
       return config;
   });
   ```

**Timeline:** 8-12 hours

#### Option C: Full SSO Integration (High Complexity)

**Approach:**
- Implement OAuth2/OIDC provider in jim-dot-tennis
- CourtHive becomes OAuth2 client
- Centralized user management

**Pros:**
- Industry-standard approach
- Most flexible for future integrations
- Proper authorization flows

**Cons:**
- Significant development effort
- Requires OAuth2 library for Go
- Complex testing scenarios
- Potential for subtle bugs

**Timeline:** 20-30 hours

### Recommended Approach

**Start with Option A** (separate auth) for initial deployment. This gets everything running quickly and safely.

**Consider Option B** after initial deployment is stable if users request unified login. The JWT approach provides good middle ground between complexity and functionality.

**Reserve Option C** for future if you plan to add more services or need enterprise-grade SSO.

---

## 9. Deployment Process

### Pre-Deployment Checklist

- [ ] All local tests passing
- [ ] Frontend builds successful (TMX and courthive-public)
- [ ] Docker images build without errors
- [ ] Environment variables configured
- [ ] Database backup created
- [ ] DNS configured correctly
- [ ] SSL certificates ready (Caddy handles automatically)
- [ ] Rollback plan documented

### Deployment Command Sequence

```bash
# 1. Local preparation
cd /home/jameshartt/Development/Tennis/jim-dot-tennis
export WRAPPED_ACCESS_PASSWORD="your-password"
export COURTHIVE_JWT_SECRET="$(openssl rand -base64 48)"

# 2. Build frontends
cd ../TMX && pnpm build
cd ../courthive-public && pnpm build
cd ../jim-dot-tennis

# 3. Test locally first
docker-compose -f docker-compose.courthive.yml up --build

# 4. Test all endpoints locally
curl http://localhost/
curl http://localhost/tournaments
curl http://localhost/public
curl http://localhost/api/courthive/

# 5. Deploy to production
./scripts/deploy-courthive-integrated.sh

# 6. Verify on production
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose ps"
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose logs -f"

# 7. Test production endpoints
curl https://jim.tennis/
curl https://jim.tennis/tournaments
curl https://jim.tennis/public
curl https://jim.tennis/api/courthive/
```

### Post-Deployment Verification

1. **Check all services are running:**
   ```bash
   ssh root@144.126.228.64
   docker-compose ps
   # Should show: app, redis, courthive-server, caddy, backup
   ```

2. **Verify logs for errors:**
   ```bash
   docker-compose logs --tail=100 courthive-server
   docker-compose logs --tail=100 app
   docker-compose logs --tail=100 redis
   ```

3. **Test each route manually:**
   - https://jim.tennis/ - Should load league home page
   - https://jim.tennis/admin/league - Admin interface
   - https://jim.tennis/tournaments - TMX interface loads
   - https://jim.tennis/public - Public viewer loads
   - https://jim.tennis/api/courthive/ - Should return API response

4. **Check SSL certificate:**
   ```bash
   openssl s_client -connect jim.tennis:443 -servername jim.tennis
   ```

5. **Monitor resource usage:**
   ```bash
   docker stats
   ```

---

## 10. Testing & Validation

### Unit Testing

**jim-dot-tennis:**
```bash
cd /home/jameshartt/Development/Tennis/jim-dot-tennis
make test
```

**competition-factory-server:**
```bash
cd /home/jameshartt/Development/Tennis/competition-factory-server
pnpm test
```

### Integration Testing

Create test script `scripts/integration-test.sh`:

```bash
#!/bin/bash

BASE_URL="https://jim.tennis"
ERRORS=0

echo "=== Integration Testing ==="

# Test jim-dot-tennis
echo "Testing league management..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/)
if [ "$RESPONSE" != "200" ]; then
  echo "❌ League home failed: $RESPONSE"
  ERRORS=$((ERRORS+1))
else
  echo "✅ League home: OK"
fi

# Test TMX
echo "Testing TMX admin..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/tournaments/)
if [ "$RESPONSE" != "200" ]; then
  echo "❌ TMX admin failed: $RESPONSE"
  ERRORS=$((ERRORS+1))
else
  echo "✅ TMX admin: OK"
fi

# Test public viewer
echo "Testing public viewer..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/public/)
if [ "$RESPONSE" != "200" ]; then
  echo "❌ Public viewer failed: $RESPONSE"
  ERRORS=$((ERRORS+1))
else
  echo "✅ Public viewer: OK"
fi

# Test API
echo "Testing CourtHive API..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/api/courthive/)
if [ "$RESPONSE" != "200" ] && [ "$RESPONSE" != "404" ]; then
  echo "❌ API failed: $RESPONSE"
  ERRORS=$((ERRORS+1))
else
  echo "✅ API: OK"
fi

# Test WebSocket
echo "Testing WebSocket..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/socket.io/)
if [ "$RESPONSE" != "200" ] && [ "$RESPONSE" != "400" ]; then
  echo "❌ WebSocket failed: $RESPONSE"
  ERRORS=$((ERRORS+1))
else
  echo "✅ WebSocket: OK"
fi

echo ""
if [ $ERRORS -eq 0 ]; then
  echo "✅ All tests passed!"
  exit 0
else
  echo "❌ $ERRORS test(s) failed"
  exit 1
fi
```

### Manual Testing Checklist

**League Management (jim-dot-tennis):**
- [ ] Can access home page
- [ ] Can login as captain
- [ ] Can view fixtures
- [ ] Can update availability
- [ ] Can select team
- [ ] Static files load correctly

**Tournament Admin (TMX):**
- [ ] Can access /tournaments
- [ ] Can login with test user
- [ ] Can create provider
- [ ] Can create tournament
- [ ] Can add participants
- [ ] Can generate draws
- [ ] Tournament data persists after refresh

**Public Tournament Viewer:**
- [ ] Can access /public
- [ ] Can view tournament list
- [ ] Can view tournament details
- [ ] Can view match schedules
- [ ] Can view results

**API Integration:**
- [ ] API responds to health checks
- [ ] WebSocket connections work
- [ ] Real-time updates work in TMX
- [ ] Data saves correctly to filesystem/Redis

---

## 11. Rollback Strategy

### Quick Rollback (5 minutes)

If deployment fails, quickly revert to previous version:

```bash
# SSH to server
ssh root@144.126.228.64
cd /opt/jim-dot-tennis

# Stop new services
docker-compose down

# Restore old configuration
tar -xzf ~/backups/jim-tennis-config-YYYYMMDD.tar.gz

# Start old version
docker-compose up -d

# Verify
docker-compose ps
curl https://jim.tennis/
```

### Database Rollback

If database corruption occurs:

```bash
# SSH to server
ssh root@144.126.228.64

# Stop application
cd /opt/jim-dot-tennis
docker-compose stop app

# Restore database
docker cp ~/backups/tennis-pre-courthive-YYYYMMDD.db jim-dot-tennis:/app/data/tennis.db

# Restart
docker-compose start app
```

### Complete System Restore

For major issues:

```bash
# SSH to server
ssh root@144.126.228.64

# Stop everything
cd /opt/jim-dot-tennis
docker-compose down -v  # WARNING: Removes volumes

# Restore volumes from backup
docker run --rm -v jim-dot-tennis-data:/data -v ~/backups:/backup \
  alpine tar xzf /backup/tennis-data-backup.tar.gz -C /

# Deploy previous version
# (use old deployment script or restore from git)
```

---

## 12. Future Considerations

### Performance Optimization

1. **CDN for Static Assets**
   - Move TMX and courthive-public builds to CDN
   - Reduce server bandwidth
   - Improve global load times

2. **Database Optimization**
   - Consider PostgreSQL for CourtHive if data grows large
   - Add database connection pooling
   - Implement caching layer

3. **Container Optimization**
   - Multi-stage builds to reduce image sizes
   - Use Alpine-based images where possible
   - Implement healthchecks for auto-healing

### Monitoring & Observability

1. **Application Monitoring**
   - Add Prometheus for metrics
   - Implement Grafana dashboards
   - Set up alerts for downtime

2. **Log Aggregation**
   - Implement ELK stack or similar
   - Centralize logs from all services
   - Set up log-based alerts

3. **Uptime Monitoring**
   - Add external monitoring (UptimeRobot, Pingdom)
   - Configure notification channels
   - Monitor SSL certificate expiration

### Scaling Considerations

1. **Horizontal Scaling**
   - Move to Kubernetes for orchestration
   - Implement load balancing
   - Add Redis cluster for high availability

2. **Data Backup**
   - Implement off-site backups
   - Add automated backup testing
   - Create disaster recovery runbooks

3. **Geographic Distribution**
   - Add CDN for static assets
   - Consider multi-region deployment
   - Implement geo-routing

### Security Enhancements

1. **Network Security**
   - Implement firewall rules
   - Add rate limiting
   - Set up DDoS protection

2. **Application Security**
   - Regular dependency updates
   - Security scanning in CI/CD
   - Penetration testing

3. **Data Security**
   - Encrypt data at rest
   - Implement backup encryption
   - Add audit logging

### Authentication Integration (Long-term)

If you decide to pursue unified authentication:

1. **Phase 1: JWT Token Sharing** (Medium complexity, ~8-12 hours)
   - jim-dot-tennis generates JWT after login
   - CourtHive accepts jim-dot-tennis JWTs
   - Share JWT_SECRET between services
   - Map jim-dot-tennis users to CourtHive permissions

2. **Phase 2: Full OAuth2/OIDC** (High complexity, ~20-30 hours)
   - Implement OAuth2 provider in jim-dot-tennis
   - Configure CourtHive as OAuth2 client
   - Implement proper authorization flows
   - Add refresh token support

### Maintenance Tasks

**Weekly:**
- Review application logs
- Check disk space usage
- Monitor container resource usage
- Verify backup completion

**Monthly:**
- Update dependencies
- Review security advisories
- Test backup restore procedure
- Review and update documentation

**Quarterly:**
- Performance testing
- Security audit
- Disaster recovery drill
- Review and update deployment procedures

---

## Appendix A: Environment Variables Reference

**Note:** These are the ACTUAL environment variables used in the implementation. See `COURTHIVE_SETUP.md` for complete setup instructions.

### jim-dot-tennis (.env)
```bash
# Authentication
WRAPPED_ACCESS_PASSWORD=your-secure-password

# CourtHive JWT Secret (must match competition-factory-server)
COURTHIVE_JWT_SECRET=<generate-with-openssl-rand-base64-48>

# Note: PORT, DB_TYPE, DB_PATH are set in docker-compose.yml
```

### competition-factory-server (.env)
```bash
# Application
APP_STORAGE=fileSystem
APP_NAME=Competition Factory Server
APP_MODE=development
APP_PORT=8383

# JWT Authentication (must match jim-dot-tennis COURTHIVE_JWT_SECRET)
JWT_SECRET=<same-as-jim-dot-tennis-COURTHIVE_JWT_SECRET>
JWT_VALIDITY=2h

# Cache
TRACKER_CACHE=cache

# Redis
REDIS_TTL=28800000
REDIS_HOST=localhost  # Use 'redis' in Docker
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_PORT=6379

# Database (optional for LevelDB)
DB_HOST=localhost
DB_PORT=3838
DB_USER=admin
DB_PASS=adminpass

# Email (optional)
MAILGUN_API_KEY=
MAILGUN_HOST=api.eu.mailgun.net
MAILGUN_DOMAIN=
```

### docker-compose.courthive.yml Environment

**Important:** NODE_ENV=production is REQUIRED for courthive-server in Docker:

```yaml
courthive-server:
  environment:
    - NODE_ENV=production  # REQUIRED
    - APP_STORAGE=fileSystem
    - APP_MODE=production
    - JWT_SECRET=${COURTHIVE_JWT_SECRET}
    - REDIS_HOST=redis  # Docker service name
    # ... other vars
```

### TMX (.env.production)
```bash
# ACTUAL variables (not VITE_API_URL as originally planned)
SERVER=https://jim.tennis/api/courthive
ENVIRONMENT=production
BASE_URL=tournaments  # CRITICAL - required for asset paths
```

### TMX (.env.local) - for local testing
```bash
SERVER=http://localhost/api/courthive
ENVIRONMENT=development
BASE_URL=tournaments
```

### courthive-public (.env.production)
```bash
# Uses VITE_SERVER (different from TMX!)
VITE_SERVER=https://jim.tennis/api/courthive
ENVIRONMENT=production
BASE_URL=public  # CRITICAL - required for asset paths
```

### courthive-public (.env.local) - for local testing
```bash
VITE_SERVER=http://localhost/api/courthive
ENVIRONMENT=development
BASE_URL=public
```

**Key Differences from Original Plan:**
1. TMX uses `SERVER` not `VITE_API_URL`
2. courthive-public uses `VITE_SERVER` not `VITE_API_URL`
3. `BASE_URL` is required for both frontends (not in original plan)
4. `NODE_ENV=production` required in docker-compose for courthive-server

---

## Appendix B: Port Reference

| Service | Internal Port | External Port | Protocol |
|---------|--------------|---------------|----------|
| jim-dot-tennis | 8080 | - | HTTP |
| courthive-server | 8383 | - | HTTP/WS |
| Redis | 6379 | - | TCP |
| Caddy | - | 80, 443 | HTTP/HTTPS |

All external traffic flows through Caddy on ports 80/443.

---

## Appendix C: Useful Commands

### Docker Commands
```bash
# View all containers
docker-compose ps

# View logs
docker-compose logs -f [service_name]

# Restart a service
docker-compose restart [service_name]

# Rebuild and restart
docker-compose up -d --build [service_name]

# Execute command in container
docker-compose exec [service_name] [command]

# View resource usage
docker stats

# Clean up unused resources
docker system prune -a
```

### Database Commands
```bash
# Backup SQLite
docker-compose exec app sqlite3 /app/data/tennis.db ".backup /app/data/backup.db"

# Access SQLite shell
docker-compose exec app sqlite3 /app/data/tennis.db

# Redis CLI
docker-compose exec redis redis-cli

# Check Redis keys
docker-compose exec redis redis-cli KEYS '*'
```

### Debugging Commands
```bash
# Check if port is listening
ss -tlnp | grep [port]

# Test endpoint
curl -I https://jim.tennis/

# Check DNS
dig jim.tennis

# Test SSL
openssl s_client -connect jim.tennis:443 -servername jim.tennis

# View certificate
echo | openssl s_client -connect jim.tennis:443 2>/dev/null | openssl x509 -text
```

---

## Appendix D: Troubleshooting Guide

**Note:** This section has been updated with actual issues encountered during Phase 1 implementation.

### Issue: Module not found errors in courthive-server

**Symptoms:** Container fails to start with `Cannot find module 'compression'` or similar errors

**Root Cause:** Using `pnpm install --prod` skips devDependencies, but some packages listed as dev are needed at runtime

**Solutions:**
1. Update Dockerfile to use: `pnpm install --frozen-lockfile --ignore-scripts`
2. Rebuild container: `docker-compose up -d --build courthive-server`
3. Check logs for other missing modules: `docker-compose logs courthive-server`

**Actual Fix Applied:** Changed Dockerfile from `--prod` to `--frozen-lockfile` to install all dependencies.

### Issue: Health checks continuously fail

**Symptoms:** Container shows as "unhealthy" even though application appears to be running

**Root Causes:**
1. **Missing NODE_ENV:** courthive-server requires NODE_ENV environment variable
2. **Health check tool not installed:** `wget` not available in Alpine containers
3. **Wrong health check command:** Node.js HTTP check unreliable

**Solutions:**
1. Add `NODE_ENV=production` to docker-compose environment variables
2. Install curl in Dockerfile: `RUN apk add --no-cache curl`
3. Use simple curl check: `CMD curl -f http://localhost:8383/ || exit 1`

**Verification:**
```bash
docker-compose ps  # Should show "healthy"
docker-compose exec courthive-server curl http://localhost:8383/
```

### Issue: Services won't start

**Symptoms:** `docker-compose up` fails or services show as unhealthy

**Solutions:**
1. Check logs: `docker-compose logs [service]`
2. Verify environment variables are set
3. Check port conflicts: `ss -tlnp | grep [port]`
4. Ensure Docker has enough resources
5. Try rebuilding: `docker-compose up -d --build --force-recreate`

### Issue: Cannot access frontend applications

**Symptoms:** 404 errors on /tournaments or /public, or blank pages with missing assets

**Root Causes:**
1. **Frontend not built:** dist/ folders don't exist or are empty
2. **BASE_URL not set:** Vite builds using relative asset paths that break when served from subdirectories
3. **Wrong Caddyfile directive:** Using `handle` + `uri strip_prefix` instead of `handle_path`

**Solutions:**

**For 404 errors:**
1. Verify frontend builds exist:
   ```bash
   docker-compose exec caddy ls -la /srv/tournaments
   docker-compose exec caddy ls -la /srv/public
   ```
2. If missing, rebuild frontends:
   ```bash
   cd /path/to/TMX && pnpm build
   cd /path/to/courthive-public && pnpm build
   ```
3. Restart Caddy: `docker-compose restart caddy`

**For blank pages with asset errors (CRITICAL):**
1. Check browser console - if you see 404s for `/assets/` instead of `/tournaments/assets/`, BASE_URL is missing
2. Add BASE_URL to .env files:
   - TMX: `BASE_URL=tournaments`
   - courthive-public: `BASE_URL=public`
3. Rebuild frontends: `pnpm build`
4. Restart Caddy: `docker-compose restart caddy`

**Caddyfile configuration:**
Use `handle_path` for SPAs, not `handle` + `uri strip_prefix`:
```caddyfile
handle_path /tournaments* {
  root * /srv/tournaments
  try_files {path} /index.html
  file_server
}
```

**Actual Issue Encountered:** Frontends built without BASE_URL showed blank pages. Adding BASE_URL and using handle_path fixed it.

### Issue: API requests fail

**Symptoms:** Network errors in browser console for API calls

**Solutions:**
1. Verify courthive-server is running: `docker-compose ps courthive-server`
2. Check API health: `curl https://jim.tennis/api/courthive/`
3. Review server logs: `docker-compose logs courthive-server`
4. Verify Redis is running: `docker-compose exec redis redis-cli ping`
5. Check CORS configuration in courthive-server

### Issue: WebSocket connections fail

**Symptoms:** Real-time updates don't work in TMX

**Solutions:**
1. Verify WebSocket route in Caddyfile
2. Check browser console for WebSocket errors
3. Test WebSocket endpoint: `curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" https://jim.tennis/socket.io/`
4. Review courthive-server logs for WebSocket connections

### Issue: Homepage showing wrong content (admin login instead of landing page)

**Symptoms:** Visiting root URL shows admin login page instead of the intended landing page

**Root Cause:** Go template block naming conflict. Multiple templates defining blocks with the same names ("head", "content") in the shared layout cause one template to override another.

**Solution:** Make the landing page template (index.html) standalone:
1. Remove template inheritance - don't use `{{define "content"}}` blocks
2. Create complete HTML document with `{{define "index.html"}}` as the root block
3. Include full `<html>`, `<head>`, and `<body>` tags
4. Don't use the shared layout template

**Example:**
```html
{{define "index.html"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <title>Your Site</title>
    <!-- ... full head ... -->
</head>
<body>
    <!-- ... page content ... -->
</body>
</html>
{{end}}
```

**Actual Issue Encountered:** index.html and login.html both defined "content" blocks, causing login to override homepage.

### Issue: Tournament data doesn't persist

**Symptoms:** Created tournaments disappear after restart

**Solutions:**
1. Check volume mounts: `docker volume inspect courthive-data`
2. Verify APP_STORAGE is set to 'fileSystem'
3. Check directory permissions in container:
   ```bash
   docker-compose exec courthive-server ls -la /app/data
   ```
4. Review server logs for storage errors

### Issue: SSL certificate issues

**Symptoms:** Browser shows certificate warnings

**Solutions:**
1. Wait for Caddy to provision certificate (can take a few minutes)
2. Check Caddy logs: `docker-compose logs caddy | grep -i certificate`
3. Verify DNS is correctly pointing to server: `dig jim.tennis`
4. Ensure ports 80 and 443 are accessible from internet
5. Check Let's Encrypt rate limits

### Issue: Cannot login - test user not working in production

**Symptoms:** Login with `axel@castle.com` / `castle` returns 401 Unauthorized

**Root Cause:** Test user only exists when `APP_MODE=development`. In production mode, the test user array is not checked.

**Solution:** Use the seed-admin.js script to create a production admin user. See Phase 5, Step 5.1 for complete instructions.

### Issue: Seed script fails with "Cannot find module 'bcrypt'"

**Symptoms:**
```
Error: Cannot find module 'bcrypt'
```

**Root Cause:** The project uses `bcryptjs` (pure JavaScript implementation), not the native `bcrypt` module.

**Solution:** Update seed script to use `require('bcryptjs')` instead of `require('bcrypt')`.

### Issue: User created but login fails with bcrypt error

**Symptoms:**
```
Error: Illegal arguments: string, undefined
    at bcryptjs compare
```

**Root Cause:** User was stored as JSON string instead of object. The @gridspace/net-level-client expects objects and handles JSON serialization automatically.

**Solution:**
1. Use `db.put(email, userObject)` NOT `db.put(email, JSON.stringify(userObject))`
2. Delete the incorrectly formatted user:
   ```bash
   docker exec courthive-server node -e "
   const nl = require('@gridspace/net-level-client');
   (async () => {
     const db = new nl();
     await db.open('localhost', 3838);
     await db.auth('admin', 'adminpass');
     await db.use('user', {create: true});
     await db.del('user@example.com');
     process.exit(0);
   })();
   "
   ```
3. Recreate user with corrected seed script

### Issue: Seed script cannot connect to LevelDB

**Symptoms:**
```
Error: Could not connect to user in 1000 ms
```

**Root Cause:** `net-level-server` is not running on port 3838.

**Solution:**
1. Check if net-level-server is running in courthive-server container:
   ```bash
   docker exec courthive-server ps aux | grep net-level
   ```
2. Verify docker-compose.courthive.yml has the correct command:
   ```yaml
   command: sh -c "npx net-level-server & node build/main.js"
   ```
3. Check the logs for net-level-server startup:
   ```bash
   docker logs courthive-server | grep "net-level"
   # Should see: 'starting net-level on port 3838 serving "data"'
   ```

---

## Conclusion

This comprehensive plan provides a complete roadmap for deploying CourtHive alongside jim-dot-tennis. The deployment is structured in phases, allowing for incremental progress and testing at each step.

**Estimated Total Time:**
- Phase 1 (Local Setup): 5-8 hours
- Phase 2 (Local Testing): 2-4 hours
- Phase 3 (Deployment Scripts): 2-3 hours
- Phase 4 (Server Deployment): 2-4 hours
- Phase 5 (Post-Deployment): 1-2 hours

**Total: 12-21 hours** (depending on experience level and issues encountered)

The plan prioritizes:
1. **Safety:** Backup and rollback strategies at every step
2. **Incrementality:** Test locally before deploying to production
3. **Simplicity:** Start with separate authentication, add complexity later if needed
4. **Documentation:** Comprehensive troubleshooting and reference sections

Remember to:
- Take your time with each phase
- Test thoroughly at each step
- Keep backups of all configuration
- Document any deviations from this plan
- Update this document with lessons learned

Good luck with your deployment!
