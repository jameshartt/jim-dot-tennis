// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

func TestValidatePasswordPolicy(t *testing.T) {
	for _, c := range []struct {
		password, username, code string
	}{
		{"elevenchars", "", PolicyTooShort},           // 11
		{"twelve chars", "", ""},                      // exactly 12, spaces count
		{"all lower case words", "", ""},              // no composition rules
		{"ÉÉÉÉÉÉÉÉÉÉÉé", "", ""},                      // 12 characters, more bytes
		{strings.Repeat("ab", 37), "", PolicyTooLong}, // 74 bytes
		{"Password1234", "", PolicyCommon},            // case does not matter
		{"password 1234", "", PolicyCommon},           // nor spaces
		{"tennisclub123", "", PolicyCommon},
		{"correct horse battery staple", "", PolicyCommon},
		{"my name is samantha ok", "Samantha", PolicyUsername},
		{"aaaaaaaaaaaaaa", "", PolicyRepeated},
	} {
		err := ValidatePassword(c.password, c.username)
		var pe *PasswordPolicyError
		switch {
		case c.code == "" && err != nil:
			t.Errorf("%q refused: %v", c.password, err)
		case c.code != "" && (!errors.As(err, &pe) || pe.Code != c.code):
			t.Errorf("%q: %v, want %s", c.password, err, c.code)
		case c.code != "":
			if !errors.Is(err, ErrPasswordPolicy) || PasswordPolicyMessage(c.code) == "" {
				t.Errorf("%q: error %v does not wrap ErrPasswordPolicy or has no message", c.password, err)
			}
			if msg, ok := PolicyMessage(err); !ok || !strings.HasSuffix(err.Error(), msg) {
				t.Errorf("%q: message %q not in %q", c.password, msg, err.Error())
			}
		}
	}
	// The embedded list loaded, and every entry is refused one way or another.
	if len(commonPasswords) < 50 {
		t.Fatalf("only %d common passwords loaded", len(commonPasswords))
	}
	for key := range commonPasswords {
		if ValidatePassword(key, "") == nil {
			t.Errorf("common password %q accepted", key)
		}
	}
}

