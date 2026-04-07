-- Add player_token column to push_subscriptions to link subscriptions to players
ALTER TABLE push_subscriptions ADD COLUMN player_token TEXT;

-- Index for efficient lookups by player token
CREATE INDEX idx_push_subscriptions_player_token ON push_subscriptions(player_token);

-- Add unique constraint on endpoint for upsert support
-- First drop the existing non-unique index, then create a unique one
DROP INDEX IF EXISTS idx_push_subscriptions_endpoint;
CREATE UNIQUE INDEX idx_push_subscriptions_endpoint ON push_subscriptions(endpoint);
