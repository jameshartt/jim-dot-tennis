// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jimdottennis "jim-dot-tennis"
)

// fakeTournament is a TournamentDatabase whose "database" is a string. A
// dump is a file holding "PGDMP" and that string.
type fakeTournament struct {
	content    string
	others     int
	stateErr   error
	dumpErr    error
	replaceErr func(call int) error
	replaces   int
	dumps      int
}

func (f *fakeTournament) Describe() string { return `the tournament database "fake"` }

func (f *fakeTournament) Dump(_ context.Context, path string) error {
	f.dumps++
	if f.dumpErr != nil {
		return f.dumpErr
	}
	return os.WriteFile(path, []byte("PGDMP"+f.content), 0o600)
}

func (f *fakeTournament) Inspect(_ context.Context, path string) (DumpInfo, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return DumpInfo{}, err
	}
	if !strings.HasPrefix(string(b), "PGDMP") {
		return DumpInfo{}, errors.New("pg_restore: input file does not appear to be a valid archive")
	}
	return DumpInfo{ServerVersion: "18.4", Entries: 3}, nil
}

func (f *fakeTournament) State(context.Context) (DatabaseState, error) {
	if f.stateErr != nil {
		return DatabaseState{}, f.stateErr
	}
	n := 0
	if f.content != "" {
		n = 5
	}
	return DatabaseState{Tables: n, Migrations: 64, LatestMigration: "064-support-cases.sql"}, nil
}

func (f *fakeTournament) OtherSessions(context.Context) (int, error) {
	if f.stateErr != nil {
		return 0, f.stateErr
	}
	return f.others, nil
}

func (f *fakeTournament) Replace(_ context.Context, path, _ string) error {
	f.replaces++
	if f.replaceErr != nil {
		if err := f.replaceErr(f.replaces); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f.content = strings.TrimPrefix(string(b), "PGDMP")
	return nil
}

// archiveNames lists an archive's entries in order.
func archiveNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var out []string
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, h.Name)
	}
}

// rawManifest is the manifest entry's text.
func rawManifest(t *testing.T, path string) string {
	t.Helper()
	f, _ := os.Open(path)
	defer f.Close()
	gz, _ := gzip.NewReader(f)
	tr := tar.NewReader(gz)
	if _, err := tr.Next(); err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(tr)
	return string(b)
}

// Without the tournament settings a backup is exactly what it was before
// tournament data existed: format 1, no tournament entry, the same files.
func TestBackupWithoutTournamentsIsUnchanged(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Format != 1 || res.Manifest.HasTournaments() {
		t.Errorf("manifest: %+v", res.Manifest)
	}
	if raw := rawManifest(t, res.Path); strings.Contains(raw, "tournament") {
		t.Errorf("an app-only manifest mentions tournaments: %s", raw)
	}
	if got := fmt.Sprint(archiveNames(t, res.Path)); got != "[manifest.json database/tennis.db files/branding/"+testLogo+"]" {
		t.Errorf("entries: %s", got)
	}
}

