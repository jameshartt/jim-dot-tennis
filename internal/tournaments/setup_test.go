// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournaments

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/courthive/courthivetest"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: filepath.Join(t.TempDir(), "tennis.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrationsFS(jimdottennis.Migrations(), "built-in"); err != nil {
		t.Fatal(err)
	}
	return db
}

func client(t *testing.T, server, identity, password string) *courthive.Client {
	t.Helper()
	c, err := courthive.New(courthive.Config{ServerURL: server, IdentityURL: identity, ServicePassword: password})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type env struct {
	db   *database.DB
	fake *courthivetest.Server
	svc  *Service
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := testDB(t)
	fake := courthivetest.New(t)
	return env{db: db, fake: fake, svc: NewService(db, client(t, fake.URL, fake.URL, courthivetest.Password))}
}

var actor = accounts.UserActor(1, "alice")

func outcomes(r *Result) map[string]Outcome {
	out := map[string]Outcome{}
	for _, s := range r.Steps {
		out[s.Key] = s.Outcome
	}
	return out
}

func setting(t *testing.T, db *database.DB, key string) string {
	t.Helper()
	v, _, err := repository.NewSettingsRepository(db).GetString(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFirstRunThenRepeat(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	res, err := e.svc.Provision(ctx, actor, Request{Abbreviation: " alpha ", Name: "Alpha Tennis Club"})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if !res.OK || res.Abbreviation != "ALPHA" || res.EngineVersion != "7.1.1" || res.By != "alice" {
		t.Fatalf("result = %+v", res)
	}
	want := map[string]Outcome{stepReach: OutcomeChecked, stepProvisioner: OutcomeMade, stepProvider: OutcomeMade, stepInstance: OutcomeMade, stepRegister: OutcomeMade}
	if got := outcomes(res); len(got) != len(want) {
		t.Fatalf("steps = %+v", res.Steps)
	} else {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("step %s = %s, want %s", k, got[k], v)
			}
		}
	}
	// What exists afterwards: one provisioner, one provider, the instance
	// administrator with the roles tournament management needs, the ids, and
	// a provider row for calendar sync.
	if e.fake.Provisioners() != 1 || len(e.fake.Providers()) != 1 {
		t.Fatalf("provisioners %d providers %+v", e.fake.Provisioners(), e.fake.Providers())
	}
	users := e.fake.Users()
	if len(users) != 1 || users[0].ExternalID != courthive.InstanceExternalID || users[0].SSOProvider != courthive.SSOProvider ||
		users[0].Membership["provider-ALPHA"] != courthive.RoleProviderAdmin || strings.Join(users[0].Roles, ",") != "client,generate,score" {
		t.Fatalf("users = %+v", users)
	}
	if setting(t, e.db, repository.SettingTournamentProviderID) != "provider-ALPHA" ||
		setting(t, e.db, repository.SettingTournamentProvisionerID) != "prov-jim-dot-tennis" ||
		setting(t, e.db, repository.SettingTournamentProviderAbbr) != "ALPHA" ||
		setting(t, e.db, repository.SettingTournamentInstanceAccountID) != "user-instance" {
		t.Fatal("ids not stored")
	}
	rows, err := repository.NewTournamentProviderRepository(e.db).FindAll(ctx)
	if err != nil || len(rows) != 1 || rows[0].ProviderAbbr != "ALPHA" || rows[0].Name != "Alpha Tennis Club" {
		t.Fatalf("provider rows = %+v, %v", rows, err)
	}

	// Again: nothing made, everything reported as existing.
	makes := e.fake.Calls(courthivetest.CallMakeProvider) + e.fake.Calls(courthivetest.CallMakeProvisioner)
	res, err = e.svc.Provision(ctx, actor, Request{Name: "Alpha Tennis Club"})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.MadeAnything() {
		t.Fatalf("second run made something: %+v", res.Steps)
	}
	if e.fake.Calls(courthivetest.CallMakeProvider)+e.fake.Calls(courthivetest.CallMakeProvisioner) != makes {
		t.Fatal("second run asked to make a provider or provisioner")
	}
	if rows, _ := repository.NewTournamentProviderRepository(e.db).FindAll(ctx); len(rows) != 1 {
		t.Fatalf("duplicate provider rows: %+v", rows)
	}
	if len(e.fake.Users()) != 1 {
		t.Fatal("a second instance account")
	}

	st, err := e.svc.Status(ctx)
	if err != nil || !st.SetUp || !st.Configured || st.Abbreviation != "ALPHA" || st.Last == nil || !st.Last.OK {
		t.Fatalf("status = %+v, %v", st, err)
	}

	// Both runs are in the audit log, as instance-level events.
	var details []string
	if err := e.db.Select(&details, `SELECT detail FROM account_audit_log WHERE action = ? ORDER BY id`, ActionSetup); err != nil {
		t.Fatal(err)
	}
	if len(details) != 2 || !strings.Contains(details[0], "made=provisioner,provider,instance,register") || !strings.Contains(details[1], "made=nothing") {
		t.Fatalf("audit = %q", details)
	}
}

func TestAbbreviationFixedAfterSetUp(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	var fixed *AbbreviationFixedError
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "BETA", Name: "Alpha"}); !errors.As(err, &fixed) || fixed.Current != "ALPHA" {
		t.Fatalf("want AbbreviationFixedError, got %v", err)
	}
	if len(e.fake.Providers()) != 1 {
		t.Fatal("a second provider was made")
	}
	// The same abbreviation, in any case, is fine.
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "alpha", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	if got := e.svc.DefaultAbbreviation(ctx, "Something Else"); got != "ALPHA" {
		t.Fatalf("default after set-up = %q", got)
	}
}

