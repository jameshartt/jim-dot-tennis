# Sprint 023: CourtHive Bundle - Club Championships in the Box

## Overview

**Goal**: Ship CourtHive (tournament server, TMX tournament management, and the public tournament pages) as an opt-in part of the package, so a club that installs jim-dot-tennis can also run its own club championships on its own server, with its own accounts and secrets, without building anything or knowing how the pieces fit.

**Duration**: dates TBD

**Status**: Planned

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. Published, club-neutral images of the three CourtHive components
2. One opt-in switch that brings up the tournament stack on the club's own server
3. Per-instance provisioning: the club's own provider, administrator and secrets
4. Upgrades and backups that include the tournament data

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-142 | Bundle design: licensing, configuration model, footprint, sign-in | High | M | Fable 5.2 | None |
| WI-143 | Publish club-neutral images of the three CourtHive components | High | L | Opus 5.5 | WI-142 |
| WI-144 | Opt-in tournament stack in the packaged deployment | High | M | Opus 5.5 | WI-142, WI-143 |
| WI-145 | Provision the club's tournament provider and administrator automatically | High | M | Opus 5.5 | WI-144 |
| WI-146 | Tournament data in backups; unattended CourtHive upgrades | High | M | Opus 5.5 | WI-144 |
| WI-147 | E2E smoke for the bundled stack | Medium | M | Sonnet 5.5 | WI-145, WI-146 |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-142** - Fable 5.2: Open-ended design across three unfamiliar codebases with legal and operational constraints; the decisions here shape the whole sprint.
- **WI-143** - Opus 5.5: Build and configuration changes across three TypeScript projects with known past pitfalls.
- **WI-144** - Opus 5.5: Multi-service deployment design with secret generation and health ordering.
- **WI-145** - Opus 5.5: Cross-system provisioning with idempotency and credential handling.
- **WI-146** - Opus 5.5: Data-safety work across two databases with ordering constraints learned the hard way.
- **WI-147** - Sonnet 5.5: Test authoring over a defined happy path.

## Execution Strategy

### Phase 1: Decide before building
- **WI-142** - Bundle design: licensing, configuration model, footprint, sign-in

### Phase 2: Images (143) and stack (144)
- **WI-143** - Publish club-neutral images of the three CourtHive components
- **WI-144** - Opt-in tournament stack in the packaged deployment

### Phase 3: Provisioning and upgrade safety (parallel)
- **WI-145** - Provision the club's tournament provider and administrator automatically
- **WI-146** - Tournament data in backups; unattended CourtHive upgrades

### Phase 4: Prove it
- **WI-147** - E2E smoke for the bundled stack

## Key Design Decisions

1. CourtHive is optional per club: off by default at install, on with one setting. A club that only wants league management never runs Postgres or Node.
2. The bundle is built from the jim-tennis-deploy branches of the three forks (competition-factory-server, TMX, courthive-public), which carry the fixes needed behind Caddy. Staying close to upstream remains the maintenance policy.
3. Redistribution needs upstream's blessing in writing. competition-factory-server and TMX declare MIT; courthive-public declares no licence in package.json and has no licence file. WI-142 resolves this with CourtHive's author before any image is published.
4. The front ends must be club-neutral: today TMX compiles the server address in at build time. Images either derive everything from the page's own address or take configuration at container start - no per-club builds.
5. Same isolation rule as the app: every secret generated on the club's server, no shared administrator, no default passwords.

## What Is NOT In This Sprint

- Running CourtHive for clubs centrally
- Feature work inside CourtHive beyond what packaging needs
- Single sign-on, unless WI-142 finds it cheap; otherwise the two logins stay separate and are documented
