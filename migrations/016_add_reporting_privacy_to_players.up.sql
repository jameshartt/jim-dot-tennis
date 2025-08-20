-- Add reporting privacy column to players table
-- This controls whether a player appears on public reports like the points table
-- States: 'visible', 'hidden'
-- Default: 'visible'

ALTER TABLE players ADD COLUMN reporting_privacy VARCHAR(20) NOT NULL DEFAULT 'visible';

-- Create index for reporting privacy lookups
CREATE INDEX IF NOT EXISTS idx_players_reporting_privacy ON players(reporting_privacy);

-- Add constraint to ensure only valid values
-- SQLite doesn't support CHECK constraints in ALTER TABLE, so we'll use a trigger
CREATE TRIGGER check_reporting_privacy_insert
    BEFORE INSERT ON players
    FOR EACH ROW
    WHEN NEW.reporting_privacy NOT IN ('visible', 'hidden')
BEGIN
    SELECT RAISE(ABORT, 'reporting_privacy must be either visible or hidden');
END;

CREATE TRIGGER check_reporting_privacy_update
    BEFORE UPDATE OF reporting_privacy ON players
    FOR EACH ROW
    WHEN NEW.reporting_privacy NOT IN ('visible', 'hidden')
BEGIN
    SELECT RAISE(ABORT, 'reporting_privacy must be either visible or hidden');
END; 