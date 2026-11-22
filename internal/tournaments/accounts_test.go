// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournaments

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/courthive/courthivetest"
)

// setUpEnv is an instance whose club is set up on the (fake) tournament server.
func setUpEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t)
	if _, err := e.svc.Provision(context.Background(), actor, Request{Abbreviation: "CLUBA", Name: "Club A"}); err != nil {
		t.Fatalf("set-up: %v", err)
	}
	return e
}

func addUser(t *testing.T, e env, username, role, access string, active bool) int64 {
	t.Helper()
	res, err := e.db.Exec(`INSERT INTO users (username, password_hash, role, is_active, tournament_access) VALUES (?, NULL, ?, ?, ?)`,
		username, role, active, access)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func setUser(t *testing.T, e env, id int64, column string, value any) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE users SET `+column+` = ? WHERE id = ?`, value, id); err != nil {
		t.Fatal(err)
	}
}

// tournamentUser finds the person's account on the fake tournament server.
func tournamentUser(e env, id int64) (courthivetest.User, bool) {
	for _, u := range e.fake.Users() {
		if u.SSOProvider == courthive.SSOProvider && u.ExternalID == strconv.FormatInt(id, 10) {
			return u, true
		}
	}
	return courthivetest.User{}, false
}

func membership(e env, id int64) string {
	u, ok := tournamentUser(e, id)
	if !ok {
		return "(no account)"
	}
	for _, role := range u.Membership {
		return role
	}
	return ""
}

func pending(t *testing.T, e env, id int64) (bool, string) {
	t.Helper()
	var p bool
	var why string
	if err := e.db.QueryRow(`SELECT pending, last_error FROM tournament_accounts WHERE user_id = ?`, id).Scan(&p, &why); err != nil {
		t.Fatal(err)
	}
	return p, why
}

func TestSyncAccountFollowsAccess(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()

	admin := addUser(t, e, "ada", "admin", "none", true)
	captain := addUser(t, e, "cap", "captain", "none", true)
	tina := addUser(t, e, "tina", "player", "own", true)

	for _, id := range []int64{admin, captain, tina} {
		if err := e.svc.SyncAccount(ctx, id); err != nil {
			t.Fatalf("SyncAccount(%d): %v", id, err)
		}
	}
	if got := membership(e, admin); got != courthive.RoleProviderAdmin {
		t.Errorf("administrator: %q, want provider administrator whatever is stored", got)
	}
	if got := membership(e, captain); got != "(no account)" {
		t.Errorf("a captain without access got a tournament account: %q", got)
	}
	if got := membership(e, tina); got != courthive.RoleDirector {
		t.Errorf("own access: %q, want director", got)
	}
	if u, _ := tournamentUser(e, tina); u.Email != "tina@jim-dot-tennis.invalid" || strings.Join(u.Roles, ",") != "client,generate,score" {
		t.Errorf("tournament account: %q roles %v", u.Email, u.Roles)
	}

	// Granted more, then a captain is granted some.
	setUser(t, e, tina, "tournament_access", "all")
	setUser(t, e, captain, "tournament_access", "own")
	for _, id := range []int64{tina, captain} {
		if err := e.svc.SyncAccount(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if membership(e, tina) != courthive.RoleProviderAdmin || membership(e, captain) != courthive.RoleDirector {
		t.Errorf("after grants: tina %q, captain %q", membership(e, tina), membership(e, captain))
	}

	// Taken away: membership and global roles both go.
	setUser(t, e, captain, "tournament_access", "none")
	if err := e.svc.SyncAccount(ctx, captain); err != nil {
		t.Fatal(err)
	}
	if u, _ := tournamentUser(e, captain); len(u.Membership) != 0 || len(u.Roles) != 0 {
		t.Errorf("after removal: membership %v roles %v", u.Membership, u.Roles)
	}

	// Disabled: gone, even for an administrator.
	setUser(t, e, admin, "is_active", false)
	if err := e.svc.SyncAccount(ctx, admin); err != nil {
		t.Fatal(err)
	}
	if got := membership(e, admin); got != "" {
		t.Errorf("disabled administrator still a member: %q", got)
	}

	// Enabled again: back, found by the id the app kept.
	setUser(t, e, admin, "is_active", true)
	if err := e.svc.SyncAccount(ctx, admin); err != nil {
		t.Fatalf("return: %v", err)
	}
	if got := membership(e, admin); got != courthive.RoleProviderAdmin {
		t.Errorf("after return: %q", got)
	}
}

func TestHandover(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	tina := addUser(t, e, "tina", "player", "all", true)
	captain := addUser(t, e, "cap", "captain", "none", true)

	token, err := e.svc.Handover(ctx, tina)
	if err != nil || token == "" {
		t.Fatalf("Handover = %q, %v", token, err)
	}
	if membership(e, tina) != courthive.RoleProviderAdmin {
		t.Error("the first hand-over did not make the tournament account")
	}
	if _, err := e.svc.Handover(ctx, captain); !errors.Is(err, ErrNoAccess) {
		t.Fatalf("no access: %v", err)
	}
	if e.fake.Calls(courthivetest.CallHandover) != 1 {
		t.Errorf("hand-over tokens requested: %d, want 1", e.fake.Calls(courthivetest.CallHandover))
	}

	// Access removed since the last visit: refused, and the membership goes
	// even though nothing told the tournament server at the time.
	setUser(t, e, tina, "tournament_access", "none")
	setUser(t, e, tina, "role", "captain")
	if _, err := e.svc.Handover(ctx, tina); !errors.Is(err, ErrNoAccess) {
		t.Fatalf("after losing access: %v", err)
	}
	if got := membership(e, tina); got != "" {
		t.Errorf("membership left behind: %q", got)
	}
}

func TestObserverEndsAndRestoresAccess(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	tina := addUser(t, e, "tina", "player", "own", true)
	if _, err := e.svc.Handover(ctx, tina); err != nil {
		t.Fatal(err)
	}

	// Sessions ended here: the tournament membership goes at once...
	e.svc.TournamentAccessChanged(ctx, accounts.TournamentAccessChange{UserID: tina, Username: "tina", What: "sessions_revoked"})
	if got := membership(e, tina); got != "" {
		t.Fatalf("after sessions were ended: %q", got)
	}
	// ...and comes back when they next open tournament management.
	if _, err := e.svc.Handover(ctx, tina); err != nil {
		t.Fatal(err)
	}
	if got := membership(e, tina); got != courthive.RoleDirector {
		t.Fatalf("after signing in again: %q", got)
	}

	// Disabled through the accounts service's hook.
	setUser(t, e, tina, "is_active", false)
	e.svc.TournamentAccessChanged(ctx, accounts.TournamentAccessChange{UserID: tina, Username: "tina", What: "active"})
	if got := membership(e, tina); got != "" {
		t.Fatalf("after being disabled: %q", got)
	}
}

func TestFailedRemovalIsRetriedUntilItSucceeds(t *testing.T) {
	e := setUpEnv(t)
	ctx := context.Background()
	tina := addUser(t, e, "tina", "player", "all", true)
	if _, err := e.svc.Handover(ctx, tina); err != nil {
		t.Fatal(err)
	}

	// The tournament server is down when tina is disabled.
	setUser(t, e, tina, "is_active", false)
	e.fake.FailNext(courthivetest.CallRemove, 503)
	e.svc.TournamentAccessChanged(ctx, accounts.TournamentAccessChange{UserID: tina, Username: "tina", What: "active"})
	if got := membership(e, tina); got != courthive.RoleProviderAdmin {
		t.Fatalf("the failed call should have changed nothing: %q", got)
	}
	isPending, why := pending(t, e, tina)
	if !isPending || why == "" {
		t.Fatalf("the change was dropped: pending=%v why=%q", isPending, why)
	}
	for _, token := range e.fake.Tokens() {
		if strings.Contains(why, token) {
			t.Fatal("the stored reason contains a token")
		}
	}
	if strings.Contains(why, courthivetest.Password) {
		t.Fatal("the stored reason contains the password")
	}
	list, err := e.svc.PendingAccounts(ctx)
	if err != nil || len(list) != 1 || list[0].UserID != tina {
		t.Fatalf("PendingAccounts = %v, %v", list, err)
	}

	// The retry applies it.
	left, err := e.svc.Reconcile(ctx)
	if err != nil || left != 0 {
		t.Fatalf("Reconcile = %d, %v", left, err)
	}
	if got := membership(e, tina); got != "" {
		t.Fatalf("after the retry: %q", got)
	}
	if isPending, _ := pending(t, e, tina); isPending {
		t.Fatal("still marked pending after a successful retry")
	}
}

func TestBeforeSetUpNothingIsToldAndNothingBreaks(t *testing.T) {
	e := newEnv(t) // configured, but the club is not set up
	ctx := context.Background()
	tina := addUser(t, e, "tina", "player", "all", true)
	captain := addUser(t, e, "cap", "captain", "none", true)

	if err := e.svc.SyncAccount(ctx, captain); err != nil {
		t.Fatalf("no access needs nothing: %v", err)
	}
	if err := e.svc.SyncAccount(ctx, tina); !errors.Is(err, ErrNotSetUp) {
		t.Fatalf("want ErrNotSetUp, got %v", err)
	}
	if _, err := e.svc.Handover(ctx, tina); !errors.Is(err, ErrNotSetUp) {
		t.Fatalf("hand-over before set-up: %v", err)
	}
	e.svc.TournamentAccessChanged(ctx, accounts.TournamentAccessChange{UserID: tina, What: "tournament_access"})
	if n := e.fake.Calls(courthivetest.CallMakeAccount); n != 0 {
		t.Fatalf("accounts made before set-up: %d", n)
	}

	// Not configured at all: the same, with a different reason.
	bare := NewService(e.db, client(t, e.fake.URL, e.fake.URL, ""))
	if _, err := bare.Handover(ctx, tina); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("hand-over without a tournament stack: %v", err)
	}
	if left, err := bare.Reconcile(ctx); err != nil || left != 0 {
		t.Fatalf("Reconcile without a tournament stack = %d, %v", left, err)
	}
}
