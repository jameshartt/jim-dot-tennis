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

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/setup"
	"jim-dot-tennis/internal/version"
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

// A printed link: the public address (when set) and /account/link/<token>.
var (
	fullLinkRE = regexp.MustCompile(`(?m)^    (https://tennis\.example\.org/account/link/([A-Za-z0-9_-]{43}))$`)
	pathLinkRE = regexp.MustCompile(`(?m)^    (/account/link/([A-Za-z0-9_-]{43}))$`)
)

// printedToken returns the token of the one link in stdout, which must be a
// full address when full is set and a bare path otherwise.
func printedToken(t *testing.T, stdout string, full bool) string {
	t.Helper()
	re := pathLinkRE
	if full {
		re = fullLinkRE
	}
	m := re.FindAllStringSubmatch(stdout, -1)
	if len(m) != 1 {
		t.Fatalf("want exactly one link (full address: %v) in output:\n%s", full, stdout)
	}
	return m[0][2]
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
	t.Setenv("PUBLIC_URL", "https://tennis.example.org")
	ctx := context.Background()
	svc := accounts.New(db)

	// create: an account with no password and an invite link, printed once.
	r := runCLITest(t, "user", "create", "alice", "--role", "admin")
	if r.code != exitOK {
		t.Fatalf("create alice: %d %s", r.code, r.stderr)
	}
	aliceInvite := printedToken(t, r.stdout, true)
	alice := loadUser(t, db, "alice")
	if alice.Role != models.RoleAdmin || !alice.IsActive || alice.PasswordHash != "" {
		t.Fatalf("alice after create: %+v", alice)
	}
	if !strings.Contains(r.stdout, "48 hours") || !strings.Contains(r.stdout, "There is no email") {
		t.Errorf("create output: %s", r.stdout)
	}
	for _, want := range []string{"action=user.create", "action=link.create", `actor="cli:user create"`, `target="alice"`, "kind=invite"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("audit records for create lack %q: %q", want, r.stderr)
		}
	}
	if strings.Contains(r.stderr, aliceInvite) {
		t.Error("the audit record contains the link")
	}
	// The command line's actions are in the account audit log table too.
	var cliRows []string
	if err := db.Select(&cliRows, `SELECT action || ' ' || target_username FROM account_audit_log
		WHERE actor_kind = 'cli' AND actor = 'user create' AND actor_user_id IS NULL ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	if strings.Join(cliRows, ",") != "user.create alice,link.create alice" {
		t.Errorf("audit log rows for create: %q", cliRows)
	}
	if link, err := svc.LookupLink(ctx, aliceInvite); err != nil || link.Kind != accounts.LinkInvite || link.Username != "alice" {
		t.Fatalf("the printed invite link: %+v %v", link, err)
	}
	if r := runCLITest(t, "user", "create", "--role=captain", "bob"); r.code != exitOK {
		t.Fatalf("create bob: %d %s", r.code, r.stderr)
	}
	if r := runCLITest(t, "user", "create", "ALICE", "--role", "captain"); r.code != exitError || !strings.Contains(r.stderr, "already used") {
		t.Errorf("duplicate create: %+v", r)
	}

	// list
	r = runCLITest(t, "user", "list")
	if r.code != exitOK || !strings.Contains(r.stdout, "alice") || !strings.Contains(r.stdout, "bob") ||
		!strings.Contains(r.stdout, "1 active administrator(s)") || !strings.Contains(r.stdout, "invite not used") ||
		strings.Contains(r.stdout, "$2a$") || strings.Contains(r.stdout, aliceInvite) {
		t.Fatalf("list: %+v", r)
	}

	// last-administrator guard from the command line
	if r := runCLITest(t, "user", "disable", "alice"); r.code != exitError || !strings.Contains(r.stderr, "last administrator who can sign in") {
		t.Errorf("disable last admin: %+v", r)
	}
	if r := runCLITest(t, "user", "role", "alice", "captain"); r.code != exitError || !strings.Contains(r.stderr, "last administrator who can sign in") {
		t.Errorf("demote last admin: %+v", r)
	}
	if !loadUser(t, db, "alice").IsActive {
		t.Fatal("alice disabled despite the guard")
	}

	// alice accepts her invite (in the browser this is the link page).
	if _, err := svc.RedeemLink(ctx, aliceInvite, accounts.Redemption{Password: "her very own secret"}); err != nil {
		t.Fatal(err)
	}

	// reset: prints a recovery link and changes nothing until it is used
	// (the server's sessions, written through its own connection, stay valid).
	for _, id := range []string{"sess-1", "sess-2"} {
		if _, err := db.Exec(`INSERT INTO sessions (token_hash, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
			VALUES (?, ?, 'admin', CURRENT_TIMESTAMP, datetime('now', '+1 day'), CURRENT_TIMESTAMP, '', '', '', TRUE)`, auth.HashSessionToken(id), alice.ID); err != nil {
			t.Fatal(err)
		}
	}
	liveSessions := func() int {
		var live int
		if err := db.Get(&live, `SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid = TRUE`, alice.ID); err != nil {
			t.Fatal(err)
		}
		return live
	}
	r = runCLITest(t, "user", "reset", "alice")
	if r.code != exitOK || !strings.Contains(r.stdout, "1 hour") || !strings.Contains(r.stdout, "Nothing has changed") {
		t.Fatalf("reset: %+v", r)
	}
	recovery := printedToken(t, r.stdout, true)
	alice = loadUser(t, db, "alice")
	if !auth.PasswordMatches(string(alice.PasswordHash), "her very own secret") || liveSessions() != 2 {
		t.Error("issuing a recovery link changed the account")
	}
	if strings.Contains(r.stderr, recovery) || !strings.Contains(r.stderr, "action=link.create") || !strings.Contains(r.stderr, "kind=recovery") ||
		!strings.Contains(r.stderr, `actor="cli:user reset"`) {
		t.Errorf("audit record for reset: %q", r.stderr)
	}
	// A second reset cancels the first link.
	r = runCLITest(t, "user", "reset", "alice")
	if r.code != exitOK || !strings.Contains(r.stdout, "earlier unused link") {
		t.Fatalf("second reset: %+v", r)
	}
	if _, err := svc.LookupLink(ctx, recovery); err == nil {
		t.Error("the first recovery link still works after a second one was issued")
	}
	recovery = printedToken(t, r.stdout, true)
	// Using the link ends every session and sets the new password.
	if _, err := svc.RedeemLink(ctx, recovery, accounts.Redemption{Password: "a recovered secret"}); err != nil {
		t.Fatalf("use the printed recovery link: %v", err)
	}
	if liveSessions() != 0 || !auth.PasswordMatches(string(loadUser(t, db, "alice").PasswordHash), "a recovered secret") {
		t.Error("using the recovery link did not end the sessions and set the password")
	}

	// role, disable, enable, sessions revoke
	if r := runCLITest(t, "user", "role", "bob", "admin"); r.code != exitOK || loadUser(t, db, "bob").Role != models.RoleAdmin {
		t.Fatalf("promote bob: %+v", r)
	}
	// bob never used his invite, so he cannot sign in: alice is still the last
	// administrator who can (WI-134 F5).
	if r := runCLITest(t, "user", "disable", "alice"); r.code != exitError || !loadUser(t, db, "alice").IsActive {
		t.Fatalf("disable alice while the other administrator cannot sign in: %+v", r)
	}
	if _, err := db.Exec(`UPDATE users SET password_hash = ? WHERE username = 'bob'`, mustHashForTest(t)); err != nil {
		t.Fatal(err)
	}
	if r := runCLITest(t, "user", "disable", "alice"); r.code != exitOK || loadUser(t, db, "alice").IsActive {
		t.Fatalf("disable alice with bob admin: %+v", r)
	}
	if r := runCLITest(t, "user", "reset", "alice"); r.code != exitOK || !strings.Contains(r.stdout, "user enable alice") {
		t.Fatalf("reset a disabled account: %+v", r)
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

// Without a public URL the command line has no address to build the link
// from: it prints the path and says how to complete it, and never guesses.
func TestCLILinkWithoutPublicURL(t *testing.T) {
	db := cliTestDB(t)
	t.Setenv("PUBLIC_URL", "")
	r := runCLITest(t, "user", "create", "alice", "--role", "admin")
	if r.code != exitOK {
		t.Fatalf("create: %+v", r)
	}
	token := printedToken(t, r.stdout, false)
	if !strings.Contains(r.stdout, "no public address set") || !strings.Contains(r.stdout, "PUBLIC_URL") ||
		!strings.Contains(r.stdout, "https://tennis.example.org/account/link/"+token) {
		t.Errorf("output without a public URL:\n%s", r.stdout)
	}
	if strings.Contains(r.stdout, "localhost") {
		t.Errorf("the command guessed a host:\n%s", r.stdout)
	}
	if _, err := accounts.New(db).LookupLink(context.Background(), token); err != nil {
		t.Fatalf("the printed link: %v", err)
	}

	// The address stored on the Club Settings page is used when PUBLIC_URL is unset.
	if err := repository.NewSettingsRepository(db).SetString(context.Background(), repository.SettingPublicURL, "https://tennis.example.org"); err != nil {
		t.Fatal(err)
	}
	r = runCLITest(t, "user", "reset", "alice")
	if r.code != exitOK {
		t.Fatalf("reset: %+v", r)
	}
	printedToken(t, r.stdout, true)
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
	if !strings.Contains(string(doc), strings.TrimPrefix(cliTournamentsHelp, "\n")) {
		t.Fatal("docs/PRODUCTION_QUICK_REFERENCE.md does not contain the current tournament commands help; copy cliTournamentsHelp into section 10")
	}
}

// mustHashForTest is a password hash, for giving an account a way to sign in.
func mustHashForTest(t *testing.T) string {
	t.Helper()
	h, err := auth.HashPassword("a perfectly fine secret")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// `version` prints version, commit and date without needing a database.
func TestCLIVersion(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "none.db"))
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var out, errOut bytes.Buffer
		if code := runCLI(args, &out, &errOut); code != exitOK {
			t.Fatalf("%v: exit %d: %s", args, code, errOut.String())
		}
		if !strings.Contains(out.String(), "jim-dot-tennis "+version.Version) || !strings.Contains(out.String(), "commit") {
			t.Errorf("%v: output %q", args, out.String())
		}
	}
}
