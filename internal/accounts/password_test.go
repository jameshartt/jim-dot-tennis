// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// storePasskey stores a passkey for userID (blocked: with a clone warning).
func storePasskey(t *testing.T, db *database.DB, userID int64, name string, blocked bool) int64 {
	t.Helper()
	cred := newPasskey(userID, name)
	if err := repository.InsertWebAuthnCredential(context.Background(), db, cred); err != nil {
		t.Fatal(err)
	}
	if blocked {
		if _, err := db.Exec(`UPDATE webauthn_credentials SET clone_warning_at = CURRENT_TIMESTAMP WHERE id = ?`, cred.ID); err != nil {
			t.Fatal(err)
		}
	}
	return cred.ID
}

// proof is a fake passkey confirmation that counts how often it is used.
type proof struct {
	valid bool
	used  int
}

func (p *proof) consume() bool { p.used++; return p.valid }

func TestSetOwnPasswordNeedsReauthentication(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleCaptain)
	addSession(t, db, "other-device", a.ID)

	// A session alone is not enough.
	if _, err := s.SetOwnPassword(ctx, a.ID, PasswordChange{New: "a brand new secret"}); !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("no proof: %v, want ErrReauthRequired", err)
	}
	if _, err := s.SetOwnPassword(ctx, a.ID, PasswordChange{Current: "not my password", New: "a brand new secret"}); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current password: %v", err)
	}
	// A refused password does not use up a passkey confirmation.
	p := &proof{valid: true}
	for _, bad := range []struct{ pw, code string }{
		{"too short!", auth.PolicyTooShort},
		{"password1234", auth.PolicyCommon},
		{goodPassword, auth.PolicySame},
		{"alice is my name ok", auth.PolicyUsername},
	} {
		_, err := s.SetOwnPassword(ctx, a.ID, PasswordChange{ConsumePasskeyProof: p.consume, New: bad.pw})
		var pe *auth.PasswordPolicyError
		if !errors.As(err, &pe) || pe.Code != bad.code {
			t.Errorf("%q: %v, want policy %s", bad.pw, err, bad.code)
		}
	}
	if p.used != 0 {
		t.Fatalf("a refused password used the passkey confirmation %d time(s)", p.used)
	}
	// An invalid (expired, used, other session) confirmation is refused.
	if _, err := s.SetOwnPassword(ctx, a.ID, PasswordChange{ConsumePasskeyProof: (&proof{}).consume, New: "a brand new secret"}); !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("invalid proof: %v", err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, a.ID), goodPassword) {
		t.Fatal("password changed without re-authentication")
	}

	// A passkey confirmation works and is used exactly once; a change ends
	// every session (the page then starts a new one for this browser).
	res, err := s.SetOwnPassword(ctx, a.ID, PasswordChange{ConsumePasskeyProof: p.consume, New: "a brand new secret"})
	if err != nil || res.Added || res.Reauth != "passkey" || res.SessionsEnded != 1 || p.used != 1 {
		t.Fatalf("change with passkey proof: %+v, %v (proof used %d)", res, err, p.used)
	}
	if validSessions(t, db, a.ID) != 0 {
		t.Fatal("other sessions survived a password change")
	}
	// And the current password works too.
	res, err = s.SetOwnPassword(ctx, a.ID, PasswordChange{Current: "a brand new secret", New: "yet another fine secret"})
	if err != nil || res.Reauth != "password" {
		t.Fatalf("change with current password: %+v, %v", res, err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, a.ID), "yet another fine secret") {
		t.Fatal("password not changed")
	}
}

