#!/usr/bin/env bash
# Deploy jim-dot-tennis app to production (jim.tennis).
#
# Rsyncs the Go source, templates, static assets, migrations, and the
# root build inputs (Dockerfile, Dockerfile.import, .dockerignore) to the
# DigitalOcean server, then backs up the DB, tags the current image for
# rollback, rebuilds the app image, restarts the container, and verifies
# it reports healthy before returning.
#
# Why a script: the rsync set is easy to get wrong by hand. Skipping
# migrations/ bakes stale migration files into the image (the app fails with
# "no such column" on startup); skipping the Dockerfile/.dockerignore means
# build changes silently never take effect on the server.
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

# Root-level build inputs. Without these, a Dockerfile/.dockerignore change
# never reaches the server and the rebuild silently uses the old build recipe;
# a go.mod/go.sum change (new dependency) breaks the server build outright.
# (Compose files are deliberately NOT synced — prod compose has diverged from
# the repo, so overwriting it would resurrect the wrong stack.)
FILES=(go.mod go.sum Dockerfile Dockerfile.import .dockerignore)

for file in "${FILES[@]}"; do
  if [ ! -f "$file" ]; then
    err "Missing build file: $file"
    exit 1
  fi
  log "Syncing $file → ${SERVER}:${REMOTE_PATH}/${file}"
  rsync $RSYNC_FLAGS -e "$SSH_OPT" "$file" "${SERVER}:${REMOTE_PATH}/${file}"
done

if [ "$DRY_RUN" = "1" ]; then
  log "Dry run complete."
  exit 0
fi

if [ "$SKIP_BUILD" = "1" ]; then
  log "Skipping build/restart (--skip-build). Rsync done."
  exit 0
fi

# Pre-deploy safety net: back up the sqlite DB and tag the current image so a
# bad build/migration can be rolled back. Both are best-effort — a fresh server
# with no prior image or DB shouldn't block the first deploy.
log "Taking pre-deploy DB backup on ${SERVER}..."
ssh -i "$SSH_KEY" "$SERVER" '
  docker run --rm -v jim-dot-tennis-data:/data -v jim-dot-tennis-backups:/backups alpine sh -c \
    "apk add --no-cache sqlite >/dev/null 2>&1 && ts=\$(date +%Y%m%d-%H%M%S) && sqlite3 /data/tennis.db \".backup /backups/pre-deploy-\$ts.db\" && echo pre-deploy-\$ts.db" \
  || echo "(backup skipped — volume/DB not present)"
'

log "Tagging current image as :prev for rollback..."
ssh -i "$SSH_KEY" "$SERVER" '
  img=$(docker inspect -f "{{.Config.Image}}" jim-dot-tennis 2>/dev/null) \
  && docker tag "$img" jim-dot-tennis-app:prev \
  && echo "rollback tag: jim-dot-tennis-app:prev -> $img" \
  || echo "(no running image to tag — first deploy?)"
'

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
