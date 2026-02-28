# Technical Implementation Plan

This document outlines the technical approach for implementing the Jim.Tennis application, focusing on the architecture, technologies, and implementation strategy.

## Architecture Overview

The application follows a server-side rendered architecture with minimal client-side JavaScript:

```
┌───────────────────┐       ┌────────────────┐      ┌────────────────┐
│                   │       │                │      │                │
│  Web Browser      │◄─────►│  Go Web Server │◄────►│  SQLite DB     │
│  (HTMX + CSS)     │       │  (Templates)   │      │                │
│                   │       │                │      │                │
└───────────────────┘       └────────────────┘      └────────────────┘
         ▲                          │
         │                          ▼
┌────────┴──────────┐      ┌────────────────┐
│                   │      │                │
│  Service Worker   │      │  Background    │
│  (PWA/Push)       │      │  Jobs          │
│                   │      │                │
└───────────────────┘      └────────────────┘
```

## Technology Stack

1. **Backend**
   - Go 1.25 (main server language)
   - Chi for HTTP routing
   - HTML templates with server-side rendering
   - SQLite for database (lightweight, easy deployment)
   - Background task processing for notifications
   - `.golangci.yml` with 11 linters for static analysis

2. **Frontend**
   - HTMX for dynamic content without heavy JavaScript
   - Minimal vanilla JavaScript for essential client-side functionality
   - CSS for styling (potentially with TailwindCSS)
   - Progressively enhanced for better experience with JavaScript

3. **PWA/Notifications**
   - Service workers for offline capabilities
   - Web Push API for notifications
   - Local storage for client-side state persistence

4. **Infrastructure**
   - Docker with Alpine for production deployment
   - SQLite database file
   - Static files served directly

## Implementation Phases

### Phase 1: Core Infrastructure (Complete)

- [x] Database schema design
- [x] Data models and relationships
- [x] Migration framework
- [x] Web server setup
- [x] Authentication system
- [x] Basic template structure
- [x] Routing architecture
- [x] Hosting on somewhere with ssl on jim.tennis

### Phase 2: Captain Selection Tools (Mostly Complete)

- [x] Division-based access control
- [x] Available player listings
- [x] Team selection interface
- [ ] Selection confirmation and notifications
- [~] Selection overview across divisions (WI-008 deferred)
- [x] Player status tracking
- [x] Summary of fixture sharing for WhatsApp

### Phase 3: Player Availability Management (Mostly Complete)

- [x] Player profile views (Sprint 001 WI-001)
- [x] Availability form (calendar-based)
- [x] Availability exception handling (Sprint 001 WI-003 - "Mark Time Away")
- [x] General availability settings (backend implemented; removed from UI)

### Phase 4: PWA and Push Notifications

- [x] Service worker implementation
- [ ] Push notification pipeline
- [ ] Offline capability for core functions
- [ ] Installation flow
- [ ] Background sync for submissions

### Phase 5: Fixture and Venue Management (Partially Complete)

- [x] Fixture listing and details (Sprint 001 WI-011)
- [x] Match result entry/importation from match cards
- [x] Venue management with maps, directions, iCal feeds, and venue overrides (Sprint 002 WI-014)
- [ ] Fixture reminder system

### Phase 6: Admin Tooling and Dashboard (Complete)

- [x] Admin dashboard reorganisation with 4 grouped quick action categories (Sprint 003)
- [x] Division editing with play_day correction (Sprint 003)
- [x] Full user management CRUD (Sprint 003)
- [x] Session management with revoke capabilities (Sprint 003)
- [x] Club/venue infrastructure with BHPLTA scraper (Sprint 002)
- [x] Season filtering fixes (Sprint 002)

### Phase 7: Code Quality and Go Tooling (Complete)

