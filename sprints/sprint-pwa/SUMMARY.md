# Sprint PWA: Push Notifications & PWA Enhancement

## Overview

**Goal**: Implement the full push notification pipeline and PWA capabilities to enable real-time player communication and offline support

**Duration**: 2 weeks (dates TBD)

**Status**: Not Started

**Type**: Special sprint (standalone, not part of the numbered sprint sequence)

**Carried from**: Sprint 001 - these items were de-prioritised from the Player Experience & Notifications MVP sprint but remain important for making the app stand out.

## Focus Areas

1. Push notification pipeline (subscribe, send, templates)
2. Event-driven notifications (selection confirmations, availability reminders)
3. PWA installation and offline support

## Work Items Summary

| ID | Title | Priority | Complexity | Parallelisable | Dependencies |
|----|-------|----------|------------|----------------|--------------|
| WI-004 | Build push notification subscription flow | High | M | Yes | None |
| WI-005 | Implement push notification sending service | High | M | No | WI-004 |
| WI-006 | Add selection confirmation notifications | High | S | No | WI-005 |
| WI-007 | Create availability reminder notification system | High | M | No | WI-005 |
| WI-009 | Implement PWA installation prompt flow | Medium | S | Yes | None |
| WI-010 | Add offline availability update with background sync | Medium | M | Yes | None |

## Execution Strategy

### Phase 1: Foundation (Parallel Execution)
Execute these items in parallel as they have no dependencies:

- **WI-004**: Push notification subscription flow (client-side)
- **WI-009**: PWA installation prompt flow
- **WI-010**: Offline availability with background sync

### Phase 2: Notification Pipeline (Sequential)
Execute after WI-004 completes:

- **WI-005**: Push notification sending service (depends on WI-004)

### Phase 3: Event Notifications (Parallel after WI-005)
Execute after WI-005 completes:

- **WI-006**: Selection confirmation notifications (depends on WI-005)
- **WI-007**: Availability reminder system (depends on WI-005)

## Critical Path

WI-004 -> WI-005 -> (WI-006 + WI-007)

WI-009 and WI-010 are fully independent and can be executed at any time.

## Technical Impact

### New Files (~8-12 files)
- Client JS: push notification subscription, PWA install prompt, offline availability sync
- Server: notification service, notification templates, availability reminder job
- Templates: PWA install prompt component

### Modified Files (~5-8 files)
- Service worker (push events, background sync, offline caching)
- Layout template (notification + PWA scripts)
- Fixture handlers (notification hooks on selection)
- Main app (job scheduler for reminders)

## Success Metrics

- [ ] Users can subscribe to and receive push notifications
- [ ] Players receive notification when selected for a fixture
- [ ] Players receive availability reminders before deadlines
- [ ] App can be installed as PWA on mobile devices
- [ ] Availability updates work offline and sync when online

## Work Items Tracking

### Completed
- None

### In Progress
- None

### Not Started
- WI-004: Push notification subscription flow
- WI-005: Push notification sending service
- WI-006: Selection confirmation notifications
- WI-007: Availability reminder notification system
- WI-009: PWA installation prompt flow
- WI-010: Offline availability with background sync