func TestAddPasswordToPasskeyOnlyAccount(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	_, link, err := s.Invite(ctx, cli, "keyonly", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Passkey: newPasskey(link.UserID, "phone")}); err != nil {
		t.Fatal(err)
	}
	addSession(t, db, "phone-session", link.UserID)
	// Typing a "current password" means nothing for an account without one.
	if _, err := s.SetOwnPassword(ctx, link.UserID, PasswordChange{Current: "anything at all", New: "a brand new secret"}); !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("add without passkey proof: %v", err)
	}
	p := &proof{valid: true}
	res, err := s.SetOwnPassword(ctx, link.UserID, PasswordChange{ConsumePasskeyProof: p.consume, New: "a brand new secret"})
	if err != nil || !res.Added || res.SessionsEnded != 0 {
		t.Fatalf("add password: %+v, %v", res, err)
	}
	if validSessions(t, db, link.UserID) != 1 {
		t.Fatal("adding a password ended sessions")
	}
}

func TestRemovePasswordNeverLeavesNoWayIn(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleCaptain)

	if err := s.RemoveOwnPassword(ctx, a.ID); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("password only: %v, want ErrLastSignInMethod", err)
	}
	// A blocked passkey cannot sign in, so it does not count.
	blocked := storePasskey(t, db, a.ID, "blocked", true)
	if err := s.RemoveOwnPassword(ctx, a.ID); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("password and a blocked passkey: %v, want ErrLastSignInMethod", err)
	}
	storePasskey(t, db, a.ID, "good", false)
	if err := s.RemoveOwnPassword(ctx, a.ID); err != nil {
		t.Fatalf("remove with a usable passkey: %v", err)
	}
	if passwordHash(t, db, a.ID) != "" {
		t.Fatal("password still set")
	}
	if err := s.RemoveOwnPassword(ctx, a.ID); !errors.Is(err, ErrNoPassword) {
		t.Fatalf("remove again: %v, want ErrNoPassword", err)
	}
	_ = blocked

	// Racing: removing the password and the last passkey at once leaves one.
	b := mustCreate(t, s, "bob", models.RoleCaptain)
	key := storePasskey(t, db, b.ID, "bob-key", false)
	creds := repository.NewWebAuthnCredentialRepository(db)
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		var errPw, errKey error
		wg.Add(2)
		go func() { defer wg.Done(); errPw = s.RemoveOwnPassword(ctx, b.ID) }()
		go func() { defer wg.Done(); errKey = creds.Delete(ctx, b.ID, key) }()
		wg.Wait()
		n, err := repository.CountSignInMethods(ctx, db, b.ID)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || (errPw == nil) == (errKey == nil) {
			t.Fatalf("round %d: %d methods left (remove password: %v, delete passkey: %v)", round, n, errPw, errKey)
		}
		// Put things back for the next round.
		if errPw == nil {
			if _, err := db.Exec(`UPDATE users SET password_hash = 'x' WHERE id = ?`, b.ID); err != nil {
				t.Fatal(err)
			}
		} else {
			key = storePasskey(t, db, b.ID, "bob-key-again-"+string(rune('a'+round)), false)
		}
	}
}

