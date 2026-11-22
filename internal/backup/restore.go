// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"bytes"
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
	"jim-dot-tennis/internal/branding"
	"jim-dot-tennis/internal/database"
)

var (
	// ErrServerRunning: restore was asked for while the app is running (or
	// another restore is in progress).
	ErrServerRunning = errors.New("the app is running")
	// ErrNewerBackup: the backup comes from a newer version of the app than
	// this program.
	ErrNewerBackup = errors.New("the backup is from a newer version of the app")
	// ErrUndoFailed: a restore failed before its commit point and putting
	// back what it had already changed did not fully work, so the data
	// folder may not be exactly as it was.
	ErrUndoFailed = errors.New("putting the data folder back as it was did not fully work")
	// ErrTournamentInUse: the tournament server or the identity service (or
	// anything else) is still connected to the tournament database.
	ErrTournamentInUse = errors.New("the tournament database is in use")
	// ErrTournamentMismatch: the restore stopped with the tournament
	// database restored but the app's database not (and the tournament
	// database could not be put back), so the two do not match.
	ErrTournamentMismatch = errors.New("the two databases no longer match")
)

// TournamentRestore says what a restore did with the tournament database.
type TournamentRestore int

const (
	// TournamentNotInvolved: neither the backup nor this deployment has
	// tournament data.
	TournamentNotInvolved TournamentRestore = iota
	// TournamentRestored: the backup's tournament data replaced the
	// tournament database.
	TournamentRestored
	// TournamentNotInBackup: this deployment has a tournament database but
	// the backup has no tournament data, so it was left as it is.
	TournamentNotInBackup
	// TournamentNotConfigured: the backup has tournament data but this
	// deployment has no tournament database settings, so it was not
	// restored.
	TournamentNotConfigured
)

// StopTournamentServices is how an operator stops what holds the tournament
// database open, for the messages.
const StopTournamentServices = "docker stop courthive-server courthive-hiveid"

// tournamentDumpAside is the name of the safety copy of the tournament
// database a restore takes first, in the data folder.
const tournamentDumpAside = "courthive.dump"

// restoreStagingPrefix names a restore's staging folder in the data folder.
const restoreStagingPrefix = ".restore-"

// swapHook, when set (tests only), is called at the named points of swapIn:
// "before-commit" (an error makes swapIn fail there and undo) and
// "after-commit".
var swapHook func(point string) error

// diskFree is freeSpace; tests replace it.
var diskFree = freeSpace

// spaceNeeded is how much free space a restore wants before it unpacks a
// backup whose files total n bytes: room for the files, again as much for
// the upgrade's and the end-of-sessions' writes (and for a copy of the old
// database where hard links are not possible), and a margin.
func spaceNeeded(n int64) uint64 {
	return 2*uint64(max(n, 0)) + 16<<20
}

// RestoreResult says what Restore did.
type RestoreResult struct {
	Manifest         Manifest
	SchemaFrom       uint   // the backup's schema version
	SchemaTo         uint   // the restored database's schema version (after any upgrade)
	ReplacedDatabase string // where the database that was replaced is kept ("" if there was none)
	ReplacedUploads  string // where the uploaded files that were replaced are kept ("" if none)
	SessionsEnded    int64  // sessions in the backup that were ended
	LinksCancelled   int64  // unused invite and recovery links in the backup that were cancelled
	// Tournament says what was done with the tournament database.
	Tournament TournamentRestore
	// TournamentSafetyCopy is where the tournament database as it was before
	// the restore is kept, a dump in the data folder ("" if it was not
	// replaced).
	TournamentSafetyCopy string
	// Warnings are problems in tidying up after the restored database was
	// put in place (the restore itself is done).
	Warnings []string
}

// replacedMarker is in the names of what a restore set aside.
const replacedMarker = ".replaced-"

