// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"

	"golang.org/x/crypto/bcrypt"
)

const (
	testOrigin   = "https://tennis.example.org"
	testRPID     = "tennis.example.org"
	testPassword = "correct horse battery"
)

// fakePublicURL is a settable stand-in for config.ClubSettings.
type fakePublicURL struct {
	mu  sync.Mutex
	url string // "" means not configured
}

func (f *fakePublicURL) set(u string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.url = u
}

func (f *fakePublicURL) ResolvePublicURL() (config.PublicURL, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url == "" {
		return config.PublicURL{}, false, nil
	}
	u, err := url.Parse(f.url)
	if err != nil {
		return config.PublicURL{}, false, err
	}
	return config.PublicURL{Origin: u.Scheme + "://" + u.Host, Base: f.url, Scheme: u.Scheme, Hostname: u.Hostname()}, true, nil
}

// passkeyEnv is an auth service with the passkey routes mounted behind the
// same cross-origin guard the server uses.
type passkeyEnv struct {
	t       *testing.T
	svc     *Service
	pk      *Passkeys
	src     *fakePublicURL
	handler http.Handler
	mu      sync.Mutex
	clock   time.Time
	events  []string
	// keepQuarantine: leave a passkey added by enrol quarantined (it cannot
	// confirm changes for a while). Off by default, so the many tests that
	// enrol a passkey and then use it behave as if it had been there a while.
	keepQuarantine bool
}

func (e *passkeyEnv) now() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.clock
}

func (e *passkeyEnv) advance(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clock = e.clock.Add(d)
}

func newPasskeyEnv(t *testing.T) *passkeyEnv {
	t.Helper()
	t.Setenv(config.EnvAppEnv, "")
	t.Setenv(config.EnvDevMode, "")
	t.Setenv("TEMPLATE_RELOAD", "")
	svc := newTestService(t, DefaultConfig())
	env := &passkeyEnv{t: t, svc: svc, src: &fakePublicURL{url: testOrigin}, clock: time.Now()}
	env.pk = NewPasskeys(svc, env.src, func() string { return "Example Tennis Club" })
	env.pk.now = env.now
	env.pk.SetAudit(func(_ context.Context, actor *models.User, action, detail string) {
		env.mu.Lock()
		defer env.mu.Unlock()
		env.events = append(env.events, actor.Username+" "+action+" "+detail)
	})

	templateDir := filepath.Join(filepath.Dir(findMigrationsPath(t)), "templates")
	mux := http.NewServeMux()
	loginHandler := NewHandler(svc, templateDir, "/admin/league")
	loginHandler.SetPasskeys(env.pk)
	loginHandler.RegisterRoutes(mux)
	NewPasskeyHandler(env.pk, templateDir, "/admin/league").RegisterRoutes(mux, NewMiddleware(svc, nil, nil))
	env.handler = CrossOriginGuard(mux)
	return env
}

