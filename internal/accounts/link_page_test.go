// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// linkEnv is the link page mounted the way the server mounts it: on a mux
// whose catch-all logs unknown paths, behind the cross-origin guard.
type linkEnv struct {
	t       *testing.T
	s       *Service
	db      *database.DB
	auth    *auth.Service
	clock   *testClock
	handler http.Handler
}

func newLinkEnv(t *testing.T) *linkEnv {
	t.Helper()
	t.Setenv("TEMPLATE_RELOAD", "")
	s, db := newTestService(t)
	env := &linkEnv{t: t, s: s, db: db, auth: auth.NewService(db, auth.DefaultConfig()), clock: withClock(s)}
	page := NewLinkPage(s, env.auth, nil, repoDir(t, "templates"), "/admin/league")
	page.now = env.clock.Now
	mux := http.NewServeMux()
	page.RegisterRoutes(mux)
	// Like cmd/jim-dot-tennis/main.go's home handler.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Not found: %s", auth.RedactPath(r.URL.Path))
		http.NotFound(w, r)
	})
	env.handler = auth.CrossOriginGuard(mux)
	return env
}

func (e *linkEnv) do(method, path, ip string, form url.Values, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var r *http.Request
	if form != nil {
		r = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.RemoteAddr = ip + ":40000"
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, r)
	return rec
}

// checkHeaders: every link response is uncacheable and sends no referrer.
func checkHeaders(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("%s: Cache-Control %q, want no-store", what, got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("%s: Referrer-Policy %q, want no-referrer", what, got)
	}
}

func passwordForm(pw string) url.Values {
	return url.Values{"new_password": {pw}, "new_password_confirm": {pw}}
}

var externalResource = regexp.MustCompile(`(?i)(src|href|action)\s*=\s*["']?\s*(https?:)?//`)

func TestLinkPageShowsWhoItIsForAndChangesNothing(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	_, invite, err := env.s.Invite(ctx, cli, "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		rec := env.do(http.MethodGet, invite.Path(), "198.51.100.1", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET invite: %d %s", rec.Code, rec.Body.String())
		}
		checkHeaders(t, "GET invite", rec)
		body := rec.Body.String()
		for _, want := range []string{`data-page="account-link"`, `data-link-kind="invite"`, ">pat<", ">captain<", "Nothing has changed yet", `id="new_password"`, `id="link-password-submit"`} {
			if !strings.Contains(body, want) {
				t.Errorf("invite page lacks %q", want)
			}
		}
		// Passkeys are not available in this environment: only the password form.
		if strings.Contains(body, "data-passkey-link") {
			t.Error("the page offers a passkey where passkeys are unavailable")
		}
		// Invites have nothing old to remove.
		if strings.Contains(body, "remove_other_methods") {
			t.Error("an invite page offers to remove old sign-in methods")
		}
		// Nothing but this site's own files.
		if m := externalResource.FindString(body); m != "" {
			t.Errorf("the page loads or links to another site: %q", m)
		}
	}
	// Viewing did not consume the link or touch the account.
	if _, err := env.s.LookupLink(ctx, invite.Token); err != nil {
		t.Fatalf("link after viewing: %v", err)
	}
	if a, _ := env.s.Get(ctx, invite.UserID); a.SignInMethods() != 0 || a.ActiveSessions != 0 {
		t.Fatalf("account after viewing: %+v", a)
	}

	// A recovery page explains the choice to remove the old methods.
	alice := mustCreate(t, env.s, "alice", models.RoleAdmin)
	addPasskey(t, env.db, alice.ID, "old-phone")
	recovery, err := env.s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	rec := env.do(http.MethodGet, recovery.Path(), "198.51.100.1", nil)
	body := rec.Body.String()
	for _, want := range []string{`data-link-kind="recovery"`, ">alice<", `id="remove_other_methods"`, "1 passkey and a password", "signed out"} {
		if !strings.Contains(body, want) {
			t.Errorf("recovery page lacks %q", want)
		}
	}
	checkHeaders(t, "GET recovery", rec)
	// HEAD and other methods.
	if rec := env.do(http.MethodPut, recovery.Path(), "198.51.100.1", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT: %d", rec.Code)
	} else {
		checkHeaders(t, "PUT", rec)
	}
}

