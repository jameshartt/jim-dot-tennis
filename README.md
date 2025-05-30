# Jim Dot Tennis

A comprehensive tennis league management system built with Go, designed to handle tennis league operations including player management, team organization, fixture scheduling, and match results tracking.

## Features

### Core Functionality
- **Season Management**: Create and manage tennis seasons with configurable date ranges
- **Week Management**: Organize fixtures into weekly schedules with automatic date calculations
- **League & Division Management**: Support for multiple leagues (Parks, Club) with multiple divisions
- **Club & Team Management**: Organize teams under clubs with configurable team limits per division
- **Player Management**: Track players with club associations and team memberships
- **Fixture Scheduling**: Schedule matches between teams with week-based organization
- **Match Results**: Track individual matchups (Men's, Women's, Mixed doubles) within fixtures
- **Captain Roles**: Support for team captains and day captains with different permissions

### Technical Features
- **Database Migrations**: Automated schema management with up/down migrations
- **Repository Pattern**: Clean data access layer with comprehensive interfaces
- **CSV Import**: Bulk import of fixture data from PDF-extracted CSV files
- **Dry Run Support**: Test data imports without making database changes
- **Comprehensive Logging**: Detailed logging with configurable verbosity levels

## Architecture

### Database Schema
The system uses a relational database with the following key entities:

- **Seasons**: Time periods for league play (e.g., "2025 Season")
- **Weeks**: Weekly organization within seasons (Week 1, Week 2, etc.)
- **Leagues**: Competition types (Parks League, Club League)
- **Divisions**: Skill/competition levels within leagues
- **Clubs**: Tennis clubs that field teams
- **Teams**: Competition units representing clubs in divisions
- **Players**: Individual participants associated with clubs
- **Fixtures**: Scheduled matches between teams in specific weeks
- **Matchups**: Individual games within fixtures (Men's, Women's, Mixed)

### Repository Layer
Each entity has a dedicated repository with:
- Basic CRUD operations
- Entity-specific queries
- Relationship management
- Statistical functions

Key repositories:
- `SeasonRepository`: Season management and active season tracking
- `WeekRepository`: Week scheduling and current week management
- `LeagueRepository`: League and season associations
- `DivisionRepository`: Division management within leagues
- `ClubRepository`: Club information and team associations
- `TeamRepository`: Team management and player associations
- `PlayerRepository`: Player data and team memberships
- `FixtureRepository`: Match scheduling and week associations
- `CaptainRepository`: Captain role management

## Getting Started

### Prerequisites
- Go 1.21 or later
- SQLite3 (default) or PostgreSQL

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd jim-dot-tennis
```

2. Install dependencies:
```bash
go mod download
```

3. Run database migrations:
```bash
make migrate-up
```

### Local Development

#### Quick Start
For local development, you can use either the Makefile commands or the convenience script:

**Using Makefile (Recommended):**
```bash
# Build and run locally (database will be created at project root)
make local

# Or run individual commands:
make build-local  # Build binary to bin/jim-dot-tennis
make run-local    # Build and run with database at ./tennis.db
make clean-local  # Clean up binary and database
```

**Using convenience script:**
```bash
# Run the application locally
./scripts/run.sh
```

Both methods will:
- Build the binary and place it in `bin/jim-dot-tennis`
- Create the database at the project root (`./tennis.db`)
- Start the server at `http://localhost:8080`

#### Production Deployment (Docker)
For production deployment, use Docker:

```bash
# Build and run with Docker
make build
make run

# View logs
make logs

# Stop the application
make stop
```

### Usage

#### Database Population
Use the populate-db script to import fixture data from CSV files:

```bash
# Build the population script
go build -o bin/populate-db cmd/populate-db/main.go

# Run with default settings
./bin/populate-db -verbose

# Dry run to preview changes
./bin/populate-db -dry-run -verbose

# Use custom CSV directory
./bin/populate-db -csv-dir ./my-fixtures -verbose
```

The script will:
- Create the 2025 season with 18 weeks
- Set up Brighton & Hove Parks Tennis League
- Create 4 divisions with appropriate play days
- Import clubs, teams, and fixtures from CSV files
- Associate fixtures with the correct weeks

#### Development

```bash
# Run tests
make test

# Build the application
make build

# Run with hot reload (if using air)
make dev
```

## CSV Import Format

The system expects CSV files with fixture data in the following format:

```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
1,April 17,Hove A,Preston Park,,
10,June 19,,,Hove,Dyke A
10,June 19,,,Preston Park,Hove A
```

- **Week**: Week number (1-18)
- **Date**: Date in "Month Day" format
- **First Half**: Teams for first half of season
- **Second Half**: Teams for second half of season

## Database Schema Highlights

### Week Management
- Weeks are automatically created for each season (1-18)
- Each week has start/end dates calculated from season start
- Support for active week tracking
- Fixtures are associated with specific weeks

### Fixture Organization
- Fixtures belong to a specific week, division, and season
- Support for home/away team assignments
- Venue location and status tracking
- Optional day captain assignments

### Team Structure
- Teams belong to clubs and compete in specific divisions
- Club name extraction from team names (e.g., "Dyke A" → Club: "Dyke")
- Support for multiple teams per club (configurable limit)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

[Add your license information here]