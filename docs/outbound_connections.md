# Instance isolation: accounts, secrets and outbound connections

Each club runs its own instance and owns it outright. This page is the
reference for what that means in practice: a new instance has no accounts and
no shared secrets, and it sends nothing to the software's author. It lists
every connection an instance (or a browser showing its pages) makes.

The list is enforced by a test: `internal/outbound/outbound_test.go` scans the
Go code, templates, static files, migrations and deployment files for URLs and
fails on any host not in its allowlist. Adding a new outbound host means
updating that allowlist and this page together.

## No accounts, no shared secrets

- **Accounts.** A database created from scratch has no rows in any table: no
  users, sessions, tokens or keys (`internal/secrets/isolation_test.go`). The
  first administrator is created through first-run setup (`/setup`), protected
  by a one-time code printed to the server log. Databases created by versions
  before Sprint 020 may still hold the administrator accounts that migration
  004 used to insert; at every start the app logs a warning naming any account
  whose password is one published in the repository (an old seeded account, or
  the end-to-end test administrator). Change or remove those on the admin users
  page. Upgrading never changes or removes an account.
- **Secrets.** Nothing has a built-in default:

  | Secret | Where it comes from | When nothing is set |
  |---|---|---|
  | Session cookies | 256-bit random token per sign-in (crypto/rand), stored in `sessions` | n/a: there is no signing key |
  | Setup code / setup ticket | Random, printed once to the log; only a hash is stored | n/a |
  | Club Wrapped access code | `WRAPPED_ACCESS_PASSWORD`, else `app_settings.wrapped_access_code` | Generated with crypto/rand on first start (60 bits, `XXXX-XXXX-XXXX`) and shown to administrators on the Club Settings page. The "unlocked" cookie is derived from the code, so it is different on every instance. |
  | Web push (VAPID) key pair | `vapid_keys` table | Generated on first start with crypto/rand (P-256) |
  | BHPLTA club code | Club Settings page, or `BHPLTA_CLUB_CODE` | Match-card import is unavailable until the club enters its own code (it is BHPLTA's credential for the club, not the app's) |
  | CourtHive JWT secret, CourtHive Postgres password | `COURTHIVE_JWT_SECRET`, `COURTHIVE_PG_PASSWORD` in `deploy/.env` (read by CourtHive, not by this app) | `deploy/docker-compose.yml` refuses to start without them. Generating them per instance is WI-144. |

- **Production self-check.** With `APP_ENV=production` the app refuses to start
  if a secret variable it reads (`WRAPPED_ACCESS_PASSWORD`) holds a placeholder
  from the documentation such as `change-me`, `example`, `your-password` or a
  `<...>` placeholder. The error names the variable, never the value. The fix
  is to unset it (the instance then generates its own code) or set a value of
  your own.

## What this instance connects to

### Made by the server

| Destination | When | What is sent |
|---|---|---|
| `www.bhplta.co.uk` (BHPLTA) | When an administrator imports match cards, imports or copies a season's fixtures, imports club details, or looks up the club's BHPLTA name and id; when someone opens the public fixture generator (league tables, `LEAGUE_TABLES_URL` can point elsewhere) | Ordinary page requests; for match cards and the club lookup, the club's own BHPLTA club code and club name/id, as the BHPLTA website itself asks for them. The fixture generator's requests carry a User-Agent naming this software and its public source repository. |
| A fixtures page URL an administrator types in | When that administrator imports a season from it (default: the BHPLTA fixtures page) | A page request |
| `api.open-meteo.com` (Open-Meteo) | When an administrator opens a fixture detail page for a fixture within the forecast window; cached for an hour | The venue's latitude and longitude and the date range. No key, no account. |
| Browser push services: `fcm.googleapis.com` / `android.googleapis.com` (Google: Chrome, Android, Edge on Android, Samsung Internet, Opera, Brave), `*.push.services.mozilla.com` (Firefox), `*.push.apple.com` (Safari), `*.notify.windows.com` (Edge on Windows) | When a notification is sent to someone who turned notifications on | The notification, end-to-end encrypted to the subscriber's browser, and a VAPID token whose subscriber field is currently `https://jim.tennis` (to become the instance's public URL, WI-126). Subscriptions to any other host are refused. |
| `courthive-server` | Inside the instance's own Docker network, when an administrator syncs tournaments from CourtHive | The provider abbreviation |

The server makes no other outbound connection. There is no telemetry,
analytics, error reporting, crash upload, licence check, update check or remote
logging; logs stay in the container's log.

### Made by visitors' browsers

| Destination | When | What the browser sends |
|---|---|---|
| `www.openstreetmap.org` | A fixture venue page or an admin club page with coordinates embeds an OpenStreetMap map | The visitor's IP address, the map area (the venue's location) and, with the browsers' default referrer policy, this site's origin |

All scripts, styles and fonts are served by the instance itself: htmx is
vendored as `static/js/htmx-1.9.4.min.js` (previously loaded from unpkg.com
and cdn.jsdelivr.net) and the Week Overview and Club Wrapped pages no longer
load Google Fonts (they use Nunito / Inter if the device has them, otherwise
the system font).

Links that are only followed if someone clicks them: Google Maps and Apple
Maps directions, WhatsApp sharing (`wa.me`), Wikipedia pages for the
professional players used in fantasy doubles names, and the About page's links
to the source code (GitHub) and CourtHive.

### Made by the rest of the stack

| Destination | When |
|---|---|
| Container registry (Docker Hub for `alpine`, `golang`, `redis`, `postgres`, `caddy`) | When you build or pull images to install or update |
| Go module proxy (`proxy.golang.org`, `sum.golang.org`) and Alpine package mirror | When the app image is built |
| Alpine package mirror (`dl-cdn.alpinelinux.org`) | Each time the `backup` container starts: it installs `sqlite` and `tzdata` with `apk add` |
| Let's Encrypt (or ZeroSSL) via Caddy | Obtaining and renewing the TLS certificate for `DOMAIN` |

CourtHive (competition-factory-server, TMX, courthive-public) is a separate
codebase and is not covered by the scan above.

### What the author can see

Nothing from a running instance reaches the author. The remaining references to
`jim.tennis` are labels, not connections: the VAPID subscriber above (seen
only by the push services), calendar event UIDs (`fixture-N@jim.tennis`, never
contacted), the fixture file format name, and the About page text. WI-126
replaces the first two with the instance's public URL.
