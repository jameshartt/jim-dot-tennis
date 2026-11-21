// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/setup"
)

// cliTestDB creates a migrated database in a temp dir, points DB_PATH at it
// (the way the server finds its database) and returns a second connection to
// it, standing in for the running server.
func cliTestDB(t *testing.T) *database.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tennis.db")
	t.Setenv("DB_PATH", path)
	t.Setenv("DB_TYPE", "")
	t.Setenv("HOME_CLUB_ID", "")
	t.Setenv("HOME_CLUB_NAME", "")
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	root, err := getProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ExecuteMigrations(filepath.Join(root, "migrations")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.SetOutput(os.Stderr); log.SetFlags(log.LstdFlags) })
	return db
}

type cliResult struct {
	code   int
	stdout string
	stderr string
}

func runCLITest(t *testing.T, args ...string) cliResult {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runCLI(args, &out, &errOut)
	return cliResult{code: code, stdout: out.String(), stderr: errOut.String()}
}

var tempPasswordRE = regexp.MustCompile(`(?m)^    ([a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4})$`)

func tempPassword(t *testing.T, stdout string) string {
	t.Helper()
	m := tempPasswordRE.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("no temporary password in output:\n%s", stdout)
	}
	return m[1]
}

func loadUser(t *testing.T, db *database.DB, username string) models.User {
	t.Helper()
	var u models.User
	if err := db.Get(&u, `SELECT * FROM users WHERE username = ?`, username); err != nil {
		t.Fatalf("load %s: %v", username, err)
	}
	return u
}

func TestCLIUserCommands(t *testing.T) {
	db := cliTestDB(t)

	// create
	r := runCLITest(t, "user", "create", "alice", "--role", "admin")
	if r.code != exitOK {
		t.Fatalf("create alice: %d %s", r.code, r.stderr)
	}
	alicePw := tempPassword(t, r.stdout)
	alice := loadUser(t, db, "alice")
	if alice.Role != models.RoleAdmin || !alice.IsActive || !alice.MustChangePassword || !auth.PasswordMatches(alice.PasswordHash, alicePw) {
		t.Fatalf("alice after create: %+v", alice)
	}
	if !strings.Contains(r.stderr, "action=user.create") || !strings.Contains(r.stderr, `actor="cli:user create"`) || strings.Contains(r.stderr, alicePw) {
		t.Errorf("audit record for create: %q", r.stderr)
	}
	if r := runCLITest(t, "user", "create", "--role=captain", "bob"); r.code != exitOK {
		t.Fatalf("create bob: %d %s", r.code, r.stderr)
	}
	if r := runCLITest(t, "user", "create", "ALICE", "--role", "player"); r.code != exitError || !strings.Contains(r.stderr, "already used") {
		t.Errorf("duplicate create: %+v", r)
	}

	// list
	r = runCLITest(t, "user", "list")
	if r.code != exitOK || !strings.Contains(r.stdout, "alice") || !strings.Contains(r.stdout, "bob") ||
		!strings.Contains(r.stdout, "1 active administrator(s)") || strings.Contains(r.stdout, "$2a$") {
		t.Fatalf("list: %+v", r)
	}

	// last-administrator guard from the command line
	if r := runCLITest(t, "user", "disable", "alice"); r.code != exitError || !strings.Contains(r.stderr, "last active administrator") {
		t.Errorf("disable last admin: %+v", r)
	}
	if r := runCLITest(t, "user", "role", "alice", "captain"); r.code != exitError || !strings.Contains(r.stderr, "last active administrator") {
		t.Errorf("demote last admin: %+v", r)
	}
	if !loadUser(t, db, "alice").IsActive {
		t.Fatal("alice disabled despite the guard")
	}

	// reset ends sessions (the server's sessions, written through its own connection)
	for _, id := range []string{"sess-1", "sess-2"} {
		if _, err := db.Exec(`INSERT INTO sessions (id, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
			VALUES (?, ?, 'admin', CURRENT_TIMESTAMP, datetime('now', '+1 day'), CURRENT_TIMESTAMP, '', '', '', TRUE)`, id, alice.ID); err != nil {
			t.Fatal(err)
		}
	}
	r = runCLITest(t, "user", "reset", "alice")
	if r.code != exitOK || !strings.Contains(r.stdout, "signed it out of 2 session(s)") {
		t.Fatalf("reset: %+v", r)
	}
	newPw := tempPassword(t, r.stdout)
	alice = loadUser(t, db, "alice")
	if !auth.PasswordMatches(alice.PasswordHash, newPw) || auth.PasswordMatches(alice.PasswordHash, alicePw) || !alice.MustChangePassword {
		t.Error("reset did not replace the password with the temporary one")
	}
	var live int
	if err := db.Get(&live, `SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid = TRUE`, alice.ID); err != nil || live != 0 {
		t.Errorf("%d sessions still valid after reset (%v)", live, err)
	}
	if strings.Contains(r.stderr, newPw) || !strings.Contains(r.stderr, "action=user.reset_password") {
		t.Errorf("audit record for reset: %q", r.stderr)
	}

	// role, disable, enable, sessions revoke
	if r := runCLITest(t, "user", "role", "bob", "admin"); r.code != exitOK || loadUser(t, db, "bob").Role != models.RoleAdmin {
		t.Fatalf("promote bob: %+v", r)
	}
	if r := runCLITest(t, "user", "disable", "alice"); r.code != exitOK || loadUser(t, db, "alice").IsActive {
		t.Fatalf("disable alice with bob admin: %+v", r)
	}
	if r := runCLITest(t, "user", "disable", "bob"); r.code != exitError {
		t.Fatalf("disable bob, now the last active admin: %+v", r)
	}
	if r := runCLITest(t, "user", "enable", "alice"); r.code != exitOK || !loadUser(t, db, "alice").IsActive {
		t.Fatalf("enable alice: %+v", r)
	}
	if r := runCLITest(t, "user", "sessions", "revoke", "bob"); r.code != exitOK || !strings.Contains(r.stdout, "0 session(s)") || !strings.Contains(r.stderr, "action=user.sessions_revoke") {
		t.Fatalf("revoke bob: %+v", r)
	}
	if r := runCLITest(t, "user", "reset", "nobody"); r.code != exitError || !strings.Contains(r.stderr, "no account") {
		t.Errorf("reset unknown user: %+v", r)
	}
}

