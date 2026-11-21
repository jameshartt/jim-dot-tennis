// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"log"
	"sync"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// HomeClubSource is the read-only view of the home club ID that services
// depend on. *HomeClubHolder implements it; tests can use StaticHomeClubID.
type HomeClubSource interface {
	ID() uint
}

// StaticHomeClubID is a fixed HomeClubSource (for tests and tools).
type StaticHomeClubID uint

// ID returns the fixed home club ID.
func (s StaticHomeClubID) ID() uint { return uint(s) }

// unresolvedLogInterval throttles "still cannot resolve" log lines on the request path.
const unresolvedLogInterval = time.Minute

// HomeClubHolder is the live, concurrency-safe home club for the web server.
//
// It is resolved at startup (see ResolveHomeClub for the order). While no home
// club is resolved, Current retries resolution on each call, so a club that
// appears later (env override pointing at a club that has since been seeded or
// imported) is picked up without a restart. Once resolved, reads take a read
// lock only and never touch the database. Refresh forces a re-resolution, e.g.
// after setup stores the chosen club.
type HomeClubHolder struct {
	clubRepo     repository.ClubRepository
	settings     repository.SettingsRepository
	clubSettings *ClubSettings

	resolveMu sync.Mutex // serialises resolution attempts

	mu         sync.RWMutex
	id         uint
	club       *models.Club
	source     string
	lastErrLog time.Time
}

// NewHomeClubHolder resolves the home club for the web server.
//
// It returns an error (the caller should exit) exactly where the server used to
// refuse to start: an invalid HOME_CLUB_ID, or an env value that matches none
// of the clubs in a database that has clubs. It does not fail when nothing is
// configured, when the stored setting is stale, or when an env value is set but
// the clubs table is still empty (a brand-new database): the holder is left
// unresolved and the server starts in setup mode.
func NewHomeClubHolder(ctx context.Context, clubRepo repository.ClubRepository, settings repository.SettingsRepository) (*HomeClubHolder, error) {
	h := &HomeClubHolder{
		clubRepo:     clubRepo,
		settings:     settings,
		clubSettings: NewClubSettings(ctx, settings),
	}

	club, source, err := ResolveHomeClub(ctx, clubRepo, settings)
	switch {
	case err == nil && club != nil:
		h.set(club, source)
		logResolvedHomeClub(club, source)
	case err == nil:
		log.Printf("No home club configured yet (HOME_CLUB_ID / HOME_CLUB_NAME unset, no stored setting): starting in setup mode")
	case isHomeClubMismatch(err) && source == SourceStored:
		log.Printf("Warning: %v; starting in setup mode", err)
	case isHomeClubMismatch(err) && h.clubsTableEmpty(ctx):
		log.Printf("Warning: %s is set but the database has no clubs yet; "+
			"it will be resolved once the club exists (starting in setup mode)", source)
	default:
		return nil, err
	}
	return h, nil
}

func (h *HomeClubHolder) clubsTableEmpty(ctx context.Context) bool {
	clubs, err := h.clubRepo.FindAll(ctx)
	return err == nil && len(clubs) == 0
}

func (h *HomeClubHolder) set(club *models.Club, source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if club == nil {
		h.id, h.club, h.source = 0, nil, SourceNone
		return
	}
	h.id, h.club, h.source = club.ID, club, source
}

// ID returns the resolved home club ID, or 0 while unresolved. It never
// queries the database.
func (h *HomeClubHolder) ID() uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.id
}

// Club returns the resolved home club, or nil while unresolved. It never
// queries the database. Callers must not modify the returned value.
func (h *HomeClubHolder) Club() *models.Club {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.club
}

// Source reports where the current home club came from (SourceEnvID,
// SourceEnvName, SourceStored, or SourceNone while unresolved).
func (h *HomeClubHolder) Source() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.source
}

// Resolved reports whether a home club is currently in effect.
func (h *HomeClubHolder) Resolved() bool {
	return h.ID() != 0
}

// Current returns the home club ID and model. While unresolved it first tries
// to resolve again (errors are logged, throttled, and otherwise ignored); the
// result is (0, nil) if that still fails.
func (h *HomeClubHolder) Current(ctx context.Context) (uint, *models.Club) {
	h.mu.RLock()
	id, club := h.id, h.club
	h.mu.RUnlock()
	if id != 0 {
		return id, club
	}

	if err := h.Refresh(ctx); err != nil {
		h.mu.Lock()
		if time.Since(h.lastErrLog) >= unresolvedLogInterval {
			h.lastErrLog = time.Now()
			log.Printf("Home club still unresolved: %v", err)
		}
		h.mu.Unlock()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.id, h.club
}

// Refresh re-resolves the home club and makes the result effective
// immediately. Call it after changing the stored home club setting. On error
// the previous value is kept. If nothing is configured any more the holder
// becomes unresolved. It also reloads the stored club settings (for example
// the BHPLTA identifiers written by PrefillClubSettings during setup).
func (h *HomeClubHolder) Refresh(ctx context.Context) error {
	h.resolveMu.Lock()
	defer h.resolveMu.Unlock()

	if err := h.clubSettings.Reload(ctx); err != nil {
		log.Printf("Warning: could not reload club settings: %v", err)
	}

	club, source, err := ResolveHomeClub(ctx, h.clubRepo, h.settings)
	if err != nil {
		return err
	}
	previous := h.ID()
	h.set(club, source)
	if club != nil && club.ID != previous {
		logResolvedHomeClub(club, source)
	}
	return nil
}

// ClubSettings returns the live club settings (name, logo, colours, BHPLTA
// identifiers, public URL), stored in app_settings with env overrides.
func (h *HomeClubHolder) ClubSettings() *ClubSettings { return h.clubSettings }

// LogoPath returns the effective home club logo path: HOME_CLUB_LOGO_PATH if
// set, else the uploaded logo, else "" (no logo).
func (h *HomeClubHolder) LogoPath() string { return h.clubSettings.LogoPath() }

// BHPLTAClubCode returns the effective BHPLTA club code: BHPLTA_CLUB_CODE if
// set, else the stored code.
func (h *HomeClubHolder) BHPLTAClubCode() string { return h.clubSettings.BHPLTAClubCode() }

// Branding returns the effective branding for the current home club.
func (h *HomeClubHolder) Branding() Branding { return h.clubSettings.Branding(h.Club()) }
