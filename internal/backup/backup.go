// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"jim-dot-tennis/internal/branding"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/version"
)

// Archive names. Finished archives are only ever given these names, by a
// rename once the archive is complete and verified, so a name of this shape
// is a whole backup; anything being written is called tmpPrefix+... and is
// never counted, listed or kept.
const (
	regularPrefix      = "jim-dot-tennis-backup-"
	preMigrationPrefix = "jim-dot-tennis-pre-migration-"
	downloadPrefix     = "jim-dot-tennis-download-"
	tmpPrefix          = ".tmp-"
	nameTimeLayout     = "20060102T150405Z"
	// staleTempAge is when a leftover temporary file (from a crash or a
	// power cut mid-backup) is removed.
	staleTempAge = 24 * time.Hour
	// staleDownloadAge is when a download copy that was not removed after
	// it was sent (the server stopped mid-download) is removed. Sending one
	// takes at most 15 minutes (admin.downloadWriteTime).
	staleDownloadAge = time.Hour
	// futureSkew is how far ahead of this machine's clock an archive's name
	// may be dated before the name is not believed (a file copied in from a
	// machine with a wrong clock, or a hand-made name). Such an archive is
	// never taken as the newest backup and is the first to be pruned.
	futureSkew = 5 * time.Minute
)

// Archive is a finished backup in the backups folder.
type Archive struct {
	Name         string
	Path         string
	Size         int64
	CreatedAt    time.Time
	PreMigration bool
}

// Result is what Create made.
type Result struct {
	Path     string
	Manifest Manifest
	Size     int64
}

// Create makes a backup of db (the instance's open database, whose file is
// cfg.DBPath) in cfg.Dir and returns where it is. It never touches the live
// database file: the copy comes from VACUUM INTO. The archive is written
// under a temporary name, read back and verified, and only then renamed to
// its final name, so a failure or a crash never leaves a file that looks
// like a backup. Regular backups (manual, scheduled) are then pruned to
// cfg.Keep; a pre-migration backup is not counted.
func Create(ctx context.Context, db *database.DB, cfg Config, kind string) (Result, error) {
	if err := ensureDir(cfg.Dir); err != nil {
		return Result{}, err
	}
	res, err := build(ctx, db, cfg, kind, cfg.Dir)
	if err != nil {
		return Result{}, err
	}
	if kind == KindManual || kind == KindScheduled {
		if _, err := prune(cfg, res.Path, time.Now()); err != nil {
			log.Printf("Backup made, but removing old backups failed: %v", err)
		}
	}
	return res, nil
}

// CreateForDownload makes a backup for the "Download backup" button: a
// verified archive in cfg.Dir under a download name, which the caller sends
// and then removes. It is not one of the kept backups.
func CreateForDownload(ctx context.Context, db *database.DB, cfg Config) (Result, error) {
	if err := ensureDir(cfg.Dir); err != nil {
		return Result{}, err
	}
	return build(ctx, db, cfg, KindDownload, cfg.Dir)
}

// DownloadName is the file name to offer the browser for an archive.
func DownloadName(res Result) string {
	return regularPrefix + res.Manifest.CreatedAt.UTC().Format(nameTimeLayout) + archiveExtension
}

// ensureDir makes the backups folder, readable by the app's user only.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create the backups folder %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("cannot make the backups folder %s private: %w", dir, err)
	}
	return nil
}

