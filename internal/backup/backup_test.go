// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

const testLogo = "logo-0123456789abcdef.png"

// instance is a data directory with a database, as the server has it.
type instance struct {
	t   *testing.T
	cfg Config
	db  *database.DB
}

func newInstance(t *testing.T, migrations fs.FS) *instance {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{DBPath: filepath.Join(dir, "tennis.db"), Dir: filepath.Join(dir, "backups"), Keep: 3, Interval: time.Hour}
	in := &instance{t: t, cfg: cfg}
	in.open()
	if migrations != nil {
		if err := in.db.ExecuteMigrationsFS(migrations, "test"); err != nil {
			t.Fatal(err)
		}
	}
	return in
}

func (in *instance) open() {
	in.t.Helper()
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: in.cfg.DBPath})
	if err != nil {
		in.t.Fatal(err)
	}
	in.db = db
	in.t.Cleanup(func() { db.Close() })
}

func (in *instance) close() {
	in.t.Helper()
	if err := in.db.Close(); err != nil {
		in.t.Fatal(err)
	}
}

func (in *instance) exec(q string, args ...any) {
	in.t.Helper()
	if _, err := in.db.Exec(q, args...); err != nil {
		in.t.Fatalf("%s: %v", q, err)
	}
}

// seed adds data that works at any schema version.
func (in *instance) seed() {
	in.exec(`INSERT INTO clubs (name, address) VALUES ('Test Club', '1 Court Road'), ('Other Club', NULL)`)
	in.exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p1', 'Ada', 'Lovelace', 1), ('p2', 'Bob', 'O''Brien', 2)`)
	if err := os.MkdirAll(in.cfg.UploadsDir(), 0o700); err != nil {
		in.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.cfg.UploadsDir(), testLogo), []byte("\x89PNG not really"), 0o600); err != nil {
		in.t.Fatal(err)
	}
}

// excluded tables change on restore by design (sessions ended, links
// cancelled, the restore recorded) or are migration bookkeeping.
var excluded = map[string]bool{"sessions": true, "account_links": true, "account_audit_log": true, "schema_migrations": true}

