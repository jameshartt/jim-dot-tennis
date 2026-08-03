-- Add two fixture statuses that surface the reschedule lifecycle:
--   'Rescheduled'        - a still-to-be-played fixture that a captain moved to a
--                          new future date. Behaves like 'Scheduled' for selection
--                          and availability, but is distinguishable for display and
--                          for the played-down rule.
--   'AwaitingReschedule' - a fixture whose play window has passed with no match card
--                          imported. The post-import sweep flips leftover fixtures
--                          into this status so captains know they need a new date.
--
-- The original chk_valid_fixture_status trigger (migration 001) only guarded
-- INSERTs. Reschedule and the import sweep both UPDATE the status, so recreate the
-- guard for both INSERT and UPDATE with the expanded valid set.

DROP TRIGGER IF EXISTS chk_valid_fixture_status;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_status
BEFORE INSERT ON fixtures
FOR EACH ROW
WHEN NEW.status NOT IN ('Scheduled', 'InProgress', 'Completed', 'Cancelled', 'Postponed', 'Rescheduled', 'AwaitingReschedule')
BEGIN
    SELECT RAISE(FAIL, 'Invalid fixture status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_status_update
BEFORE UPDATE ON fixtures
FOR EACH ROW
WHEN NEW.status NOT IN ('Scheduled', 'InProgress', 'Completed', 'Cancelled', 'Postponed', 'Rescheduled', 'AwaitingReschedule')
BEGIN
    SELECT RAISE(FAIL, 'Invalid fixture status');
END;

-- Backfill fixtures that were already rescheduled through the app before this
-- migration. The old reschedule flow never set a status, so they are still
-- 'Scheduled' with a rescheduled_reason recorded. Reclassify the still-pending
-- ones as 'Rescheduled' so they display correctly. Completed/Cancelled/Postponed
-- fixtures are untouched; any whose (new) date has already passed without a result
-- will be picked up by the next import sweep and moved to 'AwaitingReschedule'.
UPDATE fixtures
SET status = 'Rescheduled'
WHERE status = 'Scheduled' AND rescheduled_reason IS NOT NULL;
