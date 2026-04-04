# Sprint 012: CourtHive Tournament Integration

## Goal
Integrate CourtHive tournament data into jim.tennis with a provider/tournament CRUD admin interface, CourtHive sync, and a dynamic public-facing tournament listing on the index page.

## Problem Statement
CourtHive manages tournaments (via TMX and competition-factory-server), but jim.tennis has no visibility into or control over which tournaments are shown to visitors. The index page has a static link to `/public` with no ability to highlight specific tournaments. There are two CourtHive providers (St Ann's and Parks League Cup) whose tournaments need to be managed independently. Admins need to sync tournament data from CourtHive and control which tournaments are publicly listed, without manually copying UUIDs.

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-064 | Database migration: tournament_providers and tournaments tables | Critical | S | None | Yes |
| WI-065 | Tournament provider and tournament repositories | High | M | WI-064 | No |
| WI-066 | Tournament service layer with CourtHive sync | Critical | L | WI-065 | No |
| WI-067 | Admin UI: tournament providers CRUD | High | M | WI-066 | No |
| WI-068 | Admin UI: tournament list, sync, and visibility management | Critical | L | WI-066, WI-067 | No |
| WI-069 | Dynamic index page with visible tournaments and CourtHive links | High | M | WI-065 | No |
| WI-070 | E2E tests for tournament management | High | L | WI-067, WI-068, WI-069 | No |

## Execution Plan

### Phase 1 — Foundation
- **WI-064**: Migration creating `tournament_providers` and `tournaments` tables, plus model structs

### Phase 2 — Data Access
- **WI-065**: Repository interfaces and SQLite implementations for both entities

### Phase 3 — Business Logic (after Phase 2)
- **WI-066**: Service layer with provider/tournament CRUD and the `SyncFromCourtHive` method that calls `POST /api/courthive/provider/calendar`

### Phase 4 — Admin UI (after Phase 3, can partially parallel with Phase 5)
- **WI-067**: Provider management pages (list, add, edit, delete)
- **WI-068**: Tournament management page (list by provider, sync button, visibility toggle, TMX deep links)

### Phase 5 — Public Index (after Phase 2, parallel with Phase 4)
- **WI-069**: Dynamic index page showing visible tournaments, Tournament Admin link, renamed login buttons

### Phase 6 — Tests (after all above)
- **WI-070**: E2E tests for provider CRUD, tournament CRUD, visibility, and index page rendering

## Key Technical Decisions

- **CourtHive calendar API** (`POST /provider/calendar`) is the sync source — no direct DB access needed, no changes to competition-factory-server
- **Two providers** tracked independently — St Ann's and Parks League Cup each have their own abbreviation and tournament list
- **Tournaments hidden by default** — sync pulls in all tournaments but only admin-toggled ones appear on the public index
- **No auto-sync** — manual "Sync from CourtHive" button per provider. Scheduled sync can be added later if needed
- **Public page links** use CourtHive's hash-based routing: `/public/#/tournament/{courthive_tournament_id}`
- **TMX deep links** from admin: `/tournaments/#/tournament/{courthive_tournament_id}` for quick access to tournament management
- **Status derived from dates** — Upcoming / In Progress / Completed, computed at render time, not stored
- **COURTHIVE_API_URL** env var for the CourtHive server address (default `http://courthive-server:8383` for Docker networking)

## CourtHive Calendar API Shape

Request:
```json
POST /provider/calendar
{"providerAbbr": "STANN"}
```

Response:
```json
{
  "success": true,
  "calendar": {
    "provider": { ... },
    "tournaments": [
      {
        "tournamentId": "uuid-string",
        "tournament": {
          "tournamentName": "St Ann's Summer 2026",
          "startDate": "2026-06-01",
          "endDate": "2026-06-15"
        }
      }
    ]
  }
}
```

## Index Page Changes

| Element | Before | After |
|---------|--------|-------|
| Header login button | "Login" → `/login` | "League Login" → `/login` |
| Main card | Static "View Public Tournaments" → `/public` | Dynamic list of visible tournaments, each → `/public/#/tournament/{id}` |
| Tournament Admin | (none) | New card linking to `/tournaments` (CourtHive TMX) |
| Footer admin link | "Admin? Login here" → `/admin/league` | "League Admin? Login here" → `/admin/league` |
