-- Removes passkeys. Every enrolled passkey is lost: users sign in with their
-- password afterwards.
DROP INDEX IF EXISTS idx_webauthn_credentials_user;
DROP TABLE IF EXISTS webauthn_credentials;
DROP TABLE IF EXISTS webauthn_user_handles;
