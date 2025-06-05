-- Remove fixture_players table
DROP TRIGGER IF EXISTS update_fixture_players_updated_at;
DROP INDEX IF EXISTS idx_fixture_players_position;
DROP INDEX IF EXISTS idx_fixture_players_is_home;
DROP INDEX IF EXISTS idx_fixture_players_player_id;
DROP INDEX IF EXISTS idx_fixture_players_fixture_id;
DROP TABLE IF EXISTS fixture_players; 