func build(ctx context.Context, db *database.DB, cfg Config, kind, dir string) (Result, error) {
	now := time.Now().UTC()
	work, err := os.MkdirTemp(dir, tmpPrefix+"work-")
	if err != nil {
		return Result{}, fmt.Errorf("cannot write in the backups folder %s: %w", dir, err)
	}
	defer os.RemoveAll(work)

	// 1. A consistent copy of the database, and its schema version as of
	//    that copy.
	copyPath := filepath.Join(work, "tennis.db")
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, copyPath); err != nil {
		return Result{}, fmt.Errorf("copy the database: %w", err)
	}
	schema, err := checkCopy(ctx, copyPath)
	if err != nil {
		return Result{}, err
	}
	dbEntry, err := hashFile(copyPath, databaseName)
	if err != nil {
		return Result{}, err
	}

	// 2. The uploaded files, read once into memory (logos are small) so the
	//    archive holds exactly what was hashed.
	uploads, err := readUploads(cfg.UploadsDir())
	if err != nil {
		return Result{}, err
	}

	m := Manifest{
		Format:        FormatVersion,
		App:           appName,
		AppVersion:    version.Short(),
		AppCommit:     version.FullCommit(),
		SchemaVersion: schema,
		CreatedAt:     now.Truncate(time.Second),
		Kind:          kind,
		Database:      dbEntry,
		Files:         []FileEntry{},
		Notice:        manifestNotice,
	}
	for _, u := range uploads {
		m.Files = append(m.Files, u.entry)
	}
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Result{}, err
	}

	// 3. The archive, under a temporary name.
	tmp, err := os.OpenFile(filepath.Join(dir, tmpPrefix+randomSuffix()+archiveExtension), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Result{}, fmt.Errorf("cannot write in the backups folder %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			os.Remove(tmpName)
		}
	}()
	if err := writeArchive(tmp, m, manifest, copyPath, uploads); err != nil {
		tmp.Close()
		return Result{}, fmt.Errorf("write the backup: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return Result{}, fmt.Errorf("write the backup: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return Result{}, fmt.Errorf("write the backup: %w", err)
	}

	// 4. Read it back before calling it a backup.
	if _, err := VerifyFile(tmpName); err != nil {
		return Result{}, fmt.Errorf("the backup did not verify after writing: %w", err)
	}

	// 5. Its final name.
	prefix := regularPrefix
	switch kind {
	case KindPreMigration:
		prefix = preMigrationPrefix
	case KindDownload:
		prefix = downloadPrefix
	}
	final, err := publish(tmpName, dir, prefix+now.Format(nameTimeLayout))
	if err != nil {
		return Result{}, err
	}
	keep = true
	syncDir(dir)
	info, err := os.Stat(final)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: final, Manifest: m, Size: info.Size()}, nil
}

// checkCopy confirms the copy is a sound database and returns its schema
// version. Opening it with the app's settings leaves it in WAL mode; it is
// checkpointed and closed before it is archived.
func checkCopy(ctx context.Context, path string) (uint, error) {
	cp, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: path})
	if err != nil {
		return 0, fmt.Errorf("open the database copy: %w", err)
	}
	var check string
	if err := cp.GetContext(ctx, &check, `PRAGMA quick_check`); err != nil || check != "ok" {
		cp.Close()
		return 0, fmt.Errorf("the database copy failed its check (%q, %v)", check, err)
	}
	st, err := cp.ReadSchemaStatus(ctx)
	if err != nil {
		cp.Close()
		return 0, err
	}
	if err := closeCheckpointed(ctx, cp, path); err != nil {
		return 0, err
	}
	return st.Version, nil
}

// closeCheckpointed writes everything in db's WAL into the database file,
// closes it, and confirms no WAL content is left beside path.
func closeCheckpointed(ctx context.Context, db *database.DB, path string) error {
	_, cerr := db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err := db.Close(); err != nil {
		return err
	}
	if cerr != nil {
		return fmt.Errorf("checkpoint %s: %w", filepath.Base(path), cerr)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Stat(path + suffix)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if suffix == "-wal" && info.Size() > 0 {
			return fmt.Errorf("%s still has unwritten changes after closing", filepath.Base(path))
		}
		if err := os.Remove(path + suffix); err != nil {
			return err
		}
	}
	return nil
}

type upload struct {
	entry   FileEntry
	data    []byte
	modTime time.Time
}

// readUploads reads the uploaded logos (only names the app itself gives
// them; anything else in the folder is left out).
func readUploads(dir string) ([]upload, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the uploaded files in %s: %w", dir, err)
	}
	var out []upload
	for _, e := range entries {
		if !e.Type().IsRegular() || !branding.ValidLogoName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if info.Size() > maxUploadBytes {
			return nil, fmt.Errorf("the uploaded file %s is larger than %d bytes", e.Name(), maxUploadBytes)
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(data)
		out = append(out, upload{
			entry:   FileEntry{Path: uploadsPrefix + e.Name(), Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])},
			data:    data,
			modTime: info.ModTime(),
		})
	}
	return out, nil
}

func writeArchive(w io.Writer, m Manifest, manifest []byte, dbCopy string, uploads []upload) error {
	aw := newArchiveWriter(w)
	if err := aw.add(manifestName, int64(len(manifest)), m.CreatedAt, bytes.NewReader(manifest)); err != nil {
		return err
	}
	f, err := os.Open(dbCopy)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := aw.add(databaseName, m.Database.Size, m.CreatedAt, f); err != nil {
		return err
	}
	for _, u := range uploads {
		if err := aw.add(u.entry.Path, u.entry.Size, u.modTime, bytes.NewReader(u.data)); err != nil {
			return err
		}
	}
	return aw.close()
}

