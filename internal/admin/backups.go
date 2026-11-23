// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Backups page and download.
//
// The page (GET backupsPath, administrators only, declared in roles.go) shows
// when the last backup was made, warns when it is older than the schedule
// allows, lists the backups kept on the server and offers "Download backup".
//
// The download is POST BackupDownloadPath. It is under /account/ rather than
// /admin/league/ because the passkey confirmation cookie (auth/reauth.go) is
// limited to /account/; the route is wrapped with RequireAuth and
// RequireRole(admin) in RegisterRoutes. A backup holds every password hash,
// passkey record, push key and the rest of the database, more than any page
// shows, so a signed-in session alone is not enough: like a change to how an
// account signs in, it needs the current password or a passkey confirmation
// made in the last few minutes (auth.Passkeys.ProveOwnership); while
// "Require passkeys for administrators" is on (and passkeys work on the
// address), only the passkey confirmation, since a password would not even
// sign an administrator in then. Someone at an
// administrator's unlocked laptop cannot take the whole database away
// without knowing the password or holding the passkey. POST plus the
// app-wide cross-origin guard keeps other sites from triggering it. Every
// download is written to the account audit log before it is sent.

const (
	backupsPath = "/admin/league/backups"
	// BackupDownloadPath is the download endpoint (see above).
	BackupDownloadPath = "/account/backup"
	// downloadWriteTime is how long sending a backup may take (the server's
	// usual write timeout is 30 seconds).
	downloadWriteTime = 15 * time.Minute
)

// ownershipProver checks the proof sent with a request (auth.Passkeys).
type ownershipProver interface {
	ProveOwnership(w http.ResponseWriter, r *http.Request, user *models.User, currentPassword string) (string, error)
	// Available reports whether passkeys work on r's address.
	Available(r *http.Request) bool
}

// BackupsHandler serves the backups page and the download.
type BackupsHandler struct {
	backups     *backup.Service
	prover      ownershipProver
	accounts    *accounts.Service
	templateDir string
}

// SetBackups gives the admin pages the server's backup service and the
// proof checker for downloads. Without it the backups page says backups are
// not set up.
func (h *Handler) SetBackups(b *backup.Service, prover ownershipProver) {
	h.backups.backups = b
	h.backups.prover = prover
}

// backupRow is one kept backup on the page.
type backupRow struct {
	Name         string
	Size         string
	CreatedAt    time.Time
	PreMigration bool
	PreUpgrade   bool // taken before a tournament server upgrade
	// Tournaments: the backup includes the tournament data.
	Tournaments bool
}

// backupsData is what templates/admin/backups.html renders.
type backupsData struct {
	User       *models.User
	Configured bool
	// TournamentsOn: the tournament stack is part of this deployment, so
	// every backup must include its data.
	TournamentsOn bool
	Archives      []backupRow
	HasBackup     bool
	LastAt        time.Time
	Stale         bool
	StaleAfter    string
	Schedule      string
	Keep          int
	Dir           string
	LastError     string
	Confirmed     bool
	HasPassword   bool
	PasskeyOnly   bool // administrators must use passkeys: no password field
	Error         string
	DownloadPath  string
}

// backupErrors are the messages for ?error= on the page.
var backupErrors = map[string]string{
	"proof":     "Confirm it's you first: type your current password, or confirm with a passkey, then download.",
	"password":  "That password is not right.",
	"throttled": "Too many wrong passwords. Wait a few minutes and try again.",
	// "Require passkeys for administrators" is on.
	"passkey-required": "Administrators must use a passkey here: confirm it's you with a passkey, then download.",
	"failed":           "The backup could not be made. The app's log says why.",
}

