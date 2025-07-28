-- Add fixture rescheduling fields to fixtures table

-- Add previous_dates column to store JSON array of previous scheduled dates
ALTER TABLE fixtures ADD COLUMN previous_dates TEXT DEFAULT '[]';

-- Add rescheduled_reason enum column
ALTER TABLE fixtures ADD COLUMN rescheduled_reason VARCHAR(20);

-- Create check constraint for rescheduled_reason values
CREATE TRIGGER IF NOT EXISTS chk_fixture_rescheduled_reason
BEFORE INSERT ON fixtures
FOR EACH ROW
WHEN NEW.rescheduled_reason IS NOT NULL AND NEW.rescheduled_reason NOT IN ('Weather', 'CourtAvailability', 'Other')
BEGIN
    SELECT RAISE(FAIL, 'Invalid rescheduled reason: must be Weather, CourtAvailability, or Other');
END;

CREATE TRIGGER IF NOT EXISTS chk_fixture_rescheduled_reason_update
BEFORE UPDATE ON fixtures
FOR EACH ROW
WHEN NEW.rescheduled_reason IS NOT NULL AND NEW.rescheduled_reason NOT IN ('Weather', 'CourtAvailability', 'Other')
BEGIN
    SELECT RAISE(FAIL, 'Invalid rescheduled reason: must be Weather, CourtAvailability, or Other');
END;

-- Create index for rescheduled_reason for queries
CREATE INDEX IF NOT EXISTS idx_fixtures_rescheduled_reason ON fixtures(rescheduled_reason);

-- Create index for previous_dates JSON queries (SQLite 3.38+)
-- Note: This will be a no-op in older SQLite versions but won't cause errors
CREATE INDEX IF NOT EXISTS idx_fixtures_previous_dates ON fixtures(previous_dates); 