-- Migration 031 down: restore trg_set_managing_team_id exactly as created in migration 010.

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
