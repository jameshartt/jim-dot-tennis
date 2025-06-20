-- Add fantasy_match_id column to players table to link players to fantasy mixed doubles matches
ALTER TABLE players ADD COLUMN fantasy_match_id INTEGER REFERENCES fantasy_mixed_doubles(id);

-- Create unique index to enforce the unique constraint
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_fantasy_match_id_unique ON players(fantasy_match_id) WHERE fantasy_match_id IS NOT NULL;

-- Create regular index for better query performance on fantasy_match_id
CREATE INDEX IF NOT EXISTS idx_players_fantasy_match_id ON players(fantasy_match_id);