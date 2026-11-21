// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// Regression tests for the findings of the WI-134 security review.

// F5: the last-administrator guard used to count administrator accounts
// nobody can sign in to (an invite never accepted), so the only working
// administrator could be disabled or demoted.
func TestLastAdminGuardCountsOnlyAdminsWhoCanSignIn(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	bob, bobLink, err := s.Invite(ctx, cli, "bob", models.RoleAdmin, nil) // never accepted
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, UserActor(alice.ID, alice.Username), alice.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable the only administrator who can sign in: %v, want ErrLastAdmin", err)
	}
	if _, err := s.SetRole(ctx, cli, alice.ID, models.RoleCaptain); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote the only administrator who can sign in: %v, want ErrLastAdmin", err)
	}
	if a, _ := s.Get(ctx, alice.ID); !a.IsActiveAdmin() {
		t.Fatalf("alice changed: %+v", a)
	}
	// Cancelling bob's invite is harmless now: alice cannot be removed.
	if _, err := s.RevokeLink(ctx, cli, bobLink.ID); err != nil {
		t.Fatalf("revoke bob's invite: %v", err)
	}
	// The unusable administrator can be disabled or demoted: nothing is lost.
	if _, err := s.SetActive(ctx, cli, bob.ID, false); err != nil {
		t.Fatalf("disable the administrator who cannot sign in: %v", err)
	}
	if _, err := s.SetActive(ctx, cli, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	// Once bob can sign in, alice can go.
	link, err := s.IssueLink(ctx, cli, bob.ID, LinkInvite)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, cli, alice.ID, false); err != nil {
		t.Fatalf("disable alice once bob can sign in: %v", err)
	}
	if _, err := s.SetRole(ctx, cli, bob.ID, models.RoleCaptain); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote bob, now the last: %v, want ErrLastAdmin", err)
	}
}

// F5: with passkeys required for administrators, a password-only
// administrator cannot sign in, so it does not count, and enabling one is
// refused. The rescue paths from the command line still work when no
// administrator can sign in at all.
func TestLastAdminGuardWithPasskeysRequired(t *testing.T) {
	s, db := newTestService(t)
	ctx := context.Background()
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	bob := mustCreate(t, s, "bob", models.RoleAdmin)
	storePasskey(t, db, alice.ID, "alice-key", false)
	if _, err := s.SetActive(ctx, cli, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdminPasskeysRequired(ctx, cli, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, cli, bob.ID, true); !errors.Is(err, ErrEnableNeedsPasskey) {
		t.Fatalf("enable a password-only administrator while passkeys are required: %v, want ErrEnableNeedsPasskey", err)
	}
	if a, _ := s.Get(ctx, bob.ID); a.IsActive {
		t.Fatal("bob was enabled")
	}
	// As a captain he can be enabled (and promoted once he has a passkey).
	if _, err := s.SetRole(ctx, cli, bob.ID, models.RoleCaptain); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, cli, bob.ID, true); err != nil {
		t.Fatalf("enable bob as a captain: %v", err)
	}

	// A password-only administrator made active behind the service's back
	// (or left from before the requirement) does not make alice removable.
	db.MustExec(`UPDATE users SET role = 'admin' WHERE id = ?`, bob.ID)
	if _, err := s.SetActive(ctx, cli, alice.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable the only administrator with a passkey: %v, want ErrLastAdmin", err)
	}
	// A blocked passkey is not a way in either.
	storePasskey(t, db, bob.ID, "bob-blocked", true)
	if _, err := s.SetRole(ctx, cli, alice.ID, models.RoleCaptain); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote alice while bob's only passkey is blocked: %v, want ErrLastAdmin", err)
	}
	storePasskey(t, db, bob.ID, "bob-key", false)
	if _, err := s.SetActive(ctx, cli, alice.ID, false); err != nil {
		t.Fatalf("disable alice once bob has a usable passkey: %v", err)
	}

	// Rescue: nobody can sign in as an administrator (bob's passkeys gone).
	db.MustExec(`DELETE FROM webauthn_credentials WHERE user_id = ?`, bob.ID)
	if _, _, err := s.Invite(ctx, CLIActor("user create"), "carol", models.RoleAdmin, nil); err != nil {
		t.Fatalf("user create --role admin with no usable administrator: %v", err)
	}
	if _, err := s.IssueLink(ctx, CLIActor("user reset"), bob.ID, LinkRecovery); err != nil {
		t.Fatalf("user reset with no usable administrator: %v", err)
	}
	if err := s.SetAdminPasskeysRequired(ctx, CLIActor("admin-passkeys off"), false); err != nil {
		t.Fatalf("admin-passkeys off: %v", err)
	}
}

