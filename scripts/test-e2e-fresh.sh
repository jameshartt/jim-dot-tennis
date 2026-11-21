#!/usr/bin/env bash
# Fresh-install E2E run: two app instances on brand-new EMPTY volumes (one for
# the main flow, one for the demo-data flow), a Playwright runner that serves
# the saved BHPLTA pages, and nothing shared with the regular e2e volume, the
# dev stack or the live BHPLTA site. Everything is created under a unique
# compose project name and removed again, so every run starts from nothing.
#
# The setup code each instance prints at startup is saved to a temporary log
# directory that the runner mounts read-only; the tests read the code from
# there. The directory is deleted at the end and nothing is written to the repo.
set -uo pipefail

cd "$(dirname "$0")/.."

RUN_ID="$(date +%s)$$"
export COMPOSE_PROJECT_NAME="jtfresh${RUN_ID}"
export FRESH_DATA_VOLUME="${COMPOSE_PROJECT_NAME}-data"
export FRESH_DEMO_VOLUME="${COMPOSE_PROJECT_NAME}-demo-data"
FRESH_LOG_DIR="$(mktemp -d)"
export FRESH_LOG_DIR
DC=(docker compose --profile fresh)

cleanup() {
  "${DC[@]}" down -v --remove-orphans >/dev/null 2>&1
  docker volume rm "$FRESH_DATA_VOLUME" "$FRESH_DEMO_VOLUME" >/dev/null 2>&1
  rm -rf "$FRESH_LOG_DIR"
}
trap cleanup EXIT

echo "Fresh-install E2E: project ${COMPOSE_PROJECT_NAME}"
"${DC[@]}" build app-fresh e2e-fresh || exit 1

# Empty volumes: the apps come up healthy in setup mode and print a setup code.
"${DC[@]}" up -d --wait app-fresh app-fresh-demo || { "${DC[@]}" logs app-fresh app-fresh-demo; exit 1; }
"${DC[@]}" logs --no-color --no-log-prefix app-fresh > "$FRESH_LOG_DIR/app-fresh.log"
"${DC[@]}" logs --no-color --no-log-prefix app-fresh-demo > "$FRESH_LOG_DIR/app-fresh-demo.log"
grep -q 'SETUP CODE:' "$FRESH_LOG_DIR/app-fresh.log" || { echo "no setup code in app-fresh log"; exit 1; }

"${DC[@]}" run --rm --use-aliases e2e-fresh
rc=$?

if [ $rc -ne 0 ]; then
  echo "--- app-fresh log (tail) ---"
  "${DC[@]}" logs --no-color --no-log-prefix --tail 40 app-fresh | grep -v 'SETUP CODE'
fi
exit $rc
