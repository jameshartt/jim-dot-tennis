# Sprint 003: Admin Polish: Dashboard, Divisions, Users & Service Refactor

## Overview

**Goal**: Reorganise the admin dashboard for better usability, add division and user/session management, and break down the monolithic service.go into maintainable domain files

**Duration**: Short sprint (Feb 1-3, 2026)

**Status**: Completed (closed 2026-02-01)

## Focus Areas

1. Dashboard layout and quick actions reorganisation
2. Division management and editing
3. User management admin page
4. Session management admin page
5. Admin service.go refactor into domain files

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies | Status |
|----|-------|----------|------------|--------------|--------|
| WI-017 | Reorganise admin dashboard layout and quick actions | High | M | - | Completed |
| WI-018 | Add division editing with play_day correction | High | M | - | Completed |
| WI-019 | Implement user management admin page | High | M | - | Completed |
| WI-020 | Implement session management admin page | Medium | M | - | Completed |
| WI-021 | Refactor admin service.go into domain-specific files | High | L | WI-019, WI-020 | Completed |

## Work Item Details

### WI-017: Reorganise admin dashboard layout and quick actions

The admin dashboard has accumulated 11 quick action buttons in a flat list, making it hard to scan. This work item restructures the dashboard:

- **Keep**: All 5 stat cards with counts at the top (Players, Fixtures, Teams, Clubs, Preferred Name Requests)
- **Reorganise**: Group the 11 quick actions into logical categories:
  - **League Management**: Players, Teams, Fixtures, Clubs
  - **Results & Standings**: Match Card Import, Points Table
  - **Season Tools**: Season Wrapped, Preferred Name Approvals, Club Data Import
  - **System**: Users, Sessions
- **Demote**: Login attempts table moved from prominent side-by-side placement to a collapsible/secondary section
- **Clean up**: Remove debug JavaScript from the template

**Files affected**: `templates/admin_standalone.html`

### WI-018: Add division editing with play_day correction

There is currently no way to edit division properties through the admin UI. The `play_day` field is known to be incorrect on some divisions and can only be fixed via direct database access. This adds:

- Division edit form accessible from the season setup page
- Editable fields: name, level, play_day (dropdown), max_teams_per_club
- New handler and template for division editing
- Integration with existing season setup flow

**Files affected**: New `internal/admin/divisions.go`, new `templates/admin/division_edit.html`, plus modifications to `handler.go`, `service.go`, and `season_setup.html`

### WI-019: Implement user management admin page

The `/admin/league/users` page currently shows a "coming soon" placeholder. This implements the full user management UI:

- List all users with username, role, active status, last login, linked player
- Create new users with username, password, and role selection
- Toggle user active/inactive status
- Change user roles
- Reset user passwords
- Link/unlink users to player records
- Self-deactivation prevention

**Files affected**: New `templates/admin/users.html`, modifications to `internal/admin/users.go`, `internal/admin/service.go`, `internal/auth/service.go`

### WI-020: Implement session management admin page

The `/admin/league/sessions` page currently shows a "coming soon" placeholder. This implements session visibility and management:

- List all active sessions with user, IP, user agent, device info, timestamps
- Group/filter sessions by user
- Invalidate individual sessions or all sessions for a user
- Trigger expired session cleanup
- Current session visually marked
- Login attempts history relocated here from dashboard (ties into WI-017)

**Files affected**: New `templates/admin/sessions.html`, modifications to `internal/admin/sessions.go`, `internal/admin/service.go`, `internal/auth/service.go`

### WI-021: Refactor admin service.go into domain-specific files

`internal/admin/service.go` has grown to 3757 lines with ~90 methods. It cannot be easily read or navigated. This is a pure refactor to extract methods into domain-specific files while keeping everything in the `admin` package:

- `service.go` → Service struct, constructor, shared types (~300 lines)
- `service_dashboard.go` → Dashboard methods
- `service_players.go` → Player CRUD and filtering
- `service_fixtures.go` → Fixture queries and mutations
- `service_teams.go` → Team management and captains
- `service_matchups.go` → Matchup CRUD and player assignment
- `service_fixture_players.go` → Fixture-player selection and availability
- `service_seasons.go` → Season, week, and setup operations
- `service_divisions.go` → Division queries
- `service_clubs.go` → Club queries and St Anns helpers
- `service_fantasy.go` → Fantasy doubles operations
- `service_selection.go` → Selection overview operations

