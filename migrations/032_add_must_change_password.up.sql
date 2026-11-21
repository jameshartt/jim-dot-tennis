-- Sprint 021 WI-127: accounts whose password was set by someone else.
--
-- `jim-dot-tennis user create` and `user reset` (and an administrator setting
-- a password on the users page) give the account a password the person did
-- not choose. Until they choose their own, every signed-in page sends them to
-- /account/password. Cleared when they set a new password there.
ALTER TABLE users
    ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
