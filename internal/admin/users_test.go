// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

type usersTestEnv struct {
	t       *testing.T
	env     settingsTestEnv
	handler *UsersHandler
	svc     *accounts.Service
	admin   accounts.Account
}

const usersTestPassword = "correct horse battery"

func newUsersTestEnv(t *testing.T, publicURL string) *usersTestEnv {
	t.Helper()
	env := newSettingsTestEnv(t)
	if publicURL != "" {
		t.Setenv(config.EnvPublicURL, publicURL)
		env = newSettingsTestEnvOn(t, env.db)
	}
	svc := accounts.New(env.db)
	ctx := context.Background()
	_, link, err := svc.Invite(ctx, accounts.CLIActor("test"), "boss", models.RoleAdmin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RedeemLink(ctx, link.Token, accounts.Redemption{Password: usersTestPassword}); err != nil {
		t.Fatal(err)
	}
	admin, err := svc.Get(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	h := NewUsersHandler(NewService(env.db, "", env.holder, ""), findTemplatesPathAdmin(t))
	return &usersTestEnv{t: t, env: env, handler: h, svc: svc, admin: admin}
}

func (e *usersTestEnv) do(method, path string, form url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Host = "request-host.example:8080"
	ctx := config.WithHomeClub(req.Context(), e.env.holder.ID(), e.env.holder.Club(), e.env.holder.LogoPath())
	ctx = config.WithBranding(ctx, e.env.holder.Branding())
	// The signed-in administrator as RequireAuth provides it, hash included.
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: models.RoleAdmin,
		PasswordHash: "$2a$12$this-is-the-signed-in-admins-hash"})
	rec := httptest.NewRecorder()
	e.handler.HandleUsers(rec, req.WithContext(ctx))
	return rec
}

var issuedLinkRE = regexp.MustCompile(`id="issued-link-url" readonly value="([^"]+)"`)

// issuedLink returns the link the page shows, and its token.
func issuedLink(t *testing.T, rec *httptest.ResponseRecorder) (link, token string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, Location %q, body %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	m := issuedLinkRE.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("no issued link on the page:\n%s", rec.Body.String())
	}
	return m[1], m[1][strings.LastIndex(m[1], "/")+1:]
}

