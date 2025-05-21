-- Drop views first
DROP VIEW IF EXISTS suspicious_access_patterns;

-- Drop indexes
DROP INDEX IF EXISTS idx_access_logs_accessed_at;
DROP INDEX IF EXISTS idx_access_logs_ip_address;
DROP INDEX IF EXISTS idx_access_logs_token_type_token_id;
DROP INDEX IF EXISTS idx_player_access_tokens_player_id;
DROP INDEX IF EXISTS idx_player_access_tokens_token;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_username;

-- Drop tables
DROP TABLE IF EXISTS access_logs;
DROP TABLE IF EXISTS player_access_tokens;
DROP TABLE IF EXISTS users; 