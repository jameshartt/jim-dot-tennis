-- Sprint 016 WI-094: Player tennis profile data model.
-- Three tables:
--   1. player_tennis_preferences — one row per player, all columns nullable
--      (captures self-authored 'My Tennis' answers)
--   2. player_preferred_partners — positive-only join table
--      (clicks_with / would_love_to_try)
--   3. captain_player_notes — captain-only notes surface, never queried
--      from player-facing handlers
--
-- JSON TEXT columns are used only for intrinsically list-shaped values:
--   preferred_days, preferred_times, improvement_focus.

CREATE TABLE IF NOT EXISTS player_tennis_preferences (
    player_id TEXT PRIMARY KEY,

    -- Identity & Vibe
    years_playing INTEGER,
    how_i_got_into_tennis TEXT,
    tennis_hero_or_style TEXT,
    pre_match_ritual TEXT,

    -- Match Types
    mixed_doubles_appetite TEXT,
    same_gender_doubles_appetite TEXT,
    open_to_fill_in BOOLEAN,

    -- Playing Style
    handedness TEXT,
    backhand TEXT,
    serve_style TEXT,
    net_comfort TEXT,
    preferred_court_side TEXT,
    signature_shot TEXT,
    shot_im_working_on TEXT,
    favourite_tactic TEXT,

    -- Partnership (scalar parts; partner lists live in player_preferred_partners)
    partner_consistency TEXT,
    on_court_vibe TEXT,

    -- Intensity & Goals
    competitiveness INTEGER,
    pressure_response TEXT,
    season_goal TEXT,
    improvement_focus TEXT,

    -- Logistics
    preferred_days TEXT,
    preferred_times TEXT,
    max_travel_miles INTEGER,
    transport TEXT,
    home_court_matters TEXT,

    -- Health & Access
    what_to_know_about_my_game TEXT,
    accessibility_notes TEXT,
    weather_tolerance TEXT,

    -- Fun & Playful
    tennis_spirit_animal TEXT,
    walkout_song TEXT,
    celebration_style TEXT,
    post_match TEXT,
    my_tennis_in_one_line TEXT,

    -- Communications
    preferred_contact TEXT,
    best_window_for_last_minute TEXT,
    notes_to_captain TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_player_tennis_preferences_updated
    ON player_tennis_preferences(updated_at);

-- Positive-only preferred partners. 'avoid' is deliberately absent — tactical
-- or negative information lives on captain_player_notes, never player-authored.
CREATE TABLE IF NOT EXISTS player_preferred_partners (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    partner_player_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(player_id, partner_player_id, kind),
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    FOREIGN KEY (partner_player_id) REFERENCES players(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_player_preferred_partners_player
    ON player_preferred_partners(player_id);
CREATE INDEX IF NOT EXISTS idx_player_preferred_partners_partner
    ON player_preferred_partners(partner_player_id);

CREATE TRIGGER IF NOT EXISTS chk_preferred_partner_kind
BEFORE INSERT ON player_preferred_partners
FOR EACH ROW
WHEN NEW.kind NOT IN ('clicks_with', 'would_love_to_try')
BEGIN
    SELECT RAISE(FAIL, 'Invalid preferred partner kind');
END;

CREATE TRIGGER IF NOT EXISTS chk_no_self_partner
BEFORE INSERT ON player_preferred_partners
FOR EACH ROW
WHEN NEW.player_id = NEW.partner_player_id
BEGIN
    SELECT RAISE(FAIL, 'Player cannot partner with themselves');
END;

-- Captain-only notes. Never queried from any internal/players/* handler.
-- Leak-regression coverage lives in Sprint 016 WI-100 and Sprint 017 WI-107.
CREATE TABLE IF NOT EXISTS captain_player_notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    author_user_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_captain_player_notes_player
    ON captain_player_notes(player_id);
CREATE INDEX IF NOT EXISTS idx_captain_player_notes_author
    ON captain_player_notes(author_user_id);

CREATE TRIGGER IF NOT EXISTS chk_captain_note_kind
BEFORE INSERT ON captain_player_notes
FOR EACH ROW
WHEN NEW.kind NOT IN ('partnership', 'general')
BEGIN
    SELECT RAISE(FAIL, 'Invalid captain note kind');
END;
