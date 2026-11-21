#!/usr/bin/env bash
# Passkey-enabled E2E run (the Playwright "auth" project).
#
# Uses the regular e2e app instance and volume (app-e2e / jim-dot-tennis-e2e-data),
# but the browser reaches it as http://localhost:8080 (the e2e-auth runner shares
# the app's network namespace), which Chromium treats as a secure context and
# the app accepts for passkeys in DEV_MODE. Never touches the dev/prod stack.
#
# Steps:
#   1. build, then recreate app-e2e (clears the in-memory sign-in throttling so
#      repeated runs are not locked out; the database volume is kept);
#   2. seed the database;
#   3. run `jim-dot-tennis user create` / `user reset` INSIDE the app container
#      and hand the printed recovery link to cli-reset.spec.ts via an env var
#      (the Playwright container cannot reach the Docker socket);
#   4. run `playwright test --project=auth`;
#   5. as a safety net, `admin-passkeys off` in the app container, then stop it.
set -uo pipefail
cd "$(dirname "$0")/.."

export MY_TENNIS_ENABLED=1
export HOME_CLUB_ID=1
DC=(docker compose --profile test)

"${DC[@]}" build app-e2e e2e-auth || exit 1
"${DC[@]}" up -d --force-recreate --wait app-e2e || { "${DC[@]}" logs --tail 40 app-e2e; exit 1; }

cleanup() {
  "${DC[@]}" exec -T app-e2e ./jim-dot-tennis admin-passkeys off >/dev/null 2>&1
  "${DC[@]}" stop app-e2e >/dev/null 2>&1
}
trap cleanup EXIT

"${DC[@]}" run --rm --no-deps e2e-auth sh /app/tests/fixtures/seed.sh >/dev/null || exit 1

# A fresh account for the CLI test, and a recovery link for it. The link is the
# only thing this script prints from the CLI, and it is a single-use e2e secret.
CLI_USER="e2e.cli.$(date +%s)"
"${DC[@]}" exec -T app-e2e ./jim-dot-tennis user create "$CLI_USER" --role captain >/dev/null || { echo "user create failed"; exit 1; }
CLI_OUT="$("${DC[@]}" exec -T app-e2e ./jim-dot-tennis user reset "$CLI_USER")" || { echo "user reset failed"; exit 1; }
CLI_LINK="$(printf '%s\n' "$CLI_OUT" | grep -o '/account/link/[A-Za-z0-9_-]*' | head -n1)"
[ -n "$CLI_LINK" ] || { echo "no link in 'user reset' output"; exit 1; }
export E2E_CLI_USER="$CLI_USER" E2E_CLI_RESET_LINK="$CLI_LINK"

"${DC[@]}" run --rm --no-deps e2e-auth sh -c "npx playwright test --project=auth --workers=1 $*"
