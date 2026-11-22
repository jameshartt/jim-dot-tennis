-- WI-153: per-account tournament access.
--
-- 'none': no tournament management (the default for every existing account);
-- 'own':  the account's own tournaments and those it has been given;
-- 'all':  every tournament of the club.
--
-- An administrator always has access to every tournament whatever is stored
-- here: the one rule is models.EffectiveTournamentAccess, and every caller
-- uses it. A disabled account has none.
--
-- The role column and its values are unchanged. An account with the old
-- 'player' role reaches no league page; with access 'own' or 'all' it is a
-- "Tournaments only" account (the accounts service refuses to create one
-- without access, or to take its access away).
--
-- ADD COLUMN, never a rebuild of users: foreign keys are on and other tables
-- cascade from users (see migration 034).
ALTER TABLE users
    ADD COLUMN tournament_access TEXT NOT NULL DEFAULT 'none'
        CHECK (tournament_access IN ('none', 'own', 'all'));
