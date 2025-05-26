# Tennis League Fixture Parser

A Go application that extracts tennis league fixtures from PDF files and converts them to structured CSV format.

## Features

- **PDF Text Extraction**: Uses go-fitz to extract clean text from PDF files
- **Smart Parsing**: Automatically detects fixture blocks and handles both halves of the season
- **Multi-Division Support**: Handles different division formats (5 teams vs 6 teams)
- **Home/Away Logic**: Correctly processes the reversal between first and second halves
- **Team Name Preservation**: Maintains important team identifiers (A, B, C, D, E, F suffixes)
- **CSV Output**: Generates clean, structured CSV files ready for import

## Usage

### PDF Fixture Extraction

Extract fixtures from all division PDFs:

```bash
go run cmd/scraper/pdf_extractor.go
```

This will:
1. Download the latest PDF files from the league website
2. Extract text using go-fitz
3. Parse fixtures into structured format
4. Generate CSV files for each division
5. Clean up temporary files

### Output Format

The generated CSV files contain the following columns:
- `Week`: Week number (1-18)
- `Date`: Match date
- `Home_Team_First_Half`: Home team for first half of season (weeks 1-9)
- `Away_Team_First_Half`: Away team for first half of season (weeks 1-9)
- `Home_Team_Second_Half`: Home team for second half of season (weeks 10-18)
- `Away_Team_Second_Half`: Away team for second half of season (weeks 10-18)

### Division Support

- **Divisions 1-3**: 10 teams, 5 matches per week, 90 total fixtures
- **Division 4**: 12 teams, 6 matches per week, 132 total fixtures

## Dependencies

```bash
go mod tidy
```

Key dependencies:
- `github.com/gen2brain/go-fitz` - PDF text extraction

## Development

The main parser is located in `cmd/scraper/pdf_extractor.go` and handles:
- PDF downloading and text extraction
- Fixture block parsing with proper team counting
- Date parsing and formatting
- Team name cleaning while preserving identifiers
- CSV generation

## Example Output

```csv
Week,Date,Home_Team_First_Half,Away_Team_First_Half,Home_Team_Second_Half,Away_Team_Second_Half
1,April 17,Dyke A,Hove,,
1,April 17,Hove A,Preston Park,,
10,June 19,,,Hove,Dyke A
10,June 19,,,Preston Park,Hove A
```

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

### Deploying to DigitalOcean

We provide a robust two-part deployment system for DigitalOcean:

1. **Configure the deployment script**:
   ```bash
   # Edit DROPLET_IP and other settings in the script
   nano scripts/deploy-digitalocean.sh
   ```

2. **Run the deployment script**:
   ```bash
   ./scripts/deploy-digitalocean.sh
   ```

The script handles everything automatically:
- Setting up the server with Docker and security measures
- Transferring application files
- Configuring HTTPS (if you provide a domain)
- Starting the application with Docker Compose

For detailed deployment instructions, see [DigitalOcean Deployment Guide](docs/digitalocean_deployment.md).

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
│   ├── backup-manager.sh # External backup script
│   └── deploy-digitalocean.sh # DigitalOcean deployment script
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
- **PDF Fixture Processing**: OCR-based extraction of tennis league fixtures from PDF documents

## PDF Fixture Processing

The application includes a powerful OCR-based system for processing tennis league fixture cards:

### Features

- **PDF Column Splitting**: Automatically downloads and splits fixture card PDFs into individual division columns
- **OCR Text Extraction**: Uses Tesseract OCR to extract text from column images
- **Intelligent Parsing**: Handles complex dual-column structure where first and second half seasons are displayed side-by-side
- **Structured Week/Date Calculation**: Calculates proper weeks and dates based on division-specific schedules and fixture grouping
- **CSV Export**: Generates clean CSV files with properly structured fixture data
- **Advanced Team Name Cleaning**: Comprehensive cleaning system that removes OCR artifacts, month prefixes, ordinals, and normalizes team names
- **Validation**: Checks for expected number of weeks and matchups per division

