# Technical Debt Audit — jim.tennis (July 2026)

**Date:** 2026-07-02
**Scope:** Full codebase — `internal/` (~35.5k lines Go), `cmd/` (13 commands), `templates/` (63 files, ~22.3k lines), `static/`, `migrations/` (001–028), `tests/`, Docker/compose/Makefile/scripts, docs.
**Method:** Six parallel audits (backend architecture, data layer, frontend, testing/CI, security, build/deploy), every finding verified against source. Key claims independently re-verified before publication.

> **Revised 2026-07-02 after owner review.** Several findings were recalibrated against the project's actual risk posture (solo maintainer, deliberately low-stakes public surfaces):
> - **Backups:** DigitalOcean droplet snapshots are already taken — the "no offsite backup" finding is downgraded. Residual: a restore has never been tested (§1.1).
> - **Wrapped password:** `WRAPPED_ACCESS_PASSWORD` is a deliberately decorative gate ("secret zone" feel, not security). Its committed values, the static-cookie bypass, and the non-constant-time compare are **accepted by design**. The CourtHive admin password in docs remains a real finding (§1.2).
> - **Fantasy tokens:** guessability is **by design** — the goal is resistance to crawlers/bots, not humans, and the intended blast radius of a guessed token is small writes only (availability, name request, profile edit). Entropy/rate-limit findings downgraded to optional; the open question is the PII read-back, which exceeds that intended blast radius (§2.1).
> - **CI:** downgraded for a solo-maintainer project — full CI carries less weight without a team; wiring the existing unit tests into `make test` remains worthwhile (§1.3).
> - **SQLite pragmas:** ✅ **fixed 2026-07-02** — WAL, busy_timeout, synchronous=NORMAL, and foreign_keys enabled in `internal/database/database.go`, with a regression test (§3.1).
>
> **Follow-up fixes 2026-07-02 (second pass):**
> - **`make test` target:** ✅ done — Docker-based `make test` (PKG/ARGS passthrough) folded into `make check`; documented in CLAUDE.md (§1.3).
> - **Migration footguns:** ✅ done — wrote the missing 012 down file (verified round-trips in isolation); `migrate-down` now requires an explicit `-version` with a confirmation prompt (no more silent default of 5); dirty-state startup now fails fast instead of auto-forcing, with `MIGRATE_ALLOW_DIRTY_FORCE=true` as a dev-only escape hatch; README updated (§3.4). **Newly discovered:** full rollback below v16 is still blocked by a *separate* pre-existing bug — migration 013's `chk_preferred_name_unique` trigger conflicts with the table-rebuild down files in 015/016 (`no such table: main.players`). Filed as a follow-up (§3.4).
> - **Docker build waste:** ✅ done — dropped `go build -a`, added BuildKit cache mounts (module + build cache), pinned the final stage to `alpine:3.23` (matches the `golang:1.25-alpine` builder), aligned `Dockerfile.import` to Go 1.25, and rewrote `.dockerignore` to exclude `.env`, `.go-mod-cache/`, the root binary, and db backups from the build context (§7.1).

---

## Executive summary

The codebase's core patterns are healthy: a consistent repository layer with parameterized SQL throughout (no SQL injection found), bcrypt password hashing, centralized admin auth middleware with correct cookie flags, thorough index coverage, clean pointer-based models, and good e2e test infrastructure. The debt is concentrated in five clusters:

1. **Operational risk** — untested restore path, no rollback path, deploy script gaps, one real credential committed to git. *(Backups: mitigated by droplet snapshots — see revision note.)*
2. **Security gaps on the public surface** — unauthenticated push endpoints; fantasy-token guessability is accepted by design, but the PII read-back exceeds the intended blast radius.
3. **Correctness time bombs** — Club Wrapped has no season filter (wrong next season), duplicated derby scoring paths, multi-step writes without transactions.
4. **Platform misconfiguration** — ~~SQLite opened without WAL/busy_timeout/foreign_keys against a 25-connection pool~~ ✅ fixed 2026-07-02.
5. **Presentation-layer mass** — 57% of template lines are inline `<script>`/`<style>`, templates re-parsed from disk per request, a verbatim-forked template pair.

Almost nothing here requires a rewrite. The highest-risk items are mostly S-effort fixes; the large items (template extraction, N+1s, handler SQL) can be done incrementally.

---

## Top improvement candidates (ranked)

