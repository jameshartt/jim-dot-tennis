-- Sprint 021 WI-129: invite and recovery links; passwords become optional.
--
-- 1. account_links: single-use links that let a person set up (invite) or
--    regain (recovery) a way to sign in. Only the SHA-256 hash of the link's
--    random token is stored, so a copy of the database holds no usable link.
--    A link is live while used_at and revoked_at are NULL and expires_at is in
--    the future.
--
-- 2. users.password_hash becomes nullable: an invited account has no password
--    until its owner sets one, and may only ever have passkeys.
--
--    This is deliberately NOT the usual SQLite "create new table, copy, drop
--    old, rename" rebuild. Foreign keys are enforced on every connection of
--    this app and migrations run inside a transaction, where
--    PRAGMA foreign_keys = OFF does nothing; DROP TABLE users would therefore
--    delete every row that references users with ON DELETE CASCADE (sessions,
--    passkeys, passkey user handles, player notes). Instead the column is
--    replaced in place: add a nullable column, copy, drop the old column,
--    rename. No row of users is ever deleted, so nothing cascades, and the
--    rowids, the other columns, the indexes and the foreign keys pointing at
--    users are untouched. The only visible difference is that password_hash
--    is now the last column of the table.
--    (internal/database/migration_034_test.go proves the referencing rows
--    survive, up and down.)
--
-- 3. must_change_password is retired. It marked passwords chosen by someone
--    else (temporary passwords from the command line, passwords typed by an
--    administrator), which no longer exist: accounts are now handed over with
--    a link. An account still carrying the mark holds a password somebody
--    else knows, so that password is removed here; the person gets back in
--    with a recovery link (`jim-dot-tennis user reset <username>`). The
--    column stays, always FALSE, and nothing reads it.

CREATE TABLE account_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('invite', 'recovery')),
    token_hash TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    revoked_at TIMESTAMP
);

CREATE INDEX idx_account_links_user ON account_links(user_id);

ALTER TABLE users ADD COLUMN password_hash_optional TEXT;

UPDATE users
SET password_hash_optional = CASE
        WHEN must_change_password THEN NULL
        ELSE NULLIF(password_hash, '')
    END;

UPDATE users SET must_change_password = FALSE WHERE must_change_password;

ALTER TABLE users DROP COLUMN password_hash;

ALTER TABLE users RENAME COLUMN password_hash_optional TO password_hash;
