-- Sprint 021 WI-129 reversal.
--
-- Unused links are discarded with the table.
--
-- users.password_hash becomes NOT NULL again, by the same in-place column
-- replacement as the up file (no row of users is deleted, so sessions,
-- passkeys and everything else that references users survive). An account
-- with no password gets the empty string, which no password matches: it can
-- still sign in with a passkey (while migration 033 is applied) and is
-- otherwise recovered with the older version's `user reset`. The column ends
-- up with DEFAULT '' (SQLite needs a default to add a NOT NULL column), which
-- the original definition did not have; nothing depends on that.
--
-- Passwords removed by the up file (accounts that carried
-- must_change_password) are not restored.

DROP INDEX IF EXISTS idx_account_links_user;
DROP TABLE IF EXISTS account_links;

ALTER TABLE users ADD COLUMN password_hash_required TEXT NOT NULL DEFAULT '';

UPDATE users SET password_hash_required = COALESCE(password_hash, '');

ALTER TABLE users DROP COLUMN password_hash;

ALTER TABLE users RENAME COLUMN password_hash_required TO password_hash;
