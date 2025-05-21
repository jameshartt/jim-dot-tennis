-- Create access tokens table for player access
CREATE TABLE IF NOT EXISTS player_access_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,  -- The URL token based on tennis pro names
    player_id TEXT NOT NULL,     -- Reference to the player
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Create magic links table for captain/admin access
CREATE TABLE IF NOT EXISTS magic_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,  -- 'captain' or 'admin'
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (email) REFERENCES players(email) ON DELETE CASCADE
);

-- Create access logs table for security monitoring
CREATE TABLE IF NOT EXISTS access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_type TEXT NOT NULL,  -- 'player' or 'magic'
    token_id INTEGER NOT NULL, -- ID from respective token table
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    accessed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    failure_reason TEXT
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_player_access_tokens_token ON player_access_tokens(token);
CREATE INDEX IF NOT EXISTS idx_player_access_tokens_player_id ON player_access_tokens(player_id);
CREATE INDEX IF NOT EXISTS idx_magic_links_token ON magic_links(token);
CREATE INDEX IF NOT EXISTS idx_magic_links_email ON magic_links(email);
CREATE INDEX IF NOT EXISTS idx_access_logs_token_type_token_id ON access_logs(token_type, token_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_ip_address ON access_logs(ip_address);
CREATE INDEX IF NOT EXISTS idx_access_logs_accessed_at ON access_logs(accessed_at);

-- Create a view for suspicious access patterns
CREATE VIEW IF NOT EXISTS suspicious_access_patterns AS
SELECT 
    ip_address,
    token_type,
    COUNT(*) as access_count,
    COUNT(CASE WHEN success = 0 THEN 1 END) as failure_count,
    MIN(accessed_at) as first_attempt,
    MAX(accessed_at) as last_attempt
FROM access_logs
WHERE accessed_at > datetime('now', '-1 hour')
GROUP BY ip_address, token_type
HAVING access_count > 10 OR failure_count > 5; 