| # | Candidate | Risk addressed | Effort | Status |
|---|-----------|----------------|--------|--------|
| 1 | Rotate CourtHive admin password + scrub from docs | Prod admin compromise | S | open |
| 2 | Deploy hardening (sync set, pre-deploy backup, image tag rollback) | Bad deploy = data loss window | S–M | open |
| 3 | Test a droplet snapshot restore; confirm snapshot cadence | Untested recovery path | S | open |
| 4 | Auth-gate push endpoints | Anonymous broadcast to all subscribers | M | open |
| 5 | Club Wrapped: season filter + stop swallowing errors | Publicly wrong stats next season | M | open |
| 6 | Startup template cache + honest 500s | Per-request disk I/O on 1-CPU box; silent template breakage | M | open |
| 7 | Unify matchcard derby code paths | League-scoring divergence between import types | M | open |
| 8 | Transactions on season copy/create/activate + result saves | Half-written seasons and match cards | M | open |
| 9 | De-fork `fixture_team_selection` templates via partial | Silent UI drift after every HTMX swap | M | open |
| 10 | Migration footguns (012 down file, migrate-down default, dirty auto-force) | Destructive/dirty schema states | S | ✅ done 2026-07-02 |
| 11 | Unit tests for parser/matcher/points + `make test` target | Silent data-corrupting regressions | M | partial — `make test` ✅, parser + matcher tests ✅ 2026-07-02; points-calc golden test open |
| 12 | Docker build speed (`-a`, cache mounts, `.dockerignore`) | 1-CPU server pegged per deploy; secrets in build context | S | ✅ done 2026-07-02 |
| 13 | Decide on fantasy-token PII read-back (exceeds intended blast radius) | Guessed token reads full name + match history | S (decision) | open |
| 14 | CI pipeline | Deploy gating (low weight for solo maintainer) | M | deprioritized |
| — | SQLite pragmas (WAL, busy_timeout, foreign_keys) | Lock errors, unenforced cascades | S | ✅ done 2026-07-02 |
| — | Offsite backups | *Superseded: droplet snapshots in place* | — | accepted |
| — | Fantasy token entropy + rate limiting | *Accepted by design (crawler-resistance only)* | — | accepted |

---

## 1. Operational risk (highest priority)

### 1.1 Backup recovery path untested — MED *(revised: was "backups never leave the droplet", HIGH)*
**Mitigated by droplet snapshots** (owner confirmed, 2026-07-02). Residual risks: (a) a restore from snapshot has never been exercised — the recovery path is assumption, not procedure; (b) snapshot cadence determines the data-loss window (manual snapshots drift; DO scheduled snapshots are weekly, so the in-container daily sqlite backup is still the finer-grained layer — but it lives on the snapshotted disk, which is fine). The in-repo claims remain wrong: `docs/docker_setup.md:141` says S3/B2 offsite support exists but `scripts/backup-manager.sh` has only commented-out template code.
**Fix (S):** do one test restore from a snapshot to a throwaway droplet and write the steps into `docs/PRODUCTION_QUICK_REFERENCE.md`; confirm snapshots are on a schedule, not memory; correct the docker_setup.md claim.

### 1.2 CourtHive admin password tracked in git — HIGH *(revised: wrapped passwords accepted)*
`git grep` confirms the CourtHive admin password in `docs/PRODUCTION_QUICK_REFERENCE.md` (~line 389) and `COURTHIVE_SETUP.md`. The repo has a GitHub remote; it lives in history forever. *(The `WRAPPED_ACCESS_PASSWORD` values also present in docs are accepted as committed — the gate is deliberately decorative, see §2.5.)*
**Fix (S):** rotate the CourtHive admin password; replace the doc value with a pointer to the server `.env`.

### 1.3 No CI pipeline; unit tests wired into nothing — PARTIAL *(revised: was HIGH — solo maintainer)*
**`make test` ✅ done 2026-07-02** — a Docker-based `make test` target (using the `DOCKER_GO_CGO` pattern, with `PKG=`/`ARGS=` passthrough) now runs the Go unit tests and is folded into `make check`. A GitHub Actions workflow remains a nice-to-have (see below).
`.github/` has issue/PR templates but **no `workflows/` directory**. For a solo-maintainer project this carries less weight than it would for a team — nobody else's broken commits need gating. The part that still bites solo: there is no `make test` target anywhere, so the existing unit tests (456 lines vs 35.5k production lines, ~1.3%) can only be run by hand-crafting a Docker command and will silently rot. `make check` (vet/fmt/lint/deadcode) excludes them.
**Fix (S for the part that matters):** add a Docker-based `make test` target using the existing `DOCKER_GO_CGO` pattern and fold it into `check`, so the pre-deploy habit is one command. A GitHub Actions workflow (M) remains a nice-to-have safety net for future contributors, not a priority. If ever added, note the CI e2e job will hit the fresh-volume bootstrap deadlock (§6.3).