// fingerprint is the row count and a checksum of every other table.
func fingerprint(t *testing.T, db *database.DB) map[string]string {
	t.Helper()
	var tables []string
	if err := db.Select(&tables, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, table := range tables {
		if excluded[table] {
			continue
		}
		rows, err := db.Queryx(`SELECT * FROM "` + table + `" ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		n := 0
		for rows.Next() {
			vals, err := rows.SliceScan()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(h, "%v\n", vals)
			n++
		}
		rows.Close()
		out[table] = fmt.Sprintf("%d rows %s", n, hex.EncodeToString(h.Sum(nil))[:16])
	}
	return out
}

func latest(t *testing.T) uint {
	n, err := database.LatestMigration(jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// migrationsUpTo is the built-in migrations numbered at most n.
func migrationsUpTo(t *testing.T, n int) fs.FS {
	t.Helper()
	src := jimdottennis.Migrations()
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		num, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil || num > n {
			continue
		}
		data, err := fs.ReadFile(src, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = &fstest.MapFile{Data: data}
	}
	return out
}

func sum(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// The acceptance round trip: seed, back up, wipe, restore, and the app reads
// the same data back; sessions and unused links do not survive.
func TestBackupWipeRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	acc := accounts.New(in.db)
	_, _, err := acc.Invite(ctx, accounts.CLIActor("test"), "captain1", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	in.exec(`INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h', 1, 'captain', ?, '127.0.0.1', 'ua', 'dev')`, time.Now().Add(time.Hour))
	before := fingerprint(t, in.db)

	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.SchemaVersion != latest(t) || res.Manifest.Kind != KindManual || len(res.Manifest.Files) != 1 {
		t.Fatalf("manifest: %+v", res.Manifest)
	}
	for path, want := range map[string]os.FileMode{res.Path: 0o600, in.cfg.Dir: 0o700} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: mode %v, want %v (%v)", path, info.Mode().Perm(), want, err)
		}
	}

	// Wipe the instance: database, WAL files and uploads.
	in.close()
	for _, p := range []string{in.cfg.DBPath, in.cfg.DBPath + "-wal", in.cfg.DBPath + "-shm", in.cfg.UploadsDir()} {
		os.RemoveAll(p)
	}

	rr, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if rr.ReplacedDatabase != "" || rr.SchemaFrom != latest(t) || rr.SchemaTo != latest(t) || rr.SessionsEnded != 1 || rr.LinksCancelled != 1 {
		t.Errorf("restore result: %+v", rr)
	}
	in.open()
	if after := fingerprint(t, in.db); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("data differs after restore:\nbefore %v\nafter  %v", before, after)
	}
	// App-level reads.
	clubs, err := repository.NewClubRepository(in.db).FindAll(ctx)
	if err != nil || len(clubs) != 2 {
		t.Errorf("clubs after restore: %d, %v", len(clubs), err)
	}
	list, err := accounts.New(in.db).List(ctx)
	if err != nil || len(list) != 1 || list[0].Username != "captain1" || list[0].ActiveSessions != 0 {
		t.Errorf("accounts after restore: %+v, %v", list, err)
	}
	var restores int
	in.db.Get(&restores, `SELECT COUNT(*) FROM account_audit_log WHERE action = ?`, accounts.ActionBackupRestore)
	if restores != 1 {
		t.Errorf("restore not in the audit log (%d)", restores)
	}
	if got, err := os.ReadFile(filepath.Join(in.cfg.UploadsDir(), testLogo)); err != nil || string(got) != "\x89PNG not really" {
		t.Errorf("logo after restore: %q, %v", got, err)
	}
	if st, _ := in.db.ReadSchemaStatus(ctx); st.Version != latest(t) || st.Dirty {
		t.Errorf("schema after restore: %+v", st)
	}
}

// Restoring over a live instance keeps the replaced database (including
// changes only in its WAL) and uploads aside, and never applies the old WAL
// to the restored file.
func TestRestoreKeepsReplacedDatabaseAndDropsItsWAL(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	// Changes after the backup, left only in the WAL (as after a crash):
	// the database and WAL as they are on disk while still open.
	in.exec(`PRAGMA wal_autocheckpoint = 0`)
	conn, _ := in.db.Conn(ctx)
	conn.ExecContext(ctx, `PRAGMA wal_autocheckpoint = 0`)
	conn.ExecContext(ctx, `INSERT INTO clubs (name) VALUES ('Added After Backup')`)
	conn.Close()
	crashed := t.TempDir()
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(in.cfg.DBPath + suffix)
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(crashed, "tennis.db"+suffix), data, 0o600)
	}
	in.close()
	for _, suffix := range []string{"", "-wal"} {
		data, _ := os.ReadFile(filepath.Join(crashed, "tennis.db"+suffix))
		os.WriteFile(in.cfg.DBPath+suffix, data, 0o600)
	}
	os.Remove(in.cfg.DBPath + "-shm")
	if info, err := os.Stat(in.cfg.DBPath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("test setup: no WAL content (%v)", err)
	}
	os.WriteFile(filepath.Join(in.cfg.UploadsDir(), "logo-fedcba9876543210.png"), []byte("newer logo"), 0o600)

	rr, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(in.cfg.DBPath + suffix); err == nil {
			t.Errorf("%s left beside the restored database", suffix)
		}
	}
	if rr.ReplacedDatabase == "" || rr.ReplacedUploads == "" {
		t.Fatalf("nothing kept aside: %+v", rr)
	}
	if info, err := os.Stat(rr.ReplacedDatabase); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("replaced database: %v", err)
	}
	old, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: rr.ReplacedDatabase})
	if err != nil {
		t.Fatal(err)
	}
	var n int
	old.Get(&n, `SELECT COUNT(*) FROM clubs WHERE name = 'Added After Backup'`)
	old.Close()
	if n != 1 {
		t.Error("the replaced database lost the changes that were only in its WAL")
	}
	if _, err := os.Stat(filepath.Join(rr.ReplacedUploads, "logo-fedcba9876543210.png")); err != nil {
		t.Errorf("replaced uploads not kept: %v", err)
	}
	in.open()
	in.db.Get(&n, `SELECT COUNT(*) FROM clubs`)
	if n != 2 {
		t.Errorf("restored database has %d clubs, want 2 (the old WAL must not be applied)", n)
	}
	if _, err := os.Stat(filepath.Join(in.cfg.UploadsDir(), "logo-fedcba9876543210.png")); err == nil {
		t.Error("the restored uploads include a file that was not in the backup")
	}
}

