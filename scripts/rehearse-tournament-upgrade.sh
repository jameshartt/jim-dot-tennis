#!/usr/bin/env bash
# Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
#
# Rehearse a tournament upgrade on a throwaway copy of the data (WI-146).
#
#   scripts/rehearse-tournament-upgrade.sh <dump>
#
# <dump> is a dump of an existing tournament (CourtHive) database: plain SQL
# (pg_dump's default) or custom format (pg_dump -Fc, or tournaments/
# courthive.dump out of a jim-dot-tennis backup). The script:
#
#   1. restores it into a fresh Postgres 18 in its own compose project;
#   2. counts the rows of every table and notes the migrations applied;
#   3. starts the bundled tournament server against it and waits until its
#      migrations finish and it reports healthy (or fails);
#   4. counts again: migrations applied, tables dropped and added, row counts
#      before and after;
#   5. runs the identity service's password-hashing script in dry run (only
#      its count line is shown);
#   6. starts the identity service and checks it is healthy;
#   7. loads every tournament through the tournament server's API, counts
#      them by provider, and lists any that fail by tournament id.
#
# It never touches anything outside its own compose project (a random name,
# jdt-rehearsal-<hex>): its own network and volume, no published ports. It
# makes its own throwaway secrets and prints none of them, and prints no
# personal data (tournament ids, provider codes and counts only). Everything
# it made is removed when it ends, also on failure or Ctrl-C.
#
# Needs: bash, docker with compose v2, openssl. The images default to those
# deploy/docker-compose.yml ships; override with SERVER_IMAGE=... and
# HIVEID_IMAGE=... . MIGRATION_TIMEOUT (seconds, default 900) bounds the wait
# for the tournament server. KEEP=1 leaves the project running for a look
# (remove it afterwards with the command it prints).
#
# Exit status: 0 when the server came up healthy and every tournament loaded;
# 1 otherwise; 2 for a usage error.

set -euo pipefail

