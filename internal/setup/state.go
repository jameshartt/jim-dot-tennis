// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package setup holds the first-run "needs setup" state of an instance and the
// setup entry page.
//
// An instance needs setup while no home club is in effect (neither
// HOME_CLUB_ID / HOME_CLUB_NAME nor the stored home_club_id setting resolves to
// a club) or no administrator account exists. While it does, Gate sends every request except static assets, the uploaded logo, the
// health endpoint and /setup... to the setup entry page.
//
// The setup flow (WI-117) completes setup by writing the completion settings
// (WriteCompletion, inside the same transaction that creates the first
// administrator) and then calling State.Activate after commit, or by calling
// State.MarkComplete when no other writes need to share the transaction. Either
// way the home club takes effect immediately, with no restart.
package setup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

// Status is the setup state of the instance.
type Status int

const (
	// StatusReady: a home club is in effect; the app runs normally.
	StatusReady Status = iota
	// StatusNeedsSetup: no home club and no administrator: a brand-new instance.
	StatusNeedsSetup
	// StatusNeedsClub: no home club, but an administrator account exists (for
	// example a database created before seeded accounts were removed, or a
	// setup that stopped after creating the administrator).
	StatusNeedsClub
	// StatusNeedsAdmin: a home club is in effect (HOME_CLUB_ID / HOME_CLUB_NAME
	// or a stored choice) but no administrator account exists, so nobody could
	// sign in to run the instance. The code-protected setup flow creates the
	// first administrator and then hands over to the dashboard (no club step).
	StatusNeedsAdmin
)

// String returns a short name for the status (used in logs and templates).
func (s Status) String() string {
	switch s {
	case StatusReady:
		return "ready"
	case StatusNeedsSetup:
		return "needs-setup"
	case StatusNeedsClub:
		return "needs-club"
	case StatusNeedsAdmin:
		return "needs-admin"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// NeedsFirstAdmin reports whether st is a state in which the first
// administrator may be created (with the setup code): no administrator exists.
func (s Status) NeedsFirstAdmin() bool {
	return s == StatusNeedsSetup || s == StatusNeedsAdmin
}

// State answers "does this instance need setup?" and completes setup.
type State struct {
	db     *database.DB
	holder *config.HomeClubHolder
	// adminSeen latches once an administrator has been seen with a home club
	// in effect, so the ready path stays a lock-only read. Removing every
	// administrator later only takes effect at the next restart.
	adminSeen atomic.Bool
}

// NewState returns the setup state for db, using holder as the live home club.
func NewState(db *database.DB, holder *config.HomeClubHolder) *State {
	return &State{db: db, holder: holder}
}

// HomeClub returns the live home club holder.
func (s *State) HomeClub() *config.HomeClubHolder { return s.holder }

// HasAdmin reports whether any administrator account exists (active or not).
func (s *State) HasAdmin(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE role = 'admin')`)
	if err != nil {
		return false, fmt.Errorf("check for administrator accounts: %w", err)
	}
	return exists, nil
}

// Status returns the current setup status:
//
//	home club   administrator   status
//	yes         yes             StatusReady
//	yes         no              StatusNeedsAdmin
//	no          yes             StatusNeedsClub
//	no          no              StatusNeedsSetup
//
// Other (captain, player) accounts do not count as administrators. Once a home
// club and an administrator have been seen this is a lock-only read; until
// then it retries resolution and checks for administrators. A database error
// while checking is logged and treated as StatusReady when a home club is in
// effect (as before: setup steps stay unavailable) and as StatusNeedsSetup
// otherwise (the app stays gated).
func (s *State) Status(ctx context.Context) Status {
	if id, _ := s.holder.Current(ctx); id != 0 {
		if s.adminSeen.Load() {
			return StatusReady
		}
		hasAdmin, err := s.HasAdmin(ctx)
		if err != nil {
			log.Printf("setup: %v", err)
			return StatusReady
		}
		if !hasAdmin {
			return StatusNeedsAdmin
		}
		s.adminSeen.Store(true)
		return StatusReady
	}
	hasAdmin, err := s.HasAdmin(ctx)
	if err != nil {
		log.Printf("setup: %v", err)
		return StatusNeedsSetup
	}
	if hasAdmin {
		return StatusNeedsClub
	}
	return StatusNeedsSetup
}

// NeedsSetup reports whether requests should be routed to setup.
func (s *State) NeedsSetup(ctx context.Context) bool {
	return s.Status(ctx) != StatusReady
}

// WriteCompletion records the chosen home club and the setup completion time
// using db, which is normally the *sqlx.Tx that also creates the first
// administrator. The club must already exist. Nothing takes effect until the
// transaction commits and State.Activate is called.
func WriteCompletion(ctx context.Context, db sqlx.ExtContext, clubID uint) error {
	if clubID == 0 {
		return errors.New("setup: home club ID must not be 0")
	}
	var id uint
	err := sqlx.GetContext(ctx, db, &id, `SELECT id FROM clubs WHERE id = ?`, clubID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("setup: club %d does not exist", clubID)
	}
	if err != nil {
		return fmt.Errorf("setup: look up club %d: %w", clubID, err)
	}

	settings := repository.NewSettingsRepository(db)
	if err := settings.SetUint(ctx, repository.SettingHomeClubID, clubID); err != nil {
		return err
	}
	return settings.SetTime(ctx, repository.SettingSetupCompletedAt, time.Now())
}

// Activate makes the stored completion settings effective in this process
// (call it after the transaction that ran WriteCompletion has committed). It
// returns an error if no home club is in effect afterwards.
//
// HOME_CLUB_ID / HOME_CLUB_NAME, when set, still take precedence over the
// stored choice; a mismatch is logged.
func (s *State) Activate(ctx context.Context) error {
	if err := s.holder.Refresh(ctx); err != nil {
		return fmt.Errorf("setup: activate home club: %w", err)
	}
	if !s.holder.Resolved() {
		return errors.New("setup: completion recorded but no home club is in effect")
	}
	if src := s.holder.Source(); src != config.SourceStored {
		log.Printf("setup: stored home club recorded, but %s overrides it (home club ID %d)", src, s.holder.ID())
	}
	return nil
}

// MarkComplete records clubID as the home club, marks setup complete, and makes
// it effective immediately. Use WriteCompletion + Activate instead when other
// writes (such as creating the first administrator) must share the transaction.
func (s *State) MarkComplete(ctx context.Context, clubID uint) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("setup: begin: %w", err)
	}
	if err := WriteCompletion(ctx, tx, clubID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("setup: commit: %w", err)
	}
	return s.Activate(ctx)
}
