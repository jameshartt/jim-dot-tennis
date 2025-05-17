-- Extension for UUID support
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create tables
CREATE TABLE IF NOT EXISTS seasons (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    year INTEGER NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS leagues (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'Parks',
    year INTEGER NOT NULL,
    region VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Join table for leagues and seasons (many-to-many)
CREATE TABLE IF NOT EXISTS league_seasons (
    id SERIAL PRIMARY KEY,
    league_id INTEGER NOT NULL REFERENCES leagues(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(league_id, season_id)
);

CREATE TABLE IF NOT EXISTS divisions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    level INTEGER NOT NULL,
    play_day VARCHAR(10) NOT NULL, -- Day of the week
    league_id INTEGER NOT NULL REFERENCES leagues(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    max_teams_per_club INTEGER NOT NULL DEFAULT 2,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS clubs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    website VARCHAR(255),
    phone_number VARCHAR(20),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS teams (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    club_id INTEGER NOT NULL REFERENCES clubs(id),
    division_id INTEGER NOT NULL REFERENCES divisions(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS players (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(20),
    club_id INTEGER NOT NULL REFERENCES clubs(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS player_teams (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    team_id INTEGER NOT NULL REFERENCES teams(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, team_id, season_id)
);

CREATE TABLE IF NOT EXISTS captains (
    id SERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id),
    team_id INTEGER NOT NULL REFERENCES teams(id),
    role VARCHAR(20) NOT NULL DEFAULT 'Team', -- "Team" or "Day"
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, team_id, season_id)
);

CREATE TABLE IF NOT EXISTS fixtures (
    id SERIAL PRIMARY KEY,
    home_team_id INTEGER NOT NULL REFERENCES teams(id),
    away_team_id INTEGER NOT NULL REFERENCES teams(id),
    division_id INTEGER NOT NULL REFERENCES divisions(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    scheduled_date TIMESTAMP NOT NULL,
    venue_location TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Scheduled',
    completed_date TIMESTAMP,
    day_captain_id UUID REFERENCES players(id),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS matchups (
    id SERIAL PRIMARY KEY,
    fixture_id INTEGER NOT NULL REFERENCES fixtures(id),
    type VARCHAR(20) NOT NULL, -- "Mens", "Womens", "1st Mixed", "2nd Mixed"
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    home_score INTEGER NOT NULL DEFAULT 0,
    away_score INTEGER NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS matchup_players (
    id SERIAL PRIMARY KEY,
    matchup_id INTEGER NOT NULL REFERENCES matchups(id),
    player_id UUID NOT NULL REFERENCES players(id),
    is_home BOOLEAN NOT NULL, -- true for home team, false for away team
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(matchup_id, player_id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_seasons_year ON seasons(year);
CREATE INDEX IF NOT EXISTS idx_seasons_is_active ON seasons(is_active);
CREATE INDEX IF NOT EXISTS idx_leagues_year ON leagues(year);
CREATE INDEX IF NOT EXISTS idx_leagues_type ON leagues(type);
CREATE INDEX IF NOT EXISTS idx_league_seasons_league_id ON league_seasons(league_id);
CREATE INDEX IF NOT EXISTS idx_league_seasons_season_id ON league_seasons(season_id);
CREATE INDEX IF NOT EXISTS idx_players_club_id ON players(club_id);
CREATE INDEX IF NOT EXISTS idx_teams_club_id ON teams(club_id);
CREATE INDEX IF NOT EXISTS idx_teams_division_id ON teams(division_id);
CREATE INDEX IF NOT EXISTS idx_teams_season_id ON teams(season_id);
CREATE INDEX IF NOT EXISTS idx_player_teams_player_id ON player_teams(player_id);
CREATE INDEX IF NOT EXISTS idx_player_teams_team_id ON player_teams(team_id);
CREATE INDEX IF NOT EXISTS idx_player_teams_season_id ON player_teams(season_id);
CREATE INDEX IF NOT EXISTS idx_captains_player_id ON captains(player_id);
CREATE INDEX IF NOT EXISTS idx_captains_team_id ON captains(team_id);
CREATE INDEX IF NOT EXISTS idx_captains_season_id ON captains(season_id);
CREATE INDEX IF NOT EXISTS idx_captains_role ON captains(role);
CREATE INDEX IF NOT EXISTS idx_divisions_league_id ON divisions(league_id);
CREATE INDEX IF NOT EXISTS idx_divisions_season_id ON divisions(season_id);
CREATE INDEX IF NOT EXISTS idx_fixtures_division_id ON fixtures(division_id);
CREATE INDEX IF NOT EXISTS idx_fixtures_season_id ON fixtures(season_id);
CREATE INDEX IF NOT EXISTS idx_fixtures_home_team_id ON fixtures(home_team_id);
CREATE INDEX IF NOT EXISTS idx_fixtures_away_team_id ON fixtures(away_team_id);
CREATE INDEX IF NOT EXISTS idx_fixtures_scheduled_date ON fixtures(scheduled_date);
CREATE INDEX IF NOT EXISTS idx_fixtures_status ON fixtures(status);
CREATE INDEX IF NOT EXISTS idx_fixtures_day_captain_id ON fixtures(day_captain_id);
CREATE INDEX IF NOT EXISTS idx_matchups_fixture_id ON matchups(fixture_id);
CREATE INDEX IF NOT EXISTS idx_matchups_type ON matchups(type);
CREATE INDEX IF NOT EXISTS idx_matchups_status ON matchups(status);
CREATE INDEX IF NOT EXISTS idx_matchup_players_matchup_id ON matchup_players(matchup_id);
CREATE INDEX IF NOT EXISTS idx_matchup_players_player_id ON matchup_players(player_id);

-- Add some constraints for data integrity
ALTER TABLE fixtures ADD CONSTRAINT chk_different_teams CHECK (home_team_id != away_team_id);
ALTER TABLE matchups ADD CONSTRAINT chk_valid_type CHECK (type IN ('Mens', 'Womens', '1st Mixed', '2nd Mixed'));
ALTER TABLE matchups ADD CONSTRAINT chk_valid_status CHECK (status IN ('Pending', 'Playing', 'Finished', 'Defaulted'));
ALTER TABLE fixtures ADD CONSTRAINT chk_valid_fixture_status CHECK (status IN ('Scheduled', 'InProgress', 'Completed', 'Cancelled', 'Postponed'));
ALTER TABLE captains ADD CONSTRAINT chk_valid_captain_role CHECK (role IN ('Team', 'Day'));
ALTER TABLE leagues ADD CONSTRAINT chk_valid_league_type CHECK (type IN ('Parks', 'Club'));
-- Ensure only one active season at a time
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_season ON seasons(is_active) WHERE is_active = TRUE; 