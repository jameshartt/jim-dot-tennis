# Account Roles: Captain and Administrator

Every account has one of two roles. This page says what each can do. It is
written so the user guide can reuse it as it stands.

Players do not have accounts. Each player uses their own private link to give
their availability and look after their profile.

## The two roles

**Captain** runs the club's teams. A captain can see and work with **every**
team, not just their own. The club works as one squad, so nothing is hidden
from a captain. The "My Teams" filter on the planning dashboard is only a
shortcut to your own teams.

**Administrator** can do everything a captain can, and also runs the site for
the club: accounts, settings, seasons and imports.

## What each role can do

| Area | Captain | Administrator |
|------|:-------:|:-------------:|
| Dashboard (the page you land on after signing in) | Yes | Yes |
| Captain planning dashboard, availability matrix, "My Teams" | Yes | Yes |
| Linking your own account to your player record ("I am...") | Yes | Yes |
| Player availability and reminders to give availability | Yes | Yes |
| Team selection, matchups, day captains | Yes | Yes |
| Fixtures, rescheduling, fixture notes, weather, calendar files | Yes | Yes |
| Result entry and match card import from BHPLTA | Yes | Yes |
| Push notifications: "notify selected players" and "remind availability" | Yes | Yes |
| Players: add, edit, deactivate, reactivate, availability links | Yes | Yes |
| Preferred name requests from players | Yes | Yes |
| Points table, standings and selection overview | Yes | Yes |
| Captain notes | Yes | Yes |
| Teams: view every team, add players to a squad | Yes | Yes |
| Other clubs and their teams: view | Yes | Yes |
| Teams: create a team, assign or remove team captains, mark a team active or inactive | No | Yes |
| Other clubs and their teams: create, edit, delete | No | Yes |
| Users: create accounts, invite and recovery links, change roles, link an account to a player | No | Yes |
| Sessions: see who is signed in, sign people out | No | Yes |
| Account audit log | No | Yes |
| Club settings and feature switches (My Tennis, Tournaments, Club Wrapped) | No | Yes |
| BHPLTA settings and club data import | No | Yes |
| Seasons: create, copy, set up, set active, delete, start from BHPLTA | No | Yes |
| Divisions and the division review | No | Yes |
| Tournaments (CourtHive) | No | Yes |
| Season Wrapped (the club's season review page) | No | Yes |
| Demo data: load and clear | No | Yes |
| Passkey security warnings: clear a blocked passkey | No | Yes |
| Push notifications: test message to every subscriber, reset the push keys | No | Yes |
| Backups: see when the last one was made, download one (asks for your password or a passkey) | No | Yes |

Everyone with an account can manage their own sign-in methods (passkeys and
password) on the sign-in methods page. Changing them asks you to confirm it's
you first, with your current password or a passkey you already have, and signs
you out on your other devices. An administrator's passkey must check who is
holding it (a PIN, fingerprint, face or screen lock).

If a captain opens a page that is only for administrators, they see a page
saying so, with a link back to the dashboard. Links to those pages are not
shown to captains.

## Giving someone a role

An administrator chooses the role when creating the account on the users
page, and can change it there at any time. On the server the same is done with
`jim-dot-tennis user create <username> --role captain` and
`jim-dot-tennis user role <username> admin`. There must always be at least one
active administrator who can actually sign in (an account whose invite was
never accepted does not count).

On the users page an administrator can also link an account to the person's
player record ("Linked player"). A captain can do this for themselves from the
planning dashboard. With the link in place the planning dashboard marks the
teams that person captains with a star and offers a "My Teams" shortcut.

## Accounts with the old "player" role

Earlier versions offered a "player" role for accounts. It is no longer used.
An account that still has it can sign in and manage its own sign-in methods,
but it cannot open any club page until an administrator makes it a captain or
an administrator. The users page shows such an account as "player (no access)".

## For developers

The roles are enforced on the server, whatever the navigation shows:

- `/admin/league/*`: `adminRouteRoles` in `internal/admin/roles.go` declares
  the role for every route pattern, including mixed patterns where some
  actions need an administrator. Registering a route without an entry stops
  the server starting and fails `TestEveryAdminRouteHasADeclaredRole`. The
  same table drives navigation through the template function `canUse`.
- Push endpoints (`/api/push/test`, `/api/vapid-reset`): administrator, via
  the wrapper passed to `webpush.Service.SetupHandlers` in
  `cmd/jim-dot-tennis/main.go`.
- `/account/passkeys/{id}/clear-warning`: administrator
  (`internal/auth/passkey_handlers.go`); the other `/account/*` pages need any
  signed-in account.
- `/setup/demo` and `/setup/demo/clear`: administrator (`internal/setup`).
