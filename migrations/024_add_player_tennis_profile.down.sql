-- Sprint 016 WI-094 reversal.

DROP TRIGGER IF EXISTS chk_captain_note_kind;
DROP INDEX IF EXISTS idx_captain_player_notes_author;
DROP INDEX IF EXISTS idx_captain_player_notes_player;
DROP TABLE IF EXISTS captain_player_notes;

DROP TRIGGER IF EXISTS chk_no_self_partner;
DROP TRIGGER IF EXISTS chk_preferred_partner_kind;
DROP INDEX IF EXISTS idx_player_preferred_partners_partner;
DROP INDEX IF EXISTS idx_player_preferred_partners_player;
DROP TABLE IF EXISTS player_preferred_partners;

DROP INDEX IF EXISTS idx_player_tennis_preferences_updated;
DROP TABLE IF EXISTS player_tennis_preferences;
