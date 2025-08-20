-- Add conceded_by field to matchups to record which side conceded (Home/Away)
ALTER TABLE matchups ADD COLUMN conceded_by VARCHAR(10);

-- Optional check constraint for allowed values
CREATE TRIGGER IF NOT EXISTS chk_matchups_conceded_by_insert
BEFORE INSERT ON matchups
FOR EACH ROW
WHEN NEW.conceded_by IS NOT NULL AND NEW.conceded_by NOT IN ('Home', 'Away')
BEGIN
    SELECT RAISE(FAIL, 'Invalid conceded_by: must be Home or Away');
END;

CREATE TRIGGER IF NOT EXISTS chk_matchups_conceded_by_update
BEFORE UPDATE ON matchups
FOR EACH ROW
WHEN NEW.conceded_by IS NOT NULL AND NEW.conceded_by NOT IN ('Home', 'Away')
BEGIN
    SELECT RAISE(FAIL, 'Invalid conceded_by: must be Home or Away');
END;

CREATE INDEX IF NOT EXISTS idx_matchups_conceded_by ON matchups(conceded_by);


