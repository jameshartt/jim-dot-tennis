// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build integration

package backup

// Runs PGTools against a real, throwaway Postgres (never a real one: it
// drops everything in the database). Needs the Postgres client tools and
// TEST_PG_HOST, TEST_PG_PORT, TEST_PG_USER, TEST_PG_DATABASE and
// TEST_PG_PASSWORD, for example:
//
//	docker network create pgtest
//	docker run -d --rm --name pgtest-db --network pgtest -e POSTGRES_PASSWORD=x postgres:18-alpine
//	docker run --rm --network pgtest -v "$PWD":/app -w /app -e CGO_ENABLED=0 \
//	  -e TEST_PG_HOST=pgtest-db -e TEST_PG_PORT=5432 -e TEST_PG_USER=postgres \
//	  -e TEST_PG_DATABASE=postgres -e TEST_PG_PASSWORD=x golang:1.27-alpine \
//	  sh -c 'apk add -q postgresql18-client && go test -tags integration -run PG ./internal/backup/'

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func realPG(t *testing.T) *PGTools {
	t.Helper()
	p := &PGTools{Host: os.Getenv("TEST_PG_HOST"), Port: os.Getenv("TEST_PG_PORT"), User: os.Getenv("TEST_PG_USER"),
		Database: os.Getenv("TEST_PG_DATABASE"), Password: os.Getenv("TEST_PG_PASSWORD")}
	if p.Host == "" {
		t.Skip("TEST_PG_HOST is not set")
	}
	return p
}

func mustQuery(t *testing.T, p *PGTools, sql string) string {
	t.Helper()
	out, err := p.query(context.Background(), sql)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return out
}

func TestPGDumpReplaceRoundTrip(t *testing.T) {
	p := realPG(t)
	ctx := context.Background()
	work := t.TempDir()
	mustQuery(t, p, `DROP SCHEMA IF EXISTS public CASCADE; DROP SCHEMA IF EXISTS extra CASCADE; CREATE SCHEMA public;
		CREATE TABLE schema_migrations (name TEXT PRIMARY KEY); INSERT INTO schema_migrations VALUES ('001-a.sql'), ('064-support-cases.sql');
		CREATE TABLE tournaments (id TEXT PRIMARY KEY, record JSONB NOT NULL);
		INSERT INTO tournaments SELECT 't' || g, jsonb_build_object('draw', g) FROM generate_series(1, 500) g;
		CREATE SCHEMA extra; CREATE TABLE extra.kept (n INT); INSERT INTO extra.kept VALUES (1)`)
	st, err := p.State(ctx)
	if err != nil || st.Tables != 3 || st.Migrations != 2 || st.LatestMigration != "064-support-cases.sql" {
		t.Fatalf("State: %+v %v", st, err)
	}
	dump := filepath.Join(work, "a.dump")
	if err := p.Dump(ctx, dump); err != nil {
		t.Fatal(err)
	}
	info, err := p.Inspect(ctx, dump)
	if err != nil || !strings.HasPrefix(info.ServerVersion, "18") || info.Entries == 0 {
		t.Fatalf("Inspect: %+v %v", info, err)
	}

	// Change everything: rows, a new table (as a newer server would add),
	// a dropped schema.
	mustQuery(t, p, `DELETE FROM tournaments WHERE id <> 't1'; UPDATE tournaments SET record = '{}';
		CREATE TABLE added_later (n INT); DROP SCHEMA extra CASCADE`)

	if err := p.Replace(ctx, dump, work); err != nil {
		t.Fatal(err)
	}
	if got := mustQuery(t, p, `SELECT count(*) || ' ' || sum((record->>'draw')::int) FROM tournaments`); got != "500 125250" {
		t.Errorf("tournaments after restore: %s", got)
	}
	if got := mustQuery(t, p, `SELECT to_regclass('public.added_later') IS NULL AND to_regclass('extra.kept') IS NOT NULL`); got != "t" {
		t.Error("the restore left a table made after the dump, or lost a schema")
	}

	// A restore that cannot finish (a lock is held; the lock timeout ends
	// it) leaves the database exactly as it was.
	mustQuery(t, p, `CREATE TABLE added_later (n INT); INSERT INTO added_later VALUES (42)`)
	holder := exec.Command("psql", "-X", "-q", "-v", "ON_ERROR_STOP=1")
	holder.Env = p.env()
	holder.Stdin = strings.NewReader(`BEGIN; LOCK TABLE tournaments IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(40);`)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Process.Kill(); holder.Wait() })
	time.Sleep(time.Second)
	start := time.Now()
	err = p.Replace(ctx, dump, work)
	if err == nil {
		t.Fatal("a restore against a held lock succeeded")
	}
	t.Logf("failed as expected after %s: %v", time.Since(start).Round(time.Second), err)
	holder.Process.Kill()
	holder.Wait()
	if got := mustQuery(t, p, `SELECT (SELECT n FROM added_later) || ' ' || (SELECT count(*) FROM tournaments)`); got != "42 500" {
		t.Errorf("after a failed restore: %s", got)
	}
	if n, err := p.OtherSessions(ctx); err != nil || n != 0 {
		t.Errorf("OtherSessions: %d %v", n, err)
	}
}
