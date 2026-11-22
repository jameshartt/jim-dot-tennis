# CourtHive bundle design

Status: design for Sprint 023 (WI-142). Rewritten on 2026-11-22 after two
decisions by the owner that day: the bundle tracks the latest CourtHive, and
people who only run tournaments get accounts in jim-dot-tennis. Everything
marked "measured" or "proven" was run that day against locally built images;
see "How this was tested".

The bundle lets a club that installs jim-dot-tennis also run its own
tournaments on its own server, switched on with one setting: CourtHive's
tournament server, its identity service, TMX (tournament management) and the
public tournament pages.

## Decisions at a glance

| Question | Decision |
|---|---|
| Which CourtHive versions | Latest upstream at the time of each bundle. First set: tournament server 4.19.1, identity service 0.5.0, TMX 8.61.0, public pages 1.23.7. |
| Licence | All four are MIT, with a licence file at the bundled revisions. |
| One build for every club | Yes. Both front ends take every address as a path on the host that serves them. Proven under two hostnames. |
| Paths | `/tournaments` (TMX), `/public` (public pages), `/api/courthive` (tournament server and identity service) and `/socket.io`. Same for every club. |
| Sign-in | jim-dot-tennis is the only place people sign in. It creates tournament accounts and hands people over to TMX with a single-use token. No change to CourtHive's server-side source. |
| Who may run tournaments | A per-account setting in jim-dot-tennis: none, own tournaments, or all. Administrators always have all. Accounts with no league access ("Tournaments only") exist for people who only run tournaments (WI-153). |
| Minimum server | 1 GB RAM without tournaments, 2 GB with. 20 GB disk either way. |
| Opt-in | A compose profile, off by default. |

## 1. Components and licensing

| Component | Upstream | Bundled as | Our changes |
|---|---|---|---|
| Tournament server | CourtHive/competition-factory-server 4.19.1 | `ghcr.io/jameshartt/courthive-server` | None to the source. Build files only. |
| Identity service | CourtHive/courthive-hiveid 0.5.0 | `ghcr.io/jameshartt/courthive-hiveid` | None. Built from the exact upstream commit; no fork. Build files live in `deploy/images/courthive-hiveid/`. |
| TMX | CourtHive/TMX 8.61.0 | `ghcr.io/jameshartt/courthive-tmx` | Five small source changes (section 2). |
| Public pages | CourtHive/courthive-public 1.23.7 | `ghcr.io/jameshartt/courthive-public` | Five small source changes (section 2). |

All four are MIT, copyright Charles Allen (CourtHive). MIT allows
redistribution of built images; its condition is that the copyright notice and
licence text travel with the copies. Each image carries, under `/licenses`: the
upstream MIT text, a `NOTICE` naming the upstream repository and commit and
saying the image is not an official CourtHive build, and the licences of the
bundled third-party packages (generated at build time).

Upstream's own compose file says there is no official server image. A courtesy
note to CourtHive's author that these images exist is still worth sending; it
is the owner's to send and does not block anything.

Each fork has a branch `jdt-bundle`: the upstream commit, the `link:`
overrides removed with the lockfile regenerated against the npm registry, the
source changes below, and the build files. `BUNDLE.md` in each fork lists what
the branch carries and how to move it to a newer upstream. The older
`jim-tennis-deploy` branches (jim.tennis tooling: imports, plate and score
scripts) are not part of the bundle.

## 2. One build for every club

Paths are fixed; the host is whatever served the page. The jim-dot-tennis app
owns every other path on the origin, so CourtHive services sit under
`/api/courthive`, not at the root as they do on CourtHive's own hosting.

### TMX

- `.env.production`: `SERVER=/api/courthive`, `BASE_URL=tournaments`,
  `PUBLIC_URL=/public`, `AUTH_SERVER=` and `QUERY_SERVER=` empty (both then
  resolve under `SERVER`), `VITE_SCORE_RELAY_URL=disabled`, AMS and
  declarations addresses under `/api/courthive`.
- The socket connects to the origin only. socket.io reads the path of its
  address as a namespace, so `/api/courthive` must not be in it.
- The socket uses long-polling only. Kept from the earlier fix for Caddy on
  HTTP/2. In the test stack a WebSocket did connect through Caddy, so this may
  no longer be needed; it is harmless and can be revisited.
- A `PUBLIC_URL` that starts with `/` is joined to the page's origin, so shared
  links and QR codes carry the club's own address.
- `VITE_EXTERNAL_LOGIN_URL=/tournament-home/open`: TMX's "Log in" goes to
  jim-dot-tennis instead of opening TMX's own form.
