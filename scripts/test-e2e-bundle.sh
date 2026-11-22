#!/usr/bin/env bash
# Bundle smoke test: the SHIPPED deploy/docker-compose.yml and deploy/Caddyfile,
# run end to end with Playwright, once with the tournaments profile on and once
# with it off (WI-147).
#
#   scripts/test-e2e-bundle.sh [on|off|both] [-- <extra playwright args>]
#
# What it does, per run (each starts from nothing and is removed afterwards):
#   1. builds the app image from this checkout (tagged locally) and the
#      Playwright runner image; the four CourtHive images must already exist
#      locally under the names in deploy/docker-compose.yml (BUNDLE_PULL=1
#      pulls them: CI does that; they are not published yet);
#   2. writes a THROWAWAY env file (deploy/.env.example with DOMAIN=localhost
#      so Caddy issues a certificate from its own CA), runs the real
#      deploy/init-tournaments.sh against it when the profile is on, and
#      never touches deploy/.env;
#   3. starts the shipped compose file plus deploy/docker-compose.e2e.yml (own
#      project name, container names reset, volumes renamed, one loopback
#      port, BHPLTA pages from a local file server);
#   4. runs the Playwright project bundle-on / bundle-off in a container on the
#      host network, reaching https://localhost:<port>;
#   5. on failure saves container logs to tests/e2e/bundle/artifacts/, then
#      removes the project and its volumes.
#
# Environment:
#   BUNDLE_PORT_ON / BUNDLE_PORT_OFF  loopback ports (default 18743 / 18744)
#   BUNDLE_APP_TAG                    local tag for the app image (default bundle-e2e)
#   BUNDLE_SKIP_BUILD=1               reuse the images from the last run
#   BUNDLE_PULL=1                     pull the four CourtHive images first
#   BUNDLE_KEEP=1                     do not tear down (prints the project name)
#   BUNDLE_NO_TEST=1                  bring the stack up, print its address, stop
set -uo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

