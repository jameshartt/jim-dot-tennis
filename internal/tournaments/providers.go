// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournaments

// Several providers in one installation (WI-154). A provider is a separate
// set of tournaments on the tournament server: a club may have one for its
// own tournaments and another for a league cup. The app manages every
// provider it has taken on (a tournament_providers row with the tournament
// server's id), and each person is a member of the ones chosen for them.
//
// The tournament server lets a session act for one provider at a time, so a
// hand-over names the provider, and the person switches inside tournament
// management or comes back through the app for another.

import (
	"context"
	"sort"
	"strings"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/models"
)

// ManagedProvider is a provider the app has taken on.
type ManagedProvider struct {
	RowID       uint   // tournament_providers.id; 0 when the club's own provider has no row
	Name        string // for people
	Abbr        string // the provider's code on the tournament server
	CourthiveID string // the tournament server's id
	Own         bool   // the club's own provider: the one set up first
}

// ManagedProviders lists the providers the app manages: the club's own
// first, then the others by name.
func (s *Service) ManagedProviders(ctx context.Context) ([]ManagedProvider, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return s.managed(ctx, st)
}

func (s *Service) managed(ctx context.Context, st stored) ([]ManagedProvider, error) {
	rows, err := s.providers.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []ManagedProvider
	ownSeen := false
	for _, p := range rows {
		if !p.Managed() {
			continue
		}
		own := st.providerID != "" && p.CourthiveProviderID == st.providerID
		ownSeen = ownSeen || own
		out = append(out, ManagedProvider{RowID: p.ID, Name: p.Name, Abbr: strings.TrimSpace(p.ProviderAbbr), CourthiveID: p.CourthiveProviderID, Own: own})
	}
	if st.providerID != "" && !ownSeen {
		// Set up, but its row is gone or was never written: it is still the
		// club's provider.
		out = append(out, ManagedProvider{Name: st.abbreviation, Abbr: st.abbreviation, CourthiveID: st.providerID, Own: true})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Own != out[j].Own {
			return out[i].Own
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ChooseProviders is the one answer to "which providers does this account
// belong to":
//
//   - no tournament access (none, or a disabled account): none;
//   - an administrator: every managed provider;
//   - anyone else: the managed providers picked for them, and with none
//     picked the club's own provider only. So a club with one provider needs
//     no picking, and a provider added later is never given to anyone
//     without an administrator choosing it.
//
// picked holds tournament_providers ids. The result keeps managed's order.
func ChooseProviders(u models.User, managed []ManagedProvider, picked map[uint]bool) []ManagedProvider {
	if u.TournamentAccessLevel() == models.TournamentAccessNone {
		return nil
	}
	if u.Role == models.RoleAdmin {
		return append([]ManagedProvider(nil), managed...)
	}
	var out []ManagedProvider
	for _, p := range managed {
		if p.RowID != 0 && picked[p.RowID] {
			out = append(out, p)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, p := range managed {
		if p.Own {
			return []ManagedProvider{p}
		}
	}
	return nil
}

func (s *Service) picks(ctx context.Context, userID int64) (map[uint]bool, error) {
	var ids []uint
	if err := s.db.SelectContext(ctx, &ids, `SELECT tournament_provider_id FROM user_tournament_providers WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	out := make(map[uint]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// providersFor is ChooseProviders for u as stored now.
func (s *Service) providersFor(ctx context.Context, u models.User) ([]ManagedProvider, error) {
	managed, err := s.ManagedProviders(ctx)
	if err != nil {
		return nil, err
	}
	picked, err := s.picks(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return ChooseProviders(u, managed, picked), nil
}

// ProvidersFor lists the providers userID belongs to (ChooseProviders): what
// the tournament home page offers them.
func (s *Service) ProvidersFor(ctx context.Context, userID int64) ([]ManagedProvider, error) {
	u, err := s.user(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.providersFor(ctx, u)
}

// Choices is what the users page needs to show and change people's
// providers.
type Choices struct {
	Managed []ManagedProvider
	// Picked is what is stored for each account: tournament_providers ids.
	// An account with nothing stored has the club's own provider.
	Picked map[int64]map[uint]bool
}

// Choices reads the managed providers and everyone's stored picks.
func (s *Service) Choices(ctx context.Context) (Choices, error) {
	managed, err := s.ManagedProviders(ctx)
	if err != nil {
		return Choices{}, err
	}
	var rows []struct {
		UserID     int64 `db:"user_id"`
		ProviderID uint  `db:"tournament_provider_id"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT user_id, tournament_provider_id FROM user_tournament_providers`); err != nil {
		return Choices{}, err
	}
	c := Choices{Managed: managed, Picked: map[int64]map[uint]bool{}}
	for _, r := range rows {
		if c.Picked[r.UserID] == nil {
			c.Picked[r.UserID] = map[uint]bool{}
		}
		c.Picked[r.UserID][r.ProviderID] = true
	}
	return c, nil
}

// FoundProvider is a provider on the tournament server that the app has not
// taken on.
type FoundProvider struct {
	Abbr string
	Name string
}

// NotTakenOn lists the providers on the tournament server that the app does
// not manage (made by hand, or by an earlier installation), by code. It asks
// the tournament server.
func (s *Service) NotTakenOn(ctx context.Context) ([]FoundProvider, error) {
	if !s.Configured() {
		return nil, ErrNotConfigured
	}
	managed, err := s.ManagedProviders(ctx)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, p := range managed {
		have[p.CourthiveID] = true
	}
	all, err := s.client.Providers(ctx)
	if err != nil {
		return nil, err
	}
	var out []FoundProvider
	for _, p := range all {
		if !have[p.ID] && strings.TrimSpace(p.Abbreviation) != "" {
			out = append(out, FoundProvider{Abbr: p.Abbreviation, Name: p.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Abbr < out[j].Abbr })
	return out, nil
}

// AddProvider takes on a further provider beside the club's own: it makes
// the provider on the tournament server, or takes on one that is already
// there with that code (with its tournaments), adds the permanent
// administrator to it and registers it in the app. Nobody is given it: an
// administrator picks who uses it (administrators themselves always have
// every provider). It needs the club to be set up first (ErrNotSetUp). Safe
// to repeat. The outcome is stored and recorded like a set-up run.
func (s *Service) AddProvider(ctx context.Context, actor accounts.Actor, abbreviation, name string) (*Result, error) {
	if !s.Configured() {
		return nil, ErrNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if st.providerID == "" {
		return nil, ErrNotSetUp
	}
	abbr, err := NormalizeAbbreviation(abbreviation)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = abbr
	}
	return s.run(ctx, actor, st, abbr, name, strings.EqualFold(abbr, st.abbreviation))
}
