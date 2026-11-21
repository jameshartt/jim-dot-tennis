// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/database"
)

var (
	// ErrServerRunning: restore was asked for while the app is running (or
	// another restore is in progress).
	ErrServerRunning = errors.New("the app is running")
	// ErrNewerBackup: the backup comes from a newer version of the app than
	// this program.
	ErrNewerBackup = errors.New("the backup is from a newer version of the app")
)

// RestoreResult says what Restore did.
type RestoreResult struct {
	Manifest         Manifest
	SchemaFrom       uint   // the backup's schema version
	SchemaTo         uint   // the restored database's schema version (after any upgrade)
	ReplacedDatabase string // where the database that was replaced is kept ("" if there was none)
	ReplacedUploads  string // where the uploaded files that were replaced are kept ("" if none)
	SessionsEnded    int64  // sessions in the backup that were ended
	LinksCancelled   int64  // unused invite and recovery links in the backup that were cancelled
}

// replacedMarker is in the names of what a restore set aside.
const replacedMarker = ".replaced-"

// Restore puts the backup in archivePath back as the instance's database and
// uploaded files. It refuses while the server is running (the data
// directory's lock is held). It reads the archive as untrusted input,
// verifies it against its manifest, refuses a backup from a newer schema
// than migrations holds, and upgrades an older one. Everything is built and
// checked in a staging folder beside the database first; the database is
// then swapped in with one rename, so the instance has either its old
// database or the restored one, never a mixture. The replaced database (with
// its WAL file, if it had unwritten changes) and uploaded files are kept
// aside, not deleted. On any failure before the swap, nothing of the
// original is changed.
//
// The restored database keeps no sessions: every session in the backup is
// ended (a session someone signed out of, or an administrator revoked,
// after the backup was made would otherwise work again), as is every unused
// invite or recovery link. Everyone signs in again. The restore is recorded
// in the restored database's audit log.
func Restore(ctx context.Context, cfg Config, archivePath string, migrations fs.FS) (RestoreResult, error) {
	var res RestoreResult
	dataDir := cfg.DataDir()
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return res, fmt.Errorf("the data folder %s (from DB_PATH=%s) does not exist. Mount the app's data volume there, or set DB_PATH", dataDir, cfg.DBPath)
	}
	lock, err := AcquireLock(cfg.LockPath())
	if errors.Is(err, ErrLocked) {
		return res, fmt.Errorf("%w: the app (or another restore) is using %s. Stop the app first, then run restore again in a one-off container (see the backups section of the operations guide). Nothing was changed",
			ErrServerRunning, dataDir)
	}
	if err != nil {
		return res, err
	}
	defer lock.Release()

	latest, err := database.LatestMigration(migrations)
	if err != nil {
		return res, err
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return res, fmt.Errorf("cannot open the backup: %w", err)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return res, fmt.Errorf("%s is not a backup file", archivePath)
	}

	// 1. Unpack into a private staging folder on the same disk as the
	//    database (so the final rename is atomic), checking as it goes.
	staging, err := os.MkdirTemp(dataDir, ".restore-")
	if err != nil {
		return res, fmt.Errorf("cannot write in the data folder %s: %w", dataDir, err)
	}
	defer os.RemoveAll(staging)
	stagedDB := filepath.Join(staging, "tennis.db")
	stagedUploads := filepath.Join(staging, uploadsDirName)

	m, err := readArchive(f, func(m Manifest) error {
		if m.SchemaVersion > latest {
			return newerSchema(m, latest)
		}
		return nil
	}, func(name string, content io.Reader) error {
		dest := stagedDB
		if name != databaseName {
			if err := os.MkdirAll(stagedUploads, 0o700); err != nil {
				return err
			}
			dest = filepath.Join(stagedUploads, strings.TrimPrefix(name, uploadsPrefix))
		}
		return writeNew(dest, content)
	})
	if err != nil {
		return res, err
	}
	res.Manifest = m
	res.SchemaFrom = m.SchemaVersion

	// 2. Check the database itself, bring it up to this version's schema,
	//    and end its sessions and links.
	to, sessions, links, err := prepareDatabase(ctx, stagedDB, m, latest, migrations)
	if err != nil {
		return res, err
	}
	res.SchemaTo, res.SessionsEnded, res.LinksCancelled = to, sessions, links

	owned := []string{stagedDB}
	if entries, err := os.ReadDir(stagedUploads); err == nil {
		owned = append(owned, stagedUploads)
		for _, e := range entries {
			owned = append(owned, filepath.Join(stagedUploads, e.Name()))
		}
	}
	if err := matchOwner(dataDir, owned...); err != nil {
		return res, err
	}

	// 3. Swap it in.
	if err := swapIn(ctx, cfg, stagedDB, stagedUploads, &res); err != nil {
		return res, err
	}
	return res, nil
}

