// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

// The tournament database (WI-146). When the bundled tournament stack is part
// of the deployment (the COURTHIVE_PG_* settings are present), every backup
// also holds a dump of its Postgres database, and restore puts it back.
//
// The dump is made with pg_dump in its custom format, without compression
// (the archive is gzip-compressed as a whole). The custom format is chosen
// over plain SQL because pg_restore can read a custom dump right through
// without a database (pg_restore --list, then pg_restore --file=/dev/null):
// that is how a dump is checked after it is made and again before a restore
// uses it. A plain SQL file can only be checked by running it.
//
// A restore replaces the database's whole content in ONE transaction (see
// Replace): every schema is dropped, then the dump's SQL is run, then the
// transaction commits. If anything fails before the commit, Postgres rolls it
// all back and the database is exactly as it was. pg_restore --clean was not
// used: it drops only the objects the dump itself holds, so tables made since
// (by a newer tournament server, say) would survive beside the restored ones.
//
// The password is never put in a command's arguments. The Postgres tools read
// it from PGPASSWORD in the child's environment only, and any error text the
// tools print is scrubbed of it before it is shown or logged.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Environment variables for the tournament database. The deployment sets
// them only while the tournament stack is on.
const (
	EnvPGHost     = "COURTHIVE_PG_HOST"
	EnvPGPort     = "COURTHIVE_PG_PORT"
	EnvPGUser     = "COURTHIVE_PG_USER"
	EnvPGDatabase = "COURTHIVE_PG_DATABASE"
	EnvPGPassword = "COURTHIVE_PG_PASSWORD"
	// EnvServerImage names the tournament server image the deployment runs
	// (recorded in the manifest, and compared by "tournaments pre-upgrade").
	EnvServerImage = "COURTHIVE_SERVER_IMAGE"
)

// TournamentDumpFormat is the manifest's name for the dump's format.
const TournamentDumpFormat = "postgres-custom"

// TournamentDatabase is the tournament stack's Postgres database. PGTools is
// the real one; tests use a fake.
type TournamentDatabase interface {
	// Describe names the database for messages (host, port and name; never
	// the password).
	Describe() string
	// Dump writes a dump of the whole database to path.
	Dump(ctx context.Context, path string) error
	// Inspect reads the dump at path right through and says what it holds.
	// An error means it is not a whole, readable dump.
	Inspect(ctx context.Context, path string) (DumpInfo, error)
	// State reads the live database's tables and tournament-server
	// migrations.
	State(ctx context.Context) (DatabaseState, error)
	// OtherSessions counts the connections to the database other than this
	// program's own (the tournament server, the identity service, ...).
	OtherSessions(ctx context.Context) (int, error)
	// Replace makes the database's whole content that of the dump at path,
	// in one transaction: on any error the database is as it was.
	Replace(ctx context.Context, path, workDir string) error
}

// DumpInfo is what a dump says about itself.
type DumpInfo struct {
	ServerVersion string // the Postgres version it was dumped from
	Entries       int    // objects in its table of contents
}

// DatabaseState is the live tournament database's state.
type DatabaseState struct {
	Tables          int    // user tables
	Migrations      int    // tournament-server migrations applied
	LatestMigration string // the newest of them ("" if none)
}

