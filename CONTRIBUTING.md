# Contributing to jim-dot-tennis

Thanks for your interest in contributing. This guide covers two things: how to contribute code, and what adapting the software for your own club involves. Running it for your club does not need any of the developer material: start with [docs/install.md](docs/install.md).

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
make test                 # Go unit tests (in Docker; PKG=./internal/... narrows them)
make test-js              # fixture generator engine tests

# Browser tests (Playwright in Docker, no local Node.js needed)
make test-e2e             # main suite, on its own app instance and data volume
make test-e2e-auth        # passkeys, invite and recovery links, password fallback
make test-e2e-fresh       # an empty install and the first-run setup flow
make test-e2e-multiclub   # the app configured for a different home club
make test-e2e-bundle      # the shipped deploy/ stack with tournaments on and off (needs the CourtHive images)

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
make check            # vet, gofmt, lint, deadcode, Go tests and fixture generator tests
make fmt-fix          # Auto-fix formatting
make imports-fix      # Auto-fix imports
```

`.golangci.yml` is in golangci-lint v2 format. The linter version is pinned in the
`Makefile` (`GOLANGCI_LINT_VERSION`); `make lint` and CI both use it, so a bump is
a one-line change there (check `make lint` still passes, and `golangci-lint migrate`
if the config format changes). Run `make lint` before submitting.

### Continuous integration

`.github/workflows/ci.yml` runs on pushes to `main` and on every pull request, as
separate checks that each call a make target:

| Check | Runs |
|-------|------|
| `Lint` | `make vet`, a failing gofmt check, `make lint` |
| `Unit tests` | `make test`, `make test-js` |
| `E2E (main suite)` | `make test-e2e` |
| `E2E (passkeys and auth)` | `make test-e2e-auth` |
| `E2E (fresh install)` | `make test-e2e-fresh` |
| `E2E (multi-club)` | `make test-e2e-multiclub` |

When an E2E job fails, the Playwright HTML report, traces and screenshots are
attached to the run as an artifact (`playwright-*`, kept 14 days); download it and
run `npx playwright show-trace trace.zip`. A new push to a pull request cancels the
superseded run. Workflows use read-only permissions and no secrets, so pull requests
from forks run the same checks. Third-party actions are pinned to full commit SHAs
with the version in a comment; Dependabot (`.github/dependabot.yml`) opens grouped
weekly update pull requests for Go modules, Actions, Docker base images, the
Playwright image and the e2e npm packages. The Playwright image tag in
`Dockerfile.e2e` and `@playwright/test` in `tests/e2e/package.json` must be the same
version: merge their two Dependabot PRs together (or bump both in one PR).
`GOLANGCI_LINT_VERSION` and the Go image in the `Makefile` are not tracked by
Dependabot; bump them by hand.

**Maintainer setup (once, in the GitHub web UI):** Settings, Branches, add a branch
protection rule (or ruleset) for `main`: tick "Require a pull request before merging"
and "Require status checks to pass before merging" (and "Require branches to be up to
date"), then add the required checks `Lint`, `Unit tests`, `E2E (main suite)`,
`E2E (passkeys and auth)`, `E2E (fresh install)` and `E2E (multi-club)`. A check only
appears in the picker after it has run once, so open a pull request first. Also enable
Dependabot version updates (Settings, Code security).

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
  auth/               Sign-in (passkeys, optional passwords), sessions, player links
  accounts/           Accounts, invite and recovery links, roles
  backup/             Backups and restore
  tournaments/        The bundled tournament software (CourtHive) set-up
  help/               In-app guides, rendering docs/guide-*.md
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
embed.go              Builds templates, static files and migrations into the binary
deploy/               The packaged install: compose file, Caddyfile, .env.example
tests/e2e/            Playwright browser tests
docs/                 Guides for operators and users (docs/archive/ is history)
```

### Database migrations

Migrations are numbered sequentially (`001_`, `002_`, etc.) and run automatically on startup.

```bash
# Migrations live in migrations/ as .up.sql and .down.sql pairs
# The app applies them itself at start, after taking a backup.
# To run or roll back by hand (inside the Go image):
go run cmd/migrate/main.go
go run cmd/migrate-down/main.go
```

Always create both up and down migration files.

---

## Club Adaptation Guide

