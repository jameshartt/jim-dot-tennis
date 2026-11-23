# Sprint Index

This file provides a quick reference to all sprints and their status.

**Status sources**: Each `sprint-XXX/sprint.json` carries its own `status` field (`completed` / `planned`). This index is derived from those files — if an entry here disagrees with the source, the `sprint.json` is canonical.

## Active Sprints

None currently active.

## Completed Sprints

Listed newest first.

### Sprint 018: My Tennis — Tiered Wizard & Enthusiastic Consent
- **Status**: Completed (2026-04-25)
- **Work Items**: 4 (WI-108 to WI-111) — all completed
- **Goal**: Reshape the Sprint 016 My Tennis form into a stepped wizard of six priority-ordered tiers, ending each tier with equal-weight "finish here" / "keep going" CTAs that encourage stopping
- **Key Features**: Six-tier ordering (team basics → when/where → how you play → partners & pressure → goals → fun); canonical tier definitions in `internal/players/my_tennis_tiers.go`; single integer `wizard_progress_tier` on `player_tennis_preferences` (migration 027) with cascading backfill; monotonic progress (never decrements); re-edit affordance (`?edit=N`) always opens blank forms; localStorage drafts keyed per-token+per-tier; write-only privacy contract preserved end-to-end (31 my_tennis e2e tests pass)
- **Directory**: `sprint-018/`
- **Summary**: [sprint-018/SUMMARY.md](sprint-018/SUMMARY.md)

### Sprint 017: Captain Planning Dashboard — Big-Picture Week Planning
- **Status**: Completed
- **Work Items**: 7 (WI-101 to WI-107) — all completed
- **Focus**: Unified captain dashboard with week scrubbing, availability × preferences roll-up, private captain notes, and draft lineup → selection hand-off
- **Key Features**: `/admin/league/planning` dashboard; scope chooser (My Teams / My Division / Team / All); week scrubber (season-aware, past-season toggle); availability matrix with preference chips; captain-managed private notes ('no-nos' live here, never on player profile); non-binding draft lineups that promote into the existing selection_overview flow
- **Directory**: `sprint-017/`
- **Summary**: [sprint-017/SUMMARY.md](sprint-017/SUMMARY.md)
- **Schema delivered**: migration 026 (lineup_drafts, lineup_draft_players)

### Sprint 015: PWA Push Notifications
- **Status**: Completed
- **Work Items**: 5 (WI-088 to WI-092) — all completed
- **Goal**: Captain-triggered push notifications for team selection and availability reminders, with player opt-in on the availability screen
- **Key Features**: Player-linked push subscriptions, captain "Notify Selected Players" button, captain "Remind Availability" button, cross-platform support (iOS, Android, desktop)
- **Supersedes**: sprint-pwa (carried items from sprint-001, never started)
- **Directory**: `sprint-015/`
- **Summary**: [sprint-015/SUMMARY.md](sprint-015/SUMMARY.md)

### Sprint 014: Club-Agnostic Refactor
- **Status**: Completed
- **Work Items**: 11 (WI-077 to WI-087) — all completed
- **Goal**: Remove all hardcoded St Ann's assumptions; make the app deployable for any parks league club via a single `HOME_CLUB_ID` environment variable
- **Key Features**: Home club config/middleware, centralised apostrophe normalisation, service method + DTO renaming, elimination of `FindByNameLike('St Ann')` pattern, SQL query parameterisation, template genericisation, BHPLTA config, full documentation overhaul, E2E test parameterisation, multi-club verification suite, full regression validation
- **Directory**: `sprint-014/`
- **Summary**: [sprint-014/SUMMARY.md](sprint-014/SUMMARY.md)

### Sprint 013: Attribution, Licensing & Open Source Readiness
- **Status**: Completed
- **Work Items**: 6 (WI-071 to WI-076) — all completed
- **Goal**: Open-source licensing, contributor attribution, GitHub configuration, and documentation for other clubs
- **Key Features**: MIT license, GitHub `.github/` templates, README overhaul with attribution, CONTRIBUTING.md with club adaptation guide, in-app `/about` credits page (James Hartt profile + CourtHive/Charles Allen attribution), source file headers
- **Directory**: `sprint-013/`
- **Summary**: [sprint-013/SUMMARY.md](sprint-013/SUMMARY.md)