func TestBadOrMissingAbbreviation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, a := range []string{"A", "ST ANNS", "TOO-LONG", "ABCDEFGHIJKLM", "ÅBC"} {
		if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: a}); err == nil {
			t.Errorf("%q accepted", a)
		}
	}
	if _, err := e.svc.Provision(ctx, actor, Request{}); !errors.Is(err, ErrNoAbbreviation) {
		t.Fatalf("want ErrNoAbbreviation, got %v", err)
	}
	if e.fake.Calls(courthivetest.CallVersion) != 0 {
		t.Fatal("nothing should be sent for a refused abbreviation")
	}
}

// Each step can fail; the next run finishes the set-up without duplicates.
func TestFailureAtEachStepRecoversOnRetry(t *testing.T) {
	cases := []struct {
		call, step string
	}{
		{courthivetest.CallVersion, stepReach},
		{courthivetest.CallSignIn, stepProvisioner},
		{courthivetest.CallMakeProvisioner, stepProvisioner},
		{courthivetest.CallAssign, stepProvisioner},
		{courthivetest.CallListProviders, stepProvider},
		{courthivetest.CallMakeProvider, stepProvider},
		{courthivetest.CallMembers, stepInstance},
		{courthivetest.CallMakeAccount, stepInstance},
		{courthivetest.CallSetGlobalRoles, stepInstance},
	}
	for _, c := range cases {
		t.Run(c.call, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			e.fake.FailNext(c.call, http.StatusServiceUnavailable)
			res, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA", Name: "Alpha"})
			var stepErr *StepError
			if !errors.As(err, &stepErr) || res == nil || res.OK {
				t.Fatalf("want a step failure, got %v / %+v", err, res)
			}
			if res.FailedStep != stepLabels[c.step] || stepErr.Step != stepLabels[c.step] || res.Problem == "" {
				t.Fatalf("failed at %q (%q), want %q", res.FailedStep, res.Problem, stepLabels[c.step])
			}
			st, _ := e.svc.Status(ctx)
			if st.Last == nil || st.Last.OK || st.Last.FailedStep != stepLabels[c.step] {
				t.Fatalf("the failure was not stored: %+v", st.Last)
			}

			res, err = e.svc.Provision(ctx, actor, Request{Name: "Alpha"})
			if err != nil || !res.OK {
				t.Fatalf("retry: %v", err)
			}
			if len(e.fake.Providers()) != 1 || e.fake.Provisioners() != 1 || len(e.fake.Users()) != 1 {
				t.Fatalf("duplicates after retry: %+v %+v", e.fake.Providers(), e.fake.Users())
			}
			if u := e.fake.Users()[0]; len(u.Roles) != 3 || u.Membership["provider-ALPHA"] != courthive.RoleProviderAdmin {
				t.Fatalf("instance account after retry: %+v", u)
			}
			if setting(t, e.db, repository.SettingTournamentProviderID) != "provider-ALPHA" {
				t.Fatal("provider id not stored after retry")
			}
		})
	}
}

