// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/models"
)

// Tournament access (WI-153).
//
// Every account has a stored tournament access: none, own (its own
// tournaments and those it has been given) or all (every tournament of the
// club). What an account can actually use is models.EffectiveTournamentAccess:
// administrators always all, disabled accounts none, anyone else the stored
// value.
//
// A "Tournaments only" account has the role value "player"
// (models.RoleTournamentsOnly), which reaches no league page, and own or all
// access. Such an account is never made without access, and its access cannot
// be taken away: to stop it using tournaments, disable it.

// Errors about tournament access. Their messages are written to be shown to
// the administrator who asked for the change.
var (
	// ErrInvalidTournamentAccess: the value is not none, own or all.
	ErrInvalidTournamentAccess = errors.New("tournament access must be none, own or all")
	// ErrTournamentsOnlyNeedsAccess: a Tournaments only account would be
	// left (or made) with no tournament access, so it could do nothing.
	ErrTournamentsOnlyNeedsAccess = errors.New("a Tournaments only account needs access to its own tournaments or to all of them. To stop it using tournaments, disable the account instead")
	// ErrAdminTournamentAccess: administrators always have access to every
	// tournament, so theirs cannot be set.
	ErrAdminTournamentAccess = errors.New("administrators always have access to every tournament, so there is nothing to change")
)

// TournamentsOnlyLabel is what pages and the command line call an account
// with no league access and tournament access.
const TournamentsOnlyLabel = "Tournaments only"

// AccountTypeTournaments is how a form or the command line asks for a
// Tournaments only account (in place of a role name).
const AccountTypeTournaments = "tournaments"

// ParseAccountType reads what kind of account to create: a role (admin or
// captain, as ParseRole) or "tournaments" for a Tournaments only account.
func ParseAccountType(s string) (models.Role, error) {
	if strings.EqualFold(strings.TrimSpace(s), AccountTypeTournaments) {
		return models.RoleTournamentsOnly, nil
	}
	return ParseRole(s)
}

// ParseTournamentAccess returns the access named s or
// ErrInvalidTournamentAccess.
func ParseTournamentAccess(s string) (models.TournamentAccess, error) {
	a, ok := models.ParseTournamentAccess(s)
	if !ok {
		return models.TournamentAccessNone, ErrInvalidTournamentAccess
	}
	return a, nil
}

// EffectiveTournamentAccess is the account's effective tournament access
// (models.EffectiveTournamentAccess).
func (a Account) EffectiveTournamentAccess() models.TournamentAccess {
	return models.EffectiveTournamentAccess(a.Role, a.IsActive, a.TournamentAccess)
}

// IsTournamentsOnly reports whether the account has no league role (the
// stored "player" value). With no tournament access it is an account left
// over from before WI-132, which reaches nothing.
func (a Account) IsTournamentsOnly() bool { return a.Role == models.RoleTournamentsOnly }

// RoleLabel is the account's role in words: admin, captain, Tournaments only,
// or "player (no access)" for an account left with the retired role and no
// tournament access.
func (a Account) RoleLabel() string {
	if a.IsTournamentsOnly() {
		if a.TournamentAccess == models.TournamentAccessNone {
			return "player (no access)"
		}
		return TournamentsOnlyLabel
	}
	return string(a.Role)
}

// checkNewAccountAccess checks the role and tournament access of an account
// about to be created.
func checkNewAccountAccess(role models.Role, access models.TournamentAccess) error {
	if _, ok := models.ParseTournamentAccess(string(access)); !ok || access == "" {
		return ErrInvalidTournamentAccess
	}
	if role == models.RoleTournamentsOnly {
		if access == models.TournamentAccessNone {
			return ErrTournamentsOnlyNeedsAccess
		}
		return nil
	}
	if _, err := ParseRole(string(role)); err != nil {
		return err
	}
	if role == models.RoleAdmin && access != models.TournamentAccessNone {
		return ErrAdminTournamentAccess
	}
	return nil
}

// NewAccount is what InviteAccount needs to make an account.
type NewAccount struct {
	Username string
	// Role is admin, captain or models.RoleTournamentsOnly.
	Role models.Role
	// PlayerID links the account to a player record; nil or "" for none.
	PlayerID *string
	// TournamentAccess is none, own or all; "" means none. A Tournaments
	// only account needs own or all; an administrator takes none (they
	// always have all).
	TournamentAccess models.TournamentAccess
	// TournamentProviders are the sets of tournaments picked for the account
	// (tournament_providers ids; see SetTournamentProviders). None means the
	// club's own only. Not for an administrator.
	TournamentProviders []uint
}

