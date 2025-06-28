-- Migration 010 Down: Remove managing team support from matchups
-- 
-- This migration removes the managing_team_id column and related constraints
-- WARNING: This will remove the ability to handle derby matches properly

-- Drop the trigger first
DROP TRIGGER IF EXISTS trg_set_managing_team_id;

-- Drop the unique index
DROP INDEX IF EXISTS idx_matchups_fixture_type_team;

-- Drop the index on managing_team_id
DROP INDEX IF EXISTS idx_matchups_managing_team_id;

-- Note: SQLite doesn't support DROP COLUMN directly in ALTER TABLE
-- To remove the column, we would need to:
-- 1. Create a new table without the column
-- 2. Copy data from old table to new table
-- 3. Drop old table
-- 4. Rename new table
-- 
-- For now, we'll just leave the column but clear its data
-- If you need to fully remove the column, please do it manually

-- Clear the managing_team_id data
UPDATE matchups SET managing_team_id = NULL; 