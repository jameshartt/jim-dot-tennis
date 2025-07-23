# Local Import Guide

This guide shows how to run the tennis match card import locally without Docker.

## Prerequisites

1. **Go installed** on your system
2. **Local database** at `./tennis.db` (or specify custom path)
3. **TENNIS_NONCE** environment variable set

## Getting the Production Database

If you want to work with the production database locally, you can download it from the server:

```bash
# Download the production database and replace your local database
./scripts/download-database.sh
```

This script will:
- Connect to your DigitalOcean server
- Backup your existing local database (if it exists)
- Download the production database
- Verify the database integrity
- Provide clear status messages throughout the process

**Note:** The script uses the same server configuration as your deployment scripts.

## Quick Setup

```bash
# Set your nonce (get this from the tennis website)
export TENNIS_NONCE="your-nonce-here"

# Verify your database exists (or download from production)
ls -la ./tennis.db
```

## Direct Go Commands

### Single Week Import
```bash
go run cmd/import-matchcards/main.go \
  -db="./tennis.db" \
  -nonce="$TENNIS_NONCE" \
  -club-code="resident-beard-font" \
  -week=1 \
  -year=2025 \
  -club-id=10 \
  -club-name="St+Anns" \
  -verbose
```

### Dry Run (Test Mode)
```bash
go run cmd/import-matchcards/main.go \
  -db="./tennis.db" \
  -nonce="$TENNIS_NONCE" \
  -club-code="resident-beard-font" \
  -week=1 \
  -year=2025 \
  -club-id=10 \
  -club-name="St+Anns" \
  -verbose \
  -dry-run
```

### Clear Existing and Import
```bash
go run cmd/import-matchcards/main.go \
  -db="./tennis.db" \
  -nonce="$TENNIS_NONCE" \
  -club-code="resident-beard-font" \
  -week=1 \
  -year=2025 \
  -club-id=10 \
  -club-name="St+Anns" \
  -verbose \
  -clear-existing
```

## Using Local Scripts (Recommended)

### Single Week Import
```bash
# Import week 1
./scripts/local-import.sh --week=1

# Test import week 5 (dry run)
./scripts/local-import.sh --week=5 --dry-run

# Clear existing and import week 3
./scripts/local-import.sh --week=3 --clear-existing

# Use custom database
./scripts/local-import.sh --week=2 --db=./my-tennis.db
```

### All Weeks Import
```bash
# Import all weeks 1-18
./scripts/local-import-all.sh

# Test import all weeks (dry run)
./scripts/local-import-all.sh --dry-run

# Clear existing and import all weeks
./scripts/local-import-all.sh --clear-existing

# Import specific range
./scripts/local-import-all.sh --start-week=5 --end-week=10

# Use custom database and delay
./scripts/local-import-all.sh --db=./my-tennis.db --delay=3
```

## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-db` | Database file path | Required |
| `-nonce` | BHPLTA authentication nonce | Required |
| `-club-code` | Club code | "resident-beard-font" |
| `-week` | Week number (1-18) | Required |
| `-year` | Season year | 2025 |
| `-club-id` | Club ID | 10 |
| `-club-name` | Club name | "St+Anns" |
| `-verbose` | Enable verbose output | false |
| `-dry-run` | Test mode (no database changes) | false |
| `-clear-existing` | Clear existing matchups first | false |

## Script Options

### local-import.sh
- `--week=N` - Import specific week
- `--db=PATH` - Database path
- `--dry-run` - Test mode
- `--clear-existing` - Clear existing matchups
- `--quiet` - Disable verbose output

### local-import-all.sh  
- `--start-week=N` - Start week
- `--end-week=N` - End week
- `--db=PATH` - Database path
- `--dry-run` - Test mode
- `--clear-existing` - Clear existing matchups
- `--delay=N` - Seconds between requests
- `--quiet` - Disable verbose output

## Examples

### Testing Your Setup
```bash
# Test with a single week first
export TENNIS_NONCE="your-nonce-here"
./scripts/local-import.sh --week=1 --dry-run
```

### Derby Match Import
```bash
# For derby matches, use clear-existing to ensure proper handling
./scripts/local-import.sh --week=5 --clear-existing
```

### Production Import
```bash
# Import all available weeks
export TENNIS_NONCE="your-nonce-here"
./scripts/local-import-all.sh --clear-existing
```

### Custom Database
```bash
# If your database is in a different location
./scripts/local-import.sh --week=1 --db=./data/tennis.db
```

## Troubleshooting

### "Database not found"
- Check the database path exists
- Use `--db=PATH` to specify correct location

### "TENNIS_NONCE required"
- Set the environment variable: `export TENNIS_NONCE="your-nonce"`
- Get fresh nonce from tennis website if expired

### "Permission denied"
- Your nonce may have expired - get a fresh one
- Ensure you're logged into the tennis website

### Derby match issues
- Use `--clear-existing` flag to ensure clean import
- Check that both teams are St. Ann's teams in the database

## Performance Tips

- Use `--dry-run` first to test before making changes
- Use `--delay=N` to slow down requests if hitting rate limits
- Use `--clear-existing` when re-running imports to avoid conflicts
- Import single weeks for testing, then use `local-import-all.sh` for bulk import 