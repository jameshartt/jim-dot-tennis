-- Restore email and phone columns to players table
CREATE TABLE players_old (
    id TEXT PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(20),
    club_id INTEGER NOT NULL,
    fantasy_match_id INTEGER REFERENCES fantasy_mixed_doubles(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (club_id) REFERENCES clubs(id)
);

-- Copy data back (email and phone will be NULL)
INSERT INTO players_old (id, first_name, last_name, club_id, fantasy_match_id, created_at, updated_at)
SELECT id, first_name, last_name, club_id, fantasy_match_id, created_at, updated_at
FROM players;

-- Drop new table
DROP TABLE players;

-- Rename old table back
ALTER TABLE players_old RENAME TO players;

-- Recreate original indexes
CREATE INDEX IF NOT EXISTS idx_players_club_id ON players(club_id);
CREATE INDEX IF NOT EXISTS idx_players_fantasy_match_id ON players(fantasy_match_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_fantasy_match_id_unique ON players(fantasy_match_id) WHERE fantasy_match_id IS NOT NULL; 