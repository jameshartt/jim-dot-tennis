-- Drop triggers first
DROP TRIGGER IF EXISTS update_preferred_name_requests_timestamp;
DROP TRIGGER IF EXISTS chk_preferred_name_request_status;
DROP TRIGGER IF EXISTS chk_preferred_name_unique;

-- Drop indexes for preferred name requests
DROP INDEX IF EXISTS idx_preferred_name_requests_created_at;
DROP INDEX IF EXISTS idx_preferred_name_requests_status;
DROP INDEX IF EXISTS idx_preferred_name_requests_player_id;

-- Drop preferred name requests table
DROP TABLE IF EXISTS preferred_name_requests;

-- Drop preferred_name index from players
DROP INDEX IF EXISTS idx_players_preferred_name;

-- Remove preferred_name column from players table
ALTER TABLE players DROP COLUMN preferred_name; 