// Restore puts the backup in archivePath back as the instance's database and
// uploaded files. It refuses while the server is running (the data
// directory's lock is held). It reads the archive as untrusted input,
// verifies it against its manifest, refuses a backup from a newer schema
// than migrations holds, and upgrades an older one. Everything is built and
// checked in a staging folder beside the database first. The database is
// then swapped in with one rename, the commit point: before it the instance
// has its old database and the logos it uses, after it the restored
// database and the logos that one uses, whenever the process stops (see
// swapIn). The replaced database (with its WAL file, if it had unwritten
// changes) and uploaded files are kept aside, not deleted. On any failure
// before the commit point what was changed is put back; if that fails too
// the error wraps ErrUndoFailed.
//
// Cancelling ctx (the command line does on Ctrl-C) stops the restore
// cleanly before the commit point; once the swap has started it runs to the
// end. A staging folder left by a restore that was killed is removed by the
// next restore and when the server starts.
//
// The restored database keeps no sessions: every session in the backup is
// ended (a session someone signed out of, or an administrator revoked,
// after the backup was made would otherwise work again), as is every unused
// invite or recovery link. Everyone signs in again. The restore is recorded
// in the restored database's audit log.
//
// Tournament data. When the backup holds a dump of the tournament database
// and this deployment has one (cfg.Tournament), both are restored, the
// tournament database first: once the app's database is staged and checked,
// the dump is read right through, the restore refuses while anything else
// is connected to the tournament database (the tournament server and the
// identity service must be stopped), a safety copy of the tournament
// database is dumped into the data folder, and the dump replaces the
// database's content in one transaction. Only then is the app's database
// swapped in. If that swap fails, the safety copy is put back, so both are
// as they were; if putting it back fails too, the error wraps
// ErrTournamentMismatch and says what each database holds. A backup with
// tournament data on a deployment without the settings restores the app
// only (TournamentNotConfigured; RestoreTournaments can restore the rest
// later); a backup without tournament data leaves the tournament database
// as it is (TournamentNotInBackup).
func Restore(ctx context.Context, cfg Config, archivePath string, migrations fs.FS) (RestoreResult, error) {
	var res RestoreResult
	dataDir := cfg.DataDir()
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return res, fmt.Errorf("the data folder %s (from DB_PATH=%s) does not exist. Mount the app's data volume there, or set DB_PATH", dataDir, cfg.DBPath)
	}
	// The restore lock first: from here on the account and backup commands
	// refuse to start (AcquireRestoreLock says why there are two locks).
	restoreLock, err := AcquireRestoreLock(cfg.RestoreLockPath())
	if errors.Is(err, ErrLocked) {
		return res, fmt.Errorf("%w: a command (user ..., admin-passkeys, setup code or backup) or another restore is using %s. Wait for it to finish, then run restore again. Nothing was changed",
			ErrServerRunning, dataDir)
	}
	if err != nil {
		return res, err
	}
	defer restoreLock.Release()
	lock, err := AcquireLock(cfg.LockPath())
	if errors.Is(err, ErrLocked) {
		return res, fmt.Errorf("%w: the app (or another restore) is using %s. Stop the app first, then run restore again in a one-off container (see the backups section of the operations guide). Nothing was changed",
			ErrServerRunning, dataDir)
	}
	if err != nil {
		return res, err
	}
	defer lock.Release()
	// Holding both locks, no other restore is running: any staging folder
	// is a leftover.
	sweepRestoreStaging(dataDir)

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
	if err := ctx.Err(); err != nil {
		return res, stopped(err)
	}

	// 1. Unpack into a private staging folder on the same disk as the
	//    database (so the final rename is atomic), checking as it goes.
	staging, err := os.MkdirTemp(dataDir, restoreStagingPrefix)
	if err != nil {
		return res, fmt.Errorf("cannot write in the data folder %s: %w", dataDir, err)
	}
	defer os.RemoveAll(staging)
	stagedDB := filepath.Join(staging, "tennis.db")
	stagedUploads := filepath.Join(staging, uploadsDirName)
	stagedDump := filepath.Join(staging, "courthive.dump")

	m, err := readArchive(&ctxReader{ctx: ctx, r: f}, func(m Manifest) error {
		if m.SchemaVersion > latest {
			return newerSchema(m, latest)
		}
		if m.HasTournaments() && cfg.Tournament != nil {
			// Checked before anything is unpacked, so a restore that would
			// have to stop for this stops at once.
			if err := tournamentIdle(ctx, cfg.Tournament); err != nil {
				return err
			}
		}
		// Every entry is held to its manifest size, so this is the most
		// unpacking can write.
		if free, ok := diskFree(dataDir); ok && free < spaceNeeded(m.TotalSize()) {
			return fmt.Errorf("there is not enough free space in %s to restore this backup: its files take %d bytes, and restoring needs about %d bytes free; %d are. Free some space, then try again. Nothing was changed",
				dataDir, m.TotalSize(), spaceNeeded(m.TotalSize()), free)
		}
		return nil
	}, func(name string, content io.Reader) error {
		dest := stagedDB
		if name == tournamentName {
			if cfg.Tournament == nil {
				return nil // not restored here; still checked against its checksum
			}
			return writeNew(stagedDump, content)
		}
		if name != databaseName {
			if err := os.MkdirAll(stagedUploads, 0o700); err != nil {
				return err
			}
			dest = filepath.Join(stagedUploads, strings.TrimPrefix(name, uploadsPrefix))
		}
		return writeNew(dest, content)
	})
	if err != nil {
		if ctx.Err() != nil {
			return res, stopped(ctx.Err())
		}
		return res, err
	}
	res.Manifest = m
	res.SchemaFrom = m.SchemaVersion
	switch {
	case m.HasTournaments() && cfg.Tournament != nil:
		res.Tournament = TournamentRestored
	case m.HasTournaments():
		res.Tournament = TournamentNotConfigured
	case cfg.Tournament != nil:
		res.Tournament = TournamentNotInBackup
	}

	// 2. Check the database itself, bring it up to this version's schema,
	//    and end its sessions and links.
	to, sessions, links, err := prepareDatabase(ctx, stagedDB, m, latest, migrations, res.Tournament)
	if err != nil {
		if ctx.Err() != nil {
			return res, stopped(ctx.Err())
		}
		return res, err
	}
	res.SchemaTo, res.SessionsEnded, res.LinksCancelled = to, sessions, links

	// When run as root, the restored files go to whoever owns the database
	// they replace (the app's user), or the data folder's owner if there is
	// none: never root, which the app could not open.
	owner := dataDir
	if _, err := os.Stat(cfg.DBPath); err == nil {
		owner = cfg.DBPath
	}
	owned := []string{stagedDB}
	if entries, err := os.ReadDir(stagedUploads); err == nil {
		owned = append(owned, stagedUploads)
		for _, e := range entries {
			owned = append(owned, filepath.Join(stagedUploads, e.Name()))
		}
	}
	if err := matchOwner(owner, owned...); err != nil {
		return res, err
	}

	// 3. Swap it in. The last chance to stop: from here it runs to the end
	//    whatever happens to ctx, so a Ctrl-C cannot leave a mixture.
	if err := ctx.Err(); err != nil {
		return res, stopped(err)
	}
	// 3a. The tournament database, when it is restored too. Every failure
	//     in here leaves both databases as they were.
	if res.Tournament == TournamentRestored {
		safety, err := replaceTournament(ctx, cfg, stagedDump, staging, owner)
		if err != nil {
			return res, err
		}
		res.TournamentSafetyCopy = safety
	}
	// 3b. The app's database.
	if err := swapIn(context.WithoutCancel(ctx), cfg, stagedDB, stagedUploads, owner, &res); err != nil {
		if res.Tournament == TournamentRestored {
			return res, putTournamentBack(context.WithoutCancel(ctx), cfg, res.TournamentSafetyCopy, staging, err)
		}
		return res, err
	}
	return res, nil
}

