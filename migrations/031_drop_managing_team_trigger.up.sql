-- Migration 031: drop the dormant managing-team trigger.
--
-- trg_set_managing_team_id (migration 010) picked the managing team by matching
-- a hardcoded club name pattern. The application always sets managing_team_id
-- explicitly, so the trigger never did anything useful and tied every database
-- to one club's name.

DROP TRIGGER IF EXISTS trg_set_managing_team_id;
