-- Remove reporting privacy functionality from players table

-- Drop the triggers first
DROP TRIGGER IF EXISTS check_reporting_privacy_insert;
DROP TRIGGER IF EXISTS check_reporting_privacy_update;

-- Drop the index
DROP INDEX IF EXISTS idx_players_reporting_privacy;

-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- Create new table without reporting_privacy column
CREATE TABLE players_new (
    id VARCHAR(36) PRIMARY KEY,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    preferred_name VARCHAR(100),
    gender VARCHAR(20) NOT NULL DEFAULT 'Unknown',
    club_id INTEGER NOT NULL,
    fantasy_match_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (club_id) REFERENCES clubs(id),
    FOREIGN KEY (fantasy_match_id) REFERENCES fantasy_mixed_doubles(id)
);

-- Copy data from old table to new (excluding reporting_privacy)
INSERT INTO players_new (id, first_name, last_name, preferred_name, gender, club_id, fantasy_match_id, created_at, updated_at)
SELECT id, first_name, last_name, preferred_name, gender, club_id, fantasy_match_id, created_at, updated_at
FROM players;

-- Drop old table
DROP TABLE players;

-- Rename new table to original name
ALTER TABLE players_new RENAME TO players;

-- Recreate the relevant indexes
CREATE INDEX IF NOT EXISTS idx_players_club_id ON players(club_id);
CREATE INDEX IF NOT EXISTS idx_players_fantasy_match_id ON players(fantasy_match_id);
CREATE INDEX IF NOT EXISTS idx_players_gender ON players(gender); 