// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package bhplta

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/services"
	"jim-dot-tennis/internal/version"
)

// Fetcher reads a Snapshot from the BHPLTA site (*Source implements it).
type Fetcher interface {
	Fetch(ctx context.Context, progress Progress) (*Snapshot, error)
}

// Bootstrapper fetches a season from BHPLTA and imports it in one
// transaction.
type Bootstrapper struct {
	DB     *database.DB
	Source Fetcher
}

// NewBootstrapper returns a Bootstrapper reading from NewSource().
func NewBootstrapper(db *database.DB) *Bootstrapper {
	return &Bootstrapper{DB: db, Source: NewSource()}
}

// BeforeCommit runs inside the import transaction after the import has
// written everything; an error rolls the whole import back.
type BeforeCommit func(ctx context.Context, tx *sqlx.Tx, sum *Summary) error

// Run fetches from the site (before opening any transaction), then imports
// the snapshot, calls beforeCommit (if not nil) and commits. Any error leaves
// the database as it was; Describe turns it into a message for people.
func (b *Bootstrapper) Run(ctx context.Context, opts Options, progress Progress, beforeCommit BeforeCommit) (*Summary, error) {
	snap, err := b.Source.Fetch(ctx, progress)
	if err != nil {
		return nil, err
	}
	return b.Apply(ctx, snap, opts, beforeCommit)
}

// Apply imports an already-fetched snapshot in one transaction (see Run).
func (b *Bootstrapper) Apply(ctx context.Context, snap *Snapshot, opts Options, beforeCommit BeforeCommit) (*Summary, error) {
	tx, err := b.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	sum, err := Import(ctx, tx, snap, opts)
	if err != nil {
		return nil, err
	}
	if beforeCommit != nil {
		if err := beforeCommit(ctx, tx, sum); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import: %w", err)
	}
	return sum, nil
}

// Describe explains err from Run, Apply or Fetch in words for the person
// running the import, naming what failed. It does not say whether anything
// was written: callers know (nothing is, on error).
func Describe(err error) string {
	if err == nil {
		return ""
	}
	var exists *SeasonExistsError
	if errors.As(err, &exists) {
		return fmt.Sprintf("%s already has divisions, so it was not imported again. Open it from the Seasons page to change it.", exists.SeasonName)
	}

	var fe *FetchError
	if !errors.As(err, &fe) {
		// Not a fetch problem: something went wrong writing the data.
		return "The data from BHPLTA could not be saved (" + err.Error() + ")."
	}
	what := fe.What

	var status *services.HTTPStatusError
	switch services.ClassifyFetchError(err) {
	case services.FetchCancelled:
		return fmt.Sprintf("Reading %s was cancelled.", what)
	case services.FetchTimeout:
		return fmt.Sprintf("Reading %s took too long. The BHPLTA website may be slow or unreachable from this server.", what)
	case services.FetchRefused:
		return fmt.Sprintf("The BHPLTA website refused the request for %s (%s). This server reached the website, so its internet connection is working. Wait a few minutes and try again. If it keeps happening, report it at %s/issues.", what, services.FetchRefusedDetail(err), version.ProjectURL)
	case services.FetchNoConnection:
		return fmt.Sprintf("Could not reach %s (%v). This server could not connect to the BHPLTA website at all: check that it can connect to the internet and look up web addresses.", what, innermost(err))
	}
	switch {
	case errors.As(err, &status):
		return fmt.Sprintf("Reading %s failed: the BHPLTA website answered with HTTP status %d.", what, status.Status)
	case errors.Is(err, services.ErrUnexpectedMarkup):
		return fmt.Sprintf("%s did not look as expected (%v). The BHPLTA website may have changed; this app may need an update to read it.", capitalise(what), errors.Unwrap(err))
	default:
		return fmt.Sprintf("Reading %s failed: %v.", what, innermost(err))
	}
}

func innermost(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-'a'+'A') + s[1:]
	}
	return s
}