func newerSchema(m Manifest, latest uint) error {
	return fmt.Errorf("%w: it was made by version %s with database version %d, and this program only knows database versions up to %d. Restore it with version %s of the app or later. Nothing was changed",
		ErrNewerBackup, m.AppVersion, m.SchemaVersion, latest, m.AppVersion)
}

// writeNew writes content to a new file, private to the app's user.
func writeNew(path string, content io.Reader) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, content); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// prepareDatabase checks the staged database, upgrades it to latest, ends
// its sessions and unused links, records the restore and closes it with
// nothing left in a WAL file.
func prepareDatabase(ctx context.Context, path string, m Manifest, latest uint, migrations fs.FS) (uint, int64, int64, error) {
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: path})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("the backup's database cannot be opened: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			db.Close()
		}
	}()
	var check string
	if err := db.GetContext(ctx, &check, `PRAGMA integrity_check`); err != nil || check != "ok" {
		return 0, 0, 0, badArchive("the backup's database failed SQLite's integrity check (%q, %v)", check, err)
	}
	st, err := db.ReadSchemaStatus(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	switch {
	case !st.Migrated:
		return 0, 0, 0, badArchive("the backup's database was never set up by the app")
	case st.Dirty:
		return 0, 0, 0, badArchive("the backup's database was part-way through an upgrade (version %d) when it was copied", st.Version)
	case st.Version != m.SchemaVersion:
		return 0, 0, 0, badArchive("the backup's database is at version %d but its manifest says %d", st.Version, m.SchemaVersion)
	case st.Version > latest:
		return 0, 0, 0, newerSchema(m, latest)
	case st.Version < latest:
		if err := db.ExecuteMigrationsFS(migrations, "migrations built into this program"); err != nil {
			return 0, 0, 0, fmt.Errorf("upgrading the backup's database from version %d failed, so nothing was restored: %w", st.Version, err)
		}
		if st, err = db.ReadSchemaStatus(ctx); err != nil {
			return 0, 0, 0, err
		}
		if st.Dirty || st.Version != latest {
			return 0, 0, 0, fmt.Errorf("upgrading the backup's database stopped at version %d (want %d), so nothing was restored", st.Version, latest)
		}
	}

	r, err := db.ExecContext(ctx, `UPDATE sessions SET is_valid = FALSE WHERE is_valid = TRUE`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("end the backup's sessions: %w", err)
	}
	sessions, _ := r.RowsAffected()
	r, err = db.ExecContext(ctx, `UPDATE account_links SET revoked_at = ? WHERE used_at IS NULL AND revoked_at IS NULL`, time.Now())
	if err != nil {
		return 0, 0, 0, fmt.Errorf("cancel the backup's unused links: %w", err)
	}
	links, _ := r.RowsAffected()
	detail := fmt.Sprintf("backup %s (%s, version %s, schema %d -> %d) sessions_ended=%d links_cancelled=%d",
		m.CreatedAt.UTC().Format(time.RFC3339), m.Kind, m.AppVersion, m.SchemaVersion, st.Version, sessions, links)
	if err := accounts.New(db).RecordEvent(ctx, accounts.CLIActor("restore"), accounts.ActionBackupRestore, detail); err != nil {
		return 0, 0, 0, err
	}
	closed = true
	if err := closeCheckpointed(ctx, db, path); err != nil {
		return 0, 0, 0, err
	}
	return st.Version, sessions, links, nil
}

