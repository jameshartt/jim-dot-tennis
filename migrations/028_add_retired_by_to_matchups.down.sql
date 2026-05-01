DROP INDEX IF EXISTS idx_matchups_retired_by;

DROP TRIGGER IF EXISTS chk_matchups_retired_by_insert;
DROP TRIGGER IF EXISTS chk_matchups_retired_by_update;

ALTER TABLE matchups DROP COLUMN retired_by;