func hashFile(path, name string) (FileEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileEntry{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return FileEntry{}, err
	}
	return FileEntry{Path: name, Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// VerifyFile reads a whole archive and checks it against its manifest
// without writing anything.
func VerifyFile(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()
	return readArchive(f, nil, nil)
}

// publish gives the finished file tmp its final name, dir/base.tar.gz, or
// base-2, base-3... if that is taken (two backups or downloads finishing in
// the same second), and returns it. It never replaces an existing file: the
// name is taken with a hard link, which fails if the name exists, so two
// processes can never end up with (or send, or delete) the same file. Where
// the folder cannot hold hard links, the name is reserved by creating it
// exclusively and the file is then renamed over its own reservation.
func publish(tmp, dir, base string) (string, error) {
	for i := 1; i < 100; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		p := filepath.Join(dir, name+archiveExtension)
		err := os.Link(tmp, p)
		if err == nil {
			_ = os.Remove(tmp)
			return p, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		// No hard links here: reserve the name, then rename onto it.
		reserved, rerr := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(rerr, fs.ErrExist) {
			continue
		}
		if rerr != nil {
			return "", fmt.Errorf("finish the backup: %w", rerr)
		}
		reserved.Close()
		if err := os.Rename(tmp, p); err != nil {
			os.Remove(p)
			return "", fmt.Errorf("finish the backup: %w", err)
		}
		return p, nil
	}
	return "", fmt.Errorf("too many backups named %s", base)
}

func randomSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// syncDir flushes a directory's entries (a rename) to disk, where the
// platform allows it.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}

// List returns the finished backups in dir, newest first. Temporary files
// and download copies are not listed.
func List(dir string) ([]Archive, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Archive
	for _, e := range entries {
		a, ok := parseArchiveName(e.Name())
		if !ok || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		a.Path = filepath.Join(dir, e.Name())
		a.Size = info.Size()
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].Name > out[j].Name
	})
	return out, nil
}

// InFuture reports whether a's name dates it more than a few minutes after
// now. Such a name is not believed (see futureSkew).
func (a Archive) InFuture(now time.Time) bool {
	return a.CreatedAt.After(now.Add(futureSkew))
}

// noticed remembers which archives were already reported as ignored, so the
// log says so once per file, not on every hourly check or page view.
var noticed sync.Map

func noticeOnce(a Archive, why string) {
	if _, seen := noticed.LoadOrStore(a.Path+"\x00"+why, true); !seen {
		log.Printf("Backups: %s %s, so it is not counted as the latest backup", a.Name, why)
	}
}

// Newest returns the newest backup in dir that can be believed: its name is
// not dated in the future, and it opens as an archive whose manifest can be
// read (only the first entry is read, so this is cheap; VerifyFile reads the
// whole file). Pre-migration backups count only with includePreMigration.
// The bool is false when there is none.
func Newest(dir string, now time.Time, includePreMigration bool) (Archive, bool, error) {
	list, err := List(dir)
	if err != nil {
		return Archive{}, false, err
	}
	for _, a := range list {
		if a.PreMigration && !includePreMigration {
			continue
		}
		if a.InFuture(now) {
			noticeOnce(a, "is dated in the future ("+a.CreatedAt.Format(time.RFC3339)+")")
			continue
		}
		if _, err := ReadManifest(a.Path); err != nil {
			noticeOnce(a, "cannot be read as a backup")
			continue
		}
		return a, true, nil
	}
	return Archive{}, false, nil
}

// ReadManifest reads and checks only the manifest of the archive at path.
func ReadManifest(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return Manifest{}, badArchive("it is not a gzip file (%v)", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		return Manifest{}, badArchive("it is damaged or empty (%v)", err)
	}
	if hdr.Typeflag != tar.TypeReg || hdr.Name != manifestName || hdr.Size < 0 || hdr.Size > maxManifestBytes {
		return Manifest{}, badArchive("the first entry is not the manifest")
	}
	return decodeManifest(tr)
}

// parseArchiveName recognises a finished archive's name and its time.
func parseArchiveName(name string) (Archive, bool) {
	rest, ok := strings.CutSuffix(name, archiveExtension)
	if !ok {
		return Archive{}, false
	}
	a := Archive{Name: name}
	switch {
	case strings.HasPrefix(rest, regularPrefix):
		rest = strings.TrimPrefix(rest, regularPrefix)
	case strings.HasPrefix(rest, preMigrationPrefix):
		rest = strings.TrimPrefix(rest, preMigrationPrefix)
		a.PreMigration = true
	default:
		return Archive{}, false
	}
	if len(rest) < len(nameTimeLayout) {
		return Archive{}, false
	}
	t, err := time.Parse(nameTimeLayout, rest[:len(nameTimeLayout)])
	if err != nil {
		return Archive{}, false
	}
	if extra := rest[len(nameTimeLayout):]; extra != "" && !isCounterSuffix(extra) {
		return Archive{}, false
	}
	a.CreatedAt = t
	return a, true
}

