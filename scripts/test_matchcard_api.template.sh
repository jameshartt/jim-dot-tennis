#!/bin/bash

# Test script to verify match card API connection
# This script tests a single week import to verify credentials work

# === CREDENTIALS CONFIGURATION ===
# Replace these with your actual values from browser cookies/network requests
NONCE="YOUR_NONCE_HERE"
CLUB_CODE="YOUR_CLUB_CODE_HERE"

# Configuration
WEEK=1
YEAR=2025
CLUB_ID=10
CLUB_NAME="St+Anns"

echo "🏆 Testing Match Card API Connection"
echo "==============================================="
echo "Testing week $WEEK for $CLUB_NAME (ID: $CLUB_ID)"
echo "Year: $YEAR"
echo "Nonce: ${NONCE:0:10}..."
echo ""

# Run the import command in verbose mode with dry-run
echo "Running import test (dry-run mode)..."
go run ../cmd/import-matchcards/main.go \
  -db="../tennis.db" \
  -nonce="$NONCE" \
  -club-code="$CLUB_CODE" \
  -week=$WEEK \
  -year=$YEAR \
  -club-id=$CLUB_ID \
  -club-name="$CLUB_NAME" \
  -verbose \
  -dry-run

exit_code=$?

echo ""
echo "==============================================="
if [ $exit_code -eq 0 ]; then
  echo "✅ API test completed successfully!"
  echo "Your credentials appear to be working correctly."
else
  echo "❌ API test failed!"
  echo "Check your credentials and try again."
fi 