func TestHashCostAndRehashOnSignIn(t *testing.T) {
	svc := newTestService(t, DefaultConfig())
	old, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('old', ?, 'captain')`, string(old)); err != nil {
		t.Fatal(err)
	}
	stored := func() string {
		var h string
		if err := svc.db.Get(&h, `SELECT password_hash FROM users WHERE username = 'old'`); err != nil {
			t.Fatal(err)
		}
		return h
	}
	if !NeedsRehash(string(old)) || NeedsRehash(mustHash(t, "anything")) || !NeedsRehash("not a hash") {
		t.Fatal("NeedsRehash is wrong")
	}
	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.RemoteAddr = "192.0.2.9:1"
		return r
	}

	// A wrong password changes nothing.
	if _, err := svc.Login("old", "not the password", req()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if stored() != string(old) {
		t.Fatal("hash changed by a failed sign-in")
	}
	// A successful sign-in upgrades the hash to BcryptCost; the password still works.
	if _, err := svc.Login("old", testPassword, req()); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	upgraded := stored()
	if cost, err := bcrypt.Cost([]byte(upgraded)); err != nil || cost != BcryptCost {
		t.Fatalf("hash cost after sign-in: %d (%v), want %d", cost, err, BcryptCost)
	}
	if _, err := svc.Login("old", testPassword, req()); err != nil {
		t.Fatalf("sign in after upgrade: %v", err)
	}
	if stored() != upgraded {
		t.Fatal("an up-to-date hash was replaced")
	}
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func setAdminPasskeysRequired(t *testing.T, env *passkeyEnv, on bool) {
	t.Helper()
	v := "false"
	if on {
		v = "true"
	}
	if err := repository.NewSettingsRepository(env.svc.db).SetString(t.Context(), repository.SettingRequireAdminPasskeys, v); err != nil {
		t.Fatal(err)
	}
}

// loginError signs in by password in a fresh browser and returns the error
// text on the page ("" when it signed in).
func loginError(t *testing.T, env *passkeyEnv, ip, username, password string) string {
	t.Helper()
	b := env.browser(ip)
	rec := b.postForm("/login", url.Values{"username": {username}, "password": {password}})
	if b.signedIn() {
		return ""
	}
	body := rec.Body.String()
	i := strings.Index(body, `<div class="alert alert-danger">`)
	if i < 0 {
		t.Fatalf("sign-in as %s neither worked nor showed an error: %d", username, rec.Code)
	}
	end := strings.Index(body[i:], "</div>")
	return strings.TrimSpace(body[i+len(`<div class="alert alert-danger">`) : i+end])
}

func TestAdminPasskeysRequiredAtPasswordSignIn(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("root", models.RoleAdmin)
	env.addUser("cap", models.RoleCaptain)
	admin := env.browser("198.51.100.10")
	admin.passwordSignIn("root")
	a := newAuthenticator(t)
	cred := admin.enrol(a, "root's phone", false)

	setAdminPasskeysRequired(t, env, true)

	// The administrator's right password fails exactly like a wrong one and
	// like an unknown username: nothing tells a stranger which usernames
	// are administrators.
	right := loginError(t, env, "198.51.100.11", "root", testPassword)
	wrong := loginError(t, env, "198.51.100.12", "root", "not the password")
	nobody := loginError(t, env, "198.51.100.13", "nobody", testPassword)
	if right == "" || right != wrong || right != nobody {
		t.Fatalf("messages differ or sign-in worked: right=%q wrong=%q nobody=%q", right, wrong, nobody)
	}
	// Other roles are unaffected; the administrator uses the passkey.
	if msg := loginError(t, env, "198.51.100.14", "cap", testPassword); msg != "" {
		t.Fatalf("captain refused: %q", msg)
	}
	b := env.browser("198.51.100.15")
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)
	if !b.signedIn() {
		t.Fatal("administrator could not sign in with a passkey")
	}
	// The login page says so to everyone, the same for every visitor.
	if page := env.browser("198.51.100.16").do(http.MethodGet, "/login", "", nil).Body.String(); !strings.Contains(page, "data-admin-passkeys-required") {
		t.Error("login page does not say administrators use passkeys")
	}

	// With no usable public URL no passkey can work, so the requirement does
	// not apply (otherwise clearing the URL would lock every administrator out).
	env.src.set("")
	if msg := loginError(t, env, "198.51.100.17", "root", testPassword); msg != "" {
		t.Fatalf("administrator refused a password with passkeys unavailable: %q", msg)
	}
	if page := env.browser("198.51.100.18").do(http.MethodGet, "/login", "", nil).Body.String(); strings.Contains(page, "passkey") {
		t.Error("login page mentions passkeys where they are unavailable")
	}

	// Turned off (as the command line does), passwords work again at once.
	env.src.set(testOrigin)
	setAdminPasskeysRequired(t, env, false)
	if msg := loginError(t, env, "198.51.100.19", "root", testPassword); msg != "" {
		t.Fatalf("administrator refused with the requirement off: %q", msg)
	}
}

func TestPasswordSignInOffersAPasskey(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("cap", models.RoleCaptain)
	b := env.browser("198.51.100.30")
	rec := b.postForm("/login", url.Values{"username": {"cap"}, "password": {testPassword}, "redirect": {"/admin/league/players"}})
	want := SignInMethodsPath + "?prompt=1&next=" + url.QueryEscape("/admin/league/players")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("password sign-in without a passkey: %d %q, want %q", rec.Code, rec.Header().Get("Location"), want)
	}
	page := b.do(http.MethodGet, want, "", nil).Body.String()
	for _, s := range []string{"data-passkey-prompt", "data-passkey-prompt-dismiss", `action="` + PasskeyPromptDismissPath + `"`, `data-signin-continue`} {
		if !strings.Contains(page, s) {
			t.Errorf("prompt page lacks %s", s)
		}
	}
	// "Not now" carries on and is remembered in this browser.
	rec = b.postForm(PasskeyPromptDismissPath, url.Values{"next": {"/admin/league/players"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/league/players" {
		t.Fatalf("dismiss: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = b.postForm("/login", url.Values{"username": {"cap"}, "password": {testPassword}})
	if loc := rec.Header().Get("Location"); loc != "/admin/league" {
		t.Fatalf("sign-in after 'Not now': %q", loc)
	}
	// An account with a passkey is not prompted.
	b.enrol(newAuthenticator(t), "phone", false)
	c := env.browser("198.51.100.31")
	rec = c.postForm("/login", url.Values{"username": {"cap"}, "password": {testPassword}})
	if loc := rec.Header().Get("Location"); loc != "/admin/league" {
		t.Fatalf("sign-in with a passkey on the account: %q", loc)
	}
	// Nor anyone where passkeys are unavailable.
	env.addUser("cap2", models.RoleCaptain)
	env.src.set("")
	rec = env.browser("198.51.100.32").postForm("/login", url.Values{"username": {"cap2"}, "password": {testPassword}})
	if loc := rec.Header().Get("Location"); loc != "/admin/league" {
		t.Fatalf("sign-in with passkeys unavailable: %q", loc)
	}
}

func TestAdminCannotRemoveLastPasskeyWhenRequired(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("root", models.RoleAdmin)
	b := env.browser("198.51.100.40")
	b.passwordSignIn("root")
	b.enrol(newAuthenticator(t), "only key", false)
	setAdminPasskeysRequired(t, env, true)
	// The account still has a password, so this is not its last sign-in
	// method, but it is its last passkey.
	rec := b.postForm("/account/passkeys/1/delete", pwForm)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=adminpasskey") {
		t.Fatalf("delete last admin passkey: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if env.count(`SELECT COUNT(*) FROM webauthn_credentials`) != 1 {
		t.Fatal("passkey removed")
	}
	page := b.do(http.MethodGet, SignInMethodsPath+"?error=adminpasskey", "", nil).Body.String()
	if !strings.Contains(page, "data-signin-error") || !strings.Contains(page, "data-password-admin-unusable") {
		t.Error("sign-in methods page does not explain the requirement")
	}
}