- After the single-use token is exchanged, TMX renews its session once. The
  tournament server's hand-over session names the club by id only; the renewed
  session from the identity service carries the club's record, which TMX needs
  to open a tournament from the server and to enable publishing. Found by the
  end-to-end test; worth offering upstream as a server fix.

### Public pages

At the September version these pages contacted `courthive.net` for live
updates on any host that was not localhost, which is what jim.tennis does
today. Latest upstream has more such paths. The bundle closes all of them:

- `index.html` sets `window.dev` before the app starts, pointing the server,
  identity, AMS, declarations and query addresses under
  `<this host>/api/courthive`. Upstream reads these overrides everywhere.
- Both sockets (live updates and player sign-in) connect to the origin only,
  with long-polling.
- `.env.production`: `VITE_SERVER=/api/courthive`,
  `VITE_SCORE_RELAY_URL=/api/courthive/relay`, `BASE_URL=public`.
- Venue cards do not embed an OpenStreetMap frame; they show the court picture.

The `window.dev` script is inline. If a Content-Security-Policy is ever added
at the proxy it must allow that script, or the fallbacks to courthive.net
return.

### What can still reach another host

Nothing on load, in either front end (proven). Later, only through something a
person chooses:

- TMX: picking a non-default screen font in a tournament's settings loads it
  from Google Fonts, on every start-up in that browser from then on. Opening a
  map loads map tiles. Linking a Google Sheet fetches it.
- Public pages: images and links a tournament director typed in (hero image,
  venue image, images in notes) load from wherever they point.

These belong in `docs/outbound_connections.md` (WI-143).

### Features that need services the bundle does not ship

AMS registration, declarations, the query service, rankings, the score relay
and player ("HiveID") sign-in on the public pages. Each fails quietly or with a
plain message; none contacts another host. The public pages ask
`/api/rankings/...` and link to `/epixodic/` at the origin root: the app must
never serve those paths.

### How the front ends are delivered

Each front end is a content image holding only the built files (no source
maps). Its default command empties a target directory and copies the files in.
A one-shot compose service runs it against a named volume that the stock Caddy
image mounts read-only.

## 3. Footprint

Measured, idle and at peak, in the test stacks:

| Part | Idle | Peak seen |
|---|---|---|
| jim-dot-tennis app | 21 MB | not measured under load |
| Caddy | 15 to 63 MB | 75 MB |
| Tournament server | 81 to 162 MB | 190 MB |
| Identity service | 38 to 46 MB | 87 MB |
| Postgres 18 | 39 to 71 MB | 138 MB |
| Redis 7 | 5 to 8 MB | 11 MB |

Images on disk: tournament server 443 MB, identity service 303 MB, Postgres
409 MB, app 108 MB, Caddy 83 MB, Redis 61 MB, TMX 25 MB, public pages 15 MB.
About 1.5 GB in all. A fresh tournament database is about 50 MB.

| | RAM | Disk |
|---|---|---|
| League only | 1 GB | 20 GB |
| League and tournaments | 2 GB, or 1 GB with 2 GB swap | 20 GB |

Compose memory limits for WI-144: tournament server 512 MB, identity service
256 MB, Postgres 384 MB, Redis 64 MB.

Not measured: a real 1 GB or 2 GB machine, and load from a tournament in play.

## 4. Sign-in

### Principle

People sign in to jim-dot-tennis and nowhere else. Trust runs one way: the app
controls the tournament stack; the tournament stack holds nothing that gives
access to the app.

### How CourtHive is arranged

The identity service is the only signer of tokens (ES256). The tournament
server holds the public key only and verifies. They share one database, whose
schema the tournament server's migrations create. The tournament server keeps a
"provisioner" interface for exactly our case: an outside system creates
providers and password-less accounts and hands a person over to TMX with a
single-use token that expires in 60 seconds.

### Secrets, all generated on the club's server at install (WI-144)

| Secret | Held by | Purpose |
|---|---|---|
| ES256 private key | identity service | Signs every token |
| ES256 public key and key id | identity service, tournament server | Verification |
| Signer token | identity service, tournament server | Lets the tournament server ask for tournament-scoped tokens |
| Service account password | app, and used once by the set-up step | The app signs in as the service account |
| Database password | Postgres, tournament server, identity service, app (backups) | |
| `JWT_SECRET` on the tournament server | tournament server only | Must exist for the server to start; unused, because `JWT_ACCEPT_HS256=false` |

The tournament server must run with `JWT_ACCEPT_HS256=false`. Without it, it
accepts tokens signed with `JWT_SECRET`, which would make that value a second
master key (proven both ways). The identity service must not be given a
`JWT_SECRET`: with one, a missing private key makes it sign weaker tokens
silently instead of failing.

