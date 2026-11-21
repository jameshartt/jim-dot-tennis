// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/models"
)

// proofRequest is a request from browser b presenting proof token.
func proofRequest(b *browser, token string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, PasswordPath, nil)
	r.RemoteAddr = b.ip + ":40000"
	if s := b.cookies["session_token"]; s != "" {
		r.AddCookie(&http.Cookie{Name: "session_token", Value: s})
	}
	if token != "" {
		r.AddCookie(&http.Cookie{Name: reauthCookie, Value: token})
	}
	return r
}

// reauth runs a passkey re-authentication in b and returns the proof token.
func reauth(t *testing.T, b *browser, a *softAuthenticator, cred *softCredential) string {
	t.Helper()
	begin := b.postJSON(ReauthBeginPath, []byte("{}"))
	wantStatus(t, begin, http.StatusOK)
	finish := b.postJSON(ReauthFinishPath, a.get(begin.Body.Bytes(), cred))
	wantStatus(t, finish, http.StatusOK)
	var out map[string]string
	if err := json.Unmarshal(finish.Body.Bytes(), &out); err != nil || !strings.HasPrefix(out["redirect"], SignInMethodsPath) {
		t.Fatalf("finish response: %s", finish.Body.String())
	}
	token := b.cookies[reauthCookie]
	if token == "" {
		t.Fatal("no proof cookie")
	}
	return token
}

func TestReauthProofIsFreshSingleUseAndBound(t *testing.T) {
	env := newPasskeyEnv(t)
	id := env.addUser("alice", models.RoleCaptain)
	b := env.browser("198.51.100.50")
	b.passwordSignIn("alice")
	a := newAuthenticator(t)
	cred := b.enrol(a, "alice's phone", true)

	// A fresh passkey sign-in is not proof: only a re-authentication mints one.
	fresh := env.browser("198.51.100.51")
	wantStatus(t, fresh.passkeySignIn(a, cred), http.StatusOK)
	if fresh.cookies[reauthCookie] != "" || env.pk.HasReauthProof(proofRequest(fresh, ""), id) {
		t.Fatal("a sign-in counts as re-authentication")
	}

	// The begin options name only alice's passkeys.
	begin := b.postJSON(ReauthBeginPath, []byte("{}"))
	wantStatus(t, begin, http.StatusOK)
	var opts requestOptions
	if err := json.Unmarshal(begin.Body.Bytes(), &opts); err != nil || len(opts.PublicKey.AllowCredentials) != 1 || opts.PublicKey.AllowCredentials[0].ID != b64(cred.id) {
		t.Fatalf("options: %s", begin.Body.String())
	}
	finish := b.postJSON(ReauthFinishPath, a.get(begin.Body.Bytes(), cred))
	wantStatus(t, finish, http.StatusOK)
	token := b.cookies[reauthCookie]

	// Bound to the user and to this session.
	if !env.pk.HasReauthProof(proofRequest(b, token), id) {
		t.Fatal("proof not seen")
	}
	if env.pk.HasReauthProof(proofRequest(b, token), id+1) {
		t.Fatal("proof counts for another user")
	}
	other := env.browser("198.51.100.52")
	other.passwordSignIn("alice")
	if env.pk.HasReauthProof(proofRequest(other, token), id) {
		t.Fatal("proof counts in another session of the same user")
	}
	// Single use: the first consume succeeds, a replay fails.
	if !env.pk.ConsumeReauthProof(httptest.NewRecorder(), proofRequest(b, token), id) {
		t.Fatal("consume a valid proof")
	}
	if env.pk.ConsumeReauthProof(httptest.NewRecorder(), proofRequest(b, token), id) {
		t.Fatal("proof used twice")
	}
	// Consumed from the wrong session it fails, and is gone.
	token = reauth(t, b, a, cred)
	if env.pk.ConsumeReauthProof(httptest.NewRecorder(), proofRequest(other, token), id) {
		t.Fatal("proof consumed from another session")
	}
	if env.pk.ConsumeReauthProof(httptest.NewRecorder(), proofRequest(b, token), id) {
		t.Fatal("proof survived a wrong-session attempt")
	}
	// It expires.
	token = reauth(t, b, a, cred)
	env.advance(ReauthProofLifetime + time.Second)
	if env.pk.HasReauthProof(proofRequest(b, token), id) || env.pk.ConsumeReauthProof(httptest.NewRecorder(), proofRequest(b, token), id) {
		t.Fatal("expired proof accepted")
	}
	// A consume clears the cookie.
	token = reauth(t, b, a, cred)
	rec := httptest.NewRecorder()
	env.pk.ConsumeReauthProof(rec, proofRequest(b, token), id)
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == reauthCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("proof cookie not cleared")
	}
}