// With the settings, the archive holds the dump, the manifest describes it,
// and a restore after wiping both databases brings both back.
func TestBackupAndRestoreIncludeTournaments(t *testing.T) {
	ctx := context.Background()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	fake := &fakeTournament{content: "tournament with a draw"}
	in.cfg.Tournament = fake
	in.cfg.ServerImage = "ghcr.io/jameshartt/courthive-server:4.19.1-jdt.1"
	before := fingerprint(t, in.db)

	res, err := Create(ctx, in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	m := res.Manifest
	if m.Format != 2 || !m.HasTournaments() {
		t.Fatalf("manifest: %+v", m)
	}
	tm := m.Tournaments
	if tm.Path != tournamentName || tm.Format != TournamentDumpFormat || tm.ServerVersion != "18.4" || tm.Size != int64(len("PGDMPtournament with a draw")) ||
		tm.SHA256 == "" || tm.LatestMigration != "064-support-cases.sql" || tm.Migrations != 64 || tm.ServerImage != in.cfg.ServerImage {
		t.Errorf("tournament entry: %+v", tm)
	}
	if got := fmt.Sprint(archiveNames(t, res.Path)); got != "[manifest.json database/tennis.db tournaments/courthive.dump files/branding/"+testLogo+"]" {
		t.Errorf("entries: %s", got)
	}
	if v, err := VerifyFile(res.Path); err != nil || !v.HasTournaments() {
		t.Fatalf("verify: %+v %v", v, err)
	}

	// Wipe both.
	in.close()
	for _, p := range []string{in.cfg.DBPath, in.cfg.DBPath + "-wal", in.cfg.DBPath + "-shm", in.cfg.UploadsDir()} {
		os.RemoveAll(p)
	}
	fake.content = ""

	rr, err := Restore(ctx, in.cfg, res.Path, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if rr.Tournament != TournamentRestored || fake.content != "tournament with a draw" {
		t.Errorf("tournament restore: %v %q", rr.Tournament, fake.content)
	}
	if rr.TournamentSafetyCopy == "" || !strings.HasPrefix(filepath.Base(rr.TournamentSafetyCopy), "courthive.dump.replaced-") {
		t.Errorf("safety copy: %q", rr.TournamentSafetyCopy)
	} else if b, err := os.ReadFile(rr.TournamentSafetyCopy); err != nil || string(b) != "PGDMP" {
		t.Errorf("the safety copy holds %q (%v), want the empty database it replaced", b, err)
	}
	in.open()
	if after := fingerprint(t, in.db); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("app data differs after restore")
	}
	var detail string
	in.db.Get(&detail, `SELECT detail FROM account_audit_log WHERE action = 'backup.restore'`)
	if !strings.Contains(detail, "tournaments=restored") {
		t.Errorf("audit detail: %q", detail)
	}
	os.Remove(rr.TournamentSafetyCopy) // kept on purpose, like tennis.db.replaced-...
	assertNoLeftovers(t, in.cfg)
}

// A tournament dump that fails means no backup at all, and the schedule
// records the failure for the backups page.
func TestBackupFailsWhenTournamentDumpFails(t *testing.T) {
	for name, fake := range map[string]*fakeTournament{
		"unreachable":  {content: "x", stateErr: errors.New(`psql failed: connection to server at "postgres" failed: Connection refused`)},
		"dump failing": {content: "x", dumpErr: errors.New("pg_dump failed: password authentication failed for user \"courthive\"")},
	} {
		t.Run(name, func(t *testing.T) {
			in := newInstance(t, jimdottennis.Migrations())
			in.cfg.Tournament = fake
			_, err := Create(context.Background(), in.db, in.cfg, KindManual)
			if err == nil || !strings.Contains(err.Error(), "the tournament database could not be backed up, so no backup was made") {
				t.Fatalf("Create: %v", err)
			}
			if entries, _ := os.ReadDir(in.cfg.Dir); len(entries) != 0 {
				t.Errorf("left in the backups folder: %v", entries)
			}
			s := NewService(in.db, in.cfg)
			if s.RunIfDue(context.Background()) {
				t.Fatal("a failed scheduled backup was counted")
			}
			if st := s.Status(); !strings.Contains(st.LastError, "tournament database") || !st.LastSuccess.IsZero() {
				t.Errorf("status: %+v", st)
			}
			if _, ok, _ := Newest(in.cfg.Dir, time.Now(), true); ok {
				t.Error("a backup is listed")
			}
		})
	}
}

// The app's own pre-migration backup never needs the tournament database:
// it works with Postgres down and is an app-only archive.
func TestBeforeMigrationsIsAppOnly(t *testing.T) {
	older := newInstance(t, migrationsUpTo(t, 33))
	older.seed()
	fake := &fakeTournament{content: "x", stateErr: errors.New("down")}
	older.cfg.Tournament = fake
	p, err := BeforeMigrations(context.Background(), older.db, older.cfg, jimdottennis.Migrations())
	if err != nil || p == "" {
		t.Fatalf("BeforeMigrations: %q %v", p, err)
	}
	if m, err := VerifyFile(p); err != nil || m.HasTournaments() || m.Format != 1 {
		t.Errorf("manifest: %+v %v", m, err)
	}
	if fake.dumps != 0 {
		t.Error("the tournament database was dumped")
	}
}

// tournamentArchive is an instance with a tournament database and a backup
// of both; the instance and its tournament database then change.
func tournamentArchive(t *testing.T) (*instance, *fakeTournament, string, string) {
	t.Helper()
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	fake := &fakeTournament{content: "backed up"}
	in.cfg.Tournament = fake
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.exec(`INSERT INTO clubs (name) VALUES ('Added after the backup')`)
	in.close()
	fake.content = "current"
	return in, fake, res.Path, sum(t, in.cfg.DBPath)
}

// While anything else is connected to the tournament database, restore
// refuses, says how to stop the services, and changes nothing.
func TestRestoreRefusesWhileTournamentServicesRun(t *testing.T) {
	in, fake, archive, original := tournamentArchive(t)
	fake.others = 3
	_, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
	if !errors.Is(err, ErrTournamentInUse) || !strings.Contains(err.Error(), StopTournamentServices) || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("restore: %v", err)
	}
	if fake.content != "current" || fake.replaces != 0 || sum(t, in.cfg.DBPath) != original {
		t.Error("something changed")
	}
	assertNoLeftovers(t, in.cfg)
	if m, _ := filepath.Glob(filepath.Join(in.cfg.DataDir(), "courthive.dump*")); len(m) != 0 {
		t.Errorf("left %v", m)
	}
}

