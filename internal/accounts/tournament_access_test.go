// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// recordingObserver keeps every change it is told about.
type recordingObserver struct{ got []string }

func (o *recordingObserver) TournamentAccessChanged(_ context.Context, c TournamentAccessChange) {
	o.got = append(o.got, fmt.Sprintf("%s %s %s", c.Username, c.What, c.Access))
}

func (o *recordingObserver) take() string {
	s := strings.Join(o.got, "; ")
	o.got = nil
	return s
}

// auditRows returns "action detail" for every audit row about username.
func accessAuditRows(t *testing.T, s *Service, username, action string) []string {
	t.Helper()
	var rows []string
	if err := s.db.Select(&rows, `SELECT actor_kind || ' ' || detail FROM account_audit_log
		WHERE target_username = ? AND action = ? ORDER BY id`, username, action); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestCreateTournamentsOnlyAccount(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	obs := &recordingObserver{}
	s.SetTournamentAccessObserver(obs)

	a, link, err := s.InviteAccount(ctx, cli, NewAccount{Username: "tina", Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessOwn})
	if err != nil {
		t.Fatal(err)
	}
	if a.Role != models.RoleTournamentsOnly || a.TournamentAccess != models.TournamentAccessOwn ||
		a.EffectiveTournamentAccess() != models.TournamentAccessOwn || !a.IsTournamentsOnly() || a.RoleLabel() != TournamentsOnlyLabel {
		t.Fatalf("tina: %+v (%s)", a, a.RoleLabel())
	}
	if link.Kind != LinkInvite || link.Username != "tina" {
		t.Fatalf("invite: %+v", link)
	}
	if got := accessAuditRows(t, s, "tina", ActionCreate); len(got) != 1 || got[0] != "cli role player tournament_access=own" {
		t.Errorf("create audit: %q", got)
	}
	if got := obs.take(); got != "tina created own" {
		t.Errorf("observer: %q", got)
	}
	// A Tournaments only account signs in like anyone else: its invite works.
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatalf("tina's invite: %v", err)
	}

	// A captain may be given access when created; nobody is told about a
	// captain with none.
	c, _, err := s.InviteAccount(ctx, cli, NewAccount{Username: "cara", Role: models.RoleCaptain, TournamentAccess: models.TournamentAccessAll})
	if err != nil || c.EffectiveTournamentAccess() != models.TournamentAccessAll {
		t.Fatalf("cara: %+v %v", c, err)
	}
	if _, _, err := s.InviteAccount(ctx, cli, NewAccount{Username: "cliff", Role: models.RoleCaptain}); err != nil {
		t.Fatal(err)
	}
	if got := obs.take(); got != "cara created all" {
		t.Errorf("observer: %q", got)
	}
	// Administrators always have all, whatever is stored.
	ad, _, err := s.InviteAccount(ctx, cli, NewAccount{Username: "ada", Role: models.RoleAdmin})
	if err != nil || ad.TournamentAccess != models.TournamentAccessNone || ad.EffectiveTournamentAccess() != models.TournamentAccessAll {
		t.Fatalf("ada: %+v %v", ad, err)
	}

	// Refused: nothing is created and nothing recorded.
	for name, n := range map[string]NewAccount{
		"tournaments only with no access": {Username: "nobody1", Role: models.RoleTournamentsOnly},
		"tournaments only, access none":   {Username: "nobody2", Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessNone},
		"administrator with access":       {Username: "nobody3", Role: models.RoleAdmin, TournamentAccess: models.TournamentAccessAll},
		"unknown access":                  {Username: "nobody4", Role: models.RoleCaptain, TournamentAccess: "some"},
		"unknown role":                    {Username: "nobody5", Role: "owner", TournamentAccess: models.TournamentAccessOwn},
	} {
		_, _, err := s.InviteAccount(ctx, cli, n)
		var want error
		switch name {
		case "administrator with access":
			want = ErrAdminTournamentAccess
		case "unknown access":
			want = ErrInvalidTournamentAccess
		case "unknown role":
			want = ErrInvalidRole
		default:
			want = ErrTournamentsOnlyNeedsAccess
		}
		if !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
		if _, err := s.Find(ctx, n.Username); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: the account was created", name)
		}
	}
	// Invite (admin and captain only) still refuses the player role.
	if _, _, err := s.Invite(ctx, cli, "pat", models.RoleTournamentsOnly, nil); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("Invite as tournaments only: %v", err)
	}
	if got := obs.take(); got != "" {
		t.Errorf("observer told about refused changes: %q", got)
	}
}

