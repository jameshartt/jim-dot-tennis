# CourtHive bundle design

Status: design for Sprint 023 (WI-142), written 2026-11-22. Everything marked
"measured" or "proven" below was run that day against images built from the
forks; the commands are summarised in "How this was tested" at the end.

The bundle lets a club that installs jim-dot-tennis also run its own
tournaments: the CourtHive tournament server, TMX (tournament management) and
the public tournament pages, on the club's own server, switched on with one
setting.

## Decisions at a glance

| Question | Decision |
|---|---|
| Which CourtHive versions | The versions jim.tennis runs today: the `jim-tennis-deploy` branches, based on upstream as of 2026-09-24 (server 3.0.0 with factory 7.1.1, TMX 8.28.1, public pages 1.16.0). Not current upstream; see "Upgrades". |
| Licence | All three are MIT. One point needs the author's confirmation before the public-pages image is published; see "Licensing". |
| One build for every club | Yes. Both front ends take their server address as a path on the host that serves them. Proven under two hostnames. |
| Paths | `/tournaments` (TMX), `/public` (public pages), `/api/courthive` and `/socket.io` (server). Same for every club. |
| Sign-in | jim-dot-tennis is the only place people sign in. It creates tournament accounts and hands people over to TMX with a one-time token. No change to the server's source is needed. |
| Minimum server | 1 GB RAM without tournaments, 2 GB with. 20 GB disk either way. |
| Opt-in | A compose profile, off by default. |

## 1. Licensing

| Component | Licence at the bundled revision | Licence upstream today |
|---|---|---|
| competition-factory-server | `"license": "MIT"` in package.json, README says MIT; no LICENSE file | MIT, LICENSE file added 2026-11-10 |
| TMX | MIT, LICENSE.txt present | MIT |
| courthive-public | none declared; README says "Private — not published to npm" | MIT, LICENSE file added 2026-11-10 ("license the viewer under MIT, matching TMX and the factory") |
| tods-competition-factory, courthive-components (libraries inside the builds) | MIT | MIT |

Copyright holder in every licence file: Charles Allen (CourtHive).

MIT allows redistribution of built images from a fork. Its one condition is
that the copyright notice and licence text travel with the copies.

What each image must contain (WI-143):

- `/licenses/LICENSE-<component>`: the upstream MIT text for that component.
- `/licenses/NOTICE`: one paragraph saying the image is built from a fork of
  the named CourtHive repository at the named upstream commit, with the fork's
  URL, and that it is not an official CourtHive build.
- Third-party licences for bundled npm packages, generated at build time.

Open point for the owner. The public pages were unlicensed at the revision we
bundle and were put under MIT seven weeks later. The licence file upstream
covers the revisions it ships with. Two ways to close this, either is enough:

1. The author confirms in writing that MIT covers the September 2026 revision
   of courthive-public (a one-line reply). This is also the courtesy contact
   the programme asked for.
2. The public pages are moved forward to an upstream revision that contains
   the LICENSE file (2026-11-10 or later) and re-tested against the bundled
   server.

Until one of these is done, images are built and tested locally and nothing is
published. If the author refuses, the fallback in WI-142 applies: document how
a club points jim-dot-tennis at a CourtHive it installed itself.

## 2. Configuration: one build for every club

### What was wrong

- TMX compiles `SERVER` into the build. The production build carried
  `https://jim.tennis/api/courthive`.
- The public pages compile `VITE_SERVER` in for ordinary requests, but live
  updates, player sign-in, registrations and declarations ignore it and fall
  back to `https://courthive.net` on any host that is not localhost. On
  jim.tennis today, a visitor's browser therefore opens a connection to
  courthive.net when it views a tournament page. The club-neutral build
  removes this.

### The change

Paths are fixed and the host is whatever served the page.

TMX (3 files changed, 13 lines):

- `.env.production`: `SERVER=/api/courthive`, `PUBLIC_URL=/public`,
  `BASE_URL=tournaments`, `VITE_ASSISTANT_URL=` (empty),
  `QUERY_SERVER=/api/courthive/query`.
- `src/platform/web.ts`: a `SERVER` that starts with `/` is joined to
  `window.location.origin`.
