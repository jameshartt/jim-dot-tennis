# Sprint 024: Documentation and Onboarding for New Clubs

## Overview

**Goal**: A volunteer at another BHPLTA club, with no familiarity with the project and modest technical confidence, can install it, set it up, keep it backed up and upgraded, and explain it to their captains and players - using only what ships with the project. Close with version 1.0.0.

**Duration**: dates TBD

**Status**: Planned

**Programme**: Part of the club-readiness programme (Sprints 019-024). Agreed constraints: BHPLTA clubs only; each club hosts its own instance; the original author has no access to any other club's instance; passkeys by default with passwords as an optional fallback; a real captain role; SQLite only; CourtHive bundled as an opt-in so clubs can run their own championships.

## Focus Areas

1. Install, upgrade, backup and troubleshooting guides written for a first-time operator
2. Guides for administrators, captains and players, including running club championships
3. Clear out stale documents so what remains is true
4. A cold install by a reader with only the docs, then fix what tripped them

## Work Items Summary

| ID | Title | Priority | Complexity | Model | Dependencies |
|----|-------|----------|------------|-------|--------------|
| WI-148 | Operator guides: install, upgrade, back up, restore, troubleshoot | High | M | Opus 5.5 | None |
| WI-149 | User guides and in-app help: administrators, captains, players, tournaments | High | M | Sonnet 5.5 | None |
| WI-150 | Clear out stale documentation; README for a prospective club | Medium | S | Sonnet 5.5 | None |
| WI-151 | Cold install test, fixes, and version 1.0.0 | High | M | Sonnet 5.5 | WI-148, WI-149, WI-150 |

## Recommended Models

Each work item's JSON carries a `recommended_model` with the reason. In short:

- **WI-148** - Opus 5.5: Accuracy and judgement about what a newcomer does not know matter more than volume.
- **WI-149** - Sonnet 5.5: Clear explanatory writing from well-defined product behaviour.
- **WI-150** - Sonnet 5.5: Editorial clean-up with a clear target state.
- **WI-151** - Sonnet 5.5: The tester should resemble a newcomer; fixes found may be routed to a stronger model as needed.

## Execution Strategy

### Phase 1: Write (parallel)
- **WI-148** - Operator guides: install, upgrade, back up, restore, troubleshoot
- **WI-149** - User guides and in-app help: administrators, captains, players, tournaments
- **WI-150** - Clear out stale documentation; README for a prospective club

### Phase 2: Test the docs
- **WI-151** - Cold install test, fixes, and version 1.0.0

## Key Design Decisions

1. The docs assume a small rented server and a domain name, and say so with rough cost and specification. They do not assume Docker experience.
2. Player-facing wording follows the project's positive-framing principle.
3. The docs state plainly what the club owns: its server, its data, its accounts, and that nobody else - including the author - has access.
4. Author-only history (CourtHive migration runbooks, old plans) moves to docs/archive/ rather than being deleted.
5. The cold install is done by a deliberately less capable model with no project context, as a stand-in for a newcomer.

## What Is NOT In This Sprint

- Video walkthroughs
- Translations
- A hosted documentation site (plain Markdown in the repository, plus in-app help links)
