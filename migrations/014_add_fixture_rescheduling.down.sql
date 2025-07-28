-- Rollback fixture rescheduling fields

-- Drop triggers first
DROP TRIGGER IF EXISTS chk_fixture_rescheduled_reason_update;
DROP TRIGGER IF EXISTS chk_fixture_rescheduled_reason;

-- Drop indexes
DROP INDEX IF EXISTS idx_fixtures_previous_dates;
DROP INDEX IF EXISTS idx_fixtures_rescheduled_reason;

-- Remove columns from fixtures table
ALTER TABLE fixtures DROP COLUMN rescheduled_reason;
ALTER TABLE fixtures DROP COLUMN previous_dates; 