- `src/services/publishing/publicUrl.ts`: the same for `PUBLIC_URL`, so shared
  links and QR codes carry the club's own address.

Public pages (3 files changed, 26 lines):

- `index.html`: a short script sets `window.dev` before the app starts, with
  `baseURL`, `amsURL`, `declarationsURL` and `queryBaseURL` all under
  `<this host>/api/courthive`. Upstream already reads these overrides in every
  service, so no fallback to courthive.net is reached.
- `src/services/liveUpdates.ts`: the socket connects to the origin only (the
  namespace must not carry the `/api/courthive` prefix) and uses long-polling
  (the WebSocket upgrade fails behind Caddy on HTTP/2, as already found for
  TMX).
- `.env.production`: `VITE_SERVER=/api/courthive`,
  `VITE_SCORE_RELAY_URL=/api/courthive/relay`, `BASE_URL=public`.

The changes are on the local branch `bundle/club-neutral` in each front-end
fork, on top of `jim-tennis-deploy`.

### Proof

One build of each, served by one Caddy under `club-a.localhost` and
`club-b.localhost` over HTTPS:

- Public pages: load on both; every request goes to the page's own host.
- TMX: signs in (with the one-time token from section 4) on both, loads the
  calendar, connects its socket; every request goes to the page's own host.
- Built files that are served contain no `jim.tennis`, no server address and
  no account name. `courthive.net` remains only as dead fallback strings in
  two public-pages files. Source maps still mention `jim.tennis` in comments;
  WI-143 leaves source maps out of the images, which also cuts their size from
  33 MB to 11 MB (TMX) and 18 MB to 5 MB (public pages).

TMX loads fonts from Google Fonts in one code path (PDF fonts). It was not
requested during the test. WI-143 checks when it is used and lists it in
`docs/outbound_connections.md` if it stays.

### How the front ends are delivered

Each front end is a small content image holding only the built files. A
one-shot service copies them into a named volume that the stock Caddy image
mounts read-only. This keeps Caddy unmodified and adds no running container.
The copy step empties the volume first so an upgrade leaves no stale files.

## 3. Footprint

Measured on the test stack (idle, then after a burst of 150 requests and a
full sign-on run). The app figure is from the long-running local container.

| Part | Idle | Peak seen |
|---|---|---|
| jim-dot-tennis app | 21 MB | not measured under load |
| Caddy | 15 to 63 MB | 75 MB |
| Tournament server (Node) | 87 to 130 MB | 224 MB at first start on an empty database |
| Postgres 18 | 71 MB | 138 MB |
| Redis 7 | 5 to 8 MB | 11 MB |

The tournament server started and served correctly with a 256 MB memory limit
(peak 121 MB once the database existed).

Disk for images: tournament server 1.58 GB as built today, Postgres 409 MB,
app 108 MB, Caddy 83 MB, Redis 61 MB, front ends about 80 MB. Data is small: a
fresh tournament database is 51 MB.

The server image is large because it installs development dependencies (one
runtime package, `compression`, is listed as a development dependency
upstream). WI-143 should try a production-only install plus that package and
report the saving; it is not a blocker.

Minimum specification:

| | RAM | Disk |
|---|---|---|
| League only | 1 GB (512 MB works, with no headroom for upgrades) | 20 GB |
| League and tournaments | 2 GB, or 1 GB with 2 GB swap | 20 GB |

Compose memory limits for WI-144: tournament server 512 MB, Postgres 384 MB,
Redis 64 MB. jim.tennis itself runs the full stack on 2 GB.

## 4. Sign-in

### Principle

People sign in to jim-dot-tennis and nowhere else. Trust runs one way: the app
controls the tournament server; the tournament server holds nothing that gives
access to the app.

### What the tournament server already provides

The bundled server has a "provisioner" interface built for exactly this: an
outside system creates providers and password-less users, and hands a user
over to TMX with a single-use token that expires in 60 seconds. TMX already
has the receiving route (`/tournaments/#/sso?token=...`). Nothing in the
server's source needs to change.

### Mechanism

One secret: the server's signing secret (`COURTHIVE_JWT_SECRET`), generated on
the club's server at install (WI-144) and given to the app as well as to the
tournament server. Both run on the club's own machine and already share its
`.env`.

