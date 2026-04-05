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
- **SQLite** (default) or PostgreSQL
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

### Configuration

The application uses environment variables to identify your club:

| Variable | Required | Description |
|----------|----------|-------------|
| `HOME_CLUB_ID` | Yes (or `HOME_CLUB_NAME`) | Database ID of your club |
| `HOME_CLUB_NAME` | Fallback | Club name for fuzzy lookup if `HOME_CLUB_ID` is not set |
| `BHPLTA_CLUB_CODE` | For imports | Your club's code on the BHPLTA website (e.g. `STANN001`) |

See [docs/docker_setup.md](docs/docker_setup.md) for the full list of environment variables.

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
