#!/usr/bin/env bash
# Deploy jim-dot-tennis app to production (jim.tennis).
#
# Rsyncs the Go source, templates, static assets, and migrations to the
# DigitalOcean server, then rebuilds the app image and restarts the
# container. Verifies the container reports healthy before returning.
#
# Why a script: the rsync set is easy to get wrong by hand. Skipping
# migrations/ in particular bakes stale migration files into the image
# and the app fails with "no such column" on startup.
#
# Usage:
#   scripts/deploy-app.sh [OPTIONS]
#
# Options:
#   -n, --dry-run     Run rsync with --dry-run, skip build/restart.
#   -s, --skip-build  Rsync only. Useful if you want to inspect changes
#                     on the server before triggering a rebuild.
#   -h, --help        Show this help.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

SSH_KEY="${SSH_KEY:-$HOME/.ssh/digital_ocean_ssh}"
SERVER="${SERVER:-root@144.126.228.64}"
REMOTE_PATH="${REMOTE_PATH:-/opt/jim-dot-tennis}"
APP_CONTAINER="jim-dot-tennis"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${GREEN}==>${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}==>${NC} %s\n" "$*" >&2; }
err()  { printf "${RED}==>${NC} %s\n" "$*" >&2; }

usage() {
  sed -n '/^# Deploy/,/^$/p' "$0" | sed -e 's/^# \{0,1\}//' -e '/^$/d'
  exit 0
}

DRY_RUN=0
SKIP_BUILD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -n|--dry-run)    DRY_RUN=1; shift ;;
    -s|--skip-build) SKIP_BUILD=1; shift ;;
    -h|--help)       usage ;;
    *)               err "Unknown option: $1"; exit 1 ;;
  esac
done

RSYNC_FLAGS="-avz --delete"
if [ "$DRY_RUN" = "1" ]; then
  RSYNC_FLAGS="$RSYNC_FLAGS --dry-run"
  warn "DRY RUN — no changes will be made"
fi

SSH_OPT="ssh -i $SSH_KEY"

# Required source dirs. Order doesn't matter for correctness, but keep
# migrations/ at the end so a forgotten one is the most-recent stderr.
DIRS=(internal cmd templates static migrations)

for dir in "${DIRS[@]}"; do
  if [ ! -d "$dir" ]; then
    err "Missing source dir: $dir"
    exit 1
  fi
  log "Syncing $dir/ → ${SERVER}:${REMOTE_PATH}/${dir}/"
  rsync $RSYNC_FLAGS -e "$SSH_OPT" "${dir}/" "${SERVER}:${REMOTE_PATH}/${dir}/"
done

if [ "$DRY_RUN" = "1" ]; then
  log "Dry run complete."
  exit 0
fi

if [ "$SKIP_BUILD" = "1" ]; then
  log "Skipping build/restart (--skip-build). Rsync done."
  exit 0
fi

log "Building app image on ${SERVER}..."
ssh -i "$SSH_KEY" "$SERVER" "cd ${REMOTE_PATH} && docker compose build app && docker compose up -d app"

log "Waiting for app to become healthy..."
timeout=60
while [ $timeout -gt 0 ]; do
  status=$(ssh -i "$SSH_KEY" "$SERVER" "docker inspect -f '{{.State.Health.Status}}' ${APP_CONTAINER}" 2>/dev/null || echo "missing")
  if [ "$status" = "healthy" ]; then
    log "App healthy."
    log "Smoke check: $(curl -s -o /dev/null -w 'HTTP %{http_code} in %{time_total}s' https://jim.tennis/)"
    exit 0
  fi
  sleep 2
  timeout=$((timeout - 2))
done

err "App did not become healthy within 60s. Check: ssh ${SERVER} 'docker logs --tail 50 ${APP_CONTAINER}'"
exit 1
