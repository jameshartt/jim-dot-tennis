// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests pin the behaviour the app relied on under mattn/go-sqlite3 and
// must keep under modernc.org/sqlite (WI-136).

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(Config{Driver: DriverSQLite, FilePath: filepath.Join(t.TempDir(), "driver.db")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Every pooled connection gets the pragmas, not only the first one.
func TestEveryPooledConnectionHasSettings(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	var conns []*sql.Conn
	for range 5 {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		var fk, busy int64
		var jm string
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&jm); err != nil {
			t.Fatal(err)
		}
		if fk != 1 || busy != 5000 || jm != "wal" {
			t.Errorf("conn %d: foreign_keys=%d busy_timeout=%d journal_mode=%s", i, fk, busy, jm)
		}
	}
	for _, c := range conns {
		c.Close()
	}
	if err := db.CheckConnectionSettings(ctx); err != nil {
		t.Fatalf("CheckConnectionSettings: %v", err)
	}
}

// The self-check refuses a connection without the settings.
func TestCheckConnectionSettingsDetectsMissingPragmas(t *testing.T) {
	db := newTestDB(t)
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatal(err)
	}
	err := db.CheckConnectionSettings(context.Background())
	if err == nil || !strings.Contains(err.Error(), "foreign_keys=0") {
		t.Fatalf("CheckConnectionSettings = %v, want foreign_keys failure", err)
	}
}

// time.Time is written in the layout mattn/go-sqlite3 used, and every layout
// found in existing (mattn-written or CURRENT_TIMESTAMP) rows reads back.
func TestTimeStorageMatchesExistingRows(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE ts (id INTEGER PRIMARY KEY, at TIMESTAMP, d DATETIME)`); err != nil {
		t.Fatal(err)
	}
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	written := time.Date(2026, 6, 1, 19, 30, 5, 123456789, london)
	if _, err := db.Exec(`INSERT INTO ts (id, at, d) VALUES (1, ?, ?)`, written, written.UTC()); err != nil {
		t.Fatal(err)
	}
	var raw, rawUTC string
	if err := db.QueryRow(`SELECT CAST(at AS TEXT), CAST(d AS TEXT) FROM ts WHERE id = 1`).Scan(&raw, &rawUTC); err != nil {
		t.Fatal(err)
	}
	if want := "2026-06-01 19:30:05.123456789+01:00"; raw != want {
		t.Errorf("stored %q, want %q (mattn layout)", raw, want)
	}
	if want := "2026-06-01 18:30:05.123456789+00:00"; rawUTC != want {
		t.Errorf("stored UTC %q, want %q", rawUTC, want)
	}

	// Layouts present in a production database written by mattn/go-sqlite3.
	legacy := map[string]time.Time{
		"2026-04-20 18:45:12.123456789+01:00": time.Date(2026, 4, 20, 17, 45, 12, 123456789, time.UTC),
		"2026-04-20 18:45:12.12345678+01:00":  time.Date(2026, 4, 20, 17, 45, 12, 123456780, time.UTC),
		"2026-04-20 18:45:12.1234+00:00":      time.Date(2026, 4, 20, 18, 45, 12, 123400000, time.UTC),
		"2026-04-20 18:45:12+01:00":           time.Date(2026, 4, 20, 17, 45, 12, 0, time.UTC),
		"2026-04-20 18:45:12":                 time.Date(2026, 4, 20, 18, 45, 12, 0, time.UTC), // CURRENT_TIMESTAMP, UTC
		"2026-04-20T18:45:12Z":                time.Date(2026, 4, 20, 18, 45, 12, 0, time.UTC),
		"2026-04-20":                          time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
	}
	i := 10
	for text, want := range legacy {
		i++
		if _, err := db.Exec(`INSERT INTO ts (id, at) VALUES (?, ?)`, i, text); err != nil {
			t.Fatal(err)
		}
		var got time.Time
		if err := db.QueryRow(`SELECT at FROM ts WHERE id = ?`, i).Scan(&got); err != nil {
			t.Errorf("scan %q: %v", text, err)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("scan %q = %v, want %v", text, got, want)
		}
	}

	// Round trip, and a nullable column.
	var got time.Time
	if err := db.QueryRow(`SELECT at FROM ts WHERE id = 1`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(written) {
		t.Errorf("round trip = %v, want %v", got, written)
	}
	var null sql.NullTime
	if err := db.QueryRow(`SELECT d FROM ts WHERE id = 11`).Scan(&null); err != nil || null.Valid {
		t.Errorf("NULL DATETIME = %v, %v; want invalid NullTime", null, err)
	}

	// Comparisons in SQL against datetime('now') behave as before.
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM ts WHERE at < datetime('now')`); err != nil {
		t.Fatal(err)
	}
	if n != len(legacy)+1 {
		t.Errorf("rows before now = %d, want %d", n, len(legacy)+1)
	}
}

// Booleans are stored as 0/1 and read back into bool from BOOLEAN columns.
func TestBooleanStorage(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE b (id INTEGER PRIMARY KEY, v BOOLEAN NOT NULL DEFAULT FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO b (id, v) VALUES (1, ?), (2, ?), (3, TRUE)`, true, false); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.Get(&stored, `SELECT group_concat(x, ',') FROM (SELECT typeof(v) || ':' || v AS x FROM b ORDER BY id)`); err != nil {
		t.Fatal(err)
	}
	if stored != "integer:1,integer:0,integer:1" {
		t.Errorf("stored booleans %q", stored)
	}
	var row struct {
		V bool `db:"v"`
	}
	if err := db.Get(&row, `SELECT v FROM b WHERE id = 1`); err != nil || !row.V {
		t.Errorf("scan true = %v, %v", row.V, err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM b WHERE v = ?`, true); err != nil || n != 2 {
		t.Errorf("WHERE v = true: %d, %v", n, err)
	}
}

// Constraint errors keep SQLite's message, which code matches on
// (internal/repository/webauthn_credential.go), and foreign keys cascade.
func TestConstraintErrorsAndForeignKeys(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`
		CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT UNIQUE);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent(id) ON DELETE CASCADE);
		INSERT INTO parent (id, name) VALUES (1, 'a');
		INSERT INTO child (parent_id) VALUES (1);`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO parent (id, name) VALUES (2, 'a')`)
	if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed: parent.name") {
		t.Errorf("unique violation error = %v", err)
	}
	_, err = db.Exec(`INSERT INTO child (parent_id) VALUES (99)`)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("foreign key violation error = %v", err)
	}
	if _, err := db.Exec(`DELETE FROM parent WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM child`); err != nil || n != 0 {
		t.Errorf("children after cascade = %d, %v", n, err)
	}
}

// Concurrent writers wait on the busy timeout instead of failing with
// "database is locked".
func TestConcurrentWritersWait(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE w (id INTEGER PRIMARY KEY, g INTEGER, at TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	const writers, each = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for g := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				tx, err := db.Beginx()
				if err != nil {
					errs <- err
					return
				}
				if _, err := tx.Exec(`INSERT INTO w (g, at) VALUES (?, ?)`, g, time.Now()); err != nil {
					tx.Rollback()
					errs <- fmt.Errorf("writer %d insert %d: %w", g, i, err)
					return
				}
				if err := tx.Commit(); err != nil {
					errs <- fmt.Errorf("writer %d commit %d: %w", g, i, err)
					return
				}
				// A reader alongside the writers.
				var n int
				if err := db.Get(&n, `SELECT COUNT(*) FROM w`); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM w`); err != nil || n != writers*each {
		t.Errorf("rows = %d, %v; want %d", n, err, writers*each)
	}
}
