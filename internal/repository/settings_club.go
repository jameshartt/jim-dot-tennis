// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

// app_settings keys for the club settings page (WI-119). Effective values,
// defaults and environment overrides are resolved by config.ClubSettings.
const (
	// SettingClubDisplayName is the name shown in pages; unset means the home club's name.
	SettingClubDisplayName = "club_display_name"
	// SettingClubLogoFile is the file name (no directory) of the uploaded logo in the branding directory.
	SettingClubLogoFile = "club_logo_file"
	// SettingClubLogoSize is the uploaded logo's pixel size, "WIDTHxHEIGHT".
	SettingClubLogoSize = "club_logo_size"
	// SettingClubPrimaryColour and SettingClubSecondaryColour are "#rrggbb".
	SettingClubPrimaryColour   = "club_primary_colour"
	SettingClubSecondaryColour = "club_secondary_colour"
	// SettingBHPLTAClubCode is the club password for the BHPLTA club scores page.
	SettingBHPLTAClubCode = "bhplta_club_code"
	// SettingBHPLTAClubName is the club name exactly as BHPLTA's club scores page sends it.
	SettingBHPLTAClubName = "bhplta_club_name"
	// SettingBHPLTAClubID is BHPLTA's numeric id for the club.
	SettingBHPLTAClubID = "bhplta_club_id"
	// SettingPublicURL is the instance's public base URL (scheme and host, no trailing slash).
	SettingPublicURL = "public_url"

	// Feature switches (WI-122), stored as "true" or "false". Effective values
	// and defaults are resolved by config.ClubSettings.
	SettingFeatureTournaments = "feature_tournaments"
	SettingFeatureClubWrapped = "feature_club_wrapped"
	SettingFeatureMyTennis    = "feature_my_tennis"
)
