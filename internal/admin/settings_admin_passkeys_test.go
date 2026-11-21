// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func TestSettingsRequireAdminPasskeys(t *testing.T) {
	e := newSettingsTestEnv(t)
	ctx := context.Background()
	if _, err := e.db.Exec(`INSERT INTO users (id, username, password_hash, role) VALUES (1, 'someone', 'x', 'admin'), (2, 'zoe', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	toggle := func(v string, role models.Role) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, settingsAdminPasskeysPath, strings.NewReader(url.Values{"require": {v}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return e.do(req, role)
	}
	page := func(query string) string {
		return e.do(httptest.NewRequest(http.MethodGet, settingsPath+query, nil), models.RoleAdmin).Body.String()
	}

	if rec := toggle("on", models.RoleCaptain); rec.Code != http.StatusForbidden {
		t.Fatalf("captain toggles: %d", rec.Code)
	}
	body := page("")
	for _, s := range []string{`data-admin-passkeys="off"`, `id="admin-passkeys-on"`, `data-admin-passkeys-missing`, "someone", "zoe", `data-admin-passkeys-no-url`} {
		if !strings.Contains(body, s) {
			t.Errorf("settings page lacks %s", s)
		}
	}

	// Refused while administrators lack a passkey, naming them.
	rec := toggle("on", models.RoleAdmin)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "passkey_error=") || !strings.Contains(loc, "zoe") {
		t.Fatalf("enable refused: %d %q", rec.Code, loc)
	}
	u, _ := url.Parse(loc)
	if body := page("?" + u.RawQuery); !strings.Contains(body, "data-admin-passkeys-error") || !strings.Contains(body, "zoe") {
		t.Error("refusal not shown")
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, e.db); on {
		t.Fatal("enabled although refused")
	}

	for i, id := range []int{1, 2} {
		if _, err := e.db.Exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES (?, ?, x'00', 'k')`, id, []byte{byte(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	if rec := toggle("on", models.RoleAdmin); rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "passkey_error") {
		t.Fatalf("enable: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, e.db); !on {
		t.Fatal("not enabled")
	}
	if body := page(""); !strings.Contains(body, `data-admin-passkeys="on"`) || !strings.Contains(body, `id="admin-passkeys-off"`) {
		t.Error("page does not show the requirement on")
	}

	// While on, the public URL cannot be changed (passkeys would stop working).
	rec = e.do(settingsForm(t, validSettingsFields()), models.RoleAdmin)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "can&#39;t be changed while passkeys are required") {
		t.Fatalf("public URL change while required: %d", rec.Code)
	}

	if rec := toggle("off", models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("disable: %d", rec.Code)
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, e.db); on {
		t.Fatal("still on")
	}
	if rec := e.do(settingsForm(t, validSettingsFields()), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("public URL change with the requirement off: %d", rec.Code)
	}
}
