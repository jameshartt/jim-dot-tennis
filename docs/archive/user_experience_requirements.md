# User Experience Requirements

## Implementation Status Key

- [x] **Done** -- Implemented and available
- [ ] **Planned** -- Not yet implemented

## Core User Experience Principles

1. **Simplicity First**
   - All interfaces must be intuitive enough for users with minimal technical proficiency
   - Reduce cognitive load by limiting options and focusing on core functionality
   - Use progressive disclosure for advanced features

2. **Mobile-Optimized Design**
   - Design for mobile-first, ensuring all features work well on smaller screens
   - Touch-friendly interface elements (buttons, inputs, etc.)
   - Responsive layouts that adapt to any device size

3. **Minimal Friction**
   - Reduce the number of steps required to complete common tasks
   - Automate repetitive actions wherever possible
   - Maintain state to prevent data loss and redundant input

## Key User Journeys

### 1. Player Availability Management

**Priority: High**

Players need an extremely simple way to:
- [x] View upcoming fixtures they're eligible for (Sprint 001 WI-011)
- [x] Update their availability status with one-tap Available/Unavailable buttons
- [ ] See when they've been selected for a team (selection confirmation notifications -- Sprint PWA)
- [ ] Receive reminders to update availability (availability reminder notifications -- Sprint PWA)

**Requirements:**
- [x] One-tap availability updates (Available/Unavailable)
- [x] Fixture listing with details (date, time, venue, opponents) (Sprint 001 WI-011)
- [x] Player profile views (Sprint 001 WI-001)
- [x] Mark Time Away -- allows players to mark date ranges when they are unavailable (Sprint 001 WI-003)
- [ ] Push notifications for reminders (Sprint PWA)
- [x] WhatsApp-friendly sharing of fixture details (Sprint 001 WI-012)

**UX Decisions:**
- General availability preferences were intentionally removed from the UI (Sprint 001). The decision was that surfacing general preferences discourages players from providing weekly updates, which are more valuable for captains.
- The feature for marking date-range unavailability is named "Mark Time Away" rather than "availability exceptions" for clarity.

### 2. Captain Team Selection

**Priority: High**

Captains need streamlined tools to:
- [x] View all eligible and available players
- [x] Select players for upcoming fixtures
- [x] See selections made by other captains
- [x] Manage players who haven't updated their availability

**Requirements:**
- [x] Filterable player lists by availability status
- [x] Visual distinction of divisions (Division 1 selections visible to Division 2+)
- [x] Captain team selection interface
- [ ] Automatic notifications to selected players (selection confirmation notifications -- Sprint PWA)
- [x] Ability to override availability for exceptional cases

### 3. Fixture Management

**Priority: Medium**

Players and captains need to:
- [x] View upcoming fixture details (location, time, opponents) (Sprint 001 WI-011)
- [x] Access directions to match venues with map integration (Sprint 002 WI-014)
- [x] Share fixture details via WhatsApp using Web Share API with WhatsApp fallback (Sprint 001 WI-012)
- [x] Add fixtures to personal calendars via iCal feed generation (Sprint 002 WI-014)
- [ ] Record and view match results

**Requirements:**
- [x] Venue pages with maps and directions (Sprint 002 WI-014)
- [x] iCal calendar feed generation (Sprint 002 WI-014)
- [x] One-tap sharing to WhatsApp via Web Share API with WhatsApp fallback (Sprint 001 WI-012)
- [ ] Simple match score entry system

### 4. Communication & Notifications

**Priority: High**

The system needs to provide:
- [ ] Push notifications for important events (Sprint PWA)
- [ ] Easy opt-in/opt-out preferences (Sprint PWA)
- [x] Integration with existing communication channels (WhatsApp sharing)
- [ ] Timely reminders about upcoming responsibilities (Sprint PWA)

**Requirements:**
- [ ] Configurable notification preferences (Sprint PWA)
- [ ] Push notification channel (Sprint PWA)
- [x] WhatsApp sharing capabilities (Sprint 001 WI-012)
- [ ] Smart notification timing based on fixture schedules (Sprint PWA)

### 5. Admin Management

**Priority: Medium**

Administrators need tools to:
- [x] Access an organized dashboard with quick actions (Sprint 003 WI-017)
- [x] Edit divisions (Sprint 003 WI-018)
- [x] Manage users (Sprint 003 WI-019)
- [x] Manage sessions (Sprint 003 WI-020)
- [x] Filter data by season across the admin UI (Sprint 002 WI-013)

## Technical Implementation Considerations

1. **Server-Side Rendering with HTMX**
   - [x] Use HTMX for dynamic content updates without full JavaScript frameworks
   - [x] Minimize client-side processing
   - [x] Ensure core functionality works without JavaScript when possible
   - [x] Use progressive enhancement for advanced features

2. **PWA for Push Notifications** (Sprint PWA -- Pending)
   - [ ] Implement service workers for offline capability
   - [ ] Configure push notification pipeline
   - [ ] Create installation prompts for mobile users
   - [ ] Optimize for background sync of availability updates

3. **WhatsApp Integration**
   - [x] Generate shareable links for WhatsApp (Sprint 001 WI-012)
   - [x] Create formatted templates for fixture information
   - [x] Uses Web Share API with WhatsApp fallback
   - [x] Ensure messages are formatted appropriately for WhatsApp

## Accessibility Requirements

1. **Inclusive Design**
   - High contrast text and visual elements
   - Touch targets sized appropriately (minimum 44x44px)
   - Keyboard navigable interfaces
   - Screen reader compatibility

2. **Low-Tech Access Options**
   - Alternative methods for players without smartphones
   - Email notification options
   - Printable fixture lists and team sheets

## Minimum Viable Product (MVP) Features

The MVP features below reflect current implementation status:

1. [x] **Player availability submission** -- One-tap Available/Unavailable updates, Mark Time Away for date ranges
2. [x] **Captain team selection workflow** -- Full captain selection interface with filtered player lists
3. [x] **Fixture viewing and sharing** -- Fixture details, listing, venue pages with maps, iCal feeds, WhatsApp sharing via Web Share API
4. [ ] **Push notifications for selection and reminders** -- Pending (Sprint PWA): push notification pipeline, selection confirmations, availability reminders
5. [x] **WhatsApp integration for sharing key information** -- Implemented via Web Share API with WhatsApp fallback
6. [x] **Player profiles** -- Player profile views (Sprint 001 WI-001)
7. [x] **Admin management** -- Dashboard with quick actions, division editing, user management, session management, season filtering

## Pending Features (Sprint PWA)

The following features are planned for the PWA sprint and are not yet implemented:

1. **Push notification pipeline** -- Service worker registration, VAPID key management, subscription handling
2. **PWA installation prompt** -- Guided prompt for users to install the app on their home screen
3. **Offline availability with background sync** -- Allow availability updates while offline, syncing when connectivity returns
4. **Selection confirmation notifications** -- Push notification sent to players when they are selected for a fixture
5. **Availability reminder notifications** -- Timed push notifications reminding players to update their availability before fixture deadlines
