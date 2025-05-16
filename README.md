# Jim-Dot-Tennis

A Progressive Web App (PWA) using Go with server-side rendering (SSR) and minimal client-side JavaScript.

## Project Overview

Jim-Dot-Tennis is designed to be a lightweight web application with:

- **Server-Side Rendering**: Go backend handles all HTML generation
- **Progressive Web App**: Full PWA support including push notifications
- **Minimal JavaScript**: Client-side JS limited to PWA essentials

## Technical Architecture

### Backend

- **Go HTTP Server**: Uses Go's standard library for HTTP handling
- **HTML Templates**: Server-side rendering via Go's `html/template` package
- **Static File Serving**: For PWA essentials like manifest and service worker

### Frontend

- **Pure HTML**: Minimalist approach with server-rendered content
- **PWA Features**: 
  - Service Worker for offline functionality
  - Web App Manifest for installability
  - Push Notification capability

### Project Structure

```
jim-dot-tennis/
├── .cursorrules          # Cursor IDE configuration
├── .gitignore            # Git ignore patterns
├── cmd/
│   └── jim-dot-tennis/   # Main application code
│       └── main.go       # Entry point for the Go application
├── internal/             # Private application and library code
├── static/               # Static assets
│   ├── icon-192.svg      # PWA icon (192x192)
│   ├── icon-512.svg      # PWA icon (512x512)
│   ├── manifest.json     # Web App Manifest for PWA
│   └── service-worker.js # Service Worker for offline & push notifications
└── templates/            # HTML templates
    └── index.html        # Main page template
```

## Features

- **Tennis Ball Branding**: Custom SVG icons in authentic tennis ball style
- **Offline Support**: Service worker enables offline access
- **Push Notifications**: Infrastructure for sending updates to users
- **Mobile-Friendly**: Responsive design via viewport meta tag
- **Installable**: Can be added to home screen on supported devices

## Getting Started

1. **Run the server**:
   ```
   go run cmd/jim-dot-tennis/main.go
   ```

2. **Visit the site**:
   Open `http://localhost:8080` in your browser

## Development Roadmap

- [ ] Add user authentication
- [ ] Implement push notification subscription flow
- [ ] Create database models
- [ ] Develop core application features
- [ ] Add comprehensive offline support
- [ ] Deploy to production

## Technologies Used

- **Go**: Backend server
- **HTML**: Frontend markup
- **SVG**: Custom vector graphics
- **Service Workers**: PWA functionality
- **Web Push Protocol**: For push notifications 