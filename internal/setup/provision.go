// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/services"
)

// ClubChoice is the BHPLTA club picked in the "choose your club" step: one of
// services.KnownClubSlugs.
type ClubChoice = services.ClubSlugMapping

// CompleteFunc writes the setup completion settings for clubID using tx. The
// provisioner must call it exactly once, inside the same transaction that
// creates (or finds) the home club, before committing. If it returns an error
// the provisioner must roll the transaction back and return that error.
type CompleteFunc func(ctx context.Context, tx *sqlx.Tx, clubID uint) error

// ProvisionResult is what the club step reports back to the person setting up.
type ProvisionResult struct {
	ClubID      uint
	ClubName    string
	CreatedClub bool // false when an existing clubs row was reused
	// Import is what the BHPLTA bootstrap created (nil for the interim
	// provisioner).
	Import *bhplta.Summary
}

// ProvisionError is a provisioning failure with a message for the person
// setting up (naming what failed). Nothing was written.
type ProvisionError struct {
	Message string
	Err     error
}

func (e *ProvisionError) Error() string { return e.Message + ": " + e.Err.Error() }
func (e *ProvisionError) Unwrap() error { return e.Err }

// ProgressFunc receives progress messages from a slow provisioner.
type ProgressFunc func(step, total int, what string)

type progressKey struct{}

// WithProgress returns ctx carrying fn, for the provisioner to report how far
// it has got.
func WithProgress(ctx context.Context, fn ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

func progressFrom(ctx context.Context) ProgressFunc {
	if fn, ok := ctx.Value(progressKey{}).(ProgressFunc); ok && fn != nil {
		return fn
	}
	return func(int, int, string) {}
}

// HomeClubProvisioner is the seam between the "choose your club" step and
// whatever builds the instance's data for the chosen club.
//
// ProvisionHomeClub must leave the database either fully provisioned with
// completion recorded (complete called inside its transaction, then
// committed) or exactly as it was. It may do slow work such as fetching from
// the BHPLTA site before opening its transaction; it should not hold the
// transaction open across network calls. Calls are serialised by the setup
// service. The setup service activates the home club after it returns.
//
// NewBHPLTAProvisioner (the one the server uses) imports the league,
// divisions, clubs, teams, weeks and fixtures from BHPLTA. The default for
// NewService(state, nil), NewClubRowProvisioner, only creates or finds the
// clubs row.
type HomeClubProvisioner interface {
	ProvisionHomeClub(ctx context.Context, choice ClubChoice, complete CompleteFunc) (*ProvisionResult, error)
}

// NewClubRowProvisioner returns the interim provisioner: it finds the chosen
// club's row (by exact BHPLTA display name, then by a loose name comparison)
// or creates it, and records it as the home club, in one transaction.
func NewClubRowProvisioner(db *database.DB) HomeClubProvisioner {
	return &clubRowProvisioner{db: db}
}

type clubRowProvisioner struct {
	db *database.DB
}

func (p *clubRowProvisioner) ProvisionHomeClub(ctx context.Context, choice ClubChoice, complete CompleteFunc) (*ProvisionResult, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("setup: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := findOrCreateClub(ctx, tx, choice.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := complete(ctx, tx, res.ClubID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("setup: commit: %w", err)
	}
	return res, nil
}

// findOrCreateClub returns the clubs row for name, creating it if no existing
// row matches.
func findOrCreateClub(ctx context.Context, tx *sqlx.Tx, name string) (*ProvisionResult, error) {
	var clubs []struct {
		ID   uint   `db:"id"`
		Name string `db:"name"`
	}
	if err := tx.SelectContext(ctx, &clubs, `SELECT id, name FROM clubs ORDER BY id`); err != nil {
		return nil, fmt.Errorf("setup: list clubs: %w", err)
	}
	for _, c := range clubs {
		if c.Name == name {
			return &ProvisionResult{ClubID: c.ID, ClubName: c.Name}, nil
		}
	}
	want := looseClubName(name)
	for _, c := range clubs {
		if want != "" && looseClubName(c.Name) == want {
			return &ProvisionResult{ClubID: c.ID, ClubName: c.Name}, nil
		}
	}

	now := time.Now()
	result, err := tx.ExecContext(ctx, `INSERT INTO clubs (name, created_at, updated_at) VALUES (?, ?, ?)`, name, now, now)
	if err != nil {
		return nil, fmt.Errorf("setup: create club %q: %w", name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("setup: create club %q: %w", name, err)
	}
	return &ProvisionResult{ClubID: uint(id), ClubName: name, CreatedClub: true}, nil
}

// looseClubName reduces a club name to its distinguishing words, so that
// "St Ann's Tennis" and "St Anns" compare equal.
func looseClubName(name string) string {
	name = strings.ToLower(normalize.Apostrophes(name))
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return -1
	}, name)
	var words []string
	for _, w := range strings.Fields(name) {
		switch w {
		case "tennis", "club", "lawn", "the", "tc", "ltc":
			continue
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}
