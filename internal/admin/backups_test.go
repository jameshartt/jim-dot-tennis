// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

// fakeProver stands in for auth.Passkeys.ProveOwnership.
type fakeProver struct {
	result    string
	err       error
	calls     int
	available bool   // passkeys work on the address
	password  string // the password the last call was given
}

func (f *fakeProver) ProveOwnership(_ http.ResponseWriter, _ *http.Request, _ *models.User, password string) (string, error) {
	f.calls++
	f.password = password
	return f.result, f.err
}

func (f *fakeProver) Available(*http.Request) bool { return f.available }

func TestBackupsPageAndDownload(t *testing.T) {
	e := newUsersTestEnv(t, "")
	dir := t.TempDir()
	cfg := backup.Config{DBPath: filepath.Join(dir, "tennis.db"), Dir: filepath.Join(dir, "backups"), Keep: 3, Interval: 24 * time.Hour}
	prover := &fakeProver{}
	h := &BackupsHandler{backups: backup.NewService(e.env.db, cfg), prover: prover, accounts: e.svc, templateDir: findTemplatesPathAdmin(t)}
	serve := func(method, target string, role models.Role, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if form != nil {
			req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, target, nil)
		}
		ctx := config.WithBranding(req.Context(), e.env.holder.Branding())
		ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: role})
		rec := httptest.NewRecorder()
		if method == http.MethodPost {
			h.HandleDownload(rec, req.WithContext(ctx))
		} else {
			h.HandleBackups(rec, req.WithContext(ctx))
		}
		return rec
	}

	// No backup yet: a warning.
	rec := serve(http.MethodGet, backupsPath, models.RoleAdmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "There is no backup on the server yet") {
		t.Fatalf("page with no backup: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `action="/account/backup"`) || !strings.Contains(rec.Body.String(), `name="current_password"`) {
		t.Error("no download form with a password field")
	}
	if rec := serve(http.MethodGet, backupsPath, models.RoleCaptain, nil); rec.Code != http.StatusForbidden {
		t.Errorf("captain page: %d", rec.Code)
	}

	// A recent backup: its time and no warning; an old one: a warning.
	made, err := backup.Create(context.Background(), e.env.db, cfg, backup.KindManual)
	if err != nil {
		t.Fatal(err)
	}
	body := serve(http.MethodGet, backupsPath, models.RoleAdmin, nil).Body.String()
	if !strings.Contains(body, "The last backup was made on") || strings.Contains(body, "backups may have stopped") {
		t.Errorf("page with a fresh backup:\n%s", body)
	}
	old := filepath.Join(cfg.Dir, "jim-dot-tennis-backup-20200101T000000Z.tar.gz")
	if err := os.Rename(made.Path, old); err != nil {
		t.Fatal(err)
	}
	body = serve(http.MethodGet, backupsPath, models.RoleAdmin, nil).Body.String()
	if !strings.Contains(body, "backups may have stopped working") {
		t.Errorf("no warning for a stale backup:\n%s", body)
	}
	// A file named as if made in the future does not hide the warning.
	future := filepath.Join(cfg.Dir, "jim-dot-tennis-backup-20990101T000000Z.tar.gz")
	os.WriteFile(future, []byte("x"), 0o600)
	body = serve(http.MethodGet, backupsPath, models.RoleAdmin, nil).Body.String()
	if !strings.Contains(body, "backups may have stopped working") {
		t.Errorf("a future-dated file name hid the stale warning:\n%s", body)
	}
	os.Remove(future)

	// Download: refused without proof, with a wrong password, for a captain.
	prover.err = auth.ErrProofRequired
	if rec := serve(http.MethodPost, BackupDownloadPath, models.RoleAdmin, url.Values{}); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=proof") {
		t.Errorf("no proof: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	prover.err = auth.ErrWrongCurrentPassword
	if rec := serve(http.MethodPost, BackupDownloadPath, models.RoleAdmin, url.Values{"current_password": {"nope"}}); !strings.Contains(rec.Header().Get("Location"), "error=password") {
		t.Errorf("wrong password: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	calls := prover.calls
	if rec := serve(http.MethodPost, BackupDownloadPath, models.RoleCaptain, url.Values{}); rec.Code != http.StatusForbidden || prover.calls != calls {
		t.Errorf("captain download: %d", rec.Code)
	}

	// With proof: a verified archive, recorded in the audit log, and nothing
	// left in the backups folder.
	prover.err, prover.result = nil, auth.ProofPassword
	rec = serve(http.MethodPost, BackupDownloadPath, models.RoleAdmin, url.Values{"current_password": {"right"}})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/gzip" ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("download: %d %v", rec.Code, rec.Header())
	}
	got := filepath.Join(t.TempDir(), "got.tar.gz")
	os.WriteFile(got, rec.Body.Bytes(), 0o600)
	if m, err := backup.VerifyFile(got); err != nil || m.Kind != backup.KindDownload {
		t.Errorf("downloaded archive: %+v %v", m, err)
	}
	entries, _ := os.ReadDir(cfg.Dir)
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), "jim-dot-tennis-download-") || strings.HasPrefix(en.Name(), ".tmp-") {
			t.Errorf("left in the backups folder: %s", en.Name())
		}
	}
	entriesLog, _, err := e.svc.AuditPage(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesLog) == 0 || entriesLog[0].Action != accounts.ActionBackupDownload || !strings.Contains(entriesLog[0].Detail, "reauth=password") {
		t.Errorf("download not audited: %+v", entriesLog)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("<html")) {
		t.Error("download looks like a page")
	}
}

