// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"strconv"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/repository"
)

// BHPLTAClubIdentifiers are the values BHPLTA's club scores page uses to
// identify a club when fetching match cards.
type BHPLTAClubIdentifiers struct {
	Name string // club name as BHPLTA sends it (e.g. "St+Anns")
	ID   int    // BHPLTA's numeric club id
}

// knownBHPLTAClubs holds identifiers confirmed by a working match-card import,
// keyed by normalize.ForComparison of the club names they apply to.
//
// BHPLTA does not publish these: they appear only on the club scores page
// after signing in with the club password (data-clubid / data-clubname on the
// page heading). Add a club here only once its values have been seen working.
var knownBHPLTAClubs = []struct {
	names []string
	ids   BHPLTAClubIdentifiers
}{
	{names: []string{"st anns", "st anns tennis", "st anns tennis club"}, ids: BHPLTAClubIdentifiers{Name: "St+Anns", ID: 10}},
}

// KnownBHPLTAClub returns the known BHPLTA identifiers for club, if any.
func KnownBHPLTAClub(club *models.Club) (BHPLTAClubIdentifiers, bool) {
	if club == nil {
		return BHPLTAClubIdentifiers{}, false
	}
	name := normalize.ForComparison(club.Name)
	for _, k := range knownBHPLTAClubs {
		for _, n := range k.names {
			if n == name {
				return k.ids, true
			}
		}
	}
	return BHPLTAClubIdentifiers{}, false
}

// PrefillClubSettings stores whatever club settings can be derived from the
// club chosen during setup, without overwriting anything already stored.
// Today that is the BHPLTA club name and id for clubs in the known list; for
// other clubs nothing is derivable without the club password, and the settings
// page explains where to find them (or looks them up from the club code).
//
// settings may be bound to a transaction (repository.NewSettingsRepository(tx)).
// It returns the keys it stored. After the transaction commits, the live
// values are picked up by HomeClubHolder.Refresh.
func PrefillClubSettings(ctx context.Context, settings repository.SettingsRepository, club *models.Club) ([]string, error) {
	ids, ok := KnownBHPLTAClub(club)
	if !ok {
		return nil, nil
	}
	var stored []string
	set := func(key, value string) error {
		_, exists, err := settings.GetString(ctx, key)
		if err != nil || exists {
			return err
		}
		if err := settings.SetString(ctx, key, value); err != nil {
			return err
		}
		stored = append(stored, key)
		return nil
	}
	if err := set(repository.SettingBHPLTAClubName, ids.Name); err != nil {
		return stored, err
	}
	if err := set(repository.SettingBHPLTAClubID, strconv.Itoa(ids.ID)); err != nil {
		return stored, err
	}
	return stored, nil
}