// addUser inserts an account with testPassword and returns its id.
func (e *passkeyEnv) addUser(username string, role models.Role) int64 {
	e.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		e.t.Fatal(err)
	}
	res, err := e.svc.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, string(hash), role)
	if err != nil {
		e.t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func (e *passkeyEnv) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.svc.db.Get(&n, query, args...); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// browser is one browser profile: its own cookies and client address.
type browser struct {
	env     *passkeyEnv
	cookies map[string]string
	ip      string
	host    string
	headers map[string]string
}

func (e *passkeyEnv) browser(ip string) *browser {
	return &browser{env: e, cookies: map[string]string{}, ip: ip, host: testRPID, headers: map[string]string{}}
}

func (b *browser) do(method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	b.env.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Host = b.host
	req.RemoteAddr = b.ip + ":40000"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	b.env.handler.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return rec
}

func (b *browser) postJSON(path string, body []byte) *httptest.ResponseRecorder {
	b.env.t.Helper()
	return b.do(http.MethodPost, path, "application/json", body)
}

func (b *browser) postForm(path string, form url.Values) *httptest.ResponseRecorder {
	b.env.t.Helper()
	return b.do(http.MethodPost, path, "application/x-www-form-urlencoded", []byte(form.Encode()))
}

func (b *browser) signedIn() bool { return b.cookies["session_token"] != "" }

// passwordSignIn signs in with testPassword.
func (b *browser) passwordSignIn(username string) {
	b.env.t.Helper()
	rec := b.postForm("/login", url.Values{"username": {username}, "password": {testPassword}})
	if rec.Code != http.StatusSeeOther || !b.signedIn() {
		b.env.t.Fatalf("password sign-in as %s: status %d, body %s", username, rec.Code, rec.Body.String())
	}
}

// pwForm is a form that proves ownership with the current password.
var pwForm = url.Values{"current_password": {testPassword}}

// enrolWithPasskey adds a passkey to the signed-in account, proving
// ownership with a confirmation from the existing passkey cred of authenticator have.
func (b *browser) enrolWithPasskey(a *softAuthenticator, name string, have *softAuthenticator, cred *softCredential) *softCredential {
	b.env.t.Helper()
	reauth(b.env.t, b, have, cred)
	begin := b.postJSON("/account/passkeys/begin", []byte("{}"))
	wantStatus(b.env.t, begin, http.StatusOK)
	body, added := a.create(begin.Body.Bytes(), false)
	wantStatus(b.env.t, b.postJSON("/account/passkeys/finish?name="+url.QueryEscape(name), body), http.StatusOK)
	if !b.env.keepQuarantine {
		b.env.pk.enrolments = newEnrolmentLog()
	}
	return added
}

// enrolBody is what the page posts to begin adding a passkey: the current
// password, as proof of ownership.
var enrolBody = []byte(`{"current_password":"` + testPassword + `"}`)

// enrol adds a passkey to the signed-in account with authenticator a,
// proving ownership with testPassword.
func (b *browser) enrol(a *softAuthenticator, name string, counts bool) *softCredential {
	b.env.t.Helper()
	begin := b.postJSON("/account/passkeys/begin", enrolBody)
	if begin.Code != http.StatusOK {
		b.env.t.Fatalf("begin registration: status %d, body %s", begin.Code, begin.Body.String())
	}
	body, cred := a.create(begin.Body.Bytes(), counts)
	finish := b.postJSON("/account/passkeys/finish?name="+url.QueryEscape(name), body)
	if finish.Code != http.StatusOK {
		b.env.t.Fatalf("finish registration: status %d, body %s", finish.Code, finish.Body.String())
	}
	if !b.env.keepQuarantine {
		b.env.pk.enrolments = newEnrolmentLog()
	}
	return cred
}

// beginSignIn starts a passkey sign-in and returns the options JSON.
func (b *browser) beginSignIn(username string) []byte {
	b.env.t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username})
	rec := b.postJSON("/login/passkey/begin", body)
	if rec.Code != http.StatusOK {
		b.env.t.Fatalf("begin sign-in: status %d, body %s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

// passkeySignIn runs a whole discoverable sign-in and returns the finish response.
func (b *browser) passkeySignIn(a *softAuthenticator, cred *softCredential) *httptest.ResponseRecorder {
	b.env.t.Helper()
	return b.postJSON("/login/passkey/finish", a.get(b.beginSignIn(""), cred))
}

func newAuthenticator(t *testing.T) *softAuthenticator {
	return &softAuthenticator{t: t, origin: testOrigin}
}

// enrolledUser creates username, signs in by password in a fresh browser and
// enrols one passkey.
func (e *passkeyEnv) enrolledUser(username string, counts bool) (*softAuthenticator, *softCredential, int64) {
	e.t.Helper()
	id := e.addUser(username, models.RoleCaptain)
	b := e.browser("198.51.100.200")
	b.passwordSignIn(username)
	a := newAuthenticator(e.t)
	cred := b.enrol(a, username+"'s phone", counts)
	return a, cred, id
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status %d, want %d; body: %s", rec.Code, want, rec.Body.String())
	}
}

func TestPasskeyEnrolAndDiscoverableSignIn(t *testing.T) {
	env := newPasskeyEnv(t)
	userID := env.addUser("alice", models.RoleAdmin)
	b := env.browser("198.51.100.1")
	b.passwordSignIn("alice")

	// The creation options ask for what the work item specifies.
	begin := b.postJSON("/account/passkeys/begin", enrolBody)
	wantStatus(t, begin, http.StatusOK)
	var opts creationOptions
	if err := json.Unmarshal(begin.Body.Bytes(), &opts); err != nil {
		t.Fatal(err)
	}
	pk := opts.PublicKey
	if pk.RP.ID != testRPID || pk.RP.Name != "Example Tennis Club" {
		t.Errorf("rp = %+v", pk.RP)
	}
	if pk.AuthenticatorSelection.ResidentKey != "required" || !pk.AuthenticatorSelection.RequireResidentKey ||
		pk.AuthenticatorSelection.UserVerification != "required" || pk.Attestation != "none" { // alice is an administrator (F7)
		t.Errorf("selection = %+v, attestation %q", pk.AuthenticatorSelection, pk.Attestation)
	}
	if pk.Timeout != int(ceremonyLifetime/time.Millisecond) {
		t.Errorf("timeout = %d ms", pk.Timeout)
	}
	if len(unb64(t, pk.Challenge)) < 32 {
		t.Errorf("challenge is %d bytes", len(unb64(t, pk.Challenge)))
	}
	// The user handle is random: not the id, not the username.
	handle := unb64(t, pk.User.ID)
	if len(handle) != 32 || strings.Contains(string(handle), "alice") {
		t.Errorf("user handle %x", handle)
	}
	if pk.User.Name != "alice" {
		t.Errorf("user name %q", pk.User.Name)
	}

	a := newAuthenticator(t)
	a.backupEligible = true
	body, cred := a.create(begin.Body.Bytes(), false)
	wantStatus(t, b.postJSON("/account/passkeys/finish?name=Alice%27s+phone", body), http.StatusOK)

	// A second passkey, on another authenticator, with a default name.
	second := newAuthenticator(t)
	b.enrol(second, "", true)

	creds, err := env.pk.List(context.Background(), userID)
	if err != nil || len(creds) != 2 {
		t.Fatalf("passkeys = %d, %v", len(creds), err)
	}
	if creds[0].Name != "Alice's phone" || !creds[0].BackupEligible || !creds[0].BackupState || creds[0].LastUsedAt != nil {
		t.Errorf("first passkey = %+v", creds[0])
	}
	if !strings.HasPrefix(creds[1].Name, "Passkey added ") || creds[1].SignCount != 1 {
		t.Errorf("second passkey = %+v", creds[1])
	}
	if creds[0].Transports != "internal,hybrid" {
		t.Errorf("transports %q", creds[0].Transports)
	}
	// The same handle is reused for the second passkey.
	if !bytes.Equal(second.creds[0].userHandle, handle) {
		t.Error("user handle changed between enrolments")
	}

	// Sign in on a different browser with nothing typed.
	phone := env.browser("198.51.100.2")
	options := phone.beginSignIn("")
	var req requestOptions
	if err := json.Unmarshal(options, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.PublicKey.AllowCredentials) != 0 || req.PublicKey.RPID != testRPID || req.PublicKey.UserVerification != "preferred" {
		t.Errorf("discoverable request options = %+v", req.PublicKey)
	}
	finish := phone.postJSON("/login/passkey/finish?redirect=%2Fadmin%2Fleague%2Ffixtures", a.get(options, cred))
	wantStatus(t, finish, http.StatusOK)
	var result map[string]string
	if err := json.Unmarshal(finish.Body.Bytes(), &result); err != nil || result["redirect"] != "/admin/league/fixtures" {
		t.Errorf("finish result %v, %v", result, err)
	}
	if !phone.signedIn() {
		t.Fatal("no session cookie after passkey sign-in")
	}
	if _, set := phone.cookies[ceremonyCookie]; set {
		t.Error("ceremony cookie not cleared")
	}

	// The session is the same kind of session a password sign-in makes.
	fresh := env.browser("198.51.100.2")
	fresh.passwordSignIn("alice")
	var passkeySession, passwordSession models.Session
	if err := env.svc.db.Get(&passkeySession, `SELECT * FROM sessions WHERE token_hash = ?`, HashSessionToken(phone.cookies["session_token"])); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.db.Get(&passwordSession, `SELECT * FROM sessions WHERE token_hash = ?`, HashSessionToken(fresh.cookies["session_token"])); err != nil {
		t.Fatal(err)
	}
	if passkeySession.IP != passwordSession.IP || passkeySession.DeviceInfo != passwordSession.DeviceInfo ||
		len(phone.cookies["session_token"]) != len(fresh.cookies["session_token"]) {
		t.Errorf("passkey session %+v differs in shape from password session %+v", passkeySession, passwordSession)
	}
	if passkeySession.UserID != userID || passkeySession.Role != passwordSession.Role || !passkeySession.IsValid {
		t.Errorf("passkey session = %+v", passkeySession)
	}
	if got, want := passkeySession.ExpiresAt.Sub(passkeySession.CreatedAt), passwordSession.ExpiresAt.Sub(passwordSession.CreatedAt); got != want {
		t.Errorf("passkey session lifetime %v, password session lifetime %v", got, want)
	}
	if _, err := env.svc.ValidateSession(phone.cookies["session_token"], httptest.NewRequest("GET", "/", nil)); err != nil {
		t.Errorf("passkey session does not validate: %v", err)
	}

	creds, _ = env.pk.List(context.Background(), userID)
	if creds[0].LastUsedAt == nil {
		t.Error("last used time not recorded")
	}
	if n := env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = 'alice' AND success = 1`); n != 3 {
		t.Errorf("successful attempts recorded for alice = %d, want 3 (password twice, passkey once)", n)
	}

	// An unsafe redirect is ignored.
	other := env.browser("198.51.100.3")
	finish = other.postJSON("/login/passkey/finish?redirect=https%3A%2F%2Fevil.example.org%2F", a.get(other.beginSignIn(""), cred))
	wantStatus(t, finish, http.StatusOK)
	if !strings.Contains(finish.Body.String(), `"/admin/league"`) {
		t.Errorf("redirect not defaulted: %s", finish.Body.String())
	}

	if len(env.events) != 2 || !strings.HasPrefix(env.events[0], "alice "+PasskeyActionAdd) {
		t.Errorf("audit events = %v", env.events)
	}
}

func TestPasskeyChallengeIsSingleUse(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)

	// A successful finish cannot be replayed, even by the same browser.
	b := env.browser("198.51.100.1")
	body := a.get(b.beginSignIn(""), cred)
	ticket := b.cookies[ceremonyCookie]
	wantStatus(t, b.postJSON("/login/passkey/finish", body), http.StatusOK)
	replay := env.browser("198.51.100.1")
	replay.cookies[ceremonyCookie] = ticket
	wantStatus(t, replay.postJSON("/login/passkey/finish", body), http.StatusBadRequest)
	if replay.signedIn() {
		t.Fatal("replayed assertion signed in")
	}

	// A failed finish also spends the challenge: a correct answer to the
	// same challenge afterwards is refused.
	c := env.browser("198.51.100.4")
	options := c.beginSignIn("")
	ticket = c.cookies[ceremonyCookie]
	wantStatus(t, c.postJSON("/login/passkey/finish", []byte(`{"not":"a credential"}`)), http.StatusUnauthorized)
	c.cookies[ceremonyCookie] = ticket
	wantStatus(t, c.postJSON("/login/passkey/finish", a.get(options, cred)), http.StatusBadRequest)
	if c.signedIn() {
		t.Fatal("signed in with a spent challenge")
	}

	// Two ceremonies never share a challenge.
	var first, second requestOptions
	_ = json.Unmarshal(c.beginSignIn(""), &first)
	_ = json.Unmarshal(c.beginSignIn(""), &second)
	if first.PublicKey.Challenge == "" || first.PublicKey.Challenge == second.PublicKey.Challenge {
		t.Error("challenges repeat")
	}
}

func TestPasskeyChallengeExpires(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)

	b := env.browser("198.51.100.1")
	body := a.get(b.beginSignIn(""), cred)
	env.advance(ceremonyLifetime + time.Second)
	wantStatus(t, b.postJSON("/login/passkey/finish", body), http.StatusBadRequest)
	if b.signedIn() {
		t.Fatal("signed in with an expired challenge")
	}

	// Registration challenges expire too.
	b.passwordSignIn("alice")
	begin := b.postJSON("/account/passkeys/begin", enrolBody)
	wantStatus(t, begin, http.StatusOK)
	created, _ := newAuthenticator(t).create(begin.Body.Bytes(), false)
	env.advance(ceremonyLifetime + time.Second)
	wantStatus(t, b.postJSON("/account/passkeys/finish", created), http.StatusBadRequest)
}

func TestPasskeyChallengeIsBoundToTheBrowser(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)

	// The answer to browser A's challenge, posted by a browser that never
	// began a ceremony, or by one that began its own.
	victim := env.browser("198.51.100.1")
	body := a.get(victim.beginSignIn(""), cred)

	noCeremony := env.browser("198.51.100.5")
	wantStatus(t, noCeremony.postJSON("/login/passkey/finish", body), http.StatusBadRequest)

	ownCeremony := env.browser("198.51.100.6")
	ownCeremony.beginSignIn("")
	wantStatus(t, ownCeremony.postJSON("/login/passkey/finish", body), http.StatusUnauthorized)
	if noCeremony.signedIn() || ownCeremony.signedIn() {
		t.Fatal("an assertion for another browser's challenge signed in")
	}

	// The cookie that carries the binding is not readable by scripts and is
	// same-site only.
	rec := victim.postJSON("/login/passkey/begin", []byte("{}"))
	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == ceremonyCookie {
			found = true
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || !c.Secure || c.MaxAge != int(ceremonyLifetime/time.Second) {
				t.Errorf("ceremony cookie = %+v", c)
			}
		}
	}
	if !found {
		t.Error("no ceremony cookie set")
	}
}

func TestPasskeyOriginAndRelyingPartyAreVerified(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)
	b := env.browser("198.51.100.1")

	// A page on another origin relaying the ceremony.
	a.origin = "https://tennis.example.org.evil.example.com"
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusUnauthorized)
	// http instead of https, and a different port.
	a.origin = "http://tennis.example.org"
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusUnauthorized)
	a.origin = "https://tennis.example.org:8443"
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusUnauthorized)
	// The right origin but a credential scoped to another relying party.
	a.origin = testOrigin
	a.rpID = "example.org"
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusUnauthorized)
	a.rpID = ""
	if b.signedIn() {
		t.Fatal("signed in despite a wrong origin or relying party")
	}
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)

	// Registration checks them as well.
	rogue := newAuthenticator(t)
	rogue.origin = "https://evil.example.com"
	begin := b.postJSON("/account/passkeys/begin", enrolBody)
	body, _ := rogue.create(begin.Body.Bytes(), false)
	wantStatus(t, b.postJSON("/account/passkeys/finish", body), http.StatusUnauthorized)
	rogue.origin, rogue.rpID = testOrigin, "evil.example.com"
	begin = b.postJSON("/account/passkeys/begin", enrolBody)
	body, _ = rogue.create(begin.Body.Bytes(), false)
	wantStatus(t, b.postJSON("/account/passkeys/finish", body), http.StatusUnauthorized)
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials`); n != 1 {
		t.Errorf("passkeys stored = %d, want 1", n)
	}

	// The Host header plays no part when a public URL is configured.
	spoofed := env.browser("198.51.100.7")
	spoofed.host = "evil.example.com"
	var opts requestOptions
	_ = json.Unmarshal(spoofed.beginSignIn(""), &opts)
	if opts.PublicKey.RPID != testRPID {
		t.Errorf("relying party followed the Host header: %q", opts.PublicKey.RPID)
	}
}