### Sprint 012: CourtHive Tournament Integration
- **Status**: Completed
- **Work Items**: 7 (WI-064 to WI-070) — all completed
- **Goal**: Integrate CourtHive tournament data into jim.tennis admin and public pages
- **Key Features**: Tournament provider CRUD, tournament sync from CourtHive calendar API, visibility toggle, dynamic index page with tournament links, TMX admin deep links
- **Directory**: `sprint-012/`
- **Summary**: [sprint-012/SUMMARY.md](sprint-012/SUMMARY.md)

### Sprint 011: Player Lifecycle & Season-Scoped Points
- **Status**: Completed
- **Work Items**: 7 (WI-057 to WI-063) — all completed
- **Goal**: Safe player removal, season-scoped points table, clean season transitions
- **Key Features**: Player soft-delete/deactivation, season-scoped points query fix, `CopyFromPreviousSeason` skip inactive, admin deactivation UI, comprehensive E2E tests
- **Directory**: `sprint-011/`
- **Summary**: [sprint-011/SUMMARY.md](sprint-011/SUMMARY.md)

### Sprint 010: Advanced E2E & Accessibility
- **Status**: Completed (closed 2026-02-28)
- **Work Items**: 3 (WI-054 to WI-056) — all completed
- **Goal**: Complex workflows, accessibility auditing, and test reporting
- **Key Features**: Team selection/match result workflow tests, axe-core a11y testing, responsive viewport tests, Claude-parseable test output
- **Directory**: `sprint-010/`
- **Summary**: [sprint-010/SUMMARY.md](sprint-010/SUMMARY.md)

### Sprint 009: Core E2E Test Suite
- **Status**: Completed (closed 2026-02-28)
- **Work Items**: 5 (WI-049 to WI-053) — all completed
- **Goal**: Comprehensive browser tests for all critical user flows
- **Key Features**: Auth flow tests, admin dashboard/navigation, admin CRUD pages, player-facing pages, points table/standings
- **Directory**: `sprint-009/`
- **Summary**: [sprint-009/SUMMARY.md](sprint-009/SUMMARY.md)

### Sprint 008: E2E Test Infrastructure
- **Status**: Completed
- **Work Items**: 6 (WI-043 to WI-048) — all completed
- **Goal**: Playwright browser testing infrastructure with Docker Compose
- **Key Features**: Playwright scaffolding, Docker test profile, database seeding, test helpers, Makefile targets, smoke tests
- **Directory**: `sprint-008/`
- **Summary**: [sprint-008/SUMMARY.md](sprint-008/SUMMARY.md)

### Sprint 007: Communication & Project Maintenance
- **Status**: Completed
- **Work Items**: 4 (WI-038 to WI-041) — all completed
- **Goal**: Expand communication channels and keep project infrastructure current
- **Key Features**: Email notification infrastructure, notification preferences UI, CLAUDE.md update, season transition tooling
- **Directory**: `sprint-007/`
- **Summary**: [sprint-007/SUMMARY.md](sprint-007/SUMMARY.md)

### Sprint 006: Match Day — Results, Standings & Captain Tools
- **Status**: Completed (closed 2026-02-07)
- **Work Items**: 5 (WI-033 to WI-037) — all completed
- **Goal**: Complete the match day lifecycle from captain selection overview through match result recording, player-facing league standings, and fixture enrichment with weather and history
- **Directory**: `sprint-006/`
- **Summary**: [sprint-006/SUMMARY.md](sprint-006/SUMMARY.md)

