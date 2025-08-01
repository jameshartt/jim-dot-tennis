# Tennis Import Credentials Setup

This guide explains how to get the required authentication credentials for importing tennis match card data.

## Automatic Nonce Extraction (Recommended)

As of the latest version, the system can automatically extract nonces from the BHPLTA website! You no longer need to manually find nonces in most cases.

### Quick Start with Auto-Nonce

```bash
# Test nonce extraction
./cmd/extract-nonce/extract-nonce -club-code=YOUR_CLUB_CODE -verbose

# Import with automatic nonce extraction
./cmd/import-matchcards/import-matchcards -auto-nonce -club-code=YOUR_CLUB_CODE -week=1 -year=2024
```

## Manual Credentials (Fallback)

If automatic extraction doesn't work, you can still manually provide:

- **TENNIS_NONCE** - The BHPLTA nonce for API authentication

## Getting Your Credentials

### 1. Login to the Tennis Club Website

1. Go to the tennis club website
2. Login to your account
3. Navigate to the match cards section

### 2. Get the Nonce

**Using Browser Developer Tools:**

1. Open your browser's Developer Tools (F12)
2. Go to the Network tab
3. Make any request that uses the nonce (e.g., change week selection)
4. Look for requests to `admin-ajax.php`
5. Check the request payload for the `nonce` parameter
6. Copy the nonce value

**Alternative - Check Page Source:**

1. Right-click on the match cards page and "View Source"
2. Search for `"nonce"` in the page source
3. Look for a JavaScript variable or hidden input containing the nonce
4. Copy the nonce value

## Setting Up Credentials

### Using the Tennis Import Script (Recommended)

```bash
./scripts/tennis-import.sh setup
```

This will prompt you to enter your nonce and save it securely.

### Manual Setup

Create a `.tennis-credentials` file in the scripts directory:

```bash
# Tennis Import Credentials
export TENNIS_NONCE='your-nonce-here'
```

## Testing Your Setup

Test that your credentials work:

```bash
./scripts/tennis-import.sh run-dry
```

This will run a dry-run import to verify your credentials without making database changes.

## Import Options

### Regular Import
```bash
./scripts/tennis-import.sh run
```
Imports all weeks (1-18) with existing matchups being updated.

### Dry Run
```bash
./scripts/tennis-import.sh run-dry
```
Tests the import without making any database changes.

### Clear Existing and Re-import
```bash
./scripts/tennis-import.sh run-clear
```
Clears existing matchups before importing. This is useful when:
- Re-running imports to ensure clean data
- Fixing issues with previous imports
- Handling derby matches that need both team perspectives

### Single Week Import
```bash
./scripts/tennis-import.sh run-week 5
```
Imports only week 5.

### Range Import
```bash
./scripts/tennis-import.sh run-range 1-5
```
Imports weeks 1 through 5.

## Derby Match Handling

The import system now properly handles derby matches (where both teams are from St. Ann's):

- **Automatic Detection**: Derby matches are automatically detected
- **Dual Processing**: Creates separate matchups for each team's perspective
- **Clear Existing**: Use `--clear-existing` to ensure clean derby match data
- **Comprehensive Results**: Both teams get their own matchup records

When importing derby matches, you'll see output like:
```
Processing derby match: St Ann's A vs St Ann's B (fixture 123)
Created First mixed matchup for home team (fixture 123, sets: 2-1, 6-4 6-2 4-6) - Home team wins
Created First mixed matchup for away team (fixture 123, sets: 2-1, 6-4 6-2 4-6) - Home team wins
```

## Security Notes

- Keep your credentials secure and don't share them
- The nonce may expire periodically - if imports start failing, get a fresh nonce
- The `.tennis-credentials` file is excluded from git to prevent accidental commits

## Troubleshooting

### "Permission Denied" errors

This usually means:
- Your nonce has expired - get a fresh one
- You're not logged in to the tennis club website

### "No match cards found"

This can happen if:
- The week you're trying to import doesn't have data yet
- Your club/season settings are incorrect
- There's an authentication issue

### Derby Match Issues

If you're having issues with derby matches:
- Use `--clear-existing` to ensure clean data
- Check that both teams are properly identified as St. Ann's teams
- Verify that the fixture has both teams from the same club 