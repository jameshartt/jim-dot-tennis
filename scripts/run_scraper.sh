#!/bin/bash

# Script to run the tennis league data scraper

# Exit on error
set -e

# Create bin directory if it doesn't exist
mkdir -p bin

# Download dependencies 
echo "Downloading dependencies..."
go mod tidy

# Build the scraper
echo "Building scraper..."
go build -o bin/scraper cmd/scraper/main.go

# Run the scraper with default settings
echo "Running scraper..."
./bin/scraper

echo "Data import completed." 