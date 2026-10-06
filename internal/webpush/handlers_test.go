// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

// findMigrationsPath walks up from the test's working directory to locate the
// repo-root migrations directory.
func findMigrationsPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate migrations directory")
	return ""
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "push_test.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPath(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(db)
}

// stubAdminGate stands in for authMiddleware.RequireAuth + RequireRole("admin"):
// requests carrying the X-Test-Admin header pass, everything else gets 401.
func stubAdminGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Admin") == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newTestMux(t *testing.T) (*Service, *http.ServeMux) {
	t.Helper()
	s := newTestService(t)
	mux := http.NewServeMux()
	s.SetupHandlers(mux, stubAdminGate)
	return s, mux
}

func serve(mux *http.ServeMux, method, path, host, body string, admin bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if host != "" {
		req.Host = host
	}
	if admin {
		req.Header.Set("X-Test-Admin", "1")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// An anonymous caller must not be able to broadcast a push to every subscriber.
func TestTestPushBroadcastRequiresAdmin(t *testing.T) {
	_, mux := newTestMux(t)

	rec := serve(mux, http.MethodPost, "/api/push/test", "", `{"message":"pwned"}`, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous broadcast: got %d, want 401", rec.Code)
	}

	rec = serve(mux, http.MethodPost, "/api/push/test", "", `{"message":"hi"}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin broadcast: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// The VAPID reset used to trust r.Host == "localhost", which any client can
// send. Rotating keys orphans every subscription, so it must be admin-only.
func TestVAPIDResetRejectsSpoofedLocalhost(t *testing.T) {
	s, mux := newTestMux(t)
	before, _, err := s.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	for _, host := range []string{"localhost", "127.0.0.1:8080", "jim.tennis"} {
		rec := serve(mux, http.MethodPost, "/api/vapid-reset", host, "", false)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous reset with Host %q: got %d, want 401", host, rec.Code)
		}
	}
	after, _, err := s.GetVAPIDKeys()
	if err != nil {
		t.Fatalf("get keys: %v", err)
	}
	if after != before {
		t.Fatal("VAPID keys were rotated by an anonymous request")
	}

	rec := serve(mux, http.MethodPost, "/api/vapid-reset", "jim.tennis", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin reset: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// /api/push/status had no client and only served as a token-validity oracle.
func TestPushStatusOracleRemoved(t *testing.T) {
	_, mux := newTestMux(t)
	rec := serve(mux, http.MethodGet, "/api/push/status?playerToken=Federer_Nadal_Djokovic_Murray", "", "", false)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status oracle: got %d, want 404", rec.Code)
	}
}

// Player self-service routes stay public: the fantasy token in the body is the
// credential (availability page "test my notifications" button).
func TestPlayerPushRoutesStayPublic(t *testing.T) {
	_, mux := newTestMux(t)

	rec := serve(mux, http.MethodGet, "/api/vapid-public-key", "", "", false)
	if rec.Code != http.StatusOK {
		t.Errorf("vapid-public-key: got %d, want 200", rec.Code)
	}
	rec = serve(mux, http.MethodPost, "/api/push/test-player", "", `{"playerToken":"Federer_Nadal_Djokovic_Murray"}`, false)
	if rec.Code != http.StatusOK {
		t.Errorf("test-player: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}
