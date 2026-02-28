# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Jim.Tennis is a tennis league management system built for St Ann's Tennis Club to manage the Brighton and Hove Parks Tennis League. The system handles player availability, team selection, fixture scheduling, match results tracking, and integrates with the BHPLTA (Brighton & Hove Parks League Tennis Association) website to import match cards.

**Key Technical Stack:**
- Go 1.24+
- SQLite (default) or PostgreSQL
- Server-side rendered templates with HTMX
- PWA with push notifications
- Docker for production deployment

## Development Commands

### Local Development
```bash
# Build and run locally (creates database at ./tennis.db)
make local

# Or run individual commands
make build-local      # Build to bin/jim-dot-tennis
make run-local        # Build and run
make clean-local      # Clean binary and database

# Server runs at http://localhost:8080
```

### Database Management
```bash
# Migrations run automatically on startup
# To manually migrate:
go run cmd/migrate/main.go

# To rollback:
go run cmd/migrate-down/main.go
```

### Docker (Production)
```bash
make build           # Build Docker images
make run             # Start containers
make stop            # Stop containers
make logs            # View all logs
make app-logs        # View app logs only
make backup          # Create manual backup
```

### E2E Testing (Playwright in Docker)
```bash
make test-e2e              # Run full E2E test suite
make test-e2e-grep FILTER="login"  # Run tests matching pattern
make test-e2e-failed       # Re-run only previously failed tests
make test-e2e-report       # Open HTML test report
make test-e2e-results      # Output JSON results
make test-e2e-clean        # Tear down test containers and clean artifacts

# Test infrastructure:
# - tests/e2e/              Playwright project root
# - tests/e2e/helpers/      Reusable helpers (auth, htmx, navigation, assertions)
# - tests/e2e/fixtures/     Seed data (seed.sql) and test fixtures (test-fixtures.ts)
# - tests/e2e/smoke.spec.ts Smoke tests validating the full stack
# - Dockerfile.e2e          Playwright container (includes sqlite3 for seeding)
# - Seeded admin: testadmin / testpassword123
# - Seeded fantasy token: Sabalenka_Djokovic_Gauff_Sinner
```

### Utility Commands
```bash
# Build all utility tools
make build-utils

# Populate database from CSV fixtures
go build -o bin/populate-db cmd/populate-db/main.go
./bin/populate-db -verbose -dry-run

# Extract WordPress nonce from BHPLTA
./bin/extract-nonce -club-code="STANN001" -verbose

# Import match cards from BHPLTA
./bin/import-matchcards \
  -auto-nonce \
  -club-code="STANN001" \
  -week=1 \
  -year=2024 \
  -club-id=123 \
  -club-name="St Ann's Tennis Club" \
  -db="./tennis.db" \
  -verbose \
  -dry-run
```

## Architecture Overview

### Application Structure

The codebase follows a clean layered architecture:

**cmd/**: Entry points and utility commands
- `jim-dot-tennis/`: Main web application
- `migrate/`, `migrate-down/`: Database migration tools
- `populate-db/`: Bulk CSV import for fixtures
- `extract-nonce/`: WordPress nonce extraction for BHPLTA
- `import-matchcards/`: Match result import from BHPLTA
- `generate-fantasy-matches/`: Fantasy match generation utility

**internal/**: Core application logic
- `database/`: Database connection and migration handling
- `models/`: Data structures (Player, Team, Fixture, etc.)
- `repository/`: Data access layer (one per entity)
- `auth/`: Authentication service, middleware, and handlers
- `admin/`: Admin interface handlers (dashboard, players, fixtures, teams, users, sessions, match card import, points, club wrapped)
- `players/`: Player-facing handlers (availability management)
- `services/`: Business logic (match card parsing, player matching, nonce extraction)
- `webpush/`: Push notification service

**migrations/**: SQL migration files (up/down pairs)

**tests/e2e/**: Playwright E2E browser tests
- `helpers/`: Reusable test helpers (auth, htmx, navigation, assertions)
- `fixtures/`: SQL seed data and Playwright test fixtures
- `*.spec.ts`: Test spec files

**templates/**: HTML templates for server-side rendering
- `admin/`: Admin interface templates
- `players/`: Player interface templates
- Base templates: `index.html`, `layout.html`, `login.html`

### Database Schema

**Core Hierarchy:**
- Season → Weeks (1-18 per season)
- League → Divisions → Teams
- Club → Teams → Players
- Fixture (links Team, Week, Division, Season)
- Matchup (Men's/Women's/Mixed doubles within a Fixture)

**Key Tables:**
- `seasons`, `weeks`: Time organization
- `leagues`, `divisions`: Competition structure
- `clubs`, `teams`, `players`: Organizational entities
- `fixtures`, `matchups`: Match scheduling and results
- `player_availability`: Player availability tracking
- `fixture_players`: Player assignments to fixtures
- `tennis_players`: External player database from BHPLTA
- `users`, `sessions`: Authentication
- `push_subscriptions`, `vapid_keys`: Push notifications

### Repository Pattern

Each domain entity has a dedicated repository in `internal/repository/`:
- Encapsulates all database operations for that entity
- Provides CRUD and entity-specific query methods
- Used by service layer to access data

Key repositories: `SeasonRepository`, `WeekRepository`, `LeagueRepository`, `DivisionRepository`, `ClubRepository`, `TeamRepository`, `PlayerRepository`, `FixtureRepository`, `MatchupRepository`, `AvailabilityRepository`

### Authentication & Authorization

**Two Authentication Modes:**

1. **Session-based (Admin)**: Cookie-based authentication with role-based access control
   - Login at `/login` → redirects to `/admin/league/dashboard`
   - Sessions stored in database with expiry (7 days default)
   - Middleware: `auth.Middleware.RequireAuth()` + `RequireRole("admin")`
   - Protected routes: `/admin/league/*`

2. **Token-based (Fantasy Players)**: URL-embedded tokens for player availability
   - URL pattern: `/my-availability/{token}` where token is composite player names
   - Middleware: `auth.Middleware.RequireFantasyTokenAuth()`
   - No session, passwordless access for specific players

**Rate Limiting:** Login attempts tracked (5 attempts per 15 minutes)

### BHPLTA Integration

**Match Card Import Flow:**
1. Extract WordPress nonce from BHPLTA website (automated via `nonce_extractor.go`)
2. Fetch match card HTML from BHPLTA API using nonce
3. Parse HTML to extract match details (`matchcard_parser.go`)
4. Match players to local database (`player_matcher.go`)
5. Import match results and update fixture status

**Services:**
- `NonceExtractor`: Scrapes BHPLTA website for WordPress nonce
- `MatchCardParser`: Parses match card HTML into structured data
- `PlayerMatcher`: Fuzzy matches external player names to internal database
- `MatchCardService`: Orchestrates the import process

### Template Rendering

**Server-Side Rendering:**
- Templates loaded with custom functions (e.g., `currentYear`)
- Admin handlers render to `templates/admin/*.html`
- Players handlers render to `templates/players/*.html`
- Uses Go's `html/template` package

**Sub-Handler Pattern:**
Each major handler (Admin, Players) delegates to domain-specific sub-handlers:
- `admin.Handler` → `DashboardHandler`, `PlayersHandler`, `FixturesHandler`, etc.
- `players.Handler` → `AvailabilityHandler`

### Database Configuration

**Environment Variables:**
- `DB_TYPE`: "sqlite3" (default) or "postgres"
- `DB_PATH`: SQLite file path (default: "./tennis.db")
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`: PostgreSQL config
- `PORT`: HTTP server port (default: "8080")
- `APP_ENV`: Set to "production" to enforce secure cookies

### CSV Import Format

Fixture CSV files expected format:
```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
10,June 19,,,Hove,Dyke A
```

Club names extracted from team names (e.g., "Dyke A" → Club: "Dyke").

## Development Patterns

### Adding a New Entity

1. Define model in `internal/models/`
2. Create migration files in `migrations/` (up and down)
3. Create repository in `internal/repository/`
4. Add service logic if needed in `internal/services/`
5. Create handlers and templates for UI

### Adding a New Admin Feature

1. Create sub-handler in `internal/admin/` (e.g., `new_feature.go`)
2. Register routes in `internal/admin/handler.go` `RegisterRoutes()`
3. Add templates in `templates/admin/`
4. Apply `authMiddleware.RequireAuth()` + `RequireRole("admin")`

### Working with Migrations

- Migrations are numbered sequentially: `001_`, `002_`, etc.
- Always create both `.up.sql` and `.down.sql` files
- Migrations run automatically on app startup
- Database handles dirty state by forcing version

## Key Files

- `cmd/jim-dot-tennis/main.go`: Application entry point, routing setup
- `internal/database/database.go`: Database connection and migration execution
- `internal/admin/handler.go`: Admin route registration
- `internal/players/handler.go`: Player route registration
- `internal/auth/middleware.go`: Authentication middleware
- `internal/services/matchcard_service.go`: BHPLTA integration orchestration
