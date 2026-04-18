// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// PlayerTennisPreferenceRepository defines data access for the 'My Tennis'
// taxonomy (player_tennis_preferences + player_preferred_partners).
//
// Privacy note: callers served under a fantasy token URL must NEVER invoke
// FindByPlayerID or ListPreferredPartners for read-back to the player —
// those methods exist solely to back admin-session surfaces.
type PlayerTennisPreferenceRepository interface {
	FindByPlayerID(ctx context.Context, playerID string) (*models.PlayerTennisPreferences, error)
	UpsertMerge(ctx context.Context, prefs *models.PlayerTennisPreferences) error

	ListPreferredPartners(ctx context.Context, playerID string) ([]models.PlayerPreferredPartner, error)
	ReplacePartnersOfKind(ctx context.Context, playerID string, kind models.PreferredPartnerKind, partnerPlayerIDs []string) error
}

type playerTennisPreferenceRepository struct {
	db *database.DB
}

// NewPlayerTennisPreferenceRepository creates a new preferences repository.
func NewPlayerTennisPreferenceRepository(db *database.DB) PlayerTennisPreferenceRepository {
	return &playerTennisPreferenceRepository{db: db}
}

const playerTennisPreferenceColumns = `
	player_id,
	years_playing, how_i_got_into_tennis, tennis_hero_or_style, pre_match_ritual,
	mixed_doubles_appetite, same_gender_doubles_appetite, open_to_fill_in,
	handedness, backhand, serve_style, net_comfort, preferred_court_side,
	signature_shot, shot_im_working_on, favourite_tactic,
	partner_consistency, on_court_vibe,
	competitiveness, pressure_response, season_goal, improvement_focus,
	preferred_days, preferred_times, max_travel_miles, transport, home_court_matters,
	what_to_know_about_my_game, accessibility_notes, weather_tolerance,
	tennis_spirit_animal, walkout_song, celebration_style, post_match, my_tennis_in_one_line,
	preferred_contact, best_window_for_last_minute, notes_to_captain,
	created_at, updated_at`

