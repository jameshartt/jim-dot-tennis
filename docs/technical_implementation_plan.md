# Technical Implementation Plan

This document outlines the technical approach for implementing the Jim.Tennis application, focusing on the architecture, technologies, and implementation strategy.

## Architecture Overview

The application will follow a server-side rendered architecture with minimal client-side JavaScript:

```
┌───────────────────┐       ┌────────────────┐      ┌────────────────┐
│                   │       │                │      │                │
│  Web Browser      │◄─────►│  Go Web Server │◄────►│  SQLite DB     │
│  (HTMX + CSS)     │       │  (Templates)   │      │                │
│                   │       │                │      │                │
└───────────────────┘       └────────────────┘      └────────────────┘
         ▲                          │
         │                          ▼
┌────────┴──────────┐      ┌────────────────┐
│                   │      │                │
│  Service Worker   │      │  Background    │
│  (PWA/Push)       │      │  Jobs          │
│                   │      │                │
└───────────────────┘      └────────────────┘
```

## Technology Stack

1. **Backend**
   - Go (main server language)
   - Chi or Gin for HTTP routing
   - HTML templates with server-side rendering
   - SQLite for database (lightweight, easy deployment)
   - Background task processing for notifications

2. **Frontend**
   - HTMX for dynamic content without heavy JavaScript
   - Minimal vanilla JavaScript for essential client-side functionality
   - CSS for styling (potentially with TailwindCSS)
   - Progressively enhanced for better experience with JavaScript

3. **PWA/Notifications**
   - Service workers for offline capabilities
   - Web Push API for notifications
   - Local storage for client-side state persistence

4. **Infrastructure**
   - Simple deployment as a single binary
   - SQLite database file
   - Static files served directly

## Implementation Phases

### Phase 1: Core Infrastructure (Current Stage)

- [x] Database schema design
- [x] Data models and relationships
- [x] Migration framework
- [x] Web server setup
- [x] Authentication system
- [x] Basic template structure
- [x] Routing architecture
- [x] Hosting on somewhere with ssl on jim.tennis

### Phase 2: Captain Selection Tools

- [x] Division-based access control
- [x] Available player listings
- [x] Team selection interface
- [ ] Selection confirmation and notifications
- [ ] Selection overview across divisions
- [ ] Player status tracking
- [x] Summary of fixture sharing for Whatsapp

### Phase 3: Player Availability Management

- [ ] Player profile views
- [x] Availability form (calendar-based)
- [ ] Availability exception handling
- [ ] General availability settings

### Phase 4: PWA and Push Notifications

- [x] Service worker implementation
- [ ] Push notification pipeline
- [ ] Offline capability for core functions
- [ ] Installation flow
- [ ] Background sync for submissions

### Phase 5: Fixture Result Management

- [ ] Fixture listing and details
- [x] Match result entry/importation from match cards
- [ ] Venue management with maps
- [ ] Fixture reminder system

## Technical Considerations

### Server-Side Rendering Strategy

All primary rendering will occur on the server, with HTMX providing dynamic content updates without page reloads:

```html
<!-- Example of HTMX approach -->
<button hx-post="/availability/update" 
        hx-target="#availability-status" 
        hx-swap="outerHTML"
        hx-vals='{"fixture_id": 123, "status": "Available"}'>
    I'm Available
</button>
```

This approach keeps the client-side code minimal while providing a dynamic, responsive user experience.

### Database Access Pattern

Data access will follow a repository pattern:

```go
// Example repository pattern
type AvailabilityRepository interface {
    FindByPlayerAndFixture(playerID string, fixtureID uint) (*PlayerFixtureAvailability, error)
    UpdateAvailability(playerID string, fixtureID uint, status AvailabilityStatus) error
    // ...
}
```

### Push Notification System

Push notifications will be implemented using the Web Push API:

1. Client subscribes to push notifications
2. Subscription info stored in database
3. Server sends notifications via Web Push API
4. Service worker displays notifications even when app is closed


## Testing Strategy

1. **Unit Testing**
   - Model validation and business logic
   - Repository interactions
   - Service layer functionality

2. **Integration Testing**
   - API endpoints
   - Database interactions
   - Authentication flows

3. **End-to-End Testing**
   - Critical user journeys
   - Form submissions and validations

## Deployment Considerations

1. **Single Binary Deployment**
   - Compile Go application to a single binary
   - Include all static assets
   - Simple startup process

2. **Database Management**
   - SQLite file with regular backups
   - Migration handling for version updates

3. **Hosting Requirements**
   - Small VPS or similar
   - HTTPS for PWA and push notification support
   - Sufficient storage for database and logs 