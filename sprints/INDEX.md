# Sprint Index

This file provides a quick reference to all sprints and their status.

**Status sources**: Each `sprint-XXX/sprint.json` carries its own `status` field (`completed` / `planned`). This index is derived from those files — if an entry here disagrees with the source, the `sprint.json` is canonical.

## Active Sprints

None currently active.

## Completed Sprints

Listed newest first.

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

### Sprint 017: Captain Planning Dashboard — Big-Picture Week Planning
- **Status**: Planned
- **Work Items**: 7 (WI-101 to WI-107)
- **Focus**: Unified captain dashboard with week scrubbing, availability × preferences roll-up, private captain notes, and draft lineup → selection hand-off
- **Key Features**: `/admin/league/planning` dashboard; scope chooser (My Teams / My Division / Team / All); week scrubber (season-aware, past-season toggle); availability matrix with preference chips; captain-managed private notes ('no-nos' live here, never on player profile); non-binding draft lineups that promote into the existing selection_overview flow
- **Directory**: `sprint-017/`
- **Summary**: [sprint-017/SUMMARY.md](sprint-017/SUMMARY.md)
- **Dependencies**: depends on Sprint 016 (WI-094 schema delivering migration 025 user.player_id, WI-098 preference summary partial); internal: WI-102 → WI-101; WI-103 → WI-102+WI-098; WI-104/105/106 → WI-103; WI-107 → all

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
- **WI-101 through WI-107** — Sprint 017 (planned)
- **Next available**: WI-108

## Migrations Relationship

Sprint work items that deliver migrations document the migration number in their `technical_notes`:
- **024** (player_tennis_preferences, player_preferred_partners, captain_player_notes) — Sprint 016 WI-094
- **025** (users.player_id FK) — Sprint 016 WI-094 (used by Sprint 017)
- **026** (lineup_drafts, lineup_draft_players) — Sprint 017 WI-106
- **Next available**: 027

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
