-- Player availability migration

-- Player division eligibility
-- Tracks which divisions a player is eligible to play in
CREATE TABLE IF NOT EXISTS player_divisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    division_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, division_id, season_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (division_id) REFERENCES divisions(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

-- Player general availability
-- Default/base availability for the player
CREATE TABLE IF NOT EXISTS player_general_availability (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    day_of_week VARCHAR(10) NOT NULL, -- 'Monday', 'Tuesday', etc.
    status VARCHAR(20) NOT NULL DEFAULT 'Unknown', -- 'Available', 'Unavailable', 'Unknown'
    season_id INTEGER NOT NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, day_of_week, season_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

-- Player specific date availability exceptions
-- For specific dates when player is available or unavailable 
-- (overrides the general availability)
CREATE TABLE IF NOT EXISTS player_availability_exceptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'Available', 'Unavailable', 'Unknown'
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES players(id)
);

-- Add a trigger to enforce the date range constraint
CREATE TRIGGER IF NOT EXISTS check_valid_date_range
BEFORE INSERT ON player_availability_exceptions
FOR EACH ROW
WHEN NEW.end_date < NEW.start_date
BEGIN
    SELECT RAISE(FAIL, 'End date must be after or equal to start date');
END;

-- Player fixture availability
-- Records a player's explicit availability for a specific fixture
CREATE TABLE IF NOT EXISTS player_fixture_availability (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    fixture_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Unknown',
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, fixture_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (fixture_id) REFERENCES fixtures(id)
);

-- Player availability tables
CREATE TABLE IF NOT EXISTS availability_time_slots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fixture_id INTEGER NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (fixture_id) REFERENCES fixtures(id)
);

CREATE TABLE IF NOT EXISTS player_availability (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    fixture_id INTEGER NOT NULL,
    time_slot_id INTEGER,
    availability_status VARCHAR(20) NOT NULL DEFAULT 'Unknown', -- "Available", "Unavailable", "Tentative", "Unknown"
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, fixture_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (fixture_id) REFERENCES fixtures(id),
    FOREIGN KEY (time_slot_id) REFERENCES availability_time_slots(id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_player_divisions_player_id ON player_divisions(player_id);
CREATE INDEX IF NOT EXISTS idx_player_divisions_division_id ON player_divisions(division_id);
CREATE INDEX IF NOT EXISTS idx_player_divisions_season_id ON player_divisions(season_id);

CREATE INDEX IF NOT EXISTS idx_player_general_availability_player_id ON player_general_availability(player_id);
CREATE INDEX IF NOT EXISTS idx_player_general_availability_day_of_week ON player_general_availability(day_of_week);
CREATE INDEX IF NOT EXISTS idx_player_general_availability_status ON player_general_availability(status);
CREATE INDEX IF NOT EXISTS idx_player_general_availability_season_id ON player_general_availability(season_id);

CREATE INDEX IF NOT EXISTS idx_player_availability_exceptions_player_id ON player_availability_exceptions(player_id);
CREATE INDEX IF NOT EXISTS idx_player_availability_exceptions_status ON player_availability_exceptions(status);
CREATE INDEX IF NOT EXISTS idx_player_availability_exceptions_date_range ON player_availability_exceptions(start_date, end_date);

CREATE INDEX IF NOT EXISTS idx_player_fixture_availability_player_id ON player_fixture_availability(player_id);
CREATE INDEX IF NOT EXISTS idx_player_fixture_availability_fixture_id ON player_fixture_availability(fixture_id);
CREATE INDEX IF NOT EXISTS idx_player_fixture_availability_status ON player_fixture_availability(status);

CREATE INDEX IF NOT EXISTS idx_time_slots_fixture_id ON availability_time_slots(fixture_id);
CREATE INDEX IF NOT EXISTS idx_time_slots_start_time ON availability_time_slots(start_time);
CREATE INDEX IF NOT EXISTS idx_player_availability_player_id ON player_availability(player_id);
CREATE INDEX IF NOT EXISTS idx_player_availability_fixture_id ON player_availability(fixture_id);
CREATE INDEX IF NOT EXISTS idx_player_availability_time_slot_id ON player_availability(time_slot_id);
CREATE INDEX IF NOT EXISTS idx_player_availability_status ON player_availability(availability_status);

-- Create triggers to validate availability statuses
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