-- Create tables
CREATE TABLE IF NOT EXISTS seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    year INTEGER NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS leagues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'Parks',
    year INTEGER NOT NULL,
    region VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Join table for leagues and seasons (many-to-many)
CREATE TABLE IF NOT EXISTS league_seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    league_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(league_id, season_id),
    FOREIGN KEY (league_id) REFERENCES leagues(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS divisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    level INTEGER NOT NULL,
    play_day VARCHAR(10) NOT NULL, -- Day of the week
    league_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    max_teams_per_club INTEGER NOT NULL DEFAULT 2,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (league_id) REFERENCES leagues(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS weeks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    week_number INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    name VARCHAR(255), -- Optional name like "Week 1", "Semi-Finals", etc.
    is_active BOOLEAN NOT NULL DEFAULT FALSE, -- Current week
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(week_number, season_id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS clubs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    website VARCHAR(255),
    phone_number VARCHAR(20),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    club_id INTEGER NOT NULL,
    division_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (club_id) REFERENCES clubs(id),
    FOREIGN KEY (division_id) REFERENCES divisions(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS players (
    id TEXT PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(20),
    club_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (club_id) REFERENCES clubs(id)
);

CREATE TABLE IF NOT EXISTS player_teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    team_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, team_id, season_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (team_id) REFERENCES teams(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS captains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    team_id INTEGER NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'Team', -- "Team" or "Day"
    season_id INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, team_id, season_id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (team_id) REFERENCES teams(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

CREATE TABLE IF NOT EXISTS fixtures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    home_team_id INTEGER NOT NULL,
    away_team_id INTEGER NOT NULL,
    division_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    week_id INTEGER NOT NULL,
    scheduled_date TIMESTAMP NOT NULL,
    venue_location TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Scheduled',
    completed_date TIMESTAMP,
    day_captain_id TEXT,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (home_team_id) REFERENCES teams(id),
    FOREIGN KEY (away_team_id) REFERENCES teams(id),
    FOREIGN KEY (division_id) REFERENCES divisions(id),
    FOREIGN KEY (season_id) REFERENCES seasons(id),
    FOREIGN KEY (week_id) REFERENCES weeks(id),
    FOREIGN KEY (day_captain_id) REFERENCES players(id)
);

CREATE TABLE IF NOT EXISTS matchups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fixture_id INTEGER NOT NULL,
    type VARCHAR(20) NOT NULL, -- "Mens", "Womens", "1st Mixed", "2nd Mixed"
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    home_score INTEGER NOT NULL DEFAULT 0,
    away_score INTEGER NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (fixture_id) REFERENCES fixtures(id)
);

CREATE TABLE IF NOT EXISTS matchup_players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    matchup_id INTEGER NOT NULL,
    player_id TEXT NOT NULL,
    is_home BOOLEAN NOT NULL, -- true for home team, false for away team
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(matchup_id, player_id),
    FOREIGN KEY (matchup_id) REFERENCES matchups(id),
    FOREIGN KEY (player_id) REFERENCES players(id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_seasons_year ON seasons(year);
CREATE INDEX IF NOT EXISTS idx_seasons_is_active ON seasons(is_active);
CREATE INDEX IF NOT EXISTS idx_leagues_year ON leagues(year);
CREATE INDEX IF NOT EXISTS idx_leagues_type ON leagues(type);
CREATE INDEX IF NOT EXISTS idx_league_seasons_league_id ON league_seasons(league_id);
CREATE INDEX IF NOT EXISTS idx_league_seasons_season_id ON league_seasons(season_id);
CREATE INDEX IF NOT EXISTS idx_weeks_season_id ON weeks(season_id);
CREATE INDEX IF NOT EXISTS idx_weeks_week_number ON weeks(week_number);
CREATE INDEX IF NOT EXISTS idx_weeks_start_date ON weeks(start_date);
CREATE INDEX IF NOT EXISTS idx_weeks_is_active ON weeks(is_active);
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
CREATE INDEX IF NOT EXISTS idx_fixtures_week_id ON fixtures(week_id);
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

-- Create triggers to enforce constraint-like checks in SQLite
CREATE TRIGGER IF NOT EXISTS chk_different_teams
BEFORE INSERT ON fixtures
FOR EACH ROW
WHEN NEW.home_team_id = NEW.away_team_id
BEGIN
    SELECT RAISE(FAIL, 'Home and away teams must be different');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_type
BEFORE INSERT ON matchups
FOR EACH ROW
WHEN NEW.type NOT IN ('Mens', 'Womens', '1st Mixed', '2nd Mixed')
BEGIN
    SELECT RAISE(FAIL, 'Invalid matchup type');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_status
BEFORE INSERT ON matchups
FOR EACH ROW
WHEN NEW.status NOT IN ('Pending', 'Playing', 'Finished', 'Defaulted')
BEGIN
    SELECT RAISE(FAIL, 'Invalid matchup status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_fixture_status
BEFORE INSERT ON fixtures
FOR EACH ROW
WHEN NEW.status NOT IN ('Scheduled', 'InProgress', 'Completed', 'Cancelled', 'Postponed')
BEGIN
    SELECT RAISE(FAIL, 'Invalid fixture status');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_captain_role
BEFORE INSERT ON captains
FOR EACH ROW
WHEN NEW.role NOT IN ('Team', 'Day')
BEGIN
    SELECT RAISE(FAIL, 'Invalid captain role');
END;

CREATE TRIGGER IF NOT EXISTS chk_valid_league_type
BEFORE INSERT ON leagues
FOR EACH ROW
WHEN NEW.type NOT IN ('Parks', 'Club')
BEGIN
    SELECT RAISE(FAIL, 'Invalid league type');
END;

-- Ensure only one active week per season
CREATE TRIGGER IF NOT EXISTS chk_active_week
BEFORE INSERT ON weeks
FOR EACH ROW
WHEN NEW.is_active = TRUE
BEGIN
    UPDATE weeks SET is_active = FALSE WHERE season_id = NEW.season_id AND is_active = TRUE;
END;

-- Ensure only one active season at a time
CREATE TRIGGER IF NOT EXISTS chk_active_season
BEFORE INSERT ON seasons
FOR EACH ROW
WHEN NEW.is_active = TRUE
BEGIN
    UPDATE seasons SET is_active = FALSE WHERE is_active = TRUE;
END; 