### Sprint 005: Club & Away Team Management
- **Status**: Completed (closed 2026-02-07)
- **Work Items**: 6 (WI-028 to WI-032, WI-042) — all completed
- **Goal**: Enable full management of clubs and away teams through the admin interface
- **Directory**: `sprint-005/`
- **Summary**: [sprint-005/SUMMARY.md](sprint-005/SUMMARY.md)

### Sprint 004: Spring Clean — Go Tooling, Dead Code, Linting & Environment Update
- **Status**: Completed (closed 2026-02-01)
- **Work Items**: 6 (WI-022 to WI-027) — all completed
- **Goal**: Update Go/Docker versions, add tooling targets, remove dead code, fix linting, standardise formatting, clean dependencies
- **Directory**: `sprint-004/`
- **Summary**: [sprint-004/SUMMARY.md](sprint-004/SUMMARY.md)

### Sprint 003: Admin Polish — Dashboard, Divisions, Users & Service Refactor
- **Status**: Completed (closed 2026-02-01)
- **Work Items**: 5 (WI-017 to WI-021) — all completed
- **Goal**: Reorganise dashboard, add division editing, implement user/session management, refactor service.go
- **Directory**: `sprint-003/`
- **Summary**: [sprint-003/SUMMARY.md](sprint-003/SUMMARY.md)

### Sprint 002: Venue Infrastructure & Season Filtering
- **Status**: Completed (closed 2026-02-01)
- **Work Items**: 4 (WI-013 to WI-016) — all completed
- **Goal**: Add venue/club infrastructure, fix season filtering, improve player fixture UX
- **Directory**: `sprint-002/`
- **Summary**: [sprint-002/SUMMARY.md](sprint-002/SUMMARY.md)

### Sprint 001: Player Experience & Notifications MVP
- **Status**: Completed (closed 2026-02-01)
- **Work Items**: 12 (5 completed, 1 deferred, 6 carried forward — the carried push/PWA work ultimately shipped in Sprint 015)
- **Goal**: Complete core player availability features and implement push notification system
- **Directory**: `sprint-001/`
- **Summary**: [sprint-001/SUMMARY.md](sprint-001/SUMMARY.md)

## Planned Sprints

Sprints 019-024 form the **club-readiness programme**: making the app installable and usable by other BHPLTA clubs with no prior familiarity. Agreed constraints: BHPLTA only; each club hosts its own instance; the original author has no access to other clubs' instances; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in. Each work item carries a `recommended_model`. Suggested order: 019 → 020 → 021 → 022 → 023 → 024 (022 may run alongside 020/021, except its CI e2e job wants 020's fresh-install suite).

### Sprint 019: Public Repo Cleanup & SQLite-Only
- **Status**: Completed (2026-11-21)
- **Work Items**: 4 (WI-112 to WI-115)
- **Goal**: No live credentials or author-server details in tracked files; the real reference deployment in git; AGPL scraper isolated; Postgres path removed
- **Directory**: `sprint-019/`
- **Summary**: [sprint-019/SUMMARY.md](sprint-019/SUMMARY.md)

### Sprint 020: Fresh Install — From Empty Database to a Working Club
- **Status**: Completed (2026-11-21)
- **Work Items**: 10 (WI-116 to WI-125)
- **Goal**: Start on an empty database, guided setup with a one-time code, first administrator, club choice, league/divisions/season pulled from BHPLTA, club settings, no seeded accounts or shared secrets
- **Directory**: `sprint-020/`
- **Summary**: [sprint-020/SUMMARY.md](sprint-020/SUMMARY.md)
- **Schema**: 030 (app_settings), 031 (drop of the migration-010 managing-team trigger)

### Sprint 021: Authentication — Passkeys, Recovery and a Real Captain Role
- **Status**: Completed (2026-11-21)
- **Work Items**: 9 (WI-126 to WI-134)
- **Goal**: Passkeys as default sign-in, passwords as optional fallback, invite and recovery links without email, server-side account commands, enforced captain role, hardening and an independent security review
- **Directory**: `sprint-021/`
- **Summary**: [sprint-021/SUMMARY.md](sprint-021/SUMMARY.md)
- **Schema**: 032 (must_change_password, retired), 033 (WebAuthn credentials and user handles), 034 (account links, nullable password hash), 035 (hashed sessions, account audit log)

