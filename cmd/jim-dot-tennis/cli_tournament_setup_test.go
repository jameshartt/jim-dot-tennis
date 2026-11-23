// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"net/http"
	"strings"
	"testing"

	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/courthive/courthivetest"
)

func tournamentEnv(t *testing.T, server, identity, password string) {
	t.Helper()
	t.Setenv(envCourtHiveAPIURL, server)
	t.Setenv(envCourtHiveIdentityURL, identity)
	t.Setenv(envCourtHiveServicePassword, password)
}

func TestCLITournamentsProvisionAndStatus(t *testing.T) {
	db := cliTestDB(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Alpha Tennis Club')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME_CLUB_ID", "1")
	fake := courthivetest.New(t)
	id := fake.AddProvider("ALPHA", "Alpha", false)
	fake.AddAccount(id, "organiser@example.org", courthive.RoleDirector, "", "")
	tournamentEnv(t, fake.URL, fake.URL, courthivetest.Password)

	r := runCLITest(t, "tournaments", "status")
	if r.code != exitOK || !strings.Contains(r.stdout, "switched on") || !strings.Contains(r.stdout, "tournament engine 7.1.1") || !strings.Contains(r.stdout, "Club set up: no.") {
		t.Fatalf("status before: %+v", r)
	}

	// First run with the suggested code (from the club's name): adopts the
	// provider made by hand and lists the person the app does not manage.
	r = runCLITest(t, "tournaments", "provision")
	if r.code != exitOK {
		t.Fatalf("provision: %+v", r)
	}
	for _, want := range []string{"suggested tournament code ALPHA", "Tournament set-up for ALPHA (Alpha Tennis Club)", "Permanent tournament administrator", "Made",
		"Tournaments are ready", "organiser@example.org  Tournament director", "nothing was changed for them", "--role tournaments"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("provision output lacks %q:\n%s", want, r.stdout)
		}
	}
	if !strings.Contains(r.stderr, "action=tournaments.setup") || !strings.Contains(r.stderr, `actor="cli:tournaments provision"`) {
		t.Errorf("not audited: %s", r.stderr)
	}

	// Again: nothing made.
	r = runCLITest(t, "tournaments", "provision")
	if r.code != exitOK || !strings.Contains(r.stdout, "nothing was changed") || strings.Contains(r.stdout, " Made") {
		t.Fatalf("second provision: %+v", r)
	}
	// The code is fixed.
	r = runCLITest(t, "tournaments", "provision", "--abbreviation", "OTHER")
	if r.code != exitError || !strings.Contains(r.stderr, "cannot be changed") {
		t.Fatalf("other code: %+v", r)
	}
	r = runCLITest(t, "tournaments", "status")
	if r.code != exitOK || !strings.Contains(r.stdout, "Club set up: yes, with the tournament code ALPHA") || !strings.Contains(r.stdout, "finished") {
		t.Fatalf("status after: %+v", r)
	}
	for _, tok := range append(fake.Tokens(), courthivetest.Password) {
		var all strings.Builder
		all.WriteString(r.stdout + r.stderr)
		if strings.Contains(all.String(), tok) {
			t.Fatal("output contains a token or the password")
		}
	}
}

func TestCLITournamentsProvisionFailsThenRetries(t *testing.T) {
	cliTestDB(t)
	fake := courthivetest.New(t)
	tournamentEnv(t, fake.URL, fake.URL, courthivetest.Password)
	fake.FailNext(courthivetest.CallMakeProvider, http.StatusServiceUnavailable)

	r := runCLITest(t, "tournaments", "provision", "--abbreviation=beta")
	if r.code != exitError || !strings.Contains(r.stdout, "Stopped at: The club on the tournament server.") || !strings.Contains(r.stderr, "Run \"jim-dot-tennis tournaments provision\" again") {
		t.Fatalf("failed run: %+v", r)
	}
	r = runCLITest(t, "tournaments", "status")
	if !strings.Contains(r.stdout, "the last attempt used the code BETA") || !strings.Contains(r.stdout, "stopped at") {
		t.Fatalf("status after failure: %+v", r)
	}
	// The retry needs no code: the one tried is kept.
	r = runCLITest(t, "tournaments", "provision")
	if r.code != exitOK || !strings.Contains(r.stdout, "Tournament set-up for BETA") {
		t.Fatalf("retry: %+v", r)
	}
	if len(fake.Providers()) != 1 {
		t.Fatal("duplicate provider")
	}
}

func TestCLITournamentsWithoutTheBundledStack(t *testing.T) {
	cliTestDB(t)
	tournamentEnv(t, "", "", "")
	r := runCLITest(t, "tournaments", "status")
	if r.code != exitOK || !strings.Contains(r.stdout, "not switched on in this deployment") {
		t.Fatalf("status: %+v", r)
	}
	r = runCLITest(t, "tournaments", "provision")
	if r.code != exitError || !strings.Contains(r.stderr, "not switched on in this deployment") {
		t.Fatalf("provision: %+v", r)
	}
}

