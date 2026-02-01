-- Migration 019: Add venue override system
-- Adds per-fixture venue override and club-level date-range overrides

-- Per-fixture venue override: when set, fixture uses this club's venue instead of home team's
ALTER TABLE fixtures ADD COLUMN venue_club_id INTEGER REFERENCES clubs(id);

-- Club date-range venue overrides: when a club is displaced from their home venue
CREATE TABLE venue_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    club_id INTEGER NOT NULL REFERENCES clubs(id),         -- The club being displaced
    venue_club_id INTEGER NOT NULL REFERENCES clubs(id),   -- Where they play instead
    start_date DATETIME NOT NULL,
    end_date DATETIME NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_venue_overrides_club_id ON venue_overrides(club_id);
CREATE INDEX idx_venue_overrides_dates ON venue_overrides(start_date, end_date);