// A backup from an older schema is upgraded on restore.
func TestRestoreUpgradesOlderBackup(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, migrationsUpTo(t, 33))
	in.seed()
	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.SchemaVersion != 33 {
		t.Fatalf("schema in manifest: %d", res.Manifest.SchemaVersion)
	}
	in.close()
	os.Remove(in.cfg.DBPath)

	rr, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if rr.SchemaFrom != 33 || rr.SchemaTo != latest(t) {
		t.Errorf("schema %d -> %d", rr.SchemaFrom, rr.SchemaTo)
	}
	in.open()
	players, err := repository.NewPlayerRepository(in.db).FindAll(ctx)
	if err != nil || len(players) != 2 {
		t.Errorf("players after upgrade: %d, %v", len(players), err)
	}
}

// A backup from a newer schema is refused, before anything changes.
func TestRestoreRefusesNewerBackup(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	in.exec(`UPDATE schema_migrations SET version = ?`, latest(t)+1)
	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.exec(`UPDATE schema_migrations SET version = ?`, latest(t))
	in.exec(`INSERT INTO clubs (name) VALUES ('Current')`)
	in.close()
	original := sum(t, in.cfg.DBPath)

	_, err = Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if !errors.Is(err, ErrNewerBackup) || !strings.Contains(err.Error(), "newer version") || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("restore of a newer backup: %v", err)
	}
	if sum(t, in.cfg.DBPath) != original {
		t.Error("the database changed")
	}
	assertNoLeftovers(t, in.cfg)
}

func assertNoLeftovers(t *testing.T, cfg Config) {
	t.Helper()
	entries, _ := os.ReadDir(cfg.DataDir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".restore-") || strings.Contains(e.Name(), replacedMarker) {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}

// Restore refuses while the server holds the lock, and the server cannot
// start (take the lock) while a restore holds it.
func TestRestoreRefusesWhileServerRuns(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	server, err := AcquireLock(in.cfg.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	original := sum(t, in.cfg.DBPath)
	_, err = Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if !errors.Is(err, ErrServerRunning) || !strings.Contains(err.Error(), "Stop the app first") {
		t.Fatalf("restore while running: %v", err)
	}
	if sum(t, in.cfg.DBPath) != original {
		t.Error("the database changed")
	}
	server.Release()

	restoring, err := AcquireLock(in.cfg.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(in.cfg.LockPath()); !errors.Is(err, ErrLocked) {
		t.Errorf("a second holder got the lock: %v", err)
	}
	restoring.Release()
	if _, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations()); err != nil {
		t.Errorf("restore once the server stopped: %v", err)
	}
}

// The lock is held by a process, not a file: another process holding it
// blocks us, and once that process is killed (no clean exit, as in a crash)
// the lock is free although the file is still there.
func TestLockFollowsTheProcess(t *testing.T) {
	if os.Getenv("BACKUP_TEST_HOLD_LOCK") != "" {
		return // the helper process, see below
	}
	dir := t.TempDir()
	lockPath := filepath.Join(dir, lockFileName)
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), "BACKUP_TEST_HOLD_LOCK="+lockPath)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 6)
	if _, err := io.ReadFull(stdout, buf); err != nil || string(buf) != "locked" {
		cmd.Process.Kill()
		t.Fatalf("helper did not take the lock: %q %v", buf, err)
	}
	if _, err := AcquireLock(lockPath); !errors.Is(err, ErrLocked) {
		t.Errorf("lock held by another process was taken: %v", err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	l, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("lock of a killed process is still held: %v", err)
	}
	l.Release()
}

