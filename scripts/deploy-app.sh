#!/usr/bin/env bash
# Maintainer tool: deploy a released version of jim-dot-tennis to the maintainer's
# own server over SSH. A club upgrades its own copy with docs/upgrade.md instead.
#
# The server runs the published image ghcr.io/jameshartt/jim-dot-tennis at a
# named version; nothing is built there and no source is copied to it. This
# script sets JIM_DOT_TENNIS_VERSION in the server's .env, pulls that image,
# restarts the app service only, waits for it to report healthy and checks
# that /version says the version asked for.
#
# The app backs up its own database before it runs any migration (a
# jim-dot-tennis-pre-migration-*.tar.gz in the backups folder on the data
# volume) and refuses to start if that backup cannot be made.
#
# Usage:
#   scripts/deploy-app.sh X.Y.Z       deploy that released version
#   scripts/deploy-app.sh --current   print the version the server runs now
#   scripts/deploy-app.sh --help
#
# The version is the image tag: no leading "v". Cut the release first (see
# RELEASING.md). Going back to an older version that used an older database
# is not just a version change: see "Roll back" in
# docs/archive/PRODUCTION_QUICK_REFERENCE.md.
#
# Compose/Caddy: this script does not sync docker-compose or the Caddyfile.
# Update those on the server by hand when needed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

log() { printf "${GREEN}==>${NC} %s\n" "$*"; }
err() { printf "${RED}==>${NC} %s\n" "$*" >&2; }

usage() {
  sed -n '/^# Deploy/,/^$/p' "$0" | sed -e 's/^# \{0,1\}//' -e '/^$/d'
  exit 0
}

[ $# -eq 1 ] || { err "Give one version, for example: scripts/deploy-app.sh 0.1.0"; exit 1; }
case "$1" in
  -h|--help) usage ;;
esac

# SERVER, SSH_KEY, REMOTE_PATH come from the environment or scripts/.deploy.env
# (see scripts/.deploy.env.example); exits with a message if unset.
. "$REPO_ROOT/scripts/lib-deploy-env.sh"
APP_CONTAINER="jim-dot-tennis"
IMAGE="ghcr.io/jameshartt/jim-dot-tennis"

remote() { ssh -i "$SSH_KEY" "$SERVER" "$@"; }

running_version() {
  remote "docker exec ${APP_CONTAINER} ./jim-dot-tennis version 2>/dev/null | head -1" || true
}

if [ "$1" = "--current" ]; then
  running_version
  exit 0
fi

VERSION="$1"
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  err "Not a version: $VERSION (expected X.Y.Z, no leading v)"
  exit 1
fi

PREVIOUS="$(running_version)"
log "Server runs now: ${PREVIOUS:-unknown}"

log "Pulling ${IMAGE}:${VERSION} on ${SERVER}..."
remote "docker pull -q ${IMAGE}:${VERSION}"

log "Setting JIM_DOT_TENNIS_VERSION=${VERSION} and restarting the app..."
remote "cd ${REMOTE_PATH} \
  && if grep -q '^JIM_DOT_TENNIS_VERSION=' .env; then \
       sed -i 's/^JIM_DOT_TENNIS_VERSION=.*/JIM_DOT_TENNIS_VERSION=${VERSION}/' .env; \
     else \
       printf '\nJIM_DOT_TENNIS_VERSION=%s\n' '${VERSION}' >> .env; \
     fi \
  && docker compose up -d --no-deps app"

log "Waiting for the app to become healthy..."
timeout=90
while [ $timeout -gt 0 ]; do
  status=$(remote "docker inspect -f '{{.State.Health.Status}}' ${APP_CONTAINER}" 2>/dev/null || echo "missing")
  if [ "$status" = "healthy" ]; then
    got="$(curl -fsS https://jim.tennis/version || true)"
    if [ "$got" != "$VERSION" ]; then
      err "App is healthy but https://jim.tennis/version says '${got}', not '${VERSION}'."
      exit 1
    fi
    log "jim.tennis runs ${VERSION} and is healthy."
    log "Previous version was: ${PREVIOUS:-unknown} (see 'Roll back' in docs/archive/PRODUCTION_QUICK_REFERENCE.md)"
    exit 0
  fi
  sleep 3
  timeout=$((timeout - 3))
done

err "App did not become healthy within 90s. Check: ssh \$SERVER 'docker logs --tail 50 ${APP_CONTAINER}'"
exit 1
