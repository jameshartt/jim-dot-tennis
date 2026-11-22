// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournaments

// People's tournament accounts (WI-152). jim-dot-tennis decides who may run
// tournaments (models.EffectiveTournamentAccess); this file keeps the bundled
// tournament server in line with that decision and hands people over to
// tournament management.
//
// Every change goes one way: from the account here to the tournament server.
// The tournament server reads a person's membership on each request, so
// removing it takes effect at once, including for a tournament session that
// already exists.
//
// A change that cannot be applied (the tournament server is down, say) is
// never dropped: the row is marked pending and Reconcile tries again. Until
// it succeeds the person may keep the access they had, which is why the
// users page shows pending rows and why Handover always brings the account
// into line first.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/models"
)

// ErrNotSetUp is returned when a person needs a tournament account and the
// club has not been set up on the tournament server yet.
var ErrNotSetUp = errors.New("tournaments: the club is not set up on the tournament server yet")

// ErrNoAccess is returned by Handover for an account with no tournament access.
var ErrNoAccess = errors.New("tournaments: this account has no tournament access")

// syncTimeout bounds one attempt to bring one account into line.
const syncTimeout = 20 * time.Second

// accountRow is a tournament_accounts row.
type accountRow struct {
	UserID          int64  `db:"user_id"`
	CourthiveUserID string `db:"courthive_user_id"`
	Email           string `db:"email"`
	AppliedRole     string `db:"applied_role"`
	Pending         bool   `db:"pending"`
	LastError       string `db:"last_error"`
}

// tournamentRole maps the access an account has here to the role it holds at
// the club's provider. "" means no membership.
func tournamentRole(access models.TournamentAccess) string {
	switch access {
	case models.TournamentAccessAll:
		return courthive.RoleProviderAdmin
	case models.TournamentAccessOwn:
		return courthive.RoleDirector
	}
	return ""
}

func (s *Service) user(ctx context.Context, userID int64) (models.User, error) {
	var u models.User
	err := s.db.GetContext(ctx, &u, `SELECT * FROM users WHERE id = ?`, userID)
	return u, err
}