func TestLinkPageSetsPasswordSignsInAndWorksOnce(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	alice := mustCreate(t, env.s, "alice", models.RoleAdmin)
	addSession(t, env.db, "old-1", alice.ID)
	addSession(t, env.db, "old-2", alice.ID)
	addPasskey(t, env.db, alice.ID, "old-phone")
	link, err := env.s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	ip := "198.51.100.2"

	// Mistakes leave the link usable and say what was wrong.
	form := passwordForm("a brand new secret")
	form.Set("new_password_confirm", "something else entirely")
	rec := env.do(http.MethodPost, link.Path(), ip, form)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "not the same") {
		t.Fatalf("mismatched passwords: %d", rec.Code)
	}
	checkHeaders(t, "POST mismatch", rec)
	rec = env.do(http.MethodPost, link.Path(), ip, passwordForm("short"))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "data-link-error") {
		t.Fatalf("weak password: %d", rec.Code)
	}
	if validSessions(t, env.db, alice.ID) != 2 {
		t.Fatal("a refused password ended sessions")
	}
	// A cross-site form post is refused before it reaches the page.
	rec = env.do(http.MethodPost, link.Path(), ip, passwordForm("attacker chosen secret"), "Sec-Fetch-Site", "cross-site", "Origin", "https://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST: %d", rec.Code)
	}

	form = passwordForm("a brand new secret")
	form.Set("remove_other_methods", "1")
	rec = env.do(http.MethodPost, link.Path(), ip, form)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/league" {
		t.Fatalf("set password: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	checkHeaders(t, "POST success", rec)
	hash := passwordHash(t, env.db, alice.ID)
	if !auth.PasswordMatches(hash, "a brand new secret") || auth.PasswordMatches(hash, "attacker chosen secret") {
		t.Fatal("the password was not set to the one typed")
	}
	if n := countRows(t, env.db, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, alice.ID); n != 0 {
		t.Errorf("%d old passkeys left after choosing to remove them", n)
	}
	// Every earlier session is over; the browser that used the link has the
	// only valid one.
	for _, old := range []string{"old-1", "old-2"} {
		if _, err := env.auth.ValidateSession(old, httptest.NewRequest(http.MethodGet, "/", nil)); err == nil {
			t.Errorf("session %s survived the recovery", old)
		}
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session_token" && c.Value != "" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie after using the link")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie flags: %+v", cookie)
	}
	session, err := env.auth.ValidateSession(cookie.Value, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil || session.UserID != alice.ID {
		t.Fatalf("the new session: %+v %v", session, err)
	}
	if validSessions(t, env.db, alice.ID) != 1 {
		t.Errorf("%d valid sessions, want only the new one", validSessions(t, env.db, alice.ID))
	}

	// The link is spent: viewing or posting again gets the "can't be used" page.
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		var f url.Values
		if method == http.MethodPost {
			f = passwordForm("yet another secret")
		}
		rec := env.do(method, link.Path(), ip, f)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `data-page="account-link-invalid"`) {
			t.Fatalf("%s a used link: %d", method, rec.Code)
		}
		checkHeaders(t, method+" used link", rec)
	}
	if !auth.PasswordMatches(passwordHash(t, env.db, alice.ID), "a brand new secret") {
		t.Fatal("a used link changed the password again")
	}
}

