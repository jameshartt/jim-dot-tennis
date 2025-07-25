-- Drop email and phone columns from players table
-- These contained generated data and will be reimplemented later with proper validation

-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
CREATE TABLE players_new (
    id TEXT PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    club_id INTEGER NOT NULL,
    fantasy_match_id INTEGER REFERENCES fantasy_mixed_doubles(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (club_id) REFERENCES clubs(id)
);

-- Copy data excluding email and phone columns
INSERT INTO players_new (id, first_name, last_name, club_id, fantasy_match_id, created_at, updated_at)
SELECT id, first_name, last_name, club_id, fantasy_match_id, created_at, updated_at
FROM players;

-- Drop old table
DROP TABLE players;

-- Rename new table
ALTER TABLE players_new RENAME TO players;

-- Recreate indexes (excluding the old email unique index)
CREATE INDEX IF NOT EXISTS idx_players_club_id ON players(club_id);
CREATE INDEX IF NOT EXISTS idx_players_fantasy_match_id ON players(fantasy_match_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_fantasy_match_id_unique ON players(fantasy_match_id) WHERE fantasy_match_id IS NOT NULL; 