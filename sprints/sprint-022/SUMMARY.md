# Sprint 022: Packaging, CI and Releases

## Overview

**Goal**: Turn the app into something that can be shipped: one self-contained binary and a container image per tagged version, built and tested automatically, with a visible version number, safe upgrades, and a tested backup and restore - and move jim.tennis itself onto those releases.

**Duration**: started 2026-11-21

**Status**: In progress - WI-135 to WI-140 completed 2026-11-21; WI-141 (production switch) waits for the owner

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. One self-contained binary: assets embedded, no CGO
2. CI on every push; tagged releases with binaries and a multi-architecture image
3. Upgrades that back up first and stop on failure
4. Backup and restore as first-class, tested commands

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-135 | Embed templates, static files and migrations in the binary | High | M | Opus 5.5 | None |
| WI-136 | Switch to pure-Go SQLite and build without CGO | High | M | Opus 5.5 | None |
| WI-137 | Operational basics: health, shutdown, strict migrations, version | High | M | Sonnet 5.5 | None |
| WI-138 | Backup and restore as tested commands | High | M | Opus 5.5 | WI-135 |
| WI-139 | Continuous integration on every push and pull request | High | M | Sonnet 5.5 | None |
| WI-140 | Tagged releases: binaries, multi-architecture image, changelog | High | M | Sonnet 5.5 | WI-135, WI-136, WI-137, WI-139 |
| WI-141 | Move jim.tennis onto the released image | Medium | M | Opus 5.5 | WI-138, WI-140 |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-135** - Opus 5.5: Changes how every template and asset is located; moderate blast radius.
- **WI-136** - Opus 5.5: A driver swap under the whole application; needs methodical verification of subtle behavioural differences.
- **WI-137** - Sonnet 5.5: A list of small, well-understood server changes.
- **WI-138** - Opus 5.5: Data-safety feature where a subtle mistake loses a club's season.
- **WI-139** - Sonnet 5.5: Standard workflow authoring with known tools.
- **WI-140** - Sonnet 5.5: Release configuration following documented patterns.
- **WI-141** - Opus 5.5: A production change on real data; needs judgement and a careful rollback plan, with the owner present.

## Execution Strategy

### Phase 1: Make it self-contained (parallel)
- **WI-135** - Embed templates, static files and migrations in the binary
- **WI-136** - Switch to pure-Go SQLite and build without CGO
- **WI-137** - Operational basics: health, shutdown, strict migrations, version

### Phase 2: Safety net (138 parallel with 139)
- **WI-138** - Backup and restore as tested commands
- **WI-139** - Continuous integration on every push and pull request

### Phase 3: Ship
- **WI-140** - Tagged releases: binaries, multi-architecture image, changelog

### Phase 4: Dogfood
- **WI-141** - Move jim.tennis onto the released image

## Key Design Decisions

1. Images are published to GitHub Container Registry and binaries to GitHub Releases. Clubs pull a version; nothing is compiled on the club's server.
2. Pure-Go SQLite (modernc.org/sqlite) replaces the CGO driver so Linux amd64 and arm64 binaries cross-compile cleanly. If the regression run in WI-136 shows a problem that cannot be resolved, the fallback is to keep CGO and ship images only.
3. Versions follow semantic versioning starting at v1.0.0 when Sprint 024 closes; pre-releases before that are v0.x.
4. A failed migration stops startup. An automatic copy of the database is taken before any migration runs.
5. jim.tennis moves to the released image last, with Jim present, after a droplet snapshot.

## What Is NOT In This Sprint

- Windows or macOS binaries
- Automatic self-update
- Hosted error reporting or telemetry (ruled out by instance isolation)

## Outcome so far (2026-11-21)

WI-135 to WI-140 are completed and merged on `sprint-022/packaging-ci-releases`. WI-141 (move jim.tennis onto the released image) is not started: it changes production and needs the owner present, a droplet snapshot and a published release first.

- Verified on the final tree: `make check` passes; `make test-e2e-all`: main suite 226 passed (none skipped), passkey suite 12 passed, fresh install 13 passed; `make test-e2e-multiclub`: 10 passed.
- No migrations added (latest is still 035; next free 036).
- Dependencies: `modernc.org/sqlite` v1.62.0 replaces `mattn/go-sqlite3`; builds use `CGO_ENABLED=0`.
- Release tooling: plain Docker buildx and the `gh` CLI rather than GoReleaser, so the Dockerfile stays the one build definition.
- Timings on a copy of production data (medians, old driver to new): dashboard 17.1 ms to 14.0 ms; planning matrix 350.9 ms to 210.8 ms; team selection 207.0 ms to 130.7 ms.
- Added beyond the plan: browser tests for the backups page (password and passkey confirmation); an independent review of backup/restore and the workflows, with fixes; the long-skipped `workflows/match-results.spec.ts:46` re-enabled (an earlier spec reschedules fixture 1, which by design replaces its matchups; the test had hard-coded matchup ids); a stale multi-club dashboard assertion corrected.