// tournamentIdle refuses while anything else is connected to the tournament
// database: the tournament server and the identity service keep connections
// open (and a running tournament server could write in the middle of a
// restore). It also confirms the database can be reached.
func tournamentIdle(ctx context.Context, t TournamentDatabase) error {
	n, err := t.OtherSessions(ctx)
	if err != nil {
		return fmt.Errorf("%s cannot be reached, so the tournament data cannot be restored: %w. "+
			"Postgres must be running (docker start courthive-postgres) while the tournament server and identity service are stopped. Nothing was changed",
			t.Describe(), err)
	}
	if n > 0 {
		return fmt.Errorf("%w: %d other connection(s) are open to %s. The tournament server and the identity service must be stopped during a restore: "+
			"run %q (Postgres stays running), then run restore again. Nothing was changed",
			ErrTournamentInUse, n, t.Describe(), StopTournamentServices)
	}
	return nil
}

// replaceTournament restores the dump at staged over the tournament
// database: it reads the dump right through, checks nothing else is
// connected, keeps a safety copy of the database in the data folder, then
// replaces the content in one transaction. It returns the safety copy's
// path. On any error the tournament database is as it was (and the safety
// copy is removed); the message says so.
func replaceTournament(ctx context.Context, cfg Config, staged, workDir, owner string) (string, error) {
	t := cfg.Tournament
	if _, err := t.Inspect(ctx, staged); err != nil {
		return "", badArchive("the backup's tournament data cannot be read (%v). Nothing was changed", err)
	}
	if err := tournamentIdle(ctx, t); err != nil {
		return "", err
	}
	safety, err := freeAside(filepath.Join(cfg.DataDir(), tournamentDumpAside), time.Now().UTC().Format(nameTimeLayout))
	if err != nil {
		return "", err
	}
	if err := t.Dump(ctx, safety); err != nil {
		os.Remove(safety)
		return "", fmt.Errorf("the safety copy of the current tournament database could not be made (%v), so nothing was restored. Nothing was changed", err)
	}
	if _, err := t.Inspect(ctx, safety); err != nil {
		os.Remove(safety)
		return "", fmt.Errorf("the safety copy of the current tournament database did not read back (%v), so nothing was restored. Nothing was changed", err)
	}
	if err := matchOwner(owner, safety); err != nil {
		os.Remove(safety)
		return "", err
	}
	if err := ctx.Err(); err != nil {
		os.Remove(safety)
		return "", stopped(err)
	}
	// Once started it runs to the end: a single transaction either commits
	// or leaves the database as it was.
	if err := t.Replace(context.WithoutCancel(ctx), staged, workDir); err != nil {
		os.Remove(safety)
		return "", fmt.Errorf("restoring the tournament database failed and was rolled back, so it is as it was: %v. "+
			"The app's database was not touched. Nothing was changed", err)
	}
	return safety, nil
}

