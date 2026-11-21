-- +migrate Down
-- Back to migration 034: the audit log goes, and sessions are keyed by the
-- raw cookie token again. A stored hash cannot be turned back into a token,
-- so every session is ended (everyone is signed out once, as on the way up).
DROP TRIGGER IF EXISTS account_audit_log_no_delete;
DROP TRIGGER IF EXISTS account_audit_log_no_update;
DROP TABLE account_audit_log;

DROP TABLE sessions;

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    last_activity_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    device_info TEXT NOT NULL,
    is_valid BOOLEAN NOT NULL DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_is_valid ON sessions(is_valid);