func TestPasskeyFollowsThePublicURLLive(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)
	b := env.browser("198.51.100.1")

	// The address changes while a ceremony is in flight: the old ceremony
	// is refused even though the authenticator answered for the old origin.
	body := a.get(b.beginSignIn(""), cred)
	env.src.set("https://club.example.com")
	wantStatus(t, b.postJSON("/login/passkey/finish", body), http.StatusBadRequest)

	// New ceremonies use the new address at once.
	var opts requestOptions
	_ = json.Unmarshal(b.beginSignIn(""), &opts)
	if opts.PublicKey.RPID != "club.example.com" {
		t.Errorf("relying party = %q after the public URL changed", opts.PublicKey.RPID)
	}
	// The old passkey is scoped to the old host and no longer verifies.
	a.origin = "https://club.example.com"
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusUnauthorized)

	// No public URL at all: passkeys are off.
	env.src.set("")
	wantStatus(t, b.postJSON("/login/passkey/begin", []byte("{}")), http.StatusServiceUnavailable)
	if env.pk.Available(httptest.NewRequest("GET", "/login", nil)) {
		t.Error("passkeys available with no public URL")
	}
}

func TestRelyingPartyAddress(t *testing.T) {
	cases := []struct {
		name       string
		appEnv     string
		devMode    string
		publicURL  string
		host       string
		wantOrigin string
		wantRPID   string
	}{
		{"public URL", "production", "", "https://tennis.example.org", "tennis.example.org", "https://tennis.example.org", "tennis.example.org"},
		{"public URL beats a spoofed host", "production", "", "https://tennis.example.org", "evil.example.com", "https://tennis.example.org", "tennis.example.org"},
		{"public URL with port", "", "", "https://tennis.example.org:8443", "x", "https://tennis.example.org:8443", "tennis.example.org"},
		{"http public URL in production", "production", "", "http://tennis.example.org", "tennis.example.org", "", ""},
		{"http public URL in development", "", "true", "http://localhost:8080", "localhost:8080", "http://localhost:8080", "localhost"},
		{"nothing configured in production", "production", "", "", "localhost:8080", "", ""},
		{"nothing configured, no dev mode", "", "", "", "localhost:8080", "", ""},
		{"dev mode on localhost", "", "true", "", "localhost:8080", "http://localhost:8080", "localhost"},
		{"dev mode on localhost, default port", "", "1", "", "localhost", "http://localhost", "localhost"},
		{"dev mode, other host", "", "true", "", "evil.example.com", "", ""},
		{"dev mode, container name", "", "true", "", "app-e2e:8080", "", ""},
		{"dev mode, lookalike host", "", "true", "", "localhost.evil.example.com:8080", "", ""},
		{"dev mode, loopback address", "", "true", "", "127.0.0.1:8080", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(config.EnvAppEnv, tc.appEnv)
			t.Setenv(config.EnvDevMode, tc.devMode)
			p := &Passkeys{publicURL: &fakePublicURL{url: tc.publicURL}, displayName: func() string { return "" }}
			req := httptest.NewRequest("GET", "/login", nil)
			req.Host = tc.host
			origin, rpID, err := p.relyingPartyAddress(req)
			if tc.wantOrigin == "" {
				if err == nil {
					t.Fatalf("got %q / %q, want passkeys unavailable", origin, rpID)
				}
				if p.Available(req) {
					t.Error("Available = true")
				}
				return
			}
			if err != nil || origin != tc.wantOrigin || rpID != tc.wantRPID {
				t.Fatalf("got %q / %q / %v, want %q / %q", origin, rpID, err, tc.wantOrigin, tc.wantRPID)
			}
			if !p.Available(req) {
				t.Error("Available = false")
			}
		})
	}

	// A public URL whose host is an IP address cannot be a relying party.
	t.Setenv(config.EnvAppEnv, "")
	p := &Passkeys{publicURL: &fakePublicURL{url: "https://203.0.113.9"}, displayName: func() string { return "" }}
	if p.Available(httptest.NewRequest("GET", "/login", nil)) {
		t.Error("passkeys available on an IP address")
	}
}