func TestSetTournamentAccess(t *testing.T) {
	s, db := newTestService(t)
	ctx := context.Background()
	obs := &recordingObserver{}
	s.SetTournamentAccessObserver(obs)
	admin := mustCreate(t, s, "ada", models.RoleAdmin)
	captain := mustCreate(t, s, "cap", models.RoleCaptain)
	tina, _, err := s.InviteAccount(ctx, cli, NewAccount{Username: "tina", Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessAll})
	if err != nil {
		t.Fatal(err)
	}
	obs.take()
	web := UserActor(admin.ID, admin.Username)

	// A captain: none -> own -> all -> none, each recorded.
	for _, step := range []struct{ from, to models.TournamentAccess }{
		{models.TournamentAccessNone, models.TournamentAccessOwn},
		{models.TournamentAccessOwn, models.TournamentAccessAll},
		{models.TournamentAccessAll, models.TournamentAccessNone},
	} {
		prev, err := s.SetTournamentAccess(ctx, web, captain.ID, step.to)
		if err != nil || prev != step.from {
			t.Fatalf("captain %s -> %s: previous %q, %v", step.from, step.to, prev, err)
		}
		if a, _ := s.Get(ctx, captain.ID); a.TournamentAccess != step.to {
			t.Fatalf("captain stored %q, want %q", a.TournamentAccess, step.to)
		}
	}
	want := []string{"user tournament_access none -> own", "user tournament_access own -> all", "user tournament_access all -> none"}
	if got := accessAuditRows(t, s, "cap", ActionTournamentAccess); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("captain audit rows: %q", got)
	}
	if got := obs.take(); got != "cap tournament_access own; cap tournament_access all; cap tournament_access none" {
		t.Errorf("observer: %q", got)
	}
	// Setting the same value again changes nothing and tells nobody.
	if _, err := s.SetTournamentAccess(ctx, cli, captain.ID, models.TournamentAccessNone); err != nil {
		t.Fatal(err)
	}
	if got := obs.take(); got != "" {
		t.Errorf("observer told about no change: %q", got)
	}

	// A Tournaments only account: own <-> all, never none.
	if prev, err := s.SetTournamentAccess(ctx, cli, tina.ID, models.TournamentAccessOwn); err != nil || prev != models.TournamentAccessAll {
		t.Fatalf("tina all -> own: %q %v", prev, err)
	}
	if _, err := s.SetTournamentAccess(ctx, cli, tina.ID, models.TournamentAccessNone); !errors.Is(err, ErrTournamentsOnlyNeedsAccess) {
		t.Errorf("tina -> none: %v", err)
	}
	if !strings.Contains(ErrTournamentsOnlyNeedsAccess.Error(), "disable the account") {
		t.Error("the refusal does not say to disable the account instead")
	}
	if a, _ := s.Get(ctx, tina.ID); a.TournamentAccess != models.TournamentAccessOwn {
		t.Errorf("tina after the refusal: %q", a.TournamentAccess)
	}

	// Administrators: refused, they always have all.
	if _, err := s.SetTournamentAccess(ctx, cli, admin.ID, models.TournamentAccessOwn); !errors.Is(err, ErrAdminTournamentAccess) {
		t.Errorf("admin: %v", err)
	}
	// Unknown value and unknown account.
	if _, err := s.SetTournamentAccess(ctx, cli, captain.ID, "some"); !errors.Is(err, ErrInvalidTournamentAccess) {
		t.Errorf("bogus value: %v", err)
	}
	if _, err := s.SetTournamentAccess(ctx, cli, 99999, models.TournamentAccessOwn); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: %v", err)
	}
	if got := accessAuditRows(t, s, "tina", ActionTournamentAccess); len(got) != 1 {
		t.Errorf("refused changes were recorded: %q", got)
	}
	if got := accessAuditRows(t, s, "ada", ActionTournamentAccess); len(got) != 0 {
		t.Errorf("refused admin change was recorded: %q", got)
	}
	if got := obs.take(); got != "tina tournament_access own" {
		t.Errorf("observer: %q", got)
	}

	// The database refuses a value outside none, own and all.
	if _, err := db.Exec(`UPDATE users SET tournament_access = 'some' WHERE id = ?`, captain.ID); err == nil {
		t.Error("the database accepted tournament_access 'some'")
	}
}

