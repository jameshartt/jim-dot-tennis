-- Migration 011 Up: Add managing team support to fixture_players
-- 
-- This migration adds the managing_team_id column to the fixture_players table to support
-- derby matches where both teams are from the same club. This allows us to distinguish
-- which team's perspective the player selection is for.

-- Add the managing_team_id column to fixture_players
ALTER TABLE fixture_players ADD COLUMN managing_team_id INTEGER REFERENCES teams(id);

-- Create index for performance
CREATE INDEX idx_fixture_players_managing_team_id ON fixture_players(managing_team_id);

-- Create unique constraint to prevent duplicate player selections for the same team in a fixture
-- This replaces the potential for duplicate selections by ensuring each player can only be 
-- selected once per team per fixture
CREATE UNIQUE INDEX idx_fixture_players_fixture_player_team 
ON fixture_players(fixture_id, player_id, managing_team_id);

-- Create a trigger to automatically set managing_team_id for new fixture_players
-- This ensures backward compatibility and automatically populates the field
CREATE TRIGGER trg_set_fixture_player_managing_team_id
AFTER INSERT ON fixture_players
FOR EACH ROW
WHEN NEW.managing_team_id IS NULL
BEGIN
    UPDATE fixture_players 
    SET managing_team_id = (
        CASE 
            WHEN NEW.is_home = 1 THEN (SELECT home_team_id FROM fixtures WHERE id = NEW.fixture_id)
            ELSE (SELECT away_team_id FROM fixtures WHERE id = NEW.fixture_id)
        END
    )
    WHERE id = NEW.id;
END;

-- Populate existing records with managing_team_id based on is_home flag
UPDATE fixture_players 
SET managing_team_id = (
    SELECT CASE 
        WHEN fixture_players.is_home = 1 THEN f.home_team_id 
        ELSE f.away_team_id 
    END
    FROM fixtures f 
    WHERE f.id = fixture_players.fixture_id
)
WHERE managing_team_id IS NULL; 