### Set-up, once per instance (WI-144 and WI-145), safe to repeat

1. A one-shot service runs upstream's `admin-user.mjs ensure` to make the
   service account `service@jim-dot-tennis.invalid` with the generated
   password.
2. The app signs in to the identity service as the service account and gets a
   token (four hours; the app renews it as needed and holds no key).
3. With it the app creates a provisioner named `jim-dot-tennis`, assigns the
   service account to it, and creates the club's provider owned by it. A
   provider that already exists is adopted, not duplicated.
4. The app creates a permanent, never-used provider administrator
   (`instance`). The tournament server refuses to remove a provider's last
   administrator; this account means any person can always be removed.
5. The app records the provisioner and provider ids and registers the provider
   for calendar sync.

### Per person (WI-152)

When someone with tournament access opens tournament management:

1. First time only: the app creates their tournament account through the
   provisioner interface (password-less, external id = their jim-dot-tennis
   user id, address `<username>@jim-dot-tennis.invalid`) and sets the roles
   `client`, `generate`, `score` on it at the identity service. Without those
   roles the calendar is refused. The app stores the tournament account's id.
2. The app asks the tournament server for a hand-over token and redirects the
   browser to `/tournaments/#/sso?token=...`. TMX exchanges it for its own
   session: a four-hour access token and a rotating refresh token, renewed at
   the identity service.

| jim-dot-tennis tournament access | Tournament role |
|---|---|
| all (administrators always; others when granted) | Provider administrator: every tournament of the club |
| own | Director: tournaments they own or are assigned |
| none | No tournament account, or membership removed |

### Ending access

- Disabling an account, removing its tournament access or revoking its
  sessions in jim-dot-tennis removes the person's membership of the provider
  and their roles. Proven: an existing session then sees no tournaments on its
  next request, because membership is read from the database every time, and
  no new hand-over token is issued for them.
- Signing out of jim-dot-tennis: the sign-in page, which is where signing out
  or an expired session lands, removes TMX's two tokens from that browser's
  storage and calls logout with the refresh token. Both are served from the
  same origin, so this is possible.
- A change the tournament server could not take (it was unreachable) is not
  dropped: the account is marked, the app retries every minute, and the
  tournaments page says how many are waiting. Until it is applied the person
  keeps the tournament access they had.
- Limit: ending someone's sessions in jim-dot-tennis ("sign out everywhere")
  removes their tournament membership at once, but the identity service has no
  way to cancel a tournament session's refresh token from outside. When the
  person next opens tournament management their membership returns, and a
  tournament session still open in another browser works again. To cut someone
  off for good, remove their tournament access or disable the account. Worth
  asking upstream for an administrator's "end this person's sessions" route.

### What the proxy allows

Caddy strips the `/api/courthive` prefix, removes the `Cookie` header on
`/api/courthive/*` and `/socket.io/*` (the jim-dot-tennis session cookie never
reaches CourtHive; this closes security review finding F9), and routes:

| Request | Goes to |
|---|---|
| `/auth/sso/login-with-token`, `/auth/tracker-token`, `/auth/scorer-token`, `/auth/provider-scoring-token` | Tournament server |
| `/auth/refresh`, `/auth/logout`, `/auth/me`, `/auth/me/last-selected-provider`, `/auth/select-provider` | Identity service |
| Every other `/auth/*`, all `/account/*`, `/admin/*`, `/provisioner/*`, `/internal/*`, `/.well-known/*`, `/health*` | Refused (404) |
| Everything else under `/api/courthive` | Tournament server |

So password sign-in, password reset, magic links, user administration, the
provisioner interface and the signer are unreachable from the internet (proven:
each returns 404 through the proxy). The app reaches what it needs over the
internal network. The identity service has no rate limiting of its own, which
is one reason its sign-in route stays closed.

TMX's own user-administration screen will not work in the bundle; accounts are
managed in jim-dot-tennis. TMX signed out still works as a local-only app and
offers "Log in". One small TMX change (`VITE_EXTERNAL_LOGIN_URL`) sends that
action to `/tournament-home/open`, the jim-dot-tennis route that checks the
person's access, brings their tournament account into line and redirects to
TMX with the single-use token in the address's fragment, which browsers never
send to a server.

### People who exist only in CourtHive today

The tournament server cannot attach a hand-over identity to an account that
already exists. On an instance that already has tournament accounts
(jim.tennis), each such person gets a tournament-only account in jim-dot-tennis
and a new tournament account in the same provider. Someone with "own" access
needs their tournaments assigned again. This is a production step for the owner
to approve, and the old accounts stay until then.

