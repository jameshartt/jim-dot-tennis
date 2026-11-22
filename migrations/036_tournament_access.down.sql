-- WI-153 reversal. "Tournaments only" accounts keep the 'player' role, which
-- reaches no page, as before this migration.
ALTER TABLE users DROP COLUMN tournament_access;
