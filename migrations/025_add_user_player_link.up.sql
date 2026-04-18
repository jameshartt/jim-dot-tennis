-- Sprint 016 WI-094: users.player_id link so admin/captain accounts can be
-- associated with their player record (used by Sprint 017 personalisation).
--
-- Note: users.player_id already exists from migration 004 (auth tables).
-- This migration exists to make the Sprint 016 contract explicit and to add
-- an index for the reverse lookup that Sprint 017 needs.

CREATE INDEX IF NOT EXISTS idx_users_player_id ON users(player_id);
