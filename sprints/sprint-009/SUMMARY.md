# Sprint 009: Core E2E Test Suite

## Overview
- **Goal:** Test all critical user flows including authentication, admin dashboard and CRUD pages, player-facing pages, and data-heavy reporting pages with comprehensive Playwright browser tests
- **Duration:** 2 weeks
- **Status:** Planned
- **Work Items:** 5 (WI-049 to WI-053)

## Focus Areas
1. Authentication flow testing (login, logout, session, rate limiting)
2. Admin dashboard and navigation verification
3. Admin CRUD page coverage for all entities
4. Player-facing pages with token-based auth
5. Points table and standings with data filtering

## Work Items Summary

| ID | Title | Priority | Complexity | Parallelisable | Dependencies |
|----|-------|----------|------------|----------------|--------------|
| WI-049 | Authentication flow tests | High | M | Yes | WI-048 |
| WI-050 | Admin dashboard & navigation tests | High | M | Yes | WI-048 |
| WI-051 | Admin CRUD page tests | High | L | Yes | WI-048 |
| WI-052 | Player-facing page tests | High | M | Yes | WI-048 |
| WI-053 | Points table & standings tests | Medium | S | Yes | WI-048 |

## Execution Strategy

### Phase 1 (all parallel - all depend only on WI-048)
- **WI-049**: Authentication flow tests
- **WI-050**: Admin dashboard & navigation tests
- **WI-051**: Admin CRUD page tests
- **WI-052**: Player-facing page tests
- **WI-053**: Points table & standings tests

All items can be executed in parallel since they only share a dependency on the Sprint 008 infrastructure validation (WI-048).

## Technical Impact
- **New files:** ~15 (test spec files across admin/ and players/ directories)
- **Modified files:** 0
- **Database changes:** None

## Phases Addressed
- Phase 8: Automated Testing

## Success Metrics
- [ ] All admin pages have at least one "loads without error" test
- [ ] Authentication edge cases fully covered (login, logout, session, rate limit)
- [ ] Token-based player auth flow tested end-to-end
- [ ] HTMX interactions tested (filters, form submissions)
- [ ] No individual test takes longer than 10 seconds
- [ ] Full suite runs in under 2 minutes
