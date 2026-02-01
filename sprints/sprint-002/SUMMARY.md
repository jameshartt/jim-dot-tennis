# Sprint 002: Venue Infrastructure & Season Filtering

## Overview

**Goal**: Add venue/club data infrastructure with scraping, fix active season filtering across all admin views, improve player-facing fixture experience, and add CourtHive stack management

**Duration**: 2 days (Jan 31 - Feb 1, 2026)

**Status**: Completed (closed 2026-02-01)

## Focus Areas

1. Venue and club data enrichment
2. Active season filtering consistency
3. Player-facing fixture UX
4. CourtHive integration tooling

## Work Items Summary

| ID | Title | Priority | Complexity | Status |
|----|-------|----------|------------|--------|
| WI-013 | Fix active season filtering across admin UI | High | M | Completed |
| WI-014 | Add venue/club infrastructure and data enrichment | High | L | Completed |
| WI-015 | Add CourtHive stack management to Makefile | Medium | S | Completed |
| WI-016 | Improve player-facing fixture UX | Medium | S | Completed |

## Technical Impact

### New Files (~18 files)
- Club scraper service, venue resolver, iCal generator
- Admin handlers and templates for clubs
- Player venue page with maps
- Venue override model and repository
- Database migrations (018, 019)
- Club data seed SQL and import CLI tool

### Modified Files (~13 files)
- Admin handler registration, service, fixtures, teams, players
- Models, club and fixture repositories
- Player availability handler, service, templates
- Makefile

### Database Changes
- Migration 018: Enrich clubs with venue data (address, postcode, lat/lng, facilities, etc.)
- Migration 019: Add venue overrides table for fixture-specific venue changes

## Work Items Tracking

### Completed

- **WI-013**: Fix active season filtering across admin UI (2026-01-31)
  - Fixed duplicate divisions and teams on fixtures page
  - Grouped teams by season with section headings
  - Compacted season setup page with single-line team rows
  - Filtered team dropdowns by division on Create New Fixture modal
  - Filtered dashboard teams count by active season
  - Filtered teams and divisions by active season on players page

- **WI-014**: Add venue/club infrastructure and data enrichment (2026-02-01)
  - Built BHPLTA club scraper for venue data
  - Created admin club list, detail, and import pages
  - Added venue resolver for determining fixture venues
  - Built player-facing venue page with map and directions
  - Added iCal calendar feed generation
  - Created venue override system for fixture-specific changes
  - Added 2 database migrations and seed data script

- **WI-015**: Add CourtHive stack management to Makefile (2026-01-31)
  - Added make targets for TMX frontend build
  - Added docker compose management targets (courthive-up, courthive-down, etc.)

- **WI-016**: Improve player-facing fixture UX (2026-02-01)
  - Enhanced player fixture list layout and information hierarchy
  - Integrated venue information into fixture flow

## Sprint Retrospective

**Outcome**: 4 of 4 work items completed. All planned scope delivered.

**Key deliverable**: Full venue/club infrastructure from scraping through to player-facing maps and directions, plus consistency fixes across the admin UI for multi-season support.
