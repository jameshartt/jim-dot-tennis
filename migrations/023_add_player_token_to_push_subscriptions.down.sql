-- Restore non-unique endpoint index
DROP INDEX IF EXISTS idx_push_subscriptions_endpoint;
CREATE INDEX idx_push_subscriptions_endpoint ON push_subscriptions(endpoint);

-- Remove player_token index and column
DROP INDEX IF EXISTS idx_push_subscriptions_player_token;
ALTER TABLE push_subscriptions DROP COLUMN player_token;
