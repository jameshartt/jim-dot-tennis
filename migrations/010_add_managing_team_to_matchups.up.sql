-- Migration 010: Add managing team support to matchups for derby matches
-- 
-- This migration adds a managing_team_id column to the matchups table
-- to support derby matches where both teams belong to the same club.
-- For regular matches, this will be the St Ann's team ID.
-- For derby matches, this distinguishes which team's perspective this matchup represents.

-- Add managing_team_id column to matchups table
ALTER TABLE matchups ADD COLUMN managing_team_id INTEGER;

-- Add foreign key constraint
-- Note: SQLite doesn't support adding foreign key constraints to existing tables,
-- so we'll handle this constraint in the application layer for now

-- Add index for better query performance
CREATE INDEX IF NOT EXISTS idx_matchups_managing_team_id ON matchups(managing_team_id);

-- Create a unique constraint for (fixture_id, type, managing_team_id)
-- This allows multiple matchups of the same type per fixture for derby matches
CREATE UNIQUE INDEX IF NOT EXISTS idx_matchups_fixture_type_team 
ON matchups(fixture_id, type, managing_team_id);

-- Update existing matchups to have a managing_team_id
-- For existing matchups, we'll determine the managing team based on which team is St Ann's
UPDATE matchups 
SET managing_team_id = (
    SELECT CASE 
        WHEN EXISTS (
            SELECT 1 FROM teams t 
            JOIN clubs c ON t.club_id = c.id 
            JOIN fixtures f ON f.home_team_id = t.id 
            WHERE f.id = matchups.fixture_id 
            AND c.name LIKE '%St Ann%'
        ) THEN (
            SELECT f.home_team_id 
            FROM fixtures f 
            JOIN teams t ON f.home_team_id = t.id 
            JOIN clubs c ON t.club_id = c.id 
            WHERE f.id = matchups.fixture_id 
            AND c.name LIKE '%St Ann%'
        )
        WHEN EXISTS (
            SELECT 1 FROM teams t 
            JOIN clubs c ON t.club_id = c.id 
            JOIN fixtures f ON f.away_team_id = t.id 
            WHERE f.id = matchups.fixture_id 
            AND c.name LIKE '%St Ann%'
        ) THEN (
            SELECT f.away_team_id 
            FROM fixtures f 
            JOIN teams t ON f.away_team_id = t.id 
            JOIN clubs c ON t.club_id = c.id 
            WHERE f.id = matchups.fixture_id 
            AND c.name LIKE '%St Ann%'
        )
        ELSE NULL
    END
)
WHERE managing_team_id IS NULL;

-- Add a trigger to automatically set managing_team_id for new matchups if not provided
CREATE TRIGGER IF NOT EXISTS trg_set_managing_team_id
AFTER INSERT ON matchups
FOR EACH ROW
WHEN NEW.managing_team_id IS NULL
BEGIN
    UPDATE matchups 
    SET managing_team_id = (
        SELECT CASE 
            WHEN EXISTS (
                SELECT 1 FROM teams t 
                JOIN clubs c ON t.club_id = c.id 
                JOIN fixtures f ON f.home_team_id = t.id 
                WHERE f.id = NEW.fixture_id 
                AND c.name LIKE '%St Ann%'
            ) THEN (
                SELECT f.home_team_id 
                FROM fixtures f 
                JOIN teams t ON f.home_team_id = t.id 
                JOIN clubs c ON t.club_id = c.id 
                WHERE f.id = NEW.fixture_id 
                AND c.name LIKE '%St Ann%'
            )
            WHEN EXISTS (
                SELECT 1 FROM teams t 
                JOIN clubs c ON t.club_id = c.id 
                JOIN fixtures f ON f.away_team_id = t.id 
                WHERE f.id = NEW.fixture_id 
                AND c.name LIKE '%St Ann%'
            ) THEN (
                SELECT f.away_team_id 
                FROM fixtures f 
                JOIN teams t ON f.away_team_id = t.id 
                JOIN clubs c ON t.club_id = c.id 
                WHERE f.id = NEW.fixture_id 
                AND c.name LIKE '%St Ann%'
            )
            ELSE NULL
        END
    )
    WHERE id = NEW.id;
END; 