-- +migrate Up
-- WI-131: sessions are stored by the SHA-256 of their cookie token, and an
-- append-only account audit log.
--
-- Sessions. Until now sessions.id was the raw cookie token, so a copy of the
-- database (a backup, a snapshot) held a live bearer credential for every
-- signed-in user. From here on the cookie carries a random token and only its
-- SHA-256 (lowercase hex) is stored, in token_hash; sessions.id becomes a
-- plain number that the admin sessions page and the audit log use to name a
-- session without revealing anything usable.
--
-- Everyone is signed out once by this migration: existing sessions are
-- deleted rather than converted. SQLite in this build has no SHA-256 SQL
-- function, and deleting them is also the stronger fix: the raw tokens in
-- earlier backups stop working too, which hashing them in place would not do.
-- A browser with an old cookie finds no session, has the cookie cleared and
-- is sent to the sign-in page.
--
-- No table references sessions (internal/database/migration_035_test.go
-- checks this), so replacing the table cascades nothing; sessions itself
-- references users, and dropping a child table never touches its parent.
DROP TABLE sessions;

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    last_activity_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    device_info TEXT NOT NULL,
    is_valid BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_is_valid ON sessions(is_valid);

-- Account audit log: one row per account action (accounts.recordAudit),
-- written in the transaction that made the change. Rows never hold a
-- password, token, hash or session identifier. There are deliberately no
-- foreign keys: history must outlive the accounts it mentions, so the user
-- columns are plain copies (id and username at the time).
CREATE TABLE account_audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'cli', 'system')),
    actor_user_id INTEGER,
    actor TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    target_user_id INTEGER,
    target_username TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_account_audit_log_target ON account_audit_log(target_user_id);

-- Append-only: nothing in the application updates or deletes an entry, and
-- the database refuses it too. Retention is "keep": an operator who really
-- wants to prune drops these triggers by hand first (see
-- docs/PRODUCTION_QUICK_REFERENCE.md).
CREATE TRIGGER account_audit_log_no_update
BEFORE UPDATE ON account_audit_log
BEGIN
    SELECT RAISE(ABORT, 'account_audit_log is append-only');
END;

CREATE TRIGGER account_audit_log_no_delete
BEFORE DELETE ON account_audit_log
BEGIN
    SELECT RAISE(ABORT, 'account_audit_log is append-only');
END;
