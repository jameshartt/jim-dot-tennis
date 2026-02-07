# Sprint 006: Match Day - Results, Standings & Captain Tools

## Goal
Complete the match day lifecycle from captain selection overview through match result recording, player-facing league standings, and fixture enrichment with weather and history.

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-033 | Captain selection overview dashboard | High | M | None | Yes |
| WI-034 | Match result entry UX for captains | High | L | None | Yes |
| WI-035 | Player-facing league standings / points table | Medium | M | None | Yes |
| WI-036 | Match history and player statistics view | Medium | M | WI-034 | No |
| WI-037 | Fixture weather information | Low | S | None | Yes |

## Execution Plan

### Phase 1 - Parallel (WI-033, WI-034, WI-035, WI-037)
These four items have no dependencies and can be worked on simultaneously:
- **WI-033**: Captain selection overview dashboard (carried from WI-008)
- **WI-034**: Match result entry UX
- **WI-035**: Player-facing standings
- **WI-037**: Weather information widget

### Phase 2 - Sequential (WI-036)
Depends on WI-034 (needs match result data to display history):
- **WI-036**: Match history and player statistics

## Key Technical Decisions

- **Weather API**: Open-Meteo recommended (free, no API key, lat/lng support). Venue geocoding data already exists from Sprint 002.
- **Standings data**: Reuse existing admin points table logic for player-facing view.
- **Selection overview**: Builds on existing selection handler from Sprint 001 partial implementation.
- **Match results**: Uses existing matchup table score fields - no schema changes expected.

## Phase Alignment
- Phase 2: Captain Selection Tools (WI-033)
- Phase 5: Fixture and Venue Management (WI-034, WI-035, WI-036, WI-037)