// putTournamentBack is called when the app's database could not be swapped
// in after the tournament database was restored: it puts the safety copy
// back so the two match again, and returns cause with what happened.
func putTournamentBack(ctx context.Context, cfg Config, safety, workDir string, cause error) error {
	if err := cfg.Tournament.Replace(ctx, safety, workDir); err != nil {
		return fmt.Errorf("%w. %w: the tournament database had already been restored from the backup, and putting back the copy taken first failed (%v). "+
			"Now the tournament database holds the backup's tournament data while the app's database does not. Before starting anything, put the "+
			"tournament database back with: jim-dot-tennis restore --tournaments-only %s",
			cause, ErrTournamentMismatch, err, safety)
	}
	os.Remove(safety)
	return fmt.Errorf("%w. The tournament database had already been restored, so it was put back from the copy taken first: "+
		"the tournament database is as it was", cause)
}

// TournamentResult says what RestoreTournaments did.
type TournamentResult struct {
	Manifest   *Manifest // the archive's manifest; nil when the file was a bare dump
	SafetyCopy string    // where the tournament database as it was is kept
}

// RestoreTournaments puts back only the tournament database, from a backup
// archive that holds tournament data or from a bare dump (such as the
// safety copy a restore keeps). The app's database is not touched. It takes
// the same locks as Restore (the app must be stopped), and the same safety
// steps: nothing else may be connected, a safety copy is dumped first, and
// the content is replaced in one transaction.
//
// Use it with the archive whose app data is in place (the same archive
// that was restored without its tournament data, say): the app's links to
// people's tournament accounts are kept in the app's database and must
// match.
func RestoreTournaments(ctx context.Context, cfg Config, path string) (TournamentResult, error) {
	var res TournamentResult
	dataDir := cfg.DataDir()
	if cfg.Tournament == nil {
		return res, fmt.Errorf("this deployment has no tournament database settings (%s and the others are empty), so there is nowhere to restore tournament data to. "+
			"Turn the tournament stack on first (the operations guide's tournaments section). Nothing was changed", EnvPGHost)
	}
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return res, fmt.Errorf("the data folder %s (from DB_PATH=%s) does not exist. Mount the app's data volume there, or set DB_PATH", dataDir, cfg.DBPath)
	}
	restoreLock, err := AcquireRestoreLock(cfg.RestoreLockPath())
	if errors.Is(err, ErrLocked) {
		return res, fmt.Errorf("%w: a command or another restore is using %s. Wait for it to finish, then run restore again. Nothing was changed", ErrServerRunning, dataDir)
	}
	if err != nil {
		return res, err
	}
	defer restoreLock.Release()
	lock, err := AcquireLock(cfg.LockPath())
	if errors.Is(err, ErrLocked) {
		return res, fmt.Errorf("%w: the app (or another restore) is using %s. Stop the app first, then run restore again in a one-off container (see the backups section of the operations guide). Nothing was changed",
			ErrServerRunning, dataDir)
	}
	if err != nil {
		return res, err
	}
	defer lock.Release()
	sweepRestoreStaging(dataDir)

	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return res, fmt.Errorf("%s is not a backup file. Nothing was changed", path)
	}
	staging, err := os.MkdirTemp(dataDir, restoreStagingPrefix)
	if err != nil {
		return res, fmt.Errorf("cannot write in the data folder %s: %w", dataDir, err)
	}
	defer os.RemoveAll(staging)
	owner := dataDir
	if _, err := os.Stat(cfg.DBPath); err == nil {
		owner = cfg.DBPath
	}

	dump := path
	if !IsTournamentDump(path) {
		f, err := os.Open(path)
		if err != nil {
			return res, fmt.Errorf("cannot open the backup: %w", err)
		}
		defer f.Close()
		dump = filepath.Join(staging, "courthive.dump")
		m, err := readArchive(&ctxReader{ctx: ctx, r: f}, func(m Manifest) error {
			if !m.HasTournaments() {
				return fmt.Errorf("this backup (made %s) holds no tournament data. Nothing was changed", m.CreatedAt.UTC().Format(time.RFC3339))
			}
			if free, ok := diskFree(dataDir); ok && free < spaceNeeded(m.Tournaments.Size) {
				return fmt.Errorf("there is not enough free space in %s to restore this backup's tournament data (about %d bytes needed, %d free). Nothing was changed",
					dataDir, spaceNeeded(m.Tournaments.Size), free)
			}
			return tournamentIdle(ctx, cfg.Tournament)
		}, func(name string, content io.Reader) error {
			if name != tournamentName {
				return nil
			}
			return writeNew(dump, content)
		})
		if err != nil {
			if ctx.Err() != nil {
				return res, stopped(ctx.Err())
			}
			return res, err
		}
		res.Manifest = &m
	}
	safety, err := replaceTournament(ctx, cfg, dump, staging, owner)
	if err != nil {
		return res, err
	}
	res.SafetyCopy = safety
	return res, nil
}