// HandleBackups serves GET /admin/league/backups.
func (h *BackupsHandler) HandleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != backupsPath {
		http.NotFound(w, r)
		return
	}
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.Role != models.RoleAdmin {
		auth.WriteForbidden(w, r)
		return
	}
	data := backupsData{
		User:         user,
		Confirmed:    r.URL.Query().Get("confirmed") == "1",
		Error:        backupErrors[r.URL.Query().Get("error")],
		DownloadPath: BackupDownloadPath,
	}
	if h.backups != nil {
		cfg := h.backups.Config()
		data.Configured = true
		data.Keep = max(cfg.Keep, 1)
		data.Dir = cfg.Dir
		data.StaleAfter = describeDuration(cfg.StaleAfter())
		data.Schedule = "off"
		if cfg.Interval > 0 {
			data.Schedule = "every " + describeDuration(cfg.Interval)
		}
		data.LastError = h.backups.Status().LastError
		list, err := backup.List(cfg.Dir)
		if err != nil {
			log.Printf("Backups page: cannot list %s: %v", cfg.Dir, err)
		}
		data.TournamentsOn = cfg.Tournament != nil
		for _, a := range list {
			row := backupRow{Name: a.Name, Size: describeSize(a.Size), CreatedAt: a.CreatedAt.Local(), PreMigration: a.PreMigration, PreUpgrade: a.PreUpgrade}
			// Only the manifest (the archive's first entry) is read.
			if m, err := backup.ReadManifest(a.Path); err == nil {
				row.Tournaments = m.HasTournaments()
			}
			data.Archives = append(data.Archives, row)
		}
		// The last backup is the newest one that can be believed: a file
		// named as if made in the future, or one that is not a readable
		// backup, does not hide that backups have stopped.
		if last, ok, err := backup.Newest(cfg.Dir, time.Now(), true); err == nil && ok {
			data.HasBackup = true
			data.LastAt = last.CreatedAt.Local()
		}
		data.Stale = !data.HasBackup || time.Since(data.LastAt) > cfg.StaleAfter()
		if err := h.backups.DB().GetContext(r.Context(), &data.HasPassword,
			`SELECT COALESCE(password_hash, '') <> '' FROM users WHERE id = ?`, user.ID); err != nil {
			log.Printf("Backups page: %v", err)
		}
		data.PasskeyOnly = h.passkeyOnly(r)
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/backups.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// HandleDownload serves POST /account/backup: proof, a fresh backup, an
// audit entry, then the file.
func (h *BackupsHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.Role != models.RoleAdmin {
		auth.WriteForbidden(w, r)
		return
	}
	if h.backups == nil || h.prover == nil {
		http.Error(w, "Backups are not set up", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	back := func(code string) { http.Redirect(w, r, backupsPath+"?error="+code, http.StatusSeeOther) }
	password := r.PostFormValue("current_password")
	if h.passkeyOnly(r) {
		// An administrator cannot sign in with a password now, so a
		// password does not prove enough to take the whole database away.
		if password != "" {
			back("passkey-required")
			return
		}
	}
	proof, err := h.prover.ProveOwnership(w, r, user, password)
	switch {
	case errors.Is(err, auth.ErrWrongCurrentPassword):
		back("password")
		return
	case errors.Is(err, auth.ErrProofThrottled):
		back("throttled")
		return
	case err != nil:
		back("proof")
		return
	}

	res, err := backup.CreateForDownload(r.Context(), h.backups.DB(), h.backups.Config())
	if err != nil {
		log.Printf("Backup for download failed: %v", err)
		back("failed")
		return
	}
	defer os.Remove(res.Path)
	detail := fmt.Sprintf("reauth=%s schema=%d size=%d", proof, res.Manifest.SchemaVersion, res.Size)
	if err := h.accounts.RecordEvent(r.Context(), accounts.UserActor(user.ID, user.Username), accounts.ActionBackupDownload, detail); err != nil {
		log.Printf("Backup download not sent: %v", err)
		back("failed")
		return
	}
	f, err := os.Open(res.Path)
	if err != nil {
		log.Printf("Backup download: %v", err)
		back("failed")
		return
	}
	defer f.Close()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(downloadWriteTime))
	hdr := w.Header()
	hdr.Set("Content-Type", "application/gzip")
	hdr.Set("Content-Disposition", `attachment; filename="`+filepath.Base(backup.DownloadName(res))+`"`)
	hdr.Set("Content-Length", strconv.FormatInt(res.Size, 10))
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// passkeyOnly reports whether a download must be confirmed with a passkey:
// "Require passkeys for administrators" is on and passkeys work on this
// request's address (the same condition under which an administrator's
// password sign-in is refused). A setting that cannot be read counts as on,
// as it does for sign-in.
func (h *BackupsHandler) passkeyOnly(r *http.Request) bool {
	if h.prover == nil || h.backups == nil || !h.prover.Available(r) {
		return false
	}
	on, err := repository.AdminPasskeysRequired(r.Context(), h.backups.DB())
	if err != nil {
		log.Printf("Backups: could not read the administrator passkey setting (treating it as on): %v", err)
		return true
	}
	return on
}

// describeDuration says a schedule length in words: "24 hours", "7 days".
func describeDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour && d%(24*time.Hour) == 0:
		return fmt.Sprintf("%d days", d/(24*time.Hour))
	case d >= time.Hour && d%time.Hour == 0:
		if d == time.Hour {
			return "hour"
		}
		return fmt.Sprintf("%d hours", d/time.Hour)
	}
	return d.String()
}

// describeSize says a file size in KB or MB.
func describeSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", (n+1023)/1024)
}
