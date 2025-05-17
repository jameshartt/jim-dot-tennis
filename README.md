# Jim-Dot-Tennis

A Progressive Web App (PWA) using Go with server-side rendering (SSR) and minimal client-side JavaScript.

## Project Overview

Jim-Dot-Tennis is designed to be a lightweight web application with:

- **Server-Side Rendering**: Go backend handles all HTML generation
- **Progressive Web App**: Full PWA support including push notifications
- **Minimal JavaScript**: Client-side JS limited to PWA essentials

## Getting Started

### Running Locally

1. **Direct execution**:
   ```
   go run cmd/jim-dot-tennis/main.go
   ```

2. **Visit the site**:
   Open `http://localhost:8080` in your browser

### Using Docker (Recommended)

We provide a complete Docker setup with automatic backups:

1. **Build and run with Docker**:
   ```
   make
   ```
   
   Or manually:
   ```
   docker-compose up -d
   ```

2. **Visit the site**:
   Open `http://localhost:8080` in your browser

For more details on the Docker setup, see [Docker Setup Documentation](docs/docker_setup.md).

## Technical Architecture

### Backend

- **Go HTTP Server**: Uses Go's standard library for HTTP handling
- **HTML Templates**: Server-side rendering via Go's `html/template` package
- **Static File Serving**: For PWA essentials like manifest and service worker
- **SQLite Database**: Lightweight embedded database

### Frontend

- **Pure HTML**: Minimalist approach with server-rendered content
- **PWA Features**: 
  - Service Worker for offline functionality
  - Web App Manifest for installability
  - Push Notification capability

### Project Structure

```
jim-dot-tennis/
├── cmd/
│   ├── jim-dot-tennis/   # Main application code
│   │   └── main.go       # Entry point for the Go application
│   ├── migrate/          # Database migration tool
│   └── scraper/          # Data scraping utilities
├── docs/                 # Documentation files
├── internal/             # Private application and library code
│   └── models/           # Database models
├── migrations/           # Database migrations
├── scripts/              # Utility scripts
│   └── backup-manager.sh # External backup script
├── static/               # Static assets
├── templates/            # HTML templates
├── Dockerfile            # Docker container definition
├── docker-compose.yml    # Docker services configuration
└── Makefile              # Common development commands
```

## Features

- **Tennis Ball Branding**: Custom SVG icons in authentic tennis ball style
- **Offline Support**: Service worker enables offline access
- **Push Notifications**: Infrastructure for sending updates to users
- **Mobile-Friendly**: Responsive design via viewport meta tag
- **Installable**: Can be added to home screen on supported devices
- **Automated Backups**: Daily database backups when running with Docker

## Development Roadmap

- [x] Add database models and migrations
- [ ] Add user authentication
- [ ] Implement push notification subscription flow
- [ ] Create database models
- [ ] Develop core application features
- [ ] Add comprehensive offline support
- [ ] Deploy to production

## Documentation

- [Project Overview](docs/project_overview.md)
- [User Experience Requirements](docs/user_experience_requirements.md)
- [Technical Implementation Plan](docs/technical_implementation_plan.md)
- [Docker Setup](docs/docker_setup.md)

## Technologies Used

- **Go**: Backend server
- **HTML**: Frontend markup
- **SVG**: Custom vector graphics
- **Service Workers**: PWA functionality
- **Web Push Protocol**: For push notifications
- **SQLite**: Embedded database
- **Docker**: Containerization and deployment 