# Sprint 010: Advanced E2E & Accessibility

## Overview
- **Goal:** Cover complex multi-step admin workflows (team selection, match results, fixture management), add automated accessibility and responsive testing, and finalize test reporting with Claude-parseable output
- **Duration:** 2 weeks
- **Status:** Planned
- **Work Items:** 3 (WI-054 to WI-056)

## Focus Areas
1. Complex workflow testing (team selection, match results, fixture management)
2. Automated accessibility auditing with axe-core
3. Responsive design testing at mobile and tablet viewports
4. Test reporting and Claude integration for fast feedback loops

## Work Items Summary

| ID | Title | Priority | Complexity | Parallelisable | Dependencies |
|----|-------|----------|------------|----------------|--------------|
| WI-054 | Complex workflow tests | High | L | Yes | WI-051 |
| WI-055 | Accessibility & responsive tests | Medium | M | Yes | WI-048 |
| WI-056 | Test reporting & Claude integration | Medium | S | Yes | WI-047 |

## Execution Strategy

### Phase 1 (all parallel)
- **WI-054**: Complex workflow tests (depends on WI-051 from Sprint 009)
- **WI-055**: Accessibility & responsive tests (depends on WI-048 from Sprint 008)
- **WI-056**: Test reporting & Claude integration (depends on WI-047 from Sprint 008)

All items can be executed in parallel since their dependencies are from previous sprints.

## Technical Impact
- **New files:** ~8 (test specs, parse script, README)
- **Modified files:** 2 (Makefile, package.json)
- **Database changes:** None
- **New dependency:** @axe-core/playwright for accessibility testing

## Phases Addressed
- Phase 8: Automated Testing

## Success Metrics
- [ ] Multi-step workflows (team selection, match results) complete without errors
- [ ] Data flows correctly between related pages in workflow tests
- [ ] No critical accessibility violations on key pages
- [ ] Key pages usable at mobile (375px) and tablet (768px) viewports
- [ ] make test-e2e-results produces Claude-parseable failure output
- [ ] README documents complete testing workflow
- [ ] Full test suite (all 3 sprints) runs in under 5 minutes
