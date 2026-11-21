# jim.tennis

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A league management system originally built at [St Ann's Tennis Club](https://www.stannstennis.co.uk/) for the Brighton & Hove Parks League. It handles player availability, team selection, fixture scheduling, match results, and standings — and is deployable for any parks league club.

**Live at [jim.tennis](https://jim.tennis)** | **Source on [GitHub](https://github.com/jameshartt/jim-dot-tennis)**

## What it does

- **Player availability** — players mark when they're free via a token-based link (no login required). Captains see who's available at a glance.
- **Team selection** — hierarchical selection process where Division 1 picks first, then Division 2, and so on. Captains are notified when it's their turn.
- **Fixture scheduling** — weekly fixture management with venue details, maps, directions, and iCal calendar feeds.
- **Match results & standings** — import results from the BHPLTA website, track standings, and view match history.
- **Tournament & Cup management** — integrated with CourtHive for tournament draws, scheduling, and live scoring.
- **Admin dashboard** — manage seasons, divisions, clubs, teams, players, users, and sessions.

## Attribution & Credits

**Built by [James Hartt](https://github.com/jameshartt)**
jim.tennis was designed and built by James Hartt, originally for St Ann's Tennis Club. The system has been in active use since the 2025 parks league season.

**Tournament management by [CourtHive](https://courthive.com/) (Charles Allen)**
Tournament and Cup management is developed and maintained by CourtHive, with the primary development effort by Charles Allen. CourtHive provides tournament scheduling, draw management, and live scoring for Parks League Cup competitions.

**Brighton & Hove Parks League Tennis Association**
The BHPLTA organises the parks league that jim.tennis is built around. Match card data is imported from the BHPLTA website with their cooperation.

## Tech stack

- **Go 1.25** — server-side application
- **SQLite** (the only supported database)
- **HTMX** — server-rendered HTML with minimal client-side JavaScript
- **PWA** — progressive web app with push notification support
- **Docker** — production deployment with Caddy reverse proxy
- **Playwright** — 146 E2E browser tests

## Getting started

### Prerequisites

- Docker (Go is not required locally — everything builds in Docker)

### Local development

```bash
# Build and run (creates database at ./tennis.db, serves at localhost:8080)
make local

# Or individual steps
make build-local      # Build binary
make run-local        # Build and run
make clean-local      # Clean binary and database
```

### Production deployment

```bash
make build            # Build Docker images
make run              # Start containers
make stop             # Stop containers
make logs             # View logs
```

See [docs/PRODUCTION_QUICK_REFERENCE.md](docs/PRODUCTION_QUICK_REFERENCE.md) and [docs/digitalocean_deployment.md](docs/digitalocean_deployment.md) for full deployment details.

### First run

A new install has no accounts and no club. On startup the app prints a one-time
**setup code** to its log (`make app-logs`, or `docker compose logs app`). Open
the app in a browser, enter the code, create your administrator login and choose
your club. The code works once; restarting the app before setup is finished
prints a new one and cancels the old. Once an administrator exists the setup
pages are gone.

Your instance is yours alone: it ships with no accounts and no shared or default
secrets (anything it needs, such as the Club Wrapped code and push keys, is
generated on first start), and it sends nothing to the software's author. See
[What this instance connects to](docs/outbound_connections.md#what-this-instance-connects-to)
for every outbound connection it makes.

### Configuration

The application uses environment variables to identify your club:

| Variable | Required | Description |
|----------|----------|-------------|
| `HOME_CLUB_ID` | Unless chosen in setup | Database ID of your club. Overrides `HOME_CLUB_NAME` and the club stored by first-run setup; with none of the three the app starts in setup mode (`/setup`) |
| `HOME_CLUB_NAME` | Fallback | Club name for fuzzy lookup if `HOME_CLUB_ID` is not set |
| `HOME_CLUB_LOGO_PATH` | Optional | URL path to a club logo image. Normally you upload the logo on the admin Club Settings page instead; when set, this wins over the uploaded logo. |
| `BHPLTA_CLUB_CODE` | Optional | Your club's code on the BHPLTA website, for match card imports. Can be set on the Club Settings page instead; when set, this wins. |
| `WRAPPED_ACCESS_PASSWORD` | Optional | Code that unlocks the club wrapped pages. When unset (recommended) each instance generates its own code on first start; administrators see it on the Club Settings page. A placeholder value such as `change-me` stops the app starting with `APP_ENV=production`. |
| `MY_TENNIS_ENABLED` | Optional | `true`/`1` shows the "My Tennis" call-to-action. Off when unset. |
| `LEAGUE_TABLES_URL` | Optional | Source page for the public fixture generator's league tables (default: the BHPLTA league tables page). |
| `COURTHIVE_API_URL` | Optional | CourtHive API URL (default: `http://courthive-server:8383`). |
| `APP_ENV` | Production | Set to `production` to mark session cookies Secure. The reference compose sets it. |
| `PORT` | Optional | HTTP listen port (default: `8080`). |
| `DB_TYPE` | Optional | Only `sqlite3` is supported; any other value fails at startup. |
| `DB_PATH` | Optional | SQLite file path (default: `./tennis.db`; `/app/data/tennis.db` in Docker). |
| `TEMPLATE_RELOAD` | Dev only | `true` re-parses templates on every request (hot reload). Leave unset in production. |
| `MIGRATE_ALLOW_DIRTY_FORCE` | Dev only | `true` auto-forces a dirty migration version at startup. Never set in production: a dirty database should stop the deploy. |

The production stack (app, CourtHive, Caddy) is defined in [`deploy/`](deploy/): `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example`, which lists every variable the stack and the app read. See [docs/docker_setup.md](docs/docker_setup.md) for more.

### Running tests

```bash
make test-e2e                      # Full E2E suite (2 workers)
make test-e2e-grep FILTER="login"  # Run tests matching pattern
make test-e2e-results              # Formatted test summary
make test-e2e-multiclub            # Multi-club verification tests
```

See [tests/e2e/README.md](tests/e2e/README.md) for the complete testing guide.

## For other clubs

jim.tennis was originally built for St Ann's Tennis Club, but it's designed to be club-agnostic — set `HOME_CLUB_ID` and `BHPLTA_CLUB_CODE` in your environment and you're running for your own club. It's open-source under the MIT license. The [Club Adaptation Guide](CONTRIBUTING.md#club-adaptation-guide) covers what's involved, and you're welcome to [open an issue](https://github.com/jameshartt/jim-dot-tennis/issues/new?template=club_enquiry.md) to ask questions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and the club adaptation guide.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
