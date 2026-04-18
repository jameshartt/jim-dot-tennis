# Sprint 017: Captain Planning Dashboard — Big-Picture Week Planning

## Overview

**Goal**: Give captains a larger-screen planning surface that unifies availability, 'My Tennis' preferences, and captain-only private notes across the whole club, with week-by-week scrubbing and a draft-lineup flow that hands off to the existing selection workflow.

**Duration**: 2 weeks (dates TBD)

**Status**: Not Started

**Depends on**: Sprint 016 (WI-094 schema, WI-098 preference summary partial)

## Background

The cascading, week-over-week player-picking flow is a known pain point. Captains have no single place to see availability, playing preferences, and private notes together — they cascade through separate screens for each fixture. With **5 teams, 3 captains, a shared team philosophy ('we are a whole team')**, a unified dashboard removes a lot of friction.

**Team philosophy that shapes this sprint**: Any captain can plan for any team. Scope filters are for focus, not isolation. No captain is ever locked out of another team's view.

## Focus Areas

1. Unified 'whole club' planning view with scope filters (My Teams / My Division / Team / All)
2. Week scrubbing across the active season (and past seasons, read-only)
3. Availability × preferences roll-up as the core decision surface
4. Captain-only private notes (where no-nos live — explicitly not on the player profile)
5. Draft lineup → selection hand-off to eliminate the re-keying pain

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies |
|----|-------|----------|------------|--------------|
| WI-101 | user.player_id wiring + 'I am…' self-link | High | S | None (uses Sprint 016 migration 025) |
| WI-102 | Dashboard shell, scope chooser, week scrubber | High | M | WI-101 |
| WI-103 | Week roll-up matrix | High | M | Sprint 016 WI-098, WI-102 |
| WI-104 | Preference filters and chips | High | M | WI-103 |
| WI-105 | Captain-managed private player notes | High | M | Sprint 016 WI-094 |
| WI-106 | Draft lineup + hand-off to selection | High | L | WI-103, WI-104 |
| WI-107 | E2E tests | High | M | All of WI-101 to WI-106 |

## Execution Strategy

### Phase 1: Foundations
- **WI-101** — user-to-player self-link (unlocks 'My Teams' scope)

### Phase 2: Shell + core matrix
- **WI-102** — Dashboard shell, scope chooser, week scrubber
- **WI-103** — Roll-up matrix (needs Sprint 016 WI-098 partial)

### Phase 3: Data-driven layers (some parallel)
- **WI-104** — Preference filters + chips (after WI-103)
- **WI-105** — Captain notes (parallel to WI-104 — independent surface)

### Phase 4: Planning flow
- **WI-106** — Draft lineups + hand-off

### Phase 5: Lock it in
- **WI-107** — E2E regression suite

## Critical Path

```
WI-101 → WI-102 → WI-103 → WI-104 → WI-106 → WI-107
                       └─→ WI-105 ─────────→ WI-107
```

## Key Design Decisions

1. **No new auth surface**: Captains log in via the existing admin session. 'My Teams' is personalisation only, never gating. Any captain can view and plan for any team — mirrors the team philosophy.

2. **user.player_id is for personalisation**: Defaults the scope filter to 'My Teams' and lights up captaincy badges, but never restricts what a user can see or do.

3. **Whole-club and multi-team dashboards are the same page**: The tension dissolves into a scope chooser. No separate entry points.

4. **Desktop/tablet/landscape-phone first**: The dashboard needs real estate. Narrow portrait mobile gets a friendly 'rotate or open on a bigger screen' nudge with a 'view anyway' escape. Primary touchpoint is desktop at planning time; mobile availability editing remains on the availability page.

5. **Drafts are non-binding**: Saving a draft does not write to fixture_players. The hand-off to selection_overview pre-populates the form — the selection page remains the single source of truth for committed lineups.

6. **Captain notes are isolated by convention AND by test**: No package under internal/players/* imports captain_note_repository; an E2E regression test seeds a note and asserts it is absent from every token-accessed page.

## Scope Semantics

- **My Teams** — all teams where user.player_id maps to an active TeamCaptain or DayCaptain for the active season
- **My Division** — division(s) containing 'My Teams'
- **Specific Team** — single-team filter (any team)
- **All Teams** — every active team at the home club

## Data Sources

- `fixtures` + `matchups` — weekly fixture schedule
- `player_availability` — availability bitmap per player per fixture
- `player_tennis_preferences` + `player_preferred_partners` — from Sprint 016
- `captain_player_notes` — new in WI-105 (schema from Sprint 016 WI-094)
- `lineup_drafts` + `lineup_draft_players` — new in WI-106 (migration 026)

## Success Metrics

- [ ] Captain can open one page and scrub through any week, any scope
- [ ] Availability + preferences visible in one matrix, no tab-switching
- [ ] Draft → selection hand-off replaces cascaded re-keying
- [ ] Private notes visible to captains; zero leakage to player-facing URLs (CI-enforced)
- [ ] 'My Teams' defaults right when a captain logs in

## Work Items Tracking

### Completed
- None

### In Progress
- None

### Not Started
- WI-101 through WI-107