// TournamentFromEnv returns the tournament database named by the
// COURTHIVE_PG_* settings, or nil when they are absent (the tournament stack
// is not part of this deployment). Some set and some missing is an error
// that names the missing ones (it never quotes the password).
func TournamentFromEnv() (TournamentDatabase, error) {
	vals := map[string]string{}
	var missing []string
	set := 0
	for _, k := range []string{EnvPGHost, EnvPGPort, EnvPGUser, EnvPGDatabase, EnvPGPassword} {
		v := strings.TrimSpace(os.Getenv(k))
		if k == EnvPGPassword {
			v = os.Getenv(k)
		}
		vals[k] = v
		if v == "" {
			missing = append(missing, k)
		} else {
			set++
		}
	}
	if set == 0 {
		return nil, nil
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("the tournament database settings are incomplete: %s %s empty. Set all of %s, %s, %s, %s and %s, "+
			"or none of them if this deployment has no tournament stack", strings.Join(missing, ", "), isAre(len(missing)),
			EnvPGHost, EnvPGPort, EnvPGUser, EnvPGDatabase, EnvPGPassword)
	}
	if n, err := strconv.Atoi(vals[EnvPGPort]); err != nil || n < 1 || n > 65535 {
		return nil, fmt.Errorf("%s is %q: set it to the tournament database's port, normally 5432", EnvPGPort, vals[EnvPGPort])
	}
	return &PGTools{Host: vals[EnvPGHost], Port: vals[EnvPGPort], User: vals[EnvPGUser], Database: vals[EnvPGDatabase], Password: vals[EnvPGPassword]}, nil
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// PGTools reaches the tournament database with the Postgres client tools
// (pg_dump, pg_restore, psql), which the app's image carries.
type PGTools struct {
	Host, Port, User, Database string
	Password                   string // never printed
	// Bin is the folder holding the tools ("" means look them up in PATH).
	Bin string
}

// Describe implements TournamentDatabase.
func (p *PGTools) Describe() string {
	return fmt.Sprintf("the tournament database %q on %s:%s", p.Database, p.Host, p.Port)
}

// toolTimeout bounds one call of a tool that only reads a little.
const toolTimeout = 2 * time.Minute

func (p *PGTools) tool(name string) string {
	if p.Bin != "" {
		return filepath.Join(p.Bin, name)
	}
	return name
}

// env is the tools' environment: this process's, without any PG* variable of
// its own, plus the connection settings. The password is only ever here.
func (p *PGTools) env() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PG") {
			continue
		}
		out = append(out, kv)
	}
	return append(out,
		"PGHOST="+p.Host, "PGPORT="+p.Port, "PGUSER="+p.User, "PGDATABASE="+p.Database,
		"PGPASSWORD="+p.Password, "PGCONNECT_TIMEOUT=10", "PGAPPNAME=jim-dot-tennis-backup",
		"PGSSLMODE=prefer", "PGOPTIONS=", "PGTZ=UTC")
}