// A run that made the provider but stopped before storing its id adopts it
// next time, even with no abbreviation given (the one tried was stored).
func TestProviderMadeButIDNotStored(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewSettingsRepository(e.db)
	_ = repo.Delete(ctx, repository.SettingTournamentProviderID)
	_ = repo.Delete(ctx, repository.SettingTournamentInstanceAccountID)

	res, err := e.svc.Provision(ctx, actor, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if outcomes(res)[stepProvider] != OutcomeMade || !strings.Contains(res.Steps[2].Detail, "taken on") {
		t.Fatalf("provider step = %+v", res.Steps[2])
	}
	if len(e.fake.Providers()) != 1 || setting(t, e.db, repository.SettingTournamentProviderID) != "provider-ALPHA" {
		t.Fatal("not adopted")
	}
}

func TestAdoptsProviderMadeByHandAndListsUnmanagedAccounts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.fake.AddProvider("STANNS", "St Ann's", false)
	e.fake.AddAccount(id, "jim@example.org", courthive.RoleProviderAdmin, "", "")
	e.fake.AddAccount(id, "dir@example.org", courthive.RoleDirector, "other-system", "x1")
	// The app's own legacy provider row, added by hand, is kept as it is.
	if err := repository.NewTournamentProviderRepository(e.db).Create(ctx, &models.TournamentProvider{Name: "St Ann's (old)", ProviderAbbr: "stanns"}); err != nil {
		t.Fatal(err)
	}
	if got := e.svc.DefaultAbbreviation(ctx, "Another Name"); got != "STANNS" {
		t.Fatalf("default = %q, want the registered provider's", got)
	}

	res, err := e.svc.Provision(ctx, actor, Request{Abbreviation: "STANNS", Name: "St Ann's"})
	if err != nil {
		t.Fatal(err)
	}
	if o := outcomes(res); o[stepProvider] != OutcomeMade || o[stepRegister] != OutcomeExisted {
		t.Fatalf("steps = %+v", res.Steps)
	}
	if p := e.fake.Providers(); len(p) != 1 || !p[0].Managed {
		t.Fatalf("provider not adopted: %+v", p)
	}
	if len(res.Unmanaged) != 2 {
		t.Fatalf("unmanaged = %+v", res.Unmanaged)
	}
	got := map[string]string{}
	for _, a := range res.Unmanaged {
		got[a.Email] = a.Role
	}
	if got["jim@example.org"] != courthive.RoleProviderAdmin || got["dir@example.org"] != courthive.RoleDirector {
		t.Fatalf("unmanaged = %+v", res.Unmanaged)
	}
	// Nothing changed for them.
	for _, u := range e.fake.Users() {
		if u.Email == "jim@example.org" && (len(u.Roles) != 0 || u.Membership[id] != courthive.RoleProviderAdmin) {
			t.Fatalf("an unmanaged account was changed: %+v", u)
		}
	}
	rows, _ := repository.NewTournamentProviderRepository(e.db).FindAll(ctx)
	if len(rows) != 1 || rows[0].Name != "St Ann's (old)" {
		t.Fatalf("provider rows = %+v", rows)
	}
	st, _ := e.svc.Status(ctx)
	if st.Last == nil || len(st.Last.Unmanaged) != 2 {
		t.Fatal("unmanaged accounts not kept with the outcome")
	}
}

// The tournament server already has the "instance" account from an earlier
// set-up whose ids this app's database does not hold (a fresh database, or a
// second provider on the same server): it is found through the providers the
// app owns and added to the club's provider, not made twice.
func TestInstanceAccountFromAnEarlierSetUp(t *testing.T) {
	ctx := context.Background()
	fake := courthivetest.New(t)
	first := NewService(testDB(t), client(t, fake.URL, fake.URL, courthivetest.Password))
	if _, err := first.Provision(ctx, actor, Request{Abbreviation: "ALPHA"}); err != nil {
		t.Fatal(err)
	}
	second := NewService(testDB(t), client(t, fake.URL, fake.URL, courthivetest.Password))
	res, err := second.Provision(ctx, actor, Request{Abbreviation: "BETA"})
	if err != nil {
		t.Fatalf("second database: %v", err)
	}
	if outcomes(res)[stepInstance] != OutcomeMade {
		t.Fatalf("instance step = %+v", res.Steps)
	}
	users := fake.Users()
	if len(users) != 1 || users[0].Membership["provider-BETA"] != courthive.RoleProviderAdmin || users[0].Membership["provider-ALPHA"] != courthive.RoleProviderAdmin {
		t.Fatalf("users = %+v", users)
	}
	if res, err := second.Provision(ctx, actor, Request{}); err != nil || res.MadeAnything() {
		t.Fatalf("repeat: %v %+v", err, res)
	}
}

