-- Instance-wide settings stored as key/value pairs.
--
-- Introduced for the fresh-install flow (Sprint 020, WI-116): the home club is
-- now a stored setting chosen during setup ('home_club_id'), with HOME_CLUB_ID /
-- HOME_CLUB_NAME kept as environment overrides. Later work items store club
-- identity, feature switches and per-instance secrets here too.
--
-- Values are text; typed access (uint, bool, time) lives in
-- internal/repository/settings.go.

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
