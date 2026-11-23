// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/models"
)

// The sets of tournaments chosen for an account (WI-154).
//
// An installation can manage several providers on the tournament server (a
// provider is a separate set of tournaments: one for the club, one for a
// league cup). user_tournament_providers holds the ones picked for an
// account. The rule that reads it is tournaments.ChooseProviders: an
// administrator has every one whatever is stored; anyone else with
// tournament access has the ones picked, and with none picked the club's own
// only.

// Errors about an account's sets of tournaments, worded for the
// administrator who asked for the change.
var (
	// ErrUnknownTournamentProvider: a named set of tournaments is not one
	// the app manages.
	ErrUnknownTournamentProvider = errors.New("that set of tournaments is not one this app manages. Add it on the tournaments page first")
	// ErrAdminTournamentProviders: administrators always have every set.
	ErrAdminTournamentProviders = errors.New("administrators always have every set of tournaments, so there is nothing to choose")
)

// ChangeTournamentProviders tells a TournamentAccessObserver that the sets of
// tournaments chosen for an account changed.
const ChangeTournamentProviders = "tournament_providers"

// managedProviderCodes returns the code of each managed provider row by id.
func managedProviderCodes(ctx context.Context, q sqlx.QueryerContext) (map[uint]string, error) {
	var rows []struct {
		ID   uint   `db:"id"`
		Abbr string `db:"provider_abbr"`
	}
	if err := sqlx.SelectContext(ctx, q, &rows, `SELECT id, provider_abbr FROM tournament_providers WHERE courthive_provider_id <> ''`); err != nil {
		return nil, fmt.Errorf("read the sets of tournaments: %w", err)
	}
	out := make(map[uint]string, len(rows))
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Abbr)
	}
	return out, nil
}

// TournamentProviderIDs turns provider codes (case ignored) into the ids of
// the managed providers they name, for the command line.
// ErrUnknownTournamentProvider for a code that names none.
func (s *Service) TournamentProviderIDs(ctx context.Context, codes []string) ([]uint, error) {
	managed, err := managedProviderCodes(ctx, s.db)
	if err != nil {
		return nil, err
	}
	var out []uint
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		found := false
		for id, abbr := range managed {
			if strings.EqualFold(abbr, code) {
				out, found = append(out, id), true
			}
		}
		if !found {
			return nil, fmt.Errorf("%q: %w", code, ErrUnknownTournamentProvider)
		}
	}
	return out, nil
}

// codeList is the codes of ids, sorted, for the audit log.
func codeList(managed map[uint]string, ids []uint) string {
	if len(ids) == 0 {
		return "(club's own)"
	}
	var codes []string
	for _, id := range ids {
		codes = append(codes, managed[id])
	}
	sort.Strings(codes)
	return strings.Join(codes, ",")
}

// setTournamentProviders stores ids as the account's picks inside tx and
// records the change. It returns whether anything changed.
func (s *Service) setTournamentProviders(ctx context.Context, tx *sqlx.Tx, actor Actor, a Account, ids []uint) (bool, error) {
	if a.Role == models.RoleAdmin {
		return false, ErrAdminTournamentProviders
	}
	managed, err := managedProviderCodes(ctx, tx)
	if err != nil {
		return false, err
	}
	want := map[uint]bool{}
	for _, id := range ids {
		if _, ok := managed[id]; !ok {
			return false, ErrUnknownTournamentProvider
		}
		want[id] = true
	}
	var before []uint
	if err := tx.SelectContext(ctx, &before, `SELECT tournament_provider_id FROM user_tournament_providers WHERE user_id = ? ORDER BY tournament_provider_id`, a.ID); err != nil {
		return false, fmt.Errorf("read the account's sets of tournaments: %w", err)
	}
	after := make([]uint, 0, len(want))
	for id := range want {
		after = append(after, id)
	}
	sort.Slice(after, func(i, j int) bool { return after[i] < after[j] })
	changed := len(before) != len(after)
	for i := 0; !changed && i < len(after); i++ {
		changed = before[i] != after[i]
	}
	if changed {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_tournament_providers WHERE user_id = ?`, a.ID); err != nil {
			return false, fmt.Errorf("change the account's sets of tournaments: %w", err)
		}
		for _, id := range after {
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_tournament_providers (user_id, tournament_provider_id) VALUES (?, ?)`, a.ID, id); err != nil {
				return false, fmt.Errorf("change the account's sets of tournaments: %w", err)
			}
		}
	}
	// Codes of rows that are no longer managed read as "" in the old list;
	// say what is known.
	detail := fmt.Sprintf("tournament_providers %s -> %s", codeList(managed, before), codeList(managed, after))
	if !changed {
		detail += " (no change)"
	}
	return changed, recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionTournamentProviders,
		TargetUserID: a.ID, TargetUsername: a.Username, Detail: detail})
}

// SetTournamentProviders stores the sets of tournaments picked for account
// id (tournament_providers ids; none means the club's own only) and records
// the change. It refuses an administrator (ErrAdminTournamentProviders) and
// an id that is not a managed provider (ErrUnknownTournamentProvider).
func (s *Service) SetTournamentProviders(ctx context.Context, actor Actor, id int64, providerIDs []uint) error {
	var (
		account Account
		changed bool
	)
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		// A write first, so this transaction holds the write lock before it
		// reads the account it checks.
		if _, err := tx.ExecContext(ctx, `UPDATE users SET id = id WHERE id = ?`, id); err != nil {
			return fmt.Errorf("change the account's sets of tournaments: %w", err)
		}
		var err error
		if account, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		changed, err = s.setTournamentProviders(ctx, tx, actor, account, providerIDs)
		return err
	})
	if err != nil {
		return err
	}
	if changed {
		s.notifyTournamentAccess(ctx, account, ChangeTournamentProviders)
	}
	return nil
}
