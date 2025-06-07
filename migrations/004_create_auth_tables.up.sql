-- +migrate Up
-- Create users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'captain', 'player')),
    player_id TEXT UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    last_activity_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    device_info TEXT NOT NULL,
    is_valid BOOLEAN NOT NULL DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create login attempts table
CREATE TABLE login_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_is_valid ON sessions(is_valid);
CREATE INDEX idx_login_attempts_username ON login_attempts(username);
CREATE INDEX idx_login_attempts_ip ON login_attempts(ip);
CREATE INDEX idx_login_attempts_created_at ON login_attempts(created_at);

-- Insert default admin user with password
-- In production, change this password immediately!
INSERT INTO users (username, password_hash, role, is_active, created_at, last_login_at)
VALUES ('james.hartt', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'admin', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Insert additional admin users with password
-- In production, change these passwords immediately!
INSERT INTO users (username, password_hash, role, is_active, created_at, last_login_at) VALUES
('conrad.brunner', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('ed.newlands', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('elspeth.jackson', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('joss.albert', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('neeraj.nayar', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('steve.dorney', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('stuart.hehir', '$2a$12$.r6Ne5mFTS3RQ.XHcxS3MOMRmB7jn0vw3YTblwncMc9FIOnNYX4ay', 'captain', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

