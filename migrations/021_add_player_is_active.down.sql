-- Remove is_active column and index from players table
DROP INDEX IF EXISTS idx_players_is_active;
ALTER TABLE players DROP COLUMN is_active;
