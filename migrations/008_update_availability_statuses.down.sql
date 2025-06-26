-- Rollback availability status validation to original state

-- Drop updated triggers
DROP TRIGGER IF EXISTS chk_valid_general_availability_status;
DROP TRIGGER IF EXISTS chk_valid_general_availability_status_update;
DROP TRIGGER IF EXISTS chk_valid_exception_availability_status;
DROP TRIGGER IF EXISTS chk_valid_exception_availability_status_update;
DROP TRIGGER IF EXISTS chk_valid_fixture_availability_status;
DROP TRIGGER IF EXISTS chk_valid_fixture_availability_status_update;

-- Recreate original triggers without IfNeeded status
CREATE TRIGGER IF NOT EXISTS chk_valid_general_availability_status
BEFORE INSERT ON player_general_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_exception_availability_status
BEFORE INSERT ON player_availability_exceptions
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_availability_status
BEFORE INSERT ON player_fixture_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END; 