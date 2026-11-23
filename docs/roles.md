# Account Roles: Captain, Administrator and Tournaments Only

There are three kinds of account: captain, administrator and Tournaments
only. This page says what each can do. The user guides draw on it.

Players do not have accounts. Each player uses their own private link to give
their availability and look after their profile.

## The kinds of account

**Captain** runs the club's teams. A captain can see and work with **every**
team, not just their own. The club works as one squad, so nothing is hidden
from a captain. The "My Teams" filter on the planning dashboard is only a
shortcut to your own teams.

**Administrator** can do everything a captain can, and also runs the site for
the club: accounts, settings, seasons and imports.

**Tournaments only** is for people who run the club's tournaments and have
nothing to do with the league. They sign in the same way as everyone else
and land on their tournament home page, which has one button: open
tournament management. They cannot open any league page.

## Tournament access

Every account also has a tournament access setting:

- **No tournaments**: no tournament management.
- **Own tournaments**: the tournaments they run themselves, and any they
  have been given.
- **All tournaments**: every tournament the club runs.

Administrators always have all tournaments, whatever the setting says. A
captain has no tournaments unless an administrator gives them access. A
Tournaments only account always has own or all: its access cannot be taken
away. To stop someone using tournaments, disable their account instead. A
disabled account has no tournament access.

### Clubs with more than one set of tournaments

A club can keep separate sets of tournaments, for example one for the club's
own tournaments and one for a league cup. Most clubs have one and can ignore
this.

With more than one, the users page shows tick boxes under "Can manage
tournaments for" beside each person who has tournament access. The own or
all setting then applies in each set that is ticked.

- Administrators always have every set.
- Someone with nothing chosen has the club's own set only. A set added later
  is given to nobody until an administrator ticks it.
- A person with more than one set is asked "Which tournaments do you want to
  manage?" on their tournament home page, by name. To manage the other one
  they come back to that page, or switch inside tournament management by
  selecting the short code shown at the top of its list of tournaments.

An administrator adds a set, and changes the name people see for it, in the
"Tournament set-up" panel on the tournaments page.

Captains and administrators with tournament access see "Open Tournament
Management" on the dashboard. It goes to the same tournament home page.
When Tournaments is switched off for the club (Club Settings, Features), the
tournament home page says so instead of showing the button.

## What each role can do

The table covers the league. A Tournaments only account has none of it: it
has its tournament home page and its own sign-in methods.

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
| Users: create accounts, invite and recovery links, change roles and tournament access, link an account to a player | No | Yes |
| Sessions: see who is signed in, sign people out | No | Yes |
| Account audit log | No | Yes |
| Club settings and feature switches (My Tennis, Tournaments, Club Wrapped) | No | Yes |
| BHPLTA settings and club data import | No | Yes |
| Seasons: create, copy, set up, set active, delete, start from BHPLTA | No | Yes |
| Divisions and the division review | No | Yes |
| Tournaments (CourtHive): the club's tournament list and sets of tournaments | No | Yes |
| Tournament set-up: setting the club up on the tournament server, and running it again | No | Yes |
| Tournament management | If given access | Yes, always all |
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
shown to captains. If a Tournaments only account opens a league page, it sees
a page saying the league is not part of its account, with a link to its
tournament home.

## Giving someone a role

An administrator chooses the role when creating the account on the users
page, and can change it there at any time. On the server the same is done with
`jim-dot-tennis user create <username> --role captain` and
`jim-dot-tennis user role <username> admin`.

To make a Tournaments only account, choose "Tournaments only" as the role on
the users page and choose own or all tournaments, or run
`jim-dot-tennis user create <username> --role tournaments --tournaments own`.
A captain can be given tournament access when created
(`--role captain --tournaments all`). Tournament access is changed in the
"Tournaments" column of the users page, or with
`jim-dot-tennis user tournaments <username> none|own|all`. Every change is
written to the account audit log. A Tournaments only account can later be
made a captain or an administrator from the users page; a captain cannot be
turned into a Tournaments only account (disable it and make a new one). There must always be at least one
active administrator who can actually sign in (an account whose invite was
never accepted does not count).

On the users page an administrator can also link an account to the person's
player record ("Linked player"). A captain can do this for themselves from the
planning dashboard. With the link in place the planning dashboard marks the
teams that person captains with a star and offers a "My Teams" shortcut.

## Accounts with the old "player" role

Earlier versions offered a "player" role for accounts. Players no longer have
accounts. In the database, Tournaments only accounts reuse the stored role
value "player", which grants nothing in the league. An old player account has
no tournament access, so it can sign in and manage its own sign-in methods,
but it cannot open any club page until an administrator makes it a captain or
an administrator, or gives it tournament access (which makes it a
Tournaments only account). The users page shows such an account as
"player (no access)".

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
- `/tournament-home` (`internal/tournamenthome`): any signed-in account whose
  effective tournament access is not none; anyone else gets 403. Its button
  target is `tournamenthome.ManagementPath`.
- Tournament access is stored in `users.tournament_access` (migration 036:
  `none`, `own` or `all`, default `none`, checked by the database). What an
  account can use is `models.EffectiveTournamentAccess` (administrators all,
  disabled accounts none, others the stored value); every caller uses it.
  Changes go through `accounts.Service` (`InviteAccount`,
  `SetTournamentAccess`), which refuses an administrator's and taking a
  Tournaments only account's access away, records each change as
  `user.tournament_access`, and tells its `TournamentAccessObserver` after
  every committed change to tournament access, role or active state, and
  after sessions are ended by an administrator or the command line.
- After sign-in (password, passkey or link) an account with no league role
  and tournament access lands on `/tournament-home` (`auth.HomePathFor`);
  everyone else lands on the league dashboard as before.