// F8 (WI-134): there is no username-first passkey sign-in. Naming a
// username when beginning a sign-in changes nothing: the answer has the same
// shape and content (apart from the random challenge) as one without, lists
// no account's passkeys, and nothing is stored for the name.
func TestPasskeySignInTakesNoUsername(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)
	env.addUser("nopasskeys", models.RoleCaptain)
	// A leftover decoy key from the removed path is deleted at startup.
	env.svc.db.MustExec(`INSERT INTO app_settings (key, value, updated_at) VALUES ('passkey_decoy_key', 'aa', CURRENT_TIMESTAMP)`)
	NewPasskeys(env.svc, env.src, func() string { return "x" })
	if n := env.count(`SELECT COUNT(*) FROM app_settings WHERE key = 'passkey_decoy_key'`); n != 0 {
		t.Errorf("the retired decoy key is still stored")
	}

	b := env.browser("198.51.100.1")
	normalised := func(body []byte) string {
		t.Helper()
		rec := b.postJSON("/login/passkey/begin", body)
		wantStatus(t, rec, http.StatusOK)
		var generic map[string]map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
			t.Fatal(err)
		}
		pk := generic["publicKey"]
		if allow, ok := pk["allowCredentials"]; ok && allow != nil {
			if list, _ := allow.([]any); len(list) != 0 {
				t.Errorf("body %s: the answer lists credentials: %v", body, allow)
			}
		}
		if ch, _ := pk["challenge"].(string); len(ch) < 20 {
			t.Errorf("body %s: challenge %q", body, ch)
		}
		pk["challenge"] = "x" // random per request
		out, _ := json.Marshal(generic)
		return string(out)
	}
	plain := normalised([]byte("{}"))
	for _, body := range []string{
		`{"username":"alice"}`, `{"username":"nopasskeys"}`, `{"username":"nobody"}`, `{"username":"ALICE "}`, ``,
	} {
		if got := normalised([]byte(body)); got != plain {
			t.Errorf("begin with body %s answered\n%s\nwithout a username it answers\n%s", body, got, plain)
		}
	}
	if strings.Contains(plain, b64(cred.id)) {
		t.Error("a credential id is disclosed to an anonymous caller")
	}
	if n := env.count(`SELECT COUNT(*) FROM app_settings WHERE key = 'passkey_decoy_key'`); n != 0 {
		t.Errorf("a decoy key was stored")
	}

	// Discoverable sign-in still works, whatever name was sent at the start.
	begin := b.postJSON("/login/passkey/begin", []byte(`{"username":"nobody"}`))
	wantStatus(t, begin, http.StatusOK)
	wantStatus(t, b.postJSON("/login/passkey/finish", a.get(begin.Body.Bytes(), cred)), http.StatusOK)
	if !b.signedIn() {
		t.Fatal("not signed in")
	}

	// The login page and the script offer no username control for passkeys.
	page := env.browser("198.51.100.2").do(http.MethodGet, "/login", "", nil).Body.String()
	for _, gone := range []string{"passkey-signin-username", "data-passkey-username"} {
		if strings.Contains(page, gone) {
			t.Errorf("login page still has %s", gone)
		}
	}
}

