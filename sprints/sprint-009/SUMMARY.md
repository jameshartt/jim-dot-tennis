# Sprint 009: Core E2E Test Suite

## Overview
- **Goal:** Test all critical user flows including authentication, admin dashboard and CRUD pages, player-facing pages, and data-heavy reporting pages with comprehensive Playwright browser tests
- **Duration:** 1 day
- **Status:** Complete
- **Work Items:** 5 (WI-049 to WI-053) — all completed

## Focus Areas
1. Authentication flow testing (login, logout, session, rate limiting)
2. Admin dashboard and navigation verification
3. Admin CRUD page coverage for all entities
4. Player-facing pages with token-based auth
5. Points table and standings with data filtering

## Work Items Summary

| ID | Title | Priority | Complexity | Status |
|----|-------|----------|------------|--------|
| WI-049 | Authentication flow tests | High | M | Complete |
| WI-050 | Admin dashboard & navigation tests | High | M | Complete |
| WI-051 | Admin CRUD page tests | High | L | Complete |
| WI-052 | Player-facing page tests | High | M | Complete |
| WI-053 | Points table & standings tests | Medium | S | Complete |

## Deliverables

### Test Spec Files (15 files, 104 tests total)
- `tests/e2e/auth.spec.ts` — login, logout, session persistence, rate limiting (8 tests)
- `tests/e2e/admin/dashboard.spec.ts` — stat cards, action links, login attempts (7 tests)
- `tests/e2e/admin/navigation.spec.ts` — all admin section page loads (15 tests)
- `tests/e2e/admin/players.spec.ts` — player list, search, seeded data (6 tests)
- `tests/e2e/admin/clubs.spec.ts` — club list, detail pages (6 tests)
- `tests/e2e/admin/teams.spec.ts` — teams, away teams (5 tests)
- `tests/e2e/admin/fixtures.spec.ts` — fixtures, detail, week overview (7 tests)
- `tests/e2e/admin/divisions.spec.ts` — division edit, review (3 tests)
- `tests/e2e/admin/seasons.spec.ts` — season list, setup, active badge (6 tests)
- `tests/e2e/admin/users.spec.ts` — user management, sessions, create form (6 tests)
- `tests/e2e/admin/points-table.spec.ts` — points layout, sections (6 tests)
- `tests/e2e/admin/wrapped.spec.ts` — admin and public wrapped pages (3 tests)
- `tests/e2e/players/availability.spec.ts` — token auth, match cards (5 tests)
- `tests/e2e/players/profile.spec.ts` — profile, history, stats (9 tests)
- `tests/e2e/players/standings.spec.ts` — public standings, divisions (7 tests)

### Infrastructure Improvements
- `tests/e2e/global-setup.ts` — shared auth session via Playwright storageState
- Updated `tests/e2e/fixtures/test-fixtures.ts` — storageState with login fallback
- Updated `tests/e2e/playwright.config.ts` — 2 workers, 1 retry, global setup
- Updated `tests/e2e/fixtures/seed.sql` — fixed NULL columns for clubs/fixtures

## Technical Decisions

### Anti-flakiness measures
- **Global auth setup**: Login once, save storageState, reuse across all workers
- **Auth fallback**: `adminPage` fixture detects stale sessions and re-authenticates
- **2 workers**: Minimises SQLite locking from concurrent session refreshes
- **1 retry**: Handles transient database lock failures
- **Auto-waiting assertions**: Use Playwright's `expect()` with built-in timeouts

### Seed data fixes
- Added `website`, `phone_number`, `created_at`, `updated_at` to clubs seed (fixes NULL scan errors)
- Added `notes`, `created_at`, `updated_at` to fixtures seed

## Success Metrics
- [x] All admin pages have at least one "loads without error" test
- [x] Authentication edge cases fully covered (login, logout, session, rate limit)
- [x] Token-based player auth flow tested end-to-end
- [x] No individual test takes longer than 10 seconds
- [x] Full suite runs in under 2 minutes (actual: ~38s)
- [x] 104 tests, 0 flaky across 4 consecutive runs
