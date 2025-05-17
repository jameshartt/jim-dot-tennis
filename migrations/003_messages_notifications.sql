-- Messages and Notifications system

-- Message categories
CREATE TYPE message_category AS ENUM (
    'General',          -- General announcements
    'Fixture',          -- Fixture-related messages
    'Availability',     -- Messages about player availability
    'Selection',        -- Team selection messages
    'Results',          -- Match results
    'Administrative'    -- Administrative notifications
);

-- Message importance levels
CREATE TYPE message_importance AS ENUM (
    'Low',
    'Medium',
    'High',
    'Urgent'
);

-- Messages table - persistent and can be referenced by notifications
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    sender_type VARCHAR(50) NOT NULL, -- 'System', 'Captain', 'Admin', 'Player'
    sender_id VARCHAR(255), -- UUID or ID of the sender (null for system messages)
    category message_category NOT NULL DEFAULT 'General',
    importance message_importance NOT NULL DEFAULT 'Medium',
    related_entity_type VARCHAR(50), -- 'Fixture', 'Team', 'Division', 'League', 'Season'
    related_entity_id INTEGER, -- ID of the related entity
    is_draft BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at TIMESTAMP,
    season_id INTEGER REFERENCES seasons(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Message Recipients - who should receive this message
CREATE TABLE IF NOT EXISTS message_recipients (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    recipient_type VARCHAR(50) NOT NULL, -- 'Player', 'Captain', 'Team', 'Division', 'All'
    recipient_id VARCHAR(255), -- ID of recipient (null for broadcast messages)
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Notifications table - ephemeral, can point to a message or exist independently
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    recipient_id VARCHAR(255) NOT NULL, -- UUID of the player or captain
    recipient_type VARCHAR(50) NOT NULL, -- 'Player' or 'Captain'
    message_id INTEGER REFERENCES messages(id) ON DELETE SET NULL, -- Optional reference to a message
    action_type VARCHAR(50), -- 'SubmitAvailability', 'ViewSelection', 'ConfirmAttendance', etc.
    action_url VARCHAR(255), -- URL for the action
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP,
    expires_at TIMESTAMP, -- When the notification should expire/disappear
    importance message_importance NOT NULL DEFAULT 'Medium',
    related_entity_type VARCHAR(50), -- 'Fixture', 'Team', 'Division', 'League'
    related_entity_id INTEGER, -- ID of the related entity
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Scheduled Messages - for messages to be sent in the future
CREATE TABLE IF NOT EXISTS scheduled_messages (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    scheduled_time TIMESTAMP NOT NULL,
    recurrence VARCHAR(50), -- 'None', 'Daily', 'Weekly', 'Monthly'
    is_sent BOOLEAN NOT NULL DEFAULT FALSE,
    last_sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Delivery Methods - how notifications are delivered to recipients
CREATE TABLE IF NOT EXISTS delivery_preferences (
    id SERIAL PRIMARY KEY,
    player_id VARCHAR(255) NOT NULL, -- UUID of the player
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    push_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    in_app_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    availability_reminders BOOLEAN NOT NULL DEFAULT TRUE,
    selection_notifications BOOLEAN NOT NULL DEFAULT TRUE,
    fixture_reminders BOOLEAN NOT NULL DEFAULT TRUE,
    result_notifications BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id)
);

-- Index for faster queries
CREATE INDEX IF NOT EXISTS idx_messages_category ON messages(category);
CREATE INDEX IF NOT EXISTS idx_messages_importance ON messages(importance);
CREATE INDEX IF NOT EXISTS idx_messages_related_entity ON messages(related_entity_type, related_entity_id);
CREATE INDEX IF NOT EXISTS idx_messages_season_id ON messages(season_id);
CREATE INDEX IF NOT EXISTS idx_messages_sent_at ON messages(sent_at);

CREATE INDEX IF NOT EXISTS idx_message_recipients_message_id ON message_recipients(message_id);
CREATE INDEX IF NOT EXISTS idx_message_recipients_recipient ON message_recipients(recipient_type, recipient_id);
CREATE INDEX IF NOT EXISTS idx_message_recipients_is_read ON message_recipients(is_read);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications(recipient_type, recipient_id);
CREATE INDEX IF NOT EXISTS idx_notifications_message_id ON notifications(message_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_expires_at ON notifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);

CREATE INDEX IF NOT EXISTS idx_scheduled_messages_scheduled_time ON scheduled_messages(scheduled_time);
CREATE INDEX IF NOT EXISTS idx_scheduled_messages_is_sent ON scheduled_messages(is_sent);

-- Triggers for automatic timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_messages_updated_at
BEFORE UPDATE ON messages
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_message_recipients_updated_at
BEFORE UPDATE ON message_recipients
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notifications_updated_at
BEFORE UPDATE ON notifications
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_scheduled_messages_updated_at
BEFORE UPDATE ON scheduled_messages
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_delivery_preferences_updated_at
BEFORE UPDATE ON delivery_preferences
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column(); 