The software was built at St Ann's Tennis Club for the Brighton & Hove Parks League and is now packaged for any BHPLTA club. Each club runs its own copy. Nothing needs editing in the source: the club is chosen in the first-run setup in your browser, and everything else is a setting in the app or an environment variable.

### How a club runs it

- **Release image, not source.** A club runs the published image `ghcr.io/jameshartt/jim-dot-tennis:<version>` through `deploy/docker-compose.yml`. Nothing is built on the server. The binary carries its own templates, static files and migrations (`embed.go`), so there is nothing else to copy.
- **SQLite only.** The database is one SQLite file on the data volume, opened through the pure-Go driver `modernc.org/sqlite` (no C toolchain, CGO off). There is no other database option for the app.
- **Backups are built in.** The app backs itself up daily and before every upgrade, and `jim-dot-tennis backup` and `restore` do it by hand. See [docs/backup-and-restore.md](docs/backup-and-restore.md).
- **Sign-in.** Passkeys are the default. A password is an optional fallback that each person can add. There is no email: administrators pass on invite and recovery links themselves. See [docs/accounts-and-recovery.md](docs/accounts-and-recovery.md).
- **Roles.** Administrator, captain, and Tournaments only. Players have no accounts; they use their own private link. See [docs/roles.md](docs/roles.md).
- **Tournaments are optional.** The bundled CourtHive software (tournament management, public tournament pages) is a compose profile that is off until you switch it on. See [docs/guide-tournaments.md](docs/guide-tournaments.md) and [docs/courthive_bundle_design.md](docs/courthive_bundle_design.md).
- **Your copy is yours alone.** It ships with no accounts and no shared secrets, and it sends nothing to the author. See [docs/outbound_connections.md](docs/outbound_connections.md).
- **Command line.** The same binary has operator commands for accounts, passkeys, backups, setup codes and tournaments. Run `docker exec -it jim-dot-tennis ./jim-dot-tennis help` to see them.

### What you need

A small server (1 GB of memory is enough for the league; 2 GB if you switch tournaments on), Docker with the compose plugin, and a domain name. Comfort with a command line is enough: no Go knowledge is needed. The steps are in [docs/install.md](docs/install.md).

### Configuration (environment variables)

The reference file is `deploy/.env.example`, which lists every variable the stack and the app read. The ones you are most likely to meet:

| Variable | Required | Description |
|----------|----------|-------------|
| `DOMAIN` / `PUBLIC_URL` | Production | The address people use, for example `https://tennis.example.org`. The app refuses to start in production without a valid https `PUBLIC_URL`. It is used for links, calendar feeds and passkeys, so keep it stable. |
| `JIM_DOT_TENNIS_VERSION` | Production | The release to run. |
| `HOME_CLUB_ID` / `HOME_CLUB_NAME` | Optional | Normally you choose your club in first-run setup. These override it. |
| `BHPLTA_CLUB_CODE` | Optional | Your club's code on the BHPLTA website; can be set on the Club Settings page instead. |
| `BACKUP_DIR`, `BACKUP_INTERVAL`, `BACKUP_KEEP` | Optional | Where backups go, how often (default daily, or `off`) and how many to keep (default 14). |
| `COMPOSE_PROFILES=tournaments` | Optional | Turns on the bundled tournament software. |
| `DB_PATH` | Optional | SQLite file path (`/app/data/tennis.db` in the container). |
| `APP_ENV`, `DEV_MODE`, `TEMPLATE_RELOAD` | Development | `APP_ENV=production` is set by the compose file. `DEV_MODE` and `TEMPLATE_RELOAD` are for local development only and must never be set in production. |

### Club logo and branding

Upload the logo on **Admin Dashboard, Club Settings** (PNG, JPEG or WebP, up to 2 MB). It is stored on the data volume, survives upgrades, and a square logo of at least 192 pixels becomes the installed app's icon. The same page sets the display name and the two club colours.

### What is specific to the BHPLTA

The match card import, the season import and the fixture generator read the BHPLTA website. A league that is not run through that website would need those parts adapted (`internal/services/`, `internal/bhplta/`). Clubs in the BHPLTA need no code changes.

### Getting help

Ask through the **Parks Team Captains** WhatsApp group, or [open a club enquiry issue](https://github.com/jameshartt/jim-dot-tennis/issues/new?template=club_enquiry.md) on GitHub.
