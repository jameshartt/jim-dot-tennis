# Sprint 023: CourtHive Bundle - Club Championships in the Box

## Overview

**Goal**: Ship CourtHive (tournament server, TMX tournament management, and the public tournament pages) as an opt-in part of the package, so a club that installs jim-dot-tennis can also run its own club championships on its own server, with its own accounts and secrets, without building anything or knowing how the pieces fit.

**Duration**: started 2026-11-22

**Status**: Implemented and tested locally on 2026-11-22; branches pushed and pull request #30 opened on 2026-11-23; the upgrade rehearsal on a copy of jim.tennis's data passed on 2026-11-23; publishing the CourtHive images and moving jim.tennis onto the bundle wait for the owner

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. Club-neutral images of the four CourtHive components (latest upstream)
2. One opt-in switch that brings up the tournament stack on the club's own server
3. Per-instance provisioning: the club's own provider, administrator and secrets
4. Upgrades and backups that include the tournament data
5. One sign-in: jim-dot-tennis is the identity source for tournament management (decided 2026-11-21; resolves Sprint 021 review finding F9)

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-142 | Bundle design: licensing, configuration model, footprint, sign-in | High | M | Fable 5.2 | None |
| WI-143 | Club-neutral images of the four CourtHive components | High | L | Opus 5.5 | WI-142 |
| WI-144 | Opt-in tournament stack in the packaged deployment | High | M | Opus 5.5 | WI-142, WI-143 |
| WI-145 | Provision the club's tournament provider and administrator automatically | High | M | Opus 5.5 | WI-144 |
| WI-146 | Tournament data in backups; unattended CourtHive upgrades | High | M | Opus 5.5 | WI-144 |
| WI-147 | E2E smoke for the bundled stack | Medium | M | Sonnet 5.5 | WI-145, WI-146, WI-152 |
| WI-152 | Single sign-on: jim-dot-tennis as the identity source for CourtHive | High | L | Fable 5.2 | WI-142, WI-144, WI-145, WI-153 |
| WI-153 | Tournament-only accounts and per-account tournament access | High | M | Opus 5.5 | WI-142 |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-142** - Fable 5.2: Open-ended design across three unfamiliar codebases with legal and operational constraints; the decisions here shape the whole sprint.
- **WI-143** - Opus 5.5: Build and configuration changes across three TypeScript projects with known past pitfalls.
- **WI-144** - Opus 5.5: Multi-service deployment design with secret generation and health ordering.
- **WI-145** - Opus 5.5: Cross-system provisioning with idempotency and credential handling.
- **WI-146** - Opus 5.5: Data-safety work across two databases with ordering constraints learned the hard way.
- **WI-147** - Sonnet 5.5: Test authoring over a defined happy path.
- **WI-153** - Opus 5.5: Account and role changes with security consequences, in code with an established pattern.
- **WI-152** - Fable 5.2: Cross-system trust design where a mistake gives one system's users control of the other; touches a codebase we do not own.

## Execution Strategy

### Phase 1: Decide before building
- **WI-142** - Bundle design: licensing, configuration model, footprint, sign-in

### Phase 2: Images (143) and stack (144)
- **WI-143** - Publish club-neutral images of the three CourtHive components
- **WI-144** - Opt-in tournament stack in the packaged deployment

### Phase 3: Provisioning and upgrade safety (parallel)
- **WI-145** - Provision the club's tournament provider and administrator automatically
- **WI-146** - Tournament data in backups; unattended CourtHive upgrades

### Phase 4: One sign-in
- **WI-152** - Single sign-on: jim-dot-tennis as the identity source for CourtHive (may split into its own sprint if it proves too large; the bundle works without it)

### Phase 5: Prove it
- **WI-147** - E2E smoke for the bundled stack

## Key Design Decisions

Decided on 2026-11-22; the detail and the evidence are in `docs/courthive_bundle_design.md`.

1. CourtHive is optional per club: off by default at install, on with one setting.
2. The bundle tracks the latest CourtHive (owner's decision). First set: tournament server 4.19.1, identity service 0.5.0, TMX 8.61.0, public pages 1.23.7. Each fork carries a small `jdt-bundle` branch on top of an upstream commit.
3. All four components are MIT with a licence file; each image carries the licence, a notice and third-party licences.
4. One build serves every club: every address is a path on the host that serves the page.
5. jim-dot-tennis is the only place people sign in. It creates password-less tournament accounts and hands people to TMX with a single-use token. CourtHive's identity service signs every token with a key generated on the club's server; its own sign-in routes are closed at the proxy.
6. Tournament access is a per-account setting (none, own, all); people who only run tournaments get "Tournaments only" accounts (WI-153, owner's request).
7. Same isolation rule as the app: every secret generated on the club's server, no shared administrator, no default passwords.

## What Is NOT In This Sprint

- Running CourtHive for clubs centrally
- Feature work inside CourtHive beyond what packaging needs
- Services CourtHive runs alongside its own hosting and the bundle does not ship: registrations (AMS), declarations, rankings, the query service, the score relay and player sign-in on the public pages
