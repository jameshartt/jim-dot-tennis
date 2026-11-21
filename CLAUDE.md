# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Jim.Tennis is a club-agnostic tennis league management system, originally built at St Ann's Tennis Club for the Brighton and Hove Parks Tennis League. The home club is configured via environment variables (`HOME_CLUB_ID` / `HOME_CLUB_NAME`), making the system deployable for any parks league club. It handles player availability, team selection, fixture scheduling, match results tracking, and integrates with the BHPLTA (Brighton & Hove Parks League Tennis Association) website to import match cards.

**Key Technical Stack:**
- Go 1.26+
- SQLite (the only supported database), through the pure-Go driver `modernc.org/sqlite`: no CGO, `CGO_ENABLED=0` builds for any target (linux/amd64, linux/arm64)
- Server-side rendered templates with HTMX
- PWA with push notifications
- Docker for production deployment

## Development Commands

### Local Development
```bash
# Build and run locally (creates database at ./tennis.db)
make local

# Or run individual commands
make build-local      # Build to bin/jim-dot-tennis
make run-local        # Build and run
make clean-local      # Clean binary and database

# Server runs at http://localhost:8080
```

### Database Management
```bash
# Migrations run automatically on startup, from the copies built into the
# binary (embed.go); cmd/migrate and cmd/migrate-down use the same built-in
# copies (-path <dir> reads a directory instead). To manually migrate:
go run cmd/migrate/main.go

# To rollback:
go run cmd/migrate-down/main.go
```

### Docker (Production)
```bash
make build           # Build Docker images
make run             # Start containers
make stop            # Stop containers
make logs            # View all logs
make app-logs        # View app logs only
make backup          # Make a backup now (jim-dot-tennis backup in the app container)
make export-backup   # Copy the newest backup out to ./exported-backups
```

### Go Unit Tests (in Docker)
```bash
make test                                 # Run all Go unit tests (in Docker, CGO_ENABLED=0)
make test PKG=./internal/database/...     # Test a single package
make test ARGS="-run TestName -v"         # Pass flags through to `go test`
# `make check` runs vet + fmt + lint + deadcode + test.
```

### Fixture Generator Engine Tests (Node in Docker)
```bash
make test-js    # node --test tests/fixture-generator/engine.test.mjs (also part of `make check`)
```

### E2E Testing (Playwright in Docker)
```bash
make test-e2e                      # Run full E2E test suite (default: 2 workers)
make test-e2e WORKERS=4            # Run with more parallel browser tabs
make test-e2e-headed               # Headed mode (default: 1 worker/tab)
make test-e2e-headed WORKERS=3     # Headed mode with 3 browser tabs
make test-e2e-grep FILTER="login"  # Run tests matching pattern
make test-e2e-failed               # Re-run only previously failed tests
make test-e2e-report               # Open HTML test report
make test-e2e-results              # Formatted test summary (Claude-friendly)
make test-e2e-multiclub            # Run multi-club verification tests
make test-e2e-auth                 # Passkey-enabled suite (project `auth`): virtual authenticator, invite/recovery links, password fallback, proof of ownership, require-passkeys setting, `user reset` CLI; browser reaches the app as http://localhost:8080 (ARGS="-g text" filters)
make test-e2e-all                  # test-e2e + test-e2e-auth + test-e2e-fresh, all run, non-zero if any failed
make test-e2e-fresh                # Fresh-install suite: EMPTY volumes, setup flow, saved BHPLTA pages (own compose project)
make test-e2e-clean                # Tear down E2E containers, delete the E2E volume and artifacts

# Test infrastructure:
# - tests/e2e/              Playwright project root (246 tests across 37 spec files: 222 in the main suite, 11 in `auth`, 13 in `fresh-install`; multi-club.spec.ts runs separately)
# - tests/e2e/helpers/      Reusable helpers (auth, htmx, navigation, assertions)
# - tests/e2e/fixtures/     Seed data (seed.sql) and test fixtures (test-fixtures.ts)
# - tests/e2e/admin/        Admin page specs (dashboard, navigation, CRUD, points, wrapped)
# - tests/e2e/players/      Player page specs (availability, profile, standings)
# - tests/e2e/workflows/    Multi-step workflow specs (team selection, match results, fixtures)
# - tests/e2e/accessibility.spec.ts  axe-core WCAG 2.0 A/AA audits
# - tests/e2e/responsive.spec.ts     Mobile (375×812) and tablet (768×1024) viewport tests
# - tests/e2e/smoke.spec.ts Smoke tests validating the full stack
# - tests/e2e/auth.spec.ts  Authentication flow tests (login, logout, rate limit)
# - tests/e2e/roles.spec.ts  Captain vs administrator (seeded `testcaptain` / testpassword123, id 900)
# - Passkey-enabled project `auth` (passkeys.spec.ts, account-links.spec.ts, admin-passkeys.spec.ts,
#   cli-reset.spec.ts; list is AUTH_SPECS in playwright.config.ts; ignored by `chromium`):
#   WebAuthn needs a secure context and Chromium only treats http://localhost as one, so the `e2e-auth`
#   compose service shares app-e2e's network namespace and the browser uses http://localhost:8080 (the app accepts
#   exactly that origin for passkeys under DEV_MODE; no app change). Run through scripts/test-e2e-auth.sh:
#   recreates app-e2e (clears in-memory throttling), seeds, runs `jim-dot-tennis user create/reset` inside the app
#   container and passes the printed link in E2E_CLI_RESET_LINK / E2E_CLI_USER (Playwright has no Docker
#   access), runs one worker, then `admin-passkeys off` and stops app-e2e. Virtual authenticator helper:
#   helpers/webauthn.ts (CDP, Chromium only; `putAway()`/`restore()` swap devices, one attached at a time); account/link helpers (`confirmWithPasskey`, `addSecondPasskey`: changing a sign-in method needs the password or an older passkey) and DB access (`sql()`, for expiring links) in helpers/accounts.ts
# - Where passkeys work the login page folds the password form into details[data-passkey-fallback], and a
#   password sign-in of an account without a passkey detours via /account/signin-methods?prompt=1:
#   use helpers/auth.ts (openPasswordForm, continueAfterPasswordSignIn, loginAsAdmin, loginAsCaptain)
# - tests/e2e/scripts/parse-results.mjs  JSON results → Claude-friendly summary
# - tests/e2e/global-setup.ts  Shared auth session via storageState (skipped when FRESH_INSTALL is set)
# - tests/e2e/fresh-install*.spec.ts  `fresh-install` project (only with `make test-e2e-fresh`; excluded from `make test-e2e`)
# - Dockerfile.e2e          Playwright container (includes sqlite3 for seeding)
# - E2E runs against its own `app-e2e` service and `jim-dot-tennis-e2e-data` volume (never the dev/prod
#   data volume or the port-8080 app); `make test-e2e-bootstrap` prepares a fresh volume automatically
# - Seeded admin: testadmin / testpassword123
# - Seeded fantasy token: Sabalenka_Djokovic_Gauff_Sinner
```

