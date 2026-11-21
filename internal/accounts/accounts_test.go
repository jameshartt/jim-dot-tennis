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

func mustCreate(t *testing.T, s *Service, username string, role models.Role) Account {
	t.Helper()
	a, err := s.Create(context.Background(), cli, username, role, goodPassword, false)
	if err != nil {
		t.Fatalf("create %s: %v", username, err)
	}
	return a
}

// addSession inserts a valid session for userID.
func addSession(t *testing.T, db *database.DB, id string, userID int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sessions (id, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
		VALUES (?, ?, 'admin', CURRENT_TIMESTAMP, datetime('now', '+1 day'), CURRENT_TIMESTAMP, '', '', '', TRUE)`, id, userID)
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
	if _, err := s.SetRole(ctx, cli, bob.ID, models.RolePlayer); err != nil {
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
	p := mustCreate(t, s, "pat", models.RolePlayer)
	addSession(t, db, "s1", p.ID)
	if _, err := s.SetActive(ctx, cli, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if n := validSessions(t, db, p.ID); n != 0 {
		t.Fatalf("%d sessions still valid after disable", n)
	}
}

func TestResetWithTemporaryPassword(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleAdmin)
	other := mustCreate(t, s, "bob", models.RoleCaptain)
	addSession(t, db, "a1", a.ID)
	addSession(t, db, "a2", a.ID)
	addSession(t, db, "b1", other.ID)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	temp, ended, err := s.ResetWithTemporaryPassword(ctx, cli, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ended != 2 {
		t.Errorf("ended %d sessions, want 2", ended)
	}
	if n := validSessions(t, db, a.ID); n != 0 {
		t.Errorf("%d of alice's sessions still valid", n)
	}
	if n := validSessions(t, db, other.ID); n != 1 {
		t.Errorf("bob's session was ended too")
	}
	var u models.User
	if err := db.Get(&u, `SELECT * FROM users WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}
	if !u.MustChangePassword {
		t.Error("must_change_password not set after reset")
	}
	if !auth.PasswordMatches(u.PasswordHash, temp) || auth.PasswordMatches(u.PasswordHash, goodPassword) {
		t.Error("stored hash does not match the temporary password only")
	}
	out := logs.String()
	if !strings.Contains(out, "action=user.reset_password") || !strings.Contains(out, `target="alice"`) || !strings.Contains(out, `actor="cli:test"`) {
		t.Errorf("audit line missing fields: %q", out)
	}
	if strings.Contains(out, temp) {
		t.Error("audit line contains the temporary password")
	}
}

func TestCreateRules(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	mustCreate(t, s, "Alice", models.RoleAdmin)
	if _, err := s.Create(ctx, cli, "alice", models.RolePlayer, goodPassword, false); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("case-insensitive duplicate: err %v", err)
	}
	var bad *InvalidUsernameError
	if _, err := s.Create(ctx, cli, "no spaces", models.RolePlayer, goodPassword, false); !errors.As(err, &bad) {
		t.Errorf("bad username: err %v", err)
	}
	if _, err := s.Create(ctx, cli, "pat", models.RolePlayer, "short", false); !errors.Is(err, auth.ErrPasswordPolicy) {
		t.Errorf("weak password: err %v", err)
	}
	if _, err := s.Create(ctx, cli, "pat", models.Role("owner"), goodPassword, false); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("bad role: err %v", err)
	}
	a, temp, err := s.CreateWithTemporaryPassword(ctx, cli, "pat", models.RoleCaptain)
	if err != nil || !a.MustChangePassword || a.Role != models.RoleCaptain || !a.IsActive || temp == "" {
		t.Fatalf("create with temporary password: %+v %v", a, err)
	}
	if found, err := s.Find(ctx, "PAT"); err != nil || found.ID != a.ID {
		t.Errorf("find ignoring case: %+v %v", found, err)
	}
}

func TestTemporaryPasswordMeetsPolicy(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := TemporaryPassword("abcd")
		if err != nil {
			t.Fatal(err)
		}
		if err := auth.ValidatePassword(p, "abcd"); err != nil {
			t.Fatalf("%q: %v", p, err)
		}
		if len(p) != 19 || seen[p] {
			t.Fatalf("unexpected password %q", p)
		}
		seen[p] = true
	}
}

// TestMustChangePasswordFlow: a reset account is sent to the change-password
// page on every signed-in page until it chooses its own password.
func TestMustChangePasswordFlow(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleAdmin)
	temp, _, err := s.ResetWithTemporaryPassword(ctx, cli, a.ID)
	if err != nil {
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
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != auth.ChangePasswordPath {
		t.Fatalf("protected page: %d %q, want redirect to %s", rec.Code, rec.Header().Get("Location"), auth.ChangePasswordPath)
	}

	page := NewPasswordPage(s, authService, repoDir(t, "templates"), "/admin/league")
	mux := http.NewServeMux()
	page.RegisterRoutes(mux)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, withCookies(httptest.NewRequest(http.MethodGet, auth.ChangePasswordPath, nil)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "someone else gave you") {
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
	if rec := post(temp, temp, temp); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("same password again: %d", rec.Code)
	}
	rec = post(temp, "a brand new secret", "a brand new secret")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/league" {
		t.Fatalf("change password: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	var u models.User
	if err := db.Get(&u, `SELECT * FROM users WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}
	if u.MustChangePassword || !auth.PasswordMatches(u.PasswordHash, "a brand new secret") {
		t.Fatal("password not changed or must-change still set")
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
}
