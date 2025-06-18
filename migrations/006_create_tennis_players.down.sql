-- Drop triggers first
DROP TRIGGER IF EXISTS update_fantasy_mixed_doubles_updated_at;
DROP TRIGGER IF EXISTS update_tennis_players_updated_at;

-- Drop indexes
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_is_active;
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_team_b_man_id;
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_team_b_woman_id;
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_team_a_man_id;
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_team_a_woman_id;
DROP INDEX IF EXISTS idx_fantasy_mixed_doubles_auth_token;
DROP INDEX IF EXISTS idx_tennis_players_nationality;
DROP INDEX IF EXISTS idx_tennis_players_current_rank;
DROP INDEX IF EXISTS idx_tennis_players_gender;
DROP INDEX IF EXISTS idx_tennis_players_tour;

-- Drop tables
DROP TABLE IF EXISTS fantasy_mixed_doubles;
DROP TABLE IF EXISTS tennis_players; 