// The observer hears about role, active-state and session changes too, and
// a disabled account has no tournament access.
func TestTournamentAccessObserverSeesRoleActiveAndSessions(t *testing.T) {
	s, db := newTestService(t)
	ctx := context.Background()
	obs := &recordingObserver{}
	mustCreate(t, s, "ada", models.RoleAdmin)
	captain := mustCreate(t, s, "cap", models.RoleCaptain)
	tina, link, err := s.InviteAccount(ctx, cli, NewAccount{Username: "tina", Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessOwn})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	s.SetTournamentAccessObserver(obs)

	if _, err := s.SetActive(ctx, cli, tina.ID, false); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Get(ctx, tina.ID); a.EffectiveTournamentAccess() != models.TournamentAccessNone || a.TournamentAccess != models.TournamentAccessOwn {
		t.Errorf("disabled tina: stored %q, effective %q", a.TournamentAccess, a.EffectiveTournamentAccess())
	}
	if _, err := s.SetActive(ctx, cli, tina.ID, true); err != nil {
		t.Fatal(err)
	}
	// A Tournaments only account can be made a captain, and keeps its access.
	if _, err := s.SetRole(ctx, cli, tina.ID, models.RoleCaptain); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRole(ctx, cli, captain.ID, models.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RevokeSessions(ctx, cli, tina.ID); err != nil {
		t.Fatal(err)
	}
	var sessionID int64
	if err := db.Get(&sessionID, `INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info)
		VALUES ('h', ?, 'captain', '2099-01-01 00:00:00', '', '', '') RETURNING id`, captain.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeSession(ctx, cli, sessionID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeSession(ctx, cli, sessionID+100); err != nil { // no such session: nothing to tell
		t.Fatal(err)
	}
	// Refused changes tell nobody.
	if _, err := s.SetRole(ctx, cli, tina.ID, models.RoleTournamentsOnly); err == nil {
		t.Error("made an account Tournaments only through SetRole")
	}
	want := "tina active none; tina active own; tina role own; cap role all; tina sessions_revoked own; cap sessions_revoked all"
	if got := obs.take(); got != want {
		t.Errorf("observer:\n got %q\nwant %q", got, want)
	}
}

func TestParseAccountType(t *testing.T) {
	for in, want := range map[string]models.Role{"admin": models.RoleAdmin, "Captain": models.RoleCaptain, "tournaments": models.RoleTournamentsOnly, " TOURNAMENTS ": models.RoleTournamentsOnly} {
		if got, err := ParseAccountType(in); err != nil || got != want {
			t.Errorf("ParseAccountType(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"player", "", "owner"} {
		if _, err := ParseAccountType(in); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("ParseAccountType(%q): %v", in, err)
		}
	}
}

// Using an invite link signs a Tournaments only account in and sends it to
// the tournament home page; a captain still goes to the league.
func TestLinkPageLandsTournamentsOnlyAccountOnItsHome(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	_, tinaLink, err := env.s.InviteAccount(ctx, cli, NewAccount{Username: "tina", Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessAll})
	if err != nil {
		t.Fatal(err)
	}
	_, capLink, err := env.s.InviteAccount(ctx, cli, NewAccount{Username: "cap", Role: models.RoleCaptain})
	if err != nil {
		t.Fatal(err)
	}
	for link, want := range map[string]string{tinaLink.Path(): auth.TournamentHomePath, capLink.Path(): "/admin/league"} {
		rec := env.do(http.MethodPost, link, "198.51.100.4", passwordForm("a brand new secret"))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
			t.Errorf("redeem: %d %q, want %q", rec.Code, rec.Header().Get("Location"), want)
		}
	}
}
