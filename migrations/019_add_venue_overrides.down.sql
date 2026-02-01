-- Rollback Migration 019: Remove venue override system

DROP TABLE IF EXISTS venue_overrides;

-- SQLite: recreate fixtures table without venue_club_id
CREATE TABLE fixtures_backup AS SELECT
    id, home_team_id, away_team_id, division_id, season_id, week_id,
    scheduled_date, venue_location, status, completed_date, day_captain_id,
    external_match_card_id, notes, previous_dates, rescheduled_reason,
    created_at, updated_at
FROM fixtures;

DROP TABLE fixtures;

CREATE TABLE fixtures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    home_team_id INTEGER NOT NULL REFERENCES teams(id),
    away_team_id INTEGER NOT NULL REFERENCES teams(id),
    division_id INTEGER NOT NULL REFERENCES divisions(id),
    season_id INTEGER NOT NULL REFERENCES seasons(id),
    week_id INTEGER NOT NULL REFERENCES weeks(id),
    scheduled_date DATETIME NOT NULL,
    venue_location TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'Scheduled',
    completed_date DATETIME,
    day_captain_id TEXT REFERENCES players(id),
    external_match_card_id INTEGER,
    notes TEXT NOT NULL DEFAULT '',
    previous_dates TEXT,
    rescheduled_reason TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id,
    scheduled_date, venue_location, status, completed_date, day_captain_id,
    external_match_card_id, notes, previous_dates, rescheduled_reason,
    created_at, updated_at)
SELECT id, home_team_id, away_team_id, division_id, season_id, week_id,
    scheduled_date, venue_location, status, completed_date, day_captain_id,
    external_match_card_id, notes, previous_dates, rescheduled_reason,
    created_at, updated_at
FROM fixtures_backup;

DROP TABLE fixtures_backup;