// stopped is the error for a restore cancelled before its commit point.
func stopped(err error) error {
	return fmt.Errorf("restore stopped before anything was changed: %w", err)
}

// ctxReader fails reads once ctx is cancelled, so unpacking a large backup
// stops promptly on Ctrl-C.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// sweepRestoreStaging removes staging folders that an earlier restore left in
// dataDir when it was killed (each holds a full copy of a database). Only
// call it while holding the data folder's lock, when no restore can be
// running. It returns how many it removed.
func sweepRestoreStaging(dataDir string) int {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), restoreStagingPrefix) {
			continue
		}
		if os.RemoveAll(filepath.Join(dataDir, e.Name())) == nil {
			n++
		}
	}
	return n
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
func prepareDatabase(ctx context.Context, path string, m Manifest, latest uint, migrations fs.FS, tournament TournamentRestore) (uint, int64, int64, error) {
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
	switch tournament {
	case TournamentRestored:
		detail += " tournaments=restored"
	case TournamentNotInBackup:
		detail += " tournaments=not-in-backup"
	case TournamentNotConfigured:
		detail += " tournaments=not-restored"
	}
	if err := accounts.New(db).RecordEvent(ctx, accounts.CLIActor("restore"), accounts.ActionBackupRestore, detail); err != nil {
		return 0, 0, 0, err
	}
	closed = true
	if err := closeCheckpointed(ctx, db, path); err != nil {
		return 0, 0, 0, err
	}
	return st.Version, sessions, links, nil
}