func TestCLITournamentsBadSettings(t *testing.T) {
	cliTestDB(t)
	short := "short-password"
	tournamentEnv(t, "", "", short)
	r := runCLITest(t, "tournaments", "status")
	if r.code != exitError || !strings.Contains(r.stderr, "COURTHIVE_SERVICE_PASSWORD") || strings.Contains(r.stderr, short) {
		t.Fatalf("short password: %+v", r)
	}
	if _, _, err := tournamentClient(); err == nil || strings.Contains(err.Error(), short) {
		t.Fatalf("tournamentClient: %v", err)
	}
	tournamentEnv(t, "not a url", "", "")
	if _, _, err := tournamentClient(); err == nil || !strings.Contains(err.Error(), "COURTHIVE_API_URL") {
		t.Fatalf("bad address: %v", err)
	}
	// Defaults with nothing set.
	tournamentEnv(t, "", "", "")
	c, cfg, err := tournamentClient()
	if err != nil || c.Configured() || cfg.ServerURL != defaultCourtHiveAPIURL || cfg.IdentityURL != defaultCourtHiveIdentityURL {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
}

func TestCLITournamentsUsage(t *testing.T) {
	for _, args := range [][]string{
		{"tournaments"}, {"tournaments", "frobnicate"}, {"tournaments", "status", "extra"},
		{"tournaments", "provision", "--abbreviation"}, {"tournaments", "provision", "--role", "admin"},
		{"tournaments", "status", "--abbreviation", "X"}, {"user", "list", "--abbreviation", "X"},
	} {
		if r := runCLITest(t, args...); r.code != exitUsage {
			t.Errorf("%v: code %d (%s), want a usage error", args, r.code, r.stderr)
		}
	}
	r := runCLITest(t, "help")
	if !strings.Contains(r.stdout, "tournaments provision [--abbreviation <code>]") || !strings.Contains(r.stdout, "tournaments status") {
		t.Errorf("help lacks the tournament commands")
	}
}

// A second set of tournaments from the command line, and choosing who
// manages which (WI-154).
func TestCLITournamentsAddProviderAndChoose(t *testing.T) {
	db := cliTestDB(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Alpha Tennis Club')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME_CLUB_ID", "1")
	fake := courthivetest.New(t)
	tournamentEnv(t, fake.URL, fake.URL, courthivetest.Password)

	// Not before the club is set up.
	r := runCLITest(t, "tournaments", "add-provider", "CUP", "Parks", "Cup")
	if r.code != exitError || !strings.Contains(r.stderr, "tournaments provision") {
		t.Fatalf("before set-up: %+v", r)
	}
	if r := runCLITest(t, "tournaments", "provision"); r.code != exitOK {
		t.Fatalf("provision: %+v", r)
	}
	r = runCLITest(t, "tournaments", "add-provider", "cup", "Parks", "Cup")
	if r.code != exitOK || !strings.Contains(r.stdout, "Tournament set-up for CUP (Parks Cup)") || !strings.Contains(r.stdout, "--providers") ||
		!strings.Contains(r.stderr, `actor="cli:tournaments add-provider"`) || !strings.Contains(r.stderr, "further_provider") {
		t.Fatalf("add-provider: %+v", r)
	}
	if len(fake.Providers()) != 2 {
		t.Fatalf("providers on the server: %+v", fake.Providers())
	}
	r = runCLITest(t, "tournaments", "status")
	if r.code != exitOK || !strings.Contains(r.stdout, "Sets of tournaments:") || !strings.Contains(r.stdout, "Parks Cup") || !strings.Contains(r.stdout, "(the club's own)") {
		t.Fatalf("status: %+v", r)
	}

	// Someone for the cup only: a member there and nowhere else.
	r = runCLITest(t, "user", "create", "tina", "--role", "tournaments", "--tournaments", "own", "--providers", "cup")
	if r.code != exitOK || !strings.Contains(r.stdout, "in CUP") {
		t.Fatalf("create tina: %+v", r)
	}
	membership := func() string {
		for _, u := range fake.Users() {
			if u.Email == "tina@jim-dot-tennis.invalid" {
				var in []string
				for _, code := range []string{"ALPHA", "CUP"} {
					if role, ok := u.Membership["provider-"+code]; ok {
						in = append(in, code+"="+role)
					}
				}
				return strings.Join(in, ",")
			}
		}
		return "(no account)"
	}
	if got := membership(); got != "CUP=DIRECTOR" {
		t.Fatalf("tina on the tournament server: %q", got)
	}
	r = runCLITest(t, "user", "tournaments", "tina", "all", "--providers", "ALPHA,CUP")
	if r.code != exitOK || !strings.Contains(r.stdout, "ALPHA,CUP") || !strings.Contains(r.stderr, "action=user.tournament_providers") {
		t.Fatalf("both: %+v", r)
	}
	if got := membership(); got != "ALPHA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Fatalf("tina after both: %q", got)
	}
	// A wrong code changes nothing; misuse is a usage error.
	r = runCLITest(t, "user", "tournaments", "tina", "own", "--providers", "NOSUCH")
	if r.code != exitError || !strings.Contains(r.stderr, "not one this app manages") || membership() != "ALPHA=PROVIDER_ADMIN,CUP=PROVIDER_ADMIN" {
		t.Fatalf("wrong code: %+v (%s)", r, membership())
	}
	for _, args := range [][]string{
		{"user", "tournaments", "tina", "none", "--providers", "CUP"},
		{"user", "create", "tom", "--role", "admin", "--providers", "CUP"},
		{"user", "create", "tom", "--role", "captain", "--providers", "CUP"},
		{"user", "list", "--providers", "CUP"},
		{"tournaments", "add-provider", "CUP"},
		{"tournaments", "status", "--providers", "CUP"},
	} {
		if r := runCLITest(t, args...); r.code != exitUsage {
			t.Errorf("%v: code %d (%s), want a usage error", args, r.code, r.stderr)
		}
	}
}
