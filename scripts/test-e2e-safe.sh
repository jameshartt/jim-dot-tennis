#!/usr/bin/env bash
# Run an E2E make target without clobbering the local dev database.
#
# Why: tests/e2e/fixtures/seed.sh writes test fixtures directly into the
# shared tennis-data volume at /data/tennis.db. If that volume holds a
# pulled prod DB, `make test-e2e*` will mix seed rows in via
# INSERT OR REPLACE, and `make test-e2e-clean` will wipe the volume
# entirely with `down -v`.
#
# How: snapshot the live DB before tests, restore it on exit (always,
# even on failure / Ctrl-C). With --keep, leave the test-seeded state in
# place so you can re-run a subset of tests against it.
#
# Usage:
#   scripts/test-e2e-safe.sh [OPTIONS] [TARGET] [MAKE_VARS...]
#
# Options:
#   -k, --keep            Don't restore after the run. Snapshot stays at
#                         exported-backups/pre-e2e-current.db so a later
#                         invocation either reuses the snapshot or restores it.
#   -r, --restore-only    Skip the test run. Just restore from the saved
#                         snapshot, then delete it.
#   -h, --help            Show this help.
#
# Positional:
#   TARGET     make target (default: test-e2e). Common: test-e2e,
#              test-e2e-failed, test-e2e-grep, test-e2e-multiclub.
#   MAKE_VARS  passed through to make. e.g. WORKERS=4 FILTER=login.
#
# Examples:
#   scripts/test-e2e-safe.sh                              # full suite, restore after
#   scripts/test-e2e-safe.sh WORKERS=4                    # full suite, 4 workers
#   scripts/test-e2e-safe.sh --keep                       # full suite, keep test state
#   scripts/test-e2e-safe.sh --keep test-e2e-failed       # iterate on failures
#   scripts/test-e2e-safe.sh --keep test-e2e-grep FILTER=login
#   scripts/test-e2e-safe.sh --restore-only               # restore prod DB now
#
# Snapshot lifecycle:
#   First run takes a snapshot to exported-backups/pre-e2e-current.db.
#   While that file exists, subsequent runs reuse it instead of taking a
#   fresh one (so --keep iterations don't snapshot test state on top of test
#   state). Default-mode restore deletes the snapshot when it's done; so
#   does --restore-only. To force a fresh snapshot, just delete the file.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

VOLUME="jim-dot-tennis-data"
APP_CONTAINER="jim-dot-tennis"
BACKUP_DIR="./exported-backups"
SNAPSHOT_NAME="pre-e2e-current.db"
SNAPSHOT_PATH="${BACKUP_DIR}/${SNAPSHOT_NAME}"

# Ownership: docker containers run as root by default, so files they write
# end up uid 0 on the host volume. The app container runs as `appuser`
# (uid 1000 from Dockerfile), so a root-owned tennis.db opens as
# read-only — sqlite then fails every write with "attempt to write a
# readonly database". Chown after each operation to the right uid:gid.
APP_UID=1000
APP_GID=1000
HOST_UID="$(id -u)"
HOST_GID="$(id -g)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${GREEN}==>${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}==>${NC} %s\n" "$*" >&2; }
err()  { printf "${RED}==>${NC} %s\n" "$*" >&2; }

usage() {
  # Print the leading comment block as help.
  sed -n '/^# Run an E2E/,/^$/p' "$0" | sed -e 's/^# \{0,1\}//' -e '/^$/d'
  exit 0
}

KEEP=0
RESTORE_ONLY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -k|--keep)         KEEP=1; shift ;;
    -r|--restore-only) RESTORE_ONLY=1; shift ;;
    -h|--help)         usage ;;
    --)                shift; break ;;
    -*)                err "Unknown option: $1"; exit 1 ;;
    *)                 break ;;
  esac
done

# First positional that doesn't contain '=' is the make target; the rest
# are forwarded as make variables.
TARGET="test-e2e"
if [[ $# -gt 0 && "$1" != *=* ]]; then
  TARGET="$1"
  shift
fi
MAKE_ARGS=("$@")

if ! docker volume inspect "$VOLUME" >/dev/null 2>&1; then
  err "Volume '$VOLUME' not found. Nothing to back up — run 'make ${TARGET}' directly for a fresh setup."
  exit 1
fi

mkdir -p "$BACKUP_DIR"

snapshot_db() {
  log "Snapshotting DB to ${SNAPSHOT_PATH}"
  docker run --rm \
    -v "${VOLUME}:/data:ro" \
    -v "${REPO_ROOT}/${BACKUP_DIR}:/backup" \
    alpine:latest sh -c "
      apk add --no-cache sqlite >/dev/null &&
      sqlite3 /data/tennis.db \".backup /backup/${SNAPSHOT_NAME}\" &&
      chown ${HOST_UID}:${HOST_GID} /backup/${SNAPSHOT_NAME}
    "
  if [ ! -s "${SNAPSHOT_PATH}" ]; then
    err "Snapshot is missing or empty at ${SNAPSHOT_PATH}. Aborting."
    exit 1
  fi
  log "Snapshot OK: $(du -h "${SNAPSHOT_PATH}" | cut -f1)"
}

restore_db() {
  if [ ! -s "${SNAPSHOT_PATH}" ]; then
    err "No snapshot found at ${SNAPSHOT_PATH}. Cannot restore."
    return 1
  fi
  log "Restoring DB from ${SNAPSHOT_PATH}"
  docker compose stop app >/dev/null 2>&1 || warn "app container was not running"
  docker run --rm \
    -v "${VOLUME}:/data" \
    -v "${REPO_ROOT}/${BACKUP_DIR}:/backup:ro" \
    alpine:latest sh -c "
      cp /backup/${SNAPSHOT_NAME} /data/tennis.db &&
      chown ${APP_UID}:${APP_GID} /data/tennis.db &&
      rm -f /data/tennis.db-wal /data/tennis.db-shm
    "
  log "Starting app..."
  docker compose up -d app >/dev/null

  log "Waiting for app health..."
  local timeout=60
  while [ $timeout -gt 0 ]; do
    if [ "$(docker inspect -f '{{.State.Health.Status}}' "$APP_CONTAINER" 2>/dev/null)" = "healthy" ]; then
      log "App healthy."
      rm -f "${SNAPSHOT_PATH}"
      log "Snapshot consumed (deleted ${SNAPSHOT_PATH})."
      return 0
    fi
    sleep 2
    timeout=$((timeout - 2))
  done
  err "App did not become healthy within 60s. Snapshot retained at ${SNAPSHOT_PATH}"
  return 1
}

# --- restore-only mode ---
if [ "$RESTORE_ONLY" = "1" ]; then
  restore_db
  exit $?
fi

# --- snapshot (or reuse) ---
if [ -s "${SNAPSHOT_PATH}" ]; then
  log "Reusing existing snapshot at ${SNAPSHOT_PATH}"
  log "(Delete it to force a fresh snapshot from the live DB.)"
else
  snapshot_db
fi

cleanup() {
  local exit_code=$?
  echo
  if [ "$KEEP" = "1" ]; then
    log "Skipping restore (--keep). Snapshot retained at ${SNAPSHOT_PATH}"
    log "Re-run with --keep to iterate, or --restore-only to roll back."
    return $exit_code
  fi
  restore_db || warn "Restore reported a problem; check above."
  return $exit_code
}
trap cleanup EXIT

log "Running: make ${TARGET} ${MAKE_ARGS[*]}"
make "${TARGET}" "${MAKE_ARGS[@]}"
