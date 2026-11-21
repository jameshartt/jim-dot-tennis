# Contributing to jim.tennis

Thanks for your interest in contributing. This guide covers two things: how to contribute code, and how to adapt jim.tennis for your own club.

## Developer Guide

### Prerequisites

- **Docker** — all Go builds run in Docker, so you don't need Go installed locally
- **Make** — the project uses Makefile targets for common operations
- **Git** — for version control

### Setup

```bash
git clone https://github.com/jameshartt/jim-dot-tennis.git
cd jim-dot-tennis

# Build and run locally (creates database at ./tennis.db, serves at localhost:8080)
make local
```

### Running tests

```bash
# Full E2E test suite (runs Playwright in Docker — no local Node.js needed)
make test-e2e

# Run tests matching a pattern
make test-e2e-grep FILTER="login"

# View formatted results
make test-e2e-results

# Open the HTML report
make test-e2e-report
```

### Code style

The project uses Docker-based Go tooling for formatting and static analysis:

```bash
make check            # Run all checks (vet, lint, fmt, imports, deadcode)
make fmt-fix          # Auto-fix formatting
make imports-fix      # Auto-fix imports
```

The `.golangci.yml` configuration enables 11 linters. Run `make lint` to check before submitting.

### Submitting changes

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run `make check` and `make test-e2e` to verify everything passes
5. Submit a pull request using the [PR template](.github/PULL_REQUEST_TEMPLATE.md)

Keep PRs focused — one feature or fix per PR where practical.

### Project structure

```
cmd/                  Entry points and CLI utilities
internal/
  admin/              Admin interface handlers and service layer
  auth/               Authentication (session-based and token-based)
  config/             Home club configuration, context helpers, middleware
  database/           Database connection and migrations
  models/             Data structures
  normalize/          Apostrophe/Unicode normalisation utilities
  players/            Player-facing handlers
  repository/         Data access layer (one per entity)
  services/           Business logic (match card parsing, venue resolution, etc.)
  webpush/            Push notification service
migrations/           SQL migration files (up/down pairs)
templates/            Server-rendered HTML templates
  admin/              Admin interface templates
  players/            Player-facing templates
static/               CSS, JavaScript, images, PWA assets
tests/e2e/            Playwright browser tests
docs/                 Deployment and operations documentation
```

### Database migrations

Migrations are numbered sequentially (`001_`, `002_`, etc.) and run automatically on startup.

```bash
# Migrations live in migrations/ as .up.sql and .down.sql pairs
# To manually run or rollback:
docker run ... go run cmd/migrate/main.go
docker run ... go run cmd/migrate-down/main.go
```

Always create both up and down migration files.

---

## Club Adaptation Guide

jim.tennis was originally built for St Ann's Tennis Club in the Brighton & Hove Parks League. Since then, the architecture has been made club-agnostic — the home club is configured via environment variables, so you no longer need to edit Go source files to run it for your own club.

### What you'd get

- Player availability tracking with token-based access (no passwords for players)
- Team selection workflow with captain notifications
- Fixture scheduling with venue details and iCal feeds
- Match result import from the BHPLTA website
- Standings and match history
- Push notifications via PWA
- Admin dashboard for managing everything

### What you'd need

**Technical skills:** Comfort with Docker and the command line. You don't need Go knowledge for basic setup — all club-specific configuration is done through environment variables.

**A server:** jim.tennis runs on a single small VPS (1 CPU, 1GB RAM, 2GB swap). See [docs/digitalocean_deployment.md](docs/digitalocean_deployment.md) for the current setup.

### Configuration (environment variables)

Set these environment variables to run jim.tennis for your club:

| Variable | Required | Description |
|----------|----------|-------------|
| `HOME_CLUB_ID` | Unless chosen in setup | Database ID of your club. Overrides `HOME_CLUB_NAME` and the club stored by first-run setup; with none of the three the app starts in setup mode (`/setup`) |
| `HOME_CLUB_NAME` | Fallback | Club name for fuzzy lookup if `HOME_CLUB_ID` is not set |
| `HOME_CLUB_LOGO_PATH` | Optional | URL path to a club logo; overrides the logo uploaded on the Club Settings page. See "Club logo" below. |
| `BHPLTA_CLUB_CODE` | Optional | Your club's code on the BHPLTA website; overrides the code stored on the Club Settings page |
| `WRAPPED_ACCESS_PASSWORD` | Optional | Code that unlocks the club wrapped pages. When unset (recommended) each instance generates its own code on first start; administrators see it on the Club Settings page. A placeholder value such as `change-me` stops the app starting with `APP_ENV=production`. |
| `MY_TENNIS_ENABLED` | Optional | `true`/`1` shows the "My Tennis" call-to-action. Off when unset. |
| `LEAGUE_TABLES_URL` | Optional | Source page for the public fixture generator's league tables (default: the BHPLTA league tables page). |
| `COURTHIVE_API_URL` | Optional | CourtHive API URL (default: `http://courthive-server:8383`; only relevant with tournament management). |
| `PUBLIC_URL` | Production | The instance's public address, e.g. `https://tennis.example.org`. Overrides the Public URL stored on the Club Settings page. Required (https only) when `APP_ENV=production`: the app refuses to start without it. Used for links sent to players, the push-notification contact, calendar-file UIDs and (later) passkeys. The reference compose defaults it to `https://$DOMAIN`. |
| `APP_ENV` | Production | Set to `production` for a live instance: requires a valid https `PUBLIC_URL` and refuses `DEV_MODE`. The reference compose sets it. |
| `DEV_MODE` | Dev only | `true`/`1` stops session cookies being marked Secure, so the app works over plain http (local runs, the e2e containers; set by the root `docker-compose.yml`, `make run-local` and `scripts/run.sh`). Session cookies are Secure by default. Never set in production. |
| `PORT` | Optional | HTTP listen port (default: `8080`). |
| `DB_TYPE` | Optional | Only `sqlite3` is supported; any other value fails at startup. |
| `DB_PATH` | Optional | SQLite file path (default: `./tennis.db`; `/app/data/tennis.db` in Docker). |
| `TEMPLATE_RELOAD` | Dev only | `true` re-parses templates on every request (hot reload). Leave unset in production. |

You'll also want to update:
- Domain name and Caddy configuration
- Any club-specific branding in templates (optional — templates use the club name from the database)

### Club logo

The points table and weekly overview pages display a club logo. Upload it on **Admin Dashboard → Club Settings** (PNG, JPEG or WebP, up to 2 MB). It is stored on the data volume next to the database, so it survives upgrades, and a square logo of at least 192 pixels is also used as the installed app's icon. The same page sets the display name, the two club colours and the BHPLTA identifiers.

`HOME_CLUB_LOGO_PATH` still works and takes precedence: point it at an image you serve yourself (for example one mounted into `/app/static/`). Without either there is no logo.

### What's already handled

These used to require code changes but are now built in:

- **Home/away team logic** — determined by `HOME_CLUB_ID`, not hardcoded
- **Club name in templates** — injected dynamically from the database via middleware
- **Club name, logo and colours** — set on the Club Settings page (`HOME_CLUB_LOGO_PATH` overrides the logo)
- **Divisions label on the points table** — computed dynamically from the divisions your teams are playing in
- **BHPLTA club code, name and id** — set on the Club Settings page (`BHPLTA_CLUB_CODE` overrides the code)
- **Apostrophe normalisation** — a centralised normalisation layer (`internal/normalize/`) handles the various Unicode apostrophe characters that appear across data sources

### Remaining customisation (if needed)

- **BHPLTA integration** — the match card import assumes the BHPLTA's website structure and data format. If your league uses a different website, you'd need to adapt the scraper in `internal/services/`.
- **Tournament management** — the CourtHive integration is optional and can be disabled by not running the CourtHive stack.

### Rough effort estimate

| Category | Effort |
|----------|--------|
| Deployment to a new server | A few hours if you're comfortable with Docker |
| Configuring for your club | Set 2-3 environment variables |
| Customising branding/templates | Optional — a few hours if you want custom styling |
| Adapting for a non-BHPLTA league | Significant — depends entirely on the target website |

### Getting help

If you're thinking about adapting jim.tennis for your club, the easiest way to reach James is through the **Parks Team Captains** WhatsApp group — if you're already in the group, just drop a message there.

You can also [open a club enquiry issue](https://github.com/jameshartt/jim-dot-tennis/issues/new?template=club_enquiry.md) on GitHub. Happy to answer questions about what's involved.
