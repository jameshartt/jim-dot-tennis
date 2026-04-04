-- Add is_active column to players table for soft-delete support
ALTER TABLE players ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- Index for efficient filtering of active/inactive players
CREATE INDEX idx_players_is_active ON players(is_active);
