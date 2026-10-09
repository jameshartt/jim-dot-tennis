// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// A page on another site must not be able to drive a logged-in (or logging
// in) browser into state-changing requests. Same-site browser requests and
// non-browser clients (no Origin / Sec-Fetch-Site headers) still get through.
func TestCrossOriginGuard(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('captain', ?, 'admin')`, string(hash)); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	mux := http.NewServeMux()
	NewHandler(s, t.TempDir(), "/admin/league").RegisterRoutes(mux)
	var mutations int
	mux.HandleFunc("/admin/league/seasons/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mutations++
		}
		w.WriteHeader(http.StatusNoContent)
	})
	app := CrossOriginGuard(mux)

	send := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{"username": {"captain"}, "password": {"right-password"}}
		r := httptest.NewRequest(method, "https://jim.tennis"+path, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, r)
		return rec
	}
	sessionCookie := func(rec *httptest.ResponseRecorder) bool {
		for _, c := range rec.Result().Cookies() {
			if c.Name == s.config.CookieName && c.Value != "" {
				return true
			}
		}
		return false
	}

	crossSite := []map[string]string{
		{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"},
		{"Origin": "https://evil.example"}, // older browser: Origin only
	}
	for _, h := range crossSite {
		if rec := send("POST", "/admin/league/seasons/delete", h); rec.Code != http.StatusForbidden {
			t.Errorf("cross-site admin POST %v: status %d, want 403", h, rec.Code)
		}
		if rec := send("POST", "/login", h); rec.Code != http.StatusForbidden || sessionCookie(rec) {
			t.Errorf("cross-site login %v: status %d (session set: %v), want 403 without a session", h, rec.Code, sessionCookie(rec))
		}
	}
	if mutations != 0 {
		t.Errorf("%d cross-site POSTs reached the handler", mutations)
	}

	// Cross-site navigation (a plain link) is still fine.
	if rec := send("GET", "/admin/league/seasons/delete", crossSite[0]); rec.Code != http.StatusNoContent {
		t.Errorf("cross-site GET: status %d, want it allowed", rec.Code)
	}

	for _, h := range []map[string]string{
		{"Sec-Fetch-Site": "same-origin", "Origin": "https://jim.tennis"},
		{"Origin": "https://jim.tennis"},
		{}, // curl, server-to-server
	} {
		if rec := send("POST", "/admin/league/seasons/delete", h); rec.Code != http.StatusNoContent {
			t.Errorf("same-site POST %v: status %d, want it allowed", h, rec.Code)
		}
	}
	if rec := send("POST", "/login", map[string]string{"Sec-Fetch-Site": "same-origin"}); !sessionCookie(rec) {
		t.Errorf("same-origin login: status %d, no session cookie", rec.Code)
	}
}
