# Sprint 008: E2E Test Infrastructure

## Overview
- **Goal:** Stand up Playwright browser testing infrastructure with Docker Compose test profile, database seeding, test helpers, and Makefile integration so E2E tests can be written and run with a single `make` command
- **Duration:** 2 weeks
- **Status:** Planned
- **Work Items:** 6 (WI-043 to WI-048)

## Focus Areas
1. Playwright project scaffolding (TypeScript, Chromium-only for speed)
2. Docker Compose test profile (no local Node.js/Go required)
3. Test database seeding (realistic data for all test scenarios)
4. Reusable test helpers (auth, HTMX waiting, navigation)
5. Makefile integration (simple commands for running/filtering/reporting)
6. Infrastructure validation via smoke tests

## Work Items Summary

| ID | Title | Priority | Complexity | Parallelisable | Dependencies |
|----|-------|----------|------------|----------------|--------------|
| WI-043 | Playwright project scaffolding | High | M | No | - |
| WI-044 | Docker Compose test profile | High | M | Yes | WI-043 |
| WI-045 | Test database seeding | High | M | Yes | - |
| WI-046 | Playwright test helpers & fixtures | Medium | S | Yes | WI-043 |
| WI-047 | Makefile test targets | Medium | S | Yes | WI-044 |
| WI-048 | Smoke test to validate infrastructure | High | S | No | WI-044, WI-045, WI-046, WI-047 |

## Execution Strategy

### Phase 1 (parallel)
- **WI-043**: Playwright project scaffolding
- **WI-045**: Test database seeding

### Phase 2 (parallel, after WI-043)
- **WI-044**: Docker Compose test profile
- **WI-046**: Playwright test helpers & fixtures

### Phase 3 (after WI-044)
- **WI-047**: Makefile test targets

### Phase 4 (after all above)
- **WI-048**: Smoke test to validate infrastructure

## Technical Impact
- **New files:** ~15 (test config, helpers, fixtures, seed data, Dockerfile)
- **Modified files:** 2 (docker-compose.yml, Makefile)
- **Database changes:** None (seed data only, no schema changes)

## Phases Addressed
- Phase 8: Automated Testing

## Success Metrics
- [ ] `make test-e2e` runs full suite with single command
- [ ] All 5 smoke tests pass reliably
- [ ] `make test-e2e-failed` re-runs only failures
- [ ] `make test-e2e-results` outputs Claude-parseable JSON
- [ ] Full smoke test run completes in under 30 seconds
- [ ] No local Go or Node.js installation required