func TestReauthCeremonyIsBound(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("alice", models.RoleCaptain)
	b := env.browser("198.51.100.60")
	b.passwordSignIn("alice")
	a := newAuthenticator(t)
	cred := b.enrol(a, "alice's phone", true)

	// Finished from another session of the same user (with this browser's
	// ceremony cookie): refused.
	begin := b.postJSON(ReauthBeginPath, []byte("{}"))
	wantStatus(t, begin, http.StatusOK)
	other := env.browser("198.51.100.61")
	other.passwordSignIn("alice")
	other.cookies[ceremonyCookie] = b.cookies[ceremonyCookie]
	wantStatus(t, other.postJSON(ReauthFinishPath, a.get(begin.Body.Bytes(), cred)), http.StatusBadRequest)
	if other.cookies[reauthCookie] != "" {
		t.Fatal("proof minted for another session")
	}

	// A sign-in ceremony cannot finish a re-authentication.
	options := b.beginSignIn("")
	wantStatus(t, b.postJSON(ReauthFinishPath, a.get(options, cred)), http.StatusBadRequest)

	// Another user's passkey is rejected.
	env.addUser("bob", models.RoleCaptain)
	bob := env.browser("198.51.100.62")
	bob.passwordSignIn("bob")
	bobAuth := newAuthenticator(t)
	bobCred := bob.enrol(bobAuth, "bob's phone", true)
	begin = b.postJSON(ReauthBeginPath, []byte("{}"))
	wantStatus(t, begin, http.StatusOK)
	wantStatus(t, b.postJSON(ReauthFinishPath, bobAuth.assertion(mustChallenge(t, begin.Body.Bytes()), bobCred, 50)), http.StatusUnauthorized)
	if b.cookies[reauthCookie] != "" {
		t.Fatal("proof minted with another user's passkey")
	}

	// An account with no passkey has nothing to confirm with.
	env.addUser("carol", models.RoleCaptain)
	carol := env.browser("198.51.100.63")
	carol.passwordSignIn("carol")
	wantStatus(t, carol.postJSON(ReauthBeginPath, []byte("{}")), http.StatusConflict)

	// Signed out: refused.
	anon := env.browser("198.51.100.64")
	if rec := anon.postJSON(ReauthBeginPath, []byte("{}")); rec.Code == http.StatusOK {
		t.Fatal("re-authentication without a session")
	}
}

func TestSignInMethodsPasswordSection(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("alice", models.RoleCaptain)
	b := env.browser("198.51.100.70")
	b.passwordSignIn("alice")

	// Password only: change with the current password; can't remove.
	page := b.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	for _, s := range []string{`data-password-form="change"`, `id="current_password"`, `data-password-last-method`, `minlength="12"`} {
		if !strings.Contains(page, s) {
			t.Errorf("password-only page lacks %s", s)
		}
	}
	if strings.Contains(page, "data-password-remove") || strings.Contains(page, "data-passkey-reauth") {
		t.Error("password-only page offers removal or passkey confirmation")
	}

	// With a passkey: removable, and confirmable with the passkey.
	a := newAuthenticator(t)
	cred := b.enrol(a, "phone", true)
	page = b.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	for _, s := range []string{`data-password-remove`, `data-passkey-reauth`, `action="/account/password/remove"`} {
		if !strings.Contains(page, s) {
			t.Errorf("page lacks %s", s)
		}
	}
	reauth(t, b, a, cred)
	page = b.do(http.MethodGet, SignInMethodsPath+"?done=verified", "", nil).Body.String()
	if !strings.Contains(page, "data-password-confirmed") || strings.Contains(page, `id="current_password"`) || !strings.Contains(page, "data-signin-notice") {
		t.Error("confirmed page should drop the current password field")
	}
	page = b.do(http.MethodGet, SignInMethodsPath+"?perror=common", "", nil).Body.String()
	if !strings.Contains(page, "data-password-error") || !strings.Contains(page, "too common") {
		t.Error("policy error not shown")
	}
}
