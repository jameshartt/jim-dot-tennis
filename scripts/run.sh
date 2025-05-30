#!/bin/bash

# Jim-dot-tennis local runner script
# This script builds the binary and runs it with the database at the project root

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🎾 Jim-dot-tennis Local Runner${NC}"
echo

# Get script directory (project root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY_PATH="$PROJECT_ROOT/bin/jim-dot-tennis"

cd "$PROJECT_ROOT"

# Build the binary
echo -e "${YELLOW}Building binary...${NC}"
mkdir -p bin
go build -o "$BINARY_PATH" ./cmd/jim-dot-tennis

echo -e "${GREEN}✓ Binary built successfully at $BINARY_PATH${NC}"
echo

# Set environment variables
export DB_PATH="./tennis.db"
export APP_ENV="development"

echo -e "${YELLOW}Starting jim-dot-tennis...${NC}"
echo -e "${BLUE}Database location: $PROJECT_ROOT/tennis.db${NC}"
echo -e "${BLUE}Server will be available at: http://localhost:8080${NC}"
echo -e "${BLUE}Press Ctrl+C to stop${NC}"
echo

# Run the application
"$BINARY_PATH" 