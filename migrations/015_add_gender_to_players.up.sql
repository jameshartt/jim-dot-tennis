-- Add gender column to players table
-- Gender values: 'Men', 'Women', 'Unknown'
ALTER TABLE players ADD COLUMN gender VARCHAR(20) NOT NULL DEFAULT 'Unknown';

-- Create index for gender lookups
CREATE INDEX IF NOT EXISTS idx_players_gender ON players(gender);

-- Update players who have played in Men's matchups
UPDATE players 
SET gender = 'Men'
WHERE id IN (
    SELECT DISTINCT mp.player_id
    FROM matchup_players mp
    JOIN matchups m ON mp.matchup_id = m.id
    WHERE m.type = 'Mens'
);

-- Update players who have played in Women's matchups
-- Note: This will override the 'Men' setting if a player has played both,
-- but in practice this shouldn't happen in a proper tennis league
UPDATE players 
SET gender = 'Women'
WHERE id IN (
    SELECT DISTINCT mp.player_id
    FROM matchup_players mp
    JOIN matchups m ON mp.matchup_id = m.id
    WHERE m.type = 'Womens'
);

-- Players who have only played in mixed matchups or no matchups at all
-- will remain as 'Unknown' (the default value we set above)

-- Add a check constraint to ensure only valid gender values
-- Note: SQLite doesn't support adding constraints to existing tables directly,
-- but we can rely on application-level validation for this 