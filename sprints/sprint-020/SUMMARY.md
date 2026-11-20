# Sprint 020: Fresh Install - From Empty Database to a Working Club

## Overview

**Goal**: A BHPLTA club with no prior knowledge starts the app on an empty database, is walked through creating its first administrator and choosing its club, has the league, divisions, clubs and current season pulled in from BHPLTA, and ends up on a working dashboard - with no SQL, no CLI import tools, no inherited accounts and no secrets shared with any other instance.

**Duration**: dates TBD

**Status**: Planned

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. The app starts on an empty database and guides setup instead of exiting
2. League, divisions, clubs and season come from BHPLTA inside the setup flow
3. Club identity (name, logo, colours, BHPLTA identifiers) is a stored, editable setting
4. Instance isolation: no seeded accounts, no shared secrets - the author has no way in

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-116 | Setup mode: start on an empty database; home club as a stored setting | High | M | Opus 5.5 | None |
| WI-117 | First-run setup: one-time code, first administrator, choose your club | High | L | Opus 5.5 | WI-116 |
| WI-118 | Bootstrap league, divisions, clubs and season from BHPLTA in-app | High | L | Opus 5.5 | WI-116, WI-117 |
| WI-119 | Club settings page: identity, logo, colours, BHPLTA identifiers | High | M | Opus 5.5 | WI-116 |
| WI-120 | Remove the remaining St Ann's defaults and the dormant trigger | Medium | S | Sonnet 5.5 | WI-116 |
| WI-121 | Ship the fantasy name list inside the app and load it automatically | High | S | Sonnet 5.5 | WI-116 |
| WI-122 | Feature switches and a home page that fits the club | Medium | S | Sonnet 5.5 | WI-116 |
| WI-123 | Instance isolation: no seeded accounts, no shared or default secrets | High | M | Opus 5.5 | WI-117 |
| WI-124 | Optional demo data for exploring before going live | Low | M | Sonnet 5.5 | WI-117, WI-118 |
| WI-125 | E2E: fresh-install suite | High | M | Sonnet 5.5 | WI-116, WI-117, WI-118, WI-119, WI-120, WI-121, WI-122, WI-123 |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-116** - Opus 5.5: Touches startup, config injection and every handler's view of the home club; needs careful reasoning about state that used to be immutable.
- **WI-117** - Opus 5.5: Security-relevant flow (instance claiming) with real UX judgement; larger than a mechanical task but with a clear specification.
- **WI-118** - Opus 5.5: Lifts CLI logic into a transactional service across several repositories, with failure handling against an external site.
- **WI-119** - Opus 5.5: New admin surface with file upload handling and config precedence rules.
- **WI-120** - Sonnet 5.5: Wide but shallow sweep with a grep-verifiable end state.
- **WI-121** - Sonnet 5.5: Small, self-contained change.
- **WI-122** - Sonnet 5.5: Template and routing conditionals driven by a settings lookup.
- **WI-123** - Opus 5.5: The isolation guarantee is the programme's central trust promise; needs a thorough, sceptical pass rather than a mechanical one.
- **WI-124** - Sonnet 5.5: Seed data and a guarded load/clear action.
- **WI-125** - Sonnet 5.5: Test authoring against a specified flow.

## Execution Strategy

### Phase 1: Foundation
- **WI-116** - Setup mode: start on an empty database; home club as a stored setting

### Phase 2: Setup flow and settings (116 first; 117 before 118)
- **WI-117** - First-run setup: one-time code, first administrator, choose your club
- **WI-118** - Bootstrap league, divisions, clubs and season from BHPLTA in-app
- **WI-119** - Club settings page: identity, logo, colours, BHPLTA identifiers

### Phase 3: Loose ends (parallel once 116 is in)
- **WI-120** - Remove the remaining St Ann's defaults and the dormant trigger
- **WI-121** - Ship the fantasy name list inside the app and load it automatically
- **WI-122** - Feature switches and a home page that fits the club

### Phase 4: Isolation and demo (after 117)
- **WI-123** - Instance isolation: no seeded accounts, no shared or default secrets
- **WI-124** - Optional demo data for exploring before going live

### Phase 5: Lock it in
- **WI-125** - E2E: fresh-install suite

## Key Design Decisions

1. BHPLTA only. Setup offers the known BHPLTA clubs and imports structure from the BHPLTA site; there is no generic league or division creator and no configurable rules engine.
2. The home club becomes a stored setting chosen during setup. HOME_CLUB_ID / HOME_CLUB_NAME remain as overrides so existing deployments and the e2e suites keep working unchanged.
3. Setup is protected by a one-time code printed to the server log (and available from the CLI in Sprint 021), so whoever reaches the URL first cannot claim the instance.
4. Migration 004 stops seeding accounts for new installs by editing the migration file; golang-migrate does not checksum applied migrations, so existing installs are unaffected. Their existing accounts are left alone.
5. The first administrator is created with a password in this sprint; passkey enrolment is added to the same step in Sprint 021 (WI-128).

## What Is NOT In This Sprint

- Support for leagues other than BHPLTA
- Several clubs sharing one database
- Full theming beyond logo and two colours