func TestPasskeyCredentialIDsAreUniqueAcrossUsers(t *testing.T) {
	env := newPasskeyEnv(t)
	_, aliceCred, _ := env.enrolledUser("alice", false)

	bobID := env.addUser("bob", models.RoleCaptain)
	b := env.browser("198.51.100.1")
	b.passwordSignIn("bob")
	rogue := newAuthenticator(t)
	rogue.nextID = aliceCred.id // an authenticator that claims alice's credential id
	begin := b.postJSON("/account/passkeys/begin", enrolBody)
	body, _ := rogue.create(begin.Body.Bytes(), false)
	wantStatus(t, b.postJSON("/account/passkeys/finish", body), http.StatusConflict)
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, bobID); n != 0 {
		t.Errorf("bob has %d passkeys", n)
	}

	// The same authenticator cannot be enrolled twice for one account
	// either (the browser is told to exclude it, and the server refuses).
	a := newAuthenticator(t)
	first := b.enrol(a, "one", false)
	begin = b.postJSON("/account/passkeys/begin", enrolBody)
	var opts creationOptions
	_ = json.Unmarshal(begin.Body.Bytes(), &opts)
	if len(opts.PublicKey.ExcludeCredentials) != 1 || opts.PublicKey.ExcludeCredentials[0].ID != b64(first.id) {
		t.Errorf("exclude list = %+v", opts.PublicKey.ExcludeCredentials)
	}
	a.nextID = first.id
	body, _ = a.create(begin.Body.Bytes(), false)
	wantStatus(t, b.postJSON("/account/passkeys/finish", body), http.StatusConflict)
}

