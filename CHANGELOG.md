# Changelog

All notable changes to jim.tennis are recorded here, newest first. Versions
follow [semantic versioning](https://semver.org). Versions before 1.0.0 are
pre-releases: the upgrade path between them is documented but not promised to be
stable.

## How this file works

Every version has a section headed `## [X.Y.Z] - date`, in this order:

- **Summary**: one or two sentences on what the version is.
- **Added**, **Changed**, **Fixed**, **Removed**: what a club will notice. Leave out
  headings with nothing under them.
- **Upgrade notes**: what a club operator must know or do before or after moving
  to this version from the previous one. Always present. If there is nothing,
  write "None: replace the image and restart." Cover anything that changes
  required settings, signs people out, needs a manual step, changes a file or
  folder layout, or cannot be undone by simply going back to the old version
  (database migrations: the app takes a backup before it migrates, and the
  upgrade notes name any migration that makes a rollback need that backup).

The release workflow refuses to publish a tag that has no `## [version]` section
here. GitHub also generates a list of merged changes for each release; this file
is the curated version that matters to operators. Moving from several versions
back: read the Upgrade notes of every version in between.

## [Unreleased]

### Added

- **Tournaments in the packaged deployment, off by default.**
  `deploy/docker-compose.yml` can now run the tournament stack (CourtHive)
  next to the app, as the compose profile `tournaments`: Postgres, Redis, the
  tournament server, its identity service, TMX at `/tournaments` and the
  public tournament pages at `/public`. Every image is published; nothing is
  built on your server. To turn it on: run `deploy/init-tournaments.sh` once,
  add `COMPOSE_PROFILES=tournaments` to `deploy/.env`, run
  `docker compose up -d`, then switch on Tournaments in Club Settings.
- **`deploy/init-tournaments.sh`** makes every tournament secret on your own
  server (a signing key pair and its id, a signer token, the service account
  password, the database password and the tournament server's unused JWT
  secret) and adds them to `deploy/.env`. It never overwrites a value, never
  prints one, and sets `.env` to mode 600. Nothing has a default: with
  tournaments on, compose refuses to start if any secret is missing.

Bundled tournament set (tested together):

| Part | Image |
|---|---|
| Tournament server | `ghcr.io/jameshartt/courthive-server:4.19.1-jdt.1` |
| Identity service | `ghcr.io/jameshartt/courthive-hiveid:0.5.0-jdt.1` |
| TMX | `ghcr.io/jameshartt/courthive-tmx:8.61.0-jdt.1` |
| Public pages | `ghcr.io/jameshartt/courthive-public:1.23.7-jdt.1` |

### Changed

- With tournaments off, only the app and Caddy run, and Caddy answers
  `/tournaments`, `/public`, `/api/courthive` and `/socket.io` with a plain
  404. `/api/rankings` and `/epixodic` at the site root always get 404.
- With tournaments on, Caddy removes the `Cookie` header before passing a
  request to the tournament stack, and refuses (404) its sign-in, account,
  administration, provisioner and internal routes; only Caddy publishes ports.
  Memory limits: tournament server 512 MB, identity service 256 MB, Postgres
  384 MB, Redis 64 MB.
- `deploy/docker-compose.yml` no longer builds CourtHive from a source
  checkout or serves front ends from `../../TMX/dist` and
  `../../courthive-public/dist`.

### Upgrade notes

- Without tournaments: nothing to do. You may delete the lines
  `COURTHIVE_PG_PASSWORD` and `COURTHIVE_JWT_SECRET` from `deploy/.env` if
  they still say `change-me`; they are no longer needed.
- To turn tournaments on after upgrading: if `deploy/.env` still has
  `COURTHIVE_PG_PASSWORD=change-me` or `COURTHIVE_JWT_SECRET=change-me`,
  delete those lines first, then follow the steps above.
  `init-tournaments.sh` stops and says so if it finds them.
- An instance that already ran CourtHive from the old source-built layout
  (jim.tennis) must not switch to this file without a planned move: the
  identity service, the sign-in model and the tournament database version all
  change (`docs/courthive_bundle_design.md`, section 5). The old volumes
  `courthive-data` and `courthive-cache` are not used by the new file and are
  left alone.

## [0.1.0] - 2026-11-22

### Summary

The first pre-release: the state of the app after the club-readiness work of
sprints 019 to 022. A club can run its own instance from one binary or one
container image, sign in with passkeys, and back up and restore its own data.
Nothing is sent to the software's author.

### Added

- **Passkey sign-in** as the default, with passwords as an optional fallback;
  administrators can require passkeys for the administrator role.
- **Invite and recovery links** (no email): administrators issue them from the
  users page, or the operator runs `jim-dot-tennis user create` / `user reset`
  on the server.
- **A real captain role**: captains do everything for every team; users,
  sessions, settings, seasons and similar administration stay with
  administrators (`docs/roles.md`).
- **Account audit log** (administrators), and scheduled cleanup of old sessions,
  login attempts and links.
- **First-run setup** with a one-time code printed to the log; the instance
  ships with no accounts and no shared or default secrets.
- **Season import from the BHPLTA website** and a Club Settings page (name,
  logo, colours, club code, public URL, feature switches).
- **In-app backup and restore**: `jim-dot-tennis backup` and
  `jim-dot-tennis restore <file>`, a daily scheduled backup, and an automatic
  backup before any migration changes an existing database.
- **`jim-dot-tennis version`**, the version in the administrator footer, and a
  public `/version` page; a `/healthz` check that is healthy only when the
  database answers and migrations are current.
- **Graceful shutdown** and published releases: Linux amd64 and arm64 binaries
  with checksums, and a multi-architecture image on `ghcr.io`, with build
  provenance attestations.

### Changed

- SQLite is now the pure-Go driver (`modernc.org/sqlite`): no CGO, one static
  binary. Existing database files are used unchanged.
- Templates, static files and migrations are built into the binary.
- A failed or dirty migration stops startup instead of continuing.

### Removed

- The backup sidecar container of the reference stack (replaced by in-app
  backups).
- `MIGRATE_ALLOW_DIRTY_FORCE`.
- The `player` account role (an existing player-role account can still sign in
  and manage its own sign-in methods, and reaches nothing else).

### Upgrade notes

For an existing deployment moving to this version (a new install can ignore
this list):

1. **`PUBLIC_URL` is required** when `APP_ENV=production` (the reference stack
   sets that). It must be your instance's `https://` address. Without it the app
   refuses to start. Passkeys and recovery links are bound to this address, so
   changing its host later invalidates passkeys. If you serve calendar feeds,
   keep the host the same as before: calendar event ids include it.
2. **Everyone is signed out once.** Migration 035 re-keys sessions (only a hash
   of each session token is kept now), so every existing session ends.
3. **Logout is POST-only.** A bookmarked or hand-written link to `/logout` now
   returns 405; the logout buttons in the app are unchanged.
4. **Administrators no longer set passwords for other people.** Creating a user
   gives you an invite link to hand over; recovery is a link from the users page
   or `jim-dot-tennis user reset <name>` on the server. Existing passwords keep
   working.
5. **New and changed passwords need at least 12 characters** and must not be on
   the common-password list. Adding or removing a sign-in method needs the
   current password or an existing passkey and signs the account out on its
   other devices.
6. **Existing `captain` accounts gain the captain access described in
   `docs/roles.md`** (all teams). Check who holds that role.
7. **Migrations 032 to 035** run on first start. The app takes a pre-migration
   backup first (in `backups/` beside the database). If a migration fails the app
   stops and says how to recover; it never carries on. `MIGRATE_ALLOW_DIRTY_FORCE`
   is gone and a dirty state is not forced.
8. **Templates and static files are inside the binary.** Bind mounts of
   `templates/` and `static/` over the app are ignored unless `TEMPLATE_RELOAD`
   is `true` (development only). Remove those mounts from your compose file.
   A club logo is uploaded on the Club Settings page and stored on the data
   volume.
9. **The backup sidecar is replaced by in-app backups.** Remove the sidecar
   service from your compose file. The app makes a backup every `BACKUP_INTERVAL`
   (default 24h, or `off`), keeps `BACKUP_KEEP` (default 14) in `BACKUP_DIR`
   (default `backups/` beside the database), and one before each upgrade. They
   hold personal data and password hashes: copy them off the server too. Restore
   with `jim-dot-tennis restore <file>` while the server is stopped
   (`docs/PRODUCTION_QUICK_REFERENCE.md`, section 9).
10. **SQLite is pure Go.** The database file and its format are unchanged, but
    build tooling that assumed CGO or a C toolchain no longer needs it, and the
    runtime image carries no SQLite package.
11. **The reference `deploy/docker-compose.yml` runs the published image**
    (`JIM_DOT_TENNIS_VERSION`, no local build of the app). To build from source
    use the root `docker-compose.yml`.
