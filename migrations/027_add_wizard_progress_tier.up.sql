-- Sprint 018 WI-108: stepped wizard for the My Tennis form.
--
-- A single integer per player tracks how far through the wizard they've
-- progressed. The server returns this integer to the player-facing GET
-- (so completion state can be shown without ever serving stored answers
-- back), and bumps it monotonically on submit (see repository
-- BumpWizardProgressTier). Tier definitions live in
-- internal/players/my_tennis_tiers.go — the field-to-tier mapping below
-- is the canonical mirror of that file.
--
-- 0 = no submissions at all (distinct from 'started but skipped tier 1').
-- 1..6 = highest tier any answer maps to (cascading; skipping is fine).

ALTER TABLE player_tennis_preferences
    ADD COLUMN wizard_progress_tier INTEGER NOT NULL DEFAULT 0;

-- Cascading backfill: existing Sprint 016 testers keep their progress.
-- For each row, wizard_progress_tier = the highest tier N for which any
-- field in tier N is non-null, or (for tier 4) any preferred-partner row
-- exists. Higher tiers checked first so the assignment wins on first hit.
UPDATE player_tennis_preferences
SET wizard_progress_tier = CASE
    WHEN years_playing            IS NOT NULL
      OR how_i_got_into_tennis    IS NOT NULL
      OR tennis_hero_or_style     IS NOT NULL
      OR pre_match_ritual         IS NOT NULL
      OR tennis_spirit_animal     IS NOT NULL
      OR walkout_song             IS NOT NULL
      OR celebration_style        IS NOT NULL
      OR post_match               IS NOT NULL
      OR my_tennis_in_one_line    IS NOT NULL
        THEN 6
    WHEN season_goal               IS NOT NULL
      OR improvement_focus         IS NOT NULL
      OR what_to_know_about_my_game IS NOT NULL
      OR accessibility_notes       IS NOT NULL
      OR weather_tolerance         IS NOT NULL
      OR notes_to_captain          IS NOT NULL
        THEN 5
    WHEN partner_consistency IS NOT NULL
      OR on_court_vibe       IS NOT NULL
      OR competitiveness     IS NOT NULL
      OR pressure_response   IS NOT NULL
      OR EXISTS (
          SELECT 1 FROM player_preferred_partners pp
          WHERE pp.player_id = player_tennis_preferences.player_id
        )
        THEN 4
    WHEN handedness          IS NOT NULL
      OR backhand            IS NOT NULL
      OR serve_style         IS NOT NULL
      OR net_comfort         IS NOT NULL
      OR preferred_court_side IS NOT NULL
      OR signature_shot      IS NOT NULL
      OR shot_im_working_on  IS NOT NULL
      OR favourite_tactic    IS NOT NULL
        THEN 3
    WHEN preferred_days      IS NOT NULL
      OR preferred_times     IS NOT NULL
      OR max_travel_miles    IS NOT NULL
      OR transport           IS NOT NULL
      OR home_court_matters  IS NOT NULL
        THEN 2
    WHEN mixed_doubles_appetite       IS NOT NULL
      OR same_gender_doubles_appetite IS NOT NULL
      OR open_to_fill_in              IS NOT NULL
      OR preferred_contact            IS NOT NULL
      OR best_window_for_last_minute  IS NOT NULL
        THEN 1
    ELSE 0
END;
