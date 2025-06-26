#!/bin/bash

# Tennis Player Import Script for Production
# This script builds the import tool and deploys it to the production server

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

PRODUCTION_SERVER="144.126.228.64"
PRODUCTION_USER="root"
PRODUCTION_PATH="/opt/jim-dot-tennis"
BINARY_NAME="import-tennis-players"
JSON_FILE="cmd/collect_tennis_data/tennis_players.json"

echo -e "${BLUE}🎾 Tennis Player Import Deployment Script${NC}"
echo

# Check if JSON file exists locally
if [ ! -f "$JSON_FILE" ]; then
    echo -e "${RED}❌ Tennis players JSON file not found: $JSON_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}📊 Checking JSON file...${NC}"
PLAYER_COUNT=$(grep -o '"id":' "$JSON_FILE" | wc -l)
echo -e "${GREEN}✓ Found $PLAYER_COUNT players in JSON file${NC}"

# Build the binary locally
echo -e "${YELLOW}🔨 Building import tool locally...${NC}"
go build -o "bin/$BINARY_NAME" "./cmd/import-tennis-players"
echo -e "${GREEN}✓ Binary built successfully${NC}"

# Transfer files to production server
echo -e "${YELLOW}📤 Transferring files to production server...${NC}"
scp "bin/$BINARY_NAME" "$PRODUCTION_USER@$PRODUCTION_SERVER:$PRODUCTION_PATH/"
scp "$JSON_FILE" "$PRODUCTION_USER@$PRODUCTION_SERVER:$PRODUCTION_PATH/"
echo -e "${GREEN}✓ Files transferred successfully${NC}"

# Make binary executable on production server
echo -e "${YELLOW}🔧 Setting up permissions on production server...${NC}"
ssh "$PRODUCTION_USER@$PRODUCTION_SERVER" "cd $PRODUCTION_PATH && chmod +x $BINARY_NAME"
echo -e "${GREEN}✓ Permissions set${NC}"

# Check current state on production (dry run)
echo -e "${YELLOW}🔍 Checking current state on production server...${NC}"
ssh "$PRODUCTION_USER@$PRODUCTION_SERVER" "cd $PRODUCTION_PATH && ./$BINARY_NAME -db-path /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db -json-file tennis_players.json -dry-run -verbose"

# Ask user for confirmation
echo
echo -e "${YELLOW}⚠️  Ready to import tennis players to production database${NC}"
echo -e "${BLUE}This will:${NC}"
echo -e "  • Clear existing tennis players (if any)"
echo -e "  • Import $PLAYER_COUNT new players (100 ATP + 100 WTA)"
echo -e "  • Update production database with tennis player data"
echo
read -p "Continue with import? (y/N): " -n 1 -r
echo

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}🚀 Starting import on production server...${NC}"
    
    # Run the actual import
    ssh "$PRODUCTION_USER@$PRODUCTION_SERVER" "cd $PRODUCTION_PATH && ./$BINARY_NAME -db-path /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db -json-file tennis_players.json -verbose"
    
    echo -e "${GREEN}✅ Import completed successfully!${NC}"
    echo -e "${BLUE}🔗 Check your application at: http://$PRODUCTION_SERVER${NC}"
else
    echo -e "${YELLOW}❌ Import cancelled by user${NC}"
fi

# Clean up local binary
echo -e "${YELLOW}🧹 Cleaning up local build artifacts...${NC}"
rm -f "bin/$BINARY_NAME"
echo -e "${GREEN}✓ Cleanup complete${NC}"

echo
echo -e "${GREEN}🎾 Tennis player import process complete!${NC}" 