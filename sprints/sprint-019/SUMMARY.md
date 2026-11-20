# Sprint 019: Public Repo Cleanup & SQLite-Only

## Overview

**Goal**: Make the public repository safe and honest to hand to another club: no live credentials or author-specific server details in tracked files, the real reference deployment in git, no AGPL dependency in the shipped module, and SQLite declared (and coded) as the only supported database.

**Duration**: dates TBD

**Status**: Planned

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. Remove credentials and author-server details from tracked docs
2. Bring the real production compose file and Caddyfile into git as the reference deployment
3. Keep the AGPL MuPDF dependency out of anything a club receives
4. SQLite is the only supported database - remove the untested Postgres path

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-112 | Scrub credentials and author-server details from tracked files | High | S | Sonnet 5.5 | None |
| WI-113 | Commit the real reference deployment (compose + Caddyfile) | High | S | Sonnet 5.5 | None |
| WI-114 | Isolate the AGPL PDF scraper and correct acknowledgements | Medium | S | Sonnet 5.5 | None |
| WI-115 | Declare SQLite the only database and remove the Postgres path | Medium | M | Sonnet 5.5 | None |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-112** - Sonnet 5.5: Mechanical, well-specified edits across a known list of files.
- **WI-113** - Sonnet 5.5: Small, careful file consolidation with no design decisions.
- **WI-114** - Sonnet 5.5: Contained dependency surgery with a clear pass/fail check.
- **WI-115** - Sonnet 5.5: Deletion-heavy refactor guarded by the existing test suites.

## Execution Strategy

### Phase 1: Independent cleanups (all parallel)
- **WI-112** - Scrub credentials and author-server details from tracked files
- **WI-113** - Commit the real reference deployment (compose + Caddyfile)
- **WI-114** - Isolate the AGPL PDF scraper and correct acknowledgements
- **WI-115** - Declare SQLite the only database and remove the Postgres path

## Key Design Decisions

1. Rotation before scrubbing. The CourtHive admin password is rotated by Jim on the server first; git history is not rewritten, because a rotated password in history is harmless and a history rewrite breaks every fork.
2. Operator-private detail (server IP, SSH key path, account names) moves out of the public repo into a gitignored local runbook; the tracked runbook keeps procedures with placeholders.
3. SQLite only. The Postgres branch of the app was never exercised end to end (repositories use `?` placeholders). CourtHive's own Postgres database is unrelated and stays.
4. The driver swap to pure-Go SQLite is deliberately not in this sprint - it lands with packaging in Sprint 022 (WI-136) where the full regression run is planned.

## What Is NOT In This Sprint

- Rewriting git history
- Any change to production other than the password rotation Jim performs himself
