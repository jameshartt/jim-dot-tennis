-- Passkeys (WebAuthn), Sprint 021 WI-128.
--
-- webauthn_user_handles: the stable random handle a user's passkeys carry
-- (the WebAuthn "user.id"). It is what a discoverable sign-in presents, so it
-- is random and never the numeric user id or the username. Created the first
-- time a user starts enrolling a passkey. Kept in its own table so the users
-- table is untouched.
--
-- webauthn_credentials: one row per enrolled passkey. credential_id is unique
-- across all users. sign_count and the backup flags come from the
-- authenticator; clone_warning_at is set when a sign counter went backwards
-- (a possible copy of the authenticator): the passkey is then refused until
-- an administrator clears the warning or the user removes it.

CREATE TABLE IF NOT EXISTS webauthn_user_handles (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    handle BLOB NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB NOT NULL UNIQUE,
    public_key BLOB NOT NULL,
    attestation_type TEXT NOT NULL DEFAULT '',
    attestation_format TEXT NOT NULL DEFAULT '',
    aaguid BLOB,
    sign_count INTEGER NOT NULL DEFAULT 0,
    transports TEXT NOT NULL DEFAULT '',
    attachment TEXT NOT NULL DEFAULT '',
    user_present BOOLEAN NOT NULL DEFAULT FALSE,
    user_verified BOOLEAN NOT NULL DEFAULT FALSE,
    backup_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    backup_state BOOLEAN NOT NULL DEFAULT FALSE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    clone_warning_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user ON webauthn_credentials(user_id);
