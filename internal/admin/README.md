# Admin Module

The admin module provides a comprehensive administrative interface for the Jim.Tennis application. It has been refactored from a single monolithic handler into a well-organized, domain-specific structure for better maintainability and scalability.

## Architecture

The admin module follows a **composition-based architecture** where the main handler coordinates specialized sub-handlers for different domains. This provides:

- **Single Responsibility**: Each handler focuses on one domain
- **Better Testing**: Individual handlers can be tested in isolation  
- **Easier Maintenance**: Changes to one domain don't affect others
- **Code Reuse**: Common utilities are shared across handlers
- **Scalability**: New admin features can be easily added

## File Structure

```
internal/admin/
├── README.md          # This documentation
├── handler.go         # Main coordination and routing (75 lines)
├── common.go          # Shared utilities and helpers (84 lines)
├── service.go         # Business logic and data services (675 lines)
├── dashboard.go       # Dashboard functionality (67 lines)
├── players.go         # Player management (272 lines)
├── fixtures.go        # Fixture management (148 lines)
├── teams.go           # Team management (140 lines)
├── users.go           # User management (58 lines)
└── sessions.go        # Session management (50 lines)

templates/admin/       # Admin-specific templates
├── players.html       # Player management interface
├── fixtures.html      # Fixture management interface
├── teams.html         # Team management interface
├── team_detail.html   # Individual team details
├── fixture_detail.html # Individual fixture details
└── player_edit.html   # Player editing form
```

## Components

### Main Handler (`handler.go`)
- **Purpose**: Route coordination and sub-handler composition
- **Responsibilities**: 
  - Instantiates and manages domain-specific handlers
  - Registers all admin routes with authentication
  - Handles main `/admin` redirect to dashboard

### Common Utilities (`common.go`)
- **Purpose**: Shared functionality across all handlers
- **Utilities**:
  - `getUserFromContext()` - Authentication helper
  - `parseTemplate()` / `renderTemplate()` - Template management
  - `parseIDFromPath()` - URL ID extraction
  - `renderFallbackHTML()` - Graceful template fallbacks
  - `logAndError()` - Consistent error handling

### Domain Handlers

#### Dashboard Handler (`dashboard.go`)
- **Status**: ✅ **Fully Functional**
- **Features**: 
  - Statistics overview (players, fixtures, teams)
  - Recent login attempts display
  - Admin quick action navigation

#### Players Handler (`players.go`)
- **Status**: ✅ **Fully Functional**
- **Features**:
  - Player listing with search and filtering
  - HTMX-powered real-time filtering
  - Player editing with club assignment
  - Active/inactive status management

#### Fixtures Handler (`fixtures.go`)
- **Status**: ✅ **Basic Structure Complete**
- **Features**:
  - St. Ann's fixtures listing
  - Individual fixture detail pages
  - Template integration with fallbacks

#### Teams Handler (`teams.go`)
- **Status**: ✅ **Basic Structure Complete**
- **Features**:
  - St. Ann's teams listing
  - Individual team detail pages
  - Template integration with fallbacks

#### Users Handler (`users.go`)
- **Status**: 🔄 **Placeholder Implementation**
- **Current**: Simple fallback HTML placeholder
- **Planned**: Full user management interface

#### Sessions Handler (`sessions.go`)
- **Status**: 🔄 **Placeholder Implementation**
- **Current**: Simple fallback HTML placeholder
- **Planned**: Active session monitoring and management

## Routes

All admin routes are protected by authentication middleware and require admin role:

### Main Routes
- `GET /admin` → Redirects to `/admin/dashboard`
- `GET /admin/dashboard` → Dashboard overview

### Player Management
- `GET /admin/players` → Player listing with filtering
- `GET /admin/players/filter` → HTMX endpoint for filtered results
- `GET /admin/players/{id}/edit` → Player edit form
- `POST /admin/players/{id}/edit` → Update player

### Fixture Management
- `GET /admin/fixtures` → Fixture listing
- `GET /admin/fixtures/{id}` → Individual fixture details

### Team Management
- `GET /admin/teams` → Team listing  
- `GET /admin/teams/{id}` → Individual team details

### User & Session Management
- `GET /admin/users` → User management (placeholder)
- `GET /admin/sessions` → Session management (placeholder)

## Features

### Current Features

✅ **Dashboard**: Complete with statistics and login attempts  
✅ **Player Management**: Full CRUD with search, filtering, and editing  
✅ **Fixture Management**: Listing and detail views for St. Ann's fixtures  
✅ **Team Management**: Listing and detail views for St. Ann's teams  
✅ **Authentication**: Role-based access control throughout  
✅ **Template Integration**: Graceful fallbacks when templates are missing  
✅ **Error Handling**: Consistent logging and user-friendly error responses  

### Planned Features

🔄 **User Management**: Complete CRUD for system users  
🔄 **Session Management**: Active session monitoring and control  
🔄 **Bulk Operations**: Multi-select operations for data management  
🔄 **Advanced Search**: Full-text search across all entities  
🔄 **Data Export**: CSV/Excel export functionality  
🔄 **Activity Logging**: Admin action audit trail  

## Usage

### Setup in main.go

```go
import "jim-dot-tennis/internal/admin"

// Initialize admin handler with database and template directory
adminHandler := admin.New(db, templateDir)

// Register all routes with authentication middleware
adminHandler.RegisterRoutes(mux, authMiddleware)
```

### Adding New Admin Features

1. **Create new handler file** (e.g., `reports.go`)
2. **Implement handler struct** with service and templateDir
3. **Add to main handler** in `handler.go` struct and `New()` function  
4. **Register routes** in `RegisterRoutes()` method
5. **Create templates** in `templates/admin/` directory
6. **Add service methods** in `service.go` for business logic

### Example New Handler

```go
// reports.go
type ReportsHandler struct {
    service     *Service
    templateDir string
}

func NewReportsHandler(service *Service, templateDir string) *ReportsHandler {
    return &ReportsHandler{service: service, templateDir: templateDir}
}

func (h *ReportsHandler) HandleReports(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Authentication & Authorization

- **Session Required**: All admin routes require valid authentication
- **Admin Role**: Users must have `admin` role for access
- **User Context**: Current user information available in all handlers
- **Graceful Errors**: Proper HTTP status codes and error messages

## Template Integration

### Template Loading
- Uses `parseTemplate()` helper for consistent loading
- Supports graceful fallbacks when templates are missing
- Templates located in `templates/admin/` directory

### Template Data
- Consistent data structure across handlers
- Always includes current user information
- Domain-specific data passed as needed

## Development

### Testing
Each handler can be tested independently:
```bash
go test ./internal/admin -run TestPlayersHandler
go test ./internal/admin -run TestDashboardHandler
```

### Code Organization
- **Domain Separation**: Each handler focuses on one business domain
- **Shared Utilities**: Common functionality in `common.go`
- **Consistent Patterns**: All handlers follow same structure
- **Error Handling**: Centralized logging and response patterns

