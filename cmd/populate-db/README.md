# Database Population Script

This script populates the tennis league database using CSV fixture files from the 2025 Parks League season.

## Features

- Creates the 2025 season and Brighton & Hove Parks Tennis League
- Processes all 4 division CSV files (Division 1-4)
- Automatically extracts club names from team names (e.g., "Dyke A" → Club: "Dyke", Team: "Dyke A")
- Creates clubs, teams, and fixtures with proper relationships
- Supports dry-run mode to preview changes
- Handles duplicate detection to avoid recreating existing data

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

- `-csv-dir`: Directory containing CSV files (default: "test_pdf_output_fixed")
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
- Week numbers in notes
- Status set to "Scheduled"
- Venue location set to "TBD" (To Be Determined)

## CSV File Format

The script expects CSV files with the following format:

```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
1,April 17,Hove A,Preston Park,,
10,June 19,,,Hove,Dyke A
10,June 19,,,Preston Park,Hove A
```

- **Week**: Week number (1-18)
- **Date**: Date in format "Month Day" (e.g., "April 17")
- **Home_Team_First_Half/Away_Team_First_Half**: Teams for first half of season
- **Home_Team_Second_Half/Away_Team_Second_Half**: Teams for second half of season

## Team Name Parsing

The script intelligently parses team names to extract club information:

- **"Dyke A"** → Club: "Dyke", Team: "Dyke A", Suffix: "A"
- **"King Alfred"** → Club: "King Alfred", Team: "King Alfred", Suffix: ""
- **"St Ann's"** → Club: "St Ann's", Team: "St Ann's", Suffix: ""

## Error Handling

- Skips invalid CSV rows with warnings
- Continues processing if individual fixtures fail
- Detects and skips duplicate entries
- Provides detailed error messages

## Database Requirements

- Requires database migrations to be run first
- Compatible with SQLite (default) and PostgreSQL
- Creates all necessary foreign key relationships

## Troubleshooting

### Common Issues

1. **Migration errors**: Ensure migrations directory exists and is accessible
2. **CSV file not found**: Check the `-csv-dir` path
3. **Date parsing errors**: Check date format in CSV files
4. **Database connection errors**: Verify database path and permissions

### Verbose Mode

Use `-verbose` flag to see detailed information about:
- Which entities are being created
- Which entities already exist (skipped)
- Fixture creation progress
- Any warnings or errors

### Dry Run Mode

Use `-dry-run` flag to:
- See what would be created without making changes
- Validate CSV files and data parsing
- Test the script before running on production data 