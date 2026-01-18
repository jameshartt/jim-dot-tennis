# CourtHive Deployment Planning Document

## Executive Summary

This document provides a comprehensive, step-by-step plan to deploy the CourtHive tournament management system alongside the existing jim-dot-tennis application on a single DigitalOcean droplet. The deployment will integrate three CourtHive TypeScript/Node.js components with the existing Go-based calendar and team management system.

---

## Table of Contents

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

### Phase 1: Prepare Local Development Environment

**Objective:** Get all four applications running locally with the new routing structure

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

#### Step 5.1: Setup CourtHive Admin User

1. **Access https://jim.tennis/tournaments**
2. **Login with test user: axel@castle.com / castle**
3. **Create provider and admin user**
4. **Test tournament creation**
5. **Verify data persists across container restarts:**
   ```bash
   ssh root@144.126.228.64
   docker-compose restart courthive-server
   # Check if tournament data is still available
   ```

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

### jim-dot-tennis (.env)
```bash
# Application
PORT=8080
DB_TYPE=sqlite3
DB_PATH=/app/data/tennis.db

# Authentication
WRAPPED_ACCESS_PASSWORD=your-secure-password

# JWT (if implementing shared auth)
JWT_SECRET=shared-secret-with-courthive
```

### competition-factory-server (.env)
```bash
# Application
APP_STORAGE=fileSystem
APP_NAME=Competition Factory Server
APP_MODE=production
APP_PORT=8383

# JWT Authentication
JWT_SECRET=shared-secret-with-jim-tennis
JWT_VALIDITY=2h

# Cache
TRACKER_CACHE=/app/cache

# Redis
REDIS_TTL=28800000
REDIS_HOST=redis
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

### TMX (.env.production)
```bash
VITE_API_URL=https://jim.tennis/api/courthive
VITE_SOCKET_URL=https://jim.tennis
```

### courthive-public (.env.production)
```bash
VITE_API_URL=https://jim.tennis/api/courthive
```

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

### Issue: Services won't start

**Symptoms:** `docker-compose up` fails or services show as unhealthy

**Solutions:**
1. Check logs: `docker-compose logs [service]`
2. Verify environment variables are set
3. Check port conflicts: `ss -tlnp | grep [port]`
4. Ensure Docker has enough resources
5. Try rebuilding: `docker-compose up -d --build --force-recreate`

### Issue: Cannot access frontend applications

**Symptoms:** 404 errors on /tournaments or /public

**Solutions:**
1. Verify frontend builds exist:
   ```bash
   docker-compose exec caddy ls -la /srv/tournaments
   docker-compose exec caddy ls -la /srv/public
   ```
2. Check Caddyfile configuration
3. Restart Caddy: `docker-compose restart caddy`
4. Check Caddy logs: `docker-compose logs caddy`

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
