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

# Fresh-install suite (empty databases, setup flow; see below)
make test-e2e-fresh
```

## Fresh-install suite (`make test-e2e-fresh`)

The `fresh-install` Playwright project proves a new club can get from nothing to
a working instance. It is separate from `make test-e2e`, which never runs it
(`fresh-install*.spec.ts` is ignored by the `chromium` project, and the project
only exists when `FRESH_INSTALL` is set).

`scripts/test-e2e-fresh.sh` starts, under a unique compose project name:

- `app-fresh` and `app-fresh-demo`: app instances on brand-new empty volumes
  (no `seed.sql`, no `HOME_CLUB_*`), so they start in setup mode and print a
  setup code to their log. `BHPLTA_BASE_URL` points at `http://fresh-fixtures:9099`.
- `e2e-fresh`: the Playwright runner. Its network alias is `fresh-fixtures` and
  it serves `fixtures/bhplta/` (the saved BHPLTA pages) itself via
  `helpers/fresh-install.ts` `FixtureServer`, so the real BHPLTA site is never
  contacted. The server can be stopped and started, which the failure/retry test uses.

How the suite gets the setup code: after the apps are healthy the script saves
each app's `docker compose logs` into a temporary directory mounted read-only
into the runner (`FRESH_LOG_DIR`); `readSetupCode()` reads the code from there.
The directory is deleted at the end; nothing is committed. The runner also
mounts the volumes, so specs check the database with `sqlite3` (`sqlite()` helper).

Volumes, containers and the network are removed when the run ends, pass or fail,
so each run starts from nothing and the regular e2e volume is never touched.

Specs (one serial flow, one worker, no retries):

- `fresh-install.spec.ts`: empty instance gates everything to `/setup`; wrong
  code rejected; code from the log accepted; password-policy rejection; first
  administrator created; club step fails with the fixture server down and shows
  "Try again"; retry succeeds from the saved pages and the done page counts
  (4 divisions, 41 teams, 18 weeks, 228 fixtures, 12 clubs) match the database;
  dashboard; settings change takes effect; player link generation works (and
  `tennis_players` was seeded); every `/setup...` route is 404 afterwards; the
  administrator is the only row in `users` (and no passkey/credential rows).
- `fresh-install-demo.spec.ts`: on a second empty instance, load demo data,
  see the `#demo-banner`, clear it and land back at club selection with only
  the administrator left.

Not covered: the off-season fixtures variant (`fixtures/bhplta/off-season`).

## Bundle smoke test (`make test-e2e-bundle`)

Runs the packaged deployment, `deploy/docker-compose.yml` and
`deploy/Caddyfile` unmodified, end to end (`tests/e2e/bundle/`, its own
Playwright config; the main suite ignores it). Two runs from an empty instance:
`bundle-on` (tournaments profile on: first-run set-up, switching Tournaments on,
the hand-over to TMX, a tournament made in TMX and published, the public pages,
calendar sync and the home page, a Tournaments only account, ending access,
signing out, TMX's "Log in", the refused proxy routes) and `bundle-off` (no
tournament links or routes).

Needs: Docker; the app is built from the checkout; the four CourtHive images
under the names in `deploy/docker-compose.yml` must exist locally (they are not
published yet; `BUNDLE_PULL=1` pulls them once they are). Everything runs under
its own compose project (`jdtbundle<on|off>...`) with renamed volumes, no
container names and Caddy on `127.0.0.1:18743` (on) / `18744` (off), using
`deploy/docker-compose.e2e.yml`; secrets come from the real
`deploy/init-tournaments.sh` run on a throwaway env file; the stack is removed
afterwards. Options: `BUNDLE_MODE=on|off|both`, `ARGS="-g TMX"`,
`BUNDLE_SKIP_BUILD=1`, `BUNDLE_KEEP=1`, `BUNDLE_NO_TEST=1`, `BUNDLE_PORT_ON`,
`BUNDLE_PORT_OFF`. On failure container logs are saved in
`tests/e2e/bundle/artifacts/` and traces in `tests/e2e/bundle/test-results/`.
The scheduled workflow is `.github/workflows/bundle-smoke.yml`.

## Passkey-enabled suite (`make test-e2e-auth`)

The `auth` Playwright project covers the sign-in system: passkeys via a Chromium
virtual authenticator (CDP, `helpers/webauthn.ts`), invite and recovery links,
the password fallback, "require passkeys for administrators", the
`jim-dot-tennis user reset` CLI and the admin users page / audit log. Its specs
are listed in `AUTH_SPECS` in `playwright.config.ts` (the `chromium` project
ignores them): `passkeys`, `account-links`, `admin-passkeys`, `cli-reset`,
`admin/users`. `roles.spec.ts` (captain vs administrator, password sign-in) runs
in the main project.

Why a separate project: the main suite reaches the app as `http://app-e2e:8080`,
which is not a secure context, so the login page there is password-only. The
`e2e-auth` compose service shares `app-e2e`'s network namespace, so the browser
reaches the same app as `http://localhost:8080`. Chromium treats that as secure
and, with `DEV_MODE`, the app accepts exactly that origin for passkeys. No
application change is involved.

`scripts/test-e2e-auth.sh` (one worker): builds, recreates `app-e2e` (clears the
in-memory sign-in throttling; the volume is kept), seeds, then runs
`jim-dot-tennis user create` and `user reset` inside the app container and passes
the printed link to `cli-reset.spec.ts` through `E2E_CLI_RESET_LINK` and
`E2E_CLI_USER` (the Playwright container cannot reach Docker). Afterwards it runs
`admin-passkeys off` and stops `app-e2e`. `make test-e2e-all` runs the main,
auth and fresh-install suites.

Notes: tests make users with unique names (`uniqueUsername`); link expiry is
simulated by editing `account_links.expires_at` with `sql()` (helpers/accounts.ts,
the runner shares the e2e volume); the "require passkeys" spec parks other
administrators in the database and restores everything in `finally`/`afterAll`.
Keep deliberate failed logins few (5 per user and 30 per address per 15 minutes).

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
│   ├── assertions.ts         # expectTitleContains(), expectTableRowCount(), etc.
│   └── fresh-install.ts      # setup-code reader, sqlite(), FixtureServer (fresh-install only)
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
├── fresh-install.spec.ts     # Fresh-install suite (make test-e2e-fresh only)
├── fresh-install-demo.spec.ts
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

### Fresh volumes

No pre-seeding is needed before the app can start. On a brand-new `jim-dot-tennis-e2e-data` volume, `app-e2e` applies the migrations and comes up healthy (the compose health check calls `/healthz`) in setup mode, because `HOME_CLUB_ID` points at a club the empty database does not have yet. The `e2e` service then runs `seed.sh`, which inserts that club, and the app resolves `HOME_CLUB_ID` on the next request without a restart. `make test-e2e-bootstrap` now only builds the images.

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

A bare `app` hostname is unusable — `.app` is an HSTS-preloaded TLD and Chromium forces HTTPS on it. The suite talks to the dedicated `app-e2e` service (its own data volume, no host port). The base URL is `http://app-e2e:8080`.

### axe-core Violations

Accessibility tests exclude `color-contrast` by default (can be theme-dependent). If a page has known interactive-element issues, add the rule ID to the `disableRules` array for that specific test.