### 1.4 Deploy fragility: sync gaps, no rollback, repo/prod divergence — PARTIAL
- ~~`scripts/deploy-app.sh:67` syncs only `internal cmd templates static migrations` — **not** `go.mod`/`go.sum`, nor `Dockerfile`~~ ✅ **fixed 2026-07-02** — the script now also syncs `go.mod`, `go.sum`, `Dockerfile`, `Dockerfile.import`, `.dockerignore`, and (best-effort) takes a pre-deploy sqlite `.backup` + tags the running image `jim-dot-tennis-app:prev` before building. Verified against prod (deployed the §7.1 Dockerfile this way: image is now Alpine 3.23.5 and `.env` no longer leaks into it). Compose files remain **deliberately unsynced** (prod compose has diverged from the repo — see below).
- `docker-compose.courthive.yml` in the repo still describes the **pre-Postgres** stack (LevelDB/net-level, hardcoded `admin/adminpass`, port 4040 published). Production's post-cutover compose exists only as a doc artifact (`docs/courthive_postgres_phase2c.docker-compose.courthive.yml`). Disaster recovery from the repo would resurrect the dead stack.
- No rollback: `docker compose build app` overwrites the only image; migrations auto-apply on start with no pre-deploy backup; daily backup means a botched migration can cost up to 24h of data.
- Two stale deploy scripts linger: `deploy-with-import.sh` rsyncs `./` wholesale (would push `.go-mod-cache/` 228MB + local `.env` over prod's) and `deploy-digitalocean.sh` half-duplicates config.
**Fix (S–M):** extend `deploy-app.sh`'s sync set; add pre-deploy sqlite `.backup` + `docker tag jim-dot-tennis:latest :prev` before build; copy the real prod compose back into the repo; delete `deploy-with-import.sh`.

### 1.5 `make clean` / `down -v` destroys the data volume unguarded — PARTIAL
Both `clean` and `test-e2e-clean` run `down -v`, removing the live database volume. On the server (where the quick-ref encourages compose commands) this deletes prod data with only a same-disk backup. The 100-line `scripts/test-e2e-safe.sh` snapshot machinery exists because this footgun has teeth.
**✅ Confirmation guard done 2026-07-02:** both targets now require typing `delete` (or `FORCE=1` for scripted use) before running `down -v`, with a warning naming the shared `jim-dot-tennis-data` volume. Verified: empty/wrong input aborts before compose runs; `delete`/`FORCE=1` proceed. **Still open (M):** give the e2e profile its own `tennis-data-test` volume so the cleanup can't touch prod data at all, retiring `test-e2e-safe.sh`.

---

## 2. Security

### 2.1 Fantasy token guessability — ACCEPTED BY DESIGN *(revised: was HIGH)*
`internal/repository/tennis_player.go:627-651` — tokens are four pro-tennis surnames joined by underscores. **Owner-confirmed design (2026-07-02):** the four-surname combination space is large enough to resist crawler/bot discovery, which is the actual threat model; human-effort guessing is tolerated because the intended blast radius of a guessed token is small: update someone's availability, submit a name request, edit a profile. No code change planned.
**The residual finding is the blast radius itself, not the token (S, decision):** since Sprint 016 the profile routes read back the player's real first/last name and match history (`internal/players/profile.go:33-73, 115-122`) — a guessed token now *reads* PII, exceeding the intended writes-only consequence. Either accept that explicitly or trim the read-back (initials, no history) on token-authenticated surfaces. The `/api/push/status` token-validity oracle (§2.3) also matters more under this posture, since it lets a bot cheaply distinguish valid tokens.

### 2.2 No rate limiting on token endpoints — LOW *(revised: was HIGH)*
`internal/auth/middleware.go:153-198` (`RequireFantasyTokenAuth`) has no attempt throttling. Under the accepted-guessability posture this is optional hardening rather than a gap — but note it is the cheap control that keeps "resistant to automated guessing" true if bots ever do hammer the endpoint: a simple per-IP failed-lookup backoff (M) directly serves the stated design goal without touching tokens.

### 2.3 Unauthenticated push-notification endpoints — MED
`internal/webpush/handlers.go:16-27` — `/api/push/test` lets **any anonymous caller broadcast an arbitrary push to every subscriber**; `/api/push/test-player` targets any player token; `/api/push/status` is an oracle that confirms whether a token is valid (aids 2.1/2.2 enumeration).
**Fix (M):** admin-gate test/test-player; remove or rate-limit the status oracle.

### 2.4 Session and CSRF hardening — MED
- ~~Session tokens logged in plaintext on every request (`auth/middleware.go:58`, `auth/service.go:220-221`)~~ ✅ **fixed 2026-07-02** — added a `redactToken` helper (non-reversible `sha256:` fingerprint) and applied it to all 8 session-ID log sites across `auth/{middleware,service,handlers}.go`. First unit test in `internal/auth` (`service_test.go`) asserts the raw token never appears. Remaining debug-spam volume is unchanged (fingerprints still print), which is acceptable now that they are non-sensitive.
- No CSRF protection anywhere; several destructive admin actions are plain GET links (`/seasons/delete`, `/tournaments/toggle-visibility/`). `SameSite=Strict` is the only mitigation. **Fix (M):** CSRF token for admin POSTs; convert destructive GETs to POST.
- ~~Sliding session expiry with no absolute cap (`auth/service.go:194-199`) — a stolen token in use never expires.~~ ✅ **fixed 2026-07-02** — added `Config.AbsoluteSessionDuration` (default 30d, 0 disables) enforced in `ValidateSession` against `session.CreatedAt`, so a continuously-refreshed session dies at the ceiling. Covered by `TestValidateSessionAbsoluteLifetimeCap` (old-but-active session rejected, fresh one passes, cap-disabled survives).
- Login throttle keyed on username+IP (`auth/service.go:262-280`) — evaded by IP rotation or password spraying; and it fetches `LIMIT 5` rows before filtering by window. **Fix (M).**

### 2.5 Smaller items — LOW/MED
- ~~The "Club Wrapped" gate checks `cookie.Value == "granted"`~~ — **accepted by design (2026-07-02):** the wrapped password/gate is deliberately decorative ("secret zone" feel, not access control). The static cookie value and non-constant-time compare are fine as-is. No action.
- HSTS only in the commented-out block of `Caddyfile.courthive`; no CSP/Referrer-Policy. Confirm live config. **Fix (S).**
- App port `8080:8080` published on the host (`docker-compose.yml:10-11`) — plaintext bypass of Caddy if the firewall doesn't block it. **Fix (S):** bind `127.0.0.1` or drop the publish.
- **Verified clean:** no SQLi (parameterized throughout), no `template.HTML`/XSS, no path traversal, no hardcoded secrets in code/compose, non-root Docker user, current deps. Add `govulncheck` to CI (S).

---

## 3. Data layer

### 3.1 SQLite opened with no pragmas, 25-connection pool — ✅ FIXED 2026-07-02
`internal/database/database.go` — the DSN was the bare file path: no `_journal_mode=WAL`, no `_busy_timeout`, no `_foreign_keys=on`, with `SetMaxOpenConns(25)`. Consequences: concurrent writes failed immediately with `database is locked` (what the e2e suite's `retries: 1` and session-fallback fixture were papering over), and **every `FOREIGN KEY` and `ON DELETE CASCADE` clause in the schema was silently unenforced**.
**Applied:** DSN is now `file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=on`, with a regression test (`internal/database/database_test.go`) asserting all three pragmas apply. `PRAGMA foreign_key_check` on the local prod-pull DB returned zero violations. **Before the prod deploy:** run `PRAGMA foreign_key_check` against the live DB (one docker exec) — if it reports rows, clean the orphans first; new FK-violating writes will now fail loudly, and cascades will actually cascade (verify season-delete behavior is still what you want). After it beds in, remove the e2e retry workarounds (`playwright.config.ts:8`, session-fallback in `test-fixtures.ts`).

### 3.2 Missing transactions on multi-step writes — HIGH
No `Begin` in any of: `CreateSeasonWithWeeks` (`service_seasons.go:61-99`), `CopyFromPreviousSeason` (`:305-431` — which also discards errors: `_ = s.teamRepository.AddPlayer(...)` at 411, `AddCaptain` at 425), `SetActiveSeason` (`:103-133` — failure between deactivate-all and activate-one leaves **zero** active seasons), `SaveMatchupResults` + `MirrorDerbyResults` (`service_matchups.go:350-437, 486`). Season copy runs once a year under time pressure — the worst moment for a half-copied season with silently missing players. The right pattern already exists in 5 places (e.g. `repository/season.go:122 DeleteCascade`).
**Fix (M):** wrap the four flows in `BeginTxx`; propagate the discarded errors.

### 3.3 Club Wrapped: 53 raw SELECTs, swallowed errors, no season filter — HIGH
`internal/admin/club_wrapped.go` — the largest concentration of SQL outside the repository layer; 14 queries use `_ = ...Scan(...)` so schema drift renders zeros silently on a **public page**; `grep -c season_id` returns **0** — every stat spans all seasons — and `SeasonYear: 2025` is hardcoded (line 593). The moment a second season has finished matchups, every Wrapped stat is wrong.
**Fix (M):** thread `season_id` through all queries, resolve year from the active season, log errors. Extracting to a `WrappedStatsService` is polish; the season filter is urgent.

### 3.4 Migration footguns — ✅ MOSTLY FIXED 2026-07-02
- ~~Migration **012 has no down file**~~ ✅ **fixed** — `012_add_match_card_fields.down.sql` written and verified to round-trip (up 12 → down 11 → up 12) in isolation.
- ~~`cmd/migrate-down/main.go:21` **defaults to rolling back to version 5**~~ ✅ **fixed** — `-version` is now required (no default), a `>= current` target is rejected, and a confirmation prompt guards the rollback (`-yes` to skip).
- ~~`internal/database/database.go:127-133` **auto-forces dirty migration state**~~ ✅ **fixed** — startup now fails fast on a dirty DB with a remediation message; auto-force is retained only behind the dev-only `MIGRATE_ALLOW_DIRTY_FORCE=true` flag.
- ~~Numbering gap: 026 was never committed~~ ✅ **documented** in `migrations/README.md` ("never reuse 026").
- **NEW follow-up (found while verifying the above) — MED:** full rollback below **v16** still fails with `error in trigger chk_preferred_name_unique: no such table: main.players`. Migration 013 adds a trigger on `players`; the table-rebuild ("recreate players_new, drop players, rename") down files for **015** and **016** fire/dangle that trigger mid-rebuild. Down migrations are dev/test-only, so this is not a prod hazard, but it does mean versions 1–15 remain unreachable via rollback until 013/015/016's down files drop-and-recreate the trigger (or the rebuilds run with the trigger temporarily removed). Left as a distinct fix.

### 3.5 N+1 queries and misc — MED/LOW
50+ verified N+1 sites in admin services — worst: the team-selection screen runs two availability queries **per player** (`admin/fixtures.go:1771-1780`, 100+ queries per request), per-fixture team/week lookups in list loops (`service_fixtures.go:310-331, 558-579`), per-fixture lookups in the points table (`points.go:596-598`). **Fix (M):** batch `FindByIDs` methods for teams/weeks/players + one joined availability query covers ~80% mechanically.
Also: ~123 lines of SQL in 12 non-repo files (sessions/users queried from two packages with no shared repo; ~~`SELECT *` in `webpush.go:186,210` breaks on column adds~~ ✅ **fixed 2026-07-02** — both now use an explicit `subscriptionColumns` const, guarded by a reflection test that keeps it in lockstep with the struct's `db` tags); `context.Background()` ~117× in admin services so request cancellation never propagates; date functions wrapped around indexed columns defeat `idx_fixtures_scheduled_date` (`repository/fixture.go:387,606`). Index coverage otherwise verified good; models verified clean (consistent pointer-based nullables, no phantom fields).

---

## 4. Backend architecture

### 4.1 Duplicated derby scoring paths in matchcard import — HIGH
`internal/services/matchcard_service.go`: `processMatchup` (line 435) vs `processMatchupForTeam` (1199), and `processMatchupPlayers` (708) vs `processMatchupPlayersForTeam` (1296) — ~100 lines each of near-verbatim concession/halved/retirement point logic, already drifting in comments. Any league-scoring fix must be applied twice or derby imports silently diverge.
**Fix (M):** unify by passing team context into one implementation (the ForTeam variants are supersets); split fetch/parse/score/match into files.

### 4.2 Per-request template parsing — HIGH
`internal/admin/common.go:46-218` re-reads the page template **and globs + reads + parses all 12 partials on every request**, rebuilding a ~25-function FuncMap — 47 admin call sites plus a parallel copy in `internal/players/templates.go` with a **divergent FuncMap**. On the 1-CPU droplet this is real per-page CPU/disk churn, and template errors surface at request time as **HTTP 200 "coming soon" fallback pages** (`renderFallbackHTML`, `common.go:242-258`) instead of failing at startup. `cmd/jim-dot-tennis/main.go:244-248` already demonstrates the correct parse-once pattern.
**Fix (M):** single `internal/render` package, parse-once cache with dev-mode reparse flag, render to a buffer then write, return honest 500s. Biggest server-side win for the least risk.

### 4.3 Handlers bypassing layers; routing boilerplate — MED
- Raw SQL in handlers: `club_wrapped.go` (53), `points.go` (12), `players.go:878-960` (a `*Service` method defined in a handler file). Handlers also reach into `h.service.playerRepository` etc. directly (e.g. `players.go:1102-1122`), eroding the boundary.
- Hand-rolled routing: 60 `StatusMethodNotAllowed` switch blocks, 56 repeated auth-check blocks, suffix-matching sub-routers (`fixtures.go:37-119`), every route registered twice for trailing slash. Go 1.22+ `ServeMux` (`"GET /admin/fixtures/{id}/edit"`, `r.PathValue`) makes all of it deletable.
- HTML built in Go strings with **unescaped player names**: `renderPlayerGroup` (`fixtures.go:698-727`), `HandlePlayersFilter` (`players.go:582-763`, ~25 `w.Write` calls with inline onclick).
**Fix (M each):** extract Points/Wrapped SQL to services; migrate to ServeMux patterns; convert string-built HTML to template partials.

### 4.4 Duplication and dead code — MED/LOW
- Availability-reminder push flow duplicated (`fixtures.go:1707-1815` vs `players.go:1080-1197`); ~~`getPlayerFantasyToken` byte-identical in both files~~ ✅ **fixed 2026-07-02** — hoisted to a single `(*Service).getPlayerFantasyToken` in `service_fantasy.go`; both handlers call through it. The larger reminder-flow duplication remains. **Fix (S).**
- Team-name parsing has two divergent algorithms (`cmd/populate-db/main.go:438-467` Fields-based vs `repository/club.go:348-364` regex). **Fix (S).**
- `cmd/` sprawl: only 4 of 13 commands are built by the Makefile. Dead/stale: `cmd/scraper` (hardcoded 2025 PDF URLs, superseded), `cmd/collect_tennis_data` + `cmd/import-tennis-players` (one-off fantasy-name scrape), `cmd/test-tennis-pairings` (should be a test), `cmd/populate-db` (914 lines, predates import-season). All compile against `internal/` so every refactor must keep them building. **Fix (S):** delete/archive.
- Legacy `/player-selection` endpoint + its HTML-in-Go renderer (~100 lines) superseded by `/team-selection`. **Fix (S).**
- BHPLTA league URLs hardcoded across 5+ files in two layers (`nonce_extractor.go:36`, `matchcard_service.go:203`, `matchcard_importation.go:166`, cmd files). Centralize in config (S).
- **Verified clean:** no `panic()` anywhere; home-club identity is injected config (no hardcoded club IDs); scoring magic numbers deserve named constants but logic is sound.

---

## 5. Frontend

### 5.1 57% of template lines are inline script/style — HIGH
12,631 of 22,325 template lines are inline `<script>`/`<style>` (3,133 JS + 9,498 CSS). `templates/layout.html` exists but is used by **one page** (login); 47 of 63 templates are standalone documents; the `.admin-header` CSS block is pasted byte-identically into **31 templates**. Already-visible drift: two different `theme-color` values; favicon points at `/static/img/favicon.png` which doesn't exist.
**Fix (L, incremental):** adopt the existing layout block pattern page-by-page; hoist shared CSS to `main.css`/`admin.css`. Each page converted deletes ~200–300 lines. Worst offenders in order: `availability.html` (2,376 inline lines), `fixture_team_selection.html` (1,477), `team_detail.html` (653), `fixtures.html` (605), `players.html` (559). Use `fixture_detail.html`/`wrapped_club.html` as the house pattern — they already do it right.

### 5.2 Verbatim template fork: team selection — HIGH
`fixture_team_selection.html` (1,914 lines) vs `fixture_team_selection_container.html` (595) — the container (the HTMX swap response) is a verbatim copy of the full page's container subtree, with 25 of its 30 CSS selectors re-declared in the full page. Every markup change must be made twice or the page silently differs after the first HTMX swap. The partial infrastructure to fix this already exists (`planning_dashboard.html:372-375` is the working precedent).
**Fix (M):** make the container a Go partial included by the full page and rendered alone for HTMX requests.

### 5.3 `availability.html`: 2,618-line inline SPA — HIGH
The player-facing flagship page: 1,474 lines of inline CSS + 902 of inline JS. Concrete bugs beyond size: `subscribeToPushNotificationsVerbose` (2507-2555) duplicates `static/push.js:35-103` step-for-step and both load on the page — the two paths already diverge on failure; `init()` fetches `/my-availability/{token}/data` **twice** per page load (1755, 1788 — the dedupe guard at 1785 never fires); calendar dates keyed via `toISOString()` (2013) which shifts a day during BST at local midnight.
**Fix (M):** extract to `static/css|js/availability.*` (token via `data-*` attribute — the pattern `fixture_team_selection.html` already uses); delete the Verbose duplicate; fix double-fetch and UTC date handling.

### 5.4 HTMX and service worker reliability — MED
- htmx loaded from unpkg CDN in 30 templates individually; 2 templates self-heal from a *second* CDN. Only one page registers `htmx:responseError` — failed swaps elsewhere do nothing visible. Two state-changing POSTs in team selection are fire-and-forget `fetch()` with no `.catch`/`response.ok` — the optimistic UI proceeds even if the server rejected. **Fix (S–M):** vendor htmx into `static/js/`, one global error toast in the layout, handle the two fetches.
- `static/service-worker.js` has **no fetch handler** — the precache is dead code, zero offline capability despite PWA install; cache name unversioned, no asset cache-busting anywhere. Worse: `pushsubscriptionchange` (153-168) re-subscribes **without the playerToken**, silently orphaning the player's subscription after browser key rotation — notifications just stop for that device. **Fix (S for token; M for a real cache strategy or an honest push-only SW).**

### 5.5 CSS tokens and accessibility — MED
- `main.css` defines design tokens but the primary is scaffold slate `#2c3e50` while the actual brand green `#2c5530` appears as a raw literal 63× (plus `#dc3545` 56×, `#4a7c59` 42×...). Only `main.css` uses `var()`; a rebrand or dark mode is currently a 200-file find-and-replace. **Fix (S to correct tokens; sweep inline styles alongside 5.1).**
- Both core workflows are mouse-only: calendar days are `<div onclick=...>` with no keyboard/AT path (`availability.html:1954-1960`); team selection is drag-and-drop + touch emulation with click handlers on divs. 100 inline `onclick=` across templates; modals lack focus management. **Fix (M):** render cells/cards as `<button>`s; add axe assertions to the existing e2e accessibility suite once fixed.

---

## 6. Testing & tooling

### 6.1 Test coverage gaps — PARTIAL
Zero unit tests in: `internal/admin` (18,913 lines incl. points/scoring), ~~`internal/services/matchcard_*`~~, ~~`internal/auth`~~. **No computed points value is asserted anywhere in the entire test estate** — the e2e points spec only checks page structure. The existing tests are good templates (table-driven, real migration-backed SQLite harness with `findMigrationsPath` ready to reuse).
**✅ Done 2026-07-02:** `player_matcher_test.go` (normalise/levenshtein/similarity + `MatchPlayer` exact/fuzzy-threshold/apostrophe/no-match via an embedded-interface fake repo) and `matchcard_parser_test.go` (team names, division/week, dates, and the concatenated-name BHPLTA case). `internal/auth` also now has `service_test.go` (token redaction + absolute session cap, §2.4). **Still open (M):** extract the points calculation from its handler and golden-test it (the highest-value gap — no computed points value is asserted anywhere); `internal/admin` scoring coverage.

### 6.2 E2E value-level assertions — MED
19 Playwright specs (~2,300 lines) with solid infrastructure (shared auth state, helpers, axe-core, idempotent seed) but the admin specs are mostly "element visible" smoke checks. For the top 3 flows, assert actual values computed from `seed.sql` (e.g. expected points after seeded results). (M)

### 6.3 E2E bootstrap deadlock is undocumented in-repo — MED
Fresh `tennis-data` volume: app fatals on missing club config (`cmd/jim-dot-tennis/main.go:49-52`) → never healthy → the e2e container that would seed clubs never starts. The workaround exists only in out-of-repo notes; zero mentions in `tests/e2e/README.md` or docs. Will block any CI e2e job. **Fix (S):** document it, or better, make `config.Load` degrade gracefully with zero clubs.

### 6.4 Tooling — LOW/MED
- `make lint` builds `golangci-lint@latest` from source in a throwaway container on every run — slow and unpinned (a v2 resolution would break the v1 config cold). Use the official pinned image. (S)
- `.golangci.yml` excludes errcheck for `InvalidateSession` — a dropped error on **logout** is security-relevant. Handle and delete the exclusion; trial `gosec`. (S)
- `DOCKER_GO_CGO` macro relies on an unbalanced-quote hack (`Makefile:139-140`) each call site must close. (S)
- `README.md`/`CLAUDE.md` lead with `make local` / `go run` — both assume a local Go toolchain the project convention says doesn't exist; the documented happy path fails immediately. Reorder Docker-first. (S)

---

## 7. Build, deploy config & docs

### 7.1 Docker build waste on a 1-CPU server — ✅ FIXED 2026-07-02 (cheap fix)
- ~~`Dockerfile:19` uses `go build -a`~~ ✅ **dropped**; added BuildKit cache mounts on `/go/pkg/mod` and `/root/.cache/go-build` for both `go mod download` and the build step, so incremental builds reuse the stdlib/driver compilation.
- ~~`.dockerignore` misses `.go-mod-cache/`, the root binary, `.env`, backups~~ ✅ **rewritten** to exclude `.env`/`.env.*` (secrets), `.go-mod-cache/` (228MB), the root `jim-dot-tennis` binary, `*.db.backup*`, `exported-backups/`, `docs/`, `sprints/`, `.github/`, `.claude/`. **`tests/` is deliberately kept** — `Dockerfile.e2e` copies `tests/e2e/` from the same shared context.
- ~~`Dockerfile.import` builds with Go 1.24.1; final stages use unpinned `alpine:latest`~~ ✅ **aligned to Go 1.25** and both final stages **pinned to `alpine:3.23`** (matches the `golang:1.25-alpine` builder's Alpine release, keeping the CGO binary ABI-compatible). Verified with a clean image build.

### 7.2 Compose duplication and drift — MED
`docker-compose.yml` and `docker-compose.courthive.yml` both define `app` + `backup` with identical container names and already-drifted env (`MY_TENNIS_ENABLED` in one only). The backup service `apk add`s sqlite from the network on every start, sleep-loops, and has no backup-freshness healthcheck. `version: '3.8'` keys are obsolete noise.
**Fix (M):** make the courthive file an override layered on the base (the multiclub-test file already models this correctly); tiny backup image with sqlite baked in + freshness healthcheck.

### 7.3 Docs drift — MED
- `docs/PRODUCTION_QUICK_REFERENCE.md` sections 8/8b and "TMX login issues" still walk through the **net-level/LevelDB** user store — dead since the 2026-05-18 Postgres cutover. Following them on today's prod fails or writes to a dead store.
- RAM figure inconsistent (1GB in two docs vs 2GB actual).
- Root-level `PROFILE_IMPLEMENTATION.md` (superseded by Sprints 016/018) and pre-Postgres `COURTHIVE_SETUP.md` (contains an old real password — see §1.2) should be archived.
- `scripts/` sweep: ~8 overlapping BHPLTA import variants from January, three applied one-off SQL fixes at top level. Archive/delete. (S–M)
- **Verified fine:** sprint hygiene (`sprints/INDEX.md` correctly marks sprint-pwa superseded); `Dockerfile.e2e` playwright pin exactly matches package.json; git-side ignore hygiene is good (no DBs/binaries tracked today; ~5 early commits contain small tennis.db blobs, negligible at 1.08MiB total pack).

---

## Suggested sequencing

**Day 1 — the cheap high-impact sweep (all S):**
rotate the CourtHive admin password · ~~SQLite DSN pragmas~~ ✅ done · test a snapshot restore + confirm cadence · deploy-app.sh sync set + pre-deploy backup + image tag rollback · ~~drop `go build -a` + fix `.dockerignore`~~ ✅ done · ~~`make test` target~~ ✅ done · ~~migrate-down safety + dirty-state fail-fast + 012 down file~~ ✅ done · stop logging session tokens · delete `deploy-with-import.sh`.

**Sprint-sized chunk A — public-surface security (M):**
auth-gate push endpoints (incl. the token-validity oracle) · CSRF for admin POSTs + destructive GET→POST · decide on the fantasy-token PII read-back (§2.1) · optional: per-IP backoff on token lookups.

**Sprint-sized chunk B — correctness (M):**
transactions on season copy/create/activate + result saves (with error propagation) · unify derby matchcard paths · Club Wrapped season filter + error logging · unit tests for parser/matcher/points.

**Sprint-sized chunk C — server & rendering health (M):**
startup template cache + single render package + honest 500s · Go 1.22 ServeMux migration · batch queries for the top-5 N+1 hot paths · de-fork team-selection template · vendor htmx + global error handling · push resubscribe token fix.

**Ongoing campaign (L, incremental):**
per-page layout adoption + inline CSS/JS extraction (priority order in §5.1) · CSS token correction · keyboard accessibility for calendar + team selection · context propagation · docs refresh (Postgres-era quick reference, Docker-first README) · cmd/ and scripts/ cleanup.

---

## What's healthy (keep doing this)

Parameterized SQL everywhere · bcrypt + correct cookie flags + centralized admin auth · consistent repository pattern with real migration-backed tests where they exist · thorough index coverage · clean pointer-based models · good e2e infrastructure (auth state, helpers, axe-core, idempotent seeds) · non-root Docker · multi-stage builds · pinned Playwright versions · sprint file hygiene · no panics, no hardcoded club IDs in Go code.
