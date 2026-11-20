# Sprint 022: Packaging, CI and Releases

## Overview

**Goal**: Turn the app into something that can be shipped: one self-contained binary and a container image per tagged version, built and tested automatically, with a visible version number, safe upgrades, and a tested backup and restore - and move jim.tennis itself onto those releases.

**Duration**: dates TBD

**Status**: Planned

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
