-- Migration: Add match card fields for BHPLTA integration
-- Date: 2025-01-01

-- Add external match card ID to fixtures table
ALTER TABLE fixtures ADD COLUMN external_match_card_id INTEGER;

-- Add individual set scores to matchups table for 3-set tennis
ALTER TABLE matchups ADD COLUMN home_set1 INTEGER;
ALTER TABLE matchups ADD COLUMN away_set1 INTEGER;
ALTER TABLE matchups ADD COLUMN home_set2 INTEGER;
ALTER TABLE matchups ADD COLUMN away_set2 INTEGER;
ALTER TABLE matchups ADD COLUMN home_set3 INTEGER;
ALTER TABLE matchups ADD COLUMN away_set3 INTEGER;

-- Add index on external_match_card_id for quick lookups
CREATE INDEX IF NOT EXISTS idx_fixtures_external_match_card_id ON fixtures(external_match_card_id);
