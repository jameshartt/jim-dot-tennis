// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package secrets

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestIsPlaceholder(t *testing.T) {
	for _, v := range []string{"change-me", " CHANGE-ME ", "changeme", "example", "your-password", "your-password-here",
		"your-secure-password", "<wrapped-access-password>", "<anything>", "password", "secret"} {
		if !IsPlaceholder(v) {
			t.Errorf("IsPlaceholder(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "K7QP-2M9X-HR4T", "a-real-club-chosen-code", "change-me-please-not"} {
		if IsPlaceholder(v) {
			t.Errorf("IsPlaceholder(%q) = true, want false", v)
		}
	}
}

// Every "change-me"-style value in the repository's .env.example files must
// be recognised as a placeholder, so copying an example unchanged cannot
// reach production.
func TestEnvExamplePlaceholdersAreKnown(t *testing.T) {
	path := repoDir(t, "deploy") + "/.env.example"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	assign := regexp.MustCompile(`^#?\s*([A-Z][A-Z0-9_]*)=(.*)$`)
	sc := bufio.NewScanner(f)
	checked := 0
	for sc.Scan() {
		m := assign.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil || !strings.Contains(strings.ToLower(m[2]), "change") {
			continue
		}
		checked++
		if !IsPlaceholder(m[2]) {
			t.Errorf("%s: example value for %s is not in the placeholder list", path, m[1])
		}
	}
	if checked == 0 {
		t.Fatalf("no placeholder values found in %s; the test is not checking anything", path)
	}
}

func TestCheckEnv(t *testing.T) {
	env := map[string]string{EnvWrappedAccess: "change-me"}
	getenv := func(k string) string { return env[k] }
	if err := CheckEnv(false, getenv); err != nil {
		t.Fatalf("development mode must not refuse: %v", err)
	}
	err := CheckEnv(true, getenv)
	if err == nil {
		t.Fatal("production with a placeholder secret must refuse")
	}
	if !strings.Contains(err.Error(), EnvWrappedAccess) || strings.Contains(err.Error(), "change-me") {
		t.Fatalf("error must name the variable but not its value: %v", err)
	}
	env[EnvWrappedAccess] = ""
	if err := CheckEnv(true, getenv); err != nil {
		t.Fatalf("unset secret (generated per instance) must be accepted: %v", err)
	}
	env[EnvWrappedAccess] = "a-code-the-club-chose"
	if err := CheckEnv(true, getenv); err != nil {
		t.Fatalf("own value must be accepted: %v", err)
	}
}

func TestStartupCheckRefusesPlaceholderInProduction(t *testing.T) {
	db := newTestDB(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv(EnvWrappedAccess, "change-me")
	if err := StartupCheck(context.Background(), db); err == nil {
		t.Fatal("StartupCheck accepted a placeholder in production")
	}
	t.Setenv(EnvWrappedAccess, "")
	if err := StartupCheck(context.Background(), db); err != nil {
		t.Fatalf("StartupCheck with no env secret: %v", err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM app_settings WHERE key = ?`, SettingWrappedAccessCode); err != nil || n != 1 {
		t.Fatalf("StartupCheck should generate the Club Wrapped code (rows %d, err %v)", n, err)
	}
}

func TestKnownCredentialAccounts(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// The e2e seed's administrator has a published password.
	seed, err := os.ReadFile(repoDir(t, "tests") + "/e2e/fixtures/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	known := regexp.MustCompile(`\$2[aby]\$\d{2}\$[./A-Za-z0-9]{53}`).Find(seed)
	if known == nil {
		t.Fatal("no password hash found in the e2e seed")
	}
	own, err := bcrypt.GenerateFromPassword([]byte("a password only this club knows"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	db.MustExec(`INSERT INTO users (username, password_hash, role) VALUES ('seeded', ?, 'admin'), ('own', ?, 'admin')`, string(known), string(own))

	names, err := KnownCredentialAccounts(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "seeded" {
		t.Fatalf("KnownCredentialAccounts = %v, want [seeded]", names)
	}
}