// The download route is for administrators only, through the real
// middleware, and only by POST.
func TestBackupDownloadRouteNeedsAdmin(t *testing.T) {
	m := newRoleMatrixEnv(t)
	if rec := m.do(http.MethodPost, BackupDownloadPath, models.RoleCaptain); rec.Code != http.StatusForbidden {
		t.Errorf("captain: %d", rec.Code)
	}
	if rec := m.do(http.MethodPost, BackupDownloadPath, ""); rec.Code == http.StatusOK {
		t.Errorf("anonymous: %d", rec.Code)
	}
	if rec := m.do(http.MethodGet, BackupDownloadPath, models.RoleAdmin); rec.Code == http.StatusOK {
		t.Errorf("GET: %d", rec.Code)
	}
	// An administrator reaches the handler (no backup service in this test env).
	if rec := m.do(http.MethodPost, BackupDownloadPath, models.RoleAdmin); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("admin: %d", rec.Code)
	}
}

// While "Require passkeys for administrators" is on (and passkeys work), the
// download takes only the passkey confirmation: the page has no password
// field and a password sent anyway is refused without being checked.
func TestBackupDownloadNeedsPasskeyWhenRequired(t *testing.T) {
	e := newUsersTestEnv(t, "")
	dir := t.TempDir()
	cfg := backup.Config{DBPath: filepath.Join(dir, "tennis.db"), Dir: filepath.Join(dir, "backups"), Keep: 3, Interval: 24 * time.Hour}
	prover := &fakeProver{available: true, result: auth.ProofPassword}
	h := &BackupsHandler{backups: backup.NewService(e.env.db, cfg), prover: prover, accounts: e.svc, templateDir: findTemplatesPathAdmin(t)}
	serve := func(method string, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if form != nil {
			req = httptest.NewRequest(method, BackupDownloadPath, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, backupsPath, nil)
		}
		ctx := config.WithBranding(req.Context(), e.env.holder.Branding())
		ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: models.RoleAdmin})
		rec := httptest.NewRecorder()
		if method == http.MethodPost {
			h.HandleDownload(rec, req.WithContext(ctx))
		} else {
			h.HandleBackups(rec, req.WithContext(ctx))
		}
		return rec
	}
	if _, err := e.env.db.Exec(`INSERT INTO app_settings (key, value) VALUES ('require_admin_passkeys', 'true')`); err != nil {
		t.Fatal(err)
	}

	if body := serve(http.MethodGet, nil).Body.String(); strings.Contains(body, `name="current_password"`) {
		t.Error("the page offers a password while passkeys are required for administrators")
	}
	rec := serve(http.MethodPost, url.Values{"current_password": {"right"}})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=passkey-required") || prover.calls != 0 {
		t.Errorf("password while passkeys are required: %d %s (prover calls %d)", rec.Code, rec.Header().Get("Location"), prover.calls)
	}
	prover.result = auth.ProofPasskey
	if rec := serve(http.MethodPost, url.Values{}); rec.Code != http.StatusOK || prover.password != "" {
		t.Errorf("passkey confirmation: %d (password passed %q)", rec.Code, prover.password)
	}

	// Where passkeys do not work on the address, the password is accepted.
	prover.available, prover.result = false, auth.ProofPassword
	if body := serve(http.MethodGet, nil).Body.String(); !strings.Contains(body, `name="current_password"`) {
		t.Error("no password field where passkeys are unavailable")
	}
	if rec := serve(http.MethodPost, url.Values{"current_password": {"right"}}); rec.Code != http.StatusOK || prover.password != "right" {
		t.Errorf("password where passkeys are unavailable: %d", rec.Code)
	}
}
