-- Remove the fantasy_match_id column from players table
-- Drop the indexes first
DROP INDEX IF EXISTS idx_players_fantasy_match_id_unique;
DROP INDEX IF EXISTS idx_players_fantasy_match_id;

-- Drop the column
ALTER TABLE players DROP COLUMN fantasy_match_id;