// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	jimdottennis "jim-dot-tennis"
)

// futureJunk writes a file whose name says it was made years from now.
func futureJunk(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, regularPrefix+"20990101T000000Z"+archiveExtension)
	if err := os.WriteFile(p, []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A file named as if made in the future does not stop scheduled backups.
func TestFutureDatedNameDoesNotStopSchedule(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.cfg.Interval = 24 * time.Hour
	futureJunk(t, in.cfg.Dir)
	s := NewService(in.db, in.cfg)
	if !s.RunIfDue(context.Background()) {
		t.Fatal("no scheduled backup: a future-dated file name counted as the newest backup")
	}
}

// With BACKUP_KEEP=1 a fresh, verified backup is kept and a future-dated
// file is what goes.
func TestPruneKeepsFreshBackupOverFutureDatedName(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.cfg.Keep = 1
	junk := futureJunk(t, in.cfg.Dir)
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatalf("the backup just made was pruned: %v", err)
	}
	if _, err := os.Stat(junk); err == nil {
		t.Error("the future-dated file was kept instead")
	}
}

// Restore checks each entry's declared size against the manifest before
// storing anything: an entry the manifest does not list, or one larger than
// the manifest says, is refused without a byte of it being written.
func TestRestoreChecksEntryAgainstManifestBeforeWriting(t *testing.T) {
	db := []byte("not really a database")
	manifest := manifestFor(t, db)
	big := bytes.Repeat([]byte{0}, 4<<20)
	cases := map[string][]entry{
		"larger than listed": {{name: manifestName, body: manifest}, {name: databaseName, body: big}},
		"not listed":         {{name: manifestName, body: manifest}, {name: uploadsPrefix + testLogo, body: big}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "evil.tar.gz")
			writeTestArchive(t, path, entries)
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			var written int64
			_, err = readArchive(f, nil, func(_ string, r io.Reader) error {
				n, err := io.Copy(io.Discard, r)
				written += n
				return err
			})
			if err == nil || !errors.Is(err, errBadArchive) {
				t.Fatalf("accepted: %v", err)
			}
			if written != 0 {
				t.Errorf("%d bytes were handed to store before the archive was refused", written)
			}
		})
	}
}

// A restore that is cancelled (Ctrl-C) stops with the context's error and
// leaves nothing behind.
func TestRestoreHonoursCancellation(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	original := sum(t, in.cfg.DBPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled restore: %v", err)
	}
	if sum(t, in.cfg.DBPath) != original {
		t.Error("the database changed")
	}
	assertNoLeftovers(t, in.cfg)
}

// A staging folder left by an interrupted restore is removed by the next.
func TestRestoreSweepsLeftoverStaging(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	left := filepath.Join(in.cfg.DataDir(), ".restore-123456")
	os.MkdirAll(left, 0o700)
	os.WriteFile(filepath.Join(left, "tennis.db"), []byte("copy"), 0o600)
	if _, err := Restore(context.Background(), in.cfg, res.Path, jimdottennis.Migrations()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(left); err == nil {
		t.Error("the leftover staging folder is still there")
	}
}

// The old database's -wal and -journal, set aside, are private like the
// main copy (they keep their content, so they get the same protection).
func TestAsideCompanionsArePrivate(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "tennis.db")
	aside := db + replacedMarker + "20260101T000000Z"
	for _, s := range []string{"-wal", "-journal"} {
		os.WriteFile(db+s, []byte("old "+s), 0o644)
		os.Chmod(db+s, 0o644)
	}
	var undo undoList
	if err := setAsideCompanions(db, aside, &undo); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"-wal", "-journal"} {
		info, err := os.Stat(aside + s)
		if err != nil {
			t.Fatalf("%s not set aside: %v", s, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s set aside with mode %v, want 0600", s, info.Mode().Perm())
		}
		if _, err := os.Lstat(db + s); err == nil {
			t.Errorf("%s still beside the database", s)
		}
	}
}