func TestUsersPageInvitesAndRecoversWithLinks(t *testing.T) {
	e := newUsersTestEnv(t, "https://tennis.example.org")
	ctx := context.Background()

	// The page: no password field anywhere, no hashes, no reset-password action.
	rec := e.do(http.MethodGet, "/admin/league/users", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET users: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, banned := range []string{`type="password"`, "$2a$", "reset-password", "Reset Password", "data-issued-link"} {
		if strings.Contains(body, banned) {
			t.Errorf("users page contains %q", banned)
		}
	}
	for _, want := range []string{"data-user-create", `id="create-user-submit"`, `id="player_id"`, "data-user-recover", "Links not used yet (0)"} {
		if !strings.Contains(body, want) {
			t.Errorf("users page lacks %q", want)
		}
	}

	// Create: an account with no password, and its invite link shown on this
	// response only (no redirect that would put it in a URL).
	rec = e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"pat"}, "role": {"captain"}})
	link, token := issuedLink(t, rec)
	if !strings.HasPrefix(link, "https://tennis.example.org/account/link/") {
		t.Errorf("invite link %q is not built from the public URL", link)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "same-origin" || rec.Header().Get("Location") != "" {
		t.Errorf("headers of the response carrying the link: %v", rec.Header())
	}
	body = rec.Body.String()
	for _, want := range []string{`data-link-kind="invite"`, `data-link-username="pat"`, "shown only this once", "Invite not accepted", "data-user-invite", "Links not used yet (1)", "data-link-revoke"} {
		if !strings.Contains(body, want) {
			t.Errorf("page after creating a user lacks %q", want)
		}
	}
	invited, err := e.svc.LookupLink(ctx, token)
	if err != nil || invited.Username != "pat" || invited.Kind != accounts.LinkInvite || invited.CreatedBy != "user:boss(#1)" {
		t.Fatalf("the invite link shown: %+v %v", invited, err)
	}
	if pat, _ := e.svc.Find(ctx, "pat"); pat.SignInMethods() != 0 || pat.Role != models.RoleCaptain {
		t.Fatalf("invited account: %+v", pat)
	}
	// It is never shown again.
	if again := e.do(http.MethodGet, "/admin/league/users", nil).Body.String(); strings.Contains(again, token) || strings.Contains(again, "data-issued-link") {
		t.Error("the link is shown again on a later page load")
	}

	// A fresh invite for the pending account cancels the first.
	rec = e.do(http.MethodPost, "/admin/league/users/"+itoa(invited.UserID)+"/invite", url.Values{})
	_, token2 := issuedLink(t, rec)
	if _, err := e.svc.LookupLink(ctx, token); err == nil {
		t.Error("the first invite still works after a new one")
	}
	if !strings.Contains(rec.Body.String(), "earlier unused link") {
		t.Error("the page does not say the earlier link was cancelled")
	}

	// Recovery: a link, and nothing about the account changes.
	rec = e.do(http.MethodPost, "/admin/league/users/"+itoa(e.admin.ID)+"/recover", url.Values{})
	_, recovery := issuedLink(t, rec)
	if !strings.Contains(rec.Body.String(), `data-link-kind="recovery"`) {
		t.Error("not shown as a recovery link")
	}
	if l, err := e.svc.LookupLink(ctx, recovery); err != nil || l.Kind != accounts.LinkRecovery || l.UserID != e.admin.ID {
		t.Fatalf("recovery link: %+v %v", l, err)
	}
	var hash models.PasswordHash
	if err := e.env.db.Get(&hash, `SELECT password_hash FROM users WHERE id = ?`, e.admin.ID); err != nil || !auth.PasswordMatches(string(hash), usersTestPassword) {
		t.Fatalf("issuing a recovery link changed the password (%v)", err)
	}

	// An account that can sign in is not invited again.
	rec = e.do(http.MethodPost, "/admin/league/users/"+itoa(e.admin.ID)+"/invite", url.Values{})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("invite for an account with a password: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// Revoke.
	pending, err := e.svc.PendingLinks(ctx)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending links: %+v %v", pending, err)
	}
	for _, l := range pending {
		rec = e.do(http.MethodPost, "/admin/league/users/links/"+itoa(l.ID)+"/revoke", url.Values{})
		if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "success=") {
			t.Fatalf("revoke: %d %q", rec.Code, rec.Header().Get("Location"))
		}
	}
	for _, tok := range []string{token2, recovery} {
		if _, err := e.svc.LookupLink(ctx, tok); err == nil {
			t.Error("a revoked link still works")
		}
	}
	if rec := e.do(http.MethodPost, "/admin/league/users/links/"+itoa(pending[0].ID)+"/revoke", url.Values{}); !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("revoking twice: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// The old reset-password action is gone: an administrator cannot set
	// anyone's password, not even by posting to the old address.
	rec = e.do(http.MethodPost, "/admin/league/users/"+itoa(e.admin.ID)+"/reset-password", url.Values{"password": {"chosen by an admin 1"}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("old reset-password action: %d", rec.Code)
	}
	// A password posted with the create form is ignored.
	rec = e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"sam"}, "role": {"captain"}, "password": {"chosen by an admin 1"}})
	issuedLink(t, rec)
	if err := e.env.db.Get(&hash, `SELECT password_hash FROM users WHERE username = 'sam'`); err != nil || hash != "" {
		t.Errorf("an account created by an administrator has a password (%v)", err)
	}
	if err := e.env.db.Get(&hash, `SELECT password_hash FROM users WHERE id = ?`, e.admin.ID); err != nil || !auth.PasswordMatches(string(hash), usersTestPassword) {
		t.Error("the administrator's password was changed")
	}
}

// With no public URL (development only) the link uses the address the
// administrator is browsing at, and the page says so.
func TestUsersPageLinkWithoutPublicURL(t *testing.T) {
	e := newUsersTestEnv(t, "")
	rec := e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"pat"}, "role": {"captain"}})
	link, _ := issuedLink(t, rec)
	if !strings.HasPrefix(link, "http://request-host.example:8080/account/link/") {
		t.Errorf("link %q, want it built from the request host", link)
	}
	if !strings.Contains(rec.Body.String(), "No public URL is set") {
		t.Error("the page does not warn that no public URL is set")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
