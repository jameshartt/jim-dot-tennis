// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

func repoDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.IsDir() {
			return filepath.Join(dir, name)
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not find %s", name)
	return ""
}

func newTestService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(repoDir(t, "migrations")); err != nil {
		t.Fatal(err)
	}
	return New(db), db
}

var cli = CLIActor("test")

const goodPassword = "correct horse battery"

// mustCreate makes an account the way a person gets one: invited, then the
// invite link used to set goodPassword.
func mustCreate(t *testing.T, s *Service, username string, role models.Role) Account {
	t.Helper()
	ctx := context.Background()
	_, link, err := s.Invite(ctx, cli, username, role, nil)
	if err != nil {
		t.Fatalf("invite %s: %v", username, err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: goodPassword}); err != nil {
		t.Fatalf("accept invite for %s: %v", username, err)
	}
	a, err := s.Get(ctx, link.UserID)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// captureLog collects everything written to the log (audit records included)
// until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &logs
}

func passwordHash(t *testing.T, db *database.DB, userID int64) string {
	t.Helper()
	var u models.User
	if err := db.Get(&u, `SELECT * FROM users WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	return string(u.PasswordHash)
}

// addSession inserts a valid session for userID whose cookie token is token
// (only its hash is stored, as the server does).
func addSession(t *testing.T, db *database.DB, token string, userID int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sessions (token_hash, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
		VALUES (?, ?, 'admin', CURRENT_TIMESTAMP, datetime('now', '+1 day'), CURRENT_TIMESTAMP, '', '', '', TRUE)`, auth.HashSessionToken(token), userID)
	if err != nil {
		t.Fatal(err)
	}
}

func validSessions(t *testing.T, db *database.DB, userID int64) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid = TRUE`, userID); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLastActiveAdministratorGuard(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	captain := mustCreate(t, s, "carol", models.RoleCaptain)

	if _, err := s.SetActive(ctx, cli, alice.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable the only administrator: err %v, want ErrLastAdmin", err)
	}
	if _, err := s.SetRole(ctx, cli, alice.ID, models.RoleCaptain); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote the only administrator: err %v, want ErrLastAdmin", err)
	}
	if a, _ := s.Get(ctx, alice.ID); !a.IsActiveAdmin() {
		t.Fatalf("alice changed after refused operations: %+v", a)
	}

	// A disabled administrator does not count.
	bob := mustCreate(t, s, "bob", models.RoleAdmin)
	if _, err := s.SetActive(ctx, cli, bob.ID, false); err != nil {
		t.Fatalf("disable bob while alice is active: %v", err)
	}
	if _, err := s.SetActive(ctx, cli, alice.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable alice with only a disabled second admin: err %v, want ErrLastAdmin", err)
	}
	// Demoting a disabled administrator is fine.
	if _, err := s.SetRole(ctx, cli, bob.ID, models.RoleCaptain); err != nil {
		t.Fatalf("demote disabled bob: %v", err)
	}

	// With a second active administrator both changes are allowed.
	if _, err := s.SetRole(ctx, cli, captain.ID, models.RoleAdmin); err != nil {
		t.Fatalf("promote carol: %v", err)
	}
	if _, err := s.SetRole(ctx, cli, alice.ID, models.RoleCaptain); err != nil {
		t.Fatalf("demote alice with carol active: %v", err)
	}
	if _, err := s.SetActive(ctx, cli, captain.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable carol, now the last admin: err %v, want ErrLastAdmin", err)
	}
	// Disabling and enabling a non-administrator always works.
	if _, err := s.SetActive(ctx, cli, alice.ID, false); err != nil {
		t.Fatalf("disable captain alice: %v", err)
	}
	if a, err := s.SetActive(ctx, cli, alice.ID, true); err != nil || !a.IsActive {
		t.Fatalf("enable alice: %+v, %v", a, err)
	}
}

func TestDisableEndsSessions(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	mustCreate(t, s, "alice", models.RoleAdmin)
	p := mustCreate(t, s, "pat", models.RoleCaptain)
	addSession(t, db, "s1", p.ID)
	if _, err := s.SetActive(ctx, cli, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if n := validSessions(t, db, p.ID); n != 0 {
		t.Fatalf("%d sessions still valid after disable", n)
	}
}

// TestChangeOwnPasswordPage: a signed-in user with a password changes it at
// /account/password; nobody is sent there by force any more.
func TestChangeOwnPasswordPage(t *testing.T) {
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleAdmin)
	// A leftover must-change mark (retired by migration 034) is ignored.
	if _, err := db.Exec(`UPDATE users SET must_change_password = TRUE WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}

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

	mw := auth.NewMiddleware(authService, nil, nil)
	protected := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, withCookies(httptest.NewRequest(http.MethodGet, "/admin/league/dashboard", nil)))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("protected page: %d %q, want it served (no forced password change)", rec.Code, rec.Header().Get("Location"))
	}

	page := NewPasswordPage(s, authService, repoDir(t, "templates"), "/admin/league")
	mux := http.NewServeMux()
	page.RegisterRoutes(mux)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, withCookies(httptest.NewRequest(http.MethodGet, auth.ChangePasswordPath, nil)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Current password") {
		t.Fatalf("GET page: %d", rec.Code)
	}

	post := func(current, next, confirm string) *httptest.ResponseRecorder {
		form := url.Values{"current_password": {current}, "new_password": {next}, "new_password_confirm": {confirm}}
		r := httptest.NewRequest(http.MethodPost, auth.ChangePasswordPath, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, withCookies(r))
		return rec
	}
	if rec := post("wrong password", "a brand new secret", "a brand new secret"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong current password: %d", rec.Code)
	}
	if rec := post(goodPassword, goodPassword, goodPassword); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("same password again: %d", rec.Code)
	}
	if rec := post(goodPassword, "short", "short"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password: %d", rec.Code)
	}
	rec = post(goodPassword, "a brand new secret", "a brand new secret")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/league" {
		t.Fatalf("change password: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if !auth.PasswordMatches(passwordHash(t, db, a.ID), "a brand new secret") {
		t.Fatal("password not changed")
	}

	// The old session was ended; the new cookie gets through.
	rec2 := httptest.NewRecorder()
	protected.ServeHTTP(rec2, withCookies(httptest.NewRequest(http.MethodGet, "/admin/league/dashboard", nil)))
	if rec2.Code == http.StatusTeapot {
		t.Fatal("old session still works after the password change")
	}
	cookies = rec.Result().Cookies()
	rec2 = httptest.NewRecorder()
	protected.ServeHTTP(rec2, withCookies(httptest.NewRequest(http.MethodGet, "/admin/league/dashboard", nil)))
	if rec2.Code != http.StatusTeapot {
		t.Fatalf("new session after password change: %d %q", rec2.Code, rec2.Header().Get("Location"))
	}

	// An account with no password has nothing to change here.
	_, link, err := s.Invite(context.Background(), cli, "keyonly", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeOwnPassword(context.Background(), link.UserID, "", "a brand new secret"); !errors.Is(err, ErrNoPassword) {
		t.Fatalf("change password of a passwordless account: %v, want ErrNoPassword", err)
	}
}