func TestNotConfigured(t *testing.T) {
	db := testDB(t)
	for _, c := range []Client{nil, client(t, "http://a:1", "http://b:2", "")} {
		svc := NewService(db, c)
		if svc.Configured() {
			t.Fatal("configured without a password")
		}
		if _, err := svc.Provision(context.Background(), actor, Request{Abbreviation: "ALPHA"}); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("want ErrNotConfigured, got %v", err)
		}
		st, err := svc.Status(context.Background())
		if err != nil || st.Configured || st.SetUp {
			t.Fatalf("status = %+v, %v", st, err)
		}
	}
}

func TestPlainWordsAndNoSecrets(t *testing.T) {
	ctx := context.Background()
	fake := courthivetest.New(t)
	wrong := strings.Repeat("w", 40)

	// Wrong password.
	svc := NewService(testDB(t), client(t, fake.URL, fake.URL, wrong))
	res, err := svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA"})
	if err == nil || !strings.Contains(res.Problem, "COURTHIVE_SERVICE_PASSWORD") || res.FailedStep != stepLabels[stepProvisioner] {
		t.Fatalf("wrong password: %v / %+v", err, res)
	}
	if strings.Contains(err.Error()+res.Problem+res.Detail, wrong) {
		t.Fatal("the failure quotes the password")
	}

	// Identity service down.
	svc = NewService(testDB(t), client(t, fake.URL, "http://127.0.0.1:1", courthivetest.Password))
	res, err = svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA"})
	if err == nil || !strings.Contains(res.Problem, "sign-in service did not answer") {
		t.Fatalf("identity down: %v / %+v", err, res)
	}

	// Server down.
	svc = NewService(testDB(t), client(t, "http://127.0.0.1:1", fake.URL, courthivetest.Password))
	res, err = svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA"})
	if err == nil || res.FailedStep != stepLabels[stepReach] || !strings.Contains(res.Problem, "try again") {
		t.Fatalf("server down: %v / %+v", err, res)
	}

	// A good run, then check every stored text for the password and tokens.
	db := testDB(t)
	svc = NewService(db, client(t, fake.URL, fake.URL, courthivetest.Password))
	if _, err := svc.Provision(ctx, actor, Request{Abbreviation: "ALPHA"}); err != nil {
		t.Fatal(err)
	}
	var stored []string
	if err := db.Select(&stored, `SELECT value FROM app_settings UNION ALL SELECT detail FROM account_audit_log`); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(stored, "\n")
	for _, secret := range append(fake.Tokens(), courthivetest.Password, wrong) {
		if strings.Contains(all, secret) {
			t.Fatal("a token or password was stored")
		}
	}
}

func TestDeriveAbbreviation(t *testing.T) {
	for name, want := range map[string]string{
		"St Ann's":                          "STANNS",
		"St Ann's Tennis Club":              "STANNS",
		"Hove Park":                         "HOVEPARK",
		"Preston Park Lawn Tennis Club":     "PP",
		"Brighton and Hove Albion Tennis":   "BHA",
		"The Tennis Club":                   "TTC",
		"Supercalifragilistic":              "SUPERCALIF",
		"Queen’s Park":                      "QUEENSPARK",
		"":                                  "CLUB",
		"X":                                 "CLUB",
		"Dyke":                              "DYKE",
		"Withdean Sports Complex Tennis TC": "WSC",
	} {
		got := DeriveAbbreviation(name)
		if got != want {
			t.Errorf("DeriveAbbreviation(%q) = %q, want %q", name, got, want)
		}
		if got != "" {
			if _, err := NormalizeAbbreviation(got); err != nil {
				t.Errorf("%q gives an invalid abbreviation %q", name, got)
			}
		}
	}
}
