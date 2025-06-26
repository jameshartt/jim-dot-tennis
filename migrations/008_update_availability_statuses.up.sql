-- Update availability status validation to include IfNeeded

-- Drop existing triggers
DROP TRIGGER IF EXISTS chk_valid_general_availability_status;
DROP TRIGGER IF EXISTS chk_valid_exception_availability_status;
DROP TRIGGER IF EXISTS chk_valid_fixture_availability_status;

-- Recreate triggers with updated status validation
CREATE TRIGGER IF NOT EXISTS chk_valid_general_availability_status
BEFORE INSERT ON player_general_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_general_availability_status_update
BEFORE UPDATE ON player_general_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_exception_availability_status
BEFORE INSERT ON player_availability_exceptions
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_exception_availability_status_update
BEFORE UPDATE ON player_availability_exceptions
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_availability_status
BEFORE INSERT ON player_fixture_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_availability_status_update
BEFORE UPDATE ON player_fixture_availability
FOR EACH ROW
WHEN NEW.status NOT IN ('Available', 'Unavailable', 'IfNeeded', 'Unknown')
BEGIN
    SELECT RAISE(FAIL, 'Invalid availability status');
END; 