func isCounterSuffix(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// prune removes the oldest regular backups beyond cfg.Keep (never fewer than
// one is kept, so it cannot remove the only good backup), stale temporary
// files and download copies (removeStaleTemps). It never removes protect (the
// archive the caller has just written and verified, which counts as one of
// those kept). Archives whose names are dated in the future count as the
// oldest, so they are the first to go. Pre-migration backups are not counted
// or removed here (see KeepLatestPreMigration). It returns the names it
// removed.
func prune(cfg Config, protect string, now time.Time) ([]string, error) {
	keep := max(cfg.Keep, 1)
	list, err := List(cfg.Dir)
	if err != nil {
		return nil, err
	}
	// Newest first, then the future-dated ones (whose names are not
	// believed), so those are the first to go.
	var ordered, future []Archive
	kept := 0
	for _, a := range list {
		switch {
		case a.PreMigration:
		case protect != "" && a.Path == protect:
			kept++
		case a.InFuture(now):
			future = append(future, a)
		default:
			ordered = append(ordered, a)
		}
	}
	var removed []string
	for _, a := range append(ordered, future...) {
		if kept < keep {
			kept++
			continue
		}
		if err := os.Remove(a.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, a.Name)
	}
	removeStaleTemps(cfg.Dir, now, staleTempAge, staleDownloadAge)
	return removed, nil
}

// removeStaleTemps removes temporary files and folders older than tmpAge (a
// backup in progress is never that old) and download copies older than
// downloadAge. It returns how many it removed.
func removeStaleTemps(dir string, now time.Time, tmpAge, downloadAge time.Duration) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		age := tmpAge
		switch {
		case strings.HasPrefix(e.Name(), tmpPrefix):
		case strings.HasPrefix(e.Name(), downloadPrefix):
			age = downloadAge
		default:
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < age {
			continue
		}
		if os.RemoveAll(filepath.Join(dir, e.Name())) == nil {
			n++
		}
	}
	return n
}

// CleanUpAtStart is called by the server once it holds the data folder's
// lock, before anything else uses the folder. It removes what an earlier
// process left behind when it was stopped or crashed: restore staging
// folders (each a full copy of the database; no restore can be running
// while the server holds the lock), download copies (only the server makes
// them, and it has only just started) and temporary backup files more than
// a day old (a `jim-dot-tennis backup` run alongside may be writing a newer
// one). It returns how many it removed.
func CleanUpAtStart(cfg Config) int {
	return sweepRestoreStaging(cfg.DataDir()) + removeStaleTemps(cfg.Dir, time.Now(), staleTempAge, 0)
}

// KeepLatestPreMigration is called once the server has started successfully:
// the backup taken before the last upgrade's migrations has done its job, so
// any older pre-migration backups are removed. The newest one is kept (until
// a later upgrade takes another), in case a problem with the upgrade only
// shows later.
func KeepLatestPreMigration(cfg Config) ([]string, error) {
	list, err := List(cfg.Dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	seen := false
	for _, a := range list {
		if !a.PreMigration {
			continue
		}
		if !seen {
			seen = true
			continue
		}
		if err := os.Remove(a.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, a.Name)
	}
	return removed, nil
}

// BeforeMigrations takes a backup if migrations are about to change db: not
// for a brand-new empty database, not when the schema is already at the
// newest migration in migrations, and not when an earlier start left a
// migration half done (startup then stops with a MigrationError without
// changing anything, and a backup of that state on every restart would only
// fill the disk). It returns the archive's path, or "" when none was needed.
// An error means the backup could not be made, and the caller must not run
// the migrations.
func BeforeMigrations(ctx context.Context, db *database.DB, cfg Config, migrations fs.FS) (string, error) {
	var tables int
	if err := db.GetContext(ctx, &tables, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`); err != nil {
		return "", fmt.Errorf("look at the database: %w", err)
	}
	if tables == 0 {
		return "", nil // a new, empty database: nothing to keep
	}
	st, err := db.ReadSchemaStatus(ctx)
	if err != nil {
		return "", err
	}
	if st.Dirty {
		return "", nil
	}
	latest, err := database.LatestMigration(migrations)
	if err != nil {
		return "", err
	}
	if st.Migrated && st.Version >= latest {
		return "", nil
	}
	res, err := Create(ctx, db, cfg, KindPreMigration)
	if err != nil {
		return "", err
	}
	return res.Path, nil
}