// A tournament restore that fails (and is rolled back) leaves both
// databases as they were.
func TestRestoreTournamentFailureChangesNothing(t *testing.T) {
	in, fake, archive, original := tournamentArchive(t)
	fake.replaceErr = func(int) error { return errors.New("psql failed: ERROR: injected") }
	_, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
	if err == nil || !strings.Contains(err.Error(), "rolled back") || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("restore: %v", err)
	}
	if fake.content != "current" || sum(t, in.cfg.DBPath) != original {
		t.Error("something changed")
	}
	if m, _ := filepath.Glob(filepath.Join(in.cfg.DataDir(), "courthive.dump*")); len(m) != 0 {
		t.Errorf("left %v", m)
	}
	assertNoLeftovers(t, in.cfg)
}

// A failure swapping in the app's database after the tournament database
// was restored puts the tournament database back from the safety copy; if
// that fails too, the error says the two no longer match.
func TestRestoreAppSwapFailurePutsTournamentBack(t *testing.T) {
	for _, putBackFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("put back fails=%v", putBackFails), func(t *testing.T) {
			in, fake, archive, original := tournamentArchive(t)
			if putBackFails {
				fake.replaceErr = func(call int) error {
					if call == 2 {
						return errors.New("psql failed: server closed the connection")
					}
					return nil
				}
			}
			swapHook = func(point string) error {
				if point == "before-commit" {
					return errors.New("injected failure")
				}
				return nil
			}
			t.Cleanup(func() { swapHook = nil })
			_, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
			if err == nil || !strings.Contains(err.Error(), "injected failure") {
				t.Fatalf("restore: %v", err)
			}
			if sum(t, in.cfg.DBPath) != original {
				t.Error("the app's database changed")
			}
			if errors.Is(err, ErrTournamentMismatch) != putBackFails {
				t.Errorf("ErrTournamentMismatch = %v: %v", errors.Is(err, ErrTournamentMismatch), err)
			}
			if putBackFails {
				if fake.content != "backed up" || !strings.Contains(err.Error(), "restore --tournaments-only") {
					t.Errorf("content %q, error %v", fake.content, err)
				}
				return
			}
			if fake.content != "current" || !strings.Contains(err.Error(), "the tournament database is as it was") {
				t.Errorf("content %q, error %v", fake.content, err)
			}
		})
	}
}

