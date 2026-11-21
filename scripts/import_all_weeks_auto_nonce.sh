#!/bin/bash

# Auto-Nonce Import script for match cards for all weeks (1-18) in the 2025 season
# Uses automatic nonce extraction - no manual nonce required!
# Usage: ./import_all_weeks_auto_nonce.sh [--dry-run] [--start-week=N] [--end-week=N]

# Default values
DRY_RUN=""
START_WEEK=1
END_WEEK=18
DELAY_SECONDS=2
CLEAR_EXISTING=""
CLUB_CODE="${CLUB_CODE:-}"
CLUB_ID="${CLUB_ID:-}"
CLUB_NAME="${CLUB_NAME:-}"
YEAR=2025
DB_PATH="/app/data/tennis.db"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔧 Auto-Nonce Match Card Import - All Weeks${NC}"
echo "=============================================="
echo ""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN="-dry-run"
      echo -e "${YELLOW}🔍 DRY RUN MODE: No changes will be saved to database${NC}"
      shift
      ;;
    --clear-existing)
      CLEAR_EXISTING="-clear-existing"
      echo -e "${YELLOW}🧹 CLEAR EXISTING MODE: Existing matchups will be cleared before importing${NC}"
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
    --club-code=*)
      CLUB_CODE="${1#*=}"
      shift
      ;;
    --club-id=*)
      CLUB_ID="${1#*=}"
      shift
      ;;
    --club-name=*)
      CLUB_NAME="${1#*=}"
      shift
      ;;
    --year=*)
      YEAR="${1#*=}"
      shift
      ;;
    --db=*)
      DB_PATH="${1#*=}"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo -e "${BLUE}Auto-Nonce Features:${NC}"
      echo "  ✅ No manual nonce extraction required"
      echo "  ✅ Automatic WordPress nonce discovery"
      echo "  ✅ Real-time nonce expiration handling"
      echo ""
      echo -e "${BLUE}Options:${NC}"
      echo "  --dry-run           Run in dry-run mode (no database changes)"
      echo "  --clear-existing    Clear existing matchups before importing"
      echo "  --start-week=N      Start from week N (default: 1)"
      echo "  --end-week=N        End at week N (default: 18)"
      echo "  --delay=N           Delay N seconds between requests (default: 2)"
      echo "  --club-code=CODE    Club code for BHPLTA (or set CLUB_CODE)"
      echo "  --club-id=ID        Club ID (or set CLUB_ID)"
      echo "  --club-name=NAME    Club name (or set CLUB_NAME)"
      echo "  --year=YEAR         Year for import (default: 2025)"
      echo "  --db=PATH           Database path (default: /app/data/tennis.db)"
      echo "  -h, --help          Show this help message"
      echo ""
      echo -e "${GREEN}Examples:${NC}"
      echo "  $0                                    # Import all weeks with auto-nonce"
      echo "  $0 --dry-run                         # Test import without saving"
      echo "  $0 --start-week=5 --end-week=10     # Import weeks 5-10 only"
      echo "  $0 --club-code=MYCLUB001 --year=2024 # Custom club and year"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option $1${NC}"
      exit 1
      ;;
  esac
done

# Club details are required (no defaults): flags or CLUB_CODE / CLUB_ID / CLUB_NAME env vars
if [ -z "$CLUB_CODE" ] || [ -z "$CLUB_ID" ] || [ -z "$CLUB_NAME" ]; then
  echo -e "${RED}❌ Error: --club-code, --club-id and --club-name (or CLUB_CODE, CLUB_ID, CLUB_NAME) are required${NC}"
  exit 1
fi

# Validate week range
if [ "$START_WEEK" -lt 1 ] || [ "$START_WEEK" -gt 18 ] || [ "$END_WEEK" -lt 1 ] || [ "$END_WEEK" -gt 18 ] || [ "$START_WEEK" -gt "$END_WEEK" ]; then
  echo -e "${RED}❌ Error: Week range must be between 1-18 and start-week must be <= end-week${NC}"
  exit 1
fi

