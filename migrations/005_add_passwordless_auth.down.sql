-- Drop view first
DROP VIEW IF EXISTS suspicious_access_patterns;

-- Drop indexes
DROP INDEX IF EXISTS idx_player_access_tokens_token;
DROP INDEX IF EXISTS idx_player_access_tokens_player_id;
DROP INDEX IF EXISTS idx_magic_links_token;
DROP INDEX IF EXISTS idx_magic_links_email;
DROP INDEX IF EXISTS idx_access_logs_token_type_token_id;
DROP INDEX IF EXISTS idx_access_logs_ip_address;
DROP INDEX IF EXISTS idx_access_logs_accessed_at;

-- Drop tables in reverse order
DROP TABLE IF EXISTS access_logs;
DROP TABLE IF EXISTS magic_links;
DROP TABLE IF EXISTS player_access_tokens; 