usage() {
	echo "Usage: $0 <dump of a tournament database (plain SQL or pg_dump custom format)>" >&2
	exit 2
}
[ $# -eq 1 ] || usage
dump=$1
[ -f "$dump" ] && [ -r "$dump" ] || { echo "rehearse: cannot read $dump" >&2; exit 2; }
command -v docker >/dev/null || { echo "rehearse: docker is not installed" >&2; exit 2; }
command -v openssl >/dev/null || { echo "rehearse: openssl is not installed" >&2; exit 2; }
docker compose version >/dev/null 2>&1 || { echo "rehearse: docker compose (v2) is not available" >&2; exit 2; }

repo=$(cd -- "$(dirname -- "$0")/.." && pwd)
default_image() { # repository name -> image with the default tag in deploy/docker-compose.yml
	grep -oE "ghcr\.io/jameshartt/$1:\\\$\{[A-Z_]+:-[^}]+\}" "$repo/deploy/docker-compose.yml" | head -1 |
		sed -E "s/\\\$\{[A-Z_]+:-([^}]+)\}/\1/"
}
SERVER_IMAGE=${SERVER_IMAGE:-$(default_image courthive-server)}
HIVEID_IMAGE=${HIVEID_IMAGE:-$(default_image courthive-hiveid)}
[ -n "$SERVER_IMAGE" ] && [ -n "$HIVEID_IMAGE" ] || { echo "rehearse: cannot read the image tags from deploy/docker-compose.yml; set SERVER_IMAGE and HIVEID_IMAGE" >&2; exit 2; }
MIGRATION_TIMEOUT=${MIGRATION_TIMEOUT:-900}

umask 077
work=$(mktemp -d)
project="jdt-rehearsal-$(openssl rand -hex 4)"
C=(docker compose -p "$project" --env-file "$work/.env" -f "$work/compose.yml")
cleanup() {
	if [ "${KEEP:-}" = 1 ]; then
		echo
		echo "KEEP=1: left running. Remove it with:"
		echo "  docker compose -p $project down -v --remove-orphans   # then delete $work"
		return
	fi
	"${C[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

say() { printf '%s\n' "$*"; }
section() { printf '\n== %s\n' "$*"; }

# --- Throwaway secrets (never printed) -------------------------------------
private_pem=$(openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 2>/dev/null)
public_pem=$(printf '%s\n' "$private_pem" | openssl pkey -pubout 2>/dev/null)
oneline() { printf '%s\n' "$1" | awk '{ printf "%s\\n", $0 }'; }
{
	printf "PG_PASSWORD='%s'\n" "$(openssl rand -hex 24)"
	printf "JWT_SECRET='%s'\n" "$(openssl rand -hex 32)"
	printf "SIGNER_TOKEN='%s'\n" "$(openssl rand -hex 32)"
	printf "JWT_KID='rehearsal-%s'\n" "$(openssl rand -hex 4)"
	printf "JWT_PRIVATE_KEY='%s'\n" "$(oneline "$private_pem")"
	printf "JWT_PUBLIC_KEY='%s'\n" "$(oneline "$public_pem")"
	printf "SERVER_IMAGE='%s'\nHIVEID_IMAGE='%s'\n" "$SERVER_IMAGE" "$HIVEID_IMAGE"
} >"$work/.env"
private_pem='' public_pem=''

cat >"$work/compose.yml" <<'YAML'
# The rehearsal's own throwaway stack: no published ports, one volume.
services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: courthive
      POSTGRES_PASSWORD: ${PG_PASSWORD}
      POSTGRES_DB: courthive
    volumes:
      - pg:/var/lib/postgresql
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U courthive']
      interval: 2s
      retries: 60
  redis:
    image: redis:7-alpine
  server:
    image: ${SERVER_IMAGE}
    restart: "no"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    environment:
      APP_NAME: Competition Factory Server
      APP_MODE: production
      APP_PORT: "8383"
      STORAGE_PROVIDER: postgres
      ARCHIVES_PATH: /tmp/archives
      JWT_SECRET: ${JWT_SECRET}
      JWT_ACCEPT_HS256: "false"
      JWT_PUBLIC_KEY: ${JWT_PUBLIC_KEY}
      JWT_KID: ${JWT_KID}
      HIVEID_BASE_URL: http://hiveid:3140
      HIVEID_SIGNER_TOKEN: ${SIGNER_TOKEN}
      CFS_CORS_ORIGINS: https://rehearsal.invalid
      TMX_URL: /tournaments/
      PERSONS_DISABLED: "true"
      PERSONS_BASE_URL: disabled
      DECLARATIONS_DISABLED: "true"
      AMS_DISABLED: "true"
      REDIS_URL: redis://redis:6379
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      PG_HOST: postgres
      PG_PORT: "5432"
      PG_USER: courthive
      PG_PASSWORD: ${PG_PASSWORD}
      PG_DATABASE: courthive
    healthcheck:
      interval: 10s
      timeout: 10s
      retries: 3
      start_period: 900s
      start_interval: 3s
  hiveid:
    image: ${HIVEID_IMAGE}
    restart: "no"
    environment:
      NODE_ENV: production
      PORT: "3140"
      PG_HOST: postgres
      PG_PORT: "5432"
      PG_USER: courthive
      PG_PASSWORD: ${PG_PASSWORD}
      PG_DATABASE: courthive
      JWT_SIGN_ES256: "true"
      JWT_ACCEPT_HS256: "false"
      JWT_PRIVATE_KEY: ${JWT_PRIVATE_KEY}
      JWT_PUBLIC_KEY: ${JWT_PUBLIC_KEY}
      JWT_KID: ${JWT_KID}
      HIVEID_SIGNER_TOKEN: ${SIGNER_TOKEN}
      TMX_URL: /tournaments/
      PUBLIC_URL: /public/
    healthcheck:
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 120s
      start_interval: 2s
volumes:
  pg:
YAML

psql_q() { "${C[@]}" exec -T postgres psql -X -A -t -q -v ON_ERROR_STOP=1 -U courthive -d courthive -c "$1"; }
health() { # service -> healthy|unhealthy|starting|exited|...
	local id
	id=$("${C[@]}" ps -a -q "$1")
	[ -n "$id" ] || { echo missing; return; }
	docker inspect -f '{{if eq .State.Status "running"}}{{if .State.Health}}{{.State.Health.Status}}{{else}}running{{end}}{{else}}{{.State.Status}}{{end}}' "$id"
}
wait_for() { # service seconds -> final state
	local s=$1 limit=$2 st
	for _ in $(seq 1 "$limit"); do
		st=$(health "$s")
		case $st in healthy | exited | dead | unhealthy) break ;; esac
		sleep 1
	done
	echo "$st"
}
# Row counts of every table, "schema.table<TAB>count", sorted.
count_tables() {
	psql_q "SELECT n.nspname || '.' || c.relname || E'\t' ||
	  (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I', n.nspname, c.relname), false, true, '')))[1]::text
	FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition AND n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_%'
	ORDER BY 1" | sort
}
migrations() {
	if [ "$(psql_q "SELECT to_regclass('public.schema_migrations') IS NOT NULL")" = t ]; then
		psql_q "SELECT count(*) || ' applied, ' || coalesce(min(name), '-') || ' to ' || coalesce(max(name), '-') FROM schema_migrations"
	else
		echo "none recorded (no schema_migrations table)"
	fi
}

failures=0
section "Rehearsal $project"
say "Tournament server image: $SERVER_IMAGE"
say "Identity service image:  $HIVEID_IMAGE"

section "1. Restore the dump into a fresh Postgres 18"
"${C[@]}" up -d postgres >/dev/null 2>&1
[ "$(wait_for postgres 120)" = healthy ] || { say "Postgres did not start"; exit 1; }
say "Postgres: $(psql_q 'SHOW server_version')"
"${C[@]}" cp "$dump" postgres:/tmp/in.dump >/dev/null 2>&1
# Errors are shown without anything in quotes (which could be data).
clean_err() { grep -E 'ERROR|FATAL|error:' | head -3 | sed -E "s/\"[^\"]*\"/\"...\"/g; s/'[^']*'/'...'/g" | cut -c1-240; }
if [ "$(head -c 5 "$dump")" = PGDMP ]; then
	say "Format: pg_dump custom"
	if ! out=$("${C[@]}" exec -T postgres pg_restore --no-owner --no-privileges --exit-on-error -U courthive -d courthive /tmp/in.dump 2>&1); then
		say "The dump could not be restored:"; printf '%s\n' "$out" | clean_err; exit 1
	fi
else
	say "Format: plain SQL"
	if ! out=$("${C[@]}" exec -T postgres psql -X -q -v ON_ERROR_STOP=1 -U courthive -d courthive -f /tmp/in.dump 2>&1 >/dev/null); then
		say "The dump could not be restored:"; printf '%s\n' "$out" | clean_err; exit 1
	fi
fi
"${C[@]}" exec -T postgres rm -f /tmp/in.dump
count_tables >"$work/before.tsv"
say "Restored: $(wc -l <"$work/before.tsv" | tr -d ' ') tables, $(awk -F'\t' '{s+=$2} END {print s+0}' "$work/before.tsv") rows."
say "Tournament server migrations before: $(migrations)"
psql_q "SELECT name FROM schema_migrations ORDER BY name" >"$work/migrations-before.txt" 2>/dev/null || : >"$work/migrations-before.txt"

section "2. Start the bundled tournament server and wait for its migrations"
start=$(date +%s)
"${C[@]}" up -d redis server >/dev/null 2>&1
server_state=$(wait_for server "$MIGRATION_TIMEOUT")
say "Tournament server: $server_state after $(($(date +%s) - start))s"
"${C[@]}" logs --no-color server >"$work/server.log" 2>&1 || true
applied=$(grep -oE 'Applied: [0-9]{3}-[a-z0-9-]+\.sql' "$work/server.log" | sed 's/Applied: //' || true)
if [ -n "$applied" ]; then
	say "Migrations applied now: $(printf '%s\n' "$applied" | wc -l | tr -d ' '), $(printf '%s\n' "$applied" | head -1) to $(printf '%s\n' "$applied" | tail -1)"
else
	say "Migrations applied now: none"
fi
if grep -qE 'Migration [^ ]+ failed|Migration failed' "$work/server.log"; then
	say "A MIGRATION FAILED:"
	grep -E 'Migration [^ ]+ failed|Migration failed' "$work/server.log" | sed -E "s/\"[^\"]*\"/\"...\"/g" | cut -c1-240 | head -3
	failures=$((failures + 1))
fi
if [ "$server_state" != healthy ]; then
	say "The tournament server did not become healthy. Last log lines (secrets are not logged by it):"
	tail -5 "$work/server.log" | sed -E "s/\"[^\"]*\"/\"...\"/g" | cut -c1-240
	failures=$((failures + 1))
fi
say "Tournament server migrations after: $(migrations)"
version=$("${C[@]}" exec -T server node -e 'fetch("http://127.0.0.1:8383/factory/version").then(r=>r.json()).then(j=>console.log(j.version||"?"),()=>console.log("?"))' 2>/dev/null || echo '?')
say "Tournament engine now: $version"

section "3. Tables and rows before and after"
count_tables >"$work/after.tsv"
join -t "$(printf '\t')" -a1 -a2 -e '-' -o '0,1.2,2.2' "$work/before.tsv" "$work/after.tsv" >"$work/both.tsv"
printf '%-48s %10s %10s\n' TABLE BEFORE AFTER
while IFS="$(printf '\t')" read -r table b a; do
	note=''
	[ "$a" = '-' ] && note='  dropped'
	[ "$b" = '-' ] && note='  new'
	[ "$b" != '-' ] && [ "$a" != '-' ] && [ "$b" != "$a" ] && note='  changed'
	printf '%-48s %10s %10s%s\n' "$table" "$b" "$a" "$note"
done <"$work/both.tsv"
dropped=$(awk -F'\t' '$3 == "-" {print $1}' "$work/both.tsv" | tr '\n' ' ')
say "Tables dropped: ${dropped:-none}"

section "4. Password hashing step (dry run)"
if [ "$server_state" = healthy ]; then
	"${C[@]}" run --rm --no-deps -T hiveid node scripts/hash-plaintext-passwords.mjs 2>&1 | grep -E '^users: ' ||
		{ say "The hashing script did not run"; failures=$((failures + 1)); }
	say "(In the deployment the courthive-hash-passwords step runs it with --apply before the identity service starts.)"
else
	say "Skipped: the tournament server is not healthy."
fi

section "5. Identity service"
if [ "$server_state" = healthy ]; then
	"${C[@]}" up -d --no-deps hiveid >/dev/null 2>&1
	hiveid_state=$(wait_for hiveid 180)
	say "Identity service: $hiveid_state"
	[ "$hiveid_state" = healthy ] || failures=$((failures + 1))
else
	say "Skipped: the tournament server is not healthy."
fi

section "6. Every tournament through the tournament server's API"
if [ "$server_state" = healthy ]; then
	psql_q "SELECT tournament_id FROM tournaments ORDER BY tournament_id" >"$work/ids.txt"
	say "Tournaments in the database: $(wc -l <"$work/ids.txt" | tr -d ' ')"
	psql_q "SELECT provider || E'\t' || n FROM (SELECT coalesce(p.organisation_abbreviation, '(no provider row: ' || coalesce(t.provider_id, 'none') || ')') AS provider, count(*) AS n
		FROM tournaments t LEFT JOIN providers p ON p.provider_id = t.provider_id GROUP BY 1) x ORDER BY 1" | sed 's/^/  by provider (database): /; s/\t/: /'
	# A throwaway administrator token signed with the rehearsal's own key,
	# made and used inside the server's container on the project network.
	"${C[@]}" cp "$work/ids.txt" server:/tmp/ids.txt >/dev/null 2>&1
	"${C[@]}" exec -T -e JWT_PRIVATE_KEY="$(sed -n "s/^JWT_PRIVATE_KEY='\(.*\)'$/\1/p" "$work/.env")" server node -e '
const crypto = require("crypto");
const fs = require("fs");
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
const now = Math.floor(Date.now() / 1000);
const head = b64({ alg: "ES256", typ: "JWT", kid: process.env.JWT_KID });
const body = b64({ email: "rehearsal@example.invalid", roles: ["superadmin", "client"], aud: "admin", iat: now, exp: now + 3600 });
const key = crypto.createPrivateKey(process.env.JWT_PRIVATE_KEY.replace(/\\n/g, "\n"));
const sig = crypto.sign("sha256", Buffer.from(head + "." + body), { key, dsaEncoding: "ieee-p1363" }).toString("base64url");
const token = `${head}.${body}.${sig}`;
const ids = fs.readFileSync("/tmp/ids.txt", "utf8").split("\n").filter(Boolean);
(async () => {
  const byProvider = {};
  const failed = [];
  let events = 0, draws = 0, matchUps = 0, participants = 0;
  for (const id of ids) {
    try {
      const r = await fetch("http://127.0.0.1:8383/factory/fetch", { method: "POST",
        headers: { "content-type": "application/json", authorization: "Bearer " + token },
        body: JSON.stringify({ tournamentId: id }) });
      const j = await r.json().catch(() => ({}));
      const t = j.tournamentRecords && j.tournamentRecords[id];
      if (!r.ok || !t) { failed.push(`${id}: HTTP ${r.status} ${String(j.error || j.message || "no record").slice(0, 80)}`); continue; }
      const p = (t.parentOrganisation && t.parentOrganisation.organisationId) || "(none)";
      byProvider[p] = (byProvider[p] || 0) + 1;
      for (const e of t.events || []) { events++; for (const d of e.drawDefinitions || []) { draws++; for (const s of d.structures || []) matchUps += (s.matchUps || []).length; } }
      participants += (t.participants || []).length;
    } catch (e) { failed.push(`${id}: ${e.message}`); }
  }
  console.log(`Loaded through the API: ${ids.length - failed.length} of ${ids.length}`);
  for (const [p, n] of Object.entries(byProvider)) console.log(`  by provider id (API): ${p}: ${n}`);
  console.log(`  in them: ${events} events, ${draws} draws, ${matchUps} matchUps (top-level structures), ${participants} participants`);
  for (const f of failed) console.log(`  FAILED ${f}`);
  process.exit(failed.length ? 3 : 0);
})();' || failures=$((failures + 1))
	"${C[@]}" exec -T server rm -f /tmp/ids.txt
else
	say "Skipped: the tournament server is not healthy."
fi

section "Result"
if [ "$failures" -eq 0 ]; then
	say "PASS: the tournament server upgraded the copy, came up healthy, and every tournament loads."
	exit 0
fi
say "FAIL: $failures problem(s) above. Do not upgrade the real instance until they are understood."
exit 1
