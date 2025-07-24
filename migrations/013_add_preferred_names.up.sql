-- Add preferred_name column to players table
ALTER TABLE players ADD COLUMN preferred_name VARCHAR(255);

-- Create index for preferred_name uniqueness and lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_preferred_name ON players(preferred_name) WHERE preferred_name IS NOT NULL;

-- Create preferred name requests table for admin approval workflow
CREATE TABLE IF NOT EXISTS preferred_name_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    requested_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Pending', -- 'Pending', 'Approved', 'Rejected'
    admin_notes TEXT,
    approved_by TEXT, -- Admin user who approved/rejected
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP, -- When admin took action
    FOREIGN KEY (player_id) REFERENCES players(id)
);

-- Create indexes for preferred name requests
CREATE INDEX IF NOT EXISTS idx_preferred_name_requests_player_id ON preferred_name_requests(player_id);
CREATE INDEX IF NOT EXISTS idx_preferred_name_requests_status ON preferred_name_requests(status);
CREATE INDEX IF NOT EXISTS idx_preferred_name_requests_created_at ON preferred_name_requests(created_at);

-- Create trigger to ensure requested name doesn't conflict with existing preferred names
CREATE TRIGGER IF NOT EXISTS chk_preferred_name_unique
BEFORE INSERT ON preferred_name_requests
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM players WHERE preferred_name = NEW.requested_name
    UNION
    SELECT 1 FROM preferred_name_requests WHERE requested_name = NEW.requested_name AND status = 'Pending'
)
BEGIN
    SELECT RAISE(FAIL, 'Preferred name already exists or is pending approval');
END;

-- Create trigger to validate preferred name request status
CREATE TRIGGER IF NOT EXISTS chk_preferred_name_request_status
BEFORE INSERT ON preferred_name_requests
FOR EACH ROW
WHEN NEW.status NOT IN ('Pending', 'Approved', 'Rejected')
BEGIN
    SELECT RAISE(FAIL, 'Invalid preferred name request status');
END;

-- Create trigger to update timestamps on preferred name requests
CREATE TRIGGER IF NOT EXISTS update_preferred_name_requests_timestamp
AFTER UPDATE ON preferred_name_requests
FOR EACH ROW
BEGIN
    UPDATE preferred_name_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END; 