func (s *Service) row(ctx context.Context, userID int64) (accountRow, bool, error) {
	var r accountRow
	err := s.db.GetContext(ctx, &r, `SELECT user_id, courthive_user_id, email, applied_role, pending, last_error
		FROM tournament_accounts WHERE user_id = ?`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

func (s *Service) saveRow(ctx context.Context, r accountRow) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tournament_accounts
			(user_id, courthive_user_id, email, applied_role, pending, last_error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET courthive_user_id = excluded.courthive_user_id, email = excluded.email,
			applied_role = excluded.applied_role, pending = excluded.pending, last_error = excluded.last_error,
			updated_at = excluded.updated_at`,
		r.UserID, r.CourthiveUserID, r.Email, r.AppliedRole, r.Pending, r.LastError, s.now().UTC())
	return err
}

// markPending records that the tournament server could not be brought into
// line for userID. It only applies to people the tournament server knows.
func (s *Service) markPending(ctx context.Context, userID int64, cause error) {
	_, err := s.db.ExecContext(ctx, `UPDATE tournament_accounts SET pending = 1, last_error = ?, updated_at = ? WHERE user_id = ?`,
		truncate(cause.Error(), 300), s.now().UTC(), userID)
	if err != nil {
		log.Printf("tournaments: could not record a pending change for account %d: %v", userID, err)
	}
}

// SyncAccount brings the tournament server into line with what userID may do
// now: it makes or restores their tournament account with the right role, or
// removes their membership. Safe to repeat. An account that has never had a
// tournament account and has no access needs nothing and touches nothing.
func (s *Service) SyncAccount(ctx context.Context, userID int64) error {
	u, err := s.user(ctx, userID)
	if err != nil {
		return fmt.Errorf("tournaments: reading account %d: %w", userID, err)
	}
	return s.apply(ctx, u, tournamentRole(u.TournamentAccessLevel()))
}

// apply sets u's membership to want ("" removes it).
func (s *Service) apply(ctx context.Context, u models.User, want string) error {
	r, known, err := s.row(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("tournaments: reading the tournament account of %d: %w", u.ID, err)
	}
	if want == "" && !known {
		return nil
	}
	if !s.Configured() {
		if known {
			s.markPending(ctx, u.ID, ErrNotConfigured)
		}
		return ErrNotConfigured
	}
	st, err := s.load(ctx)
	if err != nil {
		return err
	}
	if st.providerID == "" {
		if known {
			s.markPending(ctx, u.ID, ErrNotSetUp)
		}
		return ErrNotSetUp
	}

	if want == "" {
		if err := s.client.RemoveAccount(ctx, st.providerID, r.CourthiveUserID, r.Email); err != nil {
			s.markPending(ctx, u.ID, err)
			return err
		}
		r.AppliedRole, r.Pending, r.LastError = "", false, ""
		return s.saveRow(ctx, r)
	}

	email := r.Email
	if !known {
		email = courthive.AccountEmail(u.Username)
	}
	id, err := s.client.EnsureAccount(ctx, st.providerID, courthive.Account{
		ExternalID: strconv.FormatInt(u.ID, 10),
		Email:      email,
		Role:       want,
		UserID:     r.CourthiveUserID,
	})
	if err != nil {
		if known {
			s.markPending(ctx, u.ID, err)
		}
		return err
	}
	return s.saveRow(ctx, accountRow{UserID: u.ID, CourthiveUserID: id, Email: email, AppliedRole: want})
}

// Handover brings userID's tournament account into line and returns a
// single-use token that signs them in to tournament management. The token
// expires in a minute. It is refused for an account with no tournament access.
func (s *Service) Handover(ctx context.Context, userID int64) (string, error) {
	u, err := s.user(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("tournaments: reading account %d: %w", userID, err)
	}
	want := tournamentRole(u.TournamentAccessLevel())
	if want == "" {
		// Make sure nothing is left behind, then refuse.
		if err := s.apply(ctx, u, ""); err != nil && !errors.Is(err, ErrNotConfigured) && !errors.Is(err, ErrNotSetUp) {
			log.Printf("tournaments: removing the tournament membership of account %d: %v", userID, err)
		}
		return "", ErrNoAccess
	}
	if err := s.apply(ctx, u, want); err != nil {
		return "", err
	}
	st, err := s.load(ctx)
	if err != nil {
		return "", err
	}
	return s.client.HandoverToken(ctx, st.providerID, strconv.FormatInt(u.ID, 10))
}

// TournamentAccessChanged implements accounts.TournamentAccessObserver: it is
// called after a committed change to an account's tournament access, role or
// active state, and after its sessions were ended.
//
// Ending someone's sessions here removes their tournament membership, so a
// tournament session they have open stops seeing the club's tournaments at
// once. The membership comes back the next time they open tournament
// management after signing in again.
func (s *Service) TournamentAccessChanged(_ context.Context, change accounts.TournamentAccessChange) {
	// The request that caused the change may already be finishing: use our
	// own deadline.
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	var err error
	if change.What == "sessions_revoked" {
		var u models.User
		if u, err = s.user(ctx, change.UserID); err == nil {
			err = s.apply(ctx, u, "")
		}
	} else {
		err = s.SyncAccount(ctx, change.UserID)
	}
	switch {
	case err == nil:
	case errors.Is(err, ErrNotConfigured), errors.Is(err, ErrNotSetUp):
		// Nothing to tell: there is no tournament server for this club yet.
		// A person who needs an account gets it at their first hand-over.
	default:
		log.Printf("tournaments: account %d (%s): the tournament server could not be updated, will try again: %v",
			change.UserID, change.What, err)
	}
}

// Reconcile retries every change the tournament server has not taken yet and
// returns how many are still outstanding.
func (s *Service) Reconcile(ctx context.Context) (outstanding int, err error) {
	if !s.Configured() {
		return 0, nil
	}
	var ids []int64
	if err := s.db.SelectContext(ctx, &ids, `SELECT user_id FROM tournament_accounts WHERE pending = 1 ORDER BY user_id`); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.SyncAccount(ctx, id); err != nil {
			outstanding++
		}
	}
	return outstanding, nil
}

// RunReconciler retries outstanding changes every interval until ctx ends.
func (s *Service) RunReconciler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 2*syncTimeout)
			if n, err := s.Reconcile(attempt); err != nil {
				log.Printf("tournaments: retrying outstanding account changes: %v", err)
			} else if n > 0 {
				log.Printf("tournaments: %d account change(s) still not applied on the tournament server", n)
			}
			cancel()
		}
	}
}

// AccountState is what the users page shows about a person's tournament account.
type AccountState struct {
	UserID  int64
	Pending bool   // the tournament server has not taken the latest change
	Problem string // why, in the client's words (no secrets)
}

// PendingAccounts lists the accounts whose latest change the tournament
// server has not taken.
func (s *Service) PendingAccounts(ctx context.Context) ([]AccountState, error) {
	var rows []accountRow
	if err := s.db.SelectContext(ctx, &rows, `SELECT user_id, courthive_user_id, email, applied_role, pending, last_error
		FROM tournament_accounts WHERE pending = 1 ORDER BY user_id`); err != nil {
		return nil, err
	}
	out := make([]AccountState, 0, len(rows))
	for _, r := range rows {
		out = append(out, AccountState{UserID: r.UserID, Pending: true, Problem: r.LastError})
	}
	return out, nil
}
