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
| `HOME_CLUB_ID` | Yes (or `HOME_CLUB_NAME`) | Database ID of your club |
| `HOME_CLUB_NAME` | Fallback | Club name for fuzzy lookup if `HOME_CLUB_ID` is not set |
| `HOME_CLUB_LOGO_PATH` | Optional | URL path to your club logo (default: `/static/st-anns-tennis.jpg`). See "Club logo" below. |
| `BHPLTA_CLUB_CODE` | For imports | Your club's code on the BHPLTA website (e.g. `STANN001`) |
| `DB_PATH` | No | Database file path (default: `./tennis.db`) |
| `COURTHIVE_API_URL` | No | CourtHive API URL (if using tournament management) |

You'll also want to update:
- Domain name and Caddy configuration
- Any club-specific branding in templates (optional — templates use the club name from the database)

### Club logo

The points table and weekly overview pages display a club logo. To swap St Ann's default for your own:

1. Drop your logo (JPG/PNG/SVG) into the `static/` directory — e.g. `static/my-club.jpg`. If deploying via Docker, mount it as a volume at `/app/static/my-club.jpg` instead of rebuilding the image.
2. Set `HOME_CLUB_LOGO_PATH=/static/my-club.jpg` in your environment.
3. Leave it unset to keep the default St Ann's logo, or set it to an empty string to hide the logo entirely.

### What's already handled

These used to require code changes but are now built in:

- **Home/away team logic** — determined by `HOME_CLUB_ID`, not hardcoded
- **Club name in templates** — injected dynamically from the database via middleware
- **Club logo** — configurable via `HOME_CLUB_LOGO_PATH`
- **Divisions label on the points table** — computed dynamically from the divisions your teams are playing in
- **BHPLTA club code** — configured via `BHPLTA_CLUB_CODE` environment variable
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