func TestPasskeyInactiveUserCannotSignIn(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, userID := env.enrolledUser("alice", false)
	b := env.browser("198.51.100.1")
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)

	if _, err := env.svc.db.Exec(`UPDATE users SET is_active = false WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	c := env.browser("198.51.100.2")
	rec := c.passkeySignIn(a, cred)
	wantStatus(t, rec, http.StatusUnauthorized)
	if c.signedIn() {
		t.Fatal("inactive user signed in with a passkey")
	}
	// The answer is the same one a forged signature for an active account gets.
	_, bobCred, _ := env.enrolledUser("bob", false)
	wrong := env.browser("198.51.100.3")
	options := wrong.beginSignIn("")
	generic := wrong.postJSON("/login/passkey/finish", newAuthenticator(t).assertion(mustChallenge(t, options), forged(t, bobCred), 0))
	wantStatus(t, generic, http.StatusUnauthorized)
	if rec.Body.String() != generic.Body.String() {
		t.Errorf("inactive account answer %q differs from a failed signature's %q", rec.Body.String(), generic.Body.String())
	}
}

// forged is cred's id and user handle with a key the server has never seen:
// what someone who knows the public details but not the private key can make.
func forged(t *testing.T, cred *softCredential) *softCredential {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &softCredential{id: cred.id, key: key, rpID: cred.rpID, userHandle: cred.userHandle}
}

// mustChallenge extracts the challenge from request options.
func mustChallenge(t *testing.T, options []byte) string {
	t.Helper()
	var opts requestOptions
	if err := json.Unmarshal(options, &opts); err != nil || opts.PublicKey.Challenge == "" {
		t.Fatalf("no challenge in %s (%v)", options, err)
	}
	return opts.PublicKey.Challenge
}

func TestPasskeySignCounterRegressionBlocksAndWarnsAdmins(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, userID := env.enrolledUser("alice", true)
	env.addUser("boss", models.RoleAdmin)

	// Normal use: the counter goes up each time.
	b := env.browser("198.51.100.1")
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)
	stored, _ := env.pk.List(context.Background(), userID)
	if stored[0].SignCount != cred.counter || cred.counter != 3 {
		t.Fatalf("stored counter %d, authenticator counter %d", stored[0].SignCount, cred.counter)
	}

	// A copy of the authenticator, behind on its counter, signs in.
	clone := env.browser("203.0.113.50")
	rec := clone.postJSON("/login/passkey/finish", a.assertion(mustChallenge(t, clone.beginSignIn("")), cred, 2))
	wantStatus(t, rec, http.StatusForbidden)
	if clone.signedIn() {
		t.Fatal("a regressed sign counter was accepted")
	}
	stored, _ = env.pk.List(context.Background(), userID)
	if stored[0].CloneWarningAt == nil || stored[0].SignCount != 3 {
		t.Fatalf("after regression: %+v", stored[0])
	}
	// An equal counter is a regression too.
	rec = clone.postJSON("/login/passkey/finish", a.assertion(mustChallenge(t, clone.beginSignIn("")), cred, 3))
	wantStatus(t, rec, http.StatusForbidden)

	// While flagged, even the genuine authenticator is refused.
	wantStatus(t, env.browser("198.51.100.9").passkeySignIn(a, cred), http.StatusForbidden)

	// Administrators see it; other users do not.
	warnings, err := env.pk.CloneWarnings(context.Background())
	if err != nil || len(warnings) != 1 || warnings[0].Username != "alice" || warnings[0].ID != stored[0].ID {
		t.Fatalf("warnings = %+v, %v", warnings, err)
	}
	found := false
	for _, e := range env.events {
		found = found || strings.HasPrefix(e, "alice "+PasskeyActionCloneWarning)
	}
	if !found {
		t.Errorf("no audit event for the clone warning: %v", env.events)
	}
	admin := env.browser("198.51.100.10")
	admin.passwordSignIn("boss")
	page := admin.do(http.MethodGet, SignInMethodsPath, "", nil)
	wantStatus(t, page, http.StatusOK)
	if !strings.Contains(page.Body.String(), "data-passkey-warnings") || !strings.Contains(page.Body.String(), "alice") {
		t.Error("administrator's page does not list the warning")
	}
	owner := env.browser("198.51.100.11")
	owner.passwordSignIn("alice")
	page = owner.do(http.MethodGet, SignInMethodsPath, "", nil)
	if strings.Contains(page.Body.String(), "data-passkey-warnings") || !strings.Contains(page.Body.String(), "data-passkey-blocked") {
		t.Error("owner's page should mark the passkey blocked without the administrators' list")
	}

	// Only an administrator can clear it; then the passkey works again.
	clearPath := PasskeysPathPrefix + "1/clear-warning"
	if stored[0].ID != 1 {
		t.Fatalf("passkey id %d", stored[0].ID)
	}
	wantStatus(t, owner.postForm(clearPath, nil), http.StatusForbidden)
	wantStatus(t, env.browser("198.51.100.12").postForm(clearPath, nil), http.StatusSeeOther)
	if w, _ := env.pk.CloneWarnings(context.Background()); len(w) != 1 {
		t.Fatal("warning cleared without an administrator")
	}
	wantStatus(t, admin.postForm(clearPath, nil), http.StatusSeeOther)
	wantStatus(t, admin.postForm(PasskeysPathPrefix+"999/clear-warning", nil), http.StatusNotFound)
	if w, _ := env.pk.CloneWarnings(context.Background()); len(w) != 0 {
		t.Fatal("warning not cleared")
	}
	wantStatus(t, env.browser("198.51.100.13").passkeySignIn(a, cred), http.StatusOK)

	// An authenticator with no counter (always zero) never trips this.
	syncA, syncCred, _ := env.enrolledUser("carol", false)
	for i := 0; i < 3; i++ {
		wantStatus(t, env.browser("198.51.100.14").passkeySignIn(syncA, syncCred), http.StatusOK)
	}
}

func TestPasskeyThrottleIsPerAddressNeverPerUser(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)

	// An attacker hammers alice's credential id with bad signatures.
	attacker := env.browser("203.0.113.66")
	forger := newAuthenticator(t)
	for i := 0; i < passkeyFailureLimit; i++ {
		options := attacker.beginSignIn("alice")
		wantStatus(t, attacker.postJSON("/login/passkey/finish", forger.assertion(mustChallenge(t, options), forged(t, cred), 0)), http.StatusUnauthorized)
	}
	// The attacker's address is now throttled, at begin and at finish.
	wantStatus(t, attacker.postJSON("/login/passkey/begin", []byte(`{"username":"alice"}`)), http.StatusTooManyRequests)
	wantStatus(t, attacker.postJSON("/login/passkey/finish", []byte(`{}`)), http.StatusTooManyRequests)

	// Alice is not locked out: her passkey works from her own address, and
	// so does her password, even from the attacker's address.
	wantStatus(t, env.browser("198.51.100.1").passkeySignIn(a, cred), http.StatusOK)
	env.browser("198.51.100.1").passwordSignIn("alice")
	env.browser("203.0.113.66").passwordSignIn("alice")
	// Nothing was recorded against her username.
	if n := env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = 'alice' AND success = 0`); n != 0 {
		t.Errorf("%d failed attempts recorded against alice", n)
	}
	if n := env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = ? AND success = 0`, passkeyAttemptUsername); n != passkeyFailureLimit {
		t.Errorf("%d failed passkey attempts recorded, want %d", n, passkeyFailureLimit)
	}

	// The throttle lifts after the window.
	env.advance(passkeyFailureWindow + time.Second)
	wantStatus(t, attacker.postJSON("/login/passkey/begin", []byte(`{}`)), http.StatusOK)

	// Beginning ceremonies is rate limited per address too.
	flood := env.browser("203.0.113.77")
	for i := 0; i < passkeyBeginLimit; i++ {
		wantStatus(t, flood.postJSON("/login/passkey/begin", []byte(`{}`)), http.StatusOK)
	}
	wantStatus(t, flood.postJSON("/login/passkey/begin", []byte(`{}`)), http.StatusTooManyRequests)
	wantStatus(t, env.browser("198.51.100.2").postJSON("/login/passkey/begin", []byte(`{}`)), http.StatusOK)
}

func TestPasskeyManagementNeedsASessionAndOwnership(t *testing.T) {
	env := newPasskeyEnv(t)
	_, _, aliceID := env.enrolledUser("alice", false)
	env.addUser("bob", models.RoleCaptain)

	// No session: every /account/ endpoint sends the browser to sign in.
	anon := env.browser("198.51.100.1")
	for _, path := range []string{"/account/passkeys/begin", "/account/passkeys/finish", "/account/passkeys/1/rename", "/account/passkeys/1/delete", "/account/passkeys/1/clear-warning"} {
		rec := anon.postJSON(path, []byte("{}"))
		if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
			t.Errorf("%s without a session: status %d, Location %q", path, rec.Code, rec.Header().Get("Location"))
		}
	}
	rec := anon.do(http.MethodGet, SignInMethodsPath, "", nil)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("sign-in methods page without a session: status %d", rec.Code)
	}

	// Bob cannot rename or remove alice's passkey (id 1).
	bob := env.browser("198.51.100.2")
	bob.passwordSignIn("bob")
	wantStatus(t, bob.postForm("/account/passkeys/1/rename", url.Values{"name": {"mine now"}}), http.StatusNotFound)
	wantStatus(t, bob.postForm("/account/passkeys/1/delete", pwForm), http.StatusNotFound)
	creds, _ := env.pk.List(context.Background(), aliceID)
	if len(creds) != 1 || creds[0].Name != "alice's phone" {
		t.Fatalf("alice's passkeys after bob's attempts: %+v", creds)
	}
	page := bob.do(http.MethodGet, SignInMethodsPath, "", nil)
	if strings.Contains(page.Body.String(), "alice") {
		t.Error("bob's page shows alice's passkey")
	}

	// Alice can rename hers; the name is cleaned.
	alice := env.browser("198.51.100.3")
	alice.passwordSignIn("alice")
	wantStatus(t, alice.postForm("/account/passkeys/1/rename", url.Values{"name": {"  Work\tlaptop\x00 " + strings.Repeat("x", 200)}}), http.StatusSeeOther)
	creds, _ = env.pk.List(context.Background(), aliceID)
	if !strings.HasPrefix(creds[0].Name, "Work laptop xxx") || len([]rune(creds[0].Name)) != MaxPasskeyNameLength {
		t.Errorf("renamed to %q", creds[0].Name)
	}
	wantStatus(t, alice.postForm("/account/passkeys/abc/rename", url.Values{"name": {"x"}}), http.StatusNotFound)

	// A registration begun in one session cannot be finished from another
	// session, or by another user.
	begin := alice.postJSON("/account/passkeys/begin", enrolBody)
	wantStatus(t, begin, http.StatusOK)
	ticket := alice.cookies[ceremonyCookie]
	body, _ := newAuthenticator(t).create(begin.Body.Bytes(), false)

	bob.cookies[ceremonyCookie] = ticket
	wantStatus(t, bob.postJSON("/account/passkeys/finish", body), http.StatusBadRequest)

	begin = alice.postJSON("/account/passkeys/begin", enrolBody)
	ticket = alice.cookies[ceremonyCookie]
	body, _ = newAuthenticator(t).create(begin.Body.Bytes(), false)
	aliceElsewhere := env.browser("198.51.100.4")
	aliceElsewhere.passwordSignIn("alice")
	aliceElsewhere.cookies[ceremonyCookie] = ticket
	wantStatus(t, aliceElsewhere.postJSON("/account/passkeys/finish", body), http.StatusBadRequest)
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials`); n != 1 {
		t.Errorf("passkeys stored = %d, want 1", n)
	}

	// A sign-in ceremony cannot be finished as a registration or vice versa.
	alice.beginSignIn("")
	wantStatus(t, alice.postJSON("/account/passkeys/finish", body), http.StatusBadRequest)
	begin = alice.postJSON("/account/passkeys/begin", enrolBody)
	wantStatus(t, begin, http.StatusOK)
	wantStatus(t, alice.postJSON("/login/passkey/finish", body), http.StatusBadRequest)
}

