// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

// app_settings keys for the club's tournament set-up (WI-145, internal/tournaments).
// They hold identifiers on the bundled tournament server, not secrets: the
// service account's password is only ever in the environment.
const (
	// SettingTournamentProvisionerID is the tournament server's id for the
	// app's provisioner ("jim-dot-tennis").
	SettingTournamentProvisionerID = "tournament_provisioner_id"
	// SettingTournamentProviderID is the tournament server's id for the
	// club's provider. Once it is set the abbreviation below is fixed.
	SettingTournamentProviderID = "tournament_provider_id"
	// SettingTournamentProviderAbbr is the club's provider abbreviation
	// (short, upper case). Stored before the provider is made, so a set-up
	// that stops part-way tries the same abbreviation next time.
	SettingTournamentProviderAbbr = "tournament_provider_abbr"
	// SettingTournamentInstanceAccountID is the tournament server's id for
	// the permanent, never-used "instance" provider administrator.
	SettingTournamentInstanceAccountID = "tournament_instance_account_id"
	// SettingTournamentSetupLast is the outcome of the last set-up run, as
	// JSON (internal/tournaments.Result): when, each step, the failure if
	// any, and the tournament accounts the app does not manage.
	SettingTournamentSetupLast = "tournament_setup_last"
)
