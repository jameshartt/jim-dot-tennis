# Jim.Tennis Project Overview

## Project Purpose
Jim.Tennis is an internal tool for St Ann's Tennis Club to facilitate team management within the Brighton and Hove Parks League. The application provides a suite of tools for team captains and players to manage availability, plan fixtures, and coordinate match participation.

## Core Goals

1. **Availability Management**
   - Allow players to update their own availability via availability exceptions ("Mark Time Away")
   - Enable captains to manage availability for players who cannot interact with the tool
   - Create an extremely intuitive and user-friendly experience for updating availability

2. **Team Selection**
   - Support hierarchical team selection process (Division 1 picks first, then Division 2, etc.)
   - Notify captains when "lower" division captains have made their selections
   - Automate and simplify the process of player selection based on availability

3. **Fixture Management**
   - Provide clear scheduling of upcoming fixtures with detailed fixture views
   - Track match details, locations, and results
   - Streamline communication about matches via WhatsApp sharing
   - Provide iCal calendar feeds for fixture subscriptions

4. **Venue & Club Infrastructure**
   - Maintain venue and club data via BHPLTA scraper integration
   - Provide player-facing venue pages with maps and directions
   - Support venue overrides for fixtures played at non-default locations

5. **Notifications & Communication**
   - Implement push notifications through PWA capabilities
   - Remind players about upcoming fixtures and availability deadlines
   - Facilitate easy sharing of information within existing communication channels (e.g., WhatsApp)

## Technical Stack

- **Go 1.25** (server-side application)
- **SQLite** (default) or **PostgreSQL**
- **Server-side rendered HTML templates** with **HTMX**
- **Progressive Web App (PWA)** with push notification support
- **Docker** for production deployment and Go tooling
- **Static analysis & formatting** via Docker-based Go tooling (vet, lint, fmt, deadcode, imports)

## Technical Approach

1. **Server-Side Rendering**
   - Prioritize server-side rendering whenever possible
   - Use HTMX to minimize client-side JavaScript
   - Create a fast, responsive experience with minimal client-side complexity

2. **Progressive Web App (PWA)**
   - Implement as a PWA to enable push notifications and offline capabilities
   - Ensure mobile-friendly design for easy access on all devices

3. **User Experience**
   - Focus on creating an extremely simple, intuitive interface
   - Design for minimal friction in all user interactions
   - Seamlessly integrate with existing communication workflows (WhatsApp)

4. **Code Quality**
   - Docker-based Go tooling with 9 Makefile targets (vet, fmt, fmt-fix, imports, imports-fix, lint, deadcode, tidy, check)
   - Static analysis via `.golangci.yml` configuration with 11 linters enabled
   - Consistent code formatting enforced across the codebase

## Target Users

1. **Team Captains**
   - Responsible for managing one or more teams
   - Need tools to select players, coordinate fixtures, and manage team communication
   - Require notifications about player availability and selection status

2. **Players**
   - Need simple, easy ways to update availability
   - Require notifications about selection status and upcoming fixtures
   - May vary in technical proficiency (app must be accessible to all skill levels)

3. **Administrators**
   - Manage clubs, divisions, users, and sessions through the admin dashboard
   - Import and maintain venue/club data from the BHPLTA website
   - Full user management with CRUD operations and session revocation

## Current Development Status

### Completed (Sprints 001-004, 005-008)

**Sprint 001 - Player Experience & Fixtures:**
- Player profile views and availability exception handling ("Mark Time Away")
- Fixture details and listing pages
- WhatsApp sharing for fixtures
- General availability preferences (implemented then removed from UI; backend remains)

**Sprint 002 - Venue & Club Infrastructure:**
- Venue and club data pipeline with BHPLTA scraper
- Admin club management pages
- Venue resolver service for mapping fixtures to venues
- Player-facing venue page with embedded map and directions
- iCal calendar feed generation for fixture subscriptions
- Venue overrides for fixtures at non-default locations
- Season filtering fixes across admin UI

**Sprint 003 - Admin Tooling & Refactoring:**
- Admin dashboard reorganization with stat cards and 4 grouped quick action categories
- Division editing with play_day correction
- Full user management CRUD (create, read, update, delete)
- Session management with revoke capabilities
- Major `service.go` refactor (3,871 lines to 117 lines, split into 12 domain-specific service files)

**Sprint 004 - Code Quality & Tooling:**
- Docker-based Go tooling with 9 Makefile targets (vet, fmt, fmt-fix, imports, imports-fix, lint, deadcode, tidy, check)
- Dead code removal (18 functions, 2 types removed)
- Static analysis configuration via `.golangci.yml` (11 linters enabled)
- Code formatting standardization across the codebase
- Go version upgrade from 1.24.1 to 1.25
- Docker Alpine image updates

**Sprint 005 - Club & Away Team Management:**
- Club and away team management features

**Sprint 006 - Match Results & Standings:**
- Match results, standings, match history, weather integration
- Admin URL routing fixes

**Sprint 008 - E2E Test Infrastructure:**
- Playwright browser testing infrastructure with Docker Compose test profile
- Test database seeding with realistic data (admin user, seasons, leagues, clubs, teams, players, fixtures)
- Reusable test helpers for authentication, HTMX waiting, navigation, and assertions
- 7 Makefile targets for running, filtering, and reporting E2E tests
- Smoke test suite validating the full testing stack (5 tests)
- No local Node.js or Go installation required - runs entirely in Docker

**Sprint 009 - Core E2E Test Suite:**
- 104 Playwright E2E tests across 15 spec files covering all critical user flows
- Authentication flows: login, logout, session persistence, rate limiting, protected routes
- Admin CRUD pages: players, clubs, teams, fixtures, divisions, seasons, users
- Admin features: dashboard, navigation, points table, wrapped
- Player-facing: availability (token auth), profile, match history, public standings
- Global auth setup with storageState for reliable, fast test execution
- Anti-flakiness measures: 2 workers, 1 retry, auth fallback, auto-waiting assertions

**Sprint 010 - Advanced E2E & Accessibility:**
- 42 new E2E tests (146 total) covering complex multi-step workflows
- Workflow tests: team selection, match result entry, fixture editing, selection overview
- Automated accessibility auditing via axe-core (WCAG 2.0 A/AA on 8 pages)
- Responsive viewport testing at mobile (375×812) and tablet (768×1024)
- Claude-friendly test results parser (parse-results.mjs) and updated Makefile target
- Comprehensive E2E testing documentation (README.md)

### Planned (Sprint PWA)

- Push notifications pipeline
- PWA installation prompt for mobile users
- Offline availability management with background sync
