// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournaments

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func codes(list []ManagedProvider) string {
	var out []string
	for _, p := range list {
		out = append(out, p.Abbr)
	}
	return strings.Join(out, ",")
}

func TestChooseProviders(t *testing.T) {
	own := ManagedProvider{RowID: 1, Name: "Club", Abbr: "CLUBA", CourthiveID: "p1", Own: true}
	cup := ManagedProvider{RowID: 2, Name: "Cup", Abbr: "CUP", CourthiveID: "p2"}
	managed := []ManagedProvider{own, cup}
	user := func(role models.Role, access models.TournamentAccess, active bool) models.User {
		return models.User{ID: 9, Role: role, TournamentAccess: access, IsActive: active}
	}
	for name, c := range map[string]struct {
		u      models.User
		picked map[uint]bool
		want   string
	}{
		"administrator: every one, whatever is stored":  {user(models.RoleAdmin, models.TournamentAccessNone, true), map[uint]bool{2: true}, "CLUBA,CUP"},
		"disabled administrator: none":                  {user(models.RoleAdmin, models.TournamentAccessNone, false), nil, ""},
		"no access: none, whatever is picked":           {user(models.RoleCaptain, models.TournamentAccessNone, true), map[uint]bool{1: true, 2: true}, ""},
		"nothing picked: the club's own only":           {user(models.RoleTournamentsOnly, models.TournamentAccessOwn, true), nil, "CLUBA"},
		"picked the cup only":                           {user(models.RoleTournamentsOnly, models.TournamentAccessAll, true), map[uint]bool{2: true}, "CUP"},
		"picked both":                                   {user(models.RoleCaptain, models.TournamentAccessOwn, true), map[uint]bool{1: true, 2: true}, "CLUBA,CUP"},
		"a pick that is not managed counts for nothing": {user(models.RoleCaptain, models.TournamentAccessOwn, true), map[uint]bool{7: true}, "CLUBA"},
		"disabled: none":                                {user(models.RoleCaptain, models.TournamentAccessAll, false), map[uint]bool{2: true}, ""},
	} {
		if got := codes(ChooseProviders(c.u, managed, c.picked)); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
	// A provider added later is never given to someone with nothing picked.
	later := append(managed, ManagedProvider{RowID: 3, Abbr: "NEW", CourthiveID: "p3"})
	if got := codes(ChooseProviders(user(models.RoleCaptain, models.TournamentAccessAll, true), later, nil)); got != "CLUBA" {
		t.Errorf("after another provider is added: %q", got)
	}
}

// pick stores the picks of a person as the accounts service would.
func pick(t *testing.T, e env, userID int64, abbrs ...string) {
	t.Helper()
	if _, err := e.db.Exec(`DELETE FROM user_tournament_providers WHERE user_id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	for _, a := range abbrs {
		if _, err := e.db.Exec(`INSERT INTO user_tournament_providers (user_id, tournament_provider_id)
			SELECT ?, id FROM tournament_providers WHERE provider_abbr = ?`, userID, a); err != nil {
			t.Fatal(err)
		}
	}
}

// memberships is the person's role in each provider on the fake, as
// "CLUBA=ROLE,CUP=ROLE".
func memberships(e env, id int64) string {
	u, ok := tournamentUser(e, id)
	if !ok {
		return "(no account)"
	}
	var out []string
	for _, p := range []string{"CLUBA", "CUP"} {
		if role, in := u.Membership["provider-"+p]; in {
			out = append(out, p+"="+role)
		}
	}
	return strings.Join(out, ",")
}

func TestAddProviderMakesOneOrTakesOneOn(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.AddProvider(ctx, actor, "CUP", "Parks Cup"); !errors.Is(err, ErrNotSetUp) {
		t.Fatalf("before the club is set up: %v, want ErrNotSetUp", err)
	}
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "CLUBA", Name: "Club A"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.svc.ManagedProviders(ctx); codes(got) != "CLUBA" || !got[0].Own || got[0].CourthiveID != "provider-CLUBA" {
		t.Fatalf("after set-up: %+v", got)
	}

	// Made by hand before the app, with someone in it: listed as not taken
	// on, then taken on with its people left alone.
	old := e.fake.AddProvider("CUP", "Old cup name", false)
	e.fake.AddAccount(old, "jim@example.org", courthive.RoleProviderAdmin, "", "")
	found, err := e.svc.NotTakenOn(ctx)
	if err != nil || len(found) != 1 || found[0].Abbr != "CUP" {
		t.Fatalf("NotTakenOn = %+v, %v", found, err)
	}
	res, err := e.svc.AddProvider(ctx, actor, " cup ", "Parks Cup")
	if err != nil || !res.OK || !res.Further {
		t.Fatalf("AddProvider: %+v, %v", res, err)
	}
	if o := outcomes(res); o[stepProvider] != OutcomeMade || o[stepInstance] != OutcomeMade || o[stepRegister] != OutcomeMade {
		t.Fatalf("steps = %+v", res.Steps)
	}
	if len(res.Unmanaged) != 1 || res.Unmanaged[0].Email != "jim@example.org" {
		t.Fatalf("unmanaged = %+v", res.Unmanaged)
	}
	for _, p := range e.fake.Providers() {
		if !p.Managed {
			t.Fatalf("%s was not taken on", p.Abbreviation)
		}
	}
	got, _ := e.svc.ManagedProviders(ctx)
	if codes(got) != "CLUBA,CUP" || got[1].Own || got[1].Name != "Parks Cup" || got[1].CourthiveID != old {
		t.Fatalf("managed = %+v", got)
	}
	if found, _ := e.svc.NotTakenOn(ctx); len(found) != 0 {
		t.Fatalf("still listed as not taken on: %+v", found)
	}
	// The club's own provider is unchanged in the settings.
	if setting(t, e.db, repository.SettingTournamentProviderID) != "provider-CLUBA" || setting(t, e.db, repository.SettingTournamentProviderAbbr) != "CLUBA" {
		t.Fatal("adding a provider changed the club's own")
	}
	// The permanent administrator is in both, as one account.
	var instance int
	for _, u := range e.fake.Users() {
		if u.ExternalID == courthive.InstanceExternalID {
			instance++
			if u.Membership["provider-CLUBA"] != courthive.RoleProviderAdmin || u.Membership[old] != courthive.RoleProviderAdmin {
				t.Fatalf("instance memberships = %v", u.Membership)
			}
		}
	}
	if instance != 1 {
		t.Fatalf("%d instance accounts", instance)
	}

	// Again: nothing made. A brand new code: made on the server.
	again, err := e.svc.AddProvider(ctx, actor, "CUP", "Parks Cup")
	if err != nil || again.MadeAnything() {
		t.Fatalf("repeat: %+v, %v", again, err)
	}
	fresh, err := e.svc.AddProvider(ctx, actor, "JUNIORS", "Junior tournaments")
	if err != nil || outcomes(fresh)[stepProvider] != OutcomeMade || len(e.fake.Providers()) != 3 {
		t.Fatalf("new provider: %+v, %v", fresh, err)
	}
	if _, err := e.svc.AddProvider(ctx, actor, "no good!", "x"); err == nil {
		t.Fatal("a bad code was accepted")
	}

	// A set-up run checks the other providers too: a permanent administrator
	// removed from one by hand is put back.
	for _, u := range e.fake.Users() {
		if u.ExternalID == courthive.InstanceExternalID {
			e.fake.RemoveMembership(u.ID, old)
		}
	}
	check, err := e.svc.Provision(ctx, actor, Request{})
	if err != nil || outcomes(check)[stepFurther] != OutcomeMade {
		t.Fatalf("set-up check: %+v, %v", check, err)
	}
	for _, u := range e.fake.Users() {
		if u.ExternalID == courthive.InstanceExternalID && u.Membership[old] != courthive.RoleProviderAdmin {
			t.Fatal("the permanent administrator was not put back")
		}
	}
}

func TestSyncAcrossTwoProviders(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	if _, err := e.svc.AddProvider(ctx, actor, "CUP", "Parks Cup"); err != nil {
		t.Fatal(err)
	}
	admin := addUser(t, e, "ada", "admin", "none", true)
	tina := addUser(t, e, "tina", "player", "own", true)
	sync := func(id int64) {
		t.Helper()
		if err := e.svc.SyncAccount(ctx, id); err != nil {
			t.Fatalf("SyncAccount(%d): %v", id, err)
		}
	}

	// An administrator is in both; someone with nothing picked is in the
	// club's own only.
	sync(admin)
	sync(tina)
	if got := memberships(e, admin); got != "CLUBA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Errorf("administrator: %q", got)
	}
	if got := memberships(e, tina); got != "CLUBA=DIRECTOR" {
		t.Errorf("nothing picked: %q", got)
	}

	// Picked for the cup only: added there, removed from the club's own.
	pick(t, e, tina, "CUP")
	sync(tina)
	if got := memberships(e, tina); got != "CUP=DIRECTOR" {
		t.Errorf("cup only: %q", got)
	}
	// Both, with more access: the one level applies in each.
	pick(t, e, tina, "CLUBA", "CUP")
	setUser(t, e, tina, "tournament_access", "all")
	sync(tina)
	if got := memberships(e, tina); got != "CLUBA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Errorf("both: %q", got)
	}
	if u, _ := tournamentUser(e, tina); strings.Join(u.Roles, ",") != "client,generate,score" {
		t.Errorf("global roles = %v", u.Roles)
	}
	// One removed: the global roles stay, since a provider is left.
	pick(t, e, tina, "CLUBA")
	sync(tina)
	if u, _ := tournamentUser(e, tina); memberships(e, tina) != "CLUBA=PROVIDER_ADMIN" || len(u.Roles) != 3 {
		t.Errorf("one removed: %q roles %v", memberships(e, tina), u.Roles)
	}
	// Access taken away: out of everything, and the global roles go.
	setUser(t, e, tina, "role", "captain")
	setUser(t, e, tina, "tournament_access", "none")
	sync(tina)
	if u, _ := tournamentUser(e, tina); len(u.Membership) != 0 || len(u.Roles) != 0 {
		t.Errorf("after removal: %v roles %v", u.Membership, u.Roles)
	}
	// Sessions ended: an administrator leaves both until they come back.
	e.svc.TournamentAccessChanged(ctx, accounts.TournamentAccessChange{UserID: admin, What: accounts.ChangeSessionsRevoked})
	if got := memberships(e, admin); got != "" {
		t.Errorf("after sessions were ended: %q", got)
	}
	if _, err := e.svc.Handover(ctx, admin, "CUP"); err != nil {
		t.Fatal(err)
	}
	if got := memberships(e, admin); got != "CLUBA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Errorf("after coming back: %q", got)
	}
}

func TestSyncKeepsTryingWhenOneProviderFails(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	if _, err := e.svc.AddProvider(ctx, actor, "CUP", "Parks Cup"); err != nil {
		t.Fatal(err)
	}
	tina := addUser(t, e, "tina", "player", "all", true)
	pick(t, e, tina, "CLUBA", "CUP")
	if err := e.svc.SyncAccount(ctx, tina); err != nil {
		t.Fatal(err)
	}
	// Removal from the cup fails once: recorded, and the retry finishes it.
	pick(t, e, tina, "CLUBA")
	e.fake.FailNext("remove-membership", 500)
	if err := e.svc.SyncAccount(ctx, tina); err == nil {
		t.Fatal("a failed removal was reported as done")
	}
	if p, _ := pending(t, e, tina); !p {
		t.Fatal("a failed removal was not kept for another try")
	}
	if n, err := e.svc.Reconcile(ctx); err != nil || n != 0 {
		t.Fatalf("Reconcile = %d, %v", n, err)
	}
	if p, _ := pending(t, e, tina); p || memberships(e, tina) != "CLUBA=PROVIDER_ADMIN" {
		t.Fatalf("after the retry: pending %v, %q", p, memberships(e, tina))
	}

	// The only administrator of a provider cannot be removed from it: the
	// change is kept as not applied, with the reason.
	for _, u := range e.fake.Users() {
		if u.ExternalID == courthive.InstanceExternalID {
			e.fake.RemoveMembership(u.ID, "provider-CLUBA")
		}
	}
	pick(t, e, tina, "CUP")
	if err := e.svc.SyncAccount(ctx, tina); !errors.Is(err, courthive.ErrLastAdministrator) {
		t.Fatalf("removing the last administrator: %v", err)
	}
	if p, why := pending(t, e, tina); !p || why == "" {
		t.Fatalf("pending %v %q", p, why)
	}
	if got := memberships(e, tina); got != "CLUBA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Errorf("after the refusal: %q", got)
	}
}

func TestHandoverNamesTheProvider(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	if _, err := e.svc.AddProvider(ctx, actor, "CUP", "Parks Cup"); err != nil {
		t.Fatal(err)
	}
	both := addUser(t, e, "both", "player", "own", true)
	cupOnly := addUser(t, e, "cup", "player", "own", true)
	clubOnly := addUser(t, e, "club", "player", "own", true)
	pick(t, e, both, "CLUBA", "CUP")
	pick(t, e, cupOnly, "CUP")

	// The fake's token says nothing about the provider, so count hand-overs
	// and check the refusals, which the fake makes for a non-member.
	if got, _ := e.svc.ProvidersFor(ctx, both); codes(got) != "CLUBA,CUP" {
		t.Fatalf("ProvidersFor(both) = %q", codes(got))
	}
	for _, c := range []struct {
		id   int64
		code string
		ok   bool
	}{
		{both, "", true}, {both, "cup", true}, {both, "CLUBA", true},
		{cupOnly, "", true}, {cupOnly, "CUP", true}, {cupOnly, "CLUBA", false},
		{clubOnly, "", true}, {clubOnly, "CUP", false}, {both, "NOSUCH", false},
	} {
		token, err := e.svc.Handover(ctx, c.id, c.code)
		switch {
		case c.ok && (err != nil || token == ""):
			t.Errorf("Handover(%d, %q) = %q, %v", c.id, c.code, token, err)
		case !c.ok && !errors.Is(err, ErrNoAccess):
			t.Errorf("Handover(%d, %q) = %v, want ErrNoAccess", c.id, c.code, err)
		}
	}
	// Someone picked for the cup only is not a member of the club's own, so
	// the tournament server itself would refuse a hand-over into it.
	if got := memberships(e, cupOnly); got != "CUP=DIRECTOR" {
		t.Errorf("cup only: %q", got)
	}
	u, _ := tournamentUser(e, cupOnly)
	if _, err := client(t, e.fake.URL, e.fake.URL, e.fake.Password).HandoverToken(ctx, "provider-CLUBA", u.ExternalID); err == nil {
		t.Error("the tournament server handed someone over into a provider they do not belong to")
	}
}

// A provider made before the app managed tournaments can have a code the app
// would never make ("St Ann's"). It is taken on as it is, however its code is
// typed; a code like that which is not on the tournament server is refused.
func TestAnExistingProviderWithAnOddCodeIsTakenOn(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.fake.AddProvider("St Ann's", "St Ann's Tennis", false)
	e.fake.AddProvider("BHPLTA", "Brighton and Hove Parks LTA", false)

	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "Not There!", Name: "x"}); err == nil {
		t.Fatal("a malformed code that names nothing on the tournament server was accepted")
	}
	if len(e.fake.Providers()) != 2 {
		t.Fatalf("a refused code made a provider: %d providers", len(e.fake.Providers()))
	}
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "bhplta", Name: "Parks Cup"}); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.AddProvider(ctx, actor, " st ann's ", "St Ann's club tournaments")
	if err != nil || !res.OK {
		t.Fatalf("AddProvider: %+v, %v", res, err)
	}
	got, _ := e.svc.ManagedProviders(ctx)
	if codes(got) != "BHPLTA,St Ann's" && codes(got) != "BHPLTA St Ann's" {
		t.Fatalf("managed = %q", codes(got))
	}
	if len(e.fake.Providers()) != 2 {
		t.Fatalf("taking on made a new provider: %d providers", len(e.fake.Providers()))
	}
	if res, err := e.svc.AddProvider(ctx, actor, "St Ann's", ""); err != nil || res.MadeAnything() {
		t.Fatalf("running it again changed something: %+v, %v", res, err)
	}
}

// The very first set-up can name an existing provider with an odd code: the
// app connects itself first, because the tournament server lists providers
// only to an app that is connected.
func TestFirstSetUpTakesOnAnOddCode(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.fake.AddProvider("St Ann's", "St Ann's Tennis", false)
	res, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "st ann's", Name: "St Ann's club tournaments"})
	if err != nil || !res.OK {
		t.Fatalf("Provision: %+v, %v", res, err)
	}
	got, _ := e.svc.ManagedProviders(ctx)
	if codes(got) != "St Ann's" || !got[0].Own || len(e.fake.Providers()) != 1 {
		t.Fatalf("managed = %+v, providers on the server = %d", got, len(e.fake.Providers()))
	}
}
