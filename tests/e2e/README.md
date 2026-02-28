# E2E Tests — Jim.Tennis

End-to-end browser tests using [Playwright](https://playwright.dev/) running inside Docker.

## Overview

The E2E test suite validates the Jim.Tennis application through real browser interactions.
Tests run against a Dockerised instance of the app with a seeded SQLite database.

- **Workflow tests** — multi-step flows: team selection, match result entry, fixture editing
- **Page tests** — every admin and player page loads correctly with expected content
- **Accessibility tests** — automated WCAG 2.0 A/AA auditing via axe-core
- **Responsive tests** — mobile (375×812) and tablet (768×1024) viewport checks
- **Auth tests** — login, logout, rate limiting, token-based access

## Prerequisites

- Docker and Docker Compose
- No local Node.js or Go installation required — everything runs in containers

## Running Tests

```bash
# Full suite (default: 2 workers)
make test-e2e

# More parallel browser tabs
make test-e2e WORKERS=4

# Visible browser (headed mode, default: 1 worker)
make test-e2e-headed

# Headed mode with multiple tabs
make test-e2e-headed WORKERS=3

# Filter by test name
make test-e2e-grep FILTER="team-selection"
make test-e2e-grep FILTER="accessibility"
make test-e2e-grep FILTER="responsive"

# Re-run only previously failed tests
make test-e2e-failed

# Tear down test containers and clean artefacts
make test-e2e-clean
```

## Viewing Results

### HTML Report

```bash
make test-e2e-report
```

Opens the Playwright HTML report in your browser.

### Claude-Friendly Summary

```bash
make test-e2e-results
```

Runs the parse script (`scripts/parse-results.mjs`) against the JSON output, producing:

```
============================================================
E2E TEST RESULTS
============================================================
Total: 130  Passed: 128  Failed: 1  Flaky: 1  Duration: 45.2s

FAILED (1):
  1. Admin Fixtures > fixture detail page loads
     File: admin/fixtures.spec.ts:28
     Error: expected 200, received 500

FLAKY (1):
  1. Player Availability > cards displayed
     File: players/availability.spec.ts:19 (passed on retry)
```

### Feeding Results to Claude

```bash
make test-e2e-results | pbcopy   # macOS
make test-e2e-results | xclip    # Linux
```

Paste the output into a Claude conversation for analysis and debugging help.

## Test Structure

```
tests/e2e/
├── fixtures/
│   ├── seed.sql              # SQLite seed data (admin user, teams, players, fixtures)
│   ├── seed.sh               # Script that loads seed.sql into the test database
│   └── test-fixtures.ts      # Playwright test fixtures (adminPage with auto-login)
├── helpers/
│   ├── auth.ts               # loginAsAdmin(), loginWith()
│   ├── htmx.ts               # waitForHtmxSettle(), waitForHtmxRequest()
│   ├── navigation.ts         # goToDashboard(), goToFixtures(), etc.
│   └── assertions.ts         # expectTitleContains(), expectTableRowCount(), etc.
├── admin/                    # Admin page specs
│   ├── dashboard.spec.ts
│   ├── navigation.spec.ts
│   ├── players.spec.ts
│   ├── clubs.spec.ts
│   ├── fixtures.spec.ts
│   ├── divisions.spec.ts
│   ├── teams.spec.ts
│   ├── seasons.spec.ts
│   ├── users.spec.ts
│   ├── points-table.spec.ts
│   └── wrapped.spec.ts
├── players/                  # Player page specs
│   ├── availability.spec.ts
│   ├── standings.spec.ts
│   └── profile.spec.ts
├── workflows/                # Multi-step workflow tests
│   ├── team-selection.spec.ts
│   ├── match-results.spec.ts
│   └── fixture-management.spec.ts
├── scripts/
│   └── parse-results.mjs     # JSON results → human-readable summary
├── smoke.spec.ts             # Smoke tests (full stack validation)
├── auth.spec.ts              # Authentication flow tests
├── accessibility.spec.ts     # axe-core WCAG auditing
├── responsive.spec.ts        # Mobile and tablet viewport tests
├── global-setup.ts           # Shared auth session (storageState)
├── playwright.config.ts      # Playwright configuration
└── package.json              # Dependencies (@playwright/test, @axe-core/playwright)
```

## Seed Data

The test database is seeded with:

| Entity | Details |
|--------|---------|
| Admin user | `testadmin` / `testpassword123` |
| Season | Summer 2025, 18 weeks |
| League | Brighton & Hove Parks League |
| Divisions | Division 1 (Monday), Division 2 (Tuesday) |
| Clubs | St Ann's Tennis Club, Hove Park Tennis Club |
| Teams | 4 teams (2 per club, 1 per division) |
| Players | 8 players (4 per club, mixed gender) |
| Fixtures | 2 fixtures (1 per division, Week 1) |
| Matchups | 8 matchups (4 per fixture) |
| Fantasy token | `Sabalenka_Djokovic_Gauff_Sinner` |

Fixture 1 (Div 1) is used for read-only tests. Fixture 2 (Div 2) is used for destructive operations (result submission).

## Writing New Tests

1. Create a `.spec.ts` file in the appropriate directory
2. Import fixtures: `import { test, expect } from "../fixtures/test-fixtures";`
3. Use `adminPage` fixture for authenticated admin pages
4. Use `{ page }` for public pages (login, standings, availability)
5. Use helpers from `helpers/` for common operations
6. For serial (order-dependent) tests: `test.describe.serial("...", () => { ... })`

## Troubleshooting

### SQLite Locking ("database is locked")

Multiple Playwright workers writing to SQLite simultaneously causes locking errors. Limit to 2 workers (the default). The `retries: 1` config handles transient lock failures.

### Rate Limiting in Auth Tests

The app enforces 5 login attempts per 15 minutes per username+IP. Use unique usernames for failed-login tests — never use `testadmin` for deliberate failures.

### Auth Session Expired

`global-setup.ts` creates a shared session. The `adminPage` fixture falls back to direct login if the session expires. If auth tests fail repeatedly, run `make test-e2e-clean` and retry.

### HSTS `.app` TLD Issue

Docker service names must not use `.app` — Chromium forces HTTPS on HSTS-preloaded TLDs. The compose config uses `webapp` as the network alias. The base URL is `http://webapp:8080`.

### axe-core Violations

Accessibility tests exclude `color-contrast` by default (can be theme-dependent). If a page has known interactive-element issues, add the rule ID to the `disableRules` array for that specific test.