echo -e "${BLUE}Configuration:${NC}"
echo "🏆 Importing weeks $START_WEEK to $END_WEEK"
echo "⏱️  Delay between requests: ${DELAY_SECONDS}s"
echo "🏢 Club: $CLUB_NAME (Code: $CLUB_CODE, ID: $CLUB_ID)"
echo "📅 Year: $YEAR"
echo "💾 Database: $DB_PATH"
echo "🔐 Nonce: Automatic extraction enabled"
if [ -n "$CLEAR_EXISTING" ]; then
  echo -e "${YELLOW}🧹 Clear existing matchups: enabled${NC}"
fi
if [ -n "$DRY_RUN" ]; then
  echo -e "${YELLOW}🔍 Dry run mode: enabled${NC}"
fi
echo ""

# Test nonce extraction first
echo -e "${BLUE}Step 1: Testing Auto-Nonce Extraction${NC}"
echo "======================================"
echo -e "${YELLOW}Testing nonce extraction from BHPLTA website...${NC}"

# Build utilities if they don't exist
if [ ! -f "./bin/extract-nonce" ] || [ ! -f "./bin/import-matchcards" ]; then
    echo -e "${YELLOW}Building utilities...${NC}"
    make build-utils
    echo ""
fi

if ./bin/extract-nonce -club-code="$CLUB_CODE" > /tmp/nonce_test.log 2>&1; then
    EXTRACTED_NONCE=$(grep "Nonce:" /tmp/nonce_test.log | cut -d' ' -f2)
    echo -e "${GREEN}✅ Nonce extraction successful: ${EXTRACTED_NONCE:0:10}...${NC}"
else
    echo -e "${RED}❌ Nonce extraction failed${NC}"
    echo ""
    echo "This could be due to:"
    echo "- Network connectivity issues"
    echo "- BHPLTA website changes"
    echo "- Invalid club code: $CLUB_CODE"
    echo ""
    echo "Check the error details:"
    cat /tmp/nonce_test.log
    exit 1
fi

echo ""

# Initialize counters
TOTAL_WEEKS=$((END_WEEK - START_WEEK + 1))
SUCCESSFUL_WEEKS=0
FAILED_WEEKS=0
TOTAL_PROCESSED=0
TOTAL_UPDATED_FIXTURES=0
TOTAL_CREATED_MATCHUPS=0
TOTAL_UPDATED_MATCHUPS=0
TOTAL_MATCHED_PLAYERS=0

echo -e "${BLUE}Step 2: Importing Match Cards${NC}"
echo "=============================="
echo ""

