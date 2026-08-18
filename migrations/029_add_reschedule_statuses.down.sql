-- Revert to the original insert-only guard with the pre-029 status set.
-- Any fixtures left in 'Rescheduled' or 'AwaitingReschedule' are folded back to
-- 'Scheduled' first so the restored trigger does not reject later updates to them.

UPDATE fixtures SET status = 'Scheduled' WHERE status IN ('Rescheduled', 'AwaitingReschedule');

DROP TRIGGER IF EXISTS chk_valid_fixture_status_update;
DROP TRIGGER IF EXISTS chk_valid_fixture_status;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_status
BEFORE INSERT ON fixtures
FOR EACH ROW
WHEN NEW.status NOT IN ('Scheduled', 'InProgress', 'Completed', 'Cancelled', 'Postponed')
BEGIN
    SELECT RAISE(FAIL, 'Invalid fixture status');
END;
