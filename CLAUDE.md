# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Jim.Tennis is a club-agnostic tennis league management system, originally built at St Ann's Tennis Club for the Brighton and Hove Parks Tennis League. The home club is configured via environment variables (`HOME_CLUB_ID` / `HOME_CLUB_NAME`), making the system deployable for any parks league club. It handles player availability, team selection, fixture scheduling, match results tracking, and integrates with the BHPLTA (Brighton & Hove Parks League Tennis Association) website to import match cards.

**Key Technical Stack:**
- Go 1.26+
- SQLite (the only supported database)
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

### Go Unit Tests (in Docker)
```bash
make test                                 # Run all Go unit tests (CGO/sqlite via Docker)
make test PKG=./internal/database/...     # Test a single package
make test ARGS="-run TestName -v"         # Pass flags through to `go test`
# `make check` runs vet + fmt + lint + deadcode + test.
```

### Fixture Generator Engine Tests (Node in Docker)
```bash
make test-js    # node --test tests/fixture-generator/engine.test.mjs (also part of `make check`)
```

### E2E Testing (Playwright in Docker)
```bash
make test-e2e                      # Run full E2E test suite (default: 2 workers)
make test-e2e WORKERS=4            # Run with more parallel browser tabs
make test-e2e-headed               # Headed mode (default: 1 worker/tab)
make test-e2e-headed WORKERS=3     # Headed mode with 3 browser tabs
make test-e2e-grep FILTER="login"  # Run tests matching pattern
make test-e2e-failed               # Re-run only previously failed tests
make test-e2e-report               # Open HTML test report
make test-e2e-results              # Formatted test summary (Claude-friendly)
make test-e2e-multiclub            # Run multi-club verification tests
make test-e2e-fresh                # Fresh-install suite: EMPTY volumes, setup flow, saved BHPLTA pages (own compose project)
make test-e2e-clean                # Tear down E2E containers, delete the E2E volume and artifacts

# Test infrastructure:
# - tests/e2e/              Playwright project root (213 tests across 30 spec files)
# - tests/e2e/helpers/      Reusable helpers (auth, htmx, navigation, assertions)
# - tests/e2e/fixtures/     Seed data (seed.sql) and test fixtures (test-fixtures.ts)
# - tests/e2e/admin/        Admin page specs (dashboard, navigation, CRUD, points, wrapped)
# - tests/e2e/players/      Player page specs (availability, profile, standings)
# - tests/e2e/workflows/    Multi-step workflow specs (team selection, match results, fixtures)
# - tests/e2e/accessibility.spec.ts  axe-core WCAG 2.0 A/AA audits
# - tests/e2e/responsive.spec.ts     Mobile (375×812) and tablet (768×1024) viewport tests
# - tests/e2e/smoke.spec.ts Smoke tests validating the full stack
# - tests/e2e/auth.spec.ts  Authentication flow tests (login, logout, rate limit)
# - tests/e2e/scripts/parse-results.mjs  JSON results → Claude-friendly summary
# - tests/e2e/global-setup.ts  Shared auth session via storageState (skipped when FRESH_INSTALL is set)
# - tests/e2e/fresh-install*.spec.ts  `fresh-install` project (only with `make test-e2e-fresh`; excluded from `make test-e2e`)
# - Dockerfile.e2e          Playwright container (includes sqlite3 for seeding)
# - E2E runs against its own `app-e2e` service and `jim-dot-tennis-e2e-data` volume (never the dev/prod
#   data volume or the port-8080 app); `make test-e2e-bootstrap` prepares a fresh volume automatically
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
./bin/extract-nonce -club-code="$BHPLTA_CLUB_CODE" -verbose

# Import match cards from BHPLTA
./bin/import-matchcards \
  -auto-nonce \
  -club-code="$BHPLTA_CLUB_CODE" \
  -week=1 \
  -year=2024 \
  -club-id=$HOME_CLUB_ID \
  -club-name="Your Club Name" \
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
- `config/`: Home club resolution and the live `HomeClubHolder`, `AppConfig` snapshot for CLI tools, context helpers, and middleware for injecting the home club into request context
- `setup/`: First-run "needs setup" state, the setup gate middleware and the setup pages
- `secrets/`: Per-instance secrets (generated Club Wrapped code) and the startup self-check; `outbound/` holds only a test that fails on any URL host not in its allowlist (keep `docs/outbound_connections.md` in step)
- `normalize/`: Centralised apostrophe/Unicode normalisation utilities
- `database/`: Database connection and migration handling
- `models/`: Data structures (Player, Team, Fixture, etc.)
- `repository/`: Data access layer (one per entity)
- `auth/`: Authentication service, middleware, and handlers
- `accounts/`: The one account service (create, list, enable/disable, role, password reset, session revocation, last-active-administrator guard, audit seam `recordAudit`), used by the admin users/sessions pages and the server command line `jim-dot-tennis user ... | setup code` (`cmd/jim-dot-tennis/cli.go`; help text reproduced in `docs/PRODUCTION_QUICK_REFERENCE.md` section 10, kept in step by a test). Also `/account/password`, where users with `must_change_password` are sent by `RequireAuth`
- `admin/`: Admin interface handlers (dashboard, players, fixtures, teams, users, sessions, match card import, points, club wrapped)
- `players/`: Player-facing handlers (availability management)
- `services/`: Business logic (match card parsing, player matching, nonce extraction)
- `webpush/`: Push notification service
- `fixturegen/`: Public fixture generator page (`/fixture-generator`) and its league-tables feed
- `fixturefile/`: Reader/validator for the shareable season fixture file (`jim.tennis/fixtures`)
- `bhplta/`: Season bootstrap from the BHPLTA website: `Source.Fetch` (network, no DB) then `Import` in a caller-owned transaction; `Bootstrapper.Run` does both

**static/fixture-generator/**: The fixture generator itself, which runs in the browser. `engine.js` holds the promotion/relegation rules, the scheduler and the file format (no DOM, tested by `tests/fixture-generator`); `app.js`/`app.css` are the interface. See `docs/fixture_generator.md`.

**migrations/**: SQL migration files (up/down pairs)

**tests/e2e/**: Playwright E2E browser tests (213 tests across 30 spec files)
- `helpers/`: Reusable test helpers (auth, htmx, navigation, assertions)
- `fixtures/`: SQL seed data and Playwright test fixtures
- `admin/`, `players/`: Page-level spec files
- `workflows/`: Multi-step workflow specs (team selection, match results, fixture management)
- `accessibility.spec.ts`: axe-core WCAG audits
- `responsive.spec.ts`: Mobile/tablet viewport tests
- `scripts/parse-results.mjs`: JSON results → Claude-friendly summary

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
   - Login at `/login` → redirects to `/admin/league/dashboard`. Two ways in, one session: a passkey (the login page leads with it) or username and password. Both end in `auth.Service.completeLogin` → `createSession`, so the sessions are identical
   - Sessions stored in database with expiry (7 days default)
   - Middleware: `auth.Middleware.RequireAuth()` + `RequireRole("admin")`
   - Protected routes: `/admin/league/*`

2. **Token-based (Fantasy Players)**: URL-embedded tokens for player availability
   - URL pattern: `/my-availability/{token}` where token is composite player names
   - Middleware: `auth.Middleware.RequireFantasyTokenAuth()`
   - No session, passwordless access for specific players

**Rate Limiting:** Password login attempts tracked per username and address (5 attempts per 15 minutes). Passkey attempts are throttled per client address only, in memory (`internal/auth/passkey_ceremony.go`), and are never recorded against a username, so they cannot lock a named user out.

**Passkeys (WebAuthn)** (`internal/auth/passkey*.go`, `internal/repository/webauthn_credential.go`, `static/passkey.js`, `github.com/go-webauthn/webauthn`; operator notes in `docs/PRODUCTION_QUICK_REFERENCE.md` section 11):
- Tables (migration 033): `webauthn_credentials` (one row per passkey; `credential_id` unique across users) and `webauthn_user_handles` (a random 32-byte handle per user, created on first enrolment; discoverable sign-in resolves the account from it, never from a username or numeric id).
- Routes: `POST /login/passkey/begin|finish` (public; optional `{"username"}` for the username-first path, which answers with decoy credential ids for unknown names), `GET /account/signin-methods`, `POST /account/passkeys/begin|finish`, `POST /account/passkeys/{id}/rename|delete` (all `RequireAuth`, own passkeys only), `POST /account/passkeys/{id}/clear-warning` (admin).
- The relying party is built on every request from `ClubSettings.ResolvePublicURL()` (`Passkeys.relyingPartyAddress`): `Origin` is the only accepted origin, `Hostname` the RP ID. No public URL means no passkeys, except `DEV_MODE=true` (never production) with a request to `localhost`, which uses `http://localhost:<port>`. The Host header is otherwise never used. Changing the public host invalidates existing passkeys.
- A ceremony's challenge lives in memory (`ceremonyStore`), keyed by a random ticket in the HttpOnly SameSite=Strict `passkey_ceremony` cookie, for 5 minutes, and is removed on the first finish attempt whatever the outcome. Registration ceremonies are also tied to the user and session that began them.
- A sign counter that does not increase sets `clone_warning_at`: that sign-in is refused and the passkey stays blocked until an administrator clears it on the sign-in methods page (dashboard shows a banner).
- `repository.CountSignInMethods` (via `auth.Service.CountSignInMethods`) is the one definition of "sign-in method" (passkeys + a non-empty password); the repository's `Delete` refuses to remove the last one. `Passkeys.SetAudit` is the audit seam (wired to `accounts.Service.RecordEvent` in main.go).
- `must_change_password` is enforced by `RequireAuth` on the session, so it applies however the user signed in: a passkey sign-in for such a user lands on `/account/password` like a password sign-in.
- Cross-site protection is the app-wide `auth.CrossOriginGuard` (all passkey endpoints are POST); the JSON endpoints also require `Content-Type: application/json`.

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

### Configuration

**Home Club:** resolved in the order `HOME_CLUB_ID` env, `HOME_CLUB_NAME` env, then the `home_club_id` row in `app_settings` (written by first-run setup). With none of them the server still starts, in setup mode: `internal/setup` gates every request except `/static/`, `/branding/`, `/healthz` and `/setup...` to the setup page. The setup flow (`/setup` code, `/setup/admin`, `/setup/club`, `/setup/done`) is protected by a one-time code printed to the log at startup (only its hash is stored); code entry and administrator creation are 404 once any administrator exists. A home club in effect with no administrator account (captain/player accounts do not count) is also gated (`setup.StatusNeedsAdmin`): a code is printed and the flow creates the administrator, then goes straight to the dashboard. The club step calls `setup.HomeClubProvisioner`; the server uses `setup.NewBHPLTAProvisioner`, which runs in the background (the page polls) and imports the league, all clubs (with venue details), and the current season's weeks, divisions, teams and fixtures from BHPLTA via `internal/bhplta` in one transaction with the completion settings. The same import is the admin "Start a new season from BHPLTA" action (`/admin/league/seasons/from-bhplta`) and `cmd/import-season`. No migration seeds user accounts. An env value matching none of the clubs in a non-empty clubs table is fatal at startup (lists the available clubs). The home club is held in a live `config.HomeClubHolder`, never a startup-frozen value.
- `HOME_CLUB_ID`: Database ID of the home club (preferred)
- `HOME_CLUB_NAME`: Club name for fuzzy lookup (fallback if `HOME_CLUB_ID` is not set)
- `HOME_CLUB_LOGO_PATH`: Optional URL path to a club logo image; overrides the logo uploaded on the settings page. Unset (or empty) means the uploaded logo, else the bundled logo of a club in `config.KnownBHPLTAClub`'s list (St Ann's: `/static/st-anns-tennis.jpg`), else no logo.
- `BHPLTA_CLUB_CODE`: Club code on the BHPLTA website — used for match card imports; overrides the code stored on the settings page
- `PUBLIC_URL`: The instance's public base URL; overrides the one stored on the settings page. Required (valid, https) when `APP_ENV=production`: `config.ValidateStartup` (called from main.go) refuses to start otherwise. Read it only through `ClubSettings.ResolvePublicURL()` (validated `PublicURL{Origin, Base, Scheme, Hostname}`; passkeys and recovery links use this, Hostname is the relying-party ID) or `ClubSettings.BaseURL(r)` / `Host(r)` for links and identifiers, which fall back to the request host only when no public URL is set (development); that fallback lives in `config.RequestBaseURL`. The web push VAPID subscriber (`PushSubscriber()`) and the iCal UID domain (`UID:fixture-<id>@<public host>`) derive from it. Changing the public host changes iCal UIDs, so calendar apps treat existing subscribed events as new: keep the host stable (the live site's host is the UID domain its subscriptions already use).

**Club settings page** (`/admin/league/settings`, admin role only): display name, logo, primary/secondary colours, BHPLTA club code/name/id and public URL, stored in `app_settings` and held live in `config.ClubSettings` (`HomeClubHolder.ClubSettings()`); changes apply without a restart. A non-empty env variable above always wins over the stored value and the page says so. Uploaded logos (PNG/JPEG/WebP, sniffed by content, max 2 MB, no SVG) are stored in `branding/` next to the SQLite database (`DB_PATH`'s directory, i.e. the data volume) and served only at `/branding/<content-hash name>`. `internal/branding` also serves `/static/manifest.json`, `/api/manifest` and `/static/service-worker.js` with the stored name, logo and colours. BHPLTA's club name and id are not published: the page can look them up from the club code, `config.KnownBHPLTAClub` covers clubs whose values are known, and `config.PrefillClubSettings` stores them during setup.

**Features:**
- `WRAPPED_ACCESS_PASSWORD`: Optional override for the Club Wrapped access code. Unset (the default) means a code generated with crypto/rand on first start and stored in `app_settings` (`wrapped_access_code`), shown to admins on the Club Settings page (`internal/secrets`). The grant cookie is derived from the code, so it is per instance.
- `MY_TENNIS_ENABLED`: `true`/`1` (or `false`/`0`) overrides the stored My Tennis switch on the settings page (Features section, with Tournaments and Club Wrapped). Defaults with nothing stored: My Tennis off, Club Wrapped on, Tournaments on unless setup_completed_at exists (new installs). A switch that is off hides its links and its routes return 404; with Tournaments off the home page is a club page.
- `LEAGUE_TABLES_URL`: Optional source page for the fixture generator's league tables (default: BHPLTA league tables page)
- `BHPLTA_BASE_URL`: Testing only; site root the BHPLTA season import reads the fixtures and club pages from (default `https://www.bhplta.co.uk`). Saved pages for tests are in `tests/e2e/fixtures/bhplta/`
- `COURTHIVE_API_URL`: Optional CourtHive API URL (default: `http://courthive-server:8383`)

**Database:**
- `DB_TYPE`: optional; only "sqlite3" is supported (any other value fails at startup)
- `DB_PATH`: SQLite file path (default: "./tennis.db")

**Application:**
- `PORT`: HTTP server port (default: "8080")
- `DEV_MODE`: `true`/`1` is the only way to turn off Secure session cookies (plain-http development). Set by the root `docker-compose.yml` (app, app-e2e, app-fresh), `make run-local` and `scripts/run.sh`; fatal with `APP_ENV=production`. Any new way of serving the app over http must set it.
- `APP_ENV`: Set to "production" for a live instance (the reference compose sets it): needs a valid https `PUBLIC_URL` and refuses `DEV_MODE`. In production the startup self-check (`secrets.StartupCheck`) refuses to start if a secret variable holds a known placeholder such as `change-me`. It also warns (in every mode) about accounts whose password hash is one seeded by old migrations or the e2e seed.
- `TEMPLATE_RELOAD`: Development only; `true` re-parses templates on every request (root `docker-compose.yml` defaults it to true)
- `MIGRATE_ALLOW_DIRTY_FORCE`: Development only; `true` auto-forces a dirty migration version at startup. Never set in production.

**Reference deployment:** `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example` describe the production stack (app, CourtHive, Caddy); `.env.example` lists every variable the stack and the app read. `scripts/deploy-app.sh` does not sync them to the server. The root `docker-compose.yml` is for local development and E2E. Superseded variants are in `docs/archive/`.

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
- A dirty migration state fails startup; auto-forcing is dev-only behind `MIGRATE_ALLOW_DIRTY_FORCE=true`

## Key Files

- `cmd/jim-dot-tennis/main.go`: Application entry point, routing setup
- `internal/config/config.go`: Home club resolution order (`ResolveHomeClub`) and `AppConfig` for CLI tools
- `internal/config/holder.go`: Live, concurrency-safe home club (`HomeClubHolder`) used by the server
- `internal/setup/`: Setup state (`State`, `Gate`, `MarkComplete`) and the setup entry page
- `internal/repository/settings.go`: Typed access to the `app_settings` key/value table
- `internal/config/context.go`: Context helpers for passing config through request context
- `internal/config/middleware.go`: Middleware that injects config into every request
- `internal/normalize/normalize.go`: Centralised apostrophe/Unicode normalisation
- `internal/database/database.go`: Database connection and migration execution
- `internal/admin/handler.go`: Admin route registration
- `internal/players/handler.go`: Player route registration
- `internal/auth/middleware.go`: Authentication middleware
- `internal/services/matchcard_service.go`: BHPLTA integration orchestration
