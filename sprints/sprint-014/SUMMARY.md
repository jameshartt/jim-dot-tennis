# Sprint 014: Club-Agnostic Refactor

## Goal
Remove all hardcoded St Ann's assumptions from the codebase so that any parks league club can deploy jim.tennis by setting a single `HOME_CLUB_ID` environment variable, with full regression testing at every phase.

## Problem Statement
jim.tennis was built for St Ann's Tennis Club and that assumption is woven throughout the codebase: 43+ calls to `FindByNameLike(ctx, "St Ann")`, 8+ methods named `GetStAnnsTeams()` / `IsStAnnsClub()`, struct fields like `IsStAnnsHome`, CSS variables named `--stanns-green`, form fields named `stanns_player_1`, and 10+ raw SQL queries with `LIKE '%St Ann%'`. The apostrophe in "St Ann's" is handled differently by three separate normalization strategies, and the player matcher doesn't handle apostrophes at all. None of this is configurable — to deploy for a different club, you'd need to search-and-replace across 50+ files.

The database schema already supports multi-club operations. This is purely an application-layer refactor.

## Scope & Risk

This is the highest-blast-radius sprint in the project's history. It touches:
- **15+ service files** (every `service_*.go` in `internal/admin/`)
- **9+ template files** (admin and player-facing)
- **2 large SQL-heavy files** (club_wrapped.go, points.go)
- **5+ integration files** (BHPLTA import, scripts)
- **7+ E2E test files** (seed data, spec assertions)
- **All cmd/ utilities** that reference club names

The risk is mitigated by:
- Strict phase ordering with compilation + E2E verification at each gate
- The Go compiler catching every missed rename in .go files
- Existing 146-test E2E suite catching template and behavioral regressions
- A dedicated multi-club verification suite (WI-085) as the ultimate proof

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-077 | Home club configuration and context injection | Critical | M | None | No |
| WI-078 | Centralised apostrophe normalization utility | Critical | M | None | Yes |
| WI-079 | Rename St Ann's-specific DTOs, struct fields, and method signatures | Critical | L | WI-077 | No |
| WI-080 | Replace FindByNameLike('St Ann') with config-driven home club ID | Critical | XL | WI-077, WI-079 | No |
| WI-081 | Refactor Club Wrapped and Points SQL queries | Critical | L | WI-077, WI-079 | No |
| WI-082 | Update templates to use dynamic home club name | High | M | WI-079, WI-080 | No |
| WI-083 | BHPLTA integration — configurable club code | High | M | WI-077 | No |
| WI-084 | E2E test infrastructure — parameterise seed data and assertions | Critical | L | WI-079, WI-080, WI-082 | No |
| WI-085 | Multi-club verification test suite | Critical | L | WI-084 | No |
| WI-086 | Full regression validation and cleanup | Critical | M | All code WIs | No |
| WI-087 | Update all documentation to reflect club-agnostic architecture | Critical | L | WI-080, WI-081, WI-082, WI-083 | No |

## Execution Plan

### Phase 1 — Foundation (can be parallel)
- **WI-077**: Home club config, env var, middleware context injection. Everything depends on this.
- **WI-078**: Apostrophe normalization utility. Independent of WI-077, can run in parallel.

**Gate: `make build-local` succeeds, existing E2E suite passes unchanged.**

### Phase 2 — Naming (after WI-077)
- **WI-079**: Rename all St Ann's-specific identifiers to generic home-club terminology. Single atomic commit. The Go compiler enforces completeness.

**Gate: `make build-local` succeeds, `grep -r 'StAnns\|stanns' internal/ templates/` returns zero, E2E suite passes.**

### Phase 3 — Core Logic (after WI-079, can be partially parallel)
- **WI-080**: Replace all 43+ `FindByNameLike("St Ann")` calls with config-driven club ID. This is the largest single work item.
- **WI-081**: Refactor Club Wrapped and Points SQL queries. Can be done in parallel with WI-080 since they touch different files.

**Gate: `make build-local` succeeds, `grep -r 'FindByNameLike.*St Ann' internal/` returns zero, E2E suite passes, Club Wrapped and Points pages render correctly.**

