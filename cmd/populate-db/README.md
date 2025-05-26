# Database Population Script

This script populates the tennis league database using CSV fixture files from the 2025 Parks League season. The script creates a complete league structure including seasons, weeks, leagues, divisions, clubs, teams, and fixtures.

## Features

- **Season Management**: Creates the 2025 season with configurable date ranges
- **Week Creation**: Automatically generates 18 weeks for the season with calculated start/end dates
- **League Setup**: Creates Brighton & Hove Parks Tennis League with proper associations
- **Division Processing**: Processes all 4 division CSV files (Division 1-4) with appropriate play days
- **Club Extraction**: Automatically extracts club names from team names (e.g., "Dyke A" → Club: "Dyke", Team: "Dyke A")
- **Team Management**: Creates teams with proper club and division associations
- **Fixture Scheduling**: Creates fixtures with proper week associations and scheduling
- **Duplicate Detection**: Handles duplicate detection to avoid recreating existing data
- **Dry-run Support**: Preview changes without making database modifications
- **Comprehensive Logging**: Detailed logging with configurable verbosity levels

## Usage

### Basic Usage

```bash
# Build the script
go build -o bin/populate-db cmd/populate-db/main.go

# Run with default settings (SQLite database)
./bin/populate-db

# Run with verbose output
./bin/populate-db -verbose

# Dry run to see what would be created
./bin/populate-db -dry-run -verbose
```

### Command Line Options

- `-csv-dir`: Directory containing CSV files (default: "pdf_output")
- `-db-path`: Path to SQLite database file (default: "./tennis.db")
- `-db-type`: Database type - sqlite3 or postgres (default: "sqlite3")
- `-dry-run`: Print what would be done without making changes
- `-verbose`: Enable verbose logging

### Examples

```bash
# Use different CSV directory
./bin/populate-db -csv-dir ./fixtures -verbose

# Use different database file
./bin/populate-db -db-path ./my-tennis.db

# Dry run with verbose output
./bin/populate-db -dry-run -verbose
```

## What Gets Created

### Season
- **Name**: "2025 Season"
- **Year**: 2025
- **Start Date**: April 1, 2025
- **End Date**: September 30, 2025
- **Status**: Active

### Weeks
The script creates 18 weeks for the season:
- **Week 1**: April 1-7, 2025 (Active by default)
- **Week 2**: April 8-14, 2025
- **Week 3**: April 15-21, 2025
- ...continuing through...
- **Week 18**: August 26 - September 1, 2025

Each week includes:
- Sequential week numbers (1-18)
- Calculated start and end dates (7-day periods)
- Descriptive names ("Week 1", "Week 2", etc.)
- Active status tracking (Week 1 is initially active)

### League
- **Name**: "Brighton & Hove Parks Tennis League"
- **Type**: Parks League
- **Year**: 2025
- **Region**: "Brighton & Hove"

### Divisions
- **Division 1**: Level 1, Play Day: Thursday
- **Division 2**: Level 2, Play Day: Tuesday  
- **Division 3**: Level 3, Play Day: Wednesday
- **Division 4**: Level 4, Play Day: Monday

### Clubs
Automatically extracted from team names:
- Dyke
- Hove
- King Alfred
- Queens
- Saltdean
- St Ann's
- Preston Park
- Blakers
- Hollingbury
- Park Avenue

### Teams
All teams from the CSV files, properly associated with their clubs and divisions.

### Fixtures
All fixtures from the CSV files with:
- Proper home/away team assignments
- Scheduled dates parsed from the CSV
- **Week associations** based on the week number in the CSV
- Status set to "Scheduled"
- Venue location set to "TBD" (To Be Determined)
- Week numbers included in fixture notes

## CSV File Format

The script expects CSV files with the following format:

```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
1,April 17,Hove A,Preston Park,,
10,June 19,,,Hove,Dyke A
10,June 19,,,Preston Park,Hove A
```

- **Week**: Week number (1-18) - used to associate fixtures with the correct week
- **Date**: Date in format "Month Day" (e.g., "April 17")
- **Home_Team_First_Half/Away_Team_First_Half**: Teams for first half of season
- **Home_Team_Second_Half/Away_Team_Second_Half**: Teams for second half of season

## Team Name Parsing

The script intelligently parses team names to extract club information:

- **"Dyke A"** → Club: "Dyke", Team: "Dyke A", Suffix: "A"
- **"King Alfred"** → Club: "King Alfred", Team: "King Alfred", Suffix: ""
- **"St Ann's"** → Club: "St Ann's", Team: "St Ann's", Suffix: ""

## Week Management Features

### Automatic Week Creation
- Creates 18 weeks automatically for each season
- Calculates start and end dates based on season start date
- Each week spans exactly 7 days
- Week 1 starts on the season start date (April 1, 2025)

### Week-Fixture Association
- Fixtures are associated with weeks based on the "Week" column in CSV files
- Each fixture belongs to exactly one week
- Week associations enable powerful querying and organization

### Active Week Tracking
- Supports marking one week as "active" per season
- Week 1 is set as active by default
- Database triggers ensure only one active week per season

## Error Handling

- Skips invalid CSV rows with warnings
- Continues processing if individual fixtures fail
- Detects and skips duplicate entries
- Provides detailed error messages
- Validates week numbers and ensures weeks exist before creating fixtures

## Database Requirements

- Requires database migrations to be run first
- Compatible with SQLite (default) and PostgreSQL
- Creates all necessary foreign key relationships
- Includes proper indexing for performance

## Troubleshooting

### Common Issues

1. **Migration errors**: Ensure migrations directory exists and is accessible
2. **CSV file not found**: Check the `-csv-dir` path
3. **Date parsing errors**: Check date format in CSV files
4. **Database connection errors**: Verify database path and permissions
5. **Week not found errors**: Ensure week numbers in CSV are between 1-18

### Verbose Mode

Use `-verbose` flag to see detailed information about:
- Which entities are being created
- Which entities already exist (skipped)
- Week creation and association progress
- Fixture creation progress
- Any warnings or errors

### Dry Run Mode

Use `-dry-run` flag to:
- See what would be created without making changes
- Validate CSV files and data parsing
- Test week creation and fixture associations
- Verify the script before running on production data

## Database Schema Integration

The script works with the enhanced database schema that includes:

### Week Table
```sql
CREATE TABLE weeks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    week_number INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    name VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(week_number, season_id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);
```

### Enhanced Fixtures Table
```sql
CREATE TABLE fixtures (
    -- ... other fields ...
    week_id INTEGER NOT NULL,
    FOREIGN KEY (week_id) REFERENCES weeks(id)
);
```

This integration provides:
- Proper relational integrity between weeks and fixtures
- Efficient querying by week
- Support for week-based reporting and analysis
- Foundation for advanced scheduling features 