### Known limits

- The app and TMX share an origin. A script-injection flaw in either could act
  in the other. This is how jim.tennis already runs. A separate hostname for
  tournaments would remove it at the cost of a second DNS record per club.
- The identity service accepts any role name from an administrator without
  checking it. Only the app acts as one.
- Several CourtHive routes report failure with a success status and an `error`
  field in the body. The app's client always reads the body.

## 5. Upgrades

- A bundled CourtHive set is the four images from one upstream moment, tested
  together. Each jim-dot-tennis release pins exact tags in
  `deploy/docker-compose.yml` and lists the set in a compatibility table in the
  changelog. A club never chooses CourtHive versions.
- Tags: `<upstream version>-jdt.<n>`. Labels record the upstream commit and
  the fork commit.
- Upstream moves fast (hundreds of commits a month, and it restructured
  sign-in in November 2026). Each move to a newer upstream is a piece of work:
  re-create the `jdt-bundle` branches, re-check every address the front ends
  resolve, rebuild, and re-run the two-hostname and sign-on tests. Plan for it
  with each jim-dot-tennis release that changes the set, not continuously.
- A club updates the same way as the app: set the new jim-dot-tennis version,
  pull, restart.

### Data migrations

- The tournament server migrates its own database at start. A fresh install
  applies all 64 migrations unaided (proven).
- From the version jim.tennis runs (3.0.0, migration 049): migrations 050 to
  064 applied unaided on a synthetic database at that schema. No hand-run
  backfill is needed in that range. Three things to know before doing it for
  real: it cannot be undone without the backup (migration 052 removes a column
  the old server reads); migrations 053 and 058 drop `pending_saves` and
  `chat_messages`; and passwords stored in plain text by old versions must be
  hashed with the identity service's script before anyone could sign in with
  one. Stored tournament records also move from engine 7.1.1 to 7.45.0, which
  has not been tested on real data. WI-146 rehearses all of this on a copy of
  jim.tennis's data.
- Before a tournament upgrade the app takes a backup that includes a dump of
  the tournament database (WI-146).

## 6. Effect on the work items

- WI-143: four images on latest upstream, built locally and tested. Remaining:
  outbound connections documented, and publishing once the owner says so.
- WI-144: compose profile with Postgres, Redis, tournament server, identity
  service, the two content services and the set-up step; secrets generated at
  install; the proxy rules of section 4; memory limits of section 3.
- WI-145: set-up as in section 4, using `internal/courthive`.
- WI-153: tournament access per account and tournament-only accounts.
- WI-152: hand-over route, access changes pushed to the tournament stack,
  sign-out, the TMX external sign-in setting.
- WI-146: tournament database in backups; upgrade rehearsal on real data.
- WI-147: end-to-end test of the bundled stack.

## How this was tested

- Each image was built in Docker from its `jdt-bundle` branch (or, for the
  identity service, the exact upstream commit). Upstream's own lint, type
  checks and unit tests pass on the two front-end branches (TMX 3,513 tests,
  public pages 434) and lint and type checks on the server branch.
- A compose project ran Postgres 18, Redis 7, the tournament server, the
  identity service and Caddy with internal TLS, serving `club-a.localhost` and
  `club-b.localhost` from the same volumes, with the proxy rules above and
  secrets generated into files.
- `internal/courthive` has an integration test (build tag `integration`) that
  ran against those services: set-up twice, account creation twice, hand-over
  and its single use, renewal at the identity service, removal with an existing
  session, return, and the last-administrator rule.
- A headless Chromium loaded the public pages and TMX under each hostname,
  signed in to TMX with a hand-over token, and recorded every host contacted
  and every failed request: own host only, no failures. The same browser
  probed sixteen routes through the proxy.
- Sockets were watched for 30 to 40 seconds: stable long-polling, and one
  refused score-relay connection with no retry loop.

- The bundle end-to-end suite (`make test-e2e-bundle`, WI-147) runs the
  shipped compose file and Caddyfile from nothing, tournaments on and off:
  first-run set-up, tournament set-up, hand-over, creating and publishing a
  tournament in TMX, the public pages (own host only), calendar sync, a
  tournaments-only person, removal of access with a tournament session open,
  sign-out, TMX's "Log in", the refused routes, and a fresh browser opening a
  tournament from the server.

Not tested: the upgrade on real data (WI-146 rehearses on synthetic data until
a copy of jim.tennis's tournament database is available); the CI workflows in
the forks and `bundle-smoke.yml`, which have never run because the images are
not published.