#### Structured Week and Date Calculation

The system uses a structured approach to calculate accurate weeks and dates:

- **Division-Specific Start Dates**: 
  - Division 1: Thursday, 17 Apr 2025
  - Division 2: Wednesday, 16 Apr 2025  
  - Division 3: Tuesday, 15 Apr 2025
  - Division 4: Tuesday, 8 Apr 2025 (one week earlier)
- **Fixture Grouping**: Groups fixtures by expected count (5 for Div 1-3, 6 for Div 4)
- **Sequential Week Assignment**: Assigns weeks 1-18 (Div 1-3) or 1-20 (Div 4) based on fixture position
- **Weekly Date Progression**: Calculates dates with 7-day intervals from start dates
- **Dual Season Structure**: Generates both first half (weeks 1-9/10) and second half (weeks 10-18/11-20)

#### Team Name Cleaning Features

The OCR system includes sophisticated team name cleaning that handles:

- **Month Prefix Removal**: "Apr King Alfred" → "King Alfred"
- **Ordinal Removal**: "th Queens" → "Queens"  
- **OCR Artifact Cleaning**: Removes "vy", "vv", "Vv" artifacts from "v" character misreads
- **Name Restoration**: "Anns" → "St Ann's" for truncated names
- **Compound Name Spacing**: "DykeA" → "Dyke A", "HoveA" → "Hove A"
- **Name Completion**: "King" → "King Alfred", "Preston" → "Preston Park"
- **Proper Apostrophes**: "StAnns" → "St Ann's"
- **Standardization**: "hove a" → "Hove A" for consistent formatting
- **Week Reference Removal**: Strips "Wk1", "Wk2" references mixed into team names
- **Date Fragment Removal**: Removes standalone numbers, years, and date components

Success rate: ~98% clean team names from raw OCR text.

### Usage

1. **Split PDF into column images**:
   ```bash
   ./bin/scraper -split-pdf
   ```

2. **OCR column images to CSV**:
   ```bash
   ./bin/scraper -ocr
   ```

3. **Run complete process**:
   ```bash
   ./scripts/full_ocr_process.sh
   ```

### Output Structure

The OCR process generates:
- **Column Images**: `output_columns/Division_X_fixtures.png`
- **CSV Files**: `output_csv/Division_X_fixtures.csv`
- **Debug Text**: `output_csv/Division_X_raw_ocr.txt`

Each CSV contains:
- Week number
- Date
- Home/Away teams for first half of season
- Home/Away teams for second half of season

### Division Configuration

- **Divisions 1-3**: 18 weeks, 5 matchups per week
- **Division 4**: 20 weeks, 6 matchups per week

### Requirements

- **Tesseract OCR**: Must be installed on the system
- **Go Dependencies**: `github.com/otiai10/gosseract/v2` for OCR bindings
- **PDF Processing**: `github.com/gen2brain/go-fitz` for PDF handling

## Development Roadmap

- [x] Add database models and migrations
- [x] Set up Docker environment with automated backups
- [x] Create DigitalOcean deployment script
- [ ] Add user authentication
- [ ] Implement push notification subscription flow
- [ ] Develop core application features
- [ ] Add comprehensive offline support
- [ ] Deploy to production

## Documentation

- [Project Overview](docs/project_overview.md)
- [User Experience Requirements](docs/user_experience_requirements.md)
- [Technical Implementation Plan](docs/technical_implementation_plan.md)
- [Docker Setup](docs/docker_setup.md)
- [DigitalOcean Deployment](docs/digitalocean_deployment.md)

## Technologies Used

- **Go**: Backend server
- **HTML**: Frontend markup
- **SVG**: Custom vector graphics
- **Service Workers**: PWA functionality
- **Web Push Protocol**: For push notifications
- **SQLite**: Embedded database
- **Docker**: Containerization and deployment