#!/usr/bin/env bash
# Shared loader for operator-specific deployment settings.
#
# Sourced by the deploy/maintenance scripts. Reads SERVER, SSH_KEY and
# REMOTE_PATH from the environment, falling back to an untracked
# scripts/.deploy.env (copy scripts/.deploy.env.example and fill it in).
# Exits non-zero with a clear message if any are missing.
#
#   SERVER       ssh target, e.g. user@host
#   SSH_KEY      path to the ssh private key
#   REMOTE_PATH  deployment directory on the server

_lib_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_deploy_env="${DEPLOY_ENV_FILE:-$_lib_dir/.deploy.env}"

# Environment variables win over the file.
_pre_server="${SERVER:-}"; _pre_key="${SSH_KEY:-}"; _pre_path="${REMOTE_PATH:-}"
if [ -f "$_deploy_env" ]; then
  # shellcheck disable=SC1090
  . "$_deploy_env"
fi
SERVER="${_pre_server:-${SERVER:-}}"
SSH_KEY="${_pre_key:-${SSH_KEY:-}}"
REMOTE_PATH="${_pre_path:-${REMOTE_PATH:-}}"

_missing=""
[ -z "$SERVER" ] && _missing="$_missing SERVER"
[ -z "$SSH_KEY" ] && _missing="$_missing SSH_KEY"
[ -z "$REMOTE_PATH" ] && _missing="$_missing REMOTE_PATH"
if [ -n "$_missing" ]; then
  {
    echo "Error: required deployment setting(s) not set:$_missing"
    echo "Set them in the environment, or copy scripts/.deploy.env.example to"
    echo "scripts/.deploy.env (untracked) and fill in your own server details."
  } >&2
  exit 1
fi
export SERVER SSH_KEY REMOTE_PATH
unset _lib_dir _deploy_env _pre_server _pre_key _pre_path _missing