### Behaviour changes

- Templates, static files and migrations are built into the binary. Bind mounts of `templates/` and `static/` are ignored unless `TEMPLATE_RELOAD=true`.
- `MIGRATE_ALLOW_DIRTY_FORCE` is gone: a failed or dirty migration always stops startup, with recovery steps in the message.
- A backup is taken before migrations change an existing database; if it cannot be made, startup stops.
- Backups run inside the app (`BACKUP_INTERVAL` default 24h, `BACKUP_KEEP` default 14). The sidecar container and `scripts/backup-manager.sh` are removed.
- `jim-dot-tennis backup`, `jim-dot-tennis restore <file>` and `jim-dot-tennis version`. Restore refuses while the server runs, refuses a newer archive, ends every session and cancels unused links.
- Administrators download a backup from `/admin/league/backups` after confirming with a password or passkey (passkey only while passkeys are required for administrators).
- `/healthz` is 200 only when the schema is clean and current; `/version` is public and shows the version only.
- `deploy/docker-compose.yml` pulls `ghcr.io/jameshartt/jim-dot-tennis` by version instead of building.
- Base images in the Dockerfiles are pinned by digest.

### Review record

Independent review (Opus) of `internal/backup` and the CI and release workflows at e19289b. No high-severity findings. All fourteen were fixed, each code fix with a regression test seen failing without it.

| # | Severity | Finding | Outcome |
|---|----------|---------|---------|
| 1 | Medium | A future-dated archive name stopped scheduled backups, hid the stale warning and won pruning | Fixed |
| 2 | Medium | Restore wrote entry bodies to disk before comparing them with the manifest | Fixed; free-space check added |
| 3 | Low | `.restore-*` staging folders left after Ctrl-C or a crash | Fixed |
| 4 | Low | Logos swapped before the database; undo errors ignored | Fixed; database rename is the commit point |
| 5 | Low | Set-aside `-wal`/`-journal` kept their original mode | Fixed |
| 6 | Low | Restore as root copied the folder's owner, not the database's | Fixed |
| 7 | Low | Two backups finishing in the same second could collide | Fixed |
| 8 | Low | Account CLI commands could write during a restore | Fixed with a separate restore lock |
| 9 | Low | Download accepted a password while passkeys were required for administrators | Fixed |
| 10 | Info | Temp-file sweeps, restore docs, set-aside copies | Fixed or documented |
| 11 | Low | `latest` could move backwards | Fixed with a `promote` job |
| 12 | Low | Binary and image not checked to match; floating base images | Fixed with a `verify` job and digest pins |
| 13 | Low | Image could publish without a GitHub Release | Fixed |
| 14 | Info | Commit date derived from a possibly annotated tag | Fixed |

### Not verified

- The release workflow has never run: publishing, attestations, the `verify` and `promote` jobs and `gh` flags are checked by actionlint only. The first real tag is the test.
- CI on GitHub runners (timings, caches, artifact uploads) until the branch is pushed. The Playwright image cache step may not hit now that `Dockerfile.e2e` is pinned by digest.
- The arm64 binary and image were built or cross-compiled, never run.
- A pre-migration backup during a real container upgrade; the restore command in the operations guide against a real server.
- The acceptance test "cut a release candidate and install it on a clean machine" (WI-140).

### Known leftovers

- The server running on jim.tennis predates the data-folder lock, so `restore` cannot detect it on the first move (WI-141): stop the app first.
- `scripts/deploy-app.sh` still builds on the server and its pre-deploy backup writes to the old backups volume; WI-141 retires that path.
- `docker compose --profile test down -v` run from the repository would try to remove the local development data volume. An agent ran it once this sprint; Docker refused because the volume was in use. Worth guarding (for example marking the volume external).
- Owner steps: push the first tag (after dating the CHANGELOG heading), make the ghcr.io package public once, add branch protection with the six CI checks, enable Dependabot.
- `tennis.db.replaced-*` copies from restores are kept until removed by hand.
