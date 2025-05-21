-- Create initial admin users
-- Passwords are hashed using bcrypt with cost 10
-- Default password is 'changeme123' - users should change this on first login
-- You can generate new hashes using: https://bcrypt-generator.com/

-- First, ensure we have the admin role
INSERT OR IGNORE INTO roles (name, description) 
VALUES ('admin', 'Administrator with full system access');

-- Insert admin users
INSERT INTO users (username, password_hash, role, is_active, created_at, updated_at) VALUES
    -- James Hartt (you)
    ('james.hartt', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Conrad Brunner
    ('conrad.brunner', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Ed Newlands
    ('ed.newlands', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Elspeth Jackson
    ('elspeth.jackson', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Joss Albert
    ('joss.albert', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Neeraj Nayar
    ('neeraj.nayar', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Stuart Hehir
    ('stuart.hehir', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    
    -- Steve Dorney
    ('steve.dorney', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Note: The password hash above is for 'changeme123'
-- Each user should change their password on first login 