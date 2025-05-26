# Tennis League Data Scraper

This tool scrapes tennis league data from the Brighton and Hove Parks Lawn Tennis Association website and imports it into the database. It currently supports importing fixtures, teams, and clubs. Results and standings table support will be added in future updates.

**NEW**: The scraper now includes PDF column splitting functionality to extract individual division fixtures from the BHPLTA fixture card PDF.

## Usage

```bash
# Build the scraper
go build -o bin/scraper cmd/scraper/main.go

# Run the scraper with default settings
./bin/scraper

# Run with custom options
./bin/scraper -db=tennis.db -year=2025 -season="Summer 2025"

# Split PDF fixture card into column images for OCR processing
./bin/scraper -split-pdf

# Split PDF with custom output directory
./bin/scraper -split-pdf -output=my_columns
```

## Command-Line Options

- `-db`: Path to SQLite database file (default: "tennis.db")
- `-fixtures`: URL to fixtures page (default: "https://www.bhplta.co.uk/bhplta_tables/fixtures/")
- `-results`: URL to results page (default: "https://www.bhplta.co.uk/bhplta_tables/results/")
- `-tables`: URL to league tables page (default: "https://www.bhplta.co.uk/bhplta_tables/league-table/")
- `-year`: Season year (default: 2025)
- `-season`: Season name (default: "Summer 2025")
- `-pdf`: URL to fixture card PDF (default: "https://www.bhplta.co.uk/wp-content/uploads/2025/03/Fixture-Card-2025.pdf")
- `-output`: Output directory for column images (default: "output_columns")
- `-split-pdf`: Split PDF into column images for OCR processing
- `-help`: Show help message

## PDF Column Splitting

The `-split-pdf` flag downloads the BHPLTA fixture card PDF and splits the second page into four separate column images, one for each division. This makes it easier for OCR tools to parse the fixture data accurately.

**Output files:**
- `Division_1_fixtures.png` - Division 1 fixtures
- `Division_2_fixtures.png` - Division 2 fixtures  
- `Division_3_fixtures.png` - Division 3 fixtures
- `Division_4_fixtures.png` - Division 4 fixtures

The images are saved as high-quality PNG files with slight overlap between columns to ensure no text is cut off.

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
4. OCR processing of the split column images

Once match card access is available, the player data can be scraped as well.

# PDF Fixture Extractor

Extracts tennis league fixtures from PDF files and converts them to CSV format.

## Usage

```bash
go run pdf_extractor.go
```

## Features

- Downloads PDFs from league website
- Extracts text using go-fitz
- Parses fixture blocks with proper team counting
- Handles both halves of season with home/away reversal
- Preserves team identifiers (A, B, C, D, E, F)
- Generates clean CSV output

## Output

Creates CSV files in `test_pdf_output_fixed/` directory:
- `Div_1_2025_fixtures.csv` (90 fixtures)
- `Div_2_2025_fixtures.csv` (90 fixtures) 
- `Div_3_2025_fixtures.csv` (90 fixtures)
- `Div_4_2025_fixtures.csv` (132 fixtures)

Each CSV contains structured fixture data with separate columns for first and second halves of the season. 