func TestPasskeyLastSignInMethodCannotBeRemoved(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, userID := env.enrolledUser("alice", false)
	ctx := context.Background()

	if n, err := env.svc.CountSignInMethods(ctx, userID); err != nil || n != 2 {
		t.Fatalf("methods = %d, %v; want 2 (password and passkey)", n, err)
	}
	// Make the account passkey-only (as WI-129 will allow) with two passkeys.
	if _, err := env.svc.db.Exec(`UPDATE users SET password_hash = '' WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	b := env.browser("198.51.100.1")
	wantStatus(t, b.passkeySignIn(a, cred), http.StatusOK)
	b.enrolWithPasskey(newAuthenticator(t), "second", a, cred)
	if n, _ := env.svc.CountSignInMethods(ctx, userID); n != 2 {
		t.Fatalf("methods = %d, want 2 passkeys", n)
	}

	// Being signed in is not enough to remove one (F2)...
	if rec := b.postForm("/account/passkeys/2/delete", nil); !strings.Contains(rec.Header().Get("Location"), "error=reauth") {
		t.Fatalf("removal without proof: Location %q", rec.Header().Get("Location"))
	}
	if n, _ := env.svc.CountSignInMethods(ctx, userID); n != 2 {
		t.Fatalf("methods = %d after a removal without proof, want 2", n)
	}
	// ...a passkey confirmation is.
	reauth(t, b, a, cred)
	if rec := b.postForm("/account/passkeys/2/delete", nil); !strings.Contains(rec.Header().Get("Location"), "done=removed") {
		t.Fatalf("removal with proof: Location %q", rec.Header().Get("Location"))
	}
	page := b.do(http.MethodGet, SignInMethodsPath, "", nil)
	if !strings.Contains(page.Body.String(), "data-passkey-last-method") || strings.Contains(page.Body.String(), "data-passkey-remove") {
		t.Error("page offers to remove the only sign-in method")
	}
	if !strings.Contains(page.Body.String(), `data-signin-password="none"`) {
		t.Error("page does not show the account has no password")
	}
	reauth(t, b, a, cred)
	rec := b.postForm("/account/passkeys/1/delete", nil)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=last") {
		t.Fatalf("removing the last method: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	if n, _ := env.svc.CountSignInMethods(ctx, userID); n != 1 {
		t.Fatalf("methods = %d after refused removal, want 1", n)
	}
	wantStatus(t, env.browser("198.51.100.2").passkeySignIn(a, cred), http.StatusOK)

	// With a password as well, the last passkey can go.
	if _, err := env.svc.db.Exec(`UPDATE users SET password_hash = 'x' WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	reauth(t, b, a, cred)
	rec = b.postForm("/account/passkeys/1/delete", nil)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "done=removed") {
		t.Fatalf("removing a passkey with a password set: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	if n, _ := env.svc.CountSignInMethods(ctx, userID); n != 1 {
		t.Fatalf("methods = %d, want the password only", n)
	}
	wantStatus(t, env.browser("198.51.100.3").passkeySignIn(a, cred), http.StatusUnauthorized)
}

func TestPasskeyEndpointsRefuseCrossSiteRequests(t *testing.T) {
	env := newPasskeyEnv(t)
	_, _, _ = env.enrolledUser("alice", false)
	victim := env.browser("198.51.100.1")
	victim.passwordSignIn("alice")

	paths := []string{
		"/login/passkey/begin", "/login/passkey/finish",
		"/account/passkeys/begin", "/account/passkeys/finish",
		"/account/passkeys/1/rename", "/account/passkeys/1/delete", "/account/passkeys/1/clear-warning",
	}
	// A request another site makes the browser send.
	victim.headers = map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example.com"}
	for _, path := range paths {
		wantStatus(t, victim.postForm(path, url.Values{"name": {"evil"}}), http.StatusForbidden)
	}
	// Older browsers: Origin that does not match the host.
	victim.headers = map[string]string{"Origin": "https://evil.example.com"}
	for _, path := range paths {
		wantStatus(t, victim.postForm(path, url.Values{"name": {"evil"}}), http.StatusForbidden)
	}
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE name = "alice's phone"`); n != 1 {
		t.Error("a cross-site request changed a passkey")
	}

	// Same-site requests pass the guard; the JSON endpoints then insist on
	// a JSON content type, which a cross-site form cannot send.
	victim.headers = map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": testOrigin}
	for _, path := range paths[:4] {
		wantStatus(t, victim.postForm(path, nil), http.StatusUnsupportedMediaType)
		wantStatus(t, victim.do(http.MethodPost, path, "text/plain", []byte("{}")), http.StatusUnsupportedMediaType)
	}
	wantStatus(t, victim.postJSON("/login/passkey/begin", []byte("{}")), http.StatusOK)

	// They are POST only.
	for _, path := range paths {
		if rec := victim.do(http.MethodGet, path, "", nil); rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status %d", path, rec.Code)
		}
	}
}

func TestLoginAndSignInMethodsPages(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("alice", models.RoleCaptain)
	b := env.browser("198.51.100.1")

	// Passkeys available: the passkey button leads, and the password form
	// keeps its field names and its single submit button.
	page := b.do(http.MethodGet, "/login?redirect=%2Fadmin%2Fleague%2Fplayers", "", nil).Body.String()
	passkeyAt := strings.Index(page, `id="passkey-signin"`)
	formAt := strings.Index(page, `<form method="POST" action="/login">`)
	if passkeyAt < 0 || formAt < 0 || passkeyAt > formAt {
		t.Errorf("passkey button at %d, password form at %d", passkeyAt, formAt)
	}
	for _, want := range []string{
		`name="username"`, `name="password"`, `data-passkey-signin data-redirect="/admin/league/players"`,
		`data-passkey-unsupported`, `data-passkey-status`, `<script src="/static/passkey.js">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("login page lacks %s", want)
		}
	}
	if n := strings.Count(page, `type="submit"`); n != 1 {
		t.Errorf("login page has %d submit buttons, want 1", n)
	}

	// Passkeys unavailable: the plain password form, no passkey script.
	env.src.set("")
	page = b.do(http.MethodGet, "/login", "", nil).Body.String()
	if strings.Contains(page, "passkey") || !strings.Contains(page, `name="password"`) {
		t.Error("login page offers passkeys with no public URL configured")
	}
	b.passwordSignIn("alice")
	page = b.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	if !strings.Contains(page, "data-passkey-unavailable") || strings.Contains(page, "data-passkey-add") {
		t.Error("sign-in methods page offers enrolment with no public URL configured")
	}

	// The sign-in methods page with passkeys available.
	env.src.set(testOrigin)
	b.enrol(newAuthenticator(t), "Phone <b>", false)
	page = b.do(http.MethodGet, SignInMethodsPath+"?done=added&enrol=1&next=%2Fsetup%2Fclub", "", nil).Body.String()
	for _, want := range []string{
		`data-page="signin-methods"`, `data-passkey-id="1"`, `Phone &lt;b&gt;`, `data-passkey-add`, `data-next="/setup/club"`,
		`data-signin-notice`, `data-signin-continue`, `href="/setup/club"`, `data-signin-password="set"`, `Not used yet`,
		`action="/account/passkeys/1/rename"`, `action="/account/passkeys/1/delete"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("sign-in methods page lacks %s", want)
		}
	}
	// An off-site "next" is dropped.
	page = b.do(http.MethodGet, SignInMethodsPath+"?next=https%3A%2F%2Fevil.example.com", "", nil).Body.String()
	if strings.Contains(page, "evil.example.com") {
		t.Error("sign-in methods page links off-site")
	}
}

func TestPasskeyLimitPerAccount(t *testing.T) {
	env := newPasskeyEnv(t)
	userID := env.addUser("alice", models.RoleCaptain)
	for i := 0; i < MaxPasskeysPerUser; i++ {
		if _, err := env.svc.db.Exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES (?, ?, 'k', 'n')`,
			userID, []byte{byte(i), 1, 2, 3}); err != nil {
			t.Fatal(err)
		}
	}
	b := env.browser("198.51.100.1")
	b.passwordSignIn("alice")
	wantStatus(t, b.postJSON("/account/passkeys/begin", enrolBody), http.StatusConflict)
}