// TestLockHelperProcess holds the lock until killed (run by TestLockFollowsTheProcess).
func TestLockHelperProcess(t *testing.T) {
	path := os.Getenv("BACKUP_TEST_HOLD_LOCK")
	if path == "" {
		t.Skip("helper process only")
	}
	if _, err := AcquireLock(path); err != nil {
		fmt.Print("failed")
		os.Exit(1)
	}
	fmt.Print("locked")
	time.Sleep(time.Minute)
	os.Exit(0)
}

// archiveFile writes a hand-made archive.
type entry struct {
	name     string
	typ      byte
	body     []byte
	size     int64 // header size when not len(body)
	linkname string
}

func writeTestArchive(t *testing.T, path string, entries []entry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		size := int64(len(e.body))
		if e.size != 0 {
			size = e.size
		}
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		if typ != tar.TypeReg {
			size = 0
		}
		if err := tw.WriteHeader(&tar.Header{Typeflag: typ, Name: e.name, Size: size, Mode: 0o600, Linkname: e.linkname}); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write(e.body)
		}
	}
	tw.Flush()
	// Deliberately not tw.Close for oversized entries: the archive is cut short.
	tw.Close()
	gz.Close()
	os.WriteFile(path, buf.Bytes(), 0o600)
}

func manifestFor(t *testing.T, db []byte, files ...FileEntry) []byte {
	t.Helper()
	s := sha256.Sum256(db)
	m := Manifest{Format: 1, App: appName, AppVersion: "test", SchemaVersion: 1, Kind: KindManual,
		Database: FileEntry{Path: databaseName, Size: int64(len(db)), SHA256: hex.EncodeToString(s[:])}, Files: files}
	b, _ := json.Marshal(m)
	return b
}

// Restore reads an untrusted file: traversal, absolute paths, links,
// oversized entries, duplicates, a missing manifest and bad checksums are
// all refused, nothing outside the staging folder is written and the
// current database is untouched.
func TestRestoreRefusesMaliciousArchives(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	in.close()
	original := sum(t, in.cfg.DBPath)
	outside := filepath.Join(filepath.Dir(in.cfg.DataDir()), "escaped")

	db := []byte("not really a database")
	good := manifestFor(t, db)
	cases := map[string][]entry{
		"traversal":          {{name: manifestName, body: good}, {name: "../escaped", body: db}},
		"nested traversal":   {{name: manifestName, body: good}, {name: "files/branding/../../../escaped", body: db}},
		"absolute":           {{name: manifestName, body: good}, {name: "/tmp/escaped", body: db}},
		"symlink":            {{name: manifestName, body: good}, {name: databaseName, typ: tar.TypeSymlink, linkname: "/etc/passwd"}},
		"hard link":          {{name: manifestName, body: good}, {name: databaseName, typ: tar.TypeLink, linkname: manifestName}},
		"directory":          {{name: manifestName, body: good}, {name: "files/branding/", typ: tar.TypeDir}},
		"unexpected file":    {{name: manifestName, body: good}, {name: "files/branding/evil.sh", body: db}},
		"oversized":          {{name: manifestName, body: good}, {name: "files/branding/" + testLogo, size: maxUploadBytes + 1}},
		"duplicate":          {{name: manifestName, body: good}, {name: databaseName, body: db}, {name: databaseName, body: db}},
		"manifest not first": {{name: databaseName, body: db}, {name: manifestName, body: good}},
		"bad checksum":       {{name: manifestName, body: good}, {name: databaseName, body: []byte("not really a databasE")}},
		"missing database":   {{name: manifestName, body: good}},
		"unlisted file":      {{name: manifestName, body: good}, {name: databaseName, body: db}, {name: "files/branding/" + testLogo, body: db}},
		"not a database":     {{name: manifestName, body: good}, {name: databaseName, body: db}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "evil.tar.gz")
			writeTestArchive(t, path, entries)
			_, err := Restore(ctx, in.cfg, path, jimdottennis.Migrations())
			if err == nil {
				t.Fatal("restored")
			}
			t.Logf("refused: %v", err)
			if sum(t, in.cfg.DBPath) != original {
				t.Error("the database changed")
			}
			if _, err := os.Lstat(outside); err == nil {
				t.Error("a file was written outside the data folder")
			}
			assertNoLeftovers(t, in.cfg)
		})
	}
	t.Run("not gzip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plain.tar.gz")
		os.WriteFile(path, []byte("hello"), 0o600)
		if _, err := Restore(ctx, in.cfg, path, jimdottennis.Migrations()); err == nil || !errors.Is(err, errBadArchive) {
			t.Fatalf("restored a non-archive: %v", err)
		}
	})
}

