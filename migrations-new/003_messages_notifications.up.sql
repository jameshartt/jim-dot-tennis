-- Messages and Notifications system

-- Messages table - persistent and can be referenced by notifications
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    sender_type VARCHAR(50) NOT NULL, -- 'System', 'Captain', 'Admin', 'Player'
    sender_id VARCHAR(255), -- UUID or ID of the sender (null for system messages)
    category VARCHAR(50) NOT NULL DEFAULT 'General', -- 'General', 'Fixture', 'Availability', 'Selection', 'Results', 'Administrative'
    importance VARCHAR(20) NOT NULL DEFAULT 'Medium', -- 'Low', 'Medium', 'High', 'Urgent'
    related_entity_type VARCHAR(50), -- 'Fixture', 'Team', 'Division', 'League', 'Season'
    related_entity_id INTEGER, -- ID of the related entity
    is_draft BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at TIMESTAMP,
    season_id INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (season_id) REFERENCES seasons(id)
);

-- Message Recipients - who should receive this message
CREATE TABLE IF NOT EXISTS message_recipients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    recipient_type VARCHAR(50) NOT NULL, -- 'Player', 'Captain', 'Team', 'Division', 'All'
    recipient_id VARCHAR(255), -- ID of recipient (null for broadcast messages)
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
);

-- Notifications table - ephemeral, can point to a message or exist independently
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    recipient_id VARCHAR(255) NOT NULL, -- UUID of the player or captain
    recipient_type VARCHAR(50) NOT NULL, -- 'Player' or 'Captain'
    message_id INTEGER, -- Optional reference to a message
    action_type VARCHAR(50), -- 'SubmitAvailability', 'ViewSelection', 'ConfirmAttendance', etc.
    action_url VARCHAR(255), -- URL for the action
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP,
    expires_at TIMESTAMP, -- When the notification should expire/disappear
    importance VARCHAR(20) NOT NULL DEFAULT 'Medium', -- 'Low', 'Medium', 'High', 'Urgent'
    related_entity_type VARCHAR(50), -- 'Fixture', 'Team', 'Division', 'League'
    related_entity_id INTEGER, -- ID of the related entity
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL
);

-- Scheduled Messages - for messages to be sent in the future
CREATE TABLE IF NOT EXISTS scheduled_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    scheduled_time TIMESTAMP NOT NULL,
    recurrence VARCHAR(50), -- 'None', 'Daily', 'Weekly', 'Monthly'
    is_sent BOOLEAN NOT NULL DEFAULT FALSE,
    last_sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
);

-- Delivery Methods - how notifications are delivered to recipients
CREATE TABLE IF NOT EXISTS delivery_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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

-- Triggers for validating message categories
CREATE TRIGGER IF NOT EXISTS chk_valid_message_category
BEFORE INSERT ON messages
FOR EACH ROW
WHEN NEW.category NOT IN ('General', 'Fixture', 'Availability', 'Selection', 'Results', 'Administrative')
BEGIN
    SELECT RAISE(FAIL, 'Invalid message category');
END;

-- Triggers for validating message importance
CREATE TRIGGER IF NOT EXISTS chk_valid_message_importance
BEFORE INSERT ON messages
FOR EACH ROW
WHEN NEW.importance NOT IN ('Low', 'Medium', 'High', 'Urgent')
BEGIN
    SELECT RAISE(FAIL, 'Invalid message importance');
END;

-- Triggers for validating notification importance
CREATE TRIGGER IF NOT EXISTS chk_valid_notification_importance
BEFORE INSERT ON notifications
FOR EACH ROW
WHEN NEW.importance NOT IN ('Low', 'Medium', 'High', 'Urgent')
BEGIN
    SELECT RAISE(FAIL, 'Invalid notification importance');
END;

-- Trigger for automatic timestamps
CREATE TRIGGER IF NOT EXISTS update_messages_updated_at
AFTER UPDATE ON messages
FOR EACH ROW
BEGIN
    UPDATE messages SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS update_message_recipients_updated_at
AFTER UPDATE ON message_recipients
FOR EACH ROW
BEGIN
    UPDATE message_recipients SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS update_notifications_updated_at
AFTER UPDATE ON notifications
FOR EACH ROW
BEGIN
    UPDATE notifications SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS update_scheduled_messages_updated_at
AFTER UPDATE ON scheduled_messages
FOR EACH ROW
BEGIN
    UPDATE scheduled_messages SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS update_delivery_preferences_updated_at
AFTER UPDATE ON delivery_preferences
FOR EACH ROW
BEGIN
    UPDATE delivery_preferences SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END; 