// Run as root, restore gives the restored files the current database's
// owner, not the data folder's.
func TestRestoreAsRootKeepsDatabaseOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root (make test runs as root in Docker)")
	}
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	const uid, gid = 4321, 4322
	if err := os.Chown(in.cfg.DBPath, uid, gid); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(in.cfg.UploadsDir()) // so the restored logo is put in place
	if _, err := Restore(context.Background(), in.cfg, res.Path, jimdottennis.Migrations()); err != nil {
		t.Fatal(err)
	}
	gotUID, gotGID := ownerOf(t, in.cfg.DBPath)
	if gotUID != uid || gotGID != gid {
		t.Errorf("restored database owned by %d:%d, want %d:%d (the replaced database's owner)", gotUID, gotGID, uid, gid)
	}
	for _, p := range []string{in.cfg.UploadsDir(), filepath.Join(in.cfg.UploadsDir(), testLogo)} {
		if u, _ := ownerOf(t, p); u != uid {
			t.Errorf("%s owned by %d, want %d", p, u, uid)
		}
	}
}

// restoreOverLiveInstance is an instance with a backup, then a newer logo
// that the backup does not have; it returns the backup.
func restoreOverLiveInstance(t *testing.T) (*instance, string, string) {
	t.Helper()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	const newer = "logo-fedcba9876543210.png"
	os.WriteFile(filepath.Join(in.cfg.UploadsDir(), newer), []byte("newer logo"), 0o600)
	return in, res.Path, newer
}

// Whenever the restore stops, the database in place has the logos it uses:
// before the commit point the old database (with its newer logo), after it
// the restored one (with the backup's logo).
func TestRestoreIsConsistentAtEveryPoint(t *testing.T) {
	in, archive, newer := restoreOverLiveInstance(t)
	original := sum(t, in.cfg.DBPath)
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(in.cfg.UploadsDir(), name))
		return err == nil
	}
	var seen []string
	swapHook = func(point string) error {
		seen = append(seen, point)
		switch point {
		case "before-commit":
			if sum(t, in.cfg.DBPath) != original {
				t.Error("before the commit point the database is not the old one")
			}
			if !has(newer) {
				t.Error("before the commit point the old database's logo is missing")
			}
		case "after-commit":
			if sum(t, in.cfg.DBPath) == original {
				t.Error("after the commit point the database is still the old one")
			}
			if !has(testLogo) {
				t.Error("after the commit point the restored database's logo is missing")
			}
		}
		return nil
	}
	t.Cleanup(func() { swapHook = nil })
	rr, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "before-commit,after-commit" {
		t.Errorf("hook points: %v", seen)
	}
	if has(newer) || !has(testLogo) {
		t.Error("after the restore the logos are not exactly the backup's")
	}
	if _, err := os.Stat(filepath.Join(rr.ReplacedUploads, newer)); err != nil {
		t.Errorf("the replaced logo was not kept aside: %v", err)
	}
	if len(rr.Warnings) != 0 {
		t.Errorf("warnings: %v", rr.Warnings)
	}
}

// A failure before the commit point puts everything back; when putting back
// fails too, the error says so (ErrUndoFailed) instead of claiming nothing
// changed.
func TestRestoreReportsFailedUndo(t *testing.T) {
	for _, undoBreaks := range []bool{false, true} {
		t.Run(map[bool]string{false: "undo works", true: "undo fails"}[undoBreaks], func(t *testing.T) {
			in, archive, newer := restoreOverLiveInstance(t)
			original := sum(t, in.cfg.DBPath)
			swapHook = func(point string) error {
				if point != "before-commit" {
					return nil
				}
				if undoBreaks {
					// Make removing the aside copy impossible: a folder that is not empty.
					matches, _ := filepath.Glob(in.cfg.DBPath + replacedMarker + "*")
					for _, m := range matches {
						if strings.HasSuffix(m, "-wal") || strings.HasSuffix(m, "-journal") {
							continue
						}
						os.Remove(m)
						os.MkdirAll(filepath.Join(m, "x"), 0o700)
					}
				}
				return errors.New("injected failure")
			}
			t.Cleanup(func() { swapHook = nil })
			_, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
			if err == nil || !strings.Contains(err.Error(), "injected failure") {
				t.Fatalf("restore: %v", err)
			}
			if errors.Is(err, ErrUndoFailed) != undoBreaks {
				t.Errorf("ErrUndoFailed = %v, want %v (%v)", errors.Is(err, ErrUndoFailed), undoBreaks, err)
			}
			if sum(t, in.cfg.DBPath) != original {
				t.Error("the database changed")
			}
			if _, err := os.Stat(filepath.Join(in.cfg.UploadsDir(), newer)); err != nil {
				t.Error("the current logo is gone")
			}
			if !undoBreaks {
				assertNoLeftovers(t, in.cfg)
			}
		})
	}
}