// Retention keeps cfg.Keep regular backups, never fewer than one, and does
// not count temporary files or pre-migration backups.
func TestPruneRetention(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, Keep: 3}
	base := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	for i := range 5 {
		os.WriteFile(filepath.Join(dir, regularPrefix+base.Add(time.Duration(i)*time.Hour).Format(nameTimeLayout)+archiveExtension), []byte("x"), 0o600)
	}
	pre := preMigrationPrefix + base.Format(nameTimeLayout) + archiveExtension
	os.WriteFile(filepath.Join(dir, pre), []byte("x"), 0o600)
	partial := tmpPrefix + "abc" + archiveExtension
	os.WriteFile(filepath.Join(dir, partial), []byte("x"), 0o600)

	removed, err := prune(cfg, "", time.Now())
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed %v, %v", removed, err)
	}
	list, _ := List(dir)
	if len(list) != 4 || list[0].CreatedAt != base.Add(4*time.Hour) {
		t.Errorf("left: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, pre)); err != nil {
		t.Error("pre-migration backup removed")
	}
	if _, err := os.Stat(filepath.Join(dir, partial)); err != nil {
		t.Error("a recent temporary file was removed (it may be a backup in progress)")
	}

	cfg.Keep = 0 // never below one
	prune(cfg, "", time.Now())
	list, _ = List(dir)
	regular := 0
	for _, a := range list {
		if !a.PreMigration {
			regular++
		}
	}
	if regular != 1 {
		t.Errorf("Keep=0 left %d regular backups, want 1", regular)
	}
}

// Create leaves nothing that looks like a backup when it fails.
func TestFailedBackupLeavesNoArchive(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Create(ctx, in.db, in.cfg, KindManual); err == nil {
		t.Fatal("a cancelled backup succeeded")
	}
	entries, _ := os.ReadDir(in.cfg.Dir)
	if len(entries) != 0 {
		t.Errorf("left in the backups folder: %v", entries)
	}
}