### Sprint 022: Packaging, CI and Releases
- **Status**: Completed 2026-11-22 (release 0.1.0 published; jim.tennis runs the released image)
- **Work Items**: 7 (WI-135 to WI-141)
- **Goal**: Self-contained binary (embedded assets, pure-Go SQLite), CI on every push, tagged releases with binaries and a multi-architecture image, safe upgrades, tested backup and restore, jim.tennis moved onto releases
- **Directory**: `sprint-022/`
- **Summary**: [sprint-022/SUMMARY.md](sprint-022/SUMMARY.md)

### Sprint 023: CourtHive Bundle — Club Championships in the Box
- **Status**: Implemented and tested locally 2026-11-22; publishing and the real-data upgrade rehearsal wait for the owner
- **Work Items**: 9 (WI-142 to WI-147, WI-152, WI-153, WI-154)
- **Goal**: Published club-neutral CourtHive images, an opt-in tournament stack in the packaged deployment, automatic per-club provisioning, single sign-on with jim-dot-tennis as the identity source, tournament data in backups, unattended upgrades
- **Directory**: `sprint-023/`
- **Summary**: [sprint-023/SUMMARY.md](sprint-023/SUMMARY.md)
- **Gate**: WI-142 must confirm redistribution is permitted before any image is published

### Sprint 024: Documentation and Onboarding for New Clubs
- **Status**: Planned
- **Work Items**: 4 (WI-148 to WI-151)
- **Goal**: Operator and user guides, stale docs archived, a cold install by a reader with only the docs, then version 1.0.0
- **Directory**: `sprint-024/`
- **Summary**: [sprint-024/SUMMARY.md](sprint-024/SUMMARY.md)

### Sprint 016: My Tennis — Profile Revamp & Player Self-Expression
- **Status**: Completed (2026-04-18)
- **Work Items**: 8 (WI-093 to WI-100)
- **Focus**: Revamp `/my-profile` into a playful 'My Tennis' experience; harden privacy on the open-internet token URL via initials-only rendering and a write-only submission flow
- **Key Features**: Initials-only on all token-accessed profile views; sectioned/playful/mobile-first 'My Tennis' form; write-only submission with merge semantics and session-only confirmation (no read-back of stored state); rich optional taxonomy (identity/match types/playing style/partnership/intensity/logistics/health/fun/comms); reusable admin-side preference summary partial for Sprint 017 consumption; availability-page CTA linking to profile
- **Directory**: `sprint-016/`
- **Summary**: [sprint-016/SUMMARY.md](sprint-016/SUMMARY.md)
- **Dependencies**: WI-095/097 → WI-094; WI-098 → WI-094; WI-099 → WI-098; WI-096 → WI-095; WI-100 → WI-093+WI-095+WI-096+WI-097+WI-099
- **Schema delivered**: migrations 024 (tennis prefs + partner prefs + captain notes) and 025 (users.player_id FK) — 025 is a Sprint 017 prerequisite delivered here

## Superseded Sprints

### Sprint PWA: Push Notifications & PWA Enhancement
- **Status**: Superseded — never started, replaced by Sprint 015 with updated scope
- **Work Items**: 6 (carried from sprint-001)
- **Directory**: `sprint-pwa/`
- **Summary**: [sprint-pwa/SUMMARY.md](sprint-pwa/SUMMARY.md)

## Sprint Metrics

All completed sprints. Granular `completed_items` accounting in sprint.json may lag actual delivery for sprints flipped to completed in bulk — the `status` field is the canonical signal.

