# Sprint 001: Player Experience & Notifications MVP

## Overview

**Goal**: Complete core player availability features and implement push notification system to enable MVP launch

**Duration**: 2 weeks (Feb 3 - Feb 17, 2026)

**Status**: Not started

## Focus Areas

1. Player availability management
2. Push notification pipeline
3. PWA capabilities
4. Captain selection improvements

## Work Items Summary

| ID | Title | Priority | Complexity | Parallelisable | Dependencies |
|----|-------|----------|------------|----------------|--------------|
| WI-001 | Implement player profile view page | High | M | ✅ | None |
| WI-002 | Add general availability preferences | Medium | L | ✅ | None |
| WI-003 | Implement availability exception handling | Medium | M | ❌ | WI-002 |
| WI-004 | Build push notification subscription flow | High | M | ✅ | None |
| WI-005 | Implement push notification sending service | High | M | ❌ | WI-004 |
| WI-006 | Add selection confirmation notifications | High | S | ❌ | WI-005 |
| WI-007 | Create availability reminder notification system | High | M | ❌ | WI-005 |
| WI-008 | Build captain selection overview dashboard | High | L | ✅ | None |
| WI-009 | Implement PWA installation prompt flow | Medium | S | ✅ | None |
| WI-010 | Add offline availability update with background sync | Medium | M | ✅ | None |
| WI-011 | Create fixture details and listing page | Medium | M | ✅ | None |
| WI-012 | Add WhatsApp sharing for fixture details | Medium | S | ✅ | WI-011 |

## Execution Strategy

### Phase 1: Foundation (Parallel Execution)
Execute these items in parallel as they have no dependencies:

- **WI-001**: Player profile view page
- **WI-002**: General availability preferences
- **WI-004**: Push notification subscription flow
- **WI-008**: Captain selection overview dashboard
- **WI-009**: PWA installation prompt flow
- **WI-010**: Offline availability with background sync
- **WI-011**: Fixture details and listing page

**Estimated Completion**: 3-5 days with parallel agents

### Phase 2: Notification Pipeline (Sequential)
Execute after WI-004 completes:

- **WI-005**: Push notification sending service (depends on WI-004)
- **WI-006**: Selection confirmation notifications (depends on WI-005)
- **WI-007**: Availability reminder system (depends on WI-005)

**Estimated Completion**: 2-3 days

### Phase 3: Enhancements (Sequential/Parallel)
Execute after prerequisites:

- **WI-003**: Availability exception handling (depends on WI-002)
- **WI-012**: WhatsApp sharing (depends on WI-011)

**Estimated Completion**: 1-2 days

## Technical Impact

### New Files (~15-20 files)
- Templates: player profile, availability preferences, selection overview, fixture listing, etc.
- Handlers: profile, preferences, notification service, fixtures
- Jobs: availability reminder cron job
- Client-side: push notifications, PWA install, WhatsApp share, offline sync

### Modified Files (~8-12 files)
- Handler registration (admin, players)
- Service worker
- Models and repositories (availability)
- Main app (routing, job scheduler)

### Database Changes
- 1 new table: `player_availability_preferences`
- Possible additions to existing `player_availability` table

## Phases Addressed

- ✅ Phase 2: Captain Selection Tools (WI-006, WI-008)
- ✅ Phase 3: Player Availability Management (WI-001, WI-002, WI-003)
- ✅ Phase 4: PWA and Push Notifications (WI-004, WI-005, WI-007, WI-009, WI-010)
- ✅ Phase 5: Fixture Result Management (WI-011, WI-012)

## Success Metrics

- [ ] Players can view their profile and upcoming fixtures
- [ ] Players can set general availability preferences
- [ ] Players receive push notifications for selections and reminders
- [ ] Captains can see hierarchical selection overview
- [ ] App can be installed as PWA on mobile devices
- [ ] Availability updates work offline and sync when online
- [ ] Fixture information can be shared via WhatsApp

## MVP Readiness

This sprint addresses critical gaps for MVP launch:

1. ✅ Player availability submission (enhanced with preferences)
2. ✅ Captain team selection workflow (enhanced with overview)
3. ✅ Fixture viewing and sharing (new functionality)
4. ✅ Push notifications for selection and reminders (new functionality)
5. ✅ WhatsApp integration for sharing (new functionality)

## Next Steps

1. Review and prioritize work items
2. Assign work items to AI agents or developers
3. Execute Phase 1 items in parallel
4. Monitor progress and adjust as needed
5. Conduct testing after each phase
6. Prepare for MVP launch after sprint completion