// A stranger cannot tell a link that never existed from one that is expired,
// used, revoked or for a disabled account.
func TestLinkPageGivesOneAnswerForEveryUnusableLink(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	mustCreate(t, env.s, "alice", models.RoleAdmin)

	_, used, _ := env.s.Invite(ctx, cli, "zq-used", models.RoleCaptain, nil)
	if _, err := env.s.RedeemLink(ctx, used.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	_, revoked, _ := env.s.Invite(ctx, cli, "zq-revoked", models.RoleCaptain, nil)
	if _, err := env.s.RevokeLink(ctx, cli, revoked.ID); err != nil {
		t.Fatal(err)
	}
	_, superseded, _ := env.s.Invite(ctx, cli, "zq-superseded", models.RoleCaptain, nil)
	if _, err := env.s.IssueLink(ctx, cli, superseded.UserID, LinkInvite); err != nil {
		t.Fatal(err)
	}
	_, disabled, _ := env.s.Invite(ctx, cli, "zq-disabled", models.RoleCaptain, nil)
	if _, err := env.s.SetActive(ctx, cli, disabled.UserID, false); err != nil {
		t.Fatal(err)
	}
	expiredAccount := mustCreate(t, env.s, "zq-expired", models.RoleCaptain)
	expired, err := env.s.IssueLink(ctx, cli, expiredAccount.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	env.clock.advance(RecoveryLinkLifetime + time.Minute)
	_, never, _ := newLinkToken()

	cases := map[string]string{
		"never existed": LinkPath(never),
		"malformed":     LinkPath("not-a-token"),
		"used":          used.Path(),
		"revoked":       revoked.Path(),
		"superseded":    superseded.Path(),
		"disabled":      disabled.Path(),
		"expired":       expired.Path(),
		"extra path":    used.Path() + "/something/else",
	}
	var wantBody string
	i := 0
	for name, path := range cases {
		i++
		// A different client address each time, so the throttle stays out of it.
		rec := env.do(http.MethodGet, path, "198.51.100."+string(rune('0'+i)), nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", name, rec.Code)
		}
		checkHeaders(t, name, rec)
		if wantBody == "" {
			wantBody = rec.Body.String()
			if !strings.Contains(wantBody, `data-page="account-link-invalid"`) {
				t.Fatalf("%s: not the invalid-link page", name)
			}
		} else if rec.Body.String() != wantBody {
			t.Errorf("%s: the response differs from the other unusable links", name)
		}
	}
	if strings.Contains(wantBody, "zq-") {
		t.Error("the invalid-link page names an account")
	}
	// The JSON endpoints give one answer too.
	for name, path := range cases {
		if name == "extra path" {
			continue
		}
		i++
		rec := env.do(http.MethodPost, path+"/passkey/begin", "198.51.101."+string(rune('0'+i%10)), nil, "Content-Type", "application/json")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s: passkey begin on an unusable link: %d %s", name, rec.Code, rec.Body.String())
		}
		checkHeaders(t, name+" passkey begin", rec)
	}
}

func TestLinkGuessingIsThrottledPerAddress(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	_, link, err := env.s.Invite(ctx, cli, "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	attacker, holder := "203.0.113.66", "198.51.100.7"

	for i := 0; i < linkFailureLimit; i++ {
		token, _, _ := newLinkToken()
		if rec := env.do(http.MethodGet, LinkPath(token), attacker, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("guess %d: %d", i, rec.Code)
		}
	}
	// The guesser is now refused, even with a real link, on every endpoint.
	token, _, _ := newLinkToken()
	for _, path := range []string{LinkPath(token), link.Path()} {
		rec := env.do(http.MethodGet, path, attacker, nil)
		if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `data-page="account-link-throttled"`) {
			t.Fatalf("after %d guesses GET: %d", linkFailureLimit, rec.Code)
		}
		checkHeaders(t, "throttled", rec)
	}
	if rec := env.do(http.MethodPost, link.Path(), attacker, passwordForm(goodPassword)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled POST: %d", rec.Code)
	}
	if rec := env.do(http.MethodPost, link.Path()+"/passkey/begin", attacker, nil, "Content-Type", "application/json"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled passkey begin: %d", rec.Code)
	}
	// The link's holder, at another address, is not affected at all.
	if rec := env.do(http.MethodGet, link.Path(), holder, nil); rec.Code != http.StatusOK {
		t.Fatalf("the holder was blocked by someone else's guessing: %d", rec.Code)
	}
	// The block lifts when the window has passed.
	env.clock.advance(linkFailureWindow + time.Second)
	if rec := env.do(http.MethodGet, link.Path(), attacker, nil); rec.Code != http.StatusOK {
		t.Fatalf("after the window: %d", rec.Code)
	}
	// Mistyping a password is not a guess at a link.
	for i := 0; i < linkFailureLimit+2; i++ {
		env.do(http.MethodPost, link.Path(), holder, passwordForm("short"))
	}
	if rec := env.do(http.MethodPost, link.Path(), holder, passwordForm(goodPassword)); rec.Code != http.StatusSeeOther {
		t.Fatalf("the holder was throttled for password mistakes: %d", rec.Code)
	}
}

// The token must never reach the log: not from the page, not from the
// account service's audit records, not from the catch-all or the cross-origin
// guard that sit around it.
func TestLinkTokenIsNeverLogged(t *testing.T) {
	env := newLinkEnv(t)
	ctx := context.Background()
	logs := captureLog(t)
	alice := mustCreate(t, env.s, "alice", models.RoleAdmin)

	var tokens []string
	_, invite, err := env.s.Invite(ctx, UserActor(alice.ID, "alice"), "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := env.s.IssueLink(ctx, CLIActor("user reset"), alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := env.s.IssueLink(ctx, cli, alice.ID, LinkRecovery) // cancels `recovery`
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.s.RevokeLink(ctx, cli, revoked.ID); err != nil {
		t.Fatal(err)
	}
	unknown, _, _ := newLinkToken()
	tokens = append(tokens, invite.Token, recovery.Token, revoked.Token, unknown)

	for i, token := range tokens {
		// One address per link, so the throttle does not hide later requests.
		ip := fmt.Sprintf("198.51.100.%d", 10+i)
		path := LinkPath(token)
		env.do(http.MethodGet, path, ip, nil)
		env.do(http.MethodGet, path+"/", ip, nil)
		env.do(http.MethodGet, path+"/nothing/here", ip, nil)
		env.do(http.MethodDelete, path, ip, nil)
		env.do(http.MethodPost, path+"/passkey/begin", ip, nil, "Content-Type", "application/json")
		env.do(http.MethodPost, path+"/passkey/finish", ip, nil, "Content-Type", "application/json")
		env.do(http.MethodPost, path, ip, passwordForm("short"))
		env.do(http.MethodPost, path, ip, passwordForm("cross site secret"), "Sec-Fetch-Site", "cross-site", "Origin", "https://evil.example")
		env.do(http.MethodGet, "/account/link"+"/../link/"+token, ip, nil)
	}
	if rec := env.do(http.MethodPost, invite.Path(), "198.51.100.77", passwordForm(goodPassword)); rec.Code != http.StatusSeeOther {
		t.Fatalf("use the invite: %d", rec.Code)
	}

	out := logs.String()
	for _, want := range []string{"action=link.create", "action=link.revoke", "action=link.use", "Blocked cross-origin POST /account/link/[redacted]"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %q, so this test is not exercising it", want)
		}
	}
	for _, token := range tokens {
		if strings.Contains(out, token) {
			t.Errorf("the log contains a link token")
		}
		if strings.Contains(out, hashLinkToken(token)) {
			t.Errorf("the log contains a link token's stored hash")
		}
		if strings.Contains(out, LinkBinding(token)) {
			t.Errorf("the log contains a link's ceremony binding")
		}
	}
	if strings.Contains(out, goodPassword) {
		t.Error("the log contains the password")
	}
	if auth.RedactPath("/account/link/abc/passkey/begin") != "/account/link/[redacted]" || auth.RedactPath("/admin/league") != "/admin/league" {
		t.Error("RedactPath")
	}
}