// swapIn sets the current database and uploads aside and moves the staged
// ones into place. The database move is the last step and a single rename;
// a failure before it puts everything back.
func swapIn(ctx context.Context, cfg Config, stagedDB, stagedUploads string, res *RestoreResult) (err error) {
	stamp := time.Now().UTC().Format(nameTimeLayout)
	var undo []func()
	defer func() {
		if err != nil {
			for i := len(undo) - 1; i >= 0; i-- {
				undo[i]()
			}
		}
	}()

	// The uploaded files: the old folder aside, the restored one in place.
	uploads := cfg.UploadsDir()
	if _, statErr := os.Lstat(uploads); statErr == nil {
		aside, err := freeAside(uploads, stamp)
		if err != nil {
			return err
		}
		if err := os.Rename(uploads, aside); err != nil {
			return fmt.Errorf("set the current uploaded files aside: %w", err)
		}
		undo = append(undo, func() { _ = os.Rename(aside, uploads) })
		res.ReplacedUploads = aside
	}
	if _, statErr := os.Stat(stagedUploads); statErr == nil {
		if err := os.Rename(stagedUploads, uploads); err != nil {
			return fmt.Errorf("put the restored uploaded files in place: %w", err)
		}
		undo = append(undo, func() { _ = os.Rename(uploads, stagedUploads) })
	}

	// The current database: written through (nothing left only in its WAL)
	// when it can be opened, then kept aside by a hard link (or a copy), with
	// its WAL and journal moved beside the copy so they can never be applied
	// to the restored file.
	if _, statErr := os.Stat(cfg.DBPath); statErr == nil {
		if old, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: cfg.DBPath}); err == nil {
			_ = closeCheckpointed(ctx, old, cfg.DBPath) // best effort: a damaged database is why people restore
		}
		aside, err := freeAside(cfg.DBPath, stamp)
		if err != nil {
			return err
		}
		if err := linkOrCopy(cfg.DBPath, aside); err != nil {
			return fmt.Errorf("keep the current database aside: %w", err)
		}
		undo = append(undo, func() { _ = os.Remove(aside) })
		res.ReplacedDatabase = aside
		for _, suffix := range []string{"-wal", "-journal"} {
			from := cfg.DBPath + suffix
			if _, statErr := os.Lstat(from); statErr != nil {
				continue
			}
			if err := os.Rename(from, aside+suffix); err != nil {
				return fmt.Errorf("set the current database's %s file aside: %w", suffix, err)
			}
			undo = append(undo, func() { _ = os.Rename(aside+suffix, from) })
		}
		if err := os.Remove(cfg.DBPath + "-shm"); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove the current database's -shm file: %w", err)
		}
	}

	// The swap.
	if err := os.Rename(stagedDB, cfg.DBPath); err != nil {
		return fmt.Errorf("put the restored database in place: %w", err)
	}
	syncDir(cfg.DataDir())
	if res.ReplacedDatabase != "" {
		_ = os.Chmod(res.ReplacedDatabase, 0o600)
	}
	return nil
}

// freeAside is a name for setting path aside that is not in use.
func freeAside(path, stamp string) (string, error) {
	for i := 1; i < 100; i++ {
		p := path + replacedMarker + stamp
		if i > 1 {
			p = fmt.Sprintf("%s-%d", p, i)
		}
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			return p, nil
		}
	}
	return "", fmt.Errorf("too many earlier copies of %s", path)
}

// linkOrCopy makes dst the same file as src (a hard link) or, where links
// are not possible, a private copy.
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := writeNew(dst, in); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}
