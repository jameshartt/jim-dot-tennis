-- Migration 012 Down: Remove match card fields for BHPLTA integration
--
-- Reverses 012_add_match_card_fields.up.sql. Historically this down file was
-- missing, which meant any rollback through version 12 failed and left
-- versions 1-11 unreachable.
--
-- Note: SQLite gained ALTER TABLE ... DROP COLUMN in 3.35 (2021); the bundled
-- driver (mattn/go-sqlite3) and PostgreSQL both support the statements below.
-- The index on external_match_card_id must be dropped before its column.

-- Drop the lookup index first (a column cannot be dropped while indexed)
DROP INDEX IF EXISTS idx_fixtures_external_match_card_id;

-- Remove the per-set score columns from matchups
ALTER TABLE matchups DROP COLUMN home_set3;
ALTER TABLE matchups DROP COLUMN away_set3;
ALTER TABLE matchups DROP COLUMN home_set2;
ALTER TABLE matchups DROP COLUMN away_set2;
ALTER TABLE matchups DROP COLUMN home_set1;
ALTER TABLE matchups DROP COLUMN away_set1;

-- Remove the external match card reference from fixtures
ALTER TABLE fixtures DROP COLUMN external_match_card_id;
