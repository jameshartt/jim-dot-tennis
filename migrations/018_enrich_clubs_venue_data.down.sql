-- Rollback Migration 018: Remove venue data columns from clubs table
-- SQLite does not support DROP COLUMN before 3.35.0, so we recreate the table

CREATE TABLE clubs_backup AS SELECT id, name, address, website, phone_number, created_at, updated_at FROM clubs;

DROP TABLE clubs;

CREATE TABLE clubs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    phone_number TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO clubs (id, name, address, website, phone_number, created_at, updated_at)
SELECT id, name, address, website, phone_number, created_at, updated_at FROM clubs_backup;

DROP TABLE clubs_backup;