func TestCLINeverTakesPasswords(t *testing.T) {
	cliTestDB(t)
	for _, args := range [][]string{
		{"user", "create", "alice", "--role", "admin", "--password", "hunter2hunter2"},
		{"user", "create", "alice", "--role", "admin", "--password=hunter2hunter2"},
		{"user", "reset", "alice", "-pass", "x"},
	} {
		r := runCLITest(t, args...)
		if r.code != exitUsage || !strings.Contains(r.stderr, "never given on the command line") {
			t.Errorf("%v: %+v", args, r)
		}
		if strings.Contains(r.stdout+r.stderr, "hunter2") {
			t.Errorf("%v: password echoed", args)
		}
	}
}

func TestCLIUsageAndHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"user", "--help"}, {"-h"}} {
		r := runCLITest(t, args...)
		if r.code != exitOK || !strings.Contains(r.stdout, "user reset <username>") || !strings.Contains(r.stdout, "already controls the server") {
			t.Errorf("%v: %+v", args, r)
		}
	}
	for _, args := range [][]string{
		{"user"}, {"user", "frobnicate"}, {"user", "create", "alice"}, {"user", "create", "alice", "--role", "owner"},
		{"user", "reset"}, {"user", "role", "alice"}, {"setup"}, {"serve"}, {"user", "list", "--verbose"},
	} {
		if r := runCLITest(t, args...); r.code != exitUsage {
			t.Errorf("%v: code %d, want usage error", args, r.code)
		}
	}
}

func TestCLIDatabaseChecks(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "missing.db"))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	r := runCLITest(t, "user", "list")
	if r.code != exitError || !strings.Contains(r.stderr, "there is no database") {
		t.Fatalf("missing database: %+v", r)
	}
	if _, err := os.Stat(os.Getenv("DB_PATH")); !os.IsNotExist(err) {
		t.Fatal("the command created a database file")
	}

	db := cliTestDB(t)
	if _, err := db.Exec(`UPDATE schema_migrations SET version = version - 1`); err != nil {
		t.Fatal(err)
	}
	r = runCLITest(t, "user", "list")
	if r.code != exitError || !strings.Contains(r.stderr, "Start (or restart) the app once") {
		t.Fatalf("schema behind: %+v", r)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET version = version + 1, dirty = TRUE`); err != nil {
		t.Fatal(err)
	}
	if r := runCLITest(t, "user", "list"); r.code != exitError || !strings.Contains(r.stderr, "part-way through an upgrade") {
		t.Fatalf("dirty schema: %+v", r)
	}
}

var setupCodeRE = regexp.MustCompile(`(?m)^    ([2-9A-Z]{4}-[2-9A-Z]{4}-[2-9A-Z]{4})$`)

func TestCLISetupCode(t *testing.T) {
	ctx := context.Background()
	db := cliTestDB(t)

	// The running server's setup service, with its startup code.
	holder, err := config.NewHomeClubHolder(ctx, repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	server := setup.NewService(setup.NewState(db, holder), nil)
	startupCode, err := server.IssueCode(ctx)
	if err != nil || startupCode == "" {
		t.Fatalf("startup code: %q %v", startupCode, err)
	}

	r := runCLITest(t, "setup", "code")
	if r.code != exitOK {
		t.Fatalf("setup code: %+v", r)
	}
	m := setupCodeRE.FindStringSubmatch(r.stdout)
	if m == nil {
		t.Fatalf("no code in output:\n%s", r.stdout)
	}
	code := m[1]
	if strings.Contains(r.stderr, code) || !strings.Contains(r.stderr, "action=setup.code_issue") {
		t.Errorf("audit record for setup code: %q", r.stderr)
	}

	// The earlier code is cancelled; the new one works once, in the server.
	if _, err := server.RedeemCode(ctx, startupCode, "client"); err == nil {
		t.Fatal("startup code still works after a new code was issued")
	}
	if _, err := server.RedeemCode(ctx, code, "client"); err != nil {
		t.Fatalf("redeem the new code in the server: %v", err)
	}
	if _, err := server.RedeemCode(ctx, code, "client"); err == nil {
		t.Fatal("the code worked twice")
	}

	// Once an administrator exists no code is issued.
	if r := runCLITest(t, "user", "create", "alice", "--role", "admin"); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	r = runCLITest(t, "setup", "code")
	if r.code != exitOK || !strings.Contains(r.stdout, "No setup code is needed") || setupCodeRE.MatchString(r.stdout) {
		t.Fatalf("setup code with an administrator: %+v", r)
	}
}

// TestHelpTextInOperationsDoc: the operations guide reproduces the help text.
func TestHelpTextInOperationsDoc(t *testing.T) {
	root, err := getProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(filepath.Join(root, "docs", "PRODUCTION_QUICK_REFERENCE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), cliHelp) {
		t.Fatal("docs/PRODUCTION_QUICK_REFERENCE.md does not contain the current help text; copy cliHelp into section 10")
	}
}
