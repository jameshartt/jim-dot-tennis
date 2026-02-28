#!/bin/sh
# Seed the test database with E2E test data.
# Runs inside the e2e container with access to the shared data volume.
set -e

DB_PATH="${DB_PATH:-/data/tennis.db}"
SEED_SQL="$(dirname "$0")/seed.sql"

echo "Seeding test database at ${DB_PATH}..."

if [ ! -f "$DB_PATH" ]; then
  echo "ERROR: Database file not found at ${DB_PATH}"
  echo "Make sure the app container has started and run migrations."
  exit 1
fi

sqlite3 "$DB_PATH" < "$SEED_SQL"

echo "Database seeded successfully."
