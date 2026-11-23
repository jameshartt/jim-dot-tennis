// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
)

func TestCLITournamentAccounts(t *testing.T) {
	db := cliTestDB(t)
	t.Setenv("PUBLIC_URL", "https://tennis.example.org")

	// A Tournaments only account, with its invite link.
	r := runCLITest(t, "user", "create", "tina", "--role", "tournaments", "--tournaments", "own")
	if r.code != exitOK {
		t.Fatalf("create tina: %d %s", r.code, r.stderr)
	}
	printedToken(t, r.stdout, true)
	tina := loadUser(t, db, "tina")
	if tina.Role != models.RoleTournamentsOnly || tina.TournamentAccess != models.TournamentAccessOwn {
		t.Fatalf("tina: %+v", tina)
	}
	if !strings.Contains(r.stdout, "Tournaments only, own tournaments") {
		t.Errorf("create output: %s", r.stdout)
	}
	for _, want := range []string{"action=user.create", `actor="cli:user create"`, "tournament_access=own"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("audit record lacks %q: %s", want, r.stderr)
		}
	}
	// A captain, given all tournaments when created; another with none.
	if r := runCLITest(t, "user", "create", "cap", "--role=captain", "--tournaments=all"); r.code != exitOK {
		t.Fatalf("create cap: %+v", r)
	}
	if r := runCLITest(t, "user", "create", "ann", "--role", "admin"); r.code != exitOK {
		t.Fatalf("create ann: %+v", r)
	}

	// Usage mistakes: nothing created.
	for _, args := range [][]string{
		{"user", "create", "tom", "--role", "tournaments"},
		{"user", "create", "tom", "--role", "tournaments", "--tournaments", "none"},
		{"user", "create", "tom", "--role", "tournaments", "--tournaments", "some"},
		{"user", "create", "tom", "--role", "admin", "--tournaments", "all"},
		{"user", "create", "tom", "--role", "player", "--tournaments", "own"},
		{"user", "create", "tom", "--role", "tournaments", "--tournaments"},
		{"user", "tournaments", "tina"},
		{"user", "tournaments", "tina", "some"},
		{"user", "role", "tina", "tournaments"},
		{"user", "list", "--tournaments", "all"},
	} {
		if r := runCLITest(t, args...); r.code != exitUsage {
			t.Errorf("%v: code %d (%s), want a usage error", args, r.code, r.stderr)
		}
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM users WHERE username = 'tom'`); err != nil || n != 0 {
		t.Errorf("tom created (%v)", err)
	}

	// Change access.
	r = runCLITest(t, "user", "tournaments", "tina", "all")
	if r.code != exitOK || !strings.Contains(r.stdout, "from own to all") || !strings.Contains(r.stderr, "action=user.tournament_access") ||
		!strings.Contains(r.stderr, `actor="cli:user tournaments"`) || !strings.Contains(r.stderr, "tournament_access own -> all") {
		t.Errorf("tina -> all: %+v", r)
	}
	r = runCLITest(t, "user", "tournaments", "tina", "all")
	if r.code != exitOK || !strings.Contains(r.stdout, "nothing changed") {
		t.Errorf("tina -> all again: %+v", r)
	}
	r = runCLITest(t, "user", "tournaments", "tina", "none")
	if r.code != exitError || !strings.Contains(r.stderr, "disable the account instead") {
		t.Errorf("tina -> none: %+v", r)
	}
	r = runCLITest(t, "user", "tournaments", "ann", "own")
	if r.code != exitError || !strings.Contains(r.stderr, "always have access to every tournament") {
		t.Errorf("ann -> own: %+v", r)
	}
	if r := runCLITest(t, "user", "tournaments", "CAP", "none"); r.code != exitOK {
		t.Errorf("cap -> none: %+v", r)
	}
	if got := loadUser(t, db, "cap").TournamentAccess; got != models.TournamentAccessNone {
		t.Errorf("cap: %q", got)
	}
	var rows []string
	if err := db.Select(&rows, `SELECT actor_kind || ' ' || target_username || ' ' || detail FROM account_audit_log WHERE action = 'user.tournament_access' ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows, "|") != "cli tina tournament_access own -> all|cli cap tournament_access all -> none" {
		t.Errorf("audit rows: %q", rows)
	}

	// The list shows the kind of account and the access.
	r = runCLITest(t, "user", "list")
	if r.code != exitOK || !strings.Contains(r.stdout, "TOURNAMENTS") || !strings.Contains(r.stdout, "Tournaments only") ||
		!strings.Contains(r.stdout, "all (administrator)") {
		t.Errorf("list: %+v", r)
	}
}
