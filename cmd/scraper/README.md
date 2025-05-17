# Tennis League Data Scraper

This tool scrapes tennis league data from the Brighton and Hove Parks Lawn Tennis Association website and imports it into the database. It currently supports importing fixtures, teams, and clubs. Results and standings table support will be added in future updates.

## Usage

```bash
# Build the scraper
go build -o bin/scraper cmd/scraper/main.go

# Run the scraper with default settings
./bin/scraper

# Run with custom options
./bin/scraper -db=tennis.db -year=2025 -season="Summer 2025"
```

## Command-Line Options

- `-db`: Path to SQLite database file (default: "tennis.db")
- `-fixtures`: URL to fixtures page (default: "https://www.bhplta.co.uk/bhplta_tables/fixtures/")
- `-results`: URL to results page (default: "https://www.bhplta.co.uk/bhplta_tables/results/")
- `-tables`: URL to league tables page (default: "https://www.bhplta.co.uk/bhplta_tables/league-table/")
- `-year`: Season year (default: 2025)
- `-season`: Season name (default: "Summer 2025")

## What Gets Imported

1. Clubs - Extracted from team names in the fixtures
2. Divisions - Extracted from fixtures page headings
3. Teams - Extracted from fixtures listings
4. Fixtures - Extracted with scheduled dates
5. Default matchups - Created for each fixture (Men's, Women's, 1st Mixed, 2nd Mixed)

## Development

The scraper can be extended to support additional data sources. Currently missing functionality includes:

1. League table scraping (standings)
2. Match results scraping
3. Player information

Once match card access is available, the player data can be scraped as well. 