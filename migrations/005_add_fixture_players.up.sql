-- Add fixture_players table for player selection before matchup assignment
CREATE TABLE IF NOT EXISTS fixture_players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fixture_id INTEGER NOT NULL,
    player_id TEXT NOT NULL,
    is_home BOOLEAN NOT NULL, -- true for home team, false for away team
    position INTEGER NOT NULL DEFAULT 0, -- Order of selection (1-8 typically)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(fixture_id, player_id), -- A player can only be selected once per fixture
    FOREIGN KEY (fixture_id) REFERENCES fixtures(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_fixture_players_fixture_id ON fixture_players(fixture_id);
CREATE INDEX IF NOT EXISTS idx_fixture_players_player_id ON fixture_players(player_id);
CREATE INDEX IF NOT EXISTS idx_fixture_players_is_home ON fixture_players(is_home);
CREATE INDEX IF NOT EXISTS idx_fixture_players_position ON fixture_players(position);

-- Trigger to update the updated_at column
CREATE TRIGGER IF NOT EXISTS update_fixture_players_updated_at
AFTER UPDATE ON fixture_players
FOR EACH ROW
BEGIN
    UPDATE fixture_players SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END; 