// An archive with tournament data restored where the deployment has no
// tournament settings: the app only, and the result says so.
func TestRestoreTournamentArchiveWithoutTournamentSettings(t *testing.T) {
	in, fake, archive, _ := tournamentArchive(t)
	in.cfg.Tournament = nil
	rr, err := Restore(context.Background(), in.cfg, archive, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if rr.Tournament != TournamentNotConfigured || rr.TournamentSafetyCopy != "" || fake.content != "current" {
		t.Errorf("result %+v, content %q", rr, fake.content)
	}
	in.open()
	var n int
	in.db.Get(&n, `SELECT COUNT(*) FROM clubs WHERE name = 'Added after the backup'`)
	if n != 0 {
		t.Error("the app's database was not restored")
	}

	// Later, with the settings: the tournament data alone.
	in.close()
	in.cfg.Tournament = fake
	appBefore := sum(t, in.cfg.DBPath)
	tr, err := RestoreTournaments(context.Background(), in.cfg, archive)
	if err != nil {
		t.Fatal(err)
	}
	if fake.content != "backed up" || tr.Manifest == nil || tr.SafetyCopy == "" || sum(t, in.cfg.DBPath) != appBefore {
		t.Errorf("tournaments-only: %+v content %q", tr, fake.content)
	}
	// And from the safety copy, a bare dump.
	if _, err := RestoreTournaments(context.Background(), in.cfg, tr.SafetyCopy); err != nil || fake.content != "current" {
		t.Errorf("from the safety copy: %v %q", err, fake.content)
	}
}

// An app-only archive on a deployment with a tournament database leaves the
// tournament database alone.
func TestRestoreAppOnlyArchiveLeavesTournamentDatabase(t *testing.T) {
	in := newInstance(t, jimdottennis.Migrations())
	in.seed()
	res, err := Create(context.Background(), in.db, in.cfg, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	in.close()
	fake := &fakeTournament{content: "current", others: 2} // the services may even be running
	in.cfg.Tournament = fake
	rr, err := Restore(context.Background(), in.cfg, res.Path, jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if rr.Tournament != TournamentNotInBackup || fake.replaces != 0 || fake.dumps != 0 || fake.content != "current" {
		t.Errorf("result %+v, fake %+v", rr, fake)
	}
	if _, err := RestoreTournaments(context.Background(), in.cfg, res.Path); err == nil || !strings.Contains(err.Error(), "holds no tournament data") {
		t.Errorf("tournaments-only from an app-only archive: %v", err)
	}
}

// The manifest's tournament entry is checked like the rest.
func TestTournamentManifestChecks(t *testing.T) {
	ok := Manifest{App: appName, Format: 2, Database: FileEntry{Path: databaseName, Size: 1},
		Tournaments: &TournamentEntry{Included: true, FileEntry: FileEntry{Path: tournamentName, Size: 10}, Format: TournamentDumpFormat}}
	if err := checkManifest(ok); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Manifest){
		"format 1":     func(m *Manifest) { m.Format = 1 },
		"wrong path":   func(m *Manifest) { m.Tournaments.Path = "tournaments/other.dump" },
		"plain format": func(m *Manifest) { m.Tournaments.Format = "plain" },
		"too big":      func(m *Manifest) { m.Tournaments.Size = maxTournamentBytes + 1 },
		"empty":        func(m *Manifest) { m.Tournaments.Size = 0 },
		"in files":     func(m *Manifest) { m.Tournaments = nil; m.Files = []FileEntry{{Path: tournamentName, Size: 1}} },
	} {
		m := ok
		tc := *ok.Tournaments
		m.Tournaments = &tc
		change(&m)
		if err := checkManifest(m); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := checkManifest(Manifest{App: appName, Format: 3, Database: FileEntry{Path: databaseName}}); !errors.Is(err, ErrNewerBackup) {
		t.Errorf("format 3: %v", err)
	}
}

// One pre-upgrade backup of each kind is kept.
func TestKeepLatestPreMigrationPerKind(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	for i := range 3 {
		stamp := base.Add(time.Duration(i)*time.Hour).Format(nameTimeLayout) + archiveExtension
		os.WriteFile(filepath.Join(dir, preMigrationPrefix+stamp), []byte("x"), 0o600)
		os.WriteFile(filepath.Join(dir, preUpgradePrefix+stamp), []byte("x"), 0o600)
	}
	removed, err := KeepLatestPreMigration(Config{Dir: dir})
	if err != nil || len(removed) != 4 {
		t.Fatalf("removed %v %v", removed, err)
	}
	list, _ := List(dir)
	if len(list) != 2 || list[0].PreUpgrade == list[1].PreUpgrade {
		t.Errorf("left %+v", list)
	}
}

func TestTournamentFromEnv(t *testing.T) {
	const secret = "s3cret-value-that-must-not-show"
	all := map[string]string{EnvPGHost: "postgres", EnvPGPort: "5432", EnvPGUser: "courthive", EnvPGDatabase: "courthive", EnvPGPassword: secret}
	for k := range all {
		t.Setenv(k, "")
	}
	if db, err := TournamentFromEnv(); db != nil || err != nil {
		t.Errorf("none set: %v %v", db, err)
	}
	for k, v := range all {
		t.Setenv(k, v)
	}
	db, err := TournamentFromEnv()
	if err != nil || db == nil {
		t.Fatalf("all set: %v", err)
	}
	if strings.Contains(db.Describe(), secret) {
		t.Error("Describe shows the password")
	}
	t.Setenv(EnvPGHost, "")
	if _, err := TournamentFromEnv(); err == nil || !strings.Contains(err.Error(), EnvPGHost) || strings.Contains(err.Error(), secret) {
		t.Errorf("host missing: %v", err)
	}
	t.Setenv(EnvPGHost, "postgres")
	t.Setenv(EnvPGPort, "five")
	if _, err := TournamentFromEnv(); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("bad port: %v", err)
	}
	t.Setenv(EnvPGPort, "5432")
	t.Setenv("DB_PATH", "/data/tennis.db")
	if c, err := FromEnv(); err != nil || c.Tournament == nil {
		t.Errorf("FromEnv: %+v %v", c, err)
	}
}

// fakeTool writes a shell script standing in for a Postgres tool: it
// records its arguments and whether it was given the password, prints what
// the test says, and fails when asked to (printing the password, as a
// careless tool might).
func fakeTool(t *testing.T, dir, name, body string) {
	t.Helper()
	script := "#!/bin/sh\n" +
		`printf '%s\n' "` + name + ` $*" >> "$TOOL_LOG"` + "\n" +
		`[ "$PGPASSWORD" = "$WANT_PASSWORD" ] && echo "` + name + ` has the password" >> "$TOOL_LOG"` + "\n" +
		body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The real PGTools: the password reaches the tools only through their
// environment, never their arguments, and never appears in an error.
func TestPGToolsKeepThePasswordOut(t *testing.T) {
	const secret = "pw-0123456789-abcdef"
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tools.log")
	t.Setenv("TOOL_LOG", logPath)
	t.Setenv("WANT_PASSWORD", secret)
	fakeTool(t, bin, "pg_dump", `for a; do case $a in --file=*) printf 'PGDMP' > "${a#--file=}";; esac; done
[ -n "$FAIL" ] && { echo "pg_dump: error: password $PGPASSWORD was refused" >&2; exit 1; }
exit 0`)
	fakeTool(t, bin, "pg_restore", `case "$*" in *--list*) printf '; Archive created\n;     Dumped from database version: 18.4\n;\n215; 1259 16385 TABLE public t courthive\n2615; 2615 2200 SCHEMA - public pg_database_owner\n';; esac
for a; do case $a in --use-list=*) cat "${a#--use-list=}" >> "$TOOL_LOG";; --file=/dev/null) ;; --file=*) echo "SELECT 1;" > "${a#--file=}";; esac; done
exit 0`)
	fakeTool(t, bin, "psql", `case "$*" in *pg_tables*) echo 7;; *to_regclass*) echo t;; *schema_migrations*) echo '64|064-support-cases.sql';; *pg_stat_activity*) echo 0;; esac
exit 0`)
	p := &PGTools{Host: "postgres", Port: "5432", User: "courthive", Database: "courthive", Password: secret, Bin: bin}
	ctx := context.Background()
	work := t.TempDir()
	dump := filepath.Join(work, "d.dump")
	if st, err := p.State(ctx); err != nil || st.Tables != 7 || st.Migrations != 64 || st.LatestMigration != "064-support-cases.sql" {
		t.Errorf("State: %+v %v", st, err)
	}
	if n, err := p.OtherSessions(ctx); err != nil || n != 0 {
		t.Errorf("OtherSessions: %d %v", n, err)
	}
	if err := p.Dump(ctx, dump); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dump); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("dump file: %v %v", info, err)
	}
	if info, err := p.Inspect(ctx, dump); err != nil || info.ServerVersion != "18.4" || info.Entries != 2 {
		t.Errorf("Inspect: %+v %v", info, err)
	}
	if err := p.Replace(ctx, dump, work); err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(work); len(left) != 1 {
		t.Errorf("Replace left files behind: %v", left)
	}
	t.Setenv("FAIL", "1")
	err := p.Dump(ctx, dump)
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "[password]") {
		t.Errorf("failing dump: %v", err)
	}
	if _, statErr := os.Stat(dump); statErr == nil {
		t.Error("a failed dump left its file")
	}

	log, _ := os.ReadFile(logPath)
	if strings.Contains(strings.ReplaceAll(string(log), " has the password", ""), secret) {
		t.Errorf("the password was in a command line:\n%s", log)
	}
	for _, tool := range []string{"pg_dump", "pg_restore", "psql"} {
		if !strings.Contains(string(log), tool+" has the password") {
			t.Errorf("%s did not get the password in its environment", tool)
		}
	}
	// The restore ran in one transaction, with the prelude first and the
	// dump's own entry for the public schema left out.
	if !strings.Contains(string(log), "psql -X -q --no-password -v ON_ERROR_STOP=1 --single-transaction -f") {
		t.Errorf("restore command:\n%s", log)
	}
	if !strings.Contains(string(log), "215; 1259 16385 TABLE public t courthive") || strings.Contains(string(log), "SCHEMA - public") {
		t.Errorf("the restore list should keep the table and leave out the public schema entry:\n%s", log)
	}

	// A missing tool is said plainly.
	p.Bin = t.TempDir()
	if _, err := p.State(ctx); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("missing tool: %v", err)
	}
}

func TestIsTournamentDump(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a"), []byte("PGDMP\x01"), 0o600)
	os.WriteFile(filepath.Join(dir, "b"), []byte("\x1f\x8b"), 0o600)
	if !IsTournamentDump(filepath.Join(dir, "a")) || IsTournamentDump(filepath.Join(dir, "b")) || IsTournamentDump(filepath.Join(dir, "c")) {
		t.Error("IsTournamentDump")
	}
}