- [x] Service.go refactor: 3871 to 117 lines, split into 12 domain-specific files (Sprint 003)
- [x] 9 Makefile targets for vet, fmt, lint, deadcode, etc. (Sprint 004)
- [x] Dead code removal via static analysis (Sprint 004)
- [x] Code formatting enforcement (Sprint 004)
- [x] Go version upgrade: 1.24.1 to 1.25 (Sprint 004)
- [x] Docker Alpine base image updates (Sprint 004)
- [x] `.golangci.yml` with 11 linters (Sprint 004)

## Technical Considerations

### Server-Side Rendering Strategy

All primary rendering occurs on the server, with HTMX providing dynamic content updates without page reloads:

```html
<!-- Example of HTMX approach -->
<button hx-post="/availability/update"
        hx-target="#availability-status"
        hx-swap="outerHTML"
        hx-vals='{"fixture_id": 123, "status": "Available"}'>
    I'm Available
</button>
```

This approach keeps the client-side code minimal while providing a dynamic, responsive user experience.

### Database Access Pattern

Data access follows a repository pattern:

```go
// Example repository pattern
type AvailabilityRepository interface {
    FindByPlayerAndFixture(playerID string, fixtureID uint) (*PlayerFixtureAvailability, error)
    UpdateAvailability(playerID string, fixtureID uint, status AvailabilityStatus) error
    // ...
}
```

### Push Notification System

Push notifications will be implemented using the Web Push API:

1. Client subscribes to push notifications
2. Subscription info stored in database
3. Server sends notifications via Web Push API
4. Service worker displays notifications even when app is closed


### Phase 8: Automated E2E Testing (Complete)

- [x] Playwright project scaffolding with TypeScript, Chromium-only for speed (Sprint 008)
- [x] Docker Compose test profile (`Dockerfile.e2e`, `e2e` service under `profiles: ["test"]`) (Sprint 008)
- [x] Test database seeding with idempotent SQL (admin user, full data hierarchy) (Sprint 008)
- [x] Reusable test helpers: auth, HTMX waiting, navigation, assertions (Sprint 008)
- [x] 7 Makefile targets: `test-e2e`, `test-e2e-grep`, `test-e2e-failed`, `test-e2e-report`, `test-e2e-results`, `test-e2e-headed`, `test-e2e-clean` (Sprint 008)
- [x] Smoke test suite with 5 tests validating the full stack (Sprint 008)
- [x] Core E2E test suite: 104 tests across 15 spec files (Sprint 009)
- [x] Authentication flow tests: login, logout, session, rate limiting, protected routes (Sprint 009)
- [x] Admin page coverage: dashboard, navigation, players, clubs, teams, fixtures, divisions, seasons, users, points table, wrapped (Sprint 009)
- [x] Player-facing tests: availability (token auth), profile, match history, public standings (Sprint 009)
- [x] Global auth setup with storageState and auth fallback for flake-free execution (Sprint 009)

## Testing Strategy

1. **Unit Testing**
   - Model validation and business logic
   - Repository interactions
   - Service layer functionality

2. **Integration Testing**
   - API endpoints
   - Database interactions
   - Authentication flows

3. **End-to-End Testing (Playwright)** *(Sprints 008–009)*
   - Runs in Docker via `make test-e2e` - no local Node.js or Go required
   - Chromium-only, 2 parallel workers, JSON + HTML reporters
   - SQL-seeded database with realistic test data
   - Global auth setup with storageState and login fallback for reliability
   - Reusable helpers for admin login, HTMX settling, and navigation
   - `--last-failed` support for fast iteration on failures
   - 104 tests covering: auth flows, all admin CRUD pages, player availability/profile, public standings

## Deployment Considerations

1. **Single Binary Deployment**
   - Compile Go application to a single binary
   - Include all static assets
   - Simple startup process

2. **Database Management**
   - SQLite file with regular backups
   - Migration handling for version updates

3. **Hosting Requirements**
   - Small VPS or similar
   - HTTPS for PWA and push notification support
   - Sufficient storage for database and logs