// F2: removing the password needs proof of ownership, and adding, changing
// or removing one ends the account's sessions.
func TestRemovePasswordNeedsProof(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleCaptain)
	storePasskey(t, db, a.ID, "key", false)
	addSession(t, db, "elsewhere", a.ID)

	if _, err := s.RemoveOwnPassword(ctx, a.ID, Proof{}); !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("no proof: %v, want ErrReauthRequired", err)
	}
	if _, err := s.RemoveOwnPassword(ctx, a.ID, Proof{Current: "not my password"}); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password: %v, want ErrWrongPassword", err)
	}
	bad := &proof{}
	if _, err := s.RemoveOwnPassword(ctx, a.ID, Proof{ConsumePasskeyProof: bad.consume}); !errors.Is(err, ErrReauthRequired) || bad.used != 1 {
		t.Fatalf("invalid passkey confirmation: %v (used %d)", err, bad.used)
	}
	if passwordHash(t, db, a.ID) == "" || validSessions(t, db, a.ID) != 1 {
		t.Fatal("a refused removal changed the account")
	}
	good := &proof{valid: true}
	res, err := s.RemoveOwnPassword(ctx, a.ID, Proof{ConsumePasskeyProof: good.consume})
	if err != nil || res.Reauth != auth.ProofPasskey || res.SessionsEnded != 1 || good.used != 1 {
		t.Fatalf("remove with a passkey confirmation: %+v, %v (used %d)", res, err, good.used)
	}
	if passwordHash(t, db, a.ID) != "" || validSessions(t, db, a.ID) != 0 {
		t.Fatal("password still set, or sessions left alive")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM account_audit_log WHERE action = ? AND detail LIKE '%reauth=passkey%sessions_ended=1%'`, ActionRemovePassword); n != 1 {
		t.Errorf("audit rows for the removal = %d, want 1", n)
	}
	// A confirmation is not used up on a removal that could never happen.
	unused := &proof{valid: true}
	if _, err := s.RemoveOwnPassword(ctx, a.ID, Proof{ConsumePasskeyProof: unused.consume}); !errors.Is(err, ErrNoPassword) || unused.used != 0 {
		t.Fatalf("no password to remove: %v (confirmation used %d times)", err, unused.used)
	}
}

// F2 over HTTP: the password page's remove needs the current password, signs
// the other devices out and keeps this browser signed in.
func TestPasswordPageRemoveNeedsProof(t *testing.T) {
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleCaptain)
	storePasskey(t, db, a.ID, "key", false)
	addSession(t, db, "other-device", a.ID)
	authService := auth.NewService(db, auth.DefaultConfig())
	rec := httptest.NewRecorder()
	if err := authService.StartSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), a.ID); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	mux := http.NewServeMux()
	NewPasswordPage(s, authService, nil).RegisterRoutes(mux)
	remove := func(form url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, auth.PasswordRemovePath, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec
	}
	if rec := remove(nil); !strings.Contains(rec.Header().Get("Location"), "perror=reauth") {
		t.Fatalf("remove with a session only: %q", rec.Header().Get("Location"))
	}
	if rec := remove(url.Values{"current_password": {"a wrong guess"}}); !strings.Contains(rec.Header().Get("Location"), "perror=wrong") {
		t.Fatalf("remove with a wrong password: %q", rec.Header().Get("Location"))
	}
	if passwordHash(t, db, a.ID) == "" {
		t.Fatal("password removed without proof")
	}
	rec = remove(url.Values{"current_password": {goodPassword}})
	if !strings.Contains(rec.Header().Get("Location"), "done=password-removed") || passwordHash(t, db, a.ID) != "" {
		t.Fatalf("remove with the password: %q", rec.Header().Get("Location"))
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid`, a.ID); n != 1 {
		t.Errorf("%d valid sessions, want only this browser's new one", n)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	if u, ok := authService.SignedInUser(r); !ok || u.ID != a.ID {
		t.Error("this browser is not signed in after removing its password")
	}
}

// I7: the link page's own form must work in a browser that sends no
// Sec-Fetch-Site (plain http to a host that is not localhost). With
// "Referrer-Policy: no-referrer" such a browser posts "Origin: null", which
// the cross-origin guard refuses; with same-origin it posts the page's origin.
func TestLinkPageFormWorksWithoutSecFetchSite(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	_, link, err := env.s.Invite(ctx, cli, "newperson", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := env.do(http.MethodGet, link.Path(), "198.51.100.9", nil)
	if got := page.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("Referrer-Policy %q, want same-origin", got)
	}
	if strings.Contains(page.Body.String(), "no-referrer") || !strings.Contains(page.Body.String(), `<meta name="referrer" content="same-origin">`) {
		t.Error("the page's meta referrer policy is not same-origin")
	}
	// What a browser sends under no-referrer: refused by the guard. This is
	// the failure the policy change avoids.
	if rec := env.do(http.MethodPost, link.Path(), "198.51.100.9", passwordForm(goodPassword), "Origin", "null"); rec.Code != http.StatusForbidden {
		t.Fatalf("Origin: null got status %d; if the guard now accepts it this test's premise has changed", rec.Code)
	}
	// What it sends under same-origin: the page's own origin. httptest's
	// requests are for example.com.
	rec := env.do(http.MethodPost, link.Path(), "198.51.100.9", passwordForm(goodPassword), "Origin", "http://example.com")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("form post with the page's own origin: status %d, body %s", rec.Code, rec.Body.String())
	}
	if passwordHash(t, env.db, link.UserID) == "" {
		t.Fatal("the link was not used")
	}
}

// F3: what the audit log stores for a sign-in is bounded.
func TestAuditDetailIsBounded(t *testing.T) {
	s, db := newTestService(t)
	if err := s.RecordSignIn(context.Background(), 1, strings.Repeat("u", 5000), "password", strings.Repeat("9", 5000)); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, `SELECT MAX(LENGTH(detail)) FROM account_audit_log`); n > maxAuditDetail {
		t.Errorf("stored detail is %d bytes, limit %d", n, maxAuditDetail)
	}
	if n := countRows(t, db, `SELECT MAX(MAX(LENGTH(actor)), MAX(LENGTH(target_username))) FROM account_audit_log`); n > maxAuditName {
		t.Errorf("stored name is %d bytes, limit %d", n, maxAuditName)
	}
}