// Restore refuses, before unpacking, a backup whose files would not fit.
func TestRestoreChecksFreeSpace(t *testing.T) {
	in, archive, _ := restoreOverLiveInstance(t)
	original := sum(t, in.cfg.DBPath)
	diskFree = func(string) (uint64, bool) { return 1 << 20, true }
	t.Cleanup(func() { diskFree = freeSpace })
	_, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
	if err == nil || !strings.Contains(err.Error(), "not enough free space") {
		t.Fatalf("restore: %v", err)
	}
	if sum(t, in.cfg.DBPath) != original {
		t.Error("the database changed")
	}
	assertNoLeftovers(t, in.cfg)
}

// At server start, leftovers of earlier processes are removed: restore
// staging folders and download copies of any age, temporary backup files
// older than a day; a recent temporary file (a backup in progress) stays.
func TestCleanUpAtStart(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DBPath: filepath.Join(dir, "tennis.db"), Dir: filepath.Join(dir, "backups")}
	os.MkdirAll(cfg.Dir, 0o700)
	staging := filepath.Join(dir, restoreStagingPrefix+"42")
	os.MkdirAll(staging, 0o700)
	download := filepath.Join(cfg.Dir, downloadPrefix+"20260101T000000Z"+archiveExtension)
	oldTmp := filepath.Join(cfg.Dir, tmpPrefix+"old"+archiveExtension)
	newTmp := filepath.Join(cfg.Dir, tmpPrefix+"new"+archiveExtension)
	kept := filepath.Join(cfg.Dir, regularPrefix+"20260101T000000Z"+archiveExtension)
	for _, p := range []string{download, oldTmp, newTmp, kept} {
		os.WriteFile(p, []byte("x"), 0o600)
	}
	long := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldTmp, long, long)
	if n := CleanUpAtStart(cfg); n != 3 {
		t.Errorf("removed %d, want 3", n)
	}
	for p, want := range map[string]bool{staging: false, download: false, oldTmp: false, newTmp: true, kept: true} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s: exists=%v, want %v", filepath.Base(p), err == nil, want)
		}
	}
}

// Download copies not removed after sending (the server stopped
// mid-download) go after an hour, not a day.
func TestStaleDownloadCopiesHaveShortLifetime(t *testing.T) {
	dir := t.TempDir()
	download := filepath.Join(dir, downloadPrefix+"20260101T000000Z"+archiveExtension)
	os.WriteFile(download, []byte("x"), 0o600)
	twoHours := time.Now().Add(-2 * time.Hour)
	os.Chtimes(download, twoHours, twoHours)
	if _, err := prune(Config{Dir: dir, Keep: 3}, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(download); err == nil {
		t.Error("a two-hour-old download copy is still there")
	}
}

// Backups finishing at once never take (or overwrite) each other's name.
func TestPublishNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	const n = 40
	start := make(chan struct{})
	type out struct {
		path string
		err  error
	}
	results := make(chan out, n)
	for i := range n {
		tmp := filepath.Join(dir, tmpPrefix+strconv.Itoa(i))
		os.WriteFile(tmp, []byte(strconv.Itoa(i)), 0o600)
		go func() {
			<-start
			p, err := publish(tmp, dir, downloadPrefix+"20260101T000000Z")
			results <- out{p, err}
		}()
	}
	close(start)
	paths := map[string]bool{}
	for range n {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if paths[r.path] {
			t.Errorf("two backups were given %s", filepath.Base(r.path))
		}
		paths[r.path] = true
	}
	contents := map[string]bool{}
	for p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(p), err)
			continue
		}
		contents[string(b)] = true
	}
	if len(contents) != n {
		t.Errorf("%d distinct backups survived, want %d", len(contents), n)
	}
}