MODE=both
if [ $# -gt 0 ] && [[ "$1" =~ ^(on|off|both)$ ]]; then
  MODE="$1"
  shift
fi
[ "${1:-}" = "--" ] && shift
PW_ARGS=("$@")

PORT_ON="${BUNDLE_PORT_ON:-18743}"
PORT_OFF="${BUNDLE_PORT_OFF:-18744}"
APP_TAG="${BUNDLE_APP_TAG:-bundle-e2e}"
RUNNER_IMAGE="jim-dot-tennis-e2e-bundle:latest"
ARTIFACTS="$ROOT/tests/e2e/bundle/artifacts"

COMPOSE_FILES=(-f "$ROOT/deploy/docker-compose.yml" -f "$ROOT/deploy/docker-compose.e2e.yml")
COURTHIVE_IMAGES=(
  "ghcr.io/jameshartt/courthive-server:4.19.1-jdt.1"
  "ghcr.io/jameshartt/courthive-hiveid:0.5.0-jdt.1"
  "ghcr.io/jameshartt/courthive-tmx:8.61.0-jdt.1"
  "ghcr.io/jameshartt/courthive-public:1.23.7-jdt.1"
)

if [ "${BUNDLE_SKIP_BUILD:-}" != 1 ]; then
  echo "== building the app image from this checkout and the runner image"
  docker build -q -t "ghcr.io/jameshartt/jim-dot-tennis:${APP_TAG}" . >/dev/null || exit 1
  docker build -q -t "$RUNNER_IMAGE" -f Dockerfile.e2e . >/dev/null || exit 1
fi
if [ "${BUNDLE_PULL:-}" = 1 ]; then
  for img in "${COURTHIVE_IMAGES[@]}"; do docker pull -q "$img" >/dev/null || exit 1; done
fi
if [ "$MODE" != off ]; then
  for img in "${COURTHIVE_IMAGES[@]}"; do
    docker image inspect "$img" >/dev/null 2>&1 || {
      echo "missing image $img (set BUNDLE_PULL=1 once the images are published)" >&2
      exit 1
    }
  done
fi

overall=0

run_profile() {
  local profile="$1" port="$2"
  local project="jdtbundle${profile}$(date +%s)$$"
  local tmp
  tmp="$(mktemp -d)"
  local envfile="$tmp/env"
  local logdir="$tmp/logs"
  mkdir -p "$logdir"
  export COMPOSE_PROJECT_NAME="$project" BUNDLE_PORT="$port" BUNDLE_RUNNER_IMAGE="$RUNNER_IMAGE"
  # A leftover COMPOSE_PROFILES in the caller's shell must not leak in.
  unset COMPOSE_PROFILES
  local dc=(docker compose -p "$project" --env-file "$envfile" "${COMPOSE_FILES[@]}")

  cp deploy/.env.example "$envfile"
  chmod 600 "$envfile"
  sed -i \
    -e "s|^DOMAIN=.*|DOMAIN=localhost|" \
    -e "s|^JIM_DOT_TENNIS_VERSION=.*|JIM_DOT_TENNIS_VERSION=${APP_TAG}|" \
    -e "s|^PUBLIC_URL=.*|PUBLIC_URL=https://localhost:${port}|" \
    "$envfile"
  if [ "$profile" = on ]; then
    ./deploy/init-tournaments.sh "$envfile" >/dev/null || { echo "init-tournaments.sh failed"; return 1; }
    echo "COMPOSE_PROFILES=tournaments" >>"$envfile"
  fi

  # Ready: the app and the long-running tournament services are healthy, Caddy
  # is up, and the one-shots finished with status 0.
  wait_ready() {
    local deadline=$((SECONDS + 300)) svc state
    local want=(app caddy)
    local oneshots=()
    if [ "$1" = on ]; then
      want+=(postgres redis courthive-server courthive-hiveid)
      oneshots=(courthive-service-account tmx-content public-content)
    fi
    while [ $SECONDS -lt $deadline ]; do
      local ok=1
      for svc in "${want[@]}"; do
        state="$("${dc[@]}" ps --format '{{.Health}}|{{.State}}' "$svc" 2>/dev/null | head -1)"
        case "$state" in
          healthy\|*) ;;
          \|running) [ "$svc" = caddy ] || ok=0 ;;
          *) ok=0 ;;
        esac
      done
      for svc in "${oneshots[@]}"; do
        state="$("${dc[@]}" ps -a --format '{{.State}} {{.ExitCode}}' "$svc" 2>/dev/null | head -1)"
        case "$state" in
          "exited 0") ;;
          "exited "*) echo "$svc failed: $state"; return 1 ;;
          *) ok=0 ;;
        esac
      done
      [ $ok = 1 ] && return 0
      sleep 2
    done
    return 1
  }

  teardown() {
    if [ "${BUNDLE_KEEP:-}" = 1 ]; then
      echo "kept: project $project (env file $envfile)"
      return
    fi
    "${dc[@]}" down -v --remove-orphans >/dev/null 2>&1
    rm -rf "$tmp"
  }
  save_logs() {
    mkdir -p "$ARTIFACTS/$profile"
    "${dc[@]}" ps -a >"$ARTIFACTS/$profile/ps.txt" 2>&1
    for svc in $("${dc[@]}" config --services 2>/dev/null); do
      # The setup code is a secret for the run only; leave it out of saved logs.
      "${dc[@]}" logs --no-color --no-log-prefix "$svc" 2>&1 | grep -v 'SETUP CODE' >"$ARTIFACTS/$profile/$svc.log"
    done
  }

  echo "== profile $profile: project $project, https://localhost:$port"
  # `up --wait` fails on the one-shot services that exit 0 (tmx-content,
  # public-content, courthive-service-account), so poll for what matters.
  if ! "${dc[@]}" up -d >"$tmp/up.log" 2>&1 || ! wait_ready "$profile"; then
    echo "stack did not come up:"
    tail -30 "$tmp/up.log"
    "${dc[@]}" ps -a
    save_logs
    teardown
    return 1
  fi
  "${dc[@]}" logs --no-color --no-log-prefix app >"$logdir/app.log" 2>&1
  grep -q 'SETUP CODE:' "$logdir/app.log" || { echo "no setup code in the app log"; save_logs; teardown; return 1; }

  if [ "${BUNDLE_NO_TEST:-}" = 1 ]; then
    echo "up: https://localhost:$port  logs: $logdir  project: $project"
    BUNDLE_KEEP=1 teardown
    return 0
  fi

  rm -rf "$ROOT/tests/e2e/bundle/test-results" "$ROOT/tests/e2e/bundle/playwright-report"
  docker run --rm --network host --user "$(id -u):$(id -g)" -e HOME=/tmp \
    -v "$logdir:/bundle-logs:ro" \
    -v "$ROOT/tests/e2e/bundle:/app/tests/bundle" \
    -v "$ROOT/tests/e2e/helpers:/app/tests/helpers:ro" \
    -e BUNDLE_URL="https://localhost:$port" \
    -e BUNDLE_PROFILE="$profile" \
    -e BUNDLE_LOG_DIR=/bundle-logs \
    -e CI="${CI:-}" \
    "$RUNNER_IMAGE" \
    npx playwright test -c bundle/playwright.config.ts --project="bundle-$profile" "${PW_ARGS[@]}"
  local rc=$?
  if [ $rc -ne 0 ]; then
    save_logs
    echo "container logs saved in $ARTIFACTS/$profile (setup code removed)"
  fi
  teardown
  return $rc
}

if [ "$MODE" = on ] || [ "$MODE" = both ]; then run_profile on "$PORT_ON" || overall=1; fi
if [ "$MODE" = off ] || [ "$MODE" = both ]; then run_profile off "$PORT_OFF" || overall=1; fi
exit $overall
