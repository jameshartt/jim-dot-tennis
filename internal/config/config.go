// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// AppConfig holds application-wide configuration loaded at startup
type AppConfig struct {
	HomeClubID     uint
	HomeClub       *models.Club
	BHPLTAClubCode string // BHPLTA club code/password for match card integration
}

// Load reads HOME_CLUB_ID (preferred) or HOME_CLUB_NAME (fallback) from environment,
// validates the club exists in the database, and returns the config.
func Load(ctx context.Context, clubRepo repository.ClubRepository) (*AppConfig, error) {
	bhpltaClubCode := os.Getenv("BHPLTA_CLUB_CODE")
	if bhpltaClubCode != "" {
		log.Printf("BHPLTA club code: configured")
	}

	// Try HOME_CLUB_ID first
	if idStr := os.Getenv("HOME_CLUB_ID"); idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("HOME_CLUB_ID=%q is not a valid unsigned integer: %w", idStr, err)
		}
		club, err := clubRepo.FindByID(ctx, uint(id))
		if err != nil {
			clubs, _ := clubRepo.FindAll(ctx)
			var names []string
			for _, c := range clubs {
				names = append(names, fmt.Sprintf("%s (ID: %d)", c.Name, c.ID))
			}
			return nil, fmt.Errorf("HOME_CLUB_ID=%d does not match any club in the database. Available clubs: %v", id, names)
		}
		log.Printf("Home club: %s (ID: %d)", club.Name, club.ID)
		return &AppConfig{HomeClubID: club.ID, HomeClub: club, BHPLTAClubCode: bhpltaClubCode}, nil
	}

	// Fallback to HOME_CLUB_NAME
	if name := os.Getenv("HOME_CLUB_NAME"); name != "" {
		clubs, err := clubRepo.FindByNameLike(ctx, name)
		if err != nil || len(clubs) == 0 {
			allClubs, _ := clubRepo.FindAll(ctx)
			var names []string
			for _, c := range allClubs {
				names = append(names, fmt.Sprintf("%s (ID: %d)", c.Name, c.ID))
			}
			return nil, fmt.Errorf("HOME_CLUB_NAME=%q did not match any club. Available clubs: %v", name, names)
		}
		club := &clubs[0]
		log.Printf("Home club (resolved via name): %s (ID: %d)", club.Name, club.ID)
		return &AppConfig{HomeClubID: club.ID, HomeClub: club, BHPLTAClubCode: bhpltaClubCode}, nil
	}

	return nil, fmt.Errorf("neither HOME_CLUB_ID nor HOME_CLUB_NAME is set; configure one in your environment")
}
