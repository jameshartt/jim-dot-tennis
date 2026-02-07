# Jim.Tennis Documentation

This directory contains documentation for the Jim.Tennis application -- an internal tool for St Ann's Tennis Club to manage team selection, player availability, fixture coordination, and venue information within the Brighton and Hove Parks League.

**Runtime:** Go 1.25 | SQLite (default) or PostgreSQL | Server-side rendering with HTMX

## Project Documentation

- [Project Overview](./project_overview.md) - Goals, purpose, and core requirements of the system
- [User Experience Requirements](./user_experience_requirements.md) - Detailed requirements for the application's UX
- [Technical Implementation Plan](./technical_implementation_plan.md) - Approach for building and implementing the application

## Infrastructure and Deployment

- [Docker Setup](./docker_setup.md) - Instructions for using Docker with this project
- [DigitalOcean Deployment](./digitalocean_deployment.md) - Guide for deploying to DigitalOcean
- [DigitalOcean Monitoring](./digitalocean_monitoring.md) - Advanced monitoring and management on DigitalOcean
- [Production Quick Reference](./PRODUCTION_QUICK_REFERENCE.md) - Quick-reference guide for production operations
- [Database Reset Guide](./database_reset_guide.md) - Steps for resetting the database

## BHPLTA Integration

- [Admin Match Card Import](./admin_match_card_import.md) - Importing match cards from the BHPLTA website
- [Automatic Nonce Extraction](./automatic_nonce_extraction.md) - WordPress nonce extraction for BHPLTA API access

## Additional Resources

- [CourtHive Deployment Plan](./courthive_deployment_plan.md) - CourtHive integration planning

## Technical Architecture

### Application Structure

The codebase follows a layered architecture with clear domain separation:

- **`internal/models/`** - Data structures (Player, Team, Fixture, Venue, etc.)
- **`internal/repository/`** - Data access layer, one repository per entity
- **`internal/admin/`** - Admin interface: handlers and a domain-split service layer
- **`internal/players/`** - Player-facing features: profiles, availability, venue pages
- **`internal/services/`** - Business logic: match card parsing, iCal generation, venue resolution
- **`internal/auth/`** - Session-based admin auth and token-based player auth
- **`internal/webpush/`** - Push notification service (subscription management in place; delivery planned)

### Service Layer Organisation

The admin service layer (`internal/admin/service*.go`) is split into domain-specific files rather than a single monolith:

`service.go` (core/shared), `service_dashboard.go`, `service_clubs.go`, `service_divisions.go`, `service_fantasy.go`, `service_fixture_players.go`, `service_fixtures.go`, `service_matchups.go`, `service_players.go`, `service_seasons.go`, `service_selection.go`, `service_teams.go`, `service_users_sessions.go`

### Key Capabilities

- **Venue and club infrastructure** - Club/venue data with maps, directions, and venue resolution
- **iCal feeds** - Calendar subscription feeds for fixtures (`ical_generator.go`)
- **Player profiles and availability** - Players manage their own availability and profile information
- **Admin management** - User management, session management, division editing, team eligibility, points, season setup
- **Match card import** - Automated import of results from the BHPLTA website via nonce extraction and HTML parsing

### Code Quality Tooling

Static analysis and formatting are run via Docker-based Go tooling (see `Makefile` targets):

- `make vet` - `go vet` static analysis
- `make fmt` / `make fmt-fix` - Format checking and auto-fix
- `make lint` - `golangci-lint` comprehensive linting (configured via `.golangci.yml`)
- `make deadcode` - Dead code detection
- `make check` - Run all of the above

### PWA

The application is a Progressive Web App. Push notification delivery is planned but not yet implemented; the subscription management infrastructure (`internal/webpush/`) is in place.