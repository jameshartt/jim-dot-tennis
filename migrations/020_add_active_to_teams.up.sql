-- Add active status to teams for marking disbanded/inactive teams
ALTER TABLE teams ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
