# Sprint 005: Club & Away Team Management

## Goal
Enable full management of clubs and away teams (non-St Ann's teams) through the admin interface, including club CRUD, away team lifecycle management, division assignment, and season transition review.

## Context

Currently, the team admin interface is hardcoded to only create and manage St Ann's teams. Other clubs' teams exist in the database (created via CSV import) and are referenced by fixtures, but there is no admin UI to manage them. Clubs can only be listed and edited, not created or deleted. The club detail page shows venue information but not the club's teams.

This sprint makes club and away team management first-class admin features while respecting the principle that we do not store other clubs' players.

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-028 | Club CRUD - add and remove clubs | High | S | None | Yes |
| WI-029 | Club detail page - teams section with navigation | High | M | None | Yes |
| WI-030 | Away team management interface | High | L | WI-028 | No |
| WI-031 | Away team division assignment and active status | Medium | M | WI-030 | No |
| WI-032 | Season transition review for away teams | Medium | M | WI-031 | No |

## Execution Plan

### Phase 1 - Parallel (WI-028, WI-029)
These two items have no dependencies and can be worked on simultaneously:
- **WI-028**: Club create and delete functionality
- **WI-029**: Teams section on club detail page

### Phase 2 - Sequential (WI-030)
Depends on WI-028 (away team creation needs club selection, which needs clubs to exist):
- **WI-030**: Away team management interface (the main body of work)

### Phase 3 - Sequential (WI-031)
Depends on WI-030 (division assignment needs away team UI to exist):
- **WI-031**: Active/inactive status and division reassignment

### Phase 4 - Sequential (WI-032)
Depends on WI-031 (season review uses active status and division assignment):
- **WI-032**: Post-season-copy review workflow for away teams

## Key Design Decisions

- **No model changes for home/away distinction**: The distinction is derived from ClubID — if the team's club is St Ann's, it's a "home" team; otherwise it's an "away" team. No new boolean or enum field needed.
- **Active/inactive status**: New `active` boolean on teams table (WI-031). Inactive teams preserved for historical fixtures but excluded from new fixture creation.
- **Away teams are a subset**: Same Team model, but the UI hides player and captain management sections. Server-side guards prevent adding players/captains to away teams.
- **Season copy is preserved**: The existing CopyFromPreviousSeason functionality is not modified. The review (WI-032) is an additional step after the copy.

## Schema Changes

- **WI-031**: Add `active` boolean column to `teams` table (default `true`). New migration required.

## Phase Alignment
- Phase 6: Admin Tooling and Dashboard (all items)
