-- WI-152: one row for each account that has, or has had, a tournament account
-- on the bundled tournament server (internal/tournaments).
--
-- courthive_user_id is the tournament server's id for the account. The app
-- must keep it: once a person's membership is removed there, the server
-- offers no way to look the account up again.
--
-- applied_role is what the tournament server was last told: 'PROVIDER_ADMIN',
-- 'DIRECTOR', or '' when the person's membership has been removed.
-- pending = 1 means the last attempt to bring the tournament server into line
-- with this account failed and must be tried again; last_error says why.
CREATE TABLE tournament_accounts (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    courthive_user_id TEXT NOT NULL,
    email TEXT NOT NULL,
    applied_role TEXT NOT NULL DEFAULT '' CHECK (applied_role IN ('', 'PROVIDER_ADMIN', 'DIRECTOR')),
    pending INTEGER NOT NULL DEFAULT 0 CHECK (pending IN (0, 1)),
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tournament_accounts_pending ON tournament_accounts(pending) WHERE pending = 1;
