# Production Quick Reference - jim.tennis

**Server:** 144.126.228.64
**Domain:** jim.tennis
**SSH:** `ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64`
**Path:** `/opt/jim-dot-tennis/`
**Go Version:** 1.25 (Sprint 004)

---

## Quick Commands

### 1. Update Go Application (jim-dot-tennis)

```bash
# Transfer and rebuild
cd ~/Development/Tennis/jim-dot-tennis
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" internal/ root@144.126.228.64:/opt/jim-dot-tennis/internal/
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" cmd/ root@144.126.228.64:/opt/jim-dot-tennis/cmd/
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" templates/ root@144.126.228.64:/opt/jim-dot-tennis/templates/
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" static/ root@144.126.228.64:/opt/jim-dot-tennis/static/
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose build app && docker compose up -d app"
```

**Time:** ~2 minutes
**Downtime:** ~10 seconds

---

### 2. Update TMX Frontend

```bash
# Build and transfer
cd ~/Development/Tennis/TMX
nvm use 24
SERVER="https://jim.tennis/api/courthive" ENVIRONMENT="production" BASE_URL="tournaments" pnpm build
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" dist/ root@144.126.228.64:/opt/jim-dot-tennis/TMX/dist/
```

**Time:** ~1 minute
**Downtime:** None

---

### 3. Update Public Frontend

```bash
# Build and transfer
cd ~/Development/Tennis/courthive-public
nvm use 20
VITE_API_URL="https://jim.tennis/api/courthive" ENVIRONMENT="production" pnpm build
rsync -avz --delete -e "ssh -i ~/.ssh/digital_ocean_ssh" dist/ root@144.126.228.64:/opt/jim-dot-tennis/courthive-public/dist/
```

**Time:** ~1 minute
**Downtime:** None

---

### 4. Update CourtHive Server

```bash
# Transfer and rebuild
cd ~/Development/Tennis/competition-factory-server
rsync -avz --exclude='node_modules' --exclude='dist' --exclude='.git' -e "ssh -i ~/.ssh/digital_ocean_ssh" ./ root@144.126.228.64:/opt/jim-dot-tennis/competition-factory-server/
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose build courthive-server && docker compose up -d courthive-server"
```

**Time:** ~3 minutes
**Downtime:** ~15 seconds
**Warning:** Interrupts active API users

---

### 5. View Logs

```bash
# All services
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose logs -f"

# Specific service (jim-dot-tennis, courthive-server, tennis-caddy, courthive-redis)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker logs -f jim-dot-tennis"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker logs --tail 50 courthive-server"
```

---

### 6. Check Status

```bash
# Container status
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker compose ps"

# Resource usage
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker stats --no-stream"

# Test endpoints
curl -I https://jim.tennis/
curl -I https://jim.tennis/admin/league
curl -I https://jim.tennis/tournaments
curl -I https://jim.tennis/public
curl https://jim.tennis/api/courthive/
```

---

### 7. Restart Services

```bash
# Restart specific service
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose restart app"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose restart courthive-server"

# Restart all
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose restart"

# Full restart (down/up)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker compose down && docker compose up -d"
```

---

### 8. Create CourtHive User

```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker exec -i courthive-server node" << 'EOFNODE'
const bcrypt = require('bcryptjs');
const netLevelClient = require('@gridspace/net-level-client');

async function createAdminUser() {
  const db = new netLevelClient();
  try {
    await db.open('localhost', 3838);
    await db.auth('admin', 'adminpass');
    await db.use('user', { create: true });

    const email = 'newuser@example.com';  // CHANGE THIS
    const password = 'SecurePassword123!';  // CHANGE THIS

    const hashedPassword = await bcrypt.hash(password, 10);
    const user = {
      email,
      roles: ['superadmin', 'admin', 'developer', 'client', 'score'],
      permissions: ['devMode'],
      password: hashedPassword,
      firstName: 'New',
      lastName: 'User',
    };

    await db.put(email, user);
    console.log('User created: ' + email);
    await db.close();
  } catch (error) {
    console.error('Error:', error.message);
  }
}

createAdminUser();
EOFNODE
```

---

### 9. Backups

```bash
# List backups
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker run --rm -v jim-dot-tennis-backups:/backups alpine ls -lh /backups"

# Manual backup
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker run --rm -v jim-dot-tennis-data:/data -v jim-dot-tennis-backups:/backups alpine sh -c 'apk add sqlite && sqlite3 /data/tennis.db \".backup /backups/manual-\$(date +%Y%m%d-%H%M%S).db\"'"

# Download backup
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker run --rm -v jim-dot-tennis-backups:/backups alpine cat /backups/tennis-YYYYMMDD-HHMMSS.db" > ~/backups/tennis-backup.db

# CourtHive data backup
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." > ~/backups/courthive-$(date +%Y%m%d).tar.gz
```