func TestAdminPasskeyRequirement(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	if err := s.SetAdminPasskeysRequired(ctx, cli, true); !errors.Is(err, ErrNoActiveAdmins) {
		t.Fatalf("enable with no administrators: %v", err)
	}
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	bob := mustCreate(t, s, "bob", models.RoleAdmin)
	carol := mustCreate(t, s, "carol", models.RoleCaptain)
	_ = carol

	// Cannot be enabled until every active administrator has a usable passkey;
	// the error names who is missing one, and nothing is stored.
	storePasskey(t, db, alice.ID, "alice-key", false)
	storePasskey(t, db, bob.ID, "bob-blocked", true)
	err := s.SetAdminPasskeysRequired(ctx, cli, true)
	var missing *AdminsWithoutPasskeyError
	if !errors.As(err, &missing) || strings.Join(missing.Usernames, ",") != "bob" {
		t.Fatalf("enable with bob lacking a usable passkey: %v", err)
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, db); on {
		t.Fatal("setting stored although refused")
	}
	// A disabled administrator does not block it.
	if _, err := s.SetActive(ctx, cli, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdminPasskeysRequired(ctx, cli, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	st, err := s.AdminPasskeyStatus(ctx)
	if err != nil || !st.Required || len(st.AdminsWithoutPasskey) != 0 {
		t.Fatalf("status: %+v, %v", st, err)
	}

	// Promoting a passkey-less account is refused; one with a passkey is fine.
	if _, err := s.SetRole(ctx, cli, carol.ID, models.RoleAdmin); !errors.Is(err, ErrAdminNeedsPasskey) {
		t.Fatalf("promote carol without a passkey: %v", err)
	}
	if a, _ := s.Get(ctx, carol.ID); a.Role != models.RoleCaptain {
		t.Fatal("carol was promoted")
	}
	storePasskey(t, db, carol.ID, "carol-key", false)
	if _, err := s.SetRole(ctx, cli, carol.ID, models.RoleAdmin); err != nil {
		t.Fatalf("promote carol with a passkey: %v", err)
	}
	// Re-setting the role of an existing administrator is not a promotion.
	if _, err := s.SetRole(ctx, cli, alice.ID, models.RoleAdmin); err != nil {
		t.Fatalf("alice admin -> admin: %v", err)
	}

	// An administrator cannot remove their last usable passkey.
	creds := repository.NewWebAuthnCredentialRepository(db)
	var aliceKey int64
	if err := db.Get(&aliceKey, `SELECT id FROM webauthn_credentials WHERE user_id = ?`, alice.ID); err != nil {
		t.Fatal(err)
	}
	if err := creds.Delete(ctx, alice.ID, aliceKey); !errors.Is(err, repository.ErrAdminNeedsPasskey) {
		t.Fatalf("delete alice's last passkey: %v", err)
	}
	second := storePasskey(t, db, alice.ID, "alice-second", false)
	if err := creds.Delete(ctx, alice.ID, second); err != nil {
		t.Fatalf("delete a spare passkey: %v", err)
	}

	// Links: an administrator cannot set a password with one (the link stays
	// usable); a passkey works. A captain can still use a password.
	link, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "a new admin secret"}); !errors.Is(err, ErrAdminPasskeyRequired) {
		t.Fatalf("admin redeems with a password: %v", err)
	}
	if _, err := s.LookupLink(ctx, link.Token); err != nil {
		t.Fatalf("link used up by a refused redemption: %v", err)
	}
	// Recovery for a locked-out administrator: the link plus a new passkey,
	// even removing the old ones.
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Passkey: newPasskey(alice.ID, "alice-new-phone"), RemoveOtherMethods: true}); err != nil {
		t.Fatalf("admin redeems with a passkey: %v", err)
	}
	dave, inviteLink, err := s.Invite(ctx, cli, "dave", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, inviteLink.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatalf("captain redeems with a password: %v", err)
	}
	_ = dave
	// Where passkeys cannot be used at all, the requirement does not apply.
	link, err = s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "a new admin secret", PasskeysUnavailable: true}); err != nil {
		t.Fatalf("admin redeems with a password where passkeys are unavailable: %v", err)
	}

	// Turning it off always works (the operator's way out).
	if err := s.SetAdminPasskeysRequired(ctx, cli, false); err != nil {
		t.Fatal(err)
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, db); on {
		t.Fatal("still on")
	}
	if _, err := s.SetRole(ctx, cli, dave.ID, models.RoleAdmin); err != nil {
		t.Fatalf("promote without passkey once the requirement is off: %v", err)
	}
}