// InviteAccount makes a new active account with no password, and an invite
// link for it, in one transaction. Unlike Invite it can make a Tournaments
// only account and set tournament access.
func (s *Service) InviteAccount(ctx context.Context, actor Actor, n NewAccount) (Account, IssuedLink, error) {
	if n.TournamentAccess == "" {
		n.TournamentAccess = models.TournamentAccessNone
	}
	if err := checkNewAccountAccess(n.Role, n.TournamentAccess); err != nil {
		return Account{}, IssuedLink{}, err
	}
	var (
		account Account
		issued  IssuedLink
	)
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		if account, err = s.createAccount(ctx, tx, actor, n.Username, n.Role, n.PlayerID, n.TournamentAccess); err != nil {
			return err
		}
		if len(n.TournamentProviders) > 0 {
			if _, err := s.setTournamentProviders(ctx, tx, actor, account, n.TournamentProviders); err != nil {
				return err
			}
		}
		issued, err = s.issueLink(ctx, tx, actor, account, LinkInvite)
		return err
	})
	if err != nil {
		return Account{}, IssuedLink{}, err
	}
	if account.TournamentAccess != models.TournamentAccessNone {
		s.notifyTournamentAccess(ctx, account, ChangeCreated)
	}
	return account, issued, nil
}

// SetTournamentAccess sets account id's stored tournament access and records
// the change. It refuses an administrator (ErrAdminTournamentAccess: they
// always have all) and taking a Tournaments only account's access away
// (ErrTournamentsOnlyNeedsAccess). It returns the stored access the account
// had before.
func (s *Service) SetTournamentAccess(ctx context.Context, actor Actor, id int64, access models.TournamentAccess) (models.TournamentAccess, error) {
	if _, ok := models.ParseTournamentAccess(string(access)); !ok || access == "" {
		return models.TournamentAccessNone, ErrInvalidTournamentAccess
	}
	var before, after Account
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		// A write first, so this transaction holds the write lock before it
		// reads the account it checks.
		if _, err := tx.ExecContext(ctx, `UPDATE users SET id = id WHERE id = ?`, id); err != nil {
			return fmt.Errorf("change tournament access: %w", err)
		}
		var err error
		if before, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		switch {
		case before.Role == models.RoleAdmin:
			return ErrAdminTournamentAccess
		case before.IsTournamentsOnly() && access == models.TournamentAccessNone:
			return ErrTournamentsOnlyNeedsAccess
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET tournament_access = ? WHERE id = ?`, access, id); err != nil {
			return fmt.Errorf("change tournament access: %w", err)
		}
		if after, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		detail := fmt.Sprintf("tournament_access %s -> %s", before.TournamentAccess, access)
		if before.TournamentAccess == access {
			detail += " (no change)"
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionTournamentAccess,
			TargetUserID: id, TargetUsername: before.Username, Detail: detail})
	})
	if err != nil {
		return models.TournamentAccessNone, err
	}
	if before.TournamentAccess != after.TournamentAccess {
		s.notifyTournamentAccess(ctx, after, ChangeTournamentAccess)
	}
	return before.TournamentAccess, nil
}

// What changed, for a TournamentAccessObserver.
const (
	ChangeCreated          = "created"           // an account with tournament access was created
	ChangeTournamentAccess = "tournament_access" // its stored tournament access changed
	ChangeRole             = "role"              // its role changed
	ChangeActive           = "active"            // it was enabled or disabled
	ChangeSessionsRevoked  = "sessions_revoked"  // one or all of its sessions were ended
)

// TournamentAccessChange describes a committed change that can affect what
// an account may do in tournament management.
type TournamentAccessChange struct {
	UserID   int64
	Username string
	// What is one of the Change... constants.
	What string
	// Access is the account's effective tournament access after the change
	// (models.EffectiveTournamentAccess).
	Access models.TournamentAccess
}

// TournamentAccessObserver is told, after the change has been committed,
// whenever an account's tournament access, role or active state changes, an
// account with tournament access is created, or an account's sessions are
// ended by an administrator or the command line. The change has already
// happened: an observer cannot undo or refuse it, and should not take long.
// The tournament hand-over (WI-152) plugs in here; until then the service
// uses NoTournamentAccessObserver.
type TournamentAccessObserver interface {
	TournamentAccessChanged(ctx context.Context, change TournamentAccessChange)
}

// NoTournamentAccessObserver does nothing. It is the default.
type NoTournamentAccessObserver struct{}

// TournamentAccessChanged does nothing.
func (NoTournamentAccessObserver) TournamentAccessChanged(context.Context, TournamentAccessChange) {}

// SetTournamentAccessObserver sets who is told about tournament access
// changes; nil means nobody (NoTournamentAccessObserver).
func (s *Service) SetTournamentAccessObserver(o TournamentAccessObserver) {
	if o == nil {
		o = NoTournamentAccessObserver{}
	}
	s.observer = o
}

// notifyTournamentAccess tells the observer about a committed change to a.
func (s *Service) notifyTournamentAccess(ctx context.Context, a Account, what string) {
	s.observer.TournamentAccessChanged(ctx, TournamentAccessChange{
		UserID: a.ID, Username: a.Username, What: what, Access: a.EffectiveTournamentAccess(),
	})
}