Set-up, once per instance (WI-145), safe to repeat:

1. A one-shot service runs upstream's own `admin-user.mjs create` to make the
   instance's service account, with a random password that is thrown away.
   WI-143 adds that script to the server image (one line in our Dockerfile).
2. The app signs 60-second tokens as the service account, with the secret, and
   uses them to create a provisioner named `jim-dot-tennis`, then the club's
   provider owned by that provisioner. Tokens that also carry the provisioner
   claim let the app use the provisioner interface directly, so no API key has
   to be stored anywhere.
3. The app records the provisioner id and provider id in its own database and
   registers the provider for calendar sync.

Per person (WI-152):

1. When someone entitled to run tournaments first opens tournament management,
   the app creates their tournament account: `POST /provisioner/users` with
   their jim-dot-tennis user id as the external id, no password, then
   `POST /auth/modify` to give the roles `client`, `generate`, `score`.
   Without those roles the calendar is refused with 403 (upstream creates
   these users with no roles).
2. The app asks for a one-time token (`POST /auth/sso/generate`) and redirects
   the browser to `/tournaments/#/sso?token=...`. TMX exchanges it for its own
   session.

Role mapping, declared in the app's role table:

| jim-dot-tennis role | Tournament role | Default |
|---|---|---|
| Administrator | Provider administrator (all of the club's tournaments) | On |
| Captain | Director (own and assigned tournaments only) | Off; the owner decides |
| Player | none | n/a |

Tournament accounts use the address `<username>@jim-dot-tennis.invalid`. The
server needs an email-shaped key; this one can never receive mail, and the
server's outgoing mail is not configured in the bundle.

### Ending access

- Disabling, demoting or revoking someone in jim-dot-tennis removes their
  provider membership and their roles on the tournament server. Proven: the
  change takes effect on the next request, even with a token issued earlier,
  because the server reads membership from its database on every request.
- A permanent, never-used tournament account (`instance`) is kept as provider
  administrator, because the server refuses to remove the last one.
- Signing out of jim-dot-tennis: the sign-out page also removes TMX's two
  tokens from browser storage and calls the server's logout with the refresh
  token. This works because both are served from the same origin.
- TMX's session is the server's own: a 4-hour access token and a rotating
  refresh token. A removed person's refresh yields a token with no membership.

### Closing the server's own doors

Caddy changes on the tournament routes (WI-152, with the F9 fix):

- Remove the `Cookie` header from requests to `/api/courthive/*` and
  `/socket.io/*`, so the jim-dot-tennis session cookie never reaches the
  tournament server (security review finding F9).
- Refuse from the internet: `/api/courthive/admin/*`,
  `/api/courthive/provisioner/*`, `/api/courthive/auth/sso/generate`,
  `/api/courthive/auth/login`, `/auth/forgot-password`,
  `/auth/reset-password`, `/auth/magic/*`, `/auth/complete-first-login`,
  `/auth/admin-create-user`, `/auth/admin-reset-password`,
  `/auth/change-password`, `/auth/modify`, `/auth/remove`, `/auth/allusers`.
  The app reaches these over the internal network, not through Caddy.
- Left open: `/auth/sso/login-with-token`, `/auth/refresh`, `/auth/logout`,
  `/auth/me`.

Password sign-in is also dead by construction: every person's tournament
account has no password, and the server refuses password sign-in for such
accounts (proven: 401).

TMX still shows its own sign-in form to someone who arrives signed out. WI-152
makes the smallest TMX change that sends them to jim-dot-tennis instead.

### What was proven

Against the unmodified server image: create the service account, provisioner,
provider and users; set roles; one-time token issued, exchanged once, refused
on replay; TMX signed in by that route under both hostnames; membership
removal effective immediately; tokens signed with another secret, expired
tokens, unsigned tokens and anonymous requests all refused with 401.

Note for implementers: the provisioner interface reports some failures as
HTTP 201 with an `error` and `code` in the body (`ABBREVIATION_EXISTS`,
`SSO_IDENTITY_EXISTS`, `EMAIL_EXISTS`). Check the body, and treat these three
as "already there" when repeating set-up.

### Known limits

- The app and TMX share an origin. A script-injection flaw in either could act
  in the other. This is how jim.tennis already runs. A separate hostname for
  tournaments would remove it at the cost of a second DNS record per club; not
  in this sprint.
- Later upstream versions remove login from the server altogether in favour of
  a separate identity service, keeping the hand-over route, key discovery and
  token verification. When the bundle moves to such a version, jim-dot-tennis
  takes that service's place: it signs tokens with its own ES256 key and the
  server holds only the public key. The verification side of this already
  exists in the bundled server (`JWT_PUBLIC_KEY`, `JWT_KID`).

## 5. Upgrades

### Which upstream versions

Upstream moves quickly: since the forks' base on 2026-09-24 the server has
gained 357 commits (3.0.0 to 4.19.1), TMX 631 and the public pages 246, and the
server's login was removed on 2026-11-05. The bundle does not follow that
pace.

- A bundled CourtHive set is one tested trio: server, TMX and public pages
  from the same upstream date, plus our small patches.
- The set moves forward deliberately, out of season, at most about twice a
  year, or sooner for a security fix that affects a club.
- Every move is rehearsed on a copy of jim.tennis's tournament data before it
  is released, and jim.tennis runs it first.
- The forks' patches stay as small as possible and are listed in each fork's
  maintenance notes. Changes useful to everyone are offered upstream: honour
  the configured server address everywhere in the public pages; give
  provisioner-created users the `client` role.

### Versions and tags

Images: `ghcr.io/jameshartt/courthive-server`, `ghcr.io/jameshartt/courthive-tmx`,
`ghcr.io/jameshartt/courthive-public`. Tag: `<upstream version>-jdt.<n>`, for
example `3.0.0-jdt.1`. Image labels record the upstream commit and the fork
commit. Each jim-dot-tennis release pins exact tags in `deploy/docker-compose.yml`
and lists the pairing in a compatibility table in the changelog; a club never
chooses CourtHive versions itself.

### How a club gets an update

Same as the app: set the new jim-dot-tennis version, pull, restart. The
compose file of that release names the CourtHive images that go with it.

### Data migrations

- The server migrates its own database at start. A fresh install on the
  bundled version runs every migration on an empty database without help
  (proven). The hand-run calendar backfill between migrations 047 and 048 only
  mattered for existing data crossing that point; the first bundled version is
  already past it.
- For later versions, the server image starts through our own entrypoint. It
  runs any step that must happen between two migrations, in order, then starts
  the server. A step that fails stops the start and says which step failed;
  the previous image still runs against the untouched backup.
- Before the app starts a tournament upgrade it takes a backup that includes a
  dump of the tournament database (WI-146). The app image gains the Postgres
  client tools for this; the dump and restore run over the internal network
  with the generated database password.

## 6. Effect on the other work items

- WI-143: build from the pinned base, not current upstream. Three images as
  described; no source maps; licence files; `admin-user.mjs` in the server
  image; nothing published until the licence point is closed.
- WI-144: the app also receives `COURTHIVE_JWT_SECRET` when tournaments are
  on; one-shot services for the front-end files and the service account;
  memory limits above; Caddy routes only when the profile is on.
- WI-145: set-up as in section 4; no credential is shown to anyone, because no
  person has one.
- WI-146: Postgres client tools in the app image; entrypoint with ordered
  steps in the server image.
- WI-152: no server source change; Caddy rules above; sign-out clears TMX's
  tokens; one small TMX change for the signed-out case.
- WI-147: unchanged in intent; sign-in to TMX is by the hand-over, not a form.

## How this was tested

- Front ends built once each in `node:24-alpine` with the changes in section
  2, as content images.
- Server image built from the fork's existing Dockerfile, unmodified source.
- A separate compose project (Postgres 18, Redis 7, server, Caddy with
  internal TLS on a local port) served both hostnames from the same volumes.
- A headless Chromium loaded the public pages and TMX under each hostname and
  recorded every host contacted and every failed request.
- The sign-on chain was driven with scripted HTTP calls through Caddy.
- Memory was read from `docker stats` and each container's cgroup peak, with
  the server also restarted under 384 MB and 256 MB limits.

Not tested here: creating a tournament through TMX's form and seeing it on the
public pages (left to WI-147), and behaviour on a real 1 GB or 2 GB machine
rather than limits on a larger one.