// TestEnableRequirementRacesPasskeyRemoval: turning the requirement on and an
// administrator removing their only passkey at the same moment never end
// with the requirement on and that administrator without a passkey.
func TestEnableRequirementRacesPasskeyRemoval(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	creds := repository.NewWebAuthnCredentialRepository(db)
	for round := 0; round < 10; round++ {
		key := storePasskey(t, db, alice.ID, "k"+string(rune('a'+round)), false)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = s.SetAdminPasskeysRequired(ctx, cli, true) }()
		go func() { defer wg.Done(); _ = creds.Delete(ctx, alice.ID, key) }()
		wg.Wait()
		on, err := repository.AdminPasskeysRequired(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		lacks, err := repository.ActiveAdminsWithoutPasskey(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if on && len(lacks) > 0 {
			t.Fatalf("round %d: requirement on and %v without a passkey", round, lacks)
		}
		if err := s.SetAdminPasskeysRequired(ctx, cli, false); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM webauthn_credentials WHERE user_id = ?`, alice.ID); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPasswordPageChangeKeepsThisBrowserSignedIn: the HTTP side. Changing
// signs every other session out and keeps this browser signed in; GET goes to
// the sign-in methods page; a wrong current password is reported.
func TestPasswordPageChangeKeepsThisBrowserSignedIn(t *testing.T) {
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleCaptain)
	addSession(t, db, "other-device", a.ID)

	authService := auth.NewService(db, auth.DefaultConfig())
	rec := httptest.NewRecorder()
	if err := authService.StartSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), a.ID); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	withCookies := func(r *http.Request) *http.Request {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	mux := http.NewServeMux()
	NewPasswordPage(s, authService, nil).RegisterRoutes(mux)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, withCookies(httptest.NewRequest(http.MethodGet, auth.PasswordPath, nil)))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), auth.SignInMethodsPath) {
		t.Fatalf("GET: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, withCookies(r))
		return rec
	}
	change := func(current, next, confirm string) *httptest.ResponseRecorder {
		return post(auth.PasswordPath, url.Values{"current_password": {current}, "new_password": {next}, "new_password_confirm": {confirm}})
	}
	for _, c := range []struct{ current, next, confirm, want string }{
		{"wrong password", "a brand new secret", "a brand new secret", "perror=wrong"},
		{goodPassword, "a brand new secret", "a different secret", "perror=mismatch"},
		{goodPassword, "short", "short", "perror=short"},
		{"", "a brand new secret", "a brand new secret", "perror=reauth"},
	} {
		rec := change(c.current, c.next, c.confirm)
		if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), c.want) {
			t.Errorf("%+v: %d %q", c, rec.Code, rec.Header().Get("Location"))
		}
	}
	if rec := post(auth.PasswordRemovePath, nil); !strings.Contains(rec.Header().Get("Location"), "perror=last") {
		t.Errorf("remove the only method: %q", rec.Header().Get("Location"))
	}

	rec = change(goodPassword, "a brand new secret", "a brand new secret")
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "done=password-changed") {
		t.Fatalf("change: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	var otherValid int
	if err := db.Get(&otherValid, `SELECT COUNT(*) FROM sessions WHERE id = 'other-device' AND is_valid = TRUE`); err != nil {
		t.Fatal(err)
	}
	if otherValid != 0 {
		t.Fatal("the other device is still signed in")
	}
	// This browser was given a fresh session that works; the old cookie does not.
	old := withCookies(httptest.NewRequest(http.MethodGet, "/", nil))
	if _, ok := authService.SignedInUser(old); ok {
		t.Fatal("the pre-change session still works")
	}
	cookies = rec.Result().Cookies()
	if u, ok := authService.SignedInUser(withCookies(httptest.NewRequest(http.MethodGet, "/", nil))); !ok || u.ID != a.ID {
		t.Fatal("this browser is not signed in after changing its password")
	}

	// Wrong current passwords are throttled per address.
	for i := 0; i < currentPasswordFailureLimit+1; i++ {
		change("wrong again", "another brand new one", "another brand new one")
	}
	if rec := change("a brand new secret", "another brand new one", "another brand new one"); !strings.Contains(rec.Header().Get("Location"), "perror=wrong") {
		t.Fatalf("throttled change went through: %q", rec.Header().Get("Location"))
	}
}
