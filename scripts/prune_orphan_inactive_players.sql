-- prune_orphan_inactive_players.sql
--
-- Purpose: remove junk player rows created by the "new player saved as
-- inactive" bug (fixed in players.go / repository/player.go where Create now
-- forces is_active = true). Those rows are invisible in the admin list
-- (FindAll filters WHERE is_active = TRUE) yet accumulate on every save,
-- e.g. the duplicate "Sean Mahon" rows.
--
-- A row is considered safe to delete ONLY if ALL of these hold:
--   * is_active = 0 (inactive / never made visible)
--   * has NO rows in fixture_players  (never selected for a fixture)
--   * has NO rows in matchup_players  (never actually played a matchup)
-- This guarantees we never delete a real player who was deactivated after
-- having any fixture involvement.
--
-- USAGE (run against a BACKED-UP copy of production first):
--   1. PREVIEW — run the SELECT block below and eyeball the list.
--   2. PRUNE   — run the BEGIN..COMMIT block.
--   sqlite3 tennis.db < scripts/prune_orphan_inactive_players.sql
--   (or paste the two blocks separately for a manual preview/confirm step)

-- ─────────────────────────────────────────────────────────────────────────
-- 1. PREVIEW: candidates that WOULD be deleted
-- ─────────────────────────────────────────────────────────────────────────
SELECT p.id, p.first_name, p.last_name, p.club_id, p.created_at
FROM players p
WHERE p.is_active = 0
  AND NOT EXISTS (SELECT 1 FROM fixture_players fp WHERE fp.player_id = p.id)
  AND NOT EXISTS (SELECT 1 FROM matchup_players mp WHERE mp.player_id = p.id)
ORDER BY p.last_name, p.first_name, p.created_at;

-- ─────────────────────────────────────────────────────────────────────────
-- 2. PRUNE: delete the candidates and their dependent rows
--    Wrapped in a transaction. PRAGMA foreign_keys ON so any unexpected
--    reference (a child row we didn't anticipate) aborts the whole thing
--    instead of silently orphaning data.
-- ─────────────────────────────────────────────────────────────────────────
PRAGMA foreign_keys = ON;

BEGIN TRANSACTION;

-- Snapshot the target ids into a temp table so every statement targets the
-- exact same set.
CREATE TEMP TABLE _prune_ids AS
SELECT p.id
FROM players p
WHERE p.is_active = 0
  AND NOT EXISTS (SELECT 1 FROM fixture_players fp WHERE fp.player_id = p.id)
  AND NOT EXISTS (SELECT 1 FROM matchup_players mp WHERE mp.player_id = p.id);

-- Detach any auth user account from the player (keep the account, null the link).
UPDATE users SET player_id = NULL WHERE player_id IN (SELECT id FROM _prune_ids);

-- Remove dependent rows in the tables that DON'T have ON DELETE CASCADE.
-- (player_tennis_preferences, player_preferred_partners and the other
--  migration-024 profile tables cascade automatically.)
DELETE FROM player_availability            WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM player_availability_exceptions WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM player_general_availability    WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM player_fixture_availability    WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM player_divisions               WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM player_teams                   WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM preferred_name_requests        WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM captain_player_notes           WHERE player_id IN (SELECT id FROM _prune_ids);
DELETE FROM captains                       WHERE player_id IN (SELECT id FROM _prune_ids);

-- Finally remove the players themselves.
DELETE FROM players WHERE id IN (SELECT id FROM _prune_ids);

DROP TABLE _prune_ids;

COMMIT;