### Phase 4 — UI and Integration (after Phase 3)
- **WI-082**: Template text content — replace remaining hardcoded "St Ann's" with dynamic club name.
- **WI-083**: BHPLTA integration — configurable club code. Can be parallel with WI-082.

**Gate: `grep -r 'St Ann' templates/` returns zero (excluding comments), E2E suite passes, import UI shows configured club code.**

### Phase 5 — Test Overhaul (after Phase 4)
- **WI-084**: Parameterise E2E seed data and test assertions. Add data-testid attributes to templates.
- **WI-085**: New multi-club verification suite. Proves the refactor works from a different club's perspective.

**Gate: Full E2E suite passes (146+ tests). Multi-club suite passes (10+ tests). No spec file contains hardcoded 'St Ann'.**

### Phase 6 — Documentation (after Phases 3-4)
- **WI-087**: Sweep every .md file in the repo. Update README, CLAUDE.md, CONTRIBUTING.md, docs/, scripts/ guides to reflect the new HOME_CLUB_ID-driven architecture. Leave historical sprint notes as-is.

**Gate: `grep -r 'St Ann' *.md docs/ scripts/` — every remaining reference is attribution/origin-story or historical. New env vars documented everywhere they need to be.**

### Phase 7 — Validation (after everything)
- **WI-086**: Full regression sweep. Linting, compilation, E2E, multi-club tests, codebase greps. No documentation in this WI — that's WI-087's job.

**Gate: Everything green. Everything documented. Sprint closed.**

## Key Technical Decisions

### HOME_CLUB_ID, not multi-tenant
This sprint makes the app configurable for a single club per deployment, not multi-tenant. Each deployment serves one club. This is the right scope — multi-tenancy would require auth changes, data isolation, and UI for club switching, which is far more complex and not what any club actually needs. One instance per club is the correct deployment model.

### Fallback to HOME_CLUB_NAME for backward compatibility
Existing deployments (St Ann's) shouldn't break. If `HOME_CLUB_ID` isn't set, the app falls back to `HOME_CLUB_NAME` and uses `FindByNameLike` once at startup. This means the current production deployment needs zero config changes.

### Apostrophe normalization is consolidated, not eliminated
We can't control what Unicode characters BHPLTA sends, what CSV files contain, or what users type. The solution is a single normalization function used everywhere, not fixing the source data. Apostrophes are normalized to ASCII U+0027 for storage and display, and removed entirely for comparison/matching.

### Derby detection is now generic
Old: "both teams are St Ann's". New: "both teams belong to the same club". This is actually more correct — it handles any club's internal derbies, not just St Ann's.

### CSS theming is punted
The colour values (green, blue) are renamed from `--stanns-green` to `--home-primary` but the actual hex values stay. Full theming (configurable brand colours) is out of scope — deployers can override the CSS manually. A future sprint could add `HOME_CLUB_PRIMARY_COLOR` env var support.

## What This Sprint Does NOT Do

- **Multi-tenancy**: One club per deployment. No club switcher, no shared databases.
- **Full theming**: Colours are renamed but not configurable via env vars.
- **BHPLTA scraper rewrite**: The scraper still targets bhplta.org.uk — it just uses a configurable club code.
- **Apostrophe elimination from source data**: We normalize on the way in, not retroactively in the database.
- **User-to-club association**: No ClubID on the User model. All users in a deployment belong to the configured home club. Multi-club user management is a future concern.

## Verification Strategy

Every phase has an explicit gate. The mantra is: **compile, test, grep, then move on**.

1. After each work item: `make build-local` must succeed
2. After each phase: full E2E suite must pass
3. After WI-080: grep confirms zero `FindByNameLike` with hardcoded names
4. After WI-082: grep confirms zero hardcoded "St Ann" in templates
5. After WI-085: multi-club suite proves the refactor works for a different club
6. WI-086 is the final sweep — nothing is missed

If any gate fails, stop and fix before proceeding. Do not accumulate breakage across phases.
