-- Tournament providers represent CourtHive provider organisations (e.g. St Ann's, Parks League Cup)
CREATE TABLE IF NOT EXISTS tournament_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    provider_abbr TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tournaments synced from CourtHive, with visibility control for the public index
CREATE TABLE IF NOT EXISTS tournaments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    courthive_tournament_id TEXT UNIQUE,
    provider_id INTEGER NOT NULL REFERENCES tournament_providers(id),
    start_date TEXT NOT NULL DEFAULT '',
    end_date TEXT NOT NULL DEFAULT '',
    is_visible BOOLEAN NOT NULL DEFAULT 0,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tournaments_is_visible ON tournaments(is_visible);
CREATE INDEX idx_tournaments_provider_id ON tournaments(provider_id);
CREATE INDEX idx_tournaments_courthive_id ON tournaments(courthive_tournament_id);