// run runs a tool and returns its standard output. A failure says which tool
// and what it printed, with the password removed.
func (p *PGTools) run(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, p.tool(name), args...)
	cmd.Env = p.env()
	cmd.Stdin = stdin
	var stdout bytes.Buffer
	stderr := &limitedBuffer{max: 4096}
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, p.toolError(name, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (p *PGTools) toolError(name string, err error, stderr string) error {
	var notFound *exec.Error
	if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s is not installed in this image (the app's image carries the Postgres client tools; use the published image)", name)
	}
	msg := strings.TrimSpace(p.scrub(stderr))
	if msg == "" {
		msg = err.Error()
	}
	msg = strings.Join(strings.Fields(msg), " ")
	return fmt.Errorf("%s failed: %s", name, truncateText(msg, 600))
}

// scrub removes the password from s.
func (p *PGTools) scrub(s string) string {
	if p.Password == "" {
		return s
	}
	return strings.ReplaceAll(s, p.Password, "[password]")
}

// Dump implements TournamentDatabase: pg_dump in the custom format. pg_dump
// reads the whole database in one repeatable-read transaction, so the dump is
// consistent while the tournament server keeps writing.
func (p *PGTools) Dump(ctx context.Context, path string) error {
	_, err := p.run(ctx, nil, "pg_dump", "--format=custom", "--compress=0", "--no-password", "--file="+path)
	if err != nil {
		os.Remove(path)
		return err
	}
	return os.Chmod(path, 0o600)
}

var dumpedFromRE = regexp.MustCompile(`(?m)^;\s+Dumped from database version: (\S+)`)

// Inspect implements TournamentDatabase: the table of contents, then a read
// of every entry (data included) into nothing.
func (p *PGTools) Inspect(ctx context.Context, path string) (DumpInfo, error) {
	var info DumpInfo
	list, err := p.list(ctx, path)
	if err != nil {
		return info, err
	}
	if m := dumpedFromRE.FindSubmatch(list); m != nil {
		info.ServerVersion = string(m[1])
	}
	for _, line := range strings.Split(string(list), "\n") {
		if line != "" && !strings.HasPrefix(line, ";") {
			info.Entries++
		}
	}
	if _, err := p.run(ctx, nil, "pg_restore", "--file=/dev/null", path); err != nil {
		return info, err
	}
	return info, nil
}

func (p *PGTools) list(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	return p.run(ctx, nil, "pg_restore", "--list", path)
}

// query runs one SQL statement with psql and returns its unaligned output.
func (p *PGTools) query(ctx context.Context, sql string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	out, err := p.run(ctx, nil, "psql", "-X", "-A", "-t", "-q", "--no-password", "-v", "ON_ERROR_STOP=1", "-c", sql)
	return strings.TrimSpace(string(out)), err
}

// State implements TournamentDatabase.
func (p *PGTools) State(ctx context.Context) (DatabaseState, error) {
	var st DatabaseState
	out, err := p.query(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog', 'information_schema')`)
	if err != nil {
		return st, err
	}
	if st.Tables, err = strconv.Atoi(out); err != nil {
		return st, fmt.Errorf("psql: unexpected answer %q", truncateText(out, 40))
	}
	has, err := p.query(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`)
	if err != nil || has != "t" {
		return st, err
	}
	out, err = p.query(ctx, `SELECT count(*) || '|' || coalesce(max(name), '') FROM public.schema_migrations`)
	if err != nil {
		return st, err
	}
	n, latest, _ := strings.Cut(out, "|")
	st.Migrations, _ = strconv.Atoi(n)
	st.LatestMigration = latest
	return st, nil
}

// OtherSessions implements TournamentDatabase.
func (p *PGTools) OtherSessions(ctx context.Context) (int, error) {
	out, err := p.query(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND backend_type = 'client backend'`)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("psql: unexpected answer %q", truncateText(out, 40))
	}
	return n, nil
}

// replacePrelude runs first in the restore's transaction: it gives up rather
// than wait long for a lock (a connection left open), then drops every
// schema in the database with everything in it, and makes an empty public
// schema for the dump to fill.
const replacePrelude = `SET lock_timeout = '30s';
DO $$
DECLARE s text;
BEGIN
  FOR s IN SELECT nspname FROM pg_namespace
    WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
      AND nspname NOT LIKE 'pg\_temp\_%' AND nspname NOT LIKE 'pg\_toast\_temp\_%'
  LOOP
    EXECUTE format('DROP SCHEMA %I CASCADE', s);
  END LOOP;
END $$;
CREATE SCHEMA public;
`

// publicSchemaEntry matches the dump's own entry for the public schema
// (made by the prelude instead, since the dump may or may not have one).
var publicSchemaEntry = regexp.MustCompile(`^\d+; \d+ \d+ SCHEMA - public `)

// Replace implements TournamentDatabase. The dump is first turned into SQL
// in workDir (so a dump that cannot be read stops the restore before the
// database is touched), then psql runs the prelude and that SQL in a single
// transaction with ON_ERROR_STOP: the first error rolls everything back.
func (p *PGTools) Replace(ctx context.Context, path, workDir string) error {
	list, err := p.list(ctx, path)
	if err != nil {
		return err
	}
	var keep []string
	for _, line := range strings.Split(string(list), "\n") {
		if publicSchemaEntry.MatchString(line) {
			continue
		}
		keep = append(keep, line)
	}
	listPath := filepath.Join(workDir, "restore.list")
	preludePath := filepath.Join(workDir, "restore-prelude.sql")
	sqlPath := filepath.Join(workDir, "restore.sql")
	defer os.Remove(listPath)
	defer os.Remove(preludePath)
	defer os.Remove(sqlPath)
	if err := os.WriteFile(listPath, []byte(strings.Join(keep, "\n")), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(preludePath, []byte(replacePrelude), 0o600); err != nil {
		return err
	}
	if _, err := p.run(ctx, nil, "pg_restore", "--no-owner", "--no-privileges", "--use-list="+listPath, "--file="+sqlPath, path); err != nil {
		return err
	}
	_, err = p.run(ctx, nil, "psql", "-X", "-q", "--no-password", "-v", "ON_ERROR_STOP=1", "--single-transaction",
		"-f", preludePath, "-f", sqlPath)
	return err
}

// limitedBuffer keeps the first max bytes written to it.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string { return l.buf.String() }

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// IsTournamentDump reports whether the file at path starts like a pg_dump
// custom-format dump (its "PGDMP" signature).
func IsTournamentDump(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 5)
	if _, err := io.ReadFull(bufio.NewReader(f), head); err != nil {
		return false
	}
	return string(head) == "PGDMP"
}
