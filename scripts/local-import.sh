#!/bin/bash

# Local Tennis Import Script
# Usage: ./scripts/local-import.sh [options]

# Default values
DB_PATH="./tennis.db"
WEEK=1
DRY_RUN=""
CLEAR_EXISTING=""
VERBOSE="-verbose"

# Check for required environment variables
if [ -z "$TENNIS_NONCE" ]; then
  echo "❌ Error: TENNIS_NONCE environment variable is required"
  echo "   Set it with: export TENNIS_NONCE='your-nonce-here'"
  exit 1
fi

show_help() {
  echo "Local Tennis Import Script"
  echo ""
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --week=N            Import week N (default: 1)"
  echo "  --db=PATH           Database path (default: ./tennis.db)"
  echo "  --dry-run           Run in dry-run mode"
  echo "  --clear-existing    Clear existing matchups before importing"
  echo "  --quiet             Disable verbose output"
  echo "  -h, --help          Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  TENNIS_NONCE        Required: The nonce for API authentication"
  echo ""
  echo "Examples:"
  echo "  $0 --week=1                    # Import week 1"
  echo "  $0 --week=5 --dry-run          # Test import week 5"
  echo "  $0 --week=3 --clear-existing   # Clear and import week 3"
  echo "  $0 --db=./my-tennis.db --week=2 # Use custom database"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --week=*)
      WEEK="${1#*=}"
      shift
      ;;
    --db=*)
      DB_PATH="${1#*=}"
      shift
      ;;
    --dry-run)
      DRY_RUN="-dry-run"
      echo "🔍 DRY RUN MODE: No changes will be saved to database"
      shift
      ;;
    --clear-existing)
      CLEAR_EXISTING="-clear-existing"
      echo "🧹 CLEAR EXISTING MODE: Existing matchups will be cleared"
      shift
      ;;
    --quiet)
      VERBOSE=""
      shift
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    *)
      echo "❌ Unknown option: $1"
      show_help
      exit 1
      ;;
  esac
done

# Validate week
if [ "$WEEK" -lt 1 ] || [ "$WEEK" -gt 18 ]; then
  echo "❌ Error: Week must be between 1 and 18"
  exit 1
fi

# Check if database exists
if [ ! -f "$DB_PATH" ]; then
  echo "❌ Error: Database file '$DB_PATH' not found"
  echo "   Make sure the database exists or specify the correct path with --db=PATH"
  exit 1
fi

echo "🏆 Starting local match card import"
echo "📅 Week: $WEEK"
echo "🗄️  Database: $DB_PATH"
echo "🔐 Nonce: ${TENNIS_NONCE:0:10}..."
echo ""

# Run the import
go run cmd/import-matchcards/main.go \
  -db="$DB_PATH" \
  -nonce="$TENNIS_NONCE" \
  -club-code="${CLUB_CODE:?set CLUB_CODE (your BHPLTA club code)}" \
  -week=$WEEK \
  -year=2025 \
  -club-id="${CLUB_ID:?set CLUB_ID (your club database id)}" \
  -club-name="${CLUB_NAME:?set CLUB_NAME (URL-encoded, e.g. My+Club)}" \
  $VERBOSE \
  $DRY_RUN \
  $CLEAR_EXISTING

echo ""
echo "✅ Import completed!" 