func TestPasskeyName(t *testing.T) {
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"":                          "Passkey added 21 Nov 2026",
		"   \t\n ":                  "Passkey added 21 Nov 2026",
		"  Sam's  phone ":           "Sam's phone",
		"line\none\x00\x1b[31m":     "line one [31m",
		strings.Repeat("é", 100):    strings.Repeat("é", MaxPasskeyNameLength),
		"bad\xffbytes":              "badbytes",
		"<script>alert(1)</script>": "<script>alert(1)</script>", // escaped by the template, not here
	}
	for in, want := range cases {
		if got := PasskeyName(in, now); got != want {
			t.Errorf("PasskeyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCeremonyStore(t *testing.T) {
	s := newCeremonyStore()
	now := time.Now()
	ticket, err := s.put(ceremony{kind: ceremonyLogin}, now)
	if err != nil || len(ticket) < 43 {
		t.Fatalf("ticket %q, %v", ticket, err)
	}
	other, _ := s.put(ceremony{kind: ceremonyLogin}, now)
	if other == ticket {
		t.Fatal("tickets repeat")
	}
	// Wrong kind: refused, and spent.
	if _, ok := s.take(ticket, ceremonyRegister, now); ok {
		t.Error("a sign-in ceremony was taken as a registration")
	}
	if _, ok := s.take(ticket, ceremonyLogin, now); ok {
		t.Error("ceremony survived a failed take")
	}
	if _, ok := s.take(other, ceremonyLogin, now.Add(ceremonyLifetime)); ok {
		t.Error("ceremony taken at its expiry time")
	}
	if _, ok := s.take("", ceremonyLogin, now); ok {
		t.Error("empty ticket matched")
	}

	// The store is bounded: expired entries go first, then the oldest.
	for i := 0; i < maxCeremonies; i++ {
		if _, err := s.put(ceremony{kind: ceremonyLogin}, now.Add(time.Duration(i)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	newest, _ := s.put(ceremony{kind: ceremonyLogin}, now.Add(time.Minute))
	if len(s.byKey) != maxCeremonies {
		t.Errorf("store holds %d ceremonies, want %d", len(s.byKey), maxCeremonies)
	}
	if _, ok := s.take(newest, ceremonyLogin, now.Add(time.Minute)); !ok {
		t.Error("the newest ceremony was evicted")
	}
	for i := 0; i < 2; i++ {
		if _, err := s.put(ceremony{kind: ceremonyLogin}, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.byKey) != 2 {
		t.Errorf("expired ceremonies not evicted: %d left", len(s.byKey))
	}
}