| Sprint | Work Items | Completed | Success Rate |
|--------|-----------|-----------|--------------|
| 015 | 5 | 5 | 100% |
| 014 | 11 | 11 | 100% |
| 013 | 6 | 6 | 100% |
| 012 | 7 | 7 | 100% |
| 011 | 7 | 7 | 100% |
| 010 | 3 | 3 | 100% |
| 009 | 5 | 5 | 100% |
| 008 | 6 | 6 | 100% |
| 007 | 4 | 4 | 100% |
| 006 | 5 | 5 | 100% |
| 005 | 6 | 6 | 100% |
| 004 | 6 | 6 | 100% |
| 003 | 5 | 5 | 100% |
| 002 | 4 | 4 | 100% |
| 001 | 12 | 5 | 42% (6 carried forward → shipped in Sprint 015) |

## Work Item Numbering

Work items are numbered sequentially:
- **WI-001 through WI-092** — sprints 001 through 015
- **WI-093 through WI-100** — Sprint 016 (completed 2026-04-18)
- **WI-101 through WI-107** — Sprint 017 (completed)
- **WI-108 through WI-111** — Sprint 018 (completed)
- **WI-112 through WI-151** — Sprints 019–024, club-readiness programme
- **WI-152** — Sprint 023, single sign-on (added 2026-11-21)
- **WI-153** — Sprint 023, tournament-only accounts (added 2026-11-22)
- **WI-154** — Sprint 023, several sets of tournaments per installation (added 2026-11-23)
- **Next available**: WI-155

## Migrations Relationship

Sprint work items that deliver migrations document the migration number in their `technical_notes`:
- **024** (player_tennis_preferences, player_preferred_partners, captain_player_notes) — Sprint 016 WI-094
- **025** (users.player_id FK) — Sprint 016 WI-094 (used by Sprint 017)
- **026** (lineup_drafts, lineup_draft_players) — Sprint 017 WI-106
- **027** (player_tennis_preferences.wizard_progress_tier) — Sprint 018 WI-108
- **028** (retired_by on matchups) and **029** (reschedule statuses) — delivered outside the sprint structure
- **030** (app_settings) and **031** (drop managing-team trigger) — Sprint 020 WI-116, WI-120
- **032** to **035** (must-change flag, WebAuthn credentials, account links and nullable password hash, hashed sessions and account audit log) — Sprint 021 WI-127, WI-128, WI-129, WI-131
- **036** to **038** (tournament access per account, tournament accounts, tournament provider ids and per-person provider picks) — Sprint 023 WI-153, WI-152, WI-154
- **Next available**: 039 (026 was never committed — never reuse it)

## How to Use This Index

1. **Review active sprints** to understand current focus
2. **Check sprint summaries** for detailed breakdown
3. **Use sprint tools** in `tools/` directory to execute work items
4. **Update this index and the corresponding `sprint.json`** together whenever a sprint's status changes — this index is derived from sprint.json and drifts otherwise

## Sprint Execution Commands

### View Sprint Details
```bash
# Using bash script
./tools/run-sprint.sh sprint-001

# Using Python script
python3 ./tools/spawn-agents.py sprint-001 --dry-run
```

### Execute Work Items
```bash
# Execute specific work item
python3 ./tools/spawn-agents.py sprint-001 --item WI-001 --dry-run

# Execute full sprint (dry run)
python3 ./tools/spawn-agents.py sprint-001 --dry-run

# Execute full sprint with parallel agents
python3 ./tools/spawn-agents.py sprint-001 --parallel
```

## Work Item Status Tracking

Work items can have the following statuses:
- **Not Started**: Default state
- **In Progress**: Agent/developer actively working
- **Blocked**: Waiting on dependencies
- **In Review**: Implementation complete, awaiting review
- **Completed**: All acceptance criteria met

To track status, you can:
1. Add a `status` field to work item JSON files
2. Use a separate tracking file (e.g., `sprint-001/status.json`)
3. Use project management tools integrated with the work items

## Adding New Sprints

1. Create a new directory: `sprint-XXX/`
2. Create `sprint.json` with metadata (include `"status": "planned"`)
3. Create work item files: `WI-XXX.json`
4. Create `SUMMARY.md` for overview
5. Update this INDEX.md file

See [README.md](README.md) for detailed format and structure.
