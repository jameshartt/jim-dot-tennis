#!/bin/bash

# Optimized Import script for match cards for all weeks (1-18) in the 2025 season
# Usage: ./import_all_weeks_optimized.sh [--dry-run] [--start-week=N] [--end-week=N]

# Default values
DRY_RUN=""
START_WEEK=1
END_WEEK=18
DELAY_SECONDS=2
CLEAR_EXISTING=""

# Check for required environment variables
if [ -z "$TENNIS_NONCE" ]; then
  echo "❌ Error: TENNIS_NONCE environment variable is required"
  echo "   Set it with: export TENNIS_NONCE='your-nonce-here'"
  exit 1
fi

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN="-dry-run"
      echo "🔍 DRY RUN MODE: No changes will be saved to database"
      shift
      ;;
    --clear-existing)
      CLEAR_EXISTING="-clear-existing"
      echo "🧹 CLEAR EXISTING MODE: Existing matchups will be cleared before importing"
      shift
      ;;
    --start-week=*)
      START_WEEK="${1#*=}"
      shift
      ;;
    --end-week=*)
      END_WEEK="${1#*=}"
      shift
      ;;
    --delay=*)
      DELAY_SECONDS="${1#*=}"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --dry-run           Run in dry-run mode (no database changes)"
      echo "  --clear-existing    Clear existing matchups before importing"
      echo "  --start-week=N      Start from week N (default: 1)"
      echo "  --end-week=N        End at week N (default: 18)"
      echo "  --delay=N           Delay N seconds between requests (default: 2)"
      echo "  -h, --help          Show this help message"
      echo ""
      echo "Required Environment Variables:"
      echo "  TENNIS_NONCE        The nonce for API authentication"
      exit 0
      ;;
    *)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

# Validate week range
if [ "$START_WEEK" -lt 1 ] || [ "$START_WEEK" -gt 18 ] || [ "$END_WEEK" -lt 1 ] || [ "$END_WEEK" -gt 18 ] || [ "$START_WEEK" -gt "$END_WEEK" ]; then
  echo "❌ Error: Week range must be between 1-18 and start-week must be <= end-week"
  exit 1
fi

echo "🏆 Starting match card import for weeks $START_WEEK to $END_WEEK"
echo "⏱️  Delay between requests: ${DELAY_SECONDS}s"
echo "🔐 Using nonce: ${TENNIS_NONCE:0:10}..."
if [ -n "$CLEAR_EXISTING" ]; then
  echo "🧹 Clear existing matchups: enabled"
fi
echo ""

# Initialize counters
TOTAL_WEEKS=$((END_WEEK - START_WEEK + 1))
SUCCESSFUL_WEEKS=0
FAILED_WEEKS=0
TOTAL_PROCESSED=0
TOTAL_UPDATED_FIXTURES=0
TOTAL_UPDATED_MATCHUPS=0
TOTAL_MATCHED_PLAYERS=0

# Loop through all weeks
for week in $(seq $START_WEEK $END_WEEK); do
  echo "📅 Processing Week $week ($((week - START_WEEK + 1))/$TOTAL_WEEKS)..."
  echo "==============================================="
  
  # Run the import command using pre-built binary
  if import-matchcards \
    -db="/app/data/tennis.db" \
    -nonce="$TENNIS_NONCE" \
    -club-code="${CLUB_CODE:?set CLUB_CODE (your BHPLTA club code)}" \
    -week=$week \
    -year=2025 \
    -club-id="${CLUB_ID:?set CLUB_ID (your club database id)}" \
    -club-name="${CLUB_NAME:?set CLUB_NAME (URL-encoded, e.g. My+Club)}" \
    -verbose $DRY_RUN $CLEAR_EXISTING 2>&1 | tee "week_${week}_import.log"; then
    
    echo "✅ Week $week completed successfully"
    SUCCESSFUL_WEEKS=$((SUCCESSFUL_WEEKS + 1))
    
    # Extract statistics from the log file (basic parsing)
    if [ -f "week_${week}_import.log" ]; then
      PROCESSED=$(grep "Processed matches:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      FIXTURES=$(grep "Updated fixtures:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      MATCHUPS=$(grep "Updated matchups:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      PLAYERS=$(grep "Matched players:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      
      # Add to totals (with error handling for non-numeric values)
      if [[ "$PROCESSED" =~ ^[0-9]+$ ]]; then
        TOTAL_PROCESSED=$((TOTAL_PROCESSED + PROCESSED))
      fi
      if [[ "$FIXTURES" =~ ^[0-9]+$ ]]; then
        TOTAL_UPDATED_FIXTURES=$((TOTAL_UPDATED_FIXTURES + FIXTURES))
      fi
      if [[ "$MATCHUPS" =~ ^[0-9]+$ ]]; then
        TOTAL_UPDATED_MATCHUPS=$((TOTAL_UPDATED_MATCHUPS + MATCHUPS))
      fi
      if [[ "$PLAYERS" =~ ^[0-9]+$ ]]; then
        TOTAL_MATCHED_PLAYERS=$((TOTAL_MATCHED_PLAYERS + PLAYERS))
      fi
      
      echo "   📊 Week $week stats: $PROCESSED matches, $FIXTURES fixtures, $MATCHUPS matchups, $PLAYERS players"
    fi
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
echo "📈 AGGREGATE STATISTICS:"
echo "   🏆 Total matches processed: $TOTAL_PROCESSED"
echo "   📋 Total fixtures updated: $TOTAL_UPDATED_FIXTURES"
echo "   🎾 Total matchups updated: $TOTAL_UPDATED_MATCHUPS"
echo "   👥 Total players matched: $TOTAL_MATCHED_PLAYERS"
echo ""

if [ "$FAILED_WEEKS" -gt 0 ]; then
  echo "⚠️  Some weeks failed. Check individual log files for details."
  echo "📁 Log files: week_N_import.log"
  exit 1
else
  echo "🎉 All weeks completed successfully!"
  if [ -n "$DRY_RUN" ]; then
    echo "🔍 Remember: This was a dry run - no changes were saved to the database"
  fi
fi 