// The pre-migration backup: none for a new empty database, none when there
// is nothing to migrate, none over a half-done migration, one otherwise.
func TestBeforeMigrations(t *testing.T) {
	ctx := context.Background()
	empty := newInstance(t, nil)
	if p, err := BeforeMigrations(ctx, empty.db, empty.cfg, jimdottennis.Migrations()); p != "" || err != nil {
		t.Errorf("empty database: %q %v", p, err)
	}

	current := newInstance(t, jimdottennis.Migrations())
	if p, err := BeforeMigrations(ctx, current.db, current.cfg, jimdottennis.Migrations()); p != "" || err != nil {
		t.Errorf("up-to-date database: %q %v", p, err)
	}

	older := newInstance(t, migrationsUpTo(t, 33))
	older.seed()
	p, err := BeforeMigrations(ctx, older.db, older.cfg, jimdottennis.Migrations())
	if err != nil || p == "" || !strings.HasPrefix(filepath.Base(p), preMigrationPrefix) {
		t.Fatalf("older database: %q %v", p, err)
	}
	if m, err := VerifyFile(p); err != nil || m.Kind != KindPreMigration || m.SchemaVersion != 33 {
		t.Errorf("pre-migration archive: %+v %v", m, err)
	}

	older.exec(`UPDATE schema_migrations SET dirty = TRUE`)
	if p, err := BeforeMigrations(ctx, older.db, older.cfg, jimdottennis.Migrations()); p != "" || err != nil {
		t.Errorf("dirty database: %q %v", p, err)
	}

	// No usable backups folder: an error (the caller must not migrate).
	blocked := newInstance(t, migrationsUpTo(t, 33))
	blocked.seed()
	os.WriteFile(filepath.Join(blocked.cfg.DataDir(), "not-a-folder"), nil, 0o600)
	blocked.cfg.Dir = filepath.Join(blocked.cfg.DataDir(), "not-a-folder", "backups")
	if _, err := BeforeMigrations(ctx, blocked.db, blocked.cfg, jimdottennis.Migrations()); err == nil {
		t.Error("an impossible backup was not reported")
	}
}

func TestKeepLatestPreMigration(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	for i := range 3 {
		os.WriteFile(filepath.Join(dir, preMigrationPrefix+base.Add(time.Duration(i)*time.Hour).Format(nameTimeLayout)+archiveExtension), []byte("x"), 0o600)
	}
	regular := regularPrefix + base.Format(nameTimeLayout) + archiveExtension
	os.WriteFile(filepath.Join(dir, regular), []byte("x"), 0o600)
	removed, err := KeepLatestPreMigration(Config{Dir: dir})
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed %v %v", removed, err)
	}
	list, _ := List(dir)
	if len(list) != 2 {
		t.Errorf("left %+v", list)
	}
}

// The scheduler takes a backup only when one is due.
func TestSchedulerRunsWhenDue(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.cfg.Interval = 24 * time.Hour
	s := NewService(in.db, in.cfg)
	ctx := context.Background()
	if !s.RunIfDue(ctx) {
		t.Fatal("no backup when there was none")
	}
	if s.RunIfDue(ctx) {
		t.Error("a second backup straight after the first")
	}
	s.now = func() time.Time { return time.Now().Add(24 * time.Hour) }
	if !s.RunIfDue(ctx) {
		t.Error("no backup a day later")
	}
	if st := s.Status(); st.LastError != "" || st.LastSuccess.IsZero() {
		t.Errorf("status %+v", st)
	}

	// Start/stop: the stop function returns.
	stop := s.Start(ctx, time.Hour, time.Hour)
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("DB_PATH", "/data/tennis.db")
	t.Setenv(EnvDir, "")
	t.Setenv(EnvKeep, "")
	t.Setenv(EnvInterval, "")
	c, err := FromEnv()
	if err != nil || c.Dir != "/data/backups" || c.Keep != DefaultKeep || c.Interval != DefaultInterval {
		t.Errorf("defaults: %+v %v", c, err)
	}
	t.Setenv(EnvInterval, "off")
	if c, _ := FromEnv(); c.Interval != 0 {
		t.Errorf("off: %v", c.Interval)
	}
	for k, v := range map[string]string{EnvKeep: "0", EnvInterval: "1m"} {
		t.Setenv(k, v)
		if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("%s=%s: %v", k, v, err)
		}
		t.Setenv(k, "")
	}
}