# Loop through all weeks
for week in $(seq $START_WEEK $END_WEEK); do
  echo -e "${BLUE}📅 Processing Week $week ($((week - START_WEEK + 1))/$TOTAL_WEEKS)...${NC}"
  echo "==============================================="
  
  # Run the import command using pre-built binary with auto-nonce
  if ./bin/import-matchcards \
    -auto-nonce \
    -db="$DB_PATH" \
    -club-code="$CLUB_CODE" \
    -week=$week \
    -year=$YEAR \
    -club-id=$CLUB_ID \
    -club-name="$CLUB_NAME" \
    -verbose $DRY_RUN $CLEAR_EXISTING 2>&1 | tee "week_${week}_import.log"; then
    
    echo -e "${GREEN}✅ Week $week completed successfully${NC}"
    SUCCESSFUL_WEEKS=$((SUCCESSFUL_WEEKS + 1))
    
    # Extract statistics from the log file (enhanced parsing)
    if [ -f "week_${week}_import.log" ]; then
      PROCESSED=$(grep "Processed matches:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      FIXTURES=$(grep "Updated fixtures:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      CREATED_MATCHUPS=$(grep "Created matchups:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      MATCHUPS=$(grep "Updated matchups:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      PLAYERS=$(grep "Matched players:" "week_${week}_import.log" | tail -1 | sed 's/.*: //')
      
      # Add to totals (with error handling for non-numeric values)
      if [[ "$PROCESSED" =~ ^[0-9]+$ ]]; then
        TOTAL_PROCESSED=$((TOTAL_PROCESSED + PROCESSED))
      fi
      if [[ "$FIXTURES" =~ ^[0-9]+$ ]]; then
        TOTAL_UPDATED_FIXTURES=$((TOTAL_UPDATED_FIXTURES + FIXTURES))
      fi
      if [[ "$CREATED_MATCHUPS" =~ ^[0-9]+$ ]]; then
        TOTAL_CREATED_MATCHUPS=$((TOTAL_CREATED_MATCHUPS + CREATED_MATCHUPS))
      fi
      if [[ "$MATCHUPS" =~ ^[0-9]+$ ]]; then
        TOTAL_UPDATED_MATCHUPS=$((TOTAL_UPDATED_MATCHUPS + MATCHUPS))
      fi
      if [[ "$PLAYERS" =~ ^[0-9]+$ ]]; then
        TOTAL_MATCHED_PLAYERS=$((TOTAL_MATCHED_PLAYERS + PLAYERS))
      fi
      
      echo -e "${GREEN}   📊 Week $week stats: $PROCESSED matches, $FIXTURES fixtures, $CREATED_MATCHUPS/$MATCHUPS matchups, $PLAYERS players${NC}"
    fi
  else
    echo -e "${RED}❌ Week $week failed${NC}"
    FAILED_WEEKS=$((FAILED_WEEKS + 1))
    
    # Show error details
    if [ -f "week_${week}_import.log" ]; then
      echo -e "${YELLOW}   Error details:${NC}"
      tail -5 "week_${week}_import.log" | sed 's/^/   /'
    fi
  fi
  
  echo ""
  
  # Add delay between requests (except for the last week)
  if [ "$week" -ne "$END_WEEK" ]; then
    echo -e "${YELLOW}⏳ Waiting ${DELAY_SECONDS}s before next request...${NC}"
    sleep $DELAY_SECONDS
    echo ""
  fi
done

# Final summary
echo -e "${BLUE}🎯 FINAL SUMMARY${NC}"
echo "==============================================="
echo -e "${BLUE}📊 Total weeks processed: $TOTAL_WEEKS${NC}"
echo -e "${GREEN}✅ Successful weeks: $SUCCESSFUL_WEEKS${NC}"
if [ "$FAILED_WEEKS" -gt 0 ]; then
  echo -e "${RED}❌ Failed weeks: $FAILED_WEEKS${NC}"
else
  echo -e "${GREEN}❌ Failed weeks: $FAILED_WEEKS${NC}"
fi
echo ""
echo -e "${BLUE}📈 AGGREGATE STATISTICS:${NC}"
echo -e "${GREEN}   🏆 Total matches processed: $TOTAL_PROCESSED${NC}"
echo -e "${GREEN}   📋 Total fixtures updated: $TOTAL_UPDATED_FIXTURES${NC}"
echo -e "${GREEN}   🎾 Total matchups created: $TOTAL_CREATED_MATCHUPS${NC}"
echo -e "${GREEN}   🎾 Total matchups updated: $TOTAL_UPDATED_MATCHUPS${NC}"
echo -e "${GREEN}   👥 Total players matched: $TOTAL_MATCHED_PLAYERS${NC}"
echo ""

# Cleanup temp files
rm -f /tmp/nonce_test.log

if [ "$FAILED_WEEKS" -gt 0 ]; then
  echo -e "${YELLOW}⚠️  Some weeks failed. Check individual log files for details.${NC}"
  echo -e "${BLUE}📁 Log files: week_N_import.log${NC}"
  exit 1
else
  echo -e "${GREEN}🎉 All weeks completed successfully!${NC}"
  if [ -n "$DRY_RUN" ]; then
    echo -e "${YELLOW}🔍 Remember: This was a dry run - no changes were saved to the database${NC}"
  fi
  echo ""
  echo -e "${GREEN}✨ Benefits of Auto-Nonce:${NC}"
  echo -e "${GREEN}   ✅ No manual browser inspection needed${NC}"
  echo -e "${GREEN}   ✅ No copying nonces from developer tools${NC}"
  echo -e "${GREEN}   ✅ Automatic nonce refresh on expiration${NC}"
  echo -e "${GREEN}   ✅ Works in automated environments${NC}"
fi 