Depends on WI-019 and WI-020 completing first so the user/session stubs are replaced before extraction.

**Files affected**: `internal/admin/service.go` split into ~12 files

## Technical Impact

### New Files (~5)
- Division admin handler (`internal/admin/divisions.go`)
- Division edit template (`templates/admin/division_edit.html`)
- User management template (`templates/admin/users.html`)
- Session management template (`templates/admin/sessions.html`)
- ~11 extracted service files (`internal/admin/service_*.go`)

### Modified Files (~6)
- Dashboard template (`templates/admin_standalone.html`) - layout overhaul
- Admin handler registration (`internal/admin/handler.go`) - new routes
- Admin service (`internal/admin/service.go`) - new methods, then refactored down
- Auth service (`internal/auth/service.go`) - new user/session list methods
- Admin users handler (`internal/admin/users.go`) - full implementation
- Admin sessions handler (`internal/admin/sessions.go`) - full implementation
- Season setup template (`templates/admin/season_setup.html`) - division edit links

### New Routes
- `GET /admin/league/divisions/{id}/edit` - division edit form
- `POST /admin/league/divisions/{id}/edit` - process division edit
- `POST /admin/league/users` - create new user
- `POST /admin/league/users/{id}/role` - update role
- `POST /admin/league/users/{id}/toggle-active` - toggle active status
- `POST /admin/league/users/{id}/reset-password` - reset password
- `POST /admin/league/sessions/{id}/invalidate` - invalidate session
- `POST /admin/league/sessions/invalidate-user/{userID}` - invalidate all user sessions
- `POST /admin/league/sessions/cleanup` - cleanup expired sessions

### Database Changes
None - all work items use existing schema and repositories.

## Outcomes

### WI-017: Dashboard Overhaul
- Quick actions reorganised into 4 logical categories: League Management, Results & Standings, Season Tools, System
- Added Manage Seasons and Selection Overview links alongside the original 11
- Login attempts demoted to collapsible `<details>` section (collapsed by default)
- All debug JavaScript removed
- Stat cards with counts preserved at the top

### WI-018: Division Editing
- New `DivisionsHandler` with GET/POST edit routes
- Edit form with play_day dropdown (Monday-Sunday), name, level, max_teams_per_club
- Edit button added to each division heading on the season setup page
- Redirects back to season setup after save

### WI-019: User Management
- Full CRUD UI replacing "coming soon" placeholder
- User list with inline role dropdown (auto-submits on change)
- Create user form, toggle active/inactive, password reset modal
- Self-deactivation prevention
- New auth service methods: ListUsers, GetUserByID, UpdateUserRole, ToggleUserActive, ResetUserPassword

### WI-020: Session Management
- Active sessions table with current session highlighted (green row + badge)
- Revoke individual session / revoke all for a user
- Cleanup expired sessions button
- Recent login attempts table (relocated from dashboard)
- New auth service methods: ListActiveSessions, ListAllLoginAttempts

### WI-021: Service Refactor
- `service.go` reduced from 3871 lines to 117 lines (Service struct, NewService, shared helpers)
- 12 domain-specific files extracted:
  - `service_dashboard.go` (172 lines)
  - `service_players.go` (214 lines)
  - `service_fixtures.go` (941 lines)
  - `service_fixture_players.go` (503 lines)
  - `service_teams.go` (377 lines)
  - `service_matchups.go` (514 lines)
  - `service_seasons.go` (392 lines)
  - `service_divisions.go` (19 lines)
  - `service_clubs.go` (19 lines)
  - `service_fantasy.go` (236 lines)
  - `service_users_sessions.go` (133 lines)
  - `service_selection.go` (337 lines)
- No method signatures changed, no interface changes, all callers unaffected
- Build verified with `make build`

## Retrospective

**What went well:**
- All 5 work items completed in a single session
- Service refactor was clean - same package means no interface changes needed
- Existing repository layer and auth service provided solid foundations for user/session management

**Lessons learned:**
- The service.go refactor should have been done earlier; 3800+ lines is far past the point where a single file is manageable
- Having the WI-019/WI-020 dependency on WI-021 was correct - the stubs needed replacing before extraction made sense
