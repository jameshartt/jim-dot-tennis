# Sprint 015: PWA Push Notifications

## Overview

**Goal**: Enable captain-triggered push notifications for team selection and availability reminders, with player opt-in on the availability screen

**Duration**: 2 weeks (dates TBD)

**Status**: Not Started

**Supersedes**: sprint-pwa (carried items from sprint-001, never started)

## Background

The backend push notification infrastructure is **already built**:
- `internal/webpush/` — full service with VAPID key management, subscription CRUD, send methods
- `static/push.js` — client-side subscription/unsubscription logic
- `static/service-worker.js` — push event handler, notification display, click handling
- Database tables: `push_subscriptions`, `vapid_keys` (migration 003)
- HTTP endpoints: `/api/push/subscribe`, `/api/push/unsubscribe`, `/api/vapid-public-key`, `/api/push/test`

**What's missing**: player-linked subscriptions, frontend UI, notification triggers, service worker registration, and cross-platform testing.

This sprint replaces the original sprint-pwa with a focused, user-driven scope: captain-triggered notifications rather than automated background jobs.

## Focus Areas

1. Link push subscriptions to specific players (via fantasy token)
2. Player opt-in UI on the availability screen
3. Captain "Notify Selected Players" button on fixture detail
4. Captain "Remind Availability" button (only players who haven't updated)
5. Cross-platform support: Chrome, Firefox, Safari (iOS 16.4+, macOS), Edge, Android

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies |
|----|-------|----------|------------|--------------|
| WI-088 | Add player association to push subscriptions | High | S | None |
| WI-089 | Player notification opt-in on availability screen | High | M | WI-088 |
| WI-090 | Captain "Notify Selected Players" button | High | M | WI-088, WI-089 |
| WI-091 | Captain "Remind Availability" button | High | M | WI-088, WI-089 |
| WI-092 | Cross-platform push notification compatibility | High | M | WI-089 |

## Execution Strategy

### Phase 1: Foundation
- **WI-088**: Database migration + backend changes to link subscriptions to players

### Phase 2: Player Opt-In
- **WI-089**: Service worker registration, notification toggle on availability screen (depends on WI-088)

### Phase 3: Captain Controls (Parallel)
Execute after WI-089 completes (need at least one subscribed player to test):
- **WI-090**: "Notify Selected Players" button (depends on WI-088, WI-089)
- **WI-091**: "Remind Availability" button (depends on WI-088, WI-089)
- **WI-092**: Cross-platform compatibility (depends on WI-089)

## Critical Path

```
WI-088 → WI-089 → (WI-090 + WI-091 + WI-092)
```

## Technical Impact

### New Files (~2-4 files)
- Migration: `add_player_token_to_push_subscriptions` (up + down)
- Possibly a notification template partial for the availability page

### Modified Files (~8-10 files)
- `internal/webpush/webpush.go` — new player-targeted methods
- `internal/webpush/handlers.go` — accept player_token in subscribe
- `internal/admin/fixtures.go` — notify + remind endpoints
- `internal/admin/handler.go` — register new routes
- `templates/players/availability.html` — notification toggle UI
- `templates/admin/fixture_detail.html` — captain buttons
- `templates/layout.html` — service worker registration script
- `static/push.js` — pass player_token, iOS detection
- `static/service-worker.js` — platform compatibility
- `static/manifest.json` — PNG icon fallbacks, completeness

## Key Design Decisions

1. **Player token, not player ID**: Subscriptions are linked via the fantasy token (used in `/my-availability/{token}` URLs) because the availability screen authenticates via token, not a numeric ID.

2. **Captain-triggered, not automated**: Unlike the original sprint-pwa which planned background cron jobs, this sprint uses manual captain buttons. This is simpler, gives captains control, and avoids spam.

3. **iOS requires PWA mode**: Push notifications on iOS Safari only work when the app is installed as a PWA (Add to Home Screen). The UI must detect this and guide users accordingly.

## Success Metrics

- [ ] Players can enable/disable push notifications from their availability page
- [ ] Captains can notify selected players with one button press
- [ ] Captains can remind only players who haven't updated availability
- [ ] Notifications work on Chrome, Firefox, Safari (iOS + macOS), Edge, and Android
- [ ] Notification clicks deep-link back to the correct page

## Work Items Tracking

### Completed
- None

### In Progress
- None

### Not Started
- WI-088: Add player association to push subscriptions
- WI-089: Player notification opt-in on availability screen
- WI-090: Captain "Notify Selected Players" button
- WI-091: Captain "Remind Availability" button
- WI-092: Cross-platform push notification compatibility