---

## Go Tooling (Sprint 004)

All commands run inside Docker (no local Go install required). Uses `golang:1.25-alpine`.

```bash
# Static analysis
make vet              # go vet ./...

# Formatting
make fmt              # Check formatting (list unformatted files)
make fmt-fix          # Fix formatting in-place

# Import ordering
make imports          # Check import ordering
make imports-fix      # Fix import ordering in-place

# Linting
make lint             # golangci-lint run ./...

# Dead code detection
make deadcode         # Find unreachable functions

# Module maintenance
make tidy             # go mod tidy

# Run all read-only checks at once
make check            # Runs: vet, fmt, lint, deadcode
```

---

## Admin Routes

### Core Admin (all under `/admin/league/`)

| Route | Purpose |
|---|---|
| `/admin/league/dashboard` | Dashboard - stat cards, quick actions (4 categories: League Management, Results & Standings, Season Tools, System) |
| `/admin/league/players` | Player management, filtering |
| `/admin/league/fixtures` | Fixture management, week overview |
| `/admin/league/teams` | Team management |
| `/admin/league/clubs` | Club management |
| `/admin/league/divisions/` | Division editing (Sprint 003) |
| `/admin/league/users` | User management - list, create, toggle active, password reset (Sprint 003) |
| `/admin/league/sessions` | Session management - view active, revoke (Sprint 003) |
| `/admin/league/points-table` | Points/standings table |
| `/admin/league/match-card-import` | BHPLTA match card import |
| `/admin/league/club-data-import` | Club data import |
| `/admin/league/seasons` | Season management, set active |
| `/admin/league/seasons/setup` | Season setup wizard |
| `/admin/league/selection-overview` | Selection overview |
| `/admin/league/wrapped` | Club Wrapped / season summary |
| `/admin/league/preferred-names` | Preferred name approvals |

### Public Routes

| Route | Purpose |
|---|---|
| `/club/wrapped` | Public Season Wrapped (access-cookie protected) |

---

## Service Architecture (Sprint 003)

`service.go` was split into 12 domain-specific files in `internal/admin/`:

| File | Domain |
|---|---|
| `service_dashboard.go` | Dashboard stats and overview |
| `service_players.go` | Player CRUD and queries |
| `service_fixtures.go` | Fixture management |
| `service_fixture_players.go` | Player-fixture assignments |
| `service_teams.go` | Team operations |
| `service_matchups.go` | Matchup logic |
| `service_seasons.go` | Season management |
| `service_divisions.go` | Division operations |
| `service_clubs.go` | Club management |
| `service_fantasy.go` | Fantasy league |
| `service_users_sessions.go` | User and session management |
| `service_selection.go` | Team selection logic |

---

## Troubleshooting

### Site not responding
```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker compose ps"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker logs --tail 50 tennis-caddy"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker compose restart caddy"
```

### API errors
```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker logs --tail 100 courthive-server"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker exec courthive-redis redis-cli ping"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker compose restart courthive-server"
```

### Admin login issues
```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker logs --tail 50 jim-dot-tennis | grep -i auth"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker exec jim-dot-tennis ls -lh /app/data/tennis.db"
```

### TMX login issues
```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker exec courthive-server ps aux | grep net-level"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 "docker exec courthive-server cat /app/data/.users"
```

---

## Credentials

**jim-dot-tennis admin:**
- User: james.hartt
- Login: https://jim.tennis/admin/league

**CourtHive/TMX admin:**
- Email: jameshartt@gmail.com
- Password: SecureP@ssw0rd2026!
- Login: https://jim.tennis/tournaments

**Environment secrets:**
- WRAPPED_ACCESS_PASSWORD: st.anns.2025
- COURTHIVE_JWT_SECRET: (in /opt/jim-dot-tennis/.env)

---

## URLs

- **Main site:** https://jim.tennis/
- **League admin:** https://jim.tennis/admin/league
- **Tournament admin (TMX):** https://jim.tennis/tournaments
- **Public tournaments:** https://jim.tennis/public
- **CourtHive API:** https://jim.tennis/api/courthive/

---

## Build Times

- jim-dot-tennis: ~60-90 seconds
- courthive-server: ~90-120 seconds
- TMX: ~30-60 seconds
- courthive-public: ~20-40 seconds

---

## Resource Usage

- Total: ~210MB / 961MB (22%)
- jim-dot-tennis: ~20MB
- courthive-server: ~120MB
- Redis: ~8MB
- Caddy: ~15MB
