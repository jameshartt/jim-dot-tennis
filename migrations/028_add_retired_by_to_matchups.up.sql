-- Add retired_by field to matchups to record which side retired (Home/Away).
-- Distinct from conceded_by: retirement means play started but stopped mid-match,
-- so set scores can still be present and the non-retiring side is credited with
-- a full match win plus both sets in the points table.
ALTER TABLE matchups ADD COLUMN retired_by VARCHAR(10);

CREATE TRIGGER IF NOT EXISTS chk_matchups_retired_by_insert
BEFORE INSERT ON matchups
FOR EACH ROW
WHEN NEW.retired_by IS NOT NULL AND NEW.retired_by NOT IN ('Home', 'Away')
BEGIN
    SELECT RAISE(FAIL, 'Invalid retired_by: must be Home or Away');
END;

CREATE TRIGGER IF NOT EXISTS chk_matchups_retired_by_update
BEFORE UPDATE ON matchups
FOR EACH ROW
WHEN NEW.retired_by IS NOT NULL AND NEW.retired_by NOT IN ('Home', 'Away')
BEGIN
    SELECT RAISE(FAIL, 'Invalid retired_by: must be Home or Away');
END;

CREATE INDEX IF NOT EXISTS idx_matchups_retired_by ON matchups(retired_by);
