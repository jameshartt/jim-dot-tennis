// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

// WI-134 I8: the planning dashboard's "I am..." link goes through the account
// service: a player already linked to another account is refused with a
// message (it used to be a 500 from the unique index), and the link is
// recorded in the audit log.
func TestPlanningLinkUsesTheAccountService(t *testing.T) {
	e := newUsersTestEnv(t, "")
	db := e.env.db
	db.MustExec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p-1', 'Pat', 'One', 1), ('p-2', 'Sam', 'Two', 1)`)
	db.MustExec(`INSERT INTO users (username, password_hash, role, player_id) VALUES ('other', 'x', 'captain', 'p-1')`)
	h := NewPlanningLinkHandler(NewService(db, "", e.env.holder, ""), findTemplatesPathAdmin(t))
	do := func(method string, target string, form url.Values) *httptest.ResponseRecorder {
		var req *http.Request
		if form != nil {
			req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, target, nil)
		}
		ctx := config.WithHomeClub(req.Context(), e.env.holder.ID(), e.env.holder.Club(), e.env.holder.LogoPath())
		ctx = config.WithBranding(ctx, e.env.holder.Branding())
		ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: models.RoleAdmin})
		rec := httptest.NewRecorder()
		h.HandleLink(rec, req.WithContext(ctx))
		return rec
	}
	linked := func() string {
		var id *string
		if err := db.Get(&id, `SELECT player_id FROM users WHERE id = ?`, e.admin.ID); err != nil {
			t.Fatal(err)
		}
		if id == nil {
			return ""
		}
		return *id
	}
	audits := func() int {
		var n int
		if err := db.Get(&n, `SELECT COUNT(*) FROM account_audit_log WHERE action = 'user.player_link' AND target_user_id = ?`, e.admin.ID); err != nil {
			t.Fatal(err)
		}
		return n
	}

	rec := do(http.MethodPost, "/admin/league/planning/link", url.Values{"player_id": {"p-1"}})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=taken") || linked() != "" {
		t.Fatalf("player linked to another account: status %d, Location %q, linked %q", rec.Code, rec.Header().Get("Location"), linked())
	}
	if page := do(http.MethodGet, rec.Header().Get("Location"), nil); !strings.Contains(page.Body.String(), "data-planning-link-error") {
		t.Error("the picker does not show why the link was refused")
	}
	rec = do(http.MethodPost, "/admin/league/planning/link", url.Values{"player_id": {"no-such-player"}})
	if !strings.Contains(rec.Header().Get("Location"), "error=unknown") {
		t.Errorf("unknown player: Location %q", rec.Header().Get("Location"))
	}
	if audits() != 0 {
		t.Fatal("a refused link was audited as done")
	}
	rec = do(http.MethodPost, "/admin/league/planning/link", url.Values{"player_id": {"p-2"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/league/planning" || linked() != "p-2" || audits() != 1 {
		t.Fatalf("link: status %d, Location %q, linked %q, audit rows %d", rec.Code, rec.Header().Get("Location"), linked(), audits())
	}
	rec = do(http.MethodPost, "/admin/league/planning/link", url.Values{"player_id": {""}})
	if rec.Code != http.StatusSeeOther || linked() != "" || audits() != 2 {
		t.Fatalf("unlink: linked %q, audit rows %d", linked(), audits())
	}
}

// WI-134 I8: the users, sessions and settings pages are never cached, and
// the users page (which can show a single-use link) sends no referrer to
// other sites while keeping its own forms working (I7).
func TestAccountAdminPagesAreNotCached(t *testing.T) {
	e := newUsersTestEnv(t, "")
	rec := e.do(http.MethodGet, "/admin/league/users", nil)
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "same-origin" {
		t.Errorf("users page headers: %v", rec.Header())
	}
	rec = e.env.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("settings page: status %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	sessions := NewSessionsHandler(NewService(e.env.db, "", e.env.holder, ""), findTemplatesPathAdmin(t))
	req := httptest.NewRequest(http.MethodGet, "/admin/league/sessions", nil)
	ctx := config.WithHomeClub(req.Context(), e.env.holder.ID(), e.env.holder.Club(), e.env.holder.LogoPath())
	ctx = config.WithBranding(ctx, e.env.holder.Branding())
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: models.RoleAdmin})
	rec = httptest.NewRecorder()
	sessions.HandleSessions(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("sessions page: status %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

// WI-134: the settings page's "passkeys can't be used" notice follows whether
// passkeys actually work at the request's address (they do on localhost in
// development with no public URL), not whether a public URL is stored.
func TestSettingsPasskeyNoticeFollowsAvailability(t *testing.T) {
	e := newSettingsTestEnv(t)
	page := func() string {
		return e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin).Body.String()
	}
	// No public URL and nothing wired: the notice shows.
	if !strings.Contains(page(), "data-admin-passkeys-no-url") {
		t.Fatal("no notice with no public URL")
	}
	// No public URL, but passkeys work here (DEV_MODE on localhost): no notice.
	e.handler.service.passkeysAvailable = func(*http.Request) bool { return true }
	if strings.Contains(page(), "data-admin-passkeys-no-url") {
		t.Error("the notice shows although passkeys are available at this address")
	}
	// Passkeys do not work at this address: the notice shows.
	e.handler.service.passkeysAvailable = func(*http.Request) bool { return false }
	if !strings.Contains(page(), "data-admin-passkeys-no-url") {
		t.Error("no notice although passkeys are unavailable at this address")
	}
}