### Utility Commands
```bash
# Build all utility tools
make build-utils

# Populate database from CSV fixtures
go build -o bin/populate-db cmd/populate-db/main.go
./bin/populate-db -verbose -dry-run

# Extract WordPress nonce from BHPLTA
./bin/extract-nonce -club-code="$BHPLTA_CLUB_CODE" -verbose

# Import match cards from BHPLTA
./bin/import-matchcards \
  -auto-nonce \
  -club-code="$BHPLTA_CLUB_CODE" \
  -week=1 \
  -year=2024 \
  -club-id=$HOME_CLUB_ID \
  -club-name="Your Club Name" \
  -db="./tennis.db" \
  -verbose \
  -dry-run
```

## Architecture Overview

### Application Structure

The codebase follows a clean layered architecture:

**cmd/**: Entry points and utility commands
- `jim-dot-tennis/`: Main web application
- `migrate/`, `migrate-down/`: Database migration tools
- `populate-db/`: Bulk CSV import for fixtures
- `extract-nonce/`: WordPress nonce extraction for BHPLTA
- `import-matchcards/`: Match result import from BHPLTA
- `generate-fantasy-matches/`: Fantasy match generation utility

**internal/**: Core application logic
- `config/`: Home club resolution and the live `HomeClubHolder`, `AppConfig` snapshot for CLI tools, context helpers, and middleware for injecting the home club into request context
- `setup/`: First-run "needs setup" state, the setup gate middleware and the setup pages
- `secrets/`: Per-instance secrets (generated Club Wrapped code) and the startup self-check; `outbound/` holds only a test that fails on any URL host not in its allowlist (keep `docs/outbound_connections.md` in step)
- `normalize/`: Centralised apostrophe/Unicode normalisation utilities
- `database/`: Database connection and migration handling
- `backup/`: Backup and restore (`jim-dot-tennis backup`, `jim-dot-tennis restore <file>`, the scheduled and pre-migration backups, the data-folder lock); see "Backups" below
- `models/`: Data structures (Player, Team, Fixture, etc.)
- `repository/`: Data access layer (one per entity)
- `auth/`: Authentication service, middleware, and handlers
- `accounts/`: The one account service (invite, list, enable/disable, role, invite and recovery links in `links.go`, session revocation, last-usable-administrator guard, audit seam `recordAudit`), used by the admin users/sessions pages and the server command line `jim-dot-tennis user ... | admin-passkeys status|on|off | setup code` (`cmd/jim-dot-tennis/cli.go`; help text reproduced in `docs/PRODUCTION_QUICK_REFERENCE.md` section 10, kept in step by a test). Also the link page `/account/link/{token}` (`link_page.go`), a user's own password (`password.go`, endpoints in `password_page.go`) and the "Require passkeys for administrators" setting (`admin_passkeys.go`)
- `admin/`: Admin interface handlers (dashboard, players, fixtures, teams, users, sessions, account audit log, match card import, points, club wrapped)
- `players/`: Player-facing handlers (availability management)
- `services/`: Business logic (match card parsing, player matching, nonce extraction)
- `webpush/`: Push notification service
- `fixturegen/`: Public fixture generator page (`/fixture-generator`) and its league-tables feed
- `fixturefile/`: Reader/validator for the shareable season fixture file (`jim.tennis/fixtures`)
- `bhplta/`: Season bootstrap from the BHPLTA website: `Source.Fetch` (network, no DB) then `Import` in a caller-owned transaction; `Bootstrapper.Run` does both

**static/fixture-generator/**: The fixture generator itself, which runs in the browser. `engine.js` holds the promotion/relegation rules, the scheduler and the file format (no DOM, tested by `tests/fixture-generator`); `app.js`/`app.css` are the interface. See `docs/fixture_generator.md`.

**migrations/**: SQL migration files (up/down pairs)

**embed.go** (package `jimdottennis`, repository root): builds `templates/`, `static/` and `migrations/*.sql` into the binary, so it runs from any directory with only its data directory. The server reads templates through `internal/render` (`render.ParseFiles`/`ParseGlob`/`Glob`/`ReadFile`, never `template.ParseFiles` or `os.ReadFile` directly; handlers' `templateDir` is `"."` inside the embedded tree) and serves `/static/` through `internal/assets`. Templates link CSS/JS as `{{asset "/static/..."}}`, which appends `?v=<hash of the static tree>`; such URLs are cached as immutable for a year, every other `/static/` URL gets `no-cache` with an ETag. Uploaded logos stay on the data volume (`/branding/`).

**tests/e2e/**: Playwright E2E browser tests (246 tests across 37 spec files: main suite 222, `auth` 11, `fresh-install` 13)
- `helpers/`: Reusable test helpers (auth, htmx, navigation, assertions)
- `fixtures/`: SQL seed data and Playwright test fixtures
- `admin/`, `players/`: Page-level spec files
- `workflows/`: Multi-step workflow specs (team selection, match results, fixture management)
- `accessibility.spec.ts`: axe-core WCAG audits
- `responsive.spec.ts`: Mobile/tablet viewport tests
- `scripts/parse-results.mjs`: JSON results → Claude-friendly summary

**templates/**: HTML templates for server-side rendering
- `admin/`: Admin interface templates
- `players/`: Player interface templates
- Base templates: `index.html`, `layout.html`, `login.html`

### Database Schema

**Core Hierarchy:**
- Season → Weeks (1-18 per season)
- League → Divisions → Teams
- Club → Teams → Players
- Fixture (links Team, Week, Division, Season)
- Matchup (Men's/Women's/Mixed doubles within a Fixture)

**Key Tables:**
- `seasons`, `weeks`: Time organization
- `leagues`, `divisions`: Competition structure
- `clubs`, `teams`, `players`: Organizational entities
- `fixtures`, `matchups`: Match scheduling and results
- `player_availability`: Player availability tracking
- `fixture_players`: Player assignments to fixtures
- `tennis_players`: External player database from BHPLTA
- `users`, `sessions`, `login_attempts`: Authentication (`sessions` holds token hashes only)
- `account_audit_log`: Append-only record of account actions
- `push_subscriptions`, `vapid_keys`: Push notifications (no part of the VAPID private key, no player token and no push endpoint is ever logged or returned)

### Repository Pattern

Each domain entity has a dedicated repository in `internal/repository/`:
- Encapsulates all database operations for that entity
- Provides CRUD and entity-specific query methods
- Used by service layer to access data

Key repositories: `SeasonRepository`, `WeekRepository`, `LeagueRepository`, `DivisionRepository`, `ClubRepository`, `TeamRepository`, `PlayerRepository`, `FixtureRepository`, `MatchupRepository`, `AvailabilityRepository`

### Authentication & Authorization

**Two Authentication Modes:**

1. **Session-based (Admin)**: Cookie-based authentication with role-based access control
   - Login at `/login` → redirects to `/admin/league/dashboard`. Two ways in, one session: a passkey (the login page leads with it; the password form is folded away in a `<details data-passkey-fallback>` wherever passkeys are available) or username and password. Both end in `auth.Service.completeLogin` → `createSession`, so the sessions are identical
   - Sessions stored in database with expiry (7 days sliding, 30 days absolute). The cookie carries a random token; only its SHA-256 is stored (`sessions.token_hash`, `auth.HashSessionToken`, migration 035, which signed everyone out once), and `sessions.id` is a non-secret number used on the admin sessions page (`/admin/league/sessions/{id}/invalidate`). `RequireAuth` puts the user (with `PasswordHash` always emptied: read the hash from the database where it is needed), the role and the session id (`auth.SessionIDFromContext`) in the request context, and logs nothing per request
   - Logout is `POST /logout` only (GET is 405); every logout control is a `<form method="POST" action="/logout">` with `data-testid="logout-button"`
   - Scheduled cleanup (`internal/auth/cleanup.go`, started by `serve` in main.go a minute after start, hourly, stopped on SIGINT/SIGTERM): deletes dead sessions, login attempts older than 90 days, and used/revoked/expired links older than 30 days; never the audit log
   - Middleware: `auth.Middleware.RequireAuth()` + `RequireRole(admin, captain)` on the `/admin/league` mount, then a per-route role check
   - Protected routes: `/admin/league/*`

**Roles** (WI-132; the role matrix for users is `docs/roles.md`): `captain` does everything a captain does for every team (no per-team isolation; scope filters are a focus aid only); `admin` does that plus users, sessions, audit log, settings and feature switches, BHPLTA import configuration, seasons, divisions, tournaments, demo data, passkey clone warnings and VAPID reset. `models.RoleAtLeast(role, min)` is the one ordering (admin ⊇ captain; anything else grants nothing). The `player` role is retired: `accounts.ParseRole` refuses it, so no account can be given it; a leftover player-role account can sign in and use `/account/*` but gets 403 everywhere else.
- `/admin/league/*`: `adminRouteRoles` in `internal/admin/roles.go` maps every pattern to `captainRoute`, `adminRoute`, or a mixed rule (`captainReadsAdminWrites`, `captainExcept(fn)`) for patterns where some actions need an administrator. `handler.go` registers on a `guardedMux`, which panics at startup on a pattern with no entry and wraps each handler with the check. Templates gate links and forms with `canUse .User "/admin/league/..."` (or `"POST /admin/league/..."`), which reads the same table; never hand-roll a role check in a template. Tests: `TestEveryAdminRouteHasADeclaredRole`, `TestAdminRouteRoleMatrix`, `TestCaptainAccessSpec` (`internal/admin/roles_test.go`).
- A refused request gets `auth.WriteForbidden`: a friendly 403 page, or for HTMX a short text body with `HX-Reswap: none`. `RequireRole` uses it too.
- Other groups: push `/api/push/test` and `/api/vapid-reset` (admin, wrapper in main.go), `/account/passkeys/{id}/clear-warning` (admin), `/setup/demo*` (admin, checked in `internal/setup`), the rest of `/account/*` (any signed-in account).
- After sign-in and after link redemption everyone lands on `/admin/league/dashboard`, which both roles can use; the dashboard hides what the role cannot use. `users.player_id` (set by the user on `/admin/league/planning/link` or by an administrator on the users page, `POST /admin/league/users/{id}/player`; both go through `accounts.SetPlayerLink`, which refuses a player linked to another account and records the change) marks "My Teams" on the planning dashboard.

2. **Token-based (Fantasy Players)**: URL-embedded tokens for player availability
   - URL pattern: `/my-availability/{token}` where token is composite player names
   - Middleware: `auth.Middleware.RequireFantasyTokenAuth()`
   - No session, passwordless access for specific players

**Rate Limiting:** Password sign-in (`internal/auth/throttle.go`, from `login_attempts` over 15 minutes): 5 failures per username and address, and 30 failures per address across all usernames (`MaxLoginFailuresPerAddress`, password spraying); a throttled attempt is refused without being recorded, so it does not extend the block. The limits hold under concurrency (`internal/auth/login_gate.go`): `Service.Login` runs the throttle check, the compare and the record under a per-address in-memory lock (`addressLocks`, at most 8 waiting per address), and all bcrypt work in the process shares 2 slots (`bcryptSlots`); a sign-in that cannot get its turn within 5 seconds gets `ErrLoginBusy` and is not recorded. Every password sign-in costs exactly one compare: an unknown, inactive or passwordless account is compared against the fixed `dummyPasswordHash` (cost 12), so timing does not reveal usernames; a username over `MaxUsernameBytes` (128) is refused with no compare. Stored client data is cut (`ClipUsername`, `ClipUserAgent` 256 bytes: login attempts, sessions; audit rows 512/128 in `accounts.recordAudit`) and anonymous POST bodies are bounded with `http.MaxBytesReader` (`/login`, passkey begin/finish, setup forms, `/api/push/subscribe`). Passkey attempts are throttled per client address only, in memory (`internal/auth/passkey_ceremony.go`), and are never recorded against a username, so they cannot lock a named user out; the per-address password count leaves them out. All throttles key on `auth.ThrottleKey(r)` (the client address, an IPv6 one reduced to its /64), which is also what `login_attempts.ip` stores. The client address comes only from `clientIP` (`internal/auth/clientip.go`): the rightmost `X-Forwarded-For` entry when the peer is private/loopback (Caddy), else the peer. That is safe only while the app port is reachable solely through the proxy (never publish 8080 in production).

**Account audit log** (migration 035, `account_audit_log`): `accounts.recordAudit` inserts one row in the transaction that made the change (a failed insert fails the change); rows hold actor (user or CLI command), action, target account and a short non-secret detail, never passwords, tokens, hashes or session ids; no foreign keys (history outlives accounts); triggers refuse UPDATE and DELETE; retention is keep. Administrators' sign-ins are recorded through `auth.Service.SetSignInAudit` (wired in main.go to `accounts.Service.RecordSignIn`). Read-only page `GET /admin/league/audit-log` (`internal/admin/audit_log.go`, `templates/admin/audit_log.html`, administrators only, 50 per page, newest first).

**Passkeys (WebAuthn)** (`internal/auth/passkey*.go`, `internal/repository/webauthn_credential.go`, `static/passkey.js`, `github.com/go-webauthn/webauthn`; operator notes in `docs/PRODUCTION_QUICK_REFERENCE.md` section 11):
- Tables (migration 033): `webauthn_credentials` (one row per passkey; `credential_id` unique across users) and `webauthn_user_handles` (a random 32-byte handle per user, created on first enrolment; discoverable sign-in resolves the account from it, never from a username or numeric id).
- Routes: `POST /login/passkey/begin|finish` (public), `GET /account/signin-methods`, `POST /account/passkeys/begin|finish`, `POST /account/passkeys/{id}/rename|delete`, `POST /account/sessions/end-others` ("Sign out everywhere else") (all `RequireAuth`, own account only), `POST /account/passkeys/{id}/clear-warning` (admin).
- Sign-in is discoverable only: `Passkeys.BeginLogin` takes no username (a body naming one is read and ignored, and gets the same answer) and never lists an account's passkeys to an anonymous caller. The username-first path and its decoy ids are gone; `NewPasskeys` deletes a leftover `passkey_decoy_key` row at startup. Enrolment requires a resident key, so every passkey made here is discoverable; someone whose authenticator cannot hold one uses a password or a recovery link.
- User verification: `preferred` for ordinary accounts (a security key with no PIN works). For an administrator (`requireUserVerification`) it is required at enrolment (also through a link) and for confirmations, and a sign-in whose signed response lacks the user-verified flag is refused with `ErrUserVerificationRequired` (not counted as a failed attempt).
- The relying party is built on every request from `ClubSettings.ResolvePublicURL()` (`Passkeys.relyingPartyAddress`): `Origin` is the only accepted origin, `Hostname` the RP ID. No public URL means no passkeys, except `DEV_MODE=true` (never production) with a request to `localhost`, which uses `http://localhost:<port>`. The Host header is otherwise never used. Changing the public host invalidates existing passkeys.
- A ceremony's challenge lives in memory (`ceremonyStore`), keyed by a random ticket in the HttpOnly SameSite=Strict `passkey_ceremony` cookie, for 5 minutes, and is removed on the first finish attempt whatever the outcome. Registration ceremonies are also tied to the user and session that began them.
- A sign counter that does not increase sets `clone_warning_at`: that sign-in is refused and the passkey stays blocked until an administrator clears it on the sign-in methods page (dashboard shows a banner).
- `repository.CountSignInMethods` (via `auth.Service.CountSignInMethods`) is the one definition of "sign-in method": usable passkeys (no clone warning) + a non-empty password. The repository's `Delete` refuses a removal that leaves none, so the only working passkey cannot go while a blocked one remains, and a blocked one can go while a usable method remains. `Passkeys.SetAudit` is the audit seam (wired to `accounts.Service.RecordEvent` in main.go).
- Passwords are optional: `users.password_hash` is NULL for an account with passkeys only or an unused invite (`models.PasswordHash` scans NULL as ""; password sign-in is refused for it).

**Passwords (optional fallback)** (`internal/auth/password.go`, `password_section.go`, `reauth.go`, `admin_passkeys.go`, `passkey_prompt.go`; `internal/accounts/password.go`, `password_page.go`, `admin_passkeys.go`; operator notes in `docs/PRODUCTION_QUICK_REFERENCE.md` section 11):
- Policy (`auth.ValidatePassword`, new and changed passwords only; stored ones keep working): at least 12 characters, at most 72 bytes, not on the embedded `common_passwords.txt` (compared ignoring case and spaces), not containing the username, not one repeated character; no composition rules. Errors are `*auth.PasswordPolicyError` with a URL-safe `Code` (`auth.PasswordPolicyMessage`).
- `auth.BcryptCost` (12) is the one hashing cost; `Service.Login` rehashes a lower-cost hash after a successful password sign-in (`auth.NeedsRehash`), guarded on the old hash.
- The password form is the "Password" section of `/account/signin-methods`. `POST /account/password` adds or changes (`accounts.SetOwnPassword`), `POST /account/password/remove` removes (`accounts.RemoveOwnPassword`), `GET /account/password` redirects to the section. Results come back as `?done=password-added|password-changed|password-removed` or `?perror=<code>`. Removal is one UPDATE that requires a usable (not clone-blocked) passkey. Adding, changing and removing each end every session, then `StartSession` gives this browser a new one.
- Proof of ownership (`internal/auth/ownership.go`): a session alone never changes how an account signs in. Adding or removing a passkey and adding, changing or removing the password each need the current password (`Service.CheckCurrentPassword`; wrong ones limited per address, 5 / 15 min, one in-memory limiter for every form) or a passkey confirmation (below). `POST /account/passkeys/begin` takes `{"current_password"}` or needs a live confirmation, which `FinishRegistration` uses up; the delete and password-remove forms carry `current_password` or use the confirmation (`Passkeys.ProveOwnership`, `accounts.Proof`). A passkey added from a session cannot confirm anything for `NewPasskeyQuarantine` (10 minutes) nor in the session that added it (`enrolmentLog`, in memory; `ErrPasskeyTooNew`), unless it is the account's only usable method; a passkey enrolled through a link is not quarantined. No exemption for new accounts (they prove with the password or passkey they just set) and none for an account with no usable method (an administrator clears the block or issues a recovery link). After a passkey is added or removed the account's other sessions are ended (`Service.EndOtherSessions`, keeps the current one) and the audit detail records `reauth=password|passkey sessions_ended=N`.
- Re-authentication: the current password, or a passkey proof. `POST /account/reauth/passkey/begin|finish` (signed in, JSON) run an assertion limited to the user's own passkeys (ceremony kind `ceremonyReauth`, tied to the browser, the user and the session fingerprint); success mints a 256-bit proof kept in memory by hash (`reauthStore`), bound to user + session fingerprint, 5 minutes, sent as the HttpOnly SameSite=Strict `reauth_proof` cookie (path `/account/`). `Passkeys.ConsumeReauthProof` removes it on first use whatever the outcome; `SetOwnPassword` and `RemoveOwnPassword` only consume it after every other check passed. A sign-in is never proof. Its limiter keys on `ThrottleKey` like the others.
- After a password sign-in, an account with no passkey (where passkeys are available) lands on `/account/signin-methods?prompt=1&next=...` with a dismissible "Add a passkey?" box; `POST /account/passkey-prompt/dismiss` sets a 30-day `passkey_prompt_dismissed` cookie.
- "Require passkeys for administrators": `app_settings` key `require_admin_passkeys` = "true", read from the database every time (never cached, so the CLI `admin-passkeys off` applies to a running server). While on: admin password sign-in fails with the same `ErrInvalidCredentials` as a wrong password (checked after the bcrypt compare; no per-username hint, the login page states the rule to everyone); `RedeemLink` refuses a password for an admin (`ErrAdminPasskeyRequired`, link stays unused); the passkey repository's `Delete` refuses an admin's last usable passkey (`repository.ErrAdminNeedsPasskey`); `SetRole` refuses promoting an account without a usable passkey (`accounts.ErrAdminNeedsPasskey`); `SetActive` refuses enabling an administrator without one (`accounts.ErrEnableNeedsPasskey`). Turning it on (`accounts.SetAdminPasskeysRequired`, settings page `POST /admin/league/settings/admin-passkeys`, CLI) writes first, then re-checks in the same transaction that every active admin has a usable passkey (`*AdminsWithoutPasskeyError` names them) and that one exists. The sign-in rules apply only where passkeys are available (`Service.PasskeysAvailable`, set by `NewPasskeys`), so a missing public URL never locks admins out; the settings page refuses to change the stored public URL while it is on. All checks are in the same statement or transaction as the write (`repository.AdminLacksRequiredPasskeySQL`, `UsablePasskeysSQL`).
- Last-administrator guard (`lastActiveAdminCondition` in `internal/accounts/accounts.go`, inside the single UPDATE of `SetActive`/`SetRole`): refuses removing the only active administrator account, and the only administrator who can actually sign in (`repository.UsableAdminSQL`: active, with a usable passkey, or with a password while the requirement is off). An administrator who cannot sign in (unaccepted invite, password-only while passkeys are required) does not count and may itself be disabled or demoted while another active administrator account exists. The guard only stops removal: `user create --role admin`, `user reset` and `admin-passkeys off` always work.
- Cross-site protection is the app-wide `auth.CrossOriginGuard` (all passkey endpoints are POST); the JSON endpoints also require `Content-Type: application/json`. `auth.NoFraming` (outermost handler in main.go) sends `X-Frame-Options: DENY` and `Content-Security-Policy: frame-ancestors 'none'` on every response; nothing in the app frames its own pages.

**Invite and recovery links** (`internal/accounts/links.go`, `link_page.go`, `templates/account/accept_link.html`, admin side in `internal/admin/users_links.go`; operator notes in `docs/PRODUCTION_QUICK_REFERENCE.md` section 12). There is no email and nobody sets a password for someone else:
- An administrator creates an account with no password (`accounts.Service.Invite`: username, role, optional `player_id`) and is shown an invite link once; `IssueLink` makes a recovery link (`POST /admin/league/users/{id}/recover`, CLI `user reset`) or a fresh invite (`.../{id}/invite`, only while the account has no sign-in method; CLI `user create`). `POST /admin/league/users/links/{id}/revoke` cancels an unused link. The link is rendered on the POST response itself, never put in a redirect.
- Table `account_links` (migration 034): 32 random bytes as the token, only its SHA-256 stored, kind `invite` (48h) or `recovery` (1h), `used_at`, `revoked_at`. Issuing a link revokes the user's earlier unused links and changes nothing else about the account.
- `GET /account/link/{token}` shows who the link is for and changes nothing. `POST /account/link/{token}` sets a password; `POST /account/link/{token}/passkey/begin|finish` enrol a passkey (offered only where passkeys are available). No session is needed. `RedeemLink` consumes the link with the first statement of its transaction (`UPDATE ... WHERE used_at IS NULL AND revoked_at IS NULL`), sets the method, ends every session of the account, optionally removes the old passkeys and password (recovery only, the person's choice), and records `link.use`; then `auth.Service.StartSession` signs the browser in. Disabled accounts cannot redeem.
- A link passkey ceremony is tied to the browser (ceremony cookie), the account and the link (`accounts.LinkBinding(token)` stands where the session fingerprint does in `Passkeys.BeginLinkRegistration` / `VerifyLinkRegistration`); the verified passkey is stored inside the redeeming transaction, so a link that loses a race leaves nothing behind.
- Every link response has `Cache-Control: no-store` and `Referrer-Policy: same-origin` (as do the admin users page and the setup pages; never `no-referrer`, which makes browsers post the page's own forms with `Origin: null`, and the cross-origin guard refuses that wherever the browser sends no `Sec-Fetch-Site`, i.e. plain http to a host other than localhost); wrong, expired, used, revoked and disabled-account links all get the same 404 page; failures are throttled per client address (10 per 15 minutes, in memory). The token must never be logged: the page logs no paths, and anything that logs a request path that could be a link uses `auth.RedactPath` (the cross-origin guard and the home handler's "Not found" do).
- The setup gate lets `/account/link/` through once an administrator exists (`StatusNeedsClub`), not before.
- `must_change_password` (migration 032) is retired: the column stays, always FALSE, and nothing reads it. Migration 034 makes `password_hash` nullable by add-copy-drop-rename of the column, never by rebuilding `users` (foreign keys are on and migrations run in a transaction, so dropping `users` would cascade-delete sessions and passkeys); use the same approach for any future change to `users`.

### BHPLTA Integration

**Match Card Import Flow:**
1. Extract WordPress nonce from BHPLTA website (automated via `nonce_extractor.go`)
2. Fetch match card HTML from BHPLTA API using nonce
3. Parse HTML to extract match details (`matchcard_parser.go`)
4. Match players to local database (`player_matcher.go`)
5. Import match results and update fixture status

**Services:**
- `NonceExtractor`: Scrapes BHPLTA website for WordPress nonce
- `MatchCardParser`: Parses match card HTML into structured data
- `PlayerMatcher`: Fuzzy matches external player names to internal database
- `MatchCardService`: Orchestrates the import process

### Template Rendering

**Server-Side Rendering:**
- Templates loaded with custom functions (e.g., `currentYear`)
- Admin handlers render to `templates/admin/*.html`
- Players handlers render to `templates/players/*.html`
- Uses Go's `html/template` package

**Sub-Handler Pattern:**
Each major handler (Admin, Players) delegates to domain-specific sub-handlers:
- `admin.Handler` → `DashboardHandler`, `PlayersHandler`, `FixturesHandler`, etc.
- `players.Handler` → `AvailabilityHandler`

### Configuration

**Home Club:** resolved in the order `HOME_CLUB_ID` env, `HOME_CLUB_NAME` env, then the `home_club_id` row in `app_settings` (written by first-run setup). With none of them the server still starts, in setup mode: `internal/setup` gates every request except `/static/`, `/branding/`, `/healthz` and `/setup...` to the setup page. The setup flow (`/setup` code, `/setup/admin`, `/setup/club`, `/setup/done`) is protected by a one-time code printed to the log at startup (only its hash is stored); code entry and administrator creation are 404 once any administrator exists. A home club in effect with no administrator account (captain/player accounts do not count) is also gated (`setup.StatusNeedsAdmin`): a code is printed and the flow creates the administrator, then goes straight to the dashboard. The club step calls `setup.HomeClubProvisioner`; the server uses `setup.NewBHPLTAProvisioner`, which runs in the background (the page polls) and imports the league, all clubs (with venue details), and the current season's weeks, divisions, teams and fixtures from BHPLTA via `internal/bhplta` in one transaction with the completion settings. The same import is the admin "Start a new season from BHPLTA" action (`/admin/league/seasons/from-bhplta`) and `cmd/import-season`. No migration seeds user accounts. An env value matching none of the clubs in a non-empty clubs table is fatal at startup (lists the available clubs). The home club is held in a live `config.HomeClubHolder`, never a startup-frozen value.
- `HOME_CLUB_ID`: Database ID of the home club (preferred)
- `HOME_CLUB_NAME`: Club name for fuzzy lookup (fallback if `HOME_CLUB_ID` is not set)
- `HOME_CLUB_LOGO_PATH`: Optional URL path to a club logo image; overrides the logo uploaded on the settings page. Unset (or empty) means the uploaded logo, else the bundled logo of a club in `config.KnownBHPLTAClub`'s list (St Ann's: `/static/st-anns-tennis.jpg`), else no logo.
- `BHPLTA_CLUB_CODE`: Club code on the BHPLTA website — used for match card imports; overrides the code stored on the settings page
- `PUBLIC_URL`: The instance's public base URL; overrides the one stored on the settings page. Required (valid, https) when `APP_ENV=production`: `config.ValidateStartup` (called from main.go) refuses to start otherwise. Read it only through `ClubSettings.ResolvePublicURL()` (validated `PublicURL{Origin, Base, Scheme, Hostname}`; passkeys and recovery links use this, Hostname is the relying-party ID) or `ClubSettings.BaseURL(r)` / `Host(r)` for links and identifiers, which fall back to the request host only when no public URL is set (development); that fallback lives in `config.RequestBaseURL`. The web push VAPID subscriber (`PushSubscriber()`) and the iCal UID domain (`UID:fixture-<id>@<public host>`) derive from it. Changing the public host changes iCal UIDs, so calendar apps treat existing subscribed events as new: keep the host stable (the live site's host is the UID domain its subscriptions already use).

**Club settings page** (`/admin/league/settings`, admin role only): display name, logo, primary/secondary colours, BHPLTA club code/name/id and public URL, stored in `app_settings` and held live in `config.ClubSettings` (`HomeClubHolder.ClubSettings()`); changes apply without a restart. A non-empty env variable above always wins over the stored value and the page says so. Uploaded logos (PNG/JPEG/WebP, sniffed by content, max 2 MB, no SVG) are stored in `branding/` next to the SQLite database (`DB_PATH`'s directory, i.e. the data volume) and served only at `/branding/<content-hash name>`. `internal/branding` also serves `/static/manifest.json`, `/api/manifest` and `/static/service-worker.js` with the stored name, logo and colours. BHPLTA's club name and id are not published: the page can look them up from the club code, `config.KnownBHPLTAClub` covers clubs whose values are known, and `config.PrefillClubSettings` stores them during setup.

**Features:**
- `WRAPPED_ACCESS_PASSWORD`: Optional override for the Club Wrapped access code. Unset (the default) means a code generated with crypto/rand on first start and stored in `app_settings` (`wrapped_access_code`), shown to admins on the Club Settings page (`internal/secrets`). The grant cookie is derived from the code, so it is per instance.
- `MY_TENNIS_ENABLED`: `true`/`1` (or `false`/`0`) overrides the stored My Tennis switch on the settings page (Features section, with Tournaments and Club Wrapped). Defaults with nothing stored: My Tennis off, Club Wrapped on, Tournaments on unless setup_completed_at exists (new installs). A switch that is off hides its links and its routes return 404; with Tournaments off the home page is a club page.
- `LEAGUE_TABLES_URL`: Optional source page for the fixture generator's league tables (default: BHPLTA league tables page)
- `BHPLTA_BASE_URL`: Testing only; site root the BHPLTA season import reads the fixtures and club pages from (default `https://www.bhplta.co.uk`). Saved pages for tests are in `tests/e2e/fixtures/bhplta/`
- `COURTHIVE_API_URL`: Optional CourtHive API URL (default: `http://courthive-server:8383`)

**Database:**
- `DB_TYPE`: optional; only "sqlite3" is supported (any other value fails at startup)
- `DB_PATH`: SQLite file path (default: "./tennis.db")
- Every connection is opened with `database.DSN` (`_pragma=` busy_timeout 5000, WAL, synchronous NORMAL, foreign_keys on, and `_time_format=sqlite` so `time.Time` is stored as `2006-01-02 15:04:05.999999999-07:00`, the layout existing rows were written in); `database.New` refuses to return a database whose connection lacks foreign keys, WAL or the busy timeout (`CheckConnectionSettings`). Open SQLite only through `database.New`/`database.DSN`, never `sql.Open("sqlite", path)` directly: without `_time_format` the driver writes `time.Time.String()`.

**Application:**
- `PORT`: HTTP server port (default: "8080")
- `DEV_MODE`: `true`/`1` is the only way to turn off Secure session cookies (plain-http development). Set by the root `docker-compose.yml` (app, app-e2e, app-fresh), `make run-local` and `scripts/run.sh`; fatal with `APP_ENV=production`. Any new way of serving the app over http must set it.
- `APP_ENV`: Set to "production" for a live instance (the reference compose sets it): needs a valid https `PUBLIC_URL` and refuses `DEV_MODE`. In production the startup self-check (`secrets.StartupCheck`) refuses to start if a secret variable holds a known placeholder such as `change-me`. It also warns (in every mode) about accounts whose password hash is one seeded by old migrations or the e2e seed.
- `TEMPLATE_RELOAD`: Development only; `true` reads `templates/` and `static/` from disk (the project root) instead of the built-in copies, re-parses templates on every request and never marks static files immutable (root `docker-compose.yml` defaults it to true and bind-mounts the two directories for the `app` service only; the e2e and fresh-install services use the built-in copies)

**Reference deployment:** `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example` describe the production stack (app, CourtHive, Caddy); `.env.example` lists every variable the stack and the app read. `scripts/deploy-app.sh` does not sync them to the server. The root `docker-compose.yml` is for local development and E2E. Superseded variants are in `docs/archive/`.

### CSV Import Format

Fixture CSV files expected format:
```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
10,June 19,,,Hove,Dyke A
```

Club names extracted from team names (e.g., "Dyke A" → Club: "Dyke").

## Development Patterns

### Adding a New Entity

1. Define model in `internal/models/`
2. Create migration files in `migrations/` (up and down)
3. Create repository in `internal/repository/`
4. Add service logic if needed in `internal/services/`
5. Create handlers and templates for UI

### Adding a New Admin Feature

1. Create sub-handler in `internal/admin/` (e.g., `new_feature.go`)
2. Declare each route pattern's role in `adminRouteRoles` (`internal/admin/roles.go`): `captainRoute` for captain work, otherwise `adminRoute`
3. Register routes in `internal/admin/handler.go` `RegisterRoutes()` (the guarded mux refuses undeclared patterns and enforces the role)
4. Add templates in `templates/admin/`; gate links and forms with `canUse .User "..."`

### Working with Migrations

- Migrations are numbered sequentially: `001_`, `002_`, etc.
- Always create both `.up.sql` and `.down.sql` files
- Migrations run automatically on app startup; a migration that fails stops the server (`migrateOrStop` in main.go), it is not a warning
- A failed or dirty migration stops startup with a `database.MigrationError` that names the migration and lists the recovery steps (stop, restore the pre-migration backup with `jim-dot-tennis restore` using the previous version, start the previous version, report). The version is never forced, in any mode; `MIGRATE_ALLOW_DIRTY_FORCE` no longer exists
- `/healthz` (`internal/health`) is 200 only when the database answers and `schema_migrations` is clean and at the latest migration the build ships; the Dockerfile HEALTHCHECK and the compose files call it
- Before migrations change an existing database, `backupThenMigrate` (main.go) takes a pre-migration backup (`backup.BeforeMigrations`: not for an empty database, not when the schema is current, not over a dirty one); if it cannot be made, startup stops without migrating
- Version, commit and build date come from `internal/version`, set by `-ldflags -X` (Dockerfile build args `VERSION`, `COMMIT`, `BUILD_DATE`); shown in the admin footers (`appVersion` template func), at public `/version` (version only, no commit) and by `jim-dot-tennis version` (all three)

### Backups

`internal/backup` (operator steps: `docs/PRODUCTION_QUICK_REFERENCE.md` section 9). An archive is `jim-dot-tennis-backup-<UTC time>.tar.gz` (pre-migration ones `jim-dot-tennis-pre-migration-...`): `manifest.json` first (format, app version and commit, schema version, created_at, kind, SHA-256 and size of every file), then `database/tennis.db` (made with `VACUUM INTO`, never by copying the live file) and `files/branding/<logo>`. Archives are written under a `.tmp-` name, read back and verified, then renamed, 0600 in a 0700 folder (`BACKUP_DIR`, default `backups/` next to the database; never served: `/static/` is the embedded tree and `/branding/` serves only the current logo name). The server holds an flock on `<data dir>/.jim-dot-tennis.lock` for its lifetime (taken in main.go before the database opens; a second server or a server during a restore refuses to start). `Restore` takes the same lock (refuses with `ErrServerRunning` while it is held), treats the archive as untrusted (only the three entry shapes above, regular files only, size limits, checksums), refuses a newer schema (`ErrNewerBackup`), migrates an older one, ends every session and cancels unused links in the restored database, records `backup.restore` in its audit log, then swaps it in with one rename after keeping the old database aside (`tennis.db.replaced-<time>`, hard link or copy, with its `-wal` moved beside it) and removing the old `-shm`. Scheduled backups run in the server (`backup.Service.Start`, stopped with the cleanup on shutdown; `BACKUP_INTERVAL` default 24h or `off`, `BACKUP_KEEP` default 14, never below 1; pre-migration backups are not counted, and on a successful start all but the newest are removed). Admin page `GET /admin/league/backups` (administrators); the download is `POST /account/backup` (administrator, needs the current password or a passkey confirmation, because the passkey proof cookie is limited to `/account/`; audited as `backup.download`; the archive is a temporary one, not kept).

## Key Files

- `cmd/jim-dot-tennis/main.go`: Application entry point, routing setup
- `internal/config/config.go`: Home club resolution order (`ResolveHomeClub`) and `AppConfig` for CLI tools
- `internal/config/holder.go`: Live, concurrency-safe home club (`HomeClubHolder`) used by the server
- `internal/setup/`: Setup state (`State`, `Gate`, `MarkComplete`) and the setup entry page
- `internal/repository/settings.go`: Typed access to the `app_settings` key/value table
- `internal/config/context.go`: Context helpers for passing config through request context
- `internal/config/middleware.go`: Middleware that injects config into every request
- `internal/normalize/normalize.go`: Centralised apostrophe/Unicode normalisation
- `internal/database/database.go`: Database connection and migration execution
- `internal/admin/handler.go`: Admin route registration
- `internal/players/handler.go`: Player route registration
- `internal/auth/middleware.go`: Authentication middleware
- `internal/services/matchcard_service.go`: BHPLTA integration orchestration
