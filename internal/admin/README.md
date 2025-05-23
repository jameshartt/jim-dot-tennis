# Admin Module

The admin module provides a comprehensive administrative interface for the Jim.Tennis application. It has been extracted from the main application file into a dedicated, organized structure.

## Structure

```
internal/admin/
├── README.md          # This documentation
├── handler.go         # Main HTTP handlers for admin routes
├── service.go         # Business logic and data services
└── (future files)     # Additional admin functionality

templates/admin/       # Admin-specific templates
├── players.html       # Player management interface
└── (future templates) # Other admin interfaces
```

## Features

### Current Features

- **Dashboard**: Main admin overview with statistics and quick actions
- **Player Management**: View and manage tennis players (template ready)
- **Fixture Management**: Handle tournament fixtures (coming soon)
- **User Management**: Manage system users and permissions (coming soon)
- **Session Management**: View and manage active user sessions (coming soon)

### Admin Dashboard

The main admin dashboard provides:
- Statistics overview (player count, fixture count, session count)
- Quick action buttons for different admin areas
- Recent login attempts table
- Protected by admin role authentication

## Routes

All admin routes are protected by authentication middleware and require admin role:

- `GET /admin` - Main admin dashboard
- `GET /admin/players` - Player management interface
- `POST /admin/players` - Player management operations
- `GET /admin/fixtures` - Fixture management interface  
- `POST /admin/fixtures` - Fixture management operations
- `GET /admin/users` - User management interface
- `POST /admin/users` - User management operations
- `GET /admin/sessions` - Session viewing interface

## Authentication & Authorization

- All admin routes require valid session authentication
- Users must have `admin` role to access any admin functionality
- User context is passed to all handlers for personalization

## Usage

### In main.go

```go
import "jim-dot-tennis/internal/admin"

// Setup
adminHandler := admin.New(db, templateDir)

// Register routes with authentication
adminHandler.RegisterRoutes(mux, authMiddleware)
```

### Adding New Admin Features

1. Add new handler methods to `handler.go`
2. Add corresponding routes in `RegisterRoutes()`
3. Create service methods in `service.go` for business logic
4. Create templates in `templates/admin/` for UI

## Templates

### Dashboard Template
- Uses `templates/admin_standalone.html` 
- Self-contained with header, navigation, and footer
- Displays user info, stats, and login attempts

### Admin Section Templates
- Located in `templates/admin/` directory
- Follow consistent design patterns
- Include breadcrumb navigation back to dashboard

## Development Status

- ✅ **Dashboard**: Fully functional with mock data
- ✅ **Authentication**: Complete with role-based access
- ✅ **Player Management**: Template created, handlers ready
- 🔄 **Fixture Management**: Basic structure, needs implementation
- 🔄 **User Management**: Basic structure, needs implementation  
- 🔄 **Session Management**: Basic structure, needs implementation

## Future Enhancements

- Real database integration for all admin features
- AJAX/HTMX integration for better UX
- Bulk operations for data management
- Export/import functionality
- Advanced filtering and search
- Admin activity logging
- Real-time updates for dashboard statistics 