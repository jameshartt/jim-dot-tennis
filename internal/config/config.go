// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// defaultHomeClubLogoPath is the fallback logo path used when HOME_CLUB_LOGO_PATH is unset.
// It points at the St Ann's asset bundled with this repository; new deployments should override
// it via the HOME_CLUB_LOGO_PATH environment variable (see CONTRIBUTING.md#club-adaptation-guide).
const defaultHomeClubLogoPath = "/static/st-anns-tennis.jpg"

// Where the home club was resolved from (see ResolveHomeClub).
const (
	SourceNone    = ""
	SourceEnvID   = "HOME_CLUB_ID"
	SourceEnvName = "HOME_CLUB_NAME"
	SourceStored  = "stored setting"
)

// AppConfig is a one-shot snapshot of the home club configuration, used by the
// command-line tools. The web server uses the live HomeClubHolder instead,
// because there the home club can change at runtime (when setup completes).
type AppConfig struct {
	HomeClubID       uint
	HomeClub         *models.Club
	HomeClubLogoPath string // Path (relative to the server) of the home club's logo image
	BHPLTAClubCode   string // BHPLTA club code/password for match card integration
}

// HomeClubError reports a configured home club that does not match any club.
type HomeClubError struct {
	Source string // SourceEnvID, SourceEnvName or SourceStored
	msg    string
}

func (e *HomeClubError) Error() string { return e.msg }

// ResolveHomeClub finds the home club, in this order:
//
//  1. HOME_CLUB_ID environment variable
//  2. HOME_CLUB_NAME environment variable (first fuzzy match)
//  3. the home_club_id stored setting (chosen during setup)
//
// It returns (nil, SourceNone, nil) when none of them is set. When a source is
// set but matches no club it returns a *HomeClubError listing the available
// clubs; a stored setting is only consulted when neither env variable is set.
// settings may be nil (stored setting skipped).
func ResolveHomeClub(ctx context.Context, clubRepo repository.ClubRepository, settings repository.SettingsRepository) (*models.Club, string, error) {
	if idStr := os.Getenv("HOME_CLUB_ID"); idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, SourceEnvID, fmt.Errorf("HOME_CLUB_ID=%q is not a valid unsigned integer: %w", idStr, err)
		}
		club, err := clubRepo.FindByID(ctx, uint(id))
		if err != nil {
			return nil, SourceEnvID, &HomeClubError{Source: SourceEnvID, msg: fmt.Sprintf(
				"HOME_CLUB_ID=%d does not match any club in the database. Available clubs: %v", id, availableClubs(ctx, clubRepo))}
		}
		return club, SourceEnvID, nil
	}

	if name := os.Getenv("HOME_CLUB_NAME"); name != "" {
		clubs, err := clubRepo.FindByNameLike(ctx, name)
		if err != nil || len(clubs) == 0 {
			return nil, SourceEnvName, &HomeClubError{Source: SourceEnvName, msg: fmt.Sprintf(
				"HOME_CLUB_NAME=%q did not match any club. Available clubs: %v", name, availableClubs(ctx, clubRepo))}
		}
		return &clubs[0], SourceEnvName, nil
	}

	if settings == nil {
		return nil, SourceNone, nil
	}
	id, ok, err := settings.GetUint(ctx, repository.SettingHomeClubID)
	if err != nil {
		return nil, SourceStored, err
	}
	if !ok || id == 0 {
		return nil, SourceNone, nil
	}
	club, err := clubRepo.FindByID(ctx, id)
	if err != nil {
		return nil, SourceStored, &HomeClubError{Source: SourceStored, msg: fmt.Sprintf(
			"stored home club setting (ID %d) does not match any club in the database. Available clubs: %v", id, availableClubs(ctx, clubRepo))}
	}
	return club, SourceStored, nil
}

func availableClubs(ctx context.Context, clubRepo repository.ClubRepository) []string {
	clubs, _ := clubRepo.FindAll(ctx)
	var names []string
	for _, c := range clubs {
		names = append(names, fmt.Sprintf("%s (ID: %d)", c.Name, c.ID))
	}
	return names
}

func logResolvedHomeClub(club *models.Club, source string) {
	switch source {
	case SourceEnvName:
		log.Printf("Home club (resolved via name): %s (ID: %d)", club.Name, club.ID)
	case SourceStored:
		log.Printf("Home club (stored setting): %s (ID: %d)", club.Name, club.ID)
	default:
		log.Printf("Home club: %s (ID: %d)", club.Name, club.ID)
	}
}

// loadStaticSettings reads the env-only settings that do not change at runtime.
func loadStaticSettings() (logoPath, bhpltaClubCode string) {
	bhpltaClubCode = os.Getenv("BHPLTA_CLUB_CODE")
	if bhpltaClubCode != "" {
		log.Printf("BHPLTA club code: configured")
	}
	logoPath = os.Getenv("HOME_CLUB_LOGO_PATH")
	if logoPath == "" {
		logoPath = defaultHomeClubLogoPath
	} else {
		log.Printf("Home club logo path: %s", logoPath)
	}
	return logoPath, bhpltaClubCode
}

// Load resolves the home club once (see ResolveHomeClub) and returns a snapshot.
// It is for the command-line tools, which cannot run without a home club;
// settings may be nil to consider the environment only.
func Load(ctx context.Context, clubRepo repository.ClubRepository, settings repository.SettingsRepository) (*AppConfig, error) {
	logoPath, bhpltaClubCode := loadStaticSettings()
	club, source, err := ResolveHomeClub(ctx, clubRepo, settings)
	if err != nil {
		return nil, err
	}
	if club == nil {
		return nil, fmt.Errorf("no home club configured: set HOME_CLUB_ID or HOME_CLUB_NAME in your environment, or complete setup in the web app")
	}
	logResolvedHomeClub(club, source)
	return &AppConfig{HomeClubID: club.ID, HomeClub: club, HomeClubLogoPath: logoPath, BHPLTAClubCode: bhpltaClubCode}, nil
}

// isHomeClubMismatch reports whether err is a "configured club not found" error.
func isHomeClubMismatch(err error) bool {
	var hcErr *HomeClubError
	return errors.As(err, &hcErr)
}
