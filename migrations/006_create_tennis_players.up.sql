-- Create tennis_players table for ATP/WTA professional players
CREATE TABLE IF NOT EXISTS tennis_players (
    id INTEGER PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    common_name TEXT NOT NULL,
    nationality TEXT NOT NULL,
    gender TEXT NOT NULL,
    current_rank INTEGER NOT NULL,
    highest_rank INTEGER NOT NULL,
    year_pro INTEGER NOT NULL,
    wikipedia_url TEXT NOT NULL,
    hand TEXT NOT NULL,
    birth_date TEXT NOT NULL,
    birth_place TEXT NOT NULL,
    tour TEXT NOT NULL, -- "ATP" or "WTA"
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create fantasy_mixed_doubles table for player authentication
CREATE TABLE IF NOT EXISTS fantasy_mixed_doubles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    team_a_woman_id INTEGER NOT NULL, -- WTA player on Team A
    team_a_man_id INTEGER NOT NULL,   -- ATP player on Team A
    team_b_woman_id INTEGER NOT NULL, -- WTA player on Team B
    team_b_man_id INTEGER NOT NULL,   -- ATP player on Team B
    auth_token TEXT NOT NULL UNIQUE, -- Concatenated surnames with underscore
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (team_a_woman_id) REFERENCES tennis_players(id),
    FOREIGN KEY (team_a_man_id) REFERENCES tennis_players(id),
    FOREIGN KEY (team_b_woman_id) REFERENCES tennis_players(id),
    FOREIGN KEY (team_b_man_id) REFERENCES tennis_players(id),
    -- Ensure all players are unique in the match
    CHECK (
        team_a_woman_id != team_a_man_id AND
        team_a_woman_id != team_b_woman_id AND
        team_a_woman_id != team_b_man_id AND
        team_a_man_id != team_b_woman_id AND
        team_a_man_id != team_b_man_id AND
        team_b_woman_id != team_b_man_id
    )
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_tennis_players_tour ON tennis_players(tour);
CREATE INDEX IF NOT EXISTS idx_tennis_players_gender ON tennis_players(gender);
CREATE INDEX IF NOT EXISTS idx_tennis_players_current_rank ON tennis_players(current_rank);
CREATE INDEX IF NOT EXISTS idx_tennis_players_nationality ON tennis_players(nationality);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_auth_token ON fantasy_mixed_doubles(auth_token);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_team_a_woman_id ON fantasy_mixed_doubles(team_a_woman_id);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_team_a_man_id ON fantasy_mixed_doubles(team_a_man_id);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_team_b_woman_id ON fantasy_mixed_doubles(team_b_woman_id);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_team_b_man_id ON fantasy_mixed_doubles(team_b_man_id);
CREATE INDEX IF NOT EXISTS idx_fantasy_mixed_doubles_is_active ON fantasy_mixed_doubles(is_active);

-- Trigger to update the updated_at column for tennis_players
CREATE TRIGGER IF NOT EXISTS update_tennis_players_updated_at
AFTER UPDATE ON tennis_players
FOR EACH ROW
BEGIN
    UPDATE tennis_players SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update the updated_at column for fantasy_mixed_doubles
CREATE TRIGGER IF NOT EXISTS update_fantasy_mixed_doubles_updated_at
AFTER UPDATE ON fantasy_mixed_doubles
FOR EACH ROW
BEGIN
    UPDATE fantasy_mixed_doubles SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END; 