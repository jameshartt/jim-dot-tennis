-- Player availability migration

-- Enum for player availability status
CREATE TYPE availability_status AS ENUM ('Available', 'Unavailable', 'Unknown');

-- Player division eligibility
-- Tracks which divisions a player is eligible to play in
CREATE TABLE IF NOT EXISTS player_divisions (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    division_id INTEGER NOT NULL REFERENCES divisions(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, division_id, season_id)
);

-- Player general availability
-- Default/base availability for the player
CREATE TABLE IF NOT EXISTS player_general_availability (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    day_of_week VARCHAR(10) NOT NULL, -- 'Monday', 'Tuesday', etc.
    status availability_status NOT NULL DEFAULT 'Unknown',
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, day_of_week, season_id)
);

-- Player specific date availability exceptions
-- For specific dates when player is available or unavailable 
-- (overrides the general availability)
CREATE TABLE IF NOT EXISTS player_availability_exceptions (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    status availability_status NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT valid_date_range CHECK (end_date >= start_date)
);

-- Player fixture availability
-- Records a player's explicit availability for a specific fixture
CREATE TABLE IF NOT EXISTS player_fixture_availability (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    fixture_id INTEGER NOT NULL REFERENCES fixtures(id),
    status availability_status NOT NULL DEFAULT 'Unknown',
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, fixture_id)
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