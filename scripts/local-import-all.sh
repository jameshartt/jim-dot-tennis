#!/bin/bash

# Local Tennis Import Script for All Weeks
# Usage: ./scripts/local-import-all.sh [options]

# Default values
DB_PATH="./tennis.db"
START_WEEK=1
END_WEEK=18
DRY_RUN=""
CLEAR_EXISTING=""
VERBOSE="-verbose"
DELAY_SECONDS=2

# Check for required environment variables
if [ -z "$TENNIS_NONCE" ]; then
  echo "❌ Error: TENNIS_NONCE environment variable is required"
  echo "   Set it with: export TENNIS_NONCE='your-nonce-here'"
  exit 1
fi

show_help() {
  echo "Local Tennis Import Script for All Weeks"
  echo ""
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --start-week=N      Start from week N (default: 1)"
  echo "  --end-week=N        End at week N (default: 18)"
  echo "  --db=PATH           Database path (default: ./tennis.db)"
  echo "  --dry-run           Run in dry-run mode"
  echo "  --clear-existing    Clear existing matchups before importing"
  echo "  --delay=N           Delay N seconds between requests (default: 2)"
  echo "  --quiet             Disable verbose output"
  echo "  -h, --help          Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  TENNIS_NONCE        Required: The nonce for API authentication"
  echo ""
  echo "Examples:"
  echo "  $0                                 # Import all weeks 1-18"
  echo "  $0 --start-week=5 --end-week=10    # Import weeks 5-10"
  echo "  $0 --dry-run                       # Test import all weeks"
  echo "  $0 --clear-existing                # Clear and import all weeks"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --start-week=*)
      START_WEEK="${1#*=}"
      shift
      ;;
    --end-week=*)
      END_WEEK="${1#*=}"
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
    --delay=*)
      DELAY_SECONDS="${1#*=}"
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

# Validate week range
if [ "$START_WEEK" -lt 1 ] || [ "$START_WEEK" -gt 18 ] || [ "$END_WEEK" -lt 1 ] || [ "$END_WEEK" -gt 18 ] || [ "$START_WEEK" -gt "$END_WEEK" ]; then
  echo "❌ Error: Week range must be between 1-18 and start-week must be <= end-week"
  exit 1
fi

# Check if database exists
if [ ! -f "$DB_PATH" ]; then
  echo "❌ Error: Database file '$DB_PATH' not found"
  echo "   Make sure the database exists or specify the correct path with --db=PATH"
  exit 1
fi

echo "🏆 Starting local match card import for weeks $START_WEEK to $END_WEEK"
echo "🗄️  Database: $DB_PATH"
echo "⏱️  Delay between requests: ${DELAY_SECONDS}s"
echo "🔐 Nonce: ${TENNIS_NONCE:0:10}..."
echo ""

# Initialize counters
TOTAL_WEEKS=$((END_WEEK - START_WEEK + 1))
SUCCESSFUL_WEEKS=0
FAILED_WEEKS=0

# Loop through all weeks
for week in $(seq $START_WEEK $END_WEEK); do
  echo "📅 Processing Week $week ($((week - START_WEEK + 1))/$TOTAL_WEEKS)..."
  echo "==============================================="
  
  # Run the import command
  if go run cmd/import-matchcards/main.go \
    -db="$DB_PATH" \
    -nonce="$TENNIS_NONCE" \
    -club-code="resident-beard-font" \
    -week=$week \
    -year=2025 \
    -club-id=10 \
    -club-name="St+Anns" \
    $VERBOSE \
    $DRY_RUN \
    $CLEAR_EXISTING; then
    
    echo "✅ Week $week completed successfully"
    SUCCESSFUL_WEEKS=$((SUCCESSFUL_WEEKS + 1))
  else
    echo "❌ Week $week failed"
    FAILED_WEEKS=$((FAILED_WEEKS + 1))
  fi
  
  echo ""
  
  # Add delay between requests (except for the last week)
  if [ "$week" -ne "$END_WEEK" ]; then
    echo "⏳ Waiting ${DELAY_SECONDS}s before next request..."
    sleep $DELAY_SECONDS
    echo ""
  fi
done

# Final summary
echo "🎯 FINAL SUMMARY"
echo "==============================================="
echo "📊 Total weeks processed: $TOTAL_WEEKS"
echo "✅ Successful weeks: $SUCCESSFUL_WEEKS"
echo "❌ Failed weeks: $FAILED_WEEKS"
echo ""

if [ "$FAILED_WEEKS" -gt 0 ]; then
  echo "⚠️  Some weeks failed. Check the output above for details."
  exit 1
else
  echo "🎉 All weeks completed successfully!"
  if [ -n "$DRY_RUN" ]; then
    echo "🔍 Remember: This was a dry run - no changes were saved to the database"
  fi
fi 