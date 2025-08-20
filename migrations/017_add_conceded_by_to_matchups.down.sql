-- Remove conceded_by field and related triggers
DROP INDEX IF EXISTS idx_matchups_conceded_by;

DROP TRIGGER IF EXISTS chk_matchups_conceded_by_insert;
DROP TRIGGER IF EXISTS chk_matchups_conceded_by_update;

ALTER TABLE matchups DROP COLUMN conceded_by;


