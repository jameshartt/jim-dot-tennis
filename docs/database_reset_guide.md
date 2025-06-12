# Database Reset Guide

This document provides a complete guide for resetting the database state on the DigitalOcean server and importing fresh data.

## Overview

The database reset process involves:
1. Stopping the application
2. Removing the existing database volume
3. Starting the application to recreate the volume and run migrations
4. Running the populate-db script to import all data
5. Restarting the application

## Prerequisites

- SSH access to the DigitalOcean droplet (root@144.126.228.64)
- Local development environment with Go installed
- CSV fixture files in `pdf_output/` directory
- Players HTML file in `players-import/players.html`

## Complete Reset Process

### Step 1: Stop the Application

```bash
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose down"
```

This stops all containers and removes the network.

### Step 2: Remove the Database Volume

```bash
# Check existing volumes
ssh root@144.126.228.64 "docker volume ls | grep jim-dot-tennis"

# Remove the data volume (this deletes all database data)
ssh root@144.126.228.64 "docker volume rm jim-dot-tennis-data"
```

**⚠️ WARNING**: This permanently deletes all database data including:
- All fixtures and results
- Player records
- User accounts
- Push notification subscriptions
- All application data

### Step 3: Clean Up Migration Files (if needed)

```bash
# Check for duplicate migration files
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && ls -la migrations/ | grep -E 'down.sql|up.sql' | sort"

# Remove any duplicate migration files
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && find migrations/ -name '*003_messages_notifications*' -delete"
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && rm -f migrations/004_add_push_notification_tables.*"
```

### Step 4: Restart Application to Recreate Volume

```bash
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose up -d"
```

This creates a new empty database volume and runs migrations.

### Step 5: Build and Transfer populate-db Tool

```bash
# Build the populate-db tool locally
go build -o populate-db ./cmd/populate-db

# Transfer the binary to the server
scp populate-db root@144.126.228.64:/opt/jim-dot-tennis/

# Transfer data files
scp -r pdf_output/ root@144.126.228.64:/opt/jim-dot-tennis/
scp -r players-import/ root@144.126.228.64:/opt/jim-dot-tennis/
```

### Step 6: Stop Application and Run Data Import

```bash
# Stop the application to avoid database conflicts
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose stop app"

# Run the populate-db script
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && chmod +x populate-db && ./populate-db -db-path /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db -csv-dir pdf_output -players-file players-import/players.html -verbose"
```

### Step 7: Start Application

```bash
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose up -d"
```

### Step 8: Verify Success

```bash
# Check application status
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose ps"

# Check application logs
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose logs app | tail -10"

# Verify database content
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker exec jim-dot-tennis sqlite3 /app/data/tennis.db 'SELECT COUNT(*) FROM fixtures;'"
```

## What Gets Imported

The populate-db script imports:

### Seasons and Weeks
- **2025 Season**: April 1 - September 30, 2025
- **18 Weeks**: Weekly schedule from April to August

### League Structure
- **Brighton & Hove Parks Tennis League**
- **4 Divisions**: Division 1-4 with different play days
  - Division 1: Thursday
  - Division 2: Tuesday  
  - Division 3: Wednesday
  - Division 4: Monday

### Clubs and Teams
- **10 Clubs**: Dyke, Hove, King Alfred, Queens, Saltdean, Preston Park, St Ann's, Blakers, Hollingbury, Park Avenue
- **42 Teams**: Multiple teams per club across different divisions

### Fixtures
- **402 Fixtures**: Complete season schedule for all divisions
- **Home and Away**: Each team plays each other twice
- **18-Week Schedule**: Fixtures spread across the season

### Players
- **88 Players**: All St Ann's club members imported from HTML file
- **Complete Profiles**: Names, emails, phone numbers generated

## Data Summary

After successful import, you should have:
- 1 Season (2025)
- 18 Weeks
- 1 League (Brighton & Hove Parks)
- 4 Divisions
- 10 Clubs
- 42 Teams
- 402 Fixtures
- 88 Players

## Troubleshooting

### Migration Errors
If you see duplicate migration file errors:
```bash
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && find migrations/ -name '*duplicate_name*' -delete"
```

### Database Connection Issues
If the populate script can't connect to the database:
```bash
# Check if the volume exists
ssh root@144.126.228.64 "docker volume inspect jim-dot-tennis-data"

# Check database file permissions
ssh root@144.126.228.64 "docker exec jim-dot-tennis ls -la /app/data/"
```

### Application Won't Start
If the application fails to start after reset:
```bash
# Check logs for specific errors
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose logs app"

# Rebuild the container if needed
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose build --no-cache app"
```

### Incomplete Data Import
If some data is missing:
```bash
# Check what was imported
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker exec jim-dot-tennis sqlite3 /app/data/tennis.db '.tables'"

# Re-run populate script with verbose logging
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && ./populate-db -db-path /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db -csv-dir pdf_output -players-file players-import/players.html -verbose -dry-run"
```

## Quick Reset Commands

For future reference, here's the complete reset in one script:

```bash
#!/bin/bash
# Quick database reset script

echo "🛑 Stopping application..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose down"

echo "🗑️ Removing database volume..."
ssh root@144.126.228.64 "docker volume rm jim-dot-tennis-data"

echo "🧹 Cleaning migration files..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && find migrations/ -name '*003_messages_notifications*' -delete"
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && rm -f migrations/004_add_push_notification_tables.*"

echo "🚀 Starting application..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose up -d"

echo "⏳ Waiting for startup..."
sleep 10

echo "🔧 Building populate tool..."
go build -o populate-db ./cmd/populate-db

echo "📤 Transferring files..."
scp populate-db root@144.126.228.64:/opt/jim-dot-tennis/
scp -r pdf_output/ root@144.126.228.64:/opt/jim-dot-tennis/
scp -r players-import/ root@144.126.228.64:/opt/jim-dot-tennis/

echo "🛑 Stopping app for import..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose stop app"

echo "📊 Importing data..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && chmod +x populate-db && ./populate-db -db-path /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db -csv-dir pdf_output -players-file players-import/players.html -verbose"

echo "🚀 Starting application..."
ssh root@144.126.228.64 "cd /opt/jim-dot-tennis && docker-compose up -d"

echo "✅ Database reset complete!"
```

## Security Notes

- The database reset removes all user accounts and authentication data
- Push notification subscriptions are lost and need to be re-registered
- Any custom data or manual entries will be lost
- Always backup important data before running a reset

## Last Reset

**Date**: June 7, 2025  
**Go Version**: 1.24.1  
**Status**: ✅ Successful  
**Data Imported**: 
- 402 fixtures across 4 divisions
- 88 St Ann's players
- Complete 2025 season structure 