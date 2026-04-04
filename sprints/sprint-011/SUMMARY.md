# Sprint 011: Player Lifecycle & Season-Scoped Points

## Goal
Enable safe player removal with full data integrity, fix the points table to be season-scoped, and ensure clean season-to-season transitions.

## Problem Statement
Players leave clubs between seasons (or mid-season). Currently there is no way to remove a player without hard-deleting them, which would destroy historical match data and break the points table. Additionally, the points table has no season filter — when a second season starts, all historical matchups will bleed into the current standings.

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-057 | Add is_active column to players table | High | S | None | Yes |
| WI-058 | Season-scope the points table calculation | Critical | M | None | Yes |
| WI-059 | Deactivate player repository and service layer | High | L | WI-057 | No |
| WI-060 | Filter inactive players from all active queries | High | M | WI-057 | No |
| WI-061 | Update CopyFromPreviousSeason to skip inactive players | High | S | WI-057 | No |
| WI-062 | Admin UI for player deactivation and reactivation | Medium | M | WI-059, WI-060 | No |
| WI-063 | E2E test suite for player lifecycle and season-scoped points | High | L | WI-058 to WI-062 | No |

## Execution Plan

### Phase 1 — Foundation (parallel)
No dependencies, can run simultaneously:
- **WI-057**: Migration to add `is_active` column to players table
- **WI-058**: Fix the points table to filter by active season (critical bug)

### Phase 2 — Backend cascade (sequential, after WI-057)
Each builds on the is_active column:
- **WI-059**: DeactivatePlayer service with full cascade logic and transaction safety
- **WI-060**: Update all player list queries to exclude inactive players by default
- **WI-061**: Update CopyFromPreviousSeason to skip inactive players

### Phase 3 — UI (after WI-059 + WI-060)
- **WI-062**: Admin interface with deactivate/reactivate buttons, confirmation dialog, inactive player toggle

### Phase 4 — Test suite (after all above)
- **WI-063**: Comprehensive E2E tests with multi-season seed data covering all edge cases

## Key Technical Decisions

- **Soft-delete via `is_active` flag**, not hard delete — matches existing pattern on `player_teams` and `captains`
- **Season-scoping the points query** is the most urgent fix and is independent of the player lifecycle work
- **Team Captain guard**: deactivation is blocked if the player is an active Team Captain — admin must reassign first
- **Transaction wrapping**: the entire deactivation cascade is atomic — all or nothing
- **Historical data is sacred**: `matchup_players` and past `fixture_players` are never touched

## Tables Affected by Player Deactivation

| Table | Action | Condition |
|-------|--------|-----------|
| `players` | Set `is_active = FALSE` | Always |
| `player_teams` | Set `is_active = FALSE` | Current season |
| `captains` | Set `is_active = FALSE` | Current season, Day role only |
| `fixtures` | Null `day_captain_id` | Future fixtures |
| `fixture_players` | Delete rows | Future fixtures only |
| `player_general_availability` | Delete | Current season |
| `player_availability_exceptions` | Delete | Future end_date only |
| `player_fixture_availability` | Delete | Future fixtures |
| `player_availability` | Delete | Future fixtures |
| `player_divisions` | Delete | Current season |
| `players.fantasy_match_id` | Set NULL | Always |
| `users` | Deactivate/unlink | If linked |
| `preferred_name_requests` | Delete | Pending status only |

## Tables NEVER Touched

| Table | Reason |
|-------|--------|
| `matchup_players` | Historical match participation — basis for points |
| `fixture_players` (past) | Historical fixture selections |
| `player_teams` (past seasons) | Historical team membership |
| `captains` (past seasons) | Historical captain records |

## Edge Cases Handled

- Player is Team Captain → blocked with error, must reassign
- Player is Day Captain for upcoming fixture → `day_captain_id` nulled
- Player selected for upcoming fixture → removed from `fixture_players`
- Player has user account → deactivated, sessions cleared
- Player has fantasy auth token → `fantasy_match_id` cleared
- Player already inactive → operation is idempotent
- Player rejoins club → reactivate, manually re-add to teams
- Season copy with inactive players → automatically excluded
- Points table with multiple seasons → filtered to active season only
- Division 4 rule → 18-match cap applies per season, not across seasons
