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
  database/           Database connection and migrations
  models/             Data structures
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

jim.tennis was built for St Ann's Tennis Club in the Brighton & Hove Parks League. It's open-source and could be adapted for another parks league club, but it wasn't designed as a multi-tenant platform. Here's an honest overview of what's involved.

### What you'd get

- Player availability tracking with token-based access (no passwords for players)
- Team selection workflow with captain notifications
- Fixture scheduling with venue details and iCal feeds
- Match result import from the BHPLTA website
- Standings and match history
- Push notifications via PWA
- Admin dashboard for managing everything

### What you'd need

**Technical skills:** Comfort with Docker, the command line, and ideally some Go knowledge for customisation. You don't need to be a Go expert for basic configuration, but deeper changes will require it.

**A server:** jim.tennis runs on a single small VPS (1 CPU, 1GB RAM, 2GB swap). See [docs/digitalocean_deployment.md](docs/digitalocean_deployment.md) for the current setup.

### Configuration changes (straightforward)

These are environment variables and config that you'd change for your club:

- `DB_PATH` / database connection settings
- `COURTHIVE_API_URL` (if using tournament management)
- Club name and branding in templates
- BHPLTA club code (currently `STANN001`)
- Domain name and Caddy configuration

### Code changes required (moderate effort)

These require editing Go source files:

- **Home/away team logic** — the system determines home vs away by checking the ClubID against St Ann's club ID. This logic appears across multiple files in `internal/admin/` and `internal/players/`. You'd need to change the reference ClubID or make it configurable.
- **Hardcoded club references** — various template strings and service logic reference "St Ann's" directly. A search for "St Ann" across the codebase will surface these (there are roughly 28 files with references).
- **BHPLTA integration** — the match card import assumes the BHPLTA's website structure and data format. If your league uses a different website, you'd need to adapt the scraper in `internal/services/`.

### Known pain points (significant effort)

**The apostrophe problem.** The name "St Ann's" contains an apostrophe that appears differently across data sources — the BHPLTA website, match cards, CSV imports, and the internal database all use different apostrophe characters (`'`, `'`, `ʼ`). This creates matching issues throughout the system. It's the single biggest barrier to clean multi-club support and is documented tech debt, not something we've solved yet. If your club name doesn't contain special characters, this won't affect you.

### Rough effort estimate

| Category | Effort |
|----------|--------|
| Deployment to a new server | A few hours if you're comfortable with Docker |
| Changing the club name and branding | Half a day |
| Updating home/away ClubID logic | A day or two, depending on Go familiarity |
| Adapting BHPLTA integration for a different league | Significant — depends entirely on the target website |

### Getting help

If you're thinking about adapting jim.tennis for your club, the easiest way to reach James is through the **Parks Team Captains** WhatsApp group — if you're already in the group, just drop a message there.

You can also [open a club enquiry issue](https://github.com/jameshartt/jim-dot-tennis/issues/new?template=club_enquiry.md) on GitHub. Happy to answer questions about what's involved.
