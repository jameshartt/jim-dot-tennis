# Sprint 007: Communication & Project Maintenance

## Goal
Expand communication channels beyond push notifications with email support and user preferences, update project documentation to reflect current state, and build tooling for season transitions.

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-038 | Email notification infrastructure | Medium | M | None | Yes |
| WI-039 | Notification preferences management UI | Medium | M | WI-038 | No |
| WI-040 | Update CLAUDE.md to reflect current project state | High | S | None | Yes |
| WI-041 | Season transition and data management tools | Medium | M | None | Yes |

## Execution Plan

### Phase 1 - Parallel (WI-038, WI-040, WI-041)
These three items have no dependencies and can be worked on simultaneously:
- **WI-038**: Email notification infrastructure
- **WI-040**: Update CLAUDE.md
- **WI-041**: Season transition tooling

### Phase 2 - Sequential (WI-039)
Depends on WI-038 (notification preferences needs email infrastructure in place):
- **WI-039**: Notification preferences management UI

## Key Technical Decisions

- **Email provider**: SMTP for flexibility, or Resend/Postmark API for simplicity. Both approaches supported.
- **Notification preferences**: New `notification_preferences` table with per-channel, per-type toggles.
- **Season transition**: Copy-forward approach for divisions and teams, with automatic week generation.
- **CLAUDE.md**: Comprehensive update covering Go 1.25, service refactor, new routes, Makefile targets.

## Relationship to Sprint PWA
Sprint PWA covers push notification subscription (WI-004) and sending (WI-005). Sprint 007 complements this with:
- Email as an alternative/additional channel (WI-038)
- User control over which channels they use (WI-039)

## Phase Alignment
- Phase 4: PWA and Push Notifications (WI-038, WI-039)
- Phase 6: Admin Tooling and Dashboard (WI-041)
- Phase 7: Code Quality and Go Tooling (WI-040)
