#!/bin/sh
# Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
#
# Creates the secrets the tournament stack needs and adds them to your .env
# file. Run it once, on the server, in the folder holding docker-compose.yml,
# before turning tournaments on:
#
#   ./init-tournaments.sh
#
# Every value is made here, on your server, with openssl. Nothing has a
# default and nothing is sent anywhere. The script:
#   - adds only the variables that .env does not have yet, and never changes
#     or removes a line that is already there, so it is safe to run again;
#   - checks every line first: if one is empty or a placeholder such as
#     change-me, it stops before writing anything;
#   - never prints a secret;
#   - makes .env readable by its owner only (mode 600).
#
# It needs only a POSIX shell and the openssl command, which every common
# Linux server has (Ubuntu, Debian, Alpine: package "openssl"). It runs openssl
# on the server itself rather than in a container so that it works before any
# image has been pulled and no secret passes through Docker.
#
# Variables it writes (see .env.example for what each is for):
#   COURTHIVE_PG_PASSWORD, COURTHIVE_JWT_SECRET, COURTHIVE_JWT_PRIVATE_KEY,
#   COURTHIVE_JWT_PUBLIC_KEY, COURTHIVE_JWT_KID, COURTHIVE_SIGNER_TOKEN,
#   COURTHIVE_SERVICE_PASSWORD
#
# Optional argument: the path of the .env file (default: .env next to this
# script).

set -eu

dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
env_file=${1:-"$dir/.env"}

fail() {
	printf 'init-tournaments: %s\n' "$*" >&2
	exit 1
}

if [ ! -f "$env_file" ]; then
	fail "$env_file not found. Create it first (cp .env.example .env, then fill in the REQUIRED values)."
fi

# The value of a variable in .env: the last line that sets it, without
# surrounding quotes. Prints nothing when it is not there.
current() {
	sed -n "s/^[[:space:]]*\(export[[:space:]]\{1,\}\)\{0,1\}$1=//p" "$env_file" | tail -n 1 |
		sed -e "s/^'\(.*\)'[[:space:]]*\$/\1/" -e 's/^"\(.*\)"[[:space:]]*$/\1/'
}

# 0 when the variable has a usable value; stops with a message when a line for
# it exists but is empty or still a placeholder (this script never edits
# existing lines); 1 when there is no line for it.
has() {
	if grep -Eq "^[[:space:]]*(export[[:space:]]+)?$1=" "$env_file"; then
		v=$(current "$1")
		case $v in
		'' | change-me | changeme | CHANGE-ME | '<'*'>')
			fail "$1 is in $env_file but has no real value. Delete that line and run this again."
			;;
		esac
		return 0
	fi
	return 1
}

hex() {
	openssl rand -hex "$1"
}

secrets="COURTHIVE_JWT_PRIVATE_KEY COURTHIVE_JWT_PUBLIC_KEY COURTHIVE_JWT_KID COURTHIVE_SIGNER_TOKEN COURTHIVE_SERVICE_PASSWORD COURTHIVE_PG_PASSWORD COURTHIVE_JWT_SECRET"

# Check every line first, so a leftover empty or placeholder line stops the
# script before anything is written (has stops with a message for those).
for var in $secrets; do
	has "$var" || :
done

# The key pair: both halves or neither. A pair cannot be completed from one
# half here without risking a mismatch, so a half pair is an error.
if has COURTHIVE_JWT_PRIVATE_KEY; then have_private=1; else have_private=0; fi
if has COURTHIVE_JWT_PUBLIC_KEY; then have_public=1; else have_public=0; fi
if [ "$have_private" -ne "$have_public" ]; then
	fail "$env_file has only one of COURTHIVE_JWT_PRIVATE_KEY and COURTHIVE_JWT_PUBLIC_KEY. Delete the one that is there (and COURTHIVE_JWT_KID) and run this again."
fi

command -v openssl >/dev/null 2>&1 || fail "openssl is not installed. Install it (for example: sudo apt install openssl) and run this again."

# Everything created from here on is private to this user.
umask 077
chmod 600 "$env_file"

added=0
header_written=0

# Appends VAR='value' to .env. Values never contain a single quote.
append() {
	if [ "$header_written" -eq 0 ]; then
		# Make sure the file ends with a newline before adding to it.
		if [ -s "$env_file" ] && [ "$(tail -c 1 "$env_file" | od -An -c | tr -d ' ')" != '\n' ]; then
			printf '\n' >>"$env_file"
		fi
		printf '\n# Tournament secrets, made on this server by init-tournaments.sh on %s.\n# Keep them private. Do not change them once tournaments are in use.\n' \
			"$(date -u +%Y-%m-%d)" >>"$env_file"
		header_written=1
	fi
	printf "%s='%s'\n" "$1" "$2" >>"$env_file"
	printf 'init-tournaments: added %s\n' "$1"
	added=$((added + 1))
}

if [ "$have_private" -eq 0 ]; then
	# ES256: an EC key on the P-256 curve. genpkey writes PKCS#8 PEM.
	private_pem=$(openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 2>/dev/null) ||
		fail "openssl could not create the key pair."
	public_pem=$(printf '%s\n' "$private_pem" | openssl pkey -pubout 2>/dev/null) ||
		fail "openssl could not derive the public key."
	# One line each, with \n where the line breaks were (the services expect
	# this form in an environment variable).
	private_line=$(printf '%s\n' "$private_pem" | awk '{ printf "%s\\n", $0 }')
	public_line=$(printf '%s\n' "$public_pem" | awk '{ printf "%s\\n", $0 }')
	append COURTHIVE_JWT_PRIVATE_KEY "$private_line"
	append COURTHIVE_JWT_PUBLIC_KEY "$public_line"
	private_pem='' public_pem='' private_line='' public_line=''
fi

has COURTHIVE_JWT_KID || append COURTHIVE_JWT_KID "jdt-$(date -u +%Y%m%d)-$(hex 4)"
has COURTHIVE_SIGNER_TOKEN || append COURTHIVE_SIGNER_TOKEN "$(hex 32)"
has COURTHIVE_SERVICE_PASSWORD || append COURTHIVE_SERVICE_PASSWORD "$(hex 24)"
has COURTHIVE_PG_PASSWORD || append COURTHIVE_PG_PASSWORD "$(hex 24)"
has COURTHIVE_JWT_SECRET || append COURTHIVE_JWT_SECRET "$(hex 32)"

chmod 600 "$env_file"

if [ "$added" -eq 0 ]; then
	echo "init-tournaments: nothing to do; every tournament secret is already in $env_file."
else
	echo "init-tournaments: added $added variable(s) to $env_file (mode 600)."
fi

if [ "$(current COMPOSE_PROFILES)" != tournaments ]; then
	cat <<'EOF'

Next, to turn tournaments on:
  1. Add this line to .env:   COMPOSE_PROFILES=tournaments
  2. Run:                      docker compose up -d
  3. In the app, as an administrator, switch on Tournaments in Club Settings.
EOF
fi