// undoList is what to do, newest first, to put the data folder back if the
// swap fails before its commit point.
type undoList []func() error

func (u *undoList) add(f func() error) { *u = append(*u, f) }

// run undoes everything and returns every error it met (nil if none).
func (u *undoList) run() error {
	var errs []error
	for i := len(*u) - 1; i >= 0; i-- {
		if err := (*u)[i](); err != nil {
			errs = append(errs, err)
		}
	}
	*u = nil
	return errors.Join(errs...)
}

// removeIfThere removes path, which may already be gone.
func removeIfThere(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// swapIn puts the staged database and logos in place. The order makes the
// database rename the commit point, so that a stop at any moment leaves a
// database with the logos it uses:
//
//  1. The current database is kept aside by a hard link (or a copy); the
//     live file is not touched.
//  2. The current logos are kept aside the same way (links into
//     branding.replaced-<time>), and the restored logos are added to the
//     live folder beside the current ones (logo names are content hashes,
//     so a name already there is the same file). The live folder now holds
//     what both databases use.
//  3. The current database's -wal and -journal are moved beside its aside
//     copy (so they can never be applied to the restored file) and its -shm
//     is removed.
//  4. The commit point: the restored database is renamed over the current
//     one, in one step.
//  5. Tidying: the logos only the old database used are removed from the
//     live folder (they are in the aside folder). A failure here is a
//     warning; the restore is done.
//
// A failure in steps 1-4 undoes what was done; an undo that fails is
// reported with ErrUndoFailed. A crash in steps 1-3 leaves the old database
// with every logo it uses (plus the restored ones, unused, and the aside
// copies), and a crash after step 4 the restored database with every logo
// it uses. One exception: an old database that could not be written through
// (a damaged one) keeps changes in its -wal, and a crash between step 3 and
// step 4 leaves it without that file; the aside copy has both.
func swapIn(ctx context.Context, cfg Config, stagedDB, stagedUploads, owner string, res *RestoreResult) (err error) {
	stamp := time.Now().UTC().Format(nameTimeLayout)
	var undo undoList
	defer func() {
		if err != nil {
			if uerr := undo.run(); uerr != nil {
				err = fmt.Errorf("%w. Then %w (%v): check the data folder %s against the operations guide before starting the app",
					err, ErrUndoFailed, uerr, cfg.DataDir())
			}
		}
	}()

	// 1. The current database, kept aside. It is first written through
	//    (nothing left only in its WAL) when it can be opened.
	_, statErr := os.Stat(cfg.DBPath)
	hadDB := statErr == nil
	var dbAside string
	if hadDB {
		if old, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: cfg.DBPath}); err == nil {
			_ = closeCheckpointed(ctx, old, cfg.DBPath) // best effort: a damaged database is why people restore
		}
		if dbAside, err = freeAside(cfg.DBPath, stamp); err != nil {
			return err
		}
		if err := linkOrCopy(cfg.DBPath, dbAside); err != nil {
			return fmt.Errorf("keep the current database aside: %w", err)
		}
		undo.add(func() error { return removeIfThere(dbAside) })
		res.ReplacedDatabase = dbAside
	}

	// 2. The logos.
	restored, err := logoNames(stagedUploads)
	if err != nil {
		return fmt.Errorf("read the restored uploaded files: %w", err)
	}
	uploads := cfg.UploadsDir()
	current, err := logoNames(uploads)
	if err != nil {
		return fmt.Errorf("read the current uploaded files: %w", err)
	}
	if info, statErr := os.Lstat(uploads); statErr == nil && info.IsDir() {
		aside, err := freeAside(uploads, stamp)
		if err != nil {
			return err
		}
		if err := os.Mkdir(aside, 0o700); err != nil {
			return fmt.Errorf("keep the current uploaded files aside: %w", err)
		}
		undo.add(func() error { return os.RemoveAll(aside) })
		for _, name := range current {
			if err := linkOrCopy(filepath.Join(uploads, name), filepath.Join(aside, name)); err != nil {
				return fmt.Errorf("keep the current uploaded files aside: %w", err)
			}
		}
		if err := matchOwner(owner, aside); err != nil {
			return err
		}
		res.ReplacedUploads = aside
	} else if len(restored) > 0 {
		if err := os.Mkdir(uploads, 0o700); err != nil {
			return fmt.Errorf("make the uploaded files folder: %w", err)
		}
		undo.add(func() error { return os.RemoveAll(uploads) })
		if err := matchOwner(owner, uploads); err != nil {
			return err
		}
	}
	for _, name := range restored {
		src, dst := filepath.Join(stagedUploads, name), filepath.Join(uploads, name)
		if _, err := os.Lstat(dst); err == nil {
			if same, err := sameContent(src, dst); err != nil || !same {
				return fmt.Errorf("the restored logo %s differs from the current file of the same name, which should not happen; nothing was restored", name)
			}
			continue
		}
		if err := linkOrCopy(src, dst); err != nil {
			return fmt.Errorf("put the restored uploaded files in place: %w", err)
		}
		undo.add(func() error { return removeIfThere(dst) })
	}

	// 3. The current database's companions.
	if hadDB {
		if err := setAsideCompanions(cfg.DBPath, dbAside, &undo); err != nil {
			return err
		}
		if err := removeIfThere(cfg.DBPath + "-shm"); err != nil {
			return fmt.Errorf("remove the current database's -shm file: %w", err)
		}
	}
	if swapHook != nil {
		if err := swapHook("before-commit"); err != nil {
			return err
		}
	}

	// 4. The commit point.
	if err := os.Rename(stagedDB, cfg.DBPath); err != nil {
		return fmt.Errorf("put the restored database in place: %w", err)
	}
	undo = nil
	syncDir(cfg.DataDir())
	if swapHook != nil {
		if err := swapHook("after-commit"); err != nil {
			res.Warnings = append(res.Warnings, err.Error())
		}
	}

	// 5. Tidying.
	if dbAside != "" {
		if err := os.Chmod(dbAside, 0o600); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not make %s private: %v", filepath.Base(dbAside), err))
		}
	}
	keep := map[string]bool{}
	for _, name := range restored {
		keep[name] = true
	}
	for _, name := range current {
		if keep[name] {
			continue
		}
		if err := os.Remove(filepath.Join(uploads, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			res.Warnings = append(res.Warnings, fmt.Sprintf("could not remove the replaced logo %s from %s (a copy is in %s): %v",
				name, uploads, filepath.Base(res.ReplacedUploads), err))
		}
	}
	syncDir(uploads)
	return nil
}

// setAsideCompanions moves dbPath's -wal and -journal beside aside, private
// to the app's user like the aside copy, adding how to move them back to
// undo.
func setAsideCompanions(dbPath, aside string, undo *undoList) error {
	for _, suffix := range []string{"-wal", "-journal"} {
		from, to := dbPath+suffix, aside+suffix
		if _, err := os.Lstat(from); err != nil {
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("set the current database's %s file aside: %w", suffix, err)
		}
		undo.add(func() error { return os.Rename(to, from) })
		if err := os.Chmod(to, 0o600); err != nil {
			return fmt.Errorf("make the current database's %s file private: %w", suffix, err)
		}
	}
	return nil
}

// logoNames lists the uploaded logos in dir (regular files with a name the
// app gives logos; nothing else there is touched). A missing dir has none.
func logoNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && branding.ValidLogoName(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// sameContent reports whether two files hold the same bytes.
func sameContent(a, b string) (bool, error) {
	x, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
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