func (r *playerTennisPreferenceRepository) FindByPlayerID(ctx context.Context, playerID string) (*models.PlayerTennisPreferences, error) {
	var prefs models.PlayerTennisPreferences
	err := r.db.GetContext(ctx, &prefs, `
		SELECT `+playerTennisPreferenceColumns+`
		FROM player_tennis_preferences
		WHERE player_id = ?
	`, playerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

// UpsertMerge applies merge semantics: non-nil fields on prefs overwrite the
// stored row; nil fields leave the existing value in place. JSON array columns
// follow the same rule — to explicitly clear one, pass an empty-array JSON
// literal ("[]") rather than nil.
func (r *playerTennisPreferenceRepository) UpsertMerge(ctx context.Context, prefs *models.PlayerTennisPreferences) error {
	now := time.Now()
	if prefs.CreatedAt.IsZero() {
		prefs.CreatedAt = now
	}
	prefs.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO player_tennis_preferences (
			player_id,
			years_playing, how_i_got_into_tennis, tennis_hero_or_style, pre_match_ritual,
			mixed_doubles_appetite, same_gender_doubles_appetite, open_to_fill_in,
			handedness, backhand, serve_style, net_comfort, preferred_court_side,
			signature_shot, shot_im_working_on, favourite_tactic,
			partner_consistency, on_court_vibe,
			competitiveness, pressure_response, season_goal, improvement_focus,
			preferred_days, preferred_times, max_travel_miles, transport, home_court_matters,
			what_to_know_about_my_game, accessibility_notes, weather_tolerance,
			tennis_spirit_animal, walkout_song, celebration_style, post_match, my_tennis_in_one_line,
			preferred_contact, best_window_for_last_minute, notes_to_captain,
			created_at, updated_at
		) VALUES (
			:player_id,
			:years_playing, :how_i_got_into_tennis, :tennis_hero_or_style, :pre_match_ritual,
			:mixed_doubles_appetite, :same_gender_doubles_appetite, :open_to_fill_in,
			:handedness, :backhand, :serve_style, :net_comfort, :preferred_court_side,
			:signature_shot, :shot_im_working_on, :favourite_tactic,
			:partner_consistency, :on_court_vibe,
			:competitiveness, :pressure_response, :season_goal, :improvement_focus,
			:preferred_days, :preferred_times, :max_travel_miles, :transport, :home_court_matters,
			:what_to_know_about_my_game, :accessibility_notes, :weather_tolerance,
			:tennis_spirit_animal, :walkout_song, :celebration_style, :post_match, :my_tennis_in_one_line,
			:preferred_contact, :best_window_for_last_minute, :notes_to_captain,
			:created_at, :updated_at
		)
		ON CONFLICT(player_id) DO UPDATE SET
			years_playing               = COALESCE(excluded.years_playing,               player_tennis_preferences.years_playing),
			how_i_got_into_tennis       = COALESCE(excluded.how_i_got_into_tennis,       player_tennis_preferences.how_i_got_into_tennis),
			tennis_hero_or_style        = COALESCE(excluded.tennis_hero_or_style,        player_tennis_preferences.tennis_hero_or_style),
			pre_match_ritual            = COALESCE(excluded.pre_match_ritual,            player_tennis_preferences.pre_match_ritual),
			mixed_doubles_appetite      = COALESCE(excluded.mixed_doubles_appetite,      player_tennis_preferences.mixed_doubles_appetite),
			same_gender_doubles_appetite= COALESCE(excluded.same_gender_doubles_appetite,player_tennis_preferences.same_gender_doubles_appetite),
			open_to_fill_in             = COALESCE(excluded.open_to_fill_in,             player_tennis_preferences.open_to_fill_in),
			handedness                  = COALESCE(excluded.handedness,                  player_tennis_preferences.handedness),
			backhand                    = COALESCE(excluded.backhand,                    player_tennis_preferences.backhand),
			serve_style                 = COALESCE(excluded.serve_style,                 player_tennis_preferences.serve_style),
			net_comfort                 = COALESCE(excluded.net_comfort,                 player_tennis_preferences.net_comfort),
			preferred_court_side        = COALESCE(excluded.preferred_court_side,        player_tennis_preferences.preferred_court_side),
			signature_shot              = COALESCE(excluded.signature_shot,              player_tennis_preferences.signature_shot),
			shot_im_working_on          = COALESCE(excluded.shot_im_working_on,          player_tennis_preferences.shot_im_working_on),
			favourite_tactic            = COALESCE(excluded.favourite_tactic,            player_tennis_preferences.favourite_tactic),
			partner_consistency         = COALESCE(excluded.partner_consistency,         player_tennis_preferences.partner_consistency),
			on_court_vibe               = COALESCE(excluded.on_court_vibe,               player_tennis_preferences.on_court_vibe),
			competitiveness             = COALESCE(excluded.competitiveness,             player_tennis_preferences.competitiveness),
			pressure_response           = COALESCE(excluded.pressure_response,           player_tennis_preferences.pressure_response),
			season_goal                 = COALESCE(excluded.season_goal,                 player_tennis_preferences.season_goal),
			improvement_focus           = COALESCE(excluded.improvement_focus,           player_tennis_preferences.improvement_focus),
			preferred_days              = COALESCE(excluded.preferred_days,              player_tennis_preferences.preferred_days),
			preferred_times             = COALESCE(excluded.preferred_times,             player_tennis_preferences.preferred_times),
			max_travel_miles            = COALESCE(excluded.max_travel_miles,            player_tennis_preferences.max_travel_miles),
			transport                   = COALESCE(excluded.transport,                   player_tennis_preferences.transport),
			home_court_matters          = COALESCE(excluded.home_court_matters,          player_tennis_preferences.home_court_matters),
			what_to_know_about_my_game  = COALESCE(excluded.what_to_know_about_my_game,  player_tennis_preferences.what_to_know_about_my_game),
			accessibility_notes         = COALESCE(excluded.accessibility_notes,         player_tennis_preferences.accessibility_notes),
			weather_tolerance           = COALESCE(excluded.weather_tolerance,           player_tennis_preferences.weather_tolerance),
			tennis_spirit_animal        = COALESCE(excluded.tennis_spirit_animal,        player_tennis_preferences.tennis_spirit_animal),
			walkout_song                = COALESCE(excluded.walkout_song,                player_tennis_preferences.walkout_song),
			celebration_style           = COALESCE(excluded.celebration_style,           player_tennis_preferences.celebration_style),
			post_match                  = COALESCE(excluded.post_match,                  player_tennis_preferences.post_match),
			my_tennis_in_one_line       = COALESCE(excluded.my_tennis_in_one_line,       player_tennis_preferences.my_tennis_in_one_line),
			preferred_contact           = COALESCE(excluded.preferred_contact,           player_tennis_preferences.preferred_contact),
			best_window_for_last_minute = COALESCE(excluded.best_window_for_last_minute, player_tennis_preferences.best_window_for_last_minute),
			notes_to_captain            = COALESCE(excluded.notes_to_captain,            player_tennis_preferences.notes_to_captain),
			updated_at                  = excluded.updated_at
	`, prefs)
	return err
}

func (r *playerTennisPreferenceRepository) ListPreferredPartners(ctx context.Context, playerID string) ([]models.PlayerPreferredPartner, error) {
	var rows []models.PlayerPreferredPartner
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, player_id, partner_player_id, kind, created_at
		FROM player_preferred_partners
		WHERE player_id = ?
		ORDER BY kind, created_at
	`, playerID)
	return rows, err
}

// ReplacePartnersOfKind atomically deletes all rows for (playerID, kind) and
// inserts the supplied partnerPlayerIDs. An empty slice clears the list.
func (r *playerTennisPreferenceRepository) ReplacePartnersOfKind(ctx context.Context, playerID string, kind models.PreferredPartnerKind, partnerPlayerIDs []string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM player_preferred_partners
		WHERE player_id = ? AND kind = ?
	`, playerID, string(kind)); err != nil {
		return err
	}

	now := time.Now()
	for _, pid := range partnerPlayerIDs {
		if pid == "" || pid == playerID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO player_preferred_partners
				(player_id, partner_player_id, kind, created_at)
			VALUES (?, ?, ?, ?)
		`, playerID, pid, string(kind), now); err != nil {
			return err
		}
	}

	return tx.Commit()
}
