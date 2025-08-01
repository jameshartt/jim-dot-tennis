#!/bin/bash

# Demo script for automatic nonce extraction
# This shows how to use the new nonce extraction features

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔧 Tennis Match Card Import - Auto Nonce Demo${NC}"
echo "=============================================="
echo ""

# Check if club code is provided
if [ -z "$1" ]; then
    echo -e "${YELLOW}Usage: $0 <club-code> [week] [year]${NC}"
    echo ""
    echo "Examples:"
    echo "  $0 STANN001        # Extract nonce for St Ann's"
    echo "  $0 STANN001 1 2024 # Extract nonce and import week 1 of 2024"
    echo ""
    exit 1
fi

CLUB_CODE="$1"
WEEK="${2:-1}"
YEAR="${3:-2024}"

echo -e "${BLUE}Configuration:${NC}"
echo "  Club Code: $CLUB_CODE"
echo "  Week: $WEEK"
echo "  Year: $YEAR"
echo ""

# Build utilities if they don't exist
if [ ! -f "./bin/extract-nonce" ] || [ ! -f "./bin/import-matchcards" ]; then
    echo -e "${YELLOW}Building utilities...${NC}"
    make build-utils
    echo ""
fi

# Step 1: Test nonce extraction
echo -e "${BLUE}Step 1: Testing Nonce Extraction${NC}"
echo "=================================="
echo ""

echo -e "${YELLOW}Extracting nonce from BHPLTA website...${NC}"
if ./bin/extract-nonce -club-code="$CLUB_CODE" -verbose; then
    echo -e "${GREEN}✅ Nonce extraction successful!${NC}"
else
    echo -e "${RED}❌ Nonce extraction failed${NC}"
    echo ""
    echo "This could be due to:"
    echo "- Network connectivity issues"
    echo "- BHPLTA website changes"
    echo "- Invalid club code"
    echo ""
    echo "You can still use manual nonce extraction as a fallback."
    exit 1
fi

echo ""

# Step 2: Show how to use with import
echo -e "${BLUE}Step 2: Using Auto-Nonce with Import${NC}"
echo "===================================="
echo ""

echo -e "${YELLOW}The following command would import match cards with automatic nonce extraction:${NC}"
echo ""
echo -e "${GREEN}./bin/import-matchcards \\${NC}"
echo -e "${GREEN}  -auto-nonce \\${NC}"
echo -e "${GREEN}  -club-code=\"$CLUB_CODE\" \\${NC}"
echo -e "${GREEN}  -week=$WEEK \\${NC}"
echo -e "${GREEN}  -year=$YEAR \\${NC}"
echo -e "${GREEN}  -club-id=123 \\${NC}"
echo -e "${GREEN}  -club-name=\"Your Club Name\" \\${NC}"
echo -e "${GREEN}  -db=\"./tennis.db\" \\${NC}"
echo -e "${GREEN}  -verbose \\${NC}"
echo -e "${GREEN}  -dry-run${NC}"
echo ""

# Step 3: Alternative approaches
echo -e "${BLUE}Step 3: Alternative Approaches${NC}"
echo "==============================="
echo ""

echo -e "${YELLOW}1. Auto-nonce without explicit flag (when nonce is empty):${NC}"
echo "./bin/import-matchcards -club-code=\"$CLUB_CODE\" -week=$WEEK -year=$YEAR ..."
echo ""

echo -e "${YELLOW}2. Manual nonce (traditional approach):${NC}"
echo "./bin/import-matchcards -nonce=\"your-manual-nonce\" -club-code=\"$CLUB_CODE\" ..."
echo ""

echo -e "${YELLOW}3. Extract nonce once and reuse:${NC}"
echo "NONCE=\$(./bin/extract-nonce -club-code=\"$CLUB_CODE\" | grep 'Nonce:' | cut -d' ' -f2)"
echo "./bin/import-matchcards -nonce=\"\$NONCE\" -club-code=\"$CLUB_CODE\" ..."
echo ""

# Step 4: Benefits
echo -e "${BLUE}Step 4: Benefits of Auto-Nonce${NC}"
echo "==============================="
echo ""
echo -e "${GREEN}✅ No more manual browser inspection${NC}"
echo -e "${GREEN}✅ No more copying nonces from developer tools${NC}"
echo -e "${GREEN}✅ Works in automated scripts and CI/CD${NC}"
echo -e "${GREEN}✅ Automatically handles nonce expiration${NC}"
echo -e "${GREEN}✅ Fallback to manual nonce if needed${NC}"
echo ""

echo -e "${BLUE}Demo completed successfully! 🎉${NC}" 