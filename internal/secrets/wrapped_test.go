// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package secrets

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"jim-dot-tennis/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

// repoDir walks up from the package directory to find a top-level directory.
func repoDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate %s", name)
	return ""
}

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(repoDir(t, "migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestAccess(db *database.DB, env map[string]string) *WrappedAccess {
	w := NewWrappedAccess(db)
	w.getenv = func(k string) string { return env[k] }
	return w
}

var codePattern = regexp.MustCompile(`^[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}$`)

func TestWrappedCodeGeneratedOnceAndStored(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	w := newTestAccess(db, nil)

	code, fromEnv, err := w.Code(ctx)
	if err != nil || fromEnv {
		t.Fatalf("Code() = fromEnv %v, err %v", fromEnv, err)
	}
	if !codePattern.MatchString(code) {
		t.Fatalf("generated code has unexpected format (length %d)", len(code))
	}
	again, _, err := newTestAccess(db, nil).Code(ctx)
	if err != nil || again != code {
		t.Fatalf("second Code() returned a different code (err %v)", err)
	}
	var stored string
	if err := db.Get(&stored, `SELECT value FROM app_settings WHERE key = ?`, SettingWrappedAccessCode); err != nil || stored != code {
		t.Fatalf("stored code differs from returned code (err %v)", err)
	}

	// Another instance (another database) gets a different code.
	other, _, err := newTestAccess(newTestDB(t), nil).Code(ctx)
	if err != nil || other == code {
		t.Fatalf("two instances share a Club Wrapped code (err %v)", err)
	}
}

func TestWrappedCodeConcurrentFirstUse(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	const n = 8
	codes := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _, err := newTestAccess(db, nil).Code(ctx)
			if err != nil {
				t.Error(err)
			}
			codes[i] = c
		}(i)
	}
	wg.Wait()
	for _, c := range codes[1:] {
		if c != codes[0] {
			t.Fatal("concurrent first uses produced different codes")
		}
	}
}

func TestWrappedCodeEnvOverrides(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	w := newTestAccess(db, map[string]string{EnvWrappedAccess: "  OurClubCode  "})
	code, fromEnv, err := w.Code(ctx)
	if err != nil || !fromEnv || code != "OurClubCode" {
		t.Fatalf("Code() with env = %q, fromEnv %v, err %v", code, fromEnv, err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM app_settings WHERE key = ?`, SettingWrappedAccessCode); err != nil || n != 0 {
		t.Fatalf("env code should not generate a stored code (rows %d, err %v)", n, err)
	}
	for in, want := range map[string]bool{"OurClubCode": true, " OurClubCode ": true, "ourclubcode": false, "": false, "granted": false} {
		if got, err := w.Check(ctx, in); err != nil || got != want {
			t.Errorf("Check(%q) = %v (err %v), want %v", in, got, err, want)
		}
	}
}

func TestWrappedCheckGeneratedCodeIsForgiving(t *testing.T) {
	ctx := context.Background()
	w := newTestAccess(newTestDB(t), nil)
	code, _, err := w.Code(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{code, " " + code + " ", normalizeCode(code), "x" + code[1:]} {
		got, err := w.Check(ctx, in)
		want := in != "x"+code[1:]
		if err != nil || got != want {
			t.Errorf("Check(variant) = %v (err %v), want %v", got, err, want)
		}
	}
	lower := []rune(code)
	for i, r := range lower {
		if r >= 'A' && r <= 'Z' {
			lower[i] = r + ('a' - 'A')
		}
	}
	if ok, _ := w.Check(ctx, string(lower)); !ok {
		t.Error("lower-case code rejected")
	}
}

func TestWrappedCookie(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	w := newTestAccess(db, nil)
	v, err := w.CookieValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	code, _, _ := w.Code(ctx)
	if v == code || v == "granted" || len(v) != 64 {
		t.Fatal("cookie value must be a derived digest, not the code or a constant")
	}
	if !w.CookieValid(ctx, v) {
		t.Fatal("current cookie rejected")
	}
	for _, bad := range []string{"", "granted", v[:63], code} {
		if w.CookieValid(ctx, bad) {
			t.Errorf("CookieValid accepted a forged value (len %d)", len(bad))
		}
	}
	// A different code (env override) invalidates the old cookie.
	wEnv := newTestAccess(db, map[string]string{EnvWrappedAccess: "another"})
	if wEnv.CookieValid(ctx, v) {
		t.Fatal("cookie survived a change of code")
	}
	// Another instance does not accept this instance's cookie.
	if newTestAccess(newTestDB(t), nil).CookieValid(ctx, v) {
		t.Fatal("cookie valid on another instance")
	}
}
